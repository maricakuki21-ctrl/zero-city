import { describe, expect, it } from 'vitest'

import {
  collectClickableSidebarPaths,
  dedupeSidebarItems,
  isSidebarGroupActive,
  normalizeSidebarPath,
  type SidebarPathItem,
} from '../sidebarNavigation'

describe('expand-only category selection', () => {
  const city = { path: '/community', expandOnly: true, children: [
    { path: '/community?district=governance', expandOnly: true, children: [{ path: '/community?district=governance&channel=votes' }] },
    { path: '/zero-city/cards' },
  ] }
  it('does not open Zero City for its detached city feed or personal page', () => {
    for (const path of ['/community', '/community?workspace=mine']) {
      expect(isSidebarGroupActive(city, target => target === path)).toBe(false)
    }
  })
  it('opens categories for actual nested destinations', () => {
    expect(isSidebarGroupActive(city, target => target === '/community?district=governance&channel=votes')).toBe(true)
    expect(isSidebarGroupActive(city, target => target === '/zero-city/cards')).toBe(true)
  })
  it('preserves clickable parent page selection', () => {
    expect(isSidebarGroupActive({ path: '/admin/orders', children: [{ path: '/admin/orders/refunds' }] }, target => target === '/admin/orders')).toBe(true)
  })
})

interface TestItem extends SidebarPathItem {
  label: string
  hideInSimpleMode?: boolean
  children?: TestItem[]
}

function expectUniqueClickablePaths(items: readonly TestItem[]) {
  const paths = collectClickableSidebarPaths(items)
  expect(new Set(paths).size).toBe(paths.length)
}

describe('normalizeSidebarPath', () => {
  it('treats hashes, duplicate slashes and trailing slashes as the same route', () => {
    expect(normalizeSidebarPath('/keys')).toBe('/keys')
    expect(normalizeSidebarPath('/keys/')).toBe('/keys')
    expect(normalizeSidebarPath('//keys//#top')).toBe('/keys')
  })

  it('sorts query parameters before comparing routes', () => {
    expect(normalizeSidebarPath('/community?district=x&channel=y')).toBe(
      normalizeSidebarPath('/community?channel=y&district=x'),
    )
  })
})

describe('dedupeSidebarItems', () => {
  it.each([
    {
      name: 'regular user',
      items: [
        {
          path: '/gateway',
          label: '中转站',
          expandOnly: true,
          children: [
            { path: '/account-square', label: '共享市场' },
            { path: '/keys', label: 'API 密钥' },
            { path: '/usage', label: '我的调用' },
          ],
        },
        { path: '/keys/', label: '旧版密钥入口' },
        { path: '/usage#recent', label: '旧版调用入口' },
      ] satisfies TestItem[],
    },
    {
      name: 'administrator',
      items: [
        {
          path: '/my-account',
          label: '我的账户',
          expandOnly: true,
          children: [
            { path: '/keys', label: 'API 密钥' },
            { path: '/usage', label: '我的调用' },
          ],
        },
        {
          path: '/admin',
          label: '管理后台',
          expandOnly: true,
          children: [
            { path: '/admin/users', label: '用户管理' },
            { path: '/admin/usage', label: '全站调用' },
          ],
        },
        { path: '/admin/usage/', label: '旧版全站调用入口' },
      ] satisfies TestItem[],
    },
    {
      name: 'simple-mode administrator',
      items: [
        { path: '/dashboard', label: '仪表盘' },
        {
          path: '/gateway',
          label: '中转站',
          expandOnly: true,
          children: [{ path: '/keys', label: 'API 密钥' }],
        },
        { path: '/admin/users', label: '用户管理' },
        { path: '/dashboard#top', label: '旧版仪表盘入口' },
      ] satisfies TestItem[],
    },
  ])('keeps every clickable path unique for $name navigation', ({ items }) => {
    const deduped = dedupeSidebarItems(items)
    expectUniqueClickablePaths(deduped)
  })

  it('does not let an expand-only parent reserve a real child route', () => {
    const items: TestItem[] = [
      {
        path: '/gateway',
        label: '中转站分组',
        expandOnly: true,
        children: [{ path: '/gateway', label: '中转站首页' }],
      },
    ]

    const deduped = dedupeSidebarItems(items)
    expect(collectClickableSidebarPaths(deduped)).toEqual(['/gateway'])
    expect(deduped[0]?.children?.[0]?.label).toBe('中转站首页')
  })

  it('keeps the built-in item and drops a later custom alias', () => {
    const items: TestItem[] = [
      { path: '/usage', label: '我的调用' },
      { path: '/usage/#custom', label: '自定义重复入口' },
    ]

    const deduped = dedupeSidebarItems(items)
    expect(deduped).toHaveLength(1)
    expect(deduped[0]?.label).toBe('我的调用')
  })
})
