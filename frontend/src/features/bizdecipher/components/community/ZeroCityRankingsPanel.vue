<template>
  <section :class="['zero-city-rankings-panel zero-city-aside-panel', `zero-city-rankings-panel--${variant}`]" :aria-labelledby="titleId" @mouseenter="hovered = true" @mouseleave="hovered = false" @focusin="focused = true" @focusout="focused = false">
    <div class="zero-city-aside-head">
      <div>
        <p class="community-kicker">{{ variant === 'top' ? `${activeDefinition?.windowLabel || '当前'}榜单` : '城市排行' }}</p>
        <h2 :id="titleId">{{ activeDefinition?.title || '本城榜单' }}</h2>
        <small v-if="dataScope" class="zero-city-ranking-scope">{{ dataScope }}</small>
      </div>
      <button v-if="variant === 'top'" type="button" class="ranking-rotation" :aria-pressed="rotating" :aria-label="rotating ? '暂停榜单轮播' : '播放榜单轮播'" :title="rotating ? '暂停榜单轮播' : '播放榜单轮播'" @click="rotating = !rotating"><Pause v-if="rotating" :size="14" /><Play v-else :size="14" /></button>
      <span v-else-if="activeDefinition" class="zero-city-ranking-window">{{ activeDefinition.windowLabel }}</span>
    </div>

    <nav v-if="definitions.length > 1" class="zero-city-ranking-tabs" aria-label="榜单类型" role="tablist">
      <button
        v-for="definition in definitions"
        :key="definition.key"
        type="button"
        role="tab"
        :id="tabId(definition.key)"
        :data-ranking-key="definition.key"
        :aria-selected="definition.key === activeKey"
        :aria-controls="panelId(definition.key)"
        :tabindex="definition.key === activeKey ? 0 : -1"
        :class="{ active: definition.key === activeKey }"
        @click="activeKey = definition.key"
        @keydown="handleTabKeydown"
      >
        {{ definition.title }}
      </button>
    </nav>

    <ZeroCityRankingPodium
      v-if="variant === 'top'"
      :rows="activeRows"
      @select="$emit('select', $event, activeDefinition)"
    />

    <div v-if="activeDefinition && variant !== 'top'" class="zero-city-ranking-intent">
      <p class="zero-city-ranking-description">{{ activeDefinition.description }}</p>
      <strong>{{ activeDefinition.behaviorLabel }}</strong>
    </div>

    <div v-if="activeDefinition && variant !== 'top'" class="zero-city-ranking-rewards" aria-label="本榜口径">
      <span><b>近期口径</b>{{ activeDefinition.weeklyRule }}</span>
      <span><b>长期边界</b>{{ activeDefinition.monthlyRule }}</span>
    </div>

    <div
      v-if="variant !== 'top' && activeRows.length"
      :id="panelId(activeKey)"
      class="zero-city-ranking-list"
      role="tabpanel"
      :aria-labelledby="definitions.length > 1 ? tabId(activeKey) : undefined"
      :aria-label="definitions.length > 1 ? undefined : activeDefinition?.title"
    >
      <button v-for="row in visibleRows" :key="`${activeKey}-${row.rank}-${row.label}`" type="button" class="zero-city-ranking-row" @click="$emit('select', row, activeDefinition)">
        <span class="zero-city-ranking-rank" :class="`rank-${row.rank}`">
          <Crown v-if="row.rank === 1" :size="16" :stroke-width="2" aria-hidden="true" />
          <span>{{ row.rank }}</span>
        </span>
        <span class="zero-city-ranking-copy">
          <strong>{{ row.label }}</strong>
          <small>{{ row.secondary }}</small>
        </span>
        <strong class="zero-city-ranking-value">{{ row.value }}</strong>
      </button>
    </div>
    <p v-else-if="variant !== 'top' && !activeRows.length" class="zero-city-aside-empty">这里还没有足够的可验证记录。</p>

    <button v-if="activeDefinition" type="button" class="zero-city-ranking-more" @click="$emit('open', activeDefinition)">
      {{ variant === 'top' ? '查看全部' : '查看相关城区' }}
    </button>
  </section>
</template>

