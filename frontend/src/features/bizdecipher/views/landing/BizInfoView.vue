<template>
  <BizPublicLayout>
    <section class="px-6 py-16 md:py-24">
      <div class="mx-auto max-w-7xl">
        <p class="text-sm font-semibold uppercase tracking-[0.3em] text-indigo-600">{{ page.eyebrow }}</p>
        <h1 class="mt-4 max-w-4xl text-4xl font-semibold tracking-[-0.035em] text-slate-950 md:text-6xl">{{ page.title }}</h1>
        <p class="mt-6 max-w-3xl text-lg leading-8 text-slate-600">{{ page.description }}</p>

        <div class="mt-10 grid gap-5 md:grid-cols-3">
          <article v-for="item in page.items" :key="item.title" class="rounded-[1.5rem] border border-slate-200 bg-white p-6 shadow-sm">
            <div class="mb-4 inline-flex rounded-full bg-indigo-50 px-3 py-1 text-xs font-semibold text-indigo-600 ring-1 ring-indigo-100">{{ item.badge }}</div>
            <h2 class="text-xl font-semibold tracking-tight text-slate-950">{{ item.title }}</h2>
            <p class="mt-3 leading-7 text-slate-600">{{ item.body }}</p>
          </article>
        </div>

        <div class="mt-10 rounded-[1.5rem] border border-slate-200 bg-white p-6 shadow-sm">
          <div class="md:flex md:items-center md:justify-between md:gap-8">
            <div>
              <h2 class="text-2xl font-semibold tracking-tight text-slate-950">{{ page.ctaTitle }}</h2>
              <p class="mt-2 text-slate-600">{{ page.ctaBody }}</p>
            </div>
            <router-link :to="page.ctaTo" class="mt-5 inline-flex rounded-full bg-slate-950 px-5 py-3 text-sm font-semibold text-white transition hover:bg-indigo-700 md:mt-0">
              {{ page.ctaLabel }}
            </router-link>
          </div>

          <form v-if="isContributorPage" class="mt-6 grid gap-4 md:grid-cols-2" @submit.prevent="submitContributor">
            <input v-model="contributor.label" class="biz-input" :placeholder="t('biz.forms.contributor.label')" />
            <input v-model="contributor.capacityTypes" class="biz-input" :placeholder="t('biz.forms.contributor.capacityTypes')" />
            <input v-model="contributor.settlementMethod" class="biz-input" :placeholder="t('biz.forms.contributor.settlementMethod')" />
            <input v-model="contributor.capacityHint" class="biz-input" :placeholder="t('biz.forms.contributor.capacityHint')" />
            <button class="biz-submit md:col-span-2" type="submit">{{ t('biz.forms.contributor.submit') }}</button>
          </form>

          <form v-else-if="isOperatorPage" class="mt-6 grid gap-4" @submit.prevent="submitOperator">
            <input v-model="operatorSkills" class="biz-input" :placeholder="t('biz.forms.operator.skills')" />
            <button class="biz-submit" type="submit">{{ t('biz.forms.operator.submit') }}</button>
          </form>

          <form v-else-if="isCustomRequestPage" class="mt-6 grid gap-4 md:grid-cols-2" @submit.prevent="submitCustomRequest">
            <input v-model="customRequest.goal" class="biz-input md:col-span-2" :placeholder="t('biz.forms.customRequest.goal')" required />
            <textarea v-model="customRequest.context" class="biz-input min-h-28 md:col-span-2" :placeholder="t('biz.forms.customRequest.context')"></textarea>
            <input v-model="customRequest.budgetRange" class="biz-input" :placeholder="t('biz.forms.customRequest.budgetRange')" />
            <input v-model="customRequest.references" class="biz-input" :placeholder="t('biz.forms.customRequest.references')" />
            <button class="biz-submit md:col-span-2" type="submit">{{ t('biz.forms.customRequest.submit') }}</button>
          </form>

          <form v-else-if="isAdminRewardsPage" class="mt-6 grid gap-4 md:grid-cols-2" @submit.prevent="submitAdminCreditGrant">
            <input v-model.number="creditGrant.userId" class="biz-input" :placeholder="t('biz.forms.adminRewards.userId')" type="number" required />
            <input v-model.number="creditGrant.amount" class="biz-input" :placeholder="t('biz.forms.adminRewards.amount')" type="number" step="0.00000001" required />
            <input v-model="creditGrant.sourceType" class="biz-input" :placeholder="t('biz.forms.adminRewards.sourceType')" />
            <input v-model="creditGrant.sourceId" class="biz-input" :placeholder="t('biz.forms.adminRewards.sourceId')" />
            <textarea v-model="creditGrant.note" class="biz-input min-h-24 md:col-span-2" :placeholder="t('biz.forms.adminRewards.note')"></textarea>
            <button class="biz-submit md:col-span-2" type="submit">{{ t('biz.forms.adminRewards.submit') }}</button>
          </form>

          <div v-if="submitMessage" class="mt-4 rounded-2xl border border-emerald-200 bg-emerald-50 p-4 text-sm text-emerald-700">{{ submitMessage }}</div>
          <div v-if="submitError" class="mt-4 rounded-2xl border border-red-200 bg-red-50 p-4 text-sm text-red-700">{{ submitError }}</div>
        </div>

        <!-- Platform status: gateway-level health, decoupled from the shared pool. -->
        <template v-if="platformStatusCards.length">
          <h2 class="mt-10 text-xl font-semibold tracking-tight text-slate-950">{{ t('biz.platformStatus.title') }}</h2>
          <p class="mt-1 text-sm text-slate-500">{{ t('biz.platformStatus.subtitle') }}</p>
          <div class="mt-4 grid gap-4 md:grid-cols-4">
            <div v-for="card in platformStatusCards" :key="card.label" class="rounded-2xl border border-slate-200 bg-white p-5 shadow-sm">
              <div class="text-2xl font-semibold tracking-tight text-slate-950">{{ card.value }}</div>
              <div class="mt-1 text-sm text-slate-500">{{ card.label }}</div>
            </div>
          </div>
        </template>

        <!-- Shared pool status: marketplace contributors & requests, kept separate. -->
        <template v-if="statusCards.length">
          <h2 class="mt-10 text-xl font-semibold tracking-tight text-slate-950">{{ t('biz.poolStatus.title') }}</h2>
          <p class="mt-1 text-sm text-slate-500">{{ t('biz.poolStatus.subtitle') }}</p>
          <div class="mt-4 grid gap-4 md:grid-cols-4">
            <div v-for="card in statusCards" :key="card.label" class="rounded-2xl border border-slate-200 bg-white p-5 shadow-sm">
              <div class="text-2xl font-semibold tracking-tight text-slate-950">{{ card.value }}</div>
              <div class="mt-1 text-sm text-slate-500">{{ card.label }}</div>
            </div>
          </div>
        </template>

        <div v-if="adminItems.length" class="mt-8 overflow-hidden rounded-[1.5rem] border border-slate-200 bg-white shadow-sm">
          <div class="border-b border-slate-200 px-5 py-4 text-sm font-semibold text-slate-950">{{ t('biz.records.latest') }}</div>
          <div class="overflow-auto">
            <table class="min-w-full divide-y divide-slate-200 text-left text-sm">
              <thead class="bg-slate-50 text-xs uppercase tracking-wide text-slate-500">
                <tr>
                  <th v-for="column in adminColumns" :key="column" class="whitespace-nowrap px-5 py-3 font-semibold">{{ column }}</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-slate-100 text-slate-700">
                <tr v-for="(row, index) in displayAdminRows" :key="index" class="hover:bg-slate-50/80">
                  <td v-for="column in adminColumns" :key="column" class="max-w-[280px] truncate px-5 py-3">{{ formatCell(row[column]) }}</td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </div>
    </section>
  </BizPublicLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'
