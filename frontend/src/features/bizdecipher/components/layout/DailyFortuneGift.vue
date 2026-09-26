<template>
  <div ref="rootRef" class="gift-shell">
    <button
      ref="triggerRef"
      class="fortune-entry gift-entry"
      :class="{ claimed: status?.free_claimed }"
      type="button"
      :aria-label="text.gift"
      aria-haspopup="dialog"
      :aria-expanded="open"
      aria-controls="daily-fortune-dialog"
      @click="openGift"
    >
      <span v-if="!status?.free_claimed" class="gift-dot"></span>
      <span class="gift-box">🎁</span>
      <span class="gift-text">{{ text.gift }}</span>
    </button>

    <transition name="fortune-pop">
      <div v-if="open" class="modal-backdrop">
        <div
          id="daily-fortune-dialog"
          ref="dialogRef"
          class="modal mini-popover"
          :class="{ 'is-drawing': view === 'draw' }"
          role="dialog"
          aria-modal="true"
          :aria-labelledby="view === 'draw' ? 'daily-fortune-draw-title' : 'daily-fortune-title'"
          :aria-describedby="view === 'draw' && selectedType === 'credit' ? 'credit-lottery-description' : view === 'draw' ? 'daily-fortune-draw-description' : 'daily-fortune-subtitle'"
          :aria-busy="Boolean(statusLoading || loadingType || recoveryLoading || panelLoading)"
          tabindex="-1"
          @click.stop
          @keydown="handleDialogKeydown"
        >
          <div class="mini-stage">
            <section ref="chooseViewRef" class="mini-view choose-view" tabindex="-1">
              <div class="modal-head">
                <div>
                  <h2 id="daily-fortune-title">{{ text.title }}</h2>
                  <div id="daily-fortune-subtitle" class="modal-sub">{{ text.subtitle }}</div>
                </div>
                <button class="close" type="button" aria-label="关闭每日礼盒" @click="closeGift">×</button>
              </div>

              <div v-if="activePanel === 'records'" class="activity-panel">
                <div class="panel-head">
                  <button class="panel-back" type="button" @click.stop="returnToChooseHome">‹</button>
                  <b>{{ text.recordTitle }}</b>
                </div>
                <div class="panel-list record-list">
                  <div v-if="panelLoading" class="panel-empty">正在加载抽奖记录…</div>
                  <div v-else-if="panelLoadError" class="panel-empty panel-load-error" role="alert">
                    <span>{{ panelLoadError }}</span>
                    <button type="button" @click="openActivityPanel('records')">重新加载</button>
                  </div>
                  <div v-else-if="!checkinRecords.length" class="panel-empty">{{ text.emptyRecords }}</div>
                  <template v-else>
                    <div v-for="record in checkinRecords" :key="record.id" class="record-row">
                      <div>
                        <b>{{ recordTypeLabel(record.type) }}</b>
                        <span>{{ recordDateLabel(record.created_at) }}</span>
                      </div>
                      <div>
                        <strong>{{ recordRewardLabel(record) }}</strong>
                        <em v-if="record.collectible_card">{{ text.rareCardFound }} · {{ collectibleCardName(record.collectible_card) }}</em>
                        <em v-else-if="record.jackpot_hit">{{ text.jackpotHit }}</em>
                      </div>
                    </div>
                  </template>
                </div>
              </div>

              <div v-else-if="activePanel === 'cards'" class="activity-panel">
                <div class="panel-head panel-head-with-action">
                  <button class="panel-back" type="button" @click.stop="returnToChooseHome">‹</button>
                  <b>{{ text.collectionTitle }}</b>
                  <button class="panel-album-link" type="button" @click="openFullAlbum()">完整卡册</button>
                </div>
                <div class="panel-list card-collection-list">
                  <div v-if="panelLoading" class="panel-empty">正在加载收藏卡…</div>
                  <div v-else-if="panelLoadError" class="panel-empty panel-load-error" role="alert">
                    <span>{{ panelLoadError }}</span>
                    <button type="button" @click="openActivityPanel('cards')">重新加载</button>
                  </div>
                  <div v-else-if="!collectibleCards.length" class="panel-empty">{{ text.emptyCards }}</div>
                  <template v-else>
                    <button
                      v-for="group in collectibleCardGroups"
                      :key="group.card.card_key"
                      type="button"
                      class="collectible-card"
                      :class="[group.card.rarity, `card-${group.card.card_key}`]"
                      @click="openFullAlbum(group.card)"
                    >
                      <img
                        class="collectible-card-base"
                        :src="collectibleCardImage(group.card)"
                        :srcset="collectibleCardImageSrcSet(group.card)"
                        :alt="collectibleCardName(group.card)"
                        loading="lazy"
                        decoding="async"
                        fetchpriority="low"
                        sizes="(max-width: 640px) 42vw, 140px"
                        @error="handleImageFallback($event, `${ZERO_CITY_CARD_BASE}/collectible/common/low_battery_sprite.png`)"
                      />
                      <div class="collectible-topline">
                        <i>{{ collectibleRarityShort(group.card.rarity) }}</i>
                      </div>
                      <div class="collectible-art" aria-hidden="true">
                        <span class="collectible-image-sheen"></span>
                      </div>
                      <div class="collectible-body">
                        <b>{{ collectibleCardName(group.card) }}</b>
                        <strong>{{ group.count > 1 ? `×${group.count}` : collectibleEditionLabel(group.card) }}</strong>
                      </div>
                    </button>
                  </template>
                </div>
              </div>

              <div v-else-if="activePanel === 'shop'" class="activity-panel card-shop-panel">
                <div class="panel-head panel-head-detail">
                  <button class="panel-back" type="button" @click.stop="returnToChooseHome">‹</button>
                  <div>
                    <b>{{ shopText.title }}</b>
                  </div>
                </div>
                <div class="panel-list card-shop-list">
                  <div v-if="panelLoading" class="panel-empty">正在读取商店状态…</div>
                  <div v-else-if="panelLoadError" class="panel-empty panel-load-error" role="alert">
                    <span>{{ panelLoadError }}</span>
                    <button type="button" @click="openActivityPanel('shop')">重新加载</button>
                  </div>
                  <template v-else>
                    <div class="shop-balance-row"><span>{{ shopText.balance }}</span><b>{{ formatAsset(status?.credit_balance ?? user?.credit_balance, text.creditShort) }}</b></div>
                    <button
                      v-for="tier in cardShopTiers"
                      :key="tier.rarity"
                      type="button"
                      class="shop-tier-card"
                      :class="tier.rarity"
                      :disabled="buyingRarity !== null || shopTierDisabled(tier)"
                      @click="buyShopTier(tier)"
                    >
                      <span>{{ tier.badge }}</span>
                      <b>{{ tier.title }}</b>
                      <strong>{{ tier.priceLabel }}</strong>
                    </button>
                  </template>
                </div>
              </div>

              <template v-else>
                <div v-if="statusLoading" class="status-read-state" role="status" aria-live="polite">
                  <span class="status-read-spinner" aria-hidden="true"></span>
                  <b>正在读取今日礼盒状态…</b>
                  <small>确认剩余次数与余额后才能发起新抽奖。</small>
                </div>
                <div v-else-if="statusLoadError" class="status-read-state status-read-error" role="alert">
                  <b>今日状态读取失败</b>
                  <small>{{ statusLoadError }}</small>
                  <button type="button" @click="retryLoadStatus">重新加载</button>
                </div>
                <template v-else>
                <div class="daily-status compact-status">
                  <div class="status-tile">
                    <div class="status-kicker">{{ text.today }}</div>
                    <div class="status-main" :class="{ done: status?.free_claimed }">
                      {{ status?.free_claimed ? text.claimed : text.unclaimed }}
                    </div>
                    <div class="status-sub">{{ text.streak }}</div>
                  </div>
                  <div class="status-tile quota-list">
                    <div class="quota-row"><b>{{ text.freeShort }}</b><span>{{ remaining('free') }} / 1</span></div>
                    <div class="quota-row"><b>{{ text.creditShort }}</b><span>{{ remaining('credit') }} / {{ creditLimit }}</span></div>
                    <div class="quota-row"><b>{{ text.balanceShort }}</b><span>{{ remaining('balance') }} / {{ balanceLimit }}</span></div>
                  </div>
                  <div v-if="milestones.length" class="milestone-mini-row">
                    <button
                      v-for="item in milestones"
                      :key="item.days"
                      type="button"
                      class="milestone-chip"
                      :title="milestoneRewardLabel(item)"
                      :aria-label="`${item.days}${text.dayUnit} · ${milestoneRewardLabel(item)} · ${milestoneStatusLabel(item)}`"
                      :class="{ eligible: milestoneClaimable(item), claimed: item.claimed, locked: !milestoneClaimable(item) && !item.claimed }"
                      :disabled="!milestoneClaimable(item) || claimingMilestone === item.days"
                      @click="claimMilestone(item.days)"
                    >
                      <b>{{ item.days }}{{ text.dayUnit }}</b><span>{{ milestoneMiniReward(item) }}</span><i>{{ milestoneStatusLabel(item) }}</i>
                    </button>
                  </div>
                  <p v-if="milestones.length" class="milestone-policy">{{ milestonePolicy }}</p>
                </div>

                <div class="sign-options">
                  <button class="records-corner" type="button" @click.stop="openActivityPanel('records')">{{ text.recordShort }}</button>
                  <button class="cards-corner" type="button" @click.stop="openActivityPanel('cards')">{{ text.collectionShort }}</button>
                  <button class="shop-corner" type="button" @click.stop="openActivityPanel('shop')">买卡</button>
                  <div
                    v-for="option in options"
                    :key="option.type"
                    class="sign-card-wrap"
                  >
                    <button
                      type="button"
                      class="sign-card"
                      :class="{ used: option.used, disabled: option.disabled }"
                      :disabled="option.disabled"
                      @click="enterDraw(option.type)"
                    >
                      <div class="sign-icon">{{ option.icon }}</div>
                      <div class="sign-title">{{ option.title }}</div>
                      <div class="sign-desc">{{ option.desc }}</div>
                      <div class="sign-cost">{{ option.costLabel }}</div>
                      <div class="sign-meta">{{ option.meta }}</div>
                      <div class="sign-prizes">{{ option.prizes }}</div>
                    </button>
                  </div>
                </div>

                <div class="pool-preview">
                  <div class="pool-preview-head"><b>{{ text.poolTitle }}</b><span>{{ text.poolSub }}</span></div>
                  <div class="jackpot-grid">
                    <div class="jackpot-tile"><b>{{ formatAsset(status?.credit_jackpot, text.creditShort) }}</b><span>{{ text.creditPool }}</span></div>
                    <div class="jackpot-tile"><b>{{ formatAsset(status?.balance_jackpot, text.balanceShort) }}</b><span>{{ text.balancePool }}</span></div>
                    <div class="jackpot-tile"><b>{{ cycleProgressText }}</b><span>{{ text.cycleTitle }}</span></div>
                  </div>
                </div>
                </template>
              </template>
            </section>

            <section ref="drawViewRef" class="mini-view draw-view" tabindex="-1">
              <div class="mini-draw-head">
                <button class="back-button" type="button" aria-label="返回抽奖方式" @click="backToChoose">‹</button>
                <div id="daily-fortune-draw-title" class="mini-type-badge">{{ selectedOption?.title || text.freeShort }}</div>
                <button class="close" type="button" aria-label="关闭每日礼盒" @click="closeGift">×</button>
              </div>

              <CreditLotteryThreeRounds
                v-if="selectedType === 'credit'"
                :remaining="remaining('credit')"
                :type-label="text.creditShort"
                :ready-title="text.ready"
                :ready-copy="text.readyCopy"
                :start-label="text.start"
                :spinning-title="text.spinning"
                :spinning-copy="text.spinningCopy"
                @refresh="handleCreditLotteryRefresh"
                @close="backToChoose"
              />

                <div v-else class="mini-draw" :class="stage">
                <button
                  class="mini-slot"
                  type="button"
                  :disabled="stage !== 'ready'"
                  :aria-label="stage === 'ready' ? text.flipCopy : blessingTitle"
                  @click="revealReward"
                >
                  <div class="mini-slot-track">
                    <div
                      v-for="(card, index) in visibleCards"
                      :key="`${card.title}-${index}`"
                      class="mini-slot-card"
                      :class="[card.rarity, { flipped: stage === 'revealed' }]"
                    >
                      <div class="card-face card-front">
                        <img
                          class="wheel-card-image"
                          :src="fortuneCardThumbnailImage(card, selectedType)"
                          :srcset="fortuneCardImageSrcSet(card, selectedType)"
                          :alt="card.title"
                          loading="lazy"
                          decoding="async"
                          fetchpriority="low"
                          sizes="(max-width: 640px) 42vw, 180px"
                          @error="handleImageFallback($event, fortuneCardImage(card, selectedType))"
                        />
                        <span class="wheel-card-title">{{ card.title }}</span>
                      </div>
                      <div class="card-face card-back">
                        <img
                          class="wheel-card-image"
                          :src="fortuneCardThumbnailImage(card, selectedType)"
                          :srcset="fortuneCardImageSrcSet(card, selectedType)"
                          :alt="card.title"
                          loading="lazy"
                          decoding="async"
                          fetchpriority="low"
                          sizes="(max-width: 640px) 42vw, 180px"
                          @error="handleImageFallback($event, fortuneCardImage(card, selectedType))"
                        />
                        <div class="wheel-card-heading">
                          <span class="rarity-badge">{{ rarityLabel(card.rarity) }}</span>
                          <b>{{ card.title }}</b>
                        </div>
                        <div class="wheel-card-footer">
                          <span class="wheel-card-line">{{ card.line }}</span>
                          <strong v-if="pendingResult">{{ compactRewardLabel(pendingResult) }}</strong>
                        </div>
                      </div>
                    </div>
                  </div>
                </button>

                <div class="draw-announcement" role="status" aria-live="polite" aria-atomic="true">
                  <div class="mini-blessing">{{ blessingTitle }}</div>
                  <div id="daily-fortune-draw-description" class="mini-copy">{{ blessingCopy }}</div>
                </div>
                <button v-if="stage === 'uncertain'" class="mini-start" type="button" :disabled="recoveryLoading" @click="reviewUncertainResult">
                  {{ recoveryLoading ? (canRetryCheckin ? flowText.retrying : flowText.refreshing) : (canRetryCheckin ? flowText.retry : flowText.review) }}
                </button>
                <button v-else class="mini-start" type="button" :disabled="stage === 'spinning'" @click="startDraw">
                  {{ stage === 'spinning' ? text.spinning : text.start }}
                </button>
                <div class="mini-pills">
                  <span v-for="pill in pills" :key="pill">{{ pill }}</span>
                </div>
              </div>
            </section>
          </div>
        </div>
      </div>
    </transition>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { useMediaQuery } from '@vueuse/core'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import userAPI from '@/api/user'
