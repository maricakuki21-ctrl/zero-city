import { reactive } from 'vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import TavernPackageImport from './TavernPackageImport.vue'

const mocks = vi.hoisted(() => ({ create: vi.fn(), auth: { state: undefined as unknown as { user: { id: number } } } }))
vi.mock('@/features/bizdecipher/api/bizdecipher', () => ({ createTavernGamePackage: mocks.create }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => mocks.auth.state }))
const payload = { version: '2.0', manifest: { schema_version: 'tavern.package.v1', runtime_kind: 'declarative', permissions: { purchases: false }, content: { opening_prompt: '开场' } } }
let wrapper: VueWrapper
async function choose(text: string, size = 100) {
  Object.defineProperty(wrapper.get('input').element, 'files', { configurable: true, value: [{ name: 'game.json', size, text: async () => text }] })
  await wrapper.get('input').trigger('change')
  await flushPromises()
}
beforeEach(() => {
  mocks.create.mockReset()
  mocks.auth.state = reactive({ user: { id: 7 } })
  wrapper = mount(TavernPackageImport, { props: { scriptId: 11 } })
})
afterEach(() => wrapper.unmount())
describe('local game package import', () => {
  it('only uploads after explicit confirmation, preserving manifest content', async () => {
    await choose(JSON.stringify(payload))
    expect(mocks.create).not.toHaveBeenCalled()
    mocks.create.mockResolvedValue({ version: '2.0' })
    await wrapper.get('.preview button').trigger('click')
    await flushPromises()
    expect(mocks.create).toHaveBeenCalledWith(11, payload)
    expect(wrapper.text()).toContain('版本 2.0 已保存为草稿')
    expect(wrapper.emitted('imported')).toHaveLength(1)
  })
  it('rejects oversized and invalid input without invoking the API', async () => {
    await choose('{}', 280 * 1024)
    expect(wrapper.get('[role="alert"]').text()).toContain('270 KB')
    await choose('{broken')
    expect(wrapper.get('[role="alert"]').text()).toContain('有效的 JSON')
    await choose(JSON.stringify({ ...payload, manifest: { ...payload.manifest, runtime_kind: 'javascript' } }))
    expect(wrapper.get('[role="alert"]').text()).toContain('声明式')
    expect(mocks.create).not.toHaveBeenCalled()
  })
  it('keeps the selected file after a server rejection', async () => {
    await choose(JSON.stringify(payload))
    mocks.create.mockRejectedValue(new Error('版本重复'))
    await wrapper.get('.preview button').trigger('click')
    await flushPromises()
    expect(wrapper.find('.preview').exists()).toBe(true)
    expect(wrapper.find('[role="alert"]').exists()).toBe(true)
    expect(wrapper.emitted('imported')).toBeUndefined()
  })
  it('resets the file when the authenticated account changes', async () => {
    await choose(JSON.stringify(payload))
    mocks.auth.state.user.id = 8
    await flushPromises()
    expect(wrapper.find('.preview').exists()).toBe(false)
  })
})
