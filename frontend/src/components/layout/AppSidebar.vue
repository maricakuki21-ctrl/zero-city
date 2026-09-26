<template>
  <aside
    ref="sidebarRef"
    class="sidebar"
    :class="[
      sidebarCollapsed ? 'w-[72px]' : 'w-[218px]',
      { '-translate-x-full lg:translate-x-0': !mobileOpen, 'z-[120]': mobileOpen }
    ]"
    tabindex="-1"
    :role="mobileOpen ? 'dialog' : undefined"
    :aria-modal="mobileOpen ? 'true' : undefined"
    @keydown.esc="handleSidebarEscape"
    @keydown.tab="handleSidebarTabKeydown"
    @click.stop
  >
    <!-- Coming soon toast -->
    <div
      v-if="soonNotice"
      class="zero-soon-toast pointer-events-none fixed left-1/2 top-5 z-[80] -translate-x-1/2 rounded-full px-4 py-2 text-sm font-semibold"
    >
      {{ soonNotice }}
    </div>

    <!-- Logo/Brand -->
    <div class="sidebar-header" :class="{ 'sidebar-header-collapsed': sidebarCollapsed }">
      <!-- Custom Logo or canonical BizDecipher brand mark -->
      <router-link
        :to="homePath"
        class="sidebar-logo zero-sidebar-logo flex h-10 w-10 items-center justify-center overflow-hidden rounded-2xl"
        :class="{ 'sidebar-logo--brand': usesBrandMark }"
        @click="handleMenuItemClick(homePath)"
      >
        <img :src="siteLogo" :alt="usesBrandMark ? '零号' : '站点 Logo'" class="h-8 w-8 object-contain" />
      </router-link>
      <div class="sidebar-brand" :class="{ 'sidebar-brand-collapsed': sidebarCollapsed }" :aria-hidden="sidebarCollapsed ? 'true' : 'false'">
        <span class="sidebar-brand-kicker">BizDecipher</span>
        <span class="sidebar-brand-title text-lg font-black">
          {{ siteName }}
        </span>
      </div>
    </div>

    <div v-if="isAdmin" class="sidebar-space-switch" aria-label="切换工作空间">
      <router-link to="/community" :class="{ active: !adminConsoleMode }" title="产品空间" @click="handleMenuItemClick('/community')">产品</router-link>
      <router-link to="/admin/dashboard" :class="{ active: adminConsoleMode }" title="运营管理" @click="handleMenuItemClick('/admin/dashboard')">运营</router-link>
    </div>

    <!-- Navigation -->
    <nav ref="sidebarNavRef" class="sidebar-nav scrollbar-hide">
      <!-- Admin View: Admin menu first, then personal menu -->
      <template v-if="adminConsoleMode">
        <!-- Admin Section -->
        <div class="sidebar-section">
          <template v-for="item in adminNavItems" :key="item.path">
            <!-- Collapsible group (has children) -->
            <template v-if="item.children?.length">
              <button
                type="button"
                class="sidebar-link mb-1 w-full"
                :aria-expanded="isGroupExpanded(item)"
                :class="{
                  'sidebar-link-active': isGroupActive(item) && !isGroupExpanded(item),
                  'sidebar-link-collapsed': sidebarCollapsed
                }"
                :title="sidebarCollapsed ? item.label : undefined"
                @click="handleGroupClick(item)"
              >
                <component :is="item.icon" class="h-5 w-5 flex-shrink-0" />
                <span
                  class="sidebar-label sidebar-label-flex"
                  :class="{ 'sidebar-label-collapsed': sidebarCollapsed }"
                  :aria-hidden="sidebarCollapsed ? 'true' : 'false'"
                >
                  <span class="min-w-0 truncate">{{ item.label }}</span>
                  <ChevronDownIcon
                    class="h-4 w-4 flex-shrink-0 transition-transform duration-200"
                    :class="isGroupExpanded(item) ? 'rotate-180' : ''"
                  />
                </span>
              </button>
              <!-- Children -->
              <div v-if="!sidebarCollapsed && isGroupExpanded(item)" class="mb-1 ml-4 border-l border-black/[0.10] pl-2">
                <router-link
                  v-for="child in item.children"
                  :key="child.path"
                  :to="child.path"
                  class="sidebar-link mb-0.5 py-1.5 text-sm"
                  :class="{ 'sidebar-link-active': isChildActive(child), 'sidebar-link-featured': child.featured }"
                  @click="handleMenuItemClick(child.path)"
                >
                  <component :is="child.icon" class="h-4 w-4 flex-shrink-0" />
                  <span class="sidebar-label sidebar-label-flex">
                    <span class="min-w-0 truncate">{{ child.label }}</span>
                    <span v-if="child.featured" class="sidebar-recommendation" aria-label="推荐入口">
                      <svg viewBox="0 0 24 24" fill="currentColor" aria-hidden="true" data-icon="heart">
                        <path d="M12 21.35l-1.45-1.32C5.4 15.36 2 12.27 2 8.5 2 5.42 4.42 3 7.5 3c1.74 0 3.41.81 4.5 2.09A5.98 5.98 0 0 1 16.5 3C19.58 3 22 5.42 22 8.5c0 3.77-3.4 6.86-8.55 11.54L12 21.35z" />
                      </svg>
                      <span>推荐</span>
                    </span>
                  </span>
                </router-link>
              </div>
            </template>
            <!-- Normal item (no children) -->
            <router-link
              v-else
              :to="item.path"
              class="sidebar-link mb-1"
              :class="{ 'sidebar-link-active': isNavItemActive(item), 'sidebar-link-collapsed': sidebarCollapsed }"
              :title="sidebarCollapsed ? item.label : undefined"
              :id="
                item.path === '/admin/accounts'
                  ? 'sidebar-channel-manage'
                  : item.path === '/admin/groups'
                    ? 'sidebar-group-manage'
                    : item.path === '/admin/redeem'
                      ? 'sidebar-wallet'
                      : undefined
              "
              @click="handleMenuItemClick(item.path)"
            >
              <span v-if="item.iconSvg" class="h-5 w-5 flex-shrink-0 sidebar-svg-icon" v-html="sanitizeSvg(item.iconSvg)"></span>
              <component v-else :is="item.icon" class="h-5 w-5 flex-shrink-0" />
              <span class="sidebar-label sidebar-label-flex" :class="{ 'sidebar-label-collapsed': sidebarCollapsed }" :aria-hidden="sidebarCollapsed ? 'true' : 'false'">
                <span class="min-w-0 truncate">{{ item.label }}</span>
              </span>
            </router-link>
          </template>
        </div>

        <!-- Personal Section for Admin (hidden in simple mode) -->
        <div v-if="!authStore.isSimpleMode" class="sidebar-section">
          <div class="sidebar-section-title" :class="{ 'sidebar-section-title-collapsed': sidebarCollapsed }" :aria-hidden="sidebarCollapsed ? 'true' : 'false'">
            <span class="sidebar-section-title-text" :class="{ 'sidebar-section-title-text-collapsed': sidebarCollapsed }">
              {{ t('nav.myAccount') }}
            </span>
          </div>

          <template v-for="item in personalNavItems" :key="item.path">
            <template v-if="item.children?.length">
              <button
                type="button"
                class="sidebar-link mb-1 w-full"
                :aria-expanded="isGroupExpanded(item)"
                :class="{
                  'sidebar-link-active': isGroupActive(item) && !isGroupExpanded(item),
                  'sidebar-link-collapsed': sidebarCollapsed
                }"
                :title="sidebarCollapsed ? item.label : undefined"
                @click="handleGroupClick(item)"
              >
                <component :is="item.icon" class="h-5 w-5 flex-shrink-0" />
                <span class="sidebar-label sidebar-label-flex" :class="{ 'sidebar-label-collapsed': sidebarCollapsed }" :aria-hidden="sidebarCollapsed ? 'true' : 'false'">
                  <span class="min-w-0 truncate">{{ item.label }}</span>
                  <ChevronDownIcon class="h-4 w-4 flex-shrink-0 transition-transform duration-200" :class="isGroupExpanded(item) ? 'rotate-180' : ''" />
                </span>
              </button>
              <div v-if="!sidebarCollapsed && isGroupExpanded(item)" class="mb-1 ml-4 border-l border-black/[0.10] pl-2">
                <template v-for="child in item.children" :key="child.path">
                  <div v-if="child.children?.length" class="sidebar-nested-group">
                    <div class="sidebar-nested-parent">
                      <button
                        type="button"
                        class="sidebar-link min-w-0 flex-1 py-1.5 text-sm"
                        :class="{ 'sidebar-link-active': isGroupActive(child) }"
                        :aria-expanded="isGroupExpanded(child)"
                        @click="toggleGroup(child)"
                      >
                        <component :is="child.icon" class="h-4 w-4 flex-shrink-0" />
                        <span class="sidebar-label sidebar-label-flex">
                          <span class="min-w-0 truncate">{{ child.label }}</span>
                          <span v-if="child.featured" class="sidebar-recommendation" aria-label="推荐入口">
                            <svg viewBox="0 0 24 24" fill="currentColor" aria-hidden="true" data-icon="heart">
                              <path d="M12 21.35l-1.45-1.32C5.4 15.36 2 12.27 2 8.5 2 5.42 4.42 3 7.5 3c1.74 0 3.41.81 4.5 2.09A5.98 5.98 0 0 1 16.5 3C19.58 3 22 5.42 22 8.5c0 3.77-3.4 6.86-8.55 11.54L12 21.35z" />
                            </svg>
                            <span>推荐</span>
                          </span>
                        </span>
                      </button>
                      <button type="button" class="sidebar-nested-toggle" :aria-label="`${child.label} 子菜单`" :aria-expanded="isGroupExpanded(child)" @click.stop="toggleGroup(child)">
                        <ChevronDownIcon class="h-3.5 w-3.5 transition-transform duration-200" :class="isGroupExpanded(child) ? 'rotate-180' : ''" />
                      </button>
                    </div>
                    <div v-if="isGroupExpanded(child)" class="sidebar-third-level">
                      <template v-for="grandchild in child.children" :key="grandchild.path">
                        <button
                          v-if="grandchild.disabled"
                          type="button"
                          class="sidebar-link sidebar-link-soon mb-0.5 w-full py-1.5 text-xs"
                          @click="handleSoonClick(grandchild.label)"
                        >
                          <component :is="grandchild.icon" class="h-3.5 w-3.5 flex-shrink-0" />
                          <span class="sidebar-label sidebar-label-flex">
                            <span class="min-w-0 truncate">{{ grandchild.label }}</span>
                            <span class="sidebar-badge">{{ grandchild.badge || t('nav.soon') }}</span>
                          </span>
                        </button>
                        <router-link
                          v-else
                          :to="grandchild.path"
                          class="sidebar-link mb-0.5 py-1.5 text-xs"
                          :class="{ 'sidebar-link-active': isChildActive(grandchild) }"
                          @click="handleMenuItemClick(grandchild.path)"
                        >
                          <component :is="grandchild.icon" class="h-3.5 w-3.5 flex-shrink-0" />
                          <span class="sidebar-label sidebar-label-flex">
                            <span class="min-w-0 truncate">{{ grandchild.label }}</span>
                          </span>
                        </router-link>
                      </template>
                    </div>
                  </div>
                  <button
                    v-else-if="child.disabled"
                    type="button"
                    class="sidebar-link sidebar-link-soon mb-0.5 w-full py-1.5 text-sm"
                    @click="handleSoonClick(child.label)"
                  >
                    <component :is="child.icon" class="h-4 w-4 flex-shrink-0" />
                    <span class="sidebar-label sidebar-label-flex">
                      <span class="min-w-0 truncate">{{ child.label }}</span>
                      <span class="sidebar-badge">{{ child.badge || t('nav.soon') }}</span>
                    </span>
                  </button>
                  <router-link
                    v-else
                    :to="child.path"
                    class="sidebar-link mb-0.5 py-1.5 text-sm"
                    :class="{ 'sidebar-link-active': isChildActive(child), 'sidebar-link-featured': child.featured, 'sidebar-link-soon': Boolean(child.badge) }"
                    :data-tour="child.path === '/keys' ? 'sidebar-my-keys' : undefined"
                    @click="handleMenuItemClick(child.path)"
                  >
                    <component :is="child.icon" class="h-4 w-4 flex-shrink-0" />
                    <span class="sidebar-label sidebar-label-flex">
                      <span class="min-w-0 truncate">{{ child.label }}</span>
                      <span v-if="child.featured" class="sidebar-recommendation" aria-label="推荐入口">
                        <svg viewBox="0 0 24 24" fill="currentColor" aria-hidden="true" data-icon="heart">
                          <path d="M12 21.35l-1.45-1.32C5.4 15.36 2 12.27 2 8.5 2 5.42 4.42 3 7.5 3c1.74 0 3.41.81 4.5 2.09A5.98 5.98 0 0 1 16.5 3C19.58 3 22 5.42 22 8.5c0 3.77-3.4 6.86-8.55 11.54L12 21.35z" />
                        </svg>
                        <span>推荐</span>
                      </span>
                      <span v-else-if="child.badge" class="sidebar-badge">{{ child.badge }}</span>
                    </span>
                  </router-link>
                </template>
              </div>
            </template>
            <router-link
              v-else
              :to="item.path"
              class="sidebar-link mb-1"
              :class="{
                'sidebar-link-active': isNavItemActive(item),
                'sidebar-link-collapsed': sidebarCollapsed,
                'sidebar-link-soon': Boolean(item.badge)
              }"
              :title="item.discoveryPending ? `${item.label} · 邀请奖励未查看` : sidebarCollapsed ? item.label : undefined"
              :aria-label="item.discoveryPending ? `${item.label}，邀请奖励未查看` : sidebarCollapsed ? item.label : undefined"
              :data-tour="item.path === '/keys' ? 'sidebar-my-keys' : undefined"
              @click="handleMenuItemClick(item.path)"
            >
              <span v-if="item.iconSvg" class="h-5 w-5 flex-shrink-0 sidebar-svg-icon" v-html="sanitizeSvg(item.iconSvg)"></span>
              <component v-else :is="item.icon" class="h-5 w-5 flex-shrink-0" />
              <span class="sidebar-label sidebar-label-flex" :class="{ 'sidebar-label-collapsed': sidebarCollapsed }" :aria-hidden="sidebarCollapsed ? 'true' : 'false'">
                <span class="min-w-0 truncate">{{ item.label }}</span>
                <span v-if="item.badge" class="sidebar-badge">{{ item.badge }}</span>
                <span v-if="item.guidePending" class="sidebar-guide-hint" aria-label="新手指南未查看"><i aria-hidden="true"></i>指南</span>
              </span>
              <span v-if="item.discoveryPending" class="sidebar-discovery-dot" aria-hidden="true" />
            </router-link>
          </template>
        </div>
      </template>

      <!-- Regular User View -->
      <template v-else-if="isAdmin || !appStore.backendModeEnabled">
        <div class="sidebar-section">
          <template v-for="(item, index) in userNavItems" :key="item.path">
            <div
              v-if="sectionLabelForItem(item, index)"
              class="sidebar-section-title"
              :class="{ 'sidebar-section-title-collapsed': sidebarCollapsed }"
              :aria-hidden="sidebarCollapsed ? 'true' : 'false'"
            >
              <span class="sidebar-section-title-text" :class="{ 'sidebar-section-title-text-collapsed': sidebarCollapsed }">
                {{ sectionLabelForItem(item, index) }}
              </span>
            </div>
            <template v-if="item.children?.length">
              <button
                type="button"
                class="sidebar-link mb-1 w-full"
                :aria-expanded="isGroupExpanded(item)"
                :class="{
                  'sidebar-link-active': isGroupActive(item) && !isGroupExpanded(item),
                  'sidebar-link-collapsed': sidebarCollapsed
                }"
                :title="sidebarCollapsed ? item.label : undefined"
                @click="handleGroupClick(item)"
              >
                <component :is="item.icon" class="h-5 w-5 flex-shrink-0" />
                <span class="sidebar-label sidebar-label-flex" :class="{ 'sidebar-label-collapsed': sidebarCollapsed }" :aria-hidden="sidebarCollapsed ? 'true' : 'false'">
                  <span class="min-w-0 truncate">{{ item.label }}</span>
                  <ChevronDownIcon class="h-4 w-4 flex-shrink-0 transition-transform duration-200" :class="isGroupExpanded(item) ? 'rotate-180' : ''" />
                </span>
              </button>
              <div v-if="!sidebarCollapsed && isGroupExpanded(item)" class="mb-1 ml-4 border-l border-black/[0.10] pl-2">
                <template v-for="child in item.children" :key="child.path">
                  <div v-if="child.children?.length" class="sidebar-nested-group">
                    <div class="sidebar-nested-parent">
                      <button
                        type="button"
                        class="sidebar-link min-w-0 flex-1 py-1.5 text-sm"
                        :class="{ 'sidebar-link-active': isGroupActive(child) }"
                        :aria-expanded="isGroupExpanded(child)"
                        @click="toggleGroup(child)"
                      >
                        <component :is="child.icon" class="h-4 w-4 flex-shrink-0" />
                        <span class="sidebar-label sidebar-label-flex">
                          <span class="min-w-0 truncate">{{ child.label }}</span>
                          <span v-if="child.featured" class="sidebar-recommendation" aria-label="推荐入口">
                            <svg viewBox="0 0 24 24" fill="currentColor" aria-hidden="true" data-icon="heart">
                              <path d="M12 21.35l-1.45-1.32C5.4 15.36 2 12.27 2 8.5 2 5.42 4.42 3 7.5 3c1.74 0 3.41.81 4.5 2.09A5.98 5.98 0 0 1 16.5 3C19.58 3 22 5.42 22 8.5c0 3.77-3.4 6.86-8.55 11.54L12 21.35z" />
                            </svg>
                            <span>推荐</span>
                          </span>
                        </span>
                      </button>
                      <button type="button" class="sidebar-nested-toggle" :aria-label="`${child.label} 子菜单`" :aria-expanded="isGroupExpanded(child)" @click.stop="toggleGroup(child)">
                        <ChevronDownIcon class="h-3.5 w-3.5 transition-transform duration-200" :class="isGroupExpanded(child) ? 'rotate-180' : ''" />
                      </button>
                    </div>
                    <div v-if="isGroupExpanded(child)" class="sidebar-third-level">
                      <template v-for="grandchild in child.children" :key="grandchild.path">
                        <button
                          v-if="grandchild.disabled"
                          type="button"
                          class="sidebar-link sidebar-link-soon mb-0.5 w-full py-1.5 text-xs"
                          @click="handleSoonClick(grandchild.label)"
                        >
                          <component :is="grandchild.icon" class="h-3.5 w-3.5 flex-shrink-0" />
                          <span class="sidebar-label sidebar-label-flex">
                            <span class="min-w-0 truncate">{{ grandchild.label }}</span>
                            <span class="sidebar-badge">{{ grandchild.badge || t('nav.soon') }}</span>
                          </span>
                        </button>
                        <router-link
                          v-else
                          :to="grandchild.path"
                          class="sidebar-link mb-0.5 py-1.5 text-xs"
                          :class="{ 'sidebar-link-active': isChildActive(grandchild) }"
                          @click="handleMenuItemClick(grandchild.path)"
                        >
                          <component :is="grandchild.icon" class="h-3.5 w-3.5 flex-shrink-0" />
                          <span class="sidebar-label sidebar-label-flex">
                            <span class="min-w-0 truncate">{{ grandchild.label }}</span>
                          </span>
                        </router-link>
                      </template>
                    </div>
                  </div>
                  <router-link
                    v-else
                    :to="child.path"
                    class="sidebar-link mb-0.5 py-1.5 text-sm"
                    :class="{ 'sidebar-link-active': isChildActive(child), 'sidebar-link-featured': child.featured }"
                    :data-tour="child.path === '/keys' ? 'sidebar-my-keys' : undefined"
                    @click="handleMenuItemClick(child.path)"
                  >
                    <component :is="child.icon" class="h-4 w-4 flex-shrink-0" />
                    <span class="sidebar-label sidebar-label-flex">
                      <span class="min-w-0 truncate">{{ child.label }}</span>
                      <span v-if="child.featured" class="sidebar-recommendation" aria-label="推荐入口">
                        <svg viewBox="0 0 24 24" fill="currentColor" aria-hidden="true" data-icon="heart">
                          <path d="M12 21.35l-1.45-1.32C5.4 15.36 2 12.27 2 8.5 2 5.42 4.42 3 7.5 3c1.74 0 3.41.81 4.5 2.09A5.98 5.98 0 0 1 16.5 3C19.58 3 22 5.42 22 8.5c0 3.77-3.4 6.86-8.55 11.54L12 21.35z" />
                        </svg>
                        <span>推荐</span>
                      </span>
                    </span>
                  </router-link>
                </template>
              </div>
            </template>
            <button
              v-else-if="item.disabled"
              type="button"
              class="sidebar-link sidebar-link-soon mb-1 w-full"
              :class="{ 'sidebar-link-collapsed': sidebarCollapsed }"
              :title="sidebarCollapsed ? `${item.label} · ${t('nav.soon')}` : undefined"
              @click="handleSoonClick(item.label)"
            >
              <span v-if="item.iconSvg" class="h-5 w-5 flex-shrink-0 sidebar-svg-icon" v-html="sanitizeSvg(item.iconSvg)"></span>
              <component v-else :is="item.icon" class="h-5 w-5 flex-shrink-0" />
              <span class="sidebar-label sidebar-label-flex" :class="{ 'sidebar-label-collapsed': sidebarCollapsed }" :aria-hidden="sidebarCollapsed ? 'true' : 'false'">
                <span class="min-w-0 truncate">{{ item.label }}</span>
                <span class="sidebar-badge">{{ item.badge || t('nav.soon') }}</span>
              </span>
            </button>
            <router-link
              v-else
              :to="item.path"
              class="sidebar-link mb-1"
              :class="{
                'sidebar-link-active': isNavItemActive(item),
                'sidebar-link-collapsed': sidebarCollapsed,
                'sidebar-link-soon': Boolean(item.badge)
              }"
              :title="item.discoveryPending ? `${item.label} · 邀请奖励未查看` : sidebarCollapsed ? item.label : undefined"
              :aria-label="item.discoveryPending ? `${item.label}，邀请奖励未查看` : sidebarCollapsed ? item.label : undefined"
              :data-tour="item.path === '/keys' ? 'sidebar-my-keys' : undefined"
              @click="handleMenuItemClick(item.path)"
            >
              <span v-if="item.iconSvg" class="h-5 w-5 flex-shrink-0 sidebar-svg-icon" v-html="sanitizeSvg(item.iconSvg)"></span>
              <component v-else :is="item.icon" class="h-5 w-5 flex-shrink-0" />
              <span class="sidebar-label sidebar-label-flex" :class="{ 'sidebar-label-collapsed': sidebarCollapsed }" :aria-hidden="sidebarCollapsed ? 'true' : 'false'">
                <span class="min-w-0 truncate">{{ item.label }}</span>
                <span v-if="item.badge" class="sidebar-badge">{{ item.badge }}</span>
                <span v-if="item.guidePending" class="sidebar-guide-hint" aria-label="新手指南未查看"><i aria-hidden="true"></i>指南</span>
              </span>
              <span v-if="item.discoveryPending" class="sidebar-discovery-dot" aria-hidden="true" />
            </router-link>
          </template>
        </div>
      </template>
    </nav>

    <!-- Bottom Section -->
    <div class="mt-auto border-t border-black/[0.06] p-3">
      <details v-if="!adminConsoleMode && secondaryNavItems.length" class="sidebar-more" @toggle="handleMoreToggle">
        <summary class="sidebar-link" :class="{ 'sidebar-link-collapsed': sidebarCollapsed }" title="更多资源工具" aria-label="更多资源工具"><MoreHorizontal :size="18" /><span v-if="!sidebarCollapsed">更多资源工具</span></summary>
        <div class="sidebar-more-menu">
          <router-link v-for="item in secondaryNavItems" :key="item.path" :to="item.path" class="sidebar-link" @click="handleMenuItemClick(item.path)"><component :is="item.icon" :size="16" /><span>{{ item.label }}</span></router-link>
        </div>
      </details>
      <router-link v-if="!adminConsoleMode" to="/docs" class="sidebar-link" :class="{ 'sidebar-link-collapsed': sidebarCollapsed }" title="文档与帮助" @click="handleMenuItemClick('/docs')">
        <BookOpen :size="18" />
        <span v-if="!sidebarCollapsed">文档与帮助</span>
      </router-link>
      <!-- Mobile close button -->
      <button
        v-if="mobileOpen"
        type="button"
        class="sidebar-link mb-2 w-full lg:hidden"
        @click="closeMobile"
      >
        <ChevronDoubleLeftIcon class="h-5 w-5 flex-shrink-0" />
        <span class="sidebar-label">关闭菜单</span>
      </button>
      <!-- Collapse Button -->
      <button
        @click="toggleSidebar"
        class="sidebar-link w-full"
        :class="{ 'sidebar-link-collapsed': sidebarCollapsed }"
        :title="sidebarCollapsed ? t('nav.expand') : t('nav.collapse')"
      >
        <ChevronDoubleLeftIcon v-if="!sidebarCollapsed" class="h-5 w-5 flex-shrink-0" />
        <ChevronDoubleRightIcon v-else class="h-5 w-5 flex-shrink-0" />
        <span class="sidebar-label" :class="{ 'sidebar-label-collapsed': sidebarCollapsed }" :aria-hidden="sidebarCollapsed ? 'true' : 'false'">{{ t('nav.collapse') }}</span>
      </button>
    </div>
  </aside>

  <!-- Mobile Overlay -->
  <transition name="fade">
    <div
      v-if="mobileOpen"
      class="fixed inset-0 z-[110] bg-black/50 lg:hidden"
      @click="closeMobile"
    ></div>
  </transition>
