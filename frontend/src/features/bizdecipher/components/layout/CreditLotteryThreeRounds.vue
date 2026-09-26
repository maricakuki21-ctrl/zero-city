<template>
  <div class="credit-lottery mini-draw" :class="drawState">
    <button
      class="mini-slot"
      type="button"
      :disabled="busy || !session || resultRevealed"
      :aria-label="session && !resultRevealed ? '翻开本轮签牌' : title"
      @click="revealResult"
    >
      <div class="mini-slot-track">
        <div
          v-for="card in displayCards"
          :key="card.key"
          class="mini-slot-card"
          :class="[card.rarity, { flipped: card.flipped }]"
        >
          <div class="card-face card-front">
            <img
              class="wheel-card-image"
              :src="card.image"
              :srcset="`${card.image} 1024w`"
              :alt="card.front"
              loading="lazy"
              decoding="async"
              sizes="(max-width: 640px) 42vw, 180px"
              @error="handleImageFallback($event, fallbackImage)"
            />
            <span class="wheel-card-title">{{ card.front }}</span>
          </div>
          <div class="card-face card-back">
            <img
              class="wheel-card-image"
              :src="card.image"
              :srcset="`${card.image} 1024w`"
              :alt="card.front"
              loading="lazy"
              decoding="async"
              sizes="(max-width: 640px) 42vw, 180px"
              @error="handleImageFallback($event, fallbackImage)"
            />
            <div class="wheel-card-heading">
              <span class="rarity-badge">{{ card.badge }}</span>
              <b>{{ card.front }}</b>
            </div>
            <div class="wheel-card-footer">
              <span class="wheel-card-line">{{ card.back }}</span>
              <strong v-if="card.reward">{{ card.reward }}</strong>
            </div>
          </div>
        </div>
      </div>
    </button>

    <div class="mini-blessing">{{ title }}</div>
    <div class="mini-copy">{{ copy }}</div>

    <div v-if="confirming" class="confirmation-panel" role="status" aria-live="polite">
      <b>结果确认中</b>
      <span>{{ confirmationMessage }}</span>
      <button class="mini-action secondary" type="button" :disabled="recoveryLoading" @click="handleConfirmationAction">
        {{ recoveryLoading ? '处理中' : confirmationActionLabel }}
      </button>
    </div>

    <div v-else-if="loading" class="mini-pills" role="status" aria-live="polite">
      <span>加载中</span>
    </div>

    <div v-else-if="loadError && !session" class="confirmation-panel load-error-panel" role="alert">
      <b>抽奖状态读取失败</b>
      <span>{{ loadError }}</span>
      <button class="mini-action secondary" type="button" :disabled="loading" @click="loadActive">
        {{ loading ? '重新加载中' : '重新加载' }}
      </button>
    </div>

    <template v-else-if="!session">
      <button class="mini-start" type="button" :disabled="startDisabled" @click="startSession">
        {{ busy ? spinningTitle : startLabel }}
      </button>
      <div class="mini-pills">
        <span>{{ typeLabel }}</span>
        <span>剩余 {{ remaining }} 次</span>
      </div>
    </template>

    <template v-else-if="!resultRevealed">
      <div class="mini-pills">
        <span>点击翻开</span>
        <span>{{ sessionModeLabel }}</span>
      </div>
    </template>

    <template v-else>
      <div class="mini-pills">
        <span>{{ session.status === 'active' ? '当前签牌' : '本次签牌' }}</span>
        <span>{{ sessionModeLabel }}</span>
        <span>{{ rewardLabel(displayRound) }}</span>
        <span v-if="displayRound?.collectible_candidate && session.status === 'active'">
          {{ cardName(displayRound.collectible_candidate) }}
        </span>
        <span v-if="displayRound?.collectible_card">
          {{ cardName(displayRound.collectible_card) }}
        </span>
        <span v-if="session.jackpot_hit && jackpotSessionLabel(session)">
          Jackpot {{ jackpotSessionLabel(session) }}
        </span>
      </div>

      <div v-if="session.status === 'active'" class="lottery-actions">
        <button class="mini-action secondary" type="button" :disabled="busy" @click="settleSession">
          收下本轮
        </button>
        <button
          class="mini-start continue"
          type="button"
          :disabled="busy || session.current_round >= session.max_rounds"
          @click="continueSession"
        >
          {{ busy ? '处理中' : `继续抽（会覆盖 ${rewardLabel(displayRound)}）` }}
        </button>
      </div>

      <p v-if="session.status === 'active'" class="overwrite-warning">
        继续抽会覆盖当前奖励；只有最后保留的一轮会结算。
      </p>

      <div v-else class="lottery-actions">
        <button class="mini-start continue" type="button" @click="completeSession">
          完成
        </button>
      </div>
    </template>

    <div v-if="!resultRevealed" class="mini-rules">
      <b>抽奖规则</b>
      <span>1. 一次消耗 20 积分，只占 1 次今日积分抽奖额度。一般情况下只抽一轮。</span>
      <span>2. 小概率触发三轮抽奖，最多抽 3 轮，下一轮会覆盖本轮奖励，提前结束或第 3 轮后自动结算奖励。</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import userAPI from '@/api/user'
import { stableCardIndex, zeroCityCardDisplayName } from '@/constants/zeroCityCardManifest'
import { useAppStore, useAuthStore } from '@/stores'
import { extractApiErrorMessage } from '@/utils/apiError'
import {
  clearPendingCreditLotteryOperation,
  isLotteryOperationNotFoundError,
  lotteryOperationRetryRemainingMs,
  readPendingCreditLotteryOperation,
  storePendingCreditLotteryOperation,
  type CreditLotteryOperationAction,
  type PendingCreditLotteryOperation
} from '@/utils/lotteryOperationRecovery'
import type { CheckinCollectibleCard, CreditLotteryRoundResult, CreditLotterySession, JackpotPayout } from '@/types'

