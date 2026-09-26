import { mount, flushPromises } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import WithdrawalPanel from '../WithdrawalPanel.vue'
const api = vi.hoisted(() => ({ getWithdrawalPolicy: vi.fn(), listWithdrawals: vi.fn(), createWithdrawal: vi.fn(), cancelWithdrawal: vi.fn() }))
vi.mock('../../../api/withdrawals', () => ({ ...api, withdrawalStatus: { pending: '待审核', processing: '人工打款处理中' } }))
describe('WithdrawalPanel', () => {
  afterEach(() => vi.unstubAllGlobals())
  beforeEach(() => {
    vi.resetAllMocks()
    sessionStorage.clear()
    api.getWithdrawalPolicy.mockResolvedValue({ enabled: true, channels: ['bank'], instructions: 'manual only' })
    api.listWithdrawals.mockResolvedValue([])
  })
  it('uses string amounts and replays the original token after an unknown result', async () => {
    api.createWithdrawal.mockRejectedValueOnce(new Error('network')).mockResolvedValueOnce({})
    const w = mount(WithdrawalPanel, { props: { ownerId: 7 } }); await flushPromises()
    await w.get('input[inputmode="decimal"]').setValue('0.000000000001')
    await w.get('input[autocomplete="off"]').setValue('recipient')
    await w.get('form').trigger('submit'); await flushPromises()
    const first = api.createWithdrawal.mock.calls[0]![0]
    expect(first.amount).toBe('0.000000000001')
    expect(w.get('input[inputmode="decimal"]').attributes('disabled')).toBeDefined()
    await w.get('form').trigger('submit'); await flushPromises()
    expect(api.createWithdrawal.mock.calls[1]![0]).toEqual(first)
    expect(w.emitted('changed')).toHaveLength(1)
  })
  it('fails closed and shows processing explicitly without cancellation', async () => {
    api.getWithdrawalPolicy.mockResolvedValue({ enabled: false, channels: [], instructions: '' })
    api.listWithdrawals.mockResolvedValue([{ id: 1, status: 'processing', amount: '1.000000000000', channel: 'bank', recipient: 'owner' }])
    const w = mount(WithdrawalPanel, { props: { ownerId: 7 } }); await flushPromises()
    expect(w.find('form').exists()).toBe(false)
    expect(w.text()).toContain('提现尚未开放')
    expect(w.text()).toContain('人工打款处理中')
    expect(w.text()).not.toContain('撤销并退回收益')
  })
  it('reconciles an uncertain submission after remount without another debit', async () => {
    sessionStorage.setItem('withdrawal-token:7', 'existing-token')
    api.listWithdrawals.mockResolvedValue([{ id: 1, operation_id: 'existing-token', status: 'pending', amount: '1', channel: 'bank', recipient: 'owner' }])
    const w = mount(WithdrawalPanel, { props: { ownerId: 7 } }); await flushPromises()
    expect(sessionStorage.getItem('withdrawal-token:7')).toBeNull()
    expect(api.createWithdrawal).not.toHaveBeenCalled()
    expect(w.emitted('changed')).toHaveLength(1)
  })
  it('uses secure random bytes on HTTP and retains the same token after a conflict', async () => {
    vi.stubGlobal('crypto', { getRandomValues: (bytes: Uint8Array) => bytes.fill(42) })
    api.createWithdrawal.mockRejectedValueOnce({ response: { status: 409 } }).mockResolvedValueOnce({})
    const w = mount(WithdrawalPanel, { props: { ownerId: 7 } }); await flushPromises()
    await w.get('input[inputmode="decimal"]').setValue('1')
    await w.get('input[autocomplete="off"]').setValue('recipient')
    await w.get('form').trigger('submit'); await flushPromises()
    expect(w.get('input[inputmode="decimal"]').attributes('disabled')).toBeUndefined()
    const first = api.createWithdrawal.mock.calls[0]![0].operation_id
    expect(first).toMatch(/^[0-9a-f]{32}$/)
    await w.get('input[inputmode="decimal"]').setValue('2')
    await w.get('form').trigger('submit'); await flushPromises()
    expect(api.createWithdrawal.mock.calls[1]![0].operation_id).toBe(first)
  })
  it('does not continue an old account load after unmount', async () => {
    let resolve!: (v: unknown) => void
    api.getWithdrawalPolicy.mockReturnValue(new Promise(r => { resolve = r }))
    const w = mount(WithdrawalPanel, { props: { ownerId: 7 } })
    w.unmount()
    resolve({ enabled: true, channels: ['bank'], instructions: '' }); await flushPromises()
    expect(api.listWithdrawals).not.toHaveBeenCalled()
  })
})
