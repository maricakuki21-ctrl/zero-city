import { describe, expect, it } from 'vitest'
import { buildHarnessAssetDraft } from '../harnessAssetDraft'

describe('Harness asset draft payload', () => {
  it('builds a draft from the completed output without persisting execution credentials or Harness ids', () => {
    const payload = buildHarnessAssetDraft({
      title: '首页文案',
      summary: '可编辑简介',
      assetType: 'prompt_solution',
      output: '最终交付正文',
      imageUrls: ['https://cdn.example.com/result.png'],
    })

    expect(payload).toEqual({
      title: '首页文案',
      summary: '可编辑简介',
      description: '最终交付正文',
      asset_type: 'prompt_solution',
      status: 'draft',
      tags: [],
      scenario_tags: [],
      integration_tags: [],
      screenshot_urls: ['https://cdn.example.com/result.png'],
      primary_action_type: 'view_detail',
      pricing_type: 'free',
      contact_enabled: false,
    })
    expect(JSON.stringify(payload)).not.toMatch(/key|credential|harness|run-/i)
  })
})
