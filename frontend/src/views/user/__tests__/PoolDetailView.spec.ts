import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { SharedPool } from '@/features/bizdecipher/api/bizdecipher'

const mocks = vi.hoisted(() => ({
  route: { params: { id: '42' } },
  routerPush: vi.fn(),
  getSharedPool: vi.fn(),
  getSharedPoolFullCheckReport: vi.fn(),
  listSharedPoolModelPricing: vi.fn(),
  joinSharedPool: vi.fn(),
  leaveSharedPool: vi.fn(),
  likeSharedPool: vi.fn(),
  unlikeSharedPool: vi.fn(),
  createSharedPoolAccessKey: vi.fn(),
  listMySharedPoolAccessKeys: vi.fn(),
  listMySeats: vi.fn(),
  reportSharedPool: vi.fn(),
  listSharedPoolSummaries: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn(),
  copyToClipboard: vi.fn(),
}))

vi.mock('vue-router', () => ({
  useRoute: () => mocks.route,
  useRouter: () => ({ push: mocks.routerPush }),
  RouterLink: defineComponent({
    name: 'RouterLink',
    template: '<a><slot /></a>',
  }),
}))

vi.mock('@/features/bizdecipher/api/bizdecipher', () => ({
  getSharedPool: mocks.getSharedPool,
  getSharedPoolFullCheckReport: mocks.getSharedPoolFullCheckReport,
  listSharedPoolModelPricing: mocks.listSharedPoolModelPricing,
  joinSharedPool: mocks.joinSharedPool,
  leaveSharedPool: mocks.leaveSharedPool,
  likeSharedPool: mocks.likeSharedPool,
  unlikeSharedPool: mocks.unlikeSharedPool,
  createSharedPoolAccessKey: mocks.createSharedPoolAccessKey,
  listMySharedPoolAccessKeys: mocks.listMySharedPoolAccessKeys,
  listMySeats: mocks.listMySeats,
  reportSharedPool: mocks.reportSharedPool,
}))

vi.mock('@/features/bizdecipher/api/community', () => ({
  communityAPI: {
    listSharedPoolSummaries: mocks.listSharedPoolSummaries,
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: mocks.showError,
    showSuccess: mocks.showSuccess,
  }),
}))

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({ copyToClipboard: mocks.copyToClipboard }),
}))

vi.mock('@/composables/useSharedPool', () => ({
  emptyPoolCommunitySummary: (poolId: number) => ({
    pool_id: poolId,
    total_posts: 0,
    discussion_posts: 0,
    feedback_posts: 0,
    incident_posts: 0,
    risk_signals: 0,
    last_post_title: '',
    latest_posts: [],
  }),
}))

import PoolDetailView from '@/features/bizdecipher/views/user/PoolDetailView.vue'

const AppLayoutStub = defineComponent({
  template: '<div><slot /></div>',
})

const RouterLinkStub = defineComponent({
  template: '<a><slot /></a>',
})

function makePool(overrides: Partial<SharedPool> = {}): SharedPool {
  return {
    id: 42,
    name: 'Public account pool',
    description: '',
    owner_label: 'Owner',
    tier: 'standard',
    status: 'healthy',
    listed: true,
    models: ['gpt-4o-mini'],
    model_configs: [],
    account_summary: {
      total_accounts: 3,
      configured_accounts: 3,
      schedulable_accounts: 0,
      gate_blocked_accounts: 0,
      disabled_accounts: 0,
      total_account_concurrency: 0,
      total_user_concurrency: 0,
      total_rpm_limit: 0,
      average_full_check_score: 0,
      full_check_passed_accounts: 0,
      full_check_total_accounts: 0,
      total_calls: 0,
      successful_calls: 0,
      failed_calls: 0,
      success_rate: 0,
      model_coverage: ['gpt-4o-mini'],
    },
    rate_multiplier: 1,
    max_users: 20,
    current_users: 0,
    min_balance_admission: 0,
    hourly_seat_fee: 0,
    hourly_min_usage_waiver: 0,
    today_availability: 100,
    seven_day_availability: 100,
    avg_latency_ms: 100,
    account_mode_enabled: true,
    verification_mode: 'full_check',
    ...overrides,
  }
}

function makeAccountPool(summary: Partial<NonNullable<SharedPool['account_summary']>>): SharedPool {
  const base = makePool()
  return makePool({
    account_summary: { ...base.account_summary!, ...summary },
    last_probe_at: '2026-07-20T00:00:00Z',
    last_probe_success: true,
    last_probe_gate_passed: true,
    last_probe_full_check_passed: 10,
    last_probe_full_check_total: 10,
    last_probe_full_check_score: 100,
  })
}

function mountPoolDetail() {
  return mount(PoolDetailView, {
    global: {
      stubs: {
        AppLayout: AppLayoutStub,
        RouterLink: RouterLinkStub,
      },
    },
  })
}

