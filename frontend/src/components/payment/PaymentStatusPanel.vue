<template>
  <div class="space-y-4">
    <!-- ═══ Terminal States: show result, user clicks to return ═══ -->

    <!-- Success -->
    <template v-if="outcome === 'success'">
      <div class="card p-6">
        <div class="flex flex-col items-center space-y-4 py-4">
          <div class="flex h-16 w-16 items-center justify-center rounded-full bg-green-100 dark:bg-green-900/30">
            <Icon name="check" size="lg" class="text-green-500" />
          </div>
          <p class="text-lg font-bold text-gray-900 dark:text-white">{{ props.orderType === 'subscription' ? t('payment.result.subscriptionSuccess') : t('payment.result.success') }}</p>
          <div v-if="paidOrder" class="w-full rounded-xl bg-gray-50 p-4 dark:bg-dark-800">
            <div class="space-y-2 text-sm">
              <div class="flex justify-between">
                <span class="text-gray-500 dark:text-gray-400">{{ t('payment.orders.orderId') }}</span>
                <span class="font-medium text-gray-900 dark:text-white">#{{ paidOrder.id }}</span>
              </div>
              <div v-if="paidOrder.out_trade_no" class="flex justify-between">
                <span class="text-gray-500 dark:text-gray-400">{{ t('payment.orders.orderNo') }}</span>
                <span class="font-medium text-gray-900 dark:text-white">{{ paidOrder.out_trade_no }}</span>
              </div>
              <div class="flex justify-between">
                <span class="text-gray-500 dark:text-gray-400">{{ t('payment.orders.amount') }}</span>
                <span class="font-medium text-gray-900 dark:text-white">{{ creditedAmountSymbol }}{{ paidOrder.amount.toFixed(2) }}</span>
              </div>
              <div class="flex justify-between">
                <span class="text-gray-500 dark:text-gray-400">{{ t('payment.orders.payAmount') }}</span>
                <span class="font-medium text-gray-900 dark:text-white">{{ formatGatewayAmount(paidOrder.pay_amount, paidOrder.currency) }}</span>
              </div>
            </div>
          </div>
          <button class="btn btn-primary" @click="handleDone">{{ t('common.confirm') }}</button>
        </div>
      </div>
    </template>

    <!-- Cancelled -->
    <template v-else-if="outcome === 'cancelled'">
      <div class="card p-6">
        <div class="flex flex-col items-center space-y-4 py-4">
          <div class="flex h-16 w-16 items-center justify-center rounded-full bg-gray-100 dark:bg-dark-700">
            <svg class="h-8 w-8 text-gray-400 dark:text-gray-500" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
              <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
            </svg>
          </div>
          <p class="text-lg font-bold text-gray-900 dark:text-white">{{ t('payment.qr.cancelled') }}</p>
          <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('payment.qr.cancelledDesc') }}</p>
          <button class="btn btn-primary" @click="handleDone">{{ t('common.confirm') }}</button>
        </div>
      </div>
    </template>

    <!-- Expired / Failed -->
    <template v-else-if="outcome === 'expired'">
      <div class="card p-6">
        <div class="flex flex-col items-center space-y-4 py-4">
          <div class="flex h-16 w-16 items-center justify-center rounded-full bg-orange-100 dark:bg-orange-900/30">
            <svg class="h-8 w-8 text-orange-500" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
              <path stroke-linecap="round" stroke-linejoin="round" d="M12 6v6h4.5m4.5 0a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
          </div>
          <p class="text-lg font-bold text-gray-900 dark:text-white">{{ t('payment.qr.expired') }}</p>
          <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('payment.qr.expiredDesc') }}</p>
          <button class="btn btn-primary" @click="handleDone">{{ t('common.confirm') }}</button>
        </div>
      </div>
    </template>

    <!-- ═══ Active States: QR or Popup waiting ═══ -->

    <!-- Crypto Transfer Mode -->
    <template v-else-if="isCryptoPayment">
      <div class="card overflow-hidden border border-teal-100 bg-white shadow-sm dark:border-teal-900/40 dark:bg-dark-900">
        <div class="bg-gradient-to-br from-slate-950 via-slate-900 to-teal-950 p-6 text-white">
          <div class="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
            <div>
              <p class="text-xs font-semibold uppercase tracking-[0.24em] text-teal-200/80">{{ t('payment.crypto.kicker') }}</p>
              <h3 class="mt-2 text-2xl font-bold">{{ t('payment.crypto.title') }}</h3>
              <p class="mt-2 max-w-2xl text-sm leading-relaxed text-slate-300">{{ t('payment.crypto.description') }}</p>
            </div>
            <span class="inline-flex shrink-0 items-center rounded-full bg-teal-300 px-3 py-1 text-xs font-bold text-slate-950">
              {{ cryptoNetworkLabel }}
            </span>
          </div>
          <div class="mt-5 grid gap-3 sm:grid-cols-3">
            <div class="rounded-2xl bg-white/10 p-4">
              <p class="text-xs text-slate-300">{{ t('payment.crypto.payExactly') }}</p>
              <p class="mt-1 text-2xl font-black tabular-nums text-teal-100">{{ cryptoPayAmountLabel }}</p>
            </div>
            <div class="rounded-2xl bg-white/10 p-4">
              <p class="text-xs text-slate-300">{{ t('payment.crypto.creditAmount') }}</p>
              <p class="mt-1 text-lg font-bold tabular-nums">{{ cryptoBalanceLabel }}</p>
            </div>
            <div class="rounded-2xl bg-white/10 p-4">
              <p class="text-xs text-slate-300">{{ t('payment.crypto.rate') }}</p>
              <p class="mt-1 text-lg font-bold">1 USDT = {{ cryptoRateLabel }}</p>
            </div>
          </div>
        </div>
        <div class="space-y-4 p-5">
          <div class="rounded-2xl border border-slate-200 bg-slate-50 p-4 dark:border-dark-700 dark:bg-dark-800">
            <div class="flex items-center justify-between gap-3">
              <p class="text-sm font-semibold text-slate-700 dark:text-slate-200">{{ t('payment.crypto.walletAddress') }}</p>
              <button class="rounded-lg bg-white px-3 py-1.5 text-xs font-bold text-slate-700 shadow-sm transition hover:bg-slate-100 dark:bg-dark-700 dark:text-slate-100" @click="copyToClipboard(cryptoDetails?.wallet_address || '')">
                {{ t('common.copy') }}
              </button>
            </div>
            <p class="mt-3 break-all rounded-xl bg-white p-3 font-mono text-sm font-semibold text-slate-900 dark:bg-dark-900 dark:text-slate-100">{{ cryptoDetails?.wallet_address }}</p>
          </div>
          <div class="grid gap-3 sm:grid-cols-2">
            <div class="rounded-2xl border border-slate-200 p-4 text-sm dark:border-dark-700">
              <p class="text-xs text-slate-500 dark:text-slate-400">{{ t('payment.crypto.orderNo') }}</p>
              <p class="mt-1 break-all font-mono font-semibold text-slate-900 dark:text-slate-100">{{ cryptoDetails?.out_trade_no || props.orderId }}</p>
            </div>
            <div class="rounded-2xl border border-slate-200 p-4 text-sm dark:border-dark-700">
              <p class="text-xs text-slate-500 dark:text-slate-400">{{ t('payment.crypto.minTransfer') }}</p>
              <p class="mt-1 font-semibold text-slate-900 dark:text-slate-100">{{ cryptoMinAmountLabel }}</p>
            </div>
          </div>
          <div class="rounded-2xl bg-amber-50 p-4 text-sm leading-relaxed text-amber-800 dark:bg-amber-950/30 dark:text-amber-200">
            {{ t('payment.crypto.precisionWarning') }}
          </div>
          <div class="rounded-2xl border border-teal-100 bg-teal-50 p-4 dark:border-teal-900/40 dark:bg-teal-950/20">
            <p class="text-sm font-semibold text-teal-900 dark:text-teal-100">{{ t('payment.crypto.txTitle') }}</p>
            <p class="mt-1 text-xs leading-relaxed text-teal-700 dark:text-teal-200">{{ t('payment.crypto.txHint') }}</p>
            <div class="mt-3 grid gap-3 md:grid-cols-[1fr_auto]">
              <input v-model.trim="cryptoTxHash" type="text" autocomplete="off" spellcheck="false" :placeholder="t('payment.crypto.txPlaceholder')" class="rounded-xl border border-teal-200 bg-white px-4 py-3 font-mono text-sm text-slate-900 outline-none transition focus:border-teal-500 dark:border-teal-900 dark:bg-dark-900 dark:text-slate-100" />
              <button class="rounded-xl bg-teal-600 px-5 py-3 text-sm font-bold text-white transition hover:bg-teal-500 disabled:cursor-not-allowed disabled:opacity-50" :disabled="!canSubmitCryptoTx || submittingCryptoTx" @click="submitCryptoTx">
                {{ submittingCryptoTx ? t('common.processing') : t('payment.crypto.submitTx') }}
              </button>
            </div>
            <p v-if="cryptoTxError" class="mt-2 text-xs font-medium text-amber-700 dark:text-amber-200">{{ cryptoTxError }}</p>
          </div>
          <div class="card p-4 text-center">
            <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('payment.qr.expiresIn') }}</p>
            <p class="mt-1 text-2xl font-bold tabular-nums text-gray-900 dark:text-white">{{ countdownDisplay }}</p>
            <p class="mt-1 text-xs text-gray-400 dark:text-gray-500">{{ t('payment.crypto.autoConfirmHint') }}</p>
          </div>
          <button class="btn btn-secondary w-full" :disabled="cancelling" @click="handleCancel">
            {{ cancelling ? t('common.processing') : t('payment.qr.cancelOrder') }}
          </button>
        </div>
      </div>
    </template>

    <!-- QR Code Mode -->
    <template v-else-if="qrUrl">
      <div class="card p-6">
        <div class="flex flex-col items-center space-y-4">
          <p class="text-lg font-semibold text-gray-900 dark:text-white">{{ scanTitle }}</p>
          <div :class="['payment-qr-shell relative rounded-lg border-2 p-4', qrBorderClass]">
            <canvas ref="qrCanvas" class="mx-auto"></canvas>
            <!-- Brand logo overlay -->
            <div class="pointer-events-none absolute inset-0 flex items-center justify-center">
              <span :class="['payment-qr-logo rounded-full p-2 shadow ring-2 ring-white', qrLogoBgClass]">
                <img :src="qrLogoIcon" alt="" class="h-5 w-5 object-contain" />
              </span>
            </div>
          </div>
          <p v-if="scanHint" class="text-center text-sm text-gray-500 dark:text-gray-400">{{ scanHint }}</p>
          <button v-if="payUrl" class="btn btn-secondary text-sm" @click="reopenPopup">
            {{ t('payment.qr.openPayWindow') }}
          </button>
        </div>
      </div>
      <div class="card p-4 text-center">
        <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('payment.qr.expiresIn') }}</p>
        <p class="mt-1 text-2xl font-bold tabular-nums text-gray-900 dark:text-white">{{ countdownDisplay }}</p>
        <p class="mt-1 text-xs text-gray-400 dark:text-gray-500">{{ t('payment.qr.waitingPayment') }}</p>
      </div>
      <button class="btn btn-secondary w-full" :disabled="cancelling" @click="handleCancel">
        {{ cancelling ? t('common.processing') : t('payment.qr.cancelOrder') }}
      </button>
    </template>

    <!-- Waiting for Popup/Redirect Mode -->
    <template v-else>
      <div class="card p-6">
        <div class="flex flex-col items-center space-y-4 py-4">
          <div class="h-10 w-10 animate-spin rounded-full border-4 border-primary-500 border-t-transparent"></div>
          <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('payment.qr.payInNewWindowHint') }}</p>
          <button v-if="payUrl" class="btn btn-secondary text-sm" @click="reopenPopup">
            {{ t('payment.qr.openPayWindow') }}
          </button>
        </div>
      </div>
      <div class="card p-4 text-center">
        <p class="mt-1 text-2xl font-bold tabular-nums text-gray-900 dark:text-white">{{ countdownDisplay }}</p>
        <p class="mt-1 text-xs text-gray-400 dark:text-gray-500">{{ t('payment.qr.waitingPayment') }}</p>
      </div>
      <button class="btn btn-secondary w-full" :disabled="cancelling" @click="handleCancel">
        {{ cancelling ? t('common.processing') : t('payment.qr.cancelOrder') }}
      </button>
    </template>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, onUnmounted, nextTick } from 'vue'
