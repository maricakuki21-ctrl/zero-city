import { createServer } from 'node:http'
import { randomUUID } from 'node:crypto'
import { runCreation } from './runtime.mjs'
import { publicCatalog, resolveSkills } from './catalog.mjs'

const upstream = process.env.BIZ_APP_URL || 'http://127.0.0.1:18081'
const active = new Set()
const maxActive = 2
function json(res, status, data) {
  res.writeHead(status, { 'content-type': 'application/json', 'cache-control': 'no-store' })
  res.end(JSON.stringify(data))
}
async function api(path, authorization, signal) {
  const response = await fetch(`${upstream}/api/v1${path}`, { headers: { authorization }, signal })
  if (!response.ok) throw new Error(`Account access rejected (${response.status})`)
  const envelope = await response.json()
  if (envelope.code !== 0 || !envelope.data) throw new Error('Account API returned an invalid response')
  return envelope.data
}
async function bodyOf(req) {
  let size = 0
  const chunks = []
  for await (const chunk of req) {
    size += chunk.length
    if (size > 32768) throw new Error('Task is too large')
    chunks.push(chunk)
  }
  return JSON.parse(Buffer.concat(chunks).toString())
}
function modelName(value) {
  if (typeof value !== 'string' || !value.trim() || value.length > 160 || /[\r\n]/.test(value)) throw new Error('Select a model')
  return value.trim()
}
async function keyFor(id, userId, authorization, signal) {
  if (!Number.isSafeInteger(id) || id <= 0) throw new Error('Select an API resource')
  const key = await api(`/keys/${id}`, authorization, signal)
  if (key.user_id !== userId || key.status !== 'active' || typeof key.key !== 'string' || !key.key) {
    throw new Error('Selected resource is unavailable')
  }
  return key.key
}
function customPlatformKey(value) {
  if (typeof value !== 'string' || !value.trim() || value.length > 512 || /[\r\n\s]/.test(value.trim())) throw new Error('Invalid resource key')
  return value.trim()
}

const server = createServer(async (req, res) => {
  const path = new URL(req.url, 'http://localhost').pathname
  if (req.method === 'GET' && path === '/health') return json(res, 200, { status: 'ok', runtime: 'harness', version: '0.1.3-alpha.2' })
  const authorization = req.headers.authorization
  if (!authorization?.startsWith('Bearer ')) return json(res, 401, { error: 'Login required' })
  const controller = new AbortController()
  const timeout = setTimeout(() => controller.abort(), 10 * 60 * 1000)
  res.on('close', () => { if (!res.writableEnded) controller.abort() })
  let userId
  let acquired = false
  try {
    const user = await api('/auth/me', authorization, controller.signal)
    userId = user.id
    if (!Number.isSafeInteger(userId) || userId <= 0) throw new Error('Invalid account')
    if (req.method === 'GET' && path === '/catalog') return json(res, 200, { skills: publicCatalog() })
    if ((req.method === 'GET' || req.method === 'POST') && path === '/models') {
      const id = Number(new URL(req.url, 'http://localhost').searchParams.get('key_id'))
      const key = req.method === 'POST' ? customPlatformKey((await bodyOf(req)).customKey) : await keyFor(id, userId, authorization, controller.signal)
      const response = await fetch(`${upstream}/v1/models`, { headers: { authorization: `Bearer ${key}` }, signal: controller.signal })
      if (!response.ok) throw new Error('Model catalog unavailable')
      const result = await response.json()
      const models = Array.isArray(result.data) ? result.data.filter(item => typeof item?.id === 'string').map(item => item.id) : []
      return json(res, 200, { models })
    }
    if (req.method !== 'POST' || path !== '/run') return json(res, 404, { error: 'Not found' })
    if (active.has(userId) || active.size >= maxActive) return json(res, 429, { error: '工作台正在执行任务，请稍后再试' })
    active.add(userId)
    acquired = true
    const body = await bodyOf(req)
    if (typeof body.intent !== 'string' || !body.intent.trim() || body.intent.length > 12000) throw new Error('请输入任务，最多 12000 字')
    const selected = resolveSkills(body.skillIds || [])
    const languageModel = modelName(body.languageModel)
    const languageKey = body.languageCustomKey ? customPlatformKey(body.languageCustomKey) : await keyFor(body.languageKeyId, userId, authorization, controller.signal)
    const imageModel = body.imageKeyId || body.imageCustomKey ? modelName(body.imageModel) : ''
    const imageKey = body.imageCustomKey ? customPlatformKey(body.imageCustomKey) : body.imageKeyId ? await keyFor(body.imageKeyId, userId, authorization, controller.signal) : ''
    const runId = randomUUID()
    res.writeHead(200, { 'content-type': 'application/x-ndjson', 'cache-control': 'no-store', 'x-accel-buffering': 'no' })
    const send = value => { if (!res.destroyed) res.write(`${JSON.stringify(value)}\n`) }
    send({ type: 'started', runId })
    const result = await runCreation({
      runId, baseURL: `${upstream}/v1`, languageKey, languageModel, imageKey, imageModel,
      skillIds: selected.map(skill => skill.id),
      intent: `${selected.length ? `本次启用技能：${selected.map(skill => skill.id).join(', ')}。先加载对应技能。\n\n` : ''}${body.intent}`,
    }, {
      signal: controller.signal,
      onEvent: envelope => {
        const event = envelope.event
        if (event?.type === 'tool/call') send({ type: 'tool', name: event.data?.name || 'tool', state: 'running' })
        if (event?.type === 'tool/result') send({ type: 'tool', name: 'tool', state: 'settled' })
      },
    })
    send({ type: 'completed', ...result })
    res.end()
  } catch (error) {
    const message = controller.signal.aborted ? '任务已停止；已产生的模型用量仍以账单为准' : '执行未完成，请检查模型、资源权限和余额。'
    // Full SDK errors can contain paths, prompts or provider diagnostics.
    console.error(JSON.stringify({ event: 'harness-run-failed', kind: error?.name || 'Error', aborted: controller.signal.aborted }))
    if (res.headersSent) {
      if (!res.destroyed) res.end(`${JSON.stringify({ type: 'failed', message })}\n`)
    } else json(res, 400, { error: message })
  } finally {
    clearTimeout(timeout)
    if (acquired) active.delete(userId)
  }
})
server.requestTimeout = 30000
server.listen(Number(process.env.PORT || 18082), '127.0.0.1', () => console.log('Harness runner listening on loopback'))
