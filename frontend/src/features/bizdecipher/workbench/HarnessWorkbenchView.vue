<template>
  <AppLayout>
    <TokenRewardStudio v-if="typeof route.query.reward === 'string' && route.query.reward" :resource-id="route.query.reward" />
    <div v-else class="studio" :class="{ 'rail-closed': !railOpen }">
      <aside v-if="railOpen" class="studio-rail">
        <div class="rail-heading"><FolderOpen :size="17" /><strong>创作空间</strong><button class="icon-button" aria-label="收起项目" title="收起项目" @click="railOpen = false"><PanelLeftClose :size="16" /></button></div>
        <button class="new-session" :disabled="running" @click="newTask"><Plus :size="16" />新建任务</button>
        <label class="rail-search"><Search :size="14" /><span class="sr-only">搜索本地任务</span><input v-model="draftQuery" aria-label="搜索本地任务" placeholder="搜索任务" /></label>
        <div class="rail-label"><span>此浏览器的任务</span><span>{{ filteredDrafts.length }}/{{ drafts.length }}</span></div>
        <button v-for="item in filteredDrafts" :key="item.id" class="project-row" :class="{ selected: taskId === item.id }" :disabled="running" @click="openDraft(item)"><MessageSquare :size="15" /><span>{{ item.title }}</span><CircleCheck v-if="item.resultState === 'verified'" :size="13" aria-label="已有确认结果" /></button>
        <p v-if="drafts.length && !filteredDrafts.length" class="muted rail-empty">没有匹配的本地任务</p>
        <p v-else-if="!drafts.length" class="muted rail-empty">暂无保存的任务</p>
        <nav class="rail-links"><RouterLink to="/assets"><Blocks :size="16" />能力资产</RouterLink><RouterLink to="/account-square"><Network :size="16" />共享市场</RouterLink></nav>
        <div class="rail-bottom"><span class="connection-dot" :class="{ connected }" /><span>{{ connectionLabel }}</span><button v-if="catalogError" class="text-button" @click="reloadAccountData">重试</button></div>
      </aside>

      <section class="studio-session" aria-label="Harness 创作工作台">
        <header class="session-header"><button v-if="!railOpen" class="icon-button" title="展开项目" aria-label="展开项目" @click="railOpen = true"><PanelLeft :size="16" /></button><div class="session-title"><input v-model="title" aria-label="任务名称" placeholder="未命名任务" /><span>{{ resultCompleted ? '已确认结果' : '本地任务' }}</span></div><button class="icon-button" title="保存任务" aria-label="保存任务" @click="saveDraft()"><Save :size="17" /></button></header>
        <nav class="session-tabs" aria-label="工作区视图"><button :class="{ active: tab === 'conversation' }" @click="tab = 'conversation'"><MessageSquare :size="15" />对话</button><button :class="{ active: tab === 'activity' }" @click="tab = 'activity'"><Activity :size="15" />执行轨迹</button><span v-if="running" class="muted">执行中</span></nav>

        <div class="session-scroll" aria-live="polite"><div class="session-content">
          <div v-if="error" class="notice error" role="alert"><span>{{ error }}</span><button v-if="catalogError" class="text-button" @click="reloadAccountData">重新连接</button></div>
          <template v-if="tab === 'conversation'">
            <article v-if="submitted" class="message user-message"><div class="message-heading"><UserRound :size="16" /><strong>你</strong></div><p>{{ submitted }}</p></article>
            <article v-if="answer" class="message assistant-message">
              <div class="message-heading"><img :src="mascot" alt="" width="24" height="24" /><strong>工作台</strong><span v-if="resultCompleted" class="verified-result"><CircleCheck :size="13" />已确认完成</span></div>
              <HarnessMarkdownResult :content="answer" />
              <div v-if="resultCompleted" class="result-actions">
                <button class="model-button" type="button" aria-label="复制结果" @click="copyResult"><Copy :size="15" />{{ copied ? '已复制' : '复制结果' }}</button>
                <button v-if="!savedAssetId" class="model-button" type="button" aria-label="保存为资产" @click="openAssetDialog"><Blocks :size="15" />保存为资产</button>
                <RouterLink v-else class="model-button" :to="{ path: '/assets', query: { tab: 'mine', asset: String(savedAssetId) } }"><ExternalLink :size="15" />查看资产草稿</RouterLink>
              </div>
            </article>
            <figure v-for="image in images" :key="image" class="generated-image"><img :src="image" alt="生成的作品" /><figcaption><a :href="image" target="_blank" rel="noopener noreferrer"><ExternalLink :size="14" />打开原图</a><button class="text-button" type="button" @click="copyImageLink(image)"><Copy :size="14" />{{ copiedImage === image ? '链接已复制' : '复制链接' }}</button></figcaption></figure>
            <div v-if="!submitted" class="session-empty"><img :src="mascot" alt="" width="64" height="64" /><h1>今天想完成什么？</h1></div>
          </template>
          <ol v-else class="trace-list"><li v-for="(event, index) in activity" :key="index"><span class="connection-dot connected" />{{ event }}</li><li v-if="!activity.length" class="muted">暂无执行记录</li></ol>
        </div></div>

        <footer class="composer-dock">
          <div v-if="status" class="draft-status" role="status">{{ status }}</div>
          <div class="selection-summary" aria-label="当前执行配置"><span><Cpu :size="13" />规划：{{ languageSummary }}</span><span><ImageIcon :size="13" />图片：{{ imageSummary }}</span><span><Blocks :size="13" />技能：{{ skillIds.length ? `${skillIds.length} 项` : '未选择' }}</span></div>
          <div class="selected-skills"><button v-for="item in selectedSkills" :key="item.id" :disabled="running" @click="toggleSkill(item.id)"><Blocks :size="13" />{{ item.title }}<X :size="12" /></button></div>
          <div class="composer"><textarea v-model="intent" aria-label="创作目标" placeholder="描述你想完成的作品…" rows="3" :disabled="running" />
            <div class="composer-toolbar"><button class="model-button" :disabled="running" @click="resourceDialog?.showModal()"><Cpu :size="15" /><span>{{ languageModel || '模型与资源' }}</span><ChevronDown :size="13" /></button><button class="model-button" :disabled="running" @click="toolDialog?.showModal()"><Plus :size="14" />添加技能</button><button v-if="running" class="send-button stop" title="停止接收结果；不代表服务端任务或计费已取消" aria-label="停止接收结果" @click="stop"><Square :size="16" /></button><button v-else class="send-button" title="发送任务" aria-label="发送任务" :disabled="!canSend" @click="run"><ArrowUp :size="18" /></button></div>
          </div>
          <div class="composer-meta"><span class="muted">{{ sendHint }}</span></div>
        </footer>
      </section>

      <HarnessResourceDialog :key="dialogGeneration" ref="resourceDialog" @select="selectResource" />
      <HarnessAssetDraftDialog :key="dialogGeneration" ref="assetDialog" :result="assetResult" @saved="handleAssetSaved" />
      <dialog ref="toolDialog" class="harness-dialog" aria-labelledby="tool-title">
        <header><h2 id="tool-title">添加平台技能</h2><button class="icon-button" aria-label="关闭工具" @click="toolDialog?.close()"><X :size="18" /></button></header>
        <label class="tool-search"><Search :size="15" /><span class="sr-only">搜索平台技能</span><input v-model="skillQuery" aria-label="搜索平台技能" placeholder="搜索技能名称或说明" /></label>
        <p v-if="catalogLoading" class="muted">正在读取平台技能…</p>
        <div v-for="item in visibleSkills" :key="item.id" class="skill-row" :class="{ selected: skillIds.includes(item.id) }"><label><input type="checkbox" :checked="skillIds.includes(item.id)" @change="toggleSkill(item.id)" /><span><strong>{{ item.title }}</strong><small>{{ item.description }}</small></span><span class="muted">平台技能 · 免费</span></label></div>
        <p v-if="catalogError" class="notice error" role="alert"><span>{{ catalogError }}</span><button class="text-button" @click="reloadAccountData">重试</button></p>
        <p v-else-if="!catalogLoading && !visibleSkills.length" class="muted">{{ skillQuery.trim() ? '没有匹配的平台技能' : '平台技能目录暂无可用项目' }}</p>
        <details @toggle="creatorToolsOpen = ($event.target as HTMLDetailsElement).open"><summary>创作者工具与工作流</summary><CreatorToolPicker v-if="creatorToolsOpen" :key="dialogGeneration" /></details>
        <footer><button class="new-session" @click="toolDialog?.close()">完成</button></footer>
      </dialog>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { RouterLink } from 'vue-router'
