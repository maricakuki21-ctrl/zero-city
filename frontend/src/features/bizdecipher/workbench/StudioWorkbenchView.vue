<template>
  <AppLayout>
    <div class="studio" :class="{ 'rail-closed': !railOpen }">
      <aside v-if="railOpen" class="studio-rail" aria-label="工作空间">
        <div class="rail-heading"><FolderOpen :size="17" /><strong>工作空间</strong><button class="icon-button" aria-label="收起项目" title="收起项目" @click="railOpen = false"><PanelLeftClose :size="16" /></button></div>
        <button class="new-session" :disabled="bootstrapLoading" @click="newProject"><Plus :size="16" />新建项目草稿</button>
        <label class="rail-search"><Search :size="14" /><input v-model="projectQuery" aria-label="搜索项目" placeholder="搜索项目" /></label>
        <div class="rail-label">本地项目 <span>{{ projects.length }}</span></div>
        <button v-for="project in visibleProjects" :key="project.id" class="project-row" :class="{ selected: project.id === projectId }" :disabled="bootstrapLoading" @click="openProject(project)"><MessageSquare :size="15" /><span>{{ project.name }}</span></button>
        <p v-if="!visibleProjects.length" class="muted rail-empty">{{ projectQuery ? '没有匹配项目' : '暂无本地项目' }}</p>
        <div class="rail-label">已保存回放 <span>{{ savedReplays.length }}</span></div>
        <button v-for="item in savedReplays" :key="item.id" class="project-row" :data-testid="`workbench-replay-${item.id}`" :disabled="replayingId === item.id" @click="showRun = true; replaySaved(item)"><History :size="15" /><span>{{ item.label }}</span></button>
        <p v-if="!savedReplays.length" class="muted rail-empty">暂无回放</p>
        <nav class="rail-links"><RouterLink to="/assets"><Blocks :size="16" />工具与工作流</RouterLink><RouterLink to="/marketplace"><Users :size="16" />找人协作</RouterLink></nav>
        <div class="rail-bottom"><span class="connection-dot" :class="{ connected: !bootstrapError && !bootstrapLoading }" /><span>{{ bootstrapLoading ? '正在连接工作区' : bootstrapError ? '服务未连接' : '工作区已连接' }}</span><button class="icon-button" aria-label="刷新工作区" title="刷新工作区" @click="refreshWorkspace"><RefreshCw :size="14" /></button></div>
      </aside>
      <section class="studio-session" aria-label="创作会话">
        <header class="session-header">
          <button v-if="!railOpen" class="icon-button" aria-label="展开项目" title="展开项目" @click="railOpen = true"><PanelLeft :size="17" /></button>
          <div class="session-title"><input v-model="projectName" aria-label="项目名称" placeholder="未命名项目" maxlength="100" /><span>本地草稿</span></div>
          <button class="icon-button" title="保存本地项目" aria-label="保存本地项目" @click="saveProject"><Save :size="17" /></button>
        </header>
        <nav class="session-tabs" aria-label="工作区视图">
          <button v-for="tab in outputTabs" :key="tab.key" :aria-pressed="outputTab === tab.key" :class="{ active: outputTab === tab.key }" @click="outputTab = tab.key"><component :is="tab.icon" :size="15" />{{ tab.label }}</button>
          <WorkbenchStatusPill v-if="visibleRun" :status="visibleRun.status" :label="runStatusText" />
        </nav>
        <div class="session-scroll" aria-live="polite">
          <div class="session-content">
            <div v-if="bootstrapLoading" class="notice" role="status">正在载入工作区…</div>
            <div v-if="bootstrapError" class="notice error" role="alert" data-testid="workbench-alert"><span>{{ bootstrapError === 'internal error' ? '工作区服务暂不可用，当前可编辑本地草稿。' : bootstrapError }}</span><button class="text-button" data-testid="workbench-reload" @click="refreshWorkspace">重试连接</button></div>
            <div v-for="warning in warnings" :key="warning" class="notice" data-testid="workbench-alert">{{ warning }}</div>
            <div v-if="actionError || streamError" class="notice error" role="alert" data-testid="workbench-alert">{{ actionError || streamError }}</div>
            <template v-if="visibleRun">
              <template v-if="outputTab === 'conversation'">
                <article class="message user-message"><div class="message-heading"><UserRound :size="16" /><strong>你</strong></div><p>{{ visibleRun.intent }}</p></article>
                <article class="message assistant-message"><div class="message-heading"><img :src="emptyArt" alt="" width="24" height="24" /><strong>工作台</strong><span class="muted">{{ runStatusText }}</span></div>
                  <WorkbenchArtifactCard v-if="visibleRun.artifact" :title="artifactTitle" :artifact-preview="artifactPreview" />
                  <p v-else>{{ visibleRun.failureMessage || (visibleRun.status === 'cancelled' ? '运行已取消。' : visibleRun.status === 'failed' ? '运行未完成。' : '暂未收到交付内容。') }}</p>
                </article>
              </template>
              <article v-else-if="outputTab === 'activity'" class="trajectory">
                <div class="message-heading"><Activity :size="16" /><strong data-testid="workbench-run-status">{{ runStatusText }}</strong></div>
                <WorkbenchStepRail :active-index="stepIndex" />
                <dl><dt>运行 ID</dt><dd>{{ visibleRun.id }}</dd><dt>连接状态</dt><dd data-testid="workbench-connection">{{ connectionState }}</dd><dt>费用</dt><dd>{{ currentRunCost }}</dd></dl>
                <p v-if="visibleRun.failureMessage" class="error">{{ visibleRun.failureMessage }}</p>
                <WorkbenchLineage :lineage="visibleRun.lineage" />
              </article>
              <article v-else class="deliverables"><WorkbenchArtifactCard v-if="visibleRun.artifact" :title="artifactTitle" :artifact-preview="artifactPreview" /><p v-else class="muted">本次运行暂无交付物。</p></article>
              <div class="run-actions">
                <button v-if="canRetry" class="text-button" data-testid="workbench-retry" :disabled="launchPending" @click="retryRun"><RotateCcw :size="14" />重试</button>
                <button v-if="canFork" class="text-button" data-testid="workbench-fork" @click="forkRun"><GitBranch :size="14" />派生</button>
                <button v-if="canSave" class="text-button" data-testid="workbench-save" :disabled="savePending" @click="saveRun"><Save :size="14" />保存结果</button>
                <button v-if="connectionState === 'offline' || connectionState === 'exhausted'" class="text-button" data-testid="workbench-reconnect" @click="reconnect">重新连接</button>
              </div>
            </template>
            <div v-else class="session-empty"><img :src="emptyArt" alt="" width="64" height="64" /><h1>{{ outputTab === 'conversation' ? currentMode.empty : outputTab === 'activity' ? '暂无执行轨迹' : '暂无交付物' }}</h1><span class="muted">{{ projectName || '未命名项目' }}</span></div>
          </div>
        </div>
        <footer class="composer-dock">
          <div v-if="localMessage || statusMessage" class="draft-status" role="status">{{ localMessage || statusMessage }}</div>
          <div v-if="sourceAssetLabel" class="draft-status" data-testid="workbench-entry-source">{{ sourceAssetLabel }}</div>
          <div class="composer">
            <textarea id="wb-intent" v-model="intent" aria-label="创作目标" data-testid="workbench-intent" :placeholder="currentMode.brief" rows="3" :disabled="bootstrapLoading" />
            <div class="composer-toolbar">
              <button class="model-button" data-testid="workbench-model-picker" aria-haspopup="dialog" @click="pickerOpen = true"><Cpu :size="16" /><span>{{ selectedCapability?.title || '选择模型与资源' }}</span><ChevronDown :size="14" /></button>
              <button class="icon-button" title="工具与模板" aria-label="工具与模板" @click="pickerTab = 'templates'; pickerOpen = true"><Blocks :size="17" /></button>
              <label class="locale-control"><span class="sr-only">输出语言</span><select v-model="locale"><option value="zh-CN">中文</option><option value="en-US">English</option><option value="ja-JP">日本語</option></select></label>
              <button v-if="canCancel" class="send-button stop" data-testid="workbench-cancel" title="停止运行" aria-label="停止运行" :disabled="cancelPending" @click="cancelRun"><Square :size="17" /></button>
              <button v-else class="send-button" data-testid="workbench-launch" title="发送任务" aria-label="发送任务" :disabled="!canLaunch || bootstrapLoading || runBusy" @click="submit"><ArrowUp :size="19" /></button>
            </div>
          </div>
          <div class="composer-meta"><div class="creation-modes" aria-label="创作类型"><button v-for="item in modes" :key="item.key" :title="item.label" :aria-label="item.label" :aria-pressed="mode === item.key" :class="{ active: mode === item.key }" @click="mode = item.key"><component :is="item.icon" :size="14" /><span>{{ item.label }}</span></button></div><span class="cost-label">{{ selectedCapability ? costPreview : '尚未选择执行能力' }}</span></div>
        </footer>
      </section>
      <StudioResourcePicker v-model:open="pickerOpen" v-model:tab="pickerTab" :capabilities="capabilities" :selected-id="capabilityId" @select="capabilityId = $event" @template="applyTemplate" />
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import { useStorage } from '@vueuse/core'
import { Activity, ArrowUp, Blocks, ChevronDown, Code2, Cpu, Film, FolderOpen, Gamepad2, GitBranch, History, Image, MessageSquare, Package, PanelLeft, PanelLeftClose, Plus, RefreshCw, RotateCcw, Save, Search, ShoppingBag, Square, UserRound, Users, Workflow } from '@lucide/vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import { useAuthStore } from '@/stores/auth'
import { zeroCityMascots } from '@/constants/zeroCityMascots'
import WorkbenchArtifactCard from '@/features/bizdecipher/components/workbench/WorkbenchArtifactCard.vue'
import WorkbenchStatusPill from '@/features/bizdecipher/components/workbench/WorkbenchStatusPill.vue'
import WorkbenchStepRail from '@/features/bizdecipher/components/workbench/WorkbenchStepRail.vue'
import WorkbenchLineage from '@/features/bizdecipher/components/workbench/WorkbenchLineage.vue'
import StudioResourcePicker from './StudioResourcePicker.vue'
import type { CreatorWorkbenchService } from './contracts'
import { useCreatorWorkbench } from './useCreatorWorkbench'
import { starterTemplates } from '@/features/bizdecipher/data/starterTemplates'
import { isDraftRecord, localDraftSerializer } from '@/utils/localDraftStorage'

