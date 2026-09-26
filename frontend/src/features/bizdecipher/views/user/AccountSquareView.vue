<template>
  <AppLayout>
    <div class="resource-market" data-tour="shared-pool-market">
      <header class="market-heading" data-tour="shared-pool-market-header">
        <div class="as-hero-copy">
          <div class="as-hero-title-block">
            <h1 class="as-title">发现共享资源</h1>
            <p class="as-hero-subtitle">先找到合适的能力，再开始你的创作。</p>
          </div>
          <div class="as-hero-actions">
            <RouterLink to="/account-square/owner" class="as-hero-link as-supply-link" data-tour="shared-pool-owner-center">
              <Plus :size="15" />
              我要供给
            </RouterLink>
            <RouterLink to="/account-square/my" class="as-hero-link as-hero-link-soft" data-tour="shared-pool-member-center">我的资源</RouterLink>
            <RouterLink to="/account-square/owner" class="as-hero-link as-hero-link-soft">池主中心</RouterLink>
            <button type="button" class="market-help" aria-label="共享池帮助" title="共享池帮助" @click="replaySharedPoolGuide"><CircleHelp :size="18" /></button>
          </div>
        </div>
      </header>

      <main class="as-section" data-tour="shared-pool-market-browse">
        <div class="as-discovery-toolbar">
          <label class="market-search as-discovery-search">
            <Search :size="17" />
            <input v-model="keyword" class="as-input" aria-label="搜索共享资源" placeholder="搜索资源、模型或池主" @input="scheduleSearch" />
          </label>
          <div class="as-source-segments" role="group" aria-label="资源来源">
            <button
              v-for="option in sourceOptions"
              :key="option.value"
              type="button"
              :class="{ active: sourceFilter === option.value }"
              :aria-pressed="sourceFilter === option.value"
              @click="sourceFilter = option.value"
            >
              {{ option.label }}
            </button>
          </div>
          <button
            type="button"
            class="as-filter-toggle"
            :class="{ active: filtersOpen || activeFilterCount > 0 }"
            :aria-expanded="filtersOpen"
            aria-controls="shared-market-filters"
            @click="filtersOpen = !filtersOpen"
          >
            <SlidersHorizontal :size="16" />
            筛选
            <span v-if="activeFilterCount" class="as-filter-count">{{ activeFilterCount }}</span>
          </button>
        </div>

        <div v-if="filtersOpen" id="shared-market-filters" class="as-filter-row">
          <select v-model="selectedStatus" class="as-select" aria-label="筛选共享池状态" @change="loadPools">
            <option value="all">全部状态</option>
            <option value="healthy">正常</option>
            <option value="limited">受限</option>
            <option value="offline">下线</option>
          </select>
          <select v-model="selectedModel" class="as-select" aria-label="筛选模型" @change="loadPools">
            <option value="all">全部模型</option>
            <option v-for="model in modelCatalog" :key="model.model_name" :value="model.model_name">
              {{ model.display_name || model.model_name }}
            </option>
          </select>
          <select v-model="sortBy" class="as-select" aria-label="共享池排序" @change="loadPools">
            <option value="recommended">推荐</option>
            <option value="availability">可用率</option>
            <option value="latency">延迟</option>
            <option value="rate">倍率</option>
          </select>
        </div>

        <div class="as-results-meta" aria-live="polite">
          <span>已显示 {{ visibleResourceCount }} 项</span>
          <label class="as-unavailable-toggle">
            <input v-model="showUnavailable" type="checkbox" />
            显示异常资源<span v-if="hiddenResourceCount">（{{ hiddenResourceCount }} 项）</span>
          </label>
        </div>

        <div v-if="sourceFilter !== 'shared' && (officialLoading || officialLoadError || !authStore.isAuthenticated || visibleOfficialGroups.length === 0)" class="as-source-state" :class="{ 'as-source-state-error': officialLoadError }" role="status">
          <span v-if="officialLoading">正在拉取官方分组…</span>
          <span v-else-if="officialLoadError">{{ officialLoadError }}</span>
          <span v-else-if="!authStore.isAuthenticated">登录后查看当前账号可用的官方分组</span>
          <span v-else>{{ officialGroups.length ? '没有匹配的官方资源' : '当前账号暂无可用官方分组' }}</span>
          <button v-if="officialLoadError" class="as-btn as-btn-sm" type="button" @click="loadOfficialGroups">重新加载</button>
        </div>

        <div v-if="loadError && sourceFilter !== 'official'" class="as-load-error" role="alert" aria-live="assertive">
          <div>
            <b>共享池暂时无法刷新</b>
            <span>{{ loadError }}</span>
            <small v-if="pools.length">当前仍展示最近一次成功加载的数据。</small>
          </div>
          <button class="as-btn as-btn-sm" type="button" :disabled="loading" @click="loadPools">重新加载</button>
        </div>

        <div v-if="loading && pools.length === 0 && sourceFilter !== 'official'" class="as-loading">正在加载共享池…</div>

        <div v-if="showEmpty" class="as-empty">
          <img :src="mascots.emptyStateSleeper" alt="" class="as-empty-img" />
          <p>暂无符合条件的<span class="as-keep-phrase">共享资源</span></p>
        </div>

        <div v-else-if="visibleResourceCount > 0" class="as-resource-grid">
          <ResourceDiscoveryCard
            v-for="resource in officialResourceCards"
            :key="`official-${resource.id}`"
            :to="resource.to"
            :title="resource.title"
            :image-url="resource.imageUrl"
            :fallback-image-url="mascots.gatewayOperator"
            :image-alt="`${resource.title}官方角色图`"
            image-fit="contain"
            :quote="resource.quote"
            :title-note="resource.titleNote"
            :provider="resource.provider"
            source-label="官方"
            source-tone="official"
            :status="resource.status"
            :status-tone="resource.status === '检测模型可用' ? 'good' : 'muted'"
            :cover-note="resource.verificationLabel"
            :tags="resource.tags"
            :metrics="resource.metrics"
            :verification-label="resource.verificationLabel"
            :link-label="`在工作台使用官方分组 ${resource.title}`"
          >
            <template #evidence>
              <OfficialObservationStrip :monitor="resource.observation" :loading="observationLoading" :error="observationError" @retry="loadObservations" />
            </template>
            <template #footer>
              <RouterLink to="/monitor" class="resource-card-footnote" title="查看完整模型检测历史">检测详情</RouterLink>
              <span class="resource-card-spacer" />
              <RouterLink v-if="!resource.unavailable" :to="{ path: '/keys', query: { source: 'official', group: resource.id } }" class="as-card-action">
                API 接入 <ArrowUpRight :size="14" />
              </RouterLink>
              <RouterLink v-if="!resource.unavailable" :to="resource.to" class="as-card-action">
                工作台 <ArrowUpRight :size="14" />
              </RouterLink>
            </template>
          </ResourceDiscoveryCard>

          <PoolMarketCard
            v-for="pool in visiblePools"
            :key="pool.id"
            :pool="pool"
            :joined="joinedPoolIds.has(pool.id)"
            :acting="actingPoolId === pool.id"
            :summary="poolCommunitySummaries[pool.id] || emptyCommunitySummary(pool.id)"
            :like-acting="actingPoolId === pool.id"
            @join="joinPool"
            @leave="leavePool"
            @report="reportPool"
            @like="likePool"
          />
        </div>

        <div v-if="hasMoreVisiblePools" class="as-load-more">
          <button class="as-btn" type="button" data-testid="shared-market-load-more" @click="showMorePools">
            再显示 {{ Math.min(MARKET_BATCH_SIZE, sourceFilteredPools.length - visiblePools.length) }} 项
          </button>
        </div>
      </main>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { ArrowUpRight, CircleHelp, Plus, Search, SlidersHorizontal } from '@lucide/vue'