import { useI18n } from 'vue-i18n'
import { usePaymentStore } from '@/stores/payment'
import { useAppStore } from '@/stores'
import { paymentAPI } from '@/api/payment'
import { extractI18nErrorMessage } from '@/utils/apiError'
import { getPaymentPopupFeatures, isBuiltInAlipayMethod, isBuiltInWxpayMethod } from '@/components/payment/providerConfig'
import { currencySymbol, formatPaymentAmount, normalizePaymentCurrency } from '@/components/payment/currency'
import type { CryptoPaymentDetails, PaymentOrder } from '@/types/payment'
import Icon from '@/components/icons/Icon.vue'
import QRCode from 'qrcode'
import alipayIcon from '@/assets/icons/alipay.svg'
import wxpayIcon from '@/assets/icons/wxpay.svg'
import paymentIcon from '@/assets/icons/payment.svg'

const props = defineProps<{
  orderId: number
  qrCode: string
  expiresAt: string
  paymentType: string
  payUrl?: string
  orderType?: string
  currency?: string
  crypto?: CryptoPaymentDetails
}>()

type PaymentOutcome = 'success' | 'cancelled' | 'expired'

const emit = defineEmits<{ done: []; success: []; settled: [outcome: PaymentOutcome] }>()

const i18n = useI18n()
const { t } = i18n
const paymentStore = usePaymentStore()
const appStore = useAppStore()

