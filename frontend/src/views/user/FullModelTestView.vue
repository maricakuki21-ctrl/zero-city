<template>
  <AppLayout>
    <div class="fmt-page space-y-6">
      <section class="fmt-hero relative overflow-hidden rounded-[2rem] p-6 lg:p-8">
        <div class="pointer-events-none absolute inset-0"></div>
        <div class="relative grid gap-8 lg:grid-cols-[1fr_0.85fr] lg:items-center">
          <div>
            <p class="fmt-muted text-xs font-bold uppercase tracking-[0.24em]">Full Model Verifier</p>
            <h1 class="fmt-title mt-3 text-4xl font-bold md:text-5xl">{{ t('fullModelTest.hero.title') }}</h1>
            <p class="fmt-muted mt-4 max-w-2xl text-base leading-8">
              {{ t('fullModelTest.hero.description') }}
            </p>
            <div class="fmt-hero-tags fmt-muted mt-6 flex flex-wrap gap-3 text-sm">
              <span class="fmt-chip rounded-full px-4 py-2 font-semibold">{{ t('fullModelTest.hero.tagPrompt') }}</span>
              <span class="fmt-chip rounded-full px-4 py-2 font-semibold">{{ t('fullModelTest.hero.tagAutoMap') }}</span>
              <span class="fmt-chip rounded-full px-4 py-2 font-semibold">{{ t('fullModelTest.hero.tagManualFallback') }}</span>
            </div>
          </div>
          <div class="fmt-code-card rounded-[1.5rem] p-5 font-mono text-sm">
            <div class="fmt-code-label">Test Endpoint</div>
            <div class="fmt-code-accent mt-2">GET /v1/models</div>
            <div class="fmt-code-accent mt-4">POST /v1/chat/completions</div>
            <div class="fmt-code-label mt-5">Result</div>
            <div class="fmt-code-accent mt-2">{{ t('fullModelTest.hero.resultPreview') }}</div>
          </div>
        </div>
      </section>

      <section class="grid gap-4 lg:grid-cols-[0.9fr_1.1fr]">
        <div class="panel p-6 lg:p-8">
          <div class="flex items-start justify-between gap-4">
            <div>
              <p class="fmt-muted text-xs font-bold uppercase tracking-[0.22em]">Connection</p>
              <h2 class="fmt-title mt-3 text-2xl font-bold">{{ t('fullModelTest.connection.title') }}</h2>
            </div>
            <span class="fmt-chip rounded-full px-3 py-1 text-xs font-bold">{{ t('fullModelTest.connection.localBrowser') }}</span>
          </div>

          <div class="mt-6 space-y-4">
            <label class="block">
              <span class="fmt-label text-sm font-semibold">{{ t('fullModelTest.connection.baseUrl') }}</span>
              <input v-model.trim="baseUrl" class="mt-2 w-full rounded-2xl px-4 py-3 text-sm outline-none transition" placeholder="https://bizdecipher.com/v1" />
            </label>
            <label class="block">
              <span class="fmt-label text-sm font-semibold">API Key</span>
              <input v-model.trim="apiKey" type="password" class="mt-2 w-full rounded-2xl px-4 py-3 text-sm outline-none transition" placeholder="sk-..." autocomplete="off" />
            </label>
            <div class="flex flex-wrap gap-3">
              <button class="btn btn-secondary" :disabled="loadingModels || !canLoadModels" @click="loadModels">
                {{ loadingModels ? t('fullModelTest.connection.loadingModels') : t('fullModelTest.connection.autoMapModels') }}
              </button>
              <button class="btn btn-secondary" @click="useCurrentSite">{{ t('fullModelTest.connection.useCurrentSite') }}</button>
            </div>
            <p v-if="modelError" class="fmt-alert rounded-2xl px-4 py-3 text-sm leading-6">{{ modelError }}</p>
          </div>

          <div class="mt-6 space-y-4">
            <label class="block">
              <span class="fmt-label text-sm font-semibold">{{ t('fullModelTest.connection.selectModel') }}</span>
              <select v-model="selectedModel" class="mt-2 w-full rounded-2xl px-4 py-3 text-sm outline-none transition">
                <option value="">{{ t('fullModelTest.connection.selectOrManual') }}</option>
                <option v-for="model in models" :key="model" :value="model">{{ model }}</option>
              </select>
            </label>
            <label class="block">
              <span class="fmt-label text-sm font-semibold">{{ t('fullModelTest.connection.manualModel') }}</span>
              <input v-model.trim="manualModel" class="mt-2 w-full rounded-2xl px-4 py-3 text-sm outline-none transition" :placeholder="t('fullModelTest.connection.manualModelPlaceholder')" />
            </label>
            <button class="btn btn-primary w-full justify-center" :disabled="testing || !canTest" @click="runTest">
              {{ testing ? t('fullModelTest.connection.testing') : t('fullModelTest.connection.startTest') }}
            </button>
          </div>
        </div>

        <div class="panel p-6 lg:p-8">
          <p class="fmt-muted text-xs font-bold uppercase tracking-[0.22em]">Probe</p>
          <h2 class="fmt-title mt-3 text-2xl font-bold">{{ t('fullModelTest.probe.title') }}</h2>
          <div class="mt-5 grid gap-3 md:grid-cols-2">
            <button
              v-for="probe in probes"
              :key="probe.id"
              class="fmt-probe-card rounded-3xl border p-4 text-left transition hover:-translate-y-0.5"
              :class="selectedProbeId === probe.id ? 'fmt-probe-card-active' : ''"
              @click="selectedProbeId = probe.id"
            >
              <div class="fmt-label font-semibold">{{ probe.title }}</div>
              <p class="fmt-muted mt-2 text-sm leading-6">{{ probe.desc }}</p>
            </button>
          </div>

          <div class="fmt-code-card mt-6 rounded-3xl p-5 font-mono text-sm">
            <div class="fmt-code-label">Prompt</div>
            <pre class="mt-3 whitespace-pre-wrap break-words leading-6">{{ selectedProbe.prompt }}</pre>
          </div>
        </div>
      </section>

      <section v-if="result || testError" class="panel p-6 lg:p-8">
        <div class="flex flex-wrap items-start justify-between gap-4">
          <div>
            <p class="fmt-muted text-xs font-bold uppercase tracking-[0.22em]">Result</p>
            <h2 class="fmt-title mt-3 text-2xl font-bold">{{ t('fullModelTest.result.title') }}</h2>
          </div>
          <span v-if="result" class="fmt-status rounded-full px-4 py-2 text-sm font-bold" :class="resultStatusClass">{{ resultStatusText }}</span>
        </div>
        <p v-if="testError" class="fmt-alert mt-5 rounded-2xl px-4 py-3 text-sm leading-6">{{ testError }}</p>
        <div v-if="result" class="mt-5 grid gap-4 lg:grid-cols-3">
          <div class="fmt-result-card rounded-3xl p-5">
            <div class="fmt-muted text-xs font-bold uppercase tracking-[0.18em]">Model</div>
            <div class="fmt-label mt-2 break-all font-semibold">{{ result.model }}</div>
          </div>
          <div class="fmt-result-card rounded-3xl p-5">
            <div class="fmt-muted text-xs font-bold uppercase tracking-[0.18em]">Latency</div>
            <div class="fmt-label mt-2 font-semibold">{{ result.latencyMs }} ms</div>
          </div>
          <div class="fmt-result-card rounded-3xl p-5">
            <div class="fmt-muted text-xs font-bold uppercase tracking-[0.18em]">Evidence</div>
            <div class="fmt-label mt-2 font-semibold">{{ t('fullModelTest.result.evidenceCount', { count: result.evidenceCount, total: selectedProbe.keywords.length }) }}</div>
          </div>
        </div>
        <div v-if="result" class="fmt-code-card mt-5 rounded-3xl p-5 text-sm">
          <div class="fmt-code-label font-mono">Model Response</div>
          <pre class="mt-3 whitespace-pre-wrap break-words leading-7">{{ result.content }}</pre>
        </div>
      </section>

      <section class="grid gap-4 lg:grid-cols-3">
        <article v-for="source in authoritySources" :key="source.name" class="fmt-source-card rounded-3xl p-5">
          <div class="fmt-label text-sm font-bold">{{ source.name }}</div>
          <p class="fmt-muted mt-2 text-sm leading-6">{{ source.desc }}</p>
          <a :href="source.url" target="_blank" rel="noreferrer" class="fmt-link mt-4 inline-flex text-sm font-semibold">{{ t('fullModelTest.sources.viewSource') }}</a>
        </article>
      </section>

      <p class="fmt-muted text-center text-xs leading-6">
        {{ t('fullModelTest.disclaimer') }}
      </p>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'

