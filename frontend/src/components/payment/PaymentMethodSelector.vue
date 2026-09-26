<template>
  <div>
    <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
      {{ t('payment.paymentMethod') }}
    </label>
    <div class="grid grid-cols-2 gap-3 sm:flex">
      <button
        v-for="method in sortedMethods"
        :key="method.type"
        type="button"
        :disabled="!method.available"
        :class="[
          'payment-method-option relative flex h-[60px] flex-col items-center justify-center rounded-lg border px-3 transition-all sm:flex-1',
          !method.available
            ? 'payment-method-disabled cursor-not-allowed opacity-50'
            : selected === method.type
              ? methodSelectedClass(method.type)
              : 'payment-method-idle',
        ]"
        @click="method.available && emit('select', method.type)"
      >
        <span class="flex items-center gap-2">
          <span v-if="method.type === 'crypto'" class="flex h-7 w-7 items-center justify-center rounded-full bg-teal-500 text-sm font-black text-white">₮</span>
          <img v-else :src="methodIcon(method.type)" :alt="methodLabel(method)" class="h-7 w-7 object-contain" />
          <span class="flex flex-col items-start leading-none">
            <span class="text-base font-semibold">{{ methodLabel(method) }}</span>
            <span
              v-if="method.fee_rate > 0"
              class="text-[10px] tracking-wide text-gray-500 dark:text-dark-400"
            >
              {{ t('payment.fee') }} {{ method.fee_rate }}%
            </span>
          </span>
        </span>
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { comparePaymentMethodsByLocale } from './providerConfig'
import { isBuiltInAlipayMethod, isBuiltInWxpayMethod } from './providerConfig'
import alipayIcon from '@/assets/icons/alipay.svg'
import wxpayIcon from '@/assets/icons/wxpay.svg'
import stripeIcon from '@/assets/icons/stripe.svg'
import airwallexIcon from '@/assets/icons/airwallex.svg'
import paymentIcon from '@/assets/icons/payment.svg'

export interface PaymentMethodOption {
  type: string
  display_name?: string
  fee_rate: number
  available: boolean
}

const props = defineProps<{
  methods: PaymentMethodOption[]
  selected: string
}>()

const emit = defineEmits<{
  select: [type: string]
}>()

const i18n = useI18n()
const { t } = i18n

const METHOD_ICONS: Record<string, string> = {
  alipay: alipayIcon,
  wxpay: wxpayIcon,
  stripe: stripeIcon,
  airwallex: airwallexIcon,
  credit_card: paymentIcon,
}

const localeCode = computed(() => {
  const raw = i18n.locale as unknown
  if (typeof raw === 'string') return raw
  if (raw && typeof raw === 'object' && 'value' in raw) {
    return String((raw as { value?: string }).value || '')
  }
  return undefined
})

const sortedMethods = computed(() =>
  [...props.methods].sort((a, b) => comparePaymentMethodsByLocale(a.type, b.type, localeCode.value)),
)

function methodIcon(type: string): string {
  if (isBuiltInAlipayMethod(type)) return METHOD_ICONS.alipay
  if (isBuiltInWxpayMethod(type)) return METHOD_ICONS.wxpay
  if (type === 'airwallex') return METHOD_ICONS.airwallex
  return METHOD_ICONS[type] || paymentIcon
}

function methodLabel(method: PaymentMethodOption): string {
  return method.display_name || t(`payment.methods.${method.type}`, method.type)
}

function methodSelectedClass(type: string): string {
  if (isBuiltInAlipayMethod(type)) return 'payment-method-selected border-[#02A9F1] bg-blue-50 text-gray-900 shadow-sm dark:bg-blue-950 dark:text-gray-100'
  if (isBuiltInWxpayMethod(type)) return 'payment-method-selected border-[#09BB07] bg-green-50 text-gray-900 shadow-sm dark:bg-green-950 dark:text-gray-100'
  if (type === 'stripe') return 'payment-method-selected border-[#676BE5] bg-indigo-50 text-gray-900 shadow-sm dark:bg-indigo-950 dark:text-gray-100'
  if (type === 'airwallex') return 'payment-method-selected border-[#FF6B3D] bg-orange-50 text-gray-900 shadow-sm dark:border-[#FF8E3C] dark:bg-orange-950 dark:text-gray-100'
  return 'payment-method-selected border-primary-500 bg-primary-50 text-gray-900 shadow-sm dark:bg-primary-950 dark:text-gray-100'
}
</script>

<style scoped>
.payment-method-option {
  border: 0 !important;
  background: var(--zc-bg) !important;
  color: var(--zc-text) !important;
  box-shadow: 7px 7px 16px var(--zc-shadow-dark), -7px -7px 16px var(--zc-shadow-light);
}

.payment-method-idle:hover {
  color: var(--zc-text-strong) !important;
  transform: translateY(-1px);
  box-shadow: 9px 9px 20px var(--zc-shadow-dark), -9px -9px 20px var(--zc-shadow-light);
}

.payment-method-selected {
  color: var(--zc-accent) !important;
  box-shadow: inset 5px 5px 12px var(--zc-shadow-dark), inset -5px -5px 12px var(--zc-shadow-light) !important;
}

.payment-method-disabled {
  color: var(--zc-subtle) !important;
  box-shadow: inset 4px 4px 10px var(--zc-shadow-dark), inset -4px -4px 10px var(--zc-shadow-light) !important;
}
</style>
