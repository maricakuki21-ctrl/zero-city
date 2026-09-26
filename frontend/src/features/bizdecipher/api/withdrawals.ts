import { apiClient } from '@/api/client'

export type WithdrawalPolicy = { enabled: boolean; channels: string[]; instructions: string }
export type Withdrawal = {
  id: number; owner_id: number; operation_id: string; amount: string; channel: string
  recipient: string; status: 'pending' | 'processing' | 'paid' | 'rejected' | 'cancelled'
  reference: string; reason: string; created_at: string; processing_by: number
}
export type WithdrawalInput = { operation_id: string; amount: string; channel: string; recipient: string }
const base = (admin: boolean) => admin ? '/admin/biz/withdrawals' : '/biz/withdrawals'
export async function getWithdrawalPolicy(admin = false): Promise<WithdrawalPolicy> {
  return (await apiClient.get<WithdrawalPolicy>(`${base(admin)}/policy`)).data
}
export async function saveWithdrawalPolicy(policy: WithdrawalPolicy): Promise<void> {
  await apiClient.put(`${base(true)}/policy`, policy)
}
export async function listWithdrawals(admin = false, before = 0): Promise<Withdrawal[]> {
  return (await apiClient.get<Withdrawal[]>(base(admin), { params: { before } })).data
}
export async function createWithdrawal(input: WithdrawalInput): Promise<Withdrawal> {
  return (await apiClient.post<Withdrawal>(base(false), input)).data
}
export async function cancelWithdrawal(id: number): Promise<void> {
  await apiClient.post(`${base(false)}/${id}/cancel`)
}
export async function actWithdrawal(id: number, action: string, reason: string, reference: string, noPaymentConfirmed = false): Promise<void> {
  await apiClient.post(`${base(true)}/${id}/actions`, { action, reason, reference, no_payment_confirmed: noPaymentConfirmed })
}
export const withdrawalStatus = {
  pending: '待审核（已从可用收益扣除）', processing: '人工打款处理中（尚未确认到账）',
  paid: '已人工核实打款', rejected: '已拒绝并退回收益', cancelled: '已撤销并退回收益',
}
