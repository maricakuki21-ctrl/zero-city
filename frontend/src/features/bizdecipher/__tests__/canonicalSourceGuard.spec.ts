import {
  existsSync,
  mkdirSync,
  mkdtempSync,
  readFileSync,
  readdirSync,
  rmSync,
  writeFileSync,
} from 'node:fs'
import { tmpdir } from 'node:os'
import { extname, join, relative, resolve } from 'node:path'
import { afterEach, describe, expect, it } from 'vitest'
import {
  inspectCanonicalImports,
  inspectLegacyWrapper,
  type CanonicalSourceIssue,
} from './canonicalSourceGuard'

type DirectoryMapping = {
  readonly canonical: string
  readonly legacy: string
  readonly ownedPath?: RegExp
}

type SourcePair = {
  readonly canonicalFile: string
  readonly legacyFile: string
  readonly canonicalImport: string
  readonly legacyImport: string
}

const SOURCE_ROOT = resolve(process.cwd(), 'src')
const DIRECTORY_MAPPINGS: readonly DirectoryMapping[] = [
  { canonical: 'features/bizdecipher/views/landing', legacy: 'views/biz' },
  {
    canonical: 'features/bizdecipher/views/admin',
    legacy: 'views/admin',
    ownedPath: /^(?:CapabilityAssetsAdminView|SharedPoolGovernanceView|TavernScriptsAdminView)\.vue$|^sharedPoolGovernanceView\.ts$/,
  },
  {
    canonical: 'features/bizdecipher/views/user',
    legacy: 'views/user',
    ownedPath: /^(?:AccountSquare.*|CapabilityAssetsView|CommunityView|IncentivesView|PoolDetailView|PoolOwnerPoolDetailView|TavernStageView|TavernView|ZeroCityCardsView|ZeroCityProfileView)\.vue$|^sharedPoolSettings\.ts$/,
  },
  { canonical: 'features/bizdecipher/components/common', legacy: 'components/biz' },
  {
    canonical: 'features/bizdecipher/components/layout',
    legacy: 'components/layout',
    ownedPath: /^DailyFortuneGift\.vue$/,
  },
  { canonical: 'features/bizdecipher/components/shared-pool', legacy: 'components/shared-pool' },
  {
    canonical: 'features/bizdecipher/api',
    legacy: 'api',
    ownedPath: /^(?:bizdecipher|community|sharedPool[^/]*)\.ts$/,
  },
  {
    canonical: 'features/bizdecipher/constants',
    legacy: 'constants',
    ownedPath: /^zeroCityMascots\.ts$/,
  },
]

const PRODUCT_SOURCE_EXTENSIONS = new Set(['.ts', '.vue'])

function moduleSpecifier(file: string): string {
  const relativePath = relative(SOURCE_ROOT, file).replaceAll('\\', '/')
  return `@/${extname(file) === '.ts' ? relativePath.slice(0, -3) : relativePath}`
}

function sourceFiles(directory: string): readonly string[] {
  if (!existsSync(directory)) return []
  return readdirSync(directory, { withFileTypes: true }).flatMap((entry) => {
    if (entry.name === '__tests__') return []
    const path = join(directory, entry.name)
    if (entry.isDirectory()) return sourceFiles(path)
    if (!entry.isFile() || !PRODUCT_SOURCE_EXTENSIONS.has(extname(entry.name))) return []
    return [path]
  })
}

function isOwnedLegacySource(file: string, legacyDirectory: string, mapping: DirectoryMapping): boolean {
  const relativePath = relative(legacyDirectory, file).replaceAll('\\', '/')
  return mapping.ownedPath?.test(relativePath) ?? true
}

function discoverSourcePairs(
  sourceRoot = SOURCE_ROOT,
  mappings: readonly DirectoryMapping[] = DIRECTORY_MAPPINGS,
): readonly SourcePair[] {
  return mappings.flatMap((mapping) => {
    const canonicalDirectory = join(sourceRoot, mapping.canonical)
    const legacyDirectory = join(sourceRoot, mapping.legacy)
    return sourceFiles(legacyDirectory)
      .filter((legacyFile) => isOwnedLegacySource(legacyFile, legacyDirectory, mapping))
      .map((legacyFile) => {
        const canonicalFile = join(canonicalDirectory, relative(legacyDirectory, legacyFile))
        return {
          canonicalFile,
          legacyFile,
          canonicalImport: moduleSpecifier(canonicalFile),
          legacyImport: moduleSpecifier(legacyFile),
        }
      })
  })
}

function discoverCanonicalSources(
  sourceRoot: string,
  mappings: readonly DirectoryMapping[],
): readonly string[] {
  return mappings.flatMap((mapping) => sourceFiles(join(sourceRoot, mapping.canonical)))
}

