import { resolveBrandAsset } from '@/utils/brandResolver'

export function updateFavicon(configuredLogo: string | null | undefined): void {
  const brandAsset = resolveBrandAsset(configuredLogo, 'favicon')

  let link = document.querySelector<HTMLLinkElement>('link[rel="icon"]')
  if (!link) {
    link = document.createElement('link')
    link.rel = 'icon'
    document.head.appendChild(link)
  }

  const cleanLogoUrl = brandAsset.url.replace(/[?#].*$/, '')
  link.type = cleanLogoUrl.endsWith('.svg') ? 'image/svg+xml' : 'image/png'
  link.href = brandAsset.url
}
