<template>
  <section class="city-pulse" :class="{ 'city-pulse--compact': compact }" aria-label="零号城精选活动与榜单">
    <article class="city-event">
      <img :src="artwork" alt="" width="152" height="176" />
      <div class="city-event-copy">
        <span>本期酒馆故事</span>
        <strong>最后一封来信</strong>
        <p>从一封写给明天的信开始，留下可继续的城市故事。</p>
        <div>
          <button class="community-btn community-btn-primary" type="button" @click="$emit('open-event')">进入故事</button>
          <button class="community-btn community-btn-secondary" type="button" @click="$emit('open-plaza')">去闲聊广场</button>
        </div>
      </div>
    </article>

    <ZeroCityRankingsPanel
      class="city-podium"
      variant="top"
      :definitions="definitions"
      :rows-by-key="rowsByKey"
      :data-scope="dataScope"
      @select="(row, definition) => $emit('select', row, definition)"
      @open="(definition) => $emit('open', definition)"
    />
  </section>
</template>

<script setup lang="ts">
import ZeroCityRankingsPanel from './ZeroCityRankingsPanel.vue'
import type { ZeroCityRankingDefinition, ZeroCityRankingRow } from '../../data/zeroCityRankings'

withDefaults(defineProps<{
  definitions: readonly ZeroCityRankingDefinition[]
  rowsByKey: Readonly<Record<string, readonly ZeroCityRankingRow[]>>
  dataScope?: string
  artwork?: string
  compact?: boolean
}>(), {
  dataScope: '基于当前可见动态',
  artwork: '/assets/zero-point-city/cards/collectible/rare/half_awake_oracle.png',
})

defineEmits<{
  'open-event': []
  'open-plaza': []
  select: [row: ZeroCityRankingRow, definition: ZeroCityRankingDefinition | undefined]
  open: [definition: ZeroCityRankingDefinition]
}>()
</script>

<style scoped>
.city-pulse { display:grid; grid-template-columns:minmax(0,1.2fr) minmax(20rem,.8fr); gap:var(--zc-city-gap); align-items:stretch; padding-block:var(--bd-space-3); border-block:var(--bd-line-width) solid var(--zc-line); }
.city-event { display:grid; grid-template-columns:9.5rem minmax(0,1fr); align-items:center; min-width:0; overflow:hidden; background:linear-gradient(112deg,color-mix(in srgb,var(--zc-city-event) 18%,var(--zc-card)),var(--zc-card) 72%); }
.city-event img { width:100%; height:11rem; object-fit:contain; align-self:end; filter:drop-shadow(0 .75rem 1rem color-mix(in srgb,var(--zc-city-night) 32%,transparent)); }
.city-event-copy { display:grid; gap:var(--bd-space-2); padding:var(--bd-space-4); }
.city-event-copy > span { color:var(--zc-accent); font-size:var(--bd-type-overline); font-weight:var(--bd-weight-emphasis); }
.city-event-copy > strong { color:var(--zc-text-strong); font-size:var(--zc-city-section); line-height:var(--zc-city-line-heading); }
.city-event-copy p { color:var(--zc-muted); font-size:var(--bd-type-caption); line-height:var(--bd-line-body-small); }
.city-event-copy div { display:flex; flex-wrap:wrap; gap:var(--bd-space-2); }
.city-event .community-btn { display:inline-flex; min-height:2.25rem; align-items:center; justify-content:center; padding:var(--bd-space-2) var(--bd-space-3); border:var(--bd-line-width) solid var(--zc-line); border-radius:var(--bd-radius-sm); color:var(--zc-text-strong); font-size:var(--bd-type-caption); font-weight:var(--bd-weight-emphasis); transition:transform var(--bd-motion-micro) ease-out, filter var(--bd-motion-micro) ease-out; }
.city-event .community-btn-primary { border-color:var(--zc-accent); background:var(--zc-accent); color:var(--zc-accent-ink); }
.city-event .community-btn-secondary { background:color-mix(in srgb,var(--zc-card) 78%,transparent); }
.city-event .community-btn:hover { transform:translateY(var(--bd-motion-lift)); filter:brightness(var(--bd-control-hover-brightness)); }
.city-event .community-btn:focus-visible { outline:var(--bd-focus-ring-width) solid var(--zc-accent); outline-offset:var(--bd-motion-shift); }
.city-podium { min-width:0; padding-left:var(--bd-space-4); border-left:var(--bd-line-width) solid var(--zc-line); }
.city-pulse--compact { grid-template-columns:minmax(0,1fr); }
.city-pulse--compact .city-event { grid-template-columns:4.5rem minmax(0,1fr); }
.city-pulse--compact .city-event img { height:7rem; }
.city-pulse--compact .city-podium { padding:var(--bd-space-3) 0 0; border-left:0; border-top:var(--bd-line-width) solid var(--zc-line); }
@media (max-width: 900px) { .city-pulse { grid-template-columns:1fr; } .city-podium { padding:var(--bd-space-3) 0 0; border-top:var(--bd-line-width) solid var(--zc-line); border-left:0; } }
@media (max-width: 560px) { .city-event { grid-template-columns:6.5rem minmax(0,1fr); } .city-event img { height:9rem; } .city-event-copy { padding:var(--bd-space-3); } }
@media (prefers-reduced-motion: reduce) { .city-event .community-btn { transition:none; } .city-event .community-btn:hover { transform:none; } }
</style>