const qrCanvas = ref<HTMLCanvasElement | null>(null)
const qrUrl = ref('')
const remainingSeconds = ref(0)
const cancelling = ref(false)
const paidOrder = ref<PaymentOrder | null>(null)
const cryptoTxHash = ref('')
const cryptoTxError = ref('')
const submittingCryptoTx = ref(false)
const paymentCurrency = computed(() => normalizePaymentCurrency(props.currency))
const creditedAmountSymbol = currencySymbol('USD')
const localeCode = computed(() => {
  const raw = i18n.locale as unknown
  if (typeof raw === 'string') return raw
  if (raw && typeof raw === 'object' && 'value' in raw) {
    return String((raw as { value?: string }).value || '')
  }
  return undefined
})

// Terminal outcome: null = still active, 'success' | 'cancelled' | 'expired'
const outcome = ref<PaymentOutcome | null>(null)

let pollTimer: ReturnType<typeof setInterval> | null = null
let countdownTimer: ReturnType<typeof setInterval> | null = null
let verifyAttempts = 0
let lastVerifyAt = 0

const VERIFY_RETRY_INTERVAL_MS = 15000
const VERIFY_RETRY_MAX_ATTEMPTS = 6

const isAlipay = computed(() => isBuiltInAlipayMethod(props.paymentType))
const isWxpay = computed(() => isBuiltInWxpayMethod(props.paymentType))
const isCryptoPayment = computed(() => props.paymentType === 'crypto')
const cryptoDetails = computed(() => props.crypto)
const cryptoToken = computed(() => cryptoDetails.value?.token || 'USDT')
const cryptoNetworkLabel = computed(() => {
  const network = cryptoDetails.value?.network || 'TRC20'
  return `${cryptoToken.value}-${network}`
})
const cryptoPayAmountLabel = computed(() => {
  const detail = cryptoDetails.value
  const amount = detail?.pay_amount_text || formatFixedAmount(detail?.pay_amount, 6)
  return `${amount} ${cryptoToken.value}`
})
const cryptoBalanceLabel = computed(() => `${formatFixedAmount(cryptoDetails.value?.balance_amount, 2)} ${t('payment.crypto.balanceUnit')}`)
const cryptoRateLabel = computed(() => `${formatFixedAmount(cryptoDetails.value?.usdt_balance_rate, 2)} ${t('payment.crypto.balanceUnit')}`)
const cryptoMinAmountLabel = computed(() => `${formatFixedAmount(cryptoDetails.value?.min_amount, 6)} ${cryptoToken.value}`)
const canSubmitCryptoTx = computed(() => /^[0-9a-fA-F]{64}$/.test(cryptoTxHash.value.trim()))

