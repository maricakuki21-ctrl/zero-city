let pending: Promise<void> | null = null

export function loadTurnstileScript(): Promise<void> {
  if (window.turnstile) return Promise.resolve()
  if (pending) return pending
  pending = new Promise<void>((resolve, reject) => {
    let script = document.querySelector<HTMLScriptElement>('script[src*="challenges.cloudflare.com/turnstile/"]')
    const owned = !script || script.dataset.bizTurnstile === 'true'
    const created = !script
    if (!script) {
      script = document.createElement('script')
      script.src = 'https://challenges.cloudflare.com/turnstile/v0/api.js?render=explicit'
      script.async = true
      script.dataset.bizTurnstile = 'true'
    }
    const element = script
    let settled = false
    const finish = (error?: Error) => {
      if (settled) return
      settled = true
      clearTimeout(timeout)
      element.removeEventListener('load', loaded)
      element.removeEventListener('error', failed)
      if (error) {
        if (owned) element.remove()
        reject(error)
      } else {
        resolve()
      }
    }
    const loaded = () => finish(window.turnstile ? undefined : new Error('Turnstile API is unavailable'))
    const failed = () => finish(new Error('Failed to load Turnstile script'))
    const timeout = setTimeout(() => finish(new Error('Turnstile loading timed out')), 15000)
    element.addEventListener('load', loaded)
    element.addEventListener('error', failed)
    if (created) document.head.appendChild(element)
  }).catch(error => {
    pending = null
    throw error
  })
  return pending
}
