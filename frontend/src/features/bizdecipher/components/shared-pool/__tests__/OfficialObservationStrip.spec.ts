import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import OfficialObservationStrip from '../OfficialObservationStrip.vue'
import type { UserMonitorView } from '@/api/channelMonitor'

describe('OfficialObservationStrip', () => {
  it('excludes monitor errors rather than reporting vendor downtime', () => {
    const monitor = {
      primary_model: 'text-model',
      timeline: ['operational', 'degraded', 'failed', 'error'].map((status, i) => ({
        status, latency_ms: 150, ping_latency_ms: null, checked_at: `2026-09-23T12:0${i}:00Z`,
      })),
    } as UserMonitorView
    const wrapper = mount(OfficialObservationStrip, { props: { monitor } })
    expect(wrapper.text()).toContain('2/3 次通过 · 66.7%')
    expect(wrapper.text()).toContain('系统异常不计入通过率')
    expect(wrapper.findAll('.observation-bars .error')).toHaveLength(1)
  })
  it('does not fabricate success from no samples', () => {
    const wrapper = mount(OfficialObservationStrip)
    expect(wrapper.text()).toContain('暂无有效检测')
    expect(wrapper.text()).not.toContain('100%')
  })
})
