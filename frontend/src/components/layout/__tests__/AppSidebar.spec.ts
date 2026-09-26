import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const componentPath = resolve(dirname(fileURLToPath(import.meta.url)), '../AppSidebar.vue')
const componentSource = readFileSync(componentPath, 'utf8')
const stylePath = resolve(dirname(fileURLToPath(import.meta.url)), '../../../style.css')
const styleSource = readFileSync(stylePath, 'utf8')

describe('AppSidebar custom SVG styles', () => {
  it('does not override uploaded SVG fill or stroke colors', () => {
    expect(componentSource).toContain('.sidebar-svg-icon {')
    expect(componentSource).toContain('color: currentColor;')
    expect(componentSource).toContain('display: block;')
    expect(componentSource).not.toContain('stroke: currentColor;')
    expect(componentSource).not.toContain('fill: none;')
  })
})

describe('AppSidebar scroll position persistence', () => {
  it('binds a template ref to the sidebar nav element', () => {
    expect(componentSource).toContain('ref="sidebarNavRef"')
    expect(componentSource).toContain('sidebar-nav')
  })

  it('declares sidebarNavRef in script setup', () => {
    expect(componentSource).toContain("const sidebarNavRef = ref<HTMLElement | null>(null)")
  })

  it('saves scroll position on beforeUnmount', () => {
    expect(componentSource).toContain('onBeforeUnmount')
    expect(componentSource).toContain('appStore.sidebarScrollTop')
    expect(componentSource).toContain('sidebarNavRef.value.scrollTop')
  })

  it('restores scroll position on mount', () => {
    expect(componentSource).toContain('onMounted')
    expect(componentSource).toContain('appStore.sidebarScrollTop')
    expect(componentSource).toContain('nextTick')
  })
})

describe('AppSidebar header styles', () => {
  it('does not render the version badge for regular users', () => {
    const sidebarHeaderBlockMatch = styleSource.match(/\.sidebar-header\s*\{[\s\S]*?\n {2}\}/)
    const sidebarBrandBlockMatch = componentSource.match(/\.sidebar-brand\s*\{[\s\S]*?\n\}/)

    expect(componentSource).not.toContain('<VersionBadge')
    expect(componentSource).not.toContain("components/common/VersionBadge.vue")
    expect(sidebarHeaderBlockMatch).not.toBeNull()
    expect(sidebarBrandBlockMatch).not.toBeNull()
  })
})

