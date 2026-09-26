import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { availableReward, type TokenGrant } from '../../api/tokenRewards'

const mocks = vi.hoisted(() => ({ get: vi.fn(), claim: vi.fn(), user: { id: 7 } }))
vi.mock('../../api/tokenRewards', async importOriginal => ({
  ...await importOriginal<object>(),
  tokenRewardsAPI: { get: mocks.get, claim: mocks.claim },
}))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({ user: mocks.user }) }))
import TokenPacketCard from '../TokenPacketCard.vue'

const grant: TokenGrant = { id: 1, packet_id: 4, resource_id: 'text', model: 'text-model', tokens: 100, used: 20, reserved: 30, expires_at: '2026-10-01T00:00:00Z' }
function packet(extra = {}) {
  return { id: 4, message_id: 30, sender_id: 1, resource_id: 'text', model: 'text-model', total_tokens: 6400, portions: 64, claimed: 0, mode: 'random', blessing: '今晚的灵感，我请了。', opens_at: '2026-09-25T00:00:00Z', closes_at: '2026-09-27T00:00:00Z', use_hours: 168, server_time: '2026-09-26T00:00:00Z', ...extra }
}
const options = { props: { packetId: 4 }, global: { stubs: { RouterLink: { template: '<a><slot /></a>' } } } }
describe('token reward cards', () => {
  beforeEach(() => { vi.useFakeTimers(); mocks.get.mockReset(); mocks.claim.mockReset(); mocks.user.id = 7 })
  afterEach(() => { vi.useRealTimers() })
  it('calculates rewards separately from cash and respects expiry and holds', () => {
    expect(availableReward(grant, Date.parse('2026-09-26'))).toBe(50)
    expect(availableReward(grant, Date.parse('2026-10-01'))).toBe(0)
  })
  it('claims once while busy and links the earned reward', async () => {
    mocks.get.mockResolvedValueOnce(packet()).mockResolvedValue(packet({ mine: grant, claimed: 1 }))
    mocks.claim.mockResolvedValue(grant)
    const wrapper = mount(TokenPacketCard, options)
    await flushPromises()
    await wrapper.get('.token-packet__button').trigger('click')
    await wrapper.get('.token-packet__button').trigger('click')
    await flushPromises()
    expect(mocks.claim).toHaveBeenCalledTimes(1)
    expect(wrapper.text()).toContain('你抢到')
    expect(wrapper.text()).toContain('用奖励开始创作')
    wrapper.unmount()
  })
  it.each([
    [packet({ claimed: 64 }), '已抢完'],
    [packet({ closes_at: '2026-09-25T00:00:00Z' }), '已过期'],
    [packet({ opens_at: '2026-09-26T00:01:00Z' }), '60 秒后开抢'],
    [packet({ sender_id: 7 }), '你发出的红包'],
  ])('does not allow claiming a non-claimable packet', async (value, label) => {
    mocks.get.mockResolvedValue(value)
    const wrapper = mount(TokenPacketCard, options)
    await flushPromises()
    expect(wrapper.get('.token-packet__button').text()).toBe(label)
    expect(wrapper.get('.token-packet__button').attributes('disabled')).toBeDefined()
    wrapper.unmount()
  })
  it('does not apply a response across accounts', async () => {
    let resolve!: (v: unknown) => void
    mocks.get.mockImplementation(() => new Promise(r => { resolve = r }))
    const wrapper = mount(TokenPacketCard, options)
    mocks.user.id = 8
    resolve(packet({ mine: grant }))
    await flushPromises()
    expect(wrapper.text()).not.toContain('你抢到')
    wrapper.unmount()
  })
})
