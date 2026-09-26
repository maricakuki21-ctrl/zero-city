import { describe, expect, it } from 'vitest'
import { canAccessProductModule, createProductModuleState, flattenProductModules, isProductModuleVisibleInSidebar, isValidProductModuleDefinition, productModuleRegistry, resolveProductModule, validateProductModuleRegistry } from './moduleRegistry'
describe('product module registry', () => {
  it('resolves canonical paths and flattens child modules', () => {
    expect(resolveProductModule('/account-square/my')?.key).toBe('sharedMarket.member')
    expect(flattenProductModules()).toEqual(expect.arrayContaining([expect.objectContaining({ key: 'gateway' })]))
  })
  it('rejects duplicates and malformed mixed registries', () => {
    expect(validateProductModuleRegistry({ ...productModuleRegistry, bad: { key: 'bad' } })).toBe(false)
    expect(validateProductModuleRegistry({ ...productModuleRegistry, dup: { ...productModuleRegistry.gateway, key: 'dup' } })).toBe(false)
  })
  it('covers runtime state primitives', () => {
    expect(createProductModuleState('gateway')).toEqual({ key: 'gateway', state: 'idle', stale: false })
    expect(createProductModuleState('gateway', 'loading', true).stale).toBe(true)
    expect(createProductModuleState('gateway', 'ready').state).toBe('ready')
    expect(createProductModuleState('gateway', 'error').state).toBe('error')
  })
  it('fails closed for malformed entries while exposing the beta market surfaces', () => {
    expect(isValidProductModuleDefinition({ key: 'x' })).toBe(false)
    expect(canAccessProductModule(productModuleRegistry.capabilityAssets.children[2], { isAuthenticated: true, isAdmin: true })).toBe(true)
    expect(canAccessProductModule(productModuleRegistry.capabilityAssets.children[3], { isAuthenticated: true, isAdmin: true })).toBe(true)
    expect(canAccessProductModule(productModuleRegistry.gateway, { isAuthenticated: false, isAdmin: false })).toBe(false)
  })

  it('fails closed for disabled modules while keeping the approved beta surfaces reachable', () => {
    const disabled = {
      ...productModuleRegistry.gateway,
      key: 'disabled',
      status: 'disabled' as const,
      showInSidebar: true,
    }

    expect(isValidProductModuleDefinition(disabled)).toBe(true)
    expect(canAccessProductModule(disabled, { isAuthenticated: true, isAdmin: true })).toBe(false)
    expect(isProductModuleVisibleInSidebar(disabled)).toBe(false)
    expect(productModuleRegistry.capabilityAssets.children[1].status).toBe('beta')
    expect(isProductModuleVisibleInSidebar(productModuleRegistry.capabilityAssets.children[1])).toBe(true)
    expect(isProductModuleVisibleInSidebar(productModuleRegistry.capabilityAssets.children[2])).toBe(true)
    expect(isProductModuleVisibleInSidebar(productModuleRegistry.capabilityAssets.children[3])).toBe(true)
  })

  it('resolves nested routes from the longest canonical registry prefix', () => {
    expect(resolveProductModule('/community/some-profile')?.key).toBe('zeroCity')
    expect(resolveProductModule('/account-square/owner/pools/42')?.key).toBe('sharedMarket.owner')
  })
})

