import { apiClient } from '@/api/client'

export interface AssetCommercePolicy { enabled: boolean; currency: string; platform_fee: string }
export interface AssetCommerceInfo {
  asset_id: number; owner_user_id: number; pricing_type: string; price: string
  can_download: boolean; purchase_id: number; has_package: boolean; status: string
}
export interface AssetPurchase {
  id: number; asset_id: number; buyer_user_id: number; owner_user_id: number
  amount: string; creator_amount: string; platform_fee: string; status: string
  created_at: string; refund_reason: string
}
export const assetCommerce = {
  policy: async () => (await apiClient.get<AssetCommercePolicy>('/biz/assets/commerce-policy')).data,
  setPolicy: async (enabled: boolean, reason: string) =>
    (await apiClient.put<AssetCommercePolicy>('/admin/biz/assets/commerce-policy', { enabled, reason })).data,
  info: async (id: number) => (await apiClient.get<AssetCommerceInfo>(`/biz/assets/${id}/commerce`)).data,
  pricing: async (id: number, mode: string, price: string) =>
    (await apiClient.patch<AssetCommerceInfo>(`/biz/assets/${id}/pricing`, { mode, price })).data,
  purchase: async (id: number, operation_id: string, expected_price: string) =>
    (await apiClient.post<AssetPurchase>(`/biz/assets/${id}/purchases`, { operation_id, expected_price })).data,
  history: async (id: number, cursor = 0) =>
    (await apiClient.get<{items: AssetPurchase[]; next_cursor: number}>(`/biz/assets/${id}/purchases`, { params: { cursor, limit: 20 } })).data,
  refund: async (id: number, purchaseID: number, operation_id: string, reason: string) =>
    (await apiClient.post<AssetPurchase>(`/admin/biz/assets/${id}/purchases/${purchaseID}/refund`, { operation_id, reason })).data,
  reuse: async (id: number, version: string, operation_id: string) =>
    (await apiClient.post(`/biz/assets/${id}/versions/${encodeURIComponent(version)}/reuse`, { operation_id })).data,
}
