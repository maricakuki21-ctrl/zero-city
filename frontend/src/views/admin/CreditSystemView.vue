<template>
  <AppLayout>
    <div class="space-y-5">
      <section class="rounded-3xl border border-slate-200 bg-white p-5 shadow-sm dark:border-dark-600 dark:bg-dark-800">
        <div class="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
          <div>
            <p class="text-sm font-semibold uppercase tracking-[0.24em] text-indigo-600 dark:text-indigo-300">Credit System</p>
            <h1 class="mt-2 text-2xl font-bold text-slate-950 dark:text-white">积分系统后台</h1>
            <p class="mt-1 max-w-4xl text-sm text-slate-500 dark:text-slate-400">
              独立管理 users.credit_balance 与 credit_ledger：积分不可提现；邀请返利是可提现额度，转余额后进入 users.balance。
            </p>
          </div>
          <div class="flex flex-wrap gap-2">
            <button class="btn btn-secondary" :disabled="loading" @click="loadAll">
              <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
              刷新
            </button>
          </div>
        </div>
      </section>

      <section class="grid gap-3 md:grid-cols-5">
        <StatCard label="全站积分余额" :value="formatAmount(overview?.total_credit_balance)" tone="indigo" />
        <StatCard label="有积分用户" :value="String(overview?.positive_credit_users || 0)" tone="emerald" />
        <StatCard label="流水总数" :value="String(overview?.ledger_entry_count || 0)" />
        <StatCard label="今日发放" :value="formatAmount(overview?.today_granted_credits)" tone="cyan" />
        <StatCard label="今日消耗" :value="formatAmount(overview?.today_consumed_credits)" tone="amber" />
      </section>

      <section class="grid gap-4 xl:grid-cols-[1.25fr_0.75fr]">
        <div class="rounded-3xl border border-slate-200 bg-white p-5 shadow-sm dark:border-dark-600 dark:bg-dark-800">
          <div class="mb-4 flex items-center justify-between gap-3">
            <div>
              <h2 class="text-lg font-semibold text-slate-950 dark:text-white">积分规则边界</h2>
              <p class="mt-1 text-xs text-slate-500 dark:text-slate-400">这里展示固定产品规则，避免把可提现返利和不可提现积分混在一起。</p>
            </div>
          </div>
          <div class="grid gap-3 md:grid-cols-2">
            <div v-for="rule in overview?.rules || []" :key="rule.key" class="rounded-2xl border border-slate-200 bg-slate-50 p-4 dark:border-dark-600 dark:bg-dark-700">
              <div class="flex items-start justify-between gap-3">
                <div>
                  <p class="font-semibold text-slate-900 dark:text-white">{{ rule.title }}</p>
                  <p class="mt-1 text-xs text-slate-500 dark:text-slate-400">{{ assetTypeLabel(rule.asset_type) }}</p>
                </div>
                <span class="rounded-full px-2.5 py-1 text-xs font-semibold" :class="assetBadgeClass(rule.asset_type)">
                  {{ rule.amount ? formatAmount(rule.amount) : rule.rate_percent ? `${rule.rate_percent}%` : '规则' }}
                </span>
              </div>
              <p class="mt-3 text-sm leading-6 text-slate-600 dark:text-slate-300">{{ rule.description }}</p>
            </div>
          </div>
        </div>

        <div class="rounded-3xl border border-slate-200 bg-white p-5 shadow-sm dark:border-dark-600 dark:bg-dark-800">
          <h2 class="text-lg font-semibold text-slate-950 dark:text-white">来源分布</h2>
          <div class="mt-4 space-y-3">
            <div v-for="item in overview?.source_type_distribution || []" :key="item.source_type" class="rounded-2xl bg-slate-50 p-3 dark:bg-dark-700">
              <div class="flex items-center justify-between gap-3 text-sm">
                <span class="font-mono text-slate-700 dark:text-slate-200">{{ item.source_type }}</span>
                <span class="text-slate-500 dark:text-slate-400">{{ item.count }} 笔</span>
              </div>
              <div class="mt-2 h-2 rounded-full bg-slate-200 dark:bg-dark-600">
                <div class="h-2 rounded-full bg-indigo-500" :style="{ width: sourcePercent(item.amount) + '%' }"></div>
              </div>
              <p class="mt-1 text-xs text-slate-500 dark:text-slate-400">合计：{{ formatAmount(item.amount) }}</p>
            </div>
            <div v-if="!(overview?.source_type_distribution?.length)" class="rounded-2xl bg-slate-50 p-4 text-sm text-slate-500 dark:bg-dark-700 dark:text-slate-400">暂无积分流水来源。</div>
          </div>
        </div>
      </section>

      <section class="grid gap-4 xl:grid-cols-[0.9fr_1.1fr]">
        <form class="rounded-3xl border border-slate-200 bg-white p-5 shadow-sm dark:border-dark-600 dark:bg-dark-800" @submit.prevent="submitGrant">
          <h2 class="text-lg font-semibold text-slate-950 dark:text-white">管理员积分调整</h2>
          <p class="mt-1 text-xs text-slate-500 dark:text-slate-400">正数为发放，负数为扣减；只影响 users.credit_balance，不影响可提现余额。</p>
          <div class="mt-4 grid gap-3 md:grid-cols-2">
            <label class="space-y-1 text-xs font-medium text-slate-600 dark:text-slate-300">
              用户 ID
              <input v-model.number="grantForm.user_id" class="input" type="number" min="1" required />
            </label>
            <label class="space-y-1 text-xs font-medium text-slate-600 dark:text-slate-300">
              积分变动
              <input v-model.number="grantForm.amount" class="input" type="number" step="0.00000001" required />
            </label>
            <label class="space-y-1 text-xs font-medium text-slate-600 dark:text-slate-300">
              来源类型
              <select v-model="grantForm.source_type" class="input">
                <option value="admin_adjustment">admin_adjustment</option>
                <option value="operator_reward">operator_reward</option>
                <option value="contribution">contribution</option>
                <option value="promo_bonus">promo_bonus</option>
              </select>
            </label>
            <label class="space-y-1 text-xs font-medium text-slate-600 dark:text-slate-300">
              来源 ID
              <input v-model="grantForm.source_id" class="input" placeholder="可选，如 ticket:123" />
            </label>
            <label class="space-y-1 text-xs font-medium text-slate-600 dark:text-slate-300 md:col-span-2">
              备注
              <textarea v-model="grantForm.note" class="input min-h-24" placeholder="说明本次发放/扣减原因"></textarea>
            </label>
          </div>
          <div class="mt-4 flex items-center justify-between gap-3">
            <p class="text-xs" :class="messageToneClass">{{ formMessage }}</p>
            <button class="btn btn-primary" :disabled="granting" type="submit">{{ granting ? '提交中...' : '提交调整' }}</button>
          </div>
        </form>

        <div class="rounded-3xl border border-slate-200 bg-white p-5 shadow-sm dark:border-dark-600 dark:bg-dark-800">
          <h2 class="text-lg font-semibold text-slate-950 dark:text-white">用户积分查询</h2>
          <div class="mt-4 flex gap-2">
            <input v-model.number="userLookupId" class="input" type="number" min="1" placeholder="输入用户 ID" @keydown.enter.prevent="lookupUser" />
            <button class="btn btn-secondary" :disabled="userLookupLoading" @click="lookupUser">查询</button>
          </div>
          <div v-if="selectedUser" class="mt-4 rounded-2xl border border-slate-200 bg-slate-50 p-4 dark:border-dark-600 dark:bg-dark-700">
            <div class="flex flex-wrap items-center justify-between gap-3">
              <div>
                <p class="font-mono text-sm text-slate-500 dark:text-slate-400">#{{ selectedUser.user_id }}</p>
                <p class="font-semibold text-slate-950 dark:text-white">{{ selectedUser.email || selectedUser.username || '-' }}</p>
              </div>
              <span class="rounded-full bg-indigo-100 px-3 py-1 text-sm font-semibold text-indigo-700 dark:bg-indigo-900/40 dark:text-indigo-200">积分 {{ formatAmount(selectedUser.credit_balance) }}</span>
            </div>
            <div class="mt-4 max-h-64 space-y-2 overflow-auto">
              <div v-for="item in selectedUser.ledger" :key="item.id" class="rounded-xl bg-white p-3 text-xs dark:bg-dark-800">
                <div class="flex items-center justify-between gap-2">
                  <span class="font-mono text-slate-700 dark:text-slate-200">{{ item.source_type }}</span>
                  <span :class="item.amount >= 0 ? 'text-red-600 dark:text-red-300' : 'text-emerald-600 dark:text-emerald-300'">{{ signedAmount(item.amount) }}</span>
                </div>
                <p class="mt-1 text-slate-500 dark:text-slate-400">{{ item.note || item.source_id || '-' }}</p>
              </div>
            </div>
          </div>
        </div>
      </section>

      <section class="rounded-3xl border border-slate-200 bg-white p-5 shadow-sm dark:border-dark-600 dark:bg-dark-800">
        <div class="mb-4 flex flex-col gap-3 lg:flex-row lg:items-end lg:justify-between">
          <div>
            <h2 class="text-lg font-semibold text-slate-950 dark:text-white">积分流水</h2>
            <p class="mt-1 text-xs text-slate-500 dark:text-slate-400">支持按用户、来源、状态、时间和关键词筛选。</p>
          </div>
          <button class="btn btn-secondary" :disabled="loading" @click="loadLedger">查询</button>
        </div>
        <div class="grid gap-3 lg:grid-cols-[1fr_120px_180px_140px_150px_150px]">
          <input v-model="filters.search" class="input" placeholder="搜索邮箱 / 用户名 / 来源 / 备注" @input="debounceLedger" />
          <input v-model.number="filters.user_id" class="input" type="number" min="1" placeholder="用户ID" @change="loadLedger" />
          <input v-model="filters.source_type" class="input" placeholder="source_type" @change="loadLedger" />
          <select v-model="filters.status" class="input" @change="loadLedger">
            <option value="">全部状态</option>
            <option value="posted">posted</option>
            <option value="pending">pending</option>
            <option value="reversed">reversed</option>
          </select>
          <input v-model="filters.start_at" class="input" type="date" @change="loadLedger" />
          <input v-model="filters.end_at" class="input" type="date" @change="loadLedger" />
        </div>

        <div class="mt-4 overflow-hidden rounded-2xl border border-slate-200 dark:border-dark-600">
          <table class="min-w-full divide-y divide-slate-200 text-sm dark:divide-dark-600">
            <thead class="bg-slate-50 text-left text-xs uppercase tracking-wide text-slate-500 dark:bg-dark-700 dark:text-slate-400">
              <tr>
                <th class="px-4 py-3">ID</th>
                <th class="px-4 py-3">用户</th>
                <th class="px-4 py-3">来源</th>
                <th class="px-4 py-3">变动</th>
                <th class="px-4 py-3">余额后</th>
                <th class="px-4 py-3">状态</th>
                <th class="px-4 py-3">备注</th>
                <th class="px-4 py-3">时间</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-slate-100 bg-white dark:divide-dark-700 dark:bg-dark-800">
              <tr v-for="item in ledgerItems" :key="item.id" class="hover:bg-slate-50 dark:hover:bg-dark-700/70">
                <td class="px-4 py-3 font-mono text-xs text-slate-500">#{{ item.id }}</td>
                <td class="px-4 py-3 font-mono text-slate-700 dark:text-slate-200">{{ item.user_id }}</td>
                <td class="px-4 py-3">
                  <div class="font-mono text-xs text-slate-700 dark:text-slate-200">{{ item.source_type }}</div>
                  <div class="max-w-44 truncate text-xs text-slate-400">{{ item.source_id || '-' }}</div>
                </td>
                <td class="px-4 py-3 font-semibold" :class="item.amount >= 0 ? 'text-red-600 dark:text-red-300' : 'text-emerald-600 dark:text-emerald-300'">{{ signedAmount(item.amount) }}</td>
                <td class="px-4 py-3 text-slate-700 dark:text-slate-200">{{ formatAmount(item.balance_after) }}</td>
                <td class="px-4 py-3"><span class="rounded-full bg-slate-100 px-2 py-1 text-xs dark:bg-dark-700">{{ item.status }}</span></td>
                <td class="px-4 py-3 max-w-xs truncate text-slate-500 dark:text-slate-400">{{ item.note || '-' }}</td>
                <td class="px-4 py-3 text-xs text-slate-500 dark:text-slate-400">{{ formatDateTime(item.created_at) }}</td>
              </tr>
              <tr v-if="!loading && ledgerItems.length === 0">
                <td colspan="8" class="px-4 py-10 text-center text-slate-500 dark:text-slate-400">暂无积分流水。</td>
              </tr>
            </tbody>
          </table>
        </div>

        <div class="mt-4 flex flex-wrap items-center justify-between gap-3 text-sm text-slate-500 dark:text-slate-400">
          <span>共 {{ pagination.total }} 条，第 {{ pagination.page }} / {{ pagination.pages || 1 }} 页</span>
          <div class="flex gap-2">
            <button class="btn btn-secondary btn-sm" :disabled="pagination.page <= 1 || loading" @click="changePage(pagination.page - 1)">上一页</button>
            <button class="btn btn-secondary btn-sm" :disabled="pagination.page >= pagination.pages || loading" @click="changePage(pagination.page + 1)">下一页</button>
          </div>
        </div>
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, defineComponent, h, onMounted, reactive, ref } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import { creditSystemAPI, type CreditLedgerEntry, type CreditSystemOverview, type CreditUserSummary } from '@/api/admin/creditSystem'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import { formatDateTime as formatDisplayDateTime } from '@/utils/format'

