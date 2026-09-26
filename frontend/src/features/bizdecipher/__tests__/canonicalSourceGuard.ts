import { extname } from 'node:path'
import {
  ScriptKind,
  ScriptTarget,
  SyntaxKind,
  createSourceFile,
  forEachChild,
  isCallExpression,
  isExportDeclaration,
  isImportDeclaration,
  isStringLiteralLike,
  type Node,
} from 'typescript'

export type CanonicalSourceIssueCode =
  | 'legacy-implementation'
  | 'missing-canonical-source'
  | 'wrong-wrapper-target'
  | 'reverse-legacy-import'

export type CanonicalSourceIssue = {
  readonly code: CanonicalSourceIssueCode
  readonly file: string
  readonly detail: string
}

const VUE_WRAPPER_PATTERN =
  /^<script\s+lang=["']ts["']>\s*import\s+Component\s+from\s+["']([^"']+)["'];?\s*export\s+default\s+Component;?\s*<\/script>$/s

const TYPESCRIPT_WRAPPER_PATTERN = /^export\s+\*\s+from\s+["']([^"']+)["'];?$/s

function normalized(source: string): string {
  return source.replace(/^\uFEFF/, '').replaceAll('\r\n', '\n').trim()
}

function wrapperTarget(source: string, extension: string): string | undefined {
  const pattern = extension === '.vue' ? VUE_WRAPPER_PATTERN : TYPESCRIPT_WRAPPER_PATTERN
  const match = normalized(source).match(pattern)
  return match?.[1]
}

export function inspectLegacyWrapper(
  file: string,
  source: string,
  expectedTarget: string,
): readonly CanonicalSourceIssue[] {
  const target = wrapperTarget(source, extname(file))
  if (target === undefined) {
    return [
      {
        code: 'legacy-implementation',
        file,
        detail: `expected a thin wrapper for ${expectedTarget}`,
      },
    ]
  }
  if (target !== expectedTarget) {
    return [
      {
        code: 'wrong-wrapper-target',
        file,
        detail: `expected ${expectedTarget}, found ${target}`,
      },
    ]
  }
  return []
}

function scriptSource(file: string, source: string): string {
  if (extname(file) !== '.vue') return source
  return Array.from(source.matchAll(/<script\b[^>]*>([\s\S]*?)<\/script>/gi))
    .map((match) => match[1] ?? '')
    .join('\n')
}

function importedSpecifiers(file: string, source: string): ReadonlySet<string> {
  const parsed = createSourceFile(
    file,
    scriptSource(file, source),
    ScriptTarget.Latest,
    true,
    ScriptKind.TS,
  )
  const specifiers = new Set<string>()

  function visit(node: Node): void {
    if (
      (isImportDeclaration(node) || isExportDeclaration(node))
      && node.moduleSpecifier
      && isStringLiteralLike(node.moduleSpecifier)
    ) {
      specifiers.add(node.moduleSpecifier.text)
    }
    if (isCallExpression(node) && node.expression.kind === SyntaxKind.ImportKeyword) {
      const argument = node.arguments[0]
      if (argument && isStringLiteralLike(argument)) {
        specifiers.add(argument.text)
      }
    }
    forEachChild(node, visit)
  }

  visit(parsed)
  return specifiers
}

export function inspectCanonicalImports(
  file: string,
  source: string,
  deprecatedSpecifiers: readonly string[],
): readonly CanonicalSourceIssue[] {
  const imports = importedSpecifiers(file, source)
  return deprecatedSpecifiers
    .filter((specifier) => imports.has(specifier))
    .map((specifier) => ({
      code: 'reverse-legacy-import' as const,
      file,
      detail: `canonical source imports deprecated ${specifier}`,
    }))
}
