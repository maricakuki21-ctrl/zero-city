import { describe, expect, it } from 'vitest'
import { platformBrandAssets, resolveBrandAsset } from '@/utils/brandResolver'

describe('resolveBrandAsset', () => {
  it('returns the platform family for empty and exact legacy defaults', () => {
    for (const legacyValue of ['', '/logo.svg', '/logo.png', '/favicon.svg', '/logo.svg?v=zero-city-light-shell-20260702', '/logo.svg?v=legacy#mark', '/brand/bizdecipher-mark.svg?v=brand-v1', '/brand/bizdecipher-mark.png?v=old', '/brand/bizdecipher-lockup.png']) {
      expect(resolveBrandAsset(legacyValue, 'mark')).toEqual({
        kind: 'platform',
        url: platformBrandAssets.mark,
        variant: 'mark'
      })
    }

    expect(resolveBrandAsset(platformBrandAssets.favicon, 'wordmark')).toEqual({
      kind: 'platform',
      url: platformBrandAssets.wordmark,
      variant: 'wordmark'
    })
  })

  it('preserves valid tenant-relative, remote and data-image URLs', () => {
    expect(resolveBrandAsset('/customer/logo.svg', 'wordmark').kind).toBe('tenant')
    expect(resolveBrandAsset('https://customer.example/logo.png', 'wordmark').url).toBe('https://customer.example/logo.png')
    expect(resolveBrandAsset('data:image/svg+xml;base64,PHN2Zy8+', 'favicon').kind).toBe('tenant')
  })
  it('upgrades cached platform logos but preserves custom logos', () => {
    for (const logo of ['/brand/zero-mark.svg', '/brand/zero-mark.svg?v=old', '/brand/zero-wordmark.svg']) {
      expect(resolveBrandAsset(logo).url).toBe('/brand/zero-mark.svg?v=zero-2')
    }
    expect(resolveBrandAsset('/customer/logo.svg?v=old').url).toBe('/customer/logo.svg?v=old')
    expect(resolveBrandAsset('/brand/zero-mark.svg', 'favicon').url).toBe('/brand/zero-favicon.svg?v=zero-2')
  })

  it('falls back safely for dangerous schemes without broad logo.png matching', () => {
    expect(resolveBrandAsset('javascript:alert(1)', 'favicon')).toEqual({
      kind: 'platform',
      url: platformBrandAssets.favicon,
      variant: 'favicon'
    })
    expect(resolveBrandAsset('https://customer.example/logo.png', 'favicon').kind).toBe('tenant')
    expect(resolveBrandAsset('https://customer.example/logo.svg?v=legacy', 'favicon').kind).toBe('tenant')
    expect(resolveBrandAsset('/customer/logo.svg?v=legacy', 'favicon').kind).toBe('tenant')
  })

  it.each(['/\\%', '/\\[', '/\\outside.example/logo.svg'])('handles malformed or cross-origin relative logo %s without throwing', value => {
    expect(resolveBrandAsset(value, 'mark')).toEqual({
      kind: 'platform',
      url: platformBrandAssets.mark,
      variant: 'mark',
    })
  })
})
