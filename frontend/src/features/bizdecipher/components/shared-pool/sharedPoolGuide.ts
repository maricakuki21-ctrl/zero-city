import type { DriveStep } from 'driver.js'

export type SharedPoolGuideScope = 'market' | 'owner'

export const SHARED_POOL_GUIDE_VERSION = 'v1'

type Translate = (key: string) => string

function step(
  t: Translate,
  key: string,
  element?: string,
  side: 'top' | 'right' | 'bottom' | 'left' = 'bottom',
): DriveStep {
  return {
    ...(element ? { element } : {}),
    popover: {
      title: t(`sharedPoolGuide.${key}.title`),
      description: t(`sharedPoolGuide.${key}.description`),
      side,
      align: element ? 'start' : 'center',
    },
  }
}

export function getSharedPoolGuideSteps(scope: SharedPoolGuideScope, t: Translate): DriveStep[] {
  if (scope === 'owner') {
    return [
      step(t, 'owner.welcome', '[data-tour="shared-pool-owner-header"]'),
      step(t, 'owner.create', '[data-tour="shared-pool-owner-create"]', 'left'),
      step(t, 'owner.earnings', '[data-tour="shared-pool-owner-wallet"]'),
      step(t, 'owner.console', '[data-tour="shared-pool-owner-pools"]', 'top'),
      step(t, 'owner.safety', '[data-tour="shared-pool-owner-header"]'),
    ]
  }

  return [
    step(t, 'market.welcome', '[data-tour="shared-pool-market-header"]'),
    step(t, 'market.browse', '[data-tour="shared-pool-market-browse"]', 'top'),
    step(t, 'market.member', '[data-tour="shared-pool-member-center"]', 'bottom'),
    step(t, 'market.owner', '[data-tour="shared-pool-owner-center"]', 'bottom'),
    step(t, 'market.pricing', '[data-tour="shared-pool-market-browse"]', 'top'),
    step(t, 'market.safety', '[data-tour="shared-pool-market-header"]'),
  ]
}

export function sharedPoolGuideStorageKey(
  scope: SharedPoolGuideScope,
  userId?: string | number | null,
): string {
  const identity = userId === undefined || userId === null || userId === ''
    ? 'device'
    : `user-${String(userId)}`
  return `shared_pool_guide_${identity}_${scope}_${SHARED_POOL_GUIDE_VERSION}`
}

export function hasSeenSharedPoolGuide(
  storage: Pick<Storage, 'getItem'>,
  scope: SharedPoolGuideScope,
  userId?: string | number | null,
): boolean {
  return storage.getItem(sharedPoolGuideStorageKey(scope, userId)) === 'seen'
}

export function markSharedPoolGuideSeen(
  storage: Pick<Storage, 'setItem'>,
  scope: SharedPoolGuideScope,
  userId?: string | number | null,
): void {
  storage.setItem(sharedPoolGuideStorageKey(scope, userId), 'seen')
}
