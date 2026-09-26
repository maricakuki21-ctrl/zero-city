import { flushPromises, mount } from '@vue/test-utils'
import { reactive } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import TavernStageView from '../TavernStageView.vue'

const mocks = vi.hoisted(() => ({
  getSession: vi.fn(),
  listTurns: vi.fn(),
  appendTurn: vi.fn(),
  push: vi.fn(),
  showError: vi.fn(),
}))
const route = reactive({ query: { session: '12', room: '9' } })

vi.mock('vue-router', () => ({
  useRoute: () => route,
  useRouter: () => ({ push: mocks.push }),
}))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError: mocks.showError }) }))
vi.mock('@/features/bizdecipher/api/bizdecipher', () => ({
  getTavernRuntimeSession: mocks.getSession,
  listTavernRoomTurns: mocks.listTurns,
  appendTavernRoomTurn: mocks.appendTurn,
}))

function session(title = '夜班房间') {
  return { config: {
    room: { title, status: 'running', current_players: 1, max_players: 4 },
    participant: { role: 'owner' },
    script: { title: '雾线', summary: '寻找线索', difficulty: 'normal', estimated_minutes: 30 },
    package: { id: 31, version: 'v4', status: 'published', runtime_kind: 'declarative' },
    budget: { billing_mode: 'free', entry_credit_cost: 0, entry_balance_cost: 0, turn_budget: 10 },
    prompts: { npc_cards: [], host_brief: '', opening_prompt: '' },
    bridge: {
      provider: 'declarative',
      mode: 'declarative_no_remote_code',
      protocol_version: '2026-09-13.package.v1',
      sandbox_mode: 'no_remote_code',
      external_runtime_isolated: true,
    },
    gateway: { source: 'server', model_strategy: 'configured' },
  } }
}

function mountStage() {
  return mount(TavernStageView, { global: { stubs: {
    AppLayout: { template: '<div><slot /></div>' },
    Icon: true,
  } } })
}

describe('TavernStageView session boundaries', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    sessionStorage.clear()
    route.query = { session: '12', room: '9' }
    sessionStorage.setItem('zero-city:tavern-runtime:12', 'private-token')
    mocks.getSession.mockResolvedValue(session())
    mocks.listTurns.mockResolvedValue([])
    mocks.appendTurn.mockResolvedValue({
      id: 1,
      room_id: 9,
      author_user_id: 7,
      author_name: '测试房主',
      author_role: 'owner',
      turn_index: 1,
      kind: 'player',
      client_message_id: 'turn-1',
      body: '检查档案柜',
      created_at: '2026-09-12T00:00:00Z',
    })
  })

  it('reads the private token and distinguishes configuration from realtime', async () => {
    const wrapper = mountStage()
    await flushPromises()
    expect(mocks.getSession).toHaveBeenCalledWith('private-token')
    expect(wrapper.text()).toContain('房间配置已读取 · 非实时连接')
    expect(wrapper.text()).not.toContain('运行桥接已准备')
    expect(wrapper.text()).not.toContain('private-token')
    expect(wrapper.text()).toContain('v4')
    expect(wrapper.text()).toContain('2026-09-13.package.v1')
    expect(wrapper.text()).toContain('no_remote_code')
    wrapper.unmount()
  })

  it('returns to my rooms while retaining the room reference', async () => {
    const wrapper = mountStage()
    await wrapper.get('.stage-secondary-button').trigger('click')
    expect(mocks.push).toHaveBeenCalledWith({
      name: 'ZeroCityTavern', query: { view: 'rooms', room: '9' },
    })
    wrapper.unmount()
  })

  it('does not call the API without a browser-session token', async () => {
    sessionStorage.clear()
    const wrapper = mountStage()
    await flushPromises()
    expect(mocks.getSession).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('舞台凭证不存在')
    expect(wrapper.get('.stage-primary-button').attributes('disabled')).toBeUndefined()
    wrapper.unmount()
  })

  it('does not display an old response after the route changes', async () => {
    let finishOld!: (value: ReturnType<typeof session>) => void
    mocks.getSession.mockImplementationOnce(() => new Promise(resolve => { finishOld = resolve }))
    const wrapper = mountStage()
    sessionStorage.setItem('zero-city:tavern-runtime:13', 'new-token')
    mocks.getSession.mockResolvedValueOnce(session('新房间'))
    route.query = { session: '13', room: '10' }
    await flushPromises()
    finishOld(session('旧房间'))
    await flushPromises()
    expect(wrapper.get('h1').text()).toBe('新房间')
    expect(wrapper.text()).not.toContain('旧房间')
    wrapper.unmount()
  })

  it('clears stale configuration and can retry a failed refresh', async () => {
    const wrapper = mountStage()
    await flushPromises()
    mocks.getSession.mockRejectedValueOnce(new Error('connection failed'))
    await wrapper.get('.stage-primary-button').trigger('click')
    await flushPromises()
    expect(wrapper.find('.stage-shell').exists()).toBe(false)
    expect(wrapper.text()).toContain('舞台配置加载失败')
    mocks.getSession.mockResolvedValueOnce(session('已恢复'))
    await wrapper.get('.stage-primary-button').trigger('click')
    await flushPromises()
    expect(wrapper.get('h1').text()).toBe('已恢复')
    wrapper.unmount()
  })

  it('loads persisted room turns and writes one idempotent client message', async () => {
    mocks.listTurns.mockResolvedValueOnce([{
      id: 4,
      room_id: 9,
      author_user_id: 7,
      author_name: '测试房主',
      author_role: 'owner',
      turn_index: 1,
      kind: 'player',
      client_message_id: 'turn-old',
      body: '先看窗外',
      created_at: '2026-09-11T23:59:00Z',
    }])
    const wrapper = mountStage()
    await flushPromises()
    expect(wrapper.text()).toContain('先看窗外')
    await wrapper.get('textarea[aria-label="房间回合内容"]').setValue('检查档案柜')
    await wrapper.get('.stage-turn-composer').trigger('submit')
    await flushPromises()
    expect(mocks.appendTurn).toHaveBeenCalledWith(9, expect.objectContaining({
      body: '检查档案柜',
      client_message_id: expect.any(String),
    }))
    expect(wrapper.text()).toContain('检查档案柜')
    expect((wrapper.get('textarea[aria-label="房间回合内容"]').element as HTMLTextAreaElement).value).toBe('')
    wrapper.unmount()
  })

  it('keeps ended room transcripts read-only', async () => {
    mocks.getSession.mockResolvedValue({
      config: {
        ...session().config,
        room: { ...session().config.room, status: 'completed' },
      },
    })
    mocks.listTurns.mockResolvedValueOnce([{
      id: 5,
      room_id: 9,
      author_user_id: 8,
      author_name: '玩家乙',
      author_role: 'player',
      turn_index: 2,
      kind: 'player',
      client_message_id: 'turn-2',
      body: '最终记录',
      created_at: '2026-09-12T00:10:00Z',
    }])
    const wrapper = mountStage()
    await flushPromises()
    expect(wrapper.text()).toContain('最终记录')
    expect(wrapper.find('.stage-turn-composer').exists()).toBe(false)
    expect(wrapper.text()).toContain('回合记录保持只读')
    wrapper.unmount()
  })
})
