export type SharedPoolEndpointKind = 'chat' | 'responses' | 'image_generation' | 'image_edit' | 'video'
export type SharedPoolBillingKind = 'token' | 'per_request' | 'image' | 'video'

export const SHARED_POOL_MIN_MULTIPLIER = 0.0001

export function parseSharedPoolMultiplier(value: unknown): number | null {
  const multiplier = Number(value)
  return Number.isFinite(multiplier) && multiplier >= SHARED_POOL_MIN_MULTIPLIER
    ? multiplier
    : null
}

export function formatSharedPoolMultiplier(value: unknown): string {
  const multiplier = Number(value)
  if (!Number.isFinite(multiplier)) return '—'
  return multiplier.toLocaleString('en-US', {
    useGrouping: false,
    minimumFractionDigits: 0,
    maximumFractionDigits: 8,
  })
}

export type SharedPoolEndpointEvidence = {
  endpoint_type: string
  enabled: boolean
  gate_status?: string | null
  endpoint_pricing_status?: string | null
  current_price?: unknown | null
}

export type SharedPoolEndpointAvailability = {
  callable: boolean
  code: 'ready' | 'disabled' | 'unverified' | 'failed' | 'stale' | 'unpriced' | 'invalid_price' | 'unsupported'
  label: string
  description: string
  tone: 'ready' | 'warning' | 'danger' | 'muted'
}

export const sharedPoolEndpointOrder: SharedPoolEndpointKind[] = [
  'chat',
  'responses',
  'image_generation',
  'image_edit',
  'video',
]

export function isSharedPoolEndpointKind(value: string): value is SharedPoolEndpointKind {
  return sharedPoolEndpointOrder.includes(value as SharedPoolEndpointKind)
}

export function sharedPoolEndpointLabel(endpoint: string): string {
  switch (endpoint) {
    case 'chat': return '文字对话（Chat）'
    case 'responses': return '文字与工具（Responses）'
    case 'image_generation': return '图片生成'
    case 'image_edit': return '图片编辑'
    case 'video': return '视频生成'
    default: return `其他服务（${endpoint || '未知'}）`
  }
}

export function sharedPoolEndpointShortLabel(endpoint: string): string {
  switch (endpoint) {
    case 'chat': return '文字对话'
    case 'responses': return 'Responses'
    case 'image_generation': return '图片生成'
    case 'image_edit': return '图片编辑'
    case 'video': return '视频生成'
    default: return endpoint || '未知服务'
  }
}

export function sharedPoolEndpointDescription(endpoint: string): string {
  switch (endpoint) {
    case 'chat': return '适合常规聊天、代码和文本输出。'
    case 'responses': return '适合 Responses API、工具调用及扩展能力。'
    case 'image_generation': return '按生成图片张数或一次请求计费。'
    case 'image_edit': return '上传原图后进行修改，按张或一次请求计费。'
    case 'video': return '按生成时长（秒）或一次请求计费；检测与结算未通过前不会放行。'
    default: return '该服务需要价格和满血检测同时通过后才能调用。'
  }
}

export function sharedPoolBillingOptions(endpoint: string): Array<{ value: SharedPoolBillingKind; label: string }> {
  if (!isSharedPoolEndpointKind(endpoint)) return []
  switch (endpoint) {
    case 'image_generation':
    case 'image_edit':
      return [
        { value: 'image', label: '按图片张数收费' },
        { value: 'per_request', label: '按请求次数收费' },
      ]
    case 'video':
      return [
        { value: 'video', label: '按生成时长收费' },
        { value: 'per_request', label: '按请求次数收费' },
      ]
    default:
      return [
        { value: 'token', label: '按 token 收费' },
        { value: 'per_request', label: '按请求次数收费' },
      ]
  }
}

export function defaultSharedPoolBillingMode(endpoint: string): SharedPoolBillingKind {
  if (endpoint === 'image_generation' || endpoint === 'image_edit') return 'image'
  if (endpoint === 'video') return 'video'
  return 'token'
}

export function sharedPoolBillingModeAllowed(endpoint: string, billingMode: string): billingMode is SharedPoolBillingKind {
  return sharedPoolBillingOptions(endpoint).some((option) => option.value === billingMode)
}

