import { afterEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import StudioWorkbenchView from '../StudioWorkbenchView.vue'
import { normalizeWorkbenchWorkspace, type CreatorWorkbenchService } from '../contracts'

vi.mock('@/components/layout/AppLayout.vue', () => ({ default: { template: '<div><slot /></div>' } }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({ user: { id: 901 } }) }))
vi.mock('vue-router', () => ({
  useRoute: () => ({ query: {} }),
  RouterLink: { props: ['to'], template: '<a :href="to"><slot /></a>' },
}))
const service = (): CreatorWorkbenchService => ({
  loadWorkspace: vi.fn(async () => normalizeWorkbenchWorkspace({
    workspace_token: 'test-token', workspace_version: 1,
    draft: { intent: 'Initial draft', capability_id: '', locale: 'zh-CN' },
    capabilities: [], saved_replays: [], current_run: null,
  })),
  createRun: vi.fn(), refreshRun: vi.fn(), cancelRun: vi.fn(),
  forkRun: vi.fn(), saveRun: vi.fn(), replayRun: vi.fn(), streamRun: vi.fn(),
})
afterEach(() => { localStorage.clear() })

describe('Studio workbench', () => {
  it('uses a bottom composer and does not invent available resources', async () => {
    const wrapper = mount(StudioWorkbenchView, { props: { service: service() } })
    await flushPromises()
    expect(wrapper.find('.composer-dock textarea').exists()).toBe(true)
    expect(wrapper.find('.studio-resources').exists()).toBe(false)
    expect(wrapper.find('[data-testid="workbench-launch"]').attributes('disabled')).toBeDefined()
    expect(wrapper.text()).not.toContain('林默')
    wrapper.unmount()
  })

  it('saves and restores a named local draft', async () => {
    const wrapper = mount(StudioWorkbenchView, { props: { service: service() } })
    await flushPromises()
    await wrapper.get('[aria-label="项目名称"]').setValue('海报项目')
    await wrapper.get('textarea').setValue('一张海报')
    await wrapper.get('[aria-label="保存本地项目"]').trigger('click')
    await wrapper.get('.new-session').trigger('click')
    expect(wrapper.get('textarea').element.value).toBe('')
    await wrapper.get('.project-row').trigger('click')
    expect(wrapper.get('textarea').element.value).toBe('一张海报')
    wrapper.unmount()
  })

  it('preserves the local prompt when reconnecting the workspace', async () => {
    const api = service()
    const wrapper = mount(StudioWorkbenchView, { props: { service: api } })
    await flushPromises()
    await wrapper.get('textarea').setValue('Unsaved local work')
    await wrapper.get('[aria-label="刷新工作区"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('textarea').element.value).toBe('Unsaved local work')
    expect(api.loadWorkspace).toHaveBeenCalledTimes(2)
    wrapper.unmount()
  })
})
