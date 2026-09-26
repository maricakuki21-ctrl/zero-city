<template>
  <dl class="wb-lineage" data-testid="workbench-lineage">
    <div v-for="row in rows" :key="row.key" class="wb-lineage-row">
      <dt>{{ row.label }}</dt>
      <dd class="wb-break-anywhere">{{ row.value || '未返回' }}</dd>
    </div>
  </dl>
</template>

<script setup lang="ts">
import type { WorkbenchLineage } from '../../workbench/contracts'

const props = defineProps<{ readonly lineage: WorkbenchLineage }>()
const rows = [
  { key: 'request', label: 'canonical request', get value() { return props.lineage.canonicalRequestId } },
  { key: 'usage', label: 'usage event', get value() { return props.lineage.canonicalUsageEventId } },
  { key: 'quote', label: 'accepted quote', get value() { return props.lineage.acceptedQuoteId } },
  { key: 'quote-sha', label: 'quote SHA', get value() { return props.lineage.acceptedQuoteSha } },
  { key: 'journal', label: 'journal', get value() { return props.lineage.journalId } },
  { key: 'media', label: 'media task', get value() { return props.lineage.mediaTaskId } },
  { key: 'media-event', label: 'media business event', get value() { return props.lineage.canonicalMediaBusinessEventId } },
  { key: 'runner', label: 'runner job', get value() { return props.lineage.runnerJobId } },
  { key: 'adapter', label: 'adapter digest', get value() { return props.lineage.adapterDigest } },
  { key: 'capability', label: 'capability digest', get value() { return props.lineage.capabilityDigest } },
  { key: 'artifact', label: 'artifact', get value() { return props.lineage.artifactId } },
  { key: 'artifact-sha', label: 'artifact digest', get value() { return props.lineage.artifactDigest } },
  { key: 'upstream', label: 'upstream task', get value() { return props.lineage.upstreamTaskId } },
]
</script>

<style scoped>
.wb-lineage {
  display: grid;
  gap: var(--bd-space-2, 8px);
  margin: var(--bd-space-4, 16px) 0 0;
}

.wb-lineage-row {
  display: grid;
  grid-template-columns: minmax(9rem, 0.35fr) minmax(0, 1fr);
  gap: var(--bd-space-3, 12px);
  padding-block: var(--bd-space-1, 4px);
  border-top: var(--bd-line-width, 1px) solid var(--wb-line);
}

.wb-lineage dt {
  color: var(--wb-muted);
  font-size: var(--bd-type-caption, 0.75rem);
  font-weight: var(--bd-weight-overline, 750);
  text-transform: uppercase;
}

.wb-lineage dd {
  margin: 0;
  color: var(--wb-ink);
  font-family: var(--bd-font-mono, JetBrains Mono, monospace);
  font-size: var(--bd-type-caption, 0.75rem);
}

.wb-break-anywhere { overflow-wrap: anywhere; word-break: break-word; }

@media (max-width: 600px) {
  .wb-lineage-row { grid-template-columns: 1fr; gap: var(--bd-space-1, 4px); }
}
</style>
