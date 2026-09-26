import { mount, flushPromises } from '@vue/test-utils'
import { describe, expect, it, vi, beforeEach } from 'vitest'
import Panel from '@/features/bizdecipher/views/admin/SharedPoolOperationsPanel.vue'
import Inspection from '@/features/bizdecipher/views/admin/SharedPoolEndpointInspection.vue'
import type { SharedPool } from '@/features/bizdecipher/api/bizdecipher'

const api = vi.hoisted(() => ({ adminInspectSharedPoolPricing: vi.fn() }))
vi.mock('@/features/bizdecipher/api/bizdecipher', () => api)
const pools = [
  { id: 1, name: '报价池', owner_label: '甲', models: ['a'], status: 'healthy', last_probe_at: '2026-09-14T00:00:00Z', last_probe_success: true },
  { id: 2, name: '投诉池', owner_label: '乙', models: ['b'], complaint_count: 1 },
  { id: 3, name: '历史池', owner_label: '丙', models: [], lifecycle_state: 'archived' },
] as SharedPool[]

describe('operations list interaction', () => {
  it('each queue really filters its own records, and search combines with that filter', async () => {
    const view = mount(Panel, { props: { pools, selectedId: null, loading: false, error: '' } })
    const queue = (label: string) => view.findAll('nav button').find(button => button.text().includes(label))!
    await queue('报价待核').trigger('click')
    expect(view.findAll('tbody tr')).toHaveLength(1)
    expect(view.find('tbody').text()).toContain('报价池')
    await queue('投诉与观察').trigger('click')
    expect(view.find('tbody').text()).toContain('投诉池')
    expect(view.find('tbody').text()).not.toContain('报价池')
    await view.find('input').setValue('甲')
    expect(view.find('table').exists()).toBe(false)
    await queue('全部资源').trigger('click')
    expect(view.find('tbody').text()).toContain('报价池')
    await view.find('.ops-pool-name').trigger('click')
    expect(view.emitted('select')?.[0]).toEqual([pools[0]])
  })

  it('reports load errors without presenting stale counts as current or returning an empty success', async () => {
    const view = mount(Panel, { props: { pools, selectedId: null, loading: false, error: '读取失败' } })
    expect(view.find('[role="alert"]').text()).toContain('读取失败')
    expect(view.find('table').exists()).toBe(false)
    expect(view.find('nav small').text()).toBe('—')
    await view.find('[role="alert"] button').trigger('click')
    expect(view.emitted('refresh')).toHaveLength(1)
  })
})

describe('endpoint inspection read state', () => {
  beforeEach(() => vi.clearAllMocks())
  it('ignores a late response for the previously selected pool', async () => {
    let resolveFirst!: (value: unknown) => void
    api.adminInspectSharedPoolPricing.mockImplementationOnce(() => new Promise(resolve => { resolveFirst = resolve }))
      .mockResolvedValueOnce([{ pool_model_id: 2, endpoint_id: 2, model_name: 'second-model', endpoint_type: 'chat', enabled: true, gate_status: 'unverified' }])
    const view = mount(Inspection, { props: { poolId: 1 } })
    await view.setProps({ poolId: 2 })
    await flushPromises()
    resolveFirst([{ pool_model_id: 1, endpoint_id: 1, model_name: 'first-model' }])
    await flushPromises()
    expect(view.text()).toContain('second-model')
    expect(view.text()).not.toContain('first-model')
    expect(view.text()).toContain('未验证')
    expect(view.text()).not.toContain('可调用')
  })
  it('allows retry after failure and represents inaccessible records explicitly', async () => {
    api.adminInspectSharedPoolPricing.mockRejectedValueOnce(new Error('offline')).mockResolvedValueOnce([])
    const view = mount(Inspection, { props: { poolId: 1 } })
    await flushPromises()
    expect(view.find('[role="alert"]').exists()).toBe(true)
    await view.find('button').trigger('click')
    await flushPromises()
    expect(view.text()).toContain('该池尚无端点记录')
  })
})
