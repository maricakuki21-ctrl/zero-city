<template>
  <AppLayout>
    <div class="zero-city-profile space-y-5">
      <section class="profile-hero" :style="heroStyle">
        <div class="hero-scrim"></div>
        <div class="hero-content">
          <div class="avatar-shell">
            <img v-if="profile?.avatar_url" :src="profile.avatar_url" :alt="displayName" />
            <img v-else :src="fallbackAvatar" :alt="displayName" class="avatar-mascot" />
          </div>
          <div class="min-w-0 flex-1">
            <p class="eyebrow">Zero City Profile</p>
            <h1>{{ displayName }}</h1>
            <p class="handle">{{ profile?.handle ? `@${profile.handle}` : `UID ${profile?.user_id || target}` }}</p>
            <p class="bio">{{ profile?.bio || '这个居民还没有写下公开小传。' }}</p>
          </div>
          <div class="hero-actions">
            <button class="profile-btn profile-btn-primary" type="button" :disabled="isFollowBusy || isSelfProfile" @click="toggleFollow">
              {{ followLabel }}
            </button>
            <button class="profile-btn profile-btn-ghost" type="button" @click="goCommunity">
              去社区
            </button>
          </div>
        </div>
      </section>

      <section class="stats-grid">
        <article v-for="stat in statCards" :key="stat.label" class="stat-card">
          <strong>{{ stat.value }}</strong>
          <span>{{ stat.label }}</span>
        </article>
      </section>

      <section class="grid gap-5 xl:grid-cols-[0.95fr_1.05fr]">
        <article class="panel-card">
          <div class="section-head">
            <div>
              <p class="eyebrow">Resource Nodes</p>
              <h2>公开资源节点</h2>
            </div>
            <button class="text-link" type="button" @click="goAccountSquare">查看资源广场</button>
          </div>
          <div v-if="isLoading" class="empty-line">正在读取主页...</div>
          <div v-else-if="sharedPools.length === 0" class="empty-line">这个居民暂时没有公开资源节点。</div>
          <div v-else class="pool-list">
            <article v-for="pool in sharedPools" :key="pool.id" class="pool-row">
              <div class="pool-row-main">
                <div>
                  <h3>{{ pool.name }}</h3>
                  <p>{{ pool.owner_label }} · {{ pool.status }} · {{ formatSharedPoolMultiplier(pool.rate_multiplier) }}x</p>
                </div>
                <div v-if="pool.owner_card_asset?.featured_card_key" class="pool-owner-card-signal">
                  <img :src="poolOwnerCardImage(pool)" :alt="pool.owner_card_asset.featured_card_key" loading="lazy" @error="handleCardImageFallback" />
                  <span>{{ poolCardSignal(pool) }}</span>
                </div>
                <div v-else-if="pool.owner_card_asset?.collectible_count" class="pool-owner-card-signal pool-owner-card-signal-empty">
                  <strong>{{ pool.owner_card_asset.collectible_count }}</strong>
                  <span>{{ poolCardSignal(pool) }}</span>
                </div>
              </div>
              <button class="text-link" type="button" @click="openPoolDiscussion(pool.id)">讨论</button>
            </article>
          </div>
        </article>

        <article class="panel-card">
          <div class="section-head">
            <div>
              <p class="eyebrow">Community</p>
              <h2>公开动态</h2>
            </div>
          </div>
          <div v-if="posts.length === 0" class="empty-line">还没有公开动态。</div>
          <div v-else class="post-list">
            <article v-for="post in posts" :key="post.id" class="post-row">
              <div class="flex flex-wrap items-center gap-2">
                <span class="tag">{{ post.kind }}</span>
                <span v-if="post.source_type === 'shared_pool'" class="tag tag-warm">资源节点 #{{ post.source_id }}</span>
              </div>
              <h3>{{ post.title }}</h3>
              <p>{{ post.body }}</p>
              <button v-if="post.source_type === 'shared_pool' && post.source_id" class="text-link" type="button" @click="openPoolDiscussion(Number(post.source_id))">
                回到池讨论
              </button>
            </article>
          </div>
        </article>
      </section>

      <section v-if="isOwnProfile" class="panel-card">
        <div class="section-head">
          <div>
            <p class="eyebrow">Card Background</p>
            <h2>主页卡牌背景</h2>
            <p class="hint">从已收藏的卡片中选择公开主页背景；完整发行信息和大图预览可在收藏卡册查看。</p>
          </div>
          <div class="background-actions">
            <button class="profile-btn profile-btn-primary" type="button" @click="router.push('/zero-city/cards')">
              查看完整卡册
            </button>
            <button class="profile-btn profile-btn-light" type="button" :disabled="isCardsLoading" @click="loadCards">
              {{ isCardsLoading ? '读取中...' : '刷新卡册' }}
            </button>
            <button v-if="profile?.background_card_key" class="profile-btn profile-btn-light" type="button" :disabled="isSavingBackground" @click="clearBackground">
              清空背景
            </button>
          </div>
        </div>
        <div v-if="activeBackgroundCard" class="active-card-summary">
          <img :src="cardImage(activeBackgroundCard)" :alt="activeBackgroundCard.card_key" loading="lazy" @error="handleCardImageFallback" />
          <div>
            <span class="tag">当前主页展示卡</span>
            <h3>{{ activeBackgroundCard.card_key }}</h3>
            <p>{{ cardEditionLine(activeBackgroundCard) }} · {{ rarityLabel(activeBackgroundCard.rarity) }}</p>
            <small>这张卡会同步成为公开主页和池主资源节点里的收藏资产信号。</small>
          </div>
        </div>
        <div v-else-if="profile?.background_card_key" class="active-card-summary active-card-summary-remote">
          <div class="active-card-fallback">{{ rarityLabel(profile.background_card_rarity || '') }}</div>
          <div>
            <span class="tag">当前主页展示卡</span>
            <h3>{{ profile.background_card_key }}</h3>
            <p>{{ profileBackgroundEditionLine }}</p>
            <small>卡册刷新后可以查看完整发行信息并更换展示卡。</small>
          </div>
        </div>
        <div v-if="isCardsLoading" class="empty-line">正在读取卡册资产...</div>
        <div v-else-if="cards.length === 0" class="empty-line">暂无可用收藏卡。先去每日礼盒获得收藏卡后再设置背景。</div>
        <div v-else class="card-grid">
          <article
            v-for="card in sortedCards"
            :key="card.serial_no"
            class="card-choice"
            :class="[`rarity-${card.rarity || 'empty'}`, { 'is-active': isBackgroundCard(card) }]"
          >
            <button class="card-preview-trigger" type="button" @click="openCardPreview(card)">
              <div class="card-choice-image">
                <img :src="cardImage(card)" :alt="card.card_key" loading="lazy" @error="handleCardImageFallback" />
                <span v-if="isBackgroundCard(card)" class="card-badge">主页底色</span>
                <span v-else class="card-badge card-badge-muted">点击预览</span>
              </div>
              <div class="card-choice-body">
                <span class="card-choice-name">{{ card.card_key }}</span>
                <div class="card-choice-meta">
                  <small>{{ rarityLabel(card.rarity) }}</small>
                  <small>{{ cardEditionLine(card) }}</small>
                  <small>序列 #{{ card.serial_no }}</small>
                </div>
              </div>
            </button>
            <button
              class="card-background-action"
              type="button"
              :disabled="isSavingBackground || isBackgroundCard(card)"
              @click="saveBackground(card)"
            >
              {{ isBackgroundCard(card) ? '当前主页背景' : '设为主页背景' }}
            </button>
          </article>
        </div>
      </section>
    </div>

    <Teleport to="body">
      <div v-if="previewCard" class="card-preview-overlay" role="presentation" @click.self="closeCardPreview">
        <section class="card-preview-dialog" role="dialog" aria-modal="true" :aria-label="`${previewCard.card_key} 收藏卡预览`">
          <button class="card-preview-close" type="button" aria-label="关闭预览" @click="closeCardPreview">×</button>
          <img :src="cardImage(previewCard)" :alt="previewCard.card_key" @error="handleCardImageFallback" />
          <div class="card-preview-copy">
            <span class="tag">{{ rarityLabel(previewCard.rarity) }}</span>
            <h2>{{ previewCard.card_key }}</h2>
            <p>{{ cardEditionLine(previewCard) }} · 序列 #{{ previewCard.serial_no }}</p>
            <button
              class="profile-btn profile-btn-primary"
              type="button"
              :disabled="isSavingBackground || isBackgroundCard(previewCard)"
              @click="saveBackground(previewCard)"
            >
              {{ isBackgroundCard(previewCard) ? '当前主页背景' : '设为主页背景' }}
            </button>
          </div>
        </section>
      </div>
    </Teleport>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { getCheckinCards } from '@/api/user'
