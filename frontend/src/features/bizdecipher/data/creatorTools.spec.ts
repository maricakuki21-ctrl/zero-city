import { describe, expect, it } from 'vitest'
import { creatorToolResources, filterCreatorToolResources, resourceBillingTone } from './creatorTools'

describe('creator tool resource catalog', () => {
  it('filters by source and current workbench mode', () => {
    const resources = filterCreatorToolResources(creatorToolResources, '', 'creator', 'video')

    expect(resources.map((resource) => resource.id)).toEqual(['creator-storyboard-kit'])
  })

  it('searches across resource titles, summaries, and tags', () => {
    const resources = filterCreatorToolResources(creatorToolResources, '酒馆', 'all', 'general')

    expect(resources).toHaveLength(1)
    expect(resources[0]?.id).toBe('creator-tavern-game-kit')
  })

  it('keeps hybrid charging on the balance route until a settlement contract exists', () => {
    expect(resourceBillingTone('official')).toBe('official')
    expect(resourceBillingTone('points')).toBe('points')
    expect(resourceBillingTone('hybrid')).toBe('balance')
    expect(resourceBillingTone('free')).toBe('free')
  })
})