import { RouterLink } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import { useSharedPool } from '@/composables/useSharedPool'
import { useSharedPoolOnboarding } from '@/composables/useSharedPoolOnboarding'
import type { SharedPoolCommunitySummary } from '@/features/bizdecipher/api/community'
import PoolMarketCard from '@/features/bizdecipher/components/shared-pool/PoolMarketCard.vue'
import ResourceDiscoveryCard from '@/features/bizdecipher/components/shared-pool/ResourceDiscoveryCard.vue'
import type { ResourceMetric } from '@/features/bizdecipher/components/shared-pool/resourceDiscovery'
import { zeroCityMascots } from '@/features/bizdecipher/constants/zeroCityMascots'
import { useAuthStore } from '@/stores/auth'
import { getAvailable } from '@/api/groups'
import type { Group } from '@/types'
import { list as listObservations, type UserMonitorView } from '@/api/channelMonitor'
import OfficialObservationStrip from '@/features/bizdecipher/components/shared-pool/OfficialObservationStrip.vue'
import { officialDiscoveryState, observedModelNames, resolveOfficialObservation } from '@/features/bizdecipher/components/shared-pool/officialObservation'

const MARKET_BATCH_SIZE = 12
type PoolSourceFilter = 'all' | 'official' | 'shared'
const sourceOptions: Array<{ value: PoolSourceFilter; label: string }> = [
  { value: 'all', label: '全部' },
  { value: 'official', label: '官方' },
  { value: 'shared', label: '共享池' },
]
const mascots = zeroCityMascots
const authStore = useAuthStore()
const { replayGuide: replaySharedPoolGuide } = useSharedPoolOnboarding({ scope: 'market' })