type LotteryUserId = number | string | null | undefined

const emit = defineEmits<{
  refresh: []
  close: []
}>()

const props = withDefaults(defineProps<{
  remaining?: number
  typeLabel?: string
  readyTitle?: string
  readyCopy?: string
  startLabel?: string
  spinningTitle?: string
  spinningCopy?: string
}>(), {
  remaining: 50,
  typeLabel: '积分',
  readyTitle: '准备抽签',
  readyCopy: '六张福运卡已就位，点击开始后旋转。',
  startLabel: '点击开始',
  spinningTitle: '卡片旋转中',
  spinningCopy: '六张福运卡正在停下。'
})

const appStore = useAppStore()
const authStore = useAuthStore()

const loading = ref(true)
const loadError = ref('')
const busy = ref(false)
const confirming = ref(false)
const recoveryLoading = ref(false)
const session = ref<CreditLotterySession | null>(null)
const resultRevealed = ref(false)
const pendingOperation = ref<PendingCreditLotteryOperation | null>(null)
const operationNotFound = ref(false)
const operationRetryReady = ref(false)
const postInFlight = ref(false)
const spinDurationMs = 1200
let confirmationTimer: number | null = null
let operationRetryTimer: number | null = null
let actionSequence = 0
let recoverySequence = 0
let loadSequence = 0
let componentActive = true
let pendingOperationUserId: LotteryUserId = null

const title = computed(() => {
  if (confirming.value) return '结果确认中'
  if (busy.value) return props.spinningTitle
  if (session.value && !resultRevealed.value) return '点击翻开'
  if (session.value && resultRevealed.value) return '本次结果'
  return props.readyTitle
})
const copy = computed(() => {
  if (confirming.value) return '请求可能已经受理，请勿重复抽取。'
  if (busy.value) return props.spinningCopy
  if (session.value && !resultRevealed.value) return '签牌已停下，点击卡片查看奖励。'
  if (session.value && resultRevealed.value) return resultFinanceSummary.value
  return props.readyCopy
})
const displayRound = computed(() => session.value?.final_result || session.value?.current_result || null)
const sessionModeLabel = computed(() => {
  if (!session.value) return props.typeLabel
  if (session.value.mode === 'single' || session.value.max_rounds <= 1) return '单抽'
  if (session.value.status === 'active') return `第 ${session.value.current_round}/${session.value.max_rounds} 轮`
  return '三轮结算'
})
const resultFortuneCard = computed(() => {
  if (!displayRound.value) return creditWheelDeck[0]
  const rarity = rewardRarity(displayRound.value.reward_amount)
  const candidates = creditWheelDeck.filter(card => card.rarity === rarity)
  const seed = [session.value?.id || 0, displayRound.value.round_no, displayRound.value.reward_amount].join(':')
  return candidates[stableCardIndex(seed, candidates.length)] || creditWheelDeck[0]
})
const resultFinanceSummary = computed(() => {
  if (!session.value || !displayRound.value) return ''
  const parts = [
    `消耗积分 ${Number(session.value.cost_credit || 0).toFixed(0)}`,
    rewardLabel(displayRound.value)
  ]
  if (typeof session.value.credit_balance_after === 'number') {
    parts.push(`当前积分 ${Number(session.value.credit_balance_after).toFixed(0)}`)
  }
  return parts.join(' · ')
})
const startDisabled = computed(() => loading.value || Boolean(loadError.value) || confirming.value || busy.value || Boolean(pendingOperation.value) || props.remaining <= 0)
const canRetryOperation = computed(() => operationNotFound.value && operationRetryReady.value && !postInFlight.value)
const confirmationMessage = computed(() => {
  if (operationRetryReady.value && postInFlight.value) return '服务器暂未登记本次操作，但原请求仍在等待返回。当前只继续查询，不会并发提交第二次。'
  if (canRetryOperation.value) return '服务器已确认暂未登记本次操作。可用原操作号安全重试，不会生成第二笔结算。'
  if (operationNotFound.value) return '服务器暂未登记本次操作，正在等待安全窗口。期间不会重复提交或扣款。'
  return '请求可能已经受理，正在读取本次会话。请勿重复抽取。'
})
const confirmationActionLabel = computed(() => canRetryOperation.value ? '重试原操作' : '刷新结果')
const drawState = computed(() => {
  if (confirming.value) return 'confirming'
  if (busy.value) return 'spinning'
  if (!session.value) return 'idle'
  return resultRevealed.value ? 'revealed' : 'ready'
})
const displayCards = computed(() => {
  if (busy.value) return spinDeck.value
  if (session.value && displayRound.value) {
    const rarity = rewardRarity(displayRound.value.reward_amount)
    const card = resultFortuneCard.value
    return [{
      key: `round-${displayRound.value.round_no}`,
      rarity,
      flipped: resultRevealed.value,
      image: wheelCardPath(rarity, card.imageKey),
      front: card.front,
      badge: card.badge,
      back: card.back,
      reward: rewardLabel(displayRound.value)
    }]
  }
  return spinDeck.value
})
const previewCardIndexes = [0, 3, 6, 9, 12, 15, 18, 19]
const spinDeck = computed(() => previewCardIndexes.map(index => creditWheelDeck[index]).map((item, index) => ({
  ...item,
  key: `${item.imageKey}-${index}`,
  flipped: false,
  image: wheelCardPath(item.rarity, item.imageKey)
})))
const fallbackImage = '/assets/zero-point-city/cards/wheel/credit/common/wheel_credit_01.png'

type WheelRarity = 'common' | 'good' | 'rare' | 'epic' | 'diamond' | 'rainbow'
type WheelCard = {
  imageKey: string
  rarity: WheelRarity
  front: string
  badge: string
  back: string
  reward?: string
}

