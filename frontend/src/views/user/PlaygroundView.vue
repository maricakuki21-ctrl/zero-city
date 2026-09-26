<template>
  <AppLayout>
    <div class="pg-shell flex h-[calc(100vh-120px)] gap-0 overflow-hidden rounded-xl">
      <!-- Left: Session List -->
      <aside class="pg-rail hidden w-64 flex-shrink-0 border-r lg:block">
        <div class="pg-rail-head flex h-14 items-center justify-between border-b px-4">
          <h3 class="pg-title text-sm font-semibold">{{ t('workbench.sessions') }}</h3>
          <button class="btn btn-ghost btn-sm" @click="createSession">
            <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
              <path stroke-linecap="round" stroke-linejoin="round" d="M12 4.5v15m7.5-7.5h-15" />
            </svg>
          </button>
        </div>
        <div class="overflow-y-auto p-2">
          <div
            v-for="session in sessions"
            :key="session.id"
            class="pg-session flex cursor-pointer items-center gap-3 rounded-lg px-3 py-2.5 text-sm transition-colors"
            :class="currentSessionId === session.id ? 'pg-session-active' : 'pg-session-idle'"
            @click="currentSessionId = session.id"
          >
            <svg class="h-4 w-4 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
              <path stroke-linecap="round" stroke-linejoin="round" d="M8.625 12a.375.375 0 11-.75 0 .375.375 0 01.75 0zm0 0H8.25m4.125 0a.375.375 0 11-.75 0 .375.375 0 01.75 0zm0 0H12m4.125 0a.375.375 0 11-.75 0 .375.375 0 01.75 0zm0 0h-.375M21 12c0 4.556-4.03 8.25-9 8.25a9.764 9.764 0 01-2.555-.337A5.972 5.972 0 015.41 20.97a5.969 5.969 0 01-.474-.065 4.48 4.48 0 00.978-2.025c.09-.457-.133-.901-.467-1.226C3.93 16.178 3 14.189 3 12c0-4.556 4.03-8.25 9-8.25s9 3.694 9 8.25z" />
            </svg>
            <span class="truncate">{{ session.name }}</span>
          </div>
          <div v-if="!sessions.length" class="pg-muted px-3 py-8 text-center text-xs">
            {{ t('workbench.noSessions') }}
          </div>
        </div>
      </aside>

      <!-- Center: Chat Area -->
      <div class="pg-main flex flex-1 flex-col">
        <!-- Chat Header -->
        <div class="pg-main-head flex h-14 items-center justify-between border-b px-4">
          <div class="flex items-center gap-3">
            <button class="btn btn-ghost btn-icon lg:hidden" @click="showMobileSidebar = true">
              <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                <path stroke-linecap="round" stroke-linejoin="round" d="M3.75 6.75h16.5M3.75 12h16.5m-16.5 5.25h16.5" />
              </svg>
            </button>
            <div>
              <h2 class="pg-title text-sm font-semibold">{{ currentSession?.name || t('workbench.newChat') }}</h2>
              <p class="pg-muted text-xs">{{ model }}</p>
            </div>
          </div>
          <div class="flex items-center gap-2">
            <button class="btn btn-ghost btn-sm" @click="clearMessages">
              <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                <path stroke-linecap="round" stroke-linejoin="round" d="M14.74 9l-.346 9m-4.788 0L9.26 9m9.968-3.21c.342.052.682.107 1.022.166m-1.022-.165L18.16 19.673a2.25 2.25 0 01-2.244 2.077H8.084a2.25 2.25 0 01-2.244-2.077L4.772 5.79m14.456 0a48.108 48.108 0 00-3.478-.397m-12 .562c.34-.059.68-.114 1.022-.165m0 0a48.11 48.11 0 013.478-.397m7.5 0v-.916c0-1.18-.91-2.164-2.09-2.201a51.964 51.964 0 00-3.32 0c-1.18.037-2.09 1.022-2.09 2.201v.916m7.5 0a48.667 48.667 0 00-7.5 0" />
              </svg>
              {{ t('workbench.clear') }}
            </button>
            <button class="btn btn-ghost btn-sm" @click="exportConversation">
              <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                <path stroke-linecap="round" stroke-linejoin="round" d="M3 16.5v2.25A2.25 2.25 0 005.25 21h13.5A2.25 2.25 0 0021 18.75V16.5M16.5 12L12 16.5m0 0L7.5 12m4.5 4.5V3" />
              </svg>
              {{ t('workbench.export') }}
            </button>
          </div>
        </div>

        <!-- Messages Area -->
        <div ref="messagesContainer" class="flex-1 overflow-y-auto p-4">
          <div v-if="!messages.length" class="flex h-full items-center justify-center">
            <div class="text-center">
              <div class="pg-empty-icon mx-auto mb-4 flex h-16 w-16 items-center justify-center rounded-2xl">
                <svg class="h-8 w-8" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M8.625 12a.375.375 0 11-.75 0 .375.375 0 01.75 0zm0 0H8.25m4.125 0a.375.375 0 11-.75 0 .375.375 0 01.75 0zm0 0H12m4.125 0a.375.375 0 11-.75 0 .375.375 0 01.75 0zm0 0h-.375M21 12c0 4.556-4.03 8.25-9 8.25a9.764 9.764 0 01-2.555-.337A5.972 5.972 0 015.41 20.97a5.969 5.969 0 01-.474-.065 4.48 4.48 0 00.978-2.025c.09-.457-.133-.901-.467-1.226C3.93 16.178 3 14.189 3 12c0-4.556 4.03-8.25 9-8.25s9 3.694 9 8.25z" />
                </svg>
              </div>
              <h3 class="pg-title text-lg font-semibold">{{ t('workbench.startChat') }}</h3>
              <p class="pg-muted mt-2 max-w-sm text-sm">{{ t('workbench.startChatDesc') }}</p>
            </div>
          </div>

          <div v-else class="space-y-4">
            <div v-for="(msg, index) in messages" :key="index" class="flex gap-3" :class="msg.role === 'user' ? 'justify-end' : 'justify-start'">
              <div v-if="msg.role === 'assistant'" class="pg-avatar flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-lg">
                <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M9.813 15.904L9 18.75l-.813-2.846a4.5 4.5 0 00-3.09-3.09L2.25 12l2.846-.813a4.5 4.5 0 003.09-3.09L9 5.25l.813 2.846a4.5 4.5 0 003.09 3.09L15.75 12l-2.846.813a4.5 4.5 0 00-3.09 3.09zM18.259 8.715L18 9.75l-.259-1.035a3.375 3.375 0 00-2.455-2.456L14.25 6l1.036-.259a3.375 3.375 0 002.455-2.456L18 2.25l.259 1.035a3.375 3.375 0 002.456 2.456L21.75 6l-1.035.259a3.375 3.375 0 00-2.456 2.456z" />
                </svg>
              </div>
              <div
                class="pg-message max-w-[80%] rounded-xl px-4 py-3 text-sm leading-relaxed"
                :class="msg.role === 'user' ? 'pg-message-user' : 'pg-message-assistant'"
              >
                <pre class="whitespace-pre-wrap font-sans">{{ msg.content }}</pre>
              </div>
              <div v-if="msg.role === 'user'" class="pg-avatar flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-lg">
                <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M15.75 6a3.75 3.75 0 11-7.5 0 3.75 3.75 0 017.5 0zM4.501 20.118a7.5 7.5 0 0114.998 0A17.933 17.933 0 0112 21.75c-2.676 0-5.216-.584-7.499-1.632z" />
                </svg>
              </div>
            </div>
          </div>
        </div>

        <!-- Input Area -->
        <div class="pg-input-bar border-t p-4">
          <div class="flex gap-3">
            <div class="relative flex-1">
              <textarea
                v-model="userInput"
                class="input min-h-[44px] max-h-32 resize-y pr-12"
                :placeholder="t('workbench.inputPlaceholder')"
                rows="1"
                @keydown.enter.exact.prevent="sendMessage"
              ></textarea>
              <button
                class="pg-send absolute right-2 bottom-2 flex h-8 w-8 items-center justify-center rounded-lg transition-colors"
                :class="sending ? 'pg-send-stop' : 'pg-send-ready'"
                :disabled="sending || !userInput.trim()"
                @click="sending ? stopGeneration() : sendMessage()"
              >
                <svg v-if="sending" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M5.25 7.5A2.25 2.25 0 017.5 5.25h9a2.25 2.25 0 012.25 2.25v9a2.25 2.25 0 01-2.25 2.25h-9a2.25 2.25 0 01-2.25-2.25v-9z" />
                </svg>
                <svg v-else class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M6 12L3.269 3.126A59.768 59.768 0 0121.485 12 59.77 59.77 0 013.27 20.876L5.999 12zm0 0h7.5" />
                </svg>
              </button>
            </div>
          </div>
          <div class="pg-muted mt-2 flex items-center justify-between text-xs">
            <span>{{ t('workbench.enterToSend') }}</span>
            <span v-if="latencyMs !== null">{{ latencyMs }}ms</span>
          </div>
        </div>
      </div>

      <!-- Right: Parameters Panel -->
      <aside class="pg-rail hidden w-80 flex-shrink-0 border-l xl:block">
        <div class="pg-rail-head flex h-14 items-center justify-between border-b px-4">
          <h3 class="pg-title text-sm font-semibold">{{ t('workbench.parameters') }}</h3>
        </div>
        <div class="space-y-4 overflow-y-auto p-4">
          <!-- Key Source -->
          <div>
            <label class="input-label">{{ t('workbench.keySource') }}</label>
            <select v-model="keySource" class="input">
              <option value="platform">{{ t('workbench.platformKey') }}</option>
              <option value="byok">{{ t('workbench.bringYourOwn') }}</option>
            </select>
          </div>

          <!-- Platform Key (auto-loaded from user's own keys) -->
          <div v-if="keySource === 'platform'">
            <label class="input-label">{{ t('workbench.selectKey') }}</label>
            <div v-if="loadingKeys" class="pg-muted py-2 text-xs">{{ t('workbench.loadingKeys') }}</div>
            <template v-else>
              <select v-if="platformKeys.length" v-model="selectedPlatformKeyId" class="input">
                <option v-for="k in platformKeys" :key="k.id" :value="k.id">{{ k.name }}</option>
              </select>
              <div v-else class="pg-note rounded-lg px-3 py-2.5 text-xs">
                {{ t('workbench.noKeys') }}
                <RouterLink to="/keys" class="pg-link hover:underline">{{ t('workbench.goCreateKey') }}</RouterLink>
              </div>
            </template>
          </div>

          <!-- API Key (for BYOK) -->
          <div v-if="keySource === 'byok'">
            <label class="input-label">{{ t('workbench.apiKey') }}</label>
            <input v-model="apiKey" class="input" type="password" placeholder="sk-..." />
          </div>

          <!-- Base URL (for BYOK) -->
          <div v-if="keySource === 'byok'">
            <label class="input-label">Base URL</label>
            <input v-model="baseUrl" class="input" placeholder="https://api.openai.com/v1" />
          </div>

          <!-- Model -->
          <div>
            <label class="input-label">{{ t('workbench.model') }}</label>
            <select v-model="model" class="input">
              <option value="gpt-4o">GPT-4o</option>
              <option value="gpt-4o-mini">GPT-4o Mini</option>
              <option value="claude-3-5-sonnet">Claude 3.5 Sonnet</option>
              <option value="claude-3-haiku">Claude 3 Haiku</option>
              <option value="gemini-pro">Gemini Pro</option>
              <option value="deepseek-chat">DeepSeek Chat</option>
            </select>
          </div>

          <!-- Temperature -->
          <div>
            <label class="input-label">{{ t('workbench.temperature') }}: {{ temperature }}</label>
            <input v-model.number="temperature" type="range" min="0" max="2" step="0.1" class="pg-range w-full" />
          </div>

          <!-- Max Tokens -->
          <div>
            <label class="input-label">{{ t('workbench.maxTokens') }}</label>
            <input v-model.number="maxTokens" class="input" type="number" min="1" max="128000" />
          </div>

          <!-- Stream Toggle -->
          <div class="flex items-center justify-between">
            <label class="input-label mb-0">{{ t('workbench.stream') }}</label>
            <button
              class="pg-toggle relative inline-flex h-6 w-11 items-center rounded-full transition-colors"
              :class="stream ? 'pg-toggle-on' : 'pg-toggle-off'"
              @click="stream = !stream"
            >
              <span
                class="pg-toggle-knob inline-block h-4 w-4 transform rounded-full transition-transform"
                :class="stream ? 'translate-x-6' : 'translate-x-1'"
              />
            </button>
          </div>

          <!-- Request Preview -->
          <div>
            <label class="input-label">{{ t('workbench.requestPreview') }}</label>
            <pre class="pg-preview overflow-x-auto rounded-lg p-3 text-xs">{{ requestPreview }}</pre>
          </div>

          <!-- Export Buttons -->
          <div class="flex gap-2">
            <button class="btn btn-ghost btn-sm flex-1" @click="copyAsCurl">curl</button>
            <button class="btn btn-ghost btn-sm flex-1" @click="copyAsJs">JS</button>
            <button class="btn btn-ghost btn-sm flex-1" @click="copyAsPython">Python</button>
          </div>
        </div>
      </aside>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, ref, nextTick, watch, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import { keysAPI } from '@/api/keys'
import type { ApiKey } from '@/types'

const { t } = useI18n()

// Session management
interface Session {
  id: string
  name: string
  createdAt: Date
}

interface Message {
  role: 'user' | 'assistant' | 'system'
  content: string
}

const sessions = ref<Session[]>([
  { id: '1', name: 'New Chat', createdAt: new Date() }
])
const currentSessionId = ref('1')
const currentSession = computed(() => sessions.value.find(s => s.id === currentSessionId.value))

// Messages
const messages = ref<Message[]>([])
const userInput = ref('')
const messagesContainer = ref<HTMLElement | null>(null)

// Parameters
const keySource = ref<'platform' | 'byok'>('platform')
const apiKey = ref('')
const baseUrl = ref('https://api.openai.com/v1')
const model = ref('gpt-4o-mini')
const temperature = ref(0.7)
const maxTokens = ref(4096)
const stream = ref(true)

// Platform keys (auto-loaded from the user's own API keys)
const platformKeys = ref<ApiKey[]>([])
const selectedPlatformKeyId = ref<number | null>(null)
const loadingKeys = ref(false)

// State
const sending = ref(false)
const latencyMs = ref<number | null>(null)
const showMobileSidebar = ref(false)
let abortController: AbortController | null = null

/**
 * Load the current user's active API keys so the workbench can call the
 * local gateway automatically without the user pasting a key.
 */
async function loadPlatformKeys(): Promise<void> {
  loadingKeys.value = true
  try {
    const res = await keysAPI.list(1, 100, { status: 'active' })
    platformKeys.value = res.items ?? []
    if (platformKeys.value.length && selectedPlatformKeyId.value === null) {
      selectedPlatformKeyId.value = platformKeys.value[0].id
    }
  } catch {
    platformKeys.value = []
  } finally {
    loadingKeys.value = false
  }
}

/**
 * Resolve the API key to send with the request based on the selected source.
 */
function resolveApiKey(): string | null {
  if (keySource.value === 'byok') {
    return apiKey.value.trim() || null
  }
  const k = platformKeys.value.find(k => k.id === selectedPlatformKeyId.value)
  return k?.key ?? null
}

onMounted(loadPlatformKeys)

const requestPreview = computed(() => JSON.stringify({
  model: model.value,
  messages: messages.value.slice(-4).map(m => ({ role: m.role, content: m.content.slice(0, 50) + '...' })),
  temperature: temperature.value,
  max_tokens: maxTokens.value,
  stream: stream.value
}, null, 2))

function createSession(): void {
  const id = Date.now().toString()
  sessions.value.unshift({
    id,
    name: `Chat ${sessions.value.length + 1}`,
    createdAt: new Date()
  })
  currentSessionId.value = id
  messages.value = []
}

function clearMessages(): void {
  messages.value = []
}

async function sendMessage(): Promise<void> {
  if (!userInput.value.trim() || sending.value) return

  const resolvedKey = resolveApiKey()
  if (!resolvedKey) {
    messages.value.push({ role: 'assistant', content: t('workbench.noKeyHint') })
    return
  }

  const userMessage = userInput.value.trim()
  messages.value.push({ role: 'user', content: userMessage })
  userInput.value = ''

  await nextTick()
  scrollToBottom()

  sending.value = true
  latencyMs.value = null
  abortController = new AbortController()

  const started = performance.now()

  try {
    const endpoint = keySource.value === 'byok'
      ? `${baseUrl.value}/chat/completions`
      : `${window.location.origin}/v1/chat/completions`

    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${resolvedKey}`
    }

    const body = JSON.stringify({
      model: model.value,
      messages: messages.value.map(m => ({ role: m.role, content: m.content })),
      temperature: temperature.value,
      max_tokens: maxTokens.value,
      stream: stream.value
    })

    const res = await fetch(endpoint, {
      method: 'POST',
      headers,
      body,
      signal: abortController.signal
    })

    if (!res.ok) {
      const errData = await res.json().catch(() => ({} as any))
      throw new Error(errData?.error?.message || `Request failed: ${res.status}`)
    }

    if (stream.value && res.body) {
      await consumeStream(res.body)
    } else {
      const data = await res.json().catch(() => ({} as any))
      const assistantMessage = data?.choices?.[0]?.message?.content || JSON.stringify(data, null, 2)
      messages.value.push({ role: 'assistant', content: assistantMessage })
    }
    latencyMs.value = Math.round(performance.now() - started)
  } catch (err: any) {
    if (err.name === 'AbortError') {
      // keep whatever was streamed; mark the stop
      if (!messages.value.length || messages.value[messages.value.length - 1].role !== 'assistant') {
        messages.value.push({ role: 'assistant', content: '[Generation stopped]' })
      }
    } else {
      messages.value.push({ role: 'assistant', content: `Error: ${err.message}` })
    }
  } finally {
    sending.value = false
    abortController = null
    await nextTick()
    scrollToBottom()
  }
}

/**
 * Consume an OpenAI-compatible SSE stream and progressively append the
 * assistant's reply to the message list.
 */
async function consumeStream(streamBody: ReadableStream<Uint8Array>): Promise<void> {
  const reader = streamBody.getReader()
  const decoder = new TextDecoder()
  let buffer = ''
  const assistant: Message = { role: 'assistant', content: '' }
  messages.value.push(assistant)
  const idx = messages.value.length - 1

  try {
    for (;;) {
      const { done, value } = await reader.read()
      if (done) break
      buffer += decoder.decode(value, { stream: true })

      const lines = buffer.split('\n')
      buffer = lines.pop() ?? ''

      for (const raw of lines) {
        const line = raw.trim()
        if (!line.startsWith('data:')) continue
        const payload = line.slice(5).trim()
        if (payload === '[DONE]') return
        try {
          const json = JSON.parse(payload)
          const delta = json?.choices?.[0]?.delta?.content
          if (delta) {
            messages.value[idx].content += delta
            await nextTick()
            scrollToBottom()
          }
        } catch {
          // ignore keep-alive / partial fragments
        }
      }
    }
  } finally {
    reader.releaseLock()
  }
}

function stopGeneration(): void {
  if (abortController) {
    abortController.abort()
  }
}

function scrollToBottom(): void {
  if (messagesContainer.value) {
    messagesContainer.value.scrollTop = messagesContainer.value.scrollHeight
  }
}

function exportConversation(): void {
  const content = messages.value.map(m => `[${m.role}]: ${m.content}`).join('\n\n')
  navigator.clipboard.writeText(content)
}

function copyAsCurl(): void {
  const curl = `curl ${keySource.value === 'byok' ? baseUrl.value : window.location.origin + '/v1'}/chat/completions \\
  -H "Authorization: Bearer ${apiKey.value || 'YOUR_API_KEY'}" \\
  -H "Content-Type: application/json" \\
  -d '${JSON.stringify({
    model: model.value,
    messages: [{ role: 'user', content: 'Hello' }],
    temperature: temperature.value,
    max_tokens: maxTokens.value
  }, null, 2)}'`
  navigator.clipboard.writeText(curl)
}

