<script setup lang="ts">
import { computed, nextTick, onScopeDispose, reactive, ref, watch } from 'vue'
import { Check, Circle, ClipboardList, Plus, RefreshCw, Vote, X } from '@lucide/vue'
import { communityAPI, getCommunityParticipation, type CommunityParticipation, type CommunityAnnouncementPolicy, type CommunityPoll, type CreateCommunityPollPayload } from '@/features/bizdecipher/api/community'
import PollDiscussion from './PollDiscussion.vue'
import { useAuthStore } from '@/stores/auth'
import { extractActionableApiErrorMessage } from '@/utils/apiError'

const auth = useAuthStore()
const props = defineProps<{ focusPollId?: number }>()
const participation = ref<CommunityParticipation | null>(null)
const policy = ref<CommunityAnnouncementPolicy | null>(null)
const proposalKind = ref<NonNullable<CommunityPoll['proposal_kind']>>('general')
const discussionId = ref(0)
const kindFilter = ref('all')
const stateFilter = ref('all')
const visiblePolls = computed(() => polls.value.filter(poll =>
  (kindFilter.value === 'all' || (poll.proposal_kind || 'general') === kindFilter.value) &&
  (stateFilter.value === 'all' || poll.status === stateFilter.value)))
const kinds = { general: '普通议题', announcement: '玩家公告', activity: '城市活动', improvement: '网站建议', rule: '规则提案' }
const canParticipate = computed(() => isAuthenticated.value && Boolean(participation.value?.is_admin || (participation.value?.level ?? 0) >= 1))
const polls = ref<CommunityPoll[]>([])
const loading = ref(false)
const saving = ref(false)
const busyPollId = ref(0)
const error = ref('')
const notice = ref('')
const createOpen = ref(false)
const title = ref('')
const body = ref('')
const closesAt = ref('')
const optionDrafts = ref(['', ''])
const selections = reactive<Record<number, number>>({})
let listRequestId = 0
let identityEpoch = 0
const mutationPending = computed(() => saving.value || busyPollId.value !== 0)

function invalidateList(): void {
  listRequestId++
  loading.value = false
}

const isAuthenticated = computed(() => Boolean(auth.isAuthenticated && auth.user?.id))
const toMessage = (value: unknown, fallback: string) => extractActionableApiErrorMessage(value, fallback)

function syncSelections(items: readonly CommunityPoll[]): void {
  for (const poll of items) {
    if (poll.viewer_option_id > 0) selections[poll.id] = poll.viewer_option_id
  }
}

function replacePoll(updated: CommunityPoll): void {
  const index = polls.value.findIndex((poll) => poll.id === updated.id)
  if (index >= 0) polls.value.splice(index, 1, updated)
  else polls.value.unshift(updated)
  syncSelections([updated])
}

async function loadPolls(): Promise<void> {
  if (mutationPending.value) return
  const requestId = ++listRequestId
  loading.value = true
  error.value = ''
  try {
    const [response, access] = await Promise.all([communityAPI.listPolls(), isAuthenticated.value ? getCommunityParticipation() : Promise.resolve(null)])
    if (requestId !== listRequestId) return
    participation.value = access
    policy.value = response.announcement_policy ?? null
    if (props.focusPollId && !response.items.some(poll => poll.id === props.focusPollId)) {
      const focused = await communityAPI.getPoll(props.focusPollId)
      if (requestId !== listRequestId) return
      response.items.unshift(focused)
    }
    polls.value = response.items
    syncSelections(response.items)
    if (props.focusPollId) {
      discussionId.value = props.focusPollId
      await nextTick()
      if (requestId === listRequestId) document.getElementById(`city-poll-${props.focusPollId}`)?.scrollIntoView?.({ block: 'start' })
    }
  } catch (value) {
    if (requestId === listRequestId) error.value = toMessage(value, '投票暂时没有加载成功，请重试。')
  } finally {
    if (requestId === listRequestId) loading.value = false
  }
}

function addOption(): void {
  if (optionDrafts.value.length < 8) optionDrafts.value.push('')
}

function removeOption(index: number): void {
  if (optionDrafts.value.length > 2) optionDrafts.value.splice(index, 1)
}

function resetCreate(): void {
  title.value = ''
  body.value = ''
  closesAt.value = ''
  optionDrafts.value = ['', '']
  proposalKind.value = 'general'
  createOpen.value = false
}

