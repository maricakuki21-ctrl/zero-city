import { mount, flushPromises, enableAutoUnmount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import WithdrawalAdminPanel from '../WithdrawalAdminPanel.vue'
enableAutoUnmount(afterEach)
const api = vi.hoisted(() => ({ getWithdrawalPolicy: vi.fn(), listWithdrawals: vi.fn(), saveWithdrawalPolicy: vi.fn(), actWithdrawal: vi.fn() }))
const auth = vi.hoisted(() => ({ store: null as null | { user: { id: number; role: string } } }))
vi.mock('@/stores/auth', async () => {
  const { reactive } = await import('vue')
  auth.store = reactive({ user: { id: 8, role: 'admin' } })
  return { useAuthStore: () => auth.store }
})
vi.mock('../../../api/withdrawals', () => ({ ...api, withdrawalStatus: { processing: '人工打款处理中' } }))
describe('WithdrawalAdminPanel', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    auth.store!.user = { id: 8, role: 'admin' }
    api.getWithdrawalPolicy.mockResolvedValue({ enabled: false, channels: ['bank'], instructions: '' })
    api.listWithdrawals.mockResolvedValue([{ id: 7, owner_id: 1, processing_by: 8, status: 'processing', amount: '1', channel: 'bank', recipient: 'owner' }])
  })
  it('requires explicit external verification for paid and warns no payment is sent', async () => {
    const w = mount(WithdrawalAdminPanel); await flushPromises()
    await w.findAll('button').find(b => b.text() === '核实外部打款')!.trigger('click')
    expect(w.text()).toContain('此操作不会发送款项')
    const form = w.findAll('form')[1]!
    expect(form.get('button').attributes('disabled')).toBeDefined()
    await form.get('input[maxlength="200"]').setValue('external-receipt')
    await form.get('input[type="checkbox"]').setValue(true)
    await form.trigger('submit'); await flushPromises()
    expect(api.actWithdrawal).toHaveBeenCalledWith(7, 'paid', '', 'external-receipt', false)
  })
  it('discards late recipient data when the administrator account changes', async () => {
    let resolve!: (v: unknown) => void
    api.listWithdrawals.mockReturnValueOnce(new Promise(r => { resolve = r })).mockResolvedValue([])
    const w = mount(WithdrawalAdminPanel); await flushPromises()
    auth.store!.user = { id: 9, role: 'admin' }; await flushPromises()
    resolve([{ id: 1, recipient: 'OLD-PRIVATE-RECIPIENT', status: 'processing', amount: '1' }]); await flushPromises()
    expect(w.text()).not.toContain('OLD-PRIVATE-RECIPIENT')
  })
  it('does not report an unsaved switch as enabled and confirms a successful save', async () => {
    api.listWithdrawals.mockResolvedValue([])
    const w = mount(WithdrawalAdminPanel); await flushPromises()
    expect(w.text()).toContain('暂无提现申请')
    await w.get('input[type="checkbox"]').setValue(true)
    expect(w.get('.policy-state').text()).toBe('申请已关闭')
    api.saveWithdrawalPolicy.mockResolvedValue(undefined)
    api.getWithdrawalPolicy.mockResolvedValue({ enabled: true, channels: ['bank'], instructions: '' })
    await w.get('form').trigger('submit'); await flushPromises()
    expect(api.saveWithdrawalPolicy).toHaveBeenCalledWith({ enabled: true, channels: ['bank'], instructions: '' })
    expect(w.get('.policy-state').text()).toBe('申请已开放')
    expect(w.get('[role="status"]').text()).toBe('提现配置已保存。')
  })
  it('shows a load failure instead of a misleading empty application list', async () => {
    api.listWithdrawals.mockRejectedValue(new Error('network unavailable'))
    const w = mount(WithdrawalAdminPanel); await flushPromises()
    expect(w.find('[role="alert"]').exists()).toBe(true)
    expect(w.text()).not.toContain('暂无提现申请')
    expect(w.get('form button').attributes('disabled')).toBeDefined()
  })
})