interface ChatCompletionResponse {
  choices?: Array<{
    message?: {
      content?: string
    }
    text?: string
  }>
  model?: string
}

interface Probe {
  id: string
  title: string
  desc: string
  keywords: string[]
  prompt: string
}

const { t, tm } = useI18n()

const normalizeBaseUrl = (value: string) => value.trim().replace(/\/+$/, '')

const baseUrl = ref('https://bizdecipher.com/v1')
const apiKey = ref('')
const models = ref<string[]>([])
const selectedModel = ref('')
const manualModel = ref('')
const loadingModels = ref(false)
const testing = ref(false)
const modelError = ref('')
const testError = ref('')
const selectedProbeId = ref('reasoning')
const result = ref<{
  model: string
  latencyMs: number
  content: string
  evidenceCount: number
} | null>(null)

function translateStringArray(key: string): string[] {
  const value = tm(key)
  return Array.isArray(value) ? value.map(String) : []
}

const probes = computed<Probe[]>(() => [
  {
    id: 'reasoning',
    title: t('fullModelTest.probe.items.reasoning.title'),
    desc: t('fullModelTest.probe.items.reasoning.desc'),
    keywords: translateStringArray('fullModelTest.probe.items.reasoning.keywords'),
    prompt: t('fullModelTest.probe.items.reasoning.prompt')
  },
  {
    id: 'code',
    title: t('fullModelTest.probe.items.code.title'),
    desc: t('fullModelTest.probe.items.code.desc'),
    keywords: translateStringArray('fullModelTest.probe.items.code.keywords'),
    prompt: t('fullModelTest.probe.items.code.prompt')
  },
  {
    id: 'long-context',
    title: t('fullModelTest.probe.items.longContext.title'),
    desc: t('fullModelTest.probe.items.longContext.desc'),
    keywords: translateStringArray('fullModelTest.probe.items.longContext.keywords'),
    prompt: t('fullModelTest.probe.items.longContext.prompt')
  },
  {
    id: 'quality',
    title: t('fullModelTest.probe.items.quality.title'),
    desc: t('fullModelTest.probe.items.quality.desc'),
    keywords: translateStringArray('fullModelTest.probe.items.quality.keywords'),
    prompt: t('fullModelTest.probe.items.quality.prompt')
  }
])

