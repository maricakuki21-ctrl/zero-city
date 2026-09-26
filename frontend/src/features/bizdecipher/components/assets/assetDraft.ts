import type {
  CapabilityAsset,
  CapabilityAssetActionType,
  CapabilityAssetPayload,
  CapabilityAssetPricingType,
  CapabilityAssetType,
} from '@/features/bizdecipher/api/bizdecipher'

export type AssetEditorMode = 'create' | 'edit'

export type AssetDraftForm = {
  title: string
  summary: string
  description: string
  assetType: CapabilityAssetType | string
  status: 'draft' | 'pending'
  tags: string
  scenarioTags: string
  integrationTags: string
  coverURL: string
  screenshotURLs: string
  videoURL: string
  demoURL: string
  docURL: string
  sourceURL: string
  templateURL: string
  primaryActionType: CapabilityAssetActionType | string
  pricingType: CapabilityAssetPricingType | string
  contactEnabled: boolean
}

function joinValues(values: string[] | undefined): string {
  return (values ?? []).join(', ')
}

export function createAssetDraftForm(asset?: CapabilityAsset): AssetDraftForm {
  return {
    title: asset?.title ?? '',
    summary: asset?.summary ?? '',
    description: asset?.description ?? '',
    assetType: asset?.asset_type ?? 'product_app',
    status: asset ? 'draft' : 'pending',
    tags: joinValues(asset?.tags),
    scenarioTags: joinValues(asset?.scenario_tags),
    integrationTags: joinValues(asset?.integration_tags),
    coverURL: asset?.cover_url ?? '',
    screenshotURLs: joinValues(asset?.screenshot_urls),
    videoURL: asset?.video_url ?? '',
    demoURL: asset?.demo_url ?? '',
    docURL: asset?.doc_url ?? '',
    sourceURL: asset?.source_url ?? '',
    templateURL: asset?.template_url ?? '',
    primaryActionType: asset?.primary_action_type ?? 'view_detail',
    pricingType: asset?.pricing_type ?? 'free',
    contactEnabled: asset?.contact_enabled ?? false,
  }
}

function splitValues(value: string): string[] {
  return value.split(',').map(item => item.trim()).filter(Boolean)
}

export function assetDraftPayload(form: AssetDraftForm, mode: AssetEditorMode): CapabilityAssetPayload {
  return {
    title: form.title,
    summary: form.summary,
    description: form.description,
    asset_type: form.assetType,
    status: mode === 'edit' ? 'draft' : form.status,
    tags: splitValues(form.tags),
    scenario_tags: splitValues(form.scenarioTags),
    integration_tags: splitValues(form.integrationTags),
    cover_url: form.coverURL,
    screenshot_urls: splitValues(form.screenshotURLs),
    video_url: form.videoURL,
    demo_url: form.demoURL,
    doc_url: form.docURL,
    source_url: form.sourceURL,
    template_url: form.templateURL,
    primary_action_type: form.primaryActionType,
    pricing_type: form.pricingType,
    contact_enabled: form.contactEnabled,
  }
}
