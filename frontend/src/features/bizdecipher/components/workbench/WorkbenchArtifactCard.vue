<template>
  <div class="wb-artifact">
    <div v-if="artifactPreview" class="wb-artifact-ready">
      <h3 class="wb-artifact-title">{{ title }}</h3>
      <p data-testid="workbench-copy" class="wb-artifact-copy wb-break-anywhere">{{ artifactPreview }}</p>
    </div>
    <div v-else data-testid="workbench-empty-artifact" class="wb-artifact-empty">
      运行成功后，工件会在这里展示预览、标题与后续保存入口。
    </div>
  </div>
</template>

<script setup lang="ts">
defineProps<{
  title: string
  artifactPreview: string
}>()
</script>

<style scoped>
.wb-artifact-ready,
.wb-artifact-empty {
  border-radius: 18px;
  border: 1px solid var(--wb-line);
  background: var(--wb-overlay, color-mix(in srgb, var(--module-panel-raised) 9%, transparent));
  padding: 1rem;
}

.wb-artifact-title {
  margin: 0 0 0.6rem;
  color: var(--wb-ink-strong);
  font-size: 0.98rem;
  font-weight: 800;
}

.wb-artifact-copy,
.wb-artifact-empty {
  margin: 0;
  color: var(--wb-ink);
  line-height: 1.75;
}

.wb-break-anywhere {
  overflow-wrap: anywhere;
  word-break: break-word;
}
</style>
