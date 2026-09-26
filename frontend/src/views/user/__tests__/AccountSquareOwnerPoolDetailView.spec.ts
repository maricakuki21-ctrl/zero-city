import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { SharedPool, SharedPoolModelEndpointPricing } from '@/features/bizdecipher/api/bizdecipher'

const mocks = vi.hoisted(() => ({
  route: { params: { id: '42' }, query: {} as Record<string, string> },
  routerPush: vi.fn(),
  listMySharedPools: vi.fn(),
  activateSharedPoolNativeBilling: vi.fn(),
  listSharedPoolAccounts: vi.fn(),
  listSharedPoolMembers: vi.fn(),
  listSharedPoolProbeHistories: vi.fn(),
  listSharedPoolModelPricing: vi.fn(),
  updateSharedPool: vi.fn(),
  createNativeSharedPoolAccount: vi.fn(),
  repairSharedPoolNativeAccount: vi.fn(),
  verifySharedPoolNativeReadiness: vi.fn(),
  createSharedPoolAccount: vi.fn(),
  createSharedPoolNativeOperationID: vi.fn(),
  createSharedPoolProbeOperationID: vi.fn(),
  deleteSharedPool: vi.fn(),
  deleteSharedPoolAccount: vi.fn(),
  fetchSharedPoolUpstreamModels: vi.fn(),
  getSharedPoolProbeJob: vi.fn(),
  importSharedPoolAccounts: vi.fn(),
  importSharedPoolOAuthPackage: vi.fn(),
  clearPoolCardSkin: vi.fn(),
  isSharedPoolProbeJobTerminal: vi.fn(),
  probeSharedPoolUpstream: vi.fn(),
  removeSharedPoolMember: vi.fn(),
  saveSharedPoolCustomPrice: vi.fn(),
  setPoolCardSkin: vi.fn(),
  waitForSharedPoolProbeJob: vi.fn(),
  getCheckinCards: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn(),
  showInfo: vi.fn(),
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
  activateSharedPoolNativeBilling: mocks.activateSharedPoolNativeBilling,
  createNativeSharedPoolAccount: mocks.createNativeSharedPoolAccount,
  repairSharedPoolNativeAccount: mocks.repairSharedPoolNativeAccount,
  createSharedPoolAccount: mocks.createSharedPoolAccount,
  createSharedPoolNativeOperationID: mocks.createSharedPoolNativeOperationID,
  createSharedPoolProbeOperationID: mocks.createSharedPoolProbeOperationID,
  deleteSharedPool: mocks.deleteSharedPool,
  deleteSharedPoolAccount: mocks.deleteSharedPoolAccount,
  fetchSharedPoolUpstreamModels: mocks.fetchSharedPoolUpstreamModels,
  getSharedPoolProbeJob: mocks.getSharedPoolProbeJob,
  importSharedPoolAccounts: mocks.importSharedPoolAccounts,
  importSharedPoolOAuthPackage: mocks.importSharedPoolOAuthPackage,
  clearPoolCardSkin: mocks.clearPoolCardSkin,
  isSharedPoolProbeJobTerminal: mocks.isSharedPoolProbeJobTerminal,
  listMySharedPools: mocks.listMySharedPools,
  listSharedPoolModelPricing: mocks.listSharedPoolModelPricing,
  listSharedPoolAccounts: mocks.listSharedPoolAccounts,
  listSharedPoolMembers: mocks.listSharedPoolMembers,
  listSharedPoolProbeHistories: mocks.listSharedPoolProbeHistories,
  probeSharedPoolUpstream: mocks.probeSharedPoolUpstream,
  removeSharedPoolMember: mocks.removeSharedPoolMember,
  saveSharedPoolCustomPrice: mocks.saveSharedPoolCustomPrice,
  setPoolCardSkin: mocks.setPoolCardSkin,
  updateSharedPool: mocks.updateSharedPool,
  verifySharedPoolNativeReadiness: mocks.verifySharedPoolNativeReadiness,
  waitForSharedPoolProbeJob: mocks.waitForSharedPoolProbeJob,
}))

vi.mock('@/api/user', () => ({
  getCheckinCards: mocks.getCheckinCards,
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: mocks.showError,
    showSuccess: mocks.showSuccess,
    showInfo: mocks.showInfo,
  }),
}))

import AccountSquareOwnerPoolDetailView from '@/features/bizdecipher/views/user/AccountSquareOwnerPoolDetailView.vue'

const AppLayoutStub = defineComponent({
  name: 'AppLayout',
  template: '<div><slot /></div>',
})

const SharedPoolOwnerWalletPanelStub = defineComponent({
  name: 'SharedPoolOwnerWalletPanel',
  template: '<div data-testid="owner-wallet" />',
})

const RouterLinkStub = defineComponent({
  name: 'RouterLink',
  template: '<a><slot /></a>',
})

