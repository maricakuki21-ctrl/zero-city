<template>
  <div class="bd-public-surface landing">
    <PublicSiteHeader />
    <main>
      <section class="landing-hero" aria-labelledby="landing-title">
        <picture>
          <source media="(max-width: 600px)" srcset="/brand/editorial/hero-mobile.jpg" />
          <img class="hero-scene" src="/brand/editorial/hero.jpg" alt="零号城向导、守钥人和档案员" fetchpriority="high" width="1920" height="820" />
        </picture>
        <div class="hero-copy">
          <p class="hero-eyebrow">创作 · 协作 · 让能力被看见</p>
          <h1 id="landing-title">BizDecipher</h1>
          <p class="hero-subtitle">把想法做成作品，<br />把能力交给世界。</p>
          <p class="hero-note">一个工作的地方，也是一群不肯轻易放弃的人。</p>
          <div class="hero-actions">
            <RouterLink to="/operator" class="public-action">我想完成一件事<ArrowUpRight :size="17" aria-hidden="true" /></RouterLink>
            <RouterLink to="/marketplace" class="public-action public-action--secondary">我想提供能力<ArrowRight :size="17" aria-hidden="true" /></RouterLink>
          </div>
        </div>
        <nav class="hero-foot" aria-label="探索产品">
          <RouterLink to="/operator">创作工作台</RouterLink><RouterLink to="/marketplace">双边市场</RouterLink><RouterLink to="/community">零号城</RouterLink>
        </nav>
      </section>
      <section class="landing-section" aria-labelledby="residents-title">
        <header class="section-heading">
          <div><h2 id="residents-title">先认识几位不服气的居民。</h2><p>同一座城市，不同的坚持。</p></div>
          <RouterLink to="/zero-city/cards" class="public-text-link">翻开卡册<ArrowUpRight :size="17" aria-hidden="true" /></RouterLink>
        </header>
        <div class="resident-grid">
          <button v-for="card in residents" :key="card.id" type="button" class="resident" :aria-label="`认识${card.name}`" @click="openResident(card)">
            <img :src="`/brand/editorial/${card.id}-crop.jpg`" :alt="card.name" loading="lazy" width="640" height="460" />
            <h3>{{ card.name }}</h3><p>{{ card.line }}</p><small>{{ card.faction }}</small>
          </button>
        </div>
      </section>
      <section class="landing-paths" aria-labelledby="paths-title">
        <div class="paths-intro"><p class="hero-eyebrow">从一件具体的事开始</p><h2 id="paths-title">做出一点东西，<br />留下一个名字。</h2></div>
        <div class="path-list">
          <RouterLink v-for="path in paths" :key="path.to" :to="path.to" class="path-link">
            <component :is="path.icon" :size="22" aria-hidden="true" />
            <span><strong>{{ path.title }}</strong><small>{{ path.description }}</small></span><ArrowUpRight :size="20" aria-hidden="true" />
          </RouterLink>
        </div>
      </section>
    </main>
    <footer class="landing-footer">
      <span>BizDecipher · 把能力交给世界。</span>
      <nav aria-label="更多入口"><RouterLink to="/account-square">共享资源</RouterLink><RouterLink to="/assets">能力资产</RouterLink><RouterLink to="/docs">文档与帮助</RouterLink></nav>
    </footer>
    <dialog ref="residentDialog" class="resident-dialog" aria-labelledby="resident-name" @close="selectedResident = null" @click="closeOnBackdrop">
      <template v-if="selectedResident">
        <header><span>{{ selectedResident.faction }}</span><button type="button" aria-label="关闭角色详情" title="关闭" autofocus @click="residentDialog?.close()"><X :size="20" /></button></header>
        <img :src="`/brand/editorial/${selectedResident.id}-crop.jpg`" :alt="selectedResident.name" width="640" height="460" />
        <h2 id="resident-name">{{ selectedResident.name }}</h2><blockquote>{{ selectedResident.line }}</blockquote><p>{{ selectedResident.lore }}</p>
        <RouterLink to="/zero-city/cards" class="public-action" @click="residentDialog?.close()">去零号城卡册<ArrowRight :size="16" /></RouterLink>
      </template>
    </dialog>
  </div>
