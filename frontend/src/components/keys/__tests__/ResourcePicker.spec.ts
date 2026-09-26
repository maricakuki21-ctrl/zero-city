import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import ResourcePicker from '../ResourcePicker.vue'

const api = vi.hoisted(() => ({
  groups: vi.fn(), pools: vi.fn(), seats: vi.fn(), detail: vi.fn(), join: vi.fn(), pricing: vi.fn(),
}))
vi.mock('@/api/groups', () => ({ getAvailable: api.groups }))
vi.mock('@/features/bizdecipher/api/bizdecipher', () => ({
  listSharedPools: api.pools, listMySeats: api.seats, getSharedPool: api.detail, joinSharedPool: api.join,
  listSharedPoolModelPricing: api.pricing,
}))
const pool = { id: 17, name: '创作者资源', models: ['image-model'], status: 'active', max_users: 10, current_users: 1, hourly_seat_fee: 0.1, min_balance_admission: 2 }
const seat = { id: 1, pool_id: 17, pool_name: pool.name, status: 'active' }
const button = (wrapper: ReturnType<typeof mount>, text: string) => wrapper.findAll('button').find(b => b.text().includes(text))!
async function render() {
  const wrapper = mount(ResourcePicker, { props: { modelValue: null } })
  await flushPromises()
  return wrapper
}
describe('unified resource selection', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    api.groups.mockResolvedValue([{ id: 4, name: '官方资源', platform: 'openai' }])
    api.pools.mockResolvedValue({ pools: [pool], total: 1 })
    api.seats.mockResolvedValue([])
    api.detail.mockResolvedValue(pool)
    api.pricing.mockResolvedValue([])
    api.join.mockResolvedValue({ seat, already_held: false })
  })
  it('shows both sources together without tabs or navigation', async () => {
    const wrapper = await render()
    expect(wrapper.text()).toContain('官方资源')
    expect(wrapper.text()).toContain('创作者资源')
    expect(wrapper.find('a').exists()).toBe(false)
    expect(wrapper.find('[role=tablist]').exists()).toBe(false)
    await button(wrapper, '官方资源').trigger('click')
    expect(wrapper.emitted('update:modelValue')?.at(-1)?.[0]).toEqual({ kind: 'official', id: 4, name: '官方资源', ready: true })
    wrapper.unmount()
  })
  it('does not join on selection and requires explicit fee consent', async () => {
    const wrapper = await render()
    await button(wrapper, '创作者资源').trigger('click')
    await flushPromises()
    expect(api.join).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('$0.1 / 小时')
    expect(button(wrapper, '确认加入此资源').attributes('disabled')).toBeDefined()
    expect(wrapper.emitted('update:modelValue')?.at(-1)?.[0]).toMatchObject({ ready: false })
    await wrapper.get('input[type=checkbox]').setValue(true)
    await button(wrapper, '确认加入此资源').trigger('click')
    await flushPromises()
    expect(api.join).toHaveBeenCalledTimes(1)
    expect(api.join).toHaveBeenCalledWith(17)
    expect(wrapper.emitted('update:modelValue')?.at(-1)?.[0]).toMatchObject({ ready: true })
    wrapper.unmount()
  })
  it('uses an existing membership without another join', async () => {
    api.seats.mockResolvedValue([seat])
    const wrapper = await render()
    await button(wrapper, '创作者资源').trigger('click')
    await flushPromises()
    expect(wrapper.emitted('update:modelValue')?.at(-1)?.[0]).toMatchObject({ ready: true })
    expect(api.join).not.toHaveBeenCalled()
    wrapper.unmount()
  })
  it('reconciles a timed-out join instead of repeating the write', async () => {
    const wrapper = await render()
    await button(wrapper, '创作者资源').trigger('click')
    await flushPromises()
    api.join.mockRejectedValueOnce(new Error('timeout'))
    api.seats.mockResolvedValue([seat])
    await wrapper.get('input[type=checkbox]').setValue(true)
    await button(wrapper, '确认加入此资源').trigger('click')
    await flushPromises()
    expect(api.join).toHaveBeenCalledTimes(1)
    expect(wrapper.emitted('update:modelValue')?.at(-1)?.[0]).toMatchObject({ ready: true })
    expect(wrapper.text()).toContain('无需重复加入')
    wrapper.unmount()
  })
  it('does not offer joining a full resource', async () => {
    api.detail.mockResolvedValue({ ...pool, current_users: 10 })
    const wrapper = await render()
    await button(wrapper, '创作者资源').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('席位已满')
    expect(wrapper.find('input[type=checkbox]').exists()).toBe(false)
    expect(api.join).not.toHaveBeenCalled()
    wrapper.unmount()
  })
  it.each([
    { owner_paused: true }, { listed: false }, { status: 'maintenance' },
    { native_onboarding_state: 'supply_ready_billing_blocked' },
  ])('does not treat membership as usable when the pool is unavailable: %j', async (change) => {
    api.detail.mockResolvedValue({ ...pool, ...change })
    api.seats.mockResolvedValue([seat])
    const wrapper = await render()
    await button(wrapper, '创作者资源').trigger('click')
    await flushPromises()
    expect(wrapper.emitted('update:modelValue')?.at(-1)?.[0]).toMatchObject({ ready: false })
    expect(wrapper.text()).toContain('现有席位不代表现在可以调用')
    expect(wrapper.find('input[type=checkbox]').exists()).toBe(false)
    expect(api.join).not.toHaveBeenCalled()
    wrapper.unmount()
  })
  it('shows endpoint-specific conditions without claiming all protocols work', async () => {
    api.pricing.mockResolvedValue([
      { pool_model_id: 1, endpoint_id: 11, model_name: 'text-model', endpoint_type: 'chat', enabled: true, endpoint_pricing_status: 'ready', current_price: { base_price: { billing_mode: 'token' } } },
      { pool_model_id: 2, endpoint_id: 12, model_name: 'image-model', endpoint_type: 'image_generation', enabled: true, gate_status: 'failed' },
      { pool_model_id: 3, endpoint_id: 13, model_name: 'disabled-model', endpoint_type: 'responses', enabled: false },
    ])
    const wrapper = await render()
    await button(wrapper, '创作者资源').trigger('click')
    await flushPromises()
    const details = wrapper.get('.resource-capabilities')
    expect(details.text()).toContain('文字对话 · 可调用')
    expect(details.text()).toContain('图片生成 · 检测未通过')
    expect(details.text()).toContain('Embeddings 和 WebSocket 尚未开放')
    expect(details.text()).not.toContain('disabled-model')
    expect(api.join).not.toHaveBeenCalled()
    wrapper.unmount()
  })
  it('distinguishes an unreadable price list from an unavailable pool', async () => {
    api.pricing.mockRejectedValueOnce(new Error('timeout'))
    api.seats.mockResolvedValue([seat])
    const wrapper = await render()
    await button(wrapper, '创作者资源').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('模型报价暂不可读')
    expect(wrapper.emitted('update:modelValue')?.at(-1)?.[0]).toMatchObject({ ready: true })
    wrapper.unmount()
  })
})