</template>

<script setup lang="ts">
import { computed, h, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAdminSettingsStore, useAppStore, useAuthStore, useOnboardingStore } from '@/stores'
import { sanitizeSvg } from '@/utils/sanitize'
import { resolveBrandAsset } from '@/utils/brandResolver'
import { FeatureFlags, makeSidebarFlag } from '@/utils/featureFlags'
import { canAccessProductModule, createProductModuleState, isProductModuleVisibleInSidebar, productModuleRegistry } from '@/product/moduleRegistry'
import { useBatchImageAccess } from '@/composables/useBatchImageAccess'
import { isLocalPreviewAuth } from '@/utils/localPreview'
import { dedupeSidebarItems, isSidebarGroupActive, type SidebarPathItem } from './sidebarNavigation'
import { useMarketplaceGuide } from '@/features/bizdecipher/composables/useMarketplaceGuide'
import { useAffiliateDiscovery } from '@/features/bizdecipher/composables/useAffiliateDiscovery'
import { Activity, BookOpen, ChartColumn, FolderOpen, Gift, Handshake, KeyRound, MessagesSquare, MoreHorizontal, Network, Sparkles, UserRound, UsersRound, Wallet } from '@lucide/vue'

type SidebarSectionKey = 'arrival' | 'core' | 'resources' | 'community' | 'accountHelp'