function makePool(overrides: Partial<SharedPool> = {}): SharedPool {
  return {
    id: 42,
    config_version: 1,
    name: 'Original pool',
    description: '',
    owner_label: 'Owner',
    tier: 'standard',
    status: 'healthy',
    listed: true,
    models: ['gpt-4o-mini'],
    model_configs: [{
      provider: 'openai',
      model_name: 'gpt-4o-mini',
      rate_multiplier: 1,
      five_hour_protection_percent: 100,
      seven_day_protection_percent: 100,
      daily_protection_percent: 100,
      max_concurrency: 1,
      model_open: true,
    }],
    account_summary: {
      total_accounts: 0,
      configured_accounts: 0,
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
      model_coverage: [],
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
    total_calls: 0,
    successful_calls: 0,
    failed_calls: 0,
    upstream_base_url: 'https://api.example.test/v1',
    has_upstream_key: true,
    account_mode_enabled: false,
    verification_mode: 'full_check',
    verification_exemption_reason: '',
    platform_fee_percent: 10,
    ...overrides,
  }
}

function mountDetail() {
  return mount(AccountSquareOwnerPoolDetailView, {
    global: {
      stubs: {
        AppLayout: AppLayoutStub,
        SharedPoolOwnerWalletPanel: SharedPoolOwnerWalletPanelStub,
        RouterLink: RouterLinkStub,
      },
    },
  })
}

async function openSettings(wrapper: ReturnType<typeof mountDetail>) {
  const settingsTab = wrapper.findAll('.aopd-tab').find((tab) => tab.text() === '设置')
  expect(settingsTab, 'settings tab').toBeDefined()
  await settingsTab?.trigger('click')
}

function saveButton(wrapper: ReturnType<typeof mountDetail>) {
  const button = wrapper.findAll('.aopd-btn-primary').find((item) => item.text().includes('保存设置'))
  expect(button, 'save settings button').toBeDefined()
  return button!
}

async function openDetection(wrapper: ReturnType<typeof mountDetail>) {
  const detectionTab = wrapper.findAll('.aopd-tab').find((tab) => tab.text() === '检测')
  expect(detectionTab, 'detection tab').toBeDefined()
  await detectionTab?.trigger('click')
}

function makeAccountModePool(summary: Partial<NonNullable<SharedPool['account_summary']>>): SharedPool {
  const baseSummary = makePool().account_summary!
  return makePool({
    account_mode_enabled: true,
    has_upstream_key: false,
    upstream_base_url: '',
    account_summary: { ...baseSummary, ...summary },
  })
}

function makeEndpointPricing(
  endpointType: SharedPoolModelEndpointPricing['endpoint_type'],
  gateStatus: string,
): SharedPoolModelEndpointPricing {
  const billingMode = endpointType === 'video'
    ? 'video'
    : (endpointType === 'image_generation' || endpointType === 'image_edit' ? 'image' : 'token')
  return {
    pool_model_id: 7,
    provider: 'openai',
    model_name: 'gpt-4o-mini',
    display_name: 'GPT-4o mini',
    pricing_source: 'official_catalog',
    pricing_status: 'ready',
    pricing_config_version: 1,
    endpoint_id: endpointType === 'chat' ? 11 : 12,
    endpoint_type: endpointType,
    enabled: true,
    gate_status: gateStatus,
    endpoint_pricing_status: 'ready',
    config_version: 1,
    current_price: {
      price_version_id: 31,
      pool_id: 42,
      pool_model_id: 7,
      endpoint_id: endpointType === 'chat' ? 11 : 12,
      model_name: 'gpt-4o-mini',
      endpoint_type: endpointType,
      pricing_source: 'official_catalog',
      pricing_status: 'ready',
      config_version: 1,
      base_price: { billing_mode: billingMode, currency: 'USD' },
      multiplier: 1,
      user_price: { billing_mode: billingMode, currency: 'USD' },
      effective_from: '2026-07-20T00:00:00Z',
      explicit_free: false,
      example_cost: 0.01,
    },
  }
}

describe('AccountSquareOwnerPoolDetailView settings flow', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    sessionStorage.clear()
    mocks.route.params = { id: '42' }
    mocks.route.query = {}
    mocks.listMySharedPools.mockResolvedValue([makePool()])
    mocks.listSharedPoolAccounts.mockResolvedValue([])
    mocks.listSharedPoolMembers.mockResolvedValue([])
    mocks.listSharedPoolProbeHistories.mockResolvedValue([])
    mocks.listSharedPoolModelPricing.mockResolvedValue([])
    mocks.getCheckinCards.mockResolvedValue([])
    mocks.createSharedPoolProbeOperationID.mockReturnValue('probe-operation')
    mocks.createSharedPoolNativeOperationID.mockReturnValue('native-account-operation')
    mocks.isSharedPoolProbeJobTerminal.mockReturnValue(true)
    mocks.waitForSharedPoolProbeJob.mockResolvedValue(undefined)
  })

  it('opens member management for a native pool', async () => {
    mocks.listMySharedPools.mockResolvedValue([makePool({ native_onboarding_state: 'billing_active' })])
    const wrapper = mountDetail()
    await flushPromises()
    const button = wrapper.findAll('.aopd-header-actions button').find(item => item.text().includes('管理成员'))
    expect(button).toBeDefined()
    await button!.trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('当前没有用户占用席位。')
    expect(wrapper.find('.aopd-member-filter').exists()).toBe(true)
    expect(wrapper.find('.aopd-native-onboarding').exists()).toBe(false)
    await wrapper.findAll('.native-pool-tabs button')[2].trigger('click')
    expect(wrapper.text()).toContain('在钱包查看本池收益与账单')
    expect(wrapper.find('.aopd-native-onboarding').exists()).toBe(false)
  })

  it('updates the displayed pool and confirms a successful settings save', async () => {
    const updated = makePool({ name: 'Renamed pool', config_version: 2 })
    mocks.updateSharedPool.mockResolvedValue(updated)

    const wrapper = mountDetail()
    await flushPromises()
    await openSettings(wrapper)

    expect(wrapper.get('.aopd-header').classes()).toContain('aopd-header-default-skin')
    expect(wrapper.text()).toContain('保存后同步所有已启用模型')
    expect(wrapper.text()).toContain('池主之后仍可随时修改')

    const nameInput = wrapper.find('.aopd-form-card input.aopd-input')
    await nameInput.setValue('Renamed pool')
    await saveButton(wrapper).trigger('click')
    await flushPromises()

    expect(mocks.updateSharedPool).toHaveBeenCalledWith(42, expect.objectContaining({
      expected_config_version: 1,
      name: 'Renamed pool',
    }))
    expect(wrapper.get('.aopd-title').text()).toBe('Renamed pool')
    expect(wrapper.text()).toContain('设置已保存')
  })

  it('keeps and surfaces the server refresh hint on a config conflict', async () => {
    mocks.updateSharedPool.mockRejectedValue({
      status: 409,
      message: '配置版本已变化，请刷新后重试',
    })

    const wrapper = mountDetail()
    await flushPromises()
    await openSettings(wrapper)

    const nameInput = wrapper.find('.aopd-form-card input.aopd-input')
    await nameInput.setValue('冲突后的名称')
    await saveButton(wrapper).trigger('click')
    await flushPromises()

    expect(mocks.showError).toHaveBeenCalledWith('配置版本已变化，请刷新后重试')
    expect(wrapper.text()).toContain('配置版本已变化，请刷新后重试')
  })
})