const appStore = useAppStore()
const loading = ref(false)
const granting = ref(false)
const userLookupLoading = ref(false)
const overview = ref<CreditSystemOverview | null>(null)
const ledgerItems = ref<CreditLedgerEntry[]>([])
const selectedUser = ref<CreditUserSummary | null>(null)
const userLookupId = ref<number | null>(null)
const formMessage = ref('')
const formError = ref(false)
let debounceTimer: ReturnType<typeof setTimeout> | null = null

const filters = reactive({
  page: 1,
  page_size: 20,
  search: '',
  user_id: null as number | null,
  source_type: '',
  status: '',
  start_at: '',
  end_at: '',
})
const pagination = reactive({ page: 1, page_size: 20, total: 0, pages: 1 })
const grantForm = reactive({ user_id: null as number | null, amount: null as number | null, source_type: 'admin_adjustment', source_id: '', note: '' })

const maxSourceAmount = computed(() => Math.max(1, ...(overview.value?.source_type_distribution || []).map((item) => Math.abs(item.amount))))
const messageToneClass = computed(() => formError.value ? 'text-red-600 dark:text-red-300' : 'text-emerald-600 dark:text-emerald-300')

async function loadAll() {
  await Promise.all([loadOverview(), loadLedger()])
}

async function loadOverview() {
  try {
    overview.value = await creditSystemAPI.getOverview()
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, '加载积分系统总览失败'))
  }
}