const {
  keyword,
  selectedStatus,
  selectedModel,
  sortBy,
  loading,
  loadError,
  pools,
  filteredPools,
  modelCatalog,
  joinedPoolIds,
  actingPoolId,
  poolCommunitySummaries,
  loadPools,
  loadModelCatalog,
  loadMySeats,
  joinPool,
  leavePool,
  reportPool,
  likePool,
} = useSharedPool()

const visibleLimit = ref(MARKET_BATCH_SIZE)
const sourceFilter = ref<PoolSourceFilter>('all')
const filtersOpen = ref(false)
const showUnavailable = ref(false)
const officialGroups = ref<Group[]>([])
const officialLoading = ref(false)
const officialLoadError = ref('')
const observations = ref<UserMonitorView[]>([])
const observationLoading = ref(false)
const observationError = ref(false)
let observationRequest = 0
let observationController: AbortController | undefined

interface OfficialResourceCard {
  id: number
  title: string
  to: string
  imageUrl: string
  quote: string
  titleNote: string
  provider: string
  status: string
  unavailable: boolean
  tags: string[]
  metrics: ResourceMetric[]
  verificationLabel: string
  observation?: UserMonitorView
}

const matchingPools = computed(() => (
  sourceFilter.value === 'official' ? [] : filteredPools.value
))
const sourceFilteredPools = computed(() => matchingPools.value.filter(pool => (
  showUnavailable.value || selectedStatus.value === 'offline'
  || !['offline', 'maintenance'].includes(pool.status)
)))

const matchingOfficialGroups = computed(() => {
  if (sourceFilter.value === 'shared') return []
  const kw = keyword.value.trim().toLocaleLowerCase()
  if (!kw) return officialGroups.value
  return officialGroups.value.filter(group => (
    group.name.toLocaleLowerCase().includes(kw) ||
    String(group.description || '').toLocaleLowerCase().includes(kw) ||
    group.platform.toLocaleLowerCase().includes(kw)
  ))
})
function discoveryState(group: Group) {
  return officialDiscoveryState(group.status, resolveOfficialObservation(group, officialGroups.value, observations.value))
}
const visibleOfficialGroups = computed(() => matchingOfficialGroups.value.filter(group => showUnavailable.value || !discoveryState(group).hidden))
const hiddenResourceCount = computed(() => matchingOfficialGroups.value.filter(group => discoveryState(group).hidden).length
  + matchingPools.value.filter(pool => ['offline', 'maintenance'].includes(pool.status)).length)

const visiblePools = computed(() => sourceFilteredPools.value.slice(0, visibleLimit.value))
const visibleResourceCount = computed(() => visibleOfficialGroups.value.length + visiblePools.value.length)
const hasMoreVisiblePools = computed(() => visiblePools.value.length < sourceFilteredPools.value.length)
const activeFilterCount = computed(() => [
  selectedStatus.value !== 'all',
  selectedModel.value !== 'all',
  sortBy.value !== 'recommended',
].filter(Boolean).length)
const showEmpty = computed(() => (
  (sourceFilter.value === 'official' || (!loading.value && !loadError.value))
  && (sourceFilter.value === 'shared' || (authStore.isAuthenticated && !officialLoading.value && !officialLoadError.value))
  && visibleResourceCount.value === 0
))

