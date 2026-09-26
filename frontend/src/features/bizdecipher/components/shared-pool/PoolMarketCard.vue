<template>
  <ResourceDiscoveryCard
    class="pmc compact-pool"
    :to="`/account-square/pool/${pool.id}`"
    :title="pool.name"
    :image-url="coverImage"
    :fallback-image-url="pool.avatarUrl || zeroCityMascots.sharedPoolOwner"
    :image-alt="`${pool.name}封面`"
    :image-position="cardPresentation ? 'center 28%' : 'center bottom'"
    :image-fit="cardPresentation ? 'cover' : 'contain'"
    :quote="cardPresentation?.line || ''"
    :cover-note="cardPresentation?.badgeLabel || ''"
    :provider="`池主 ${pool.owner}`"
    :state-label="joined ? '已加入' : ''"
    source-label="共享池"
    source-tone="shared"
    :status="statusLabel"
    :status-tone="statusTone"
    :tags="visibleModels"
    :metrics="metrics"
    :verification-label="cardPresentation ? `${cardPresentation.badgeLabel} · ${pool.verificationBadgeLabel}` : pool.verificationBadgeLabel"
    :link-label="`查看共享池 ${pool.name} 的详情`"
  >
    <template #evidence>
      <div class="pmc-history-area">
        <PoolHistoryBar :pool-id="pool.id" @observed="observation = $event" />
      </div>
    </template>

    <template #footer>
      <button
        class="pmc-like-btn"
        :class="{ liked: pool.likedByMe, acting: likeActing }"
        type="button"
        :disabled="likeActing"
        :aria-pressed="pool.likedByMe"
        :aria-label="pool.likedByMe ? `取消喜欢 ${pool.name}` : `喜欢 ${pool.name}`"
        @click="$emit('like', pool)"
      >
        <Heart :size="15" :fill="pool.likedByMe ? 'currentColor' : 'none'" />
        <em>{{ pool.likes || 0 }}</em>
      </button>
      <RouterLink
        :to="`/account-square/pool/${pool.id}`"
        class="pmc-discuss-btn"
        :aria-label="`查看 ${pool.name} 的讨论`"
      >
        <MessageCircle :size="15" />
        <span>{{ summary.total_posts || 0 }}</span>
      </RouterLink>
      <details class="pmc-more">
      <summary title="更多操作" aria-label="更多操作"><Ellipsis :size="16" /></summary>
      <button
        class="pmc-report-btn"
        type="button"
        :aria-label="`投诉 ${pool.name}`"
        title="投诉"
        @click="$emit('report', pool)"
      >
        <Flag :size="14" /> 投诉
      </button>
      </details>
      <span class="resource-card-spacer" />
      <button
        v-if="!joined"
        class="pmc-join-btn"
        type="button"
        :disabled="acting || joinDisabled"
        :aria-label="joinDisabled ? (joinDisabledLabel || `${pool.name} 暂不可加入`) : `加入 ${pool.name}`"
        :title="joinDisabled ? (joinDisabledLabel || '暂不可加入') : ''"
        @click="!joinDisabled && $emit('join', pool)"
      >
        {{ acting ? '…' : (joinDisabled ? (joinDisabledLabel || '暂不可加入') : '加入') }}
      </button>
      <button
        v-else
        class="pmc-leave-btn"
        type="button"
        :disabled="acting"
        :aria-label="`退出 ${pool.name}`"
        @click="$emit('leave', pool)"
      >
        退出
      </button>
    </template>
  </ResourceDiscoveryCard>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { Ellipsis, Flag, Heart, MessageCircle } from '@lucide/vue'
import ResourceDiscoveryCard from './ResourceDiscoveryCard.vue'
import type { ResourceMetric } from './resourceDiscovery'
import PoolHistoryBar from './PoolHistoryBar.vue'
import { summarizeProbeObservations, type ProbeObservation } from './probeObservation'
const observation = ref<ProbeObservation>(summarizeProbeObservations([], Date.now()))
import { zeroCityMascots } from '@/features/bizdecipher/constants/zeroCityMascots'
import type { PoolVM } from '@/composables/useSharedPool'
import type { SharedPoolCommunitySummary } from '@/features/bizdecipher/api/community'
import { formatSharedPoolMultiplier } from './sharedPoolPricing'
import {
  resolveSharedPoolCardPresentation,
  sharedPoolDecision,
  sharedPoolServiceGrade,
} from './sharedPoolPresentation'

