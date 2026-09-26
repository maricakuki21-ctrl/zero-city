import { onBeforeUnmount, onMounted, type Ref } from 'vue'

type ElementRef = Readonly<Ref<HTMLElement | null>>

export function usePointerDownOutside(
  elementRefs: readonly ElementRef[],
  onOutside: (event: PointerEvent) => void,
  enabled?: Readonly<Ref<boolean>>
) {
  const handlePointerDown = (event: PointerEvent) => {
    if (enabled && !enabled.value) return

    const path = typeof event.composedPath === 'function' ? event.composedPath() : []
    const target = event.target
    const isInside = elementRefs.some((elementRef) => {
      const element = elementRef.value
      if (!element) return false
      if (path.includes(element)) return true
      return target instanceof Node && element.contains(target)
    })

    if (!isInside) onOutside(event)
  }

  onMounted(() => document.addEventListener('pointerdown', handlePointerDown))
  onBeforeUnmount(() => document.removeEventListener('pointerdown', handlePointerDown))
}
