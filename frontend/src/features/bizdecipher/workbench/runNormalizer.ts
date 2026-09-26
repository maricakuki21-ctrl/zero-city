import {
  WORKBENCH_ARTIFACT_KINDS,
  WORKBENCH_RUN_STATUSES,
  type WorkbenchArtifact,
  type WorkbenchArtifactKind,
  type WorkbenchCapability,
  type WorkbenchLineage,
  type WorkbenchRunRecord,
  type WorkbenchRunStatus,
} from './contracts'
import {
  isRecord,
  readArray,
  readBoolean,
  readCursor,
  readExactDecimal,
  readField,
  readIntegerText,
  readNestedRecord,
  readString,
  readTimestamp,
  readUnsignedInteger,
  unwrapApiData,
  type UnknownRecord,
} from './contractValues'

function readRunStatus(value: unknown): WorkbenchRunStatus {
  const status = readString(value, 'idle')
  return WORKBENCH_RUN_STATUSES.find((candidate) => candidate === status) ?? 'idle'
}

function readArtifactKind(value: unknown): WorkbenchArtifactKind {
  const kind = readString(value, 'text')
  return WORKBENCH_ARTIFACT_KINDS.find((candidate) => candidate === kind) ?? 'text'
}

function normalizeMoney(value: unknown, fallbackCurrency = ''): { readonly currency: string; readonly amount: string } | null {
  if (typeof value === 'string') {
    const amount = readExactDecimal(value)
    return amount === null ? null : { currency: fallbackCurrency, amount }
  }
  if (!isRecord(value)) return null
  const amount = readExactDecimal(readField(value, 'amount'))
  if (amount === null) return null
  return { currency: readString(readField(value, 'currency'), fallbackCurrency), amount }
}

export function normalizeLineage(record: UnknownRecord): WorkbenchLineage {
  return {
    canonicalRequestId: readString(readField(record, 'canonicalRequestId', 'canonical_request_id')),
    canonicalUsageEventId: readString(readField(record, 'canonicalUsageEventId', 'canonical_usage_event_id')),
    acceptedQuoteId: readString(readField(record, 'acceptedQuoteId', 'accepted_quote_id')),
    acceptedQuoteSha: readString(readField(record, 'acceptedQuoteSha', 'accepted_quote_sha')),
    journalId: readString(readField(record, 'journalId', 'journal_id')),
    mediaTaskId: readString(readField(record, 'mediaTaskId', 'media_task_id')),
    canonicalMediaBusinessEventId: readString(readField(record, 'canonicalMediaBusinessEventId', 'canonical_media_business_event_id')),
    runnerJobId: readString(readField(record, 'runnerJobId', 'runner_job_id')),
    adapterDigest: readString(readField(record, 'adapterDigest', 'adapter_digest')),
    capabilityDigest: readString(readField(record, 'capabilityDigest', 'capability_digest')),
    artifactId: readString(readField(record, 'artifactId', 'artifact_id', 'id')),
    artifactDigest: readString(readField(record, 'artifactDigest', 'artifact_digest')),
    upstreamTaskId: readString(readField(record, 'upstreamTaskId', 'upstream_task_id')),
  }
}

export function normalizeArtifact(value: unknown): WorkbenchArtifact | null {
  if (!isRecord(value)) return null
  const id = readString(readField(value, 'artifactId', 'artifact_id', 'id'))
  const kind = readArtifactKind(readField(value, 'kind'))
  return {
    artifactId: id,
    id,
    runId: readString(readField(value, 'runId', 'run_id')),
    kind,
    title: readString(readField(value, 'title'), `${kind} 工件`),
    preview: readString(readField(value, 'preview', 'text_preview')),
    downloadLabel: readString(readField(value, 'downloadLabel', 'download_label'), kind === 'code' ? '下载代码' : '打开工件'),
    contentType: readString(readField(value, 'contentType', 'content_type')),
    href: readString(readField(value, 'href', 'content_url')),
    storageUri: readString(readField(value, 'storageUri', 'storage_uri')),
    byteSize: readIntegerText(readField(value, 'byteSize', 'byte_size')),
    verifiedAcceptedTask: readBoolean(
      readField(value, 'verifiedAcceptedTask', 'verified_accepted_task', 'acceptedTaskVerified', 'accepted_task_verified'),
    ),
    createdAt: readTimestamp(readField(value, 'createdAt', 'created_at')),
    lineage: normalizeLineage(value),
  }
}

