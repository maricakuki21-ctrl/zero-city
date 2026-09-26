<template>
  <DialogRoot :open="open" @update:open="$emit('update:open', $event)">
    <DialogPortal>
      <DialogOverlay class="ranking-dialog-overlay" />
      <DialogContent class="ranking-dialog">
        <header>
          <div>
            <span>{{ definition?.windowLabel || '当前范围' }} · {{ dataScope }}</span>
            <DialogTitle as="h2">{{ definition?.title || '榜单详情' }}</DialogTitle>
            <DialogDescription>{{ definition?.description || '当前没有可用的榜单定义。' }}</DialogDescription>
          </div>
          <DialogClose class="ranking-dialog-close" aria-label="关闭榜单详情">×</DialogClose>
        </header>

        <div v-if="rows.length" class="ranking-dialog-list" role="list">
          <button v-for="row in rows" :key="`${row.rank}-${row.label}`" type="button" role="listitem" @click="$emit('select', row)">
            <span class="ranking-dialog-rank">{{ row.rank }}</span>
            <span><strong>{{ row.label }}</strong><small>{{ row.secondary }}</small></span>
            <b>{{ row.value }}</b>
          </button>
        </div>
        <div v-else class="ranking-dialog-empty">
          <strong>暂无可验证记录</strong>
          <p>这个席位不会用模拟居民或未确认数据填充。</p>
        </div>

        <footer>
          <span>{{ definition?.weeklyRule }}</span>
          <button v-if="definition?.route" class="community-btn community-btn-secondary" type="button" @click="$emit('related')">打开相关城区</button>
        </footer>
      </DialogContent>
    </DialogPortal>
  </DialogRoot>
</template>

<script setup lang="ts">
import { DialogClose, DialogContent, DialogDescription, DialogOverlay, DialogPortal, DialogRoot, DialogTitle } from 'reka-ui'
import type { ZeroCityRankingDefinition, ZeroCityRankingRow } from '../../data/zeroCityRankings'

withDefaults(defineProps<{
  open: boolean
  definition?: ZeroCityRankingDefinition
  rows: readonly ZeroCityRankingRow[]
  dataScope?: string
}>(), {
  dataScope: '基于当前可见动态',
})

defineEmits<{
  'update:open': [open: boolean]
  select: [row: ZeroCityRankingRow]
  related: []
}>()

</script>

<style scoped>
.ranking-dialog-overlay { position:fixed; inset:0; z-index:250; background:color-mix(in srgb,var(--bd-brand-ink) 48%,transparent); }
.ranking-dialog { position:fixed; z-index:251; top:50%; left:50%; display:grid; width:min(42rem,calc(100vw - 2rem)); max-height:min(42rem,calc(100vh - 2rem)); overflow:auto; transform:translate(-50%,-50%); border:var(--bd-line-width) solid var(--zc-line); border-radius:var(--bd-radius-md); background:var(--zc-card); color:var(--zc-text); box-shadow:0 1.5rem 4rem color-mix(in srgb,var(--bd-brand-ink) 28%,transparent); }
header { display:flex; align-items:flex-start; justify-content:space-between; gap:var(--bd-space-4); padding:var(--bd-space-5); border-bottom:var(--bd-line-width) solid var(--zc-line); }
header > div { display:grid; gap:var(--bd-space-2); }
header span, header p, footer span { color:var(--zc-muted); font-size:var(--bd-type-caption); line-height:var(--bd-line-body-small); }
header h2 { color:var(--zc-text-strong); font-size:var(--zc-city-section); line-height:var(--zc-city-line-heading); }
.ranking-dialog-close { display:grid; width:2rem; height:2rem; flex:0 0 auto; place-items:center; border:var(--bd-line-width) solid var(--zc-line); border-radius:var(--bd-radius-sm); color:var(--zc-muted); font-size:1.125rem; }
.ranking-dialog-close:hover, .ranking-dialog-close:focus-visible { border-color:var(--zc-accent); color:var(--zc-text-strong); }
.ranking-dialog-list { display:grid; }
.ranking-dialog-list button { display:grid; grid-template-columns:2rem minmax(0,1fr) auto; align-items:center; gap:var(--bd-space-3); padding:var(--bd-space-3) var(--bd-space-5); border-bottom:var(--bd-line-width) solid var(--zc-line); text-align:left; }
.ranking-dialog-list button:hover, .ranking-dialog-list button:focus-visible { background:color-mix(in srgb,var(--zc-accent) 7%,transparent); }
.ranking-dialog-list button > span:nth-child(2) { display:grid; gap:var(--bd-space-1); min-width:0; }
.ranking-dialog-list strong, .ranking-dialog-list b, .ranking-dialog-empty strong { color:var(--zc-text-strong); font-size:var(--bd-type-caption); }
.ranking-dialog-list small, .ranking-dialog-empty p { color:var(--zc-muted); font-size:var(--bd-type-overline); line-height:var(--bd-line-caption); }
.ranking-dialog-rank { display:grid; width:2rem; height:2rem; place-items:center; background:var(--zc-city-level); color:var(--zc-accent); font-size:var(--bd-type-overline); font-weight:var(--bd-weight-emphasis); }
.ranking-dialog-empty { display:grid; gap:var(--bd-space-2); padding:var(--bd-space-8) var(--bd-space-5); text-align:center; }
footer { display:flex; align-items:center; justify-content:space-between; gap:var(--bd-space-4); padding:var(--bd-space-4) var(--bd-space-5); }
footer span { max-width:28rem; }
footer .community-btn { display:inline-flex; min-height:2.25rem; flex:0 0 auto; align-items:center; justify-content:center; padding:var(--bd-space-2) var(--bd-space-3); border:var(--bd-line-width) solid var(--zc-line); border-radius:var(--bd-radius-sm); background:transparent; color:var(--zc-text-strong); font-size:var(--bd-type-caption); font-weight:var(--bd-weight-emphasis); }
footer .community-btn:hover, footer .community-btn:focus-visible { border-color:var(--zc-accent); color:var(--zc-accent); }
.ranking-dialog-close:focus-visible, .ranking-dialog-list button:focus-visible { outline:var(--bd-focus-ring-width) solid var(--zc-accent); outline-offset:calc(var(--bd-motion-shift) * -1); }
@media (max-width:560px) { .ranking-dialog-list button { grid-template-columns:2rem minmax(0,1fr); } .ranking-dialog-list b { grid-column:2; } footer { align-items:flex-start; flex-direction:column; } }
</style>