function userTimezone(): string {
  try {
    return Intl.DateTimeFormat().resolvedOptions().timeZone
  } catch {
    return 'UTC'
  }
}

async function loadLedger() {
  loading.value = true
  try {
    const data = await creditSystemAPI.listLedger({
      page: filters.page,
      page_size: filters.page_size,
      search: filters.search.trim() || undefined,
      user_id: filters.user_id || undefined,
      source_type: filters.source_type.trim() || undefined,
      status: filters.status || undefined,
      start_at: filters.start_at || undefined,
      end_at: filters.end_at || undefined,
      timezone: userTimezone(),
    })
    ledgerItems.value = data.items
    pagination.page = data.page
    pagination.page_size = data.page_size
    pagination.total = data.total
    pagination.pages = data.pages
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, '加载积分流水失败'))
  } finally {
    loading.value = false
  }
}

function debounceLedger() {
  if (debounceTimer) clearTimeout(debounceTimer)
  debounceTimer = setTimeout(() => {
    filters.page = 1
    loadLedger()
  }, 350)
}

function changePage(page: number) {
  filters.page = Math.max(1, page)
  loadLedger()
}

async function submitGrant() {
  formMessage.value = ''
  formError.value = false
  if (!grantForm.user_id || !grantForm.amount) {
    formMessage.value = '请填写用户 ID 和非零积分变动。'
    formError.value = true
    return
  }
  granting.value = true
  try {
    await creditSystemAPI.grantCredit({
      user_id: grantForm.user_id,
      amount: grantForm.amount,
      source_type: grantForm.source_type || 'admin_adjustment',
      source_id: grantForm.source_id,
      note: grantForm.note,
    })
    formMessage.value = '积分调整已入账。'
    grantForm.amount = null
    grantForm.note = ''
    await loadAll()
  } catch (error) {
    formError.value = true
    formMessage.value = extractApiErrorMessage(error, '积分调整失败')
  } finally {
    granting.value = false
  }
}