const creditWheelDeck: WheelCard[] = [
  { imageKey: 'wheel_credit_01', rarity: 'common', front: '积分搬砖人', badge: '普通', back: '小数字也有尊严，堆起来就是我的高楼。' },
  { imageKey: 'wheel_credit_02', rarity: 'common', front: '余额旁听生', badge: '普通', back: '我还没发财，但已经开始认真听课。' },
  { imageKey: 'wheel_credit_03', rarity: 'common', front: '概率实习生', badge: '普通', back: '我不懂玄学，只会在角落偷偷加权。' },
  { imageKey: 'wheel_credit_04', rarity: 'common', front: '手气修理工', badge: '普通', back: '手气坏了别扔，我先拆开看看是不是松了。' },
  { imageKey: 'wheel_credit_05', rarity: 'common', front: '小奖收纳师', badge: '普通', back: '大奖还在路上，小奖先别乱跑。' },
  { imageKey: 'wheel_credit_06', rarity: 'common', front: '欧气旁观者', badge: '普通', back: '我站在旁边看着看着，突然被好运点名。' },
  { imageKey: 'wheel_credit_07', rarity: 'good', front: '积分炼金师', badge: '进阶', back: '小数字堆在一起，也会炼出一点明亮。' },
  { imageKey: 'wheel_credit_08', rarity: 'good', front: '前摇研究员', badge: '进阶', back: '别急，钱包正在进行一个很长的前摇。' },
  { imageKey: 'wheel_credit_09', rarity: 'good', front: '期待管理员', badge: '进阶', back: '期待不能太满，但可以先开个小窗口。' },
  { imageKey: 'wheel_credit_10', rarity: 'good', front: '玄学合规员', badge: '进阶', back: '我不迷信，我只是尊重概率的情绪价值。' },
  { imageKey: 'wheel_credit_11', rarity: 'good', front: '数字拾荒者', badge: '进阶', back: '别人看不上零碎，我看见一地未来。' },
  { imageKey: 'wheel_credit_12', rarity: 'rare', front: '概率叛逃者', badge: '稀有', back: '概率说不行，我说我只是路过一下规则。' },
  { imageKey: 'wheel_credit_13', rarity: 'rare', front: '反悔锦鲤', badge: '稀有', back: '我刚说不抽了，命运就开始装作没听见。' },
  { imageKey: 'wheel_credit_14', rarity: 'rare', front: '好运缓存员', badge: '稀有', back: '今天没爆发没关系，我先把欧气缓存起来。' },
  { imageKey: 'wheel_credit_15', rarity: 'rare', front: '小赚哲学家', badge: '稀有', back: '赚一点也是赚，宇宙没有规定快乐起步价。' },
  { imageKey: 'wheel_credit_16', rarity: 'epic', front: '冷静暴富学家', badge: '史诗', back: '先别激动，财富正在进行系统更新。' },
  { imageKey: 'wheel_credit_17', rarity: 'epic', front: '命中率驯兽师', badge: '史诗', back: '概率有点野，但我带了零食和耐心。' },
  { imageKey: 'wheel_credit_18', rarity: 'epic', front: '奇迹测试员', badge: '史诗', back: '本次奇迹可能不稳定，但值得灰度发布。' },
  { imageKey: 'wheel_credit_19', rarity: 'diamond', front: '隐藏欧皇', badge: '钻石', back: '我没有炫耀，我只是被好运误伤得很自然。' },
  { imageKey: 'wheel_credit_20', rarity: 'rainbow', front: '奖池破壁人', badge: '彩虹', back: '门没开，我就先和门聊成了熟人。' }
]

onMounted(() => {
  void initializeCreditLottery()
})

watch(() => authStore.user?.id, (nextUserId, previousUserId) => {
  if (isSameLotteryUserId(nextUserId, previousUserId)) return
  invalidateCreditLotteryAsyncState()
  resetOperationRetryState()
  pendingOperation.value = null
  pendingOperationUserId = null
  session.value = null
  resultRevealed.value = false
  loadError.value = ''
  confirming.value = false
  loading.value = true
  void initializeCreditLottery()
})

async function initializeCreditLottery() {
  const operationUserId = authStore.user?.id
  if (!isCreditUserContextCurrent(operationUserId)) return
  const storedOperation = readPendingCreditLotteryOperation(operationUserId)
  if (storedOperation) {
    pendingOperation.value = storedOperation
    pendingOperationUserId = operationUserId
    loading.value = false
    confirming.value = true
    await recoverSession(false, true, operationUserId)
    return
  }
  await loadActiveForUser(operationUserId)
}

onBeforeUnmount(() => {
  componentActive = false
  invalidateCreditLotteryAsyncState()
  resetOperationRetryState()
})

async function loadActive() {
  await loadActiveForUser(authStore.user?.id)
}

async function loadActiveForUser(operationUserId: LotteryUserId) {
  const requestSequence = ++loadSequence
  loading.value = true
  loadError.value = ''
  confirming.value = false
  startConfirmationTimer()
  try {
    const restored = await userAPI.getActiveCreditLotterySession()
    if (!isCreditLoadCurrent(requestSequence, operationUserId)) return
    if (restored !== null) assertCreditLotterySession(restored, operationUserId)
    session.value = restored
    resultRevealed.value = session.value?.status === 'settled'
    patchBalance(session.value, operationUserId)
    confirming.value = false
  } catch (error) {
    if (!isCreditLoadCurrent(requestSequence, operationUserId)) return
    confirming.value = false
    loadError.value = extractApiErrorMessage(error, '加载积分抽奖失败，请重新加载。')
    appStore.showError(loadError.value)
  } finally {
    if (requestSequence === loadSequence && isCreditUserContextCurrent(operationUserId)) {
      clearConfirmationTimer()
      loading.value = false
    }
  }
}

