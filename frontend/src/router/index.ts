/**
 * Vue Router configuration for Sub2API frontend
 * Defines all application routes with lazy loading and navigation guards
 */

import { createRouter, createWebHistory, type RouteMeta, type RouteRecordRaw } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { useAppStore } from '@/stores/app'
import { useAdminSettingsStore } from '@/stores/adminSettings'
import { useAdminComplianceStore } from '@/stores/adminCompliance'
import { useNavigationLoadingState } from '@/composables/useNavigationLoading'
import { useRoutePrefetch } from '@/composables/useRoutePrefetch'
import { getSetupStatus } from '@/api/setup'
import { resolveCompletedSetupRedirectPath } from './setupRedirect'
import { resolveRouteDocumentTitle } from './title'
import { isLocalPreviewAuth } from '@/utils/localPreview'
import {
  canAccessProductModule,
  productModulePath,
  productModuleRegistry,
  productModuleRouteMeta,
  resolveProductModule,
} from '@/product/moduleRegistry'

function productRouteMeta(module: Parameters<typeof productModuleRouteMeta>[0], overrides: RouteMeta = {}): RouteMeta {
  return { ...overrides, ...productModuleRouteMeta(module) }
}

/**
 * Route definitions with lazy loading
 */
const routes: RouteRecordRaw[] = [
  // ==================== Setup Routes ====================
  {
    path: '/setup',
    name: 'Setup',
    component: () => import('@/views/setup/SetupWizardView.vue'),
    meta: {
      requiresAuth: false,
      title: 'Setup'
    }
  },

  // ==================== Public Routes ====================
  {
    path: '/home',
    name: 'Home',
    component: () => import('@/views/biz/BizLandingView.vue'),
    meta: {
      requiresAuth: false,
      title: 'BizDecipher AI Workspace',
      titleKey: 'biz.pageTitles.home'
    }
  },
  // Pruned marketing placeholders: /gateway, /models, /pool were standalone
  // info-only pages (i18n text cards, no backend). Consolidated into the
  // landing page to keep a single, focused public surface.
  { path: '/gateway', redirect: '/home' },
  { path: '/pool', redirect: '/home' },
  {
    path: '/contribute',
    name: 'contribute',
    component: () => import('@/views/biz/BizInfoView.vue'),
    meta: {
      requiresAuth: false,
      title: 'Contribute',
      titleKey: 'biz.pageTitles.contribute'
    }
  },
  {
    path: '/credits',
    name: 'credits',
    component: () => import('@/views/biz/BizInfoView.vue'),
    meta: {
      requiresAuth: false,
      title: 'Account Balance',
      titleKey: 'biz.pageTitles.credits'
    }
  },
  {
    path: '/pricing',
    redirect: '/credits'
  },
  {
    path: '/status',
    name: 'status',
    component: () => import('@/views/biz/BizInfoView.vue'),
    meta: {
      requiresAuth: false,
      title: 'Service Status',
      titleKey: 'biz.pageTitles.status'
    }
  },
  // Pruned marketing placeholder: /work was info-only (no backend).
  { path: '/work', redirect: '/home' },
  {
    path: '/custom-request',
    name: 'custom-request',
    component: () => import('@/views/biz/BizInfoView.vue'),
    meta: {
      requiresAuth: false,
      title: 'Custom Request',
      titleKey: 'biz.pageTitles.customRequest'
    }
  },
  {
    path: '/operators/apply',
    name: 'operators-apply',
    component: () => import('@/views/biz/BizInfoView.vue'),
    meta: {
      requiresAuth: false,
      title: 'Operators',
      titleKey: 'biz.pageTitles.operatorsApply'
    }
  },
  {
    path: '/login',
    name: 'Login',
    component: () => import('@/views/auth/LoginView.vue'),
    meta: {
      requiresAuth: false,
      title: 'Login',
      titleKey: 'home.login'
    }
  },
  {
    path: '/register',
    name: 'Register',
    component: () => import('@/views/auth/RegisterView.vue'),
    meta: {
      requiresAuth: false,
      title: 'Register',
      titleKey: 'auth.createAccount'
    }
  },
  {
    path: '/email-verify',
    name: 'EmailVerify',
    component: () => import('@/views/auth/EmailVerifyView.vue'),
    meta: {
      requiresAuth: false,
      title: 'Verify Email'
    }
  },
  {
    path: '/auth/callback',
    name: 'OAuthCallback',
    alias: '/auth/oauth/callback',
    component: () => import('@/views/auth/OAuthCallbackView.vue'),
    meta: {
      requiresAuth: false,
      title: 'OAuth Callback',
      titleKey: 'auth.oauthCallbackPageTitle'
    }
  },
  {
    path: '/auth/linuxdo/callback',
    name: 'LinuxDoOAuthCallback',
    component: () => import('@/views/auth/LinuxDoCallbackView.vue'),
    meta: {
      requiresAuth: false,
      title: 'LinuxDo OAuth Callback',
      titleKey: 'auth.linuxdoCallbackPageTitle'
    }
  },
  {
    path: '/auth/wechat/callback',
    name: 'WeChatOAuthCallback',
    component: () => import('@/views/auth/WechatCallbackView.vue'),
    meta: {
      requiresAuth: false,
      title: 'WeChat OAuth Callback',
      titleKey: 'auth.wechatCallbackPageTitle'
    }
  },
  {
    path: '/auth/wechat/payment/callback',
    name: 'WeChatPaymentOAuthCallback',
    component: () => import('@/views/auth/WechatPaymentCallbackView.vue'),
    meta: {
      requiresAuth: false,
      title: 'WeChat Payment Callback',
      titleKey: 'auth.wechatPaymentCallbackPageTitle'
    }
  },
  {
    path: '/auth/dingtalk/callback',
    name: 'DingTalkOAuthCallback',
    component: () => import('@/views/auth/DingTalkCallbackView.vue'),
    meta: {
      requiresAuth: false,
      title: 'DingTalk OAuth Callback',
      titleKey: 'auth.dingtalkCallbackPageTitle'
    }
  },
  {
    path: '/auth/dingtalk/email-completion',
    name: 'dingtalk-email-completion',
    component: () => import('@/views/auth/DingTalkEmailCompletionView.vue'),
    meta: {
      requiresAuth: false,
      title: 'DingTalk Email Completion'
    }
  },
  {
    path: '/auth/oidc/callback',
    name: 'OIDCOAuthCallback',
    component: () => import('@/views/auth/OidcCallbackView.vue'),
    meta: {
      requiresAuth: false,
      title: 'OIDC OAuth Callback',
      titleKey: 'auth.oidcCallbackPageTitle'
    }
  },
  {
    path: '/forgot-password',
    name: 'ForgotPassword',
    component: () => import('@/views/auth/ForgotPasswordView.vue'),
    meta: {
      requiresAuth: false,
      title: 'Forgot Password',
      titleKey: 'auth.forgotPasswordTitle'
    }
  },
  {
    path: '/reset-password',
    name: 'ResetPassword',
    component: () => import('@/views/auth/ResetPasswordView.vue'),
    meta: {
      requiresAuth: false,
      title: 'Reset Password'
    }
  },
  {
    path: '/key-usage',
    name: 'KeyUsage',
    component: () => import('@/views/KeyUsageView.vue'),
    meta: {
      requiresAuth: false,
      title: 'Key Usage',
    }
  },
  {
    path: '/legal/:documentId',
    name: 'LegalDocument',
    component: () => import('@/views/public/LegalDocumentView.vue'),
    meta: {
      requiresAuth: false,
      title: 'Legal Document'
    }
  },

  // ==================== User Routes ====================
  {
    path: '/',
    redirect: '/home'
  },
  {
    path: '/dashboard-preview',
    name: 'DashboardPreview',
    component: () => import('@/views/user/DashboardView.vue'),
    meta: {
      requiresAuth: false,
      requiresAdmin: false,
      title: 'BizDecipher Workspace',
      titleKey: 'dashboard.title',
      descriptionKey: 'dashboard.welcomeMessage'
    }
  },
  {
    path: '/zero-city-preview',
    name: 'ZeroCityPreview',
    redirect: '/community',
    meta: {
      requiresAuth: false,
      requiresAdmin: false,
      title: 'Zero City Preview',
    }
  },
  {
    path: '/dashboard',
    name: 'Dashboard',
    redirect: '/community',
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      title: 'Dashboard',
      titleKey: 'dashboard.title',
      descriptionKey: 'dashboard.welcomeMessage'
    }
  },
  {
    path: productModulePath(productModuleRegistry.gateway),
    name: 'Keys',
    component: () => import('@/views/user/KeysView.vue'),
    meta: productRouteMeta(productModuleRegistry.gateway, {
      title: 'API Keys',
      descriptionKey: 'keys.description'
    })
  },
  {
    path: '/batch-image',
    name: 'BatchImageGuide',
    alias: '/docs/batch-image',
    component: () => import('@/views/user/BatchImageGuideView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      title: 'Batch Image Guide',
      titleKey: 'batchImageGuide.title',
      descriptionKey: 'batchImageGuide.description'
    }
  },
  {
    path: '/usage',
    name: 'Usage',
    component: () => import('@/views/user/UsageView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      title: 'Usage Records',
      titleKey: 'usage.title',
      descriptionKey: 'usage.description'
    }
  },
  {
    path: '/docs',
    name: 'Docs',
    component: () => import('@/views/user/DocsView.vue'),
    meta: {
      requiresAuth: false,
      requiresAdmin: false,
      title: 'Docs',
      titleKey: 'nav.docs',
      description: 'BizDecipher workspace documentation'
    }
  },
  {
    path: '/full-model-test',
    name: 'FullModelTest',
    component: () => import('@/views/user/FullModelTestView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      title: 'Full Model Test',
      titleKey: 'nav.fullModelTest',
      description: 'Verify model service access with reproducible probes'
    }
  },
  {
    path: '/redeem',
    name: 'Redeem',
    component: () => import('@/views/user/RedeemView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      title: 'Redeem Code',
      titleKey: 'redeem.title',
      descriptionKey: 'redeem.description'
    }
  },
  {
    path: '/affiliate',
    name: 'Affiliate',
    component: () => import('@/views/user/AffiliateView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      title: 'Affiliate',
      titleKey: 'affiliate.title',
      descriptionKey: 'affiliate.description'
    }
  },
  {
    path: productModulePath(productModuleRegistry.capabilityAssets.children[3]),
    name: 'Incentives',
    component: () => import('@/views/user/IncentivesView.vue'),
    meta: productRouteMeta(productModuleRegistry.capabilityAssets.children[3], {
      title: 'Incentives',
      descriptionKey: 'incentives.description'
    })
  },
  {
    path: '/wallet',
    name: 'Wallet',
    component: () => import('@/views/user/WalletView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      title: '钱包',
      description: '站内余额、积分与共享池经营收益'
    }
  },
  {
    path: productModulePath(productModuleRegistry.workbench),
    name: 'Operator',
    component: () => import('@/features/bizdecipher/workbench/HarnessWorkbenchView.vue'),
    meta: productRouteMeta(productModuleRegistry.workbench, {
      title: 'Workbench',
      description: 'Capability asset workbench'
    })
  },
  {
    path: productModulePath(productModuleRegistry.bilateralMarket),
    name: 'CapabilityMarketplace',
    component: () => import('@/features/bizdecipher/views/user/MarketplaceView.vue'),
    meta: productRouteMeta(productModuleRegistry.bilateralMarket, {
      title: 'Marketplace',
      description: 'Capability asset bilateral marketplace'
    })
  },
  {
    path: productModulePath(productModuleRegistry.zeroCity),
    name: 'Community',
    component: () => import('@/views/user/CommunityView.vue'),
    meta: productRouteMeta(productModuleRegistry.zeroCity, {
      title: 'Zero City',
      descriptionKey: 'community.description'
    })
  },
  {
    path: '/tavern',
    name: 'ZeroCityTavern',
    component: () => import('@/views/user/TavernView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      title: '剧本与游戏',
      description: '零号城本地故事、公开剧本与真实房间管理'
    }
  },
  {
    path: '/tavern/lounge',
    name: 'ZeroCityTavernLounge',
    redirect: { path: '/community', query: { district: 'tavern', chat: '1' } },
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      title: '零号城聊天',
      description: '旧实时大厅入口，落到全站聊天窗口'
    }
  },
  {
    path: '/tavern-stage',
    name: 'ZeroCityTavernStage',
    component: () => import('@/views/user/TavernStageView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      title: '酒馆舞台',
      description: '零号城酒馆短期舞台会话页'
    }
  },
  {
    path: productModulePath(productModuleRegistry.capabilityAssets),
    name: 'CapabilityAssets',
    component: () => import('@/views/user/CapabilityAssetsView.vue'),
    meta: productRouteMeta(productModuleRegistry.capabilityAssets, {
      title: '能力资产',
      description: '用户产品、工作流、Agent、工具和成果的能力货架'
    })
  },
  {
    path: '/zero-city/cards',
    name: 'ZeroCityCards',
    component: () => import('@/views/user/ZeroCityCardsView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      title: '收藏卡册'
    }
  },
  {
    path: '/zero-city/chronicle',
    name: 'ZeroCityChronicle',
    component: () => import('@/features/bizdecipher/views/user/ZeroCityChronicleView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      title: '城市编年史',
      description: '零号城世界观、城区生活与收藏卡角色故事'
    }
  },
  {
    path: '/zero-city/:target',
    name: 'ZeroCityProfile',
    component: () => import('@/views/user/ZeroCityProfileView.vue'),
    meta: {
      requiresAuth: false,
      requiresAdmin: false,
      title: 'Zero City Profile'
    }
  },
  { path: '/models', redirect: '/available-channels' },
  {
    path: '/available-channels',
    name: 'UserAvailableChannels',
    component: () => import('@/views/user/AvailableChannelsView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      title: 'Available Channels',
      titleKey: 'availableChannels.title',
      descriptionKey: 'availableChannels.description'
    }
  },
  {
    path: '/playground',
    name: 'Playground',
    component: () => import('@/views/user/PlaygroundView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      title: 'AI Playground'
    }
  },
  {
    path: productModulePath(productModuleRegistry.sharedMarket),
    name: 'AccountSquare',
    component: () => import('@/views/user/AccountSquareView.vue'),
    meta: productRouteMeta(productModuleRegistry.sharedMarket, {
      title: '共享市场',
      sharedMarketSpace: 'market'
    })
  },
  {
    path: productModulePath(productModuleRegistry.sharedMarket.children[1]),
    name: 'AccountSquareMy',
    component: () => import('@/views/user/AccountSquareMyView.vue'),
    meta: productRouteMeta(productModuleRegistry.sharedMarket.children[1], {
      title: '我的共享池',
      sharedMarketSpace: 'member'
    })
  },
  {
    path: productModulePath(productModuleRegistry.sharedMarket.children[2]),
    name: 'AccountSquareOwner',
    component: () => import('@/views/user/AccountSquareOwnerView.vue'),
    meta: productRouteMeta(productModuleRegistry.sharedMarket.children[2], {
      title: '池主中心',
      sharedMarketSpace: 'owner'
    })
  },
  {
    path: '/account-square/pool/:id',
    name: 'PoolDetail',
    component: () => import('@/views/user/PoolDetailView.vue'),
    meta: {
      requiresAuth: !import.meta.env.DEV,
      requiresAdmin: false,
      title: 'Pool Detail'
    }
  },
  {
    path: '/account-square/owner/pools/:id',
    name: 'AccountSquareOwnerPoolDetail',
    component: () => import('@/views/user/AccountSquareOwnerPoolDetailView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      title: '池主管理'
    }
  },
  {
    path: '/profile',
    redirect: '/community?workspace=mine',
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      title: '我的零号城',
      titleKey: 'nav.myCity'
    }
  },
  {
    path: '/settings/account',
    name: 'AccountSettings',
    component: () => import('@/views/user/ProfileView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      title: '账户设置',
      descriptionKey: 'profile.description'
    }
  },
  {
    path: '/subscriptions',
    name: 'Subscriptions',
    component: () => import('@/views/user/SubscriptionsView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      title: 'My Subscriptions',
      titleKey: 'userSubscriptions.title',
      descriptionKey: 'userSubscriptions.description'
    }
  },
  {
    path: '/purchase',
    name: 'PurchaseSubscription',
    component: () => import('@/views/user/PaymentView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      title: 'Purchase Subscription',
      titleKey: 'nav.buySubscription',
      descriptionKey: 'purchase.description'
    }
  },
  {
    path: '/orders',
    name: 'OrderList',
    component: () => import('@/views/user/UserOrdersView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      title: 'My Orders',
      titleKey: 'nav.myOrders',
      requiresPayment: true
    }
  },
  {
    path: '/payment/qrcode',
    name: 'PaymentQRCode',
    component: () => import('@/views/user/PaymentQRCodeView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      title: 'Payment',
      titleKey: 'payment.qr.scanToPay',
      requiresPayment: true
    }
  },
  {
    path: '/payment/result',
    name: 'PaymentResult',
    component: () => import('@/views/user/PaymentResultView.vue'),
    meta: {
      requiresAuth: false,
      requiresAdmin: false,
      title: 'Payment Result',
      titleKey: 'payment.result.success',
      requiresPayment: false
    }
  },
  {
    path: '/payment/stripe',
    name: 'StripePayment',
    component: () => import('@/views/user/StripePaymentView.vue'),
    meta: {
      requiresAuth: false,
      requiresAdmin: false,
      title: 'Stripe Payment',
      titleKey: 'payment.stripePay',
      requiresPayment: false
    }
  },
  {
    path: '/payment/airwallex',
    name: 'AirwallexPayment',
    component: () => import('@/views/user/AirwallexPaymentView.vue'),
    meta: {
      requiresAuth: false,
      requiresAdmin: false,
      title: 'Airwallex Payment',
      titleKey: 'payment.airwallexPay',
      requiresPayment: false
    }
  },
  {
    path: '/payment/stripe-popup',
    name: 'StripePopup',
    component: () => import('@/views/user/StripePopupView.vue'),
    meta: {
      requiresAuth: false,
      requiresAdmin: false,
      title: 'Payment',
      requiresPayment: false
    }
  },
  {
    path: '/custom/:id',
    name: 'CustomPage',
    component: () => import('@/views/user/CustomPageView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      title: 'Custom Page',
      titleKey: 'customPage.title',
    }
  },

  // ==================== Admin Routes ====================
  {
    path: '/admin',
    redirect: '/admin/dashboard'
  },
  {
    path: '/admin/dashboard',
    name: 'AdminDashboard',
    component: () => import('@/views/admin/DashboardView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'Admin Dashboard',
      titleKey: 'admin.dashboard.title',
      descriptionKey: 'admin.dashboard.description'
    }
  },
  {
    path: '/admin/ops',
    name: 'AdminOps',
    component: () => import('@/views/admin/ops/OpsDashboard.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'Ops Monitoring',
      titleKey: 'admin.ops.title',
      descriptionKey: 'admin.ops.description'
    }
  },
  {
    path: '/admin/shared-pools',
    name: 'AdminSharedPoolGovernance',
    component: () => import('@/views/admin/SharedPoolGovernanceView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: '共享市场治理',
      description: '共享池排序、抽水、奖惩、上下架和治理状态管理'
    }
  },
  {
    path: '/admin/capability-assets',
    name: 'AdminCapabilityAssets',
    component: () => import('@/views/admin/CapabilityAssetsAdminView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: '能力资产审核',
      description: '审核用户提交的能力资产，管理上架、推荐和下架'
    }
  },
  {
    path: '/admin/tavern-scripts',
    name: 'AdminTavernScripts',
    component: () => import('@/views/admin/TavernScriptsAdminView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: '酒馆剧本审核',
      description: '审核用户提交的酒馆剧本，管理上架、驳回和下架'
    }
  },
  {
    path: '/admin/audit-logs',
    name: 'AdminAuditLogs',
    component: () => import('@/views/admin/AuditLogView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'Audit Logs',
      titleKey: 'admin.audit.title',
      descriptionKey: 'admin.audit.description'
    }
  },
  {
    path: '/admin/users',
    name: 'AdminUsers',
    component: () => import('@/views/admin/UsersView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'User Management',
      titleKey: 'admin.users.title',
      descriptionKey: 'admin.users.description'
    }
  },
  {
    path: '/admin/groups',
    name: 'AdminGroups',
    component: () => import('@/views/admin/GroupsView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'Group Management',
      titleKey: 'admin.groups.title',
      descriptionKey: 'admin.groups.description'
    }
  },
  {
    path: '/admin/channels',
    redirect: '/admin/channels/pricing'
  },
  {
    path: '/admin/channels/pricing',
    name: 'AdminChannels',
    component: () => import('@/views/admin/ChannelsView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'Channel Management',
      titleKey: 'admin.channels.title',
      descriptionKey: 'admin.channels.description'
    }
  },
  {
    path: '/admin/channels/monitor',
    name: 'AdminChannelMonitor',
    component: () => import('@/views/admin/ChannelMonitorView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'Channel Monitor',
      titleKey: 'admin.channelMonitor.title',
      descriptionKey: 'admin.channelMonitor.description'
    }
  },
  {
    path: '/monitor',
    name: 'ChannelStatus',
    component: () => import('@/views/user/ChannelStatusView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      title: 'Channel Status',
      titleKey: 'nav.channelStatus'
    }
  },
  {
    path: '/admin/subscriptions',
    name: 'AdminSubscriptions',
    component: () => import('@/views/admin/SubscriptionsView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'Subscription Management',
      titleKey: 'admin.subscriptions.title',
      descriptionKey: 'admin.subscriptions.description'
    }
  },
  {
    path: '/admin/accounts',
    name: 'AdminAccounts',
    component: () => import('@/views/admin/AccountsView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'Account Management',
      titleKey: 'admin.accounts.title',
      descriptionKey: 'admin.accounts.description'
    }
  },
  {
    path: '/admin/announcements',
    name: 'AdminAnnouncements',
    component: () => import('@/views/admin/AnnouncementsView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'Announcements',
      titleKey: 'admin.announcements.title',
      descriptionKey: 'admin.announcements.description'
    }
  },
  {
    path: '/admin/proxies',
    name: 'AdminProxies',
    component: () => import('@/views/admin/ProxiesView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'Proxy Management',
      titleKey: 'admin.proxies.title',
      descriptionKey: 'admin.proxies.description'
    }
  },
  {
    path: '/admin/redeem',
    name: 'AdminRedeem',
    component: () => import('@/views/admin/RedeemView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'Redeem Code Management',
      titleKey: 'admin.redeem.title',
      descriptionKey: 'admin.redeem.description'
    }
  },
  {
    path: '/admin/promo-codes',
    name: 'AdminPromoCodes',
    component: () => import('@/views/admin/PromoCodesView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'Promo Code Management',
      titleKey: 'admin.promo.title',
      descriptionKey: 'admin.promo.description'
    }
  },
  {
    path: '/admin/promo-campaigns',
    name: 'AdminPromoCampaigns',
    component: () => import('@/views/admin/PromoCampaignsView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'Promo Campaign Management',
      titleKey: 'admin.promoCampaigns.title',
      descriptionKey: 'admin.promoCampaigns.description'
    }
  },
  {
    path: '/admin/settings',
    name: 'AdminSettings',
    component: () => import('@/views/admin/SettingsView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'System Settings',
      titleKey: 'admin.settings.title',
      descriptionKey: 'admin.settings.description'
    }
  },
  {
    path: '/admin/risk-control',
    name: 'AdminRiskControl',
    component: () => import('@/views/admin/RiskControlView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'Risk Control',
      titleKey: 'admin.riskControl.title',
      descriptionKey: 'admin.riskControl.description',
      requiresRiskControl: true
    }
  },
  {
    path: '/admin/prompt-audit',
    name: 'AdminPromptAudit',
    component: () => import('@/features/prompt-audit/PromptAuditView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'Prompt Audit',
      titleKey: 'admin.promptAudit.title',
      descriptionKey: 'admin.promptAudit.description',
      requiresRiskControl: true
    }
  },
  {
    path: '/admin/usage',
    name: 'AdminUsage',
    component: () => import('@/views/admin/UsageView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'Usage Records',
      titleKey: 'admin.usage.title',
      descriptionKey: 'admin.usage.description'
    }
  },
  {
    path: '/admin/affiliates',
    redirect: '/admin/affiliates/invites'
  },
  {
    path: '/admin/affiliates/invites',
    name: 'AdminAffiliateInvites',
    component: () => import('@/views/admin/affiliates/AdminAffiliateInvitesView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'Affiliate Invite Records',
      titleKey: 'nav.affiliateInviteRecords',
      descriptionKey: 'admin.affiliates.invitesDescription'
    }
  },
  {
    path: '/admin/affiliates/rebates',
    name: 'AdminAffiliateRebates',
    component: () => import('@/views/admin/affiliates/AdminAffiliateRebatesView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'Affiliate Rebate Records',
      titleKey: 'nav.affiliateRebateRecords',
      descriptionKey: 'admin.affiliates.rebatesDescription'
    }
  },
  {
    path: '/admin/affiliates/transfers',
    name: 'AdminAffiliateTransfers',
    component: () => import('@/views/admin/affiliates/AdminAffiliateTransfersView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'Affiliate Transfer Records',
      titleKey: 'nav.affiliateTransferRecords',
      descriptionKey: 'admin.affiliates.transfersDescription'
    }
  },


  // ==================== BizDecipher Admin Routes ====================
  // 这些入口接真实 /admin/biz/* 接口（贡献者、运营商、定制请求、积分发放、池/平台状态）。
  {
    path: '/admin/identities',
    name: 'admin-identities',
    component: () => import('@/views/biz/BizInfoView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'BizDecipher Identities',
      titleKey: 'biz.pageTitles.adminIdentities'
    }
  },
  {
    path: '/admin/pool',
    name: 'admin-pool',
    component: () => import('@/views/biz/BizInfoView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'Pool Admin',
      titleKey: 'biz.pageTitles.adminPool'
    }
  },
  {
    path: '/admin/contributions',
    name: 'admin-contributions',
    component: () => import('@/views/biz/BizInfoView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'Contribution Review',
      titleKey: 'biz.pageTitles.adminContributions'
    }
  },
  {
    path: '/admin/rewards',
    name: 'admin-rewards',
    component: () => import('@/views/biz/BizInfoView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'Rewards Admin',
      titleKey: 'biz.pageTitles.adminRewards'
    }
  },
  {
    path: '/admin/credit-system',
    redirect: '/admin/credits'
  },
  {
    path: '/admin/credits',
    name: 'admin-credits',
    component: () => import('@/views/admin/CreditSystemView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: '积分系统后台',
      titleKey: 'biz.pageTitles.adminCredits'
    }
  },
  {
    path: '/admin/requests',
    name: 'admin-requests',
    component: () => import('@/views/biz/BizInfoView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'Custom Requests',
      titleKey: 'biz.pageTitles.adminRequests'
    }
  },
  {
    path: '/admin/operators',
    name: 'admin-operators',
    component: () => import('@/views/biz/BizInfoView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'Operators Admin',
      titleKey: 'biz.pageTitles.adminOperators'
    }
  },
  {
    path: '/admin/status',
    name: 'admin-status',
    component: () => import('@/views/biz/BizInfoView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'Gateway Status Admin',
      titleKey: 'biz.pageTitles.adminStatus'
    }
  },

  // ==================== Payment Admin Routes ====================
  {
    path: '/admin/orders/dashboard',
    name: 'AdminPaymentDashboard',
    component: () => import('@/views/admin/orders/AdminPaymentDashboardView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'Payment Dashboard',
      titleKey: 'nav.paymentDashboard',
      requiresPayment: true
    }
  },
  {
    path: '/admin/orders',
    name: 'AdminOrders',
    component: () => import('@/views/admin/orders/AdminOrdersView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'Order Management',
      titleKey: 'nav.orderManagement',
      requiresPayment: true
    }
  },
  {
    path: '/admin/orders/plans',
    name: 'AdminPaymentPlans',
    component: () => import('@/views/admin/orders/AdminPaymentPlansView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'Subscription Plans',
      titleKey: 'nav.paymentPlans',
      requiresPayment: true
    }
  },

  // ==================== 404 Not Found ====================
  {
    path: '/:pathMatch(.*)*',
    name: 'NotFound',
    component: () => import('@/views/NotFoundView.vue'),
    meta: {
      title: '404 Not Found'
    }
  }
]

