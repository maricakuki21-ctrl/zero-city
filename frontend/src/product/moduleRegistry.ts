export type ProductModuleStatus = 'stable' | 'beta' | 'preview' | 'hidden' | 'disabled'
export type ProductModuleDomain = 'service' | 'commerce' | 'asset' | 'zeroCity' | 'account' | 'admin'

export interface ProductModuleDefinition {
  readonly key: string
  readonly titleKey: string
  readonly localeKey: string
  readonly status: ProductModuleStatus
  readonly domain: ProductModuleDomain
  readonly path: string
  readonly group?: boolean
  readonly routePath?: string
  readonly routeAliasOf?: string
  readonly requiresAuth: boolean
  readonly requiresAdmin?: boolean
  readonly showInSidebar: boolean
  readonly showInDashboard: boolean
  readonly children?: readonly ProductModuleDefinition[]
}

export const productModuleRegistry = {
  gateway: { key: 'gateway', titleKey: 'keys.title', localeKey: 'keys.title', status: 'stable', domain: 'service', path: '/keys', requiresAuth: true, showInSidebar: true, showInDashboard: true },
  workbench: {
    key: 'workbench', titleKey: 'nav.operatorWorkbench', localeKey: 'nav.operatorWorkbench', status: 'beta', domain: 'service', path: '/operator', requiresAuth: true, showInSidebar: true, showInDashboard: true,
  },
  bilateralMarket: {
    key: 'bilateralMarket', titleKey: 'nav.marketplace', localeKey: 'nav.marketplace', status: 'beta', domain: 'commerce', path: '/marketplace', requiresAuth: true, showInSidebar: true, showInDashboard: true,
  },
  sharedMarket: {
    key: 'sharedMarket', group: true, routePath: '/account-square', titleKey: 'nav.accountSquare', localeKey: 'nav.accountSquare', status: 'beta', domain: 'service', path: '', requiresAuth: true, showInSidebar: true, showInDashboard: true,
    children: [
      { key: 'sharedMarket.browse', group: true, titleKey: 'nav.sharedMarketBrowse', localeKey: 'nav.sharedMarketBrowse', status: 'beta', domain: 'service', path: '', requiresAuth: true, showInSidebar: true, showInDashboard: false },
      { key: 'sharedMarket.member', titleKey: 'nav.sharedMarketMember', localeKey: 'nav.sharedMarketMember', status: 'beta', domain: 'service', path: '/account-square/my', requiresAuth: true, showInSidebar: true, showInDashboard: false },
      { key: 'sharedMarket.owner', titleKey: 'nav.sharedMarketOwner', localeKey: 'nav.sharedMarketOwner', status: 'beta', domain: 'service', path: '/account-square/owner', requiresAuth: true, showInSidebar: true, showInDashboard: false },
    ],
  },
  capabilityAssets: {
    key: 'capabilityAssets', titleKey: 'nav.capabilityAssets', localeKey: 'nav.capabilityAssets', status: 'beta', domain: 'asset', path: '/assets', requiresAuth: true, showInSidebar: true, showInDashboard: true,
    children: [
      { key: 'capabilityAssets.gallery', group: true, titleKey: 'nav.capabilityAssetGallery', localeKey: 'nav.capabilityAssetGallery', status: 'beta', domain: 'asset', path: '', requiresAuth: true, showInSidebar: true, showInDashboard: false },
      { key: 'capabilityAssets.workbench', titleKey: 'nav.operatorWorkbench', localeKey: 'nav.operatorWorkbench', status: 'beta', domain: 'asset', path: '/operator', routeAliasOf: 'workbench', requiresAuth: true, showInSidebar: true, showInDashboard: false },
      { key: 'capabilityAssets.marketplace', titleKey: 'nav.marketplace', localeKey: 'nav.marketplace', status: 'beta', domain: 'commerce', path: '/marketplace', routeAliasOf: 'bilateralMarket', requiresAuth: true, showInSidebar: true, showInDashboard: false },
      { key: 'capabilityAssets.incentives', titleKey: 'nav.incentives', localeKey: 'nav.incentives', status: 'beta', domain: 'asset', path: '/incentives', requiresAuth: true, showInSidebar: true, showInDashboard: false },
    ],
  },
  zeroCity: { key: 'zeroCity', titleKey: 'nav.community', localeKey: 'nav.community', status: 'beta', domain: 'zeroCity', path: '/community', requiresAuth: true, showInSidebar: true, showInDashboard: true },
} as const satisfies Readonly<Record<string, ProductModuleDefinition>>