function sharedPoolCurrentBillingMode(currentPrice: unknown): string {
  if (!currentPrice || typeof currentPrice !== 'object') return ''
  const basePrice = (currentPrice as { base_price?: unknown }).base_price
  if (!basePrice || typeof basePrice !== 'object') return ''
  const billingMode = (basePrice as { billing_mode?: unknown }).billing_mode
  return typeof billingMode === 'string' ? billingMode : ''
}

export function sharedPoolEndpointHasCompatiblePrice(item: SharedPoolEndpointEvidence): boolean {
  if (String(item.endpoint_pricing_status || '').toLowerCase() !== 'ready' || !item.current_price) return false
  return sharedPoolBillingModeAllowed(item.endpoint_type, sharedPoolCurrentBillingMode(item.current_price))
}

export function sharedPoolEndpointRequiresMediaGate(endpoint: string): boolean {
  return endpoint === 'image_generation' || endpoint === 'image_edit' || endpoint === 'video'
}

export function sharedPoolEndpointGateSatisfied(item: SharedPoolEndpointEvidence): boolean {
  return !sharedPoolEndpointRequiresMediaGate(item.endpoint_type)
    || String(item.gate_status || '').trim().toLowerCase() === 'passed'
}

export function sharedPoolBillingExampleLabel(billingMode: string): string {
  switch (billingMode) {
    case 'token': return '示例：输入 1000 token、输出 500 token'
    case 'image': return '示例：生成或编辑 1 张图片'
    case 'video': return '示例：生成 1 个默认 8 秒视频'
    case 'per_request': return '示例：完成 1 次请求'
    default: return '计费示例'
  }
}

export function sharedPoolEndpointAvailability(item: SharedPoolEndpointEvidence): SharedPoolEndpointAvailability {
  if (!item.enabled) {
    return {
      callable: false,
      code: 'disabled',
      label: '服务已关闭',
      description: '池主没有启用这个服务，用户请求不会进入该端点。',
      tone: 'muted',
    }
  }

  if (!isSharedPoolEndpointKind(item.endpoint_type)) {
    return {
      callable: false,
      code: 'unsupported',
      label: '端点待适配',
      description: '当前共享池尚未完成此端点的调用与计费适配。',
      tone: 'muted',
    }
  }

  const requiresMediaGate = sharedPoolEndpointRequiresMediaGate(item.endpoint_type)
  const gate = String(item.gate_status || '').trim().toLowerCase()
  if (requiresMediaGate && gate !== 'passed') {
    if (gate === 'failed') {
      return {
        callable: false,
        code: 'failed',
        label: '检测未通过',
        description: '这个服务的满血检测没有通过，修好上游后需要重新检测。',
        tone: 'danger',
      }
    }
    if (gate === 'stale') {
      return {
        callable: false,
        code: 'stale',
        label: '检测已过期',
        description: '以前通过过，但证据已经过期；重新满血检测后才能恢复调用。',
        tone: 'warning',
      }
    }
    return {
      callable: false,
      code: 'unverified',
      label: '等待满血检测',
      description: '价格可以先设置，但检测通过前系统仍会拒绝调用。',
      tone: 'warning',
    }
  }

  if (String(item.endpoint_pricing_status || '').toLowerCase() !== 'ready' || !item.current_price) {
    return {
      callable: false,
      code: 'unpriced',
      label: '等待补齐价格',
      description: requiresMediaGate
        ? '媒体检测已通过，但价格还不完整；系统不会按 0 元放行。'
        : '文字服务不单独等待媒体检测，但价格必须完整；系统不会按 0 元放行。',
      tone: 'danger',
    }
  }

  if (!sharedPoolEndpointHasCompatiblePrice(item)) {
    return {
      callable: false,
      code: 'invalid_price',
      label: '价格单位不匹配',
      description: '服务类型和价格单位对不上。为防止错扣费，系统会拒绝调用，请重新发布正确单位的价格。',
      tone: 'danger',
    }
  }

  return {
    callable: true,
    code: 'ready',
    label: '可调用',
    description: requiresMediaGate
      ? '这个媒体服务已启用，媒体检测和价格都已通过。'
      : '这个文字服务已启用且价格完整，调用时沿用池级满血检测，不需要单独的图片/视频检测。',
    tone: 'ready',
  }
}
