<script setup lang="ts">
import { computed, ref } from 'vue'
import { ArrowUpRight, Code2, Film, Gamepad2, Image, Workflow } from '@lucide/vue'
import { starterTemplates } from '@/features/bizdecipher/data/starterTemplates'
const category = ref('全部')
const categories = ['全部', '图片', '视频', '游戏', '代码', '工作流']
const icons = { 图片: Image, 视频: Film, 游戏: Gamepad2, 代码: Code2, 工作流: Workflow }
const items = computed(() => starterTemplates.filter(item => category.value === '全部' || item.category === category.value))
</script>
<template>
  <section class="starter-templates">
    <header><div><h2>从一个模板开始</h2><p>平台任务模板 · 免费起稿，实际运行按工作台报价</p></div><RouterLink to="/operator">打开工作台 <ArrowUpRight :size="15" /></RouterLink></header>
    <nav aria-label="模板类型"><button v-for="item in categories" :key="item" :aria-pressed="category === item" :class="{ active: category === item }" @click="category = item">{{ item }}</button></nav>
    <div class="template-grid"><article v-for="item in items" :key="item.id"><div class="template-icon"><component :is="icons[item.category]" :size="22" /></div><div><span>{{ item.category }}</span><h3>{{ item.title }}</h3><p>{{ item.summary }}</p><RouterLink :to="{ path: '/operator', query: { template: item.id } }">使用模板 <ArrowUpRight :size="14" /></RouterLink></div></article></div>
  </section>
</template>
<style scoped>
.starter-templates{color:var(--bd-text-primary);padding:0 0 28px;border-bottom:1px solid var(--bd-ui-line);margin-bottom:28px}.starter-templates header{display:flex;gap:16px;align-items:center;justify-content:space-between;flex-wrap:wrap}.starter-templates h2{font-size:18px;font-weight:650}.starter-templates header p{font-size:12px;color:var(--bd-text-secondary);margin-top:8px}.starter-templates a{display:inline-flex;align-items:center;gap:6px;color:var(--bd-accent-teal);font-size:12px}.starter-templates nav{display:flex;gap:8px;margin:20px 0;flex-wrap:wrap}.starter-templates nav button{padding:8px 12px;font-size:12px;border-radius:6px}.starter-templates nav .active{background:var(--bd-surface);color:var(--bd-accent-teal)}.template-grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(min(280px,100%),1fr));gap:16px}.template-grid article{display:flex;gap:16px;padding:20px;background:var(--bd-surface);border:1px solid var(--bd-ui-line);border-radius:8px;min-width:0}.template-icon{width:40px;height:40px;flex-shrink:0;border-radius:6px;display:grid;place-items:center;background:var(--bd-canvas);color:var(--bd-accent-blue)}.template-grid article:nth-child(3n+2) .template-icon{color:var(--bd-accent-teal)}.template-grid article:nth-child(3n) .template-icon{color:var(--bd-accent-gold)}.template-grid span{font-size:11px;color:var(--bd-text-secondary)}.template-grid h3{font-size:16px;font-weight:650;margin:4px 0 8px}.template-grid p{font-size:12px;line-height:1.8;color:var(--bd-text-secondary);margin-bottom:16px}.starter-templates :focus-visible{outline:2px solid var(--bd-accent-teal);outline-offset:3px}
</style>
