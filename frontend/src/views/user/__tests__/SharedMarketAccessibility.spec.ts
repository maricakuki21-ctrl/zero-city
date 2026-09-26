import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const testDir = dirname(fileURLToPath(import.meta.url))
const canonicalUserViews = resolve(testDir, '../../../features/bizdecipher/views/user')
const marketSource = readFileSync(resolve(canonicalUserViews, 'AccountSquareView.vue'), 'utf8')
const memberSource = readFileSync(resolve(canonicalUserViews, 'AccountSquareMyView.vue'), 'utf8')
const ownerSource = readFileSync(resolve(canonicalUserViews, 'AccountSquareOwnerView.vue'), 'utf8')

describe('shared market accessibility contracts', () => {
  it('implements keyboard-operable tab patterns for market and member views', () => {
    expect(marketSource).toContain(':aria-expanded="filtersOpen"')
    expect(marketSource).toContain('aria-controls="shared-market-filters"')
    expect(marketSource).toContain('class="as-source-segments" role="group"')
    for (const contract of ['role="tablist"', 'role="tab"', ':aria-selected=', ':aria-controls=', ':tabindex=', 'role="tabpanel"']) {
      expect(memberSource).toContain(contract)
    }
    expect(memberSource).toContain('handleMemberTabKeydown')
  })

  it('distinguishes request failures from true empty states', () => {
    expect(marketSource).toContain('role="alert"')
    expect(marketSource).toContain('loadError')
    expect(ownerSource).toContain('class="asow-load-error"')
    expect(ownerSource).toContain('role="alert"')
  })

  it('protects critical Chinese phrases from awkward mobile breaks', () => {
    expect(marketSource).toContain('class="as-keep-phrase"')
    expect(ownerSource).toContain('class="asow-keep-phrase"')
  })
})