const authoritySources = computed(() => [
  { name: 'Chatbot Arena', desc: t('fullModelTest.sources.chatbotArena'), url: 'https://lmarena.ai/' },
  { name: 'Artificial Analysis', desc: t('fullModelTest.sources.artificialAnalysis'), url: 'https://artificialanalysis.ai/' },
  { name: 'OpenCompass', desc: t('fullModelTest.sources.openCompass'), url: 'https://opencompass.org.cn/' },
  { name: 'HELM', desc: t('fullModelTest.sources.helm'), url: 'https://crfm.stanford.edu/helm/' },
  { name: 'SWE-bench', desc: t('fullModelTest.sources.sweBench'), url: 'https://www.swebench.com/' },
  { name: 'GPQA / MMLU', desc: t('fullModelTest.sources.gpqaMmlu'), url: 'https://paperswithcode.com/' }
])

const selectedProbe = computed(() => probes.value.find((probe) => probe.id === selectedProbeId.value) ?? probes.value[0])
const effectiveModel = computed(() => manualModel.value || selectedModel.value)
const canLoadModels = computed(() => Boolean(baseUrl.value && apiKey.value))
const canTest = computed(() => Boolean(baseUrl.value && apiKey.value && effectiveModel.value))

const resultStatusKey = computed<'pass' | 'retry' | 'abnormal'>(() => {
  if (!result.value) return 'abnormal'
  if (result.value.evidenceCount >= Math.min(3, selectedProbe.value.keywords.length)) return 'pass'
  if (result.value.content.length > 120) return 'retry'
  return 'abnormal'
})

const resultStatusText = computed(() => result.value ? t(`fullModelTest.status.${resultStatusKey.value}`) : '')

const resultStatusClass = computed(() => {
  if (resultStatusKey.value === 'pass') return 'fmt-status-pass'
  if (resultStatusKey.value === 'retry') return 'fmt-status-retry'
  return 'fmt-status-abnormal'
})

async function loadModels() {
  if (!canLoadModels.value) return
  loadingModels.value = true
  modelError.value = ''
  models.value = []
  try {
    const response = await fetch(`${normalizeBaseUrl(baseUrl.value)}/models`, {
      headers: { Authorization: `Bearer ${apiKey.value}` }
    })
    if (!response.ok) throw new Error(t('fullModelTest.errors.modelsHttp', { status: response.status }))
    const payload = await response.json()
    const data = Array.isArray(payload?.data) ? payload.data : []
    models.value = data
      .map((item: unknown) => typeof item === 'string' ? item : (item as { id?: unknown })?.id)
      .filter((id: unknown): id is string => typeof id === 'string' && id.length > 0)
      .sort((a: string, b: string) => a.localeCompare(b))
    if (!models.value.length) throw new Error(t('fullModelTest.errors.emptyModels'))
    selectedModel.value = models.value[0]
  } catch (error) {
    modelError.value = error instanceof Error
      ? t('fullModelTest.errors.modelLoadWithFallback', { message: error.message })
      : t('fullModelTest.errors.modelAutoMapFailed')
  } finally {
    loadingModels.value = false
  }
}