import { zeroCityMascots } from '@/features/bizdecipher/constants/zeroCityMascots'
import { formatSharedPoolMultiplier } from '@/features/bizdecipher/components/shared-pool/sharedPoolPricing'
import type { CheckinCollectibleCard } from '@/types'
import {
  followZeroCityProfile,
  getZeroCityPublicProfile,
  unfollowZeroCityProfile,
  updateZeroCityProfileBackground,
  type BizProfile,
  type SharedPool,
  type ZeroCityFollowState,
  type ZeroCityPublicProfile
} from '@/features/bizdecipher/api/bizdecipher'
import type { CommunityPost } from '@/features/bizdecipher/api/community'

const ZERO_CITY_CARD_BASE = '/assets/zero-point-city/cards'
const RARITY_WEIGHT: Record<string, number> = {
  mythic: 6,
  legendary: 5,
  epic: 4,
  rare: 3,
  good: 2,
  common: 1
}

const route = useRoute()
const router = useRouter()
const appStore = useAppStore()
const authStore = useAuthStore()

const target = computed(() => String(route.params.target || ''))
const publicProfile = ref<ZeroCityPublicProfile | null>(null)
const isLoading = ref(false)
const isFollowBusy = ref(false)
const isCardsLoading = ref(false)
const isSavingBackground = ref(false)
const cards = ref<CheckinCollectibleCard[]>([])
const previewCard = ref<CheckinCollectibleCard | null>(null)

