import { describe, expect, it } from 'vitest'
import {
  isZeroCityModuleActive,
  zeroCityModuleDefinitions,
  zeroCityWorkspaceKeys,
} from './zeroCityModules'

describe('zero city module definitions', () => {
  it('keeps one ordered top-level entry for each product module', () => {
    expect(zeroCityModuleDefinitions.map((module) => module.key)).toEqual([
      'home',
      'tavern',
      'forum',
      'market',
      'mine',
    ])
  })

  it('assigns every workspace to exactly one top-level module', () => {
    for (const workspace of zeroCityWorkspaceKeys) {
      const owners = zeroCityModuleDefinitions.filter((module) =>
        isZeroCityModuleActive(module, workspace),
      )
      expect(owners, workspace).toHaveLength(1)
    }
  })

  it('keeps each navigation entry inside its active workspace set', () => {
    for (const module of zeroCityModuleDefinitions) {
      expect(module.workspaces).toContain(module.entryWorkspace)
    }
  })

  it('keeps formal collaboration outside the community workflow', () => {
    const market = zeroCityModuleDefinitions.find((module) => module.key === 'market')

    expect(market?.workflowOwnership).toBe('deep-link-only')
    expect(
      zeroCityModuleDefinitions
        .filter((module) => module.key !== 'market')
        .every((module) => module.workflowOwnership === 'community-only'),
    ).toBe(true)
  })

  it('defines a return and sharing reason for every module', () => {
    for (const module of zeroCityModuleDefinitions) {
      expect(module.purpose.length).toBeGreaterThan(0)
      expect(module.scene.length).toBeGreaterThan(0)
      expect(module.primaryObject.length).toBeGreaterThan(0)
      expect(module.primaryAction.length).toBeGreaterThan(0)
      expect(module.returnTrigger.length).toBeGreaterThan(0)
      expect(module.shareTrigger.length).toBeGreaterThan(0)
      expect(module.boundary.length).toBeGreaterThan(0)
      expect(module.secondarySurfaces.length).toBeGreaterThan(0)
    }
  })
})
