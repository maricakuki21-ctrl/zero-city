import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import HarnessAssetDraftDialog from '../HarnessAssetDraftDialog.vue'

const assetApi = vi.hoisted(() => ({ create: vi.fn(), listMine: vi.fn() }))
vi.mock('@/features/bizdecipher/api/bizdecipher', () => ({
  createCapabilityAsset: assetApi.create,
  listMyCapabilityAssets: assetApi.listMine,
}))

beforeEach(() => {
  vi.resetAllMocks()
  HTMLDialogElement.prototype.showModal = vi.fn()
  HTMLDialogElement.prototype.close = vi.fn()
  assetApi.create.mockResolvedValue({ id: 73 })
  assetApi.listMine.mockResolvedValue([{ id: 73 }])
})

describe('Harness asset draft dialog', () => {
  it('submits edited metadata once and confirms the returned asset id by reading my assets', async () => {
    let finishCreate: ((value: { id: number }) => void) | undefined
    assetApi.create.mockImplementation(() => new Promise(resolve => { finishCreate = resolve }))
    const wrapper = mount(HarnessAssetDraftDialog, {
      props: {
        result: { title: '原始标题', output: '完成结果', imageUrls: [] },
      },
    })
    wrapper.vm.showModal()
    await wrapper.get('[aria-label="资产标题"]').setValue('编辑后的标题')
    await wrapper.get('[aria-label="一句话简介"]').setValue('编辑后的简介')
    await wrapper.get('[aria-label="资产类型"]').setValue('workflow')
    const submit = wrapper.get('[aria-label="保存资产草稿"]')
    await wrapper.get('form').trigger('submit')

    expect(submit.attributes('disabled')).toBeDefined()
    await wrapper.get('form').trigger('submit')
    expect(assetApi.create).toHaveBeenCalledTimes(1)

    finishCreate?.({ id: 73 })
    await flushPromises()
    expect(assetApi.listMine).toHaveBeenCalledWith({ status: 'draft', limit: 50 })
    expect(wrapper.emitted('saved')?.[0]).toEqual([73])
    expect(assetApi.create.mock.calls[0]?.[0]).toMatchObject({
      title: '编辑后的标题',
      summary: '编辑后的简介',
      asset_type: 'workflow',
      description: '完成结果',
      status: 'draft',
    })
    wrapper.unmount()
  })

  it('keeps the created id and retries only the readback when owner confirmation fails', async () => {
    assetApi.listMine.mockRejectedValueOnce(new Error('temporary read failure')).mockResolvedValueOnce([{ id: 73 }])
    const wrapper = mount(HarnessAssetDraftDialog, {
      props: { result: { title: '标题', output: '完成结果', imageUrls: [] } },
    })
    wrapper.vm.showModal()
    await wrapper.get('form').trigger('submit')
    await flushPromises()

    expect(wrapper.emitted('saved')?.[0]).toEqual([73])
    expect(wrapper.get('[role="alert"]').text()).toContain('已创建')
    expect(wrapper.get('[aria-label="保存资产草稿"]').text()).toBe('重新确认')

    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(assetApi.create).toHaveBeenCalledTimes(1)
    expect(assetApi.listMine).toHaveBeenCalledTimes(2)
    expect(wrapper.emitted('saved')).toHaveLength(1)
    wrapper.unmount()
  })

  it('ignores a delayed save after the account context is reset', async () => {
    let finishCreate: ((value: { id: number }) => void) | undefined
    assetApi.create.mockImplementation(() => new Promise(resolve => { finishCreate = resolve }))
    const wrapper = mount(HarnessAssetDraftDialog, {
      props: { result: { title: '旧账号结果', output: '不应挂到新账号', imageUrls: [] } },
    })
    wrapper.vm.showModal()
    await wrapper.get('form').trigger('submit')
    wrapper.vm.resetForAccountSwitch()
    finishCreate?.({ id: 91 })
    await flushPromises()

    expect(wrapper.emitted('saved')).toBeUndefined()
    expect(assetApi.listMine).not.toHaveBeenCalled()
    wrapper.unmount()
  })
})
