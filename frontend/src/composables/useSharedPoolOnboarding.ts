import { nextTick, onBeforeUnmount, onMounted } from 'vue'
import { driver, type Driver } from 'driver.js'
import 'driver.js/dist/driver.css'
import { useI18n } from 'vue-i18n'
import {
  getSharedPoolGuideSteps,
  hasSeenSharedPoolGuide,
  markSharedPoolGuideSeen,
  type SharedPoolGuideScope,
} from '@/components/shared-pool/sharedPoolGuide'
import { useAuthStore } from '@/stores/auth'
import { useOnboardingStore } from '@/stores/onboarding'

type SharedPoolOnboardingOptions = {
  scope: SharedPoolGuideScope
  autoStart?: boolean
}

const FIRST_START_DELAY_MS = 1400
const ACTIVE_TOUR_RETRY_MS = 800

export function useSharedPoolOnboarding(options: SharedPoolOnboardingOptions) {
  const { t } = useI18n()
  const authStore = useAuthStore()
  const onboardingStore = useOnboardingStore()
  let currentDriver: Driver | null = null
  let autoStartTimer: ReturnType<typeof setTimeout> | null = null
  let pendingStartRequestId: number | null = null

  const userId = () => authStore.user?.id ?? null

  const hasSeen = (): boolean => {
    try {
      return hasSeenSharedPoolGuide(localStorage, options.scope, userId())
    } catch {
      return false
    }
  }

  const markAsSeen = (): void => {
    try {
      markSharedPoolGuideSeen(localStorage, options.scope, userId())
    } catch {
      // Storage can be unavailable in privacy-restricted browser contexts.
    }
  }

  const startGuide = async (replaceActive = false): Promise<boolean> => {
    const startRequestId = onboardingStore.beginDriverStartRequest()
    pendingStartRequestId = startRequestId
    const releaseStartRequest = () => {
      if (pendingStartRequestId === startRequestId) pendingStartRequestId = null
    }

    await nextTick()
    if (!onboardingStore.isDriverStartRequestCurrent(startRequestId)) {
      releaseStartRequest()
      return false
    }

    const activeDriver = onboardingStore.getDriverInstance()
    if (activeDriver?.isActive() && activeDriver !== currentDriver) {
      if (!replaceActive) {
        releaseStartRequest()
        return false
      }
    }

    const steps = getSharedPoolGuideSteps(options.scope, t)
    const ownsGuide = () => onboardingStore.getDriverInstance() === guide
    const cleanupGuideOwnership = () => {
      if (currentDriver === guide) currentDriver = null
    }
    const destroyOwnedGuide = () => {
      if (!ownsGuide()) return
      guide.destroy()
      onboardingStore.clearDriverInstance(guide)
    }
    const finish = () => {
      if (!ownsGuide()) return
      markAsSeen()
      destroyOwnedGuide()
    }

    releaseStartRequest()
    const guide = onboardingStore.replaceDriverInstance(() => driver({
      steps,
      showProgress: true,
      progressText: '{{current}} / {{total}}',
      animate: true,
      allowClose: true,
      stagePadding: 6,
      stageRadius: 8,
      popoverClass: 'theme-tour-popover shared-pool-tour-popover',
      showButtons: ['next', 'previous', 'close'],
      nextBtnText: t('sharedPoolGuide.next'),
      prevBtnText: t('sharedPoolGuide.previous'),
      doneBtnText: t('sharedPoolGuide.done'),
      onNextClick: (_element, _step, { state }) => {
        if (!ownsGuide()) return
        if ((state.activeIndex ?? 0) >= steps.length - 1) {
          finish()
          return
        }
        guide.moveNext()
      },
      onPrevClick: () => {
        if (ownsGuide()) guide.movePrevious()
      },
      onCloseClick: finish,
      onPopoverRender: (popover) => {
        const skipLabel = t('sharedPoolGuide.skip')
        popover.closeButton.title = skipLabel
        popover.closeButton.setAttribute('aria-label', skipLabel)
      },
      onDestroyed: () => {
        markAsSeen()
        onboardingStore.clearDriverInstance(guide)
      },
    }), cleanupGuideOwnership)

    currentDriver = guide
    guide.drive()
    return true
  }

  const replayGuide = (): void => {
    void startGuide(true)
  }

  const scheduleAutoStart = (delay = FIRST_START_DELAY_MS): void => {
    if (autoStartTimer) clearTimeout(autoStartTimer)
    autoStartTimer = setTimeout(() => {
      autoStartTimer = null
      if (hasSeen()) return
      if (onboardingStore.isDriverActive()) {
        scheduleAutoStart(ACTIVE_TOUR_RETRY_MS)
        return
      }
      void startGuide(false)
    }, delay)
  }

  onMounted(() => {
    if (options.autoStart === true && !hasSeen()) scheduleAutoStart()
  })

  onBeforeUnmount(() => {
    if (autoStartTimer) clearTimeout(autoStartTimer)
    autoStartTimer = null
    if (pendingStartRequestId !== null) {
      onboardingStore.cancelDriverStartRequest(pendingStartRequestId)
      pendingStartRequestId = null
    }
    const ownedDriver = currentDriver
    if (ownedDriver && onboardingStore.getDriverInstance() === ownedDriver) {
      if (ownedDriver.isActive()) ownedDriver.destroy()
      onboardingStore.clearDriverInstance(ownedDriver)
    }
    if (currentDriver === ownedDriver) currentDriver = null
  })

  return {
    hasSeen,
    replayGuide,
    startGuide,
  }
}