<script setup lang="ts">
import { computed, getCurrentInstance, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { Crown, Pause, Play } from '@lucide/vue'
import ZeroCityRankingPodium from '@/features/bizdecipher/components/community/ZeroCityRankingPodium.vue'
import type { ZeroCityRankingDefinition, ZeroCityRankingRow } from '@/features/bizdecipher/data/zeroCityRankings'

const props = defineProps<{
  definitions: readonly ZeroCityRankingDefinition[]
  rowsByKey: Readonly<Record<string, readonly ZeroCityRankingRow[]>>
  variant?: 'side' | 'top'
  dataScope?: string
}>()

defineEmits<{
  select: [row: ZeroCityRankingRow, definition: ZeroCityRankingDefinition | undefined]
  open: [definition: ZeroCityRankingDefinition]
}>()

const activeKey = ref('')
const rotating = ref(true)
const hovered = ref(false)
const focused = ref(false)
let rotationTimer: ReturnType<typeof setInterval> | undefined
onMounted(() => {
  rotating.value = !window.matchMedia?.('(prefers-reduced-motion: reduce)').matches
  rotationTimer = setInterval(() => {
    if (props.variant !== 'top' || !rotating.value || hovered.value || focused.value || document.hidden || props.definitions.length < 2) return
    const index = props.definitions.findIndex(item => item.key === activeKey.value)
    activeKey.value = props.definitions[(index + 1) % props.definitions.length]?.key || ''
  }, 8000)
})
onBeforeUnmount(() => { if (rotationTimer) clearInterval(rotationTimer) })
const instanceUid = getCurrentInstance()?.uid ?? 0
const idPrefix = `zero-city-rankings-${instanceUid}`
const titleId = `${idPrefix}-title`

function tabId(key: string): string {
  return `${idPrefix}-tab-${key}`
}

function panelId(key: string): string {
  return `${idPrefix}-panel-${key}`
}

function handleTabKeydown(event: KeyboardEvent): void {
  const currentTab = event.currentTarget
  if (!(currentTab instanceof HTMLButtonElement)) return

  const tabList = currentTab.closest('[role="tablist"]')
  if (!(tabList instanceof HTMLElement)) return

  const tabs = [...tabList.querySelectorAll<HTMLButtonElement>('[role="tab"]')]
  const currentIndex = tabs.indexOf(currentTab)
  if (currentIndex < 0 || tabs.length === 0) return

  let nextIndex: number
  switch (event.key) {
    case 'ArrowLeft':
      nextIndex = (currentIndex - 1 + tabs.length) % tabs.length
      break
    case 'ArrowRight':
      nextIndex = (currentIndex + 1) % tabs.length
      break
    case 'Home':
      nextIndex = 0
      break
    case 'End':
      nextIndex = tabs.length - 1
      break
    default:
      return
  }

  const nextTab = tabs[nextIndex]
  const nextKey = nextTab?.dataset.rankingKey
  if (!nextTab || !nextKey) return

  event.preventDefault()
  activeKey.value = nextKey
  nextTab.focus()
}

watch(
  () => props.definitions,
  (definitions) => {
    if (!definitions.some((definition) => definition.key === activeKey.value)) {
      activeKey.value = definitions[0]?.key || ''
    }
  },
  { immediate: true },
)

const activeDefinition = computed(() => props.definitions.find((definition) => definition.key === activeKey.value))
const activeRows = computed(() => activeDefinition.value ? props.rowsByKey[activeDefinition.value.key] || [] : [])
const variant = computed(() => props.variant || 'side')
const visibleRows = computed(() => activeRows.value)
</script>

<style scoped>
.ranking-rotation { display:grid; place-items:center; width:28px; height:28px; border:1px solid var(--zc-line); border-radius:6px; color:var(--zc-muted); }
.zero-city-rankings-panel {
  display: grid;
  gap: var(--bd-space-3);
}

.zero-city-aside-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: var(--bd-space-3);
}

.zero-city-ranking-scope {
  display: block;
  margin-top: var(--bd-space-1);
  color: var(--zc-muted);
  font-size: var(--bd-type-overline);
  line-height: var(--bd-line-caption);
}

.zero-city-rankings-panel--top {
  align-content: center;
  gap: var(--bd-space-2);
  min-width: 0;
  padding: var(--bd-space-1) 0 var(--bd-space-1) var(--bd-space-4);
  border-left: var(--bd-line-width) solid color-mix(in srgb, var(--zc-line) 90%, transparent);
}

