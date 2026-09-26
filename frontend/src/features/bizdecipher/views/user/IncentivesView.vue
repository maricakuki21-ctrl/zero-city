<template>
  <AppLayout>
    <div class="incentives-page space-y-6">
      <header class="incentives-heading">
        <h1>激励中心</h1>
        <div><span>可用积分</span><strong>{{ balanceText }}</strong></div>
      </header>

      <section class="grid gap-4 md:grid-cols-3">
        <div class="panel p-5">
          <h2 class="incentive-title text-sm font-semibold">积分流水</h2>
          <p class="incentive-copy mt-2 text-xs leading-5">签到、邀请、管理员发放与贡献奖励会进入这里。</p>
          <RouterLink class="btn btn-secondary mt-4" to="/credits">打开积分页</RouterLink>
        </div>
        <div class="panel p-5">
          <h2 class="incentive-title text-sm font-semibold">贡献申请</h2>
          <p class="incentive-copy mt-2 text-xs leading-5">提交可复用能力、供给与服务，进入管理员审核队列。</p>
          <RouterLink class="btn btn-secondary mt-4" to="/contribute">去申请</RouterLink>
        </div>
        <div class="panel p-5">
          <h2 class="incentive-title text-sm font-semibold">Token 情报站</h2>
          <p class="incentive-copy mt-2 text-xs leading-5">周榜只统计真实付费调用，人工复核后发放激励。</p>
          <RouterLink class="btn btn-secondary mt-4" to="/community">查看情报站</RouterLink>
        </div>
      </section>

      <section class="panel">
        <div class="incentive-section-head border-b px-6 py-4 flex items-center justify-between gap-3">
          <div>
            <h2 class="incentive-title text-sm font-semibold">最近积分记录</h2>
          </div>
          <button class="btn btn-secondary" type="button" :disabled="creditsLoading" @click="loadCredits">
            {{ creditsLoading ? '刷新中…' : '刷新' }}
          </button>
        </div>
        <div class="p-6">
          <div v-if="creditsLoading && creditItems.length === 0" class="incentive-copy text-sm">正在加载积分记录…</div>
          <div v-else-if="creditsError" class="incentive-copy text-sm">
            积分记录暂时无法加载。
            <button class="btn btn-secondary ml-2" type="button" @click="loadCredits">重试</button>
          </div>
          <div v-else-if="creditItems.length === 0" class="incentive-copy text-sm">
            暂无积分记录。完成签到、邀请或获得管理员发放后会显示在这里。
          </div>
          <div v-else class="overflow-auto">
            <table class="min-w-full text-left text-sm">
              <thead class="text-xs uppercase tracking-wide text-[color:var(--zc-muted)]">
                <tr>
                  <th class="px-2 py-2">时间</th>
                  <th class="px-2 py-2">来源</th>
                  <th class="px-2 py-2">金额</th>
                  <th class="px-2 py-2">余额后</th>
                  <th class="px-2 py-2">备注</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="item in creditItems" :key="item.id" class="border-t border-[color:var(--zc-line)]">
                  <td class="px-2 py-3 whitespace-nowrap">{{ formatTime(item.created_at) }}</td>
                  <td class="px-2 py-3">{{ item.source_type || '—' }}</td>
                  <td class="px-2 py-3 font-semibold">{{ formatAmount(item.amount) }}</td>
                  <td class="px-2 py-3">{{ formatAmount(item.balance_after) }}</td>
                  <td class="px-2 py-3 max-w-[280px] truncate">{{ item.note || '—' }}</td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </section>

      <section class="panel">
        <div class="incentive-section-head border-b px-6 py-4 flex items-center justify-between gap-3">
          <div>
            <h2 class="incentive-title text-sm font-semibold">Token 情报周榜</h2>
            <p class="incentive-copy mt-0.5 text-xs">
              {{ leaderboard?.week_start || '—' }} 至 {{ leaderboard?.week_end || '—' }} · 奖励上限 ${{ leaderboard?.reward_cap ?? 200 }}
            </p>
          </div>
          <button class="btn btn-secondary" type="button" :disabled="leaderboardLoading" @click="loadLeaderboard">
            {{ leaderboardLoading ? '刷新中…' : '刷新' }}
          </button>
        </div>
        <div class="p-6">
          <div v-if="leaderboardLoading && !(leaderboard?.rows?.length)" class="incentive-copy text-sm">正在加载周榜…</div>
          <div v-else-if="leaderboardError" class="incentive-copy text-sm">
            周榜暂时无法加载。
            <button class="btn btn-secondary ml-2" type="button" @click="loadLeaderboard">重试</button>
          </div>
          <div v-else-if="!(leaderboard?.rows?.length)" class="incentive-copy text-sm">
            本周暂无有效付费调用上榜数据。
          </div>
          <div v-else class="overflow-auto">
            <table class="min-w-full text-left text-sm">
              <thead class="text-xs uppercase tracking-wide text-[color:var(--zc-muted)]">
                <tr>
                  <th class="px-2 py-2">排名</th>
                  <th class="px-2 py-2">用户</th>
                  <th class="px-2 py-2">有效消费</th>
                  <th class="px-2 py-2">请求数</th>
                  <th class="px-2 py-2">审核状态</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="row in leaderboard.rows" :key="`${row.rank}-${row.user_id}`" class="border-t border-[color:var(--zc-line)]">
                  <td class="px-2 py-3">#{{ row.rank }}</td>
                  <td class="px-2 py-3">{{ row.display_name || row.user_id }}</td>
                  <td class="px-2 py-3 font-semibold">${{ Number(row.effective_spend || 0).toFixed(2) }}</td>
                  <td class="px-2 py-3">{{ row.request_count || 0 }}</td>
                  <td class="px-2 py-3">{{ row.review_status || 'pending_review' }}</td>
                </tr>
              </tbody>
            </table>
          </div>
          <ul v-if="leaderboard?.rules?.length" class="mt-4 space-y-1">
            <li v-for="rule in leaderboard.rules" :key="rule" class="incentive-copy text-xs">· {{ rule }}</li>
          </ul>
        </div>
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import { listMyBizCredits, type BizCreditLedgerEntry } from '@/features/bizdecipher/api/bizdecipher'
import { communityAPI, type TokenPowerLeaderboard } from '@/features/bizdecipher/api/community'