import { stableCardIndex, zeroCityCardDisplayName, zeroCityCardImagePath, zeroCityCardImageSrcSet } from '@/constants/zeroCityCardManifest'
import { zeroCityWheelImagePath, zeroCityWheelImageSrcSet, zeroCityWheelThumbnailPath } from '@/constants/zeroCityWheelImages'
import CreditLotteryThreeRounds from '@/features/bizdecipher/components/layout/CreditLotteryThreeRounds.vue'
import { usePointerDownOutside } from '@/composables/usePointerDownOutside'
import { useAppStore, useAuthStore } from '@/stores'
import { extractApiErrorMessage } from '@/utils/apiError'
import { isLocalPreviewAuth } from '@/utils/localPreview'
import {
  clearPendingCheckinOperation,
  isLotteryOperationNotFoundError,
  lotteryOperationRetryRemainingMs,
  readPendingCheckinOperation,
  storePendingCheckinOperation,
  type PendingCheckinOperation
} from '@/utils/lotteryOperationRecovery'
import type { CheckinClaimResponse, CheckinCollectibleCard, CheckinMilestoneStatus, CheckinRecord, CheckinStatus, CreditLotterySession, JackpotPayout } from '@/types'

const props = defineProps<{ autoOpen?: boolean }>()

type FortuneType = 'free' | 'credit' | 'balance'
type Rarity = 'common' | 'good' | 'rare' | 'epic' | 'diamond' | 'rainbow'
type ShopRarity = 'common' | 'good' | 'rare'
type LotteryUserId = number | string | null | undefined

interface FortuneCard {
  title: string
  line: string
  rarity: Rarity
  imageKey?: string
}

interface CardShopTier {
  rarity: ShopRarity
  price: number
  title: string
  badge: string
  priceLabel: string
}

interface FortuneText {
  gift: string
  title: string
  subtitle: string
  today: string
  unclaimed: string
  claimed: string
  streak: string
  freeShort: string
  creditShort: string
  balanceShort: string
  freeDesc: string
  creditDesc: string
  balanceDesc: string
  freeMeta: string
  creditMeta: string
  balanceMeta: string
  done: string
  poolTitle: string
  poolSub: string
  ready: string
  readyCopy: string
  chooseHint: string
  start: string
  spinning: string
  spinningCopy: string
  waitOpen: string
  remaining: string
  flip: string
  flipCopy: string
  jackpot: string
  jackpotCopy: string
  creditPool: string
  balancePool: string
  streakTitle: string
  cycleTitle: string
  dayUnit: string
  costFree: string
  costCredit: string
  costBalance: string
  prizesFree: string
  prizesCredit: string
  prizesBalance: string
  insufficient: string
  jackpotHit: string
  jackpotShare: string
  failed: string
  milestoneTitle: string
  milestoneClaim: string
  milestoneClaimed: string
  milestoneLocked: string
  milestoneActivate: string
  milestoneProgress: string
  milestoneNeedActivation: string
  recordShort: string
  recordTitle: string
  collectionShort: string
  collectionTitle: string
  emptyRecords: string
  emptyCards: string
  rareCardFound: string
  rarityRare: string
  rarityEpic: string
  rarityLegendary: string
  rarityMythic: string
}

const appStore = useAppStore()
const authStore = useAuthStore()
const router = useRouter()
const { locale } = useI18n()

const rootRef = ref<HTMLElement | null>(null)
const triggerRef = ref<HTMLButtonElement | null>(null)
const dialogRef = ref<HTMLElement | null>(null)
const chooseViewRef = ref<HTMLElement | null>(null)
const drawViewRef = ref<HTMLElement | null>(null)
const open = ref(false)
const view = ref<'choose' | 'draw'>('choose')
const stage = ref<'' | 'idle' | 'spinning' | 'ready' | 'revealed' | 'uncertain'>('')
const status = ref<CheckinStatus | null>(null)
const statusLoading = ref(false)
const statusLoadError = ref('')
const activeCreditLotterySession = ref<CreditLotterySession | null>(null)
const selectedType = ref<FortuneType>('free')
const loadingType = ref<FortuneType | null>(null)
const recoveryLoading = ref(false)
const pendingCheckinOperation = ref<PendingCheckinOperation | null>(null)
const checkinOperationNotFound = ref(false)
const checkinRetryReady = ref(false)
const checkinPostInFlight = ref(false)
const pendingResult = ref<CheckinClaimResponse | null>(null)
const pendingCard = ref<FortuneCard | null>(null)
const spinCards = ref<FortuneCard[]>([])
const blessingTitle = ref('')
const blessingCopy = ref('')
const pills = ref<string[]>([])
const claimingMilestone = ref<number | null>(null)
const buyingRarity = ref<ShopRarity | null>(null)
const activePanel = ref<'records' | 'cards' | 'shop' | null>(null)
const panelLoading = ref(false)
const panelLoadError = ref('')
const checkinRecords = ref<CheckinRecord[]>([])
const collectibleCards = ref<CheckinCollectibleCard[]>([])
const prefersReducedMotion = useMediaQuery('(prefers-reduced-motion: reduce)')
let previouslyFocusedElement: HTMLElement | null = null
let drawStepTrigger: HTMLElement | null = null
let statusLoadSequence = 0
let checkinRetryTimer: number | null = null
let checkinConfirmationTimer: number | null = null
let checkinAttemptSequence = 0
let checkinRecoverySequence = 0
let checkinComponentActive = true
let pendingCheckinOperationUserId: LotteryUserId = null
const focusableSelector = 'a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])'
const collectibleCardGroups = computed(() => {
  const groups = new Map<string, { card: CheckinCollectibleCard; count: number }>()
  for (const card of collectibleCards.value) {
    const current = groups.get(card.card_key)
    if (current) {
      current.count += 1
    } else {
      groups.set(card.card_key, { card, count: 1 })
    }
  }
  return [...groups.values()]
})

const user = computed(() => authStore.user)
const lang = computed<'zh' | 'en' | 'ru'>(() => {
  const htmlLang = typeof document !== 'undefined' ? document.documentElement.lang : ''
  const value = String(locale.value || htmlLang || '').toLowerCase()
  if (value.includes('ru')) return 'ru'
  if (value.includes('en')) return 'en'
  return 'zh'
})

const dictionaries: Record<'zh' | 'en' | 'ru', FortuneText> = {
  zh: {
    gift: '每日礼盒', title: '每日福运', subtitle: '选一种方式，进入今日签。', today: '今日状态', unclaimed: '未签到', claimed: '已签到', streak: '连续福运每日累积，今日可继续开启。', freeShort: '免费', creditShort: '积分', balanceShort: '余额', freeDesc: '每日一次，不空奖', creditDesc: '20 积分 / 最多 50 次', balanceDesc: '5 余额 / 最多 10 次', freeMeta: '基础福运', creditMeta: '大奖池更高', balanceMeta: '大奖池最高', done: '今日已完成', poolTitle: '实时大奖池', poolSub: '额度来自历史抽奖池', ready: '准备抽签', readyCopy: '六张福运卡已就位，点击开始后旋转。', chooseHint: '选择一种方式后，会转场进入卡片旋转。', start: '点击开始', spinning: '卡片旋转中', spinningCopy: '六张福运卡正在停下。', waitOpen: '等待开启', remaining: '剩余 {n} 次', flip: '点击翻面', flipCopy: '点击签牌，翻到背面查看今日祝福。', jackpot: '福运临门', jackpotCopy: '今日签意已开，极低概率大奖同时降临。', creditPool: '积分大奖池', balancePool: '余额大奖池', streakTitle: '累计天数', cycleTitle: '本轮进度', dayUnit: '天', costFree: '消耗：免费', costCredit: '消耗：20 积分', costBalance: '消耗：5 余额', prizesFree: '可开：5 / 7 / 10 / 15 积分', prizesCredit: '可开：5 / 10 / 20 / 50 / 100 积分', prizesBalance: '可开：1 / 2 / 5 / 10 / 20 余额', jackpotHit: '命中大奖池', jackpotShare: '全服撒花 {n} 人共享', insufficient: '余额或积分不足', failed: '签到失败，请稍后再试', milestoneTitle: '阶梯小礼盒', milestoneClaim: '领取', milestoneClaimed: '已领', milestoneLocked: '未达成', milestoneActivate: '激活后领', milestoneProgress: '已累计 {n} 天', milestoneNeedActivation: '7 天礼盒需邮箱验证和真实 API 调用', recordShort: '记录', recordTitle: '抽奖记录', collectionShort: '收藏', collectionTitle: '零点城小人卡', emptyRecords: '还没有抽奖记录', emptyCards: '还没有零点城小人卡', rareCardFound: '小人卡', rarityRare: '稀有', rarityEpic: '史诗', rarityLegendary: '传说', rarityMythic: '神话'
  },
  en: {
    gift: 'Daily Gift', title: 'Daily Fortune', subtitle: 'Choose a draw type.', today: 'Today', unclaimed: 'Not claimed', claimed: 'Claimed', streak: 'Keep your daily fortune streak alive.', freeShort: 'Free', creditShort: 'Points', balanceShort: 'Balance', freeDesc: 'Once daily, always wins', creditDesc: '20 pts / up to 50', balanceDesc: '5 balance / up to 10', freeMeta: 'Basic luck', creditMeta: 'Higher pool odds', balanceMeta: 'Highest pool odds', done: 'Done today', poolTitle: 'Live Jackpot', poolSub: 'Funded by historical draws', ready: 'Ready', readyCopy: 'Six fortune cards are ready. Tap start.', chooseHint: 'Choose a type to enter the card spin.', start: 'Start', spinning: 'Spinning', spinningCopy: 'Cards are slowing down.', waitOpen: 'Waiting', remaining: '{n} left', flip: 'Tap to flip', flipCopy: 'Tap the card to reveal today’s blessing.', jackpot: 'Fortune Arrives', jackpotCopy: 'The rare jackpot arrives with today’s blessing.', creditPool: 'Points pool', balancePool: 'Balance pool', streakTitle: 'Total days', cycleTitle: 'Cycle progress', dayUnit: 'days', costFree: 'Cost: free', costCredit: 'Cost: 20 pts', costBalance: 'Cost: 5 balance', prizesFree: 'Can open: 5 / 7 / 10 / 15 pts', prizesCredit: 'Can open: 5 / 10 / 20 / 50 / 100 pts', prizesBalance: 'Can open: 1 / 2 / 5 / 10 / 20 balance', jackpotHit: 'Jackpot hit', jackpotShare: '{n} active users shared celebration', insufficient: 'Not enough balance or points', failed: 'Check-in failed. Please try again.', milestoneTitle: 'Milestone gifts', milestoneClaim: 'Claim', milestoneClaimed: 'Claimed', milestoneLocked: 'Locked', milestoneActivate: 'Activate', milestoneProgress: '{n} days total', milestoneNeedActivation: '7-day gift needs verified email and real API usage', recordShort: 'Logs', recordTitle: 'Draw records', collectionShort: 'Cards', collectionTitle: 'Rare card collection', emptyRecords: 'No draw records yet', emptyCards: 'No rare cards yet', rareCardFound: 'Rare card', rarityRare: 'Rare', rarityEpic: 'Epic', rarityLegendary: 'Legend', rarityMythic: 'Mythic'
  },
  ru: {
    gift: 'Ежедневный подарок', title: 'Удача дня', subtitle: 'Выберите способ розыгрыша.', today: 'Сегодня', unclaimed: 'Не получено', claimed: 'Получено', streak: 'Продолжайте ежедневную серию удачи.', freeShort: 'Бесплатно', creditShort: 'Баллы', balanceShort: 'Баланс', freeDesc: 'Раз в день, без пустого приза', creditDesc: '20 баллов / до 50 раз', balanceDesc: '5 баланса / до 10 раз', freeMeta: 'Базовая удача', creditMeta: 'Выше шанс пула', balanceMeta: 'Макс. шанс пула', done: 'Сегодня готово', poolTitle: 'Живой джекпот', poolSub: 'Из прошлых розыгрышей', ready: 'Готово', readyCopy: 'Шесть карт готовы. Нажмите старт.', chooseHint: 'Выберите тип, чтобы запустить карты.', start: 'Старт', spinning: 'Крутится', spinningCopy: 'Карты замедляются.', waitOpen: 'Ожидание', remaining: 'Осталось {n}', flip: 'Открыть', flipCopy: 'Нажмите карту, чтобы увидеть благословение.', jackpot: 'Удача пришла', jackpotCopy: 'Редкий джекпот пришёл вместе с удачей.', creditPool: 'Пул баллов', balancePool: 'Пул баланса', streakTitle: 'Всего дней', cycleTitle: 'Прогресс цикла', dayUnit: 'дн.', costFree: 'Цена: бесплатно', costCredit: 'Цена: 20 баллов', costBalance: 'Цена: 5 баланса', prizesFree: 'Можно открыть: 5 / 7 / 10 / 15 баллов', prizesCredit: 'Можно открыть: 5 / 10 / 20 / 50 / 100 баллов', prizesBalance: 'Можно открыть: 1 / 2 / 5 / 10 / 20 баланса', jackpotHit: 'Джекпот', jackpotShare: 'Праздник разделили {n} пользователей', insufficient: 'Недостаточно баланса или баллов', failed: 'Не удалось открыть. Попробуйте позже.', milestoneTitle: 'Подарки этапов', milestoneClaim: 'Получить', milestoneClaimed: 'Получено', milestoneLocked: 'Закрыто', milestoneActivate: 'Активируйте', milestoneProgress: 'Всего {n} дн.', milestoneNeedActivation: 'Подарок за 7 дней требует проверку email и реальное API-использование', recordShort: 'Логи', recordTitle: 'История розыгрышей', collectionShort: 'Карты', collectionTitle: 'Редкие карты', emptyRecords: 'Истории пока нет', emptyCards: 'Редких карт пока нет', rareCardFound: 'Редкая карта', rarityRare: 'Редк.', rarityEpic: 'Эпик', rarityLegendary: 'Легенда', rarityMythic: 'Миф'
  }
}