interface NavItem extends SidebarPathItem {
  path: string
  label: string
  icon: unknown
  section?: SidebarSectionKey
  iconSvg?: string
  hideInSimpleMode?: boolean
  disabled?: boolean
  badge?: string
  guidePending?: boolean
  discoveryPending?: boolean
  featured?: boolean
  children?: NavItem[]
  /**
   * When true, the parent item only toggles the expand/collapse state and
   * does NOT navigate to its `path`. The `path` is purely a stable key.
   */
  expandOnly?: boolean
  /**
   * 可选的功能开关 getter。返回 false 时菜单项被隐藏；返回 undefined/true 时显示。
   * 宽容策略（undefined → 显示）避免 public settings 未加载完成时菜单闪烁消失。
   * Getter 里访问的 reactive 来源（store / composable）会被 computed 自动追踪，
   * 开关切换时菜单自动更新。
   */
  featureFlag?: () => boolean | undefined
}

// applyFeatureFlags 递归过滤掉 featureFlag() === false 的节点（含子节点）。
// 使用 `!== false` 宽容语义：undefined（设置未加载）或 true 都视为显示。
function applyFeatureFlags(items: NavItem[]): NavItem[] {
  const out: NavItem[] = []
  for (const item of items) {
    if (item.featureFlag && item.featureFlag() === false) continue
    if (item.children) {
      out.push({ ...item, children: applyFeatureFlags(item.children) })
    } else {
      out.push(item)
    }
  }
  return out
}

const { t } = useI18n()

const route = useRoute()
const router = useRouter()
const appStore = useAppStore()
const authStore = useAuthStore()
const marketplaceGuide = useMarketplaceGuide(computed(() => Number(authStore.user?.id ?? 0)))
const affiliateDiscovery = useAffiliateDiscovery(computed(() => Number(authStore.user?.id ?? 0)))
const onboardingStore = useOnboardingStore()
const adminSettingsStore = useAdminSettingsStore()
const { canUseBatchImage, refreshBatchImageAccess } = useBatchImageAccess()

const sidebarCollapsed = computed(() => appStore.sidebarCollapsed && !appStore.mobileOpen)
const mobileOpen = computed(() => appStore.mobileOpen)
const isAdmin = computed(() => authStore.isAdmin)
const adminConsoleMode = computed(() => isAdmin.value && route.path.startsWith('/admin'))
const sidebarNavRef = ref<HTMLElement | null>(null)
const sidebarRef = ref<HTMLElement | null>(null)

const homePath = computed(() => (adminConsoleMode.value ? '/admin/dashboard' : '/community'))
const previewMode = computed(() => import.meta.env.DEV && isLocalPreviewAuth())
const registryAccess = computed(() => ({
  isAuthenticated: authStore.isAuthenticated,
  isAdmin: authStore.isAdmin,
  allowPreview: previewMode.value,
}))
const registryModuleState = (module: unknown) => {
  if (!isProductModuleVisibleInSidebar(module, previewMode.value)) return null
  const state = canAccessProductModule(module, registryAccess.value) ? 'ready' : 'error'
  return createProductModuleState(module.key, state)
}
const isRegistryVisible = (module: unknown) => registryModuleState(module)?.state === 'ready'
const sidebarSectionLabels = computed<Record<SidebarSectionKey, string>>(() => ({
  arrival: '进城',
  core: '工作',
  resources: '资源',
  community: '创作与社区',
  accountHelp: '账户与成长',
}))

function sectionLabelForItem(item: NavItem, index: number): string | undefined {
  if (!item.section || userNavItems.value[index - 1]?.section === item.section) return undefined
  return sidebarSectionLabels.value[item.section]
}

// Track which parent nav groups are expanded
const expandedGroups = ref<Set<string>>(new Set())
const soonNotice = ref('')
const previouslyFocusedElement = ref<HTMLElement | null>(null)
let soonNoticeTimer: ReturnType<typeof setTimeout> | null = null

// Site settings from appStore (cached, no flicker). Normalize legacy OEM values during brand migration.
const siteName = computed(() => appStore.siteName === 'Aura API' || appStore.siteName === 'Sub2API' ? 'BizDecipher' : appStore.siteName)
const resolvedSiteLogo = computed(() => resolveBrandAsset(appStore.siteLogo, 'mark'))
const siteLogo = computed(() => resolvedSiteLogo.value.url)
const usesBrandMark = computed(() => resolvedSiteLogo.value.kind === 'platform')

// SVG Icon Components
const DashboardIcon = {
  render: () =>
    h(
      'svg',
      { fill: 'none', viewBox: '0 0 24 24', 'stroke-width': '1.5' },
      [
        h('rect', { x: '3', y: '3', width: '8', height: '8', rx: '2', fill: 'currentColor', opacity: '0.15', stroke: 'none' }),
        h('rect', { x: '13', y: '3', width: '8', height: '5', rx: '1.5', fill: 'currentColor', opacity: '0.1', stroke: 'none' }),
        h('rect', { x: '3', y: '13', width: '8', height: '5', rx: '1.5', fill: 'currentColor', opacity: '0.1', stroke: 'none' }),
        h('rect', { x: '13', y: '10', width: '8', height: '8', rx: '2', fill: 'currentColor', opacity: '0.15', stroke: 'none' }),
        h('path', { 'stroke-linecap': 'round', 'stroke-linejoin': 'round', d: 'M3 6a2 2 0 012-2h3a2 2 0 012 2v3a2 2 0 01-2 2H5a2 2 0 01-2-2V6zm10 0a2 2 0 012-2h3a2 2 0 012 2v2a2 2 0 01-2 2h-3a2 2 0 01-2-2V6zM3 16a2 2 0 012-2h3a2 2 0 012 2v2a2 2 0 01-2 2H5a2 2 0 01-2-2v-2zm10-3a2 2 0 012-2h3a2 2 0 012 2v5a2 2 0 01-2 2h-3a2 2 0 01-2-2v-5z' })
      ]
    )
}

