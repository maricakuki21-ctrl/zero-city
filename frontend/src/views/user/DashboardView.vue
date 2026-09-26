<template>
  <AppLayout>
    <main class="work-overview">
      <header class="overview-head">
        <h1>概览</h1>
        <div class="overview-actions">
          <button class="overview-icon" type="button" aria-label="刷新概览" title="刷新概览" :disabled="loading" @click="refresh"><RefreshCw :size="17" :class="{ spinning: loading }" /></button>
          <RouterLink :to="{ path: '/operator', query: { new: '1' } }" class="overview-button primary"><Plus :size="16" />新建任务</RouterLink>
        </div>
      </header>

      <div v-if="errors.length" class="overview-errors" role="alert">
        <span v-for="message in errors" :key="message">{{ message }}</span>
        <button type="button" :disabled="loading" @click="refresh">重试</button>
      </div>

      <section class="overview-summary" aria-label="账户与工作摘要">
        <div><span>待继续任务 · 此浏览器</span><strong>{{ draftError ? '未知' : unfinishedCount }}</strong></div>
        <div><span>已确认成果 · 此浏览器</span><strong>{{ draftError ? '未知' : results.length }}</strong></div>
        <RouterLink to="/wallet"><span>站内余额</span><strong>{{ balance }}</strong></RouterLink>
        <RouterLink to="/account-square/my"><span>有效共享席位</span><strong>{{ seatsCount ?? (loading ? '加载中' : '未知') }}</strong></RouterLink>
      </section>

      <div class="overview-body">
        <div class="overview-main">
          <section class="overview-section" aria-labelledby="recent-tasks-title">
            <header class="overview-section-head">
              <h2 id="recent-tasks-title">最近任务</h2>
              <span>此浏览器</span>
              <RouterLink to="/operator">打开工作台<ArrowUpRight :size="15" /></RouterLink>
            </header>
            <div class="task-filters">
              <label class="overview-search"><Search :size="16" /><input v-model="query" type="search" placeholder="搜索任务" aria-label="搜索最近任务" /></label>
              <select v-model="taskFilter" aria-label="任务状态"><option value="all">全部任务</option><option value="draft">待继续</option><option value="verified">已确认成果</option></select>
            </div>
            <div v-if="draftError" class="overview-empty"><p>本地任务暂时无法读取</p><button class="overview-button" type="button" @click="refreshDrafts">重新读取</button></div>
            <div v-else-if="!drafts.length" class="overview-empty">
              <img :src="zeroCityMascots.gatewayOperator" alt="" width="76" height="76" />
              <h3>从一个想法开始</h3>
              <RouterLink :to="{ path: '/operator', query: { new: '1' } }" class="overview-button"><Plus :size="16" />新建任务</RouterLink>
            </div>
            <div v-else-if="!filteredTasks.length" class="overview-empty"><p>没有匹配的任务</p><button class="overview-button" type="button" @click="query = ''; taskFilter = 'all'">重置筛选</button></div>
            <div v-else class="overview-table-wrap">
              <table class="overview-table">
                <thead><tr><th scope="col">任务</th><th scope="col">类型</th><th scope="col">状态</th><th scope="col">更新时间</th></tr></thead>
                <tbody>
                  <tr v-for="draft in visibleTasks" :key="draft.id">
                    <td><RouterLink :to="taskLink(draft)" class="task-name">{{ draft.title || '未命名任务' }}</RouterLink></td>
                    <td>{{ draft.images?.length ? '图片与文本' : '文本' }}</td>
                    <td><span :class="{ 'confirmed-status': hasConfirmedDraftResult(draft) }">{{ hasConfirmedDraftResult(draft) ? '已确认成果' : draft.answer ? '结果待确认' : '待继续' }}</span></td>
                    <td><time :datetime="draft.updatedAt || draft.completedAt">{{ formatTime(draft.updatedAt || draft.completedAt) }}</time></td>
                  </tr>
                </tbody>
              </table>
              <button v-if="visibleTasks.length < filteredTasks.length" class="overview-button load-more" type="button" @click="visibleLimit += 10">显示更多</button>
            </div>
          </section>

          <section class="overview-section" aria-labelledby="recent-results-title">
            <header class="overview-section-head"><h2 id="recent-results-title">最近成果</h2><RouterLink to="/assets">能力资产<ArrowUpRight :size="15" /></RouterLink></header>
            <p v-if="!results.length" class="overview-no-results">{{ draftError ? '任务读取失败，暂无法展示成果' : '还没有已确认的本地成果' }}</p>
            <div v-else class="overview-results">
              <article v-for="draft in results.slice(0, 4)" :key="draft.id" class="overview-result">
                <RouterLink :to="taskLink(draft)" class="result-art">
                  <img v-if="draft.images?.[0] && !failedImages.has(draft.id)" :src="draft.images[0]" :alt="draft.title" loading="lazy" @error="failedImages.add(draft.id)" />
                  <FileText v-else :size="32" aria-hidden="true" />
                </RouterLink>
                <h3><RouterLink :to="taskLink(draft)">{{ draft.title }}</RouterLink></h3>
                <p>{{ draft.answer.slice(0, 100) }}</p>
                <RouterLink v-if="draft.savedAssetId" :to="{ path: '/assets', query: { tab: 'mine', asset: String(draft.savedAssetId) } }" class="result-link">查看资产<ArrowUpRight :size="14" /></RouterLink>
                <RouterLink v-else :to="taskLink(draft)" class="result-link">继续整理<ArrowRight :size="14" /></RouterLink>
              </article>
            </div>
          </section>
        </div>

        <aside class="overview-side" aria-label="资源与城市">
          <section>
            <h2>我的资源</h2>
            <RouterLink to="/keys" class="overview-side-row"><KeyRound :size="17" /><span>有效密钥</span><b>{{ keyCount ?? (loading ? '加载中' : '未知') }}</b></RouterLink>
            <RouterLink to="/account-square/my" class="overview-side-row"><Network :size="17" /><span>共享席位</span><b>{{ seatsCount ?? (loading ? '加载中' : '未知') }}</b></RouterLink>
            <RouterLink to="/account-square" class="overview-side-row"><Search :size="17" /><span>发现共享资源</span><ArrowUpRight :size="15" /></RouterLink>
          </section>
          <section>
            <h2>零号城</h2>
            <img class="city-guide" :src="zeroCityMascots.archiveLibrarian" alt="零号城档案员" width="88" height="88" />
            <RouterLink to="/community?district=activity" class="overview-side-row"><CalendarDays :size="17" /><span>城市活动</span><ArrowUpRight :size="15" /></RouterLink>
            <RouterLink to="/community?workspace=mine" class="overview-side-row"><UserRound :size="17" /><span>我的零号城</span><ArrowUpRight :size="15" /></RouterLink>
          </section>
        </aside>
      </div>
    </main>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { RouterLink } from 'vue-router'
