import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import CreatorWorkbenchView from '../CreatorWorkbenchView.vue'
import type {
  CreatorWorkbenchService,
  WorkbenchRunRecord,
  WorkbenchWorkspace,
} from '../contracts'

const baseWorkspace = (): WorkbenchWorkspace => ({
  workspaceToken: 'wb_tok_live',
  workspaceVersion: 4,
  lastHydratedAt: '2026-08-19T07:00:00Z',
  draft: {
    intent: '请生成一段适合首页卡片展示的 CJK 文案，确保超长文本也能换行。',
    capabilityId: 'cap-copy',
    locale: 'zh-CN',
    updatedAt: '2026-08-19T07:00:00Z',
  },
  capabilities: [
    {
      id: 'cap-copy',
      version: '1',
      title: '品牌文案生成',
      summary: '输出短文案、活动标题与 CTA。',
      tokenBudget: 12000,
      averageSeconds: 18,
      canonicalModelId: 'gpt-5.5',
      canonicalModelVersion: '2026-08-19',
      capabilityDigest: 'sha256:cap-copy',
      price: { currency: 'CNY', amount: '0.28', unitLabel: '每次', acceptedQuoteId: 'quote-copy', acceptedQuoteSha: 'sha256:quote-copy' },
    },
    {
      id: 'cap-image',
      version: '1',
      title: '封面提示词打样',
      summary: '输出图片提示词与构图说明。',
      tokenBudget: 24000,
      averageSeconds: 32,
      canonicalModelId: 'gpt-image-2',
      canonicalModelVersion: '2026-08-19',
      capabilityDigest: 'sha256:cap-image',
      price: { currency: 'CNY', amount: '0.66', unitLabel: '每次', acceptedQuoteId: 'quote-image', acceptedQuoteSha: 'sha256:quote-image' },
    },
  ],
  currentRun: null,
  savedReplays: [],
  warnings: [],
})

function createRunRecord(overrides: Partial<WorkbenchRunRecord> = {}): WorkbenchRunRecord {
  return {
    id: 'run-001',
    status: 'running',
    capabilityId: 'cap-copy',
    capabilityLabel: '品牌文案生成',
    intent: '请生成一段适合首页卡片展示的 CJK 文案，确保超长文本也能换行。',
    requestedAt: '2026-08-19T07:10:00Z',
    updatedAt: '2026-08-19T07:10:00Z',
    workspaceVersion: 4,
    cost: { currency: 'CNY', estimate: '0.28', actual: null, explanation: '按接受报价结算。' },
    artifact: null,
    cancellable: true,
    retryable: false,
    saveable: true,
    replayable: false,
    staleCapability: false,
    canonicalRequestId: 'request-001',
    canonicalUsageEventId: '',
    acceptedQuoteId: 'quote-copy',
    acceptedQuoteSha: 'sha256:quote-copy',
    journalId: '',
    mediaTaskId: '',
    terminalAt: '',
    cursor: '0',
    artifacts: [],
    forkable: true,
    replayOfRunId: '',
    forkedFromRunId: '',
    failureCode: '',
    failureMessage: '',
    lineage: {
      canonicalRequestId: 'request-001',
      canonicalUsageEventId: '',
      acceptedQuoteId: 'quote-copy',
      acceptedQuoteSha: 'sha256:quote-copy',
      journalId: '',
      mediaTaskId: '',
      canonicalMediaBusinessEventId: '',
      runnerJobId: '',
      adapterDigest: '',
      capabilityDigest: 'sha256:cap-copy',
      artifactId: '',
      artifactDigest: '',
      upstreamTaskId: '',
    },
    ...overrides,
  }
}

function mountView(service: CreatorWorkbenchService) {
  return mount(CreatorWorkbenchView, {
    props: { service },
    global: {
      stubs: {
        AppLayout: {
          template: '<div data-testid="app-layout"><slot /></div>',
        },
      },
    },
  })
}