const KeyIcon = {
  render: () =>
    h(
      'svg',
      { fill: 'none', viewBox: '0 0 24 24', 'stroke-width': '1.5' },
      [
        h('circle', { cx: '8.25', cy: '8.25', r: '4.5', fill: 'currentColor', opacity: '0.15', stroke: 'none' }),
        h('path', { 'stroke-linecap': 'round', 'stroke-linejoin': 'round', d: 'M15.75 5.25a3 3 0 013 3m3 0a6 6 0 01-7.029 5.912c-.563-.097-1.159.026-1.563.43L10.5 17.25H8.25v2.25H6v2.25H2.25v-2.818c0-.597.237-1.17.659-1.591l6.499-6.499c.404-.404.527-1 .43-1.563A6 6 0 1121.75 8.25z' })
      ]
    )
}

const BatchImageIcon = {
  render: () =>
    h(
      'svg',
      { fill: 'none', viewBox: '0 0 24 24', stroke: 'currentColor', 'stroke-width': '1.5' },
      [
        h('path', {
          'stroke-linecap': 'round',
          'stroke-linejoin': 'round',
          d: 'M6.827 6.175A2.31 2.31 0 015.186 7.23c-.38.054-.757.112-1.134.175C2.999 7.58 2.25 8.507 2.25 9.574V18a2.25 2.25 0 002.25 2.25h15A2.25 2.25 0 0021.75 18V9.574c0-1.067-.75-1.994-1.802-2.169a47.865 47.865 0 00-1.134-.175 2.31 2.31 0 01-1.64-1.055l-.822-1.316a2.25 2.25 0 00-1.906-1.059H9.554a2.25 2.25 0 00-1.906 1.059l-.821 1.316z'
        }),
        h('path', {
          'stroke-linecap': 'round',
          'stroke-linejoin': 'round',
          d: 'M16.5 12.75a4.5 4.5 0 11-9 0 4.5 4.5 0 019 0zM18.75 10.5h.008v.008h-.008V10.5z'
        })
      ]
    )
}

const ChartIcon = {
  render: () =>
    h(
      'svg',
      { fill: 'none', viewBox: '0 0 24 24', 'stroke-width': '1.5' },
      [
        h('rect', { x: '3', y: '13', width: '4', height: '7', rx: '1', fill: 'currentColor', opacity: '0.12', stroke: 'none' }),
        h('rect', { x: '9.5', y: '9', width: '4', height: '11', rx: '1', fill: 'currentColor', opacity: '0.15', stroke: 'none' }),
        h('rect', { x: '16', y: '4.5', width: '4', height: '15.5', rx: '1', fill: 'currentColor', opacity: '0.18', stroke: 'none' }),
        h('path', { 'stroke-linecap': 'round', 'stroke-linejoin': 'round', d: 'M3 13.125C3 12.504 3.504 12 4.125 12h2.25c.621 0 1.125.504 1.125 1.125v6.75C7.5 20.496 6.996 21 6.375 21h-2.25A1.125 1.125 0 013 19.875v-6.75zM9.75 8.625c0-.621.504-1.125 1.125-1.125h2.25c.621 0 1.125.504 1.125 1.125v11.25c0 .621-.504 1.125-1.125 1.125h-2.25a1.125 1.125 0 01-1.125-1.125V8.625zM16.5 4.125c0-.621.504-1.125 1.125-1.125h2.25C20.496 3 21 3.504 21 4.125v15.75c0 .621-.504 1.125-1.125 1.125h-2.25a1.125 1.125 0 01-1.125-1.125V4.125z' })
      ]
    )
}

const GiftIcon = {
  render: () =>
    h(
      'svg',
      { fill: 'none', viewBox: '0 0 24 24', stroke: 'currentColor', 'stroke-width': '1.5' },
      [
        h('path', {
          'stroke-linecap': 'round',
          'stroke-linejoin': 'round',
          d: 'M21 11.25v8.25a1.5 1.5 0 01-1.5 1.5H5.25a1.5 1.5 0 01-1.5-1.5v-8.25M12 4.875A2.625 2.625 0 109.375 7.5H12m0-2.625V7.5m0-2.625A2.625 2.625 0 1114.625 7.5H12m0 0V21m-8.625-9.75h18c.621 0 1.125-.504 1.125-1.125v-1.5c0-.621-.504-1.125-1.125-1.125h-18c-.621 0-1.125.504-1.125 1.125v1.5c0 .621.504 1.125 1.125 1.125z'
        })
      ]
    )
}

const UsersIcon = {
  render: () =>
    h(
      'svg',
      { fill: 'none', viewBox: '0 0 24 24', stroke: 'currentColor', 'stroke-width': '1.5' },
      [
        h('path', {
          'stroke-linecap': 'round',
          'stroke-linejoin': 'round',
          d: 'M15 19.128a9.38 9.38 0 002.625.372 9.337 9.337 0 004.121-.952 4.125 4.125 0 00-7.533-2.493M15 19.128v-.003c0-1.113-.285-2.16-.786-3.07M15 19.128v.106A12.318 12.318 0 018.624 21c-2.331 0-4.512-.645-6.374-1.766l-.001-.109a6.375 6.375 0 0111.964-3.07M12 6.375a3.375 3.375 0 11-6.75 0 3.375 3.375 0 016.75 0zm8.25 2.25a2.625 2.625 0 11-5.25 0 2.625 2.625 0 015.25 0z'
        })
      ]
    )
}

const FolderIcon = {
  render: () =>
    h(
      'svg',
      { fill: 'none', viewBox: '0 0 24 24', stroke: 'currentColor', 'stroke-width': '1.5' },
      [
        h('path', {
          'stroke-linecap': 'round',
          'stroke-linejoin': 'round',
          d: 'M2.25 12.75V12A2.25 2.25 0 014.5 9.75h15A2.25 2.25 0 0121.75 12v.75m-8.69-6.44l-2.12-2.12a1.5 1.5 0 00-1.061-.44H4.5A2.25 2.25 0 002.25 6v12a2.25 2.25 0 002.25 2.25h15A2.25 2.25 0 0021.75 18V9a2.25 2.25 0 00-2.25-2.25h-5.379a1.5 1.5 0 01-1.06-.44z'
        })
      ]
    )
}

const ChannelIcon = {
  render: () =>
    h(
      'svg',
      { fill: 'none', viewBox: '0 0 24 24', stroke: 'currentColor', 'stroke-width': '1.5' },
      [
        h('path', {
          'stroke-linecap': 'round',
          'stroke-linejoin': 'round',
          d: 'M6.429 9.75L2.25 12l4.179 2.25m0-4.5l5.571 3 5.571-3m-11.142 0L2.25 7.5 12 2.25l9.75 5.25-4.179 2.25m0 0l4.179 2.25L12 17.25 2.25 12m15.321-2.25l4.179 2.25L12 17.25l-9.75-5.25'
        })
      ]
    )
}

const CreditCardIcon = {
  render: () =>
    h(
      'svg',
      { fill: 'none', viewBox: '0 0 24 24', stroke: 'currentColor', 'stroke-width': '1.5' },
      [
        h('path', {
          'stroke-linecap': 'round',
          'stroke-linejoin': 'round',
          d: 'M2.25 8.25h19.5M2.25 9h19.5m-16.5 5.25h6m-6 2.25h3m-3.75 3h15a2.25 2.25 0 002.25-2.25V6.75A2.25 2.25 0 0019.5 4.5h-15a2.25 2.25 0 00-2.25 2.25v10.5A2.25 2.25 0 004.5 19.5z'
        })
      ]
    )
}

const GlobeIcon = {
  render: () =>
    h(
      'svg',
      { fill: 'none', viewBox: '0 0 24 24', stroke: 'currentColor', 'stroke-width': '1.5' },
      [
        h('path', {
          'stroke-linecap': 'round',
          'stroke-linejoin': 'round',
          d: 'M12 21a9.004 9.004 0 008.716-6.747M12 21a9.004 9.004 0 01-8.716-6.747M12 21c2.485 0 4.5-4.03 4.5-9S14.485 3 12 3m0 18c-2.485 0-4.5-4.03-4.5-9S9.515 3 12 3m0 0a8.997 8.997 0 017.843 4.582M12 3a8.997 8.997 0 00-7.843 4.582m15.686 0A11.953 11.953 0 0112 10.5c-2.998 0-5.74-1.1-7.843-2.918m15.686 0A8.959 8.959 0 0121 12c0 .778-.099 1.533-.284 2.253m0 0A17.919 17.919 0 0112 16.5c-3.162 0-6.133-.815-8.716-2.247m0 0A9.015 9.015 0 013 12c0-1.605.42-3.113 1.157-4.418'
        })
      ]
    )
}

const ServerIcon = {
  render: () =>
    h(
      'svg',
      { fill: 'none', viewBox: '0 0 24 24', stroke: 'currentColor', 'stroke-width': '1.5' },
      [
        h('path', {
          'stroke-linecap': 'round',
          'stroke-linejoin': 'round',
          d: 'M5.25 14.25h13.5m-13.5 0a3 3 0 01-3-3m3 3a3 3 0 100 6h13.5a3 3 0 100-6m-16.5-3a3 3 0 013-3h13.5a3 3 0 013 3m-19.5 0a4.5 4.5 0 01.9-2.7L5.737 5.1a3.375 3.375 0 012.7-1.35h7.126c1.062 0 2.062.5 2.7 1.35l2.587 3.45a4.5 4.5 0 01.9 2.7m0 0a3 3 0 01-3 3m0 3h.008v.008h-.008v-.008zm0-6h.008v.008h-.008v-.008zm-3 6h.008v.008h-.008v-.008zm0-6h.008v.008h-.008v-.008z'
        })
      ]
    )
}

const BellIcon = {
  render: () =>
    h(
      'svg',
      { fill: 'none', viewBox: '0 0 24 24', stroke: 'currentColor', 'stroke-width': '1.5' },
      [
        h('path', {
          'stroke-linecap': 'round',
          'stroke-linejoin': 'round',
          d: 'M14.857 17.082a23.848 23.848 0 005.454-1.31A8.967 8.967 0 0118 9.75V9a6 6 0 10-12 0v.75a8.967 8.967 0 01-2.312 6.022c1.733.64 3.56 1.085 5.455 1.31m5.714 0a24.255 24.255 0 01-5.714 0m5.714 0a3 3 0 11-5.714 0'
        })
      ]
    )
}