</template>
<script setup lang="ts">
import { nextTick, onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { ArrowRight, ArrowUpRight, Boxes, PanelsTopLeft, Store, X } from '@lucide/vue'
import PublicSiteHeader from '@/components/layout/PublicSiteHeader.vue'
import { ZERO_CITY_CARD_MANIFEST } from '@/constants/zeroCityCardManifest'
import { useAppStore } from '@/stores'
const appStore = useAppStore()
onMounted(() => { void appStore.fetchPublicSettings() })
const residentIds = ['hope_inventory_keeper', 'miracle_tester', 'cloud_patch_worker', 'hope_night_shift'] as const
const residents = residentIds.map(id => ({ id, ...ZERO_CITY_CARD_MANIFEST[id] }))
type Resident = typeof residents[number]
const selectedResident = ref<Resident | null>(null)
const residentDialog = ref<HTMLDialogElement | null>(null)
async function openResident(card: Resident) {
  selectedResident.value = card
  await nextTick()
  residentDialog.value?.showModal()
}
function closeOnBackdrop(event: MouseEvent) {
  if (event.target === residentDialog.value) residentDialog.value?.close()
}
const paths = [
  { to: '/operator', title: '开始创作', description: '从一个想法，到一份自己的作品。', icon: PanelsTopLeft },
  { to: '/marketplace', title: '找到合作的人', description: '发现服务、公开需求，也让别人认识你的能力。', icon: Store },
  { to: '/assets', title: '发现能力资产', description: '看看工具、工作流和创作者的产品。', icon: Boxes },
]
</script>
<style scoped>
.landing { min-height: 100svh; }
.landing-hero { position: relative; display: flex; align-items: center; justify-content: center; min-height: 530px; height: calc(100svh - 165px); max-height: 720px; text-align: center; isolation: isolate; }
.hero-scene { position: absolute; inset: 0; z-index: -1; width: 100%; height: 100%; object-fit: cover; object-position: center; }
.hero-copy { max-width: 740px; padding: 15px 20px 55px; }
.hero-eyebrow { font-size: 12px; color: var(--public-muted); }
.hero-copy h1 { font-size: 80px; line-height: 1.15; font-weight: 600; margin: 18px 0; }
.hero-subtitle { font-size: 27px; font-weight: 500; line-height: 1.6; }
.hero-note { color: var(--public-muted); font-size: 14px; margin-top: 14px; }
.hero-actions { display: flex; justify-content: center; flex-wrap: wrap; gap: 12px; margin-top: 27px; }
.hero-actions .public-action { min-height: 45px; }
.hero-foot { position: absolute; bottom: 20px; display: flex; gap: 22px; font-size: 11px; }
.hero-foot a { color: var(--public-muted); }
.hero-foot a:hover { color: var(--public-action); }
.landing-section { padding: 34px 5% 64px; border-top: 1px solid var(--public-line); }
.section-heading { display: flex; align-items: end; justify-content: space-between; gap: 24px; margin-bottom: 25px; }
.section-heading h2 { font-size: 26px; font-weight: 600; line-height: 1.4; }
.section-heading p { font-size: 13px; color: var(--public-muted); margin-top: 6px; }
.section-heading > a { flex-shrink: 0; }
.resident-grid { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 22px; }
.resident { border: 0; background: none; padding: 0; text-align: left; color: var(--public-text); cursor: pointer; align-self: start; }
.resident:nth-child(even) { margin-top: 30px; }
.resident img { display: block; width: 100%; height: auto; aspect-ratio: 640 / 460; object-fit: cover; border-radius: var(--public-radius); }
.resident h3 { font-size: 16px; font-weight: 600; margin-top: 12px; }
.resident p { margin-top: 5px; font-size: 12px; color: var(--public-muted); }
.resident small { display: block; margin-top: 6px; font-size: 11px; color: #88613b; }
.resident:hover h3 { color: var(--public-action); }
.landing-paths { border-top: 1px solid var(--public-line); background: var(--public-field); padding: 55px 5%; display: grid; grid-template-columns: 1fr 1fr; gap: 50px; }
.paths-intro h2 { font-size: 30px; line-height: 1.6; font-weight: 500; margin-top: 14px; }
.path-link { display: flex; align-items: center; gap: 20px; padding: 22px 0; border-bottom: 1px solid var(--public-line); color: var(--public-text); }
.path-link:first-child { padding-top: 0; }
.path-link span { flex: 1; min-width: 0; }
.path-link svg { flex-shrink: 0; }
.path-link strong { font-size: 17px; font-weight: 550; display: block; }
.path-link small { display: block; color: var(--public-muted); margin-top: 4px; font-size: 12px; }
.path-link:hover strong { color: var(--public-action); }
.landing-footer { display: flex; justify-content: space-between; flex-wrap: wrap; gap: 20px; padding: 30px 5%; border-top: 1px solid var(--public-line); font-size: 12px; color: var(--public-muted); }
.landing-footer nav { display: flex; gap: 20px; flex-wrap: wrap; }
.landing-footer a { color: inherit; }
.resident-dialog { width: min(420px, calc(100% - 32px)); max-height: calc(100svh - 32px); padding: 24px; border: 1px solid var(--public-line); border-radius: 8px; background: var(--public-canvas); color: var(--public-text); }
.resident-dialog::backdrop { background: #14202880; }
.resident-dialog header { display: flex; align-items: center; justify-content: space-between; gap: 16px; margin-bottom: 14px; color: var(--public-muted); font-size: 12px; }
.resident-dialog header button { display: grid; place-items: center; width: 34px; height: 34px; border: 0; border-radius: 6px; background: var(--public-field); color: var(--public-text); cursor: pointer; }
.resident-dialog > img { display: block; width: 100%; height: auto; border-radius: 6px; }
.resident-dialog h2 { font-size: 23px; margin-top: 18px; }
.resident-dialog blockquote { margin: 10px 0; font-size: 16px; }
.resident-dialog p { color: var(--public-muted); font-size: 13px; margin-bottom: 20px; }
@media (max-width: 1100px) {
  .hero-scene { object-fit: contain; object-position: center bottom; }
  .hero-copy { padding-bottom: 100px; }
  .hero-copy h1 { font-size: 64px; }
}
@media (max-width: 800px) {
  .landing-hero { min-height: 550px; max-height: 680px; }
  .hero-copy { padding-bottom: 160px; }
  .resident-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .landing-paths { grid-template-columns: 1fr; gap: 30px; padding: 36px 5%; }
}
@media (max-width: 600px) {
  .landing-hero { height: calc(100svh - 145px); min-height: 520px; max-height: 640px; }
  .hero-scene { top: auto; bottom: 25px; height: 245px; object-fit: contain; object-position: center bottom; }
  .hero-copy { width: 100%; padding: 0 16px 155px; }
  .hero-copy h1 { font-size: 44px; margin: 16px 0; }
  .hero-subtitle { font-size: 21px; }
  .hero-note { max-width: 280px; margin: 12px auto 0; font-size: 12px; }
  .hero-eyebrow { font-size: 11px; }
  .hero-actions { gap: 8px; margin-top: 22px; }
  .hero-actions .public-action { padding: 9px 11px; font-size: 11px; min-height: 40px; }
  .hero-actions svg { width: 14px; }
  .hero-foot { bottom: 14px; }
  .landing-section { padding: 27px 20px 40px; }
  .section-heading { gap: 14px; align-items: start; }
  .section-heading h2 { font-size: 22px; }
  .section-heading > a { font-size: 11px; margin-top: 5px; }
  .resident-grid { gap: 22px 12px; }
  .resident:nth-child(even) { margin-top: 16px; }
  .resident h3 { font-size: 14px; }
  .resident p { font-size: 11px; }
}
@media (max-height: 680px) and (min-width: 801px) { .landing-hero { min-height: 430px; } }
</style>
