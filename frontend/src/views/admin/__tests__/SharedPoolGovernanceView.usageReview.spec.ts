import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const viewSource = readFileSync(
  resolve(dirname(fileURLToPath(import.meta.url)), '../../../features/bizdecipher/views/admin/SharedPoolGovernanceView.vue'),
  'utf8',
)

describe('SharedPoolGovernanceView usage review contract', () => {
  it('explains frozen funds in plain language and shows the required evidence fields', () => {
    expect(viewSource).toContain('<strong>冻结不是扣款。</strong>')
    expect(viewSource).toContain('旧复核记录不能作为人工扣费依据')
    expect(viewSource).toContain('实际扣费只认 canonical usage')
    expect(viewSource).toContain('当前冻结额（尚未扣款）')
    expect(viewSource).toContain('label="池"')
    expect(viewSource).toContain('label="用户"')
    expect(viewSource).toContain('label="模型"')
    expect(viewSource).toContain('label="触发原因"')
    expect(viewSource).toContain('label="冻结时间"')
  })

  it('only permits release while preserving historical action labels for audit', () => {
    expect(viewSource).toContain('本页只能释放冻结，不会创建扣费')
    expect(viewSource).not.toContain('<option value="capture_hold">')
    expect(viewSource).not.toContain('<option value="settle_amount">')
    expect(viewSource).not.toContain('v-model="usageReviewForms[review.reservation_id].amount"')
    expect(viewSource).not.toContain('batchReviewAction')
    expect(viewSource).toContain("if (action === 'capture_hold') return '按全部冻结额扣款'")
    expect(viewSource).toContain("if (action === 'settle_amount') return '按明确金额扣款'")
    expect(viewSource).toContain('处理备注（必填，会进入审计记录）')
    expect(viewSource).toContain('maxlength="500"')
    expect(viewSource).toContain('required')
  })

  it('requires a second confirmation and prevents duplicate clicks', () => {
    expect(viewSource).toContain('window.confirm(usageReviewConfirmMessage(review, form))')
    expect(viewSource).toContain(':disabled="resolvingReviewId !== null"')
    expect(viewSource).toContain('处理中，请勿重复点击…')
  })

  it('keeps the same operation id for an identical failed retry', () => {
    expect(viewSource).toContain('form.operationSignature !== signature')
    expect(viewSource).toContain("operation_id: form.operationId")
    expect(viewSource).toContain('同样内容再次提交时会沿用原操作编号，避免重复处理。')
  })

  it('makes routine failures automatic and supports one-click batch review', () => {
    expect(viewSource).toContain('明确的上游 HTTP 失败会立即释放')
    expect(viewSource).toContain('低额待定自动释放')
    expect(viewSource).toContain('reviewPolicy.auto_release_minutes')
    expect(viewSource).toContain('reviewPolicy.auto_release_max_hold')
    expect(viewSource).toContain('adminBatchResolveSharedPoolUsageReviews')
    expect(viewSource).toContain('选择本页待定')
    expect(viewSource).toContain("action: 'release'")
    expect(viewSource).toContain('每笔都会写入独立审计记录')
  })

  it('shows queue totals and grouped reasons instead of an unbounded flat inbox', () => {
    expect(viewSource).toContain('usageReviewSummary.pending_count')
    expect(viewSource).toContain('usageReviewSummary.pending_hold')
    expect(viewSource).toContain('usageReviewSummary.groups.slice(0, 6)')
  })

  it('renders missing review amounts as unknown instead of zero', () => {
    expect(viewSource).toContain("if (value == null || !Number.isFinite(Number(value))) return '—'")
    expect(viewSource).not.toContain('formatReviewAmount(usageReviewSummary.pending_hold || 0)')
  })
})
