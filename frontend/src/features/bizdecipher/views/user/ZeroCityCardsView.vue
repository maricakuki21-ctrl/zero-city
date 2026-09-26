<template>
  <AppLayout>
    <div class="card-album-page">
      <section class="album-hero">
        <div class="album-hero-copy">
          <p class="album-kicker">Zero City Collection</p>
          <h1>我的收藏卡册</h1>
          <p>按卡面归档重复收藏，翻到背面查看签语与背景故事。</p>
          <div class="album-hero-actions">
            <button class="album-btn album-btn-primary" type="button" @click="router.push('/community')">返回零号城</button>
            <button class="album-btn" type="button" @click="router.push('/zero-city/chronicle')">城市编年史</button>
            <button class="album-btn" type="button" :disabled="isLoading" @click="loadAlbum">
              {{ isLoading ? '刷新中...' : '刷新卡册' }}
            </button>
          </div>
        </div>
        <div class="album-summary" aria-label="收藏卡统计">
          <article>
            <strong>{{ assetTotal }}</strong>
            <span>资产实例总数</span>
          </article>
          <article>
            <strong>{{ unlockedCardCount }} / {{ catalogTotal }}</strong>
            <span>已解锁卡面 / 图鉴</span>
          </article>
          <article>
            <strong>{{ filteredCatalog.length }}</strong>
            <span>当前筛选结果</span>
          </article>
        </div>
      </section>

      <section class="album-toolbar">
        <div class="album-filter-groups">
          <div class="ownership-filters" role="tablist" aria-label="按拥有状态筛选">
            <button v-for="filter in ownershipFilters" :key="filter.key" class="filter-chip" :class="{ 'is-active': activeOwnership === filter.key }" type="button" @click="activeOwnership = filter.key">
              {{ filter.label }}<span>{{ filter.count }}</span>
            </button>
          </div>
          <div class="rarity-filters" role="tablist" aria-label="按稀有度筛选">
          <button
            v-for="filter in rarityFilters"
            :key="filter.key"
            class="filter-chip"
            :class="{ 'is-active': activeRarity === filter.key }"
            type="button"
            @click="activeRarity = filter.key"
          >
            {{ filter.label }}
            <span>{{ filter.count }}</span>
          </button>
          </div>
        </div>
        <div class="album-toolbar-actions">
          <button v-if="activeBackgroundItem" class="album-btn album-btn-danger" type="button" :disabled="isSavingBackground" @click="clearBackground">
            清空主页背景
          </button>
        </div>
      </section>

      <section v-if="activeBackgroundItem" class="active-background-card">
        <img
          v-if="activeBackgroundItem.representative"
          :src="cardImage(activeBackgroundItem.representative)"
          :srcset="cardImageSrcSet(activeBackgroundItem.representative)"
          :alt="activeBackgroundItem.definition.name"
          loading="lazy"
          decoding="async"
          sizes="64px"
          @error="handleImageFallback"
        />
        <div>
          <span>当前主页展示卡</span>
          <h2>{{ activeBackgroundItem.definition.name }}</h2>
          <p>{{ rarityLabel(activeBackgroundItem.rarity) }} · {{ groupEditionLabel(activeBackgroundItem) }}</p>
        </div>
        <button class="album-btn" type="button" :disabled="!activeBackgroundItem.representative" @click="openCard(activeBackgroundItem)">
          {{ activeBackgroundItem.representative ? '查看详情' : '详情未载入' }}
        </button>
      </section>

      <section class="album-content">
        <div v-if="isLoading" class="album-state">
          <strong>正在读取收藏卡册</strong>
          <p>正在核对本次收藏记录。</p>
        </div>
        <div v-else-if="loadError" class="album-state">
          <strong>卡册暂时读取失败</strong>
          <p>请检查登录状态，或稍后重新读取。</p>
          <button class="album-btn album-btn-primary" type="button" @click="loadAlbum">重新读取</button>
        </div>
        <template v-else>
          <p class="album-scope-note">
            已完整读取 {{ loadedCollectionCount }} 张收藏资产；图鉴按卡面去重，重复版本仍保留在详情中。
          </p>
          <div class="album-grid">
            <button
              v-for="item in paginatedCatalog"
              :key="item.key"
              class="album-card"
              :class="[`rarity-${item.rarity}`, { 'is-background': isBackgroundItem(item), 'is-locked': !item.owned }]"
              type="button"
              :aria-label="item.owned ? `查看${item.definition.name}` : `预览未拥有的${item.definition.name}`"
              @click="openCard(item)"
            >
              <div class="album-card-image">
                <img
                  :src="catalogImage(item)"
                  :srcset="catalogImageSrcSet(item)"
                  :alt="item.definition.name"
                  loading="lazy"
                  decoding="async"
                  fetchpriority="low"
                  sizes="(max-width: 560px) 44vw, (max-width: 960px) 28vw, 190px"
                  @error="handleImageFallback"
                />
                <span class="rarity-badge">{{ rarityLabel(item.rarity) }}</span>
                <span v-if="isBackgroundItem(item)" class="background-badge">主页展示</span>
                <span v-else-if="!item.owned" class="background-badge">未拥有</span>
              </div>
              <div class="album-card-body">
                <strong>{{ item.definition.name }}</strong>
                <span>{{ item.owned ? ownedSummary(item) : '尚未获得' }}</span>
              </div>
            </button>
          </div>
          <nav v-if="pageCount > 1" class="album-pagination" aria-label="卡册分页">
            <button class="album-btn" type="button" :disabled="currentPage <= 1" @click="currentPage -= 1">上一页</button>
            <span>第 {{ currentPage }} / {{ pageCount }} 页</span>
            <button class="album-btn" type="button" :disabled="currentPage >= pageCount" @click="currentPage += 1">下一页</button>
          </nav>
        </template>
      </section>

      <transition name="card-detail-fade">
        <div v-if="selectedItem" class="card-detail-backdrop" role="presentation" @click.self="closeCard" @keydown.esc="closeCard">
          <article ref="detailDialog" class="card-detail-dialog" role="dialog" aria-modal="true" :aria-label="`${selectedItem.definition.name} 详情`" tabindex="-1">
            <button ref="detailCloseButton" class="card-detail-close" type="button" aria-label="关闭" @click="closeCard">×</button>
            <button
              class="detail-card-shell"
              :class="[`rarity-${selectedItem.rarity}`, { 'is-flipped': detailFlipped }]"
              type="button"
              :aria-label="detailFlipped ? '返回卡牌正面' : '翻到卡牌背面'"
              @click="toggleDetailFace"
            >
              <span class="detail-card-inner">
                <span class="detail-card-face detail-card-front">
                  <img
                    :src="catalogImage(selectedItem)"
                    :srcset="catalogImageSrcSet(selectedItem)"
                    :alt="selectedItem.definition.name"
                    decoding="async"
                    sizes="(max-width: 900px) 72vw, 360px"
                    @error="handleImageFallback"
                  />
                  <span class="detail-face-caption">
                    <b>{{ selectedItem.definition.name }}</b>
                    <small>{{ rarityLabel(selectedItem.rarity) }} · {{ groupEditionLabel(selectedItem) }}</small>
                  </span>
                </span>
                <span class="detail-card-face detail-card-back">
                  <span class="detail-back-code">{{ selectedItem.definition.code }}</span>
                  <strong>{{ selectedItem.definition.name }}</strong>
                  <em>{{ selectedItem.definition.line }}</em>
                  <span>{{ selectedItem.definition.faction }} · {{ selectedItem.definition.role }}</span>
                  <p>{{ selectedItem.definition.lore }}</p>
                </span>
              </span>
            </button>
            <div class="card-detail-copy">
              <p class="album-kicker">{{ rarityLabel(selectedItem.rarity) }}</p>
              <h2>{{ selectedItem.definition.name }}</h2>
              <div class="detail-facts">
                <article>
                  <span>拥有数量</span>
                  <strong>{{ selectedItem.count }}</strong>
                </article>
                <article>
                  <span>收藏序列</span>
                  <strong>{{ selectedItem.representative ? `#${selectedItem.representative.serial_no}` : '尚未拥有' }}</strong>
                </article>
                <article>
                  <span>获得方式</span>
                  <strong>{{ selectedItem.representative ? sourceLabel(selectedItem.representative) : '获得后记录' }}</strong>
                </article>
                <article>
                  <span>获得时间</span>
                  <strong>{{ selectedItem.representative ? acquiredAtLabel(selectedItem.representative.created_at) : '尚未拥有' }}</strong>
                </article>
              </div>
              <div class="card-detail-actions">
                <button class="album-btn" type="button" @click="toggleDetailFace">
                  {{ detailFlipped ? '返回正面' : '查看背面故事' }}
                </button>
                <button
                  class="album-btn album-btn-primary"
                  type="button"
                  :disabled="isSavingBackground || isBackgroundItem(selectedItem) || !selectedItem.representative"
                  @click="selectedItem.representative && saveBackground(selectedItem.representative)"
                >
                  {{ !selectedItem.representative ? '拥有后可展示' : isBackgroundItem(selectedItem) ? '当前主页展示卡' : isSavingBackground ? '保存中...' : '设为主页背景' }}
                </button>
                <button class="album-btn" type="button" @click="closeCard">继续浏览</button>
              </div>
            </div>
          </article>
        </div>
      </transition>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import { getAllCheckinCards } from '@/api/checkinCards'
