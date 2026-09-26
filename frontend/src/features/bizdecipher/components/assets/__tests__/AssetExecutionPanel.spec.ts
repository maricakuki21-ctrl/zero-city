import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import AssetExecutionPanel from '../AssetExecutionPanel.vue'

const mocks = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn(), auth: { user: { id: 7 } } }))
vi.mock('@/api/client', () => ({ apiClient: { get: mocks.get, post: mocks.post } }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => mocks.auth }))

describe('AssetExecutionPanel', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    sessionStorage.clear()
    mocks.get.mockImplementation(async (url: string) => {
      if (url.endsWith('/execution')) return { data: { plan_digest: 'plan-sha', package_digest: 'package-sha', prompt: 'Summarize', permissions: ['model.generate'] } }
      if (url.endsWith('/workspace')) return { data: { capabilities: [{
        id: 'model-1', version: 'v1', digest: 'cap-sha', title: 'Selected resource', protocol: 'chat',
        canonical_model_id: 'actual-model', canonical_model_version: 'v1', accepted_quote_id: 'quote-1',
        accepted_quote_sha: 'quote-sha', estimate: { currency: 'USD', amount: '0.1' },
      }] } }
      if (url.endsWith('/content')) return { data: 'verified output' }
      return { data: { run: { id: 'run-1', state: 'succeeded', artifacts: [{ artifact_id: 'artifact-1' }] } } }
    })
    mocks.post.mockResolvedValue({ data: { run: { id: 'run-1', state: 'queued', artifacts: [] } } })
  })
  it('requires quote/permission confirmation, launches a pinned plan and saves real run results', async () => {
    const wrapper = mount(AssetExecutionPanel, { props: { assetId: 41, version: 'v1' } })
    await flushPromises()
    const launch = wrapper.findAll('button').find(b => b.text() === '运行')!
    expect(launch.attributes('disabled')).toBeDefined()
    expect(wrapper.text()).not.toContain('quote-1')
    expect(wrapper.text()).toContain('按现行价格及实际用量结算')
    await wrapper.get('textarea').setValue('user input')
    await wrapper.get('input[type=checkbox]').setValue(true)
    await launch.trigger('click')
    await flushPromises()
    expect(mocks.post).toHaveBeenCalledWith('/biz/assets/41/versions/v1/execution', expect.objectContaining({
      plan_digest: 'plan-sha', permissions: ['model.generate'], intent: 'user input',
      accepted_quote_id: 'quote-1', accepted_quote_sha: 'quote-sha', capability_id: 'model-1',
      request_id: expect.any(String),
    }), { timeout: 180000 })
    expect(wrapper.text()).toContain('verified output')
    await wrapper.findAll('button').find(b => b.text() === '保存运行结果')!.trigger('click')
    await flushPromises()
    expect(mocks.post).toHaveBeenCalledWith('/biz/workbench/runs/run-1/save', expect.any(Object), expect.objectContaining({
      headers: { 'Idempotency-Key': 'asset-save-run-1' },
    }))
    expect(wrapper.text()).toContain('已保存')
    wrapper.unmount()
  })
  it('retains operation ID after an uncertain launch failure', async () => {
    mocks.post.mockRejectedValue(new Error('network'))
    const wrapper = mount(AssetExecutionPanel, { props: { assetId: 41, version: 'v1' } })
    await flushPromises()
    await wrapper.get('textarea').setValue('user input')
    await wrapper.get('input[type=checkbox]').setValue(true)
    const launch = wrapper.findAll('button').find(b => b.text() === '运行')!
    await launch.trigger('click'); await flushPromises()
    await wrapper.findAll('button').find(b => b.text() === '重试原请求')!.trigger('click'); await flushPromises()
    expect(mocks.post.mock.calls[1][1].request_id).toBe(mocks.post.mock.calls[0][1].request_id)
    wrapper.unmount()
  })
  it('retains the exact selected resource authorization after a reload', async () => {
    mocks.post.mockRejectedValue(new Error('network'))
    const wrapper = mount(AssetExecutionPanel, { props: { assetId: 41, version: 'v1' } })
    await flushPromises()
    await wrapper.get('textarea').setValue('user input')
    await wrapper.get('input[type=checkbox]').setValue(true)
    await wrapper.findAll('button').find(b => b.text() === '运行')!.trigger('click')
    await flushPromises()
    const first = mocks.post.mock.calls[0][1]
    wrapper.unmount()
    const restored = mount(AssetExecutionPanel, { props: { assetId: 41, version: 'v1' } })
    await flushPromises()
    expect(restored.find('textarea').exists()).toBe(false)
    await restored.findAll('button').find(b => b.text() === '重试原请求')!.trigger('click')
    await flushPromises()
    expect(mocks.post.mock.calls[1][1]).toEqual(first)
    restored.unmount()
  })
  it('preserves in-memory pending on refresh when session storage is unavailable', async () => {
    const getItem = vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => { throw new Error('blocked') })
    const setItem = vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => { throw new Error('blocked') })
    mocks.post.mockRejectedValue(new Error('network'))
    const wrapper = mount(AssetExecutionPanel, { props: { assetId: 41, version: 'v1' } })
    await flushPromises()
    await wrapper.get('textarea').setValue('user input')
    await wrapper.get('input[type=checkbox]').setValue(true)
    await wrapper.findAll('button').find(b => b.text() === '运行')!.trigger('click')
    await flushPromises()
    const original = mocks.post.mock.calls[0][1]
    await wrapper.findAll('button').find(b => b.text() === '刷新')!.trigger('click')
    await flushPromises()
    await wrapper.findAll('button').find(b => b.text() === '重试原请求')!.trigger('click')
    await flushPromises()
    expect(mocks.post.mock.calls[1][1]).toEqual(original)
    wrapper.unmount()
    getItem.mockRestore(); setItem.mockRestore()
  })
  it('allows resource reselection after a conclusive pre-dispatch expiry rejection', async () => {
    mocks.post.mockRejectedValue({ reason: 'WORKBENCH_CATALOG_UNAVAILABLE', message: 'expired' })
    const wrapper = mount(AssetExecutionPanel, { props: { assetId: 41, version: 'v1' } })
    await flushPromises()
    await wrapper.get('textarea').setValue('user input')
    await wrapper.get('input[type=checkbox]').setValue(true)
    await wrapper.findAll('button').find(b => b.text() === '运行')!.trigger('click')
    await flushPromises()
    expect(wrapper.text()).not.toContain('重试原请求')
    expect(wrapper.get('input[type=checkbox]').element).toHaveProperty('checked', false)
    expect(sessionStorage.getItem('asset-run:7:41:v1')).toBeNull()
    wrapper.unmount()
  })
  it('opens older saved asset outputs without dispatching another model request', async () => {
    const original = mocks.get.getMockImplementation()!
    mocks.get.mockImplementation(async (url: string) => {
      const result = await original(url)
      if (url.endsWith('/workspace')) result.data.saved_snapshots = [
        { id: 'saved-1', run_id: 'old-run', label: 'Older output', input: { intent: '[asset-declarative/v1] plan=plan-sha\n' } },
      ]
      return result
    })
    const wrapper = mount(AssetExecutionPanel, { props: { assetId: 41, version: 'v1' } })
    await flushPromises()
    await wrapper.findAll('button').find(b => b.text() === 'Older output')!.trigger('click')
    await flushPromises()
    expect(mocks.get).toHaveBeenCalledWith('/biz/workbench/runs/old-run')
    expect(wrapper.text()).toContain('verified output')
    expect(mocks.post).not.toHaveBeenCalled()
    wrapper.unmount()
  })
  it('does not launch an unavailable or unentitled asset', async () => {
    mocks.get.mockRejectedValue(new Error('forbidden'))
    const wrapper = mount(AssetExecutionPanel, { props: { assetId: 41, version: 'v1' } })
    await flushPromises()
    expect(wrapper.findAll('button').some(b => b.text() === '运行')).toBe(false)
    expect(mocks.post).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('advances a multi-step workflow with one immutable request and displays its persisted result', async () => {
    const original = mocks.get.getMockImplementation()!
    mocks.get.mockImplementation(async (url: string) => {
      const result = await original(url)
      if (url.endsWith('/execution')) result.data.steps = [{ action: 'model.generate' }, { action: 'model.generate' }]
      return result
    })
    mocks.post.mockResolvedValueOnce({ data: { workflow: { request_id: 'workflow-id', state: 'running', total: 2, steps: [{ index: 0, action: 'model.generate', state: 'succeeded' }], output: 'first' } } })
      .mockResolvedValueOnce({ data: { workflow: { request_id: 'workflow-id', state: 'succeeded', total: 2, steps: [{ index: 0, action: 'model.generate', state: 'succeeded' }, { index: 1, action: 'model.generate', state: 'succeeded' }], output: 'final output' } } })
    const wrapper = mount(AssetExecutionPanel, { props: { assetId: 41, version: 'v1' } })
    await flushPromises()
    await wrapper.get('textarea').setValue('input')
    await wrapper.get('input[type=checkbox]').setValue(true)
    await wrapper.findAll('button').find(b => b.text() === '运行')!.trigger('click')
    await flushPromises()
    expect(mocks.post).toHaveBeenCalledTimes(2)
    expect(mocks.post.mock.calls[1][1]).toEqual(mocks.post.mock.calls[0][1])
    expect(wrapper.text()).toContain('final output')
    expect(wrapper.get('[aria-label="工作流进度"]').text()).toContain('2 / 2')
    expect(wrapper.text()).not.toContain('重试原请求')
    expect(wrapper.text()).toContain('下载结果')
    wrapper.unmount()
  })

  it('runs a plugin without requiring a model key and never reads a fabricated model run', async () => {
    mocks.get.mockImplementation(async (url: string) => {
      if (url.endsWith('/execution')) return { data: { plan_digest: 'plugin-plan', package_digest: 'package-sha', permissions: ['plugin.wasm'], steps: [{ action: 'plugin.wasm', module_file: 'tool.wasm' }] } }
      return { data: { capabilities: [] } }
    })
    mocks.post.mockResolvedValue({ data: { workflow: { request_id: 'plugin-id', state: 'succeeded', total: 1, steps: [{ index: 0, action: 'plugin.wasm', state: 'succeeded' }], output: 'plugin output' } } })
    const wrapper = mount(AssetExecutionPanel, { props: { assetId: 41, version: 'v1' } })
    await flushPromises()
    expect(wrapper.find('select').exists()).toBe(false)
    await wrapper.get('textarea').setValue('input')
    await wrapper.get('input[type=checkbox]').setValue(true)
    await wrapper.findAll('button').find(b => b.text() === '运行')!.trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('plugin output')
    expect(mocks.get.mock.calls.some(call => call[0].includes('/runs/'))).toBe(false)
    wrapper.unmount()
  })

  it('keeps a partially completed workflow pending on expired model authorization', async () => {
    const original = mocks.get.getMockImplementation()!
    mocks.get.mockImplementation(async (url: string) => {
      const result = await original(url)
      if (url.endsWith('/execution')) result.data.steps = [{ action: 'model.generate' }, { action: 'model.generate' }]
      return result
    })
    mocks.post.mockResolvedValueOnce({ data: { workflow: { request_id: 'workflow-id', state: 'running', total: 2, steps: [{ index: 0, action: 'model.generate', state: 'succeeded' }], output: 'first' } } })
      .mockRejectedValue({ reason: 'WORKBENCH_CATALOG_UNAVAILABLE', message: 'expired' })
    const wrapper = mount(AssetExecutionPanel, { props: { assetId: 41, version: 'v1' } })
    await flushPromises()
    await wrapper.get('textarea').setValue('input')
    await wrapper.get('input[type=checkbox]').setValue(true)
    await wrapper.findAll('button').find(b => b.text() === '运行')!.trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('重试原请求')
    expect(wrapper.text()).toContain('刷新资源授权并继续')
    expect(wrapper.text()).toContain('first')
    expect(JSON.parse(sessionStorage.getItem('asset-run:7:41:v1')!).request_id).toBe(mocks.post.mock.calls[0][1].request_id)
    wrapper.unmount()
  })

  it('renews only the same native resource without changing the workflow request ID', async () => {
    let renewal = false
    mocks.get.mockImplementation(async (url: string) => {
      if (url.endsWith('/execution')) return { data: { plan_digest: 'plan-sha', package_digest: 'package-sha', permissions: ['model.generate'], steps: [{ action: 'model.generate' }, { action: 'model.generate' }] } }
      return { data: { capabilities: [{ id: 'native-key', version: 'native-metered-v1', digest: 'cap-sha', title: 'original key', protocol: 'chat', canonical_model_id: 'model', canonical_model_version: 'native-metered-v1', accepted_quote_id: renewal ? 'new-token' : 'old-token', accepted_quote_sha: 'signature' }] } }
    })
    mocks.post.mockRejectedValueOnce({ reason: 'WORKBENCH_CATALOG_UNAVAILABLE', message: 'expired' })
      .mockResolvedValueOnce({ data: { workflow: { request_id: 'done', state: 'succeeded', total: 2, steps: [], output: 'result' } } })
    const wrapper = mount(AssetExecutionPanel, { props: { assetId: 41, version: 'v1' } })
    await flushPromises()
    await wrapper.get('textarea').setValue('input')
    await wrapper.get('input[type=checkbox]').setValue(true)
    await wrapper.findAll('button').find(b => b.text() === '运行')!.trigger('click')
    await flushPromises()
    const first = { ...mocks.post.mock.calls[0][1] }
    renewal = true
    await wrapper.get('input[type=checkbox]').setValue(true)
    await wrapper.findAll('button').find(b => b.text() === '刷新资源授权并继续')!.trigger('click')
    await flushPromises()
    expect(mocks.post.mock.calls[1][1]).toEqual({ ...first, accepted_quote_id: 'new-token' })
    wrapper.unmount()
  })

  it('reads a completed workflow after reload without running any step', async () => {
    sessionStorage.setItem('asset-run:7:41:v1:completed', 'completed-request')
    mocks.get.mockImplementation(async (url: string) => {
      if (url.includes('?request_id=')) return { data: { request_id: 'completed-request', state: 'succeeded', total: 1, steps: [], output: 'persisted result' } }
      if (url.endsWith('/execution')) return { data: { plan_digest: 'plan', package_digest: 'package', permissions: ['plugin.wasm'], steps: [{ action: 'plugin.wasm' }] } }
      return { data: { capabilities: [] } }
    })
    const wrapper = mount(AssetExecutionPanel, { props: { assetId: 41, version: 'v1' } })
    await flushPromises()
    expect(wrapper.text()).toContain('persisted result')
    expect(mocks.post).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('lists server-side history without local storage and opens a previous result read-only', async () => {
    mocks.get.mockImplementation(async (url: string) => {
      const state = { request_id: 'server-history-id', state: 'succeeded', total: 1, steps: [{ index: 0, action: 'plugin.wasm', state: 'succeeded' }], output: 'historical output' }
      if (url.includes('?history=1')) return { data: [{ ...state, output: undefined }] }
      if (url.includes('?request_id=')) return { data: state }
      if (url.endsWith('/execution')) return { data: { plan_digest: 'plan', package_digest: 'package', permissions: ['plugin.wasm'], steps: [{ action: 'plugin.wasm' }] } }
      return { data: { capabilities: [] } }
    })
    const wrapper = mount(AssetExecutionPanel, { props: { assetId: 41, version: 'v1' } })
    await flushPromises()
    await wrapper.get('[aria-label="工作流历史"] button').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('historical output')
    expect(mocks.post).not.toHaveBeenCalled()
    wrapper.unmount()
  })
})