.zero-city-rankings-panel--top .zero-city-aside-head {
  justify-content: space-between;
}

.zero-city-rankings-panel--top .zero-city-ranking-list {
  gap: 0;
  border-top: var(--bd-line-width) solid color-mix(in srgb, var(--zc-line) 82%, transparent);
  border-bottom: var(--bd-line-width) solid color-mix(in srgb, var(--zc-line) 82%, transparent);
}

.zero-city-rankings-panel--top .zero-city-ranking-row {
  padding: var(--bd-space-2) 0;
  border-radius: 0;
  border-bottom: var(--bd-line-width) solid color-mix(in srgb, var(--zc-line) 72%, transparent);
  background: transparent;
}

.zero-city-rankings-panel--top .zero-city-ranking-row:last-child {
  border-bottom: 0;
}

.zero-city-rankings-panel--top .zero-city-ranking-row:hover {
  background: color-mix(in srgb, var(--zc-accent) 8%, transparent);
  transform: translateX(var(--bd-motion-shift));
}

.zero-city-rankings-panel--top .zero-city-ranking-copy strong,
.zero-city-rankings-panel--top .zero-city-ranking-copy small {
  -webkit-line-clamp: 1;
}

.zero-city-rankings-panel--top .zero-city-ranking-value {
  font-size: var(--bd-type-caption);
}

.zero-city-rankings-panel--top .zero-city-ranking-more {
  justify-self: end;
}

.zero-city-ranking-window {
  border-radius: var(--bd-radius-pill);
  background: color-mix(in srgb, var(--zc-accent) 10%, var(--zc-card));
  padding: var(--bd-space-1) var(--bd-space-2);
  color: var(--zc-accent);
  font-size: var(--bd-type-overline);
  font-weight: var(--bd-weight-emphasis);
}

.zero-city-ranking-tabs {
  display: flex;
  gap: var(--bd-space-1);
  overflow-x: auto;
  padding-bottom: var(--bd-space-1);
}

.zero-city-ranking-tabs button {
  flex: 0 0 auto;
  border-radius: var(--bd-radius-pill);
  padding: var(--bd-space-2) var(--bd-space-3);
  color: var(--zc-muted);
  font-size: var(--bd-type-overline);
  font-weight: var(--bd-weight-emphasis);
  transition: filter var(--bd-motion-micro) ease-out, opacity var(--bd-motion-micro) ease-out;
}

.zero-city-ranking-tabs button.active {
  background: var(--zc-accent);
  color: var(--zc-accent-ink);
}

.zero-city-ranking-tabs button:hover:not(.active) {
  background: color-mix(in srgb, var(--zc-accent) 12%, transparent);
  color: var(--zc-text-strong);
}

.zero-city-ranking-tabs button.active:hover {
  filter: brightness(var(--bd-control-hover-brightness));
}

.zero-city-ranking-tabs button:focus-visible,
.zero-city-ranking-row:focus-visible,
.zero-city-ranking-more:focus-visible {
  outline: var(--bd-focus-ring-width) solid var(--zc-accent);
  outline-offset: var(--bd-motion-shift);
}

.zero-city-ranking-description {
  color: var(--zc-muted);
  font-size: var(--bd-type-caption);
  line-height: var(--bd-line-body-small);
}

.zero-city-ranking-intent {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: var(--bd-space-3);
}

.zero-city-ranking-intent strong {
  flex: 0 0 auto;
  color: var(--zc-accent);
  font-size: var(--bd-type-overline);
  font-weight: var(--bd-weight-emphasis);
}

.zero-city-ranking-rewards {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: var(--bd-space-2);
}

.zero-city-ranking-rewards span {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: var(--bd-space-2);
  padding: var(--bd-space-2);
  border: var(--bd-line-width) solid color-mix(in srgb, var(--zc-line) 86%, transparent);
  border-radius: var(--bd-radius-md);
  color: var(--zc-muted);
  font-size: var(--bd-type-overline);
  line-height: var(--bd-line-caption);
}

.zero-city-ranking-rewards b {
  flex: 0 0 auto;
  color: var(--zc-text-strong);
}

.zero-city-ranking-list {
  display: grid;
  gap: var(--bd-space-2);
}