import { getZeroCityPublicProfile, updateZeroCityProfileBackground, type BizProfile } from '@/features/bizdecipher/api/bizdecipher'
import {
  zeroCityCardDisplayName,
  zeroCityCardImagePath,
  zeroCityCardImageSrcSet,
  zeroCityCardManifestEntries,
  type ZeroCityCardDefinition,
  type ZeroCityCardRarity
} from '@/constants/zeroCityCardManifest'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import type { CheckinCollectibleCard } from '@/types'

const PAGE_SIZE = 12
const RARITY_ORDER: ZeroCityCardRarity[] = ['mythic', 'legendary', 'epic', 'rare', 'good', 'common', 'legacy']
const manifestEntries = zeroCityCardManifestEntries()
const manifestKeys = new Set(manifestEntries.map(entry => entry.key))
const catalogTotal = manifestEntries.length

type RarityFilter = 'all' | ZeroCityCardRarity
type OwnershipFilter = 'all' | 'owned' | 'missing'
type CatalogItem = {
  key: string
  definition: ZeroCityCardDefinition
  rarity: ZeroCityCardRarity
  legacy: boolean
  owned: boolean
  count: number
  cards: CheckinCollectibleCard[]
  representative: CheckinCollectibleCard | null
}

const route = useRoute()
const router = useRouter()
const appStore = useAppStore()
const authStore = useAuthStore()

