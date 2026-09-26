<script setup lang="ts">
withDefaults(defineProps<{
  eyebrow?: string
  title: string
  description?: string
}>(), {
  eyebrow: '',
  description: ''
})
</script>

<template>
  <div>
    <p
      v-if="eyebrow"
      class="text-xs font-semibold uppercase tracking-[0.25em] text-teal-700"
    >
      {{ eyebrow }}
    </p>
    <h1 class="mt-3 max-w-3xl text-3xl font-semibold tracking-tight text-slate-950 md:text-4xl">
      {{ title }}
    </h1>
    <p v-if="description" class="mt-4 max-w-2xl text-base leading-7 text-slate-600">
      {{ description }}
    </p>
  </div>
</template>
