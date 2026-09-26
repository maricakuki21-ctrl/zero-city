<template>
  <AppLayout>
    <div class="op-page">
      <RouterLink to="/account-square" class="op-back">← 返回池主中转站</RouterLink>

      <section v-if="loading" class="op-card op-empty">加载中…</section>
      <section v-else-if="!pool" class="op-card op-empty">找不到这个共享池，或当前账号不是池主。</section>

      <template v-else>
        <section class="op-hero">
          <div>
            <p class="op-eyebrow">Pool Owner Console</p>
            <h1>{{ pool.name }}</h1>
            <p>集中管理上游账号、模型、检测、成员席位与收益记录。</p>
          </div>
          <div class="op-hero-actions">
            <button class="op-btn" :disabled="syncingModelsKey === String(pool.id)" @click="syncUpstreamModels(pool.id)">
              {{ syncingModelsKey === String(pool.id) ? '拉取中…' : '一键拉取模型' }}
            </button>
            <button class="op-btn" :disabled="probingPoolKey === String(pool.id)" @click="probePool({ poolId: pool.id })">
              {{ probingPoolKey === String(pool.id) ? '检测中…' : '发起满血检测' }}
            </button>
            <button class="op-btn op-btn-primary" @click="refreshPoolDetail">刷新</button>
          </div>
        </section>

        <section class="op-grid op-grid-four">
          <div class="op-metric">
            <span>状态</span>
            <b>{{ pool.status }}</b>
          </div>
          <div class="op-metric">
            <span>池模式</span>
            <b>{{ pool.account_mode_enabled ? '多账号' : '单凭证' }}</b>
          </div>
          <div class="op-metric">
            <span>成员席位</span>
            <b>{{ pool.current_users }}/{{ pool.max_users }}</b>
          </div>
          <div class="op-metric">
            <span>调用倍率</span>
            <b>{{ pool.rate_multiplier }}x</b>
          </div>
          <div class="op-metric">
            <span>可用率</span>
            <b>{{ pool.today_availability || 0 }}%</b>
          </div>
        </section>

        <section class="op-card op-mode-card">
          <div>
            <h2>池模式</h2>
            <p class="op-note">单凭证模式适合一个上游账号；多账号模式可统一管理多个账号。切换后需重新检测并上架。</p>
          </div>
          <div class="op-mode-actions">
            <button class="op-btn" :class="{ 'op-btn-selected': !pool.account_mode_enabled }" :disabled="savingPoolId === pool.id" @click="changePoolMode(false)">单凭证模式</button>
            <button class="op-btn" :class="{ 'op-btn-selected': pool.account_mode_enabled }" :disabled="savingPoolId === pool.id" @click="changePoolMode(true)">多账号池模式</button>
          </div>
        </section>

        <section v-if="poolModels.length > 0" class="op-card">
          <div class="op-card-head">
            <div>
              <h2>上游模型候选</h2>
              <p>查看上游当前返回的模型，再选择需要开放的模型并设置倍率与并发。</p>
            </div>
            <b>{{ poolModels.length }} 个</b>
          </div>
          <div class="op-models">
            <span v-for="modelName in poolModels" :key="modelName">{{ modelName }}</span>
          </div>
        </section>

        <section class="op-grid op-grid-two">
          <div class="op-card">
            <div class="op-card-head">
              <div>
                <h2>账号 / API Key</h2>
                <p>池主接入上游资源，普通用户不可见上游 Key。</p>
              </div>
              <button class="op-btn op-btn-sm" @click="loadPoolAccounts(pool.id)">刷新账号</button>
            </div>
            <div v-if="accountLoadingPoolId === pool.id" class="op-empty-sm">加载账号…</div>
            <div v-else-if="poolAccountRows.length === 0" class="op-empty-sm">暂无账号。回到池主中转站添加或批量导入账号。</div>
            <div v-else class="op-list">
              <div v-for="account in poolAccountRows" :key="account.id" class="op-row">
                <div>
                  <b>{{ account.name || account.upstream_base_url }}</b>
                  <span>{{ account.provider || 'openai' }} · {{ account.status }}</span>
                </div>
                <div class="op-row-actions">
                  <button class="op-btn op-btn-xs" :disabled="syncingModelsKey === `${pool.id}-${account.id}`" @click="syncUpstreamModels(pool.id, account.id)">
                    {{ syncingModelsKey === `${pool.id}-${account.id}` ? '拉取中…' : '拉取模型' }}
                  </button>
                  <button class="op-btn op-btn-xs" :disabled="probingPoolKey === `${pool.id}-${account.id}`" @click="probePool({ poolId: pool.id, accountId: account.id })">
                    检测
                  </button>
                </div>
                <div v-if="accountModels(account.id).length > 0" class="op-account-models">
                  <span v-for="modelName in accountModels(account.id).slice(0, 8)" :key="modelName">{{ modelName }}</span>
                  <em v-if="accountModels(account.id).length > 8">+{{ accountModels(account.id).length - 8 }}</em>
                </div>
              </div>
            </div>
          </div>

          <div class="op-card">
            <div class="op-card-head">
              <div>
                <h2>成员与席位</h2>
                <p>展示真实成员席位，支持移除异常成员。</p>
              </div>
              <button class="op-btn op-btn-sm" @click="loadPoolMembers(pool.id)">刷新成员</button>
            </div>
            <div v-if="memberLoadingPoolId === pool.id" class="op-empty-sm">加载成员…</div>
            <div v-else-if="poolMemberRows.length === 0" class="op-empty-sm">暂无成员。</div>
            <div v-else class="op-list">
              <div v-for="member in poolMemberRows" :key="member.id" class="op-row">
                <div>
                  <b>用户 #{{ member.user_id }}</b>
                  <span>{{ member.status }} · 已扣 {{ formatBalance(member.total_charged || 0) }}</span>
                </div>
                <button class="op-btn op-btn-xs op-btn-danger" :disabled="removingSeatId === member.id" @click="removePoolMember(pool.id, member.id)">
                  {{ removingSeatId === member.id ? '移除中…' : '移除' }}
                </button>
              </div>
            </div>
          </div>
        </section>

        <section class="op-grid op-grid-two">
          <div class="op-card">
            <h2>收益账本</h2>
            <p class="op-note">API 分润和席位费收益都从真实共享池账本读取，不混入积分余额。</p>
            <div v-if="poolLedgerRows.length === 0" class="op-empty-sm">暂无这个池的收益记录。</div>
            <div v-else class="op-list">
              <div v-for="entry in poolLedgerRows" :key="entry.id" class="op-row">
                <span>{{ entry.source_type }} · {{ entry.note || '' }}</span>
                <b>{{ entry.amount > 0 ? '+' : '' }}{{ formatBalance(entry.amount) }}</b>
              </div>
            </div>
          </div>

          <div class="op-card">
            <h2>投诉与治理</h2>
            <p class="op-note">投诉处理、降权/下架/封禁仍由平台治理中枢执行。池主侧只展示状态，不提供平台抽水配置。</p>
            <div class="op-governance-list">
              <div><span>治理状态</span><b>{{ pool.governance_status || 'normal' }}</b></div>
              <div><span>投诉数</span><b>{{ pool.complaint_count || 0 }}</b></div>
              <div><span>平台抽水</span><b>平台统一管理</b></div>
              <div><span>公开状态</span><b>{{ pool.listed ? '已上架' : '未上架' }}</b></div>
            </div>
          </div>
        </section>
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import type { SharedPool } from '@/features/bizdecipher/api/bizdecipher'
import { usePoolOwner } from '@/composables/usePoolOwner'