async function runTest() {
  if (!canTest.value) return
  testing.value = true
  testError.value = ''
  result.value = null
  const startedAt = performance.now()
  try {
    const response = await fetch(`${normalizeBaseUrl(baseUrl.value)}/chat/completions`, {
      method: 'POST',
      headers: {
        Authorization: `Bearer ${apiKey.value}`,
        'Content-Type': 'application/json'
      },
      body: JSON.stringify({
        model: effectiveModel.value,
        messages: [
          { role: 'system', content: t('fullModelTest.systemPrompt') },
          { role: 'user', content: selectedProbe.value.prompt }
        ],
        temperature: 0.2,
        max_tokens: 900
      })
    })
    if (!response.ok) throw new Error(t('fullModelTest.errors.testHttp', { status: response.status }))
    const payload = await response.json() as ChatCompletionResponse
    const content = payload.choices?.[0]?.message?.content || payload.choices?.[0]?.text || ''
    if (!content) throw new Error(t('fullModelTest.errors.noContent'))
    const lowerContent = content.toLowerCase()
    const evidenceCount = selectedProbe.value.keywords.filter((keyword) => lowerContent.includes(keyword.toLowerCase())).length
    result.value = {
      model: payload.model || effectiveModel.value,
      latencyMs: Math.round(performance.now() - startedAt),
      content,
      evidenceCount
    }
  } catch (error) {
    testError.value = error instanceof Error
      ? t('fullModelTest.errors.testFailedWithMessage', { message: error.message })
      : t('fullModelTest.errors.testFailed')
  } finally {
    testing.value = false
  }
}

function useCurrentSite() {
  baseUrl.value = `${window.location.origin}/v1`
}
</script>

<style scoped>
.fmt-page {
  --fmt-raise: 16px 16px 34px var(--zc-shadow-dark), -16px -16px 34px var(--zc-shadow-light);
  --fmt-raise-sm: 9px 9px 20px var(--zc-shadow-dark), -9px -9px 20px var(--zc-shadow-light);
  --fmt-inset: inset 7px 7px 16px var(--zc-shadow-dark), inset -7px -7px 16px var(--zc-shadow-light);
  color: var(--zc-text);
}

.fmt-hero,
.fmt-page .panel,
.fmt-source-card {
  border: 0 !important;
  background: var(--zc-bg) !important;
  box-shadow: var(--fmt-raise) !important;
}

.fmt-hero > .pointer-events-none {
  background:
    radial-gradient(circle at 12% 0%, color-mix(in srgb, var(--zc-accent-2) 12%, transparent), transparent 34%),
    radial-gradient(circle at 86% 10%, color-mix(in srgb, var(--zc-accent) 10%, transparent), transparent 30%) !important;
}

.fmt-chip,
.fmt-result-card,
.fmt-source-card {
  border: 0 !important;
  background: var(--zc-bg) !important;
  box-shadow: var(--fmt-raise-sm) !important;
}

.fmt-title,
.fmt-label {
  color: var(--zc-text-strong) !important;
  letter-spacing: 0;
}

.fmt-muted,
.fmt-code-label {
  color: var(--zc-muted) !important;
}

.fmt-link,
.fmt-code-accent {
  color: var(--zc-accent-2) !important;
}

.fmt-code-card,
.fmt-result-card {
  border: 0 !important;
  background: var(--zc-bg) !important;
  color: var(--zc-text) !important;
  box-shadow: var(--fmt-inset) !important;
}

.fmt-code-card pre {
  color: var(--zc-text-strong) !important;
}

.fmt-page input,
.fmt-page select {
  border: 0 !important;
  background: var(--zc-bg) !important;
  color: var(--zc-text-strong) !important;
  box-shadow: var(--fmt-inset) !important;
}

.fmt-status,
.fmt-alert {
  background: var(--zc-bg) !important;
  color: var(--zc-text-strong) !important;
  box-shadow: var(--fmt-inset) !important;
}

.fmt-status-pass {
  color: var(--zc-accent) !important;
}

.fmt-status-retry {
  color: var(--zc-accent-2) !important;
}

.fmt-page input:focus,
.fmt-page select:focus {
  box-shadow: var(--fmt-inset), 0 0 0 3px color-mix(in srgb, var(--zc-accent) 14%, transparent) !important;
}

.fmt-probe-card {
  border: 0 !important;
  background: var(--zc-bg) !important;
  box-shadow: var(--fmt-raise-sm) !important;
}

.fmt-probe-card:hover {
  transform: translateY(-1px) !important;
}

.fmt-probe-card-active {
  box-shadow: var(--fmt-inset) !important;
}

.fmt-page .btn-secondary {
  border: 0 !important;
  background: var(--zc-bg) !important;
  box-shadow: var(--fmt-raise-sm) !important;
}

@media (max-width: 640px) {
  .fmt-hero,
  .fmt-page .panel {
    border-radius: 1.5rem;
    padding: 1.25rem !important;
  }
}
</style>
