export type CheckinOperationType = 'free' | 'balance'
export type CreditLotteryOperationAction = 'start' | 'continue' | 'settle'

export interface PendingCheckinOperation {
  operationId: string
  type: CheckinOperationType
  createdAt: number
  lastAttemptAt?: number
}

export interface PendingCreditLotteryOperation {
  operationId: string
  action: CreditLotteryOperationAction
  sessionId?: number
  expectedRound?: number
  createdAt: number
  lastAttemptAt?: number
}

export const LOTTERY_OPERATION_RETRY_AFTER_MS = 15_000

const CHECKIN_STORAGE_PREFIX = 'bizdecipher:checkin-operation:'
const CREDIT_LOTTERY_STORAGE_PREFIX = 'bizdecipher:credit-lottery-operation:'

function userStorageSuffix(userId: number | string | null | undefined) {
  return String(userId ?? 'anonymous')
}

export function checkinOperationStorageKey(userId: number | string | null | undefined) {
  return `${CHECKIN_STORAGE_PREFIX}${userStorageSuffix(userId)}`
}

export function creditLotteryOperationStorageKey(userId: number | string | null | undefined) {
  return `${CREDIT_LOTTERY_STORAGE_PREFIX}${userStorageSuffix(userId)}`
}

export function readPendingCheckinOperation(userId: number | string | null | undefined): PendingCheckinOperation | null {
  const value = readStorage(checkinOperationStorageKey(userId))
  if (!value || typeof value.operationId !== 'string' || !['free', 'balance'].includes(String(value.type))) return null
  return {
    operationId: value.operationId,
    type: value.type as CheckinOperationType,
    createdAt: Number(value.createdAt || 0),
    lastAttemptAt: optionalTimestamp(value.lastAttemptAt)
  }
}

export function storePendingCheckinOperation(userId: number | string | null | undefined, operation: PendingCheckinOperation) {
  writeStorage(checkinOperationStorageKey(userId), operation)
}

export function clearPendingCheckinOperation(userId: number | string | null | undefined, operationId?: string) {
  clearStorage(checkinOperationStorageKey(userId), operationId)
}

export function readPendingCreditLotteryOperation(userId: number | string | null | undefined): PendingCreditLotteryOperation | null {
  const value = readStorage(creditLotteryOperationStorageKey(userId))
  if (!value || typeof value.operationId !== 'string' || !['start', 'continue', 'settle'].includes(String(value.action))) return null
  return {
    operationId: value.operationId,
    action: value.action as CreditLotteryOperationAction,
    sessionId: typeof value.sessionId === 'number' ? value.sessionId : undefined,
    expectedRound: typeof value.expectedRound === 'number' ? value.expectedRound : undefined,
    createdAt: Number(value.createdAt || 0),
    lastAttemptAt: optionalTimestamp(value.lastAttemptAt)
  }
}

export function storePendingCreditLotteryOperation(userId: number | string | null | undefined, operation: PendingCreditLotteryOperation) {
  writeStorage(creditLotteryOperationStorageKey(userId), operation)
}

export function clearPendingCreditLotteryOperation(userId: number | string | null | undefined, operationId?: string) {
  clearStorage(creditLotteryOperationStorageKey(userId), operationId)
}

export function lotteryOperationRetryRemainingMs(
  operation: Pick<PendingCheckinOperation, 'createdAt' | 'lastAttemptAt'>,
  now = Date.now()
) {
  const attemptAt = Number(operation.lastAttemptAt || operation.createdAt || now)
  return Math.max(LOTTERY_OPERATION_RETRY_AFTER_MS - Math.max(now - attemptAt, 0), 0)
}

export type LotteryOperationNotFoundReason = 'CHECKIN_OPERATION_NOT_FOUND' | 'CREDIT_LOTTERY_OPERATION_NOT_FOUND'

export function isLotteryOperationNotFoundError(error: unknown, expectedReason: LotteryOperationNotFoundReason) {
  if (!error || typeof error !== 'object') return false
  const value = error as {
    status?: number
    code?: string
    reason?: string
    response?: { status?: number; data?: { code?: string; reason?: string } }
  }
  const statusCode = Number(value.status ?? value.response?.status ?? 0)
  const reason = String(value.reason ?? value.response?.data?.reason ?? '')
  const legacyCode = String(value.code ?? value.response?.data?.code ?? '')
  return statusCode === 404 && (reason === expectedReason || legacyCode === expectedReason)
}

function optionalTimestamp(value: unknown) {
  const timestamp = Number(value || 0)
  return Number.isFinite(timestamp) && timestamp > 0 ? timestamp : undefined
}

function readStorage(key: string): Record<string, unknown> | null {
  if (typeof sessionStorage === 'undefined') return null
  try {
    const raw = sessionStorage.getItem(key)
    if (!raw) return null
    const parsed = JSON.parse(raw)
    return parsed && typeof parsed === 'object' ? parsed as Record<string, unknown> : null
  } catch {
    sessionStorage.removeItem(key)
    return null
  }
}

function writeStorage(key: string, value: object) {
  if (typeof sessionStorage === 'undefined') return
  try {
    sessionStorage.setItem(key, JSON.stringify(value))
  } catch {
    // Recovery remains best-effort when browser storage is unavailable.
  }
}

function clearStorage(key: string, operationId?: string) {
  if (typeof sessionStorage === 'undefined') return
  try {
    if (operationId) {
      const current = readStorage(key)
      if (current?.operationId !== operationId) return
    }
    sessionStorage.removeItem(key)
  } catch {
    // Ignore storage failures after the server result is confirmed.
  }
}
