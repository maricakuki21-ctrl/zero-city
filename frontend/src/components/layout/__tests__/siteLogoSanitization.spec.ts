import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'
import { platformBrandAssets, resolveBrandAsset } from '@/utils/brandResolver'

const dir = dirname(fileURLToPath(import.meta.url))
const sidebarSource = readFileSync(resolve(dir, '../AppSidebar.vue'), 'utf8')
const bizPublicLayoutSource = readFileSync(
  resolve(dir, '../../../features/bizdecipher/components/common/BizPublicLayout.vue'),
  'utf8',
)
const homeViewSource = readFileSync(resolve(dir, '../../../views/HomeView.vue'), 'utf8')
const keyUsageViewSource = readFileSync(resolve(dir, '../../../views/KeyUsageView.vue'), 'utf8')

describe('site_logo sanitization', () => {
  it('routes every site logo surface through the shared safe brand resolver', () => {
    for (const source of [sidebarSource, homeViewSource, keyUsageViewSource, bizPublicLayoutSource]) {
      expect(source).toContain("from '@/utils/brandResolver'")
      expect(source).toContain('resolveBrandAsset(appStore.')
      expect(source).toContain(':src="siteLogo"')
    }
  })

  it.each(['/logo.svg?v=old', '/logo.svg#mark', '/brand/bizdecipher-mark.svg?v=brand-v1'])(
    'maps legacy platform asset %s to the current brand family',
    logo => {
      expect(resolveBrandAsset(logo)).toMatchObject({ kind: 'platform', url: platformBrandAssets.mark })
    },
  )

  it.each(['javascript:alert(1)', 'data:text/html,<script>alert(1)</script>', 'vbscript:msgbox(1)', '//evil.example/logo.png', 'file:///etc/passwd'])(
    'rejects unsafe logo input %s',
    logo => {
      expect(resolveBrandAsset(logo)).toMatchObject({ kind: 'platform', url: platformBrandAssets.mark })
    },
  )

  it.each(['/customer/logo.svg', '/customer/logo.png?v=2', 'https://customer.example/logo.png', 'data:image/png;base64,aGVsbG8=', 'data:image/svg+xml;base64,PHN2Zy8+'])(
    'preserves a valid tenant logo %s',
    logo => {
      expect(resolveBrandAsset(logo)).toMatchObject({ kind: 'tenant', url: logo })
    },
  )

  it('keeps platform styling dependent on the resolver identity instead of a filename substring', () => {
    expect(sidebarSource).toContain("resolvedSiteLogo.value.kind === 'platform'")
    expect(sidebarSource).not.toContain("logo.includes('logo.png')")
  })
})