const rarityDictionary: Record<'zh' | 'en' | 'ru', Record<Rarity, string>> = {
  zh: { common: '普通', good: '进阶', rare: '稀有', epic: '史诗', diamond: '钻石', rainbow: '彩虹' },
  en: { common: 'Common', good: 'Good', rare: 'Rare', epic: 'Epic', diamond: 'Diamond', rainbow: 'Rainbow' },
  ru: { common: 'Обычн.', good: 'Улучш.', rare: 'Редк.', epic: 'Эпик', diamond: 'Алмаз', rainbow: 'Радуга' }
}

const pools: Record<'zh' | 'en' | 'ru', { rewards: string[], deck: FortuneCard[], blessings: Record<FortuneType, FortuneCard[]> }> = {
  zh: {
    rewards: ['积分小福|常见', '积分进阶|常见', '余额小喜|少见', '钻石卡|积分奖池', '彩虹卡|余额奖池'],
    deck: [],
    blessings: {
      free: [
        { title: '低电量小人', line: '今天电量不满，也允许缓慢发光。', rarity: 'common' },
        { title: '早起失败者', line: '我没早起成功，但我成功出现了。', rarity: 'common' },
        { title: '情绪保洁员', line: '坏心情先放门口，我下班前试着扫掉。', rarity: 'common' },
        { title: '躺平观察员', line: '我躺着不是放弃，是换个角度观察命运。', rarity: 'common' },
        { title: '轻微掉线者', line: '我只是掉线了一小会儿，回来时带了点光。', rarity: 'common' },
        { title: '周一逃兵', line: '我没有逃跑，我在给勇气重新排队。', rarity: 'common' },
        { title: '云端修补匠', line: '天空破了个洞，我先拿一点温柔补上。', rarity: 'good' },
        { title: '热水守护者', line: '先喝一口热的，世界会稍微讲点道理。', rarity: 'good' },
        { title: '缓慢启动机', line: '启动慢不代表坏了，我只是比较庄重。', rarity: 'good' },
        { title: '小步冠军', line: '今天只走一步，也算对命运发动了攻击。', rarity: 'good' },
        { title: '自救便利店', line: '希望暂时缺货，但勇气还有临期特价。', rarity: 'good' },
        { title: '反向锦鲤', line: '我越说算了，好事越想证明一下自己。', rarity: 'rare' },
        { title: '焦虑园丁', line: '我把不安种下去，明天也许会长出答案。', rarity: 'rare' },
        { title: '好运试吃员', line: '大的还没上桌，小的我先替你尝一口。', rarity: 'rare' },
        { title: '失败回收站', line: '坏掉的今天不会浪费，明天还能再编译。', rarity: 'rare' },
        { title: '小概率乐观派', line: '虽然概率不高，但我已经把座位留给奇迹。', rarity: 'epic' },
        { title: '黑夜开灯员', line: '我在最暗的地方上班，负责给你留一盏灯。', rarity: 'epic' },
        { title: '命运客服', line: '你的好运工单已提交，正在转人工奇迹。', rarity: 'epic' },
        { title: '低调欧皇', line: '我真的没开挂，只是好运比较没有边界感。', rarity: 'diamond' },
        { title: '命运插队员', line: '不好意思，今天的好运说认识我。', rarity: 'rainbow' }
      ],
      credit: [
        { title: '积分搬砖人', line: '小数字也有尊严，堆起来就是我的高楼。', rarity: 'common' },
        { title: '余额旁听生', line: '我还没发财，但已经开始认真听课。', rarity: 'common' },
        { title: '概率实习生', line: '我不懂玄学，只会在角落偷偷加权。', rarity: 'common' },
        { title: '手气修理工', line: '手气坏了别扔，我先拆开看看是不是松了。', rarity: 'common' },
        { title: '小奖收纳师', line: '大奖还在路上，小奖先别乱跑。', rarity: 'common' },
        { title: '欧气旁观者', line: '我站在旁边看着看着，突然被好运点名。', rarity: 'common' },
        { title: '积分炼金师', line: '小数字堆在一起，也会炼出一点明亮。', rarity: 'good' },
        { title: '前摇研究员', line: '别急，钱包正在进行一个很长的前摇。', rarity: 'good' },
        { title: '期待管理员', line: '期待不能太满，但可以先开个小窗口。', rarity: 'good' },
        { title: '玄学合规员', line: '我不迷信，我只是尊重概率的情绪价值。', rarity: 'good' },
        { title: '数字拾荒者', line: '别人看不上零碎，我看见一地未来。', rarity: 'good' },
        { title: '概率叛逃者', line: '概率说不行，我说我只是路过一下规则。', rarity: 'rare' },
        { title: '反悔锦鲤', line: '我刚说不抽了，命运就开始装作没听见。', rarity: 'rare' },
        { title: '好运缓存员', line: '今天没爆发没关系，我先把欧气缓存起来。', rarity: 'rare' },
        { title: '小赚哲学家', line: '赚一点也是赚，宇宙没有规定快乐起步价。', rarity: 'rare' },
        { title: '冷静暴富学家', line: '先别激动，财富正在进行系统更新。', rarity: 'epic' },
        { title: '命中率驯兽师', line: '概率有点野，但我带了零食和耐心。', rarity: 'epic' },
        { title: '奇迹测试员', line: '本次奇迹可能不稳定，但值得灰度发布。', rarity: 'epic' },
        { title: '隐藏欧皇', line: '我没有炫耀，我只是被好运误伤得很自然。', rarity: 'diamond' },
        { title: '奖池破壁人', line: '门没开，我就先和门聊成了熟人。', rarity: 'rainbow' }
      ],
      balance: [
        { title: '余额守门员', line: '门票已验，今天允许一点小惊喜进场。', rarity: 'common' },
        { title: '现金流学徒', line: '我还不富有，但我已经学会和数字握手。', rarity: 'common' },
        { title: '钱包观察员', line: '钱包很安静，通常这是剧情开始前的安静。', rarity: 'common' },
        { title: '风险小猫', line: '我知道有风险，但爪子已经伸出去了。', rarity: 'common' },
        { title: '开奖围观者', line: '我只是看看，怎么命运还给我递了椅子。', rarity: 'common' },
        { title: '理性放风人', line: '理性今天也在，但它允许我出去透口气。', rarity: 'common' },
        { title: '现金流诗人', line: '数字很现实，但风也会把现实吹软。', rarity: 'good' },
        { title: '惊喜验票员', line: '小惊喜请出示编号，我好把它放进今天。', rarity: 'good' },
        { title: '好运保安', line: '可疑的好运正在靠近，我决定不拦。', rarity: 'good' },
        { title: '余额园丁', line: '我给小数点浇水，等它长成大一点的期待。', rarity: 'good' },
        { title: '刺激降噪师', line: '心跳可以快一点，但别吵到希望工作。', rarity: 'good' },
        { title: '大奖池观察员', line: '我假装路过，其实已经盯它很久了。', rarity: 'rare' },
        { title: '回本幻想家', line: '幻想不是计划，但偶尔能给计划续命。', rarity: 'rare' },
        { title: '小赚逃逸者', line: '我本来想冷静，可收益先从窗户探头。', rarity: 'rare' },
        { title: '紧张收藏家', line: '我把心跳收好，万一等下用得上。', rarity: 'rare' },
        { title: '奖池潜水员', line: '我潜下去不是冲动，是想看看光在哪。', rarity: 'epic' },
        { title: '命运出纳员', line: '今天的好运已入账，备注写着别声张。', rarity: 'epic' },
        { title: '高光借阅员', line: '我向明天借一点高光，今天先用。', rarity: 'epic' },
        { title: '闪卡预备役', line: '我还没发光，但边框已经开始不讲道理。', rarity: 'diamond' },
        { title: '彩虹账本持有人', line: '这不是余额，这是命运给我开的隐藏支线。', rarity: 'rainbow' }
      ]
    }
  },
  en: {
    rewards: ['Point Luck|Common', 'Point Plus|Common', 'Balance Joy|Uncommon', 'Diamond Card|Point pool', 'Rainbow Card|Balance pool'],
    deck: [
      { title: 'Clear Day', line: 'A small piece of luck lands gently today.', rarity: 'common' }, { title: 'Tailwind', line: 'Steady steps will meet better returns.', rarity: 'good' }, { title: 'Starlit', line: 'A quiet surprise is coming closer.', rarity: 'rare' }, { title: 'Long Wish', line: 'Today is worth a brighter turn.', rarity: 'epic' }, { title: 'Pure Glow', line: 'Rare fortune is near. Receive it with care.', rarity: 'diamond' }, { title: 'Rainbow Near', line: 'The jackpot light falls beyond the usual.', rarity: 'rainbow' }
    ],
    blessings: {
      free: [{ title: 'Moon Clears', line: 'After the mist, what you care about will answer slowly.', rarity: 'common' }, { title: 'Spring Note', line: 'No rush today. Take good care of yourself first.', rarity: 'good' }, { title: 'Soft Light', line: 'A tiny certainty will steady your day.', rarity: 'rare' }, { title: 'Warm Trace', line: 'A good small deed will echo back soon.', rarity: 'epic' }],
      credit: [{ title: 'Stars Near', line: 'May your thoughts echo and your path glow.', rarity: 'good' }, { title: 'One Spark', line: 'With one thought, the answer appears around the corner.', rarity: 'rare' }, { title: 'Pearl Shows', line: 'Effort treated with care will quietly shine today.', rarity: 'epic' }, { title: 'Clear Wind', line: 'A light idea will loosen what once felt stuck.', rarity: 'diamond' }],
      balance: [{ title: 'Auspice', line: 'A new chance is approaching. Receive it calmly.', rarity: 'rare' }, { title: 'Fortune Star', line: 'Luck is quiet, but it moves you toward ease.', rarity: 'epic' }, { title: 'Long Wind', line: 'Your wish already has wind behind it.', rarity: 'diamond' }, { title: 'Still Gold', line: 'Stay steady. Good things are gathering for you.', rarity: 'rainbow' }]
    }
  },
  ru: {
    rewards: ['Малые баллы|Часто', 'Больше баллов|Часто', 'Баланс|Редко', 'Алмазная карта|Пул баллов', 'Радужная карта|Пул баланса'],
    deck: [
      { title: 'Ясный день', line: 'Мягкая удача первой приходит сегодня.', rarity: 'common' }, { title: 'Попутный ветер', line: 'Ровный путь приведёт к лучшей награде.', rarity: 'good' }, { title: 'Звёздный свет', line: 'Небольшой сюрприз уже близко.', rarity: 'rare' }, { title: 'Долгое желание', line: 'Сегодня возможен яркий поворот.', rarity: 'epic' }, { title: 'Чистое сияние', line: 'Редкая удача рядом. Примите её спокойно.', rarity: 'diamond' }, { title: 'Радуга рядом', line: 'Свет джекпота выходит за пределы обычного.', rarity: 'rainbow' }
    ],
    blessings: {
      free: [{ title: 'Луна ясна', line: 'Когда туман уйдёт, важное даст тихий ответ.', rarity: 'common' }, { title: 'Весна', line: 'Не спешите сегодня, сначала позаботьтесь о себе.', rarity: 'good' }, { title: 'Свет', line: 'Маленькая уверенность поддержит этот день.', rarity: 'rare' }, { title: 'След', line: 'Доброе малое дело скоро вернётся эхом.', rarity: 'epic' }],
      credit: [{ title: 'Звёзды', line: 'Пусть мысли имеют отклик, а путь — свет.', rarity: 'good' }, { title: 'Искра', line: 'Одна мысль — и ответ уже за поворотом.', rarity: 'rare' }, { title: 'Жемчуг', line: 'Труд, к которому отнеслись серьёзно, засияет.', rarity: 'epic' }, { title: 'Ветер', line: 'Лёгкая мысль развяжет старый узел.', rarity: 'diamond' }],
      balance: [{ title: 'Знак', line: 'Новая возможность близко. Примите её спокойно.', rarity: 'rare' }, { title: 'Звезда', line: 'Удача тиха, но ведёт вас к более лёгкому пути.', rarity: 'epic' }, { title: 'Ветер желаний', line: 'У вашего желания уже появился попутный ветер.', rarity: 'diamond' }, { title: 'Тихое золото', line: 'Сохраняйте спокойствие: хорошее собирается рядом.', rarity: 'rainbow' }]
    }
  }
}


const ZERO_CITY_CARD_BASE = '/assets/zero-point-city/cards'
const wheelImageKeys: Record<FortuneType, Record<Rarity, string[]>> = {
  free: {
    common: ['wheel_free_01', 'wheel_free_02', 'wheel_free_03', 'wheel_free_04', 'wheel_free_05', 'wheel_free_06'],
    good: ['wheel_free_07', 'wheel_free_08', 'wheel_free_09', 'wheel_free_10', 'wheel_free_11'],
    rare: ['wheel_free_12', 'wheel_free_13', 'wheel_free_14', 'wheel_free_15'],
    epic: ['wheel_free_16', 'wheel_free_17', 'wheel_free_18'],
    diamond: ['wheel_free_19'],
    rainbow: ['wheel_free_20']
  },
  credit: {
    common: ['wheel_credit_01', 'wheel_credit_02', 'wheel_credit_03', 'wheel_credit_04', 'wheel_credit_05', 'wheel_credit_06'],
    good: ['wheel_credit_07', 'wheel_credit_08', 'wheel_credit_09', 'wheel_credit_10', 'wheel_credit_11'],
    rare: ['wheel_credit_12', 'wheel_credit_13', 'wheel_credit_14', 'wheel_credit_15'],
    epic: ['wheel_credit_16', 'wheel_credit_17', 'wheel_credit_18'],
    diamond: ['wheel_credit_19'],
    rainbow: ['wheel_credit_20']
  },
  balance: {
    common: ['wheel_balance_01', 'wheel_balance_02', 'wheel_balance_03', 'wheel_balance_04', 'wheel_balance_05', 'wheel_balance_06'],
    good: ['wheel_balance_07', 'wheel_balance_08', 'wheel_balance_09', 'wheel_balance_10', 'wheel_balance_11'],
    rare: ['wheel_balance_12', 'wheel_balance_13', 'wheel_balance_14', 'wheel_balance_15'],
    epic: ['wheel_balance_16', 'wheel_balance_17', 'wheel_balance_18'],
    diamond: ['wheel_balance_19'],
    rainbow: ['wheel_balance_20']
  }
}

const text = computed(() => dictionaries[lang.value])
const currentPool = computed(() => pools[lang.value])
const rarityLabels = computed(() => rarityDictionary[lang.value])
const creditLimit = computed(() => status.value?.credit_limit ?? 50)
const balanceLimit = computed(() => status.value?.balance_limit ?? 10)
const selectedOption = computed(() => options.value.find(option => option.type === selectedType.value))
const milestones = computed(() => status.value?.milestones ?? [])
const milestonePolicy = computed(() => lang.value === 'zh'
  ? '签到里程碑仅奖励积分，不再赠送余额；历史余额保留。'
  : lang.value === 'ru'
    ? 'Этапы дают только баллы. Ранее начисленный баланс сохраняется.'
    : 'Milestones award points only. Previously credited balance is kept.')
