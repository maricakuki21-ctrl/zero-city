import { test } from 'node:test'
import assert from 'node:assert/strict'
import { generateImage } from '../src/media.mjs'
import { resolveSkills, publicCatalog } from '../src/catalog.mjs'

test('catalog only resolves known skills and hides instruction bodies', () => {
  assert.equal(resolveSkills(['commerce-image-director']).length, 1)
  assert.throws(() => resolveSkills(['arbitrary-upload']))
  assert.equal('content' in publicCatalog()[0], false)
})
test('image tool sends selected credentials and returns only actual URLs', async () => {
  let sent
  const result = await generateImage({
    baseURL: 'http://localhost/v1', apiKey: 'test-only', model: 'test-image',
    prompt: 'test', size: '1024x1024',
    fetcher: async (url, options) => {
      sent = { url, options }
      return Response.json({ data: [{ url: 'https://example.test/image.png' }] })
    },
  })
  assert.equal(sent.options.headers.authorization, 'Bearer test-only')
  assert.equal(JSON.parse(sent.options.body).n, 1)
  assert.deepEqual(result.images, ['https://example.test/image.png'])
})
test('missing output and gateway errors cannot become successful images', async () => {
  const input = { baseURL: 'http://localhost/v1', apiKey: 'test-only', model: 'test-image', prompt: 'test', size: '1024x1024' }
  await assert.rejects(generateImage({ ...input, fetcher: async () => Response.json({ data: [] }) }))
  await assert.rejects(generateImage({ ...input, fetcher: async () => new Response('', { status: 403 }) }))
})