async function recoverSession(showFailure = true, invalidatePendingPostOnSuccess = true, operationUserId: LotteryUserId = authStore.user?.id) {
  if (!isCreditUserContextCurrent(operationUserId)) return
  if (recoveryLoading.value) return
  const requestSequence = ++recoverySequence
  const requestActionSequence = actionSequence
  recoveryLoading.value = true
  const operation = pendingCreditLotteryOperationForUser(operationUserId)
  if (operation) {
    pendingOperation.value = operation
    pendingOperationUserId = operationUserId
  }
  try {
    const restored = operation
      ? await userAPI.getCreditLotteryOperation(operation.operationId)
      : await userAPI.getActiveCreditLotterySession()
    if (!isCreditRecoveryCurrent(requestSequence, requestActionSequence, operationUserId)) return
    if (restored) {
      assertCreditLotterySession(restored, operationUserId, operation)
      if (invalidatePendingPostOnSuccess && postInFlight.value) invalidateActiveAction()
      session.value = restored
      resultRevealed.value = restored.status === 'settled' || operation?.action === 'settle'
      patchBalance(restored, operationUserId)
      clearConfirmationTimer()
      loading.value = false
      if (!postInFlight.value) busy.value = false
      confirming.value = false
      if (operation) {
        clearPendingCreditLotteryOperationState(operationUserId, operation.operationId)
      }
      resetOperationRetryState()
      emit('refresh')
    } else if (!operation) {
      // A read-only refresh that returns no active session is a confirmed idle
      // state. It must release the six-second safety screen instead of trapping
      // the user behind an endless "confirming" state.
      session.value = null
      resultRevealed.value = false
      clearConfirmationTimer()
      loading.value = false
      loadError.value = ''
      busy.value = false
      confirming.value = false
    } else if (showFailure) {
      appStore.showError('操作查询返回了空结果，暂时不能确认是否受理；请继续刷新，系统不会开放重复提交。')
    }
  } catch (error) {
    if (!isCreditRecoveryCurrent(requestSequence, requestActionSequence, operationUserId)) return
    if (operation && isLotteryOperationNotFoundError(error, 'CREDIT_LOTTERY_OPERATION_NOT_FOUND')) {
      markOperationNotFound(operation)
      if (showFailure) appStore.showError(canRetryOperation.value
        ? '服务器尚未登记本次操作，可点击“重试原操作”安全续办。'
        : (postInFlight.value
            ? '服务器尚未登记本次操作，但原请求仍在等待返回；当前只继续查询。'
            : '服务器尚未登记本次操作，安全窗口结束后可用原操作号重试。'))
    } else if (showFailure) {
      appStore.showError(extractApiErrorMessage(error, '刷新抽奖结果失败'))
    }
  } finally {
    if (requestSequence === recoverySequence) recoveryLoading.value = false
  }
}

async function handleConfirmationAction() {
  if (canRetryOperation.value) {
    await retryPendingOperation()
    return
  }
  await recoverSession()
}

async function retryPendingOperation() {
  const operationUserId = authStore.user?.id
  const operation = pendingCreditLotteryOperationForUser(operationUserId)
  if (!operation || !canRetryOperation.value) return
  if (operation.action !== 'start' && (!operation.sessionId || !operation.expectedRound)) {
    appStore.showError('原操作缺少会话信息，暂时只能继续查询结果。')
    return
  }

  const retriedOperation: PendingCreditLotteryOperation = { ...operation, lastAttemptAt: Date.now() }
  pendingOperation.value = retriedOperation
  pendingOperationUserId = operationUserId
  storePendingCreditLotteryOperation(operationUserId, retriedOperation)
  resetOperationRetryState()
  await runAction(retriedOperation, operationUserId, async () => {
    if (retriedOperation.action === 'start') {
      return userAPI.createCreditLotterySession(retriedOperation.operationId)
    }
    if (retriedOperation.action === 'continue') {
      return userAPI.continueCreditLotterySession(
        retriedOperation.sessionId!,
        retriedOperation.expectedRound!,
        retriedOperation.operationId
      )
    }
    return userAPI.settleCreditLotterySession(
      retriedOperation.sessionId!,
      retriedOperation.expectedRound!,
      retriedOperation.operationId
    )
  }, (restored) => {
    if (retriedOperation.action === 'settle') {
      applySettleResult(restored, operationUserId)
      return
    }
    session.value = restored
    resultRevealed.value = restored.status === 'settled'
    patchBalance(restored, operationUserId)
  })
}

async function startSession() {
  if (startDisabled.value) return
  const operationUserId = authStore.user?.id
  const operation = beginOperation('start', undefined, operationUserId)
  await runAction(operation, operationUserId, async () => {
    const [nextSession] = await Promise.all([
      userAPI.createCreditLotterySession(operation.operationId),
      waitForSpin()
    ])
    return nextSession
  }, (nextSession) => {
    session.value = nextSession
    resultRevealed.value = false
    patchBalance(nextSession, operationUserId)
  })
}

async function continueSession() {
  if (!session.value) return
  const operationUserId = authStore.user?.id
  const operation = beginOperation('continue', session.value, operationUserId)
  await runAction(operation, operationUserId, async () => {
    const [nextSession] = await Promise.all([
      userAPI.continueCreditLotterySession(operation.sessionId!, operation.expectedRound!, operation.operationId),
      waitForSpin()
    ])
    return nextSession
  }, (nextSession) => {
    session.value = nextSession
    resultRevealed.value = false
    patchBalance(nextSession, operationUserId)
  })
}

async function settleSession() {
  if (!session.value) return
  const operationUserId = authStore.user?.id
  const operation = beginOperation('settle', session.value, operationUserId)
  await runAction(
    operation,
    operationUserId,
    () => userAPI.settleCreditLotterySession(operation.sessionId!, operation.expectedRound!, operation.operationId),
    (settledSession) => {
      applySettleResult(settledSession, operationUserId)
    }
  )
}