const route = useRoute()
const loading = ref(false)
const pool = ref<SharedPool | null>(null)

const {
  myPools, sharedPoolLedger, poolAccounts, poolMembers,
  accountLoadingPoolId, memberLoadingPoolId, probingPoolKey, removingSeatId,
  syncingModelsKey, syncedModels, savingPoolId,
  loadMyPools, loadSharedPoolLedger, loadPoolAccounts, loadPoolMembers,
  updatePool, probePool, syncUpstreamModels, removePoolMember,
} = usePoolOwner()

const poolId = computed(() => Number(route.params.id || 0))
const poolAccountRows = computed(() => poolAccounts[poolId.value] || [])
const poolMemberRows = computed(() => poolMembers[poolId.value] || [])
const poolLedgerRows = computed(() => (sharedPoolLedger.value.withdrawable || []).filter((entry) => entry.pool_id === poolId.value).slice(0, 12))
const poolModels = computed(() => syncedModels[String(poolId.value)] || [])

function accountModels(accountId: number): string[] {
  return syncedModels[`${poolId.value}-${accountId}`] || []
}

function formatBalance(value: number): string {
  return Number(value || 0).toFixed(4)
}

async function changePoolMode(enabled: boolean): Promise<void> {
  if (!pool.value || pool.value.account_mode_enabled === enabled) return
  if (enabled && poolAccountRows.value.length === 0) {
    const confirmed = window.confirm('当前还没有池账号。切换后池子会保持未上架，请继续添加账号并完成满血检测。是否继续？')
    if (!confirmed) return
  }
  const confirmed = window.confirm('切换池模式会将池子设为未上架，并要求重新完成满血检测。不会恢复旧席位或旧绑定。是否继续？')
  if (!confirmed) return
  await updatePool(pool.value.id, {
    expected_config_version: Number(pool.value.config_version || 0),
    account_mode_enabled: enabled,
    listed: false,
  })
  await refreshPoolDetail()
}

async function refreshPoolDetail(): Promise<void> {
  if (!poolId.value) return
  loading.value = true
  try {
    await loadMyPools()
    pool.value = myPools.value.find((item) => item.id === poolId.value) || null
    if (!pool.value) return
    await Promise.all([loadPoolAccounts(poolId.value), loadPoolMembers(poolId.value), loadSharedPoolLedger()])
  } catch {
    pool.value = null
  } finally {
    loading.value = false
  }
}

