<script setup lang="ts">
import { computed, onScopeDispose, ref, watch } from 'vue'
import { Award, RefreshCw, ScrollText } from '@lucide/vue'
import { apiClient } from '@/api/client'
import { communityAPI, type CommunityPoll } from '@/features/bizdecipher/api/community'
import { useAuthStore } from '@/stores/auth'
import { extractActionableApiErrorMessage } from '@/utils/apiError'

interface Rule { id: number; title: string; body: string; adopted_by: string; adopted_at: string }
interface Badge { key: string; name: string; description: string }
interface Grant { grant_id: number; user_id: number; badge_key: string; badge_name: string; reason: string; granted_at: string }
const props = defineProps<{ kind: 'badges' | 'rules' }>()
const auth = useAuthStore()
const admin = computed(() => auth.user?.role === 'admin')
const rules = ref<Rule[]>([])
const badges = ref<Badge[]>([])
const grants = ref<Grant[]>([])
const polls = ref<CommunityPoll[]>([])
const target = ref('')
const reason = ref('')
const selectedBadge = ref('')
const participationLevel = ref(1)
const loading = ref(false)
const busy = ref(false)
const error = ref('')
const notice = ref('')
let epoch = 0
let request = 0
const date = (value: string) => Number.isNaN(Date.parse(value)) ? '时间未记录' : new Date(value).toLocaleString('zh-CN')

async function load() {
  const current = ++request
  loading.value = true
  error.value = ''
  try {
    if (props.kind === 'rules') {
      const [result, available] = await Promise.all([
        apiClient.get<{ items: Rule[] }>('/biz/community/governance/rules'),
        admin.value ? communityAPI.listPolls() : Promise.resolve({ items: [] as CommunityPoll[] }),
      ])
      if (current !== request) return
      rules.value = result.data.items
      polls.value = available.items.filter(p => p.status === 'closed')
    } else {
      const userID = admin.value && target.value ? Number(target.value) : auth.user?.id
      if (userID !== undefined && (!Number.isSafeInteger(userID) || userID <= 0)) throw new Error('请输入有效用户编号')
      const [definitions, owned] = await Promise.all([
        apiClient.get<{ items: Badge[] }>('/biz/community/badges'),
        userID ? apiClient.get<{ items: Grant[] }>(`/biz/community/users/${userID}/badges`) : Promise.resolve({ data: { items: [] as Grant[] } }),
      ])
      if (current !== request) return
      badges.value = definitions.data.items
      grants.value = owned.data.items
    }
  } catch (value) {
    if (current === request) error.value = extractActionableApiErrorMessage(value, '治理数据加载失败，请重试。')
  } finally { if (current === request) loading.value = false }
}

async function mutate(path: string, body: object) {
  if (!admin.value || busy.value || loading.value) return
  const current = epoch
  busy.value = true
  notice.value = ''
  error.value = ''
  try {
    await apiClient.post(path, body)
    if (current !== epoch) return
    notice.value = '操作已保存。'
    await load()
  } catch (value) {
    if (current === epoch) error.value = extractActionableApiErrorMessage(value, '未收到成功确认，请刷新核对后再操作。')
  } finally { if (current === epoch) busy.value = false }
}

function grant() {
  const userID = Number(target.value)
  if (!Number.isSafeInteger(userID) || userID <= 0 || !selectedBadge.value) { error.value = '请选择徽章并填写有效用户编号'; return }
  void mutate(`/admin/biz/community/badges/${encodeURIComponent(selectedBadge.value)}/grant`, { user_id: userID, reason: reason.value })
}

async function setParticipation() {
  const userID = Number(target.value)
  if (!admin.value || busy.value || loading.value) return
  if (!Number.isSafeInteger(userID) || userID <= 0 || !reason.value.trim()) {
    error.value = '请填写用户编号及贡献或外部作品依据'
    return
  }
  const current = epoch
  busy.value = true
  error.value = ''
  notice.value = ''
  try {
    await apiClient.put(`/admin/biz/community/users/${userID}/participation`, { level: participationLevel.value, reason: reason.value })
    if (current === epoch) notice.value = `用户 #${userID} 的参与资格已更新为 L${participationLevel.value}。`
  } catch (cause) {
    if (current === epoch) error.value = extractActionableApiErrorMessage(cause, '资格更新未确认，请重试。')
  } finally { if (current === epoch) busy.value = false }
}

watch(() => [props.kind, auth.user?.id, auth.user?.role], () => {
  epoch++; request++
  rules.value = []; badges.value = []; grants.value = []; polls.value = []
  target.value = ''; selectedBadge.value = ''; reason.value = ''; notice.value = ''; busy.value = false
  void load()
}, { immediate: true })
onScopeDispose(() => { epoch++; request++ })
</script>

