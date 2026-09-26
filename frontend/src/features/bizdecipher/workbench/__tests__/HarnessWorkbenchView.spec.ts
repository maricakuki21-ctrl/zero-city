import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import HarnessWorkbenchView from '../HarnessWorkbenchView.vue'

const api = vi.hoisted(() => ({ get: vi.fn(), listKeys: vi.fn() }))
const authState = vi.hoisted(() => ({ store: null as unknown as { user: { id: number } | null } }))
const clipboard = vi.hoisted(() => ({ copy: vi.fn(), copied: { value: false } }))
const dialogMethods = vi.hoisted(() => ({
  showModal: vi.fn(),
  showRequested: vi.fn(),
  resetForAccountSwitch: vi.fn(),
}))

vi.mock('@/api/client', () => ({ apiClient: api, buildApiUrl: (path: string) => path }))
vi.mock('@/api/keys', () => ({ list: api.listKeys }))
vi.mock('@/stores/auth', async () => {
  const { reactive } = await import('vue')
  authState.store = reactive({ user: { id: 41 } })
  return { useAuthStore: () => authState.store }
})
vi.mock('@/composables/useClipboard', () => ({ useClipboard: () => ({ copied: clipboard.copied, copyToClipboard: clipboard.copy }) }))
vi.mock('@/components/layout/AppLayout.vue', () => ({ default: { template: '<div><slot /></div>' } }))
const routeState = vi.hoisted(() => ({ query: {} as Record<string, unknown> }))
vi.mock('vue-router', () => ({
  RouterLink: { name: 'RouterLink', props: ['to'], template: '<a><slot /></a>' },
  useRoute: () => routeState,
}))
vi.mock('../HarnessResourceDialog.vue', () => ({
  default: { name: 'HarnessResourceDialog', emits: ['select'], template: '<div />', methods: dialogMethods },
}))
vi.mock('../HarnessAssetDraftDialog.vue', () => ({
  default: { name: 'HarnessAssetDraftDialog', props: ['result'], emits: ['saved'], template: '<div />', methods: { showModal() {}, resetForAccountSwitch() {} } },
}))

const completedResponse = (text = '可沉淀的完成结果') => new Response(
  `${JSON.stringify({ type: 'completed', text, images: [] })}\n`,
  { status: 200 },
)

beforeEach(() => {
  localStorage.clear()
  routeState.query = {}
  vi.resetAllMocks()
  authState.store.user = { id: 41 }
  api.get.mockResolvedValue({ data: { skills: [] } })
  api.listKeys.mockResolvedValue({ items: [] })
  clipboard.copy.mockResolvedValue(true)
  vi.stubGlobal('fetch', vi.fn(async () => completedResponse()))
  dialogMethods.showModal.mockReset()
  dialogMethods.showRequested.mockReset()
  dialogMethods.resetForAccountSwitch.mockReset()
})

afterEach(() => { vi.unstubAllGlobals() })