import { Activity, ArrowUp, Blocks, ChevronDown, CircleCheck, Copy, Cpu, ExternalLink, FolderOpen, Image as ImageIcon, MessageSquare, Network, PanelLeft, PanelLeftClose, Plus, Save, Search, Square, UserRound, X } from '@lucide/vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import TokenRewardStudio from './TokenRewardStudio.vue'
import { apiClient, buildApiUrl } from '@/api/client'
import { list as listKeys } from '@/api/keys'
import { useClipboard } from '@/composables/useClipboard'
import { zeroCityMascots } from '@/constants/zeroCityMascots'
import { useAuthStore } from '@/stores/auth'
import { isDraftRecord } from '@/utils/localDraftStorage'
import { useRoute } from 'vue-router'
import { harnessDraftSerializer, harnessDraftStorageKey, readHarnessDrafts, type HarnessDraft as Draft } from './harnessDrafts'
import { sanitizeUrl } from '@/utils/url'
import HarnessAssetDraftDialog from './HarnessAssetDraftDialog.vue'
import HarnessMarkdownResult from './HarnessMarkdownResult.vue'
import HarnessResourceDialog from './HarnessResourceDialog.vue'
import CreatorToolPicker from './CreatorToolPicker.vue'
const creatorToolsOpen = ref(false)