onMounted(refreshPoolDetail)
</script>

<style scoped>
.op-page {
  --bg: #1c1e22;
  --nd: #13151a;
  --nl: #272a30;
  --text: #e8e4df;
  --muted: rgba(232, 228, 223, .56);
  --teal: #5aa7a4;
  --gold: #f2cf72;
  --raise: 8px 8px 16px var(--nd), -8px -8px 16px var(--nl);
  --inset: inset 5px 5px 10px var(--nd), inset -5px -5px 10px var(--nl);
  max-width: 1180px;
  margin: 0 auto;
  padding: 28px 28px 80px;
  color: var(--text);
  display: flex;
  flex-direction: column;
  gap: 18px;
}
:root.theme-daylight .op-page {
  --bg: #e8edf3;
  --nd: #c5cad2;
  --nl: #ffffff;
  --text: #1a2030;
  --muted: rgba(26, 32, 48, .56);
  --teal: #3d9e9b;
  --gold: #b07d2a;
}
.op-back { color: var(--muted); font-size: 13px; font-weight: 800; text-decoration: none; }
.op-hero, .op-card, .op-metric {
  background: var(--bg);
  box-shadow: var(--raise);
  border-radius: 24px;
}
.op-hero { display: flex; justify-content: space-between; gap: 18px; padding: 24px; }
.op-eyebrow { margin: 0; color: var(--gold); font-size: 11px; font-weight: 900; letter-spacing: .22em; text-transform: uppercase; }
.op-hero h1 { margin: 8px 0 0; font-size: clamp(24px, 4vw, 40px); font-weight: 950; letter-spacing: -.06em; }
.op-hero p, .op-note { margin: 8px 0 0; color: var(--muted); font-size: 13px; line-height: 1.7; }
.op-hero-actions { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; }
.op-mode-card { display: flex; align-items: center; justify-content: space-between; gap: 18px; }
.op-mode-actions { display: flex; gap: 10px; flex-wrap: wrap; }
.op-btn-selected { color: var(--teal); box-shadow: var(--inset); }
.op-grid { display: grid; gap: 14px; }
.op-grid-four { grid-template-columns: repeat(4, minmax(0, 1fr)); }
.op-grid-two { grid-template-columns: repeat(2, minmax(0, 1fr)); }
.op-metric { padding: 18px; display: flex; flex-direction: column; gap: 8px; }
.op-metric span, .op-row span, .op-governance-list span { color: var(--muted); font-size: 12px; font-weight: 700; }
.op-metric b { font-size: 22px; letter-spacing: -.05em; }
.op-card { padding: 20px; }
.op-card h2 { margin: 0; font-size: 18px; font-weight: 900; letter-spacing: -.04em; }
.op-card-head { display: flex; align-items: flex-start; justify-content: space-between; gap: 12px; margin-bottom: 14px; }
.op-card-head p { margin: 5px 0 0; color: var(--muted); font-size: 12px; }
.op-list { display: flex; flex-direction: column; gap: 8px; }
.op-row { display: flex; align-items: center; justify-content: space-between; gap: 12px; padding: 12px; border-radius: 14px; box-shadow: var(--inset); }
.op-row div { min-width: 0; display: flex; flex-direction: column; gap: 4px; }
.op-row b { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.op-row-actions { flex-direction: row !important; flex-shrink: 0; }
.op-models, .op-account-models { display: flex; flex-wrap: wrap; gap: 8px; }
.op-models span, .op-account-models span, .op-account-models em { padding: 7px 10px; border-radius: 999px; box-shadow: var(--inset); color: var(--muted); font-size: 11px; font-style: normal; font-weight: 800; }
.op-account-models { width: 100%; margin-top: 8px; }
.op-governance-list { display: grid; gap: 8px; margin-top: 14px; }
.op-governance-list div { display: flex; justify-content: space-between; gap: 12px; padding: 12px; border-radius: 14px; box-shadow: var(--inset); }
.op-empty, .op-empty-sm { text-align: center; color: var(--muted); }
.op-empty { padding: 46px; }
.op-empty-sm { padding: 22px; font-size: 13px; }
.op-btn { min-height: 36px; padding: 0 14px; border: 0; border-radius: 12px; background: var(--bg); box-shadow: var(--raise); color: var(--text); font-weight: 800; cursor: pointer; }
.op-btn:disabled { opacity: .55; cursor: not-allowed; }
.op-btn-primary { color: #f5f0e8; background: #1a2030; }
.op-btn-sm { min-height: 32px; font-size: 12px; }
.op-btn-xs { min-height: 28px; padding: 0 10px; font-size: 11px; }
.op-btn-danger { color: #dc2626; }
@media (max-width: 860px) {
  .op-page { padding: 16px 16px 60px; }
  .op-hero, .op-mode-card { flex-direction: column; align-items: stretch; }
  .op-grid-four, .op-grid-two { grid-template-columns: 1fr; }
}
</style>
