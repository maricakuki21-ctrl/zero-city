import { describe, expect, it } from 'vitest'
import { extractActionableApiErrorMessage } from '@/utils/apiError'

describe('extractActionableApiErrorMessage', () => {
  it('keeps a specific server message actionable', () => {
    expect(extractActionableApiErrorMessage(
      { message: '当前账号没有访问该共享池的权限' },
      '共享池市场加载失败，请重试。',
    )).toBe('当前账号没有访问该共享池的权限')
  })

  it.each(['Network Error', 'Request failed with status code 503'])(
    'replaces the generic transport message %s with the supplied fallback',
    (message) => {
      expect(extractActionableApiErrorMessage(
        { message },
        '共享池市场加载失败，请重试。',
      )).toBe('共享池市场加载失败，请重试。')
    },
  )
})