interface Skill { id: string; title: string; description: string }
interface ResourceChoice { slot: 'language' | 'image'; keyId: number; model: string; customKey: string; label: string }
const auth = useAuthStore()
const route = useRoute()
const mascot = zeroCityMascots.gatewayOperator
const railOpen = ref(window.innerWidth >= 768)
const title = ref(''); const taskId = ref(''); const intent = ref(''); const submitted = ref(''); const answer = ref('')
const images = ref<string[]>([]); const error = ref(''); const status = ref(''); const tab = ref<'conversation' | 'activity'>('conversation')
const connected = ref(false); const catalogLoading = ref(false); const catalogError = ref(''); const running = ref(false)
const resultCompleted = ref(false); const completedAt = ref(''); const savedAssetId = ref<number | null>(null)
const skills = ref<Skill[]>([]); const skillIds = ref<string[]>([]); const skillQuery = ref('')
const languageKeyId = ref(0); const languageModel = ref(''); const languageResourceLabel = ref(''); const imageKeyId = ref(0); const imageModel = ref(''); const imageResourceLabel = ref('')
const languageCustomKey = ref(''); const imageCustomKey = ref(''); const activity = ref<string[]>([]); const drafts = ref<Draft[]>([]); const draftQuery = ref('')
const dialogGeneration = ref(0); const resourceDialog = ref<InstanceType<typeof HarnessResourceDialog> | null>(null); const assetDialog = ref<InstanceType<typeof HarnessAssetDraftDialog> | null>(null); const toolDialog = ref<HTMLDialogElement | null>(null)
const { copied, copyToClipboard } = useClipboard(); const copiedImage = ref('')
let accountEpoch = 0; let assetDialogEpoch = 0; let controller: AbortController | null = null