export type ProductModuleKey = string

export const PRODUCT_ROUTE_PATHS = {
  gateway: productModuleRegistry.gateway.path,
  workbench: productModuleRegistry.workbench.path,
  bilateralMarket: productModuleRegistry.bilateralMarket.path,
  sharedMarket: productModuleRegistry.sharedMarket.routePath ?? productModuleRegistry.sharedMarket.path,
  capabilityAssets: productModuleRegistry.capabilityAssets.path,
  zeroCity: productModuleRegistry.zeroCity.path,
  operatorWorkbench: productModuleRegistry.workbench.path,
  marketplace: productModuleRegistry.bilateralMarket.path,
  incentives: productModuleRegistry.capabilityAssets.children[3].path,
} as const
export type ProductModuleState = 'idle' | 'loading' | 'ready' | 'error'

export interface ProductModuleRuntimeState {
  readonly key: ProductModuleKey
  readonly state: ProductModuleState
  readonly stale: boolean
}

export interface ProductModuleRouteMeta {
  readonly productModuleKey: string
  readonly requiresAuth: boolean
  readonly requiresAdmin: boolean
  readonly titleKey: string
  readonly localeKey: string
}

export function createProductModuleState(key: ProductModuleKey, state: ProductModuleState = 'idle', stale = false): ProductModuleRuntimeState {
  return { key, state, stale }
}

export function productModuleRouteMeta(module: ProductModuleDefinition): ProductModuleRouteMeta {
  return {
    productModuleKey: module.key,
    requiresAuth: module.requiresAuth,
    requiresAdmin: module.requiresAdmin ?? false,
    titleKey: module.titleKey,
    localeKey: module.localeKey,
  }
}

export function productModulePath(module: ProductModuleDefinition): string {
  return module.routePath ?? module.path
}

export function isProductModuleState(value: unknown): value is ProductModuleState {
  return value === 'idle' || value === 'loading' || value === 'ready' || value === 'error'
}

export function validateProductModuleRegistry(registry: unknown = productModuleRegistry): boolean {
  if (!registry || typeof registry !== 'object') return false
  const seen = new Map<string, string>()
  const keys = new Set<string>()
  const visit = (value: unknown): boolean => {
    if (!isValidProductModuleDefinition(value)) return false
    const module = value
    if (keys.has(module.key)) return false
    keys.add(module.key)
    if (!module.group || module.routePath) {
      const normalized = normalizeProductPath(module.routePath ?? module.path)
      const existingKey = seen.get(normalized)
      if (existingKey && module.routeAliasOf !== existingKey) return false
      seen.set(normalized, existingKey ?? module.key)
    }
    return !module.children || module.children.every(visit)
  }
  const valid = Object.values(registry as Record<string, unknown>).every(visit)
  if (!valid || seen.size === 0) return false
  const allModules = Object.values(registry as Record<string, unknown>).flatMap((item) => collectProductModules(item))
  const moduleKeys = new Set(allModules.map((module) => module.key))
  return allModules.every((module) => !module.routeAliasOf || moduleKeys.has(module.routeAliasOf))
}

const validStatuses = new Set<ProductModuleStatus>(['stable', 'beta', 'preview', 'hidden', 'disabled'])
const validDomains = new Set<ProductModuleDomain>(['service', 'commerce', 'asset', 'zeroCity', 'account', 'admin'])