describe('AccountSquareOwnerPoolDetailView native onboarding', () => {
  function customPriceItem() {
    return { ...makeEndpointPricing('chat', 'unverified'), pricing_source: 'owner_custom' as const, current_price: null }
  }
  function savedQuote() {
    return {
      ...makeEndpointPricing('chat', 'unverified').current_price!,
      pricing_source: 'owner_custom' as const,
      base_price: { billing_mode: 'token' as const, currency: 'USD', input_price: 0.000003, output_price: 0.000009 },
    }
  }
  async function setNativePrice(wrapper: ReturnType<typeof mountDetail>) {
    const editor = wrapper.get('.native-pricing__editor')
    await editor.get('summary').trigger('click')
    for (const [label, value] of [['输入 / 百万 token', '3'], ['输出 / 百万 token', '9']]) {
      const field = editor.findAll('label').find(item => item.get('span').text() === label)!
      await field.get('input').setValue(value)
    }
    return editor.get('button.native-pricing__save')
  }
  beforeEach(() => {
    vi.clearAllMocks()
    sessionStorage.clear()
    mocks.route.params = { id: '42' }
    mocks.route.query = {}
    mocks.listMySharedPools.mockResolvedValue([makePool({
      native_onboarding_state: 'supply_ready_billing_blocked',
      listed: false,
    })])
    mocks.listSharedPoolAccounts.mockResolvedValue([{
      id: 7,
      pool_id: 42,
      owner_id: 1,
      name: 'Primary',
      provider: 'openai',
      auth_type: 'api_key',
      upstream_base_url: 'https://api.example.test/v1',
      has_upstream_key: true,
      has_oauth_credentials: false,
      key_preview: 'sk-***',
      auto_pause_on_expired: false,
      schedulable: false,
      status: 'pending',
      account_weight: 1,
      priority: 0,
      rpm_limit: 0,
      account_concurrency: 0,
      user_concurrency: 0,
      ttl_seconds: 0,
      model_configs: [],
      full_check_score: 0,
      full_check_passed: 0,
      full_check_total: 0,
      gate_required: false,
      gate_passed: false,
      total_calls: 0,
      successful_calls: 0,
      failed_calls: 0,
      native_operation_id: 'server-operation-7',
      native_binding_state: 'ready',
      native_binding_step: 'ready',
      native_models: ['gpt-4o-mini'],
      native_connection_status: 'verified',
      native_connection_verified_at: '2026-09-06T00:00:00Z',
      native_evidence_stale: false,
      native_models_verified_at: '2026-09-06T00:00:00Z',
      created_at: '2026-09-06T00:00:00Z',
      updated_at: '2026-09-06T00:00:00Z',
    }])
    mocks.listSharedPoolMembers.mockResolvedValue([])
    mocks.listSharedPoolProbeHistories.mockResolvedValue([])
    mocks.listSharedPoolModelPricing.mockResolvedValue([])
    mocks.getCheckinCards.mockResolvedValue([])
    mocks.createSharedPoolNativeOperationID.mockReturnValue('native-account-operation')
    mocks.saveSharedPoolCustomPrice.mockReset()
  })

  it('submits native custom prices in per-token units and preserves confirmed success after a read failure', async () => {
    mocks.listSharedPoolModelPricing.mockResolvedValueOnce([customPriceItem()]).mockRejectedValueOnce(new Error('read failed'))
    mocks.saveSharedPoolCustomPrice.mockResolvedValue(savedQuote())
    const wrapper = mountDetail()
    await flushPromises()
    const save = await setNativePrice(wrapper)
    await save.trigger('click')
    await flushPromises()
    expect(mocks.saveSharedPoolCustomPrice).toHaveBeenCalledWith(42, expect.objectContaining({
      model_name: 'gpt-4o-mini', endpoint_type: 'chat', input_price: 0.000003, output_price: 0.000009,
      billing_mode: 'token', multiplier: 1, operation_id: expect.stringMatching(/^shared-price:42:/),
    }))
    expect(wrapper.get('.native-pricing__message').text()).toContain('新价格已发布')
    expect(wrapper.get('.native-pricing__message').text()).toContain('无需重复发布')
    mocks.listSharedPoolModelPricing.mockResolvedValue([{
      ...customPriceItem(),
      current_price: savedQuote(),
      pricing_status: 'ready',
      endpoint_pricing_status: 'ready',
    }])
    await save.trigger('click')
    await flushPromises()
    expect(mocks.saveSharedPoolCustomPrice).toHaveBeenCalledTimes(2)
    expect(mocks.saveSharedPoolCustomPrice.mock.calls[1][1].operation_id)
      .toBe(mocks.saveSharedPoolCustomPrice.mock.calls[0][1].operation_id)
  })

  it('retries unconfirmed price submissions using the same payload and operation', async () => {
    mocks.listSharedPoolModelPricing.mockResolvedValue([customPriceItem()])
    mocks.saveSharedPoolCustomPrice.mockRejectedValueOnce(new Error('timeout')).mockResolvedValueOnce(savedQuote())
    const wrapper = mountDetail()
    await flushPromises()
    const save = await setNativePrice(wrapper)
    await save.trigger('click')
    await flushPromises()
    expect(wrapper.get('.native-pricing__message').text()).toContain('未能确认')
    const first = mocks.saveSharedPoolCustomPrice.mock.calls[0]
    await save.trigger('click')
    await flushPromises()
    expect(mocks.saveSharedPoolCustomPrice.mock.calls[1]).toEqual(first)
    expect(wrapper.get('.native-pricing__message').text()).toContain('新价格已发布')
  })

  it('rejects negative optional prices instead of silently treating them as unset', async () => {
    mocks.listSharedPoolModelPricing.mockResolvedValue([customPriceItem()])
    const wrapper = mountDetail()
    await flushPromises()
    const save = await setNativePrice(wrapper)
    const minimum = wrapper.get('.native-pricing__editor').findAll('label').find(item => item.get('span').text() === '最低收费（可选）')!
    await minimum.get('input').setValue('-1')
    await save.trigger('click')
    await flushPromises()
    expect(mocks.saveSharedPoolCustomPrice).not.toHaveBeenCalled()
    expect(wrapper.get('.native-pricing__message').text()).toContain('非负的有限数字')
  })

  it('does not claim price publication when the server returned a different pool quote', async () => {
    mocks.listSharedPoolModelPricing.mockResolvedValue([customPriceItem()])
    mocks.saveSharedPoolCustomPrice.mockResolvedValue({ ...savedQuote(), pool_id: 999 })
    const wrapper = mountDetail()
    await flushPromises()
    await (await setNativePrice(wrapper)).trigger('click')
    await flushPromises()
    expect(wrapper.get('.native-pricing__message').text()).toContain('未能确认')
    expect(wrapper.get('.native-pricing__message').text()).not.toContain('新价格已发布')
  })

  it('renders server-owned blocked readiness without legacy publication, key, or call controls', async () => {
    const wrapper = mountDetail()
    await flushPromises()

    expect(wrapper.get('[data-testid="pool-onboarding-status"]').attributes('data-state')).toBe('supply_ready_billing_blocked')
    expect(wrapper.text()).toContain('资源已连接，待开通计费与上架')
    expect(wrapper.find('.aopd-tabs').exists()).toBe(false)
    expect(wrapper.find('[data-testid="pool-onboarding-publish"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="pool-onboarding-key"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="pool-onboarding-call"]').exists()).toBe(false)
  })

  it('publishes from the native preview and reflects the server listing', async () => {
    const wrapper = mountDetail()
    await flushPromises()
    mocks.activateSharedPoolNativeBilling.mockResolvedValue({
      pool_id: 42, state: 'billing_active', config_version: 2, activated_config_version: 2, already_active: false,
    })
    mocks.listMySharedPools.mockResolvedValue([makePool({ native_onboarding_state: 'billing_active', config_version: 2, listed: true })])
    await wrapper.findAll('.setup-tabs button')[2].trigger('click')
    await wrapper.get('[data-testid="pool-onboarding-publish"]').trigger('click')
    await flushPromises()
    expect(mocks.activateSharedPoolNativeBilling).toHaveBeenCalledWith(42, {
      operation_id: 'native-account-operation', expected_config_version: 1,
    })
    expect(wrapper.get('.aopd-listed-chip').text()).toBe('已上架')
    expect(wrapper.get('.publication-result').text()).toContain('已上架')
  })

  it('distinguishes a member read error from an empty pool and supports retry', async () => {
    mocks.listSharedPoolMembers.mockRejectedValueOnce(new Error('offline'))
    const wrapper = mountDetail()
    await flushPromises()
    await wrapper.findAll('.native-pool-tabs button')[1].trigger('click')
    expect(wrapper.text()).toContain('成员列表加载失败')
    expect(wrapper.text()).not.toContain('当前没有用户占用席位。')
    await wrapper.get('[role="alert"] button').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('当前没有用户占用席位。')
  })

  it('does not invite duplicate credential entry when existing resources fail to load', async () => {
    mocks.listSharedPoolAccounts.mockRejectedValueOnce(new Error('offline'))
    const wrapper = mountDetail()
    await flushPromises()
    expect(wrapper.get('[data-testid="native-accounts-load-error"]').text()).toContain('无需重新提交已有凭证')
    expect(wrapper.find('[data-testid="native-account-form"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="pool-onboarding-publish"]').exists()).toBe(false)
    await wrapper.get('[data-testid="native-accounts-load-error"] button').trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-testid="native-accounts-load-error"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="pool-onboarding-status"]').isVisible()).toBe(true)
    expect(wrapper.find('[data-testid="native-account-form"]').exists()).toBe(false)
  })

  it('opens resource setup for a new empty draft without another add-account click', async () => {
    mocks.listMySharedPools.mockResolvedValue([makePool({ native_onboarding_state: 'draft' })])
    mocks.listSharedPoolAccounts.mockResolvedValue([])
    const wrapper = mountDetail()
    await flushPromises()
    expect(wrapper.get('[data-testid="native-account-form"]').isVisible()).toBe(true)
    expect(wrapper.get('[data-testid="native-account-name"]').element.value).not.toBe('')
    expect(wrapper.get('[aria-label="市场卡片预览"]').text()).toContain('等待接入模型')
  })

  it('pulls metadata after saving an account and clears the submitted secret', async () => {
    mocks.createNativeSharedPoolAccount.mockResolvedValue({ id: 8 })
    mocks.verifySharedPoolNativeReadiness.mockResolvedValue({ id: 8 })
    const wrapper = mountDetail()
    await flushPromises()
    await wrapper.findAll('.aopd-btn').find(button => button.text() === '添加账号')?.trigger('click')
    await wrapper.get('[data-testid="native-account-url"]').setValue('https://api.example.test/v1')
    await wrapper.get('[data-testid="native-account-secret"]').setValue('fixture-only-key')
    await wrapper.get('[data-testid="native-account-submit"]').trigger('click')
    await flushPromises()
    expect(mocks.verifySharedPoolNativeReadiness).toHaveBeenCalledWith(42, 8, { expected_config_version: 1 })
    expect(wrapper.find('[data-testid="native-account-form"]').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('fixture-only-key')
  })

  it('does not invite duplicate creation when post-save refresh fails', async () => {
    mocks.createNativeSharedPoolAccount.mockResolvedValue({ id: 8 })
    const wrapper = mountDetail()
    await flushPromises()
    mocks.listMySharedPools.mockRejectedValueOnce(new Error('refresh unavailable'))
    await wrapper.findAll('.aopd-btn').find(button => button.text() === '添加账号')?.trigger('click')
    await wrapper.get('[data-testid="native-account-url"]').setValue('https://api.example.test/v1')
    await wrapper.get('[data-testid="native-account-secret"]').setValue('fixture-only-key')
    await wrapper.get('[data-testid="native-account-submit"]').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('资源已保存，模型状态刷新失败')
    expect(wrapper.text()).not.toContain('提交失败，请检查账号配置')
    expect(wrapper.find('[data-testid="native-account-form"]').exists()).toBe(false)
    expect(mocks.createNativeSharedPoolAccount).toHaveBeenCalledTimes(1)
  })

  it('clears a rejected API key and does not expose a 403 response body', async () => {
    mocks.createNativeSharedPoolAccount.mockRejectedValue({ status: 403, message: 'R1_SECRET_MUST_NOT_RENDER' })
    const wrapper = mountDetail()
    await flushPromises()

    await wrapper.findAll('.aopd-btn').find((button) => button.text().includes('添加账号'))?.trigger('click')
    await wrapper.get('[data-testid="native-account-name"]').setValue('Retry account')
    await wrapper.get('[data-testid="native-account-url"]').setValue('https://api.example.test/v1')
    await wrapper.get('[data-testid="native-account-secret"]').setValue('sk-very-secret')
    await wrapper.get('[data-testid="native-account-submit"]').trigger('click')
    await flushPromises()

    expect(mocks.createNativeSharedPoolAccount).toHaveBeenCalledWith(42, expect.objectContaining({
      operation_id: 'native-account-operation',
      upstream_api_key: 'sk-very-secret',
    }))
    expect((wrapper.get('[data-testid="native-account-secret"]').element as HTMLInputElement).value).toBe('')
    expect(wrapper.text()).toContain('提交失败，请检查账号配置后使用相同操作重试。')
    expect(wrapper.text()).not.toContain('R1_SECRET_MUST_NOT_RENDER')
    expect(wrapper.text()).not.toContain('sk-very-secret')
  })

  it('uses server account identity for repair, sends a new versioned PATCH, and never falls back to create or probe', async () => {
    sessionStorage.setItem('shared-pool-native-account:42:primary', 'browser-operation')
    mocks.listSharedPoolAccounts.mockResolvedValue([{
      id: 7,
      name: 'Primary',
      provider: 'openai',
      auth_type: 'api_key',
      upstream_base_url: 'https://api.example.test/v1',
      native_operation_id: 'server-operation-7',
      native_binding_state: 'needs_attention',
      native_binding_step: 'verifying',
      native_connection_status: 'failed',
      native_evidence_stale: true,
    }])
    const wrapper = mountDetail()
    await flushPromises()

    await wrapper.get('[data-testid="native-account-retry"]').trigger('click')

    const form = wrapper.get('[data-testid="native-account-form"]')
    expect(form.attributes('data-mode')).toBe('repair')
    expect(form.attributes('data-repair-account-id')).toBe('7')
    expect(form.attributes('data-native-operation-id')).toBe('server-operation-7')
    expect((wrapper.get('[data-testid="native-account-secret"]').element as HTMLInputElement).value).toBe('')

    await wrapper.get('[data-testid="native-account-secret"]').setValue('sk-corrected-secret')
    await wrapper.get('[data-testid="native-account-submit"]').trigger('click')
    await flushPromises()

    expect(mocks.repairSharedPoolNativeAccount).toHaveBeenCalledWith(42, 7, {
      repair_operation_id: 'native-account-operation',
      expected_config_version: 1,
      provider: 'openai',
      auth_type: 'api_key',
      upstream_base_url: 'https://api.example.test/v1',
      upstream_api_key: 'sk-corrected-secret',
    })
    expect(mocks.createNativeSharedPoolAccount).not.toHaveBeenCalled()
    expect(mocks.probeSharedPoolUpstream).not.toHaveBeenCalled()
    expect(wrapper.find('[data-testid="native-account-form"]').exists()).toBe(false)
  })

  it('starts readiness with the current server version and never invokes the legacy probe', async () => {
    mocks.verifySharedPoolNativeReadiness.mockResolvedValue({ id: 7 })
    const wrapper = mountDetail()
    await flushPromises()

    await wrapper.get('[data-testid="native-account-readiness"]').trigger('click')
    await flushPromises()

    expect(mocks.verifySharedPoolNativeReadiness).toHaveBeenCalledWith(42, 7, { expected_config_version: 1 })
    expect(mocks.probeSharedPoolUpstream).not.toHaveBeenCalled()
  })

  it('refreshes the server version after a repair conflict and requires explicit resubmission', async () => {
    mocks.listMySharedPools
      .mockResolvedValueOnce([makePool({ supply_mode: 'native', native_onboarding_state: 'supply_needs_attention', config_version: 1 })])
      .mockResolvedValueOnce([makePool({ supply_mode: 'native', native_onboarding_state: 'supply_needs_attention', config_version: 2 })])
    mocks.listSharedPoolAccounts.mockResolvedValue([{
      id: 7,
      name: 'Primary',
      provider: 'openai',
      auth_type: 'api_key',
      upstream_base_url: 'https://api.example.test/v1',
      native_operation_id: 'server-operation-7',
      native_binding_state: 'needs_attention',
      native_binding_step: 'verifying',
      native_connection_status: 'failed',
      native_evidence_stale: true,
    }])
    mocks.repairSharedPoolNativeAccount.mockRejectedValue({ status: 409 })
    const wrapper = mountDetail()
    await flushPromises()
    await wrapper.get('[data-testid="native-account-retry"]').trigger('click')
    await wrapper.get('[data-testid="native-account-secret"]').setValue('sk-corrected-secret')
    await wrapper.get('[data-testid="native-account-submit"]').trigger('click')
    await flushPromises()

    expect(mocks.repairSharedPoolNativeAccount).toHaveBeenCalledWith(42, 7, expect.objectContaining({
      repair_operation_id: 'native-account-operation',
      expected_config_version: 1,
    }))
    expect((wrapper.get('[data-testid="native-account-secret"]').element as HTMLInputElement).value).toBe('')
    expect(wrapper.text()).toContain('账号配置已变化，已刷新服务器版本，请重新明确提交修复。')
    expect(mocks.createNativeSharedPoolAccount).not.toHaveBeenCalled()
  })

  it('loads owned cards and applies or clears the native pool appearance background', async () => {
    mocks.listMySharedPools
      .mockResolvedValueOnce([makePool({ native_onboarding_state: 'draft', listed: false })])
      .mockResolvedValue([makePool({
        native_onboarding_state: 'draft',
        listed: false,
        card_skin_key: 'pioneer',
        card_skin_rarity: 'rare',
      })])
    mocks.listSharedPoolAccounts.mockResolvedValue([])
    mocks.getCheckinCards.mockResolvedValue([
      { card_key: 'pioneer', rarity: 'rare', serial_no: 1 },
    ])
    mocks.setPoolCardSkin.mockResolvedValue(undefined)
    mocks.clearPoolCardSkin.mockResolvedValue(undefined)

    const wrapper = mountDetail()
    await flushPromises()
    await wrapper.findAll('.setup-tabs button')[2].trigger('click')

    const setBackground = wrapper.findAll('.native-appearance button').find((button) => button.text() === '设置收藏卡背景')
    expect(setBackground, 'set background button').toBeDefined()
    await setBackground?.trigger('click')
    await flushPromises()

    expect(mocks.getCheckinCards).toHaveBeenCalledWith()
    const cardButton = wrapper.get('.native-skin-grid button')
    await cardButton.trigger('click')
    await flushPromises()

    expect(mocks.setPoolCardSkin).toHaveBeenCalledWith(42, 'pioneer', 'rare')
    expect(mocks.showSuccess).toHaveBeenCalledWith('共享池背景已更新')

    const clearBackground = wrapper.findAll('.native-appearance button').find((button) => button.text() === '使用公共背景')
    expect(clearBackground, 'clear background button').toBeDefined()
    await clearBackground?.trigger('click')
    await flushPromises()

    expect(mocks.clearPoolCardSkin).toHaveBeenCalledWith(42)
    expect(mocks.showSuccess).toHaveBeenCalledWith('共享池背景已清除')
  })
})

