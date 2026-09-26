import { describe, expect, it } from 'vitest'

import en from '../locales/en'
import enCustom from '../locales/en/custom'
import zh from '../locales/zh'
import zhCustom from '../locales/zh/custom'
import { mergeLocaleMessages } from '../locales/merge'

function messageAt(messages: Record<string, unknown>, path: string[]): unknown {
  return path.reduce<unknown>((value, key) => {
    if (value === null || typeof value !== 'object' || Array.isArray(value)) return undefined
    return (value as Record<string, unknown>)[key]
  }, messages)
}

function expectMessageSubset(actual: unknown, expected: unknown): void {
  if (expected !== null && typeof expected === 'object' && !Array.isArray(expected)) {
    expect(actual).toBeDefined()
    for (const [key, value] of Object.entries(expected as Record<string, unknown>)) {
      const actualObject = actual as Record<string, unknown>
      expectMessageSubset(actualObject[key], value)
    }
    return
  }

  expect(actual).toEqual(expected)
}

describe('modular locale composition', () => {
  it.each([
    ['en', en, enCustom],
    ['zh', zh, zhCustom],
  ] as const)('%s retains official admin modules and BizDecipher translations', (_locale, messages, custom) => {
    expect(messageAt(messages, ['admin', 'promptAudit', 'title'])).toBeTypeOf('string')
    expect(messageAt(messages, ['admin', 'audit', 'title'])).toBeTypeOf('string')
    expectMessageSubset(messageAt(messages, ['biz']), messageAt(custom, ['biz']))
    expectMessageSubset(messageAt(messages, ['community']), messageAt(custom, ['community']))
    const payment = messageAt(messages, ['payment']) as Record<string, unknown>
    expectMessageSubset(payment, messageAt(custom, ['payment']))
    expect(payment).toHaveProperty('status')
    expect((payment.status as Record<string, unknown>).refund_pending).toBeTypeOf('string')
  })

  it('uses later custom translations without dropping official nested keys', () => {
    expect(mergeLocaleMessages(
      { admin: { promptAudit: { title: 'Prompt Audit', description: 'Official' } } },
      { admin: { promptAudit: { title: 'Biz Audit' } } },
    )).toEqual({
      admin: { promptAudit: { title: 'Biz Audit', description: 'Official' } },
    })
  })
})
