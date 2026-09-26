<template>
  <section class="wb-panel zc-material-soft">
    <header class="wb-panel-head">
      <div>
        <p v-if="kicker" class="wb-panel-kicker">{{ kicker }}</p>
        <h2 class="wb-panel-title">{{ title }}</h2>
      </div>
      <div v-if="$slots.meta" class="wb-panel-meta">
        <slot name="meta" />
      </div>
    </header>
    <div class="wb-panel-body">
      <slot />
    </div>
  </section>
</template>

<script setup lang="ts">
defineProps<{
  title: string
  kicker?: string
}>()
</script>

<style scoped>
.wb-panel {
  border-radius: 24px;
  padding: 1.25rem;
  color: var(--wb-ink);
}

.wb-panel-head {
  display: flex;
  justify-content: space-between;
  gap: 1rem;
  align-items: flex-start;
}

.wb-panel-kicker {
  margin: 0;
  color: var(--wb-accent);
  font-size: 0.72rem;
  font-weight: 800;
  letter-spacing: 0.16em;
  text-transform: uppercase;
}

.wb-panel-title {
  margin: 0.3rem 0 0;
  font-size: 1.05rem;
  font-weight: 800;
  color: var(--wb-ink-strong);
}

.wb-panel-body {
  margin-top: 1rem;
}
</style>