const officialMascots = [
  mascots.gatewayOperator,
  mascots.keyKeeper,
  mascots.statusInspector,
  mascots.usageMeterReader,
  mascots.archiveLibrarian,
]

function billingLabel(group: Group): string {
  return group.billing_asset_type === 'credits' ? '积分计费' : '余额计费'
}

function capabilityTags(group: Group): string[] {
  const names = observedModelNames(resolveOfficialObservation(group, officialGroups.value, observations.value))
  if (!names.length) return group.default_mapped_model ? [group.default_mapped_model] : ['模型待确认']
  return [...names.slice(0, 2), ...(names.length > 2 ? [`+${names.length - 2} 个模型`] : [])]
}

function officialLimitLabel(group: Group): string {
  if (Number(group.daily_limit_usd) > 0) return `每日额度 $${group.daily_limit_usd}`
  if (Number(group.weekly_limit_usd) > 0) return `每周额度 $${group.weekly_limit_usd}`
  if (Number(group.monthly_limit_usd) > 0) return `每月额度 $${group.monthly_limit_usd}`
  return group.is_exclusive ? '账号专属分组' : ''
}

const officialResourceCards = computed<OfficialResourceCard[]>(() => visibleOfficialGroups.value.map((group, index) => ({
  id: group.id,
  title: group.name,
  to: discoveryState(group).hidden ? '/monitor' : `/operator?resourceSource=official&resourceId=${group.id}`,
  imageUrl: officialMascots[Math.abs(group.id || index) % officialMascots.length],
  quote: group.description || '',
  titleNote: group.default_mapped_model ? `默认模型 ${group.default_mapped_model}` : officialLimitLabel(group),
  provider: `官方分组 · ${group.platform}`,
  status: discoveryState(group).label,
  unavailable: discoveryState(group).hidden,
  tags: capabilityTags(group),
  metrics: [
    { label: '计费资产', value: billingLabel(group) },
    { label: '分组基础倍率', value: group.rate_multiplier != null && Number.isFinite(Number(group.rate_multiplier)) ? `${Number(group.rate_multiplier).toFixed(2)}×` : '待确认', note: '以实际计价为准' },
    { label: '接入方式', value: 'API / 工作台' },
  ],
  verificationLabel: group.is_exclusive ? '专属供给' : '平台供给',
  observation: resolveOfficialObservation(group, officialGroups.value, observations.value),
})))

watch(
  [keyword, selectedStatus, selectedModel, sortBy, sourceFilter, showUnavailable],
  () => { visibleLimit.value = MARKET_BATCH_SIZE },
)

function showMorePools(): void {
  visibleLimit.value += MARKET_BATCH_SIZE
}

let searchTimer: ReturnType<typeof setTimeout> | undefined
function scheduleSearch(): void {
  clearTimeout(searchTimer)
  searchTimer = setTimeout(() => { void loadPools() }, 250)
}

function emptyCommunitySummary(poolId: number): SharedPoolCommunitySummary {
  return {
    pool_id: poolId,
    total_posts: 0,
    discussion_posts: 0,
    feedback_posts: 0,
    incident_posts: 0,
    risk_signals: 0,
    last_post_title: '',
  }
}

async function loadOfficialGroups(): Promise<void> {
  if (!authStore.isAuthenticated) return
  officialLoading.value = true
  officialLoadError.value = ''
  try {
    officialGroups.value = await getAvailable()
  } catch {
    officialGroups.value = []
    officialLoadError.value = '官方分组暂时无法刷新'
  } finally {
    officialLoading.value = false
  }
}

async function loadObservations(): Promise<void> {
  if (!authStore.isAuthenticated) return
  const request = ++observationRequest
  observationController?.abort()
  observationController = new AbortController()
  observationLoading.value = true
  observationError.value = false
  try {
    const result = await listObservations({ signal: observationController.signal })
    if (request === observationRequest) observations.value = result.items || []
  } catch {
    if (request === observationRequest) {
      observations.value = []
      observationError.value = true
    }
  } finally {
    if (request === observationRequest) observationLoading.value = false
  }
}

let observationTimer: ReturnType<typeof setInterval> | undefined
onBeforeUnmount(() => {
  clearInterval(observationTimer)
  clearTimeout(searchTimer)
  observationRequest += 1
  observationController?.abort()
})