const serializer = harnessDraftSerializer
const userId = computed(() => auth.user?.id ?? null)
const storageKey = computed(() => harnessDraftStorageKey(userId.value))
const filteredDrafts = computed(() => { const query = draftQuery.value.trim().toLocaleLowerCase(); return query ? drafts.value.filter(item => [item.title, item.intent, item.answer].some(value => value.toLocaleLowerCase().includes(query))) : drafts.value })
const selectedSkills = computed(() => skills.value.filter(item => skillIds.value.includes(item.id)))
const visibleSkills = computed(() => { const query = skillQuery.value.trim().toLocaleLowerCase(); return query ? skills.value.filter(item => `${item.title} ${item.description}`.toLocaleLowerCase().includes(query)) : skills.value })
const assetResult = computed(() => ({ title: title.value || submitted.value.slice(0, 60) || 'Harness 完成结果', output: answer.value, imageUrls: images.value }))
const canSend = computed(() => connected.value && Boolean(intent.value.trim()) && Boolean((languageCustomKey.value || languageKeyId.value > 0) && languageModel.value.trim()) && (!(imageKeyId.value || imageCustomKey.value) || Boolean(imageModel.value.trim())))
const connectionLabel = computed(() => catalogLoading.value ? '正在连接工作台' : connected.value ? 'Harness 已连接' : 'Harness 未连接')
const languageSummary = computed(() => languageModel.value ? `${languageResourceLabel.value || '已选资源'} / ${languageModel.value}` : '未选择')
const imageSummary = computed(() => imageModel.value ? `${imageResourceLabel.value || '已选资源'} / ${imageModel.value}` : '未启用')
const sendHint = computed(() => catalogLoading.value ? '正在连接 Harness 目录' : catalogError.value ? '目录不可用，请重试连接' : !intent.value.trim() ? '先描述创作目标' : !languageModel.value || !(languageKeyId.value || languageCustomKey.value) ? '请选择可用的规划模型与资源' : '技能免费 · 模型用量按所选资源规则执行')

