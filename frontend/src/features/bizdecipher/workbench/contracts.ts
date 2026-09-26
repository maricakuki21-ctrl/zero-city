export const WORKBENCH_RUN_STATUSES = [
  'idle',
  'queued',
  'running',
  'cancel_requested',
  'succeeded',
  'failed',
  'cancelled',
] as const

export type WorkbenchRunStatus = (typeof WORKBENCH_RUN_STATUSES)[number]

export const WORKBENCH_ARTIFACT_KINDS = ['text', 'code', 'image', 'video', 'bundle'] as const

export type WorkbenchArtifactKind = (typeof WORKBENCH_ARTIFACT_KINDS)[number]
export type WorkbenchConnectionState =
  | 'idle'
  | 'connecting'
  | 'connected'
  | 'reconnecting'
  | 'offline'
  | 'exhausted'
  | 'closed'

export type WorkbenchMoney = {
  readonly currency: string
  readonly amount: string | null
}

export type WorkbenchPrice = WorkbenchMoney & {
  readonly unitLabel: string
  readonly acceptedQuoteId: string
  readonly acceptedQuoteSha: string
}

export type WorkbenchCapability = {
  readonly id: string
  readonly version: string
  readonly title: string
  readonly summary: string
  readonly tokenBudget: number
  readonly averageSeconds: number
  readonly canonicalModelId: string
  readonly canonicalModelVersion: string
  readonly capabilityDigest: string
  readonly price: WorkbenchPrice
}

export type WorkbenchCost = {
  readonly currency: string
  readonly estimate: string | null
  readonly actual: string | null
  readonly explanation: string
}

export type WorkbenchLineage = {
  readonly canonicalRequestId: string
  readonly canonicalUsageEventId: string
  readonly acceptedQuoteId: string
  readonly acceptedQuoteSha: string
  readonly journalId: string
  readonly mediaTaskId: string
  readonly canonicalMediaBusinessEventId: string
  readonly runnerJobId: string
  readonly adapterDigest: string
  readonly capabilityDigest: string
  readonly artifactId: string
  readonly artifactDigest: string
  readonly upstreamTaskId: string
}

export type WorkbenchArtifact = {
  readonly id: string
  readonly artifactId: string
  readonly runId: string
  readonly kind: WorkbenchArtifactKind
  readonly title: string
  readonly preview: string
  readonly downloadLabel: string
  readonly contentType: string
  readonly href: string
  readonly storageUri: string
  readonly byteSize: string
  readonly verifiedAcceptedTask: boolean
  readonly createdAt: string
  readonly lineage: WorkbenchLineage
}

export type WorkbenchRunRecord = {
  readonly id: string
  readonly status: WorkbenchRunStatus
  readonly capabilityId: string
  readonly capabilityLabel: string
  readonly intent: string
  readonly requestedAt: string
  readonly updatedAt: string
  readonly terminalAt: string
  readonly workspaceVersion: number
  readonly cursor: string
  readonly cost: WorkbenchCost
  readonly artifact: WorkbenchArtifact | null
  readonly artifacts: readonly WorkbenchArtifact[]
  readonly cancellable: boolean
  readonly retryable: boolean
  readonly saveable: boolean
  readonly replayable: boolean
  readonly forkable: boolean
  readonly staleCapability: boolean
  readonly replayOfRunId: string
  readonly forkedFromRunId: string
  readonly failureCode: string
  readonly failureMessage: string
  readonly lineage: WorkbenchLineage
}

export type WorkbenchSavedReplay = {
  readonly id: string
  readonly label: string
  readonly capabilityId: string
  readonly capabilityLabel: string
  readonly intent: string
  readonly savedAt: string
  readonly staleCapability: boolean
  readonly sourceRunId: string
}

export type WorkbenchDraft = {
  readonly intent: string
  readonly capabilityId: string
  readonly locale: string
  readonly updatedAt: string
}

export type WorkbenchWorkspace = {
  readonly workspaceToken: string
  readonly workspaceVersion: number
  readonly lastHydratedAt: string
  readonly cursor: string
  readonly draft: WorkbenchDraft
  readonly capabilities: readonly WorkbenchCapability[]
  readonly currentRun: WorkbenchRunRecord | null
  readonly savedReplays: readonly WorkbenchSavedReplay[]
  readonly warnings: readonly string[]
}

export type WorkbenchLaunchInput = {
  readonly workspaceToken: string
  readonly capabilityId: string
  readonly capabilityVersion: string
  readonly capabilityDigest: string
  readonly canonicalModelId: string
  readonly canonicalModelVersion: string
  readonly acceptedQuoteId: string
  readonly acceptedQuoteSha: string
  readonly intent: string
  readonly locale: string
  readonly retryOfRunId?: string
}

export type WorkbenchSaveInput = { readonly label: string }

export type WorkbenchStreamMessage =
  | { readonly kind: 'event'; readonly cursor: string; readonly eventKind: string; readonly run: WorkbenchRunRecord | null }
  | { readonly kind: 'reset'; readonly cursor: string; readonly reason: string; readonly run: WorkbenchRunRecord }

export type WorkbenchStreamObserver = {
  readonly onMessage: (message: WorkbenchStreamMessage) => void
  readonly onConnection: (state: WorkbenchConnectionState) => void
  readonly onMalformed: (message: string) => void
}

export type WorkbenchStreamInput = {
  readonly runId: string
  readonly cursor: string
  readonly signal: AbortSignal
  readonly observer: WorkbenchStreamObserver
}

export type WorkbenchStreamOutcome = {
  readonly kind: 'terminal' | 'exhausted' | 'aborted'
  readonly cursor: string
}

export interface CreatorWorkbenchService {
  loadWorkspace(): Promise<WorkbenchWorkspace>
  createRun(input: WorkbenchLaunchInput): Promise<WorkbenchRunRecord>
  refreshRun(runId: string): Promise<WorkbenchRunRecord>
  cancelRun(runId: string): Promise<WorkbenchRunRecord>
  saveRun(runId: string, payload: WorkbenchSaveInput): Promise<WorkbenchSavedReplay>
  replayRun(savedReplayId: string): Promise<WorkbenchRunRecord>
  forkRun(runId: string): Promise<WorkbenchRunRecord>
  streamRun(input: WorkbenchStreamInput): Promise<WorkbenchStreamOutcome>
}

export {
  formatDateTime,
  formatMoney,
  normalizeStreamMessage,
  normalizeStreamReset,
  normalizeWorkbenchRun,
  normalizeWorkbenchSavedReplay,
  normalizeWorkbenchWorkspace,
  runStatusLabel,
  stepIndexFromRun,
} from './normalizers'
