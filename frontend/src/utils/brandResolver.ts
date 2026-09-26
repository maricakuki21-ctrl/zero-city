import { sanitizeUrl } from '@/utils/url'

export const brandAssetVariants = ['lockup', 'mark', 'wordmark', 'tagline', 'favicon'] as const

export type BrandAssetVariant = typeof brandAssetVariants[number]

export type ResolvedBrandAsset = {
  readonly kind: 'platform' | 'tenant'
  readonly url: string
  readonly variant: BrandAssetVariant
}

export const platformBrandAssets: Readonly<Record<BrandAssetVariant, string>> = {
  lockup: '/brand/zero-wordmark.svg?v=zero-2',
  mark: '/brand/zero-mark.svg?v=zero-2',
  wordmark: '/brand/zero-wordmark.svg?v=zero-2',
  tagline: '/brand/bizdecipher-tagline.png',
  favicon: '/brand/zero-favicon.svg?v=zero-2'
}

const platformDefaultUrls = new Set<string>([
  ...Object.values(platformBrandAssets),
  '/brand/zero-mark.svg',
  '/brand/zero-wordmark.svg',
  '/brand/zero-favicon.svg',
  '/brand/bizdecipher-lockup.png',
  '/brand/bizdecipher-mark.png',
  '/brand/bizdecipher-wordmark.png',
  '/brand/bizdecipher-favicon.png',
  '/logo.svg',
  '/logo.png',
  '/favicon.svg',
  '/brand/bizdecipher-mark.svg',
  '/brand/bizdecipher-lockup.svg',
  '/brand/bizdecipher-wordmark.svg',
  '/brand/bizdecipher-favicon.svg',
  '/logo.svg?v=zero-city-light-shell-20260702'
])

function platformAsset(variant: BrandAssetVariant): ResolvedBrandAsset {
  return { kind: 'platform', url: platformBrandAssets[variant], variant }
}

export function resolveBrandAsset(
  configuredUrl: string | null | undefined,
  variant: BrandAssetVariant = 'mark'
): ResolvedBrandAsset {
  const candidate = configuredUrl?.trim() ?? ''
  if (!candidate || platformDefaultUrls.has(candidate)) {
    return platformAsset(variant)
  }

  const sanitizedUrl = sanitizeUrl(candidate, { allowRelative: true, allowDataUrl: true })
  if (sanitizedUrl.startsWith('/') && !sanitizedUrl.startsWith('//')) {
    try {
      const parsed = new URL(sanitizedUrl, 'https://branding.invalid')
      if (parsed.origin !== 'https://branding.invalid' || platformDefaultUrls.has(parsed.pathname)) {
        return platformAsset(variant)
      }
    } catch {
      return platformAsset(variant)
    }
  }
  return sanitizedUrl
    ? { kind: 'tenant', url: sanitizedUrl, variant }
    : platformAsset(variant)
}
