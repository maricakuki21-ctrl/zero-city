import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import AssetCommercePanel from '../AssetCommercePanel.vue'
vi.mock('../AssetExecutionPanel.vue', () => ({ default: { template: '<section />' } }))

const mocks = vi.hoisted(() => ({
  auth: { user: { id: 7 }, isAdmin: false },
  api: { policy: vi.fn(), info: vi.fn(), history: vi.fn(), purchase: vi.fn(), pricing: vi.fn(), refund: vi.fn(), reuse: vi.fn(), setPolicy: vi.fn() },
  versions: vi.fn(), download: vi.fn(), saveAs: vi.fn(),
}))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => mocks.auth }))
vi.mock('@/features/bizdecipher/api/assetCommerce', () => ({ assetCommerce: mocks.api }))
vi.mock('@/features/bizdecipher/api/bizdecipher', () => ({
  listCapabilityAssetVersions: mocks.versions, downloadCapabilityAssetPackage: mocks.download,
}))
vi.mock('file-saver', () => ({ saveAs: mocks.saveAs }))

describe('AssetCommercePanel', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    sessionStorage.clear()
    mocks.auth.user = { id: 7 }
    mocks.auth.isAdmin = false
    mocks.api.policy.mockResolvedValue({ enabled: true, currency: 'USD', platform_fee: '0.00000000' })
    mocks.api.info.mockResolvedValue({
      asset_id: 41, owner_user_id: 9, pricing_type: 'paid', price: '10.12345678',
      can_download: false, purchase_id: 0, has_package: true, status: 'listed',
    })
    mocks.api.history.mockResolvedValue({ items: [], next_cursor: 0 })
    mocks.versions.mockResolvedValue([{ id: 1, version: 'v1', status: 'published' }])
  })
  function render(assetId: number | undefined = 41) {
    return mount(AssetCommercePanel, { props: { assetId }, global: { stubs: { Icon: true } } })
  }
  it('requires explicit confirmation and reuses the operation ID after an uncertain failure', async () => {
    mocks.api.purchase.mockRejectedValue(new Error('network'))
    const wrapper = render()
    await flushPromises()
    const buy = wrapper.findAll('button').find(b => b.text() === '购买资产')!
    expect(buy.attributes('disabled')).toBeDefined()
    await wrapper.get('input[type=checkbox]').setValue(true)
    await buy.trigger('click')
    await flushPromises()
    const first = mocks.api.purchase.mock.calls[0]
    expect(first[0]).toBe(41)
    expect(first[2]).toBe('10.12345678')
    await buy.trigger('click')
    await flushPromises()
    expect(mocks.api.purchase.mock.calls[1][1]).toBe(first[1])
    expect(mocks.download).not.toHaveBeenCalled()
  })
  it('hides delivery without entitlement and disables closed purchases', async () => {
    mocks.api.policy.mockResolvedValue({ enabled: false })
    const wrapper = render()
    await flushPromises()
    expect(wrapper.text()).not.toContain('下载 ZIP')
    expect(wrapper.findAll('button').find(b => b.text() === '购买资产')!.attributes('disabled')).toBeDefined()
  })
  it('delivers ZIP and actual reuse payload for an entitled buyer', async () => {
    mocks.api.info.mockResolvedValue({
      asset_id: 41, owner_user_id: 9, pricing_type: 'paid', price: '1.00000000',
      can_download: true, purchase_id: 3, has_package: true, status: 'listed',
    })
    mocks.download.mockResolvedValue(new Blob(['zip']))
    mocks.api.reuse.mockResolvedValue({ version: { version: 'v1' }, files: [{ content_base64: 'ZmlsZQ==' }] })
    const wrapper = render()
    await flushPromises()
    await wrapper.findAll('button').find(b => b.text() === '下载 ZIP')!.trigger('click')
    await flushPromises()
    expect(mocks.download).toHaveBeenCalledWith(41, 'v1')
    await wrapper.findAll('button').find(b => b.text() === '导出复用包')!.trigger('click')
    await flushPromises()
    expect(mocks.api.reuse).toHaveBeenCalledWith(41, 'v1', expect.any(String))
    expect(mocks.saveAs).toHaveBeenCalledTimes(2)
  })
  it('requires an administrator reason before changing the default policy', async () => {
    mocks.auth.isAdmin = true
    const wrapper = render(undefined)
    await wrapper.setProps({ assetId: undefined })
    await flushPromises()
    const button = wrapper.findAll('button').find(b => b.text() === '暂停新购买')!
    expect(button.attributes('disabled')).toBeDefined()
  })
})
