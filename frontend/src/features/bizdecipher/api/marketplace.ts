import { apiClient } from '@/api/client'

export type MarketplaceListingKind = 'service' | 'demand' | 'talent'
export type MarketplaceListingStatus = 'published' | 'archived' | 'taken_down'
export type MarketplaceOrderStatus = 'quoted' | 'confirmed' | 'delivered' | 'accepted' | 'settled' | 'canceled' | 'disputed'
export type MarketplaceOrderRole = 'buyer' | 'seller' | 'admin' | ''
export type MarketplaceOrderAction = 'confirm' | 'deliver' | 'accept' | 'settle' | 'cancel' | 'dispute' | 'review'

export interface MarketplaceListing {
  id: number
  kind: MarketplaceListingKind
  owner_user_id: number
  owner_display_name?: string
  canonical_asset_id?: number
  title: string
  summary: string
  category: string
  price_text: string
  delivery_text: string
  tags: string[]
  status: MarketplaceListingStatus
  created_at: string
  updated_at: string
}

export interface MarketplaceListingInput {
  kind: MarketplaceListingKind
  canonical_asset_id?: number
  title: string
  summary: string
  category: string
  price_text: string
  delivery_text: string
  tags: string[]
}

export interface MarketplacePage<T> {
  items: T[]
  next_cursor?: number | null
}

export interface MarketplaceListingQuery {
  kind?: MarketplaceListingKind
  category?: string
  tag?: string
  cursor?: number
  limit?: number
}

export interface MarketplaceInquiry {
  id: number
  listing_id: number
  listing: MarketplaceListing
  initiator_user_id: number
  listing_owner_user_id: number
  created_at: string
  updated_at: string
  last_message_at: string
}

export interface MarketplaceMessage {
  id: number
  inquiry_id: number
  sender_user_id: number
  client_message_id: string
  body: string
  created_at: string
}

export interface MarketplaceOrderQuoteInput {
  scope_text: string
  amount_text: string
  delivery_text: string
  revision_limit: number
}

export interface MarketplaceOrderActionInput {
  action: MarketplaceOrderAction
  note?: string
  rating?: number
}

export interface MarketplaceOrderEvent {
  id: number
  order_id: number
  actor_user_id: number
  actor_name: string
  event: string
  from_status: MarketplaceOrderStatus | ''
  to_status: MarketplaceOrderStatus
  note: string
  created_at: string
}

export interface MarketplaceOrderReview {
  id: number
  order_id: number
  author_user_id: number
  author_name: string
  rating: number
  body: string
  created_at: string
}

export interface MarketplaceOrder {
  legacy_cooperation?: boolean
  id: number
  inquiry_id: number
  listing_id: number
  listing_title: string
  buyer_user_id: number
  buyer_display_name: string
  seller_user_id: number
  seller_display_name: string
  status: MarketplaceOrderStatus
  scope_text: string
  amount_text: string
  delivery_text: string
  revision_limit: number
  delivery_note: string
  accept_note: string
  dispute_note: string
  cancel_reason: string
  quoted_at: string
  confirmed_at?: string
  delivered_at?: string
  accepted_at?: string
  settled_at?: string
  canceled_at?: string
  created_at: string
  updated_at: string
  viewer_role: MarketplaceOrderRole
  available_actions: MarketplaceOrderAction[]
  events?: MarketplaceOrderEvent[]
  reviews?: MarketplaceOrderReview[]
}

