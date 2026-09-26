import { createServer } from 'node:http'
import { randomUUID } from 'node:crypto'
import { fileURLToPath } from 'node:url'
import assert from 'node:assert/strict'
import { access } from 'node:fs/promises'
import { runCreation } from '../src/runtime.mjs'

let calls = 0
let images = 0
let toolNames = []
const server = createServer(async (req, res) => {
  const chunks = []
  for await (const chunk of req) chunks.push(chunk)
  const body = JSON.parse(Buffer.concat(chunks).toString() || '{}')
  if (req.url === '/v1/images/generations') {
    images += 1
    assert.equal(body.model, 'fixture-image')
    res.setHeader('content-type', 'application/json')
    res.end(JSON.stringify({ data: [{ url: 'https://example.test/fixture-image.png' }] }))
    return
  }
  assert.equal(req.url, '/v1/chat/completions')
  calls += 1
  toolNames = body.tools?.map(tool => tool.function.name) || []
  const step = calls === 1
    ? { tool_calls: [{ index: 0, id: 'fixture-skill', type: 'function', function: { name: 'skill', arguments: '{"name":"commerce-image-director"}' } }] }
    : calls === 2
      ? { tool_calls: [{ index: 0, id: 'fixture-image', type: 'function', function: { name: 'generate_image', arguments: '{"prompt":"fixture image","size":"1024x1024"}' } }] }
      : { content: 'Fixture completed.' }
  res.setHeader('content-type', 'text/event-stream')
  const payload = delta => ({ id: 'fixture', object: 'chat.completion.chunk', created: 1, model: 'fixture-language', choices: [{ index: 0, delta, finish_reason: null }] })
  res.write(`data: ${JSON.stringify(payload({ role: 'assistant', ...step }))}\n\n`)
  res.write(`data: ${JSON.stringify({ ...payload({}), choices: [{ index: 0, delta: {}, finish_reason: calls < 3 ? 'tool_calls' : 'stop' }], usage: { prompt_tokens: 10, completion_tokens: 10, total_tokens: 20 } })}\n\n`)
  res.end('data: [DONE]\n\n')
})
await new Promise(resolve => server.listen(0, '127.0.0.1', resolve))
const runId = randomUUID()
try {
  const result = await runCreation({
    runId, baseURL: `http://127.0.0.1:${server.address().port}/v1`,
    languageKey: 'fixture-only', languageModel: 'fixture-language',
    imageKey: 'fixture-only', imageModel: 'fixture-image',
    skillIds: ['commerce-image-director'], intent: 'Create a product image.',
  }, { signal: AbortSignal.timeout(180000), patches: process.platform === 'win32' ? [fileURLToPath(new URL('./memory.patch.yml', import.meta.url))] : [] })
  assert.equal(result.text, 'Fixture completed.')
  assert.equal(images, 1)
  assert.deepEqual(toolNames.sort(), ['generate_image', 'skill'])
  await assert.rejects(access(fileURLToPath(new URL(`../.runtime/${runId}`, import.meta.url))), { code: 'ENOENT' })
  const failedRunId = randomUUID()
  await assert.rejects(runCreation({
    runId: failedRunId, baseURL: `http://127.0.0.1:${server.address().port}/v1`,
    languageKey: 'fixture-failure-key', languageModel: 'fixture-language',
    skillIds: [], intent: 'Initialization failure fixture.',
  }, { signal: AbortSignal.timeout(30000), patches: [fileURLToPath(new URL('./missing-fixture.patch.yml', import.meta.url))] }))
  await assert.rejects(access(fileURLToPath(new URL(`../.runtime/${failedRunId}`, import.meta.url))), { code: 'ENOENT' })
  console.log(JSON.stringify({ kind: 'fixture-gateway-real-harness', calls, images, toolNames, passed: true }))
} finally {
  server.closeAllConnections()
  await new Promise(resolve => server.close(resolve))
}
