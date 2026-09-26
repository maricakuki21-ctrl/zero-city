import { describe, expect, it } from 'vitest'
import type { CapabilityAsset } from '@/features/bizdecipher/api/bizdecipher'
import { assetDraftPayload, createAssetDraftForm } from '../assetDraft'

const asset: CapabilityAsset = {
  id: 73, user_id: 7, author: '资产主人', title: '原草稿', slug: 'draft-73', summary: '原摘要',
  description: '原正文', asset_type: 'workflow', status: 'draft', tags: ['AI', '自动化'],
  scenario_tags: ['企业服务'], integration_tags: ['Slack'], cover_url: 'https://example.test/cover.png',
  screenshot_urls: ['https://example.test/1.png', 'https://example.test/2.png'], video_url: 'https://example.test/video',
  demo_url: 'https://example.test/demo', doc_url: 'https://example.test/docs', source_url: 'https://example.test/source',
  template_url: 'https://example.test/template', primary_action_type: 'view_workflow', pricing_type: 'contact',
  contact_enabled: true, is_featured: false, featured_weight: 0, view_count: 0, like_count: 0, favorite_count: 0,
  download_count: 0, use_count: 0, liked_by_me: false, favorited_by_me: false, downloaded_by_me: false, viewed_today: false,
  comment_count: 0, rating_avg: 0, rating_count: 0, review_note: '', created_at: '2026-09-08T00:00:00Z',
  updated_at: '2026-09-09T00:00:00Z',
}

describe('asset draft editor mapping', () => {
  it('prefills every editable backend field and emits a full draft payload', () => {
    const form = createAssetDraftForm(asset)
    const payload = assetDraftPayload(form, 'edit')

    expect(form.screenshotURLs).toBe('https://example.test/1.png, https://example.test/2.png')
    expect(payload).toEqual({
      title: asset.title, summary: asset.summary, description: asset.description, asset_type: asset.asset_type,
      status: 'draft', tags: asset.tags, scenario_tags: asset.scenario_tags, integration_tags: asset.integration_tags,
      cover_url: asset.cover_url, screenshot_urls: asset.screenshot_urls, video_url: asset.video_url,
      demo_url: asset.demo_url, doc_url: asset.doc_url, source_url: asset.source_url, template_url: asset.template_url,
      primary_action_type: asset.primary_action_type, pricing_type: asset.pricing_type, contact_enabled: true,
    })
  })

  it('forces edit saves to remain drafts even if the form status is changed', () => {
    const form = createAssetDraftForm(asset)
    form.status = 'pending'
    expect(assetDraftPayload(form, 'edit').status).toBe('draft')
  })
})