describe('CreatorWorkbenchView', () => {
  beforeEach(() => {
    vi.useRealTimers()
  })

  afterEach(() => {
    window.history.replaceState({}, '', '/')
  })

  it('consumes a direct Zero City asset entry by preselecting the mapped Workbench capability', async () => {
    window.history.replaceState({}, '', '/operator?asset=23&entry=zero-city-asset')
    const workspace = baseWorkspace()
    const sourceCapability = workspace.capabilities[1]
    if (!sourceCapability) throw new Error('test workspace must include a source capability')
    sourceCapability.id = '23'
    sourceCapability.title = 'Zero City 来源能力'
    const createRun = vi.fn().mockResolvedValue(createRunRecord({ capabilityId: '23', capabilityLabel: sourceCapability.title }))
    const service: CreatorWorkbenchService = {
      loadWorkspace: vi.fn().mockResolvedValue(workspace),
      createRun,
      refreshRun: vi.fn(),
      cancelRun: vi.fn(),
      saveRun: vi.fn(),
      replayRun: vi.fn(),
      forkRun: vi.fn(),
      streamRun: vi.fn().mockResolvedValue({ kind: 'aborted', cursor: '' }),
    }

    const wrapper = mountView(service)
    await flushPromises()

    expect(wrapper.get('[data-testid="workbench-entry-source"]').text()).toContain('Zero City 资产 #23')
    await wrapper.get('[data-testid="workbench-launch"]').trigger('click')
    await flushPromises()
    expect(createRun).toHaveBeenCalledWith(expect.objectContaining({ capabilityId: '23' }))
  })

  it('rejects malformed and stale Zero City asset context instead of silently launching a different capability', async () => {
    window.history.replaceState({}, '', '/operator?asset=23&asset=24&entry=zero-city-asset')
    const service: CreatorWorkbenchService = {
      loadWorkspace: vi.fn().mockResolvedValue(baseWorkspace()),
      createRun: vi.fn(),
      refreshRun: vi.fn(),
      cancelRun: vi.fn(),
      saveRun: vi.fn(),
      replayRun: vi.fn(),
      forkRun: vi.fn(),
      streamRun: vi.fn().mockResolvedValue({ kind: 'aborted', cursor: '' }),
    }

    const malformed = mountView(service)
    await flushPromises()
    expect(malformed.text()).toContain('入口参数重复')
    expect(malformed.get('[data-testid="workbench-launch"]').attributes('disabled')).toBeDefined()
    malformed.unmount()

    window.history.replaceState({}, '', '/operator?asset=23&entry=zero-city-asset')
    const stale = mountView(service)
    await flushPromises()
    expect(stale.text()).toContain('当前未映射到可执行能力')
    expect(stale.get('[data-testid="workbench-launch"]').attributes('disabled')).toBeDefined()
  })

  it('covers loading, malformed stale data, empty saved replays, and accessible CJK-safe inputs', async () => {
    const staleRun = createRunRecord({
      id: 'run-stale',
      status: 'succeeded',
      capabilityId: 'cap-missing',
      capabilityLabel: '已下线能力',
      intent: '这是一个引用失效能力的旧运行。',
      staleCapability: true,
      saveable: false,
      cancellable: false,
      artifact: {
        id: 'old', artifactId: 'old', runId: 'run-stale', kind: 'text', title: '旧稿', preview: '旧稿内容',
        downloadLabel: '复制', contentType: 'text/plain', href: '', storageUri: '', byteSize: '1',
        verifiedAcceptedTask: false, createdAt: '2026-08-19T07:00:00Z', lineage: {
          canonicalRequestId: '', canonicalUsageEventId: '', acceptedQuoteId: '', acceptedQuoteSha: '', journalId: '',
          mediaTaskId: '', canonicalMediaBusinessEventId: '', runnerJobId: '', adapterDigest: '', capabilityDigest: '',
          artifactId: '', artifactDigest: '', upstreamTaskId: '',
        },
      },
    })
    const service: CreatorWorkbenchService = {
      loadWorkspace: vi.fn().mockResolvedValue({
        ...baseWorkspace(),
        currentRun: staleRun,
        warnings: ['工作区版本已更新'],
      }),
      createRun: vi.fn(),
      refreshRun: vi.fn(),
      cancelRun: vi.fn(),
      saveRun: vi.fn(),
      replayRun: vi.fn(),
      forkRun: vi.fn(),
      streamRun: vi.fn().mockResolvedValue({ kind: 'aborted', cursor: '' }),
    }

    const wrapper = mountView(service)
    expect(wrapper.text()).toContain('正在加载工作台')
    await flushPromises()

    expect(wrapper.get('[data-testid="workbench-intent"]').attributes('aria-label')).toContain('创作意图')
    expect(wrapper.get('[data-testid="workbench-capability-select"]').attributes('aria-label')).toContain('能力')
    expect(wrapper.get('[data-testid="workbench-copy"]').classes()).toContain('wb-break-anywhere')
    expect(wrapper.text()).toContain('工作区版本已更新')
    expect(wrapper.text()).toContain('需重新选择能力')
    expect(wrapper.text()).toContain('还没有已保存回放')
  })

  it('deduplicates launch/retry clicks and supports cancel, save, and replay', async () => {
    const createRun = vi
      .fn()
      .mockResolvedValueOnce(createRunRecord())
      .mockResolvedValueOnce(
        createRunRecord({
          id: 'run-002',
          status: 'succeeded',
          updatedAt: '2026-08-19T07:15:00Z',
          cancellable: false,
          retryable: true,
          saveable: true,
          replayable: true,
          cost: { currency: 'CNY', estimate: '0.28', actual: '0.26', explanation: '命中较短输出，最终低于预估。' },
          artifact: {
            id: 'artifact-002',
            artifactId: 'artifact-002',
            runId: 'run-002',
            kind: 'text',
            title: '首页首屏文案',
            preview: '你好，创作者。这里是一段适合首页卡片的简洁文案。',
            downloadLabel: '复制文本',
            contentType: 'text/plain',
            href: '/api/v1/biz/workbench/runs/run-002/artifact',
            storageUri: 's3://artifact-002',
            byteSize: '128',
            verifiedAcceptedTask: false,
            createdAt: '2026-08-19T07:15:00Z',
            lineage: {
              canonicalRequestId: 'request-002',
              canonicalUsageEventId: 'usage-002',
              acceptedQuoteId: 'quote-copy',
              acceptedQuoteSha: 'sha256:quote-copy',
              journalId: 'journal-002',
              mediaTaskId: '',
              canonicalMediaBusinessEventId: '',
              runnerJobId: '',
              adapterDigest: 'sha256:adapter-1',
              capabilityDigest: 'sha256:cap-copy',
              artifactId: 'artifact-002',
              artifactDigest: 'sha256:artifact-002',
              upstreamTaskId: '',
            },
          },
          artifacts: [],
        }),
      )
    const refreshRun = vi.fn().mockResolvedValue(
      createRunRecord({
        status: 'failed',
        updatedAt: '2026-08-19T07:12:00Z',
        cancellable: false,
        retryable: true,
        cost: { currency: 'CNY', estimate: '0.28', actual: '0.28', explanation: '网关重试后仍失败。' },
      }),
    )
    const cancelRun = vi.fn().mockResolvedValue(
      createRunRecord({
        status: 'cancelled',
        updatedAt: '2026-08-19T07:11:00Z',
        cancellable: false,
        retryable: true,
      }),
    )
    const saveRun = vi.fn().mockResolvedValue({
      id: 'save-001',
      label: '首页首屏文案',
      capabilityId: 'cap-copy',
      capabilityLabel: '品牌文案生成',
      intent: '请生成一段适合首页卡片展示的 CJK 文案，确保超长文本也能换行。',
      savedAt: '2026-08-19T07:16:00Z',
      staleCapability: false,
      sourceRunId: 'run-002',
    })
    const replayRun = vi.fn().mockResolvedValue(
      createRunRecord({
        id: 'run-003',
        status: 'running',
        updatedAt: '2026-08-19T07:17:00Z',
        forkable: true,
      }),
    )
    const forkRun = vi.fn().mockResolvedValue(
      createRunRecord({
        id: 'run-004',
        status: 'running',
        forkable: true,
      }),
    )
    const service: CreatorWorkbenchService = {
      loadWorkspace: vi.fn().mockResolvedValue(baseWorkspace()),
      createRun,
      refreshRun,
      cancelRun,
      saveRun,
      replayRun,
      forkRun,
      streamRun: vi.fn().mockResolvedValue({ kind: 'aborted', cursor: '' }),
    }

    const wrapper = mountView(service)
    await flushPromises()

    await wrapper.get('[data-testid="workbench-launch"]').trigger('click')
    await wrapper.get('[data-testid="workbench-launch"]').trigger('click')
    await flushPromises()
    expect(createRun).toHaveBeenCalledTimes(1)
    expect(wrapper.get('[data-testid="workbench-run-status"]').text()).toContain('运行中')
    await wrapper.get('[data-testid="workbench-save"]').trigger('click')
    await flushPromises()
    expect(saveRun).toHaveBeenCalledWith('run-001', expect.objectContaining({ label: '品牌文案生成' }))

    await wrapper.get('[data-testid="workbench-cancel"]').trigger('click')
    await flushPromises()
    expect(cancelRun).toHaveBeenCalledWith('run-001')
    expect(wrapper.get('[data-testid="workbench-run-status"]').text()).toContain('已取消')

    await wrapper.get('[data-testid="workbench-retry"]').trigger('click')
    await wrapper.get('[data-testid="workbench-retry"]').trigger('click')
    await flushPromises()
    expect(createRun.mock.calls.length).toBeGreaterThanOrEqual(2)

    await wrapper.get('[data-testid="workbench-replay-save-001"]').trigger('click')
    await flushPromises()
    expect(replayRun).toHaveBeenCalledWith('save-001')

    await wrapper.get('[data-testid="workbench-fork"]').trigger('click')
    await flushPromises()
    expect(forkRun).toHaveBeenCalled()
  })

  it('shows bootstrap error, retries successfully, and does not fall back to polling after unmount', async () => {
    vi.useFakeTimers()
    const loadWorkspace = vi
      .fn()
      .mockRejectedValueOnce(new Error('工作台入口暂时不可用'))
      .mockResolvedValueOnce(baseWorkspace())
    const refreshRun = vi.fn()
    const forkRun = vi.fn()

    const service: CreatorWorkbenchService = {
      loadWorkspace,
      createRun: vi.fn().mockResolvedValue(createRunRecord()),
      refreshRun,
      cancelRun: vi.fn(),
      saveRun: vi.fn(),
      replayRun: vi.fn(),
      forkRun,
      streamRun: vi.fn().mockResolvedValue({ kind: 'aborted', cursor: '' }),
    }

    const wrapper = mountView(service)
    await flushPromises()
    expect(wrapper.text()).toContain('工作台入口暂时不可用')

    await wrapper.get('[data-testid="workbench-reload"]').trigger('click')
    await flushPromises()
    expect(loadWorkspace).toHaveBeenCalledTimes(2)

    await wrapper.get('[data-testid="workbench-launch"]').trigger('click')
    await flushPromises()
    wrapper.unmount()

    await vi.advanceTimersByTimeAsync(2600)
    await nextTick()
    expect(refreshRun).not.toHaveBeenCalled()
  })
})
