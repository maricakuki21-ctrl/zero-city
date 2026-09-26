import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import ZeroCityGovernancePanel from '../ZeroCityGovernancePanel.vue'

const mocks = vi.hoisted(() => ({ auth: { user: { id: 2, role: 'user' } }, get: vi.fn(), post: vi.fn(), put: vi.fn(), polls: vi.fn() }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => mocks.auth }))
vi.mock('@/api/client', () => ({ apiClient: { get: mocks.get, post: mocks.post, put: mocks.put } }))
vi.mock('@/features/bizdecipher/api/community', () => ({ communityAPI: { listPolls: mocks.polls } }))
enableAutoUnmount(afterEach)
beforeEach(() => {
  vi.clearAllMocks()
  mocks.auth.user = { id: 2, role: 'user' }
  mocks.get.mockResolvedValue({ data: { items: [] } })
  mocks.polls.mockResolvedValue({ items: [] })
})

it('does not expose administrator mutation controls to members', async () => {
  const wrapper = mount(ZeroCityGovernancePanel, { props: { kind: 'badges' } })
  await flushPromises()
  expect(wrapper.find('form').exists()).toBe(false)
  expect(mocks.get).toHaveBeenCalledWith('/biz/community/users/2/badges')
  expect(mocks.post).not.toHaveBeenCalled()
})

it('uses the grant recipient rather than the edited lookup field when revoking', async () => {
  mocks.auth.user.role = 'admin'
  mocks.get.mockImplementation((url: string) => Promise.resolve({ data: { items: url.includes('/users/')
    ? [{ grant_id: 4, user_id: 2, badge_key: 'host', badge_name: 'Host', reason: '', granted_at: '2026-09-12T00:00:00Z' }]
    : [{ key: 'host', name: 'Host', description: '' }] } }))
  mocks.post.mockResolvedValue({ data: {} })
  const wrapper = mount(ZeroCityGovernancePanel, { props: { kind: 'badges' } })
  await flushPromises()
  await wrapper.get('input[type="number"]').setValue('99')
  await wrapper.get('input[maxlength="300"]').setValue('revoke reason')
  const button = wrapper.findAll('button').find(b => b.text() === '撤销徽章')!
  await button.trigger('click')
  await flushPromises()
  expect(mocks.post).toHaveBeenCalledWith('/admin/biz/community/badges/host/revoke', { user_id: 2, reason: 'revoke reason' })
})

it('shows load failure rather than claiming an empty rule registry', async () => {
  mocks.get.mockRejectedValue(new Error('offline'))
  const wrapper = mount(ZeroCityGovernancePanel, { props: { kind: 'rules' } })
  await flushPromises()
  expect(wrapper.find('[role="alert"]').exists()).toBe(true)
  expect(wrapper.text()).not.toContain('暂无已采纳规则')
})

it('requires an evidence reason and saves participation independently from badges', async () => {
  mocks.auth.user.role = 'admin'
  mocks.put.mockResolvedValue({ data: { level: 1 } })
  const wrapper = mount(ZeroCityGovernancePanel, { props: { kind: 'badges' } })
  await flushPromises()
  await wrapper.get('input[type="number"]').setValue('99')
  await wrapper.get('input[maxlength="300"]').setValue('Verified external project')
  await wrapper.get('.participation-controls').trigger('submit')
  await flushPromises()
  expect(mocks.put).toHaveBeenCalledWith('/admin/biz/community/users/99/participation', { level: 1, reason: 'Verified external project' })
  expect(mocks.post).not.toHaveBeenCalled()
  expect(wrapper.text()).toContain('参与资格已更新')
})