function completeSession() {
  resetSession()
  emit('close')
}

async function runAction(
  operation: PendingCreditLotteryOperation,
  operationUserId: LotteryUserId,
  request: () => Promise<CreditLotterySession>,
  applyResult: (result: CreditLotterySession) => void
) {
  if (busy.value) return
  const sequence = ++actionSequence
  busy.value = true
  postInFlight.value = true
  confirming.value = false
  startConfirmationTimer()
  try {
    const result = await request()
    if (!isCreditActionCurrent(sequence, operationUserId)) return
    assertCreditLotterySession(result, operationUserId, operation)
    applyResult(result)
    clearPendingCreditLotteryOperationState(operationUserId, operation.operationId)
    resetOperationRetryState()
    confirming.value = false
    emit('refresh')
  } catch (error) {
    if (!isCreditActionCurrent(sequence, operationUserId)) return
    if (!isUncertainRequestError(error)) {
      // Validation, quota and balance failures are explicit rejections. Keeping
      // their operation marker would permanently disable the draw button even
      // though the server did not accept the operation.
      clearPendingCreditLotteryOperationState(operationUserId, operation.operationId)
      resetOperationRetryState()
      confirming.value = false
      appStore.showError(extractApiErrorMessage(error, '抽奖请求未被受理，请确认次数和积分后重试'))
      return
    }

    confirming.value = true
    await recoverSession(false, false, operationUserId)
    if (confirming.value) {
      appStore.showError(extractApiErrorMessage(error, '结果暂未确认，请刷新结果，暂勿重复抽取'))
    }
  } finally {
    if (isCreditActionCurrent(sequence, operationUserId)) {
      clearConfirmationTimer()
      postInFlight.value = false
      busy.value = false
    }
  }
}

function isUncertainRequestError(error: unknown) {
  if (!error || typeof error !== 'object') return true
  const value = error as { status?: number; response?: { status?: number } }
  const statusCode = Number(value.status ?? value.response?.status ?? 0)
  return statusCode === 0 || statusCode === 408 || statusCode === 429 || statusCode >= 500
}

function startConfirmationTimer() {
  clearConfirmationTimer()
  confirmationTimer = window.setTimeout(() => {
    confirming.value = true
  }, 6000)
}

function clearConfirmationTimer() {
  if (confirmationTimer === null) return
  window.clearTimeout(confirmationTimer)
  confirmationTimer = null
}

function markOperationNotFound(operation: PendingCreditLotteryOperation) {
  operationNotFound.value = true
  clearOperationRetryTimer()
  const remaining = lotteryOperationRetryRemainingMs(operation)
  if (remaining === 0) {
    operationRetryReady.value = true
    return
  }
  operationRetryReady.value = false
  operationRetryTimer = window.setTimeout(() => {
    operationRetryTimer = null
    operationRetryReady.value = true
  }, remaining)
}

function resetOperationRetryState() {
  clearOperationRetryTimer()
  operationNotFound.value = false
  operationRetryReady.value = false
}

function clearOperationRetryTimer() {
  if (operationRetryTimer === null) return
  window.clearTimeout(operationRetryTimer)
  operationRetryTimer = null
}

function invalidateActiveAction() {
  actionSequence += 1
  clearConfirmationTimer()
  postInFlight.value = false
  busy.value = false
}

function invalidateCreditLotteryAsyncState() {
  invalidateActiveAction()
  recoverySequence += 1
  loadSequence += 1
  recoveryLoading.value = false
  loading.value = false
}

function isSameLotteryUserId(left: LotteryUserId, right: LotteryUserId) {
  return String(left ?? 'anonymous') === String(right ?? 'anonymous')
}

function assertCreditLotterySession(
  value: unknown,
  operationUserId: LotteryUserId,
  operation?: PendingCreditLotteryOperation | null
): asserts value is CreditLotterySession {
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    throw new Error('credit lottery response is empty or malformed')
  }

  const candidate = value as Partial<CreditLotterySession>
  if (!isPositiveInteger(candidate.id)
    || !isPositiveInteger(candidate.user_id)
    || !isSameLotteryUserId(candidate.user_id, operationUserId)
    || typeof candidate.session_date !== 'string'
    || candidate.session_date.trim() === ''
    || (candidate.status !== 'active' && candidate.status !== 'settled')
    || !isFiniteNumber(candidate.cost_credit)
    || candidate.cost_credit < 0
    || !isPositiveInteger(candidate.max_rounds)
    || !isPositiveInteger(candidate.current_round)
    || candidate.current_round > candidate.max_rounds) {
    throw new Error('credit lottery response is missing required session fields')
  }

  if (operation?.sessionId !== undefined && candidate.id !== operation.sessionId) {
    throw new Error('credit lottery response session mismatch')
  }
  if (operation?.action === 'settle'
    && candidate.status === 'active'
    && (!isPositiveInteger(operation.expectedRound) || candidate.current_round <= operation.expectedRound)) {
    throw new Error('credit lottery settle response did not return a newer authoritative round')
  }

  const round = candidate.status === 'settled' ? candidate.final_result : candidate.current_result
  if (!isCreditLotteryRoundResult(round, candidate.current_round)) {
    throw new Error('credit lottery response is missing the effective round')
  }
  if (candidate.balance_after !== undefined && !isFiniteNumber(candidate.balance_after)) {
    throw new Error('credit lottery response contains an invalid balance')
  }
  if (candidate.credit_balance_after !== undefined && !isFiniteNumber(candidate.credit_balance_after)) {
    throw new Error('credit lottery response contains an invalid credit balance')
  }
}