const profile = computed<BizProfile | null>(() => publicProfile.value?.profile ?? null)
const stats = computed(() => publicProfile.value?.stats)
const followState = computed<ZeroCityFollowState>(() => publicProfile.value?.follow_state ?? { is_following: false, followers: 0, following: 0 })
const sharedPools = computed<SharedPool[]>(() => publicProfile.value?.shared_pools ?? [])
const posts = computed<CommunityPost[]>(() => publicProfile.value?.posts ?? [])
const sortedCards = computed(() => [...cards.value].sort((a, b) => {
  const rarityDelta = (RARITY_WEIGHT[b.rarity] || 0) - (RARITY_WEIGHT[a.rarity] || 0)
  if (rarityDelta !== 0) return rarityDelta
  return Number(b.serial_no || 0) - Number(a.serial_no || 0)
}))
const activeBackgroundCard = computed(() => sortedCards.value.find((card) => isBackgroundCard(card)) || null)
const profileBackgroundEditionLine = computed(() => {
  const edition = profile.value?.background_card_edition_no
  const supply = profile.value?.background_card_edition_supply
  const serial = profile.value?.background_card_serial_no
  if (edition && supply) return `发行 #${edition} / ${supply}`
  if (edition) return `发行 #${edition}`
  if (serial) return `序列 #${serial}`
  return '已设置为主页底色'
})
const isOwnProfile = computed(() => Number(profile.value?.user_id || 0) > 0 && Number(authStore.user?.id || 0) === Number(profile.value?.user_id || 0))
const isSelfProfile = computed(() => isOwnProfile.value)
const displayName = computed(() => profile.value?.display_name || profile.value?.handle || `居民 ${profile.value?.user_id || target.value}`)
const fallbackAvatar = computed(() => zeroCityMascots.archiveLibrarian)
const followLabel = computed(() => {
  if (isSelfProfile.value) return '自己的主页'
  return followState.value.is_following ? '已关注' : '关注'
})
const statCards = computed(() => [
  { label: '关注者', value: stats.value?.followers ?? followState.value.followers ?? 0 },
  { label: '正在关注', value: stats.value?.following ?? followState.value.following ?? 0 },
  { label: '公开动态', value: stats.value?.community_posts ?? 0 },
  { label: '资源节点', value: stats.value?.shared_pools ?? 0 },
  { label: '收藏卡', value: stats.value?.collectible_cards ?? 0 }
])