function runRecord(value: unknown): UnknownRecord | null {
  const unwrapped = unwrapApiData(value)
  if (!isRecord(unwrapped)) return null
  const nested = readField(unwrapped, 'run')
  return isRecord(nested) ? nested : unwrapped
}

export function normalizeWorkbenchRun(
  value: unknown,
  capabilities: readonly WorkbenchCapability[] = [],
): WorkbenchRunRecord | null {
  const record = runRecord(value)
  if (!record) return null
  const id = readString(readField(record, 'id'))
  if (!id) return null
  const input = readNestedRecord(record, 'input')
  const capabilityId = readString(readField(input, 'capabilityId', 'capability_id'))
    || readString(readField(record, 'capabilityId', 'capability_id'))
  const capability = capabilities.find((candidate) => candidate.id === capabilityId)
  const status = readRunStatus(readField(record, 'state', 'status'))
  const legacyCost = readNestedRecord(record, 'cost')
  const legacyCurrency = readString(readField(legacyCost, 'currency'), capability?.price.currency ?? '')
  const artifactValues = readArray(readField(record, 'artifacts'))
  const legacyArtifact = readField(record, 'artifact')
  const artifacts = (artifactValues.length ? artifactValues : legacyArtifact === undefined ? [] : [legacyArtifact])
    .map(normalizeArtifact)
    .filter((artifact): artifact is WorkbenchArtifact => artifact !== null)
  const failure = readNestedRecord(record, 'failure')
  const estimate = normalizeMoney(readField(record, 'estimatedCost', 'estimated_cost'), legacyCurrency)
    ?? normalizeMoney(readField(legacyCost, 'estimate'), legacyCurrency)
  const actual = normalizeMoney(readField(record, 'actualCost', 'actual_cost'), legacyCurrency)
    ?? normalizeMoney(readField(legacyCost, 'actual'), legacyCurrency)
  return {
    id,
    status,
    capabilityId,
    capabilityLabel: capability?.title ?? readString(readField(record, 'capabilityLabel', 'capability_label'), '已下线能力'),
    intent: readString(readField(input, 'intent')) || readString(readField(record, 'intent')),
    requestedAt: readTimestamp(readField(record, 'createdAt', 'created_at', 'requestedAt', 'requested_at')),
    updatedAt: readTimestamp(readField(record, 'updatedAt', 'updated_at')),
    terminalAt: readTimestamp(readField(record, 'terminalAt', 'terminal_at')),
    workspaceVersion: readUnsignedInteger(readField(record, 'workspaceVersion', 'workspace_version', 'version')),
    cursor: readCursor(readField(record, 'cursor')),
    cost: {
      currency: estimate?.currency || actual?.currency || legacyCurrency,
      estimate: estimate?.amount ?? null,
      actual: actual?.amount ?? null,
      explanation: readString(readField(legacyCost, 'explanation')),
    },
    artifact: artifacts[0] ?? null,
    artifacts,
    cancellable: readBoolean(readField(record, 'cancellable'), status === 'queued' || status === 'running' || status === 'cancel_requested'),
    retryable: readBoolean(readField(record, 'retryable'), status === 'failed' || status === 'cancelled'),
    saveable: readBoolean(readField(record, 'saveable'), status === 'succeeded' && artifacts.length > 0),
    replayable: readBoolean(readField(record, 'replayable'), status === 'succeeded'),
    forkable: readBoolean(readField(record, 'forkable'), status === 'succeeded'),
    staleCapability: !capability,
    replayOfRunId: readString(readField(record, 'replayOfRunId', 'replay_of_run_id')),
    forkedFromRunId: readString(readField(record, 'forkedFromRunId', 'forked_from_run_id')),
    failureCode: readString(readField(failure, 'code')),
    failureMessage: readString(readField(failure, 'message')) || readString(readField(record, 'failureMessage', 'failure_message')),
    lineage: normalizeLineage(record),
  }
}