const TicketIcon = {
  render: () =>
    h(
      'svg',
      { fill: 'none', viewBox: '0 0 24 24', stroke: 'currentColor', 'stroke-width': '1.5' },
      [
        h('path', {
          'stroke-linecap': 'round',
          'stroke-linejoin': 'round',
          d: 'M16.5 6v.75m0 3v.75m0 3v.75m0 3V18m-9-5.25h5.25M7.5 15h3M3.375 5.25c-.621 0-1.125.504-1.125 1.125v3.026a2.999 2.999 0 010 5.198v3.026c0 .621.504 1.125 1.125 1.125h17.25c.621 0 1.125-.504 1.125-1.125v-3.026a2.999 2.999 0 010-5.198V6.375c0-.621-.504-1.125-1.125-1.125H3.375z'
        })
      ]
    )
}

const CogIcon = {
  render: () =>
    h(
      'svg',
      { fill: 'none', viewBox: '0 0 24 24', stroke: 'currentColor', 'stroke-width': '1.5' },
      [
        h('path', {
          'stroke-linecap': 'round',
          'stroke-linejoin': 'round',
          d: 'M9.594 3.94c.09-.542.56-.94 1.11-.94h2.593c.55 0 1.02.398 1.11.94l.213 1.281c.063.374.313.686.645.87.074.04.147.083.22.127.324.196.72.257 1.075.124l1.217-.456a1.125 1.125 0 011.37.49l1.296 2.247a1.125 1.125 0 01-.26 1.431l-1.003.827c-.293.24-.438.613-.431.992a6.759 6.759 0 010 .255c-.007.378.138.75.43.99l1.005.828c.424.35.534.954.26 1.43l-1.298 2.247a1.125 1.125 0 01-1.369.491l-1.217-.456c-.355-.133-.75-.072-1.076.124a6.57 6.57 0 01-.22.128c-.331.183-.581.495-.644.869l-.213 1.28c-.09.543-.56.941-1.11.941h-2.594c-.55 0-1.02-.398-1.11-.94l-.213-1.281c-.062-.374-.312-.686-.644-.87a6.52 6.52 0 01-.22-.127c-.325-.196-.72-.257-1.076-.124l-1.217.456a1.125 1.125 0 01-1.369-.49l-1.297-2.247a1.125 1.125 0 01.26-1.431l1.004-.827c.292-.24.437-.613.43-.992a6.932 6.932 0 010-.255c.007-.378-.138-.75-.43-.99l-1.004-.828a1.125 1.125 0 01-.26-1.43l1.297-2.247a1.125 1.125 0 011.37-.491l1.216.456c.356.133.751.072 1.076-.124.072-.044.146-.087.22-.128.332-.183.582-.495.644-.869l.214-1.281z'
        }),
        h('path', {
          'stroke-linecap': 'round',
          'stroke-linejoin': 'round',
          d: 'M15 12a3 3 0 11-6 0 3 3 0 016 0z'
        })
      ]
    )
}

const ChevronDoubleLeftIcon = {
  render: () =>
    h(
      'svg',
      { fill: 'none', viewBox: '0 0 24 24', stroke: 'currentColor', 'stroke-width': '1.5' },
      [
        h('path', {
          'stroke-linecap': 'round',
          'stroke-linejoin': 'round',
          d: 'm18.75 4.5-7.5 7.5 7.5 7.5m-6-15L5.25 12l7.5 7.5'
        })
      ]
    )
}

const OrderIcon = {
  render: () =>
    h(
      'svg',
      { fill: 'none', viewBox: '0 0 24 24', stroke: 'currentColor', 'stroke-width': '1.5' },
      [
        h('path', {
          'stroke-linecap': 'round',
          'stroke-linejoin': 'round',
          d: 'M9 12h3.75M9 15h3.75M9 18h3.75m3 .75H18a2.25 2.25 0 002.25-2.25V6.108c0-1.135-.845-2.098-1.976-2.192a48.424 48.424 0 00-1.123-.08m-5.801 0c-.065.21-.1.433-.1.664 0 .414.336.75.75.75h4.5a.75.75 0 00.75-.75 2.25 2.25 0 00-.1-.664m-5.8 0A2.251 2.251 0 0113.5 2.25H15a2.25 2.25 0 012.15 1.586m-5.8 0c-.376.023-.75.05-1.124.08C9.095 4.01 8.25 4.973 8.25 6.108V8.25m0 0H4.875c-.621 0-1.125.504-1.125 1.125v11.25c0 .621.504 1.125 1.125 1.125h9.75c.621 0 1.125-.504 1.125-1.125V9.375c0-.621-.504-1.125-1.125-1.125H8.25zM6.75 12h.008v.008H6.75V12zm0 3h.008v.008H6.75V15zm0 3h.008v.008H6.75V18z'
        })
      ]
    )
}

const ChevronDoubleRightIcon = {
  render: () =>
    h(
      'svg',
      { fill: 'none', viewBox: '0 0 24 24', stroke: 'currentColor', 'stroke-width': '1.5' },
      [
        h('path', {
          'stroke-linecap': 'round',
          'stroke-linejoin': 'round',
          d: 'm5.25 4.5 7.5 7.5-7.5 7.5m6-15 7.5 7.5-7.5 7.5'
        })
      ]
    )
}

const SignalIcon = {
  render: () =>
    h(
      'svg',
      { fill: 'none', viewBox: '0 0 24 24', 'stroke-width': '1.5' },
      [
        h('circle', { cx: '12', cy: '12', r: '2', fill: 'currentColor', opacity: '0.25', stroke: 'none' }),
        h('circle', { cx: '12', cy: '12', r: '4.5', fill: 'currentColor', opacity: '0.1', stroke: 'none' }),
        h('path', { 'stroke-linecap': 'round', 'stroke-linejoin': 'round', d: 'M9.348 14.651a3.75 3.75 0 010-5.303m5.304 0a3.75 3.75 0 010 5.303m-7.425 2.122a6.75 6.75 0 010-9.546m9.546 0a6.75 6.75 0 010 9.546M5.106 18.894c-3.808-3.807-3.808-9.98 0-13.788m13.788 0c3.808 3.807 3.808 9.98 0 13.788M12 12h.008v.008H12V12zm.375 0a.375.375 0 11-.75 0 .375.375 0 01.75 0z' })
      ]
    )
}

const SparkIcon = {
  render: () =>
    h(
      'svg',
      { fill: 'none', viewBox: '0 0 24 24', 'stroke-width': '1.5' },
      [
        h('circle', { cx: '12', cy: '12', r: '3.5', fill: 'currentColor', opacity: '0.15', stroke: 'none' }),
        h('path', { 'stroke-linecap': 'round', 'stroke-linejoin': 'round', d: 'M9.813 15.904L9 18.75l-.813-2.846a4.5 4.5 0 00-3.09-3.09L2.25 12l2.846-.813a4.5 4.5 0 003.09-3.09L9 5.25l.813 2.846a4.5 4.5 0 003.09 3.09L15.75 12l-2.846.813a4.5 4.5 0 00-3.09 3.09zM18.259 8.715L18 9.75l-.259-1.035a3.375 3.375 0 00-2.455-2.456L14.25 6l1.036-.259a3.375 3.375 0 002.455-2.456L18 2.25l.259 1.035a3.375 3.375 0 002.456 2.456L21.75 6l-1.035.259a3.375 3.375 0 00-2.456 2.456z' })
      ]
    )
}

const CommunityIcon = {
  render: () =>
    h(
      'svg',
      { fill: 'none', viewBox: '0 0 24 24', stroke: 'currentColor', 'stroke-width': '1.5' },
      [
        h('path', {
          'stroke-linecap': 'round',
          'stroke-linejoin': 'round',
          d: 'M7.5 8.25h9m-9 3h5.25M12 20.25c4.97 0 9-3.582 9-8s-4.03-8-9-8-9 3.582-9 8c0 1.67.576 3.22 1.56 4.5-.18.96-.56 1.88-1.13 2.69a.375.375 0 00.39.58 9.88 9.88 0 004.05-1.45A10.26 10.26 0 0012 20.25z'
        })
      ]
    )
}

const ShareMarketIcon = {
  render: () =>
    h(
      'svg',
      { fill: 'none', viewBox: '0 0 24 24', 'stroke-width': '1.5' },
      [
        h('path', {
          fill: 'currentColor',
          opacity: '0.16',
          stroke: 'none',
          d: 'M12 20.25s-7.5-4.6-8.96-9.32C2.1 7.9 3.96 5.25 6.86 5.25c1.62 0 3.07.83 4.14 2.13 1.07-1.3 2.52-2.13 4.14-2.13 2.9 0 4.76 2.65 3.82 5.68C19.5 15.65 12 20.25 12 20.25z'
        }),
        h('path', {
          'stroke-linecap': 'round',
          'stroke-linejoin': 'round',
          d: 'M12 20.25s-7.5-4.6-8.96-9.32C2.1 7.9 3.96 5.25 6.86 5.25c1.62 0 3.07.83 4.14 2.13 1.07-1.3 2.52-2.13 4.14-2.13 2.9 0 4.76 2.65 3.82 5.68C19.5 15.65 12 20.25 12 20.25z'
        }),
        h('path', {
          'stroke-linecap': 'round',
          'stroke-linejoin': 'round',
          opacity: '0.72',
          d: 'M8.25 12h7.5M10.25 9.75h3.5M10.25 14.25h3.5'
        })
      ]
    )
}

const OrbitIcon = {
  render: () =>
    h(
      'svg',
      { fill: 'none', viewBox: '0 0 24 24', 'stroke-width': '1.5' },
      [
        h('circle', { cx: '12', cy: '12', r: '2.5', fill: 'currentColor', opacity: '0.2', stroke: 'none' }),
        h('ellipse', { cx: '12', cy: '12', rx: '10', ry: '4', transform: 'rotate(-30 12 12)', stroke: 'currentColor', opacity: '0.4' }),
        h('ellipse', { cx: '12', cy: '12', rx: '10', ry: '4', transform: 'rotate(30 12 12)', stroke: 'currentColor', opacity: '0.4' }),
        h('circle', { cx: '19', cy: '7', r: '1.5', fill: 'currentColor', opacity: '0.35', stroke: 'none' }),
        h('circle', { cx: '5', cy: '17', r: '1.5', fill: 'currentColor', opacity: '0.35', stroke: 'none' })
      ]
    )
}

const ShieldIcon = {
  render: () =>
    h(
      'svg',
      { fill: 'none', viewBox: '0 0 24 24', stroke: 'currentColor', 'stroke-width': '1.5' },
      [
        h('path', {
          'stroke-linecap': 'round',
          'stroke-linejoin': 'round',
          d: 'M9 12.75L11.25 15 15 9.75m-3-7.036A11.959 11.959 0 013.598 6 11.99 11.99 0 003 9.749c0 5.592 3.824 10.29 9 11.623 5.176-1.332 9-6.03 9-11.622 0-1.31-.21-2.571-.598-3.751h-.152c-3.196 0-6.1-1.248-8.25-3.285z'
        })
      ]
    )
}

