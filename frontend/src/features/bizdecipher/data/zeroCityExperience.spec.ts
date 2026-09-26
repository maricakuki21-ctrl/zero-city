import { describe, expect, it } from 'vitest'
import { getZeroCitySurface, zeroCitySurfaceDefinitions } from './zeroCityExperience'

describe('zero city surface definitions', () => {
  it('assigns a distinct task surface to every community district', () => {
    expect(zeroCitySurfaceDefinitions.map((item) => [item.workspace, item.kind])).toEqual([
      ['tavern', 'plaza'],
      ['workshop', 'workshop'],
      ['market', 'collaboration'],
      ['governance', 'activity'],
    ])
  })

  it('does not invent a personal qualification surface', () => {
    expect(getZeroCitySurface('mine')).toBeUndefined()
  })
})
