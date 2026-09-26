<script setup lang="ts">
import { computed, onScopeDispose, reactive, ref, watch } from 'vue'
import { Check, Circle, ClipboardList, Plus, RefreshCw, Vote, X } from '@lucide/vue'
import { communityAPI, type CommunityPoll, type CreateCommunityPollPayload } from '@/features/bizdecipher/api/community'
import { useAuthStore } from '@/stores/auth'
import { extractActionableApiErrorMessage } from '@/utils/apiError'

const auth = useAuthStore()
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
    const response = await communityAPI.listPolls()
    if (requestId !== listRequestId) return
    polls.value = response.items
    syncSelections(response.items)
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
  createOpen.value = false
}

async function createPoll(): Promise<void> {
  if (!isAuthenticated.value || mutationPending.value) return
  const options = optionDrafts.value.map((option) => option.trim()).filter(Boolean)
  if (!title.value.trim() || !body.value.trim() || options.length < 2) {
    error.value = '请填写议题、说明和至少两个选项。'
    return
  }
  const payload: CreateCommunityPollPayload = { title: title.value.trim(), body: body.value.trim(), options }
  if (closesAt.value) payload.closes_at = new Date(closesAt.value).toISOString()
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
  if (!isAuthenticated.value || !optionId || mutationPending.value || poll.status === 'closed' || poll.viewer_option_id > 0) return
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
  if (!isAuthenticated.value || mutationPending.value || !poll.can_close || poll.status === 'closed') return
  const epoch = identityEpoch
  invalidateList()
  busyPollId.value = poll.id
  error.value = ''
  notice.value = ''
  try {
    const updated = await communityAPI.closePoll(poll.id)
    if (epoch !== identityEpoch) return
    replacePoll(updated)
    notice.value = '投票已结束，结果保持公开。'
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

watch(() => [auth.user?.id, auth.isAuthenticated] as const, () => {
  identityEpoch++
  invalidateList()
  polls.value = []
  for (const id of Object.keys(selections)) delete selections[Number(id)]
  saving.value = false
  busyPollId.value = 0
  notice.value = ''
  error.value = ''
  resetCreate()
  void loadPolls()
}, { immediate: true, flush: 'sync' })
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
        <button v-if="isAuthenticated" class="city-activity-primary" type="button" data-testid="poll-open-create" @click="createOpen = !createOpen"><ClipboardList :size="17" />提交议题</button>
      </div>
    </header>

    <form v-if="createOpen" class="city-poll-create" data-testid="poll-create-form" @submit.prevent="createPoll">
      <div class="city-poll-create-head"><div><strong>新建议题</strong><p>保持问题明确，让每个选项都能独立选择。</p></div><button type="button" aria-label="关闭创建表单" @click="resetCreate"><X :size="18" /></button></div>
      <label>议题<input v-model="title" data-testid="poll-title" maxlength="180" /></label>
      <label>说明<textarea v-model="body" data-testid="poll-body" maxlength="4000"></textarea></label>
      <fieldset><legend>选项</legend><div v-for="(_, index) in optionDrafts" :key="index" class="city-poll-option-draft"><input v-model="optionDrafts[index]" data-testid="poll-option-input" :placeholder="`选项 ${index + 1}`" maxlength="180" /><button type="button" :disabled="optionDrafts.length <= 2" aria-label="删除选项" @click="removeOption(index)"><X :size="16" /></button></div></fieldset>
      <div class="city-poll-create-footer"><label>截止时间（可选）<input v-model="closesAt" type="datetime-local" /></label><button class="city-activity-secondary" type="button" :disabled="optionDrafts.length >= 8" @click="addOption"><Plus :size="16" />添加选项</button><button class="city-activity-primary" type="submit" :disabled="saving">{{ saving ? '发布中…' : '公开投票' }}</button></div>
    </form>

    <p v-if="notice" class="city-poll-notice" role="status">{{ notice }}</p>
    <div v-if="loading" class="city-activity-state" role="status">正在读取真实投票…</div>
    <div v-else-if="error && !polls.length" class="city-activity-state city-activity-state--error" role="alert"><h2>投票暂时没有加载成功</h2><p>{{ error }}</p><button class="city-activity-secondary" type="button" @click="loadPolls">重新读取</button></div>
    <div v-else-if="polls.length" class="city-poll-list">
      <p v-if="error" class="city-poll-error" role="alert">{{ error }}</p>
      <article v-for="poll in polls" :key="poll.id" class="city-poll-item" :class="{ 'is-closed': poll.status === 'closed' }">
        <header><div><span>{{ poll.status === 'closed' ? '结果已公布' : '开放投票' }}</span><h2>{{ poll.title }}</h2><p>{{ poll.body }}</p></div><small>{{ poll.author }} · {{ deadline(poll) }}</small></header>
        <form :data-testid="`poll-vote-${poll.id}`" @submit.prevent="votePoll(poll)">
          <label v-for="option in poll.options" :key="option.id" class="city-poll-option" :class="{ selected: selections[poll.id] === option.id }">
            <input v-model="selections[poll.id]" type="radio" :name="`poll-${poll.id}`" :value="option.id" :disabled="poll.status === 'closed' || poll.viewer_option_id > 0" />
            <span class="city-poll-check" aria-hidden="true"><Check v-if="selections[poll.id] === option.id" :size="15" /><Circle v-else :size="15" /></span>
            <strong>{{ option.label }}</strong><span>{{ option.vote_count }} 票 · {{ percentage(poll, option.vote_count) }}%</span>
            <i :style="{ width: `${percentage(poll, option.vote_count)}%` }"></i>
          </label>
          <footer><span>共 {{ poll.total_votes }} 票</span><button v-if="poll.status === 'open' && !poll.viewer_option_id && isAuthenticated" class="city-activity-primary" type="submit" :disabled="!selections[poll.id] || busyPollId === poll.id">{{ busyPollId === poll.id ? '提交中…' : '确认这一项' }}</button><span v-else-if="poll.viewer_option_id" class="city-poll-voted">你的选择已记录</span><span v-else-if="!isAuthenticated">登录后可投票</span><button v-if="poll.status === 'open' && poll.can_close" class="city-activity-secondary" type="button" :data-testid="`poll-close-${poll.id}`" :disabled="busyPollId === poll.id" @click="closePoll(poll)">结束投票</button></footer>
        </form>
      </article>
    </div>
    <div v-else class="city-activity-state"><Vote :size="30" aria-hidden="true" /><h2>暂无开放议题</h2><p>这里还没有真实投票。已结束的投票也会保留公开结果。</p><button v-if="isAuthenticated" class="city-activity-primary" type="button" @click="createOpen = true"><ClipboardList :size="17" />提交议题</button></div>
  </section>
</template>

<style src="./zero-city-activity.css"></style>
