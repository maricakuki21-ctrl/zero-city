import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { reactive } from 'vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import ZeroCityPollHall from '../ZeroCityPollHall.vue'
import type { CommunityPoll } from '@/features/bizdecipher/api/community'

const mocks = vi.hoisted(() => ({
  auth: { user: { id: 31 }, token: 'first-token', isAuthenticated: true, isAdmin: false },
  api: { listPolls: vi.fn(), createPoll: vi.fn(), votePoll: vi.fn(), closePoll: vi.fn() },
  participation: vi.fn(),
}))

const auth = reactive(mocks.auth)
enableAutoUnmount(afterEach)
vi.mock('@/stores/auth', () => ({ useAuthStore: () => auth }))
vi.mock('@/features/bizdecipher/api/community', () => ({ communityAPI: mocks.api, getCommunityParticipation: mocks.participation }))

const poll = (overrides: Partial<CommunityPoll> = {}): CommunityPoll => ({
  id: 9, post_id: 101, owner_user_id: 31, author: '发起人', title: '先做哪个功能？', body: '请选择一个方向。',
  status: 'open', total_votes: 2, viewer_option_id: 0, can_close: true, closes_at: '', closed_at: '', created_at: '2026-09-10T01:00:00Z',
  options: [
    { id: 77, label: '搜索', position: 1, vote_count: 1 },
    { id: 78, label: '分享', position: 2, vote_count: 1 },
  ],
  ...overrides,
})

beforeEach(() => {
  auth.user = { id: 31 }; auth.token = 'first-token'; auth.isAuthenticated = true; auth.isAdmin = false
  for (const fn of Object.values(mocks.api)) fn.mockReset()
  mocks.api.listPolls.mockResolvedValue({ items: [poll()] })
  mocks.participation.mockReset().mockResolvedValue({ user_id: 31, level: 1, is_admin: false, reason: '贡献确认' })
})

function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (reason: Error) => void
  const promise = new Promise<T>((yes, no) => { resolve = yes; reject = no })
  return { promise, resolve, reject }
}