function repositoryIssues(
  sourceRoot = SOURCE_ROOT,
  mappings: readonly DirectoryMapping[] = DIRECTORY_MAPPINGS,
): readonly CanonicalSourceIssue[] {
  const pairs = discoverSourcePairs(sourceRoot, mappings)
  const deprecatedSpecifiers = pairs.map((pair) => pair.legacyImport)
  const wrapperIssues = pairs.flatMap((pair) =>
    existsSync(pair.canonicalFile)
      ? inspectLegacyWrapper(
          relative(sourceRoot, pair.legacyFile),
          readFileSync(pair.legacyFile, 'utf8'),
          pair.canonicalImport,
        )
      : [{
          code: 'missing-canonical-source' as const,
          file: relative(sourceRoot, pair.legacyFile),
          detail: `missing ${relative(sourceRoot, pair.canonicalFile)}`,
        }],
  )
  const importIssues = discoverCanonicalSources(sourceRoot, mappings).flatMap((canonicalFile) =>
    inspectCanonicalImports(
      relative(sourceRoot, canonicalFile),
      readFileSync(canonicalFile, 'utf8'),
      deprecatedSpecifiers,
    ),
  )
  return [...wrapperIssues, ...importIssues]
}

const fixtureDirectories: string[] = []

function createBoundaryFixture(): {
  readonly sourceRoot: string
  readonly mapping: DirectoryMapping
} {
  const sourceRoot = mkdtempSync(join(tmpdir(), 'bizdecipher-source-guard-'))
  fixtureDirectories.push(sourceRoot)
  const mapping = {
    canonical: 'features/bizdecipher/views/user',
    legacy: 'views/user',
  } satisfies DirectoryMapping
  mkdirSync(join(sourceRoot, mapping.canonical), { recursive: true })
  mkdirSync(join(sourceRoot, mapping.legacy), { recursive: true })
  return { sourceRoot, mapping }
}

afterEach(() => {
  for (const directory of fixtureDirectories.splice(0)) {
    rmSync(directory, { recursive: true, force: true })
  }
})

describe('canonical source guard', () => {
  it('accepts a thin Vue compatibility wrapper', () => {
    // Given
    const source = `<script lang="ts">\nimport Component from '@/features/bizdecipher/views/user/ExampleView.vue'\n\nexport default Component\n</script>`

    // When
    const issues = inspectLegacyWrapper(
      'views/user/ExampleView.vue',
      source,
      '@/features/bizdecipher/views/user/ExampleView.vue',
    )

    // Then
    expect(issues).toEqual([])
  })

  it('rejects a substantive legacy Vue implementation', () => {
    // Given
    const source = '<template><main>duplicate implementation</main></template>'

    // When
    const issues = inspectLegacyWrapper(
      'views/user/ExampleView.vue',
      source,
      '@/features/bizdecipher/views/user/ExampleView.vue',
    )

    // Then
    expect(issues.map((issue) => issue.code)).toEqual(['legacy-implementation'])
  })

  it('rejects a wrapper targeting the wrong source', () => {
    // Given
    const source = `<script lang="ts">\nimport Component from '@/views/user/ExampleView.vue'\nexport default Component\n</script>`

    // When
    const issues = inspectLegacyWrapper(
      'views/user/ExampleView.vue',
      source,
      '@/features/bizdecipher/views/user/ExampleView.vue',
    )

    // Then
    expect(issues.map((issue) => issue.code)).toEqual(['wrong-wrapper-target'])
  })

  it('rejects canonical code importing a deprecated counterpart', () => {
    // Given
    const source = `import { loadPool } from '@/api/bizdecipher'`

    // When
    const issues = inspectCanonicalImports('features/bizdecipher/example.ts', source, [
      '@/api/bizdecipher',
    ])

    // Then
    expect(issues.map((issue) => issue.code)).toEqual(['reverse-legacy-import'])
  })

  it('rejects a side-effect import from canonical code to a deprecated counterpart', () => {
    // Given
    const source = `import '@/views/user/ExampleView.vue'`

    // When
    const issues = inspectCanonicalImports('features/bizdecipher/example.ts', source, [
      '@/views/user/ExampleView.vue',
    ])

    // Then
    expect(issues.map((issue) => issue.code)).toEqual(['reverse-legacy-import'])
  })

  it('rejects an unpaired product implementation in a legacy directory', () => {
    // Given
    const { sourceRoot, mapping } = createBoundaryFixture()
    writeFileSync(
      join(sourceRoot, mapping.legacy, 'OnlyLegacyView.vue'),
      '<template><main>legacy only</main></template>',
    )

    // When
    const issues = repositoryIssues(sourceRoot, [mapping])

    // Then
    expect(issues.map((issue) => issue.code)).toEqual(['missing-canonical-source'])
  })

  it('rejects a nested substantive legacy implementation', () => {
    // Given
    const { sourceRoot, mapping } = createBoundaryFixture()
    const canonicalDirectory = join(sourceRoot, mapping.canonical, 'nested')
    const legacyDirectory = join(sourceRoot, mapping.legacy, 'nested')
    mkdirSync(canonicalDirectory, { recursive: true })
    mkdirSync(legacyDirectory, { recursive: true })
    writeFileSync(join(canonicalDirectory, 'ExampleView.vue'), '<template><main>canonical</main></template>')
    writeFileSync(join(legacyDirectory, 'ExampleView.vue'), '<template><main>duplicate</main></template>')

    // When
    const issues = repositoryIssues(sourceRoot, [mapping])

    // Then
    expect(issues.map((issue) => issue.code)).toEqual(['legacy-implementation'])
  })

  it('keeps every matching legacy product source as a thin compatibility wrapper', () => {
    // Given / When
    const issues = repositoryIssues()

    // Then
    expect(issues, JSON.stringify(issues, null, 2)).toEqual([])
  })
})