import BizPublicLayout from '@/features/bizdecipher/components/common/BizPublicLayout.vue'
import {
  adminGrantBizCredit,
  adminListBizContributors,
  adminListBizCredits,
  adminListBizOperators,
  adminListBizRequests,
  applyBizContributor,
  applyBizOperator,
  createBizCustomRequest,
  getBizPoolStatus,
  getBizPlatformStatus,
  listMyBizCredits,
  type BizPoolStatus,
  type PlatformStatus
} from '@/features/bizdecipher/api/bizdecipher'

type PageConfig = {
  eyebrow: string
  title: string
  description: string
  items: Array<{ badge: string; title: string; body: string }>
  ctaTitle: string
  ctaBody: string
  ctaLabel: string
  ctaTo: string
}

type AdminRow = Record<string, unknown>

const route = useRoute()
const { t, tm } = useI18n()

const routeName = computed(() => String(route.name || 'gateway'))
const pages = computed(() => tm('biz.pages') as Record<string, PageConfig>)
const page = computed(() => pages.value[routeName.value] ?? pages.value.gateway)

const submitMessage = ref('')
const submitError = ref('')
const poolStatus = ref<BizPoolStatus | null>(null)
const platformStatus = ref<PlatformStatus | null>(null)
const adminItems = ref<unknown[]>([])