const PriceTagIcon = {
  render: () =>
    h(
      'svg',
      { fill: 'none', viewBox: '0 0 24 24', stroke: 'currentColor', 'stroke-width': '1.5' },
      [
        h('path', {
          'stroke-linecap': 'round',
          'stroke-linejoin': 'round',
          d: 'M9.568 3H5.25A2.25 2.25 0 003 5.25v4.318c0 .597.237 1.17.659 1.591l9.581 9.581c.699.699 1.78.872 2.607.33a18.095 18.095 0 005.223-5.223c.542-.827.369-1.908-.33-2.607L11.16 3.66A2.25 2.25 0 009.568 3z'
        }),
        h('path', {
          'stroke-linecap': 'round',
          'stroke-linejoin': 'round',
          d: 'M6 6h.008v.008H6V6z'
        })
      ]
    )
}

const ChevronDownIcon = {
  render: () =>
    h(
      'svg',
      { fill: 'none', viewBox: '0 0 24 24', stroke: 'currentColor', 'stroke-width': '1.5' },
      [
        h('path', {
          'stroke-linecap': 'round',
          'stroke-linejoin': 'round',
          d: 'm19.5 8.25-7.5 7.5-7.5-7.5'
        })
      ]
    )
}

// Public-settings flags go through the registry in utils/featureFlags.ts,
// which handles the opt-in vs opt-out fallback when settings haven't loaded
// yet. Admin-only flags (not in public settings) stay inline below.
const flagChannelMonitor = makeSidebarFlag(FeatureFlags.channelMonitor)
const flagAvailableChannels = makeSidebarFlag(FeatureFlags.availableChannels)
const flagAffiliate = makeSidebarFlag(FeatureFlags.affiliate)
const flagRiskControl = makeSidebarFlag(FeatureFlags.riskControl)
const flagOpsMonitoring = () => adminSettingsStore.opsMonitoringEnabled
const flagAdminPayment = () => adminSettingsStore.paymentEnabled
const flagBatchImageAccess = () => canUseBatchImage.value

function buildSelfNavItems(): NavItem[] {
  const items: NavItem[] = []
  if (isRegistryVisible(productModuleRegistry.zeroCity)) {
    items.push({ path: '/community', label: '全城动态', icon: CommunityIcon, section: 'arrival' })
  }

  if (isRegistryVisible(productModuleRegistry.capabilityAssets.children[1])) {
    items.push({
      path: '/operator',
      label: t('nav.operatorWorkbench'),
      icon: Sparkles,
      section: 'core',
    })
  }

  if (isRegistryVisible(productModuleRegistry.sharedMarket)) {
    items.push({
      path: (productModuleRegistry.sharedMarket.routePath ?? productModuleRegistry.sharedMarket.path) || '/account-square',
      label: t('nav.accountSquare'),
      icon: Network,
      section: 'resources',
    })
  }

  if (isRegistryVisible(productModuleRegistry.gateway)) {
    items.push(
      { path: productModuleRegistry.gateway.path, label: '我的密钥', icon: KeyRound, section: 'resources' },
      { path: '/usage', label: t('nav.myUsage'), icon: ChartColumn, section: 'resources', hideInSimpleMode: true },
    )
  }

  if (isRegistryVisible(productModuleRegistry.zeroCity)) {
    items.push({
      path: '/community',
      label: t('nav.community'),
      icon: MessagesSquare,
      section: 'community',
      expandOnly: true,
      children: [
        { path: '/community?district=tavern&channel=chat-hall', label: '闲聊广场', icon: CommunityIcon },
        {
          path: '/tavern-group',
          label: '酒馆',
          icon: SparkIcon,
          expandOnly: true,
          children: [
            { path: '/tavern', label: '剧本与游戏', icon: SparkIcon },
            { path: '/tavern?view=rooms', label: '我的房间', icon: UsersIcon },
          ],
        },
        {
          path: '/community?district=workshop',
          label: '技术工坊',
          icon: SparkIcon,
          expandOnly: true,
          children: [
            { path: '/community?district=workshop&channel=help-desk', label: '答疑互助', icon: ShieldIcon },
            { path: '/community?district=workshop&channel=tech-share', label: '技术分享', icon: FolderIcon },
            { path: '/community?district=workshop&channel=news-radar', label: '模型情报', icon: SignalIcon },
          ],
        },
        {
          path: '/community?district=market',
          label: '协作交流',
          icon: ShareMarketIcon,
          children: [
            { path: '/community?district=market&channel=capability-showcase', label: '能力展示', icon: FolderIcon },
            { path: '/community?district=market&channel=demand-posting', label: '需求发布', icon: UsersIcon },
            { path: '/community?district=market&channel=delivery-certification', label: '交付与认证', icon: ShieldIcon },
          ],
        },
        {
          path: '/community?district=governance',
          label: '城市活动',
          icon: ShieldIcon,
          children: [
            { path: '/community?district=governance&channel=votes', label: '投票大厅', icon: UsersIcon },
            { path: '/community?district=governance&channel=badges', label: '勋章墙', icon: GiftIcon },
            { path: '/community?district=governance&channel=rules', label: '规则公示', icon: ShieldIcon },
          ],
        },
        { path: '/zero-city/chronicle', label: '城市编年史', icon: BookOpen },
        { path: '/zero-city/cards', label: t('nav.cards'), icon: GiftIcon },
      ],
    })
  }

  if (isRegistryVisible(productModuleRegistry.capabilityAssets.children[2])) {
    items.push({
      path: '/marketplace',
      label: t('nav.marketplace'),
      icon: Handshake,
      section: 'community',
      guidePending: marketplaceGuide.pending.value,
    })
  }

  if (isRegistryVisible(productModuleRegistry.capabilityAssets)) {
    items.push({
      path: '/assets',
      label: t('nav.capabilityAssets'),
      icon: FolderOpen,
      section: 'community',
    })
  }

  if (isRegistryVisible(productModuleRegistry.capabilityAssets.children[3])) {
    items.push({ path: '/wallet', label: '钱包', icon: Wallet, section: 'accountHelp' })
    items.push({ path: '/incentives', label: t('nav.incentives'), icon: Gift, section: 'accountHelp' })
    items.push({ path: '/affiliate', label: '邀请伙伴', icon: UsersRound, section: 'accountHelp', featureFlag: flagAffiliate, discoveryPending: affiliateDiscovery.pending.value })
  }

  items.push({ path: '/community?workspace=mine', label: t('nav.myCity'), icon: UserRound, section: 'accountHelp' })

  items.push(
    ...customMenuItemsForUser.value.map((item): NavItem => ({ path: `/custom/${item.id}`, label: item.label, icon: null, iconSvg: item.icon_svg, section: 'accountHelp' })),
  )
  return items
}

// finalizeNav 合并三重过滤：featureFlag 过滤 + simple 模式过滤。
function finalizeNav(items: NavItem[]): NavItem[] {
  const visible = applyFeatureFlags(items)
  const modeVisible = authStore.isSimpleMode && !previewMode.value ? visible.filter(item => !item.hideInSimpleMode) : visible
  return dedupeSidebarItems(modeVisible)
}

// User navigation items (for regular users)
const userNavItems = computed((): NavItem[] => finalizeNav(buildSelfNavItems()))
const secondaryNavItems = computed((): NavItem[] => isRegistryVisible(productModuleRegistry.gateway) ? finalizeNav([
  { path: '/monitor', label: '完整监控', icon: Activity },
  { path: '/available-channels', label: t('nav.availableChannels'), icon: Network, hideInSimpleMode: true, featureFlag: flagAvailableChannels },
  { path: '/subscriptions', label: t('nav.subscriptions'), icon: Wallet, hideInSimpleMode: true },
  { path: '/batch-image', label: t('nav.batchImage'), icon: BatchImageIcon, hideInSimpleMode: true, featureFlag: flagBatchImageAccess },
]) : [])

// Personal navigation items (for admin's "My Account" section, without Dashboard).
// Admins access 可用渠道 from this section just like regular users — there is no
// separate admin entry, since the page is purely a user-facing view.
const personalNavItems = computed((): NavItem[] => finalizeNav(buildSelfNavItems()))

// Custom menu items filtered by visibility
const customMenuItemsForUser = computed(() => {
  const items = appStore.cachedPublicSettings?.custom_menu_items ?? []
  return items
    .filter((item) => item.visibility === 'user')
    .sort((a, b) => a.sort_order - b.sort_order)
})

const customMenuItemsForAdmin = computed(() => {
  return adminSettingsStore.customMenuItems
    .filter((item) => item.visibility === 'admin')
    .sort((a, b) => a.sort_order - b.sort_order)
})

