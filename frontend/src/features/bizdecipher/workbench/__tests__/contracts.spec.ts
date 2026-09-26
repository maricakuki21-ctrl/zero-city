import { describe, expect, it } from 'vitest'
import { normalizeWorkbenchWorkspace } from '../contracts'

describe('normalizeWorkbenchWorkspace', () => {
  it('drops malformed capability rows and marks stale run references', () => {
    const workspace = normalizeWorkbenchWorkspace({
      workspace_token: 'wb_tok_live',
      workspace_version: 7,
      draft: {
        intent: '给中文用户写一段不会溢出的工作台说明。',
        capability_id: 'cap-missing',
        updated_at: '2026-08-18T11:00:00Z',
      },
      capabilities: [
        {
          id: 'cap-copy', version: '1',
          title: '品牌文案生成',
          summary: '适合活动标题、卖点与站内卡片文案。',
          token_budget: 12000,
          average_seconds: 18,
          canonical_model_id: 'gpt-5.5',
          canonical_model_version: '2026-08-19',
          capability_digest: 'sha256:cap-copy',
          price: {
            currency: 'CNY', amount: '0.0001', unit_label: '每次',
            accepted_quote_id: 'quote-1', accepted_quote_sha: 'sha256:quote-1',
          },
        },
        {
          title: '缺少主键的坏数据',
        },
        null,
      ],
      current_run: {
        id: 'run-stale',
        status: 'succeeded',
        capability_id: 'cap-missing',
        intent: '旧运行引用了已经下线的能力。',
        artifact: {
          kind: 'text',
          title: '',
          preview: 42,
        },
      },
      saved_replays: [
        {
          id: 'save-1',
          label: '上周版本',
          capability_id: 'cap-missing',
          intent: '旧版回放',
        },
      ],
    })

    expect(workspace.capabilities).toHaveLength(1)
    expect(workspace.capabilities[0]?.id).toBe('cap-copy')
    expect(workspace.capabilities[0]?.price.amount).toBe('0.0001')
    expect(workspace.draft.capabilityId).toBe('cap-copy')
    expect(workspace.currentRun?.staleCapability).toBe(true)
    expect(workspace.currentRun?.artifact.preview).toBe('')
    expect(workspace.savedReplays[0]?.staleCapability).toBe(true)
    expect(workspace.savedReplays[0]?.capabilityLabel).toContain('已下线')
  })

  it('creates safe empty defaults when the payload is unusable', () => {
    const workspace = normalizeWorkbenchWorkspace({
      workspace_token: '',
      draft: 'not-an-object',
      capabilities: 'bad-data',
      saved_replays: [{ id: '', label: '', capability_id: '', intent: '' }],
    })

    expect(workspace.workspaceToken).toBe('')
    expect(workspace.capabilities).toEqual([])
    expect(workspace.draft.intent).toBe('')
    expect(workspace.savedReplays).toEqual([])
  })
})