/**
 * Create router instance
 */
const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes,
  scrollBehavior(to, _from, savedPosition) {
    if (savedPosition) {
      return savedPosition
    }
    if (to.hash) {
      return {
        el: to.hash,
        top: 88,
        behavior: 'smooth'
      }
    }
    return { top: 0 }
  }
})

/**
 * Navigation guard: Authentication check
 */
let authInitialized = false

// 初始化导航加载状态和预加载
const navigationLoading = useNavigationLoadingState()
// 延迟初始化预加载，传入 router 实例
let routePrefetch: ReturnType<typeof useRoutePrefetch> | null = null
const BACKEND_MODE_ALLOWED_PATHS = ['/login', '/key-usage', '/setup', '/payment/result', '/payment/airwallex', '/legal']
const BACKEND_MODE_CALLBACK_PATHS = [
  '/auth/callback',
  '/auth/linuxdo/callback',
  '/auth/dingtalk/callback',
  '/auth/dingtalk/email-completion',
  '/auth/oidc/callback',
  '/auth/wechat/callback',
  '/auth/wechat/payment/callback',
]
const BACKEND_MODE_PENDING_AUTH_PATHS = ['/register', '/email-verify']
const DEV_AUTH_BYPASS = import.meta.env.DEV

function isBackendModePublicRouteAllowed(path: string, hasPendingAuthSession: boolean): boolean {
  if (BACKEND_MODE_ALLOWED_PATHS.some((allowedPath) => path === allowedPath || path.startsWith(allowedPath))) {
    return true
  }

  if (BACKEND_MODE_CALLBACK_PATHS.some((callbackPath) => path === callbackPath)) {
    return true
  }

  if (hasPendingAuthSession && BACKEND_MODE_PENDING_AUTH_PATHS.some((allowedPath) => path === allowedPath)) {
    return true
  }

  return false
}