<template>
  <section class="governance-panel" :aria-label="kind === 'rules' ? '规则公示' : '勋章墙'">
    <header><h1><ScrollText v-if="kind === 'rules'" :size="22" /><Award v-else :size="22" />{{ kind === 'rules' ? '规则公示' : '勋章墙' }}</h1><button type="button" title="刷新" aria-label="刷新治理数据" :disabled="busy || loading" @click="load"><RefreshCw :size="18" /></button></header>
    <p v-if="error" role="alert">{{ error }}</p><p v-if="notice" role="status">{{ notice }}</p>
    <p v-if="loading" role="status">正在读取…</p>
    <template v-if="kind === 'rules'">
      <article v-for="rule in rules" :key="rule.id"><h2>{{ rule.title }}</h2><p class="body">{{ rule.body }}</p><small>{{ rule.adopted_by || '管理员' }} · {{ date(rule.adopted_at) }}</small><button v-if="admin" type="button" :disabled="busy || loading || !reason.trim()" @click="mutate(`/admin/biz/community/governance/rules/${rule.id}/revoke`, { reason })">撤销规则</button></article>
      <p v-if="!loading && !error && !rules.length">暂无已采纳规则</p>
      <form v-if="admin" @submit.prevent><label>操作原因<input v-model="reason" maxlength="300" :disabled="busy" /></label><h2>已结束的投票</h2><div v-for="poll in polls" :key="poll.id" class="poll-row"><span>{{ poll.title }} · {{ poll.total_votes }} 票</span><button type="button" :disabled="busy || loading" @click="mutate(`/biz/community/polls/${poll.id}/adopt`, {})">采纳并公示</button></div></form>
    </template>
    <template v-else>
      <form v-if="admin" @submit.prevent="grant"><label>用户编号<input v-model="target" type="number" min="1" step="1" required :disabled="busy" /></label><label>徽章<select v-model="selectedBadge" required :disabled="busy"><option value="">选择徽章</option><option v-for="badge in badges" :key="badge.key" :value="badge.key">{{ badge.name }}</option></select></label><label>授予或撤销原因<input v-model="reason" maxlength="300" :disabled="busy" /></label><button type="button" :disabled="busy || loading" @click="load">查看持有记录</button><button type="submit" :disabled="busy || loading">授予徽章</button></form>
      <form v-if="admin" class="participation-controls" @submit.prevent="setParticipation">
        <h2>社区参与资格</h2>
        <label>参与等级<select v-model.number="participationLevel" :disabled="busy"><option :value="0">L0 新来者 · 闲聊广场</option><option :value="1">L1 居民 · 技术与协作</option></select></label>
        <p>使用上方用户编号和依据，独立于徽章授予；不附带管理权限。</p>
        <button type="submit" :disabled="busy || loading || !target || !reason.trim()">保存参与资格</button>
      </form>
      <article v-for="item in grants" :key="item.grant_id"><h2><Award :size="18" />{{ item.badge_name }}</h2><p>{{ item.reason || '未填写授予原因' }}</p><small>用户 #{{ item.user_id }} · {{ date(item.granted_at) }}</small><button v-if="admin" type="button" :disabled="busy || loading || !reason.trim()" @click="mutate(`/admin/biz/community/badges/${encodeURIComponent(item.badge_key)}/revoke`, { user_id: item.user_id, reason })">撤销徽章</button></article>
      <p v-if="!loading && !error && !grants.length">暂无持有的徽章</p>
      <h2>城市徽章</h2><div class="definitions"><article v-for="badge in badges" :key="badge.key"><h3>{{ badge.name }}</h3><p>{{ badge.description }}</p></article></div>
    </template>
  </section>
</template>

<style scoped>
.participation-controls{border-block:1px solid var(--bd-ui-line);padding-block:16px}.participation-controls p{flex:1 1 220px;font-size:13px;line-height:1.6}.participation-controls h2{margin:0}
.governance-panel{color:var(--bd-text-primary);display:grid;gap:16px;min-width:0}header,h1,h2,.poll-row{display:flex;align-items:center;gap:10px}header,.poll-row{justify-content:space-between}h1{font-size:22px}h2{font-size:16px}h3{font-size:14px}article{padding:16px 0;border-bottom:1px solid var(--bd-ui-line);overflow-wrap:anywhere}p,small{color:var(--bd-text-secondary)}.body{white-space:pre-wrap}form{display:flex;flex-wrap:wrap;gap:12px;align-items:end}form h2,.poll-row{width:100%}label{display:grid;gap:6px;min-width:0}input,select,button{background:var(--bd-surface);color:var(--bd-text-primary);border:1px solid var(--bd-ui-line);border-radius:6px;min-height:36px;padding:8px;max-width:100%}button{cursor:pointer}button:disabled{opacity:.5;cursor:not-allowed}button:focus-visible,input:focus-visible,select:focus-visible{outline:2px solid var(--bd-accent-teal);outline-offset:2px}.definitions{display:grid;grid-template-columns:repeat(auto-fit,minmax(180px,1fr));gap:16px}@media(max-width:600px){label{width:100%}.poll-row{flex-wrap:wrap}}
</style>
