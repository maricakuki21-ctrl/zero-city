export interface SidebarPathItem {
  path: string
  expandOnly?: boolean
  children?: SidebarPathItem[]
}

/** Category state keys are not page destinations. */
export function isSidebarGroupActive(item: SidebarPathItem, matches: (path: string) => boolean): boolean {
  if (!item.children?.length) return false
  return item.children.some(child => child.children?.length
    ? isSidebarGroupActive(child, matches)
    : matches(child.path)) || (item.expandOnly !== true && matches(item.path))
}

/**
 * Return one stable key for an in-app navigation target.
 *
 * Hashes do not change the routed page, duplicate slashes/trailing slashes are
 * collapsed, and query parameters are sorted so equivalent links can be
 * compared before they reach the template.
 */
export function normalizeSidebarPath(input: string): string {
  const raw = String(input || '').trim()
  if (!raw) return '/'

  const withoutHash = raw.split('#', 1)[0] || '/'
  const questionIndex = withoutHash.indexOf('?')
  const rawPath = questionIndex >= 0 ? withoutHash.slice(0, questionIndex) : withoutHash
  const rawQuery = questionIndex >= 0 ? withoutHash.slice(questionIndex + 1) : ''

  let pathname = rawPath.replace(/\/{2,}/g, '/')
  if (!pathname.startsWith('/')) pathname = `/${pathname}`
  if (pathname.length > 1) pathname = pathname.replace(/\/+$/, '')

  if (!rawQuery) return pathname

  const sortedQuery = [...new URLSearchParams(rawQuery).entries()]
    .sort(([leftKey, leftValue], [rightKey, rightValue]) =>
      leftKey.localeCompare(rightKey) || leftValue.localeCompare(rightValue)
    )
  const normalizedQuery = new URLSearchParams(sortedQuery).toString()
  return normalizedQuery ? `${pathname}?${normalizedQuery}` : pathname
}

/**
 * Preserve the first clickable occurrence of a route and drop later aliases.
 * Expand-only groups use their path as a UI state key, so they do not reserve a
 * route and may safely contain a child whose real link uses that path.
 */
export function dedupeSidebarItems<T extends SidebarPathItem>(items: readonly T[]): T[] {
  const seen = new Set<string>()

  const visit = (entries: readonly T[]): T[] => {
    const output: T[] = []

    for (const item of entries) {
      const isClickable = !item.children?.length || item.expandOnly !== true
      if (isClickable) {
        const normalizedPath = normalizeSidebarPath(item.path)
        if (seen.has(normalizedPath)) continue
        seen.add(normalizedPath)
      }

      const children = item.children?.length
        ? visit(item.children as unknown as readonly T[])
        : undefined
      output.push({
        ...item,
        ...(children ? { children } : item.children ? { children: [] } : {}),
      } as T)
    }

    return output
  }

  return visit(items)
}

/** Collect only real navigation targets; expand-only group state keys are skipped. */
export function collectClickableSidebarPaths(items: readonly SidebarPathItem[]): string[] {
  const paths: string[] = []
  for (const item of items) {
    if (!item.children?.length || item.expandOnly !== true) {
      paths.push(normalizeSidebarPath(item.path))
    }
    if (item.children?.length) paths.push(...collectClickableSidebarPaths(item.children))
  }
  return paths
}
