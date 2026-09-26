import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import PoolMarketCard from '@/features/bizdecipher/components/shared-pool/PoolMarketCard.vue'
import type { PoolVM } from '@/composables/useSharedPool'

const pool = {
  id: 271,
  mark: '虾',
  name: '虾蹬2',
  owner: '虾蹬',
  ownerId: 9143,
  ownerCardAsset: {
    collectible_count: 1,
    highest_rarity: 'rare',
    featured_card_key: 'owner-profile-card',
    featured_card_rarity: 'rare',
    featured_card_serial_no: null,
    featured_card_edition_no: null,
    featured_card_supply: null,
    profile_background_ready: true,
  },
  cardSkinKey: 'zero_hour_lighthouse_keeper',
  cardSkinRarity: 'legendary',
  tier: 'Standard',
  status: 'healthy',
  models: ['gpt-5.6-sol'],
  todayAvailability: 99,
  sevenDayAvailability: 99,
  rate: 0.02,
  currentUsers: 11,
  maxUsers: 20,
  minBalance: 0,
  hourlyFee: 0,
  hourlyMinUsageWaiver: 0,
  latency: 100,
  complaints: 0,
  likes: 3,
  likedByMe: true,
  successRate: '100.0',
  avatarUrl: '',
  statusNote: '',
  disabledReason: '',
  governanceStatus: 'normal',
  governanceNote: '',
  observation: false,
  lastProbeAt: '2026-07-25T00:00:00Z',
  lastProbeSuccess: true,
  lastProbeErrorType: '',
  lastProbeErrorMessage: '',
  consecutiveProbeFailures: 0,
  lastSuccessfulProbeAt: '2026-07-25T00:00:00Z',
  lastProbeCheckLevel: 'full',
  lastProbeGateRequired: true,
  lastProbeGatePassed: true,
  lastProbeFullCheckPassed: 1,
  lastProbeFullCheckTotal: 1,
  lastProbeFullCheckScore: 100,
  verificationMode: 'full_check',
  verificationExemptionReason: '',
  verificationBadgeLabel: '满血验证',
  verificationTrusted: true,
  accountSummary: {
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
    last_probe_at: null,
  },
  modelConfigs: [],
} satisfies PoolVM

const summary = {
  pool_id: pool.id,
  total_posts: 0,
  discussion_posts: 0,
  feedback_posts: 0,
  incident_posts: 0,
  risk_signals: 0,
  last_post_title: '',
  latest_posts: [],
}

function mountPoolMarketCard(poolValue: PoolVM) {
  return mount(PoolMarketCard, {
    props: {
      pool: poolValue,
      joined: false,
      acting: false,
      summary: { ...summary, pool_id: poolValue.id },
    },
    global: {
      stubs: {
        RouterLink: { template: '<a><slot /></a>' },
        PoolHistoryBar: { name: 'PoolHistoryBar', template: '<div data-testid="history-stub" />' },
      },
    },
  })
}