onMounted(async () => {
  observationTimer = setInterval(() => {
    if (document.visibilityState === 'visible' && !observationLoading.value) void loadObservations()
  }, 60000)
  const requests = [loadPools(), loadModelCatalog(), loadOfficialGroups(), loadObservations()]
  if (authStore.isAuthenticated) requests.push(loadMySeats())
  await Promise.all(requests)
})
</script>

<style scoped>
.as-unavailable-toggle {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  margin-left: auto;
  cursor: pointer;
  font-size: 12px;
}
.resource-market {
  --bg: var(--module-panel, var(--zc-surface));
  --text: var(--module-ink-strong, var(--zc-text-strong));
  --muted: var(--module-muted, var(--zc-muted));
  --teal: var(--bd-accent-teal);
  --nd: color-mix(in srgb, var(--zc-shadow-dark) 26%, transparent);
  --nl: color-mix(in srgb, var(--zc-shadow-light) 60%, transparent);
  --raise-sm: 0 10px 28px var(--nd);
  --inset: inset 0 1px 0 var(--nl), 0 0 0 1px var(--module-line, var(--zc-line));
  --inset-sm: inset 0 1px 0 var(--nl), 0 0 0 1px var(--module-line, var(--zc-line));
  max-width: 1600px;
  margin: 0 auto;
  width: 100%;
  padding: 0 0 40px;
  display: flex;
  flex-direction: column;
  gap: 20px;
  color: var(--text);
}

