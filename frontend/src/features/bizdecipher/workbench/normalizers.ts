import type {
  WorkbenchCapability,
  WorkbenchRunRecord,
  WorkbenchRunStatus,
  WorkbenchSavedReplay,
  WorkbenchStreamMessage,
  WorkbenchWorkspace,
} from './contracts'
import {
  isRecord,
  readArray,
  readCursor,
  readExactDecimal,
  readField,
  readNestedRecord,
  readString,
  readTimestamp,
  readUnsignedInteger,
  unwrapApiData,
  type UnknownRecord,
} from './contractValues'
import { normalizeWorkbenchRun } from './runNormalizer'
export { normalizeWorkbenchRun }

function normalizeCapability(value: unknown): WorkbenchCapability | null {
  if (!isRecord(value)) return null
  const id = readString(readField(value, 'id'))
  if (!id) return null
  const price = readNestedRecord(value, 'price')
  const estimate = readNestedRecord(value, 'estimate')
  const amount = readExactDecimal(readField(price, 'amount')) ?? readExactDecimal(readField(estimate, 'amount'))
  return {
    id,
    version: readString(readField(value, 'version'), '1'),
    title: readString(readField(value, 'title'), '未命名能力'),
    summary: readString(readField(value, 'summary'), '暂无能力说明。'),
    tokenBudget: readUnsignedInteger(readField(value, 'tokenBudget', 'token_budget')),
    averageSeconds: readUnsignedInteger(readField(value, 'averageSeconds', 'average_seconds')),
    canonicalModelId: readString(readField(value, 'canonicalModelId', 'canonical_model_id')),
    canonicalModelVersion: readString(readField(value, 'canonicalModelVersion', 'canonical_model_version')),
    capabilityDigest: readString(readField(value, 'capabilityDigest', 'capability_digest', 'digest')),
    price: {
      currency: readString(readField(price, 'currency')) || readString(readField(estimate, 'currency')),
      amount,
      unitLabel: readString(readField(price, 'unitLabel', 'unit_label'), '每次'),
      acceptedQuoteId: readString(readField(price, 'acceptedQuoteId', 'accepted_quote_id'))
        || readString(readField(value, 'acceptedQuoteId', 'accepted_quote_id')),
      acceptedQuoteSha: readString(readField(price, 'acceptedQuoteSha', 'accepted_quote_sha'))
        || readString(readField(value, 'acceptedQuoteSha', 'accepted_quote_sha')),
    },
  }
}

export function normalizeWorkbenchSavedReplay(
  value: unknown,
  capabilities: readonly WorkbenchCapability[] = [],
): WorkbenchSavedReplay | null {
  const unwrapped = unwrapApiData(value)
  if (!isRecord(unwrapped)) return null
  const nested = readField(unwrapped, 'snapshot')
  const record = isRecord(nested) ? nested : unwrapped
  const id = readString(readField(record, 'id'))
  if (!id) return null
  const input = readNestedRecord(record, 'input')
  const capabilityId = readString(readField(input, 'capabilityId', 'capability_id'))
    || readString(readField(record, 'capabilityId', 'capability_id'))
  const capability = capabilities.find((candidate) => candidate.id === capabilityId)
  return {
    id,
    label: readString(readField(record, 'label'), '未命名回放'),
    capabilityId,
    capabilityLabel: capability?.title ?? readString(readField(record, 'capabilityLabel', 'capability_label'), '已下线能力'),
    intent: readString(readField(input, 'intent')) || readString(readField(record, 'intent')),
    savedAt: readTimestamp(readField(record, 'createdAt', 'created_at', 'savedAt', 'saved_at')),
    staleCapability: !capability,
    sourceRunId: readString(readField(record, 'runId', 'run_id', 'sourceRunId', 'source_run_id')),
  }
}

function workspaceRecord(value: unknown): UnknownRecord {
  const unwrapped = unwrapApiData(value)
  return isRecord(unwrapped) ? unwrapped : {}
}

