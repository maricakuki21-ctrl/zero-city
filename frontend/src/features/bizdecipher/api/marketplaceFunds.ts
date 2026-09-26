import { apiClient } from '@/api/client'

export interface OrderFunds {
  order_id: number
  amount: string
  currency: 'USD'
  status: 'unpaid' | 'held' | 'released' | 'refunded'
}
export interface FundsPolicy { enabled: boolean; currency: 'USD' }
export const marketplaceFundsAPI = {
  async get(id: number) { return (await apiClient.get<OrderFunds | null>(`/biz/market/orders/${id}/funds`)).data },
  async price(id: number, amount: string) { return (await apiClient.put<OrderFunds>(`/biz/market/orders/${id}/funds`, { amount, currency: 'USD' })).data },
  async pay(id: number, amount: string, operation_id: string) { return (await apiClient.post<OrderFunds>(`/biz/market/orders/${id}/pay`, { expected_amount: amount, currency: 'USD', operation_id })).data },
  async policy() { return (await apiClient.get<FundsPolicy>('/biz/market/funds/policy')).data },
  async setPolicy(enabled: boolean, reason: string) { return (await apiClient.put<FundsPolicy>('/admin/biz/market/funds/policy', { enabled, reason })).data },
}