async function createPoll(): Promise<void> {
  if (!canParticipate.value || mutationPending.value) return
  const options = proposalKind.value === 'announcement' ? ['支持发布', '暂不发布'] : optionDrafts.value.map((option) => option.trim()).filter(Boolean)
  if (!title.value.trim() || !body.value.trim() || options.length < 2) {
    error.value = '请填写议题、说明和至少两个选项。'
    return
  }
  const payload: CreateCommunityPollPayload = { title: title.value.trim(), body: body.value.trim(), options, proposal_kind: proposalKind.value }
  if (closesAt.value && proposalKind.value !== 'announcement') payload.closes_at = new Date(closesAt.value).toISOString()
  const epoch = identityEpoch
  invalidateList()
  saving.value = true
  error.value = ''
  notice.value = ''
  try {
    const created = await communityAPI.createPoll(payload)
    if (epoch !== identityEpoch) return
    replacePoll(created)
    resetCreate()
    notice.value = '议题已公开，居民现在可以投票。'
  } catch (value) {
    if (epoch !== identityEpoch) return
    error.value = toMessage(value, '议题发布失败，请重试。')
  } finally {
    if (epoch === identityEpoch) saving.value = false
  }
}

async function votePoll(poll: CommunityPoll): Promise<void> {
  const optionId = selections[poll.id] ?? 0
  if (!canParticipate.value || !optionId || mutationPending.value || poll.status === 'closed' || poll.viewer_option_id > 0) return
  const epoch = identityEpoch
  invalidateList()
  busyPollId.value = poll.id
  error.value = ''
  notice.value = ''
  try {
    const updated = await communityAPI.votePoll(poll.id, optionId)
    if (epoch !== identityEpoch) return
    replacePoll(updated)
    notice.value = '投票已记录。'
  } catch (value) {
    if (epoch !== identityEpoch) return
    error.value = toMessage(value, '投票提交失败，请重试。')
  } finally {
    if (epoch === identityEpoch) busyPollId.value = 0
  }
}

async function closePoll(poll: CommunityPoll): Promise<void> {
  if (!isAuthenticated.value || mutationPending.value || !poll.can_close || (poll.status === 'closed' && poll.proposal_kind !== 'announcement')) return
  if (poll.proposal_kind === 'announcement' && !window.confirm('撤回后将停止投票与展示，讨论记录保留。确定撤回？')) return
  const epoch = identityEpoch
  invalidateList()
  busyPollId.value = poll.id
  error.value = ''
  notice.value = ''
  try {
    const updated = await communityAPI.closePoll(poll.id)
    if (epoch !== identityEpoch) return
    replacePoll(updated)
    notice.value = poll.proposal_kind === 'announcement' ? '公告已撤回，讨论记录保留。' : '投票已结束，结果保持公开。'
  } catch (value) {
    if (epoch !== identityEpoch) return
    error.value = toMessage(value, '无法结束投票，请重试。')
  } finally {
    if (epoch === identityEpoch) busyPollId.value = 0
  }
}

function percentage(poll: CommunityPoll, votes: number): number {
  return poll.total_votes > 0 ? Math.round((votes / poll.total_votes) * 100) : 0
}

function deadline(poll: CommunityPoll): string {
  if (poll.closed_at) return `已于 ${new Date(poll.closed_at).toLocaleString()} 结束`
  if (poll.closes_at) return `截止 ${new Date(poll.closes_at).toLocaleString()}`
  return '由发起人或管理员结束'
}

function resultLabel(poll: CommunityPoll): string {
  const labels = { voting: '公告表决中', published: '已通过 · 全城展示中', rejected: '未达发布条件', expired: '展示已到期', withdrawn: '已撤回' }
  return poll.decision ? labels[poll.decision] : poll.status === 'closed' ? '结果已公布' : '开放投票'
}

watch(() => [auth.user?.id, auth.isAuthenticated] as const, () => {
  identityEpoch++
  invalidateList()
  polls.value = []
  participation.value = null
  kindFilter.value = 'all'
  stateFilter.value = 'all'
  discussionId.value = 0
  for (const id of Object.keys(selections)) delete selections[Number(id)]
  saving.value = false
  busyPollId.value = 0
  notice.value = ''
  error.value = ''
  resetCreate()
  void loadPolls()
}, { immediate: true, flush: 'sync' })
watch(() => props.focusPollId, () => { kindFilter.value = 'all'; stateFilter.value = 'all'; void loadPolls() })
onScopeDispose(() => { identityEpoch++; invalidateList() })
</script>

