import { describe, expect, it } from 'vitest'
import {
  isLotteryOperationNotFoundError,
  lotteryOperationRetryRemainingMs,
} from '../lotteryOperationRecovery'

describe('lotteryOperationRecovery', () => {
  it.each([
    { error: { status: 404, reason: 'CHECKIN_OPERATION_NOT_FOUND' }, expected: true },
    { error: { status: 404, code: 'CHECKIN_OPERATION_NOT_FOUND' }, expected: true },
    { error: { response: { status: 404, data: { reason: 'CHECKIN_OPERATION_NOT_FOUND' } } }, expected: true },
    { error: { status: 500, code: 'CHECKIN_OPERATION_NOT_FOUND' }, expected: false },
    { error: { status: 500, reason: 'CHECKIN_OPERATION_NOT_FOUND' }, expected: false },
    { error: { status: 404 }, expected: false },
    { error: { status: 404, code: 'CREDIT_LOTTERY_OPERATION_NOT_FOUND' }, expected: false },
    { error: { status: 404, code: 'ROUTE_NOT_FOUND' }, expected: false },
  ])('requires a dedicated HTTP 404 for checkin recovery: $expected', ({ error, expected }) => {
    expect(isLotteryOperationNotFoundError(error, 'CHECKIN_OPERATION_NOT_FOUND')).toBe(expected)
  })

  it.each([
    { error: { status: 404, reason: 'CREDIT_LOTTERY_OPERATION_NOT_FOUND' }, expected: true },
    { error: { response: { status: 404, data: { code: 'CREDIT_LOTTERY_OPERATION_NOT_FOUND' } } }, expected: true },
    { error: { status: 500, reason: 'CREDIT_LOTTERY_OPERATION_NOT_FOUND' }, expected: false },
    { error: { status: 404 }, expected: false },
    { error: { status: 404, code: 'ROUTE_NOT_FOUND' }, expected: false },
    { error: { status: 404, reason: 'CHECKIN_OPERATION_NOT_FOUND' }, expected: false },
  ])('requires a dedicated HTTP 404 for credit lottery recovery: $expected', ({ error, expected }) => {
    expect(isLotteryOperationNotFoundError(error, 'CREDIT_LOTTERY_OPERATION_NOT_FOUND')).toBe(expected)
  })

  it('restarts the safety window from the most recent POST attempt', () => {
    expect(lotteryOperationRetryRemainingMs({ createdAt: 1_000 }, 10_000)).toBe(6_000)
    expect(lotteryOperationRetryRemainingMs({ createdAt: 1_000, lastAttemptAt: 8_000 }, 10_000)).toBe(13_000)
    expect(lotteryOperationRetryRemainingMs({ createdAt: 1_000, lastAttemptAt: 8_000 }, 23_000)).toBe(0)
  })
})
