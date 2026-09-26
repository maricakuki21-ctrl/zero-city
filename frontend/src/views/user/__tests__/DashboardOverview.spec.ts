import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import DashboardView from '../DashboardView.vue'

const mocks = vi.hoisted(() => ({
  listKeys: vi.fn(),
  listSeats: vi.fn(),
  auth: { user: { id: 41, balance: 12.5 } as { id: number; balance: number } | null },
}))

vi.mock('@/stores/auth', () => ({ useAuthStore: () => mocks.auth }))
vi.mock('@/api/keys', () => ({ list: mocks.listKeys }))
vi.mock('@/features/bizdecipher/api/bizdecipher', () => ({ listMySeats: mocks.listSeats }))
vi.mock('@/components/layout/AppLayout.vue', () => ({ default: { template: '<div><slot /></div>' } }))

const RouterLinkStub = {
  props: ['to'],
  template: '<a :data-to="JSON.stringify(to)"><slot /></a>',
}

function draft(overrides: Record<string, unknown> = {}) {
  return {
    id: 'task-1',
    title: '整理一个真实结果',
    intent: '整理一个真实结果',
    answer: '已经确认的内容',
    submitted: '整理一个真实结果',
    skillIds: [],
    languageKeyId: 1,
    languageModel: 'model',
    imageKeyId: 0,
    imageModel: '',
    updatedAt: '2026-09-12T08:00:00.000Z',
    resultState: 'verified' as const,
    ...overrides,
  }
}

function render() {
  return mount(DashboardView, {
    global: { stubs: { RouterLink: RouterLinkStub } },
  })
}

describe('Dashboard overview', () => {
  beforeEach(() => {
    localStorage.clear()
    vi.clearAllMocks()
    mocks.auth.user = { id: 41, balance: 12.5 }
    mocks.listKeys.mockResolvedValue({ total: 3 })
    mocks.listSeats.mockResolvedValue([{ status: 'active' }, { status: 'released' }])
  })

  it('loads only the current account drafts and separates unfinished work from verified results', async () => {
    localStorage.setItem('harness-drafts-v1-41', JSON.stringify([
      draft(),
      draft({ id: 'task-2', title: '尚未完成', answer: '', resultState: undefined }),
    ]))
    localStorage.setItem('harness-drafts-v1-42', JSON.stringify([draft({ id: 'other-user' })]))

    const wrapper = render()
    await flushPromises()

    expect(wrapper.text()).toContain('概览')
    expect(wrapper.get('.overview-summary').text()).toContain('待继续任务')
    expect(wrapper.get('.overview-summary').text()).toContain('1')
    expect(wrapper.get('.overview-summary').text()).toContain('已确认成果')
    expect(wrapper.text()).toContain('整理一个真实结果')
    expect(wrapper.text()).toContain('尚未完成')
    expect(wrapper.text()).not.toContain('other-user')
    expect(wrapper.text()).toContain('$12.50')
    expect(wrapper.text()).toContain('3')
    expect(wrapper.text()).toContain('1')
  })

  it('links each task back to the same workbench draft and the task list refreshes on storage changes', async () => {
    localStorage.setItem('harness-drafts-v1-41', JSON.stringify([draft()]))
    const wrapper = render()
    await flushPromises()

    const taskLink = wrapper.findAll('a').find(link => link.text() === '整理一个真实结果')
    expect(taskLink?.attributes('data-to')).toContain('/operator')
    expect(taskLink?.attributes('data-to')).toContain('task-1')

    window.dispatchEvent(new StorageEvent('storage', { key: 'harness-drafts-v1-41' }))
    await flushPromises()
    expect(wrapper.text()).toContain('整理一个真实结果')
  })

  it('does not render another account or an unconfirmed answer as a completed result', async () => {
    localStorage.setItem('harness-drafts-v1-41', JSON.stringify([
      draft({ id: 'legacy', title: '旧任务', resultState: undefined }),
      draft({ id: 'empty', title: '空结果', answer: '', resultState: 'verified' }),
    ]))
    const wrapper = render()
    await flushPromises()

    expect(wrapper.get('.overview-summary').text()).toContain('待继续任务')
    expect(wrapper.get('.overview-summary').text()).toContain('2')
    expect(wrapper.get('.overview-summary').text()).toContain('已确认成果')
    expect(wrapper.get('.overview-summary').text()).toContain('0')
    expect(wrapper.find('.overview-results').exists()).toBe(false)
  })
})
