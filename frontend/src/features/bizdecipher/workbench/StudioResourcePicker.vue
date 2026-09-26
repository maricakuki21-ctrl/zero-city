<template>
  <dialog ref="dialog" class="resource-dialog" aria-labelledby="resource-title" @close="emit('update:open', false)">
    <header><div><h2 id="resource-title">模型与资源</h2><p>本次执行</p></div><button title="关闭" aria-label="关闭资源选择" @click="emit('update:open', false)"><X :size="18" /></button></header>
    <nav aria-label="资源来源"><button v-for="item in tabs" :key="item.key" :aria-pressed="tab === item.key" :class="{ active: tab === item.key }" @click="emit('update:tab', item.key)">{{ item.label }}</button></nav>
    <template v-if="tab === 'available'">
      <label class="resource-search"><Search :size="16" /><input v-model="query" aria-label="搜索模型与能力" placeholder="搜索模型与能力" /></label>
      <div class="resource-rows">
        <button v-for="item in filtered" :key="item.id" class="resource-row" :aria-pressed="item.id === selectedId" @click="select(item.id)"><Cpu :size="18" /><span><strong>{{ item.title }}</strong><small>{{ item.canonicalModelId }} · {{ item.summary }}</small></span><span class="resource-price">{{ formatMoney(item.price) }}<small>{{ item.price.unitLabel }}</small></span><Check v-if="item.id === selectedId" :size="16" /></button>
        <p v-if="!filtered.length" class="resource-empty">{{ capabilities.length ? '没有匹配的能力' : '尚未取得可执行能力，请先连接工作区。' }}</p>
      </div>
      <p class="resource-note">按工作区实际返回的能力和报价执行；来源尚未分类。</p>
    </template>
    <div v-else-if="tab === 'templates'" class="resource-rows">
      <button v-for="item in starterTemplates" :key="item.id" class="resource-row" @click="emit('template', item.id); emit('update:open', false)"><Blocks :size="18" /><span><strong>{{ item.title }}</strong><small>{{ item.summary }}</small></span><span class="resource-price">免费模板</span></button>
    </div>
    <div v-else class="resource-empty"><component :is="tab === 'shared' ? Network : Blocks" :size="28" /><h3>{{ tab === 'shared' ? '共享池路由尚未接通' : '创作者工具尚未接通' }}</h3><p>{{ tab === 'shared' ? '此处暂不能直接用共享池发起任务。' : '此处暂不能安装或调用创作者工具。' }}</p><RouterLink :to="tab === 'shared' ? '/account-square' : '/assets'">{{ tab === 'shared' ? '查看共享市场' : '查看能力资产' }}<ArrowUpRight :size="14" /></RouterLink></div>
    <footer><KeyRound :size="14" /><RouterLink to="/keys">管理官方 API Key</RouterLink></footer>
  </dialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { RouterLink } from 'vue-router'
import { ArrowUpRight, Blocks, Check, Cpu, KeyRound, Network, Search, X } from '@lucide/vue'
import { formatMoney, type WorkbenchCapability } from './contracts'
import { starterTemplates } from '@/features/bizdecipher/data/starterTemplates'
type Tab = 'available' | 'shared' | 'creator' | 'templates'
const props = defineProps<{ open: boolean; tab: Tab; capabilities: readonly WorkbenchCapability[]; selectedId: string }>()
const emit = defineEmits<{ 'update:open': [value: boolean]; 'update:tab': [value: Tab]; select: [id: string]; template: [id: string] }>()
const dialog = ref<HTMLDialogElement | null>(null)
const query = ref('')
const tabs = [{ key: 'available', label: '可用能力' }, { key: 'shared', label: '共享池' }, { key: 'templates', label: '内置模板' }, { key: 'creator', label: '创作者' }] as const
const filtered = computed(() => props.capabilities.filter(item => `${item.title} ${item.canonicalModelId} ${item.summary}`.toLocaleLowerCase().includes(query.value.trim().toLocaleLowerCase())))
watch(() => props.open, open => {
  if (open && !dialog.value?.open) dialog.value?.showModal()
  else if (!open && dialog.value?.open) dialog.value?.close()
}, { flush: 'post' })
function select(id: string): void {
  emit('select', id)
  emit('update:open', false)
}
</script>

<style scoped>
.resource-dialog{width:min(640px,calc(100vw - 32px));max-height:80dvh;margin:auto;padding:0;border:1px solid var(--bd-ui-line);border-radius:8px;background:var(--bd-surface);color:var(--bd-text-primary)}
.resource-dialog::backdrop{background:rgb(0 0 0 / .28)}
header,footer,.resource-search,.resource-row{display:flex;align-items:center;gap:12px}
header{justify-content:space-between;padding:20px 24px}h2{font-size:18px;font-weight:650;margin:0}header p,.resource-note{font-size:12px;color:var(--bd-text-secondary)}header button{padding:8px;border-radius:6px}
nav{display:flex;gap:24px;padding:0 24px;border-bottom:1px solid var(--bd-ui-line)}nav button{font-size:14px;padding:12px 0;border-bottom:2px solid transparent}nav button.active{border-color:var(--bd-accent-teal);color:var(--bd-accent-teal)}
.resource-search{margin:20px 24px;padding:8px 12px;border:1px solid var(--bd-ui-line);border-radius:6px}.resource-search input{width:100%;min-width:0;background:transparent;font-size:14px;outline:none}.resource-rows{padding:0 12px;max-height:320px;overflow:auto}.resource-row{width:100%;padding:16px 12px;text-align:left;border-bottom:1px solid var(--bd-ui-line);font-size:14px}.resource-row>span:first-of-type{flex:1;min-width:0}.resource-row small{display:block;color:var(--bd-text-secondary);font-size:12px;margin-top:4px;overflow-wrap:anywhere}.resource-row[aria-pressed=true]{background:var(--bd-canvas);color:var(--bd-accent-teal)}.resource-price{text-align:right}.resource-note{padding:12px 24px}.resource-empty{display:grid;justify-items:center;gap:12px;text-align:center;padding:40px 24px;color:var(--bd-text-secondary);font-size:14px}.resource-empty h3{font-size:16px;color:var(--bd-text-primary)}.resource-empty a,footer a{display:inline-flex;align-items:center;gap:4px;color:var(--bd-accent-teal)}footer{padding:16px 24px;background:var(--bd-canvas);font-size:12px}button:focus-visible,a:focus-visible,input:focus-visible{outline:2px solid var(--bd-accent-teal);outline-offset:3px}
@media(max-width:480px){nav{gap:16px}.resource-row{flex-wrap:wrap}.resource-price{font-size:12px}}
</style>