function copyAsJs(): void {
  const js = `const response = await fetch('${keySource.value === 'byok' ? baseUrl.value : window.location.origin + '/v1'}/chat/completions', {
  method: 'POST',
  headers: {
    'Authorization': 'Bearer ${apiKey.value || 'YOUR_API_KEY'}',
    'Content-Type': 'application/json'
  },
  body: JSON.stringify({
    model: '${model.value}',
    messages: [{ role: 'user', content: 'Hello' }],
    temperature: ${temperature.value},
    max_tokens: ${maxTokens.value}
  })
});
const data = await response.json();`
  navigator.clipboard.writeText(js)
}

function copyAsPython(): void {
  const python = `import requests

response = requests.post(
    "${keySource.value === 'byok' ? baseUrl.value : window.location.origin + '/v1'}/chat/completions",
    headers={
        "Authorization": "Bearer ${apiKey.value || 'YOUR_API_KEY'}",
        "Content-Type": "application/json"
    },
    json={
        "model": "${model.value}",
        "messages": [{"role": "user", "content": "Hello"}],
        "temperature": ${temperature.value},
        "max_tokens": ${maxTokens.value}
    }
)
data = response.json()`
  navigator.clipboard.writeText(python)
}

watch(currentSessionId, () => {
  messages.value = []
})
</script>

