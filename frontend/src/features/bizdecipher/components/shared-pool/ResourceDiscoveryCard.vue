<template>
  <article class="resource-card" :class="[`source-${sourceTone}`, { 'has-footer': Boolean($slots.footer) }]">
    <RouterLink :to="to" class="resource-card-link" :aria-label="linkLabel">
      <div class="resource-cover" :class="{ 'is-contain': imageFit === 'contain' }">
        <img
          v-if="resolvedImage"
          :src="resolvedImage"
          :alt="imageAlt"
          :style="{ objectPosition: imagePosition }"
          @error="imageFailed = true"
        />
        <div v-else class="resource-cover-fallback" aria-hidden="true">
          <Archive :size="34" />
        </div>
        <div class="resource-cover-fade" aria-hidden="true" />
        <span class="resource-source-badge" :title="verificationLabel">{{ sourceLabel }}</span>
        <span v-if="status" class="resource-status" :class="`tone-${statusTone}`">
          <span class="resource-status-dot" aria-hidden="true" />
          {{ status }}
        </span>
        <p v-if="coverNote" class="resource-cover-note" :title="coverNote">{{ coverNote }}</p>
      </div>

      <div class="resource-body">
        <p v-if="quote" class="resource-quote" :title="quote">“{{ quote }}”</p>

        <div class="resource-title-row">
          <h3>{{ title }}</h3>
          <span v-if="stateLabel" class="resource-state">{{ stateLabel }}</span>
        </div>
        <p class="resource-meta" :title="[provider, titleNote].filter(Boolean).join(' · ')">
          <span v-if="provider" class="resource-provider">{{ provider }}</span>
          <span v-if="provider && titleNote" aria-hidden="true"> · </span>
          <span v-if="titleNote" class="resource-title-note" :title="titleNote">{{ titleNote }}</span>
        </p>

        <div v-if="tags?.length" class="resource-tags" aria-label="能力标签">
          <span v-for="tag in tags" :key="tag">{{ tag }}</span>
        </div>

        <dl v-if="metrics?.length" class="resource-metrics">
          <div v-for="metric in metrics" :key="metric.label">
            <dt>{{ metric.label }}</dt>
            <dd :class="metric.tone ? `tone-${metric.tone}` : undefined">{{ metric.value }}</dd>
            <small v-if="metric.note">{{ metric.note }}</small>
          </div>
        </dl>

      </div>
    </RouterLink>

    <div v-if="$slots.evidence" class="resource-evidence"><slot name="evidence" /></div>

    <footer v-if="$slots.footer" class="resource-footer">
      <slot name="footer" />
    </footer>
  </article>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { RouterLink } from 'vue-router'
import { Archive } from '@lucide/vue'
import type { ResourceMetric } from './resourceDiscovery'

const props = withDefaults(defineProps<{
  to: string
  title: string
  imageUrl?: string
  fallbackImageUrl?: string
  imageAlt?: string
  imagePosition?: string
  imageFit?: 'cover' | 'contain'
  quote?: string
  titleNote?: string
  provider?: string
  stateLabel?: string
  sourceLabel: string
  sourceTone: 'official' | 'shared'
  status?: string
  statusTone?: 'good' | 'watch' | 'danger' | 'muted'
  coverNote?: string
  tags?: string[]
  metrics?: ResourceMetric[]
  verificationLabel?: string
  linkLabel?: string
}>(), {
  imageUrl: '',
  fallbackImageUrl: '',
  imageAlt: '',
  imagePosition: 'center',
  imageFit: 'cover',
  quote: '',
  titleNote: '',
  provider: '',
  stateLabel: '',
  status: '',
  statusTone: 'muted',
  coverNote: '',
  tags: () => [],
  metrics: () => [],
  verificationLabel: '',
  linkLabel: '',
})

const imageFailed = ref(false)
watch(() => [props.imageUrl, props.fallbackImageUrl], () => {
  imageFailed.value = false
})

const resolvedImage = computed(() => {
  if (imageFailed.value) return props.fallbackImageUrl || ''
  return props.imageUrl || props.fallbackImageUrl || ''
})
</script>

<style scoped>
.resource-card {
  position: relative;
  isolation: isolate;
  display: flex;
  min-width: 0;
  flex-direction: column;
  overflow: hidden;
  border: 1px solid var(--bd-ui-line);
  border-radius: 8px;
  background: var(--bd-surface);
  color: var(--bd-text-primary);
}

.resource-card:hover {
  border-color: color-mix(in srgb, var(--bd-accent-teal) 45%, var(--bd-ui-line));
}

.resource-card-link {
  display: flex;
  min-width: 0;
  flex: 1;
  flex-direction: column;
  color: inherit;
  text-decoration: none;
}

.resource-cover {
  position: relative;
  overflow: hidden;
  height: 140px;
  min-height: 140px;
}

.resource-cover.is-contain {
  background: transparent;
}

.resource-cover img {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  object-fit: cover;
  opacity: 1;
  pointer-events: none;
}

.resource-cover.is-contain img {
  inset: 8px 4px auto auto;
  width: 180px;
  height: 200px;
  object-fit: contain;
  object-position: center top !important;
}

.resource-cover-fallback {
  display: grid;
  height: 100%;
  place-items: center;
  color: var(--bd-text-muted);
  background:
    linear-gradient(135deg, color-mix(in srgb, var(--bd-accent-teal) 10%, transparent), transparent 58%),
    var(--bd-canvas);
}