function loadDrafts() { try { drafts.value = readHarnessDrafts(userId.value) } catch { drafts.value = []; error.value = '本地任务无法读取，未载入这些记录' } }
function persistDrafts() { try { localStorage.setItem(storageKey.value, serializer.write(drafts.value)) } catch { error.value = '本地任务无法保存' } }
function resetWorkspace() {
  taskId.value = ''; title.value = ''; intent.value = ''; submitted.value = ''; answer.value = ''; images.value = []; activity.value = []; error.value = ''; status.value = ''; resultCompleted.value = false; completedAt.value = ''; savedAssetId.value = null
  skillIds.value = []; languageKeyId.value = 0; languageModel.value = ''; languageResourceLabel.value = ''; imageKeyId.value = 0; imageModel.value = ''; imageResourceLabel.value = ''; languageCustomKey.value = ''; imageCustomKey.value = ''; copiedImage.value = ''; tab.value = 'conversation'
}
function toggleSkill(id: string) { skillIds.value = skillIds.value.includes(id) ? skillIds.value.filter(value => value !== id) : [...skillIds.value, id] }
function selectResource(choice: ResourceChoice) { if (choice.slot === 'language') { languageKeyId.value = choice.keyId; languageModel.value = choice.model; languageResourceLabel.value = choice.label; languageCustomKey.value = choice.customKey } else { imageKeyId.value = choice.keyId; imageModel.value = choice.model; imageResourceLabel.value = choice.label; imageCustomKey.value = choice.customKey } }
function saveDraft() {
  const verified = resultCompleted.value && Boolean(answer.value)
  const language = languageCustomKey.value ? { keyId: 0, model: '', label: '' } : { keyId: languageKeyId.value, model: languageModel.value, label: languageResourceLabel.value }
  const image = imageCustomKey.value ? { keyId: 0, model: '', label: '' } : { keyId: imageKeyId.value, model: imageModel.value, label: imageResourceLabel.value }
  const draft: Draft = { id: taskId.value || `${Date.now()}`, title: title.value || intent.value.slice(0, 24) || '未命名任务', intent: intent.value, submitted: submitted.value, answer: answer.value, skillIds: [...skillIds.value], languageKeyId: language.keyId, languageModel: language.model, languageResourceLabel: language.label, imageKeyId: image.keyId, imageModel: image.model, imageResourceLabel: image.label, images: [...images.value], ...(verified ? { resultState: 'verified' as const, completedAt: completedAt.value || new Date().toISOString(), ...(savedAssetId.value ? { savedAssetId: savedAssetId.value } : {}) } : {}) }
  draft.updatedAt = new Date().toISOString()
  drafts.value = [draft, ...drafts.value.filter(item => item.id !== draft.id)]; taskId.value = draft.id; title.value = draft.title; persistDrafts(); status.value = '已保存在此浏览器'
}
function newTask() { resetWorkspace() }
function openDraft(item: Draft) {
  resetWorkspace(); taskId.value = item.id; title.value = item.title; intent.value = item.intent; submitted.value = item.submitted; answer.value = item.answer; skillIds.value = [...item.skillIds]
  languageKeyId.value = item.languageKeyId; languageModel.value = item.languageModel; languageResourceLabel.value = item.languageResourceLabel || ''; imageKeyId.value = item.imageKeyId; imageModel.value = item.imageModel; imageResourceLabel.value = item.imageResourceLabel || ''; images.value = [...(item.images || [])]
  resultCompleted.value = item.resultState === 'verified' && Boolean(item.answer); completedAt.value = resultCompleted.value ? item.completedAt || '' : ''; savedAssetId.value = resultCompleted.value && item.savedAssetId ? item.savedAssetId : null
  status.value = resultCompleted.value ? '已打开确认完成的本地结果' : item.answer ? '已打开旧任务；完成状态未经确认' : '已打开本地任务'
}
function stop() { status.value = '正在停止接收结果…'; controller?.abort() }
function applyFrame(line: string, epoch: number) {
  if (epoch !== accountEpoch) return false
  const value: unknown = JSON.parse(line); if (!isDraftRecord(value)) throw new Error('Invalid event')
  if (value.type === 'started') activity.value.push('任务已提交到 Harness')
  if (value.type === 'tool') activity.value.push(`${value.name === 'skill' ? '加载技能' : value.name === 'generate_image' ? '生成图片' : '工具执行'} · ${value.state === 'running' ? '开始' : '结束'}`)
  if (value.type === 'completed' && typeof value.text === 'string') { answer.value = value.text; images.value = Array.isArray(value.images) ? value.images.map(item => typeof item === 'string' ? sanitizeUrl(item) : '').filter(Boolean) : []; resultCompleted.value = true; completedAt.value = new Date().toISOString(); savedAssetId.value = null; activity.value.push('任务完成'); status.value = '执行完成'; return true }
  if (value.type === 'failed') throw new Error(typeof value.message === 'string' ? value.message : '执行失败')
  return false
}
async function run() {
  if (!canSend.value || running.value) return
  const epoch = accountEpoch; const runController = new AbortController(); controller = runController
  running.value = true; error.value = ''; answer.value = ''; images.value = []; submitted.value = intent.value; activity.value = []; status.value = '正在启动 Harness'; resultCompleted.value = false; completedAt.value = ''; savedAssetId.value = null
  try {
    const response = await fetch(buildApiUrl('/biz/harness/run'), { method: 'POST', headers: { authorization: `Bearer ${localStorage.getItem('auth_token') || ''}`, 'content-type': 'application/json' }, body: JSON.stringify({ intent: intent.value, languageKeyId: languageKeyId.value, languageModel: languageModel.value, languageCustomKey: languageCustomKey.value, imageKeyId: imageKeyId.value, imageModel: imageModel.value, imageCustomKey: imageCustomKey.value, skillIds: skillIds.value }), signal: runController.signal })
    if (epoch !== accountEpoch) return
    if (!response.ok || !response.body) throw new Error(response.status === 429 ? '任务正在执行，请稍后再试' : '工作台连接失败，请检查登录和资源配置')
    const reader = response.body.getReader(); const decoder = new TextDecoder(); let buffer = ''; let completed = false
    for (;;) { const chunk = await reader.read(); if (epoch !== accountEpoch) return; buffer += decoder.decode(chunk.value, { stream: !chunk.done }); const lines = buffer.split('\n'); buffer = lines.pop() || ''; for (const line of lines) if (line.trim()) completed = applyFrame(line, epoch) || completed; if (chunk.done) break }
    if (buffer.trim()) completed = applyFrame(buffer, epoch) || completed
    if (!completed) throw new Error('连接已结束，但没有收到完成结果')
    saveDraft()
  } catch (cause) {
    if (epoch !== accountEpoch) return
    error.value = runController.signal.aborted ? '已停止接收结果；服务端任务与计费状态未确认，请查看账单或运行记录' : cause instanceof Error ? cause.message : '执行未完成'; status.value = ''
  } finally { if (epoch === accountEpoch) running.value = false; if (controller === runController) controller = null }
}
async function reloadAccountData() {
  const epoch = accountEpoch; catalogLoading.value = true; catalogError.value = ''; connected.value = false; skills.value = []
  const [catalog, activeKeys] = await Promise.allSettled([apiClient.get<{ skills: Skill[] }>('/biz/harness/catalog'), listKeys(1, 100, { status: 'active' })])
  if (epoch !== accountEpoch) return
  catalogLoading.value = false
  if (catalog.status === 'fulfilled') { skills.value = catalog.value.data.skills; connected.value = true } else { catalogError.value = '平台技能目录连接失败，请重试'; error.value = catalogError.value }
  if (activeKeys.status === 'rejected' && !catalogError.value) { catalogError.value = '资源列表连接失败，请重试'; error.value = catalogError.value }
}
function activateAccount() { accountEpoch += 1; controller?.abort(); controller = null; running.value = false; if (typeof toolDialog.value?.close === 'function') toolDialog.value.close(); assetDialog.value?.resetForAccountSwitch(); resourceDialog.value?.resetForAccountSwitch(); dialogGeneration.value += 1; resetWorkspace(); skills.value = []; connected.value = false; catalogError.value = ''; catalogLoading.value = false; draftQuery.value = ''; skillQuery.value = ''; loadDrafts(); void reloadAccountData() }
function openAssetDialog() { assetDialogEpoch = accountEpoch; assetDialog.value?.showModal() }
function handleAssetSaved(assetId: number) { if (assetDialogEpoch !== accountEpoch || !resultCompleted.value) return; savedAssetId.value = assetId; saveDraft() }
async function copyResult() { await copyToClipboard(answer.value, '结果已复制') }
async function copyImageLink(image: string) { if (await copyToClipboard(image, '图片链接已复制')) { copiedImage.value = image; window.setTimeout(() => { if (copiedImage.value === image) copiedImage.value = '' }, 2000) } }

