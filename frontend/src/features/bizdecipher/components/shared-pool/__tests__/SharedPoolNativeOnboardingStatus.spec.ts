import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import SharedPoolNativeOnboardingStatus from '../SharedPoolNativeOnboardingStatus.vue'

const readyAccount = {
  id: 7,
  name: 'A'.repeat(120),
  native_binding_state: 'ready',
  native_binding_step: 'ready',
  native_models: ['gpt-4o-mini'],
  native_connection_status: 'verified',
  native_evidence_stale: false,
  native_models_verified_at: '2026-09-06T00:00:00Z',
  native_connection_verified_at: '2026-09-06T00:00:00Z',
}

describe('SharedPoolNativeOnboardingStatus', () => {
  it.each(['draft', 'supply_configuring', 'supply_needs_attention', 'supply_ready_billing_blocked', 'billing_active', 'legacy_existing'] as const)(
    'renders the %s state from server-owned onboarding data',
    (state) => {
      const wrapper = mount(SharedPoolNativeOnboardingStatus, { props: { state, accounts: [readyAccount] } })

      expect(wrapper.get('[data-testid="pool-onboarding-status"]').attributes('data-state')).toBe(state)
      expect(wrapper.get('[data-testid="pool-onboarding-readiness"]').exists()).toBe(true)
    },
  )

  it('renders ready-but-blocked evidence without publication, key, or call actions', () => {
    const wrapper = mount(SharedPoolNativeOnboardingStatus, {
      props: { state: 'supply_ready_billing_blocked', accounts: [readyAccount] },
    })

    expect(wrapper.get('[data-testid="pool-onboarding-readiness"]').text()).toContain('待开通计费与上架')
    expect(wrapper.get('[data-testid="pool-onboarding-evidence"]').text()).toContain('gpt-4o-mini')
    expect(wrapper.find('[data-testid="pool-onboarding-publish"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="pool-onboarding-key"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="pool-onboarding-call"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="native-account-connection"]').attributes('data-connection-status')).toBe('verified')
    expect(wrapper.get('[data-testid="native-account-connection-verified-at"]').attributes('data-has-timestamp')).toBe('true')
    expect(wrapper.get('[data-testid="native-account-models-verified-at"]').attributes('data-has-timestamp')).toBe('true')
  })

  it('redacts long error content and exposes retry only for the failed item', () => {
    const wrapper = mount(SharedPoolNativeOnboardingStatus, {
      props: {
        state: 'supply_needs_attention',
        accounts: [
          readyAccount,
          {
            id: 8,
            name: 'Failed account',
            native_binding_state: 'needs_attention',
            native_binding_step: 'verifying',
            native_error_code: 'unsafe_upstream_endpoint',
            native_error_message: 'R1_SECRET_MUST_NOT_RENDER 很长的原始错误内容'.repeat(8),
            native_models: [],
            native_connection_status: 'failed',
            native_evidence_stale: false,
          },
        ],
      },
    })

    expect(wrapper.text()).toContain('unsafe_upstream_endpoint')
    expect(wrapper.text()).not.toContain('R1_SECRET_MUST_NOT_RENDER')
    expect(wrapper.findAll('[data-testid="native-account-retry"]')).toHaveLength(1)
    expect(wrapper.get('[data-testid="native-account-retry"]').attributes('data-account-id')).toBe('8')
  })
  it('keeps large model directories compact without losing names', () => {
    const wrapper = mount(SharedPoolNativeOnboardingStatus, {
      props: { state: 'supply_configuring', accounts: [{ ...readyAccount, native_models: ['a', 'b', 'c', 'd', 'e'] }] },
    })
    expect(wrapper.findAll('.native-onboarding__models > span')).toHaveLength(3)
    expect(wrapper.get('.native-onboarding__model-overflow summary').text()).toBe('其余 2 个模型')
    expect(wrapper.findAll('.native-onboarding__model-overflow span').map(item => item.text())).toEqual(['d', 'e'])
  })

  it('falls back safely for an unexpected onboarding state and marks stale evidence', () => {
    const wrapper = mount(SharedPoolNativeOnboardingStatus, {
      props: {
        state: 'server_state_added_later',
        accounts: [{
          ...readyAccount,
          native_connection_status: 'future_connection_state',
          native_evidence_stale: true,
          native_connection_verified_at: null,
          native_models_verified_at: null,
        }],
      },
    })

    expect(wrapper.get('[data-testid="pool-onboarding-status"]').attributes('data-state')).toBe('unknown')
    expect(wrapper.get('[data-testid="native-account-connection"]').attributes('data-connection-status')).toBe('unknown')
    expect(wrapper.get('[data-testid="native-account-evidence-stale"]').attributes('data-stale')).toBe('true')
    expect(wrapper.get('[data-testid="native-account-connection-verified-at"]').attributes('data-has-timestamp')).toBe('false')
  })

  it('labels metadata reachability without claiming inference readiness and exposes one pending-safe check per account', async () => {
    const wrapper = mount(SharedPoolNativeOnboardingStatus, {
      props: {
        state: 'supply_configuring',
        accounts: [{ ...readyAccount, native_connection_status: 'authenticated_metadata_reachable' }],
        readinessPendingAccountId: 7,
      },
    })

    const button = wrapper.get('[data-testid="native-account-readiness"]')
    expect(wrapper.text()).toContain('认证元数据可达（不代表推理可用）')
    expect(button.attributes('data-account-id')).toBe('7')
    expect(button.attributes('disabled')).toBeDefined()
    expect(button.text()).toBe('检查中…')
  })
})