function isCreditLotteryRoundResult(value: unknown, expectedRound: number): value is CreditLotteryRoundResult {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return false
  const candidate = value as Partial<CreditLotteryRoundResult>
  return candidate.round_no === expectedRound
    && typeof candidate.reward_asset === 'string'
    && candidate.reward_asset.trim() !== ''
    && isFiniteNumber(candidate.reward_amount)
}

function isPositiveInteger(value: unknown): value is number {
  return typeof value === 'number' && Number.isInteger(value) && value > 0
}

function isFiniteNumber(value: unknown): value is number {
  return typeof value === 'number' && Number.isFinite(value)
}

function isCreditUserContextCurrent(operationUserId: LotteryUserId) {
  return componentActive && isSameLotteryUserId(operationUserId, authStore.user?.id)
}

function isCreditActionCurrent(sequence: number, operationUserId: LotteryUserId) {
  return sequence === actionSequence && isCreditUserContextCurrent(operationUserId)
}

function isCreditRecoveryCurrent(requestSequence: number, requestActionSequence: number, operationUserId: LotteryUserId) {
  return requestSequence === recoverySequence
    && requestActionSequence === actionSequence
    && isCreditUserContextCurrent(operationUserId)
}

function isCreditLoadCurrent(sequence: number, operationUserId: LotteryUserId) {
  return sequence === loadSequence && isCreditUserContextCurrent(operationUserId)
}

function pendingCreditLotteryOperationForUser(operationUserId: LotteryUserId) {
  if (pendingOperation.value && isSameLotteryUserId(pendingOperationUserId, operationUserId)) {
    return pendingOperation.value
  }
  return readPendingCreditLotteryOperation(operationUserId)
}

function clearPendingCreditLotteryOperationState(operationUserId: LotteryUserId, operationId: string) {
  clearPendingCreditLotteryOperation(operationUserId, operationId)
  if (pendingOperation.value?.operationId === operationId
    && isSameLotteryUserId(pendingOperationUserId, operationUserId)) {
    pendingOperation.value = null
    pendingOperationUserId = null
  }
}

function patchBalance(value: CreditLotterySession | null, operationUserId: LotteryUserId) {
  if (!value || !isCreditUserContextCurrent(operationUserId)) return
  const hasBalanceAfter = typeof value.balance_after === 'number'
  const hasCreditAfter = typeof value.credit_balance_after === 'number'
  if (!hasBalanceAfter && !hasCreditAfter) return
  authStore.patchUserBalance(
    hasBalanceAfter ? value.balance_after! : authStore.user?.balance ?? 0,
    hasCreditAfter ? value.credit_balance_after! : authStore.user?.credit_balance ?? 0
  )
}

function applySettleResult(value: CreditLotterySession, operationUserId: LotteryUserId) {
  patchBalance(value, operationUserId)
  if (value.status === 'settled') {
    resetSession()
    return
  }
  session.value = value
  resultRevealed.value = true
}

function resetSession() {
  session.value = null
  resultRevealed.value = false
  confirming.value = false
}

function beginOperation(action: CreditLotteryOperationAction, currentSession: CreditLotterySession | undefined, operationUserId: LotteryUserId) {
  resetOperationRetryState()
  const operation: PendingCreditLotteryOperation = {
    operationId: makeIdempotencyKey(action),
    action,
    sessionId: currentSession?.id,
    expectedRound: currentSession?.current_round,
    createdAt: Date.now()
  }
  pendingOperation.value = operation
  pendingOperationUserId = operationUserId
  storePendingCreditLotteryOperation(operationUserId, operation)
  return operation
}

function revealResult() {
  if (busy.value || !session.value || resultRevealed.value) return
  resultRevealed.value = true
}

function waitForSpin() {
  return new Promise(resolve => window.setTimeout(resolve, spinDurationMs))
}

function rewardLabel(round: CreditLotteryRoundResult | null) {
  if (!round) return '等待开奖'
  return `积分 +${formatNumber(round.reward_amount)}`
}

function cardName(card: CheckinCollectibleCard) {
  return zeroCityCardDisplayName(card.card_key)
}

function formatNumber(value: number) {
  return Number(value || 0).toFixed(0)
}

function jackpotSessionLabel(value: CreditLotterySession) {
  const labels = jackpotPayoutLabels(value.jackpot_payouts)
  if (labels.length > 0) return labels.join(' / ')
  if (value.jackpot_winner_amount) return `+${Number(value.jackpot_winner_amount).toFixed(2)}`
  return ''
}

function jackpotPayoutLabels(payouts: JackpotPayout[] | undefined) {
  return (payouts || [])
    .filter(payout => Number(payout.winner_amount || 0) > 0)
    .map(payout => `${jackpotAssetLabel(payout.reward_asset)} +${Number(payout.winner_amount || 0).toFixed(2)}`)
}

function jackpotAssetLabel(asset: string) {
  return asset === 'balance' ? '余额' : '积分'
}

function rewardRarity(value: number): WheelRarity {
  if (value >= 100) return 'diamond'
  if (value >= 50) return 'epic'
  if (value >= 20) return 'rare'
  if (value >= 10) return 'good'
  return 'common'
}

function wheelCardPath(rarity: WheelRarity, key: string) {
  return `/assets/zero-point-city/cards/wheel/credit/${rarity}/${key}.png`
}

function handleImageFallback(event: Event, fallbackSrc: string) {
  const target = event.target as HTMLImageElement | null
  if (!target || target.dataset.fallbackApplied === '1') return
  target.dataset.fallbackApplied = '1'
  target.onerror = null
  target.src = fallbackSrc
}

function makeIdempotencyKey(action: string) {
  const suffix = typeof crypto !== 'undefined' && 'randomUUID' in crypto
    ? crypto.randomUUID()
    : `${Date.now()}-${Math.random().toString(16).slice(2)}`
  return `${action}-${suffix}`
}
</script>