const cards = ref<CheckinCollectibleCard[]>([])
const assetTotal = ref(0)
const profile = ref<BizProfile | null>(null)
const selectedItem = ref<CatalogItem | null>(null)
const detailFlipped = ref(false)
const detailDialog = ref<HTMLElement | null>(null)
const detailCloseButton = ref<HTMLButtonElement | null>(null)
let detailReturnFocus: HTMLElement | null = null
const activeRarity = ref<RarityFilter>('all')
const activeOwnership = ref<OwnershipFilter>('all')
const currentPage = ref(1)
const isLoading = ref(false)
const isSavingBackground = ref(false)
const loadError = ref(false)

const ownedGroups = computed(() => {
  const groups = new Map<string, CheckinCollectibleCard[]>()
  for (const card of cards.value) {
    const key = String(card.card_key || '').trim()
    if (!key) continue
    const group = groups.get(key)
    if (group) group.push(card)
    else groups.set(key, [card])
  }
  for (const group of groups.values()) {
    group.sort((left, right) => Number(right.serial_no || 0) - Number(left.serial_no || 0))
  }
  return groups
})

const catalogItems = computed<CatalogItem[]>(() => {
  const known = manifestEntries.map((entry) => {
    const ownedCards = ownedGroups.value.get(entry.key) || []
    return {
      key: entry.key,
      definition: entry.definition,
      rarity: entry.rarity,
      legacy: entry.legacy,
      owned: ownedCards.length > 0,
      count: ownedCards.length,
      cards: ownedCards,
      representative: ownedCards[0] || null
    }
  })
  const unknown = [...ownedGroups.value.entries()]
    .filter(([key]) => !manifestKeys.has(key))
    .map(([key, ownedCards]): CatalogItem => ({
      key,
      definition: {
        code: 'ZC-UNKNOWN',
        name: zeroCityCardDisplayName(key),
        faction: '未收录档案',
        role: '待核对卡面',
        line: '这张收藏卡尚未进入当前图鉴版本。',
        lore: '卡片资产仍然保留；待图鉴定义补齐后会显示正式故事。',
        visual: '未收录卡面'
      },
      rarity: normalizeRarity(ownedCards[0]?.rarity),
      legacy: false,
      owned: true,
      count: ownedCards.length,
      cards: ownedCards,
      representative: ownedCards[0] || null
    }))
  return [...known, ...unknown]
})

const loadedCollectionCount = computed(() => cards.value.length)
const unlockedCardCount = computed(() => catalogItems.value.filter(item => item.owned && manifestKeys.has(item.key)).length)
const ownershipFilteredCatalog = computed(() => {
  if (activeOwnership.value === 'owned') return catalogItems.value.filter(item => item.owned)
  if (activeOwnership.value === 'missing') return catalogItems.value.filter(item => !item.owned)
  return catalogItems.value
})
const filteredCatalog = computed(() => activeRarity.value === 'all'
  ? ownershipFilteredCatalog.value
  : ownershipFilteredCatalog.value.filter(item => item.rarity === activeRarity.value))
