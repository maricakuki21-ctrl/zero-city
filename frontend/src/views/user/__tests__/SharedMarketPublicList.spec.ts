import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'
import { getSharedPoolGuideSteps } from '@/features/bizdecipher/components/shared-pool/sharedPoolGuide'

const testDir = dirname(fileURLToPath(import.meta.url))
const marketSource = readFileSync(
  resolve(testDir, '../../../features/bizdecipher/views/user/AccountSquareView.vue'),
  'utf8',
)
const routerSource = readFileSync(resolve(testDir, '../../../router/index.ts'), 'utf8')

describe('shared market public list contracts', () => {
  it('keeps source failures separate from the combined empty state', () => {
    expect(marketSource).toContain('v-if="loadError && sourceFilter !== \'official\'"')
    expect(marketSource).toContain('v-if="showEmpty"')
    expect(marketSource).toContain('&& !loadError.value')
    expect(marketSource).toContain('&& !officialLoadError.value')
    expect(marketSource).toContain("sourceFilter.value === 'official' ||")
    expect(marketSource).toContain("sourceFilter.value === 'shared' ||")
  })

  it('describes merged results and view-local batching without claiming a full-market total', () => {
    expect(marketSource).toContain('v-for="pool in visiblePools"')
    expect(marketSource).toContain('v-for="resource in officialResourceCards"')
    expect(marketSource).toContain('class="as-resource-grid"')
    expect(marketSource).toContain('已显示 {{ visibleResourceCount }} 项')
    expect(marketSource).toContain('data-testid="shared-market-load-more"')
    expect(marketSource).toContain('visibleLimit.value = MARKET_BATCH_SIZE')
  })

  it('keeps member and owner routes in their dedicated views', () => {
    expect(routerSource).toContain("component: () => import('@/views/user/AccountSquareView.vue')")
    expect(routerSource).toContain("component: () => import('@/views/user/AccountSquareMyView.vue')")
    expect(routerSource).toContain("component: () => import('@/views/user/AccountSquareOwnerView.vue')")
    expect(marketSource).not.toContain("activeSpace === 'member'")
    expect(marketSource).not.toContain("activeSpace === 'owner'")
    expect(marketSource).not.toContain('usePoolOwner')
    expect(marketSource).not.toContain('SharedPoolOwnerWalletPanel')
    expect(marketSource).not.toContain('showCreateForm')
  })

  it('removes the guide step that targeted the deleted flow strip', () => {
    const steps = getSharedPoolGuideSteps('market', key => key)
    expect(steps).toHaveLength(6)
    expect(steps.map(step => step.element)).not.toContain('[data-tour="shared-pool-market-how"]')
  })

  it('keeps the confirmed discovery structure on the real market page', () => {
    expect(marketSource).toContain('发现共享资源')
    expect(marketSource).toContain('先找到合适的能力，再开始你的创作。')
    expect(marketSource).toContain('我要供给')
    expect(marketSource).toContain('as-source-segments')
    expect(marketSource).toContain('搜索资源、模型或池主')
    expect(marketSource).toContain('sourceFilteredPools')
    expect(marketSource).toContain('visibleOfficialGroups')
    expect(marketSource).toContain('官方分组')
    expect(marketSource).toContain("getAvailable()")
    expect(marketSource).toContain('ResourceDiscoveryCard')
  })

  it('keeps an explicit official supply state when the source is empty or unavailable', () => {
    expect(marketSource).toContain('sourceFilter !== \'shared\'')
    expect(marketSource).toContain('官方分组暂时无法刷新')
    expect(marketSource).toContain('登录后查看当前账号可用的官方分组')
    expect(marketSource).toContain('当前账号暂无可用官方分组')
    expect(marketSource).toContain('重新加载')
  })
})
