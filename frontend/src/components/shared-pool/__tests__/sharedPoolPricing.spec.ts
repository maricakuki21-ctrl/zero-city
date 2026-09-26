import { describe, expect, it } from 'vitest'
import {
  defaultSharedPoolBillingMode,
  formatSharedPoolMultiplier,
  parseSharedPoolMultiplier,
  sharedPoolBillingExampleLabel,
  sharedPoolBillingModeAllowed,
  sharedPoolBillingOptions,
  sharedPoolEndpointAvailability,
  sharedPoolEndpointGateSatisfied,
  sharedPoolEndpointLabel,
  sharedPoolEndpointRequiresMediaGate,
} from '@/features/bizdecipher/components/shared-pool/sharedPoolPricing'

describe('shared-pool endpoint pricing presentation contract', () => {
  it('preserves small valid multipliers without rounding them into another price', () => {
    expect(parseSharedPoolMultiplier(0.0001)).toBe(0.0001)
    expect(parseSharedPoolMultiplier('0.001')).toBe(0.001)
    expect(parseSharedPoolMultiplier(0.00009)).toBeNull()
    expect(parseSharedPoolMultiplier(Number.POSITIVE_INFINITY)).toBeNull()
    expect(formatSharedPoolMultiplier(0.0001)).toBe('0.0001')
    expect(formatSharedPoolMultiplier(0.001)).toBe('0.001')
    expect(formatSharedPoolMultiplier(1)).toBe('1')
  })

  it('lets text endpoints use the pool-level check without waiting for a media gate', () => {
    for (const endpointType of ['chat', 'responses']) {
      for (const gateStatus of ['unverified', 'failed', 'stale', '']) {
        const result = sharedPoolEndpointAvailability({
          endpoint_type: endpointType,
          enabled: true,
          gate_status: gateStatus,
          endpoint_pricing_status: 'ready',
          current_price: { price_version_id: 1, base_price: { billing_mode: 'token' } },
        })

        expect(sharedPoolEndpointRequiresMediaGate(endpointType)).toBe(false)
        expect(result.callable).toBe(true)
        expect(result.code).toBe('ready')
        expect(result.description).toContain('不需要单独的图片/视频检测')
      }
    }
  })

  it('still requires enabled, a passed gate and compatible pricing for media endpoints', () => {
    const base = {
      endpoint_type: 'image_generation',
      enabled: true,
      gate_status: 'passed',
      endpoint_pricing_status: 'ready',
      current_price: { price_version_id: 1, base_price: { billing_mode: 'image' } },
    }

    expect(sharedPoolEndpointAvailability(base).callable).toBe(true)
    expect(sharedPoolEndpointAvailability({ ...base, enabled: false }).code).toBe('disabled')
    expect(sharedPoolEndpointAvailability({ ...base, gate_status: 'failed' }).code).toBe('failed')
    expect(sharedPoolEndpointAvailability({ ...base, gate_status: 'stale' }).code).toBe('stale')
    expect(sharedPoolEndpointAvailability({ ...base, gate_status: 'unverified' }).code).toBe('unverified')
    expect(sharedPoolEndpointAvailability({ ...base, endpoint_pricing_status: 'pending' }).code).toBe('unpriced')
    expect(sharedPoolEndpointAvailability({ ...base, current_price: null }).code).toBe('unpriced')
    expect(sharedPoolEndpointAvailability({ ...base, current_price: { base_price: { billing_mode: 'token' } } }).code).toBe('invalid_price')
  })

  it('keeps text ready while an unverified media endpoint remains protected in a mixed pool', () => {
    const text = {
      endpoint_type: 'chat',
      enabled: true,
      gate_status: 'unverified',
      endpoint_pricing_status: 'ready',
      current_price: { base_price: { billing_mode: 'token' } },
    }
    const image = {
      endpoint_type: 'image_generation',
      enabled: true,
      gate_status: 'unverified',
      endpoint_pricing_status: 'ready',
      current_price: { base_price: { billing_mode: 'image' } },
    }

    expect(sharedPoolEndpointGateSatisfied(text)).toBe(true)
    expect(sharedPoolEndpointAvailability(text).code).toBe('ready')
    expect(sharedPoolEndpointGateSatisfied(image)).toBe(false)
    expect(sharedPoolEndpointAvailability(image).code).toBe('unverified')
  })

  it('uses human-readable media names and never mixes pricing units', () => {
    expect(sharedPoolEndpointLabel('image_generation')).toBe('图片生成')
    expect(sharedPoolEndpointLabel('image_edit')).toBe('图片编辑')
    expect(sharedPoolEndpointLabel('video')).toBe('视频生成')
    expect(defaultSharedPoolBillingMode('image_generation')).toBe('image')
    expect(defaultSharedPoolBillingMode('video')).toBe('video')
    expect(sharedPoolBillingOptions('image_edit').map((item) => item.value)).toEqual(['image', 'per_request'])
    expect(sharedPoolBillingOptions('video').map((item) => item.value)).toEqual(['video', 'per_request'])
    expect(sharedPoolBillingModeAllowed('video', 'token')).toBe(false)
    expect(sharedPoolBillingExampleLabel('image')).toContain('1 张图片')
    expect(sharedPoolBillingExampleLabel('video')).toContain('默认 8 秒视频')
  })
})