const pageCount = computed(() => Math.max(1, Math.ceil(filteredCatalog.value.length / PAGE_SIZE)))
const paginatedCatalog = computed(() => {
  const start = (currentPage.value - 1) * PAGE_SIZE
  return filteredCatalog.value.slice(start, start + PAGE_SIZE)
})
const activeBackgroundItem = computed(() => catalogItems.value.find(isBackgroundItem) || null)
const rarityFilters = computed(() => [
  { key: 'all' as const, label: '全部', count: catalogItems.value.length },
  ...RARITY_ORDER.map(rarity => ({
    key: rarity,
    label: rarityLabel(rarity),
    count: catalogItems.value.filter(item => item.rarity === rarity).length
  }))
])
const ownershipFilters = computed(() => [
  { key: 'all' as const, label: '全部图鉴', count: catalogItems.value.length },
  { key: 'owned' as const, label: '已拥有', count: catalogItems.value.filter(item => item.owned).length },
  { key: 'missing' as const, label: '未拥有', count: catalogItems.value.filter(item => !item.owned).length },
])

async function loadAlbum() {
  isLoading.value = true
  loadError.value = false
  try {
    const result = await getAllCheckinCards()
    cards.value = Array.isArray(result.items) ? result.items : []
    assetTotal.value = Number(result.total || cards.value.length)
    const userID = Number(authStore.user?.id || 0)
    if (userID > 0) {
      try {
        profile.value = (await getZeroCityPublicProfile(userID)).profile
      } catch {
        profile.value = null
      }
    }
    openCardFromQuery()
  } catch {
    cards.value = []
    assetTotal.value = 0
    loadError.value = true
  } finally {
    isLoading.value = false
  }
}

function normalizeRarity(rarity?: string | null): ZeroCityCardRarity {
  const normalized = String(rarity || 'common').toLowerCase() as ZeroCityCardRarity
  return RARITY_ORDER.includes(normalized) ? normalized : 'common'
}

function rarityLabel(rarity?: string | null) {
  const labels: Record<ZeroCityCardRarity, string> = {
    mythic: '神话',
    legendary: '传说',
    epic: '史诗',
    rare: '稀有',
    good: '进阶',
    common: '普通',
    legacy: '旧版'
  }
  return labels[normalizeRarity(rarity)]
}

function cardImage(card: Pick<CheckinCollectibleCard, 'card_key' | 'rarity'>) {
  return zeroCityCardImagePath(card)
}

function cardImageSrcSet(card: Pick<CheckinCollectibleCard, 'card_key' | 'rarity'>) {
  return zeroCityCardImageSrcSet(card)
}

function catalogImage(item: CatalogItem): string {
  return zeroCityCardImagePath({ card_key: item.key, rarity: item.rarity })
}

function catalogImageSrcSet(item: CatalogItem): string {
  return zeroCityCardImageSrcSet({ card_key: item.key, rarity: item.rarity })
}

function editionLabel(card: CheckinCollectibleCard) {
  if (card.edition_no && card.edition_supply) return `#${card.edition_no} / ${card.edition_supply}`
  if (card.edition_no) return `#${card.edition_no}`
  if (card.serial_no) return `序列 #${card.serial_no}`
  return '暂未记录'
}

function groupEditionLabel(item: CatalogItem) {
  if (item.count > 1) return `×${item.count}`
  return item.representative ? editionLabel(item.representative) : '尚未获得'
}

function ownedSummary(item: CatalogItem) {
  return item.count > 1 ? `×${item.count}` : groupEditionLabel(item)
}

function sourceLabel(card: CheckinCollectibleCard) {
  const source = String(card.source_label || card.source_type || '').trim()
  const labels: Record<string, string> = {
    free: '每日免费礼盒',
    balance: '余额礼盒',
    credit: '积分礼盒',
    paid: '积分礼盒',
    credit_shop: '积分买卡'
  }
  return labels[source] || source || '暂未记录'
}

function acquiredAtLabel(value?: string) {
  if (!value) return '暂未记录'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString('zh-CN', { year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })
}

function isBackgroundItem(item: CatalogItem) {
  const serial = Number(profile.value?.background_card_serial_no || 0)
  if (serial > 0) return item.cards.some(card => Number(card.serial_no || 0) === serial)
  return Boolean(profile.value?.background_card_key) && profile.value?.background_card_key === item.key
}

async function saveBackground(card: CheckinCollectibleCard) {
  if (isSavingBackground.value) return
  isSavingBackground.value = true
  try {
    profile.value = await updateZeroCityProfileBackground({ card_key: card.card_key, serial_no: card.serial_no })
    appStore.showSuccess('已设置为零号城主页背景。')
  } catch {
    appStore.showError('主页背景保存失败，请稍后再试。')
  } finally {
    isSavingBackground.value = false
  }
}

async function clearBackground() {
  if (isSavingBackground.value) return
  isSavingBackground.value = true
  try {
    profile.value = await updateZeroCityProfileBackground({ card_key: '', serial_no: null })
    appStore.showSuccess('已清空零号城主页背景。')
  } catch {
    appStore.showError('主页背景清空失败，请稍后再试。')
  } finally {
    isSavingBackground.value = false
  }
}

