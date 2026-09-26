import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { extractApiErrorMessage } from '@/utils/apiError'
import {
  formatDateTime,
  formatMoney,
  runStatusLabel,
  stepIndexFromRun,
  type CreatorWorkbenchService,
  type WorkbenchCapability,
  type WorkbenchConnectionState,
  type WorkbenchRunRecord,
  type WorkbenchSavedReplay,
  type WorkbenchWorkspace,
} from './contracts'
import { parseWorkbenchEntryContext, type WorkbenchEntryContext } from './entry'
import { createStreamController } from './streamController'

export function useCreatorWorkbench(service?: CreatorWorkbenchService, entrySearch = window.location.search) {
  let workbenchService: CreatorWorkbenchService | null = service ?? null
  const entryContext: WorkbenchEntryContext = parseWorkbenchEntryContext(entrySearch)
  const workspace = ref<WorkbenchWorkspace | null>(null)
  const intent = ref('')
  const capabilityId = ref('')
  const locale = ref('zh-CN')
  const bootstrapLoading = ref(true)
  const bootstrapError = ref('')
  const actionError = ref('')
  const statusMessage = ref('')
  const entryError = ref('')
  const streamError = ref('')
  const connectionState = ref<WorkbenchConnectionState>('idle')
  const streamCursor = ref('')
  const launchPending = ref(false)
  const cancelPending = ref(false)
  const savePending = ref(false)
  const forking = ref(false)
  const replayingId = ref('')
  const currentRun = ref<WorkbenchRunRecord | null>(null)
  const hydratedFingerprint = ref('')
  const lastLaunchFingerprint = ref('')
  const lastLaunchAt = ref(0)

  const capabilities = computed(() => workspace.value?.capabilities ?? [])
  const selectedCapability = computed<WorkbenchCapability | null>(() => capabilities.value.find((item) => item.id === capabilityId.value) ?? null)
  const savedReplays = computed(() => workspace.value?.savedReplays ?? [])
  const warnings = computed(() => entryError.value ? [...(workspace.value?.warnings ?? []), entryError.value] : [...(workspace.value?.warnings ?? [])])
  const fingerprint = () => JSON.stringify({ intent: intent.value.trim(), capabilityId: capabilityId.value, locale: locale.value })
  const dirty = computed(() => hydratedFingerprint.value !== '' && hydratedFingerprint.value !== fingerprint())
  const costPreview = computed(() => {
    const capability = selectedCapability.value
    if (!capability) return '请先选择一个能力。'
    return `${formatMoney(capability.price ?? null)} · ${capability.price?.unitLabel || '每次'} · 预算 ${(capability.tokenBudget ?? 0).toLocaleString('zh-CN')} tokens`
  })
  const canLaunch = computed(() => {
    const capability = selectedCapability.value
    return !entryError.value && !launchPending.value && Boolean(intent.value.trim()) && Boolean(
      capability?.version && capability.capabilityDigest && capability.canonicalModelId && capability.canonicalModelVersion
      && capability.price?.acceptedQuoteId && capability.price?.acceptedQuoteSha,
    )
  })
  const canCancel = computed(() => Boolean(currentRun.value?.cancellable) && !cancelPending.value)
  const canRetry = computed(() => Boolean(currentRun.value?.retryable) && !launchPending.value)
  const canSave = computed(() => Boolean(currentRun.value?.saveable) && !savePending.value)
  const canFork = computed(() => Boolean(currentRun.value?.forkable) && !forking.value)
  const stepIndex = computed(() => stepIndexFromRun(currentRun.value))
  const runStatusText = computed(() => runStatusLabel(currentRun.value?.status ?? 'idle'))
  const currentRunCost = computed(() => currentRun.value ? `预估 ${formatMoney({ currency: currentRun.value.cost.currency, amount: currentRun.value.cost.estimate })} / 实际 ${formatMoney({ currency: currentRun.value.cost.currency, amount: currentRun.value.cost.actual })}` : '')
  const draftUpdatedAt = computed(() => formatDateTime(workspace.value?.draft.updatedAt ?? ''))
  const workspaceToken = computed(() => workspace.value?.workspaceToken ?? '')
  const sourceAssetLabel = computed(() => entryContext.kind === 'source-asset' && selectedCapability.value ? `Zero City 资产 #${entryContext.assetId} 已预选为「${selectedCapability.value.title}」。` : '')

  async function ensureService(): Promise<CreatorWorkbenchService> {
    if (workbenchService) return workbenchService
    const module = await import('./service')
    const resolvedService = module.createHttpCreatorWorkbenchService()
    workbenchService = resolvedService
    return resolvedService
  }

  const streamController = createStreamController({
    ensureService,
    currentRun,
    streamCursor,
    connectionState,
    streamError,
    statusMessage,
  })

  function applyEntryContext(loaded: WorkbenchWorkspace): void {
    entryError.value = ''
    if (entryContext.kind === 'none') return
    if (entryContext.kind === 'invalid') {
      entryError.value = entryContext.message
      return
    }
    if (!loaded.capabilities.some((item) => item.id === entryContext.assetId)) {
      entryError.value = `Zero City 资产 #${entryContext.assetId} 当前未映射到可执行能力；请返回资产页后重试。`
      return
    }
    capabilityId.value = entryContext.assetId
  }

  async function loadWorkspace(): Promise<void> {
    streamController.stopStream()
    bootstrapLoading.value = true
    bootstrapError.value = ''
    actionError.value = ''
    streamError.value = ''
    connectionState.value = 'connecting'
    try {
      const loaded = await (await ensureService()).loadWorkspace()
      workspace.value = loaded
      intent.value = loaded.draft.intent
      capabilityId.value = loaded.draft.capabilityId
      locale.value = loaded.draft.locale
      applyEntryContext(loaded)
      hydratedFingerprint.value = fingerprint()
      streamController.applyRun(loaded.currentRun, true)
    } catch (error) {
      bootstrapError.value = extractApiErrorMessage(error, '工作台入口暂时不可用')
      connectionState.value = 'idle'
    } finally {
      bootstrapLoading.value = false
    }
  }

  async function refreshRun(): Promise<void> {
    if (!currentRun.value) return
    try {
      streamController.applyRun(await (await ensureService()).refreshRun(currentRun.value.id), false)
    } catch (error) {
      actionError.value = extractApiErrorMessage(error, '运行状态刷新失败')
    }
  }

  async function launchRun(retryOfRunId?: string): Promise<void> {
    if (!canLaunch.value || launchPending.value) return
    const key = JSON.stringify({ fingerprint: fingerprint(), retryOfRunId: retryOfRunId ?? '' })
    const now = Date.now()
    if (key === lastLaunchFingerprint.value && now - lastLaunchAt.value < 1000) return
    lastLaunchFingerprint.value = key
    lastLaunchAt.value = now
    launchPending.value = true
    actionError.value = ''
    try {
      const capability = selectedCapability.value
      if (!capability) return
      const run = await (await ensureService()).createRun({
        workspaceToken: workspaceToken.value,
        capabilityId: capability.id,
        capabilityVersion: capability.version,
        capabilityDigest: capability.capabilityDigest,
        canonicalModelId: capability.canonicalModelId,
        canonicalModelVersion: capability.canonicalModelVersion,
        acceptedQuoteId: capability.price.acceptedQuoteId,
        acceptedQuoteSha: capability.price.acceptedQuoteSha,
        intent: intent.value.trim(),
        locale: locale.value,
        retryOfRunId,
      })
      streamController.applyRun(run, true)
      statusMessage.value = run.status === 'succeeded' ? '运行已完成，可保存或回放。' : '运行已启动，正在接收持久化事件。'
      hydratedFingerprint.value = fingerprint()
    } catch (error) {
      actionError.value = extractApiErrorMessage(error, '发起运行失败，请稍后重试')
    } finally {
      launchPending.value = false
    }
  }

  async function cancelRun(): Promise<void> {
    if (!currentRun.value || !canCancel.value) return
    cancelPending.value = true
    try {
      streamController.applyRun(await (await ensureService()).cancelRun(currentRun.value.id), false)
      statusMessage.value = '已请求取消当前运行。'
    } catch (error) {
      actionError.value = extractApiErrorMessage(error, '取消运行失败，请稍后重试')
    } finally {
      cancelPending.value = false
    }
  }

  async function forkRun(): Promise<void> {
    if (!currentRun.value || !canFork.value) return
    forking.value = true
    try {
      streamController.applyRun(await (await ensureService()).forkRun(currentRun.value.id), true)
      statusMessage.value = '已从当前结果创建新的运行分支。'
    } catch (error) {
      actionError.value = extractApiErrorMessage(error, '创建分支失败，请稍后重试')
    } finally {
      forking.value = false
    }
  }

  async function saveRun(): Promise<void> {
    if (!currentRun.value || !canSave.value) return
    savePending.value = true
    try {
      const saved = await (await ensureService()).saveRun(currentRun.value.id, { label: currentRun.value.artifacts[0]?.title ?? currentRun.value.capabilityLabel })
      if (workspace.value) workspace.value = { ...workspace.value, savedReplays: [saved, ...workspace.value.savedReplays] }
      statusMessage.value = '结果已保存到回放列表。'
    } catch (error) {
      actionError.value = extractApiErrorMessage(error, '保存结果失败，请稍后重试')
    } finally {
      savePending.value = false
    }
  }

  async function replaySaved(item: WorkbenchSavedReplay): Promise<void> {
    if (replayingId.value) return
    replayingId.value = item.id
    try {
      intent.value = item.intent
      capabilityId.value = item.capabilityId
      streamController.applyRun(await (await ensureService()).replayRun(item.id), true)
      statusMessage.value = `已按「${item.label}」重新发起运行。`
    } catch (error) {
      actionError.value = extractApiErrorMessage(error, '回放失败，请稍后重试')
    } finally {
      replayingId.value = ''
    }
  }

  const artifactTitle = computed(() => currentRun.value?.artifact?.title || '运行工件')
  const artifactPreview = computed(() => {
    const artifact = currentRun.value?.artifact
    if (!artifact) return ''
    if (artifact.kind === 'video' && !artifact.verifiedAcceptedTask) return ''
    return artifact.preview
  })

  onMounted(() => { void loadWorkspace() })
  onBeforeUnmount(() => { streamController.stopStream() })

  return {
    actionError, artifactPreview, artifactTitle, artifactKinds: ['text', 'code', 'image', 'video', 'bundle'] as const, bootstrapError, bootstrapLoading,
    canCancel, canFork, canLaunch, canRetry, canSave, cancelPending, capabilityId, capabilities, connectionState,
    costPreview, currentRun, currentRunCost, dirty, draftUpdatedAt, formatDateTime, formatMoney, forkRun,
    intent, launchPending, locale, reconnect: streamController.reconnect, replaySaved, replayingId,
    retryRun: () => launchRun(currentRun.value?.id), runStatusText, savedReplays, savePending, saveRun,
    selectedCapability, sourceAssetLabel, statusMessage, stepIndex, streamCursor, streamError, warnings, workspaceToken,
    cancelRun, launchRun, loadWorkspace, refreshRun,
  }
}
