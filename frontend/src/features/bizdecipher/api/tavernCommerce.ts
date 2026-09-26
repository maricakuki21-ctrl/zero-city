import { apiClient } from '@/api/client'
export interface TavernTicket { id: number; room_id: number; amount: string; status: 'held' | 'released' | 'refunded'; refund_reason: string }
export interface TavernTicketQuote { room_id: number; owner_id: number; status: string; title: string; author_user_id?: number; price: string; currency: 'USD'; enabled: boolean; ticket?: TavernTicket }
export const tavernCommerceAPI = {
  async quote(room: number) { return (await apiClient.get<TavernTicketQuote>(`/biz/tavern/rooms/${room}/ticket`)).data },
  async buy(room: number, expected_price: string, operation_id: string) { return (await apiClient.post<TavernTicket>(`/biz/tavern/rooms/${room}/ticket`, { expected_price, currency: 'USD', operation_id })).data },
  async refund(room: number, ticket_id: number) { return (await apiClient.post<TavernTicket>(`/biz/tavern/rooms/${room}/ticket/refund`, { ticket_id })).data },
  async price(script: number, price: string) { return (await apiClient.put(`/biz/tavern/scripts/${script}/pricing`, { price, currency: 'USD' })).data },
  async policy() { return (await apiClient.get<{ enabled: boolean; currency: 'USD' }>('/biz/tavern/commerce/policy')).data },
  async setPolicy(enabled: boolean, reason: string) { return (await apiClient.put('/admin/biz/tavern/commerce/policy', { enabled, reason })).data },
}