function openCard(item: CatalogItem) {
  detailReturnFocus = document.activeElement instanceof HTMLElement ? document.activeElement : null
  selectedItem.value = item
  detailFlipped.value = false
  document.body.style.overflow = 'hidden'
  void router.replace({ path: '/zero-city/cards', query: item.representative ? { serial: String(item.representative.serial_no) } : { card: item.key } })
  void nextTick(() => detailCloseButton.value?.focus())
}

function closeCard() {
  const returnFocus = detailReturnFocus
  detailReturnFocus = null
  selectedItem.value = null
  detailFlipped.value = false
  document.body.style.overflow = ''
  void router.replace({ path: '/zero-city/cards' })
  void nextTick(() => returnFocus?.focus())
}

function toggleDetailFace() {
  detailFlipped.value = !detailFlipped.value
}

function openCardFromQuery() {
  const serial = Number(route.query.serial || 0)
  const cardKey = String(route.query.card || '')
  const item = serial
    ? catalogItems.value.find(entry => entry.cards.some(card => Number(card.serial_no) === serial))
    : catalogItems.value.find(entry => entry.key === cardKey)
  if (item) {
    selectedItem.value = item
    detailFlipped.value = false
    document.body.style.overflow = 'hidden'
    void nextTick(() => detailCloseButton.value?.focus())
  }
}

function handleImageFallback(event: Event) {
  const image = event.target as HTMLImageElement | null
  if (!image || image.dataset.fallbackApplied === '1') return
  image.dataset.fallbackApplied = '1'
  image.onerror = null
  image.src = '/assets/zero-point-city/cards/collectible/common/low_battery_sprite.png'
  image.srcset = ''
}

function handleKeydown(event: KeyboardEvent) {
  if (!selectedItem.value) return
  if (event.key === 'Escape') {
    event.preventDefault()
    closeCard()
    return
  }
  if (event.key !== 'Tab' || !detailDialog.value) return
  const focusable = Array.from(detailDialog.value.querySelectorAll<HTMLElement>(
    'button:not([disabled]), [href], input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])'
  ))
  if (focusable.length === 0) {
    event.preventDefault()
    detailDialog.value.focus()
    return
  }
  const first = focusable[0]
  const last = focusable[focusable.length - 1]
  if (event.shiftKey && document.activeElement === first) {
    event.preventDefault()
    last.focus()
  } else if (!event.shiftKey && document.activeElement === last) {
    event.preventDefault()
    first.focus()
  }
}

watch([activeRarity, activeOwnership], () => { currentPage.value = 1 })
watch(pageCount, (nextPageCount) => {
  if (currentPage.value > nextPageCount) currentPage.value = nextPageCount
})
watch(() => [route.query.serial, route.query.card] as const, () => {
  if (!route.query.serial && !route.query.card) {
    selectedItem.value = null
    detailFlipped.value = false
    document.body.style.overflow = ''
    return
  }
  openCardFromQuery()
})

onMounted(() => {
  document.addEventListener('keydown', handleKeydown)
  void loadAlbum()
})

onBeforeUnmount(() => {
  document.removeEventListener('keydown', handleKeydown)
  document.body.style.overflow = ''
})
</script>

<style scoped>
.card-album-page {
  width: min(100%, 1320px);
  margin: 0 auto;
  padding: 1.5rem 1.5rem 5rem;
  color: var(--zc-text);
}

.album-hero,
.album-toolbar,
.active-background-card,
.album-content {
  border-radius: var(--zc-radius-panel);
  background: var(--zc-bg);
  box-shadow: var(--zc-shadow-soft);
}

.album-hero {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(26rem, 0.9fr);
  gap: 2rem;
  padding: clamp(1.25rem, 3vw, 2rem);
}

.album-hero-copy,
.album-summary {
  position: relative;
  z-index: 1;
}

.album-kicker {
  color: var(--zc-accent);
  font-size: 0.72rem;
  font-weight: 900;
  letter-spacing: 0;
  text-transform: uppercase;
}

.album-hero h1 {
  margin-top: 0.55rem;
  color: var(--zc-text-strong);
  font-size: clamp(1.75rem, 3vw, 2.4rem);
  font-weight: 950;
  letter-spacing: 0;
  line-height: 1.08;
}

.album-hero-copy > p:not(.album-kicker) {
  max-width: 44rem;
  margin-top: 1rem;
  color: var(--zc-muted);
  font-size: 0.95rem;
  font-weight: 650;
  line-height: 1.8;
}

.album-hero-actions,
.album-toolbar-actions,
.card-detail-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.7rem;
  margin-top: 1.25rem;
}

.album-summary {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 0.85rem;
  align-self: stretch;
}

.album-summary article {
  display: flex;
  min-height: 6rem;
  flex-direction: column;
  justify-content: center;
  border-radius: var(--zc-radius-card);
  padding: 1rem;
  background: var(--zc-bg);
  box-shadow: var(--zc-shadow-inset);
}