describe('PoolDetailView account summary status', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mocks.route.params = { id: '42' }
    mocks.getSharedPoolFullCheckReport.mockResolvedValue(null)
    mocks.listSharedPoolModelPricing.mockResolvedValue([])
    mocks.listMySharedPoolAccessKeys.mockResolvedValue([])
    mocks.listMySeats.mockResolvedValue([])
    mocks.listSharedPoolSummaries.mockResolvedValue({ items: [] })
  })

  it.each([
    {
      name: 'ready account pool',
      summary: { full_check_passed_accounts: 1, full_check_total_accounts: 3, schedulable_accounts: 1, average_full_check_score: 82 },
      badge: '账号池可调度',
      badgeClass: 'pd-tag-verified',
      health: '账号池可调度',
      note: '1/3 个账号通过，1 个当前可调度',
    },
    {
      name: 'missing account total',
      summary: { full_check_passed_accounts: 1, full_check_total_accounts: 0, schedulable_accounts: 1, average_full_check_score: 100 },
      badge: '待账号检测',
      badgeClass: 'pd-tag-pending',
      health: '基础可调度',
      note: '等待满血检测证据',
    },
    {
      name: 'no schedulable account',
      summary: { full_check_passed_accounts: 1, full_check_total_accounts: 3, schedulable_accounts: 0, average_full_check_score: 100 },
      badge: '待账号检测',
      badgeClass: 'pd-tag-pending',
      health: '账号检测未达标',
      note: '1/3 个账号通过，0 个当前可调度',
    },
  ])('uses account_summary instead of stale pool probe fields for $name', async ({ summary, badge, badgeClass, health, note }) => {
    mocks.getSharedPool.mockResolvedValue(makeAccountPool(summary))

    const wrapper = mountPoolDetail()
    await flushPromises()

    const verificationTag = wrapper.findAll('.pd-tags .pd-tag').find((tag) => tag.text() === badge)
    expect(verificationTag, 'verification tag').toBeDefined()
    expect(verificationTag?.classes()).toContain(badgeClass)
    expect(wrapper.get('.pd-tags').text()).not.toContain('满血验证')
    expect(wrapper.get('.pd-health-head').text()).toContain(health)
    expect(wrapper.get('.pd-health-note').text()).toContain(note)
  })

  it('shows the exact small pool and model multiplier', async () => {
    mocks.getSharedPool.mockResolvedValue(makePool({ rate_multiplier: 0.0001 }))
    mocks.listSharedPoolModelPricing.mockResolvedValue([{
      pool_model_id: 1,
      provider: 'openai',
      model_name: 'gpt-5.6-sol',
      display_name: 'GPT-5.6 Sol',
      pricing_source: 'official_catalog',
      pricing_status: 'ready',
      pricing_config_version: 1,
      endpoint_id: 1,
      endpoint_type: 'chat',
      enabled: true,
      gate_status: 'passed',
      endpoint_pricing_status: 'ready',
      config_version: 1,
      current_price: {
        price_version_id: 1,
        pool_id: 42,
        pool_model_id: 1,
        endpoint_id: 1,
        model_name: 'gpt-5.6-sol',
        endpoint_type: 'chat',
        pricing_source: 'official_catalog',
        pricing_status: 'ready',
        config_version: 1,
        base_price: { billing_mode: 'token', currency: 'USD', input_price: 0.000001, output_price: 0.000006 },
        multiplier: 0.0001,
        user_price: { billing_mode: 'token', currency: 'USD', input_price: 0.0000000001, output_price: 0.0000000006 },
        effective_from: '2026-07-26T00:00:00Z',
        explicit_free: false,
        example_cost: 0.0000004,
      },
    }])

    const wrapper = mountPoolDetail()
    await flushPromises()

    expect(wrapper.get('.pd-metric-card:nth-child(3)').text()).toContain('0.0001x')
    expect(wrapper.text()).toContain('0.0001 倍')
    expect(wrapper.text()).not.toContain('1.00 倍')
  })

  it('shows a neutral visual instead of a blank card area when no collectible is configured', async () => {
    mocks.getSharedPool.mockResolvedValue(makePool({
      card_skin_key: '',
      card_skin_rarity: '',
      owner_card_asset: {
        collectible_count: 0,
        highest_rarity: '',
        featured_card_key: '',
        featured_card_rarity: '',
        featured_card_serial_no: null,
        featured_card_edition_no: null,
        featured_card_supply: null,
        profile_background_ready: false,
      },
    }))

    const wrapper = mountPoolDetail()
    await flushPromises()

    expect(wrapper.get('.pd-card-showcase-default-image').attributes('src')).toBe('/assets/zero-point-city/mascots/shared-pool-owner.png')
    expect(wrapper.get('.pd-card-showcase-empty').text()).toContain('共享池公共主题')
    expect(wrapper.get('.pd-card-showcase-empty').text()).toContain('经营控制台更换专属背景')
  })
})
