import { beforeEach, describe, expect, it, vi } from 'vitest'
import { getMyZeroCityProfile, updateMyZeroCityProfile } from '../bizdecipher'

const { get, put } = vi.hoisted(() => ({ get: vi.fn(), put: vi.fn() }))
vi.mock('@/api/client', () => ({ apiClient: { get, put } }))

describe('personal city profile API', () => {
  beforeEach(() => { get.mockReset(); put.mockReset() })

  it('uses the authenticated own-profile initializer, not public profile lookup', async () => {
    const profile = { user_id: 7, display_name: '居民昵称' }
    get.mockResolvedValue({ data: profile })
    expect(await getMyZeroCityProfile()).toEqual(profile)
    expect(get).toHaveBeenCalledTimes(1)
    expect(get).toHaveBeenCalledWith('/biz/profile')
  })

  it('propagates failures rather than returning an empty successful profile', async () => {
    get.mockRejectedValue(new Error('service unavailable'))
    await expect(getMyZeroCityProfile()).rejects.toThrow('service unavailable')
  })

  it('updates only the explicit public identity fields', async () => {
    const profile = { user_id: 9, display_name: 'New name', avatar_url: '' }
    put.mockResolvedValue({ data: profile })

    expect(await updateMyZeroCityProfile({ display_name: 'New name', avatar_url: '' })).toEqual(profile)
    expect(put).toHaveBeenCalledWith('/biz/profile', { display_name: 'New name', avatar_url: '' })
  })
})