.zero-city-ranking-row {
  display: grid;
  grid-template-columns: var(--bd-space-6) minmax(0, 1fr) auto;
  align-items: center;
  gap: var(--bd-space-2);
  width: 100%;
  padding: var(--bd-space-3) var(--bd-space-2);
  border-radius: var(--bd-radius-md);
  text-align: left;
  transition: transform var(--bd-motion-micro) ease-out, opacity var(--bd-motion-micro) ease-out;
}

.zero-city-ranking-row:hover {
  background: color-mix(in srgb, var(--zc-accent) 8%, transparent);
  transform: translateY(var(--bd-motion-lift));
}

@media (max-width: 768px) {
  .zero-city-rankings-panel--top {
    gap: var(--bd-space-2);
    padding: var(--bd-space-3) 0 0;
    border-top: var(--bd-line-width) solid color-mix(in srgb, var(--zc-line) 90%, transparent);
    border-left: 0;
  }

  .zero-city-rankings-panel--top .zero-city-ranking-row {
    padding: var(--bd-space-2) 0;
  }
}

.zero-city-ranking-rank {
  display: inline-flex;
  height: var(--bd-space-6);
  width: var(--bd-space-6);
  align-items: center;
  justify-content: center;
  border-radius: var(--bd-radius-sm);
  background: color-mix(in srgb, var(--zc-muted) 12%, var(--zc-card));
  color: var(--zc-muted);
  font-size: var(--bd-type-overline);
  font-weight: var(--bd-weight-emphasis);
}

.zero-city-ranking-rank svg {
  display: none;
}

.zero-city-ranking-rank.rank-1 {
  width: auto;
  min-width: calc(var(--bd-space-6) + var(--bd-space-3));
  gap: var(--bd-space-1);
  background: color-mix(in srgb, var(--bd-accent-gold) 30%, var(--zc-card));
  color: var(--bd-accent-gold);
}

.zero-city-ranking-rank.rank-1 svg {
  display: block;
}

@media (max-width: 560px) {
  .zero-city-ranking-intent {
    align-items: flex-start;
    flex-direction: column;
    gap: var(--bd-space-1);
  }

  .zero-city-ranking-rewards {
    grid-template-columns: 1fr;
  }
}

.zero-city-ranking-rank.rank-2 {
  background: color-mix(in srgb, var(--bd-text-secondary) 26%, var(--zc-card));
  color: var(--bd-text-secondary);
}

.zero-city-ranking-rank.rank-3 {
  background: color-mix(in srgb, var(--zc-accent-3) 26%, var(--zc-card));
  color: var(--zc-accent-3);
}

.zero-city-ranking-copy {
  display: grid;
  min-width: 0;
  gap: var(--bd-space-1);
}

.zero-city-ranking-copy strong {
  color: var(--zc-text-strong);
  font-size: var(--bd-type-caption);
  text-overflow: ellipsis;
  overflow-wrap: anywhere;
  display: -webkit-box;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
  overflow: hidden;
}

.zero-city-ranking-copy small {
  color: var(--zc-muted);
  font-size: var(--bd-type-overline);
  line-height: var(--bd-line-caption);
  text-overflow: ellipsis;
  overflow-wrap: anywhere;
  display: -webkit-box;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
  overflow: hidden;
}

.zero-city-ranking-value {
  color: var(--zc-accent);
  font-size: var(--bd-type-overline);
  white-space: nowrap;
}

.zero-city-ranking-more {
  justify-self: start;
  color: var(--zc-accent);
  font-size: var(--bd-type-overline);
  font-weight: var(--bd-weight-emphasis);
  transition: transform var(--bd-motion-micro) ease-out, opacity var(--bd-motion-micro) ease-out;
}

.zero-city-ranking-more:hover {
  color: var(--zc-text-strong);
  transform: translateX(var(--bd-motion-shift));
}

.zero-city-ranking-tabs button:active,
.zero-city-ranking-row:active,
.zero-city-ranking-more:active {
  opacity: var(--bd-control-pressed-opacity);
}

@media (prefers-reduced-motion: reduce) {
  .zero-city-ranking-tabs button,
  .zero-city-ranking-row,
  .zero-city-ranking-more {
    transition: none;
  }

  .zero-city-rankings-panel--top .zero-city-ranking-row:hover,
  .zero-city-ranking-row:hover,
  .zero-city-ranking-more:hover {
    transform: none;
  }
}
</style>