const canRestoreCreditLotterySession = computed(() => activeCreditLotterySession.value !== null)
const canRetryCheckin = computed(() => checkinOperationNotFound.value && checkinRetryReady.value && !checkinPostInFlight.value)
const cycleProgressText = computed(() => {
  const cycleNo = status.value?.cycle_no ?? 0
  const cycleDay = status.value?.cycle_day ?? status.value?.streak_days ?? 0
  return cycleNo > 0 ? `第${cycleNo}轮 ${cycleDay}/7` : `${cycleDay}/7`
})
const shopText = computed(() => {
  if (lang.value === 'en') return { title: 'Card Shop', balance: 'Point balance' }
  if (lang.value === 'ru') return { title: 'Магазин карт', balance: 'Баланс баллов' }
  return { title: '卡片商店', balance: '积分余额' }
})
const flowText = computed(() => {
  if (lang.value === 'en') return { revealed: 'Fortune revealed', uncertain: 'Confirming the result', uncertainCopy: 'This request may have been accepted. Only the original result will be checked; no new operation ID will be created.', postingCopy: 'The server has not registered it yet, but the original request is still pending. Only lookup is available until it returns.', waitingCopy: 'The server has not registered this operation yet. Waiting for the safety window before an exact-ID retry.', retryCopy: 'The server still has no record of this operation. Retry safely with the same operation ID; it can only settle once.', review: 'Check original result', refreshing: 'Checking', retry: 'Retry original operation', retrying: 'Retrying safely' }
  if (lang.value === 'ru') return { revealed: 'Карта открыта', uncertain: 'Проверяем результат', uncertainCopy: 'Запрос мог быть принят. Проверяется только исходный результат; новый номер операции не создаётся.', postingCopy: 'Операция пока не найдена, но исходный запрос ещё ожидает ответа. До его завершения доступна только проверка.', waitingCopy: 'Сервер ещё не зарегистрировал операцию. Ждём безопасное окно перед повтором с тем же номером.', retryCopy: 'Операция не найдена. Можно безопасно повторить её с тем же номером; расчёт выполнится только один раз.', review: 'Проверить результат', refreshing: 'Проверяем', retry: 'Повторить исходную операцию', retrying: 'Безопасный повтор' }
  return { revealed: '今日签已揭晓', uncertain: '结果确认中', uncertainCopy: '请求可能已经受理，只查询本次原操作号，不会生成第二笔抽奖或重复扣款。', postingCopy: '服务器暂未登记本次操作，但原请求仍在等待返回；当前只允许查询，不会并发提交第二次。', waitingCopy: '服务器暂未登记本次操作，正在等待安全窗口；期间不会重复提交或扣款。', retryCopy: '服务器仍未登记本次操作，可用原操作号安全重试；无论原请求是否晚到都只会结算一次。', review: '查询原结果', refreshing: '查询中', retry: '重试原操作', retrying: '安全重试中' }
})
const cardShopTiers = computed<CardShopTier[]>(() => [
  { rarity: 'common', price: 260, title: rarityLabels.value.common, badge: 'C', priceLabel: `260 ${text.value.creditShort}` },
  { rarity: 'good', price: 780, title: rarityLabels.value.good, badge: 'G', priceLabel: `780 ${text.value.creditShort}` },
  { rarity: 'rare', price: 2400, title: text.value.rarityRare, badge: 'R', priceLabel: `2400 ${text.value.creditShort}` }
])
const visibleCards = computed(() => {
  if ((stage.value === 'ready' || stage.value === 'revealed') && pendingCard.value) return [pendingCard.value]
  if (spinCards.value.length) return spinCards.value
  return buildSpinDeck(selectedType.value)
})
const options = computed(() => {
  const creditCount = status.value?.credit_count ?? 0
  const balanceCount = status.value?.balance_count ?? 0
  const creditCost = status.value?.credit_cost ?? 20
  const balanceCost = status.value?.balance_cost ?? 5
  const pointBalance = status.value?.credit_balance ?? user.value?.credit_balance ?? 0
  const cashBalance = status.value?.balance ?? user.value?.balance ?? 0
  const statusUnconfirmed = statusLoading.value || Boolean(statusLoadError.value) || !status.value
  const creditDisabled = !canRestoreCreditLotterySession.value && (statusUnconfirmed || creditCount >= creditLimit.value || pointBalance < creditCost)
  return [
    { type: 'free' as const, icon: '☁', title: text.value.freeShort, desc: text.value.freeDesc, costLabel: text.value.costFree, prizes: text.value.prizesFree, meta: status.value?.free_claimed ? text.value.done : text.value.freeMeta, used: Boolean(status.value?.free_claimed), disabled: statusUnconfirmed || Boolean(status.value?.free_claimed) },
    { type: 'credit' as const, icon: '✦', title: text.value.creditShort, desc: text.value.creditDesc, costLabel: text.value.costCredit, prizes: text.value.prizesCredit, meta: creditCount >= creditLimit.value ? text.value.done : format(text.value.remaining, { n: remaining('credit') }), used: creditCount > 0 || canRestoreCreditLotterySession.value, disabled: creditDisabled },
    { type: 'balance' as const, icon: '◇', title: text.value.balanceShort, desc: text.value.balanceDesc, costLabel: text.value.costBalance, prizes: text.value.prizesBalance, meta: balanceCount >= balanceLimit.value ? text.value.done : format(text.value.remaining, { n: remaining('balance') }), used: balanceCount > 0, disabled: statusUnconfirmed || balanceCount >= balanceLimit.value || cashBalance < balanceCost }
  ]
})

function format(template: string, vars: Record<string, string | number>) {
  return Object.entries(vars).reduce((result, [key, value]) => result.replace(`{${key}}`, String(value)), template)
}

function remaining(type: FortuneType) {
  if (type === 'free') return status.value?.free_claimed ? 0 : 1
  if (type === 'credit') return Math.max(creditLimit.value - (status.value?.credit_count ?? 0), 0)
  return Math.max(balanceLimit.value - (status.value?.balance_count ?? 0), 0)
}

function rarityLabel(rarity: Rarity) {
  return rarityLabels.value[rarity]
}

function shuffleCards(items: FortuneCard[]): FortuneCard[] {
  return [...items].sort(() => Math.random() - 0.5)
}

function wheelCardKey(type: FortuneType, card: FortuneCard, index?: number) {
  if (card.imageKey) return card.imageKey
  const rarityKeys = wheelImageKeys[type][card.rarity]
  if (!rarityKeys.length) return wheelImageKeys[type].common[0]
  const pool = fortuneCards(type).filter(item => item.rarity === card.rarity)
  const sourceIndex = index ?? pool.findIndex(item => item.title === card.title && item.line === card.line)
  const safeIndex = sourceIndex >= 0 ? sourceIndex : 0
  return rarityKeys[safeIndex % rarityKeys.length]
}

function wheelCardPath(type: FortuneType, key: string, rarity: Rarity) {
  return zeroCityWheelImagePath(type, rarity, key)
}

function wheelCardThumbnailPath(type: FortuneType, key: string, rarity: Rarity) {
  return zeroCityWheelThumbnailPath(type, rarity, key, 320)
}

function handleImageFallback(event: Event, fallbackSrc: string) {
  const target = event.target as HTMLImageElement | null
  if (!target || !fallbackSrc) return
  if (target.dataset.fallbackApplied === '1') return
  target.dataset.fallbackApplied = '1'
  target.onerror = null
  target.removeAttribute('srcset')
  target.src = fallbackSrc
}

function fortuneCardImage(card: FortuneCard, type: FortuneType) {
  return wheelCardPath(type, wheelCardKey(type, card), card.rarity)
}

function fortuneCardThumbnailImage(card: FortuneCard, type: FortuneType) {
  return wheelCardThumbnailPath(type, wheelCardKey(type, card), card.rarity)
}

function fortuneCardImageSrcSet(card: FortuneCard, type: FortuneType) {
  return zeroCityWheelImageSrcSet(type, card.rarity, wheelCardKey(type, card))
}

function collectibleCardImage(card: CheckinCollectibleCard) {
  return zeroCityCardImagePath(card)
}

function collectibleCardImageSrcSet(card: CheckinCollectibleCard) {
  return zeroCityCardImageSrcSet(card)
}

function fortuneCards(type: FortuneType) {
  const pool = currentPool.value.blessings[type]
  const fallback = Object.values(currentPool.value.blessings).flat()
  return pool.length ? pool : fallback
}

function buildSpinDeck(type: FortuneType) {
  const typeCards = fortuneCards(type).map((card, index) => ({ ...card, imageKey: wheelCardKey(type, card, index) }))
  const baseCards = [...typeCards, ...currentPool.value.deck]
  const source = baseCards.length ? baseCards : typeCards
  const deck: FortuneCard[] = []
  while (deck.length < 8 && source.length) {
    deck.push(...shuffleCards(source).slice(0, Math.min(source.length, 8)))
  }
  return deck.slice(0, 8)
}

function rewardRarity(type: FortuneType, reward: number): Rarity {
  if (type === 'balance') {
    if (reward >= 20) return 'diamond'
    if (reward >= 10) return 'epic'
    if (reward >= 5) return 'rare'
    if (reward >= 2) return 'good'
    return 'common'
  }
  if (type === 'credit') {
    if (reward >= 100) return 'diamond'
    if (reward >= 50) return 'epic'
    if (reward >= 20) return 'rare'
    if (reward >= 10) return 'good'
    return 'common'
  }
  if (reward >= 15) return 'epic'
  if (reward >= 10) return 'rare'
  if (reward >= 7) return 'good'
  return 'common'
}

function pickBlessing(type: FortuneType, rarity: Rarity, result: CheckinClaimResponse) {
  const pool = fortuneCards(type).map((card, index) => ({ ...card, imageKey: wheelCardKey(type, card, index) }))
  const candidates = pool.filter(card => card.rarity === rarity)
  const softerCandidates = pool.filter(card => card.rarity === 'good' || card.rarity === 'common')
  const selection = candidates.length ? candidates : softerCandidates.length ? softerCandidates : pool
  const seed = [result.date, result.type, result.reward, result.balance_after, result.credit_balance_after, result.collectible_card?.id || 0].join(':')
  return selection[stableCardIndex(seed, selection.length)]
}

function milestoneRewardLabel(item: CheckinMilestoneStatus) {
  const parts: string[] = []
  if (item.credit_reward > 0) parts.push(`${text.value.creditShort} +${Number(item.credit_reward).toFixed(0)}`)
  if (item.balance_reward > 0) parts.push(`${text.value.balanceShort} +${Number(item.balance_reward).toFixed(0)}`)
  return parts.join(' · ')
}

function milestoneClaimable(item: CheckinMilestoneStatus) {
  if (item.claimed) return false
  if (item.activation_required) return false
  if (typeof item.claimable === 'boolean') return item.claimable
  return Boolean(item.eligible && !item.claimed && !item.activation_required)
}

function milestoneStatusLabel(item: CheckinMilestoneStatus) {
  if (item.claimed) return text.value.milestoneClaimed
  if (item.activation_required) return text.value.milestoneActivate
  if (milestoneClaimable(item)) return text.value.milestoneClaim
  return text.value.milestoneLocked
}

async function claimMilestone(days: number) {
  if (claimingMilestone.value) return
  const operationUserId = user.value?.id
  const item = milestones.value.find(entry => entry.days === days)
  if (!item || !milestoneClaimable(item)) return
  claimingMilestone.value = days
  try {
    const result = await userAPI.claimCheckinMilestone(days)
    if (!isCheckinUserContextCurrent(operationUserId)) return
    authStore.patchUserBalance(result.balance_after, result.credit_balance_after)
    await loadStatus()
    if (!isCheckinUserContextCurrent(operationUserId)) return
    appStore.showSuccess(milestoneRewardLabel({ ...item, credit_reward: result.credit_reward, balance_reward: result.balance_reward }))
  } catch (error) {
    if (!isCheckinUserContextCurrent(operationUserId)) return
    const message = extractApiErrorMessage(error, text.value.failed)
    // already claimed / not eligible: refresh so UI stops showing a green claimable state
    await loadStatus().catch(() => undefined)
    if (/already been claimed|已领取|ALREADY_CLAIMED/i.test(message)) {
      appStore.showError(lang.value === 'en' ? 'This milestone was already claimed' : '该里程碑已领取')
    } else {
      appStore.showError(message)
    }
  } finally {
    claimingMilestone.value = null
  }
}


function milestoneMiniReward(item: CheckinMilestoneStatus) {
  const parts: string[] = []
  if (item.credit_reward > 0) parts.push(`+${Number(item.credit_reward).toFixed(0)} ${text.value.creditShort}`)
  if (item.balance_reward > 0) parts.push(`+${Number(item.balance_reward).toFixed(0)} ${text.value.balanceShort}`)
  return parts.join('/')
}

function collectibleEditionLabel(card: CheckinCollectibleCard) {
  const editionNo = card.edition_no || card.serial_no || 0
  const editionSupply = card.edition_supply || 0
  if (editionNo > 0 && editionSupply > 0) {
    const width = String(editionSupply).length
    return `#${String(editionNo).padStart(width, '0')}/${editionSupply}`
  }
  return `#${String(editionNo).padStart(4, '0')}`
}

function collectibleCardName(card: CheckinCollectibleCard) {
  return zeroCityCardDisplayName(card.card_key)
}

function collectibleRarityShort(rarity: string) {
  if (rarity === 'mythic') return text.value.rarityMythic
  if (rarity === 'legendary') return text.value.rarityLegendary
  if (rarity === 'epic') return text.value.rarityEpic
  if (rarity === 'rare') return text.value.rarityRare
  if (rarity === 'good') return rarityLabels.value.good
  return rarityLabels.value.common
}

function shopTierDisabled(tier: CardShopTier) {
  const pointBalance = status.value?.credit_balance ?? user.value?.credit_balance ?? 0
  return pointBalance < tier.price
}

