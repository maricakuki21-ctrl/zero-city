import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { SharedPool } from '@/features/bizdecipher/api/bizdecipher'

const mocks = vi.hoisted(() => ({
  createNativeSharedPoolDraft: vi.fn(),
  createSharedPoolNativeOperationID: vi.fn(() => 'native-operation-1'),
  createSharedPool: vi.fn(),
  listMySharedPools: vi.fn(),
  updateSharedPool: vi.fn(),
  deleteSharedPool: vi.fn(),
  fetchSharedPoolUpstreamModels: vi.fn(),
  routerPush: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn(),
  showInfo: vi.fn(),
}))

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: mocks.routerPush }),
}))

vi.mock('@/features/bizdecipher/api/bizdecipher', () => ({
  createNativeSharedPoolDraft: mocks.createNativeSharedPoolDraft,
  createSharedPoolNativeOperationID: mocks.createSharedPoolNativeOperationID,
  createSharedPool: mocks.createSharedPool,
  listMySharedPools: mocks.listMySharedPools,
  updateSharedPool: mocks.updateSharedPool,
  deleteSharedPool: mocks.deleteSharedPool,
  fetchSharedPoolUpstreamModels: mocks.fetchSharedPoolUpstreamModels,
}))

vi.mock('@/composables/useSharedPoolOnboarding', () => ({
  useSharedPoolOnboarding: () => ({ replayGuide: vi.fn() }),
}))

vi.mock('@/composables/usePoolOwner', () => ({
  usePoolCardSkin: () => ({
    skinSelectorPoolId: { value: null },
    availableCards: { value: [] },
    loadingCards: { value: false },
    settingSkinPoolId: { value: null },
    clearingSkinPoolId: { value: null },
    setCardSkin: vi.fn(),
    clearCardSkin: vi.fn(),
    openSkinSelector: vi.fn(),
    closeSkinSelector: vi.fn(),
  }),
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: mocks.showError,
    showSuccess: mocks.showSuccess,
    showInfo: mocks.showInfo,
  }),
}))

vi.mock('@/utils/apiError', () => ({
  extractApiErrorMessage: (error: unknown, fallback: string) => {
    if (error && typeof error === 'object' && 'message' in error) {
      return String((error as { message?: unknown }).message || fallback)
    }
    return fallback
  },
}))

import AccountSquareOwnerView from '@/features/bizdecipher/views/user/AccountSquareOwnerView.vue'

const AppLayoutStub = defineComponent({
  name: 'AppLayout',
  template: '<div><slot /></div>',
})

const SharedPoolGuideButtonStub = defineComponent({
  name: 'SharedPoolGuideButton',
  template: '<button type="button"><slot /></button>',
})

const SharedPoolOwnerWalletPanelStub = defineComponent({
  name: 'SharedPoolOwnerWalletPanel',
  template: '<div data-testid="owner-wallet" />',
})

const RouterLinkStub = defineComponent({
  name: 'RouterLink',
  template: '<a><slot /></a>',
})

function mountOwnerView() {
  return mount(AccountSquareOwnerView, {
    global: {
      stubs: {
        AppLayout: AppLayoutStub,
        SharedPoolGuideButton: SharedPoolGuideButtonStub,
        SharedPoolOwnerWalletPanel: SharedPoolOwnerWalletPanelStub,
        RouterLink: RouterLinkStub,
      },
    },
  })
}

function makePool(overrides: Partial<SharedPool> = {}): SharedPool {
  return {
    id: 42,
    config_version: 1,
    name: 'Account pool',
    description: '',
    owner_label: 'Owner',
    tier: 'standard',
    status: 'healthy',
    listed: false,
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
    today_availability: 100,
    seven_day_availability: 100,
    avg_latency_ms: 100,
    account_mode_enabled: true,
    verification_mode: 'full_check',
    ...overrides,
  }
}

function makeAccountPool(
  summary: Partial<NonNullable<SharedPool['account_summary']>>,
  overrides: Partial<SharedPool> = {},
): SharedPool {
  const base = makePool()
  return makePool({
    ...overrides,
    account_summary: { ...base.account_summary!, ...summary },
  })
}