const heroStyle = computed(() => {
  const cardKey = profile.value?.background_card_key
  const rarity = profile.value?.background_card_rarity
  if (!cardKey || !rarity) return {}
  return {
    backgroundImage: `linear-gradient(110deg, rgba(17, 20, 18, 0.86), rgba(17, 20, 18, 0.44)), url(${ZERO_CITY_CARD_BASE}/collectible/${rarity}/${cardKey}.png)`
  }
})

async function loadProfile() {
  if (!target.value) return
  isLoading.value = true
  try {
    publicProfile.value = await getZeroCityPublicProfile(target.value)
  } catch {
    publicProfile.value = null
    appStore.showError('零号城主页读取失败。')
  } finally {
    isLoading.value = false
  }
}

async function toggleFollow() {
  const userID = profile.value?.user_id
  if (!userID || isSelfProfile.value) return
  isFollowBusy.value = true
  try {
    const next = followState.value.is_following ? await unfollowZeroCityProfile(userID) : await followZeroCityProfile(userID)
    if (publicProfile.value) {
      publicProfile.value = { ...publicProfile.value, follow_state: next, stats: { ...publicProfile.value.stats, followers: next.followers, following: next.following } }
    }
  } catch {
    appStore.showError('关注状态更新失败。')
  } finally {
    isFollowBusy.value = false
  }
}

async function loadCards() {
  isCardsLoading.value = true
  try {
    cards.value = await getCheckinCards()
  } catch {
    cards.value = []
    appStore.showError('卡册读取失败。')
  } finally {
    isCardsLoading.value = false
  }
}

function openCardPreview(card: CheckinCollectibleCard) {
  previewCard.value = card
}

function closeCardPreview() {
  previewCard.value = null
}

async function saveBackground(card: CheckinCollectibleCard) {
  isSavingBackground.value = true
  try {
    const next = await updateZeroCityProfileBackground({ card_key: card.card_key, serial_no: card.serial_no })
    if (publicProfile.value) {
      publicProfile.value = { ...publicProfile.value, profile: next }
    }
    previewCard.value = null
    appStore.showSuccess('主页背景已更新。')
  } catch {
    appStore.showError('主页背景保存失败。')
  } finally {
    isSavingBackground.value = false
  }
}

async function clearBackground() {
  isSavingBackground.value = true
  try {
    const next = await updateZeroCityProfileBackground({ card_key: '', serial_no: null })
    if (publicProfile.value) {
      publicProfile.value = { ...publicProfile.value, profile: next }
    }
    appStore.showSuccess('主页背景已清空。')
  } catch {
    appStore.showError('主页背景清空失败。')
  } finally {
    isSavingBackground.value = false
  }
}

function cardImage(card: Pick<CheckinCollectibleCard, 'card_key' | 'rarity'>) {
  return `${ZERO_CITY_CARD_BASE}/collectible/${card.rarity}/${card.card_key}.png`
}

function poolOwnerCardImage(pool: SharedPool) {
  const asset = pool.owner_card_asset
  return `${ZERO_CITY_CARD_BASE}/collectible/${asset?.featured_card_rarity || 'common'}/${asset?.featured_card_key || 'clerk-common'}.png`
}

function isBackgroundCard(card: CheckinCollectibleCard) {
  const serial = profile.value?.background_card_serial_no
  if (serial) return Number(serial) === Number(card.serial_no)
  return profile.value?.background_card_key === card.card_key
}

function rarityLabel(rarity?: string | null) {
  const labels: Record<string, string> = {
    mythic: '神话',
    legendary: '传说',
    epic: '史诗',
    rare: '稀有',
    good: '良好',
    common: '普通'
  }
  return labels[String(rarity || '').toLowerCase()] || '未定级'
}

function cardEditionLine(card: CheckinCollectibleCard) {
  if (card.edition_no && card.edition_supply) return `发行 #${card.edition_no} / ${card.edition_supply}`
  if (card.edition_no) return `发行 #${card.edition_no}`
  return `序列 #${card.serial_no}`
}