function openRequestedDraft() {
  if (running.value) return
  if (route.query.new === '1') { resetWorkspace(); return }
  const requested = route.query.task
  if (typeof requested !== 'string' || !requested) return
  const draft = drafts.value.find(item => item.id === requested)
  if (draft) openDraft(draft)
  else error.value = '此浏览器中找不到该账号的任务，请从左侧选择或新建任务'
}
function openRequestedResource() {
  const source = route.query.resourceSource
  const rawId = route.query.resourceId
  if ((source !== 'official' && source !== 'shared') || typeof rawId !== 'string') return
  const id = Number(rawId)
  if (!Number.isInteger(id) || id <= 0) return
  status.value = '已从共享市场带入资源；确认后才会使用'
  resourceDialog.value?.showRequested(source, id)
}
let initializedAccount = false
watch(userId, () => {
  const isInitialAccount = !initializedAccount
  initializedAccount = true
  activateAccount()
  openRequestedDraft()
  if (isInitialAccount) void nextTick(openRequestedResource)
}, { immediate: true })
watch(() => [route.query.task, route.query.new], openRequestedDraft)
watch(() => [route.query.resourceSource, route.query.resourceId], () => { void nextTick(openRequestedResource) })
onBeforeUnmount(() => { accountEpoch += 1; controller?.abort() })
</script>