.album-summary strong {
  color: var(--zc-text-strong);
  font-size: 1.2rem;
  font-weight: 950;
}

.album-summary span,
.detail-facts span {
  margin-top: 0.3rem;
  color: var(--zc-muted);
  font-size: 0.74rem;
  font-weight: 800;
}

.album-btn {
  min-height: 2.6rem;
  border: 0;
  border-radius: 999px;
  padding: 0 1rem;
  background: var(--zc-bg);
  box-shadow: var(--zc-shadow-button);
  color: var(--zc-text-strong);
  font-size: 0.82rem;
  font-weight: 900;
  transition: transform 0.16s ease, box-shadow 0.16s ease, color 0.16s ease;
}

.album-btn:hover:not(:disabled) {
  transform: translateY(-1px);
}

.album-btn:active:not(:disabled) {
  box-shadow: var(--zc-shadow-inset);
}

.album-btn:disabled {
  cursor: not-allowed;
  opacity: 0.58;
}

.album-btn-primary {
  background: var(--zc-accent);
  color: var(--zc-accent-ink);
  box-shadow: 0 10px 22px color-mix(in srgb, var(--zc-accent) 25%, transparent);
}

.album-btn-danger {
  color: var(--zc-danger);
}

.album-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  margin-top: 1.2rem;
  padding: 1rem;
}

.album-filter-groups { display: grid; gap: 12px; min-width: 0; }
.ownership-filters { display: flex; flex-wrap: wrap; gap: 8px; padding-bottom: 12px; border-bottom: 1px solid var(--bd-ui-line); }

.album-toolbar-actions {
  margin-top: 0;
}

.rarity-filters {
  display: flex;
  flex: 1 1 auto;
  flex-wrap: wrap;
  gap: 0.55rem;
}

.filter-chip {
  display: inline-flex;
  min-height: 2.35rem;
  align-items: center;
  gap: 0.45rem;
  border: 0;
  border-radius: 999px;
  padding: 0 0.85rem;
  background: var(--zc-bg);
  box-shadow: var(--zc-shadow-button);
  color: var(--zc-muted);
  font-size: 0.78rem;
  font-weight: 900;
}

.filter-chip span {
  display: grid;
  min-width: 1.25rem;
  height: 1.25rem;
  place-items: center;
  border-radius: 999px;
  background: color-mix(in srgb, var(--zc-accent) 10%, var(--zc-bg));
  color: var(--zc-accent);
  font-size: 0.66rem;
}

.filter-chip.is-active {
  box-shadow: var(--zc-shadow-inset);
  color: var(--zc-text-strong);
}

.active-background-card {
  display: flex;
  align-items: center;
  gap: 1rem;
  margin-top: 1.2rem;
  padding: 1rem;
}

.active-background-card img {
  width: 4rem;
  height: 5.6rem;
  flex: 0 0 auto;
  border-radius: 0.7rem;
  object-fit: cover;
  box-shadow: 0 10px 24px color-mix(in srgb, var(--zc-shadow-dark) 55%, transparent);
}

.active-background-card div {
  min-width: 0;
  flex: 1 1 auto;
}

.active-background-card span {
  color: var(--zc-accent);
  font-size: 0.7rem;
  font-weight: 900;
}

.active-background-card h2 {
  margin-top: 0.25rem;
  color: var(--zc-text-strong);
  font-size: 1rem;
  font-weight: 950;
}

.active-background-card p {
  margin-top: 0.3rem;
  color: var(--zc-muted);
  font-size: 0.76rem;
  font-weight: 750;
}

.album-content {
  min-height: 24rem;
  margin-top: 1.2rem;
  padding: clamp(1rem, 3vw, 1.5rem);
}

.album-scope-note {
  margin: 0 0 1rem;
  color: var(--zc-muted);
  font-size: 0.76rem;
  font-weight: 750;
  line-height: 1.55;
}

.album-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(11.5rem, 1fr));
  gap: 1rem;
}

.album-card {
  overflow: hidden;
  border: 0;
  border-radius: 1.25rem;
  background: var(--zc-bg);
  box-shadow: var(--zc-shadow-button);
  color: var(--zc-text);
  text-align: left;
  transition: transform 0.18s ease, box-shadow 0.18s ease;
}

.album-card:hover {
  transform: translateY(-4px);
  box-shadow: var(--zc-shadow-soft);
}

.album-card.is-locked .album-card-image img { filter: saturate(.55) contrast(.92); opacity: .72; }

.album-card.is-background {
  box-shadow: 0 0 0 2px color-mix(in srgb, var(--zc-accent) 52%, transparent), var(--zc-shadow-soft);
}

.album-card-image {
  position: relative;
  overflow: hidden;
  background: color-mix(in srgb, var(--zc-surface-solid) 76%, var(--zc-bg));
}

