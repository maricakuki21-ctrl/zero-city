import { mount, flushPromises } from '@vue/test-utils'
import { beforeEach, expect, it, vi } from 'vitest'
import SharedPoolKeysPanel from '../SharedPoolKeysPanel.vue'
const { list, remove, copy } = vi.hoisted(() => ({ list: vi.fn(), remove: vi.fn(), copy: vi.fn() }))
vi.mock('@/features/bizdecipher/api/bizdecipher', () => ({ listMySharedPoolAccessKeys: list, deleteSharedPoolAccessKey: remove }))
vi.mock('@/composables/useClipboard', () => ({ useClipboard: () => ({ copyToClipboard: copy }) }))
const render = () => mount(SharedPoolKeysPanel, {
  props: { revision: 0, baseUrl: 'https://example.test/v1' },
  global: { stubs: {
    RouterLink: { template: '<a><slot /></a>' },
    ConfirmDialog: { props: ['show', 'message'], emits: ['confirm'], template: '<div v-if="show"><p>{{ message }}</p><button data-test="confirm-revoke" @click="$emit(\'confirm\')">confirm</button></div>' },
  } },
})
beforeEach(() => { list.mockReset(); remove.mockReset(); copy.mockReset() })
it('masks secrets and requires confirmation before revoking a multi-pool key', async () => {
  const key = { id: 1, pool_id: 17, pool_name: '池 A', api_key_id: 15, name: '共享 Key', key: 'sk-share-private-test', key_preview: 'sk-share-***', status: 'active' }
  list.mockResolvedValueOnce([key, { ...key, id: 2, pool_id: 18, pool_name: '池 B' }]).mockResolvedValueOnce([])
  remove.mockResolvedValue(undefined)
  const view = render()
  await flushPromises()
  expect(view.text()).not.toContain('sk-share-private-test')
  expect(view.findAll('.member-key-row')).toHaveLength(1)
  await view.get('button[aria-label="复制完整 Key"]').trigger('click')
  expect(copy).toHaveBeenCalledWith('sk-share-private-test')
  await view.get('button[aria-label="删除 Key"]').trigger('click')
  expect(remove).not.toHaveBeenCalled()
  expect(view.text()).toContain('绑定的所有共享池')
  await view.get('[data-test="confirm-revoke"]').trigger('click')
  await flushPromises()
  expect(remove).toHaveBeenCalledWith(15)
  expect(view.text()).toContain('还没有共享池密钥')
  view.unmount()
})
it('reports load failure rather than claiming there are no keys', async () => {
  list.mockRejectedValue(new Error('load failed'))
  const view = render()
  await flushPromises()
  expect(view.find('[role=alert]').exists()).toBe(true)
  expect(view.text()).not.toContain('还没有共享池密钥')
  view.unmount()
})
