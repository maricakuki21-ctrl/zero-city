export type SharedPoolProviderPresetId =
  | 'openai_compatible'
  | 'openai'
  | 'azure_openai'
  | 'anthropic'
  | 'gemini'
  | 'custom'

export interface SharedPoolProviderPreset {
  id: SharedPoolProviderPresetId
  label: string
  providerValue: string
  protocol: 'openai_compatible' | 'openai' | 'azure_openai' | 'anthropic' | 'gemini' | 'custom'
  defaultBaseURL: string
  keyPlaceholder: string
  supportLevel: 'primary' | 'limited' | 'custom'
  hint: string
  autoNormalizeV1: boolean
}

export const SHARED_POOL_PROVIDER_PRESETS: SharedPoolProviderPreset[] = [
  {
    id: 'openai_compatible',
    label: 'OpenAI 兼容中转',
    providerValue: 'openai_compatible',
    protocol: 'openai_compatible',
    defaultBaseURL: 'https://your-gateway.example.com/v1',
    keyPlaceholder: 'sk-...',
    supportLevel: 'primary',
    hint: 'NewAPI / Sub2API / 各类 OpenAI 兼容中转。系统会自动规范 /v1。',
    autoNormalizeV1: true
  },
  {
    id: 'openai',
    label: 'OpenAI 官方',
    providerValue: 'openai',
    protocol: 'openai',
    defaultBaseURL: 'https://api.openai.com/v1',
    keyPlaceholder: 'sk-...',
    supportLevel: 'primary',
    hint: '官方 OpenAI 接口。',
    autoNormalizeV1: true
  },
  {
    id: 'azure_openai',
    label: 'Azure OpenAI',
    providerValue: 'azure_openai',
    protocol: 'azure_openai',
    defaultBaseURL: 'https://YOUR_RESOURCE.openai.azure.com',
    keyPlaceholder: 'azure-key',
    supportLevel: 'limited',
    hint: '共享池二期能力，当前仅作模板；完整 api-version 适配建设中。',
    autoNormalizeV1: false
  },
  {
    id: 'anthropic',
    label: 'Anthropic',
    providerValue: 'anthropic',
    protocol: 'anthropic',
    defaultBaseURL: 'https://api.anthropic.com',
    keyPlaceholder: 'sk-ant-...',
    supportLevel: 'limited',
    hint: '平台网关支持，共享池主路径仍以 OpenAI 兼容为主。',
    autoNormalizeV1: false
  },
  {
    id: 'gemini',
    label: 'Gemini',
    providerValue: 'gemini',
    protocol: 'gemini',
    defaultBaseURL: 'https://generativelanguage.googleapis.com',
    keyPlaceholder: 'AIza...',
    supportLevel: 'limited',
    hint: '平台网关支持，共享池主路径仍以 OpenAI 兼容为主。',
    autoNormalizeV1: false
  },
  {
    id: 'custom',
    label: '自定义',
    providerValue: 'custom',
    protocol: 'custom',
    defaultBaseURL: '',
    keyPlaceholder: 'api-key',
    supportLevel: 'custom',
    hint: '完全自定义供应商名与 Base URL。',
    autoNormalizeV1: false
  }
]

export function getSharedPoolProviderPreset(id: string | undefined | null): SharedPoolProviderPreset {
  return SHARED_POOL_PROVIDER_PRESETS.find((item) => item.id === id) || SHARED_POOL_PROVIDER_PRESETS[0]
}

export function normalizeOpenAICompatibleBaseURL(input: string, autoNormalizeV1 = true): string {
  const raw = String(input || '').trim().replace(/\/+$/, '')
  if (!raw) return ''
  if (!autoNormalizeV1) return raw
  if (/\/v1$/i.test(raw)) return raw
  if (/\/v1\//i.test(raw)) return raw.replace(/\/+$/, '')
  return `${raw}/v1`
}

export function providerPresetLabel(providerValue: string): string {
  const hit = SHARED_POOL_PROVIDER_PRESETS.find((item) => item.providerValue === providerValue)
  return hit?.label || providerValue || 'openai_compatible'
}
