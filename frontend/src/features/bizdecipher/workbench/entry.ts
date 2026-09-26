const ZERO_CITY_ASSET_ENTRY = 'zero-city-asset'

export type WorkbenchEntryContext =
  | { readonly kind: 'none' }
  | { readonly kind: 'source-asset'; readonly assetId: string }
  | { readonly kind: 'invalid'; readonly message: string }

export function parseWorkbenchEntryContext(search: string): WorkbenchEntryContext {
  const params = new URLSearchParams(search)
  const assets = params.getAll('asset')
  const entries = params.getAll('entry')
  if (assets.length === 0 && entries.length === 0) return { kind: 'none' }
  if (assets.length !== 1 || entries.length !== 1) {
    return { kind: 'invalid', message: 'Zero City 入口参数重复，无法确定来源资产。' }
  }
  const assetId = assets[0] ?? ''
  const entry = entries[0] ?? ''
  if (entry !== ZERO_CITY_ASSET_ENTRY || !/^[1-9]\d*$/.test(assetId)) {
    return { kind: 'invalid', message: 'Zero City 入口参数无效，未选择可执行能力。' }
  }
  return { kind: 'source-asset', assetId }
}
