<template>
  <div class="relative" ref="containerRef">
    <button
      type="button"
      @click="toggleDropdown"
      :disabled="switching"
      class="locale-switcher-trigger flex items-center gap-1.5 rounded-lg px-2 py-1.5 text-sm font-medium text-gray-600 transition-colors hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-dark-700"
      :title="currentLocale?.name"
    >
      <span class="text-base">{{ currentLocale?.flag }}</span>
      <span class="hidden sm:inline">{{ currentLocale?.code.toUpperCase() }}</span>
      <Icon
        name="chevronDown"
        size="xs"
        class="text-gray-400 transition-transform duration-200"
        :class="{ 'rotate-180': isOpen }"
      />
    </button>

    <Teleport to="body">
      <transition name="dropdown">
        <div
          v-if="isOpen"
          ref="dropdownRef"
          class="locale-switcher-dropdown locale-switcher-menu fixed z-[1000] w-32 overflow-hidden rounded-lg border border-gray-200 bg-white shadow-lg dark:border-dark-700 dark:bg-dark-800"
          :style="dropdownStyle"
          role="menu"
        >
          <button
            v-for="locale in availableLocales"
            :key="locale.code"
            type="button"
            :disabled="switching"
            @click="selectLocale(locale.code)"
            class="locale-switcher-item flex w-full items-center gap-2 px-3 py-2 text-sm text-gray-700 transition-colors hover:bg-gray-100 dark:text-gray-200 dark:hover:bg-dark-700"
            :class="{
              'bg-primary-50 text-primary-600 dark:bg-primary-900/20 dark:text-primary-400':
                locale.code === currentLocaleCode
            }"
            role="menuitem"
          >
            <span class="text-base">{{ locale.flag }}</span>
            <span>{{ locale.name }}</span>
            <Icon v-if="locale.code === currentLocaleCode" name="check" size="sm" class="ml-auto text-primary-500" />
          </button>
        </div>
      </transition>
    </Teleport>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, nextTick, onMounted, onUnmounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { setLocale, availableLocales } from '@/i18n'
import { usePointerDownOutside } from '@/composables/usePointerDownOutside'

const { locale } = useI18n()

const isOpen = ref(false)
const containerRef = ref<HTMLElement | null>(null)
const dropdownRef = ref<HTMLElement | null>(null)
const dropdownStyle = ref<Record<string, string>>({
  top: '0px',
  left: '0px'
})
const switching = ref(false)

const currentLocaleCode = computed(() => locale.value)
const currentLocale = computed(() => availableLocales.find((l) => l.code === locale.value))

function toggleDropdown() {
  isOpen.value = !isOpen.value
  if (isOpen.value) {
    void nextTick(updateDropdownPosition)
  }
}

async function selectLocale(code: string) {
  if (switching.value || code === currentLocaleCode.value) {
    isOpen.value = false
    return
  }
  switching.value = true
  try {
    await setLocale(code)
    isOpen.value = false
  } finally {
    switching.value = false
  }
}

usePointerDownOutside([containerRef, dropdownRef], () => {
  isOpen.value = false
}, isOpen)

function handleEscape(event: KeyboardEvent) {
  if (event.key === 'Escape' && isOpen.value) {
    isOpen.value = false
  }
}

function updateDropdownPosition() {
  if (!containerRef.value || !isOpen.value) return

  const rect = containerRef.value.getBoundingClientRect()
  const viewportWidth = window.innerWidth || document.documentElement.clientWidth
  const dropdownWidth = dropdownRef.value?.offsetWidth || 128
  const margin = 12
  const left = Math.min(
    Math.max(rect.right - dropdownWidth, margin),
    Math.max(margin, viewportWidth - dropdownWidth - margin)
  )

  dropdownStyle.value = {
    top: `${rect.bottom + 4}px`,
    left: `${left}px`
  }
}

watch(isOpen, async (open) => {
  if (open) {
    await nextTick()
    updateDropdownPosition()
  }
})

onMounted(() => {
  document.addEventListener('keydown', handleEscape)
  window.addEventListener('resize', updateDropdownPosition)
  window.addEventListener('scroll', updateDropdownPosition, true)
})

onUnmounted(() => {
  document.removeEventListener('keydown', handleEscape)
  window.removeEventListener('resize', updateDropdownPosition)
  window.removeEventListener('scroll', updateDropdownPosition, true)
})
</script>

<style scoped>
.locale-switcher-trigger {
  border: 0;
  border-radius: 999px;
  background: var(--zc-bg) !important;
  color: var(--zc-text-strong) !important;
  box-shadow: 5px 5px 12px var(--zc-shadow-dark), -5px -5px 12px var(--zc-shadow-light);
}

.locale-switcher-trigger:hover {
  background: var(--zc-bg) !important;
  color: var(--zc-accent) !important;
  box-shadow: 7px 7px 16px var(--zc-shadow-dark), -7px -7px 16px var(--zc-shadow-light);
}

.locale-switcher-menu {
  border: 0 !important;
  border-radius: 18px !important;
  background: var(--zc-bg) !important;
  color: var(--zc-text) !important;
  box-shadow: 12px 12px 28px var(--zc-shadow-dark), -12px -12px 28px var(--zc-shadow-light) !important;
}

.locale-switcher-item {
  background: transparent !important;
  color: var(--zc-muted) !important;
  font-weight: 800;
}

.locale-switcher-item:hover {
  background: var(--zc-bg) !important;
  color: var(--zc-text-strong) !important;
  box-shadow: inset 4px 4px 9px var(--zc-shadow-dark), inset -4px -4px 9px var(--zc-shadow-light);
}

.locale-switcher-item.bg-primary-50 {
  color: var(--zc-accent) !important;
  box-shadow: inset 4px 4px 9px var(--zc-shadow-dark), inset -4px -4px 9px var(--zc-shadow-light);
}

.dropdown-enter-active,
.dropdown-leave-active {
  transition: all 0.15s ease;
}

.dropdown-enter-from,
.dropdown-leave-to {
  opacity: 0;
  transform: scale(0.95) translateY(-4px);
}
</style>