describe('Harness workbench account-safe history', () => {
  it('reopens only newly verified results as completed and retains the confirmed asset id', async () => {
    const wrapper = mount(HarnessWorkbenchView)
    await flushPromises()
    wrapper.getComponent({ name: 'HarnessResourceDialog' }).vm.$emit('select', {
      slot: 'language', keyId: 12, model: 'test-model', customKey: '', label: '测试资源',
    })
    await wrapper.get('[aria-label="创作目标"]').setValue('完成一个可保存结果')
    await wrapper.get('[aria-label="发送任务"]').trigger('click')
    await flushPromises()

    expect(wrapper.get('[aria-label="保存为资产"]').text()).toContain('保存为资产')
    await wrapper.get('[aria-label="保存为资产"]').trigger('click')
    wrapper.getComponent({ name: 'HarnessAssetDraftDialog' }).vm.$emit('saved', 73)
    await wrapper.vm.$nextTick()
    expect(JSON.parse(localStorage.getItem('harness-drafts-v1-41') || '[]')[0]).toMatchObject({ resultState: 'verified', savedAssetId: 73 })

    await wrapper.get('button.new-session').trigger('click')
    await wrapper.findAll('.project-row')[0]?.trigger('click')
    const link = wrapper.findAllComponents({ name: 'RouterLink' }).find(item => item.text().includes('查看资产草稿'))
    expect(link?.props('to')).toEqual({ path: '/assets', query: { tab: 'mine', asset: '73' } })
    wrapper.unmount()
  })

  it('does not label a legacy draft with ambiguous answer text as completed', async () => {
    localStorage.setItem('harness-drafts-v1-41', JSON.stringify([{
      id: 'legacy', title: '旧草稿', intent: '旧输入', answer: '旧答案', submitted: '旧输入', skillIds: [],
      languageKeyId: 1, languageModel: 'old-model', imageKeyId: 0, imageModel: '', images: [],
    }]))
    const wrapper = mount(HarnessWorkbenchView)
    await flushPromises()
    await wrapper.get('.project-row').trigger('click')

    expect(wrapper.text()).toContain('完成状态未经确认')
    expect(wrapper.find('[aria-label="保存为资产"]').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('已确认完成')
    wrapper.unmount()
  })

  it('switches storage namespaces and ignores delayed catalog and run responses from the previous account', async () => {
    let finishCatalog: ((value: { data: { skills: { id: string; title: string; description: string }[] } }) => void) | undefined
    api.get
      .mockImplementationOnce(() => new Promise(resolve => { finishCatalog = resolve }))
      .mockResolvedValueOnce({ data: { skills: [{ id: 'new-skill', title: '新账号技能', description: 'new' }] } })
    let finishRun: ((value: Response) => void) | undefined
    vi.stubGlobal('fetch', vi.fn(() => new Promise(resolve => { finishRun = resolve })))
    const wrapper = mount(HarnessWorkbenchView)
    await flushPromises()

    wrapper.getComponent({ name: 'HarnessResourceDialog' }).vm.$emit('select', {
      slot: 'language', keyId: 12, model: 'old-model', customKey: '', label: '旧资源',
    })
    await wrapper.get('[aria-label="创作目标"]').setValue('旧账号任务')
    await wrapper.get('[aria-label="发送任务"]').trigger('click')
    authState.store.user = { id: 42 }
    await flushPromises()
    finishCatalog?.({ data: { skills: [{ id: 'old-skill', title: '旧账号技能', description: 'old' }] } })
    finishRun?.(completedResponse('旧账号迟到结果'))
    await flushPromises()

    expect(wrapper.text()).not.toContain('旧账号迟到结果')
    expect(wrapper.text()).not.toContain('旧账号技能')
    expect(wrapper.text()).toContain('新账号技能')
    expect(localStorage.getItem('harness-drafts-v1-42')).toBeNull()
    expect(wrapper.get('[aria-label="创作目标"]').element).toHaveProperty('value', '')
    wrapper.unmount()
  })

  it('ignores a delayed asset saved event after an account switch', async () => {
    const wrapper = mount(HarnessWorkbenchView)
    await flushPromises()
    wrapper.getComponent({ name: 'HarnessResourceDialog' }).vm.$emit('select', {
      slot: 'language', keyId: 12, model: 'test-model', customKey: '', label: '测试资源',
    })
    await wrapper.get('[aria-label="创作目标"]').setValue('旧账号完成结果')
    await wrapper.get('[aria-label="发送任务"]').trigger('click')
    await flushPromises()
    const oldDialog = wrapper.getComponent({ name: 'HarnessAssetDraftDialog' })
    await wrapper.get('[aria-label="保存为资产"]').trigger('click')
    authState.store.user = { id: 42 }
    await flushPromises()
    oldDialog.vm.$emit('saved', 91)
    await wrapper.vm.$nextTick()

    expect(localStorage.getItem('harness-drafts-v1-42')).toBeNull()
    expect(wrapper.text()).not.toContain('查看资产草稿')
    wrapper.unmount()
  })

  it('opens a requested market resource only for the initial account context', async () => {
    routeState.query = { resourceSource: 'official', resourceId: '2' }
    const wrapper = mount(HarnessWorkbenchView)
    await flushPromises()

    expect(dialogMethods.showRequested).toHaveBeenCalledTimes(1)
    expect(dialogMethods.showRequested).toHaveBeenCalledWith('official', 2)
    expect(wrapper.text()).toContain('确认后才会使用')

    dialogMethods.showRequested.mockClear()
    authState.store.user = { id: 42 }
    await flushPromises()

    expect(dialogMethods.resetForAccountSwitch).toHaveBeenCalled()
    expect(dialogMethods.showRequested).not.toHaveBeenCalled()
    wrapper.unmount()
  })
})
