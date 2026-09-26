import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import axios from 'axios'
import type { AxiosInstance } from 'axios'

// 需要在导入 client 之前设置 mock
vi.mock('@/i18n', () => ({
  getLocale: () => 'zh-CN',
}))

describe('API Client', () => {
  let apiClient: AxiosInstance

  beforeEach(async () => {
    localStorage.clear()
    window.history.replaceState({}, '', '/')
    // 每次测试重新导入以获取干净的模块状态
    vi.resetModules()
    const mod = await import('@/api/client')
    apiClient = mod.apiClient
  })

  afterEach(() => {
    vi.restoreAllMocks()
    vi.unstubAllEnvs()
  })

  // --- 请求拦截器 ---

  describe('authentication session isolation', () => {
    it('expires the generation before pending successful responses arrive', async () => {
      const { getAuthGeneration } = await import('@/utils/authSession')
      window.history.replaceState({}, '', '/login')
      localStorage.setItem('auth_token', 'A')
      let resolve!: (value: unknown) => void
      let sent: unknown
      apiClient.defaults.adapter = vi.fn(config => {
        if (config.url === '/late') {
          sent = config
          return new Promise(res => { resolve = res })
        }
        return Promise.reject({ config, response: { status: 401, data: {} } })
      })
      const before = getAuthGeneration()
      const late = expect(apiClient.get('/late')).rejects.toMatchObject({ code: 'AUTH_SESSION_CHANGED' })
      await vi.waitFor(() => expect(sent).toBeDefined())
      await expect(apiClient.get('/expired')).rejects.toBeDefined()
      expect(getAuthGeneration()).toBeGreaterThan(before)
      resolve({ config: sent, status: 200, headers: {}, data: { code: 0, data: { id: 'A' } } })
      await late
      expect(localStorage.getItem('auth_token')).toBeNull()
    })
    it('does not refresh or clear the new session for an old 401', async () => {
      const { advanceAuthGeneration } = await import('@/utils/authSession')
      localStorage.setItem('auth_token', 'A')
      localStorage.setItem('refresh_token', 'refresh-A')
      let reject!: (error: unknown) => void
      let sentConfig: unknown
      apiClient.defaults.adapter = vi.fn(config => {
        sentConfig = config
        return new Promise((_resolve, rej) => { reject = rej })
      })
      const refreshSpy = vi.spyOn(axios, 'post')
      const pending = apiClient.get('/auth/me')
      const checked = expect(pending).rejects.toMatchObject({ code: 'AUTH_SESSION_CHANGED' })
      await vi.waitFor(() => expect(sentConfig).toBeDefined())
      advanceAuthGeneration()
      localStorage.setItem('auth_token', 'B')
      localStorage.setItem('refresh_token', 'refresh-B')
      reject({ config: sentConfig, response: { status: 401, data: {} } })
      await checked
      expect(refreshSpy).not.toHaveBeenCalled()
      expect(localStorage.getItem('auth_token')).toBe('B')
    })

    it.each(['success', 'failure'] as const)('settles old refresh and queued requests on %s without touching B', async (outcome) => {
      const { advanceAuthGeneration } = await import('@/utils/authSession')
      localStorage.setItem('auth_token', 'A')
      localStorage.setItem('refresh_token', 'refresh-A')
      let resolveA!: (value: unknown) => void
      let rejectA!: (error: unknown) => void
      const refreshSpy = vi.spyOn(axios, 'post').mockImplementationOnce(() =>
        new Promise((resolve, reject) => { resolveA = resolve; rejectA = reject }) as ReturnType<typeof axios.post>,
      ).mockResolvedValueOnce({ data: { code: 0, data: {
        access_token: 'B-rotated', refresh_token: 'refresh-B-rotated', expires_in: 3600,
      } } })
      const adapter = vi.fn(async (config) => {
        if (!config._retry) throw { config, response: { status: 401, data: { code: 'TOKEN_EXPIRED' } } }
        return { config, status: 200, statusText: 'OK', headers: {}, data: { code: 0, data: { ok: true } } }
      })
      apiClient.defaults.adapter = adapter
      const first = apiClient.get('/a-first')
      const second = apiClient.get('/a-queued')
      const settled = Promise.allSettled([first, second])
      await vi.waitFor(() => expect(adapter).toHaveBeenCalledTimes(2))
      expect(refreshSpy).toHaveBeenCalledTimes(1)
      advanceAuthGeneration()
      localStorage.setItem('auth_token', 'B')
      localStorage.setItem('refresh_token', 'refresh-B')
      const b = await apiClient.get('/b')
      expect(b.data).toEqual({ ok: true })
      if (outcome === 'success') resolveA({ data: { code: 0, data: {
        access_token: 'A-rotated', refresh_token: 'refresh-A-rotated', expires_in: 3600,
      } } })
      else rejectA(new Error('A refresh failed'))
      const results = await settled
      expect(results).toHaveLength(2)
      for (const result of results) {
        expect(result.status).toBe('rejected')
        if (result.status === 'rejected') expect(result.reason.code).toBe('AUTH_SESSION_CHANGED')
      }
      expect(localStorage.getItem('auth_token')).toBe('B-rotated')
      expect(localStorage.getItem('refresh_token')).toBe('refresh-B-rotated')
      expect(refreshSpy).toHaveBeenCalledTimes(2)
      expect(adapter).toHaveBeenCalledTimes(4)
    })

    it('discards stale successful responses but accepts rotation within one session', async () => {
      const { advanceAuthGeneration } = await import('@/utils/authSession')
      let resolve!: (value: unknown) => void
      let sentConfig: unknown
      apiClient.defaults.adapter = vi.fn(config => {
        sentConfig = config
        return new Promise(res => { resolve = res })
      })
      const stale = apiClient.get('/auth/me')
      const checked = expect(stale).rejects.toMatchObject({ code: 'AUTH_SESSION_CHANGED' })
      await vi.waitFor(() => expect(sentConfig).toBeDefined())
      advanceAuthGeneration()
      resolve({ config: sentConfig, status: 200, headers: {}, data: { code: 0, data: { id: 'A' } } })
      await checked
      apiClient.defaults.adapter = vi.fn(async config => {
        localStorage.setItem('auth_token', 'rotated')
        return { config, status: 200, statusText: 'OK', headers: {}, data: { code: 0, data: { id: 'B' } } }
      })
      expect((await apiClient.get('/auth/me')).data).toEqual({ id: 'B' })
    })
  })

  describe('请求拦截器', () => {
    it('规范化相对 API base，避免在回调页拼出相对 v1 路径', async () => {
      vi.resetModules()
      vi.stubEnv('VITE_API_BASE_URL', 'api/v1')

      const mod = await import('@/api/client')

      expect(mod.apiClient.defaults.baseURL).toBe('/api/v1')
      expect(mod.buildApiUrl('/auth/oauth/github/callback?code=abc')).toBe(
        '/api/v1/auth/oauth/github/callback?code=abc'
      )
    })

    it('自动附加 Authorization 头', async () => {
      localStorage.setItem('auth_token', 'my-jwt-token')

      // 拦截实际请求
      const adapter = vi.fn().mockResolvedValue({
        status: 200,
        data: { code: 0, data: {} },
        headers: {},
        config: {},
        statusText: 'OK',
      })
      apiClient.defaults.adapter = adapter

      await apiClient.get('/test')

      const config = adapter.mock.calls[0][0]
      expect(config.headers.get('Authorization')).toBe('Bearer my-jwt-token')
    })

    it('无 token 时不附加 Authorization 头', async () => {
      const adapter = vi.fn().mockResolvedValue({
        status: 200,
        data: { code: 0, data: {} },
        headers: {},
        config: {},
        statusText: 'OK',
      })
      apiClient.defaults.adapter = adapter

      await apiClient.get('/test')

      const config = adapter.mock.calls[0][0]
      expect(config.headers.get('Authorization')).toBeFalsy()
    })

    it('GET 请求自动附加 timezone 参数', async () => {
      const adapter = vi.fn().mockResolvedValue({
        status: 200,
        data: { code: 0, data: {} },
        headers: {},
        config: {},
        statusText: 'OK',
      })
      apiClient.defaults.adapter = adapter

      await apiClient.get('/test')

      const config = adapter.mock.calls[0][0]
      expect(config.params).toHaveProperty('timezone')
    })

    it('POST 请求不附加 timezone 参数', async () => {
      const adapter = vi.fn().mockResolvedValue({
        status: 200,
        data: { code: 0, data: {} },
        headers: {},
        config: {},
        statusText: 'OK',
      })
      apiClient.defaults.adapter = adapter

      await apiClient.post('/test', { foo: 'bar' })

      const config = adapter.mock.calls[0][0]
      expect(config.params?.timezone).toBeUndefined()
    })

    it('请求默认带 withCredentials 以支持跨域 cookie', async () => {
      const adapter = vi.fn().mockResolvedValue({
        status: 200,
        data: { code: 0, data: {} },
        headers: {},
        config: {},
        statusText: 'OK',
      })
      apiClient.defaults.adapter = adapter

      await apiClient.post('/auth/oauth/bind-token')

      const config = adapter.mock.calls[0][0]
      expect(config.withCredentials).toBe(true)
    })

    it('Admin API 在进入管理页面前也带 Admin UI 标记', async () => {
      const adapter = vi.fn().mockResolvedValue({
        status: 200,
        data: { code: 0, data: {} },
        headers: {},
        config: {},
        statusText: 'OK',
      })
      apiClient.defaults.adapter = adapter

      await apiClient.get('/admin/users')

      const config = adapter.mock.calls[0][0]
      expect(config.headers.get('X-Admin-UI-Request')).toBe('1')
    })

    it('管理页面调用共享 API 时带 Admin UI 标记', async () => {
      window.history.replaceState({}, '', '/admin/dashboard')
      const adapter = vi.fn().mockResolvedValue({
        status: 200,
        data: { code: 0, data: {} },
        headers: {},
        config: {},
        statusText: 'OK',
      })
      apiClient.defaults.adapter = adapter

      await apiClient.get('/groups/available')

      const config = adapter.mock.calls[0][0]
      expect(config.headers.get('X-Admin-UI-Request')).toBe('1')
    })

    it('普通用户页面调用共享 API 时不带 Admin UI 标记', async () => {
      const adapter = vi.fn().mockResolvedValue({
        status: 200,
        data: { code: 0, data: {} },
        headers: {},
        config: {},
        statusText: 'OK',
      })
      apiClient.defaults.adapter = adapter

      await apiClient.get('/groups/available')

      const config = adapter.mock.calls[0][0]
      expect(config.headers.get('X-Admin-UI-Request')).toBeFalsy()
    })

    it('用户侧 timing API 自动带 User UI 标记', async () => {
      const adapter = vi.fn().mockResolvedValue({
        status: 200,
        data: { code: 0, data: {} },
        headers: {},
        config: {},
        statusText: 'OK',
      })
      apiClient.defaults.adapter = adapter

      await apiClient.get('/auth/me')

      const config = adapter.mock.calls[0][0]
      expect(config.headers.get('X-User-UI-Request')).toBe('1')
      expect(config.headers.get('X-Admin-UI-Request')).toBeFalsy()
    })

    it('支付用户 API 带 User UI 标记，公开支付 API 不带', async () => {
      const adapter = vi.fn().mockResolvedValue({
        status: 200,
        data: { code: 0, data: {} },
        headers: {},
        config: {},
        statusText: 'OK',
      })
      apiClient.defaults.adapter = adapter

      await apiClient.get('/payment/plans')
      expect(adapter.mock.calls[0][0].headers.get('X-User-UI-Request')).toBe('1')

      await apiClient.post('/payment/public/orders/verify', {})
      expect(adapter.mock.calls[1][0].headers.get('X-User-UI-Request')).toBeFalsy()
    })

    it('管理页调用共享 API 时同时带 Admin 与 User UI 标记', async () => {
      window.history.replaceState({}, '', '/admin/dashboard')
      const adapter = vi.fn().mockResolvedValue({
        status: 200,
        data: { code: 0, data: {} },
        headers: {},
        config: {},
        statusText: 'OK',
      })
      apiClient.defaults.adapter = adapter

      await apiClient.get('/keys')

      const config = adapter.mock.calls[0][0]
      expect(config.headers.get('X-Admin-UI-Request')).toBe('1')
      expect(config.headers.get('X-User-UI-Request')).toBe('1')
    })
  })

  // --- 响应拦截器 ---

  describe('响应拦截器', () => {
    it('code=0 时解包 data 字段', async () => {
      const adapter = vi.fn().mockResolvedValue({
        status: 200,
        data: { code: 0, data: { name: 'test' }, message: 'ok' },
        headers: {},
        config: {},
        statusText: 'OK',
      })
      apiClient.defaults.adapter = adapter

      const response = await apiClient.get('/test')
      expect(response.data).toEqual({ name: 'test' })
    })

    it('code!=0 时拒绝并返回结构化错误', async () => {
      const adapter = vi.fn().mockResolvedValue({
        status: 200,
        data: { code: 1001, message: '参数错误', data: null },
        headers: {},
        config: {},
        statusText: 'OK',
      })
      apiClient.defaults.adapter = adapter

      await expect(apiClient.get('/test')).rejects.toEqual(
        expect.objectContaining({
          code: 1001,
          message: '参数错误',
        })
      )
    })

    it('部署与运营合规未确认时广播事件且保留登录态', async () => {
      localStorage.setItem('auth_token', 'admin-token')
      const listener = vi.fn()
      window.addEventListener('admin-compliance-required', listener)

      const adapter = vi.fn().mockRejectedValue({
        response: {
          status: 423,
          data: {
            code: 'ADMIN_COMPLIANCE_ACK_REQUIRED',
            message: 'administrator compliance acknowledgement is required',
            metadata: {
              version: 'v2026.06.10',
              document_path_zh: 'docs/legal/admin-compliance.zh.md',
              document_path_en: 'docs/legal/admin-compliance.en.md',
            },
          },
        },
        config: {
          url: '/admin/users',
          headers: { Authorization: 'Bearer admin-token' },
        },
        code: 'ERR_BAD_REQUEST',
      })
      apiClient.defaults.adapter = adapter

      await expect(apiClient.get('/admin/users')).rejects.toEqual(
        expect.objectContaining({
          status: 423,
          code: 'ADMIN_COMPLIANCE_ACK_REQUIRED',
          metadata: expect.objectContaining({
            version: 'v2026.06.10',
          }),
        })
      )

      expect(listener).toHaveBeenCalledTimes(1)
      expect((listener.mock.calls[0][0] as CustomEvent).detail).toEqual(
        expect.objectContaining({
          version: 'v2026.06.10',
        })
      )
      expect(localStorage.getItem('auth_token')).toBe('admin-token')

      window.removeEventListener('admin-compliance-required', listener)
    })
  })

  // --- 401 Token 刷新 ---

  describe('401 Token 刷新', () => {
    it('刷新请求最多等待五秒，失败时清空刷新队列', async () => {
      localStorage.setItem('auth_token', 'expired-token')
      localStorage.setItem('refresh_token', 'refresh-token')

      const originalLocation = window.location
      Object.defineProperty(window, 'location', {
        value: { ...originalLocation, pathname: '/dashboard', href: '/dashboard' },
        writable: true,
      })

      const refreshError = Object.assign(new Error('refresh timeout'), { code: 'ECONNABORTED' })
      const refreshSpy = vi.spyOn(axios, 'post').mockRejectedValue(refreshError)
      const adapter = vi.fn().mockRejectedValue({
        response: {
          status: 401,
          data: { code: 'TOKEN_EXPIRED', message: 'Token expired' },
        },
        config: {
          url: '/test',
          headers: { Authorization: 'Bearer expired-token' },
        },
        code: 'ERR_BAD_REQUEST',
      })
      apiClient.defaults.adapter = adapter

      const first = apiClient.get('/test')
      const second = apiClient.get('/test')
      const results = await Promise.allSettled([first, second])

      expect(refreshSpy).toHaveBeenCalledTimes(1)
      expect(results.every((result) => result.status === 'rejected')).toBe(true)
      const errorCodes = results
        .filter((result): result is PromiseRejectedResult => result.status === 'rejected')
        .map((result) => result.reason?.code)
      expect(errorCodes).toContain('TOKEN_REFRESH_FAILED')
      expect(errorCodes).toContain('TOKEN_EXPIRED')
      expect(refreshSpy.mock.calls[0][2]).toEqual(
        expect.objectContaining({
          timeout: 5000,
          timeoutErrorMessage: '登录状态确认超时，请重新登录。',
        })
      )

      Object.defineProperty(window, 'location', {
        value: originalLocation,
        writable: true,
      })
    })

    it('无 refresh_token 时 401 清除 localStorage', async () => {
      localStorage.setItem('auth_token', 'expired-token')
      // 不设置 refresh_token

      // Mock window.location
      const originalLocation = window.location
      Object.defineProperty(window, 'location', {
        value: { ...originalLocation, pathname: '/dashboard', href: '/dashboard' },
        writable: true,
      })

      const adapter = vi.fn().mockRejectedValue({
        response: {
          status: 401,
          data: { code: 'TOKEN_EXPIRED', message: 'Token expired' },
        },
        config: {
          url: '/test',
          headers: { Authorization: 'Bearer expired-token' },
        },
        code: 'ERR_BAD_REQUEST',
      })
      apiClient.defaults.adapter = adapter

      await expect(apiClient.get('/test')).rejects.toBeDefined()

      expect(localStorage.getItem('auth_token')).toBeNull()

      // 恢复 location
      Object.defineProperty(window, 'location', {
        value: originalLocation,
        writable: true,
      })
    })

    it('本地预览 token 收到 401 时保留 localStorage 且不跳转登录', async () => {
      localStorage.setItem('auth_token', 'preview-token-local-only')
      localStorage.setItem('auth_user', JSON.stringify({ id: 1, role: 'user' }))

      const originalLocation = window.location
      Object.defineProperty(window, 'location', {
        value: { ...originalLocation, pathname: '/community', href: '/community' },
        writable: true,
      })

      const adapter = vi.fn().mockRejectedValue({
        response: {
          status: 401,
          data: { code: 'INVALID_TOKEN', message: 'Invalid token' },
        },
        config: {
          url: '/subscriptions/active',
          headers: { Authorization: 'Bearer preview-token-local-only' },
        },
        code: 'ERR_BAD_REQUEST',
      })
      apiClient.defaults.adapter = adapter

      await expect(apiClient.get('/subscriptions/active')).rejects.toEqual(
        expect.objectContaining({
          status: 401,
          code: 'INVALID_TOKEN',
          message: 'Invalid token',
        })
      )

      expect(localStorage.getItem('auth_token')).toBe('preview-token-local-only')
      expect(localStorage.getItem('auth_user')).toBe(JSON.stringify({ id: 1, role: 'user' }))
      expect(window.location.href).toBe('/community')

      Object.defineProperty(window, 'location', {
        value: originalLocation,
        writable: true,
      })
    })
  })

  describe('共享池探测请求', () => {
    it('只提交后台任务并附带同一个幂等键', async () => {
      const adapter = vi.fn().mockResolvedValue({
        status: 202,
        data: { code: 0, data: { id: 'probe-job-1', status: 'queued' } },
        headers: {},
        config: {},
        statusText: 'Accepted',
      })
      apiClient.defaults.adapter = adapter
      const { probeSharedPoolUpstream } = await import('@/api/bizdecipher')

      await probeSharedPoolUpstream({
        pool_id: 168,
        probe_type: 'manual',
        operation_id: 'probe-operation-168',
      })

      const config = adapter.mock.calls[0][0]
      expect(config.timeout).toBe(30000)
      expect(config.headers.get('Idempotency-Key')).toBe('probe-operation-168')
      expect(JSON.parse(config.data)).toEqual(expect.objectContaining({
        operation_id: 'probe-operation-168',
      }))
    })
  })

  // --- 网络错误 ---

  describe('网络错误', () => {
    it('超时错误返回可识别的超时信息', async () => {
      const adapter = vi.fn().mockRejectedValue({
        code: 'ECONNABORTED',
        message: '满血检测超过 3 分钟，请稍后重试或检查上游响应速度。',
        config: { url: '/biz/upstream/probe' },
      })
      apiClient.defaults.adapter = adapter

      await expect(apiClient.post('/biz/upstream/probe')).rejects.toEqual(
        expect.objectContaining({
          status: 0,
          code: 'REQUEST_TIMEOUT',
          message: '满血检测超过 3 分钟，请稍后重试或检查上游响应速度。',
        })
      )
    })

    it('网络错误返回 status 0 的错误', async () => {
      const adapter = vi.fn().mockRejectedValue({
        code: 'ERR_NETWORK',
        message: 'Network Error',
        config: { url: '/test' },
        // 没有 response
      })
      apiClient.defaults.adapter = adapter

      await expect(apiClient.get('/test')).rejects.toEqual(
        expect.objectContaining({
          status: 0,
          message: 'Network error. Please check your connection.',
        })
      )
    })
  })

  // --- 请求取消 ---

  describe('请求取消', () => {
    it('取消的请求保持原始取消错误', async () => {
      const source = axios.CancelToken.source()

      const adapter = vi.fn().mockRejectedValue(
        new axios.Cancel('Operation canceled')
      )
      apiClient.defaults.adapter = adapter

      await expect(
        apiClient.get('/test', { cancelToken: source.token })
      ).rejects.toBeDefined()
    })
  })
})
