import { beforeEach, describe, expect, it, vi } from 'vitest'
import { marketplaceAPI, type MarketplaceListingInput } from '../marketplace'

const client = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn(), patch: vi.fn() }))
vi.mock('@/api/client', () => ({ apiClient: client }))

const listingInput: MarketplaceListingInput = {
  kind: 'service',
  title: '自动化验收',
  summary: '交付可运行脚本和验收记录',
  category: '代码与自动化',
  price_text: '按项目沟通',
  delivery_text: '5 个工作日',
  tags: ['自动化', '验收'],
}

describe('marketplace API', () => {
  it('uses protected admin dispute endpoints without a client-supplied actor', async () => {
    client.get.mockResolvedValue({ data: { items: [] } })
    client.post.mockResolvedValue({ data: { id: 51, status: 'canceled' } })
    await marketplaceAPI.listDisputes(80)
    await marketplaceAPI.getDispute(51)
    await marketplaceAPI.resolveDispute(51, { outcome: 'canceled', reason: 'Cannot fulfill' })
    expect(client.get).toHaveBeenNthCalledWith(1, '/admin/biz/market/disputes', { params: { cursor: 80, limit: 20 } })
    expect(client.get).toHaveBeenNthCalledWith(2, '/admin/biz/market/disputes/51')
    expect(client.post).toHaveBeenCalledWith('/admin/biz/market/disputes/51/resolve', { outcome: 'canceled', reason: 'Cannot fulfill' })
  })
  beforeEach(() => {
    client.get.mockReset()
    client.post.mockReset()
    client.patch.mockReset()
  })

  it('passes public listing filters and cursor to the real endpoint', async () => {
    const page = { items: [], next_cursor: 81 }
    client.get.mockResolvedValue({ data: page })

    await expect(marketplaceAPI.listListings({ kind: 'demand', category: '研究', tag: '报告', cursor: 99, limit: 20 })).resolves.toEqual(page)
    expect(client.get).toHaveBeenCalledWith('/biz/market/listings', {
      params: { kind: 'demand', category: '研究', tag: '报告', cursor: 99, limit: 20 },
    })
  })

  it('publishes, edits, and archives through owner endpoints', async () => {
    client.post.mockResolvedValue({ data: { id: 12 } })
    client.patch.mockResolvedValue({ data: { id: 12 } })

    await marketplaceAPI.createListing(listingInput)
    await marketplaceAPI.updateListing(12, listingInput)
    await marketplaceAPI.archiveListing(12)

    expect(client.post).toHaveBeenNthCalledWith(1, '/biz/market/listings', listingInput)
    expect(client.patch).toHaveBeenCalledWith('/biz/market/listings/12', listingInput)
    expect(client.post).toHaveBeenNthCalledWith(2, '/biz/market/listings/12/archive')
  })

  it('uses server inquiry threads and stable client message ids', async () => {
    client.post.mockResolvedValue({ data: { id: 31 } })
    client.get.mockResolvedValue({ data: { items: [], next_cursor: null } })

    await marketplaceAPI.createInquiry(12)
    await marketplaceAPI.listInquiries(40)
    await marketplaceAPI.listMessages(31, 22)
    await marketplaceAPI.sendMessage(31, { client_message_id: 'market-msg-1', body: '请确认交付范围' })

    expect(client.post).toHaveBeenNthCalledWith(1, '/biz/market/listings/12/inquiries')
    expect(client.get).toHaveBeenNthCalledWith(1, '/biz/market/inquiries', { params: { cursor: 40, limit: 20 } })
    expect(client.get).toHaveBeenNthCalledWith(2, '/biz/market/inquiries/31/messages', { params: { cursor: 22, limit: 20 } })
    expect(client.post).toHaveBeenNthCalledWith(2, '/biz/market/inquiries/31/messages', {
      client_message_id: 'market-msg-1',
      body: '请确认交付范围',
    })
  })

  it('quotes, lists, loads, and acts on marketplace orders', async () => {
    client.post.mockResolvedValue({ data: { id: 51 } })
    client.get.mockResolvedValue({ data: { items: [], next_cursor: null } })

    await marketplaceAPI.quoteOrder(31, {
      scope_text: '交付范围',
      amount_text: '100 积分',
      delivery_text: '2 天',
      revision_limit: 1,
    })
    await marketplaceAPI.listOrders(80)
    await marketplaceAPI.getOrder(51)
    await marketplaceAPI.applyOrderAction(51, { action: 'deliver', note: '交付包已上传' })

    expect(client.post).toHaveBeenNthCalledWith(1, '/biz/market/inquiries/31/orders', {
      scope_text: '交付范围',
      amount_text: '100 积分',
      delivery_text: '2 天',
      revision_limit: 1,
    })
    expect(client.get).toHaveBeenNthCalledWith(1, '/biz/market/orders', { params: { cursor: 80, limit: 20 } })
    expect(client.get).toHaveBeenNthCalledWith(2, '/biz/market/orders/51')
    expect(client.post).toHaveBeenNthCalledWith(2, '/biz/market/orders/51/actions', { action: 'deliver', note: '交付包已上传' })
  })
})