router.beforeEach(async (to, _from, next) => {
  // 开始导航加载状态
  navigationLoading.startNavigation()

  const authStore = useAuthStore()

  // Restore auth state from localStorage on first navigation (page refresh)
  if (!authInitialized) {
    authStore.checkAuth()
    authInitialized = true
  }

  // Resolve the route title after authentication state has been restored.
  const appStore = useAppStore()
  const adminSettingsStore = useAdminSettingsStore()
  const customMenuItems = [
    ...(appStore.cachedPublicSettings?.custom_menu_items ?? []),
    ...(authStore.isAdmin ? adminSettingsStore.customMenuItems : []),
  ]
  document.title = resolveRouteDocumentTitle(to, appStore.siteName, customMenuItems)

  // Check if route requires authentication
  const requiresAuth = to.meta.requiresAuth !== false // Default to true
  const requiresAdmin = to.meta.requiresAdmin === true

  // Canonical product registry is the final fail-closed gate for module routes.
  const productModule = resolveProductModule(to.path)
  if (productModule) {
    const registryAccess = {
      isAuthenticated: authStore.isAuthenticated || DEV_AUTH_BYPASS,
      isAdmin: authStore.isAdmin,
      allowPreview: DEV_AUTH_BYPASS && isLocalPreviewAuth(),
    }
    if (!canAccessProductModule(productModule, registryAccess)) {
      if (productModule.status === 'hidden' || productModule.status === 'preview' || productModule.status === 'disabled') {
        next(authStore.isAdmin ? '/admin/dashboard' : '/dashboard')
        return
      }
      if (productModule.requiresAuth && !authStore.isAuthenticated && !DEV_AUTH_BYPASS) {
        next({ path: '/login', query: { redirect: to.fullPath } })
        return
      }
      if (productModule.requiresAdmin && !authStore.isAdmin && !DEV_AUTH_BYPASS) {
        next('/dashboard')
        return
      }
    }
  }

  if (to.path === '/setup') {
    try {
      const status = await getSetupStatus()
      if (!status.needs_setup) {
        next(resolveCompletedSetupRedirectPath(authStore.isAuthenticated, authStore.isAdmin))
        return
      }
    } catch {
      // If setup status cannot be determined, keep the setup page reachable.
    }
  }

  // If route doesn't require auth, allow access
  if (!requiresAuth) {
    // If already authenticated and trying to access login/register, redirect to appropriate dashboard
    if (authStore.isAuthenticated && (to.path === '/login' || to.path === '/register')) {
      // In backend mode, non-admin users should NOT be redirected away from login
      // (they are blocked from all protected routes, so redirecting would cause a loop)
      if (appStore.backendModeEnabled && !authStore.isAdmin) {
        next()
        return
      }
      // Admin users go to admin dashboard, regular users go to user dashboard
      next(authStore.isAdmin ? '/admin/dashboard' : '/dashboard')
      return
    }
    // Backend mode: block public pages for unauthenticated users (except login, key-usage, setup)
    if (appStore.backendModeEnabled && !authStore.isAuthenticated) {
      const isAllowed = isBackendModePublicRouteAllowed(to.path, authStore.hasPendingAuthSession)
      if (!isAllowed) {
        next('/login')
        return
      }
    }
    next()
    return
  }

  // Route requires authentication
  if (!authStore.isAuthenticated && !DEV_AUTH_BYPASS) {
    // Not authenticated, redirect to login
    next({
      path: '/login',
      query: { redirect: to.fullPath } // Save intended destination
    })
    return
  }

  // Check admin requirement
  if (requiresAdmin && !authStore.isAdmin && !DEV_AUTH_BYPASS) {
    // User is authenticated but not admin, redirect to user dashboard
    next('/dashboard')
    return
  }

  if (requiresAdmin && authStore.isAdmin) {
    const adminComplianceStore = useAdminComplianceStore()
    if (!adminComplianceStore.initialized) {
      try {
        await adminComplianceStore.fetchStatus()
      } catch (error) {
        const err = error as { status?: number; code?: string; metadata?: Record<string, string> }
        if (err.status === 423 && err.code === 'ADMIN_COMPLIANCE_ACK_REQUIRED') {
          adminComplianceStore.requireAcknowledgement(err.metadata)
        }
      }
    }
  }


  // 公共设置可能尚未加载（App.vue 的 onMounted 异步拉取晚于首次导航，且纯静态部署
  // 无 __APP_CONFIG__ 注入）。此时 cachedPublicSettings 为空会把 payment/risk_control
  // 误判为“未启用”而错误拦截，故这里先确保设置加载完成。
  if ((to.meta.requiresPayment || to.meta.requiresRiskControl) && !appStore.publicSettingsLoaded) {
    try {
      await appStore.fetchPublicSettings()
    } catch (error) {
      console.warn('Failed to load public settings in route guard', error)
    }
  }

  // Only an explicit value from successfully loaded settings can disable a route.
  // A transient settings failure is unknown state, not a confirmed feature toggle.
  if (
    to.meta.requiresPayment &&
    appStore.publicSettingsLoaded &&
    appStore.cachedPublicSettings?.payment_enabled === false
  ) {
    next(authStore.isAdmin ? '/admin/dashboard' : '/dashboard')
    return
  }

  if (
    to.meta.requiresRiskControl &&
    appStore.publicSettingsLoaded &&
    appStore.cachedPublicSettings?.risk_control_enabled === false
  ) {
    next(authStore.isAdmin ? '/admin/settings' : '/dashboard')
    return
  }

  // 简易模式下限制访问某些页面
  if (authStore.isSimpleMode) {
    const restrictedPaths = [
      '/admin/groups',
      '/admin/subscriptions',
      '/admin/redeem',
      '/subscriptions',
      '/redeem'
    ]

    if (restrictedPaths.some((path) => to.path.startsWith(path))) {
      // 简易模式下访问受限页面,重定向到仪表板
      next(authStore.isAdmin ? '/admin/dashboard' : '/dashboard')
      return
    }
  }

  // Backend mode: admin gets full access, non-admin blocked
  if (appStore.backendModeEnabled) {
    if (authStore.isAuthenticated && authStore.isAdmin) {
      next()
      return
    }
    const isAllowed = isBackendModePublicRouteAllowed(to.path, authStore.hasPendingAuthSession)
    if (!isAllowed) {
      next('/login')
      return
    }
  }

  // All checks passed, allow navigation
  next()
})