const qrBorderClass = computed(() => {
  if (isAlipay.value) return 'payment-provider-alipay'
  if (isWxpay.value) return 'payment-provider-wxpay'
  return 'payment-provider-default'
})

const qrLogoBgClass = computed(() => {
  if (isAlipay.value) return 'payment-provider-alipay'
  if (isWxpay.value) return 'payment-provider-wxpay'
  return 'payment-provider-default'
})

const qrLogoIcon = computed(() => {
  if (isAlipay.value) return alipayIcon
  if (isWxpay.value) return wxpayIcon
  return paymentIcon
})

const scanTitle = computed(() => {
  if (isAlipay.value) return t('payment.qr.scanAlipay')
  if (isWxpay.value) return t('payment.qr.scanWxpay')
  return t('payment.qr.scanToPay')
})

const scanHint = computed(() => {
  if (isAlipay.value) return t('payment.qr.scanAlipayHint')
  if (isWxpay.value) return t('payment.qr.scanWxpayHint')
  return ''
})

const countdownDisplay = computed(() => {
  const m = Math.floor(remainingSeconds.value / 60)
  const s = remainingSeconds.value % 60
  return m.toString().padStart(2, '0') + ':' + s.toString().padStart(2, '0')
})

function formatGatewayAmount(value: number, currency?: string | null): string {
  return formatPaymentAmount(value, currency || paymentCurrency.value, localeCode.value)
}

