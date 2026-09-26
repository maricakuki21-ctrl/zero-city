import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import OpsSettingsDialog from '../OpsSettingsDialog.vue'

const api = vi.hoisted(() => ({
  getAlertRuntimeSettings: vi.fn(),
  getEmailNotificationConfig: vi.fn(),
  getAdvancedSettings: vi.fn(),
  getMetricThresholds: vi.fn(),
  updateAlertRuntimeSettings: vi.fn(),
  updateEmailNotificationConfig: vi.fn(),
  updateAdvancedSettings: vi.fn(),
  updateMetricThresholds: vi.fn(),
}))
vi.mock('@/api/admin/ops', () => ({ opsAPI: api }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError: vi.fn(), showSuccess: vi.fn() }) }))

beforeEach(() => {
  vi.clearAllMocks()
  api.getAlertRuntimeSettings.mockResolvedValue({ evaluation_interval_seconds: 30 })
  api.getEmailNotificationConfig.mockResolvedValue({
    alert: { enabled: false, recipients: [] }, report: { enabled: false, recipients: [] },
  })
  api.getAdvancedSettings.mockResolvedValue({
    data_retention: { error_log_retention_days: 30, minute_metrics_retention_days: 30, hourly_metrics_retention_days: 30 },
    aggregation: {}, openai_account_quota_auto_pause: { default_threshold_5h: .9, default_threshold_7d: .8 },
  })
  api.getMetricThresholds.mockResolvedValue({})
})

async function openDialog() {
  const wrapper = mount(OpsSettingsDialog, {
    props: { show: false },
    global: {
      stubs: {
        BaseDialog: { template: '<div><slot /><slot name="footer" /></div>' },
        Select: true, Toggle: true,
      },
    },
  })
  await wrapper.setProps({ show: true })
  await flushPromises()
  return wrapper
}

describe('platform thresholds in existing settings', () => {
  it('defaults legacy responses to disabled and saves independent platform values', async () => {
    const wrapper = await openDialog()
    expect((wrapper.get('[data-testid="ops-anthropic-threshold"]').element as HTMLInputElement).value).toBe('100')
    await wrapper.get('[data-testid="ops-anthropic-threshold"]').setValue('65')
    await wrapper.get('[data-testid="ops-grok-threshold"]').setValue('75')
    await wrapper.get('button.btn-primary').trigger('click')
    await flushPromises()
    expect(api.updateAdvancedSettings).toHaveBeenCalledWith(expect.objectContaining({
      openai_account_quota_auto_pause: {
        default_threshold_5h: .9, default_threshold_7d: .8, anthropic_threshold: 65, grok_threshold: 75,
      },
    }))
    expect(wrapper.emitted('saved')).toHaveLength(1)
    wrapper.unmount()
  })

  it.each(['0', '101', '65.5', ''])('rejects invalid threshold %s before saving', async value => {
    const wrapper = await openDialog()
    await wrapper.get('[data-testid="ops-grok-threshold"]').setValue(value)
    expect(wrapper.get('button.btn-primary').attributes('disabled')).toBeDefined()
    expect(api.updateAdvancedSettings).not.toHaveBeenCalled()
    wrapper.unmount()
  })
})