describe('AccountSquareOwnerPoolDetailView account-mode readiness', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    sessionStorage.clear()
    mocks.route.params = { id: '42' }
    mocks.route.query = {}
    mocks.listSharedPoolAccounts.mockResolvedValue([])
    mocks.listSharedPoolMembers.mockResolvedValue([])
    mocks.listSharedPoolProbeHistories.mockResolvedValue([])
    mocks.listSharedPoolModelPricing.mockResolvedValue([])
    mocks.getCheckinCards.mockResolvedValue([])
  })

  it('shows account-pool readiness and the passed/total count without calling it full blood', async () => {
    mocks.listMySharedPools.mockResolvedValue([makeAccountModePool({
      full_check_passed_accounts: 1,
      full_check_total_accounts: 3,
      schedulable_accounts: 1,
    })])

    const wrapper = mountDetail()
    await flushPromises()
    const statusRow = wrapper.get('.aopd-status-row')
    expect(statusRow.text()).toContain('账号池可调度')
    expect(statusRow.text()).not.toContain('满血检测通过')

    await openDetection(wrapper)
    expect(wrapper.get('.aopd-passed-banner').text()).toContain('账号池可调度')
    expect(wrapper.get('.aopd-passed-banner').text()).toContain('账号 1/3')
    expect(wrapper.text()).not.toContain('满血检测通过')
  })

  it.each([
    ['no account total', { full_check_passed_accounts: 1, full_check_total_accounts: 0, schedulable_accounts: 1 }],
    ['no schedulable account', { full_check_passed_accounts: 1, full_check_total_accounts: 3, schedulable_accounts: 0 }],
  ])('does not show a ready badge when %s', async (_label, summary) => {
    mocks.listMySharedPools.mockResolvedValue([makeAccountModePool(summary)])

    const wrapper = mountDetail()
    await flushPromises()
    expect(wrapper.get('.aopd-status-row').text()).not.toContain('账号池可调度')
    expect(wrapper.find('.aopd-status-row .aopd-passed-chip').exists()).toBe(false)

    await openDetection(wrapper)
    expect(wrapper.find('.aopd-passed-banner').exists()).toBe(false)
  })
})