.album-card-image img {
  display: block;
  width: 100%;
  aspect-ratio: 2 / 3;
  object-fit: cover;
  transition: transform 0.22s ease;
}

.locked-card-art {
  display: grid;
  width: 100%;
  aspect-ratio: 2 / 3;
  place-content: center;
  gap: 0.6rem;
  padding: 1rem;
  color: color-mix(in srgb, var(--zc-muted) 78%, transparent);
  background:
    linear-gradient(145deg, color-mix(in srgb, var(--zc-surface-solid) 78%, var(--zc-bg)), var(--zc-bg));
  text-align: center;
}

.locked-card-art b { font-size: 0.72rem; font-weight: 950; }
.locked-card-art span { font-size: 0.78rem; font-weight: 850; }
.album-card.is-locked .album-card-body { opacity: 0.72; }

.album-card:hover .album-card-image img {
  transform: scale(1.025);
}

.rarity-badge,
.background-badge {
  position: absolute;
  top: 0.65rem;
  border-radius: 999px;
  padding: 0.25rem 0.55rem;
  background: color-mix(in srgb, var(--zc-bg) 82%, transparent);
  font-size: 0.66rem;
  font-weight: 950;
  backdrop-filter: blur(0.75rem);
}

.rarity-badge {
  left: 0.65rem;
  color: var(--zc-text-strong);
}

.background-badge {
  right: 0.65rem;
  color: var(--zc-accent);
}

.album-card-body {
  display: grid;
  gap: 0.3rem;
  padding: 0.9rem;
}

