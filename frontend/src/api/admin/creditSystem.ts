import { apiClient } from '../client'
import type { PaginatedResponse } from '@/types'

export interface CreditLedgerEntry {
  id: number
  user_id: number
  source_type: string
  source_id: string
  asset_type?: string
  amount: number
  balance_after: number
  status: string
  note: string
  created_by?: number | null
  created_at: string
  posted_at?: string | null
}

export interface CreditSourceDistribution {
  source_type: string
  count: number
  amount: number
}

export interface CreditSystemRule {
  key: string
  title: string
  asset_type: string
  amount?: number
  rate_percent?: number
  description: string
}

export interface CreditSystemOverview {
  total_credit_balance: number
  positive_credit_users: number
  ledger_entry_count: number
  today_granted_credits: number
  today_consumed_credits: number
  source_type_distribution: CreditSourceDistribution[]
  rules: CreditSystemRule[]
}

export interface CreditUserSummary {
  user_id: number
  email: string
  username: string
  credit_balance: number
  ledger: CreditLedgerEntry[]
}

export interface ListCreditLedgerParams {
  page?: number
  page_size?: number
  user_id?: number | null
  search?: string
  source_type?: string
  status?: string
  start_at?: string
  end_at?: string
  timezone?: string
}

export interface CreditGrantPayload {
  user_id: number
  source_type?: string
  source_id?: string
  amount: number
  note?: string
}

export async function getCreditSystemOverview(): Promise<CreditSystemOverview> {
  const { data } = await apiClient.get<CreditSystemOverview>('/admin/credit-system/overview')
  return data
}

export async function listCreditLedger(
  params: ListCreditLedgerParams = {},
): Promise<PaginatedResponse<CreditLedgerEntry>> {
  const { data } = await apiClient.get<PaginatedResponse<CreditLedgerEntry>>(
    '/admin/credit-system/ledger',
    {
      params: {
        page: params.page ?? 1,
        page_size: params.page_size ?? 20,
        user_id: params.user_id || undefined,
        search: params.search || undefined,
        source_type: params.source_type || undefined,
        status: params.status || undefined,
        start_at: params.start_at || undefined,
        end_at: params.end_at || undefined,
        timezone: params.timezone || undefined,
      },
    },
  )
  return data
}

export async function getCreditUserSummary(userId: number): Promise<CreditUserSummary> {
  const { data } = await apiClient.get<CreditUserSummary>(`/admin/credit-system/users/${userId}`)
  return data
}

export async function grantCredit(payload: CreditGrantPayload): Promise<CreditLedgerEntry> {
  const { data } = await apiClient.post<CreditLedgerEntry>('/admin/credit-system/grant', payload)
  return data
}

export const creditSystemAPI = {
  getOverview: getCreditSystemOverview,
  listLedger: listCreditLedger,
  getUserSummary: getCreditUserSummary,
  grantCredit,
}
