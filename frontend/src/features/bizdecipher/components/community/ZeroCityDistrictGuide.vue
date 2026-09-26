<template>
  <div class="district-guide" :class="`district-guide--${surface.kind}`">
    <div class="district-guide-copy">
      <span>{{ surface.eyebrow }}</span>
      <h1>{{ surface.title }}</h1>
      <p>{{ surface.description }}</p>
    </div>
    <div class="district-guide-cues" :aria-label="`${surface.title}内容边界`">
      <span v-for="cue in surface.cues" :key="cue">{{ cue }}</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import type { ZeroCitySurfaceDefinition } from '../../data/zeroCityExperience'

defineProps<{ surface: ZeroCitySurfaceDefinition }>()
</script>

<style scoped>
.district-guide { display:grid; grid-template-columns:minmax(0,1.1fr) minmax(14rem,.9fr); gap:var(--zc-city-gap); align-items:end; padding:var(--bd-space-5) 0 var(--bd-space-4); border-bottom:var(--bd-line-width) solid var(--zc-line); }
.district-guide-copy { display:grid; gap:var(--bd-space-2); }
.district-guide-copy > span { color:var(--zc-accent); font-size:var(--bd-type-overline); font-weight:var(--bd-weight-emphasis); }
.district-guide h1 { color:var(--zc-text-strong); font-size:var(--zc-city-title); line-height:var(--zc-city-line-heading); }
.district-guide p { max-width:45rem; color:var(--zc-muted); font-size:var(--zc-city-body); line-height:var(--zc-city-line-body); }
.district-guide-cues { display:flex; flex-wrap:wrap; justify-content:flex-end; gap:var(--bd-space-2); }
.district-guide-cues span { padding:var(--bd-space-2) var(--bd-space-3); border-left:2px solid var(--zc-city-cue); background:color-mix(in srgb,var(--zc-city-cue) 7%,transparent); color:var(--zc-text); font-size:var(--bd-type-overline); font-weight:var(--bd-weight-emphasis); }
.district-guide--workshop { --zc-city-cue:var(--bd-accent-blue); }
.district-guide--collaboration { --zc-city-cue:var(--bd-accent-gold); }
.district-guide--activity { --zc-city-cue:var(--zc-accent-2); }
.district-guide--plaza { --zc-city-cue:var(--zc-accent); }
@media (max-width: 720px) { .district-guide { grid-template-columns:1fr; } .district-guide-cues { justify-content:flex-start; } }
</style>
