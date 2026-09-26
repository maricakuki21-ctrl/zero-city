export async function generateImage({ baseURL, apiKey, model, prompt, size, signal, fetcher = fetch }) {
  if (!apiKey || !model) throw new Error('Image resource is not configured')
  const response = await fetcher(`${baseURL}/images/generations`, {
    method: 'POST',
    headers: { authorization: `Bearer ${apiKey}`, 'content-type': 'application/json' },
    body: JSON.stringify({ model, prompt, size, n: 1, response_format: 'url' }),
    signal,
  })
  if (!response.ok) throw new Error(`Image gateway rejected request (${response.status}); check resource permissions and balance`)
  const result = await response.json()
  const images = Array.isArray(result?.data) ? result.data : []
  const urls = images.flatMap(item => {
    if (typeof item?.url !== 'string') return []
    try {
      const url = new URL(item.url)
      return ['https:', 'http:'].includes(url.protocol) ? [url.href] : []
    } catch { return [] }
  })
  if (!urls.length) throw new Error('Gateway returned no downloadable image URL; no image delivery was recorded')
  return { images: urls, model }
}