function formatFixedAmount(value: number | null | undefined, fractionDigits: number): string {
  const amount = Number(value)
  return Number.isFinite(amount) ? amount.toFixed(fractionDigits) : (0).toFixed(fractionDigits)
}

function isSuccessStatus(status: string | null | undefined): boolean {
  return status === 'COMPLETED' || status === 'PAID' || status === 'RECHARGING'
}

function handleOrderStatus(order: PaymentOrder): boolean {
  if (isSuccessStatus(order.status)) {
    cleanup()
    paidOrder.value = order
    setOutcome('success')
    emit('success')
    return true
  }
  if (order.status === 'CANCELLED') {
    cleanup()
    setOutcome('cancelled')
    return true
  }
  if (order.status === 'EXPIRED' || order.status === 'FAILED') {
    cleanup()
    setOutcome('expired')
    return true
  }
  return false
}

function reopenPopup() {
  if (props.payUrl) {
    const win = window.open(props.payUrl, 'paymentPopup', getPaymentPopupFeatures())
    if (!win || win.closed) {
      window.location.href = props.payUrl
    }
  }
}

function setOutcome(next: PaymentOutcome) {
  if (outcome.value === next) return
  outcome.value = next
  emit('settled', next)
}

async function renderQR() {
  await nextTick()
  if (!qrCanvas.value || !qrUrl.value) return
  await QRCode.toCanvas(qrCanvas.value, qrUrl.value, {
    width: 220, margin: 2,
    errorCorrectionLevel: 'M',
  })
}

async function tryRecoverPendingOrder(order: PaymentOrder): Promise<PaymentOrder> {
  if (!isWxpay.value) return order
  const outTradeNo = String(order.out_trade_no || '').trim()
  if (!outTradeNo) return order
  const normalizedStatus = String(order.status || '').trim().toUpperCase()
  if (normalizedStatus !== 'PENDING') return order
  const now = Date.now()
  if (verifyAttempts >= VERIFY_RETRY_MAX_ATTEMPTS || now - lastVerifyAt < VERIFY_RETRY_INTERVAL_MS) {
    return order
  }

  lastVerifyAt = now
  verifyAttempts += 1
  try {
    const result = await paymentAPI.verifyOrder(outTradeNo)
    return result.data ?? order
  } catch {
    return order
  }
}

