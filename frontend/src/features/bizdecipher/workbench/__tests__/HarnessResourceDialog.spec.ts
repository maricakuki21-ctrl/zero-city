import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import HarnessResourceDialog from '../HarnessResourceDialog.vue'

const api = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
  list: vi.fn(),
  create: vi.fn(),
  shared: vi.fn(),
}))

vi.mock('@/api/client', () => ({ apiClient: { get: api.get, post: api.post } }))
vi.mock('@/api/keys', () => ({ list: api.list, create: api.create }))
vi.mock('@/features/bizdecipher/api/bizdecipher', () => ({ createSharedPoolAccessKey: api.shared }))

const resourcePicker = {
  name: 'ResourcePicker',
  props: ['modelValue'],
  emits: ['update:modelValue', 'busy'],
  template: `
    <div>
      <button data-test="official" @click="$emit('update:modelValue', { kind: 'official', id: 4, name: '官方测试', ready: true })">官方</button>
      <button data-test="shared" @click="$emit('update:modelValue', { kind: 'shared', id: 17, name: '共享测试', ready: true })">共享</button>
    </div>
  `,
}

const byText = (wrapper: ReturnType<typeof mount>, text: string) =>
  wrapper.findAll('button').find(button => button.text().includes(text))!

async function render() {
  const wrapper = mount(HarnessResourceDialog, { global: { stubs: { ResourcePicker: resourcePicker } } })
  wrapper.vm.showModal()
  await flushPromises()
  return wrapper
}

describe('HarnessResourceDialog unified resource flow', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    HTMLDialogElement.prototype.showModal = function () { this.open = true }
    HTMLDialogElement.prototype.close = function () { this.open = false; this.dispatchEvent(new Event('close')) }
    api.list.mockResolvedValue({ items: [], pages: 1, total: 0 })
    api.get.mockResolvedValue({ data: { models: ['available-model'] } })
    api.post.mockResolvedValue({ data: { models: ['custom-model'] } })
    api.create.mockResolvedValue({ id: 6 })
    api.shared.mockResolvedValue({ access_key: { api_key_id: 7 } })
  })

  it('同一弹窗连接官方资源并发出模型选择', async () => {
    const wrapper = await render()
    await wrapper.get('[data-test=official]').trigger('click')
    await byText(wrapper, '读取此资源的模型').trigger('click')
    await flushPromises()

    expect(api.create).toHaveBeenCalledWith('工作台 · 官方测试', 4)
    await byText(wrapper, '使用此资源').trigger('click')
    expect(wrapper.emitted('select')?.[0]?.[0]).toMatchObject({
      keyId: 6,
      model: 'available-model',
      slot: 'language',
      customKey: '',
    })
  })

  it('共享资源在当前上下文加入并创建访问密钥，不跳转共享市场', async () => {
    const wrapper = await render()
    await wrapper.get('[data-test=shared]').trigger('click')
    await byText(wrapper, '读取此资源的模型').trigger('click')
    await flushPromises()

    expect(api.shared).toHaveBeenCalledWith(17, '工作台 · 共享测试')
    expect(wrapper.find('a').exists()).toBe(false)
    expect(api.get).toHaveBeenCalledWith('/biz/harness/models', { params: { key_id: 7 } })
  })

  it('优先复用已有密钥，不重复创建密钥', async () => {
    api.list.mockResolvedValue({ items: [{ id: 12, name: '已有密钥' }], pages: 1, total: 1 })
    const wrapper = await render()
    await byText(wrapper, '已有密钥').trigger('click')
    await flushPromises()

    expect(api.get).toHaveBeenCalledWith('/biz/harness/models', { params: { key_id: 12 } })
    expect(api.create).not.toHaveBeenCalled()
    expect(wrapper.get('select').element.value).toBe('available-model')
  })

  it('自定义密钥只用于当前页面，输入变化会清空旧模型', async () => {
    const wrapper = await render()
    const input = wrapper.get('input[type=password]')
    await input.setValue('example-key')
    await byText(wrapper, '读取模型').trigger('click')
    await flushPromises()

    expect(api.post).toHaveBeenCalledWith('/biz/harness/models', { customKey: 'example-key' })
    expect(wrapper.get('select').element.value).toBe('custom-model')
    await input.setValue('different-key')
    expect(wrapper.get('footer .primary').attributes('disabled')).toBeDefined()
  })

  it('切换账号上下文后丢弃旧的模型响应', async () => {
    let finish: ((value: { data: { models: string[] } }) => void) | undefined
    api.get.mockImplementation(() => new Promise(resolve => { finish = resolve }))
    const wrapper = await render()
    await wrapper.get('[data-test=official]').trigger('click')
    await byText(wrapper, '读取此资源的模型').trigger('click')
    wrapper.vm.resetForAccountSwitch()
    finish?.({ data: { models: ['stale-model'] } })
    await flushPromises()

    expect(wrapper.find('select').exists()).toBe(false)
    expect(wrapper.get('footer .primary').attributes('disabled')).toBeDefined()
  })
})
