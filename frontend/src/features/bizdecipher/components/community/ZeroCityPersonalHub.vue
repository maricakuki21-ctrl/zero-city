<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { ArrowRight, BookOpen, ExternalLink, RefreshCw, Sparkles, X } from '@lucide/vue'
import { getCheckinCards } from '@/api/user'
import type { CheckinCollectibleCard, User } from '@/types'
import {
  getMyZeroCityProfile,
  updateMyZeroCityProfile,
  updateZeroCityProfileBackground,
  type BizProfile,
} from '@/features/bizdecipher/api/bizdecipher'
import { zeroCityMascots } from '@/features/bizdecipher/constants/zeroCityMascots'
import { zeroCityCardDisplayName, zeroCityCardImagePath, zeroCityCardRarityDisplayName } from '@/constants/zeroCityCardManifest'

type HubHistory = { posts: number; accepted: number; confirmed: number; assets: number }
type PersonalPost = {
  readonly id?: number
  readonly title: string
  readonly author?: string
  readonly card?: string
  readonly body?: string
  readonly replies: number
}

const props = defineProps<{
  user: User | null
  history: HubHistory
  historyError: boolean
  posts: readonly PersonalPost[]
  loading: boolean
}>()

const emit = defineEmits<{
  refresh: []
  profile: []
  cards: []
  assets: []
}>()

const profile = ref<BizProfile | null>(null)
const profileLoading = ref(false)
const profileError = ref(false)
const editorOpen = ref(false)
const nicknameDraft = ref('')
const avatarDraft = ref('')
const cards = ref<CheckinCollectibleCard[]>([])
const cardsLoading = ref(false)
const cardsError = ref(false)
const savingIdentity = ref(false)
const savingBackground = ref(false)
const editorMessage = ref('')
const editorError = ref(false)
const avatarFailed = ref(false)
let identityRequestID = 0
let cardsRequestID = 0
let identitySaveRequestID = 0
let backgroundSaveRequestID = 0

const displayName = computed(() => profile.value?.display_name || profile.value?.handle || props.user?.username || '我的城市')
const avatarSource = computed(() => profile.value?.avatar_url || props.user?.avatar_url || '')
const avatar = computed(() => avatarFailed.value || !avatarSource.value ? zeroCityMascots.archiveLibrarian : avatarSource.value)
const recentPosts = computed(() => props.posts.slice(0, 3))
const activeBackgroundSerial = computed(() => profile.value?.background_card_serial_no ?? null)
const backgroundStyle = computed(() => {
  const card = profile.value?.background_card_key
  const rarity = profile.value?.background_card_rarity
  if (!card || !rarity) return {}
  const image = `/assets/zero-point-city/cards/collectible/${rarity}/${card}.png`
  return { backgroundImage: `linear-gradient(90deg, color-mix(in srgb, var(--bd-surface) 96%, transparent), color-mix(in srgb, var(--bd-surface) 56%, transparent)), url("${image}")` }
})

async function loadIdentity(): Promise<boolean> {
  const userID = props.user?.id
  const requestID = ++identityRequestID
  if (!userID) {
    profile.value = null
    profileLoading.value = false
    profileError.value = false
    return false
  }
  profileLoading.value = true
  profileError.value = false
  try {
    const loaded = await getMyZeroCityProfile()
    if (requestID !== identityRequestID || props.user?.id !== userID) return false
    profile.value = loaded
    return true
  } catch {
    if (requestID !== identityRequestID || props.user?.id !== userID) return false
    profileError.value = true
    return false
  } finally {
    if (requestID === identityRequestID) profileLoading.value = false
  }
}

async function openEditor() {
  nicknameDraft.value = profile.value?.display_name || props.user?.username || ''
  avatarDraft.value = profile.value?.avatar_url || props.user?.avatar_url || ''
  editorMessage.value = ''
  editorError.value = false
  editorOpen.value = true
  if (cards.value.length || cardsLoading.value) return
  await loadCards()
}

async function loadCards() {
  const userID = props.user?.id
  if (!userID || cardsLoading.value) return
  const requestID = ++cardsRequestID
  cardsLoading.value = true
  cardsError.value = false
  try {
    const loaded = await getCheckinCards()
    if (requestID !== cardsRequestID || props.user?.id !== userID) return
    cards.value = loaded
  } catch {
    if (requestID === cardsRequestID && props.user?.id === userID) cardsError.value = true
  } finally {
    if (requestID === cardsRequestID) cardsLoading.value = false
  }
}

