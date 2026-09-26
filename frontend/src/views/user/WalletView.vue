<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import TokenRewardWallet from '@/features/bizdecipher/components/wallet/TokenRewardWallet.vue'
import TokenRewardReviews from '@/features/bizdecipher/components/wallet/TokenRewardReviews.vue'
import SharedPoolOwnerWalletPanel from '@/features/bizdecipher/components/shared-pool/SharedPoolOwnerWalletPanel.vue'
import WalletAccountSummary from '@/features/bizdecipher/components/wallet/WalletAccountSummary.vue'
import WithdrawalPanel from '@/features/bizdecipher/components/wallet/WithdrawalPanel.vue'
import { useAuthStore } from '@/stores/auth'
import { extractActionableApiErrorMessage } from '@/utils/apiError'
import type { User } from '@/types'
import { authAPI } from '@/api'
import { listMySharedPoolLedger, type SharedPoolOwnerWallet } from '@/features/bizdecipher/api/bizdecipher'

const authStore = useAuthStore()
const route = useRoute()
const earningsPoolId = computed(() => {
  const raw = route.query.pool_id
  if (typeof raw !== 'string' || !/^[1-9]\d*$/.test(raw)) return undefined
  const id = Number(raw)
  return Number.isSafeInteger(id) ? id : undefined
})
const accountLoading = ref(false)
const accountError = ref('')
const accountUser = ref<User | null>(authStore.user)
const creatorWallet = ref<SharedPoolOwnerWallet | undefined>(undefined)
const earningsLoading = ref(false)
const earningsError = ref('')
const withdrawalRevision = ref(0)
let refreshEpoch = 0
let earningsEpoch = 0
const siteBalance = computed(() => accountUser.value ? Number(accountUser.value.balance) : undefined)
const creditBalance = computed(() => accountUser.value ? Number(accountUser.value.credit_balance) : undefined)

watch(
  () => [authStore.token, authStore.user?.id] as const,
  () => {
    refreshEpoch += 1
    earningsEpoch += 1
    accountUser.value = authStore.user
    accountLoading.value = false
    accountError.value = ''
    creatorWallet.value = undefined
    earningsLoading.value = false
    earningsError.value = ''
  },
  { immediate: true },
)

async function refreshAccount(): Promise<void> {
  if (accountLoading.value) return
  const epoch = ++refreshEpoch
  const token = authStore.token
  const userId = authStore.user?.id
  accountLoading.value = true
  accountError.value = ''
  try {
    const response = await authAPI.getCurrentUser()
    if (epoch !== refreshEpoch || authStore.token !== token || authStore.user?.id !== userId) return
    accountUser.value = response.data
  } catch (error) {
    if (epoch !== refreshEpoch || authStore.token !== token || authStore.user?.id !== userId) return
    accountError.value = extractActionableApiErrorMessage(error, '账户余额同步失败。')
  } finally {
    if (epoch === refreshEpoch) accountLoading.value = false
  }
}

onMounted(refreshAccount)

async function refreshEarnings(): Promise<void> {
  if (earningsLoading.value) return
  const epoch = ++earningsEpoch
  const token = authStore.token
  const userId = authStore.user?.id
  earningsLoading.value = true
  earningsError.value = ''
  try {
    const view = await listMySharedPoolLedger()
    if (epoch !== earningsEpoch || authStore.token !== token || authStore.user?.id !== userId) return
    creatorWallet.value = view.wallet
  } catch (error) {
    if (epoch !== earningsEpoch || authStore.token !== token || authStore.user?.id !== userId) return
    earningsError.value = extractActionableApiErrorMessage(error, '创作与供给收益同步失败，请稍后重试。')
  } finally {
    if (epoch === earningsEpoch) earningsLoading.value = false
  }
}

onMounted(refreshEarnings)

function onLedgerRefreshed(view?: { wallet?: SharedPoolOwnerWallet }): void {
  if (view?.wallet) {
    creatorWallet.value = view.wallet
    earningsError.value = ''
  }
  void refreshAccount()
}

function onWithdrawalChanged(): void {
  withdrawalRevision.value += 1
  void refreshEarnings()
}
</script>

