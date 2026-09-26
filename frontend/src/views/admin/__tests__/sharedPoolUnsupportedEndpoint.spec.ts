import { describe, expect, it } from 'vitest'
import { sharedPoolEndpointAvailability, sharedPoolEndpointHasCompatiblePrice } from '@/features/bizdecipher/components/shared-pool/sharedPoolPricing'

describe('shared pool unsupported endpoint', () => {
  it.each(['embeddings', 'audio', 'realtime', 'provider-specific'])('does not claim %s is callable merely because it has a token price', endpoint => {
    const evidence = { endpoint_type: endpoint, enabled: true, gate_status: 'passed', endpoint_pricing_status: 'ready', current_price: { base_price: { billing_mode: 'token' } } }
    expect(sharedPoolEndpointAvailability(evidence)).toMatchObject({ callable: false, code: 'unsupported' })
    expect(sharedPoolEndpointHasCompatiblePrice(evidence)).toBe(false)
  })
})