let pollInFlight = false
async function pollStatus() {
  if (!props.orderId || outcome.value) return
  // 防重入：接口（含 verifyOrder 二次确认）响应慢于 3 秒轮询间隔时避免并发重叠请求。
  if (pollInFlight) return
  pollInFlight = true
  try {
    let order = await paymentStore.pollOrderStatus(props.orderId)
    if (!order) return
    // 已进入终态则不再处理迟到的响应。
    if (outcome.value) return
    order = await tryRecoverPendingOrder(order)
    if (outcome.value) return
    if (isSuccessStatus(order.status)) {
      cleanup()
      paidOrder.value = order
      setOutcome('success')
      emit('success')
    } else if (order.status === 'CANCELLED') {
      cleanup()
      setOutcome('cancelled')
    } else if (order.status === 'EXPIRED' || order.status === 'FAILED') {
      cleanup()
      setOutcome('expired')
    }
  } finally {
    pollInFlight = false
  }
}

async function copyToClipboard(value: string) {
  const text = value.trim()
  if (!text) return
  try {
    await navigator.clipboard.writeText(text)
    appStore.showSuccess(t('common.copiedToClipboard'))
  } catch {
    appStore.showError(t('common.copyFailed'))
  }
}

async function submitCryptoTx() {
  if (!props.orderId || !canSubmitCryptoTx.value || submittingCryptoTx.value) return
  submittingCryptoTx.value = true
  cryptoTxError.value = ''
  try {
    const response = await paymentAPI.submitCryptoTx(props.orderId, cryptoTxHash.value.trim())
    const order = response.data
    if (!handleOrderStatus(order)) {
      appStore.showInfo(t('payment.crypto.txSubmitted'))
      await pollStatus()
    }
  } catch (err: unknown) {
    cryptoTxError.value = extractI18nErrorMessage(err, t, 'payment.errors', t('payment.crypto.txFailed'))
  } finally {
    submittingCryptoTx.value = false
  }
}

function startCountdown(seconds: number) {
  remainingSeconds.value = Math.max(0, seconds)
  if (remainingSeconds.value <= 0) { setOutcome('expired'); return }
  countdownTimer = setInterval(() => {
    remainingSeconds.value--
    if (remainingSeconds.value <= 0) { setOutcome('expired'); cleanup() }
  }, 1000)
}

async function handleCancel() {
  if (!props.orderId || cancelling.value) return
  cancelling.value = true
  try {
    await paymentAPI.cancelOrder(props.orderId)
    cleanup()
    setOutcome('cancelled')
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, 'payment.errors', t('common.error')))
  } finally {
    cancelling.value = false
  }
}

function handleDone() { cleanup(); emit('done') }

function cleanup() {
  if (pollTimer) { clearInterval(pollTimer); pollTimer = null }
  if (countdownTimer) { clearInterval(countdownTimer); countdownTimer = null }
}

// Initialize on mount
qrUrl.value = props.qrCode
verifyAttempts = 0
lastVerifyAt = 0
let seconds = 30 * 60
if (props.expiresAt) {
  seconds = Math.floor((new Date(props.expiresAt).getTime() - Date.now()) / 1000)
}
startCountdown(seconds)
pollTimer = setInterval(pollStatus, 3000)
renderQR()

watch(() => qrUrl.value, () => renderQR())
onUnmounted(() => cleanup())
</script>

<style scoped>
.payment-qr-shell {
  border: 0 !important;
  background: var(--zc-bg) !important;
  box-shadow: inset 8px 8px 18px var(--zc-shadow-dark), inset -8px -8px 18px var(--zc-shadow-light);
}

.payment-qr-logo {
  border: 0 !important;
  background: var(--zc-bg) !important;
  color: var(--zc-accent) !important;
  box-shadow: 7px 7px 16px var(--zc-shadow-dark), -7px -7px 16px var(--zc-shadow-light) !important;
}

.payment-provider-alipay,
.payment-provider-wxpay,
.payment-provider-default {
  border-color: transparent !important;
}
</style>