export function normalizeWorkbenchWorkspace(value: unknown): WorkbenchWorkspace {
  const record = workspaceRecord(value)
  const capabilities = readArray(readField(record, 'capabilities'))
    .map(normalizeCapability)
    .filter((item): item is WorkbenchCapability => item !== null)
  const currentRun = normalizeWorkbenchRun(readField(record, 'currentRun', 'current_run'), capabilities)
  const draftRecord = readNestedRecord(record, 'draft')
  const input = currentRun === null ? {} : readNestedRecord(workspaceRecord(readField(record, 'currentRun', 'current_run')), 'input')
  const requestedCapabilityId = readString(readField(draftRecord, 'capabilityId', 'capability_id'))
    || readString(readField(input, 'capabilityId', 'capability_id'))
  const capabilityId = capabilities.some((item) => item.id === requestedCapabilityId)
    ? requestedCapabilityId
    : capabilities[0]?.id ?? ''
  const warnings: string[] = []
  if (requestedCapabilityId && requestedCapabilityId !== capabilityId) {
    warnings.push('工作区版本已更新，草稿里的能力已下线，需重新选择能力。')
  }
  if (currentRun?.staleCapability) warnings.push('当前运行引用的能力已下线，仅保留历史证据。')
  const savedReplays = readArray(readField(record, 'savedReplays', 'saved_replays', 'savedSnapshots', 'saved_snapshots'))
    .map((item) => normalizeWorkbenchSavedReplay(item, capabilities))
    .filter((item): item is WorkbenchSavedReplay => item !== null)
  return {
    workspaceToken: readString(readField(record, 'workspaceToken', 'workspace_token', 'id')),
    workspaceVersion: readUnsignedInteger(readField(record, 'workspaceVersion', 'workspace_version', 'version')),
    lastHydratedAt: readTimestamp(readField(record, 'lastHydratedAt', 'last_hydrated_at', 'hydratedAt', 'hydrated_at')),
    cursor: readCursor(readField(record, 'cursor')),
    draft: {
      intent: readString(readField(draftRecord, 'intent')) || currentRun?.intent || '',
      capabilityId,
      locale: readString(readField(draftRecord, 'locale'), 'zh-CN'),
      updatedAt: readTimestamp(readField(draftRecord, 'updatedAt', 'updated_at')),
    },
    capabilities,
    currentRun,
    savedReplays,
    warnings,
  }
}

export function normalizeStreamMessage(
  eventName: string,
  eventId: string,
  value: unknown,
  capabilities: readonly WorkbenchCapability[] = [],
): WorkbenchStreamMessage | null {
  const unwrapped = unwrapApiData(value)
  if (!isRecord(unwrapped)) return null
  if (eventName === 'reset' || readString(readField(unwrapped, 'kind')) === 'reset') {
    return normalizeStreamReset(value, capabilities)
  }
  const run = normalizeWorkbenchRun(readField(unwrapped, 'run', 'snapshot'), capabilities)
    ?? normalizeWorkbenchRun(unwrapped, capabilities)
  const nestedEvent = readNestedRecord(unwrapped, 'event')
  const cursor = readCursor(readField(unwrapped, 'cursor')) || readCursor(readField(nestedEvent, 'seq')) || eventId
  if (!cursor) return null
  return {
    kind: 'event',
    cursor,
    eventKind: eventName || readString(readField(nestedEvent, 'kind')) || readString(readField(unwrapped, 'event_kind', 'kind')),
    run,
  }
}

export function normalizeStreamReset(
  value: unknown,
  capabilities: readonly WorkbenchCapability[] = [],
): Extract<WorkbenchStreamMessage, { readonly kind: 'reset' }> | null {
  const unwrapped = unwrapApiData(value)
  if (!isRecord(unwrapped)) return null
  const run = normalizeWorkbenchRun(readField(unwrapped, 'run', 'snapshot'), capabilities)
  const cursor = readCursor(readField(unwrapped, 'cursor'))
  if (!run || !cursor) return null
  return { kind: 'reset', cursor, reason: readString(readField(unwrapped, 'reason'), 'retention_window_advanced'), run }
}

export function formatMoney(money: { readonly currency: string; readonly amount: string | null } | null): string {
  if (!money?.currency || money.amount === null) return '待报价'
  return `${money.currency} ${money.amount}`
}

export function formatDateTime(value: string): string {
  if (!value) return '刚刚'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '刚刚'
  return new Intl.DateTimeFormat('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' }).format(date)
}

export function runStatusLabel(status: WorkbenchRunStatus): string {
  const labels: Readonly<Record<WorkbenchRunStatus, string>> = {
    idle: '未开始', queued: '排队中', running: '运行中', cancel_requested: '正在取消',
    succeeded: '已完成', failed: '失败', cancelled: '已取消',
  }
  return labels[status]
}

export function stepIndexFromRun(run: WorkbenchRunRecord | null): number {
  if (!run) return 1
  if (run.status === 'queued' || run.status === 'running' || run.status === 'cancel_requested') return 3
  return run.status === 'succeeded' ? 5 : 4
}