export const marketplaceAPI = {
  listDisputes(cursor?: number): Promise<MarketplacePage<MarketplaceOrder>> {
    return apiClient.get<MarketplacePage<MarketplaceOrder>>('/admin/biz/market/disputes', { params: { cursor, limit: 20 } }).then(response => response.data)
  },
  getDispute(id: number): Promise<MarketplaceOrder> {
    return apiClient.get<MarketplaceOrder>(`/admin/biz/market/disputes/${id}`).then(response => response.data)
  },
  resolveDispute(id: number, payload: { outcome: 'confirmed' | 'canceled'; reason: string }): Promise<MarketplaceOrder> {
    return apiClient.post<MarketplaceOrder>(`/admin/biz/market/disputes/${id}/resolve`, payload).then(response => response.data)
  },
  listListings(query: MarketplaceListingQuery): Promise<MarketplacePage<MarketplaceListing>> {
    return apiClient
      .get<MarketplacePage<MarketplaceListing>>('/biz/market/listings', { params: query })
      .then((response) => response.data)
  },

  listMyListings(cursor?: number, limit = 20): Promise<MarketplacePage<MarketplaceListing>> {
    return apiClient
      .get<MarketplacePage<MarketplaceListing>>('/biz/market/my-listings', { params: { cursor, limit } })
      .then((response) => response.data)
  },

  getListing(id: number): Promise<MarketplaceListing> {
    return apiClient
      .get<MarketplaceListing>(`/biz/market/listings/${id}`)
      .then((response) => response.data)
  },

  createListing(payload: MarketplaceListingInput): Promise<MarketplaceListing> {
    return apiClient
      .post<MarketplaceListing>('/biz/market/listings', payload)
      .then((response) => response.data)
  },

  updateListing(id: number, payload: MarketplaceListingInput): Promise<MarketplaceListing> {
    return apiClient
      .patch<MarketplaceListing>(`/biz/market/listings/${id}`, payload)
      .then((response) => response.data)
  },

  archiveListing(id: number): Promise<MarketplaceListing> {
    return apiClient
      .post<MarketplaceListing>(`/biz/market/listings/${id}/archive`)
      .then((response) => response.data)
  },

  createInquiry(listingId: number): Promise<MarketplaceInquiry> {
    return apiClient
      .post<MarketplaceInquiry>(`/biz/market/listings/${listingId}/inquiries`)
      .then((response) => response.data)
  },

  listInquiries(cursor?: number, limit = 20): Promise<MarketplacePage<MarketplaceInquiry>> {
    return apiClient
      .get<MarketplacePage<MarketplaceInquiry>>('/biz/market/inquiries', { params: { cursor, limit } })
      .then((response) => response.data)
  },

  listMessages(inquiryId: number, cursor?: number, limit = 20): Promise<MarketplacePage<MarketplaceMessage>> {
    return apiClient
      .get<MarketplacePage<MarketplaceMessage>>(`/biz/market/inquiries/${inquiryId}/messages`, { params: { cursor, limit } })
      .then((response) => response.data)
  },

  sendMessage(inquiryId: number, payload: { client_message_id: string; body: string }): Promise<MarketplaceMessage> {
    return apiClient
      .post<MarketplaceMessage>(`/biz/market/inquiries/${inquiryId}/messages`, payload)
      .then((response) => response.data)
  },

  quoteOrder(inquiryId: number, payload: MarketplaceOrderQuoteInput): Promise<MarketplaceOrder> {
    return apiClient
      .post<MarketplaceOrder>(`/biz/market/inquiries/${inquiryId}/orders`, payload)
      .then((response) => response.data)
  },

  listOrders(cursor?: number, limit = 20): Promise<MarketplacePage<MarketplaceOrder>> {
    return apiClient
      .get<MarketplacePage<MarketplaceOrder>>('/biz/market/orders', { params: { cursor, limit } })
      .then((response) => response.data)
  },

  getOrder(id: number): Promise<MarketplaceOrder> {
    return apiClient
      .get<MarketplaceOrder>(`/biz/market/orders/${id}`)
      .then((response) => response.data)
  },

  applyOrderAction(id: number, payload: MarketplaceOrderActionInput): Promise<MarketplaceOrder> {
    return apiClient
      .post<MarketplaceOrder>(`/biz/market/orders/${id}/actions`, payload)
      .then((response) => response.data)
  },
}