const creditsLoading = ref(false)
const creditsError = ref(false)
const creditItems = ref<BizCreditLedgerEntry[]>([])
const leaderboardLoading = ref(false)
const leaderboardError = ref(false)
const leaderboard = ref<TokenPowerLeaderboard | null>(null)

const balanceText = computed(() => {
  if (!creditItems.value.length) return '—'
  const latest = creditItems.value[0]
  const value = Number(latest?.balance_after)
  if (Number.isNaN(value)) return '—'
  return value.toFixed(2)
})

function formatAmount(value?: number): string {
  if (value === null || value === undefined || Number.isNaN(Number(value))) return '—'
  const n = Number(value)
  return `${n >= 0 ? '+' : ''}${n.toFixed(4)}`
}

function formatTime(value?: string): string {
  if (!value) return '—'
  const d = new Date(value)
  if (Number.isNaN(d.getTime())) return value
  return d.toLocaleString()
}

async function loadCredits(): Promise<void> {
  creditsLoading.value = true
  creditsError.value = false
  try {
    const res = await listMyBizCredits()
    creditItems.value = Array.isArray(res?.items) ? res.items : Array.isArray(res) ? res as BizCreditLedgerEntry[] : []
  } catch {
    creditsError.value = true
    creditItems.value = []
  } finally {
    creditsLoading.value = false
  }
}

async function loadLeaderboard(): Promise<void> {
  leaderboardLoading.value = true
  leaderboardError.value = false
  try {
    leaderboard.value = await communityAPI.getTokenPower(10)
  } catch {
    leaderboardError.value = true
    leaderboard.value = null
  } finally {
    leaderboardLoading.value = false
  }
}

onMounted(async () => {
  await Promise.all([loadCredits(), loadLeaderboard()])
})
</script>

<style scoped>
.incentives-heading{display:flex;align-items:center;justify-content:space-between;gap:16px;padding-bottom:18px;border-bottom:1px solid var(--zc-line)}.incentives-heading h1{font-size:22px;font-weight:600;color:var(--zc-text-strong)}.incentives-heading>div{display:flex;align-items:baseline;gap:10px}.incentives-heading span{font-size:12px;color:var(--zc-muted)}.incentives-heading strong{font-size:22px;font-weight:600;font-variant-numeric:tabular-nums}
.incentives-page .panel{border-radius:0;box-shadow:none;background:transparent;border:0;border-bottom:1px solid var(--zc-line)}
.incentives-page {
  color: var(--zc-text);
}

.incentive-title {
  color: var(--zc-text-strong);
}

.incentive-copy {
  color: var(--zc-muted);
}

.incentive-chip,
.incentive-icon {
  color: var(--zc-accent);
  background: var(--zc-bg);
  box-shadow:
    inset 3px 3px 7px var(--zc-shadow-dark),
    inset -3px -3px 7px var(--zc-shadow-light);
}

.incentive-card,
.incentive-tile {
  border: 0;
  background: var(--zc-bg);
  box-shadow:
    7px 7px 16px var(--zc-shadow-dark),
    -7px -7px 16px var(--zc-shadow-light);
}

.incentive-tile {
  box-shadow:
    inset 5px 5px 12px var(--zc-shadow-dark),
    inset -5px -5px 12px var(--zc-shadow-light);
}

.incentive-section-head {
  border-color: var(--zc-line) !important;
}
</style>