function poolCardSignal(pool: SharedPool) {
  const asset = pool.owner_card_asset
  if (!asset) return '暂无收藏资产'
  if (asset.featured_card_key) {
    const badge = asset.profile_background_ready ? '主页底色' : '精选卡'
    const edition = asset.featured_card_edition_no && asset.featured_card_supply ? ` #${asset.featured_card_edition_no}/${asset.featured_card_supply}` : ''
    return `${badge} · ${rarityLabel(asset.featured_card_rarity)}${edition}`
  }
  if (asset.collectible_count > 0) return `收藏 ${asset.collectible_count} 张 · 最高 ${rarityLabel(asset.highest_rarity)}`
  return '暂无收藏资产'
}

function handleCardImageFallback(event: Event) {
  const img = event.target as HTMLImageElement | null
  if (img) img.style.display = 'none'
}

function openPoolDiscussion(poolID: number) {
  router.push({ path: '/community', query: { source_type: 'shared_pool', source_id: String(poolID), kind: 'pool' } })
}

function goCommunity() {
  router.push('/community')
}

function goAccountSquare() {
  router.push('/account-square')
}

watch(target, () => {
  cards.value = []
  void loadProfile()
}, { immediate: true })

watch(isOwnProfile, (own) => {
  if (!own) {
    cards.value = []
    return
  }
  if (cards.value.length === 0) {
    void loadCards()
  }
}, { immediate: true })
</script>

<style scoped>
.zero-city-profile {
  color: var(--zc-text);
}

.profile-hero {
  position: relative;
  min-height: 18rem;
  overflow: hidden;
  border: 1px solid var(--zc-line);
  border-radius: var(--zc-radius-panel);
  background:
    linear-gradient(135deg, color-mix(in srgb, var(--zc-bg) 94%, transparent), color-mix(in srgb, var(--zc-accent) 18%, var(--zc-bg)) 54%, color-mix(in srgb, var(--zc-accent-2) 18%, var(--zc-bg)));
  background-position: center;
  background-size: cover;
  color: var(--zc-text-strong);
  box-shadow: var(--zc-shadow-soft);
}

.hero-scrim {
  position: absolute;
  inset: 0;
  background:
    linear-gradient(180deg, transparent, color-mix(in srgb, var(--zc-bg) 38%, transparent)),
    radial-gradient(circle at 82% 20%, color-mix(in srgb, var(--zc-accent-2) 20%, transparent), transparent 28%);
}

.hero-content {
  position: relative;
  display: flex;
  min-height: 18rem;
  align-items: flex-end;
  gap: 1rem;
  padding: 2rem;
}

.avatar-shell {
  display: grid;
  width: 5.8rem;
  height: 5.8rem;
  flex: 0 0 auto;
  place-items: center;
  overflow: hidden;
  border: 1px solid color-mix(in srgb, var(--zc-accent) 28%, var(--zc-line));
  border-radius: 1.4rem;
  background:
    radial-gradient(circle at 50% 78%, color-mix(in srgb, var(--zc-accent) 24%, transparent), transparent 48%),
    color-mix(in srgb, var(--zc-surface-solid) 72%, transparent);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.14), 0 18px 38px rgba(0, 0, 0, 0.24);
  font-size: 2rem;
  font-weight: 900;
}

.avatar-shell img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.avatar-shell .avatar-mascot {
  width: 92%;
  height: 92%;
  object-fit: contain;
  padding: 0.18rem;
  filter: drop-shadow(0 12px 16px rgba(0, 0, 0, 0.22));
}

.eyebrow {
  color: var(--zc-accent);
  font-size: 0.72rem;
  font-weight: 900;
  letter-spacing: 0.16em;
  text-transform: uppercase;
}

.profile-hero .eyebrow {
  color: color-mix(in srgb, var(--zc-accent) 78%, white);
}

h1 {
  margin-top: 0.35rem;
  color: var(--zc-text-strong);
  font-size: clamp(2rem, 4vw, 4rem);
  font-weight: 950;
  letter-spacing: 0;
  line-height: 1;
}