describe('AccountSquareOwnerView create flow', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mocks.listMySharedPools.mockResolvedValue([])
    mocks.fetchSharedPoolUpstreamModels.mockResolvedValue({ models: [], http_status: 200 })
  })

  it('creates a name-only native draft and opens its detail with the real id', async () => {
    mocks.createNativeSharedPoolDraft.mockResolvedValue({ id: 314 })

    const wrapper = mountOwnerView()
    await flushPromises()

    await wrapper.get('[data-tour="shared-pool-owner-create"]').trigger('click')
    await wrapper.get('.asow-create-panel input.asow-input').setValue('多账号测试池')
    await wrapper.get('.asow-create-panel .asow-form-actions .asow-btn-primary').trigger('click')
    await flushPromises()

    expect(mocks.createNativeSharedPoolDraft).toHaveBeenCalledWith(expect.objectContaining({
      name: '多账号测试池',
      operation_id: expect.any(String),
    }))
    expect(mocks.routerPush).toHaveBeenCalledWith({
      name: 'AccountSquareOwnerPoolDetail',
      params: { id: 314 },
      query: { setup: 'connect' },
    })
    expect(wrapper.text()).not.toContain('创建共享池失败')
  })

  it('keeps earnings off the default page and previews the pool name before creating', async () => {
    const wrapper = mountOwnerView()
    await flushPromises()
    expect(wrapper.find('[data-testid="owner-wallet"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="owner-wallet-link"]').attributes('to')).toBe('/wallet#shared-pool-earnings')
    await wrapper.get('[data-tour="shared-pool-owner-create"]').trigger('click')
    await wrapper.get('.asow-create-panel input.asow-input').setValue('预览名称')
    expect(wrapper.get('[aria-label="市场卡片预览"]').text()).toContain('预览名称')
    expect(wrapper.get('[aria-label="市场卡片预览"]').text()).toContain('未发布')
  })

  it('retains the saved draft when opening its detail fails without another create action', async () => {
    mocks.createNativeSharedPoolDraft.mockResolvedValueOnce({ id: 319 })
    mocks.routerPush.mockRejectedValueOnce(new Error('navigation failed'))
    const wrapper = mountOwnerView()
    await flushPromises()
    await wrapper.get('[data-tour="shared-pool-owner-create"]').trigger('click')
    await wrapper.get('.asow-create-panel input.asow-input').setValue('保留已保存草稿')
    await wrapper.get('.asow-create-panel .asow-form-actions .asow-btn-primary').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('草稿已保存')
    expect(wrapper.text()).toContain('继续已创建的草稿 #319')
    expect(wrapper.text()).not.toContain('重新开始草稿')
    expect(wrapper.find('.asow-create-panel button.asow-btn-primary').exists()).toBe(false)
    expect(mocks.createNativeSharedPoolDraft).toHaveBeenCalledTimes(1)
  })

  it('can create and retain a draft when session storage is unavailable', async () => {
    const read = vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => { throw new Error('storage blocked') })
    const remove = vi.spyOn(Storage.prototype, 'removeItem').mockImplementation(() => { throw new Error('storage blocked') })
    mocks.createNativeSharedPoolDraft.mockResolvedValueOnce({ id: 320 })
    try {
      const wrapper = mountOwnerView()
      await flushPromises()
      await wrapper.get('[data-tour="shared-pool-owner-create"]').trigger('click')
      await wrapper.get('.asow-create-panel input.asow-input').setValue('禁用存储草稿')
      await wrapper.get('.asow-create-panel .asow-form-actions .asow-btn-primary').trigger('click')
      await flushPromises()
      expect(mocks.createNativeSharedPoolDraft).toHaveBeenCalledTimes(1)
      expect(mocks.routerPush).toHaveBeenCalledWith(expect.objectContaining({ params: { id: 320 } }))
      expect(wrapper.text()).toContain('继续已创建的草稿 #320')
    } finally {
      read.mockRestore()
      remove.mockRestore()
    }
  })

  it('does not present an unavailable owner list as zero pools', async () => {
    mocks.listMySharedPools.mockRejectedValueOnce(new Error('unavailable'))
    const wrapper = mountOwnerView()
    await flushPromises()
    expect(wrapper.get('.asow-overview').text()).not.toContain('0')
    expect(wrapper.text()).toContain('池列表暂时无法刷新')
  })

  it('filters pools without treating an unfinished native pool as ready to publish', async () => {
    mocks.listMySharedPools.mockResolvedValue([
      makePool({ id: 42, name: '原生草稿', native_onboarding_state: 'draft', listed: false }),
      makePool({ id: 43, name: '已上架资源', listed: true }),
    ])
    const wrapper = mountOwnerView()
    await flushPromises()
    await wrapper.get('[aria-label="搜索我的共享池"]').setValue('原生草稿')
    expect(wrapper.findAll('.asow-pool-card')).toHaveLength(1)
    expect(wrapper.get('.asow-status-mini').text()).toBe('等待接入资源')
    await wrapper.get('[aria-label="筛选池状态"]').setValue('listed')
    expect(wrapper.text()).toContain('没有匹配的共享池')
  })

  it('keeps the same native operation id when an unknown result is retried', async () => {
    mocks.createNativeSharedPoolDraft
      .mockRejectedValueOnce({ message: '请求超时' })
      .mockResolvedValueOnce({ id: 315 })

    const wrapper = mountOwnerView()
    await flushPromises()

    await wrapper.get('[data-tour="shared-pool-owner-create"]').trigger('click')
    await wrapper.get('.asow-create-panel input.asow-input').setValue('重复名称')
    await wrapper.get('.asow-create-panel .asow-form-actions .asow-btn-primary').trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('请求超时')
    await wrapper.get('.asow-create-panel .asow-form-actions .asow-btn-primary').trigger('click')
    await flushPromises()

    expect(mocks.createNativeSharedPoolDraft).toHaveBeenCalledTimes(2)
    expect(mocks.createNativeSharedPoolDraft.mock.calls[0][0].operation_id)
      .toBe(mocks.createNativeSharedPoolDraft.mock.calls[1][0].operation_id)
    expect(mocks.routerPush).toHaveBeenCalledWith({
      name: 'AccountSquareOwnerPoolDetail',
      params: { id: 315 },
      query: { setup: 'connect' },
    })
  })

  it('offers the 0.0001 multiplier floor and inherits pool concurrency by default', async () => {
    const wrapper = mountOwnerView()
    await flushPromises()
    await wrapper.get('[data-tour="shared-pool-owner-create"]').trigger('click')
    await wrapper.get('.asow-create-panel .asow-section-head .asow-btn').trigger('click')

    const multiplier = wrapper.get('input[step="0.0001"]')
    expect(multiplier.attributes('min')).toBe('0.0001')
    const modelConcurrency = wrapper.get('input[step="1"][min="0"]')
    expect((modelConcurrency.element as HTMLInputElement).value).toBe('0')
    expect(wrapper.text()).toContain('0 = 继承池级并发')
  })
})