<style scoped src="./studio-workbench.css"></style>
<style scoped>
.generated-image{margin:0;padding:16px 0}.generated-image img{display:block;max-width:100%;max-height:640px;object-fit:contain;border-radius:6px}.generated-image figcaption{display:flex;gap:14px;align-items:center;margin-top:10px}.generated-image a{display:inline-flex;align-items:center;gap:6px;font-size:12px;color:var(--bd-accent-teal)}.result-actions{display:flex;align-items:center;gap:10px;flex-wrap:wrap;margin:14px 0 0 28px}.result-actions .model-button{display:inline-flex;align-items:center;gap:7px;width:max-content;text-decoration:none}.verified-result{display:inline-flex;align-items:center;gap:5px;margin-left:auto;color:var(--bd-accent-teal);font-size:12px}.selection-summary{display:flex;gap:8px;flex-wrap:wrap;margin-bottom:8px}.selection-summary span{display:inline-flex;align-items:center;gap:5px;min-width:0;padding:4px 7px;border:1px solid var(--bd-ui-line);border-radius:6px;color:var(--bd-text-secondary);font-size:12px;overflow-wrap:anywhere}.selected-skills{display:flex;gap:8px;flex-wrap:wrap;margin-bottom:8px}.selected-skills button{display:flex;align-items:center;gap:6px;padding:6px 8px;background:var(--bd-canvas);border-radius:6px;font-size:12px}.trace-list{display:grid;gap:16px;font-size:14px}.trace-list li{display:flex;align-items:center;gap:12px}.harness-dialog{margin:auto;padding:24px;width:min(560px,calc(100vw - 32px));max-height:85dvh;overflow:auto;background:var(--bd-surface);color:var(--bd-text-primary);border:1px solid var(--bd-ui-line);border-radius:8px}.harness-dialog::backdrop{background:rgb(0 0 0 / .3)}.harness-dialog header{display:flex;align-items:center;justify-content:space-between;margin-bottom:16px}.harness-dialog h2{font-size:18px;font-weight:600}.harness-dialog footer{margin-top:20px;display:flex;justify-content:flex-end}.harness-dialog footer button{padding:8px 20px}.tool-search{display:flex;align-items:center;gap:8px;padding:8px 10px;border:1px solid var(--bd-ui-line);border-radius:6px}.tool-search input{flex:1;min-width:0;background:transparent;font-size:14px}.skill-row{padding:14px 8px;border-bottom:1px solid var(--bd-ui-line);border-left:2px solid transparent}.skill-row.selected{border-left-color:var(--bd-accent-teal);background:var(--bd-canvas)}.skill-row label{display:flex;gap:12px;align-items:center;font-size:14px}.skill-row label>span:first-of-type{flex:1}.skill-row small{display:block;font-size:12px;color:var(--bd-text-secondary);margin-top:6px}.skill-row input{accent-color:var(--bd-accent-teal)}
@media(max-width:767px){.result-actions{margin-left:0}.verified-result{margin-left:0}.message-heading{flex-wrap:wrap}}
</style>
