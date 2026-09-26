import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get, put } = vi.hoisted(() => ({
  get: vi.fn(),
  put: vi.fn(),
}))

vi.mock('@/api/client', () => ({
  apiClient: { get, put },
}))

vi.mock('@/i18n', () => ({
  getLocale: () => 'zh',
}))

import {
  listSharedPoolModelPricing,
  saveSharedPoolCustomPrice,
  type SaveSharedPoolCustomPricePayload,
} from '@/api/bizdecipher'

describe('shared-pool versioned pricing API', () => {
  beforeEach(() => {
    get.mockReset()
    put.mockReset()
  })

  it('reads endpoint, source, base price and user price without losing DTO fields', async () => {
    const item = {
      pool_model_id: 12,
      provider: 'openai',
      model_name: 'owner-model',
      display_name: 'Owner Model',
      pricing_source: 'owner_custom',
      pricing_status: 'ready',
      pricing_config_version: 3,
      endpoint_id: 42,
      endpoint_type: 'chat',
      enabled: true,
      gate_status: 'passed',
      endpoint_pricing_status: 'ready',
      config_version: 4,
      current_price: {
        price_version_id: 100,
        pool_id: 9,
        pool_model_id: 12,
        endpoint_id: 42,
        model_name: 'owner-model',
        endpoint_type: 'chat',
        pricing_source: 'owner_custom',
        pricing_status: 'ready',
        config_version: 4,
        base_price: { billing_mode: 'token', currency: 'USD', input_price: 0.000002, output_price: 0.000008 },
        multiplier: 1.25,
        user_price: { billing_mode: 'token', currency: 'USD', input_price: 0.0000025, output_price: 0.00001 },
        effective_from: '2026-07-19T00:00:00Z',
        explicit_free: false,
        example_cost: 0.0075,
      },
    }
    get.mockResolvedValue({ data: { items: [item] } })

    const result = await listSharedPoolModelPricing(9)

    expect(get).toHaveBeenCalledWith('/biz/pools/9/pricing')
    expect(result[0].current_price?.price_version_id).toBe(100)
    expect(result[0].current_price?.user_price.output_price).toBe(0.00001)
  })

  it('sends an explicit operation id and complete custom token price', async () => {
    const payload: SaveSharedPoolCustomPricePayload = {
      model_name: 'owner-model',
      endpoint_type: 'responses',
      operation_id: 'shared-price:9:12:responses:fixed',
      billing_mode: 'token',
      input_price: 0.000002,
      output_price: 0.000008,
      cache_read_price: null,
      cache_write_price: null,
      per_request_price: null,
      multiplier: 1.25,
      minimum_charge: null,
      maximum_charge: 1,
    }
    put.mockResolvedValue({ data: { price: { price_version_id: 101 } } })

    const result = await saveSharedPoolCustomPrice(9, payload)

    expect(put).toHaveBeenCalledWith('/biz/pools/9/pricing', payload)
    expect(result.price_version_id).toBe(101)
  })

  it('preserves image pricing fields and sends a USD-per-image custom price', async () => {
    const item = {
      pool_model_id: 22,
      provider: 'openai',
      model_name: 'owner-image-model',
      display_name: 'Owner Image Model',
      pricing_source: 'owner_custom',
      pricing_status: 'ready',
      pricing_config_version: 2,
      endpoint_id: 52,
      endpoint_type: 'image_generation',
      enabled: true,
      gate_status: 'passed',
      endpoint_pricing_status: 'ready',
      config_version: 2,
      current_price: {
        price_version_id: 110,
        pool_id: 9,
        pool_model_id: 22,
        endpoint_id: 52,
        model_name: 'owner-image-model',
        endpoint_type: 'image_generation',
        pricing_source: 'owner_custom',
        pricing_status: 'ready',
        config_version: 2,
        base_price: { billing_mode: 'image', currency: 'USD', image_item_price: 0.08 },
        multiplier: 1.5,
        user_price: { billing_mode: 'image', currency: 'USD', image_item_price: 0.12 },
        effective_from: '2026-07-19T00:00:00Z',
        explicit_free: false,
        example_cost: 0.12,
      },
    }
    get.mockResolvedValue({ data: { items: [item] } })

    const result = await listSharedPoolModelPricing(9)

    expect(result[0].endpoint_type).toBe('image_generation')
    expect(result[0].current_price?.user_price.image_item_price).toBe(0.12)

    const payload: SaveSharedPoolCustomPricePayload = {
      model_name: 'owner-image-model',
      endpoint_type: 'image_generation',
      operation_id: 'shared-price:9:22:image:fixed',
      billing_mode: 'image',
      image_item_price: 0.08,
      multiplier: 1.5,
      minimum_charge: null,
      maximum_charge: null,
    }
    put.mockResolvedValue({ data: { price: { price_version_id: 111 } } })

    await saveSharedPoolCustomPrice(9, payload)

    expect(put).toHaveBeenCalledWith('/biz/pools/9/pricing', payload)
  })

  it('keeps video seconds explicit instead of pretending they are image or token units', async () => {
    const payload: SaveSharedPoolCustomPricePayload = {
      model_name: 'owner-video-model',
      endpoint_type: 'video',
      operation_id: 'shared-price:9:23:video:fixed',
      billing_mode: 'video',
      video_second_price: 0.03,
      multiplier: 1.2,
      minimum_charge: null,
      maximum_charge: null,
    }
    put.mockResolvedValue({ data: { price: { price_version_id: 112 } } })

    await saveSharedPoolCustomPrice(9, payload)

    expect(put).toHaveBeenCalledWith('/biz/pools/9/pricing', payload)
  })
})