describe('AccountSquareOwnerView account summary status', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mocks.fetchSharedPoolUpstreamModels.mockResolvedValue({ models: [], http_status: 200 })
  })

  it.each([
    {
      name: 'ready account pool',
      summary: { full_check_passed_accounts: 1, full_check_total_accounts: 3, schedulable_accounts: 1 },
      status: '账号检测 1/3·自动上架中',
      nextStep: '账号检测已达标（1/3，当前 1 个可调度）',
      statusClass: 'asow-chip-ready',
    },
    {
      name: 'missing account total',
      summary: { full_check_passed_accounts: 1, full_check_total_accounts: 0, schedulable_accounts: 1 },
      status: '等待账号满血检测',
      nextStep: '对至少一个账号执行满血检测',
      statusClass: 'asow-chip-pending',
    },
    {
      name: 'no schedulable account',
      summary: { full_check_passed_accounts: 1, full_check_total_accounts: 3, schedulable_accounts: 0 },
      status: '账号检测 1/3·未达门槛',
      nextStep: '当前账号检测 1/3',
      statusClass: 'asow-chip-blocked',
    },
  ])('uses account_summary for $name', async ({ summary, status, nextStep, statusClass }) => {
    mocks.listMySharedPools.mockResolvedValue([makeAccountPool(summary, {
      last_probe_at: '2026-07-20T00:00:00Z',
      last_probe_success: true,
      last_probe_gate_passed: true,
      last_probe_full_check_score: 100,
      last_probe_full_check_total: 10,
    })])

    const wrapper = mountOwnerView()
    await flushPromises()

    expect(wrapper.get('.asow-status-mini').text()).toContain(status)
    expect(wrapper.get('.asow-status-mini').classes()).toContain(statusClass)
    expect(wrapper.get('.asow-pool-next-step').text()).toContain(nextStep)
  })

  it('does not let a stale pool-level success make an unschedulable professional account pool ready', async () => {
    mocks.listMySharedPools.mockResolvedValue([makeAccountPool({
      full_check_passed_accounts: 1,
      full_check_total_accounts: 3,
      schedulable_accounts: 0,
    }, {
      verification_mode: 'professional_review',
      verification_exemption_reason: '自研模型已完成人工核验',
      last_probe_at: '2026-07-20T00:00:00Z',
      last_probe_success: true,
    })])

    const wrapper = mountOwnerView()
    await flushPromises()

    expect(wrapper.get('.asow-status-mini').text()).toContain('专业核验未完成')
    expect(wrapper.get('.asow-status-mini').classes()).toContain('asow-chip-blocked')
    expect(wrapper.get('.asow-pool-next-step').text()).toContain('至少保留一个可调度账号')
  })
})
