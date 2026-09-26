import { effectScope, reactive, nextTick } from 'vue'
import { flushPromises } from '@vue/test-utils'
import { beforeEach, expect, it, vi } from 'vitest'
import { useCommunityParticipation } from '../useCommunityParticipation'

const mocks = vi.hoisted(() => ({ get: vi.fn(), auth: null as any }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => mocks.auth }))
vi.mock('@/features/bizdecipher/api/community', () => ({ getCommunityParticipation: mocks.get }))
beforeEach(() => {
  mocks.auth = reactive({ user: { id: 1 } })
  mocks.get.mockReset()
})
it('allows plaza but blocks qualified channels for L0', async () => {
  mocks.get.mockResolvedValue({ user_id: 1, level: 0, is_admin: false, reason: '' })
  const scope = effectScope()
  const state = scope.run(useCommunityParticipation)!
  await flushPromises()
  expect(state.canPost('tavern', 'chat-hall')).toBe(true)
  expect(state.canPost('workshop', 'help-desk')).toBe(false)
  expect(state.canPost('governance', 'rules')).toBe(false)
  scope.stop()
})
it('discards stale administrator access after account change', async () => {
  let finish!: (value: unknown) => void
  mocks.get.mockReturnValueOnce(new Promise(resolve => { finish = resolve }))
  const scope = effectScope()
  const state = scope.run(useCommunityParticipation)!
  mocks.get.mockResolvedValue({ user_id: 2, level: 0, is_admin: false, reason: '' })
  mocks.auth.user = { id: 2 }
  await nextTick()
  await flushPromises()
  finish({ user_id: 1, level: 1, is_admin: true, reason: '' })
  await flushPromises()
  expect(state.access.value?.user_id).toBe(2)
  expect(state.canPost('governance', 'rules')).toBe(false)
  scope.stop()
})
it('fails closed and exposes a retry on a read failure', async () => {
  mocks.get.mockRejectedValue(new Error('offline'))
  const scope = effectScope()
  const state = scope.run(useCommunityParticipation)!
  await flushPromises()
  expect(state.error.value).not.toBe('')
  expect(state.canPost('tavern', 'chat-hall')).toBe(false)
  mocks.get.mockResolvedValue({ user_id: 1, level: 1, is_admin: false, reason: '' })
  await state.refresh()
  expect(state.canPost('workshop', 'help-desk')).toBe(true)
  scope.stop()
})