.album-card-body strong {
  overflow: hidden;
  color: var(--zc-text-strong);
  font-size: 0.84rem;
  font-weight: 950;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.album-card-body span,
.album-card-body small {
  color: var(--zc-muted);
  font-size: 0.7rem;
  font-weight: 750;
}

.album-pagination {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 0.8rem;
  margin-top: 1.25rem;
}

.album-pagination span {
  color: var(--zc-muted);
  font-size: 0.76rem;
  font-weight: 850;
}

.album-state {
  display: grid;
  min-height: 20rem;
  place-items: center;
  align-content: center;
  gap: 0.7rem;
  color: var(--zc-muted);
  text-align: center;
}

.album-state strong {
  color: var(--zc-text-strong);
  font-size: 1rem;
  font-weight: 950;
}

.album-state p {
  max-width: 30rem;
  font-size: 0.82rem;
  font-weight: 700;
  line-height: 1.65;
}

.album-empty img {
  width: 8rem;
  max-height: 9rem;
  object-fit: contain;
  filter: drop-shadow(0 0.8rem 1rem color-mix(in srgb, var(--zc-shadow-dark) 30%, transparent));
}

.card-detail-backdrop {
  position: fixed;
  z-index: 220;
  inset: 0;
  display: grid;
  place-items: center;
  overflow-y: auto;
  padding: 1.2rem;
  background: color-mix(in srgb, #06101a 58%, transparent);
  backdrop-filter: blur(1rem);
}

.card-detail-dialog {
  position: relative;
  display: grid;
  width: min(100%, 54rem);
  grid-template-columns: minmax(17rem, 0.8fr) minmax(0, 1.2fr);
  gap: 1.5rem;
  border-radius: 1.25rem;
  padding: 1.25rem;
  background: var(--zc-bg);
  box-shadow: 0 2rem 6rem rgba(0, 0, 0, 0.34);
}

.card-detail-close {
  position: absolute;
  z-index: 2;
  top: 1rem;
  right: 1rem;
  display: grid;
  width: 2.5rem;
  height: 2.5rem;
  place-items: center;
  border: 0;
  border-radius: 50%;
  background: var(--zc-bg);
  box-shadow: var(--zc-shadow-button);
  color: var(--zc-text-strong);
  font-size: 1.25rem;
  font-weight: 800;
}

.detail-card-shell {
  width: min(100%, 22rem);
  aspect-ratio: 2 / 3;
  align-self: center;
  justify-self: center;
  border: 0;
  padding: 0;
  border-radius: 1rem;
  background: transparent;
  perspective: 80rem;
  color: inherit;
  cursor: pointer;
}

.detail-card-inner {
  position: relative;
  display: block;
  width: 100%;
  height: 100%;
  transform-style: preserve-3d;
  transition: transform 0.58s cubic-bezier(.16, 1, .3, 1);
}

.detail-card-shell.is-flipped .detail-card-inner { transform: rotateY(180deg); }

.detail-card-face {
  position: absolute;
  inset: 0;
  display: flex;
  overflow: hidden;
  border-radius: 1rem;
  background: var(--zc-bg);
  box-shadow: var(--zc-shadow-soft);
  backface-visibility: hidden;
}

.detail-card-front { align-items: stretch; }

.detail-card-front img {
  display: block;
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.detail-face-caption {
  position: absolute;
  right: 0.8rem;
  bottom: 0.8rem;
  left: 0.8rem;
  display: grid;
  gap: 0.25rem;
  padding: 0.7rem;
  border-radius: 0.7rem;
  background: color-mix(in srgb, var(--zc-bg) 88%, transparent);
  color: var(--zc-text-strong);
  text-align: left;
  backdrop-filter: blur(0.7rem);
}

.detail-face-caption b { font-size: 1rem; font-weight: 950; }
.detail-face-caption small { color: var(--zc-muted); font-size: 0.72rem; font-weight: 800; }

.detail-card-back {
  flex-direction: column;
  justify-content: center;
  gap: 0.8rem;
  padding: 1.5rem;
  border: 1px solid color-mix(in srgb, var(--rarity-color) 52%, transparent);
  background: linear-gradient(155deg, var(--zc-bg), color-mix(in srgb, var(--rarity-color) 9%, var(--zc-bg)));
  transform: rotateY(180deg);
  text-align: left;
}

.detail-card-back .detail-back-code { color: var(--zc-accent); font-size: 0.7rem; font-weight: 950; }
.detail-card-back strong { color: var(--zc-text-strong); font-size: 1.35rem; font-weight: 950; }
.detail-card-back em { color: var(--zc-text-strong); font-size: 0.9rem; font-style: normal; font-weight: 850; line-height: 1.6; }
.detail-card-back > span:not(.detail-back-code) { color: var(--zc-accent); font-size: 0.74rem; font-weight: 850; line-height: 1.5; }
.detail-card-back p { color: var(--zc-muted); font-size: 0.8rem; font-weight: 700; line-height: 1.7; }

.card-detail-copy {
  display: flex;
  flex-direction: column;
  justify-content: center;
  padding: 1rem 1rem 1rem 0;
}

.card-detail-copy h2 {
  margin-top: 0.5rem;
  color: var(--zc-text-strong);
  font-size: clamp(1.55rem, 3vw, 2.35rem);
  font-weight: 950;
  letter-spacing: 0;
  line-height: 1.04;
}

.detail-facts {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.75rem;
  margin-top: 1.4rem;
}

.detail-facts article {
  display: grid;
  gap: 0.25rem;
  border-radius: 1rem;
  padding: 0.85rem;
  background: var(--zc-bg);
  box-shadow: var(--zc-shadow-inset);
}

.detail-facts strong {
  color: var(--zc-text-strong);
  font-size: 0.85rem;
  font-weight: 900;
  overflow-wrap: anywhere;
}

.card-detail-note {
  margin-top: 1.2rem;
  color: var(--zc-muted);
  font-size: 0.78rem;
  font-weight: 700;
  line-height: 1.7;
}

.rarity-mythic { --rarity-color: #9f4bd7; }
.rarity-legendary { --rarity-color: #d08a24; }
.rarity-epic { --rarity-color: #6869d7; }
.rarity-rare { --rarity-color: #2c8fcc; }
.rarity-good { --rarity-color: #3a9b74; }
.rarity-common { --rarity-color: var(--zc-line-strong); }

.album-card[class*='rarity-'],
.detail-card-shell[class*='rarity-'] {
  outline: 1px solid color-mix(in srgb, var(--rarity-color) 54%, transparent);
  outline-offset: -1px;
}

.card-detail-fade-enter-active,
.card-detail-fade-leave-active {
  transition: opacity 0.18s ease;
}

.card-detail-fade-enter-from,
.card-detail-fade-leave-to {
  opacity: 0;
}

@media (max-width: 900px) {
  .card-album-page {
    padding: 1rem 1rem 4rem;
  }

  .album-hero,
  .card-detail-dialog {
    grid-template-columns: 1fr;
  }

  .album-toolbar,
  .active-background-card {
    align-items: flex-start;
    flex-direction: column;
  }

  .card-detail-copy {
    padding: 0.5rem;
  }
}

@media (max-width: 560px) {
  .album-summary,
  .detail-facts {
    grid-template-columns: 1fr 1fr;
  }

  .album-summary article {
    min-height: 5.7rem;
  }

  .album-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 0.75rem;
  }

  .album-card-body {
    padding: 0.7rem;
  }

  .card-detail-backdrop {
    padding: 0.5rem;
  }

  .card-detail-dialog {
    gap: 0.75rem;
    border-radius: 1.3rem;
    padding: 0.75rem;
  }

  .detail-card-shell { width: min(76vw, 20rem); }
}

@media (prefers-reduced-motion: reduce) {
  .album-card,
  .album-card-image img,
  .detail-card-inner,
  .card-detail-fade-enter-active,
  .card-detail-fade-leave-active { transition-duration: .01ms !important; }
}
</style>