const contributor = reactive({ label: '', capacityTypes: '', settlementMethod: '', capacityHint: '' })
const operatorSkills = ref('')
const customRequest = reactive({ goal: '', context: '', budgetRange: '', references: '' })
const creditGrant = reactive({ userId: null as number | null, amount: null as number | null, sourceType: 'admin_adjustment', sourceId: '', note: '' })

const isContributorPage = computed(() => routeName.value === 'contribute')
const isOperatorPage = computed(() => routeName.value === 'operators-apply')
const isCustomRequestPage = computed(() => routeName.value === 'custom-request')
const isAdminRewardsPage = computed(() => routeName.value === 'admin-rewards')
const isUserCreditLedgerPage = computed(() => ['credits', 'dashboard-rewards', 'dashboard-credits'].includes(routeName.value))
const isPoolLikePage = computed(() => ['pool', 'status', 'admin-pool', 'admin-status', 'dashboard-pool'].includes(routeName.value))

const statusCards = computed(() => {
  if (!poolStatus.value || !isPoolLikePage.value) return []
  return [
    { label: t('biz.statusCards.health'), value: poolStatus.value.health },
    { label: t('biz.statusCards.activeResources'), value: String(poolStatus.value.active_resources) },
    { label: t('biz.statusCards.testingResources'), value: String(poolStatus.value.testing_resources) },
    { label: t('biz.statusCards.pendingContributors'), value: String(poolStatus.value.pending_contributors) }
  ]
})

// Platform status is rendered separately from the pool status, satisfying the
// product rule that the platform status bar must be decoupled from the
// shared-pool status. Shown on the dedicated /status page.
const platformStatusCards = computed(() => {
  if (!platformStatus.value || !isPoolLikePage.value) return []
  return [
    { label: t('biz.platformCards.health'), value: platformStatus.value.health },
    { label: t('biz.platformCards.channels'), value: `${platformStatus.value.active_channels}/${platformStatus.value.total_channels}` },
    { label: t('biz.platformCards.accounts'), value: `${platformStatus.value.active_accounts}/${platformStatus.value.total_accounts}` },
    { label: t('biz.platformCards.errorAccounts'), value: String(platformStatus.value.error_accounts) }
  ]
})

const displayAdminRows = computed<AdminRow[]>(() =>
  adminItems.value.map((item) => {
    if (item && typeof item === 'object' && !Array.isArray(item)) {
      return item as AdminRow
    }
    return { value: item }
  })
)

const adminColumns = computed(() => {
  const priority = ['id', 'user_id', 'status', 'amount', 'source_type', 'label', 'goal', 'created_at', 'updated_at']
  const keys = new Set<string>()
  displayAdminRows.value.slice(0, 20).forEach((row) => {
    Object.keys(row).forEach((key) => keys.add(key))
  })
  return [...priority.filter((key) => keys.has(key)), ...[...keys].filter((key) => !priority.includes(key))].slice(0, 8)
})