.handle {
  margin-top: 0.45rem;
  color: var(--zc-muted);
  font-size: 0.9rem;
}

.bio {
  margin-top: 0.8rem;
  max-width: 42rem;
  color: color-mix(in srgb, var(--zc-text) 78%, transparent);
  font-size: 0.95rem;
  line-height: 1.75;
}

.hero-actions {
  display: flex;
  flex: 0 0 auto;
  flex-wrap: wrap;
  gap: 0.65rem;
}

.profile-btn {
  border-radius: 999px;
  padding: 0.65rem 0.95rem;
  font-size: 0.85rem;
  font-weight: 850;
  transition: transform 0.16s ease, border-color 0.16s ease, background-color 0.16s ease, color 0.16s ease;
}

.profile-btn:hover:not(:disabled) {
  transform: translateY(-1px);
}

.profile-btn-primary {
  border: 1px solid color-mix(in srgb, var(--zc-accent) 70%, transparent);
  background: linear-gradient(135deg, var(--zc-accent), color-mix(in srgb, var(--zc-accent) 74%, var(--zc-accent-2)));
  color: #07110d;
}

.profile-btn-ghost,
.profile-btn-light {
  border: 1px solid var(--zc-line-strong);
  background: color-mix(in srgb, var(--zc-card-strong) 84%, transparent);
  color: var(--zc-text-strong);
}

.profile-btn:disabled {
  cursor: not-allowed;
  opacity: 0.55;
}

.background-actions {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 0.65rem;
}

.stats-grid {
  display: grid;
  gap: 0.75rem;
  grid-template-columns: repeat(5, minmax(0, 1fr));
}

.stat-card,
.panel-card {
  border: 1px solid var(--zc-line);
  border-radius: var(--zc-radius-card);
  background: var(--zc-surface);
  box-shadow: var(--zc-shadow-soft);
  backdrop-filter: var(--zc-blur);
  -webkit-backdrop-filter: var(--zc-blur);
}

.stat-card {
  padding: 1rem;
}

.stat-card strong {
  display: block;
  color: var(--zc-text-strong);
  font-size: 1.45rem;
  font-weight: 950;
}

.stat-card span {
  display: block;
  margin-top: 0.25rem;
  color: var(--zc-muted);
  font-size: 0.78rem;
  font-weight: 800;
}

.panel-card {
  padding: 1.25rem;
}

.section-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
}

.section-head h2 {
  margin-top: 0.25rem;
  color: var(--zc-text-strong);
  font-size: 1.2rem;
  font-weight: 950;
}

.hint,
.empty-line {
  margin-top: 0.5rem;
  color: var(--zc-muted);
  font-size: 0.86rem;
  line-height: 1.7;
}

.text-link {
  color: var(--zc-accent);
  font-size: 0.82rem;
  font-weight: 900;
}

.text-link:hover {
  color: var(--zc-text-strong);
}

.pool-list,
.post-list {
  margin-top: 1rem;
  display: grid;
  gap: 0.75rem;
}

.pool-row,
.post-row {
  border: 1px solid var(--zc-line);
  border-radius: 1rem;
  background: var(--zc-card);
  padding: 0.95rem;
}

.pool-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
}

.pool-row-main {
  display: flex;
  min-width: 0;
  flex: 1 1 auto;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
}

.pool-owner-card-signal {
  display: inline-flex;
  max-width: 16rem;
  flex: 0 0 auto;
  align-items: center;
  gap: 0.55rem;
  border: 1px solid color-mix(in srgb, var(--zc-accent) 22%, var(--zc-line));
  border-radius: 999px;
  background: color-mix(in srgb, var(--zc-accent) 8%, var(--zc-card));
  padding: 0.25rem 0.65rem 0.25rem 0.25rem;
  color: var(--zc-text-strong);
  font-size: 0.72rem;
  font-weight: 850;
}

.pool-owner-card-signal img {
  width: 2.1rem;
  height: 2.1rem;
  flex: 0 0 auto;
  border-radius: 999px;
  object-fit: cover;
}