.market-heading { display: flex; align-items: flex-start; justify-content: space-between; flex-wrap: wrap; gap: 20px; }
.as-hero-copy { display: flex; justify-content: space-between; align-items: center; gap: 16px; flex: 1; flex-wrap: wrap; }
.as-hero-title-block { display: grid; gap: 6px; min-width: 0; }
.as-kicker { margin: 0; color: var(--teal); font-size: 11px; font-weight: 600; letter-spacing: 0; }
.as-title { margin: 0; font-size: 24px; font-weight: 700; color: var(--text); }
.as-hero-subtitle { margin: 0; color: var(--muted); font-size: 14px; line-height: 1.6; }
.resource-market .as-hero-actions { display: flex; gap: 8px; flex-wrap: wrap; margin: 0; }
.resource-market .as-hero-link { display: inline-flex; align-items: center; justify-content: center; min-height: 36px; padding: 0 12px; font-size: 13px; color: var(--text); text-decoration: none; }
.resource-market .as-supply-link { background: var(--teal); color: var(--bd-on-action, #fff); }
.market-help { width: 36px; height: 36px; display: grid; place-items: center; color: var(--muted); border-radius: 6px; }
.as-supply-link { gap: 7px; border-color: transparent; background: var(--teal); color: #fff; }
.as-section { display: flex; flex-direction: column; gap: 18px; }
.as-source-state { display: flex; align-items: center; justify-content: space-between; gap: 12px; min-height: 56px; padding: 12px 14px; border: 1px dashed var(--module-line, var(--zc-line)); border-radius: 6px; color: var(--muted); font-size: 13px; line-height: 1.6; }
.as-source-state-error { border-style: solid; color: var(--zc-danger, #b63346); background: color-mix(in srgb, var(--zc-danger, #b63346) 6%, var(--bg)); }
.as-discovery-toolbar { display: flex; align-items: center; gap: 12px; flex-wrap: wrap; }
.resource-market .market-search { display: flex; align-items: center; gap: 8px; flex: 1 1 260px; max-width: 410px; min-height: 40px; padding: 0 12px; border: 1px solid var(--bd-ui-line); border-radius: 6px; background: var(--bg); color: var(--muted); }
.resource-market .market-search .as-input { flex: 1; width: 100%; min-width: 0; border: 0; padding: 0; background: transparent; box-shadow: none; font-size: 14px; }
.as-source-segments { display: flex; gap: 4px; padding: 4px; border: 1px solid var(--module-line, var(--zc-line)); border-radius: 6px; background: var(--bg); }
.as-source-segments button { min-height: 32px; padding: 0 14px; border-radius: 4px; color: var(--muted); font-size: 13px; }
.as-source-segments button.active { background: var(--bd-canvas); color: var(--text); font-weight: 600; }
.as-filter-toggle { display: inline-flex; align-items: center; gap: 6px; min-height: 38px; padding: 0 12px; border: 1px solid var(--module-line, var(--zc-line)); border-radius: 6px; background: var(--bg); color: var(--muted); font-size: 13px; }
.as-filter-toggle:hover, .as-filter-toggle.active { color: var(--text); border-color: color-mix(in srgb, var(--teal) 45%, var(--module-line, var(--zc-line))); }
.as-filter-count { display: inline-grid; min-width: 18px; height: 18px; place-items: center; border-radius: 999px; background: var(--teal); color: #fff; font-size: 10px; }
.as-filter-row { display: flex; gap: 10px; flex-wrap: wrap; align-items: center; padding: 12px; border: 1px solid var(--module-line, var(--zc-line)); border-radius: 8px; background: var(--bg); }
.as-input, .as-select { min-height: 40px; border: 1px solid var(--module-line, var(--zc-line)); border-radius: 6px; background: var(--bg); color: var(--text); }
.as-input { min-width: 0; padding: 0 10px; }
.as-select { padding: 0 12px; cursor: pointer; }
.as-results-meta { display: flex; align-items: center; justify-content: space-between; gap: 12px; color: var(--muted); font-size: 12px; }
.as-load-error { display: flex; align-items: flex-start; justify-content: space-between; gap: 12px; padding: 14px 16px; border-radius: 8px; background: color-mix(in srgb, var(--zc-danger) 8%, var(--bg)); color: var(--text); }
.as-load-error > div { display: grid; gap: 4px; min-width: 0; }
.as-load-error b { color: var(--zc-danger); font-size: 13px; }
.as-load-error span, .as-load-error small { color: var(--muted); font-size: 12px; line-height: 1.55; }
.as-resource-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(260px, 1fr)); gap: 14px; align-items: stretch; }
.official-evidence { min-height: 46px; display: flex; align-items: center; justify-content: space-between; gap: 12px; color: var(--muted); font-size: 12px; }
.official-evidence span, .official-evidence a { display: inline-flex; align-items: center; gap: 6px; }
.official-evidence a { color: var(--teal); }
.as-card-action { display: inline-flex; align-items: center; gap: 5px; color: var(--teal); font-size: 12px; font-weight: 600; text-decoration: none; }
.resource-card-footnote { color: var(--muted); font-size: 11px; }
.resource-card-spacer { flex: 1; }
.as-loading { padding: 40px; text-align: center; color: var(--muted); }
.as-empty { padding: 40px 16px; text-align: center; color: var(--muted); }
.as-empty-img { width: 120px; height: auto; }
.as-keep-phrase { white-space: nowrap; }
.as-load-more { display: flex; justify-content: center; padding-top: 18px; }
.as-btn { display: inline-flex; align-items: center; justify-content: center; min-height: 36px; padding: 0 14px; border: 1px solid var(--module-line, var(--zc-line)); border-radius: 6px; background: var(--bg); color: var(--text); cursor: pointer; }
.as-btn:disabled { opacity: .5; cursor: not-allowed; }
.as-btn-sm { min-height: 32px; padding: 0 12px; font-size: 12px; }

:is(.as-filter-toggle, .as-btn, .as-hero-link, .as-input, .as-select):focus-visible {
  outline: 2px solid var(--teal);
  outline-offset: 2px;
}

@media (min-width: 1700px) { .as-resource-grid { grid-template-columns: repeat(5, minmax(0, 1fr)); } }
@media (min-width: 768px) and (max-width: 1199px) { .as-resource-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
@media (max-width: 767px) {
  .market-heading { display: block; }
  .as-discovery-toolbar { align-items: stretch; flex-direction: column; }
  .resource-market .market-search { flex: 0 0 auto; width: 100%; max-width: none; }
  .as-filter-toggle { width: 100%; justify-content: center; }
  .as-source-segments { width: 100%; }
  .as-source-segments button { flex: 1; }
  .as-load-error { flex-direction: column; }
  .as-select { min-width: 0; flex: 1; }
  .as-results-meta { align-items: flex-start; flex-direction: column; gap: 4px; }
  .as-resource-grid { grid-template-columns: 1fr; }
}
</style>