<style scoped>
.pg-shell {
  border: 0;
  background: var(--zc-bg);
  color: var(--zc-text);
  box-shadow:
    12px 12px 28px var(--zc-shadow-dark),
    -12px -12px 28px var(--zc-shadow-light);
}

.pg-rail,
.pg-main {
  background: var(--zc-bg);
}

.pg-rail,
.pg-rail-head,
.pg-main-head,
.pg-input-bar {
  border-color: var(--zc-line);
}

.pg-title {
  color: var(--zc-text-strong);
}

.pg-muted {
  color: var(--zc-muted);
}

.pg-session,
.pg-empty-icon,
.pg-avatar,
.pg-note,
.pg-preview,
.pg-toggle,
.pg-send,
.pg-message {
  background: var(--zc-bg);
  box-shadow:
    5px 5px 12px var(--zc-shadow-dark),
    -5px -5px 12px var(--zc-shadow-light);
}

.pg-session {
  color: var(--zc-muted);
}

.pg-session:hover,
.pg-session-active {
  color: var(--zc-accent);
  box-shadow:
    inset 5px 5px 12px var(--zc-shadow-dark),
    inset -5px -5px 12px var(--zc-shadow-light);
}

.pg-empty-icon,
.pg-avatar,
.pg-link,
.pg-send-ready {
  color: var(--zc-accent);
}