async function buyShopTier(tier: CardShopTier) {
  if (buyingRarity.value || shopTierDisabled(tier)) return
  const operationUserId = user.value?.id
  buyingRarity.value = tier.rarity
  try {
    const result = await userAPI.buyCheckinCard(tier.rarity)
    if (!isCheckinUserContextCurrent(operationUserId)) return
    authStore.patchUserBalance(result.balance_after, result.credit_balance_after)
    await loadStatus()
    if (!isCheckinUserContextCurrent(operationUserId)) return
    collectibleCards.value = await userAPI.getCheckinCards()
    if (!isCheckinUserContextCurrent(operationUserId)) return
    appStore.showSuccess(`${shopText.value.title} · ${collectibleCardName(result.collectible_card)} ${collectibleEditionLabel(result.collectible_card)}`)
  } catch (error) {
    if (!isCheckinUserContextCurrent(operationUserId)) return
    appStore.showError(extractApiErrorMessage(error, text.value.insufficient))
  } finally {
    buyingRarity.value = null
  }
}

function recordTypeLabel(type: string) {
  if (type === 'balance') return text.value.balanceShort
  if (type === 'credit' || type === 'paid') return text.value.creditShort
  return text.value.freeShort
}

function jackpotAssetLabel(asset: string) {
  return asset === 'balance' ? text.value.balanceShort : text.value.creditShort
}

function jackpotPayoutLabels(payouts: JackpotPayout[] | undefined) {
  return (payouts || [])
    .filter(payout => Number(payout.winner_amount || 0) > 0)
    .map(payout => `${jackpotAssetLabel(payout.reward_asset)} +${Number(payout.winner_amount || 0).toFixed(2)}`)
}

function recordRewardLabel(record: CheckinRecord) {
  const unit = record.reward_asset === 'balance' ? text.value.balanceShort : text.value.creditShort
  const reward = `${unit} +${Number(record.reward || 0).toFixed(record.reward_asset === 'balance' ? 2 : 0)}`
  if (!record.jackpot_hit) return reward
  const payoutLabels = jackpotPayoutLabels(record.jackpot_payouts)
  if (payoutLabels.length > 0) return `${reward} · ${payoutLabels.join(' / ')}`
  if (!record.jackpot_winner_amount) return reward
  const jackpotUnit = record.jackpot_pool === 'balance' ? text.value.balanceShort : text.value.creditShort
  return `${reward} · ${jackpotUnit} +${Number(record.jackpot_winner_amount).toFixed(2)}`
}

function recordDateLabel(value: string) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString(lang.value === 'zh' ? 'zh-CN' : lang.value === 'ru' ? 'ru-RU' : 'en-US', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })
}

async function openActivityPanel(panel: 'records' | 'cards' | 'shop') {
  activePanel.value = panel
  panelLoading.value = true
  panelLoadError.value = ''
  try {
    if (panel === 'records') {
      checkinRecords.value = await userAPI.getCheckinRecords()
    } else if (panel === 'cards') {
      collectibleCards.value = await userAPI.getCheckinCards()
    } else if (panel === 'shop') {
      await loadStatus()
    }
  } catch (error) {
    panelLoadError.value = extractApiErrorMessage(error, '内容加载失败，请重新加载。')
    appStore.showError(panelLoadError.value)
  } finally {
    panelLoading.value = false
  }
}

function returnToChooseHome() {
  view.value = 'choose'
  activePanel.value = null
}

function openFullAlbum(card?: CheckinCollectibleCard) {
  closeGift()
  const query = card?.serial_no ? { serial: String(card.serial_no) } : undefined
  void router.push({ path: '/zero-city/cards', query })
}

function rewardLabel(result: CheckinClaimResponse) {
  const asset = result.reward_asset === 'balance' ? text.value.balanceShort : text.value.creditShort
  const base = `${asset} +${result.reward.toFixed(2)}`
  const parts = [base]
  if (result.jackpot_hit) {
    const payoutLabels = jackpotPayoutLabels(result.jackpot_payouts)
    if (payoutLabels.length > 0) {
      parts.push(`${text.value.jackpotHit} ${payoutLabels.join(' / ')}`)
    } else if (result.jackpot_winner_amount) {
    const jackpotAsset = result.jackpot_pool === 'balance' ? text.value.balanceShort : text.value.creditShort
    parts.push(`${text.value.jackpotHit} ${jackpotAsset} +${Number(result.jackpot_winner_amount).toFixed(2)}`)
    }
  }
  if (result.collectible_card) {
    parts.push(`${text.value.rareCardFound} · ${collectibleCardName(result.collectible_card)}`)
  }
  return parts.join(' · ')
}

function formatAsset(value: number | null | undefined, unit: string) {
  return `${Number(value ?? 0).toFixed(2)} ${unit}`
}

function applyClaimBalance(result: CheckinClaimResponse) {
  authStore.patchUserBalance(result.balance_after, result.credit_balance_after)
}

function previewStatus(): CheckinStatus {
  return {
    date: new Date().toISOString().slice(0, 10),
    free_claimed: false,
    paid_claimed: false,
    paid_cost: 18,
    free_reward_min: 0.2,
    free_reward_max: 0.5,
    paid_reward_min: 0.4,
    paid_reward_max: 0.7,
    credit_reward_min: 5,
    credit_reward_max: 100,
    balance_reward_min: 1,
    balance_reward_max: 20,
    balance: 88,
    credit_balance: 2680,
    streak_days: 6,
    credit_jackpot: 12888,
    balance_jackpot: 688,
    milestones: [
      { days: 3, eligible: true, claimed: false, activation_required: false, claimable: true, requires_verified: false, requires_api_usage: false, credit_reward: 30, balance_reward: 0 },
      { days: 5, eligible: true, claimed: false, activation_required: false, claimable: true, requires_verified: false, requires_api_usage: false, credit_reward: 50, balance_reward: 0 },
      { days: 7, eligible: false, claimed: false, activation_required: false, claimable: false, requires_verified: false, requires_api_usage: false, credit_reward: 100, balance_reward: 0 }
    ],
    credit_count: 0,
    balance_count: 0,
    credit_limit: 50,
    balance_limit: 10,
    credit_cost: 20,
    balance_cost: 5
  }
}

async function loadStatus() {
  const operationUserId = user.value?.id
  const sequence = ++statusLoadSequence
  statusLoading.value = true
  statusLoadError.value = ''
  try {
  if (isLocalPreviewAuth()) {
    status.value = previewStatus()
    activeCreditLotterySession.value = null
    return
  }

  if (!user.value) {
    status.value = props.autoOpen ? previewStatus() : null
    activeCreditLotterySession.value = null
    return
  }
  const [nextStatus, nextActiveCreditLotterySession] = await Promise.all([
    userAPI.getCheckinStatus(),
    userAPI.getActiveCreditLotterySession().catch(() => null)
  ])
  if (!isStatusLoadCurrent(sequence, operationUserId)) return
  status.value = nextStatus
  activeCreditLotterySession.value = nextActiveCreditLotterySession
  authStore.patchUserBalance(status.value.balance, status.value.credit_balance)
  } catch (error) {
    if (!isStatusLoadCurrent(sequence, operationUserId)) return
    status.value = null
    activeCreditLotterySession.value = null
    statusLoadError.value = extractApiErrorMessage(error, '无法读取今日剩余次数和余额，请重新加载。')
    throw error
  } finally {
    if (isStatusLoadCurrent(sequence, operationUserId)) statusLoading.value = false
  }
}

function retryLoadStatus() {
  void loadStatus().catch(() => undefined)
}

function openGift() {
  rememberDialogFocus()
  open.value = true
  resetFlow()
  void loadStatus().catch(() => undefined)
  void restorePendingCheckinOperation(false)
  void focusView(chooseViewRef)
}

function closeGift() {
  open.value = false
  resetFlow()
  drawStepTrigger = null
  void restoreDialogFocus()
}

function resetFlow() {
  view.value = 'choose'
  stage.value = ''
  activePanel.value = null
  selectedType.value = 'free'
  pendingResult.value = null
  pendingCard.value = null
  spinCards.value = []
  blessingTitle.value = text.value.ready
  blessingCopy.value = text.value.chooseHint
  pills.value = [text.value.waitOpen]
}

function enterDraw(type: FortuneType) {
  if (statusLoading.value || statusLoadError.value || !status.value) return
  const option = options.value.find(item => item.type === type)
  if (!option || option.disabled) return
  drawStepTrigger = document.activeElement instanceof HTMLElement ? document.activeElement : null
  selectedType.value = type
  pendingResult.value = null
  pendingCard.value = null
  spinCards.value = buildSpinDeck(type)
  view.value = 'draw'
  stage.value = 'idle'
  blessingTitle.value = text.value.ready
  blessingCopy.value = text.value.readyCopy
  pills.value = [option.title, format(text.value.remaining, { n: remaining(type) })]
  void focusView(drawViewRef)
}

function backToChoose() {
  if (pendingCheckinOperationForUser(user.value?.id)) {
    void restorePendingCheckinOperation(false)
    return
  }
  view.value = 'choose'
  stage.value = ''
  pendingResult.value = null
  pendingCard.value = null
  spinCards.value = []
  const target = drawStepTrigger
  drawStepTrigger = null
  void nextTick(() => (target?.isConnected ? target : chooseViewRef.value)?.focus())
}

async function startDraw() {
  if (stage.value === 'spinning' || stage.value === 'uncertain' || loadingType.value) return
  const option = options.value.find(item => item.type === selectedType.value)
  if (!option || option.disabled) return
  if (selectedType.value === 'credit') return
  if (pendingCheckinOperationForUser(user.value?.id)) {
    await restorePendingCheckinOperation(false)
    return
  }
  if (statusLoading.value || statusLoadError.value || !status.value) {
    appStore.showError('今日状态尚未确认，请返回后重新加载。')
    return
  }

  const operationUserId = user.value?.id
  const operation: PendingCheckinOperation = {
    operationId: makeOperationId(`checkin-${selectedType.value}`),
    type: selectedType.value,
    createdAt: Date.now()
  }
  pendingCheckinOperation.value = operation
  pendingCheckinOperationUserId = operationUserId
  storePendingCheckinOperation(operationUserId, operation)
  resetCheckinRetryState()
  const attemptSequence = beginCheckinPostAttempt(operation, operationUserId)

  spinCards.value = buildSpinDeck(selectedType.value)
  stage.value = 'spinning'
  loadingType.value = selectedType.value
  blessingTitle.value = text.value.spinning
  blessingCopy.value = text.value.spinningCopy
  pills.value = [option.title, text.value.waitOpen]

  try {
    const [result] = await Promise.all([
      userAPI.claimCheckin(selectedType.value, operation.operationId),
      waitForMinimumSpin()
    ])
    if (!isCheckinAttemptCurrent(attemptSequence, operationUserId)) return
    applyRecoveredCheckinResult(result, operation, operationUserId, false)
  } catch (error) {
    if (!isCheckinAttemptCurrent(attemptSequence, operationUserId)) return
    if (isUncertainRequestError(error)) {
      showPendingCheckinOperation(operation, operationUserId)
    } else {
      clearPendingCheckinOperationState(operationUserId, operation.operationId)
      resetCheckinRetryState()
      stage.value = 'idle'
      blessingTitle.value = text.value.ready
      blessingCopy.value = text.value.readyCopy
      pills.value = [option.title]
    }
    appStore.showError(extractApiErrorMessage(error, text.value.failed))
  } finally {
    if (attemptSequence === checkinAttemptSequence) {
      clearCheckinConfirmationTimer()
      checkinPostInFlight.value = false
      loadingType.value = null
      updatePendingCheckinCopy()
    }
  }
}

function waitForMinimumSpin() {
  return new Promise(resolve => window.setTimeout(resolve, prefersReducedMotion.value ? 0 : 900))
}

async function refreshAfterDraw(operationUserId: LotteryUserId) {
  if (!isCheckinUserContextCurrent(operationUserId)) return
  await loadStatus().catch(() => undefined)
}

function isUncertainRequestError(error: unknown) {
  if (!error || typeof error !== 'object') return true
  const value = error as { status?: number; response?: { status?: number } }
  const statusCode = Number(value.status ?? value.response?.status ?? 0)
  return statusCode === 0 || statusCode === 408 || statusCode === 429 || statusCode >= 500
}

async function reviewUncertainResult() {
  if (canRetryCheckin.value) {
    await retryPendingCheckinOperation()
    return
  }
  await restorePendingCheckinOperation(true)
}

async function restorePendingCheckinOperation(showFailure: boolean) {
  const operationUserId = user.value?.id
  const operation = pendingCheckinOperationForUser(operationUserId)
  if (!operation) return false
  pendingCheckinOperation.value = operation
  pendingCheckinOperationUserId = operationUserId
  showPendingCheckinOperation(operation, operationUserId)
  if (recoveryLoading.value) return false
  const attemptSequence = checkinAttemptSequence
  const recoverySequence = ++checkinRecoverySequence
  recoveryLoading.value = true
  try {
    const result = await userAPI.getCheckinOperation(operation.operationId)
    if (!isCheckinRecoveryCurrent(recoverySequence, attemptSequence, operationUserId)) return false
    if (!result || typeof result !== 'object') {
      if (showFailure) appStore.showError('操作查询返回了空结果，暂时不能确认是否受理；请继续查询，系统不会开放重复提交。')
      return false
    }
    applyRecoveredCheckinResult(result, operation, operationUserId, true)
    return true
  } catch (error) {
    if (!isCheckinRecoveryCurrent(recoverySequence, attemptSequence, operationUserId)) return false
    if (isLotteryOperationNotFoundError(error, 'CHECKIN_OPERATION_NOT_FOUND')) {
      markCheckinOperationNotFound(operation)
      if (showFailure) appStore.showError(canRetryCheckin.value
        ? '服务器尚未登记本次操作，可点击“重试原操作”安全续办。'
        : (checkinPostInFlight.value
            ? '服务器尚未登记本次操作，但原请求仍在等待返回；当前只继续查询。'
            : '服务器尚未登记本次操作，安全窗口结束后可用原操作号重试。'))
    } else if (showFailure) {
      appStore.showError(extractApiErrorMessage(error, '原结果尚未确认，请稍后继续查询；系统不会重新扣款。'))
    }
    return false
  } finally {
    if (recoverySequence === checkinRecoverySequence) recoveryLoading.value = false
  }
}