const props = defineProps<{
  pool: PoolVM
  joined: boolean
  acting: boolean
  likeActing?: boolean
  joinDisabled?: boolean
  joinDisabledLabel?: string
  summary: SharedPoolCommunitySummary
}>()

defineEmits<{
  join: [pool: PoolVM]
  leave: [pool: PoolVM]
  report: [pool: PoolVM]
  like: [pool: PoolVM]
}>()

const cardPresentation = computed(() => resolveSharedPoolCardPresentation({
  cardKey: props.pool.cardSkinKey,
  cardRarity: props.pool.cardSkinRarity,
  fallbackCardKey: props.pool.ownerCardAsset.featured_card_key,
  fallbackCardRarity: props.pool.ownerCardAsset.featured_card_rarity,
}))

const coverImage = computed(() => cardPresentation.value?.imageUrl || '')

const statusLabel = computed(() => {
  if (props.pool.observation) return '观察中'
  if (props.pool.status === 'healthy') {
    if (observation.value.state === 'error') return '检测暂不可读'
    if (observation.value.state === 'stale') return '待重新检测'
    return observation.value.latest === null ? '待检测' : observation.value.latest ? '最近检测通过' : '最近检测失败'
  }
  return ({
    healthy: '当前可用',
    limited: '受限',
    offline: '已下线',
    maintenance: '维护中',
    unknown: '待验证',
  } as Record<string, string>)[props.pool.status] ?? props.pool.status
})

const statusTone = computed<ResourceMetric['tone']>(() => {
  if (props.pool.observation) return 'watch'
  if (props.pool.status === 'healthy') return observation.value.latest === null ? 'muted' : observation.value.latest ? 'good' : 'watch'
  if (props.pool.status === 'unknown') return 'muted'
  if (props.pool.status === 'offline' || props.pool.status === 'maintenance') return 'danger'
  if (props.pool.status === 'limited' || props.pool.observation) return 'watch'
  return 'good'
})

function normalizedAvailability(value: number): number {
  const numeric = Number(value)
  if (!Number.isFinite(numeric)) return 0
  return Math.max(0, Math.min(100, numeric <= 1 ? numeric * 100 : numeric))
}

const hasProbeEvidence = computed(() => Boolean(
  props.pool.lastProbeAt
  || props.pool.lastSuccessfulProbeAt
  || props.pool.lastProbeFullCheckTotal > 0
  || props.pool.consecutiveProbeFailures > 0
))

const hasServiceEvidence = computed(() => props.pool.status !== 'unknown' && Boolean(
  hasProbeEvidence.value
  || props.pool.accountSummary.total_calls > 0
  || normalizedAvailability(props.pool.sevenDayAvailability) > 0
))

const serviceGrade = computed(() => sharedPoolServiceGrade({
  status: props.pool.status,
  observation: props.pool.observation,
  todayAvailability: props.pool.todayAvailability,
  sevenDayAvailability: props.pool.sevenDayAvailability,
  avgLatencyMs: props.pool.latency,
  hasEvidence: hasServiceEvidence.value,
}))

const decision = computed(() => sharedPoolDecision({
  status: props.pool.status,
  observation: props.pool.observation,
  todayAvailability: props.pool.todayAvailability,
  sevenDayAvailability: props.pool.sevenDayAvailability,
  avgLatencyMs: props.pool.latency,
  hasEvidence: hasServiceEvidence.value,
}))

const visibleModels = computed(() => {
  const values = props.pool.models.slice(0, 3)
  const extra = Math.max(0, props.pool.models.length - values.length)
  if (extra > 0) values.push(`+${extra} 个模型`)
  return values.length ? values : ['模型待公布']
})

function formatCharge(value: number): string {
  const amount = Number(value || 0)
  return amount.toLocaleString(undefined, { maximumFractionDigits: 6 })
}

const feeText = computed(() => {
  if (props.pool.hourlyFee > 0) return `席位 ${formatCharge(props.pool.hourlyFee)}/时`
  return '免席位费'
})

