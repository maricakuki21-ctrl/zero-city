import { describe, expect, it, vi } from 'vitest'

const apiClient = vi.hoisted(() => ({ post: vi.fn(), patch: vi.fn() }))

vi.mock('@/api/client', () => ({ apiClient }))

import {
  activateSharedPoolNativeBilling,
  createNativeSharedPoolAccount,
  createNativeSharedPoolDraft,
  repairSharedPoolNativeAccount,
  verifySharedPoolNativeReadiness,
} from '../bizdecipher'

describe('shared pool native onboarding API', () => {
  it('activates only with the requested version and matching idempotency header', async () => {
    const result = { pool_id: 314, state: 'billing_active', config_version: 4 }
    apiClient.post.mockResolvedValue({ data: result })
    expect(await activateSharedPoolNativeBilling(314, {
      operation_id: 'publish-1', expected_config_version: 3,
    })).toBe(result)
    expect(apiClient.post).toHaveBeenCalledWith('/biz/pools/314/native-activation', {
      operation_id: 'publish-1', expected_config_version: 3,
    }, { headers: { 'Idempotency-Key': 'publish-1' } })
  })
  it('sends a name-only native draft with one idempotency operation', async () => {
    apiClient.post.mockResolvedValue({ data: { id: 314 } })

    await createNativeSharedPoolDraft({ name: 'R1 native pool', description: 'recoverable', operation_id: 'operation-1' })

    expect(apiClient.post).toHaveBeenCalledWith('/biz/pools', {
      name: 'R1 native pool', description: 'recoverable', supply_mode: 'native', operation_id: 'operation-1',
    }, { headers: { 'Idempotency-Key': 'operation-1' } })
  })

  it('submits native credentials without browser-supplied binding or scheduling fields', async () => {
    apiClient.post.mockResolvedValue({ data: { id: 7 } })

    await createNativeSharedPoolAccount(314, {
      operation_id: 'account-operation-1', name: 'Primary', provider: 'openai', auth_type: 'api_key',
      upstream_base_url: 'https://api.example.test/v1', upstream_api_key: 'sk-secret',
    })

    expect(apiClient.post).toHaveBeenCalledWith('/biz/pools/314/accounts', {
      operation_id: 'account-operation-1', name: 'Primary', provider: 'openai', auth_type: 'api_key',
      upstream_base_url: 'https://api.example.test/v1', upstream_api_key: 'sk-secret',
    }, { headers: { 'Idempotency-Key': 'account-operation-1' } })
  })

  it('repairs a server-selected native account with an independent idempotency operation', async () => {
    apiClient.patch.mockResolvedValue({ data: { id: 7 } })

    await repairSharedPoolNativeAccount(314, 7, {
      repair_operation_id: 'repair-operation-1',
      expected_config_version: 3,
      provider: 'openai',
      auth_type: 'api_key',
      upstream_base_url: 'https://api.example.test/v1',
      upstream_api_key: 'sk-repaired',
    })

    expect(apiClient.patch).toHaveBeenCalledWith('/biz/pools/314/accounts/7', {
      repair_operation_id: 'repair-operation-1',
      expected_config_version: 3,
      provider: 'openai',
      auth_type: 'api_key',
      upstream_base_url: 'https://api.example.test/v1',
      upstream_api_key: 'sk-repaired',
    }, { headers: { 'Idempotency-Key': 'repair-operation-1' } })
  })

  it('starts native readiness only through its versioned endpoint', async () => {
    apiClient.post.mockResolvedValue({ data: { id: 7 } })

    await verifySharedPoolNativeReadiness(314, 7, { expected_config_version: 3 })

    expect(apiClient.post).toHaveBeenCalledWith('/biz/pools/314/accounts/7/native-readiness', {
      expected_config_version: 3,
    })
  })
})