function formatCell(value: unknown): string {
  if (value === null || value === undefined || value === '') return '-'
  if (Array.isArray(value)) return value.join(', ')
  if (typeof value === 'object') return JSON.stringify(value)
  return String(value)
}

function splitList(value: string): string[] {
  return value.split(',').map((item) => item.trim()).filter(Boolean)
}

async function submitContributor() {
  await runSubmit(async () => {
    await applyBizContributor({
      capacity_types: splitList(contributor.capacityTypes),
      settlement_method: contributor.settlementMethod,
      capacity_hint: contributor.capacityHint,
      label: contributor.label
    })
    submitMessage.value = t('biz.messages.contributionSubmitted')
  })
}

async function submitOperator() {
  await runSubmit(async () => {
    await applyBizOperator({ skills: splitList(operatorSkills.value) })
    submitMessage.value = t('biz.messages.operatorSubmitted')
  })
}

async function submitCustomRequest() {
  await runSubmit(async () => {
    await createBizCustomRequest({
      goal: customRequest.goal,
      context: customRequest.context,
      budget_range: customRequest.budgetRange,
      references: splitList(customRequest.references)
    })
    submitMessage.value = t('biz.messages.customRequestSubmitted')
  })
}

async function submitAdminCreditGrant() {
  await runSubmit(async () => {
    if (!creditGrant.userId || !creditGrant.amount) throw new Error(t('biz.messages.userAndAmountRequired'))
    await adminGrantBizCredit({
      user_id: creditGrant.userId,
      amount: creditGrant.amount,
      source_type: creditGrant.sourceType || 'admin_adjustment',
      source_id: creditGrant.sourceId,
      note: creditGrant.note
    })
    submitMessage.value = t('biz.messages.creditGrantPosted')
    creditGrant.amount = null
    creditGrant.note = ''
    await loadPageData()
  })
}

async function runSubmit(fn: () => Promise<void>) {
  submitMessage.value = ''
  submitError.value = ''
  try {
    await fn()
  } catch (error) {
    submitError.value = error instanceof Error ? error.message : t('biz.messages.submitFailed')
  }
}

async function loadPageData() {
  adminItems.value = []
  if (isPoolLikePage.value) {
    try { poolStatus.value = await getBizPoolStatus() } catch { poolStatus.value = null }
    try { platformStatus.value = await getBizPlatformStatus() } catch { platformStatus.value = null }
  }
  try {
    if (isUserCreditLedgerPage.value) adminItems.value = (await listMyBizCredits()).items
    if (routeName.value === 'admin-contributions') adminItems.value = (await adminListBizContributors()).items
    if (routeName.value === 'admin-operators') adminItems.value = (await adminListBizOperators()).items
    if (routeName.value === 'admin-requests') adminItems.value = (await adminListBizRequests()).items
    if (routeName.value === 'admin-credits' || routeName.value === 'admin-rewards') adminItems.value = (await adminListBizCredits()).items
  } catch {
    adminItems.value = []
  }
}

onMounted(loadPageData)
watch(() => route.name, loadPageData)
</script>

<style scoped>
.biz-input {
  width: 100%;
  border-radius: 1rem;
  border: 1px solid #dbe3ea;
  background: #ffffff;
  padding: 0.85rem 1rem;
  color: #0f172a;
  outline: none;
  box-shadow: 0 1px 2px rgba(15, 23, 42, 0.04);
}

.biz-input::placeholder {
  color: #94a3b8;
}

.biz-input:focus {
  border-color: #0f766e;
  box-shadow: 0 0 0 3px rgba(15, 118, 110, 0.14);
}

.biz-submit {
  border-radius: 9999px;
  background: linear-gradient(135deg, #0f766e 0%, #0891b2 100%);
  padding: 0.85rem 1.25rem;
  font-size: 0.875rem;
  font-weight: 700;
  color: #ffffff;
  transition: background 0.2s ease, transform 0.05s ease;
}

.biz-submit:hover {
  background: linear-gradient(135deg, #115e59 0%, #0e7490 100%);
}

.biz-submit:active {
  transform: translateY(1px) scale(0.99);
}
</style>
