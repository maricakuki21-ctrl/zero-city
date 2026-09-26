import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import NativePoolPricingEditor, { type NativePricingDraft } from '../NativePoolPricingEditor.vue'
import type { SharedPoolModelEndpointPricing } from '@/api/bizdecipher'

function pricing(source: 'official_catalog' | 'owner_custom'): SharedPoolModelEndpointPricing {
  return {
    pool_model_id: 7,
    provider: 'openai',
    model_name: 'example-model',
    display_name: 'Example model',
    pricing_source: source,
    pricing_status: source === 'official_catalog' ? 'ready' : 'pending',
    pricing_config_version: 1,
    endpoint_id: 9,
    endpoint_type: 'chat',
    enabled: true,
    gate_status: 'unverified',
    endpoint_pricing_status: source === 'official_catalog' ? 'ready' : 'pending',
    config_version: 1,
    current_price: source === 'official_catalog'
      ? {
        price_version_id: 1, pool_id: 1, pool_model_id: 7, endpoint_id: 9, model_name: 'example-model',
        endpoint_type: 'chat', pricing_source: 'official_catalog', pricing_status: 'ready', config_version: 1,
        base_price: { billing_mode: 'token', currency: 'USD', input_price: 0.000001, output_price: 0.000002 },
        multiplier: 1, user_price: { billing_mode: 'token', currency: 'USD' }, effective_from: '2026-09-13T00:00:00Z',
        explicit_free: false, example_cost: 0.01,
      }
      : null,
  } as SharedPoolModelEndpointPricing
}

const draft: NativePricingDraft = {
  billing_mode: 'token',
  input_per_million: 1,
  output_per_million: 2,
  cache_read_per_million: null,
  cache_write_per_million: null,
  image_item_price: null,
  video_second_price: null,
  per_request_price: null,
  multiplier: 1,
  minimum_charge: null,
  maximum_charge: null,
}

describe('NativePoolPricingEditor', () => {
  it('keeps official catalog pricing read-only', () => {
    const wrapper = mount(NativePoolPricingEditor, {
      props: { items: [pricing('official_catalog')], drafts: {} },
    })
    expect(wrapper.text()).toContain('官方目录价格由平台维护')
    expect(wrapper.find('details').exists()).toBe(false)
    expect(wrapper.get('.native-pricing__facts').text()).toContain('输入 $1.0000 / 输出 $2.0000 每百万 token')
    expect(wrapper.text()).toContain('调用未验证')
    expect(wrapper.text()).not.toContain('可调用')
  })

  it('opens owner pricing editor and emits the selected item', async () => {
    const item = pricing('owner_custom')
    const wrapper = mount(NativePoolPricingEditor, {
      props: { items: [item], drafts: { '7:chat': draft } },
    })
    await wrapper.get('details summary').trigger('click')
    await wrapper.get('select').setValue('per_request')
    const change = wrapper.emitted('updateDraft')?.[0] as [SharedPoolModelEndpointPricing, NativePricingDraft]
    expect(change).toEqual([item, { ...draft, billing_mode: 'per_request' }])
    expect(draft.billing_mode).toBe('token')
    await wrapper.setProps({ drafts: { '7:chat': change[1] } })
    const rate = wrapper.findAll('label').find(label => label.text() === '每次请求')!
    await rate.get('input').setValue('0.04')
    expect(wrapper.emitted('updateDraft')?.[1]).toEqual([item, { ...change[1], per_request_price: 0.04 }])
    await wrapper.get('button.native-pricing__save').trigger('click')
    expect(wrapper.emitted('save')?.[0]).toEqual([item])
  })

  it('disables all price editors during an in-flight save', () => {
    const wrapper = mount(NativePoolPricingEditor, {
      props: { items: [pricing('owner_custom')], drafts: { '7:chat': draft }, savingKey: 'other:chat' },
    })
    expect(wrapper.get('fieldset').attributes('disabled')).toBeDefined()
    expect(wrapper.get('button.native-pricing__save').attributes('disabled')).toBeDefined()
  })
})