const feeNote = computed(() => {
  const waiver = props.pool.hourlyFee > 0 && props.pool.hourlyMinUsageWaiver > 0
    ? `用量达 ${formatCharge(props.pool.hourlyMinUsageWaiver)} 减免`
    : ''
  return [`单价 ×${formatSharedPoolMultiplier(props.pool.rate)}`, waiver].filter(Boolean).join(' · ')
})

const remainingSeatText = computed(() => {
  if (!props.pool.maxUsers) return '不限'
  return `${Math.max(0, props.pool.maxUsers - props.pool.currentUsers)} 席`
})

const seatUsageText = computed(() => {
  if (!props.pool.maxUsers) return '席位不限'
  return `${props.pool.currentUsers}/${props.pool.maxUsers} 已使用`
})

const seatTone = computed<ResourceMetric['tone']>(() => {
  if (!props.pool.maxUsers) return 'good'
  const ratio = props.pool.currentUsers / props.pool.maxUsers
  if (ratio >= 0.92) return 'danger'
  if (ratio >= 0.72) return 'watch'
  return 'good'
})

const metrics = computed<ResourceMetric[]>(() => [
  {
    label: '近期检测通过率',
    value: observation.value.state === 'error' ? '读取失败' : observation.value.count ? `${(observation.value.passed * 100 / observation.value.count).toFixed(1)}%` : '待检测',
    note: observation.value.state === 'error' ? '请重试，不代表服务故障'
      : observation.value.state === 'stale' ? '历史样本 · 待更新'
        : observation.value.count ? `最近 ${observation.value.count} 次${observation.value.smallSample ? ' · 样本较少' : '检测'}`
          : '无样本不代表故障',
    tone: observation.value.state !== 'fresh' || observation.value.smallSample ? 'muted'
      : observation.value.passed < observation.value.count ? 'watch' : 'good',
  },
  {
    label: '计费规则',
    value: feeText.value,
    note: feeNote.value,
  },
  {
    label: '剩余席位',
    value: remainingSeatText.value,
    note: seatUsageText.value,
    tone: seatTone.value,
  },
])

defineExpose({ serviceGrade, decision })
</script>

<style scoped>
.pmc-history-area {
  padding: 0;
}
.pmc-history-area :deep(.pool-history) { padding: 0; }
.pmc-history-area :deep(.pool-history-bars span) { height: 9px; }
.pmc-more { position: relative; }
.pmc-more summary { display: flex; align-items: center; justify-content: center; width: 30px; height: 30px; cursor: pointer; list-style: none; }
.pmc-more summary::-webkit-details-marker { display: none; }
.pmc-more[open] .pmc-report-btn { position: absolute; bottom: 34px; left: 0; z-index: 3; padding: 8px 12px; white-space: nowrap; background: var(--bd-surface); border: 1px solid var(--bd-ui-line); border-radius: 4px; }
.pmc-more summary:focus-visible { outline: 2px solid var(--bd-accent-teal); }
.pmc-history-area :deep(.pool-history-footer) { display: none; }

.pmc-like-btn,
.pmc-report-btn,
.pmc-discuss-btn,
.pmc-join-btn,
.pmc-leave-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 5px;
  min-height: 30px;
  padding: 0 8px;
  border: 0;
  border-radius: 5px;
  background: transparent;
  color: var(--bd-text-secondary);
  font-size: 12px;
  text-decoration: none;
}

.pmc-like-btn:hover,
.pmc-report-btn:hover,
.pmc-discuss-btn:hover {
  background: var(--bd-canvas);
  color: var(--bd-text-primary);
}

.pmc-like-btn.liked {
  color: var(--bd-status-danger);
}

.pmc-like-btn em {
  font-style: normal;
}

.pmc-join-btn {
  padding: 0 14px;
  background: var(--bd-accent-teal);
  color: var(--bd-on-action, #fff);
  font-weight: 600;
}

.pmc-leave-btn {
  padding: 0 12px;
  border: 1px solid var(--bd-ui-line);
  background: var(--bd-surface);
}

.pmc-like-btn:disabled,
.pmc-report-btn:disabled,
.pmc-join-btn:disabled,
.pmc-leave-btn:disabled {
  cursor: not-allowed;
  opacity: 0.55;
}

@media (max-width: 520px) {
  .pmc-join-btn,
  .pmc-leave-btn {
    margin-left: auto;
  }
}
</style>
