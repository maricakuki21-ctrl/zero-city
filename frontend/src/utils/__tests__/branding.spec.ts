import { beforeEach, describe, expect, it } from 'vitest'
import { updateFavicon } from '@/utils/branding'
import { platformBrandAssets } from '@/utils/brandResolver'

describe('updateFavicon', () => {
  beforeEach(() => {
    document.head.innerHTML = `<link rel="icon" href="${platformBrandAssets.favicon}">`
  })

  it('replaces the default favicon with the configured logo', () => {
    updateFavicon('https://example.com/custom-logo.png')

    const link = document.querySelector<HTMLLinkElement>('link[rel="icon"]')
    expect(link?.href).toBe('https://example.com/custom-logo.png')
  })

  it('ignores unsafe logo URLs', () => {
    updateFavicon('javascript:alert(1)')

    const link = document.querySelector<HTMLLinkElement>('link[rel="icon"]')
    expect(link?.getAttribute('href')).toBe(platformBrandAssets.favicon)
  })

  it('keeps the SVG MIME type for a valid tenant SVG URL with a query or hash', () => {
    updateFavicon('/customer/logo.svg?v=brand#mark')

    const link = document.querySelector<HTMLLinkElement>('link[rel="icon"]')
    expect(link?.type).toBe('image/svg+xml')
    expect(link?.getAttribute('href')).toBe('/customer/logo.svg?v=brand#mark')
  })
})