import { ArrowRight, ArrowUpRight, CalendarDays, FileText, KeyRound, Network, Plus, RefreshCw, Search, UserRound } from '@lucide/vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import { useAuthStore } from '@/stores/auth'
import { list as listKeys } from '@/api/keys'
import { listMySeats } from '@/features/bizdecipher/api/bizdecipher'
import { zeroCityMascots } from '@/constants/zeroCityMascots'
import { hasConfirmedDraftResult, harnessDraftStorageKey, readHarnessDrafts, type HarnessDraft } from '@/features/bizdecipher/workbench/harnessDrafts'

const auth = useAuthStore()
const drafts = ref<HarnessDraft[]>([])
const draftError = ref('')
const resourceError = ref('')
const keyError = ref('')
const errors = computed(() => [draftError.value, resourceError.value, keyError.value].filter(Boolean))
const query = ref('')
const taskFilter = ref<'all' | 'draft' | 'verified'>('all')
const visibleLimit = ref(10)
const failedImages = ref(new Set<string>())
const loading = ref(false)
const seatsCount = ref<number | null>(null)
const keyCount = ref<number | null>(null)
const results = computed(() => drafts.value.filter(hasConfirmedDraftResult))
const unfinishedCount = computed(() => drafts.value.length - results.value.length)
const filteredTasks = computed(() => drafts.value.filter(draft =>
  draft.title.toLocaleLowerCase().includes(query.value.trim().toLocaleLowerCase())
  && (taskFilter.value === 'all' || (taskFilter.value === 'verified' ? hasConfirmedDraftResult(draft) : !hasConfirmedDraftResult(draft))),
))
const visibleTasks = computed(() => filteredTasks.value.slice(0, visibleLimit.value))
const balance = computed(() => {
  const value = auth.user?.balance
  return value !== undefined && value !== null && Number.isFinite(Number(value)) ? `$${Number(value).toFixed(2)}` : '未知'
})
let epoch = 0
let controller: AbortController | null = null
function taskLink(draft: HarnessDraft) { return { path: '/operator', query: { task: draft.id } } }
function formatTime(value?: string): string {
  if (!value || !Number.isFinite(Date.parse(value))) return '未记录'
  return new Date(value).toLocaleString('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })
}
function refreshDrafts() {
  draftError.value = ''
  drafts.value = []
  if (!auth.user) return
  try { drafts.value = readHarnessDrafts(auth.user.id) }
  catch { draftError.value = '本地任务无法读取，原记录未被修改' }
}
async function refresh() {
  const current = ++epoch
  controller?.abort()
  controller = new AbortController()
  refreshDrafts()
  seatsCount.value = null
  keyCount.value = null
  resourceError.value = ''
  keyError.value = ''
  if (!auth.user) { loading.value = false; return }
  loading.value = true
  await Promise.all([
    listMySeats().then(seats => {
      if (current === epoch) seatsCount.value = seats.filter(seat => ['active', 'held'].includes(seat.status)).length
    }).catch(() => { if (current === epoch) resourceError.value = '共享资源加载失败' }),
    listKeys(1, 1, { status: 'active' }, { signal: controller.signal }).then(data => {
      if (current === epoch) keyCount.value = data.total
    }).catch(() => { if (current === epoch) keyError.value = '密钥数量加载失败' }),
  ])
  if (current === epoch) loading.value = false
}
function onStorage(event: StorageEvent) {
  if (auth.user && (event.key === null || event.key === harnessDraftStorageKey(auth.user.id))) refreshDrafts()
}
watch(() => auth.user?.id, () => {
  query.value = ''
  taskFilter.value = 'all'
  failedImages.value.clear()
  void refresh()
}, { immediate: true })
watch([query, taskFilter], () => { visibleLimit.value = 10 })
onMounted(() => { window.addEventListener('storage', onStorage) })
onBeforeUnmount(() => { ++epoch; controller?.abort(); window.removeEventListener('storage', onStorage) })
</script>

<style scoped>
.work-overview { width: min(1400px, 100%); margin: 0 auto; color: var(--bd-text-primary); font-size: 14px; }
.work-overview * { box-sizing: border-box; letter-spacing: 0; }
.overview-head, .overview-actions, .overview-section-head { display: flex; align-items: center; gap: 12px; }
.overview-head { justify-content: space-between; margin-bottom: 24px; }
.overview-head h1 { font-size: 24px; line-height: 1.4; font-weight: 600; margin: 0; }
.overview-button, .overview-icon { display: inline-flex; align-items: center; justify-content: center; gap: 8px; min-height: 36px; padding: 7px 12px; font-size: 13px; border: 1px solid var(--bd-ui-line); border-radius: 6px; background: var(--bd-surface); color: var(--bd-text-primary); text-decoration: none; cursor: pointer; }
.overview-icon { width: 36px; padding: 0; }
.overview-button.primary { background: var(--bd-accent-teal); color: var(--zc-accent-ink, #fff); border-color: transparent; }
.work-overview button:disabled { cursor: wait; opacity: .55; }
.work-overview :is(a,button,input,select):focus-visible { outline: 2px solid var(--bd-accent-teal); outline-offset: 3px; }
.overview-errors { display: flex; flex-wrap: wrap; gap: 10px; padding: 12px 0; color: var(--bd-status-danger); font-size: 12px; }
.overview-errors button { text-decoration: underline; }
.overview-summary { display: grid; grid-template-columns: repeat(4,minmax(0,1fr)); gap: 24px; padding: 20px 0; border-block: 1px solid var(--bd-ui-line); }
.overview-summary > * { display: flex; flex-direction: column; gap: 7px; color: inherit; text-decoration: none; }
.overview-summary span { font-size: 12px; color: var(--bd-text-secondary); }
.overview-summary strong { font-size: 23px; font-weight: 550; font-variant-numeric: tabular-nums; }
.overview-body { display: grid; grid-template-columns: minmax(0,1fr) 230px; gap: 32px; margin-top: 28px; }
.overview-main { min-width: 0; }
.overview-section + .overview-section { margin-top: 32px; }
.overview-section-head { margin-bottom: 16px; flex-wrap: wrap; }
.overview-section-head h2, .overview-side h2 { margin: 0; font-size: 17px; font-weight: 600; }
.overview-section-head > span { font-size: 11px; color: var(--bd-text-secondary); }
.overview-section-head > a { margin-left: auto; display: inline-flex; align-items: center; gap: 5px; color: var(--bd-accent-teal); font-size: 12px; text-decoration: none; }
.task-filters { display: flex; gap: 10px; margin-bottom: 12px; }
.overview-search { display: flex; align-items: center; gap: 8px; flex: 1; border: 1px solid var(--bd-ui-line); border-radius: 6px; padding: 0 10px; color: var(--bd-text-secondary); }
.overview-search input { height: 36px; width: 100%; min-width: 0; background: transparent; color: var(--bd-text-primary); border: 0; font-size: 13px; }
.task-filters select { background: var(--bd-surface); color: var(--bd-text-primary); border: 1px solid var(--bd-ui-line); border-radius: 6px; padding: 0 10px; font-size: 12px; }
.overview-table-wrap { overflow: auto; }
.overview-table { border-collapse: collapse; table-layout: fixed; width: 100%; min-width: 530px; font-size: 12px; }
.overview-table th { text-align: left; color: var(--bd-text-secondary); font-weight: 500; padding: 10px 8px; background: var(--bd-canvas); }
.overview-table th:first-child { width: 40%; }
.overview-table th:last-child { width: 22%; }
.overview-table td { padding: 14px 8px; border-bottom: 1px solid var(--bd-ui-line); overflow-wrap: anywhere; }
.task-name { font-size: 13px; color: var(--bd-text-primary); text-decoration: none; }
.task-name:hover { color: var(--bd-accent-teal); }
.confirmed-status { color: var(--bd-accent-teal); }
.overview-empty { display: flex; align-items: center; flex-direction: column; gap: 14px; padding: 35px 16px; color: var(--bd-text-secondary); }
.overview-empty img { object-fit: contain; }
.overview-empty h3 { margin: 0; font-size: 16px; font-weight: 500; color: var(--bd-text-primary); }
.overview-no-results { color: var(--bd-text-secondary); padding: 20px 0; font-size: 13px; }
.overview-results { display: grid; grid-template-columns: repeat(2,minmax(0,1fr)); gap: 18px; }
.overview-result { min-width: 0; padding-bottom: 14px; }
.result-art { height: 130px; display: flex; justify-content: center; align-items: center; background: var(--bd-canvas); border-radius: 6px; color: var(--bd-text-secondary); overflow: hidden; }
.result-art img { width: 100%; height: 100%; object-fit: contain; }
.overview-result h3 { font-size: 14px; font-weight: 550; margin: 10px 0 5px; overflow-wrap: anywhere; }
.overview-result h3 a { color: inherit; text-decoration: none; }
.overview-result p { font-size: 12px; color: var(--bd-text-secondary); line-height: 1.6; display: -webkit-box; -webkit-line-clamp: 2; -webkit-box-orient: vertical; overflow: hidden; overflow-wrap: anywhere; margin: 0 0 8px; }
.result-link { display: inline-flex; gap: 5px; align-items: center; color: var(--bd-accent-teal); font-size: 12px; text-decoration: none; }
.overview-side { border-left: 1px solid var(--bd-ui-line); padding-left: 24px; }
.overview-side section + section { margin-top: 30px; padding-top: 24px; border-top: 1px solid var(--bd-ui-line); }
.overview-side-row { display: flex; align-items: center; gap: 10px; color: var(--bd-text-secondary); text-decoration: none; padding: 14px 0; font-size: 12px; }
.overview-side-row span { flex: 1; }
.overview-side-row b { font-weight: 500; }
.overview-side-row:hover { color: var(--bd-accent-teal); }
.city-guide { object-fit: contain; display: block; margin: 15px auto 0; }
.load-more { margin-top: 12px; }
.spinning { animation: overview-spin 1s linear infinite; }
@keyframes overview-spin { to { transform: rotate(360deg); } }
@media (max-width: 1024px) { .overview-body { grid-template-columns: minmax(0,1fr); }.overview-side { display: none; } }
@media (max-width: 600px) { .overview-summary { grid-template-columns: repeat(2,minmax(0,1fr)); gap: 20px; }.overview-head { margin-bottom: 16px; }.overview-summary strong { font-size: 21px; }.overview-results { gap: 12px; } }
@media (prefers-reduced-motion: reduce) { .spinning { animation: none; } }
</style>