async function saveIdentity() {
  const userID = props.user?.id
  if (!userID || savingIdentity.value) return
  const username = nicknameDraft.value.trim()
  if (!username) {
    editorMessage.value = '昵称不能为空。'
    editorError.value = true
    return
  }
  const requestID = ++identitySaveRequestID
  const isCurrent = () => requestID === identitySaveRequestID && props.user?.id === userID
  savingIdentity.value = true
  editorMessage.value = ''
  editorError.value = false
  try {
    const saved = await updateMyZeroCityProfile({ display_name: username, avatar_url: avatarDraft.value.trim() })
    if (!isCurrent()) return
    profile.value = saved
    const reread = await loadIdentity()
    if (!isCurrent()) return
    editorMessage.value = reread ? '公开身份已保存并重新读取。' : '公开身份已保存，但重新读取失败，请刷新重试。'
    editorError.value = !reread
  } catch {
    if (!isCurrent()) return
    editorMessage.value = '公开身份保存失败，请稍后重试。'
    editorError.value = true
  } finally {
    if (isCurrent()) savingIdentity.value = false
  }
}

async function setBackground(card: CheckinCollectibleCard | null) {
  const userID = props.user?.id
  if (!userID || savingBackground.value) return
  const requestID = ++backgroundSaveRequestID
  const isCurrent = () => requestID === backgroundSaveRequestID && props.user?.id === userID
  savingBackground.value = true
  editorMessage.value = ''
  editorError.value = false
  try {
    const saved = await updateZeroCityProfileBackground(card
      ? { card_key: card.card_key, serial_no: card.serial_no }
      : { card_key: '', serial_no: null })
    if (!isCurrent()) return
    profile.value = saved
    editorMessage.value = card ? '卡片背景已更新。' : '卡片背景已清空。'
  } catch {
    if (!isCurrent()) return
    editorMessage.value = '卡片背景保存失败，请稍后重试。'
    editorError.value = true
  } finally {
    if (isCurrent()) savingBackground.value = false
  }
}

onMounted(() => { void loadIdentity() })
watch(avatarSource, () => { avatarFailed.value = false })
watch(() => props.user?.id, () => {
  identitySaveRequestID++
  backgroundSaveRequestID++
  savingIdentity.value = false
  savingBackground.value = false
  editorMessage.value = ''
  editorError.value = false
  nicknameDraft.value = ''
  avatarDraft.value = ''
  cardsRequestID++
  cards.value = []
  cardsLoading.value = false
  cardsError.value = false
  profile.value = null
  editorOpen.value = false
  void loadIdentity()
}, { flush: 'sync' })
</script>