<template>
  <AppLayout>
    <main class="wallet-page">
      <header class="wallet-page-head">
        <div>
          <h1>钱包</h1>
        </div>
      </header>

      <TokenRewardWallet />
      <TokenRewardReviews v-if="authStore.user?.role === 'admin'" :key="authStore.user.id" />
      <WalletAccountSummary
        :balance="siteBalance"
        :credits="creditBalance"
        :earnings-available="creatorWallet?.available_amount"
        :earnings-total="creatorWallet?.total_earned"
        :earnings-pending="creatorWallet?.pending_amount"
        :loading="accountLoading"
        :earnings-loading="earningsLoading"
        :error="accountError"
        :earnings-error="earningsError"
        @refresh="refreshAccount"
        @refresh-earnings="refreshEarnings"
      />

      <WithdrawalPanel v-if="authStore.user" :key="authStore.user.id" :owner-id="authStore.user.id" @changed="onWithdrawalChanged" />

      <section id="shared-pool-earnings" class="wallet-earnings" aria-label="经营与创作收益">
        <div v-if="earningsPoolId" class="wallet-earnings-context">
          <span>正在查看共享池 #{{ earningsPoolId }} 的收益记录</span>
          <RouterLink to="/wallet#shared-pool-earnings">查看全部收益</RouterLink>
          <RouterLink :to="{ name: 'AccountSquareOwnerPoolDetail', params: { id: earningsPoolId } }">返回池详情</RouterLink>
        </div>
        <SharedPoolOwnerWalletPanel :key="`${authStore.user?.id ?? 'anonymous'}:${earningsPoolId ?? 'all'}:${withdrawalRevision}`" compact :pool-id="earningsPoolId" @refreshed="onLedgerRefreshed" />
      </section>
    </main>
  </AppLayout>
</template>

<style scoped>
.wallet-page { --text: var(--bd-text-primary); --muted: var(--bd-text-secondary); --teal: var(--bd-accent-teal); --module-panel: var(--bd-surface); --module-panel-raised: var(--bd-surface-raised); display: grid; gap: var(--bd-space-6); width: min(1120px, 100%); margin: 0 auto; color: var(--bd-text-primary); }
.wallet-page-head { display: flex; align-items: flex-end; justify-content: space-between; gap: var(--bd-space-6); padding-bottom: var(--bd-space-5); border-bottom: 1px solid var(--bd-ui-line); }
.wallet-page-head span { color: var(--bd-accent-teal); font-size: var(--bd-type-overline); font-weight: var(--bd-weight-overline); }
.wallet-page-head h1 { margin: var(--bd-space-1) 0 var(--bd-space-2); font-size: 24px; line-height: 1.35; }
.wallet-page-head p { max-width: 68ch; margin: 0; color: var(--bd-text-secondary); font-size: 13px; line-height: 1.65; }
.wallet-page-head a { flex: 0 0 auto; min-height: 36px; display: inline-flex; align-items: center; padding: 0 var(--bd-space-3); color: var(--bd-accent-teal); border: 1px solid var(--bd-ui-line); border-radius: var(--bd-radius-sm); font-size: var(--bd-type-caption); text-decoration: none; }
.wallet-page-head nav { display: flex; flex-wrap: wrap; gap: 8px; }
.wallet-withdrawal-note { display: flex; align-items: center; gap: var(--bd-space-3); padding: var(--bd-space-3) var(--bd-space-4); border: 1px solid var(--bd-ui-line); border-left: 3px solid var(--bd-accent-gold); background: var(--bd-surface); font-size: var(--bd-type-caption); }
.wallet-withdrawal-note span { color: var(--bd-text-secondary); }
.wallet-earnings { min-width: 0; scroll-margin-top: 90px; }
.wallet-earnings-context { display: flex; flex-wrap: wrap; gap: 12px; align-items: center; padding-bottom: 16px; font-size: 12px; color: var(--bd-text-secondary); }
.wallet-earnings-context a { color: var(--bd-accent-teal); }
@media (max-width: 640px) { .wallet-page-head { align-items: flex-start; flex-direction: column; } .wallet-withdrawal-note { align-items: flex-start; flex-direction: column; } }
</style>