const props = defineProps<{ service?: CreatorWorkbenchService }>()
const { artifactPreview, artifactTitle, bootstrapError, bootstrapLoading, capabilities, capabilityId, canCancel, canFork, canLaunch, canRetry, canSave, cancelPending, connectionState, costPreview, currentRun, currentRunCost, forkRun, intent, launchPending, locale, reconnect, replaySaved, replayingId, retryRun, runStatusText, savedReplays, savePending, saveRun, selectedCapability, sourceAssetLabel, statusMessage, stepIndex, streamError, warnings, actionError, cancelRun, launchRun, loadWorkspace } = useCreatorWorkbench(props.service)
const modes = [
  { key: 'general', label: '综合', icon: Workflow, brief: '你想完成什么？', empty: '开始创作' },
  { key: 'image', label: '图片', icon: Image, brief: '描述图片主题、风格与尺寸…', empty: '从第一张作品开始' },
  { key: 'video', label: '视频', icon: Film, brief: '描述视频主题、时长与镜头…', empty: '从第一个视频开始' },
  { key: 'game', label: '游戏', icon: Gamepad2, brief: '描述游戏玩法、角色与规则…', empty: '构思一个新世界' },
  { key: 'code', label: '代码', icon: Code2, brief: '描述功能、技术栈与验收要求…', empty: '开始构建' },
  { key: 'commerce', label: '电商', icon: ShoppingBag, brief: '描述商品、卖点与交付要求…', empty: '创建商品内容' },
] as const
type Mode = typeof modes[number]['key']
interface LocalProject { id: string; name: string; mode: Mode; intent: string; locale: string }
const mode = ref<Mode>('general')
const currentMode = computed(() => modes.find(item => item.key === mode.value) ?? modes[0])
const outputTabs = [{ key: 'conversation', label: '对话', icon: MessageSquare }, { key: 'activity', label: '执行轨迹', icon: Activity }, { key: 'deliverables', label: '交付物', icon: Package }] as const
const outputTab = ref<typeof outputTabs[number]['key']>('conversation')
const railOpen = ref(window.innerWidth >= 768)
const pickerOpen = ref(false)
const pickerTab = ref<'available' | 'shared' | 'creator' | 'templates'>('available')
const showRun = ref(true)
const visibleRun = computed(() => showRun.value ? currentRun.value : null)
const runBusy = computed(() => Boolean(currentRun.value && ['queued', 'running', 'cancel_requested'].includes(currentRun.value.status)))
const auth = useAuthStore()
const localMessage = ref('')
function isProjects(value: unknown): value is LocalProject[] {
  return Array.isArray(value) && value.every(item => isDraftRecord(item)
    && ['id', 'name', 'intent', 'locale'].every(key => typeof item[key] === 'string')
    && modes.some(mode => mode.key === item.mode))
}
const projects = useStorage<LocalProject[]>(`studio-projects-v1-${auth.user?.id ?? 'local'}`, [], undefined, {
  serializer: localDraftSerializer(isProjects),
  onError: () => { localMessage.value = '本地项目无法读取或保存，原记录未被自动删除。' },
})
const projectId = ref('')
const projectName = ref('')
const projectQuery = ref('')
const visibleProjects = computed(() => projects.value.filter(item => item.name.toLocaleLowerCase().includes(projectQuery.value.trim().toLocaleLowerCase())))
const route = useRoute()
let appliedTemplate = ''
watch([bootstrapLoading, () => route.query.template], ([loading, templateId]) => {
  if (loading || typeof templateId !== 'string' || templateId === appliedTemplate) return
  const template = starterTemplates.find(item => item.id === templateId)
  if (!template) return
  intent.value = template.brief
  mode.value = template.mode
  projectName.value = template.title
  appliedTemplate = templateId
  showRun.value = false
  localMessage.value = '已载入模板，尚未发起运行。'
}, { immediate: true })
const emptyArt = zeroCityMascots.gatewayOperator
function applyTemplate(id: string): void {
  const template = starterTemplates.find(item => item.id === id)
  if (!template) return
  intent.value = intent.value.trim() ? `${intent.value}\n\n${template.brief}` : template.brief
  mode.value = template.mode
  localMessage.value = `已加入「${template.title}」模板，尚未运行`
}
function saveProject(): void {
  const id = projectId.value || `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`
  const project: LocalProject = { id, name: projectName.value.trim() || intent.value.trim().slice(0, 24) || '未命名项目', mode: mode.value, intent: intent.value, locale: locale.value }
  projects.value = [project, ...projects.value.filter(item => item.id !== id)]
  projectId.value = id
  projectName.value = project.name
  localMessage.value = '已保存至此浏览器'
}
function openProject(project: LocalProject): void {
  projectId.value = project.id
  projectName.value = project.name
  mode.value = project.mode
  intent.value = project.intent
  locale.value = project.locale
  showRun.value = false
  localMessage.value = '已打开本地草稿'
}
function newProject(): void {
  projectId.value = ''
  projectName.value = ''
  intent.value = ''
  showRun.value = false
  outputTab.value = 'conversation'
  localMessage.value = ''
}
async function submit(): Promise<void> {
  if (!canLaunch.value || runBusy.value) return
  showRun.value = true
  outputTab.value = 'conversation'
  await launchRun()
}
async function refreshWorkspace(): Promise<void> {
  const draft = { intent: intent.value, locale: locale.value, capabilityId: capabilityId.value }
  await loadWorkspace()
  intent.value = draft.intent
  locale.value = draft.locale
  if (capabilities.value.some(item => item.id === draft.capabilityId)) capabilityId.value = draft.capabilityId
}
</script>

<style scoped src="./studio-workbench.css"></style>
