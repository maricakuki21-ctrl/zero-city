import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import HarnessResourceDialog from '../HarnessResourceDialog.vue'
const api = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn(), list: vi.fn(), create: vi.fn(), shared: vi.fn() }))
vi.mock('@/api/client', () => ({ apiClient: { get: api.get, post: api.post } }))
vi.mock('@/api/keys', () => ({ list: api.list, create: api.create }))
vi.mock('@/features/bizdecipher/api/bizdecipher', () => ({ createSharedPoolAccessKey: api.shared }))
const picker = {
  name: 'ResourcePicker',
  props: ['modelValue'],
  emits: ['update:modelValue', 'busy'],
  template: `<div><button data-test="official" @click="$emit('update:modelValue', {kind:'official',id:4,name:'官方测试',ready:true})">官方</button><button data-test="shared" @click="$emit('update:modelValue', {kind:'shared',id:17,name:'共享测试',ready:true})">共享</button></div>`,
}
const byText = (wrapper: ReturnType<typeof mount>, text: string) => wrapper.findAll('button').find(b => b.text().includes(text))!
async function render() {
  const wrapper = mount(HarnessResourceDialog, { global: { stubs: { ResourcePicker: picker } } })
  wrapper.vm.showModal()
  await flushPromises()
  return wrapper
}
describe('workbench unified resource connection', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    HTMLDialogElement.prototype.showModal = function () { this.open = true }
    HTMLDialogElement.prototype.close = function () { this.open = false; this.dispatchEvent(new Event('close')) }
    api.list.mockResolvedValue({ items: [], pages: 1, total: 0 })
    api.get.mockResolvedValue({ data: { models: ['available-model'] } })
    api.create.mockResolvedValue({ id: 6 })
    api.shared.mockResolvedValue({ access_key: { api_key_id: 7 } })
  })
  it('connects official resources and emits the selected model', async () => {
    const wrapper = await render()
    await wrapper.get('[data-test=official]').trigger('click')
    await byText(wrapper, '读取此资源的模型').trigger('click')
    await flushPromises()
    expect(api.create).toHaveBeenCalledWith('工作台 · 官方测试', 4)
    await byText(wrapper, '使用此资源').trigger('click')
    expect(wrapper.emitted('select')?.[0]?.[0]).toMatchObject({ keyId: 6, model: 'available-model', slot: 'language' })
    wrapper.unmount()
  })
  it('uses the same in-context shared resource choice without market navigation', async () => {
    const wrapper = await render()
    await wrapper.get('[data-test=shared]').trigger('click')
    await byText(wrapper, '读取此资源的模型').trigger('click')
    await flushPromises()
    expect(api.shared).toHaveBeenCalledWith(17, '工作台 · 共享测试')
    expect(wrapper.find('a').exists()).toBe(false)
    expect(api.get).toHaveBeenCalledWith('/biz/harness/models', { params: { key_id: 7 } })
    wrapper.unmount()
  })
  it('clears a previous model when the resource changes', async () => {
    const wrapper = await render()
    await wrapper.get('[data-test=official]').trigger('click')
    await byText(wrapper, '读取此资源的模型').trigger('click')
    await flushPromises()
    await wrapper.get('[data-test=shared]').trigger('click')
    expect(byText(wrapper, '使用此资源').attributes('disabled')).toBeDefined()
    wrapper.unmount()
  })
  it('loads existing keys through the unified server-paginated list', async () => {
    const wrapper = await render()
    expect(api.list).toHaveBeenCalledWith(1, 20, { scope: 'all', status: 'active', search: '' })
    wrapper.unmount()
  })
})