async function lookupUser() {
  if (!userLookupId.value) return
  userLookupLoading.value = true
  try {
    selectedUser.value = await creditSystemAPI.getUserSummary(userLookupId.value)
  } catch (error) {
    selectedUser.value = null
    appStore.showError(extractApiErrorMessage(error, '查询用户积分失败'))
  } finally {
    userLookupLoading.value = false
  }
}

function sourcePercent(amount: number): number {
  return Math.max(4, Math.min(100, Math.round((Math.abs(amount) / maxSourceAmount.value) * 100)))
}

function formatAmount(value?: number | null): string {
  const n = Number(value || 0)
  return n.toLocaleString('zh-CN', { minimumFractionDigits: 0, maximumFractionDigits: 8 })
}

function signedAmount(value: number): string {
  const sign = value > 0 ? '+' : ''
  return sign + formatAmount(value)
}

function formatDateTime(value?: string | null): string {
  return value ? formatDisplayDateTime(value) : '-'
}

function assetTypeLabel(type: string): string {
  switch (type) {
    case 'credit': return '不可提现积分'
    case 'withdrawable_balance_quota': return '可提现返利额度'
    case 'mixed': return '混合奖励'
    default: return type || '规则'
  }
}

function assetBadgeClass(type: string): string {
  if (type === 'credit') return 'bg-indigo-100 text-indigo-700 dark:bg-indigo-900/40 dark:text-indigo-200'
  if (type === 'withdrawable_balance_quota') return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-200'
  return 'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-200'
}

const StatCard = defineComponent({
  name: 'CreditStatCard',
  props: {
    label: { type: String, required: true },
    value: { type: String, required: true },
    tone: { type: String, default: 'slate' },
  },
  setup(props) {
    return () => h('div', { class: 'rounded-2xl border border-slate-200 bg-white p-4 shadow-sm dark:border-dark-600 dark:bg-dark-800' }, [
      h('p', { class: 'text-xs text-slate-500 dark:text-slate-400' }, props.label),
      h('p', { class: ['mt-1 text-2xl font-bold', toneClass(props.tone)] }, props.value),
    ])
  },
})

function toneClass(tone: string): string {
  switch (tone) {
    case 'indigo': return 'text-indigo-600 dark:text-indigo-300'
    case 'emerald': return 'text-emerald-600 dark:text-emerald-300'
    case 'cyan': return 'text-cyan-600 dark:text-cyan-300'
    case 'amber': return 'text-amber-600 dark:text-amber-300'
    default: return 'text-slate-950 dark:text-white'
  }
}

onMounted(loadAll)
</script>
