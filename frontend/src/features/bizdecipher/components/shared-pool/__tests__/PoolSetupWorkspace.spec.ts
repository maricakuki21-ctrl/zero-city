import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import PoolSetupWorkspace from '../PoolSetupWorkspace.vue'
import type { SharedPool, SharedPoolAccount } from '@/api/bizdecipher'

function render(state = 'draft', activation = false, accounts = 0, listed = false) {
  return mount(PoolSetupWorkspace, {
    props: {
      pool: { id: 1, name: 'Example', native_onboarding_state: state, billing_activation_required: activation, listed } as SharedPool,
      accounts: Array.from({ length: accounts }, (_, id) => ({ id, native_models: ['example-model'] })) as SharedPoolAccount[],
    },
    slots: { appearance: '<button data-testid="appearance">选择背景</button>' },
    global: { stubs: { PoolListingPreview: true, SharedPoolNativeOnboardingStatus: true } },
  })
}
describe('native pool publication context', () => {
  it.each([
    ['draft', true, 0, '尚未接入资源'],
    ['supply_needs_attention', true, 1, '资源连接需要处理'],
    ['supply_ready_billing_blocked', true, 1, '待开通计费与上架'],
    ['supply_configuring', true, 1, '尚未发布'],
  ])('uses server state %s instead of a universal billing claim', async (state, activation, count, expected) => {
    const wrapper = render(state, activation, count)
    await wrapper.findAll('.setup-tabs button')[2].trigger('click')
    expect(wrapper.get('.publish-note').text()).toContain(expected)
    expect(wrapper.text()).not.toContain('当前新池的计费上架尚未开放')
    expect(wrapper.get('[data-testid="appearance"]').isVisible()).toBe(true)
  })
  it('does not show a publication blocker for a listed pool', async () => {
    const wrapper = render('supply_configuring', false, 1, true)
    await wrapper.findAll('.setup-tabs button')[2].trigger('click')
    expect(wrapper.find('.publish-note').exists()).toBe(false)
    expect(wrapper.get('.publish-summary').text()).toContain('已上架')
  })
  it('shows one controlled publish action and disables it during submission', async () => {
    const wrapper = render('supply_ready_billing_blocked', true, 1)
    expect(wrapper.find('[data-testid="pool-onboarding-publish"]').exists()).toBe(false)
    await wrapper.setProps({ canPublish: true })
    await wrapper.findAll('.setup-tabs button')[2].trigger('click')
    await wrapper.get('[data-testid="pool-onboarding-publish"]').trigger('click')
    expect(wrapper.emitted('publish')).toHaveLength(1)
    await wrapper.setProps({ publishing: true })
    expect(wrapper.get('[data-testid="pool-onboarding-publish"]').attributes('disabled')).toBeDefined()
  })
  it('restores unresolved publication in preview and locks configuration', async () => {
    const wrapper = render('billing_active', false, 1, true)
    await wrapper.setProps({ unresolvedPublication: true, canPublish: true, writeLocked: true })
    expect(wrapper.get('[data-testid="pool-onboarding-publish"]').isVisible()).toBe(true)
    expect(wrapper.get('[data-testid="pool-onboarding-publish"]').text()).toContain('重试确认')
    expect(wrapper.get('fieldset[aria-label="接入资源"]').attributes('disabled')).toBeDefined()
  })
})
