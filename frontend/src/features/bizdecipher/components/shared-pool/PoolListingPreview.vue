<template>
  <article class="pool-preview" aria-label="市场卡片预览">
    <div class="preview-art"><img :src="image || zeroCityMascots.sharedPoolOwner" alt="" :class="{ mascot: !image }" /></div>
    <div class="preview-body">
      <span class="preview-state">{{ listed ? '已上架' : '未发布' }}</span>
      <h3>{{ name.trim() || '你的共享池' }}</h3>
      <p>{{ description?.trim() || '添加简介，让大家更了解你的资源。' }}</p>
      <div class="preview-models">
        <span v-for="model in models.slice(0, 3)" :key="model">{{ model }}</span>
        <span v-if="models.length > 3">+{{ models.length - 3 }}</span>
        <span v-if="!models.length" class="pending">等待接入模型</span>
      </div>
      <footer><span>调用价格</span><strong>{{ price || '待配置' }}</strong></footer>
    </div>
  </article>
</template>

<script setup lang="ts">
import { zeroCityMascots } from '@/features/bizdecipher/constants/zeroCityMascots'
withDefaults(defineProps<{
  name: string
  description?: string
  models?: string[]
  image?: string
  price?: string
  listed?: boolean
}>(), { models: () => [], listed: false })
</script>

<style scoped>
.pool-preview { position: relative; isolation: isolate; overflow: hidden; min-width: 0; border: 1px solid var(--bd-ui-line); border-radius: 8px; background: var(--bd-surface); color: var(--bd-text-primary); }
.preview-art { position: relative; height: 140px; background: var(--bd-canvas); overflow: hidden; mask-image: linear-gradient(#000 45%, transparent); }
.preview-art img { width: 100%; height: 180px; object-fit: cover; object-position: center 30%; }
.preview-art img.mascot { display: block; object-fit: contain; height: 130px; width: 130px; margin: 6px auto; }
.preview-body { position: relative; padding: 0 20px 16px; margin-top: -24px; }
.preview-state { font-size: 12px; color: var(--bd-accent-teal); }
h3 { font-size: 18px; font-weight: 600; line-height: 1.4; margin: 8px 0; overflow-wrap: anywhere; }
p { font-size: 13px; color: var(--bd-text-secondary); line-height: 1.6; margin: 0; overflow-wrap: anywhere; }
.preview-models { display: flex; gap: 6px; flex-wrap: wrap; min-height: 30px; margin: 16px 0; }
.preview-models span { padding: 3px 6px; font-size: 12px; background: var(--bd-canvas); border-radius: 4px; overflow-wrap: anywhere; max-width: 100%; }
.preview-models .pending { padding-left: 0; background: transparent; color: var(--bd-text-secondary); }
footer { display: flex; gap: 12px; justify-content: space-between; padding-top: 12px; border-top: 1px solid var(--bd-ui-line); font-size: 12px; }
footer strong { color: var(--bd-text-primary); font-weight: 500; }
</style>