describe('AccountSquareOwnerPoolDetailView endpoint gate readiness', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    sessionStorage.clear()
    mocks.route.params = { id: '42' }
    mocks.route.query = {}
    mocks.listSharedPoolAccounts.mockResolvedValue([])
    mocks.listSharedPoolMembers.mockResolvedValue([])
    mocks.listSharedPoolProbeHistories.mockResolvedValue([])
    mocks.getCheckinCards.mockResolvedValue([])
  })

  it('marks a priced text-only pool ready without requiring a media gate', async () => {
    mocks.listMySharedPools.mockResolvedValue([makePool({
      last_probe_gate_passed: true,
      last_probe_full_check_score: 100,
      last_probe_full_check_passed: 3,
      last_probe_full_check_total: 3,
      last_probe_success: true,
    })])
    mocks.listSharedPoolModelPricing.mockResolvedValue([
      makeEndpointPricing('chat', 'unverified'),
      makeEndpointPricing('responses', 'unverified'),
    ])

    const wrapper = mountDetail()
    await flushPromises()

    expect(wrapper.get('.aopd-readiness h2').text()).toBe('池子已公开运营')
    const detectionStep = wrapper.findAll('.aopd-readiness-step').find((step) => step.text().includes('满血检测'))
    expect(detectionStep?.classes()).toContain('aopd-readiness-step-done')
    expect(detectionStep?.text()).toContain('文字服务不需要单独的图片/视频检测')
  })

  it('keeps media protected without lowering a passed pool listing to 80 percent', async () => {
    mocks.listMySharedPools.mockResolvedValue([makePool({
      listed: true,
      last_probe_gate_passed: true,
      last_probe_full_check_score: 100,
      last_probe_full_check_passed: 3,
      last_probe_full_check_total: 3,
      last_probe_success: true,
    })])
    mocks.listSharedPoolModelPricing.mockResolvedValue([
      makeEndpointPricing('chat', 'unverified'),
      makeEndpointPricing('image_generation', 'unverified'),
    ])

    const wrapper = mountDetail()
    await flushPromises()

    expect(wrapper.get('.aopd-readiness h2').text()).toBe('池子已公开运营，媒体服务待验证')
    expect(wrapper.get('.aopd-readiness-copy').text()).toContain('媒体服务 0/1 检测通过')
    expect(wrapper.get('.aopd-readiness-progress').text()).toContain('100%')
    const detectionStep = wrapper.findAll('.aopd-readiness-step').find((step) => step.text().includes('满血检测'))
    expect(detectionStep?.classes()).toContain('aopd-readiness-step-done')
    expect(detectionStep?.text()).toContain('媒体服务 0/1 检测通过')
  })
})

