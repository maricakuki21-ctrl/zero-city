import { DeepSeekHarness } from '@deepseek-ai/dsh-sdk-client'
import { mkdir, readFile, rm } from 'node:fs/promises'
import { fileURLToPath } from 'node:url'
import { dirname, resolve } from 'node:path'
import { resolveSkills } from './catalog.mjs'

const patch = fileURLToPath(new URL('./creation.patch.yml', import.meta.url))
const root = fileURLToPath(new URL('../', import.meta.url))

export async function runCreation(input, { signal, onEvent = () => {}, patches = [] } = {}) {
  if (!/^[a-zA-Z0-9-]{1,80}$/.test(input.runId)) throw new Error('Invalid run ID')
  const selected = resolveSkills(input.skillIds)
  const runtimeRoot = resolve(root, '.runtime')
  const taskRoot = resolve(runtimeRoot, input.runId)
  if (dirname(taskRoot) !== runtimeRoot) throw new Error('Invalid task directory')
  await mkdir(runtimeRoot, { recursive: true })
  await mkdir(taskRoot, { mode: 0o700 })
  try {
  const env = {}
  for (const key of ['PATH', 'SystemRoot', 'WINDIR', 'TEMP', 'TMP', 'HOME', 'USERPROFILE', 'APPDATA', 'LOCALAPPDATA']) {
    if (process.env[key]) env[key] = process.env[key]
  }
  Object.assign(env, {
    BIZ_GATEWAY_URL: input.baseURL,
    BIZ_LANGUAGE_KEY: input.languageKey,
    BIZ_LANGUAGE_MODEL: input.languageModel,
    BIZ_IMAGE_KEY: input.imageKey || '',
    BIZ_IMAGE_MODEL: input.imageModel || '',
    BIZ_SKILL_IDS: JSON.stringify(selected.map(item => item.id)),
    BIZ_OUTPUT_FILE: resolve(taskRoot, 'artifacts.json'),
  })
  const harness = new DeepSeekHarness({
    profile: 'sdk-minimal',
    patches: [patch, ...patches],
    dshHome: resolve(taskRoot, 'home'),
    processCwd: root,
    cwd: taskRoot,
    provider: 'biz',
    model: input.languageModel,
    maxTokens: 4096,
    initializeTimeoutMs: 120000,
    env,
  })
  const abort = () => { void harness.close().catch(() => {}) }
  signal?.addEventListener('abort', abort, { once: true })
  try {
    if (signal?.aborted) throw new Error('Task cancelled before launch')
    const result = await harness.run(input.intent, {
      onNotification: event => {
        // Only durable session events are exposed; runtime diagnostics stay private.
        if (event.method === 'session.event') onEvent(event.params)
      },
    })
    if (signal?.aborted) throw new Error('Task cancelled')
    if (!result.finalResponse) throw new Error('Harness finished without an assistant response')
    let images = []
    try {
      const value = JSON.parse(await readFile(resolve(taskRoot, 'artifacts.json'), 'utf8'))
      if (Array.isArray(value) && value.every(item => typeof item === 'string')) images = value
    } catch (error) {
      if (error.code !== 'ENOENT') throw error
    }
    return { sessionId: result.sessionId, text: result.finalResponse, images }
  } finally {
    signal?.removeEventListener('abort', abort)
    await harness.close()
  }
  } finally {
    await rm(taskRoot, { recursive: true, force: true })
  }
}