.pg-message-user {
  color: var(--zc-accent);
  box-shadow:
    inset 5px 5px 12px var(--zc-shadow-dark),
    inset -5px -5px 12px var(--zc-shadow-light);
}

.pg-message-assistant,
.pg-note,
.pg-preview {
  color: var(--zc-text);
  box-shadow:
    inset 5px 5px 12px var(--zc-shadow-dark),
    inset -5px -5px 12px var(--zc-shadow-light);
}

.pg-send-stop,
.pg-note-warning {
  color: var(--zc-warning);
}

.pg-range {
  accent-color: var(--zc-accent);
}

.pg-toggle {
  border: 0;
  padding: 0;
}

.pg-toggle-on {
  color: var(--zc-accent);
  box-shadow:
    inset 4px 4px 10px var(--zc-shadow-dark),
    inset -4px -4px 10px var(--zc-shadow-light);
}

.pg-toggle-off {
  color: var(--zc-muted);
  box-shadow:
    inset 4px 4px 10px var(--zc-shadow-dark),
    inset -4px -4px 10px var(--zc-shadow-light);
}

.pg-toggle-knob {
  background: var(--zc-bg);
  box-shadow:
    3px 3px 7px var(--zc-shadow-dark),
    -3px -3px 7px var(--zc-shadow-light);
}

.pg-preview {
  color: var(--zc-muted);
}
</style>