export function isValidProductModuleDefinition(value: unknown): value is ProductModuleDefinition {
  if (!value || typeof value !== 'object') return false
  const item = value as Record<string, unknown>
  if (typeof item.key !== 'string' || !item.key.trim() || typeof item.titleKey !== 'string' || !item.titleKey.trim() || typeof item.localeKey !== 'string' || !item.localeKey.trim()) return false
  if (!validStatuses.has(item.status as ProductModuleStatus) || !validDomains.has(item.domain as ProductModuleDomain)) return false
  if (typeof item.path !== 'string' || (!item.group && !item.path.startsWith('/')) || (item.group && item.path !== '')) return false
  if (item.routePath !== undefined && (typeof item.routePath !== 'string' || !item.routePath.startsWith('/'))) return false
  if (item.routeAliasOf !== undefined && (typeof item.routeAliasOf !== 'string' || !item.routeAliasOf.trim())) return false
  if (typeof item.requiresAuth !== 'boolean' || typeof item.showInSidebar !== 'boolean' || typeof item.showInDashboard !== 'boolean') return false
  if (item.requiresAdmin !== undefined && typeof item.requiresAdmin !== 'boolean') return false
  if (item.children !== undefined && (!Array.isArray(item.children) || item.children.some((child) => !isValidProductModuleDefinition(child)))) return false
  return true
}

function collectProductModules(value: unknown): ProductModuleDefinition[] {
  if (!isValidProductModuleDefinition(value)) return []
  return [value, ...(value.children?.flatMap((child) => collectProductModules(child)) ?? [])]
}

export function flattenProductModules(registry: unknown = productModuleRegistry): ProductModuleDefinition[] {
  if (!validateProductModuleRegistry(registry)) return []
  const result: ProductModuleDefinition[] = []
  const visit = (value: unknown): void => {
    if (!isValidProductModuleDefinition(value)) return
    result.push(value)
    value.children?.forEach(visit)
  }
  Object.values(registry as Record<string, unknown>).forEach(visit)
  return result
}

export function normalizeProductPath(path: string): string {
  return path.split(/[?#]/, 1)[0].replace(/\/{2,}/g, '/').replace(/\/$/, '') || '/'
}

export function resolveProductModule(path: string, registry: unknown = productModuleRegistry): ProductModuleDefinition | undefined {
  const normalized = typeof path === 'string' ? normalizeProductPath(path) : ''
  const modules = flattenProductModules(registry)
  const exact = modules.find((module) => {
    const modulePath = productModulePath(module)
    return modulePath !== '' && normalizeProductPath(modulePath) === normalized
  })
  if (exact) return exact

  return modules
    .filter((module) => {
      const modulePath = normalizeProductPath(productModulePath(module))
      return modulePath !== '/' && normalized.startsWith(`${modulePath}/`)
    })
    .sort((left, right) => normalizeProductPath(productModulePath(right)).length - normalizeProductPath(productModulePath(left)).length)[0]

}

export interface ProductModuleAccessContext {
  readonly isAuthenticated: boolean
  readonly isAdmin: boolean
  readonly allowPreview?: boolean
}

export function canAccessProductModule(module: unknown, context: ProductModuleAccessContext): boolean {
  if (!isValidProductModuleDefinition(module)) return false
  if (module.status === 'hidden' || module.status === 'disabled') return false
  if (module.status === 'preview' && context.allowPreview !== true) return false
  if (module.requiresAuth && !context.isAuthenticated) return false
  if (module.requiresAdmin && !context.isAdmin) return false
  return true
}

export function isProductModuleEnabled(module: ProductModuleDefinition): boolean {
  return canAccessProductModule(module, { isAuthenticated: true, isAdmin: true })
}

export function isProductModuleVisibleInSidebar(module: unknown, allowPreview = false): module is ProductModuleDefinition {
  if (!isValidProductModuleDefinition(module) || !module.showInSidebar) return false
  return module.status === 'stable' || module.status === 'beta' || (allowPreview && module.status === 'preview')
}