<template>
  <section class="city-personal-hub" aria-labelledby="city-personal-title">
    <header class="city-personal-hero" :style="backgroundStyle">
      <div class="city-personal-identity">
        <img class="city-personal-avatar" :src="avatar" :alt="displayName" @error="avatarFailed = true" />
        <div class="city-personal-copy">
          <p class="city-personal-kicker">个人身份</p>
          <h1 id="city-personal-title">{{ displayName }}</h1>
          <p>{{ profile?.bio || '还没有写公开小传。先留下一个作品、问题或回应。' }}</p>
          <span v-if="profile?.background_card_key" class="city-personal-card-signal"><Sparkles :size="14" /> {{ zeroCityCardDisplayName(profile.background_card_key) }} · 已佩戴</span>
        </div>
      </div>
      <div class="city-personal-actions">
        <button class="city-personal-secondary" type="button" :disabled="profileLoading" @click="emit('refresh'); loadIdentity()"><RefreshCw :size="16" /> 刷新</button>
        <button class="city-personal-primary" type="button" :disabled="!user" @click="openEditor">编辑公开资料</button>
      </div>
    </header>

    <section v-if="editorOpen" class="city-personal-editor" aria-label="编辑公开身份">
      <header class="city-personal-section-head">
        <div><p>公开身份</p><h2>昵称、头像与卡片背景</h2></div>
        <button class="city-personal-icon" type="button" aria-label="关闭身份编辑" @click="editorOpen = false"><X :size="18" /></button>
      </header>
      <form class="city-personal-editor-form" @submit.prevent="saveIdentity">
        <label>昵称<input v-model="nicknameDraft" name="nickname" maxlength="64" autocomplete="nickname" /></label>
        <label>头像地址<input v-model="avatarDraft" name="avatar" type="url" placeholder="https://..." /></label>
        <button class="city-personal-primary" type="submit" :disabled="savingIdentity">{{ savingIdentity ? '保存中…' : '保存昵称与头像' }}</button>
      </form>
      <div class="city-personal-card-picker">
        <div class="city-personal-card-picker-head">
          <strong>卡片背景</strong>
          <button v-if="profile?.background_card_key" class="city-personal-link" type="button" :disabled="savingBackground" @click="setBackground(null)">清空背景</button>
        </div>
        <p v-if="cardsLoading" class="city-personal-note">正在读取真实卡册…</p>
        <div v-else-if="cardsError" class="city-personal-card-error" role="alert">
          <p>卡册暂时无法读取，昵称和头像仍可保存。</p>
          <button class="city-personal-secondary" type="button" @click="loadCards"><RefreshCw :size="15" /> 重试卡册</button>
        </div>
        <p v-else-if="!cards.length" class="city-personal-note">卡册中暂无可用收藏卡。</p>
        <div v-else class="city-personal-card-options">
          <button
            v-for="card in cards"
            :key="card.serial_no"
            type="button"
            :class="{ 'is-active': activeBackgroundSerial === card.serial_no }"
            :aria-pressed="activeBackgroundSerial === card.serial_no"
            :disabled="savingBackground || activeBackgroundSerial === card.serial_no"
            @click="setBackground(card)"
          >
            <img :src="zeroCityCardImagePath(card)" :alt="zeroCityCardDisplayName(card.card_key)" loading="lazy" />
            <span>{{ zeroCityCardDisplayName(card.card_key) }}<small>{{ zeroCityCardRarityDisplayName(card.rarity) }} · #{{ card.serial_no }}</small><small v-if="activeBackgroundSerial === card.serial_no" class="city-personal-equipped">已佩戴</small></span>
          </button>
        </div>
      </div>
      <p v-if="editorMessage" class="city-personal-editor-message" :class="{ 'is-error': editorError }" role="status">{{ editorMessage }}</p>
    </section>

    <div class="city-personal-stats" aria-label="城市历史统计">
      <div><strong>{{ historyError ? '—' : history.posts }}</strong><span>我的帖子</span></div>
      <div><strong>{{ historyError ? '—' : history.accepted }}</strong><span>已采纳</span></div>
      <div><strong>{{ historyError ? '—' : history.confirmed }}</strong><span>已确认</span></div>
      <div><strong>{{ historyError ? '—' : history.assets }}</strong><span>资产线索</span></div>
    </div>

    <div class="city-personal-grid">
      <section class="city-personal-section city-personal-section--history">
        <header class="city-personal-section-head">
          <div><p>留下的痕迹</p><h2>最近动态</h2></div>
          <button class="city-personal-link" type="button" @click="emit('refresh')">刷新历史 <ArrowRight :size="15" /></button>
        </header>
        <div v-if="loading" class="city-personal-empty">正在读取真实历史…</div>
        <div v-else-if="historyError" class="city-personal-empty city-personal-history-error" role="alert">
          <BookOpen :size="24" /><strong>历史暂时没有加载成功</strong><span>这些统计目前是未知状态，不会按零条记录展示。</span>
          <button class="city-personal-secondary" type="button" @click="emit('refresh')"><RefreshCw :size="15" /> 重新读取</button>
        </div>
        <div v-else-if="!recentPosts.length" class="city-personal-empty">
          <BookOpen :size="24" /><strong>还没有公开动态</strong><span>从闲聊广场、技术工坊或协作交流开始留下第一条记录。</span>
        </div>
        <div v-else class="city-personal-posts">
          <article v-for="post in recentPosts" :key="post.id" class="city-personal-post">
            <div><strong>{{ post.title }}</strong><span>{{ post.author || post.card || '居民' }} · {{ post.replies }} 回复</span></div>
            <small>{{ post.body }}</small>
          </article>
        </div>
      </section>

      <aside class="city-personal-section city-personal-section--next">
        <header class="city-personal-section-head"><div><p>下一步</p><h2>把身份变成经历</h2></div></header>
        <button class="city-personal-link-row" type="button" @click="emit('cards')"><span><Sparkles :size="17" />收藏卡册</span><ArrowRight :size="16" /></button>
        <button class="city-personal-link-row" type="button" @click="emit('assets')"><span><BookOpen :size="17" />能力资产</span><ArrowRight :size="16" /></button>
        <button class="city-personal-link-row" type="button" @click="emit('profile')"><span><ExternalLink :size="17" />公开主页设置</span><ArrowRight :size="16" /></button>
        <p v-if="profileError" class="city-personal-note">公开身份服务暂时没有返回资料，当前仍保留你的真实社区历史。</p>
      </aside>
    </div>

    <details class="city-personal-growth">
      <summary><span><strong>成长与资格</strong><small>等级和验证是服务结果，不在这里提前判定</small></span><ArrowRight :size="17" /></summary>
      <div class="city-personal-growth-body">
        <p>L0–L3 只在权威服务确认后显示。城内参与和可验证的外部作品都可以成为进入下一阶段的路径，当前页面不会把浏览、消费或热度直接当成等级。</p>
        <span>服务状态：{{ profileLoading ? '读取中' : profileError ? '暂未返回' : '已读取' }}</span>
      </div>
    </details>
  </section>
</template>

<style src="./zero-city-personal-hub.css"></style>