describe('PoolMarketCard', () => {
  it('keeps business information below the artwork and uses the canonical card signature', () => {
    const wrapper = mountPoolMarketCard(pool)
    expect(wrapper.get('.resource-cover').find('h3').exists()).toBe(false)
    expect(wrapper.get('.resource-body').get('h3').text()).toBe(pool.name)
    expect(wrapper.get('.resource-quote').text()).toBe('“世界快重启了，我先把灯调到你看得见。”')
    expect(wrapper.get('.resource-quote').attributes('title')).toBe('世界快重启了，我先把灯调到你看得见。')
  })
  it('renders the pool-specific background before the owner profile fallback', () => {
    const wrapper = mountPoolMarketCard(pool)

    expect(wrapper.get('.resource-cover img').attributes('src')).toContain('/legendary/zero_hour_lighthouse_keeper.png')
    expect(wrapper.get('.resource-source-badge').attributes('title')).toContain('传说收藏卡')
    expect(wrapper.get('.resource-cover-note').text()).toBe('传说收藏卡')
    expect(wrapper.find('.resource-title-note').exists()).toBe(false)
    expect(wrapper.text()).toContain('待检测')
    expect(wrapper.text()).toContain('近期检测通过率')
    expect(wrapper.text()).toContain('计费规则')
    expect(wrapper.text()).toContain('剩余席位')
    expect(wrapper.find('.resource-cover').exists()).toBe(true)
    expect(wrapper.find('.resource-metrics').exists()).toBe(true)
    expect(wrapper.text()).not.toContain('高可用')
    expect(wrapper.text()).not.toContain('进入二级详情页')
    expect(wrapper.text()).not.toContain('502')
  })

  it('shows free seat access separately from the model billing multiplier', () => {
    // Given / When
    const wrapper = mountPoolMarketCard(pool)
    const feeMetric = wrapper.findAll('.resource-metrics > div')[1]

    // Then
    expect(feeMetric?.find('dd').text()).toBe('免席位费')
    expect(feeMetric?.find('small').text()).toBe('单价 ×0.02')
  })

  it('keeps history outside the navigation link and displays the seat waiver threshold', () => {
    const wrapper = mountPoolMarketCard({ ...pool, hourlyFee: 0.01, hourlyMinUsageWaiver: 0.1 })
    expect(wrapper.get('[data-testid="history-stub"]').element.closest('a')).toBeNull()
    expect(wrapper.text()).toContain('用量达 0.1 减免')
  })

  it('does not display a zero-percent today when no service evidence exists', () => {
    const wrapper = mountPoolMarketCard({
      ...pool, todayAvailability: 0, sevenDayAvailability: 0,
      lastProbeAt: '', lastSuccessfulProbeAt: '', lastProbeFullCheckTotal: 0,
      consecutiveProbeFailures: 0, accountSummary: { ...pool.accountSummary, total_calls: 0 },
    })
    expect(wrapper.text()).not.toContain('今日 暂无样本')
    expect(wrapper.text()).not.toContain('今日 0.0%')
  })

  it('does not present stale or unknown runtime data as currently available', () => {
    const wrapper = mountPoolMarketCard({ ...pool, status: 'unknown' })
    expect(wrapper.text()).toContain('待验证')
    expect(wrapper.text()).not.toContain('当前可用')
    expect(wrapper.findAll('.resource-metrics > div')[0]?.find('dd').text()).toBe('待检测')
    expect(wrapper.vm.serviceGrade.code).toBe('OBSERVE')
  })

  it('distinguishes stale and failed observation reads from a healthy service', async () => {
    const wrapper = mountPoolMarketCard(pool)
    const history = wrapper.findComponent({ name: 'PoolHistoryBar' })
    history.vm.$emit('observed', { count: 1, passed: 1, latest: null, latestAt: '2026-09-23T00:00:00Z', state: 'stale', smallSample: true })
    await wrapper.vm.$nextTick()
    expect(wrapper.text()).toContain('待重新检测')
    expect(wrapper.text()).toContain('历史样本 · 待更新')
    expect(wrapper.text()).not.toContain('最近检测通过')
    history.vm.$emit('observed', { count: 0, passed: 0, latest: null, latestAt: null, state: 'error', smallSample: false })
    await wrapper.vm.$nextTick()
    expect(wrapper.text()).toContain('检测暂不可读')
    expect(wrapper.text()).toContain('读取失败')
    expect(wrapper.text()).toContain('请重试，不代表服务故障')
  })

  it('uses the neutral shared-pool theme when the owner has no collectible card', () => {
    const wrapper = mountPoolMarketCard({
      ...pool,
      cardSkinKey: '',
      cardSkinRarity: '',
      ownerCardAsset: {
        ...pool.ownerCardAsset,
        collectible_count: 0,
        featured_card_key: '',
        featured_card_rarity: '',
        profile_background_ready: false,
      },
    })

    expect(wrapper.get('.resource-cover img').attributes('src')).toContain('/mascots/shared-pool-owner.png')
    expect(wrapper.find('.resource-title-note').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('收藏卡')
  })

  it('preserves join, leave, like, and report actions', async () => {
    const joinWrapper = mountPoolMarketCard(pool)
    await joinWrapper.get('.pmc-join-btn').trigger('click')
    await joinWrapper.get('.pmc-like-btn').trigger('click')
    await joinWrapper.get('.pmc-report-btn').trigger('click')

    expect(joinWrapper.emitted('join')?.[0]).toEqual([pool])
    expect(joinWrapper.emitted('like')?.[0]).toEqual([pool])
    expect(joinWrapper.emitted('report')?.[0]).toEqual([pool])

    const leaveWrapper = mount(PoolMarketCard, {
      props: { pool, joined: true, acting: false, summary },
      global: {
        stubs: {
          RouterLink: { template: '<a><slot /></a>' },
          PoolHistoryBar: { template: '<div data-testid="history-stub" />' },
        },
      },
    })
    await leaveWrapper.get('.pmc-leave-btn').trigger('click')
    expect(leaveWrapper.emitted('leave')?.[0]).toEqual([pool])
  })

  it('does not emit join while observation mode disables joining', async () => {
    const wrapper = mount(PoolMarketCard, {
      props: {
        pool,
        joined: false,
        acting: false,
        summary,
        joinDisabled: true,
        joinDisabledLabel: '观察中',
      },
      global: {
        stubs: {
          RouterLink: { template: '<a><slot /></a>' },
          PoolHistoryBar: { template: '<div data-testid="history-stub" />' },
        },
      },
    })

    await wrapper.get('.pmc-join-btn').trigger('click')
    expect(wrapper.emitted('join')).toBeUndefined()
  })
})
