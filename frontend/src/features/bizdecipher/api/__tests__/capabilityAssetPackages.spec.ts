import { beforeEach, describe, expect, it, vi } from 'vitest'

const apiClient = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
  put: vi.fn(),
}))

vi.mock('@/api/client', () => ({ apiClient }))

import {
  createCapabilityAssetVersion,
  downloadCapabilityAssetPackage,
  finalizeCapabilityAssetVersion,
  listCapabilityAssetVersions,
  publishCapabilityAssetVersion,
  putCapabilityAssetFile,
  revokeCapabilityAssetVersion,
} from '../bizdecipher'

describe('capability asset package API', () => {
  beforeEach(() => {
    apiClient.get.mockReset()
    apiClient.post.mockReset()
    apiClient.put.mockReset()
  })

  it('lists immutable versions through the asset-scoped endpoint', async () => {
    apiClient.get.mockResolvedValue({ data: { items: [{ id: 1, version: 'v1.0.0' }] } })

    await expect(listCapabilityAssetVersions(41)).resolves.toEqual([{ id: 1, version: 'v1.0.0' }])
    expect(apiClient.get).toHaveBeenCalledWith('/biz/assets/41/versions')
  })

  it('creates a declarative version with an object manifest', async () => {
    apiClient.post.mockResolvedValue({ data: { id: 2, version: 'v1.0.0' } })

    await createCapabilityAssetVersion(41, {
      version: 'v1.0.0',
      runtime_kind: 'workflow',
      manifest: { schema_version: 1, capabilities: [] },
    })

    expect(apiClient.post).toHaveBeenCalledWith('/biz/assets/41/versions', {
      version: 'v1.0.0',
      runtime_kind: 'workflow',
      manifest: { schema_version: 1, capabilities: [] },
    })
  })

  it('uploads file bytes and runs the immutable lifecycle actions', async () => {
    apiClient.put.mockResolvedValue({ data: { id: 2, file_count: 1 } })
    apiClient.post.mockResolvedValue({ data: { id: 2, status: 'ready' } })

    await putCapabilityAssetFile(41, 'v1.0.0', {
      path: 'workflow.json',
      content_type: 'application/json',
      content_base64: 'e30=',
    })
    await finalizeCapabilityAssetVersion(41, 'v1.0.0')
    await publishCapabilityAssetVersion(41, 'v1.0.0')
    await revokeCapabilityAssetVersion(41, 'v1.0.0')

    expect(apiClient.put).toHaveBeenCalledWith(
      '/biz/assets/41/versions/v1.0.0/files',
      {
        path: 'workflow.json',
        content_type: 'application/json',
        content_base64: 'e30=',
      },
      { timeout: 120000 },
    )
    expect(apiClient.post).toHaveBeenNthCalledWith(1, '/biz/assets/41/versions/v1.0.0/finalize')
    expect(apiClient.post).toHaveBeenNthCalledWith(2, '/biz/assets/41/versions/v1.0.0/publish')
    expect(apiClient.post).toHaveBeenNthCalledWith(3, '/biz/assets/41/versions/v1.0.0/revoke')
  })

  it('downloads the authenticated zip as a blob without unwrapping it as JSON', async () => {
    const blob = new Blob(['zip'])
    apiClient.get.mockResolvedValue({ data: blob })

    await expect(downloadCapabilityAssetPackage(41, 'v1.0.0')).resolves.toBe(blob)
    expect(apiClient.get).toHaveBeenCalledWith(
      '/biz/assets/41/versions/v1.0.0/download',
      { responseType: 'blob', timeout: 120000 },
    )
  })
})
