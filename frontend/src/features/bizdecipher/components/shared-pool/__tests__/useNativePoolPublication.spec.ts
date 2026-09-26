import { effectScope, ref } from 'vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { SharedPool } from '@/features/bizdecipher/api/bizdecipher'

const mocks = vi.hoisted(() => ({
  activateSharedPoolNativeBilling: vi.fn(),
  createSharedPoolNativeOperationID: vi.fn(),
  listMySharedPools: vi.fn(),
}))
vi.mock('@/features/bizdecipher/api/bizdecipher', () => mocks)
import { useNativePoolPublication } from '../useNativePoolPublication'

const draft = { id: 42, config_version: 1, listed: false, native_onboarding_state: 'supply_ready_billing_blocked' } as SharedPool
const activation = { pool_id: 42, state: 'billing_active', config_version: 2, activated_config_version: 2, already_active: false }
const published = { ...draft, config_version: 2, listed: true, native_onboarding_state: 'billing_active' } as SharedPool
const scopes: ReturnType<typeof effectScope>[] = []
function setup(value = draft) {
  const pool = ref<SharedPool | null>({ ...value })
  const scope = effectScope()
  scopes.push(scope)
  return { pool, ...scope.run(() => useNativePoolPublication(pool))! }
}
beforeEach(() => {
  vi.resetAllMocks()
  sessionStorage.clear()
  mocks.createSharedPoolNativeOperationID.mockReturnValue('publish-operation-1')
  mocks.activateSharedPoolNativeBilling.mockResolvedValue(activation)
  mocks.listMySharedPools.mockResolvedValue([published])
})
afterEach(() => { scopes.splice(0).forEach(scope => scope.stop()) })

describe('native publication recovery', () => {
  it('cannot publish a manually paused pool', async () => {
    const control = setup({ ...draft, owner_paused: true })
    expect(control.canPublish.value).toBe(false)
    await control.publish()
    expect(mocks.activateSharedPoolNativeBilling).not.toHaveBeenCalled()
  })
  it('uses the versioned API, refreshes authoritative state, and clears the saved operation', async () => {
    const control = setup()
    await control.publish()
    expect(mocks.activateSharedPoolNativeBilling).toHaveBeenCalledWith(42, { operation_id: 'publish-operation-1', expected_config_version: 1 })
    expect(control.pool.value?.listed).toBe(true)
    expect(control.state.value).toBe('success')
    expect(control.canPublish.value).toBe(false)
    expect(sessionStorage.getItem('shared-pool-native-publication:42')).toBeNull()
  })
  it('keeps the original payload after a lost response, reload, and changed pool version', async () => {
    mocks.activateSharedPoolNativeBilling.mockRejectedValueOnce({ status: 0 })
    const first = setup()
    await first.publish()
    expect(first.state.value).toBe('uncertain')
    const recovered = setup(published)
    expect(recovered.unresolved.value).toBe(true)
    await recovered.publish()
    expect(mocks.activateSharedPoolNativeBilling.mock.calls[1]).toEqual(mocks.activateSharedPoolNativeBilling.mock.calls[0])
    expect(mocks.createSharedPoolNativeOperationID).toHaveBeenCalledTimes(1)
    expect(recovered.state.value).toBe('success')
  })
  it('never reports a confirmed activation as a failed write when refresh fails', async () => {
    mocks.listMySharedPools.mockRejectedValue(new Error('offline'))
    const control = setup()
    await control.publish()
    expect(control.state.value).toBe('success')
    expect(control.message.value).toContain('计费开通已确认')
    expect(control.message.value).toContain('刷新失败')
    expect(control.pool.value?.listed).toBe(false)
    expect(control.canPublish.value).toBe(false)
  })
  it('requires an explicit new submit after a configuration conflict', async () => {
    mocks.activateSharedPoolNativeBilling.mockRejectedValueOnce({ status: 409 })
    mocks.listMySharedPools.mockResolvedValue([{ ...draft, config_version: 3 }])
    const control = setup()
    await control.publish()
    expect(control.state.value).toBe('error')
    expect(control.pool.value?.config_version).toBe(3)
    expect(mocks.activateSharedPoolNativeBilling).toHaveBeenCalledTimes(1)
    expect(control.unresolved.value).toBe(false)
    expect(control.canPublish.value).toBe(true)
  })
  it('blocks stale resubmission when a conflict cannot refresh', async () => {
    mocks.activateSharedPoolNativeBilling.mockRejectedValueOnce({ status: 409 })
    mocks.listMySharedPools.mockRejectedValue(new Error('offline'))
    const control = setup()
    await control.publish()
    expect(control.canPublish.value).toBe(false)
    await control.publish()
    expect(mocks.activateSharedPoolNativeBilling).toHaveBeenCalledTimes(1)
  })
  it.each([422, 403, 404])('shows safe feedback for a definitive %i refusal', async status => {
    mocks.activateSharedPoolNativeBilling.mockRejectedValue({ status, message: 'sk-SECRET upstream-token' })
    const control = setup()
    await control.publish()
    expect(control.state.value).toBe('error')
    expect(control.message.value).not.toContain('SECRET')
    expect(control.unresolved.value).toBe(false)
    expect(control.pool.value?.listed).toBe(false)
  })
  it('does not treat malformed or wrong-pool responses as success', async () => {
    mocks.activateSharedPoolNativeBilling.mockResolvedValue({ ...activation, pool_id: 43 })
    const control = setup()
    await control.publish()
    expect(control.state.value).toBe('uncertain')
    expect(control.pool.value?.listed).toBe(false)
    expect(control.unresolved.value).toBe(true)
  })
  it('does not publish unknown states or accept duplicate clicks', async () => {
    expect(setup({ ...draft, native_onboarding_state: 'future-state' } as SharedPool).canPublish.value).toBe(false)
    let finish!: (value: typeof activation) => void
    mocks.activateSharedPoolNativeBilling.mockReturnValue(new Promise(resolve => { finish = resolve }))
    const control = setup()
    const first = control.publish()
    await control.publish()
    expect(mocks.activateSharedPoolNativeBilling).toHaveBeenCalledTimes(1)
    finish(activation)
    await first
  })
})