async function retryPendingCheckinOperation() {
  const operationUserId = user.value?.id
  const operation = pendingCheckinOperationForUser(operationUserId)
  if (!operation || !canRetryCheckin.value) return
  const retriedOperation: PendingCheckinOperation = { ...operation, lastAttemptAt: Date.now() }
  pendingCheckinOperation.value = retriedOperation
  pendingCheckinOperationUserId = operationUserId
  storePendingCheckinOperation(operationUserId, retriedOperation)
  resetCheckinRetryState()
  const attemptSequence = beginCheckinPostAttempt(retriedOperation, operationUserId)
  recoveryLoading.value = true
  let shouldRecover = false
  try {
    const result = await userAPI.claimCheckin(retriedOperation.type, retriedOperation.operationId)
    if (!isCheckinAttemptCurrent(attemptSequence, operationUserId)) return
    applyRecoveredCheckinResult(result, retriedOperation, operationUserId, false)
  } catch (error) {
    if (!isCheckinAttemptCurrent(attemptSequence, operationUserId)) return
    if (isUncertainRequestError(error)) {
      showPendingCheckinOperation(retriedOperation, operationUserId)
      shouldRecover = true
    } else {
      clearPendingCheckinOperationState(operationUserId, retriedOperation.operationId)
      resetCheckinRetryState()
      stage.value = 'idle'
      blessingTitle.value = text.value.ready
      blessingCopy.value = text.value.readyCopy
      pills.value = [selectedOption.value?.title || '']
      appStore.showError(extractApiErrorMessage(error, '原操作未被受理，请确认次数和余额后重试。'))
    }
  } finally {
    if (attemptSequence === checkinAttemptSequence) {
      clearCheckinConfirmationTimer()
      checkinPostInFlight.value = false
      recoveryLoading.value = false
      updatePendingCheckinCopy()
    }
  }
  if (shouldRecover && attemptSequence === checkinAttemptSequence) {
    await restorePendingCheckinOperation(false)
    if (pendingCheckinOperation.value) appStore.showError('原操作仍在确认中，请稍后继续查询；系统不会重复扣款。')
  }
}

function showPendingCheckinOperation(operation: PendingCheckinOperation, operationUserId: LotteryUserId) {
  if (!isCheckinUserContextCurrent(operationUserId)) return
  const option = options.value.find(item => item.type === operation.type)
  if (!open.value) rememberDialogFocus()
  open.value = true
  selectedType.value = operation.type
  pendingResult.value = null
  pendingCard.value = null
  spinCards.value = buildSpinDeck(operation.type)
  view.value = 'draw'
  stage.value = 'uncertain'
  blessingTitle.value = flowText.value.uncertain
  updatePendingCheckinCopy()
  pills.value = option ? [option.title] : []
  void focusView(drawViewRef)
}

function applyRecoveredCheckinResult(result: CheckinClaimResponse, operation: PendingCheckinOperation, operationUserId: LotteryUserId, invalidatePendingPost = false) {
  if (!isCheckinUserContextCurrent(operationUserId)) return
  assertCheckinClaimResponse(result, operation)
  if (invalidatePendingPost && checkinPostInFlight.value) invalidateCheckinPostAttempt()
  const option = options.value.find(item => item.type === operation.type)
  const rarity = rewardRarity(operation.type, result.reward)
  pendingResult.value = result
  pendingCard.value = pickBlessing(operation.type, rarity, result)
  applyClaimBalance(result)
  selectedType.value = operation.type
  view.value = 'draw'
  stage.value = 'ready'
  blessingTitle.value = text.value.flip
  blessingCopy.value = text.value.flipCopy
  pills.value = result.collectible_card
    ? [option?.title || '', `${text.value.rareCardFound} · ${collectibleCardName(result.collectible_card)}`].filter(Boolean)
    : option ? [option.title] : []
  clearPendingCheckinOperationState(operationUserId, operation.operationId)
  resetCheckinRetryState()
  void refreshAfterDraw(operationUserId)
}

function markCheckinOperationNotFound(operation: PendingCheckinOperation) {
  checkinOperationNotFound.value = true
  clearCheckinRetryTimer()
  const remaining = lotteryOperationRetryRemainingMs(operation)
  if (remaining === 0) {
    checkinRetryReady.value = true
    updatePendingCheckinCopy()
    return
  }
  checkinRetryReady.value = false
  updatePendingCheckinCopy()
  checkinRetryTimer = window.setTimeout(() => {
    checkinRetryTimer = null
    checkinRetryReady.value = true
    updatePendingCheckinCopy()
  }, remaining)
}

function beginCheckinPostAttempt(operation: PendingCheckinOperation, operationUserId: LotteryUserId) {
  const sequence = ++checkinAttemptSequence
  checkinPostInFlight.value = true
  clearCheckinConfirmationTimer()
  checkinConfirmationTimer = window.setTimeout(() => {
    if (!isCheckinAttemptCurrent(sequence, operationUserId) || !checkinPostInFlight.value) return
    checkinConfirmationTimer = null
    showPendingCheckinOperation(operation, operationUserId)
  }, 6000)
  return sequence
}

function invalidateCheckinPostAttempt() {
  checkinAttemptSequence += 1
  clearCheckinConfirmationTimer()
  checkinPostInFlight.value = false
  loadingType.value = null
}

function invalidateCheckinAsyncState() {
  invalidateCheckinPostAttempt()
  checkinRecoverySequence += 1
  statusLoadSequence += 1
  recoveryLoading.value = false
  statusLoading.value = false
}

function isSameLotteryUserId(left: LotteryUserId, right: LotteryUserId) {
  return String(left ?? 'anonymous') === String(right ?? 'anonymous')
}

function isStatusLoadCurrent(sequence: number, operationUserId: LotteryUserId) {
  return sequence === statusLoadSequence && isCheckinUserContextCurrent(operationUserId)
}

function assertCheckinClaimResponse(value: unknown, operation: PendingCheckinOperation): asserts value is CheckinClaimResponse {
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    throw new Error('checkin operation response is empty or malformed')
  }
  const candidate = value as Partial<CheckinClaimResponse>
  if (candidate.operation_id !== operation.operationId
    || candidate.type !== operation.type
    || typeof candidate.date !== 'string'
    || candidate.date.trim() === ''
    || !isFiniteNumber(candidate.cost)
    || candidate.cost < 0
    || !isFiniteNumber(candidate.reward)
    || candidate.reward < 0
    || (candidate.reward_asset !== 'credit' && candidate.reward_asset !== 'balance')
    || !isFiniteNumber(candidate.balance_after)
    || !isFiniteNumber(candidate.credit_balance_after)
    || typeof candidate.already_claimed !== 'boolean') {
    throw new Error('checkin operation response is missing required fields')
  }
}

function isFiniteNumber(value: unknown): value is number {
  return typeof value === 'number' && Number.isFinite(value)
}

function isCheckinUserContextCurrent(operationUserId: LotteryUserId) {
  return checkinComponentActive && isSameLotteryUserId(operationUserId, user.value?.id)
}

function isCheckinAttemptCurrent(sequence: number, operationUserId: LotteryUserId) {
  return sequence === checkinAttemptSequence && isCheckinUserContextCurrent(operationUserId)
}

function isCheckinRecoveryCurrent(recoverySequence: number, attemptSequence: number, operationUserId: LotteryUserId) {
  return recoverySequence === checkinRecoverySequence
    && attemptSequence === checkinAttemptSequence
    && isCheckinUserContextCurrent(operationUserId)
}

function pendingCheckinOperationForUser(operationUserId: LotteryUserId) {
  if (pendingCheckinOperation.value && isSameLotteryUserId(pendingCheckinOperationUserId, operationUserId)) {
    return pendingCheckinOperation.value
  }
  return readPendingCheckinOperation(operationUserId)
}

function clearPendingCheckinOperationState(operationUserId: LotteryUserId, operationId: string) {
  clearPendingCheckinOperation(operationUserId, operationId)
  if (pendingCheckinOperation.value?.operationId === operationId
    && isSameLotteryUserId(pendingCheckinOperationUserId, operationUserId)) {
    pendingCheckinOperation.value = null
    pendingCheckinOperationUserId = null
  }
}

function updatePendingCheckinCopy() {
  if (stage.value !== 'uncertain') return
  if (checkinRetryReady.value && checkinPostInFlight.value) {
    blessingCopy.value = flowText.value.postingCopy
  } else if (canRetryCheckin.value) {
    blessingCopy.value = flowText.value.retryCopy
  } else if (checkinOperationNotFound.value) {
    blessingCopy.value = flowText.value.waitingCopy
  } else {
    blessingCopy.value = flowText.value.uncertainCopy
  }
}

function resetCheckinRetryState() {
  clearCheckinRetryTimer()
  checkinOperationNotFound.value = false
  checkinRetryReady.value = false
}

function clearCheckinRetryTimer() {
  if (checkinRetryTimer === null) return
  window.clearTimeout(checkinRetryTimer)
  checkinRetryTimer = null
}

function clearCheckinConfirmationTimer() {
  if (checkinConfirmationTimer === null) return
  window.clearTimeout(checkinConfirmationTimer)
  checkinConfirmationTimer = null
}

function makeOperationId(action: string) {
  const suffix = typeof crypto !== 'undefined' && 'randomUUID' in crypto
    ? crypto.randomUUID()
    : `${Date.now()}-${Math.random().toString(16).slice(2)}`
  return `${action}-${suffix}`
}

async function handleCreditLotteryRefresh() {
  const operationUserId = user.value?.id
  await loadStatus().catch(() => undefined)
  if (!isCheckinUserContextCurrent(operationUserId)) return
}

function compactRewardLabel(result: CheckinClaimResponse) {
  const asset = result.reward_asset === 'balance' ? text.value.balanceShort : text.value.creditShort
  const digits = result.reward_asset === 'balance' ? 2 : 0
  return `${asset} +${Number(result.reward || 0).toFixed(digits)}`
}

function resultFinanceSummary(result: CheckinClaimResponse) {
  const costUnit = result.type === 'balance' ? text.value.balanceShort : text.value.creditShort
  const currentValue = result.type === 'balance' ? result.balance_after : result.credit_balance_after
  const parts = result.cost > 0 ? [`消耗 ${Number(result.cost).toFixed(result.type === 'balance' ? 2 : 0)} ${costUnit}`] : []
  parts.push(`获得 ${compactRewardLabel(result)}`)
  parts.push(`当前${costUnit} ${Number(currentValue || 0).toFixed(result.type === 'balance' ? 2 : 0)}`)
  return parts.join(' · ')
}

function revealReward() {
  if (stage.value !== 'ready' || !pendingResult.value || !pendingCard.value) return
  stage.value = 'revealed'
  blessingTitle.value = flowText.value.revealed
  blessingCopy.value = resultFinanceSummary(pendingResult.value)
  const label = rewardLabel(pendingResult.value)
  pills.value = pendingResult.value.collectible_card ? [`${text.value.rareCardFound} · ${collectibleCardName(pendingResult.value.collectible_card)}`] : []
  appStore.showSuccess(label)
}

usePointerDownOutside([rootRef], closeGift, open)

function rememberDialogFocus() {
  if (open.value) return
  const active = document.activeElement
  previouslyFocusedElement = active instanceof HTMLElement && active !== document.body ? active : triggerRef.value
}

async function restoreDialogFocus() {
  const target = previouslyFocusedElement || triggerRef.value
  previouslyFocusedElement = null
  await nextTick()
  if (target?.isConnected) target.focus()
}

async function focusView(target: { value: HTMLElement | null }) {
  await nextTick()
  target.value?.focus()
}

function handleDialogKeydown(event: KeyboardEvent) {
  if (event.key === 'Escape') {
    event.preventDefault()
    event.stopPropagation()
    closeGift()
    return
  }
  if (event.key !== 'Tab') return

  const scope = view.value === 'draw' ? drawViewRef.value : chooseViewRef.value
  if (!scope) return
  const focusable = Array.from(scope.querySelectorAll<HTMLElement>(focusableSelector))
    .filter(element => element.getAttribute('aria-hidden') !== 'true')
  if (!focusable.length) {
    event.preventDefault()
    scope.focus()
    return
  }

  const first = focusable[0]
  const last = focusable[focusable.length - 1]
  const active = document.activeElement
  if (event.shiftKey && (active === first || !scope.contains(active))) {
    event.preventDefault()
    last.focus()
  } else if (!event.shiftKey && (active === last || !scope.contains(active))) {
    event.preventDefault()
    first.focus()
  }
}

watch(() => user.value?.id, async () => {
  invalidateCheckinAsyncState()
  resetCheckinRetryState()
  pendingCheckinOperation.value = null
  pendingCheckinOperationUserId = null
  await loadStatus().catch(() => undefined)
  await restorePendingCheckinOperation(false)
}, { immediate: true })
watch(lang, () => resetFlow())
watch(() => props.autoOpen, (enabled) => {
  if (enabled) openGift()
}, { immediate: true })

onBeforeUnmount(() => {
  checkinComponentActive = false
  invalidateCheckinAsyncState()
  clearCheckinRetryTimer()
  clearCheckinConfirmationTimer()
  if (open.value && previouslyFocusedElement?.isConnected) previouslyFocusedElement.focus()
})
</script>