.resource-cover-fade {
  position: absolute;
  inset: 0;
  pointer-events: none;
  background: linear-gradient(180deg, transparent 52%, color-mix(in srgb, var(--bd-surface) 24%, transparent) 70%, color-mix(in srgb, var(--bd-surface) 78%, transparent) 90%, var(--bd-surface) 100%);
}

.resource-source-badge,
.resource-status {
  position: absolute;
  top: 10px;
  z-index: 1;
  display: inline-flex;
  align-items: center;
  min-height: 24px;
  padding: 0 8px;
  border: 1px solid color-mix(in srgb, var(--bd-ui-line) 75%, transparent);
  border-radius: 4px;
  background: color-mix(in srgb, var(--bd-surface) 90%, transparent);
  color: var(--bd-text-secondary);
  font-size: 11px;
  font-weight: 600;
  backdrop-filter: blur(8px);
}

.resource-source-badge {
  left: 10px;
}

.source-official .resource-source-badge {
  color: var(--bd-accent-blue);
}

.source-shared .resource-source-badge {
  color: var(--bd-accent-teal);
}

.resource-status {
  right: 10px;
  gap: 5px;
}

.resource-status-dot {
  width: 6px;
  height: 6px;
  border-radius: 999px;
  background: currentColor;
}

.tone-good {
  color: var(--bd-accent-teal) !important;
}

.tone-watch {
  color: var(--bd-accent-gold) !important;
}

.tone-danger {
  color: var(--bd-status-danger) !important;
}

.tone-muted {
  color: var(--bd-text-secondary) !important;
}

.resource-cover-note {
  position: absolute;
  right: auto;
  bottom: 6px;
  left: 12px;
  z-index: 1;
  margin: 0;
  max-width: calc(100% - 24px);
  padding: 3px 6px;
  border-left: 2px solid var(--bd-accent-gold);
  background: color-mix(in srgb, var(--bd-surface) 88%, transparent);
  overflow: hidden;
  color: var(--bd-text-primary);
  font-size: 12px;
  font-weight: 600;
  line-height: 1.35;
  text-overflow: ellipsis;
  white-space: nowrap;
  text-shadow: 0 1px 6px color-mix(in srgb, var(--bd-surface) 82%, transparent);
}

.resource-body {
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 4px;
  padding: 0 12px 8px;
}

.resource-quote {
  min-height: 0;
  margin: 0 0 2px;
  overflow: hidden;
  color: var(--bd-text-secondary);
  font-size: 12px;
  line-height: 1.45;
  text-overflow: ellipsis;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
}

.resource-title-row {
  display: flex;
  min-width: 0;
  align-items: flex-start;
  justify-content: space-between;
  gap: 8px;
}

.resource-title-row h3 {
  display: -webkit-box;
  min-width: 0;
  margin: 0;
  overflow: hidden;
  font-size: 16px;
  font-weight: 600;
  line-height: 1.4;
  overflow-wrap: anywhere;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
}

.resource-state {
  flex: 0 0 auto;
  padding: 2px 6px;
  border-radius: 4px;
  background: color-mix(in srgb, var(--bd-accent-teal) 10%, transparent);
  color: var(--bd-accent-teal);
  font-size: 10px;
  font-weight: 600;
}

.resource-meta {
  margin: 0;
  overflow: hidden;
  color: var(--bd-text-secondary);
  font-size: 12px;
  line-height: 1.45;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.resource-tags {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  min-height: 22px;
  flex-wrap: wrap;
  gap: 5px;
}

.resource-tags span {
  max-width: 100%;
  padding: 2px 6px;
  overflow: hidden;
  border-radius: 4px;
  background: var(--bd-canvas);
  color: var(--bd-text-secondary);
  font-size: 12px;
  line-height: 1.3;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.resource-metrics {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 10px;
  margin: 4px 0 0;
  padding-top: 7px;
  border-top: 1px solid var(--bd-ui-line);
}

.resource-metrics div {
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 3px;
}

.resource-metrics dt,
.resource-metrics small {
  color: var(--bd-text-muted);
  font-size: 10px;
  line-height: 1.35;
}

.resource-metrics dd {
  margin: 0;
  overflow-wrap: anywhere;
  color: var(--bd-text-primary);
  font-size: 13px;
  font-weight: 600;
  line-height: 1.35;
}

.resource-evidence { min-height: 38px; padding: 0 12px 6px; margin-top: auto; }
.resource-card-link:focus-visible { outline: 2px solid var(--bd-accent-teal); outline-offset: -2px; }

.resource-footer {
  display: flex;
  min-height: 40px;
  align-items: center;
  gap: 8px;
  padding: 5px 12px;
  border-top: 1px solid var(--bd-ui-line);
}

.resource-footer :deep(.resource-card-spacer) {
  flex: 1;
}

.resource-footer :deep(.resource-card-footnote) {
  color: var(--bd-text-muted);
  font-size: 11px;
}

.resource-footer :deep(a),
.resource-footer :deep(button) {
  min-height: 30px;
}

@media (max-width: 520px) {
  .resource-cover {
    height: 140px;
    min-height: 140px;
  }

  .resource-metrics {
    gap: 7px;
  }
}
</style>