describe('ZeroCityPollHall', () => {
  it('filters proposal types and states and lets residents clear an empty filter', async () => {
    mocks.api.listPolls.mockResolvedValue({ items: [poll(), poll({ id: 10, title: '玩家公告样本', proposal_kind: 'announcement', status: 'closed' })] })
    const wrapper = mount(ZeroCityPollHall); await flushPromises()
    await wrapper.get('[aria-label="议题类型筛选"]').setValue('announcement')
    expect(wrapper.findAll('.city-poll-item')).toHaveLength(1)
    expect(wrapper.text()).toContain('玩家公告样本')
    await wrapper.get('[aria-label="议题状态筛选"]').setValue('open')
    expect(wrapper.findAll('.city-poll-item')).toHaveLength(0)
    await wrapper.get('.city-poll-filter-empty button').trigger('click')
    expect(wrapper.findAll('.city-poll-item')).toHaveLength(2)
  })
  it('allows L0 to see discussion but not create or cast formal votes', async () => {
    mocks.participation.mockResolvedValue({ user_id: 31, level: 0, is_admin: false, reason: '' })
    const wrapper = mount(ZeroCityPollHall); await flushPromises()
    expect(wrapper.find('[data-testid="poll-open-create"]').exists()).toBe(false)
    expect(wrapper.get('input[value="77"]').attributes('disabled')).toBeDefined()
    expect(wrapper.text()).toContain('参与讨论')
    await wrapper.get('[data-testid="poll-vote-9"]').trigger('submit')
    expect(mocks.api.votePoll).not.toHaveBeenCalled()
  })

  it('publishes an announcement proposal with fixed options and shows its policy', async () => {
    mocks.api.listPolls.mockResolvedValue({ items: [], announcement_policy: { minimum_votes: 3, support_percent: 60, voting_hours: 24, display_days: 7 } })
    mocks.api.createPoll.mockResolvedValue(poll({ proposal_kind: 'announcement', decision: 'voting' }))
    const wrapper = mount(ZeroCityPollHall); await flushPromises()
    await wrapper.get('[data-testid="poll-open-create"]').trigger('click')
    await wrapper.get('[data-testid="poll-kind"]').setValue('announcement')
    expect(wrapper.find('[data-testid="poll-option-input"]').exists()).toBe(false)
    expect(wrapper.text()).toContain('支持率 ≥ 60%')
    await wrapper.get('[data-testid="poll-title"]').setValue('玩家共建日')
    await wrapper.get('[data-testid="poll-body"]').setValue('来分享你的作品')
    await wrapper.get('[data-testid="poll-create-form"]').trigger('submit'); await flushPromises()
    expect(mocks.api.createPoll).toHaveBeenCalledWith({ title: '玩家共建日', body: '来分享你的作品', proposal_kind: 'announcement', options: ['支持发布', '暂不发布'] })
  })

  it('submits one selected option and preserves the server selection after refresh', async () => {
    mocks.api.votePoll.mockResolvedValue(poll({ viewer_option_id: 77, total_votes: 3, options: [
      { id: 77, label: '搜索', position: 1, vote_count: 2 }, { id: 78, label: '分享', position: 2, vote_count: 1 },
    ] }))
    mocks.api.listPolls.mockResolvedValueOnce({ items: [poll()] }).mockResolvedValueOnce({ items: [poll({ viewer_option_id: 77 })] })
    const wrapper = mount(ZeroCityPollHall); await flushPromises()

    await wrapper.get('input[value="77"]').setValue(); await wrapper.get('[data-testid="poll-vote-9"]').trigger('submit'); await flushPromises()

    expect(mocks.api.votePoll).toHaveBeenCalledWith(9, 77)
    expect(wrapper.get<HTMLInputElement>('input[value="77"]').element.checked).toBe(true)
    await wrapper.get('[aria-label="刷新投票"]').trigger('click'); await flushPromises()
    expect(wrapper.get<HTMLInputElement>('input[value="77"]').element.checked).toBe(true)
  })

  it('creates a poll with real options and lets only the owner close it', async () => {
    mocks.api.createPoll.mockResolvedValue(poll({ id: 10 }))
    mocks.api.closePoll.mockResolvedValue(poll({ status: 'closed', closed_at: '2026-09-10T02:00:00Z', can_close: false }))
    const wrapper = mount(ZeroCityPollHall); await flushPromises()

    await wrapper.get('[data-testid="poll-open-create"]').trigger('click')
    await wrapper.get('[data-testid="poll-title"]').setValue('真实投票')
    await wrapper.get('[data-testid="poll-body"]').setValue('请选择一个选项')
    const optionInputs = wrapper.findAll('[data-testid="poll-option-input"]')
    await optionInputs[0].setValue('方案 A'); await optionInputs[1].setValue('方案 B')
    await wrapper.get('[data-testid="poll-create-form"]').trigger('submit'); await flushPromises()
    expect(mocks.api.createPoll).toHaveBeenCalledWith(expect.objectContaining({ options: ['方案 A', '方案 B'] }))

    await wrapper.get('[data-testid="poll-close-9"]').trigger('click'); await flushPromises()
    expect(mocks.api.closePoll).toHaveBeenCalledWith(9)
  })

  it('keeps empty and API error states actionable', async () => {
    mocks.api.listPolls.mockResolvedValueOnce({ items: [] })
    const wrapper = mount(ZeroCityPollHall); await flushPromises()
    expect(wrapper.text()).toContain('暂无开放议题')

    mocks.api.listPolls.mockRejectedValueOnce(new Error('offline'))
    await wrapper.get('[aria-label="刷新投票"]').trigger('click'); await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toContain('投票暂时没有加载成功')
  })

  it('clears the previous viewer ballot and rejects a late list after switching accounts', async () => {
    mocks.api.listPolls.mockResolvedValueOnce({ items: [poll({ viewer_option_id: 77 })] })
    const wrapper = mount(ZeroCityPollHall); await flushPromises()
    const oldList = deferred<{ items: CommunityPoll[] }>()
    mocks.api.listPolls.mockReturnValueOnce(oldList.promise).mockResolvedValueOnce({ items: [poll({ can_close: false })] })
    await wrapper.get('[aria-label="刷新投票"]').trigger('click')
    auth.user = { id: 32 }
    await flushPromises()
    oldList.resolve({ items: [poll({ viewer_option_id: 77 })] })
    await flushPromises()
    expect(wrapper.get<HTMLInputElement>('input[value="77"]').element.checked).toBe(false)
    expect(wrapper.get('input[value="77"]').attributes('disabled')).toBeUndefined()
    expect(wrapper.find('[data-testid="poll-close-9"]').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('你的选择已记录')
  })

  it.each([
    ['create', 'success'], ['create', 'failure'],
    ['vote', 'success'], ['vote', 'failure'],
    ['close', 'success'], ['close', 'failure'],
  ])('ignores old %s %s and preserves the new account pending action', async (action, result) => {
    const oldWrite = deferred<CommunityPoll>()
    const newWrite = deferred<CommunityPoll>()
    const writer = action === 'create' ? mocks.api.createPoll : action === 'vote' ? mocks.api.votePoll : mocks.api.closePoll
    writer.mockReturnValueOnce(oldWrite.promise).mockReturnValueOnce(newWrite.promise)
    const wrapper = mount(ZeroCityPollHall); await flushPromises()
    const act = async () => {
      if (action === 'create') {
        await wrapper.get('[data-testid="poll-open-create"]').trigger('click')
        await wrapper.get('[data-testid="poll-title"]').setValue('当前议题')
        await wrapper.get('[data-testid="poll-body"]').setValue('选择方向')
        const inputs = wrapper.findAll('[data-testid="poll-option-input"]')
        await inputs[0]!.setValue('甲'); await inputs[1]!.setValue('乙')
        await wrapper.get('[data-testid="poll-create-form"]').trigger('submit')
      } else if (action === 'vote') {
        await wrapper.get('input[value="77"]').setValue()
        await wrapper.get('[data-testid="poll-vote-9"]').trigger('submit')
      } else await wrapper.get('[data-testid="poll-close-9"]').trigger('click')
    }
    await act()
    auth.user = { id: 32 }
    await flushPromises()
    await act()
    if (result === 'success') oldWrite.resolve(poll({ title: '旧账号回包' }))
    else oldWrite.reject(new Error('old account failure'))
    await flushPromises()
    expect(wrapper.text()).not.toContain('旧账号回包')
    expect(wrapper.text()).not.toContain('old account failure')
    expect(wrapper.find('.city-poll-notice').exists()).toBe(false)
    expect(wrapper.get('[aria-label="刷新投票"]').attributes('disabled')).toBeDefined()
    newWrite.resolve(poll({ title: '当前账号回包' }))
    await flushPromises()
    expect(wrapper.text()).toContain('当前账号回包')
    expect(wrapper.get('[aria-label="刷新投票"]').attributes('disabled')).toBeUndefined()
  })

  it('does not let a pending refresh replace a subsequently confirmed vote', async () => {
    const wrapper = mount(ZeroCityPollHall); await flushPromises()
    await wrapper.get('input[value="77"]').setValue()
    const oldList = deferred<{ items: CommunityPoll[] }>()
    mocks.api.listPolls.mockReturnValueOnce(oldList.promise)
    mocks.api.votePoll.mockResolvedValue(poll({ viewer_option_id: 77, total_votes: 3 }))
    const voteForm = wrapper.get('[data-testid="poll-vote-9"]')
    const refreshing = wrapper.get('[aria-label="刷新投票"]').trigger('click')
    voteForm.element.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }))
    await refreshing; await flushPromises()
    oldList.resolve({ items: [poll()] })
    await flushPromises()
    expect(wrapper.text()).toContain('你的选择已记录')
    expect(wrapper.text()).toContain('共 3 票')
    expect(wrapper.get('[aria-label="刷新投票"]').attributes('disabled')).toBeUndefined()
  })

  it('keeps a confirmed vote across normal token rotation', async () => {
    const wrapper = mount(ZeroCityPollHall); await flushPromises()
    const vote = deferred<CommunityPoll>()
    mocks.api.votePoll.mockReturnValueOnce(vote.promise)
    await wrapper.get('input[value="77"]').setValue()
    await wrapper.get('[data-testid="poll-vote-9"]').trigger('submit')
    auth.token = 'renewed-token'
    await flushPromises()
    expect(mocks.api.listPolls).toHaveBeenCalledTimes(1)
    vote.resolve(poll({ viewer_option_id: 77, total_votes: 3 }))
    await flushPromises()
    expect(wrapper.get<HTMLInputElement>('input[value="77"]').element.checked).toBe(true)
    expect(wrapper.text()).toContain('投票已记录')
  })
})