// Admin navigation items
const adminNavItems = computed((): NavItem[] => {
  const baseItems: NavItem[] = [
    { path: '/admin/dashboard', label: t('nav.dashboard'), icon: DashboardIcon },
    { path: '/admin/ops', label: t('nav.ops'), icon: ChartIcon, featureFlag: flagOpsMonitoring },
    { path: '/admin/shared-pools', label: '共享市场治理', icon: OrbitIcon, hideInSimpleMode: true },
    { path: '/admin/capability-assets', label: '能力资产审核', icon: FolderIcon, hideInSimpleMode: true },
    { path: '/admin/tavern-scripts', label: '酒馆剧本审核', icon: FolderIcon, hideInSimpleMode: true },
    { path: '/admin/credits', label: t('nav.credits'), icon: CreditCardIcon, hideInSimpleMode: true },
    { path: '/admin/users', label: t('nav.users'), icon: UsersIcon, hideInSimpleMode: true },
    { path: '/admin/groups', label: t('nav.groups'), icon: FolderIcon, hideInSimpleMode: true },
    {
      path: '/admin/channels',
      label: t('nav.channelManagement'),
      icon: ChannelIcon,
      hideInSimpleMode: true,
      expandOnly: true,
      children: [
        { path: '/admin/channels/pricing', label: t('nav.channelPricing'), icon: PriceTagIcon },
        { path: '/admin/channels/monitor', label: t('nav.channelMonitor'), icon: SignalIcon, featureFlag: flagChannelMonitor },
      ],
    },
    { path: '/admin/subscriptions', label: t('nav.subscriptions'), icon: CreditCardIcon, hideInSimpleMode: true },
    { path: '/admin/accounts', label: t('nav.accounts'), icon: GlobeIcon },
    { path: '/admin/announcements', label: t('nav.announcements'), icon: BellIcon },
    { path: '/admin/proxies', label: t('nav.proxies'), icon: ServerIcon },
    {
      path: '/admin/security-audit',
      label: t('nav.securityAudit'),
      icon: ShieldIcon,
      hideInSimpleMode: true,
      expandOnly: true,
      featureFlag: flagRiskControl,
      children: [
        { path: '/admin/risk-control', label: t('nav.contentModeration'), icon: ShieldIcon },
        { path: '/admin/prompt-audit', label: t('nav.promptAudit'), icon: ShieldIcon },
      ],
    },
    { path: '/admin/redeem', label: t('nav.redeemCodes'), icon: TicketIcon, hideInSimpleMode: true },
    { path: '/admin/promo-codes', label: t('nav.promoCodes'), icon: GiftIcon, hideInSimpleMode: true },
    { path: '/admin/promo-campaigns', label: t('nav.promoCampaigns'), icon: GiftIcon, hideInSimpleMode: true },
    {
      path: '/admin/affiliates',
      label: t('nav.affiliateManagement'),
      icon: UsersIcon,
      hideInSimpleMode: true,
      expandOnly: true,
      featureFlag: flagAffiliate,
      children: [
        { path: '/admin/affiliates/invites', label: t('nav.affiliateInviteRecords'), icon: UsersIcon },
        { path: '/admin/affiliates/rebates', label: t('nav.affiliateRebateRecords'), icon: OrderIcon },
        { path: '/admin/affiliates/transfers', label: t('nav.affiliateTransferRecords'), icon: CreditCardIcon },
      ],
    },
    {
      path: '/admin/orders',
      label: t('nav.orderManagement'),
      icon: OrderIcon,
      hideInSimpleMode: true,
      expandOnly: true,
      featureFlag: flagAdminPayment,
      children: [
        { path: '/admin/orders/dashboard', label: t('nav.paymentDashboard'), icon: ChartIcon },
        { path: '/admin/orders', label: t('nav.orderManagement'), icon: OrderIcon },
        { path: '/admin/orders/plans', label: t('nav.paymentPlans'), icon: CreditCardIcon },
      ],
    },
    { path: '/admin/usage', label: t('nav.platformUsage'), icon: ChartIcon },
    { path: '/admin/audit-logs', label: t('nav.auditLogs'), icon: ShieldIcon, hideInSimpleMode: true }
  ]

  const visible = applyFeatureFlags(baseItems)

  // 简单模式下，在系统设置前插入 API密钥
  if (authStore.isSimpleMode) {
    const filtered = visible.filter(item => !item.hideInSimpleMode)
    if (isRegistryVisible(productModuleRegistry.gateway)) {
      filtered.push({ path: productModuleRegistry.gateway.path, label: t(productModuleRegistry.gateway.localeKey), icon: KeyIcon })
    }
    filtered.push({ path: '/admin/settings', label: t('nav.settings'), icon: CogIcon })
    for (const cm of customMenuItemsForAdmin.value) {
      filtered.push({ path: `/custom/${cm.id}`, label: cm.label, icon: null, iconSvg: cm.icon_svg })
    }
    return dedupeSidebarItems(filtered)
  }

  visible.push({ path: '/admin/settings', label: t('nav.settings'), icon: CogIcon })
  for (const cm of customMenuItemsForAdmin.value) {
    visible.push({ path: `/custom/${cm.id}`, label: cm.label, icon: null, iconSvg: cm.icon_svg })
  }
  return dedupeSidebarItems(visible)
})

function toggleSidebar() {
  // Mobile drawer is controlled by mobileOpen, not desktop collapse width.
  if (mobileOpen.value) {
    closeMobile()
    return
  }
  appStore.toggleSidebar()
}

function closeMobile() {
  appStore.setMobileOpen(false)
}

function handleMoreToggle(event: Event) {
  if ((event.target as HTMLDetailsElement).open && sidebarCollapsed.value) appStore.toggleSidebar()
}

function focusMobileSidebar() {
  void nextTick(() => {
    const firstControl = sidebarNavRef.value?.querySelector<HTMLElement>('a, button')
    firstControl?.focus()
  })
}

function restorePreviousFocus() {
  const previous = previouslyFocusedElement.value
  previouslyFocusedElement.value = null
  if (previous && document.contains(previous)) {
    previous.focus()
  }
}

function setBackgroundInert(open: boolean) {
  const shellContent = document.querySelector<HTMLElement>('.bd-app-frame')
  if (shellContent) shellContent.inert = open
}

function handleSidebarTabKeydown(event: KeyboardEvent) {
  if (!mobileOpen.value || !sidebarRef.value) return
  const focusable = Array.from(
    sidebarRef.value.querySelectorAll<HTMLElement>('a[href], button:not([disabled]), summary, input:not([disabled]), select:not([disabled]), textarea:not([disabled])'),
  ).filter(element => element.getClientRects().length > 0 && getComputedStyle(element).visibility !== 'hidden')
  if (focusable.length === 0) return
  const first = focusable[0]
  const last = focusable[focusable.length - 1]
  const active = document.activeElement
  if (event.shiftKey && active === first) {
    event.preventDefault()
    last.focus()
  } else if (!event.shiftKey && active === last) {
    event.preventDefault()
    first.focus()
  }
}

function handleSidebarEscape(event: KeyboardEvent) {
  if (!mobileOpen.value) return
  event.preventDefault()
  closeMobile()
  restorePreviousFocus()
}

function handleGlobalSidebarKeydown(event: KeyboardEvent) {
  if (event.key === 'Escape') {
    handleSidebarEscape(event)
  }
}

function handleSoonClick(label: string) {
  soonNotice.value = t('nav.soonNotice', { label })
  if (soonNoticeTimer) {
    clearTimeout(soonNoticeTimer)
  }
  soonNoticeTimer = setTimeout(() => {
    soonNotice.value = ''
    soonNoticeTimer = null
  }, 1800)
}

function handleMenuItemClick(itemPath: string) {
  if (mobileOpen.value) {
    setTimeout(() => {
      appStore.setMobileOpen(false)
    }, 150)
  }

  // Map paths to tour selectors
  const pathToSelector: Record<string, string> = {
    '/admin/groups': '#sidebar-group-manage',
    '/admin/accounts': '#sidebar-channel-manage',
    '/keys': '[data-tour="sidebar-my-keys"]'
  }

  const selector = pathToSelector[itemPath]
  if (selector && onboardingStore.isCurrentStep(selector)) {
    onboardingStore.nextStep(500)
  }
}

function queryValue(value: unknown): string {
  if (Array.isArray(value)) return value.length > 0 ? String(value[0] ?? '') : ''
  return value == null ? '' : String(value)
}

function targetQueryMatches(targetPath: string): boolean {
  const target = router.resolve(targetPath)
  const targetQueryKeys = Object.keys(target.query)
  if (targetQueryKeys.length === 0) {
    if (target.path === '/community') {
      return !queryValue(route.query.district) && !queryValue(route.query.channel) && !queryValue(route.query.workspace)
    }
    if (target.path === '/tavern') {
      return !queryValue(route.query.view)
    }
    return true
  }
  return targetQueryKeys.every((key) => queryValue(route.query[key]) === queryValue(target.query[key]))
}

function isTargetActive(targetPath: string, exactPath: boolean): boolean {
  const target = router.resolve(targetPath)
  const pathMatches = exactPath ? route.path === target.path : route.path === target.path || route.path.startsWith(target.path + '/')
  return pathMatches && targetQueryMatches(targetPath)
}

function isNavItemActive(item: NavItem): boolean {
  if (item.children?.length) return isGroupActive(item)
  return isTargetActive(item.path, false)
}

function isChildActive(child: NavItem): boolean {
  return isTargetActive(child.path, true)
}

function isGroupActive(item: NavItem): boolean {
  return isSidebarGroupActive(item, path => isTargetActive(path, true))
}

function isGroupExpanded(item: NavItem): boolean {
  return expandedGroups.value.has(item.path)
}

function toggleGroup(item: NavItem) {
  if (expandedGroups.value.has(item.path)) {
    expandedGroups.value.delete(item.path)
    return
  }
  // Desktop product map: keep one top-level product domain open at a time.
  const topLevelPaths = new Set(['/gateway', '/community', '/operator', '/assets', '/marketplace', '/settings/account'])
  if (topLevelPaths.has(item.path)) {
    for (const path of topLevelPaths) {
      if (path !== item.path) expandedGroups.value.delete(path)
    }
  }
  expandedGroups.value.add(item.path)
}

/**
 * Click handler for collapsible parent items.
 * - When sidebar is collapsed: do nothing (children are not visible).
 * - When `expandOnly` is true: only toggle expand state.
 * - Otherwise (default, e.g. /admin/orders): navigate to the parent path
 *   (router-link semantics) and ensure the group is expanded.
 */
function handleGroupClick(item: NavItem) {
  if (sidebarCollapsed.value) return
  if (item.expandOnly) {
    toggleGroup(item)
    return
  }
  // Push to path and ensure expanded
  if (route.path !== item.path) {
    router.push(item.path)
  }
  if (!expandedGroups.value.has(item.path)) {
    expandedGroups.value.add(item.path)
  }
}

// Fetch admin settings (for feature-gated nav items like Ops).
watch(
  isAdmin,
  (v) => {
    if (v) {
      adminSettingsStore.fetch()
    }
  },
  { immediate: true }
)

function collectActiveExpandPaths(items: NavItem[]): string[] {
  const paths: string[] = []
  for (const item of items) {
    if (!item.children?.length) continue
    if (isGroupActive(item)) {
      paths.push(item.path)
      for (const child of item.children) {
        if (child.children?.length && isGroupActive(child)) {
          paths.push(child.path)
        }
      }
    }
  }
  return paths
}

function expandActiveRouteGroups() {
  const allItems = [
    ...userNavItems.value,
    ...personalNavItems.value,
    ...adminNavItems.value,
  ]
  const next = new Set(expandedGroups.value)
  // Prefer accordion for top-level product domains: close inactive top groups.
  for (const item of allItems) {
    if (!item.children?.length) continue
    if (['/gateway', '/community', '/operator', '/assets', '/marketplace', '/settings/account'].includes(item.path)) {
      if (!isGroupActive(item)) next.delete(item.path)
    }
  }
  for (const path of collectActiveExpandPaths(allItems)) {
    next.add(path)
  }
  expandedGroups.value = next
}

watch(
  () => [route.fullPath, isAdmin.value, authStore.isSimpleMode] as const,
  () => {
    expandActiveRouteGroups()
  },
  { immediate: true }
)

watch(
  mobileOpen,
  (open, wasOpen) => {
    setBackgroundInert(open)
    if (open && !wasOpen) {
      previouslyFocusedElement.value = document.activeElement instanceof HTMLElement ? document.activeElement : null
      focusMobileSidebar()
    } else if (!open && wasOpen) {
      restorePreviousFocus()
    }
  },
  { flush: 'post' },
)

onMounted(() => {
  window.addEventListener('keydown', handleGlobalSidebarKeydown)
  void refreshBatchImageAccess()
  if (isAdmin.value) {
    adminSettingsStore.fetch()
  }
  expandActiveRouteGroups()
  // Restore sidebar scroll position after route change re-mounts the component
  if (appStore.sidebarScrollTop > 0 && sidebarNavRef.value) {
    void nextTick(() => {
      if (sidebarNavRef.value) {
        sidebarNavRef.value.scrollTop = appStore.sidebarScrollTop
      }
    })
  }
})

onBeforeUnmount(() => {
  setBackgroundInert(false)
  window.removeEventListener('keydown', handleGlobalSidebarKeydown)
  if (sidebarNavRef.value) {
    appStore.sidebarScrollTop = sidebarNavRef.value.scrollTop
  }
})
</script>

