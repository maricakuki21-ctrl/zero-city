import type { CapabilityAssetPayload, CapabilityAssetType } from '@/features/bizdecipher/api/bizdecipher'

export type HarnessAssetDraftInput = {
  readonly title: string
  readonly summary: string
  readonly assetType: CapabilityAssetType
  readonly output: string
  readonly imageUrls: readonly string[]
}

export function buildHarnessAssetDraft(input: HarnessAssetDraftInput): CapabilityAssetPayload {
  return {
    title: input.title.trim(),
    summary: input.summary.trim(),
    description: input.output.trim(),
    asset_type: input.assetType,
    status: 'draft',
    tags: [],
    scenario_tags: [],
    integration_tags: [],
    screenshot_urls: [...input.imageUrls],
    primary_action_type: 'view_detail',
    pricing_type: 'free',
    contact_enabled: false,
  }
}

export function summarizeHarnessOutput(output: string): string {
  return output.replace(/\s+/g, ' ').trim().slice(0, 160)
}