/**
 * Navigation guard: End loading and trigger prefetch
 */
router.afterEach((to) => {
  // 结束导航加载状态
  navigationLoading.endNavigation()

  // 懒初始化预加载（首次导航时创建，传入 router 实例）
  if (!routePrefetch) {
    routePrefetch = useRoutePrefetch(router)
  }
  // 触发路由预加载（在浏览器空闲时执行）
  routePrefetch.triggerPrefetch(to)
})

/**
 * Navigation guard: Error handling
 * Handles dynamic import failures caused by deployment updates
 */
router.onError((error) => {
  console.error('Router error:', error)

  // Check if this is a dynamic import failure (chunk loading error)
  const isChunkLoadError =
    error.message?.includes('Failed to fetch dynamically imported module') ||
    error.message?.includes('Loading chunk') ||
    error.message?.includes('Loading CSS chunk') ||
    error.name === 'ChunkLoadError'

  if (isChunkLoadError) {
    // Avoid infinite reload loop by checking sessionStorage
    const reloadKey = 'chunk_reload_attempted'
    const lastReload = sessionStorage.getItem(reloadKey)
    const now = Date.now()

    // Allow reload if never attempted or more than 10 seconds ago
    if (!lastReload || now - parseInt(lastReload) > 10000) {
      sessionStorage.setItem(reloadKey, now.toString())
      console.warn('Chunk load error detected, reloading page to fetch latest version...')
      window.location.reload()
    } else {
      console.error('Chunk load error persists after reload. Please clear browser cache.')
    }
  }
})

export default router