describe('AppSidebar navigation contracts', () => {
  it('removes the duplicate overview while retaining the city feed', () => {
    expect(componentSource).not.toContain("label: '概览'")
    expect(componentSource).toContain("{ path: '/community', label: '全城动态'")
    const router = readFileSync(resolve(__dirname, '../../../router/index.ts'), 'utf8')
    expect(router).toMatch(/path: '\/dashboard',\s*name: 'Dashboard',\s*redirect: '\/community'/)
  })
  it('puts the city feed above tools and resources in the arrival position', () => {
    const nav = componentSource.slice(componentSource.indexOf('function buildSelfNavItems'))
    expect(nav.indexOf("section: 'arrival'")).toBeLessThan(nav.indexOf("path: '/operator'"))
    expect(componentSource).toContain("arrival: '进城'")
  })
  it('keeps Shared Market as a first-class resource destination', () => {
    expect(componentSource).toContain('productModuleRegistry.sharedMarket.routePath ?? productModuleRegistry.sharedMarket.path')
    const marketBlock = componentSource.split('if (isRegistryVisible(productModuleRegistry.sharedMarket))')[1]?.split('if (isRegistryVisible(productModuleRegistry.gateway))')[0] || ''
    expect(marketBlock).not.toContain('children:')
    expect(marketBlock).not.toContain('expandOnly:')
    expect(componentSource).toContain("label: '我的密钥'")
    expect(componentSource).not.toContain("{ path: '/tavern/lounge', label: '实时大厅'")
    expect(componentSource.indexOf('productModuleRegistry.gateway.path')).toBeGreaterThan(
      componentSource.indexOf('productModuleRegistry.sharedMarket.routePath ?? productModuleRegistry.sharedMarket.path'),
    )
    expect(componentSource).toContain('aria-label="推荐入口"')
    expect(componentSource.match(/data-icon="heart"/g)?.length).toBeGreaterThanOrEqual(3)
    expect(componentSource).not.toContain('sidebar-open-pill')
    expect(componentSource).not.toContain('opened: true')
  })

  it('exposes the approved Zero City order and expand-only category parents', () => {
    const cityStart = componentSource.indexOf("{ path: '/community', label: '全城动态'")
    const plazaStart = componentSource.indexOf("label: '闲聊广场'")
    const tavernStart = componentSource.indexOf("path: '/tavern-group'")
    const workshopStart = componentSource.indexOf("label: '技术工坊'")
    const collaborationStart = componentSource.indexOf("label: '协作交流'")
    const activityStart = componentSource.indexOf("label: '城市活动'")
    const cardsStart = componentSource.indexOf("path: '/zero-city/cards'")

    expect([cityStart, plazaStart, tavernStart, workshopStart, collaborationStart, activityStart, cardsStart])
      .toEqual([...([cityStart, plazaStart, tavernStart, workshopStart, collaborationStart, activityStart, cardsStart])].sort((left, right) => left - right))
    expect(componentSource).toContain("{ path: '/tavern?view=rooms', label: '我的房间'")
    expect(componentSource).toContain("path: '/community?district=tavern&channel=chat-hall'")
    expect(componentSource).not.toContain('handleNestedParentNavigate')
    expect(componentSource.match(/@click="toggleGroup\(child\)"/g)?.length).toBeGreaterThanOrEqual(2)
    expect(componentSource).toContain("if (target.path === '/tavern')")
    expect(componentSource).toContain('return !queryValue(route.query.view)')
  })

  it('keeps Zero City expandable from every workspace route', () => {
    expect(componentSource).not.toContain('children: cityContext.value ? [')
    expect(componentSource).toContain("path: '/community',\n      label: t('nav.community')")
    expect(componentSource).toContain("path: '/zero-city/cards', label: t('nav.cards')")
  })

  it('replaces the standalone profile entry with one top-level My Zero City entry', () => {
    const zeroCityStart = componentSource.indexOf('if (isRegistryVisible(productModuleRegistry.zeroCity))')
    const capabilityAssetsStart = componentSource.indexOf('if (isRegistryVisible(productModuleRegistry.capabilityAssets))')
    const zeroCityBlock = componentSource.slice(zeroCityStart, capabilityAssetsStart)

    expect(zeroCityBlock).not.toContain("path: '/community?workspace=mine'")
    expect(componentSource).toContain("items.push({ path: '/community?workspace=mine', label: t('nav.myCity')")
    expect(componentSource).not.toContain("items.push({ path: '/profile', label: t('nav.profile')")
  })

  it('uses separate labels for personal and platform usage', () => {
    expect(componentSource).toContain("t('nav.myUsage')")
    expect(componentSource).toContain("t('nav.platformUsage')")
  })

  it('keeps wallet, incentives and invites as peer destinations', () => {
    const walletStart = componentSource.indexOf("path: '/wallet'")
    const incentivesStart = componentSource.indexOf("path: '/incentives'")
    const affiliateStart = componentSource.indexOf("path: '/affiliate'")

    expect(walletStart).toBeGreaterThanOrEqual(0)
    expect(incentivesStart).toBeGreaterThan(walletStart)
    expect(affiliateStart).toBeGreaterThan(incentivesStart)
    expect(componentSource).toContain("label: '邀请伙伴'")
  })

  it('shows invitation discovery on both personal menus, including collapsed mode', () => {
    expect(componentSource).toContain('discoveryPending: affiliateDiscovery.pending.value')
    expect(componentSource.match(/v-if="item.discoveryPending" class="sidebar-discovery-dot"/g)).toHaveLength(2)
    expect(componentSource).toContain('.sidebar-link-collapsed .sidebar-discovery-dot')
    expect(componentSource).toContain('邀请奖励未查看')
    expect(componentSource).not.toContain('affiliateDiscovery.markSeen(')
  })

  it('separates the user map into stable top-level navigation sections', () => {
    const workbenchStart = componentSource.indexOf('if (isRegistryVisible(productModuleRegistry.capabilityAssets.children[1]))')
    const resourcesStart = componentSource.indexOf('if (isRegistryVisible(productModuleRegistry.sharedMarket))')
    const cityStart = componentSource.indexOf('if (isRegistryVisible(productModuleRegistry.zeroCity))')
    expect(workbenchStart).toBeGreaterThanOrEqual(0)
    expect(resourcesStart).toBeGreaterThan(workbenchStart)
    expect(cityStart).toBeLessThan(workbenchStart)
    expect(componentSource).toContain("type SidebarSectionKey = 'arrival' | 'core' | 'resources' | 'community' | 'accountHelp'")
    expect(componentSource).toContain("section: 'core'")
    expect(componentSource).toContain("section: 'community'")
    expect(componentSource).toContain("core: '工作'")
    expect(componentSource).toContain("resources: '资源'")
    expect(componentSource).toContain("community: '创作与社区'")
    expect(componentSource).not.toContain("'工作与资源'")
    expect(componentSource).not.toContain("'共享资源'")
    expect(componentSource).toContain("section: 'accountHelp'")
    expect(componentSource).toContain('sectionLabelForItem(item, index)')
  })

  it('closes the mobile drawer with Escape and restores the invoking focus', () => {
    expect(componentSource).toContain('@keydown.esc="handleSidebarEscape"')
    expect(componentSource).toContain("window.addEventListener('keydown', handleGlobalSidebarKeydown)")
    expect(componentSource).toContain('focusMobileSidebar()')
    expect(componentSource).toContain('restorePreviousFocus()')
  })

  it('exposes expanded state and traps focus while the mobile drawer is modal', () => {
    expect(componentSource).toContain(':aria-expanded="isGroupExpanded(item)"')
    expect(componentSource).toContain(':aria-expanded="isGroupExpanded(child)"')
    expect(componentSource).toContain(':role="mobileOpen ? \'dialog\' : undefined"')
    expect(componentSource).toContain(':aria-modal="mobileOpen ? \'true\' : undefined"')
    expect(componentSource).toContain('@keydown.tab="handleSidebarTabKeydown"')
    expect(componentSource).toContain('setBackgroundInert(open)')
  })

  it('deduplicates user, personal-admin and administrator menus', () => {
    expect(componentSource).toContain('return dedupeSidebarItems(modeVisible)')
    expect(componentSource.match(/return dedupeSidebarItems\(/g)?.length).toBeGreaterThanOrEqual(3)
  })
})
