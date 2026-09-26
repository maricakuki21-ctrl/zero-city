import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import TavernAIHost from './TavernAIHost.vue'

const mocks = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn() }))
vi.mock('@/api/client', () => ({ apiClient: mocks }))
const model = {
  id: 'text', version: '1', digest: 'digest', title: 'Text model', protocol: 'chat',
  canonical_model_id: 'text', canonical_model_version: '1',
  accepted_quote_id: 'quote', accepted_quote_sha: 'sha', estimate: { currency: 'USD', amount: '0.10' },
}
let wrapper: VueWrapper
beforeEach(async () => {
  sessionStorage.clear()
  mocks.get.mockReset().mockResolvedValue({ data: { capabilities: [model, { ...model, id: 'image', protocol: 'image' }] } })
  mocks.post.mockReset()
  wrapper = mount(TavernAIHost, { props: { roomId: 9, throughTurn: 3 }, global: { stubs: { RouterLink: true } } })
  await flushPromises()
})
afterEach(() => wrapper.unmount())
describe('tavern owner AI turn', () => {
  it('requires consent and only lists text models', async () => {
    expect(wrapper.findAll('option')).toHaveLength(1)
    expect(wrapper.get('.ai-generate').attributes('disabled')).toBeDefined()
    await wrapper.get('input[type="checkbox"]').setValue(true)
    expect(wrapper.get('.ai-generate').attributes('disabled')).toBeUndefined()
    expect(mocks.post).not.toHaveBeenCalled()
  })
  it('preserves the same request and context when a network result is uncertain', async () => {
    mocks.post.mockRejectedValueOnce(new Error('network')).mockResolvedValueOnce({ data: { recorded: true, run: { state: 'succeeded' } } })
    await wrapper.get('input[type="checkbox"]').setValue(true)
    await wrapper.get('.ai-generate').trigger('click')
    await flushPromises()
    const first = { ...mocks.post.mock.calls[0][1] }
    await wrapper.setProps({ throughTurn: 5 })
    await wrapper.get('.ai-generate').trigger('click')
    await flushPromises()
    expect(mocks.post.mock.calls[1][1]).toEqual(first)
    expect(first.through_turn).toBe(3)
    expect(wrapper.emitted('recorded')).toHaveLength(1)
    expect(sessionStorage.getItem('tavern-ai-request:9')).toBeNull()
  })
  it('does not claim a room turn was written when persistence failed', async () => {
    mocks.post.mockResolvedValue({ data: { recorded: false, message: '房间已关闭', run: { state: 'succeeded' } } })
    await wrapper.get('input[type="checkbox"]').setValue(true)
    await wrapper.get('.ai-generate').trigger('click')
    await flushPromises()
    expect(wrapper.emitted('recorded')).toBeUndefined()
    expect(wrapper.text()).toContain('房间已关闭')
    expect(wrapper.get('.ai-generate').text()).toContain('重试')
  })
  it('allows a new explicitly authorized request after a confirmed terminal failure', async () => {
    mocks.post.mockResolvedValue({ data: { recorded: false, run: { state: 'failed', failure: { message: '模型请求失败' } } } })
    await wrapper.get('input[type="checkbox"]').setValue(true)
    await wrapper.get('.ai-generate').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('模型请求失败')
    expect(wrapper.get('.ai-generate').attributes('disabled')).toBeDefined()
    expect((wrapper.get('input[type="checkbox"]').element as HTMLInputElement).checked).toBe(false)
    expect(sessionStorage.getItem('tavern-ai-request:9')).toBeNull()
  })
  it('returns to resource confirmation when authorization expires before dispatch', async () => {
    mocks.post.mockRejectedValue({ reason: 'WORKBENCH_CATALOG_UNAVAILABLE', message: '授权过期' })
    await wrapper.get('input[type="checkbox"]').setValue(true)
    await wrapper.get('.ai-generate').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('授权过期')
    expect(wrapper.get('select').exists()).toBe(true)
    expect(wrapper.get('.ai-generate').attributes('disabled')).toBeDefined()
    expect(sessionStorage.getItem('tavern-ai-request:9')).toBeNull()
  })
})