<style scoped>
.gift-shell { position: relative; display: inline-flex; overflow: visible; }
.fortune-entry { position: relative; display: inline-flex; align-items: center; gap: 8px; padding: 8px 14px 8px 10px; border: 0; border-radius: 999px; color: var(--zc-text-strong); background: var(--zc-bg); box-shadow: 5px 5px 12px var(--zc-shadow-dark), -5px -5px 12px var(--zc-shadow-light); cursor: pointer; font-size: 14px; font-weight: 900; line-height: 1.15; transition: transform .2s ease, box-shadow .2s ease, color .2s ease; }
.fortune-entry:hover { color: var(--zc-accent); transform: translateY(-1px); box-shadow: 7px 7px 16px var(--zc-shadow-dark), -7px -7px 16px var(--zc-shadow-light); }
.gift-box { width: 24px; height: 24px; display: grid; place-items: center; border-radius: 9px; background: var(--zc-bg); box-shadow: inset 3px 3px 7px var(--zc-shadow-dark), inset -3px -3px 7px var(--zc-shadow-light); font-size: 15px; transform-origin: 50% 100%; animation: giftShake 1.75s ease-in-out infinite; }
.gift-entry.claimed .gift-box { animation: none; }
.gift-dot { position: absolute; top: 5px; right: 10px; width: 9px; height: 9px; border-radius: 50%; background: var(--zc-danger); box-shadow: 0 0 0 4px color-mix(in srgb, var(--zc-danger) 14%, transparent); z-index: 2; }
@keyframes giftShake { 0%, 78%, 100% { transform: rotate(0deg) translateY(0); } 82% { transform: rotate(-7deg) translateY(-1px); } 86% { transform: rotate(7deg) translateY(-1px); } 90% { transform: rotate(-5deg); } 94% { transform: rotate(4deg); } }
.modal-backdrop { position: absolute; top: 42px; right: 0; z-index: 180; width: min(320px, calc(100vw - 24px)); max-width: calc(100vw - 24px); }
.modal { width: min(320px, calc(100vw - 24px)); max-width: 100%; max-height: min(720px, calc(100dvh - 72px)); overflow-x: hidden; overflow-y: auto; padding: 0; border-radius: 22px; background: rgba(255,255,255,.94); border: 1px solid rgba(0,0,0,.06); box-shadow: 0 24px 70px rgba(0,0,0,.16); backdrop-filter: blur(22px); }
.mini-stage { display: block; width: 100%; }
.mini-view:focus { outline: none; }
.mini-view { width: 100%; padding: 12px; }
.mini-popover:not(.is-drawing) .draw-view,
.mini-popover.is-drawing .choose-view { display: none; }
.choose-view { transition: opacity .3s ease, transform .46s cubic-bezier(.16, 1, .3, 1), filter .3s ease; }
.draw-view { transition: opacity .2s ease, transform .2s ease; }
.modal-head, .mini-draw-head { display: flex; align-items: center; justify-content: space-between; gap: 8px; margin-bottom: 8px; }
h2 { margin: 0; font-size: 16px; font-weight: 950; letter-spacing: -0.05em; }
.modal-sub { color: #6e6e73; margin-top: 3px; font-size: 11px; }
.close, .back-button { border: 0; width: 28px; height: 28px; border-radius: 50%; background: rgba(0,0,0,.06); cursor: pointer; font-size: 14px; display: grid; place-items: center; }
.daily-status { display: grid; grid-template-columns: .9fr 1.1fr; gap: 7px; margin-top: 8px; }
.status-read-state { min-height: 238px; display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 8px; padding: 22px; border-radius: 18px; border: 1px solid rgba(200,145,47,.16); background: rgba(255,255,255,.68); color: #1d1d1f; text-align: center; }
.status-read-state b { font-size: 13px; font-weight: 950; }
.status-read-state small { max-width: 220px; color: #6e6e73; font-size: 10px; font-weight: 800; line-height: 1.55; }
.status-read-state button, .panel-load-error button { min-height: 30px; border: 0; border-radius: 999px; padding: 0 12px; background: var(--zc-bg); color: var(--zc-accent); box-shadow: var(--gift-raise-sm); font-size: 10px; font-weight: 950; cursor: pointer; }
.status-read-spinner { width: 22px; height: 22px; border-radius: 999px; border: 2px solid rgba(154,101,12,.18); border-top-color: #9a650c; animation: giftStatusSpin .8s linear infinite; }
.status-read-error { border-color: color-mix(in srgb, var(--zc-danger) 30%, transparent); }
@keyframes giftStatusSpin { to { transform: rotate(360deg); } }
.status-tile { padding: 7px 9px; border-radius: 15px; background: linear-gradient(180deg, rgba(255,255,255,.92), rgba(255,248,232,.72)); border: 1px solid rgba(200,145,47,.16); box-shadow: 0 8px 18px rgba(116,75,10,.05); }
.status-kicker { color: #6e6e73; font-size: 9px; font-weight: 800; }
.status-main { margin-top: 1px; font-size: 15px; font-weight: 950; letter-spacing: -.03em; }
.status-main.done { color: #16833a; }
.status-sub { margin-top: 1px; color: #9a650c; font-size: 8px; font-weight: 850; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.quota-list { display: grid; gap: 3px; }
.quota-row { display: flex; align-items: center; justify-content: space-between; gap: 8px; font-size: 9px; font-weight: 850; color: #4b5563; }
.quota-row b { color: #1d1d1f; }
.quota-row span:last-child { color: #9a650c; }
.milestone-mini-row { grid-column: 1 / -1; display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 4px; padding: 4px; border-radius: 13px; background: rgba(255,249,235,.72); border: 1px solid rgba(200,145,47,.14); }
.milestone-policy { grid-column: 1 / -1; margin: 0; color: #6e6e73; font-size: 10px; line-height: 1.5; }
.milestone-chip { min-width: 0; height: 25px; display: grid; grid-template-columns: auto 1fr auto; align-items: center; gap: 4px; padding: 0 6px; border-radius: 10px; border: 1px solid rgba(0,0,0,.06); background: rgba(255,255,255,.76); color: #6e6e73; cursor: pointer; }
.milestone-chip b { color: #1d1d1f; font-size: 10px; font-weight: 950; }
.milestone-chip span { color: #9a650c; font-size: 8px; font-weight: 950; white-space: nowrap; }
.milestone-chip i { width: 5px; height: 5px; border-radius: 999px; background: #c7c7cc; overflow: hidden; text-indent: 999px; }
.milestone-chip.eligible { border-color: rgba(22,131,58,.28); background: linear-gradient(180deg, #fff, #ecfdf3); }
.milestone-chip.eligible i { background: #16833a; box-shadow: 0 0 0 3px rgba(22,131,58,.12); }
.milestone-chip.claimed { opacity: .58; }
.milestone-chip.claimed i { background: #9a650c; }
.milestone-chip:disabled { cursor: default; }
.sign-options { position: relative; display: grid; grid-template-columns: repeat(3, 1fr); gap: 6px; margin-top: 8px; padding-top: 22px; }
.records-corner, .cards-corner, .shop-corner { position: absolute; top: 0; z-index: 3; height: 19px; padding: 0 6px; border-radius: 999px; border: 1px solid rgba(200,145,47,.22); background: rgba(255,255,255,.88); color: #9a650c; font-size: 8px; font-weight: 950; cursor: pointer; box-shadow: 0 5px 12px rgba(116,75,10,.08); }
.records-corner { right: 6px; }
.cards-corner { right: 40px; }
.shop-corner { right: 76px; color: var(--zc-accent); background: var(--zc-bg); border-color: transparent; }
.sign-card-wrap { position: relative; min-width: 0; min-height: 122px; display: flex; }
.sign-card { width: 100%; min-width: 0; min-height: 122px; display: flex; flex-direction: column; padding: 9px 7px; border-radius: 16px; border: 1px solid rgba(0,0,0,.06); background: linear-gradient(180deg, rgba(255,255,255,.94), rgba(255,248,232,.74)); cursor: pointer; transition: transform .2s ease, box-shadow .2s ease, opacity .2s ease; text-align: left; }
.sign-card:hover { transform: translateY(-2px); border-color: rgba(200,145,47,.42); box-shadow: 0 14px 26px rgba(116,75,10,.1); }
.sign-card.used { opacity: .72; }
.sign-card.disabled { opacity: .42; cursor: default; }
.sign-card.disabled:hover { transform: none; border-color: rgba(0,0,0,.06); box-shadow: none; }
.sign-icon { font-size: 16px; margin-bottom: 7px; }
.sign-title { font-size: 13px; font-weight: 950; }
.sign-desc { display: block; margin-top: 7px; color: #6e6e73; font-size: 9px; line-height: 1.3; }
.sign-cost { margin-top: 6px; color: #9a650c; font-size: 9px; line-height: 1.25; font-weight: 950; }
.sign-prizes { margin-top: auto; padding-top: 7px; color: #1d1d1f; font-size: 8px; line-height: 1.25; font-weight: 850; }
.sign-meta { font-size: 10px; margin-top: 8px; color: #9a650c; font-weight: 850; }
.pool-preview { margin-top: 8px; padding: 8px; border-radius: 18px; background: rgba(255,255,255,.66); border: 1px solid rgba(200,145,47,.16); overflow: hidden; }
.pool-preview-head { display: flex; align-items: center; justify-content: space-between; gap: 8px; margin-bottom: 6px; font-size: 10px; color: #6e6e73; }
.pool-preview-head b { color: #1d1d1f; font-size: 11px; }
.jackpot-grid { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 6px; }
.jackpot-tile { min-width: 0; padding: 7px 6px; border-radius: 14px; background: linear-gradient(180deg, #fff, #f7f7fa); border: 1px solid rgba(0,0,0,.06); text-align: center; box-shadow: 0 8px 18px rgba(0,0,0,.05); }
.jackpot-tile b { display: block; color: #1d1d1f; font-size: 10px; font-weight: 950; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.jackpot-tile span { display: block; margin-top: 3px; color: #6e6e73; font-size: 8px; font-weight: 850; white-space: nowrap; }
.activity-panel { min-height: 280px; max-height: min(360px, calc(100dvh - 112px)); display: flex; flex-direction: column; border-radius: 18px; border: 1px solid rgba(200,145,47,.16); background: rgba(255,255,255,.64); overflow: hidden; }
.panel-head { display: flex; align-items: center; gap: 8px; padding: 8px; border-bottom: 1px solid rgba(0,0,0,.05); }
.panel-head b { font-size: 12px; font-weight: 950; color: #1d1d1f; }
.panel-head-with-action b { min-width: 0; flex: 1 1 auto; }
.panel-album-link { min-height: 24px; border: 0; border-radius: 999px; padding: 0 8px; background: var(--zc-bg); box-shadow: var(--gift-raise-sm); color: var(--zc-accent); font-size: 9px; font-weight: 950; cursor: pointer; }
.panel-back { width: 24px; height: 24px; border: 0; border-radius: 50%; background: rgba(0,0,0,.06); cursor: pointer; font-size: 14px; }
.panel-list { flex: 1; overflow: auto; padding: 8px; display: grid; align-content: start; gap: 6px; }
.panel-empty { min-height: 120px; display: grid; place-items: center; color: #8a8a8e; font-size: 11px; font-weight: 850; }
.panel-load-error { align-content: center; gap: 10px; padding: 16px; text-align: center; line-height: 1.5; }
.record-row { display: grid; grid-template-columns: 1fr auto; gap: 8px; align-items: center; padding: 8px; border-radius: 13px; border: 1px solid rgba(0,0,0,.05); background: rgba(255,255,255,.78); }
.record-row b { display: block; color: #1d1d1f; font-size: 11px; font-weight: 950; }
.record-row span { display: block; margin-top: 2px; color: #6e6e73; font-size: 9px; font-weight: 800; }
.record-row strong { display: block; color: #9a650c; font-size: 10px; font-weight: 950; text-align: right; }
.record-row em { display: block; margin-top: 2px; color: #16833a; font-size: 8px; font-style: normal; font-weight: 850; text-align: right; }
.card-collection-list { grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 8px; }
.collectible-card { position: relative; min-height: 236px; aspect-ratio: 2 / 3; display: flex; flex-direction: column; justify-content: space-between; gap: 7px; padding: 9px; overflow: hidden; border-radius: 19px; border: 1px solid rgba(129,92,30,.18); background: #f1e2bf; box-shadow: 0 12px 24px rgba(116,75,10,.1); color: inherit; text-align: left; cursor: zoom-in; }
.collectible-card::before { content: ""; position: absolute; inset: 6px; border-radius: 14px; border: 1px solid rgba(255,255,255,.62); pointer-events: none; z-index: 2; }
.collectible-card::after { content: ""; position: absolute; inset: 0; z-index: 1; background: linear-gradient(180deg, rgba(255,250,240,.18), transparent 36%, rgba(18,16,12,.18)); pointer-events: none; }
.collectible-card-base { position: absolute; inset: 0; width: 100%; height: 100%; object-fit: cover; z-index: 0; }
.collectible-topline { position: relative; z-index: 3; display: flex; align-items: center; justify-content: flex-end; gap: 6px; color: rgba(73,50,17,.82); font-size: 8px; font-weight: 950; letter-spacing: .04em; }
.collectible-topline span, .collectible-topline i { padding: 3px 6px; border-radius: 999px; background: rgba(255,250,240,.78); border: 1px solid rgba(129,92,30,.18); box-shadow: 0 4px 10px rgba(116,75,10,.05); backdrop-filter: blur(6px); }
.collectible-topline i { font-style: normal; white-space: nowrap; }
.collectible-art { position: relative; z-index: 1; flex: 1; min-height: 74px; pointer-events: none; }
.collectible-image-sheen { position: absolute; inset: 0; background: linear-gradient(110deg, transparent 18%, rgba(255,255,255,.18) 48%, transparent 72%); opacity: .5; transform: translateX(-32%); }
.collectible-body { position: relative; z-index: 3; display: flex; align-items: end; justify-content: space-between; gap: 6px; margin: 0 -2px -2px; min-height: 48px; padding: 8px; border-radius: 13px; background: linear-gradient(180deg, rgba(255,250,240,.82), rgba(255,250,240,.94)); border: 1px solid rgba(255,255,255,.52); box-shadow: 0 8px 18px rgba(54,38,13,.08); backdrop-filter: blur(8px); }
.collectible-body b { min-width: 0; overflow: hidden; color: #1d1d1f; font-size: 12px; font-weight: 950; letter-spacing: -.04em; text-overflow: ellipsis; white-space: nowrap; }
.collectible-body strong { flex: 0 0 auto; color: rgba(29,29,31,.72); font-size: 9px; font-weight: 950; }
.collectible-card.good { border-color: rgba(72,150,70,.24); }
.collectible-card.good .collectible-body span { color: #2f6d34; }
.collectible-card.rare { border-color: rgba(74,115,168,.3); }
.collectible-card.rare .collectible-body span { color: #275f9d; }
.collectible-card.epic { color: #0f5266; border-color: rgba(21,147,185,.34); }
.collectible-card.epic .collectible-body span { color: #175c74; }
.collectible-card.legendary { border-color: rgba(151,105,25,.48); box-shadow: 0 16px 30px rgba(116,75,10,.18); }
.collectible-card.legendary::after { background: linear-gradient(180deg, rgba(255,250,240,.12), transparent 32%, rgba(78,50,8,.22)); }
.collectible-card.mythic { border-color: rgba(151,89,220,.46); box-shadow: 0 18px 34px rgba(123,60,196,.18); }
.collectible-card.mythic::before { border-color: rgba(255,255,255,.88); box-shadow: inset 0 0 0 1px rgba(151,89,220,.18); }
.collectible-card.mythic::after { background: linear-gradient(180deg, rgba(255,250,240,.1), transparent 30%, rgba(68,39,112,.2)); }
.collectible-card.mythic .collectible-body span { color: #4d2d86; }
.card-shop-panel { background: linear-gradient(180deg, rgba(255,255,255,.78), rgba(250,248,241,.76)); }
.card-shop-list { gap: 8px; }
.shop-balance-row { display: flex; align-items: center; justify-content: space-between; padding: 8px 10px; border-radius: 14px; background: rgba(255,255,255,.82); border: 1px solid rgba(141,103,35,.16); color: #6e6e73; font-size: 10px; font-weight: 850; }
.shop-balance-row b { color: #9a650c; font-size: 12px; font-weight: 950; }
.shop-tier-card { position: relative; display: grid; grid-template-columns: 30px 1fr auto; gap: 8px; align-items: center; min-height: 52px; padding: 10px; border-radius: 17px; border: 0; background: var(--zc-bg); text-align: left; cursor: pointer; box-shadow: var(--gift-inset); transition: transform .18s ease, box-shadow .18s ease, opacity .18s ease; }
.shop-tier-card:hover { transform: translateY(-1px); box-shadow: 0 14px 26px rgba(116,75,10,.12); }
.shop-tier-card:disabled { opacity: .46; cursor: default; transform: none; }
.shop-tier-card span { width: 28px; height: 32px; display: grid; place-items: center; border-radius: 12px; color: #5f3d08; background: rgba(255,255,255,.76); border: 1px solid rgba(129,92,30,.16); font-size: 13px; font-weight: 950; }
.shop-tier-card b { min-width: 0; color: #1d1d1f; font-size: 12px; font-weight: 950; }
.shop-tier-card strong { justify-self: end; color: #9a650c; font-size: 10px; font-weight: 950; white-space: nowrap; }
.shop-tier-card.good { background: radial-gradient(circle at 90% 10%, rgba(72,150,70,.16), transparent 34%), linear-gradient(135deg, #fbfff5, #fff); }
.shop-tier-card.rare { background: radial-gradient(circle at 90% 10%, rgba(74,115,168,.18), transparent 34%), linear-gradient(135deg, #f7fbff, #fff); }
.mini-type-badge { display: inline-flex; align-items: center; gap: 5px; padding: 5px 8px; border-radius: 999px; color: #9a650c; background: rgba(255,247,226,.92); border: 1px solid rgba(200,145,47,.22); font-size: 11px; font-weight: 850; }
.mini-draw { padding: 8px; border-radius: 18px; border: 1px solid rgba(200,145,47,.14); background: rgba(255,255,255,.58); transition: background .24s ease, border-color .24s ease, transform .24s ease; }
.mini-slot { position: relative; width: 100%; height: 128px; overflow: hidden; border-radius: 20px; border: 0; padding: 0; background: var(--zc-bg); box-shadow: var(--gift-inset); color: inherit; }
.mini-slot:disabled { opacity: 1; cursor: default; }
.mini-slot::before, .mini-slot::after { content: ""; position: absolute; top: 0; bottom: 0; width: 36px; z-index: 2; pointer-events: none; }
.mini-slot::before { left: 0; background: linear-gradient(90deg, var(--zc-bg), transparent); }
.mini-slot::after { right: 0; background: linear-gradient(270deg, var(--zc-bg), transparent); }
.mini-slot-track { height: 100%; display: flex; align-items: center; gap: 8px; padding: 0 36px; transform: translateX(0); }
.mini-draw.ready .mini-slot-track, .mini-draw.revealed .mini-slot-track { justify-content: center; padding: 0; }
.mini-draw.spinning .mini-slot-track { animation: miniSlotSpin .72s linear infinite; }
@keyframes miniSlotSpin { from { transform: translateX(0); } to { transform: translateX(-420px); } }
.mini-slot-card { position: relative; flex: 0 0 62px; height: 93px; border-radius: 18px; transform-style: preserve-3d; transition: transform .58s cubic-bezier(.16, 1, .3, 1), box-shadow .25s ease; box-shadow: 0 10px 22px rgba(116,75,10,.12); }
.mini-slot-card::before { content: ""; position: absolute; inset: -1px; border-radius: 19px; background: linear-gradient(160deg, rgba(255,255,255,.96), rgba(230,170,68,.26), rgba(255,255,255,.72)); z-index: -1; }
.mini-slot-card.flipped, .mini-draw.revealed .mini-slot-card { transform: rotateY(180deg); }
.card-face { position: absolute; inset: 0; display: grid; place-items: center; overflow: hidden; border-radius: 18px; backface-visibility: hidden; border: 1px solid rgba(200,145,47,.24); background: #fff2cf; }
.card-face::before { content: ""; position: absolute; inset: 6px; z-index: 2; border-radius: 12px; border: 1px solid rgba(255,255,255,.58); pointer-events: none; }
.card-face::after { content: ""; position: absolute; inset: 0; z-index: 1; background: linear-gradient(180deg, rgba(255,250,240,.1), transparent 42%, rgba(55,36,8,.22)); pointer-events: none; }
.wheel-card-image { position: absolute; inset: 0; width: 100%; height: 100%; object-fit: cover; }
.wheel-card-title { position: absolute; left: 7px; right: 7px; top: 8px; z-index: 3; display: block; padding: 4px 5px; border-radius: 10px; color: #1d1d1f; background: rgba(255,250,240,.82); border: 1px solid rgba(255,255,255,.5); font-size: 9px; font-weight: 950; line-height: 1.12; text-align: center; box-shadow: 0 6px 12px rgba(54,38,13,.08); backdrop-filter: blur(7px); }
.card-front { color: #1d1d1f; font-size: 15px; font-weight: 950; letter-spacing: 0; line-height: 1.12; }
:global(html[lang="en"]) .card-front, :global(html[lang="ru"]) .card-front { letter-spacing: 0; padding: 0; line-height: 1.18; font-size: 12px; text-align: center; }
.mini-slot-card.good .card-front { border-color: rgba(72,150,70,.25); }
.mini-slot-card.rare .card-front { border-color: rgba(70,128,210,.28); }
.mini-slot-card.epic .card-front { border-color: rgba(150,80,190,.3); }
.mini-slot-card.diamond .card-front { color: #175c74; border-color: rgba(21,147,185,.36); }
.mini-slot-card.rainbow .card-front { color: #7b3cc4; border-color: rgba(151,89,220,.36); }
.card-back { padding: 0; color: #5f3d08; transform: rotateY(180deg); font-size: 12px; font-weight: 800; line-height: 1.55; letter-spacing: 0; text-align: center; }
.wheel-card-heading { position: absolute; top: 7px; left: 7px; right: 7px; z-index: 3; display: grid; justify-items: center; gap: 3px; padding: 5px 6px; border-radius: 10px; background: rgba(255,250,240,.86); box-shadow: 0 6px 12px rgba(54,38,13,.08); backdrop-filter: blur(7px); }
.wheel-card-heading b { max-width: 100%; overflow: hidden; color: #1d1d1f; font-size: 10px; font-weight: 950; line-height: 1.12; text-overflow: ellipsis; white-space: nowrap; }
.wheel-card-footer { position: absolute; left: 7px; right: 7px; bottom: 8px; z-index: 3; display: grid; gap: 4px; padding: 6px; border-radius: 10px; background: rgba(255,250,240,.88); box-shadow: 0 6px 12px rgba(54,38,13,.08); backdrop-filter: blur(7px); }
.wheel-card-line { display: -webkit-box; max-height: 38px; overflow: hidden; color: #5f3d08; font-size: 9px; font-weight: 850; line-height: 1.28; text-align: center; -webkit-line-clamp: 2; -webkit-box-orient: vertical; }
.wheel-card-footer strong { color: #9a650c; font-size: 9px; font-weight: 950; line-height: 1.1; }
.rarity-badge { display: inline-flex; padding: 2px 6px; border-radius: 999px; background: rgba(255,250,240,.82); border: 1px solid rgba(200,145,47,.2); color: #9a650c; font-size: 8px; font-weight: 950; letter-spacing: .04em; line-height: 1.1; white-space: nowrap; backdrop-filter: blur(7px); }
.mini-slot-card.good .card-back { color: #2f6d34; background: linear-gradient(180deg, #fbfff8, #ecf9e5); border-color: rgba(72,150,70,.25); }
.mini-slot-card.rare .card-back { color: #275f9d; background: linear-gradient(180deg, #f8fcff, #e5f1ff); border-color: rgba(70,128,210,.28); }
.mini-slot-card.epic .card-back { color: #7d43a8; background: linear-gradient(180deg, #fffaff, #f3e3ff); border-color: rgba(150,80,190,.3); }
.mini-slot-card.diamond .card-back { color: #175c74; background: linear-gradient(135deg, #f7fdff, #dff7ff 52%, #ffffff); border-color: rgba(21,147,185,.36); }
.mini-slot-card.rainbow .card-back { color: #7b3cc4; background: linear-gradient(135deg, #fff7d8, #ffd8ef 42%, #dfe7ff 78%, #ffffff); border-color: rgba(151,89,220,.36); }
.mini-start { width: 100%; margin-top: 8px; border: 0; border-radius: 15px; padding: 9px 12px; color: #fff; background: linear-gradient(135deg, #1d1d1f, #3b3b42); box-shadow: 0 12px 24px rgba(0,0,0,.15); font-size: 12px; font-weight: 900; cursor: pointer; }
.mini-start:disabled { opacity: .58; cursor: wait; }
.mini-draw.ready .mini-start, .mini-draw.revealed .mini-start { display: none; }
.mini-draw.ready .mini-slot, .mini-draw.revealed .mini-slot { cursor: pointer; }
.mini-draw.ready .mini-slot, .mini-draw.revealed .mini-slot { height: 214px; }
.mini-draw.ready .mini-slot-card, .mini-draw.revealed .mini-slot-card { flex-basis: 132px; height: 198px; }
.mini-draw.revealed { background: linear-gradient(180deg, #fff, #fff8e8); border-color: rgba(200,145,47,.34); animation: miniFlip .34s ease both; }
@keyframes miniFlip { from { transform: rotateX(8deg) scale(.98); opacity: .8; } to { transform: none; opacity: 1; } }
.mini-blessing { margin-top: 8px; font-size: 15px; font-weight: 900; letter-spacing: .04em; text-align: center; }
.mini-copy { margin-top: 4px; min-height: 30px; color: #6e6e73; font-size: 11px; line-height: 1.35; text-align: center; }
.draw-announcement { display: contents; }
.mini-pills { display: flex; justify-content: center; gap: 5px; margin-top: 8px; flex-wrap: wrap; }
.mini-pills span { padding: 4px 7px; border-radius: 999px; background: rgba(245,245,247,.9); border: 1px solid rgba(0,0,0,.06); font-size: 10px; font-weight: 800; color: #4b5563; }

/* Cold-white neumorphism pass for the header gift popover. */
.gift-shell {
  --gift-raise: 14px 14px 30px var(--zc-shadow-dark), -14px -14px 30px var(--zc-shadow-light);
  --gift-raise-sm: 7px 7px 16px var(--zc-shadow-dark), -7px -7px 16px var(--zc-shadow-light);
  --gift-inset: inset 6px 6px 14px var(--zc-shadow-dark), inset -6px -6px 14px var(--zc-shadow-light);
}

.modal,
.status-tile,
.sign-card,
.pool-preview,
.activity-panel,
.mini-draw {
  border: 0;
  background: var(--zc-bg);
  box-shadow: var(--gift-raise);
  backdrop-filter: none;
}

.modal-sub,
.status-kicker,
.status-sub,
.quota-row,
.pool-preview-head,
.panel-empty,
.record-row span,
.mini-copy {
  color: var(--zc-muted);
}

h2,
.quota-row b,
.status-main,
.sign-title,
.sign-prizes,
.pool-preview-head b,
.jackpot-tile b,
.panel-head b,
.record-row b,
.shop-tier-card b,
.mini-blessing,
.wheel-card-title,
.collectible-body b {
  color: var(--zc-text-strong);
}

.close,
.back-button,
.panel-back,
.records-corner,
.cards-corner,
.shop-corner,
.milestone-chip,
.jackpot-tile,
.record-row,
.shop-balance-row,
.shop-tier-card,
.mini-type-badge,
.mini-pills span,
.collectible-topline span,
.collectible-topline i,
.collectible-body,
.wheel-card-title,
.wheel-card-line,
.rarity-badge {
  border: 0;
  background: var(--zc-bg);
  box-shadow: var(--gift-inset);
}

.milestone-mini-row,
.mini-slot {
  border: 0;
  background: var(--zc-bg);
  box-shadow: var(--gift-inset);
}

.milestone-chip.eligible,
.shop-tier-card.good,
.shop-tier-card.rare,
.card-shop-panel,
.mini-draw.revealed {
  background: var(--zc-bg);
}

.status-main.done,
.milestone-chip.eligible i,
.record-row em,
.shop-balance-row b {
  color: var(--zc-success);
}

.quota-row span:last-child,
.records-corner,
.cards-corner,
.shop-corner,
.sign-cost,
.sign-meta,
.record-row strong,
.shop-tier-card strong,
.mini-type-badge,
.rarity-badge {
  color: var(--zc-accent);
}

.sign-card:hover,
.shop-tier-card:hover {
  transform: translateY(-1px);
  box-shadow: var(--gift-raise-sm);
}

.mini-slot::before {
  background: linear-gradient(90deg, var(--zc-bg), transparent);
}

.mini-slot::after {
  background: linear-gradient(270deg, var(--zc-bg), transparent);
}

.mini-start {
  background: var(--zc-bg);
  color: var(--zc-accent);
  box-shadow: var(--gift-raise-sm);
}

.card-face {
  border: 0;
  background: var(--zc-bg);
}

.collectible-card {
  border: 0;
  background: var(--zc-bg);
  box-shadow: var(--gift-raise-sm);
}

.collectible-card::after {
  background: linear-gradient(180deg, transparent 42%, color-mix(in srgb, var(--zc-text-strong) 14%, transparent));
}

.card-face::before {
  border-color: color-mix(in srgb, var(--zc-shadow-light) 70%, transparent);
}

.card-face::after {
  background: linear-gradient(180deg, transparent 46%, color-mix(in srgb, var(--zc-text-strong) 16%, transparent));
}

.wheel-card-line,
.collectible-body em,
.collectible-body span,
.collectible-body small,
.collectible-body strong {
  color: var(--zc-muted);
}

@media (max-width: 640px) {
  .modal-backdrop {
    position: fixed;
    top: 56px;
    right: auto;
    left: max(12px, calc((100vw - 320px) / 2));
    width: min(320px, calc(100vw - 24px));
    max-width: calc(100vw - 24px);
  }

  .modal {
    width: 100%;
    max-height: calc(100dvh - 68px);
  }
}

@media (prefers-reduced-motion: reduce) {
  .gift-box,
  .mini-slot-track,
  .mini-draw.revealed,
  .mini-stage,
  .choose-view,
  .draw-view,
  .mini-slot-card { animation: none !important; transition-duration: .01ms !important; }
}

.fortune-pop-enter-active, .fortune-pop-leave-active { transition: opacity .18s ease, transform .18s ease; }
.fortune-pop-enter-from, .fortune-pop-leave-to { opacity: 0; transform: translateY(-4px) scale(.98); }
@media (prefers-reduced-motion: reduce) {
  .fortune-pop-enter-active,
  .fortune-pop-leave-active { transition: none !important; }
}
</style>
