<template>
  <img v-if="card && !failed" :key="card.imageUrl" class="pool-card-backdrop" :src="card.imageUrl" alt="" aria-hidden="true" @error="failed = true" />
  <span v-if="card && failed" class="pool-card-failure" role="status">
    收藏卡背景暂不可用，已保留你的选择
    <button type="button" @click.stop="failed = false">重试</button>
  </span>
</template>
<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { resolveSharedPoolCardPresentation } from './sharedPoolPresentation'
const props = defineProps<{ cardKey?: string | null; rarity?: string | null }>()
const card = computed(() => resolveSharedPoolCardPresentation({ cardKey: props.cardKey, cardRarity: props.rarity }))
const failed = ref(false)
watch(() => card.value?.imageUrl, () => { failed.value = false })
</script>
<style scoped>
.pool-card-backdrop{position:absolute;inset:0;width:100%;height:100%;object-fit:cover;object-position:center 30%;pointer-events:none}
.pool-card-failure{position:relative;z-index:2;font-size:12px;color:var(--text);background:var(--bg);padding:6px 10px;border-radius:6px}
.pool-card-failure button{margin-left:8px;color:var(--teal);text-decoration:underline}
.pool-card-failure button:focus-visible{outline:2px solid currentColor;outline-offset:3px}
</style>
