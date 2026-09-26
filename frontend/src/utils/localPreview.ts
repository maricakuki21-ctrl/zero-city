export const LOCAL_PREVIEW_TOKEN = 'preview-token-local-only'

export function isLocalPreviewHost(): boolean {
  if (typeof window === 'undefined') {
    return false
  }
  return ['localhost', '127.0.0.1', '::1'].includes(window.location.hostname)
}

export function isLocalPreviewAuth(): boolean {
  if (!import.meta.env.DEV || typeof localStorage === 'undefined') {
    return false
  }
  return isLocalPreviewHost() && localStorage.getItem('auth_token') === LOCAL_PREVIEW_TOKEN
}
