<template>
  <div class="ranking-podium" aria-label="排行榜前三名">
    <button
      v-for="row in podiumRows"
      :key="`${row.rank}-${row.label}`"
      type="button"
      class="podium-place"
      :class="`podium-place-${row.rank}`"
      :disabled="row.vacant"
      @click="selectRow(row)"
    >
      <Crown v-if="row.rank === 1 && !row.vacant" class="podium-crown" :size="24" :stroke-width="2" aria-hidden="true" />
      <span class="podium-avatar" :class="{ vacant: row.vacant }" aria-hidden="true">{{ row.vacant ? row.rank : row.label.slice(0, 1) }}</span>
      <strong>{{ row.label }}</strong>
      <small>{{ row.value }}</small>
      <span class="podium-step"><b>{{ row.rank }}</b><em>{{ placeLabel(row.rank) }}</em></span>
    </button>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { Crown } from '@lucide/vue'
import type { ZeroCityRankingRow } from '@/features/bizdecipher/data/zeroCityRankings'

const props = defineProps<{
  rows: readonly ZeroCityRankingRow[]
}>()

const emit = defineEmits<{
  select: [row: ZeroCityRankingRow]
}>()

type PodiumRow = ZeroCityRankingRow & { vacant?: boolean }

const rankedRows = computed<PodiumRow[]>(() => [1, 2, 3].map((rank) => {
  const row = props.rows.find((item) => item.rank === rank)
  return row || { rank, label: '席位待定', value: '暂无记录', secondary: '等待可验证结果', vacant: true }
}))

const podiumRows = computed<PodiumRow[]>(() => [rankedRows.value[1]!, rankedRows.value[0]!, rankedRows.value[2]!])

function selectRow(row: PodiumRow): void {
  if (!row.vacant) emit('select', row)
}

function placeLabel(rank: number): string {
  if (rank === 1) return '冠军'
  if (rank === 2) return '亚军'
  return '季军'
}
</script>

<style scoped>
.ranking-podium {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  align-items: end;
  gap: var(--bd-space-2);
  min-height: 7.5rem;
  padding-top: var(--bd-space-4);
}

.podium-place {
  position: relative;
  display: grid;
  min-width: 0;
  justify-items: center;
  gap: var(--bd-space-1);
  min-height: 8.25rem;
  padding: var(--bd-space-2) var(--bd-space-1) 0;
  border: var(--bd-line-width) solid var(--zc-line);
  border-radius: var(--bd-radius-md) var(--bd-radius-md) 0 0;
  background: color-mix(in srgb, var(--zc-card) 82%, var(--zc-bg));
  color: var(--zc-text-strong);
  text-align: center;
  transition: transform var(--bd-motion-micro) ease-out, filter var(--bd-motion-micro) ease-out;
}

.podium-place:hover,
.podium-place:focus-visible {
  transform: translateY(var(--bd-motion-lift));
  filter: brightness(var(--bd-control-hover-brightness));
}

.podium-place:disabled { cursor: default; }
.podium-place:disabled:hover { transform: none; }
.podium-avatar.vacant { border-style: dashed; background: transparent; color: var(--zc-muted); }

.podium-place:focus-visible {
  outline: var(--bd-focus-ring-width) solid var(--zc-accent);
  outline-offset: var(--bd-motion-shift);
}

.podium-place-1 {
  grid-column: 2;
  border-color: color-mix(in srgb, var(--bd-accent-gold) 65%, var(--zc-line));
  background: color-mix(in srgb, var(--bd-accent-gold) 13%, var(--zc-card));
}

.podium-place-2 { grid-column: 1; }
.podium-place-3 { grid-column: 3; }

.podium-crown {
  position: absolute;
  top: -1.35rem;
  color: var(--bd-accent-gold);
  filter: drop-shadow(0 3px 7px color-mix(in srgb, var(--bd-accent-gold) 34%, transparent));
}

.podium-avatar {
  display: grid;
  width: 2rem;
  height: 2rem;
  place-items: center;
  border: var(--bd-line-width) solid color-mix(in srgb, var(--zc-accent) 45%, var(--zc-line));
  border-radius: 50%;
  background: var(--zc-bg);
  color: var(--zc-accent);
  font-weight: var(--bd-weight-emphasis);
}

.podium-place strong,
.podium-place small {
  overflow: hidden;
  max-width: 100%;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.podium-place strong { font-size: var(--bd-type-caption); }
.podium-place small { color: var(--zc-muted); font-size: var(--bd-type-overline); }

.podium-step {
  display: grid;
  width: calc(100% + var(--bd-space-4));
  min-height: 2.75rem;
  place-content: center;
  margin-top: auto;
  border-top: var(--bd-line-width) solid var(--zc-line);
  color: var(--zc-muted);
}

.podium-step b { color: var(--zc-text-strong); font-size: 1rem; }
.podium-step em { font-size: var(--bd-type-overline); font-style: normal; }
.podium-place-1 .podium-step b { color: var(--bd-accent-gold); }
.podium-place-1 .podium-step { min-height: 4rem; }
.podium-place-2 .podium-step { min-height: 3.25rem; }

@media (max-width: 560px) {
  .ranking-podium { min-height: 9rem; }
  .podium-place { min-height: 8rem; }
  .podium-place-1 .podium-step { min-height: 3.75rem; }
  .podium-place-2 .podium-step { min-height: 3.15rem; }
  .podium-place-3 .podium-step { min-height: 2.65rem; }
}

@media (prefers-reduced-motion: reduce) {
  .podium-place { transition: none; }
  .podium-place:hover,
  .podium-place:focus-visible { transform: none; }
}
</style>
