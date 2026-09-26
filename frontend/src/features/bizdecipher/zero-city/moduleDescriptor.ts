import type { CapabilityAsset } from '@/features/bizdecipher/api/bizdecipher'
import type { ZeroCityExecutionEntry, ZeroCityExecutionTarget, ZeroCityObjectKind } from './contracts'

export interface ZeroCityModuleDescriptor {
  readonly key: 'zero-city-bounded-journeys'
  readonly objectKinds: readonly ZeroCityObjectKind[]
  readonly routeWiring: readonly ZeroCityRouteWiring[]
}

export interface ZeroCityRouteWiring {
  readonly target: ZeroCityExecutionTarget
  readonly routeName: string
  readonly path: string
  readonly owner: 'task-17-workbench-relay-wiring'
  readonly requiredQuery: readonly ('asset' | 'entry')[]
}

export const zeroCityModuleDescriptor: ZeroCityModuleDescriptor = {
  key: 'zero-city-bounded-journeys',
  objectKinds: ['space', 'content', 'room', 'asset', 'action'],
  routeWiring: [
    {
      target: 'workbench',
      routeName: 'Operator',
      path: '/operator',
      owner: 'task-17-workbench-relay-wiring',
      requiredQuery: ['asset', 'entry'],
    },
  ],
}

function routeWiringFor(target: ZeroCityExecutionTarget): ZeroCityRouteWiring {
  const route = zeroCityModuleDescriptor.routeWiring.find((candidate) => candidate.target === target)
  if (!route) throw new Error(`Zero City route wiring missing for ${target}`)
  return route
}

export function executionEntryForAsset(asset: CapabilityAsset, target: ZeroCityExecutionTarget): ZeroCityExecutionEntry {
  const route = routeWiringFor(target)
  return {
    target,
    path: route.path,
    query: {
      asset: String(asset.id),
      entry: 'zero-city-asset',
    },
    activation: 'explicit-user-navigation',
  }
}
