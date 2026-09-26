import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import TavernView from '../TavernView.vue'

const apiMocks = vi.hoisted(() => ({
  listTavernScripts: vi.fn(),
  listMyTavernScripts: vi.fn(),
  listMyTavernRooms: vi.fn(),
  listTavernGamePackages: vi.fn(),
  createTavernGamePackage: vi.fn(),
  publishTavernGamePackage: vi.fn(),
  revokeTavernGamePackage: vi.fn(),
  createTavernRoom: vi.fn(),
}))
const routerMocks = vi.hoisted(() => ({
  route: { query: { view: 'rooms' } as Record<string, string> },
  push: vi.fn(),
  replace: vi.fn(),
}))

vi.mock('vue-router', () => ({
  useRoute: () => routerMocks.route,
  useRouter: () => ({ push: routerMocks.push, replace: routerMocks.replace }),
}))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showSuccess: vi.fn(), showError: vi.fn() }) }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({ user: { id: 7, role: 'user' } }) }))
vi.mock('@/features/bizdecipher/api/bizdecipher', () => ({
  listTavernScripts: apiMocks.listTavernScripts,
  listMyTavernScripts: apiMocks.listMyTavernScripts,
  listMyTavernRooms: apiMocks.listMyTavernRooms,
  listTavernGamePackages: apiMocks.listTavernGamePackages,
  createTavernGamePackage: apiMocks.createTavernGamePackage,
  publishTavernGamePackage: apiMocks.publishTavernGamePackage,
  revokeTavernGamePackage: apiMocks.revokeTavernGamePackage,
  createTavernRoom: apiMocks.createTavernRoom,
  createTavernRuntimeSession: vi.fn(),
  createTavernScript: vi.fn(),
  openTavernRoom: vi.fn(),
  joinTavernRoom: vi.fn(),
  startTavernRoom: vi.fn(),
  completeTavernRoom: vi.fn(),
  cancelTavernRoom: vi.fn(),
}))

function mountTavern() {
  return mount(TavernView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        StarterStories: { template: '<div data-test="local-stories">本地故事</div>' },
        Icon: true,
      },
    },
  })
}

describe('TavernView route views', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    routerMocks.route.query = { view: 'rooms' }
    apiMocks.listTavernScripts.mockResolvedValue([])
    apiMocks.listMyTavernScripts.mockResolvedValue([])
    apiMocks.listMyTavernRooms.mockResolvedValue([{
      id: 9,
      script_id: 3,
      owner_id: 7,
      owner: '我',
      script_title: '雾线末班车',
      package_version: 'v2',
      title: '周五夜班',
      status: 'lobby',
      visibility: 'private',
      host_mode: 'human_host',
      billing_mode: 'free',
      entry_credit_cost: 0,
      entry_balance_cost: 0,
      max_players: 4,
      current_players: 1,
      current_user_joined: true,
      current_phase: 'lobby',
      room_config: {},
      created_at: '2026-09-09T00:00:00Z',
      updated_at: '2026-09-09T00:00:00Z',
    }])
    apiMocks.listTavernGamePackages.mockResolvedValue([])
    apiMocks.createTavernGamePackage.mockResolvedValue({})
    apiMocks.publishTavernGamePackage.mockResolvedValue({})
    apiMocks.revokeTavernGamePackage.mockResolvedValue({})
    apiMocks.createTavernRoom.mockResolvedValue({})
  })

  it('opens my server rooms directly from view=rooms', async () => {
    const wrapper = mountTavern()
    await flushPromises()

    expect(wrapper.get('h1').text()).toBe('我的房间')
    expect(wrapper.text()).toContain('周五夜班')
    expect(wrapper.text()).toContain('雾线末班车')
    expect(wrapper.text()).toContain('v2')
    expect(wrapper.find('[data-test="local-stories"]').exists()).toBe(false)
    expect(apiMocks.listMyTavernRooms).toHaveBeenCalled()
  })

  it('writes the rooms view into the URL when the rooms tab is selected', async () => {
    routerMocks.route.query = {}
    const wrapper = mountTavern()
    await flushPromises()
    const roomsTab = wrapper.findAll('.tavern-tab').find(button => button.text() === '我的房间')
    expect(roomsTab).toBeTruthy()
    await roomsTab?.trigger('click')

    expect(routerMocks.replace).toHaveBeenCalledWith({ path: '/tavern', query: { view: 'rooms' } })
  })

  it('routes the empty-room action back to the catalog URL', async () => {
    apiMocks.listMyTavernRooms.mockResolvedValue([])
    const wrapper = mountTavern()
    await flushPromises()
    const catalogButton = wrapper.findAll('button').find(button => button.text() === '去剧本大厅开房')
    expect(catalogButton).toBeTruthy()
    await catalogButton?.trigger('click')
    await flushPromises()

    expect(routerMocks.replace).toHaveBeenCalledWith({ path: '/tavern', query: {} })
  })

  it('routes to the rooms URL after a server room is created', async () => {
    routerMocks.route.query = {}
    apiMocks.listTavernScripts.mockResolvedValue([{
      id: 3,
      title: '雾线末班车',
      summary: '一段公开剧本',
      genre: 'mystery',
      difficulty: 'normal',
      pricing_mode: 'free',
      entry_credit_cost: 0,
      entry_balance_cost: 0,
      player_min: 1,
      player_max: 4,
      estimated_minutes: 60,
      tags: [],
      quality_score: 8,
    }])
    const wrapper = mountTavern()
    await flushPromises()
    await wrapper.get('.tavern-script-card').trigger('click')
    await wrapper.get('.tavern-control-panel input[type="text"]').setValue('周五夜班')
    await wrapper.get('.tavern-control-panel form').trigger('submit')
    await flushPromises()

    expect(apiMocks.createTavernRoom).toHaveBeenCalled()
    expect(routerMocks.replace).toHaveBeenLastCalledWith({ path: '/tavern', query: { view: 'rooms' } })
  })

  it('lets a creator create and publish a declarative package version', async () => {
    routerMocks.route.query = { view: 'scripts' }
    apiMocks.listMyTavernScripts.mockResolvedValue([{
      id: 3,
      user_id: 7,
      title: '雾线末班车',
      slug: 'misty-line',
      summary: '一段自有剧本',
      description: '剧本说明',
      genre: 'mystery',
      status: 'draft',
      visibility: 'private',
      tags: [],
      npc_cards: [],
      host_brief: '主持提示',
      opening_prompt: '开场提示',
      safety_notes: '安全边界',
      created_at: '2026-09-09T00:00:00Z',
    }])
    apiMocks.listTavernGamePackages.mockResolvedValue([])
    const wrapper = mountTavern()
    await flushPromises()

    const packageButton = wrapper.findAll('button').find(button => button.text() === '游戏包版本')
    expect(packageButton).toBeTruthy()
    await packageButton?.trigger('click')
    await flushPromises()
    await wrapper.get('.tavern-package-form input[type="text"]').setValue('v2')
    await wrapper.get('.tavern-package-form').trigger('submit')
    await flushPromises()

    expect(apiMocks.createTavernGamePackage).toHaveBeenCalledWith(3, expect.objectContaining({
      version: 'v2',
      manifest: expect.objectContaining({
        runtime_kind: 'declarative',
        protocol_version: '2026-09-13.package.v1',
      }),
    }))
  })
})
