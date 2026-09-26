import { apiClient } from '@/api/client'

export interface CreatorColumn {
  id: number
  owner_user_id: number
  author_name: string
  title: string
  description: string
  status: 'active' | 'suspended'
  moderation_reason: string
  mode?: 'free' | 'paid'
  price?: string
  can_read?: boolean
  viewer_purchase_id?: number
  article_count: number
  created_at: string
  updated_at: string
}

export interface ColumnArticle {
  id: number
  column_id: number
  author_user_id: number
  title: string
  summary: string
  body?: string
  status: 'draft' | 'published' | 'archived'
  created_at: string
  updated_at: string
}

export interface ColumnPage<T> { items: T[]; next_cursor?: number | null }
export interface ColumnInput { title: string; description: string }
export interface ArticleInput { title: string; summary: string; body: string; status: ColumnArticle['status'] }
export interface ColumnCommercePolicy { enabled: boolean; currency: string; platform_fee: string }
export interface ColumnPurchase {
  id: number; column_id: number; buyer_user_id: number; owner_user_id: number
  operation_id: string; column_title: string; amount: string; platform_fee: string
  creator_amount: string; status: 'active' | 'refunded'; created_at: string
  refunded_at?: string; refund_reason: string
}
const base = '/biz/community/columns'

export const columnsAPI = {
  async policy() {
    return (await apiClient.get<ColumnCommercePolicy>(`${base}/commerce-policy`)).data
  },
  async setPolicy(enabled: boolean, reason: string) {
    return (await apiClient.put<ColumnCommercePolicy>('/admin/biz/community/columns/commerce-policy', { enabled, reason })).data
  },
  async setPricing(id: number, mode: 'free' | 'paid', price: string) {
    return (await apiClient.patch<CreatorColumn>(`${base}/${id}/pricing`, { mode, price })).data
  },
  async purchase(id: number, operation_id: string, expected_price: string) {
    return (await apiClient.post<ColumnPurchase>(`${base}/${id}/purchases`, { operation_id, expected_price })).data
  },
  async purchases(id: number, cursor?: number) {
    return (await apiClient.get<ColumnPage<ColumnPurchase>>(`${base}/${id}/purchases`, { params: { cursor, limit: 20 } })).data
  },
  async refund(id: number, purchaseID: number, operation_id: string, reason: string) {
    return (await apiClient.post<ColumnPurchase>(`/admin/biz/community/columns/${id}/purchases/${purchaseID}/refund`, { operation_id, reason })).data
  },
  async list(mine: boolean, cursor?: number) {
    return (await apiClient.get<ColumnPage<CreatorColumn>>(base, { params: { mine, cursor, limit: 20 } })).data
  },
  async listAdmin(cursor?: number) {
    return (await apiClient.get<ColumnPage<CreatorColumn>>('/admin/biz/community/columns', { params: { cursor, limit: 20 } })).data
  },
  async get(id: number) { return (await apiClient.get<CreatorColumn>(`${base}/${id}`)).data },
  async create(input: ColumnInput) { return (await apiClient.post<CreatorColumn>(base, input)).data },
  async update(id: number, input: ColumnInput) { return (await apiClient.patch<CreatorColumn>(`${base}/${id}`, input)).data },
  async moderate(id: number, status: CreatorColumn['status'], reason: string) {
    return (await apiClient.patch<CreatorColumn>(`/admin/biz/community/columns/${id}`, { status, moderation_reason: reason })).data
  },
  async articles(id: number, mine: boolean, cursor?: number) {
    return (await apiClient.get<ColumnPage<ColumnArticle>>(`${base}/${id}/articles`, { params: { mine, cursor, limit: 20 } })).data
  },
  async article(id: number, articleID: number) {
    return (await apiClient.get<ColumnArticle>(`${base}/${id}/articles/${articleID}`)).data
  },
  async createArticle(id: number, input: ArticleInput) {
    return (await apiClient.post<ColumnArticle>(`${base}/${id}/articles`, input)).data
  },
  async updateArticle(id: number, articleID: number, input: ArticleInput) {
    return (await apiClient.patch<ColumnArticle>(`${base}/${id}/articles/${articleID}`, input)).data
  },
}