<template>
  <section class="city-activity-board city-poll-hall" aria-label="投票大厅">
    <header class="city-activity-header">
      <div class="city-activity-heading">
        <span class="city-activity-icon" aria-hidden="true"><Vote :size="22" /></span>
        <div><p class="city-activity-eyebrow">城市共识</p><h1>投票大厅</h1><p>一起决定产品路线、社区规则和功能优先级。</p></div>
      </div>
      <div class="city-activity-actions">
        <button class="city-activity-refresh" type="button" aria-label="刷新投票" title="刷新" :disabled="loading || mutationPending" @click="loadPolls"><RefreshCw :size="18" /></button>
        <button v-if="canParticipate" class="city-activity-primary" type="button" data-testid="poll-open-create" @click="createOpen = !createOpen"><ClipboardList :size="17" />提交议题</button>
      </div>
    </header>

    <p v-if="participation && !canParticipate" class="city-poll-access">L0 访客 · 可以阅读和参与讨论。正式提案与投票需要 L1 居民资格；可将作品或贡献发到闲聊广场，供管理员确认。</p>
    <p v-else-if="canParticipate" class="city-poll-access">{{ participation?.is_admin ? '管理员' : 'L1 居民' }} · 一人一票。公告由居民表决，规则及网站改动另行采纳。</p>

    <details v-if="policy" class="city-poll-rules">
      <summary>玩家公告如何发布</summary>
      <p>L1 居民提交后，正文和选项锁定。投票 {{ policy.voting_hours }} 小时，至少 {{ policy.minimum_votes }} 人参与且支持率达到 {{ policy.support_percent }}%，自动在全城动态展示 {{ policy.display_days }} 天。作者可撤回，管理员可下架，记录保留。</p>
      <p>玩家公告不等于官方公告。L0 可以参与讨论；充值和消费不会增加票权。</p>
    </details>

    <div v-if="polls.length" class="city-poll-filters" aria-label="筛选议题">
      <label>类型<select v-model="kindFilter" aria-label="议题类型筛选"><option value="all">全部类型</option><option v-for="(label, value) in kinds" :key="value" :value="value">{{ label }}</option></select></label>
      <label>状态<select v-model="stateFilter" aria-label="议题状态筛选"><option value="all">全部状态</option><option value="open">进行中</option><option value="closed">已结束</option></select></label>
      <span>{{ visiblePolls.length }} / {{ polls.length }} 条已加载议题</span>
    </div>

    <form v-if="createOpen" class="city-poll-create" data-testid="poll-create-form" @submit.prevent="createPoll">
      <div class="city-poll-create-head"><div><strong>新建议题</strong><p>保持问题明确，让每个选项都能独立选择。</p></div><button type="button" aria-label="关闭创建表单" @click="resetCreate"><X :size="18" /></button></div>
      <label>议题类型<select v-model="proposalKind" data-testid="poll-kind"><option v-for="(label, value) in kinds" :key="value" :value="value">{{ label }}</option></select></label>
      <p v-if="proposalKind === 'announcement' && policy" class="city-poll-policy">投票 {{ policy.voting_hours }} 小时 · 至少 {{ policy.minimum_votes }} 人 · 支持率 ≥ {{ policy.support_percent }}% · 通过后展示 {{ policy.display_days }} 天。发布后正文锁定，撤回不会视为通过。</p>
      <label>议题<input v-model="title" data-testid="poll-title" maxlength="180" /></label>
      <label>说明<textarea v-model="body" data-testid="poll-body" maxlength="4000"></textarea></label>
      <fieldset v-if="proposalKind !== 'announcement'"><legend>选项</legend><div v-for="(_, index) in optionDrafts" :key="index" class="city-poll-option-draft"><input v-model="optionDrafts[index]" data-testid="poll-option-input" :placeholder="`选项 ${index + 1}`" maxlength="180" /><button type="button" :disabled="optionDrafts.length <= 2" aria-label="删除选项" @click="removeOption(index)"><X :size="16" /></button></div></fieldset>
      <div class="city-poll-create-footer"><label v-if="proposalKind !== 'announcement'">截止时间（可选）<input v-model="closesAt" type="datetime-local" /></label><button v-if="proposalKind !== 'announcement'" class="city-activity-secondary" type="button" :disabled="optionDrafts.length >= 8" @click="addOption"><Plus :size="16" />添加选项</button><button class="city-activity-primary" type="submit" :disabled="saving || !canParticipate">{{ saving ? '发布中…' : '公开投票' }}</button></div>
    </form>

    <p v-if="notice" class="city-poll-notice" role="status">{{ notice }}</p>
    <div v-if="loading" class="city-activity-state" role="status">正在读取真实投票…</div>
    <div v-else-if="error && !polls.length" class="city-activity-state city-activity-state--error" role="alert"><h2>投票暂时没有加载成功</h2><p>{{ error }}</p><button class="city-activity-secondary" type="button" @click="loadPolls">重新读取</button></div>
    <div v-else-if="polls.length" class="city-poll-list">
      <p v-if="error" class="city-poll-error" role="alert">{{ error }}</p>
      <div v-if="!visiblePolls.length" class="city-poll-filter-empty">暂无符合条件的议题。<button type="button" @click="kindFilter = 'all'; stateFilter = 'all'">清除筛选</button></div>
      <article v-for="poll in visiblePolls" :id="`city-poll-${poll.id}`" :key="poll.id" class="city-poll-item" :class="{ 'is-closed': poll.status === 'closed' }">
        <header><div><span>{{ kinds[poll.proposal_kind || 'general'] }} · {{ resultLabel(poll) }}</span><h2>{{ poll.title }}</h2><p>{{ poll.body }}</p></div><small>{{ poll.author }} · {{ deadline(poll) }}</small></header>
        <p v-if="poll.proposal_kind === 'announcement'" class="city-poll-policy">至少 {{ poll.minimum_votes }} 人参与，支持率 ≥ {{ poll.support_percent }}%；通过后展示 {{ poll.display_days }} 天<span v-if="poll.expires_at">，至 {{ new Date(poll.expires_at).toLocaleString() }}</span>。</p>
        <form :data-testid="`poll-vote-${poll.id}`" @submit.prevent="votePoll(poll)">
          <label v-for="option in poll.options" :key="option.id" class="city-poll-option" :class="{ selected: selections[poll.id] === option.id }">
            <input v-model="selections[poll.id]" type="radio" :name="`poll-${poll.id}`" :value="option.id" :disabled="!canParticipate || poll.status === 'closed' || poll.viewer_option_id > 0" />
            <span class="city-poll-check" aria-hidden="true"><Check v-if="selections[poll.id] === option.id" :size="15" /><Circle v-else :size="15" /></span>
            <strong>{{ option.label }}</strong><span>{{ option.vote_count }} 票 · {{ percentage(poll, option.vote_count) }}%</span>
            <i :style="{ width: `${percentage(poll, option.vote_count)}%` }"></i>
          </label>
          <footer><span>共 {{ poll.total_votes }} 票</span><button v-if="poll.status === 'open' && !poll.viewer_option_id && canParticipate" class="city-activity-primary" type="submit" :disabled="!selections[poll.id] || mutationPending">{{ busyPollId === poll.id ? '提交中…' : '确认这一项' }}</button><span v-else-if="poll.viewer_option_id" class="city-poll-voted">你的选择已记录</span><span v-else-if="!canParticipate">L1 居民可投票</span><button v-if="poll.can_close && (poll.proposal_kind === 'announcement' ? ['voting', 'published'].includes(poll.decision || '') : poll.status === 'open')" class="city-activity-secondary" type="button" :data-testid="`poll-close-${poll.id}`" :disabled="mutationPending" @click="closePoll(poll)">{{ poll.proposal_kind === 'announcement' ? '撤回公告' : '结束投票' }}</button></footer>
        </form>
        <button class="city-activity-secondary" type="button" :aria-expanded="discussionId === poll.id" @click="discussionId = discussionId === poll.id ? 0 : poll.id">{{ discussionId === poll.id ? '收起讨论' : '参与讨论' }}</button>
        <PollDiscussion v-if="discussionId === poll.id" :post-id="poll.post_id" />
      </article>
    </div>
    <div v-else class="city-activity-state"><Vote :size="30" aria-hidden="true" /><h2>暂无开放议题</h2><p>这里还没有真实投票。已结束的投票也会保留公开结果。</p><button v-if="canParticipate" class="city-activity-primary" type="button" @click="createOpen = true"><ClipboardList :size="17" />提交议题</button></div>
  </section>
</template>

<style src="./zero-city-activity.css"></style>