<style scoped>
.sidebar {
  position: fixed;
  inset: 0 auto 0 0;
  z-index: 40;
  width: 218px;
  display: flex;
  flex-direction: column;
  transition: width .2s ease, transform .3s ease;
}

.sidebar.w-\[72px\] {
  width: 72px;
}

@media (max-width: 1023px) {
  .sidebar.-translate-x-full {
    transform: translateX(-100%);
    visibility: hidden;
    pointer-events: none !important;
  }

  .sidebar:not(.-translate-x-full) {
    visibility: visible;
  }
}

.sidebar-logo {
  flex: 0 0 2.5rem;
  min-width: 2.5rem;
}

.zero-sidebar-logo {
  border: none;
  background: transparent;
  box-shadow: none;
}

.sidebar-logo--brand {
  border: 1px solid var(--bd-line-brand-subtle);
  background: var(--bd-brand-paper);
  box-shadow: var(--bd-shadow-brand-mark);
}

.zero-soon-toast {
  border: 1px solid var(--zc-line-strong);
  background: var(--zc-surface-raised);
  color: var(--zc-text-strong);
  box-shadow: var(--zc-shadow-lift);
  backdrop-filter: var(--zc-blur);
  -webkit-backdrop-filter: var(--zc-blur);
}

.sidebar-header-collapsed {
  gap: 0;
  padding-left: 0.875rem;
  padding-right: 0.875rem;
}

.sidebar-brand {
  min-width: 0;
  flex: 1 1 auto;
  white-space: nowrap;
  transition: max-width 0.2s ease, opacity 0.12s ease, transform 0.12s ease;
  max-width: 11rem;
}

.sidebar-brand-collapsed {
  max-width: 0;
  overflow: hidden;
  opacity: 0;
  transform: translateX(-4px);
  pointer-events: none;
}

.sidebar-brand-title {
  display: block;
  overflow: hidden;
  color: var(--zc-text-strong) !important;
  text-overflow: ellipsis;
  white-space: nowrap;
  letter-spacing: 0;
}

.sidebar-brand-kicker {
  display: none;
}

.sidebar-link-collapsed {
  gap: 0;
  padding-left: 0.75rem;
  padding-right: 0.75rem;
}

.sidebar-section-title {
  position: relative;
  display: flex;
  align-items: center;
  min-height: 1.25rem;
  overflow: hidden;
  white-space: nowrap;
}

.sidebar-section-title-text {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  transition: opacity 0.14s ease, transform 0.14s ease;
}

.sidebar-section-title::after {
  content: '';
  position: absolute;
  left: 0.75rem;
  right: 0.75rem;
  top: 50%;
  height: 1px;
  background: var(--zc-line);
  opacity: 0;
  transform: translateY(-50%);
  transition: opacity 0.16s ease;
}

.sidebar-section-title-text-collapsed {
  opacity: 0;
  transform: translateX(-4px);
}

.sidebar-section-title-collapsed::after {
  opacity: 1;
  transition-delay: 0.06s;
}

.sidebar-label {
  display: block;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  transition: max-width 0.18s ease, opacity 0.1s ease, transform 0.1s ease;
  max-width: 11rem;
}

.sidebar-label-flex {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
}

.sidebar-label-collapsed {
  max-width: 0;
  opacity: 0;
  transform: translateX(-4px);
  pointer-events: none;
}

.sidebar.sidebar {
  background: var(--bd-surface);
  border-right: 1px solid var(--bd-ui-line);
  box-shadow: none;
  backdrop-filter: none !important;
  -webkit-backdrop-filter: none !important;
  pointer-events: auto !important;
  z-index: 120 !important;
  overflow: hidden;
  isolation: isolate;
}

:global(.sidebar),
:global(.sidebar *) {
  pointer-events: auto;
}

.sidebar::before {
  content: none;
}

.sidebar .sidebar-header {
  min-height: 58px;
  padding: 10px 14px;
  gap: 10px;
  border-bottom: 1px solid var(--bd-ui-line);
  position: relative;
}

.sidebar .sidebar-link {
  display: flex;
  align-items: center;
  gap: 10px;
  width: 100%;
  min-height: 36px;
  box-sizing: border-box;
  color: var(--zc-muted);
  border: 1px solid transparent;
  border-radius: 5px;
  margin: 2px 0;
  padding: 7px 10px;
  position: relative;
  font-size: 13px;
  font-weight: 500;
  text-decoration: none;
  text-shadow: none;
  background: transparent;
  box-shadow: none;
  transition: color 0.15s ease, background-color 0.15s ease;
}

.sidebar .sidebar-link:hover {
  background: var(--bd-canvas);
  border-color: transparent;
  color: var(--bd-text-primary);
  transform: none;
  box-shadow: none;
}

:global(.sidebar-link-featured:not(.sidebar-link-active)) {
  background: var(--zc-surface);
  border-color: var(--zc-line);
  color: #d14f67;
  box-shadow: 0 7px 18px color-mix(in srgb, var(--zc-shadow-dark) 18%, transparent);
}

:global(.sidebar-link-featured:not(.sidebar-link-active):hover) {
  background: var(--zc-bg);
  color: var(--zc-text-strong);
  box-shadow:
    6px 6px 14px var(--zc-shadow-dark),
    -6px -6px 14px var(--zc-shadow-light);
}

.sidebar .sidebar-link-active {
  background: color-mix(in srgb, var(--bd-accent-teal) 9%, var(--bd-surface));
  border-color: transparent;
  color: var(--bd-accent-teal);
  box-shadow: none;
  font-weight: 600;
}

.sidebar-link-active::before {
  content: none;
}

:global(.sidebar-link-soon) {
  color: var(--zc-subtle);
  opacity: 1;
}

:global(.sidebar-link-soon:hover) {
  background: var(--zc-bg);
  color: var(--zc-muted);
  box-shadow:
    4px 4px 10px var(--zc-shadow-dark),
    -4px -4px 10px var(--zc-shadow-light);
}

:global(.sidebar-link-soon svg),
:global(.sidebar-link-soon .sidebar-svg-icon) {
  color: var(--zc-subtle);
}

.sidebar-guide-hint{display:inline-flex;align-items:center;gap:5px;font-size:10px;color:var(--bd-accent-teal);margin-left:auto}
.sidebar-guide-hint i{width:6px;height:6px;background:currentColor;border-radius:50%}
.sidebar-discovery-dot { flex: 0 0 8px; width: 8px; height: 8px; margin-left: auto; border-radius: 50%; background: #dc2626; }
.sidebar-link-collapsed .sidebar-discovery-dot { position: absolute; top: 7px; right: 8px; margin: 0; }
.sidebar-badge {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  border-radius: 999px;
  border: 0;
  background: var(--zc-bg);
  color: var(--zc-muted);
  padding: 0.12rem 0.42rem;
  font-size: 0.68rem;
  font-weight: 900;
  letter-spacing: 0.01em;
  line-height: 1.15;
  box-shadow:
    3px 3px 7px var(--zc-shadow-dark),
    -3px -3px 7px var(--zc-shadow-light);
}

.sidebar-link-soon:hover .sidebar-badge {
  background: var(--zc-bg);
  color: var(--zc-text);
}

.sidebar-nested-group {
  margin-bottom: 0.2rem;
}

.sidebar-nested-parent {
  display: flex;
  align-items: center;
  gap: 0.2rem;
}

.sidebar-nested-parent .sidebar-link {
  margin-bottom: 0;
}

.sidebar-nested-toggle {
  display: inline-flex;
  width: 1.75rem;
  height: 1.75rem;
  flex-shrink: 0;
  align-items: center;
  justify-content: center;
  border: 0;
  border-radius: 0.7rem;
  background: var(--zc-bg);
  color: var(--zc-muted);
  box-shadow:
    3px 3px 7px var(--zc-shadow-dark),
    -3px -3px 7px var(--zc-shadow-light);
}

.sidebar-nested-toggle:hover {
  color: var(--zc-text);
}

.sidebar-third-level {
  margin: 0.2rem 0 0.35rem 0.8rem;
  padding-left: 0.45rem;
  border-left: 1px solid color-mix(in srgb, var(--zc-muted) 18%, transparent);
}

.sidebar-third-level .sidebar-link {
  min-height: 2rem;
  padding-left: 0.55rem;
  padding-right: 0.45rem;
}

.sidebar-recommendation {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 0.2rem;
  height: 1.25rem;
  flex-shrink: 0;
  border-radius: 999px;
  background: var(--zc-bg);
  color: var(--zc-accent);
  border: 0;
  padding: 0 0.38rem 0 0.28rem;
  font-size: 0.58rem;
  font-weight: 950;
  letter-spacing: 0.03em;
  box-shadow:
    3px 3px 7px var(--zc-shadow-dark),
    -3px -3px 7px var(--zc-shadow-light);
}

.sidebar-recommendation svg {
  width: 0.68rem;
  height: 0.68rem;
}

.sidebar-link-active .sidebar-recommendation {
  background: var(--zc-bg);
  color: var(--zc-accent);
  box-shadow:
    inset 3px 3px 7px var(--zc-shadow-dark),
    inset -3px -3px 7px var(--zc-shadow-light);
}

.sidebar .sidebar-section-title {
  color: var(--zc-subtle);
  min-height: 0;
  font-size: 11px;
  font-weight: 500;
  letter-spacing: 0;
  margin: 0;
  padding: 18px 10px 5px;
}

.sidebar .sidebar-nav { padding: 0 12px 12px; flex: 1; overflow-y: auto; min-height: 0; }
.sidebar .sidebar-brand-title { font-size: 16px; font-weight: 600; }
.sidebar .sidebar-section { margin: 0; }
.sidebar .sidebar-space-switch { padding: 8px 12px 0; }
.sidebar .sidebar-link > svg { width: 18px; height: 18px; flex-shrink: 0; }
.sidebar .sidebar-link-collapsed { justify-content: center; padding: 7px; }
.sidebar .sidebar-link:focus-visible, .sidebar summary:focus-visible { outline: 2px solid var(--bd-accent-teal); outline-offset: 1px; }
.sidebar-more { position: relative; }
.sidebar-more summary { list-style: none; cursor: pointer; }
.sidebar-more summary::-webkit-details-marker { display: none; }
.sidebar-more-menu { position: absolute; z-index: 2; bottom: 100%; left: 0; width: 192px; padding: 6px; border: 1px solid var(--bd-ui-line); border-radius: 6px; background: var(--bd-surface); box-shadow: 0 4px 14px rgb(0 0 0 / .08); }
@media (max-width: 1023px) {
  .sidebar.sidebar { width: min(280px, calc(100vw - 48px)); }
  .sidebar .sidebar-link { min-height: 44px; }
}

.sidebar-svg-icon {
  color: currentColor;
}

.sidebar-svg-icon :deep(svg) {
  display: block;
  width: 1.25rem;
  height: 1.25rem;
}
</style>