.pool-owner-card-signal span {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.pool-owner-card-signal-empty {
  padding-left: 0.55rem;
}

.pool-owner-card-signal-empty strong {
  color: var(--zc-accent);
  font-size: 0.88rem;
  font-weight: 950;
}

.pool-row h3,
.post-row h3 {
  color: var(--zc-text-strong);
  font-size: 0.95rem;
  font-weight: 900;
}

.pool-row p,
.post-row p {
  margin-top: 0.35rem;
  color: var(--zc-muted);
  font-size: 0.78rem;
  line-height: 1.65;
}

.tag {
  border-radius: 999px;
  background: color-mix(in srgb, var(--zc-accent) 12%, transparent);
  padding: 0.18rem 0.55rem;
  color: var(--zc-accent);
  font-size: 0.68rem;
  font-weight: 900;
}

.tag-warm {
  background: color-mix(in srgb, var(--zc-accent-2) 16%, transparent);
  color: var(--zc-accent-2);
}

.active-card-summary {
  margin-top: 1rem;
  display: flex;
  align-items: center;
  gap: 1rem;
  border: 1px solid color-mix(in srgb, var(--zc-accent) 34%, var(--zc-line));
  border-radius: 1rem;
  background:
    linear-gradient(135deg, color-mix(in srgb, var(--zc-accent) 10%, transparent), transparent 54%),
    var(--zc-card);
  padding: 0.85rem;
}

.active-card-summary img,
.active-card-fallback {
  width: 5.2rem;
  height: 7.8rem;
  flex: 0 0 auto;
  border-radius: 0.75rem;
  object-fit: cover;
  box-shadow: 0 14px 28px rgba(0, 0, 0, 0.2);
}

.active-card-fallback {
  display: grid;
  place-items: center;
  border: 1px solid var(--zc-line);
  background: color-mix(in srgb, var(--zc-accent) 12%, var(--zc-card));
  color: var(--zc-accent);
  font-size: 0.78rem;
  font-weight: 950;
}

.active-card-summary h3 {
  margin-top: 0.5rem;
  color: var(--zc-text-strong);
  font-size: 1rem;
  font-weight: 950;
}

.active-card-summary p,
.active-card-summary small {
  display: block;
  margin-top: 0.28rem;
  color: var(--zc-muted);
  font-size: 0.78rem;
  line-height: 1.6;
}

.card-grid {
  margin-top: 1rem;
  display: grid;
  gap: 0.75rem;
  grid-template-columns: repeat(auto-fill, minmax(9.6rem, 1fr));
}

.card-choice {
  overflow: hidden;
  border: 1px solid var(--zc-line);
  border-radius: 1rem;
  background: var(--zc-card);
  color: var(--zc-text);
  text-align: left;
  transition: transform 0.16s ease, border-color 0.16s ease, box-shadow 0.16s ease;
}

.card-choice:hover {
  border-color: color-mix(in srgb, var(--zc-accent) 36%, var(--zc-line));
  transform: translateY(-2px);
}

.card-preview-trigger {
  width: 100%;
  border: 0;
  background: transparent;
  color: inherit;
  text-align: left;
  cursor: zoom-in;
}

.card-background-action {
  width: calc(100% - 1.5rem);
  min-height: 2.2rem;
  margin: 0 0.75rem 0.75rem;
  border: 1px solid color-mix(in srgb, var(--zc-accent) 38%, var(--zc-line));
  border-radius: 0.7rem;
  background: color-mix(in srgb, var(--zc-accent) 9%, var(--zc-card));
  color: var(--zc-accent);
  font-size: 0.74rem;
  font-weight: 900;
  cursor: pointer;
}

.card-background-action:disabled {
  cursor: default;
  opacity: 0.62;
}

.card-choice.is-active {
  border-color: color-mix(in srgb, var(--zc-accent) 64%, var(--zc-line));
  box-shadow: 0 0 0 2px color-mix(in srgb, var(--zc-accent) 18%, transparent), var(--zc-shadow-soft);
}

.card-choice.rarity-mythic {
  border-color: color-mix(in srgb, #a855f7 50%, var(--zc-line));
}

.card-choice.rarity-legendary {
  border-color: color-mix(in srgb, #f59e0b 48%, var(--zc-line));
}

.card-choice.rarity-epic {
  border-color: color-mix(in srgb, #6366f1 42%, var(--zc-line));
}

.card-choice.rarity-rare {
  border-color: color-mix(in srgb, #0ea5e9 38%, var(--zc-line));
}

.card-choice.rarity-good {
  border-color: color-mix(in srgb, #22c55e 34%, var(--zc-line));
}

.card-choice-image {
  position: relative;
  background: color-mix(in srgb, var(--zc-bg) 70%, transparent);
}

.card-choice-image img {
  width: 100%;
  aspect-ratio: 2 / 3;
  object-fit: cover;
  display: block;
}

.card-badge {
  position: absolute;
  left: 0.55rem;
  top: 0.55rem;
  border: 1px solid color-mix(in srgb, var(--zc-accent) 42%, transparent);
  border-radius: 999px;
  background: color-mix(in srgb, var(--zc-bg) 70%, transparent);
  padding: 0.16rem 0.48rem;
  color: var(--zc-accent);
  font-size: 0.66rem;
  font-weight: 950;
  backdrop-filter: blur(10px);
  -webkit-backdrop-filter: blur(10px);
}

.card-badge-muted {
  border-color: color-mix(in srgb, var(--zc-line) 80%, transparent);
  color: var(--zc-muted);
}

.card-choice-body {
  padding: 0.7rem 0.75rem 0.8rem;
}

.card-choice-name {
  display: block;
  color: var(--zc-text-strong);
  font-size: 0.78rem;
  font-weight: 900;
  overflow-wrap: anywhere;
}

.card-choice-meta {
  margin-top: 0.45rem;
  display: grid;
  gap: 0.18rem;
}

.card-choice-meta small {
  color: var(--zc-muted);
  font-size: 0.7rem;
  line-height: 1.35;
}

.card-preview-overlay {
  position: fixed;
  inset: 0;
  z-index: 2400;
  display: grid;
  place-items: center;
  padding: 1rem;
  background: rgba(8, 15, 26, 0.72);
  backdrop-filter: blur(12px);
  -webkit-backdrop-filter: blur(12px);
}

.card-preview-dialog {
  position: relative;
  width: min(92vw, 46rem);
  max-height: calc(100vh - 2rem);
  overflow: auto;
  display: grid;
  grid-template-columns: minmax(15rem, 24rem) minmax(14rem, 1fr);
  gap: 1.25rem;
  align-items: center;
  border: 1px solid var(--zc-line);
  border-radius: var(--zc-radius-card);
  background: var(--zc-surface);
  box-shadow: var(--zc-shadow-strong);
  padding: 1.25rem;
}

.card-preview-dialog > img {
  width: 100%;
  max-height: calc(100vh - 4.5rem);
  object-fit: contain;
  border-radius: 0.85rem;
  background: var(--zc-bg);
}

.card-preview-copy {
  min-width: 0;
}

.card-preview-copy h2 {
  margin-top: 0.65rem;
  color: var(--zc-text-strong);
  font-size: 1.35rem;
  overflow-wrap: anywhere;
}

.card-preview-copy p {
  margin: 0.55rem 0 1rem;
  color: var(--zc-muted);
  line-height: 1.65;
}

.card-preview-close {
  position: absolute;
  top: 0.65rem;
  right: 0.65rem;
  z-index: 2;
  width: 2.25rem;
  height: 2.25rem;
  border: 1px solid var(--zc-line);
  border-radius: 50%;
  background: var(--zc-surface);
  color: var(--zc-text-strong);
  font-size: 1.35rem;
  line-height: 1;
  cursor: pointer;
}

@media (max-width: 720px) {
  .card-preview-dialog {
    grid-template-columns: 1fr;
    width: min(94vw, 28rem);
  }

  .card-preview-dialog > img {
    max-height: 58vh;
  }
}

@media (max-width: 900px) {
  .hero-content {
    align-items: flex-start;
    flex-direction: column;
    justify-content: flex-end;
  }

  .pool-row,
  .pool-row-main,
  .active-card-summary {
    align-items: flex-start;
    flex-direction: column;
  }

  .pool-owner-card-signal {
    max-width: 100%;
  }

  .stats-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
</style>