<style scoped>
.credit-lottery {
  --gift-raise: 14px 14px 30px var(--zc-shadow-dark), -14px -14px 30px var(--zc-shadow-light);
  --gift-raise-sm: 7px 7px 16px var(--zc-shadow-dark), -7px -7px 16px var(--zc-shadow-light);
  --gift-inset: inset 6px 6px 14px var(--zc-shadow-dark), inset -6px -6px 14px var(--zc-shadow-light);
  --result-card-width: min(150px, 54vw);
}

.mini-draw {
  padding: 8px;
  border: 0;
  border-radius: 18px;
  background: var(--zc-bg);
  box-shadow: var(--gift-raise);
  transition: transform .24s ease;
}

.credit-lottery.revealed {
  display: flex;
  flex-direction: column;
}

.credit-lottery.revealed .lottery-actions {
  margin-top: 8px;
}

.credit-lottery.revealed .mini-slot {
  flex: 0 0 auto;
  height: auto;
  min-height: 0;
  padding: 8px;
}

.mini-slot {
  position: relative;
  width: 100%;
  height: 128px;
  overflow: hidden;
  border: 0;
  padding: 0;
  border-radius: 20px;
  color: inherit;
  background: var(--zc-bg);
  box-shadow: var(--gift-inset);
  font: inherit;
}

.mini-slot:disabled { opacity: 1; cursor: default; }

.mini-slot::before,
.mini-slot::after {
  content: "";
  position: absolute;
  top: 0;
  bottom: 0;
  z-index: 2;
  width: 36px;
  pointer-events: none;
}

.mini-slot::before {
  left: 0;
  background: linear-gradient(90deg, var(--zc-bg), transparent);
}

.mini-slot::after {
  right: 0;
  background: linear-gradient(270deg, var(--zc-bg), transparent);
}

.mini-slot-track {
  height: 100%;
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 0 36px;
  transform: translateX(0);
}

.mini-draw.ready .mini-slot-track,
.mini-draw.revealed .mini-slot-track {
  align-items: center;
  justify-content: center;
  padding: 0;
}

.mini-draw.revealed .mini-slot-track {
  position: static;
  display: grid;
  place-items: center;
  height: auto;
  min-height: 0;
}

.mini-draw.ready .mini-slot {
  height: 174px;
}

.mini-draw.spinning .mini-slot-track {
  animation: miniSlotSpin .72s linear infinite;
}

@keyframes miniSlotSpin {
  from { transform: translateX(0); }
  to { transform: translateX(-420px); }
}

.mini-slot-card {
  position: relative;
  flex: 0 0 62px;
  height: 93px;
  border-radius: 18px;
  transform-style: preserve-3d;
  transform-origin: center center;
  transition: transform .58s cubic-bezier(.16, 1, .3, 1), box-shadow .25s ease;
  box-shadow: 0 10px 22px rgba(116, 75, 10, .12);
}

.mini-slot-card::before {
  content: "";
  position: absolute;
  inset: -1px;
  z-index: -1;
  border-radius: 19px;
  background: linear-gradient(160deg, rgba(255, 255, 255, .96), rgba(230, 170, 68, .26), rgba(255, 255, 255, .72));
}

.mini-slot-card.flipped,
.mini-draw.revealed .mini-slot-card {
  transform: rotateY(180deg);
}

.mini-draw.ready .mini-slot-card {
  flex-basis: 104px;
  height: auto;
  aspect-ratio: 2 / 3;
}

.mini-draw.revealed .mini-slot-card {
  flex: 0 0 var(--result-card-width);
  width: var(--result-card-width);
  height: auto;
  aspect-ratio: 2 / 3;
}

.mini-draw.revealed .mini-slot-card {
  transform: rotateY(180deg);
}

.card-face {
  position: absolute;
  inset: 0;
  display: grid;
  place-items: center;
  overflow: hidden;
  border-radius: 18px;
  border: 1px solid rgba(200, 145, 47, .24);
  background: #fff2cf;
  backface-visibility: hidden;
}

.card-face::before {
  content: "";
  position: absolute;
  inset: 6px;
  z-index: 2;
  border-radius: 12px;
  border: 1px solid rgba(255, 255, 255, .58);
  pointer-events: none;
}

.card-face::after {
  content: "";
  position: absolute;
  inset: 0;
  z-index: 1;
  background: linear-gradient(180deg, rgba(255, 250, 240, .1), transparent 42%, rgba(55, 36, 8, .22));
  pointer-events: none;
}

.card-back {
  padding: 0;
  color: var(--zc-muted);
  transform: rotateY(180deg);
  font-size: 12px;
  font-weight: 800;
  line-height: 1.55;
  text-align: center;
}

