import { describe, expect, it } from 'vitest'

import router from '../index'

describe('account and My Zero City routes', () => {
  it('keeps /profile as a compatibility redirect into My Zero City', () => {
    const route = router.getRoutes().find(record => record.path === '/profile')

    expect(route?.redirect).toBe('/community?workspace=mine')
  })

  it('keeps account security and identity controls on the account settings route', () => {
    const resolved = router.resolve('/settings/account')

    expect(resolved.name).toBe('AccountSettings')
    expect(resolved.meta.requiresAuth).toBe(true)
    expect(resolved.meta.title).toBe('账户设置')
  })
})
