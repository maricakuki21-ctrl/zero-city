/**
 * Onboarding Store
 * Manages onboarding tour state and control methods
 */

import { defineStore } from 'pinia'
import { markRaw, ref, shallowRef } from 'vue'
import type { Driver } from 'driver.js'

type VoidCallback = () => void
type NextStepCallback = (delay?: number) => Promise<void>
type IsCurrentStepCallback = (selector: string) => boolean

export const useOnboardingStore = defineStore('onboarding', () => {
  const replayCallback = ref<VoidCallback | null>(null)
  const nextStepCallback = ref<NextStepCallback | null>(null)
  const isCurrentStepCallback = ref<IsCurrentStepCallback | null>(null)

  // 全局 driver 实例，跨组件保持
  const driverInstance = shallowRef<Driver | null>(null)
  let driverCleanup: VoidCallback | null = null
  let driverStartRequestId = 0

  function setReplayCallback(callback: VoidCallback | null): void {
    replayCallback.value = callback
  }

  function setControlMethods(methods: {
    nextStep: NextStepCallback,
    isCurrentStep: IsCurrentStepCallback
  }): void {
    nextStepCallback.value = methods.nextStep
    isCurrentStepCallback.value = methods.isCurrentStep
  }

  function clearControlMethods(): void {
    nextStepCallback.value = null
    isCurrentStepCallback.value = null
  }

  function beginDriverStartRequest(): number {
    driverStartRequestId += 1
    return driverStartRequestId
  }

  function isDriverStartRequestCurrent(requestId: number): boolean {
    return driverStartRequestId === requestId
  }

  function cancelDriverStartRequest(requestId: number): void {
    if (driverStartRequestId === requestId) driverStartRequestId += 1
  }

  function replaceDriverInstance(
    createDriver: () => Driver,
    cleanup?: VoidCallback,
  ): Driver {
    const previous = driverInstance.value
    const previousCleanup = driverCleanup
    driverCleanup = null
    previousCleanup?.()

    if (previous) {
      // driver.js keeps its config and state globally. The old facade must be
      // destroyed before constructing the replacement, otherwise destroy()
      // dispatches the replacement's callbacks.
      previous.destroy()
      if (driverInstance.value === previous) driverInstance.value = null
    }

    const next = markRaw(createDriver())
    driverInstance.value = next
    driverCleanup = cleanup ?? null
    return next
  }

  function clearDriverInstance(expected: Driver): void {
    if (driverInstance.value === expected) {
      const cleanup = driverCleanup
      driverCleanup = null
      driverInstance.value = null
      cleanup?.()
    }
  }

  function getDriverInstance(): Driver | null {
    return driverInstance.value
  }

  function isDriverActive(): boolean {
    return driverInstance.value?.isActive?.() ?? false
  }

  function replay(): void {
    if (replayCallback.value) {
      replayCallback.value()
    }
  }

  /**
   * Manually advance to the next step
   * @param delay Optional delay in ms (useful for waiting for animations)
   */
  async function nextStep(delay = 0): Promise<void> {
    if (nextStepCallback.value) {
      await nextStepCallback.value(delay)
    }
  }

  /**
   * Check if the tour is currently highlighting a specific element
   */
  function isCurrentStep(selector: string): boolean {
    if (isCurrentStepCallback.value) {
      return isCurrentStepCallback.value(selector)
    }
    return false
  }

  return {
    setReplayCallback,
    setControlMethods,
    clearControlMethods,
    beginDriverStartRequest,
    isDriverStartRequestCurrent,
    cancelDriverStartRequest,
    replaceDriverInstance,
    clearDriverInstance,
    getDriverInstance,
    isDriverActive,
    replay,
    nextStep,
    isCurrentStep
  }
})