.wheel-card-image {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.wheel-card-title {
  position: absolute;
  left: 7px;
  right: 7px;
  top: 8px;
  z-index: 3;
  display: block;
  padding: 4px 5px;
  border: 0;
  border-radius: 10px;
  color: var(--zc-text-strong);
  background: var(--zc-bg);
  font-size: 9px;
  font-weight: 950;
  line-height: 1.12;
  text-align: center;
  box-shadow: var(--gift-inset);
}

.wheel-card-heading {
  position: absolute;
  top: 7px;
  left: 7px;
  right: 7px;
  z-index: 3;
  display: grid;
  justify-items: center;
  gap: 3px;
  padding: 5px 6px;
  border-radius: 10px;
  background: rgba(255, 250, 240, .88);
  box-shadow: 0 6px 12px rgba(54, 38, 13, .08);
  backdrop-filter: blur(7px);
}

.wheel-card-heading b {
  max-width: 100%;
  overflow: hidden;
  color: var(--zc-text-strong);
  font-size: 10px;
  font-weight: 950;
  line-height: 1.12;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.wheel-card-footer {
  position: absolute;
  left: 7px;
  right: 7px;
  bottom: 8px;
  z-index: 3;
  display: grid;
  gap: 4px;
  padding: 6px;
  border-radius: 10px;
  background: rgba(255, 250, 240, .9);
  box-shadow: 0 6px 12px rgba(54, 38, 13, .08);
  backdrop-filter: blur(7px);
}

.wheel-card-line {
  display: -webkit-box;
  max-height: 38px;
  overflow: hidden;
  color: var(--zc-muted);
  font-size: 9px;
  font-weight: 850;
  line-height: 1.28;
  text-align: center;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
}

.wheel-card-footer strong {
  color: #9a650c;
  font-size: 9px;
  font-weight: 950;
  line-height: 1.1;
}

.rarity-badge {
  display: inline-flex;
  padding: 2px 6px;
  border: 0;
  border-radius: 999px;
  background: var(--zc-bg);
  color: var(--zc-accent);
  font-size: 8px;
  font-weight: 950;
  letter-spacing: .04em;
  line-height: 1.1;
  white-space: nowrap;
  box-shadow: var(--gift-inset);
}

.mini-slot-card.good .card-front { border-color: rgba(72, 150, 70, .25); }
.mini-slot-card.rare .card-front { border-color: rgba(70, 128, 210, .28); }
.mini-slot-card.epic .card-front { border-color: rgba(150, 80, 190, .3); }
.mini-slot-card.diamond .card-front { color: #175c74; border-color: rgba(21, 147, 185, .36); }
.mini-slot-card.rainbow .card-front { color: #7b3cc4; border-color: rgba(151, 89, 220, .36); }
.mini-slot-card.good .card-back { color: #2f6d34; background: linear-gradient(180deg, #fbfff8, #ecf9e5); border-color: rgba(72, 150, 70, .25); }
.mini-slot-card.rare .card-back { color: #275f9d; background: linear-gradient(180deg, #f8fcff, #e5f1ff); border-color: rgba(70, 128, 210, .28); }
.mini-slot-card.epic .card-back { color: #7d43a8; background: linear-gradient(180deg, #fffaff, #f3e3ff); border-color: rgba(150, 80, 190, .3); }
.mini-slot-card.diamond .card-back { color: #175c74; background: linear-gradient(135deg, #f7fdff, #dff7ff 52%, #fff); border-color: rgba(21, 147, 185, .36); }
.mini-slot-card.rainbow .card-back { color: #7b3cc4; background: linear-gradient(135deg, #fff7d8, #ffd8ef 42%, #dfe7ff 78%, #fff); border-color: rgba(151, 89, 220, .36); }

.mini-blessing {
  margin-top: 8px;
  color: var(--zc-text-strong);
  font-size: 15px;
  font-weight: 900;
  text-align: center;
}

.mini-copy {
  min-height: 30px;
  margin-top: 4px;
  color: var(--zc-muted);
  font-size: 11px;
  line-height: 1.35;
  text-align: center;
}

.mini-draw.revealed .mini-copy {
  min-height: 18px;
}

.mini-draw.revealed .mini-pills {
  margin-top: 4px;
}

.mini-start,
.mini-action {
  border: 0;
  border-radius: 15px;
  color: #fff;
  background: linear-gradient(135deg, #1d1d1f, #3b3b42);
  box-shadow: 0 12px 24px rgba(0, 0, 0, .15);
  font-size: 12px;
  font-weight: 900;
  cursor: pointer;
}

.mini-start {
  width: 100%;
  margin-top: 8px;
  padding: 9px 12px;
}

.lottery-actions {
  display: flex;
  gap: 8px;
  margin-top: 8px;
}

.lottery-actions .mini-start {
  flex: 1;
  width: auto;
  margin-top: 0;
}

.mini-action {
  flex: 1;
  min-height: 36px;
  padding: 8px 10px;
}

.mini-action.secondary {
  background: rgba(245, 245, 247, .9);
  color: var(--zc-muted);
  box-shadow: none;
}

.confirmation-panel {
  display: grid;
  gap: 7px;
  margin-top: 8px;
  padding: 10px;
  border-radius: 14px;
  color: #7c3f00;
  background: rgba(255, 244, 221, .94);
  border: 1px solid rgba(214, 137, 18, .2);
  font-size: 10px;
  line-height: 1.42;
  text-align: center;
}

.confirmation-panel b { color: #8d4b00; font-size: 12px; font-weight: 950; }
.confirmation-panel .mini-action { width: 100%; }

.overwrite-warning {
  margin: 7px 0 0;
  color: #a13d2d;
  font-size: 10px;
  font-weight: 850;
  line-height: 1.4;
  text-align: center;
}

.mini-start:disabled,
.mini-action:disabled {
  opacity: .58;
  cursor: wait;
}

.mini-pills {
  display: flex;
  justify-content: center;
  gap: 5px;
  flex-wrap: wrap;
  margin-top: 8px;
}

.mini-pills span {
  padding: 4px 7px;
  border: 0;
  border-radius: 999px;
  background: var(--zc-bg);
  box-shadow: var(--gift-inset);
  color: #4b5563;
  font-size: 10px;
  font-weight: 800;
}

.mini-rules {
  display: grid;
  gap: 4px;
  margin-top: 12px;
  padding: 8px 10px;
  border-radius: 14px;
  color: var(--zc-muted);
  background: var(--zc-bg);
  box-shadow: var(--gift-inset);
  font-size: 9px;
  line-height: 1.35;
}

.mini-rules b {
  color: var(--zc-text-strong);
  font-size: 10px;
  font-weight: 950;
}

.mini-rules span {
  display: block;
}

@media (prefers-reduced-motion: reduce) {
  .mini-slot-track,
  .mini-slot-card,
  .mini-draw { animation: none !important; transition-duration: .01ms !important; }
}
</style>