describe('AccountSquareOwnerPoolDetailView terminal probe result', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    sessionStorage.clear()
    mocks.route.params = { id: '42' }
    mocks.route.query = {}
    mocks.listSharedPoolAccounts.mockResolvedValue([])
    mocks.listSharedPoolMembers.mockResolvedValue([])
    mocks.listSharedPoolProbeHistories.mockResolvedValue([])
    mocks.listSharedPoolModelPricing.mockResolvedValue([])
    mocks.getCheckinCards.mockResolvedValue([])
    mocks.createSharedPoolProbeOperationID.mockReturnValue('probe-operation-terminal-failure')
    mocks.isSharedPoolProbeJobTerminal.mockReturnValue(true)
  })

  it('treats an explicit failed terminal result as authoritative over an old passing snapshot', async () => {
    const oldPassingPool = makePool({
      listed: true,
      last_probe_gate_passed: true,
      last_probe_full_check_passed: 10,
      last_probe_full_check_total: 10,
      last_probe_full_check_score: 100,
      last_probe_success: true,
    })
    mocks.listMySharedPools.mockResolvedValue([oldPassingPool])
    mocks.probeSharedPoolUpstream.mockResolvedValue({
      id: 'job-terminal-failure',
      status: 'succeeded',
      model_name: 'gpt-4o-mini',
      attempt: 1,
      max_attempts: 1,
      result: {
        ok: false,
        gate_passed: false,
        full_check_passed: 0,
        full_check_total: 3,
        full_check_score: 20,
        message: '发布闸门未通过',
      },
    })

    const wrapper = mountDetail()
    await flushPromises()
    await openDetection(wrapper)
    await wrapper.get('.aopd-probe-model-field select').setValue('gpt-4o-mini')

    const trigger = wrapper.findAll('.aopd-btn-primary').find((button) => button.text().includes('触发池级满血检测'))
    expect(trigger, 'pool probe button').toBeDefined()
    await trigger?.trigger('click')
    await flushPromises()

    expect(mocks.probeSharedPoolUpstream).toHaveBeenCalledWith(expect.objectContaining({
      pool_id: 42,
      probe_model: 'gpt-4o-mini',
      operation_id: 'probe-operation-terminal-failure',
    }))
    expect(mocks.showError).toHaveBeenCalledWith(expect.stringContaining('未达到上架门槛'))
    expect(wrapper.text()).toContain('未达到上架门槛')
    expect(mocks.showSuccess).not.toHaveBeenCalledWith(expect.stringContaining('满血检测完成'))
  })
})
