<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { Archive, BriefcaseBusiness, ClipboardList, Edit3, MessageSquare, Plus, RefreshCw, Search, Send, Users, X } from '@lucide/vue'
import { useAuthStore } from '@/stores/auth'
import { extractActionableApiErrorMessage } from '@/utils/apiError'
import {
  marketplaceAPI,
  type MarketplaceInquiry,
  type MarketplaceListing,
  type MarketplaceListingInput,
  type MarketplaceListingKind,
  type MarketplaceMessage,
  type MarketplaceOrderQuoteInput,
} from '@/features/bizdecipher/api/marketplace'
import MarketplaceOrdersPanel from './MarketplaceOrdersPanel.vue'
import MarketplaceDisputesPanel from './MarketplaceDisputesPanel.vue'
import { useMarketplaceGuide } from '@/features/bizdecipher/composables/useMarketplaceGuide'

type MarketView = 'browse' | 'mine' | 'messages' | 'orders' | 'disputes'
type LegacyDraft = MarketplaceListingInput & { id: string }
const auth = useAuthStore()
const currentUserId = computed(() => Number(auth.user?.id ?? 0))
const guide = useMarketplaceGuide(currentUserId)
const guideExpanded = ref(false)
const showGuide = computed(() => guide.pending.value || guideExpanded.value)
async function chooseGuide(intent: 'hire' | 'sell' | 'browse') {
  const userId = currentUserId.value
  if (!await guide.complete(intent) || userId !== currentUserId.value) return
  guideExpanded.value = false
  if (intent !== 'browse') {
    resetForm()
    form.kind = intent === 'hire' ? 'demand' : 'service'
    editorOpen.value = true
  }
}
const tabs = [{ key: 'service', label: '找服务', icon: BriefcaseBusiness }, { key: 'talent', label: '找人才', icon: Users }, { key: 'demand', label: '接需求', icon: Search }] as const
const categories = ['全部领域', '图片与设计', '视频与声音', '代码与自动化', '研究与咨询', '游戏与故事']
const view = ref<MarketView>('browse')
const kind = ref<MarketplaceListingKind>('service')
const category = ref('全部领域')
const tag = ref('')
const listings = ref<MarketplaceListing[]>([])
const myListings = ref<MarketplaceListing[]>([])
const inquiries = ref<MarketplaceInquiry[]>([])
const messages = ref<MarketplaceMessage[]>([])
const selected = ref<MarketplaceListing | null>(null)
const activeInquiry = ref<MarketplaceInquiry | null>(null)
const nextListingCursor = ref<number | null>(null)
const nextMineCursor = ref<number | null>(null)
const nextInquiryCursor = ref<number | null>(null)
const nextMessageCursor = ref<number | null>(null)
const loading = ref(false)
const saving = ref(false)
const sending = ref(false)
const error = ref('')
const notice = ref('')
const editorOpen = ref(false)
const editingId = ref<number | null>(null)
const messageBody = ref('')
const pendingMessage = ref<{ clientMessageId: string; body: string } | null>(null)
const quoteOpen = ref(false)
const quoteSaving = ref(false)
const legacyDrafts = ref<LegacyDraft[]>([])
const legacyDraftError = ref('')
const form = reactive<MarketplaceListingInput>({ kind: 'service', title: '', summary: '', category: '代码与自动化', price_text: '', delivery_text: '', tags: [] })
const quoteForm = reactive<MarketplaceOrderQuoteInput>({ scope_text: '', amount_text: '', delivery_text: '', revision_limit: 0 })
const tagText = ref('')
let listingRequestId = 0
let myListingsRequestId = 0
let inquiryListRequestId = 0
let messageRequestId = 0
let actionRequestId = 0
let sendRequestId = 0
let quoteRequestId = 0
const publishedMine = computed(() => myListings.value.filter(item => item.status === 'published'))
const isSelectedOwner = computed(() => selected.value?.owner_user_id === currentUserId.value)
const isActiveInquiryOwner = computed(() => activeInquiry.value?.listing_owner_user_id === currentUserId.value)
const canLoadMore = computed(() => view.value === 'browse' ? nextListingCursor.value !== null : nextMineCursor.value !== null)

const ownerName = (listing: MarketplaceListing) => listing.owner_display_name?.trim() || `用户 #${listing.owner_user_id}`
const toMessage = (value: unknown, fallback: string) => extractActionableApiErrorMessage(value, fallback)
const listingQuery = (cursor?: number) => ({ kind: kind.value, category: category.value === '全部领域' ? undefined : category.value, tag: tag.value.trim() || undefined, cursor, limit: 20 })

async function loadListings(append = false): Promise<void> {
  const requestId = ++listingRequestId
  loading.value = true; error.value = ''
  try {
    const page = await marketplaceAPI.listListings(listingQuery(append ? nextListingCursor.value ?? undefined : undefined))
    if (requestId !== listingRequestId) return
    listings.value = append ? [...listings.value, ...page.items] : page.items
    nextListingCursor.value = page.next_cursor ?? null
  } catch (value) {
    if (requestId === listingRequestId) error.value = toMessage(value, '市场内容暂时无法读取，请重试。')
  } finally { if (requestId === listingRequestId) loading.value = false }
}
async function loadMyListings(append = false): Promise<void> {
  const requestId = ++myListingsRequestId
  const userId = currentUserId.value
  loading.value = true; error.value = ''
  try {
    const page = await marketplaceAPI.listMyListings(append ? nextMineCursor.value ?? undefined : undefined)
    if (requestId !== myListingsRequestId || userId !== currentUserId.value) return
    myListings.value = append ? [...myListings.value, ...page.items] : page.items
    nextMineCursor.value = page.next_cursor ?? null
  } catch (value) {
    if (requestId === myListingsRequestId && userId === currentUserId.value) error.value = toMessage(value, '我的发布暂时无法读取，请重试。')
  } finally { if (requestId === myListingsRequestId && userId === currentUserId.value) loading.value = false }
}
async function loadInquiries(append = false): Promise<void> {
  const requestId = ++inquiryListRequestId
  const userId = currentUserId.value
  loading.value = true; error.value = ''
  try {
    const page = await marketplaceAPI.listInquiries(append ? nextInquiryCursor.value ?? undefined : undefined)
    if (requestId !== inquiryListRequestId || userId !== currentUserId.value) return
    inquiries.value = append ? [...inquiries.value, ...page.items] : page.items
    nextInquiryCursor.value = page.next_cursor ?? null
    if (!activeInquiry.value && inquiries.value[0]) await openInquiry(inquiries.value[0])
  } catch (value) {
    if (requestId === inquiryListRequestId && userId === currentUserId.value) error.value = toMessage(value, '私聊列表暂时无法读取，请重试。')
  } finally { if (requestId === inquiryListRequestId && userId === currentUserId.value) loading.value = false }
}
async function loadMessages(append = false): Promise<void> {
  if (!activeInquiry.value) return
  const requestId = ++messageRequestId
  const userId = currentUserId.value
  const inquiryId = activeInquiry.value.id
  loading.value = true; error.value = ''
  try {
    const page = await marketplaceAPI.listMessages(inquiryId, append ? nextMessageCursor.value ?? undefined : undefined)
    if (requestId !== messageRequestId || userId !== currentUserId.value || inquiryId !== activeInquiry.value?.id) return
    const chronological = [...page.items].reverse()
    messages.value = append ? [...chronological, ...messages.value] : chronological
    nextMessageCursor.value = page.next_cursor ?? null
  } catch (value) {
    if (requestId === messageRequestId && userId === currentUserId.value && inquiryId === activeInquiry.value?.id) error.value = toMessage(value, '消息暂时无法读取，请重试。')
  } finally { if (requestId === messageRequestId && userId === currentUserId.value && inquiryId === activeInquiry.value?.id) loading.value = false }
}
async function openInquiry(inquiry: MarketplaceInquiry): Promise<void> {
  messageRequestId++; sendRequestId++
  activeInquiry.value = inquiry; messages.value = []; nextMessageCursor.value = null; pendingMessage.value = null; messageBody.value = ''; sending.value = false
  quoteOpen.value = false; quoteSaving.value = false; Object.assign(quoteForm, { scope_text: '', amount_text: '', delivery_text: '', revision_limit: 0 })
  await loadMessages()
}
async function startInquiry(listing: MarketplaceListing): Promise<void> {
  if (saving.value || listing.owner_user_id === currentUserId.value) return
  const requestId = ++actionRequestId
  const userId = currentUserId.value
  saving.value = true; error.value = ''
  try {
    const inquiry = await marketplaceAPI.createInquiry(listing.id)
    if (requestId !== actionRequestId || userId !== currentUserId.value) return
    inquiries.value = [inquiry, ...inquiries.value.filter(item => item.id !== inquiry.id)]
    selected.value = null; view.value = 'messages'; await openInquiry(inquiry)
  } catch (value) {
    if (requestId === actionRequestId && userId === currentUserId.value) error.value = toMessage(value, '无法发起私聊，请重试。')
  } finally { if (requestId === actionRequestId && userId === currentUserId.value) saving.value = false }
}
function resetForm(): void {
  editingId.value = null
  Object.assign(form, { kind: kind.value, title: '', summary: '', category: '代码与自动化', price_text: '', delivery_text: '', tags: [] })
  delete form.canonical_asset_id
  tagText.value = ''
}
function openCreate(): void { resetForm(); editorOpen.value = true }
function openEdit(listing: MarketplaceListing): void {
  editingId.value = listing.id
  Object.assign(form, { kind: listing.kind, title: listing.title, summary: listing.summary, category: listing.category, price_text: listing.price_text, delivery_text: listing.delivery_text, tags: [...listing.tags] })
  if (listing.canonical_asset_id) form.canonical_asset_id = listing.canonical_asset_id; else delete form.canonical_asset_id
  tagText.value = listing.tags.join('，'); selected.value = null; editorOpen.value = true
}
function importLegacyDraft(draft: LegacyDraft): void {
  resetForm(); Object.assign(form, draft, { tags: [...draft.tags] }); tagText.value = draft.tags.join('，'); editorOpen.value = true
  notice.value = '旧草稿已填入发布表单，请确认后再公开发布。原本地记录未删除。'
}
async function saveListing(): Promise<void> {
  if (saving.value) return
  form.tags = tagText.value.split(/[,，]/).map(item => item.trim()).filter(Boolean)
  const requestId = ++actionRequestId
  const userId = currentUserId.value
  saving.value = true; error.value = ''; notice.value = ''
  const wasEditing = editingId.value !== null
  try {
    const saved = editingId.value ? await marketplaceAPI.updateListing(editingId.value, { ...form }) : await marketplaceAPI.createListing({ ...form })
    if (requestId !== actionRequestId || userId !== currentUserId.value) return
    myListings.value = [saved, ...myListings.value.filter(item => item.id !== saved.id)]
    editorOpen.value = false; view.value = 'mine'; notice.value = wasEditing ? '发布已更新。' : '发布已公开，市场中的其他用户现在可以看到。'; resetForm()
  } catch (value) {
    if (requestId === actionRequestId && userId === currentUserId.value) error.value = toMessage(value, wasEditing ? '更新失败，请重试。' : '发布失败，请重试。')
  } finally { if (requestId === actionRequestId && userId === currentUserId.value) saving.value = false }
}
async function archiveListing(listing: MarketplaceListing): Promise<void> {
  if (saving.value) return
  const requestId = ++actionRequestId
  const userId = currentUserId.value
  saving.value = true; error.value = ''
  try {
    const archived = await marketplaceAPI.archiveListing(listing.id)
    if (requestId !== actionRequestId || userId !== currentUserId.value) return
    myListings.value = myListings.value.map(item => item.id === archived.id ? archived : item)
    selected.value = null; notice.value = '发布已归档，不再出现在公开市场。'
  } catch (value) {
    if (requestId === actionRequestId && userId === currentUserId.value) error.value = toMessage(value, '归档失败，请重试。')
  } finally { if (requestId === actionRequestId && userId === currentUserId.value) saving.value = false }
}
const nextClientMessageId = () => globalThis.crypto?.randomUUID?.() ?? `market-${Date.now()}-${Math.random().toString(36).slice(2)}`
async function sendMessage(): Promise<void> {
  if (sending.value || !activeInquiry.value || !messageBody.value.trim()) return
  const requestId = ++sendRequestId
  const userId = currentUserId.value
  const inquiryId = activeInquiry.value.id
  const body = messageBody.value.trim()
  const retry = pendingMessage.value?.body === body ? pendingMessage.value : { clientMessageId: nextClientMessageId(), body }
  pendingMessage.value = retry; sending.value = true; error.value = ''
  try {
    const sent = await marketplaceAPI.sendMessage(inquiryId, { client_message_id: retry.clientMessageId, body: retry.body })
    if (requestId !== sendRequestId || userId !== currentUserId.value || inquiryId !== activeInquiry.value?.id) return
    if (!messages.value.some(item => item.id === sent.id)) messages.value.push(sent)
    messageBody.value = ''; pendingMessage.value = null
  } catch (value) {
    if (requestId === sendRequestId && userId === currentUserId.value && inquiryId === activeInquiry.value?.id) error.value = toMessage(value, '消息发送失败，内容已保留，可直接重试。')
  } finally { if (requestId === sendRequestId && userId === currentUserId.value && inquiryId === activeInquiry.value?.id) sending.value = false }
}

async function submitQuote(): Promise<void> {
  if (!activeInquiry.value || !isActiveInquiryOwner.value || quoteSaving.value || !quoteForm.scope_text.trim()) return
  const requestId = ++quoteRequestId
  const userId = currentUserId.value
  const inquiryId = activeInquiry.value.id
  quoteSaving.value = true; error.value = ''; notice.value = ''
  try {
    await marketplaceAPI.quoteOrder(inquiryId, { ...quoteForm, scope_text: quoteForm.scope_text.trim(), amount_text: quoteForm.amount_text.trim(), delivery_text: quoteForm.delivery_text.trim() })
    if (requestId !== quoteRequestId || userId !== currentUserId.value || inquiryId !== activeInquiry.value?.id) return
    quoteOpen.value = false
    Object.assign(quoteForm, { scope_text: '', amount_text: '', delivery_text: '', revision_limit: 0 })
    notice.value = '报价已记录，买卖双方可在订单页查看后续状态。'
  } catch (value) {
    if (requestId === quoteRequestId && userId === currentUserId.value && inquiryId === activeInquiry.value?.id) error.value = toMessage(value, '报价提交失败，请重试。')
  } finally {
    if (requestId === quoteRequestId && userId === currentUserId.value && inquiryId === activeInquiry.value?.id) quoteSaving.value = false
  }
}
function parseLegacyDraft(value: unknown): LegacyDraft | null {
  if (!value || typeof value !== 'object') return null
  const item = value as Record<string, unknown>
  if (typeof item.id !== 'string' || typeof item.title !== 'string' || typeof item.summary !== 'string' || typeof item.category !== 'string' || !['service', 'talent', 'demand'].includes(String(item.kind)) || !Array.isArray(item.tags) || !item.tags.every(tagValue => typeof tagValue === 'string')) return null
  return { id: item.id, kind: item.kind as MarketplaceListingKind, title: item.title, summary: item.summary, category: item.category, price_text: String(item.price_text ?? item.price ?? ''), delivery_text: String(item.delivery_text ?? item.delivery ?? ''), tags: item.tags as string[] }
}
function readLegacyDrafts(): void {
  const keys = new Set([`market-drafts-v1-${auth.user?.id ?? 'local'}`, 'market-drafts-v1-local'])
  const found: LegacyDraft[] = []
  for (const key of keys) {
    const raw = localStorage.getItem(key); if (!raw) continue
    try {
      const parsed: unknown = JSON.parse(raw); if (!Array.isArray(parsed)) throw new Error('invalid')
      const normalized = parsed.map(parseLegacyDraft); if (normalized.some(item => item === null)) throw new Error('invalid')
      for (const item of normalized) if (item && !found.some(draft => draft.id === item.id)) found.push(item)
    } catch { legacyDraftError.value = '检测到无法读取的旧市场草稿；原始数据已保留，未自动删除或发布。' }
  }
  legacyDrafts.value = found
}
async function switchView(nextView: MarketView): Promise<void> {
  view.value = nextView; selected.value = null
  if (nextView === 'mine') await loadMyListings()
  if (nextView === 'messages') await loadInquiries()
}
watch([kind, category], () => { if (view.value === 'browse') void loadListings() })
watch(currentUserId, () => {
  guideExpanded.value = false
  myListingsRequestId++; inquiryListRequestId++; messageRequestId++; actionRequestId++; sendRequestId++; quoteRequestId++
  myListings.value = []; inquiries.value = []; messages.value = []; activeInquiry.value = null; selected.value = null
  editorOpen.value = false; messageBody.value = ''; pendingMessage.value = null; loading.value = false; saving.value = false; sending.value = false; quoteOpen.value = false; quoteSaving.value = false; error.value = ''; notice.value = ''; legacyDrafts.value = []; legacyDraftError.value = ''; resetForm(); Object.assign(quoteForm, { scope_text: '', amount_text: '', delivery_text: '', revision_limit: 0 })
  readLegacyDrafts()
  if (view.value === 'mine') void loadMyListings()
  if (view.value === 'messages') void loadInquiries()
})
onMounted(() => { readLegacyDrafts(); void loadListings() })
</script>

<template>
  <section class="market-experience">
    <header class="market-head"><div><h1>双边市场</h1><p class="market-subtitle">让需求遇见能力，从一次真实合作开始。</p></div><div class="market-actions"><button class="market-button" type="button" data-testid="market-guide-toggle" :aria-expanded="showGuide" @click="guideExpanded = !showGuide; guide.load(true)"><span v-if="guide.pending.value" class="market-guide-dot" aria-hidden="true"></span>新手指南</button><button class="market-button" type="button" data-testid="market-open-messages" @click="switchView('messages')"><MessageSquare :size="16" />私聊</button><button class="market-button primary" type="button" data-testid="market-open-create" @click="openCreate"><Plus :size="16" />发布</button></div></header>
    <section v-if="showGuide" class="market-guide" data-testid="market-guide" aria-label="双边市场新手指南">
      <div class="market-section-head"><div><p class="market-meta">你的第一步</p><h2>这次，你想完成什么？</h2></div><button class="market-detail-link" type="button" :disabled="guide.state.value.saving || guide.state.value.loading" @click="chooseGuide('browse')">先逛逛，跳过引导</button></div>
      <div class="market-guide-options">
        <button type="button" data-testid="market-guide-hire" :disabled="guide.state.value.saving || guide.state.value.loading" @click="chooseGuide('hire')"><Search :size="22" /><strong>我要找人做事</strong><span>写下目标、预算和交付标准，让合适的人找到你。</span><b>整理我的需求 →</b></button>
        <button type="button" data-testid="market-guide-sell" :disabled="guide.state.value.saving || guide.state.value.loading" @click="chooseGuide('sell')"><BriefcaseBusiness :size="22" /><strong>我想接单</strong><span>展示技能、作品和服务范围，开始第一场合作。</span><b>发布我的服务 →</b></button>
        <button type="button" data-testid="market-guide-browse" :disabled="guide.state.value.saving || guide.state.value.loading" @click="chooseGuide('browse')"><Users :size="22" /><strong>先看看有什么</strong><span>浏览真实服务和需求，不必先发布或充值。</span><b>自由逛逛 →</b></button>
      </div>
      <p class="market-guide-flow">先沟通范围 → 确认报价 → 按订单约定交付与验收。选择方向只打开表单，不会自动发布或扣款。</p>
    </section>
    <p v-if="guide.state.value.error" class="market-notice" role="status">{{ guide.state.value.error }} <button type="button" class="market-detail-link" @click="guide.load(true)">重试同步</button></p>
    <p v-if="error" class="market-notice error" role="alert">{{ error }}</p><p v-else-if="notice || legacyDraftError" class="market-notice" role="status">{{ notice || legacyDraftError }}</p>
    <form v-if="editorOpen" class="market-editor" data-testid="market-editor" @submit.prevent="saveListing">
      <div class="market-section-head"><div><p class="market-meta">{{ editingId ? '管理发布' : '自助发布' }}</p><h2>{{ editingId ? '编辑市场内容' : '公开一条真实内容' }}</h2></div><button type="button" class="market-icon" aria-label="关闭发布表单" @click="editorOpen = false"><X :size="18" /></button></div>
      <div class="market-form-grid"><label>类型<select v-model="form.kind"><option value="service">提供服务</option><option value="talent">人才名片</option><option value="demand">发布需求</option></select></label><label>领域<select v-model="form.category"><option v-for="item in categories.slice(1)" :key="item">{{ item }}</option></select></label></div>
      <label>标题<input v-model="form.title" data-testid="market-title" required maxlength="100" placeholder="清楚说明你能交付什么，或需要什么" /></label><label>内容与交付标准<textarea v-model="form.summary" data-testid="market-summary" required rows="4" maxlength="5000" placeholder="范围、边界、验收方式和必要背景" /></label>
      <div class="market-form-grid"><label>价格或预算<input v-model="form.price_text" maxlength="80" placeholder="例如：按项目沟通" /></label><label>交付周期<input v-model="form.delivery_text" maxlength="80" placeholder="例如：5 个工作日" /></label></div><label>标签<input v-model="tagText" maxlength="200" placeholder="用逗号分隔" /></label>
      <div class="market-editor-actions"><span>点击发布后将立即公开，不会生成订单或付款。</span><button class="market-button primary" data-testid="market-save" type="submit" :disabled="saving"><Send :size="16" />{{ saving ? '提交中…' : editingId ? '保存更新' : '确认公开发布' }}</button></div>
    </form>
    <section v-if="legacyDrafts.length" class="legacy-drafts" aria-label="旧本地草稿"><div><strong>旧本地草稿</strong><span>仅供预览，必须手动确认后才会公开。</span></div><button v-for="draft in legacyDrafts" :key="draft.id" type="button" @click="importLegacyDraft(draft)">{{ draft.title }}</button></section>
    <nav class="market-tabs" aria-label="市场栏目"><button v-for="tabItem in tabs" :key="tabItem.key" type="button" :class="{ active: view === 'browse' && kind === tabItem.key }" @click="view = 'browse'; kind = tabItem.key"><component :is="tabItem.icon" :size="16" />{{ tabItem.label }}</button><button type="button" :class="{ active: view === 'mine' }" @click="switchView('mine')"><Users :size="16" />我的发布 <span>{{ publishedMine.length }}</span></button><button type="button" :class="{ active: view === 'orders' }" @click="switchView('orders')"><ClipboardList :size="16" />订单</button><button v-if="auth.isAdmin" type="button" :class="{ active: view === 'disputes' }" @click="switchView('disputes')"><ClipboardList :size="16" />争议管理</button></nav>
    <template v-if="view === 'browse' || view === 'mine'">
      <form v-if="view === 'browse'" class="market-toolbar" @submit.prevent="loadListings()"><label>领域<select v-model="category" aria-label="筛选领域"><option v-for="item in categories" :key="item">{{ item }}</option></select></label><label class="market-search"><Search :size="17" /><input v-model="tag" type="search" aria-label="按标签筛选" placeholder="按标签筛选" /></label><button class="market-button" type="submit"><Search :size="16" />筛选</button><button class="market-icon" type="button" aria-label="刷新市场" :disabled="loading" @click="loadListings()"><RefreshCw :size="17" /></button></form>
      <div class="market-section-head"><span class="market-meta">{{ view === 'mine' ? `${myListings.length} 条个人发布` : `${listings.length} 条公开结果` }}</span><span class="market-meta">公开昵称来自用户身份资料</span></div>
      <div class="market-results" :class="{ 'with-detail': selected }"><div class="market-list">
        <article v-for="item in view === 'mine' ? myListings : listings" :key="item.id" class="market-item" :class="{ archived: item.status !== 'published' }"><div class="market-item-top"><span class="market-category">{{ item.category }}</span><span v-if="item.status !== 'published'" class="market-status">{{ item.status === 'archived' ? '已归档' : '已下架' }}</span></div><button class="market-item-title" type="button" @click="selected = item">{{ item.title }}</button><p>{{ item.summary }}</p><div class="market-tags"><span v-for="itemTag in item.tags" :key="itemTag">{{ itemTag }}</span></div><footer><span>{{ ownerName(item) }}</span><strong>{{ item.price_text || '待沟通' }}</strong></footer><button class="market-detail-link" type="button" @click="selected = item">查看详情</button></article>
        <div v-if="!loading && !(view === 'mine' ? myListings : listings).length" class="market-empty"><Search :size="28" /><h2>{{ view === 'mine' ? '还没有发布内容' : '没有符合条件的公开内容' }}</h2><p>调整筛选，或公开一条真实服务、人才名片或需求。</p><button class="market-button" type="button" @click="openCreate">新建发布</button></div><button v-if="canLoadMore" class="market-button load-more" type="button" :disabled="loading" @click="view === 'browse' ? loadListings(true) : loadMyListings(true)">加载更多</button>
      </div><aside v-if="selected" class="market-detail"><div class="market-section-head"><span class="market-category">{{ selected.kind === 'demand' ? '需求' : selected.kind === 'talent' ? '人才' : '服务' }}</span><button class="market-icon" aria-label="关闭详情" @click="selected = null"><X :size="18" /></button></div><h2>{{ selected.title }}</h2><p>{{ selected.summary }}</p><dl><dt>发布者</dt><dd>{{ ownerName(selected) }}</dd><dt>价格或预算</dt><dd>{{ selected.price_text || '待沟通' }}</dd><dt>交付周期</dt><dd>{{ selected.delivery_text || '待约定' }}</dd></dl><div v-if="isSelectedOwner && selected.status === 'published'" class="market-detail-actions"><button class="market-button" type="button" @click="openEdit(selected)"><Edit3 :size="16" />编辑</button><button class="market-button danger" type="button" :disabled="saving" @click="archiveListing(selected)"><Archive :size="16" />归档</button></div><button v-else-if="!isSelectedOwner && selected.status === 'published'" class="market-button primary" data-testid="market-start-inquiry" type="button" :disabled="saving" @click="startInquiry(selected)"><MessageSquare :size="16" />发起私聊</button><p class="market-boundary">私聊只用于确认合作范围。当前不创建订单，也不处理付款。</p></aside></div>
    </template>
    <MarketplaceOrdersPanel v-else-if="view === 'orders'" />
    <MarketplaceDisputesPanel v-else-if="view === 'disputes' && auth.isAdmin" />
    <section v-else class="market-inbox"><aside><div class="market-section-head"><h2>私聊</h2><button class="market-icon" type="button" aria-label="刷新私聊" @click="loadInquiries()"><RefreshCw :size="17" /></button></div><button v-for="inquiry in inquiries" :key="inquiry.id" class="market-thread" :class="{ active: activeInquiry?.id === inquiry.id }" type="button" @click="openInquiry(inquiry)"><strong>{{ inquiry.listing.title }}</strong><small>{{ ownerName(inquiry.listing) }} · {{ inquiry.last_message_at ? '已有消息' : '等待沟通' }}</small></button><p v-if="!loading && !inquiries.length" class="market-meta">从公开内容详情发起私聊后，线程会出现在这里。</p><button v-if="nextInquiryCursor" class="market-detail-link" type="button" @click="loadInquiries(true)">加载更多私聊</button></aside>
      <div v-if="activeInquiry" class="market-conversation"><header><p class="market-meta">私密线程 #{{ activeInquiry.id }}</p><h2>{{ activeInquiry.listing.title }}</h2><button v-if="isActiveInquiryOwner" class="market-button" type="button" data-testid="market-order-quote-toggle" @click="quoteOpen = !quoteOpen"><ClipboardList :size="15" />发起报价</button></header><form v-if="quoteOpen && isActiveInquiryOwner" class="market-quote-form" data-testid="market-order-quote-form" @submit.prevent="submitQuote"><label>合作范围<textarea v-model="quoteForm.scope_text" required rows="3" maxlength="2000" placeholder="交付内容、边界和验收标准" /></label><div><label>金额约定<input v-model="quoteForm.amount_text" maxlength="120" placeholder="文字约定，不执行扣款" /></label><label>交付周期<input v-model="quoteForm.delivery_text" maxlength="240" placeholder="例如 3 天" /></label><label>修改次数<input v-model.number="quoteForm.revision_limit" type="number" min="0" max="10" /></label></div><button class="market-button primary" type="submit" :disabled="quoteSaving || !quoteForm.scope_text.trim()"><ClipboardList :size="15" />{{ quoteSaving ? '提交中…' : '生成订单报价' }}</button></form><p class="market-boundary compact">私聊用于确认范围；订单状态由双方在“我的订单”中单独推进，当前不处理付款。</p><button v-if="nextMessageCursor" class="market-detail-link" type="button" @click="loadMessages(true)">查看更早消息</button><div class="market-messages" aria-live="polite"><p v-for="entry in messages" :key="entry.id" :class="{ mine: entry.sender_user_id === currentUserId }"><span>{{ entry.body }}</span><small>{{ entry.sender_user_id === currentUserId ? '我' : entry.sender_user_id === activeInquiry.listing_owner_user_id ? ownerName(activeInquiry.listing) : `用户 #${entry.sender_user_id}` }}</small></p><div v-if="!loading && !messages.length" class="market-empty compact"><MessageSquare :size="24" /><p>还没有消息，可以先确认目标、范围和交付标准。</p></div></div><form class="market-message-form" @submit.prevent="sendMessage"><textarea v-model="messageBody" data-testid="market-message-body" rows="3" maxlength="4000" placeholder="输入私聊消息" /><div><span v-if="pendingMessage">上次发送失败，将使用同一消息编号重试。</span><button class="market-button primary" data-testid="market-send-message" type="submit" :disabled="sending || !messageBody.trim()"><Send :size="16" />{{ sending ? '发送中…' : pendingMessage ? '重试发送' : '发送' }}</button></div></form></div>
      <div v-else class="market-empty"><MessageSquare :size="28" /><h2>选择一条私聊</h2><p>这里不会显示虚构回复或未发送的报价。</p></div>
    </section>
  </section>
</template>

<style scoped>
.market-guide{display:grid;gap:16px;padding:20px;border:1px solid var(--zc-line);border-radius:12px;background:linear-gradient(135deg,color-mix(in srgb,var(--zc-accent) 7%,var(--zc-surface)),var(--zc-surface))}
.market-guide-options{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:12px}
.market-guide-options button{display:grid;gap:9px;text-align:left;border:1px solid var(--zc-line);border-radius:8px;background:var(--zc-surface);color:var(--zc-text);padding:18px;cursor:pointer}
.market-guide-options button:hover{border-color:var(--zc-accent)}.market-guide-options button:focus-visible{outline:2px solid var(--zc-accent);outline-offset:3px}.market-guide-options button:disabled{opacity:.6;cursor:wait}
.market-guide-options svg,.market-guide-options b{color:var(--zc-accent)}.market-guide-options strong{font-size:15px}.market-guide-options span,.market-guide-flow{font-size:12px;line-height:1.7;color:var(--zc-muted)}.market-guide-options b{font-size:12px}.market-guide-flow{margin:0}
.market-guide-dot{width:7px;height:7px;border-radius:50%;background:var(--zc-accent)}
@media(max-width:680px){.market-guide-options{grid-template-columns:1fr}.market-guide{padding:14px}.market-actions{flex-wrap:wrap}}
.market-experience{color:var(--zc-text);display:grid;gap:16px}.market-head,.market-section-head,.market-item-top,.market-editor-actions,.market-message-form>div{display:flex;align-items:center;justify-content:space-between;gap:12px}.market-head{border-bottom:1px solid var(--zc-line);padding-bottom:16px}h1{margin:2px 0 0;color:var(--zc-text-strong);font-size:1.5rem}h2{margin:0;color:var(--zc-text-strong);font-size:1rem}.market-subtitle{margin:6px 0 0;color:var(--zc-muted);font-size:.8125rem}.market-meta{margin:0;color:var(--zc-muted);font-size:.75rem}.market-actions,.market-detail-actions{display:flex;gap:8px}.market-button,.market-icon,.market-tabs button,.market-detail-link,.market-thread{border:1px solid var(--zc-line);border-radius:6px;background:var(--zc-surface);color:var(--zc-text);cursor:pointer;font:inherit}.market-button{min-height:36px;display:inline-flex;align-items:center;justify-content:center;gap:7px;padding:0 12px;font-size:.8125rem;font-weight:650}.market-button.primary{border-color:var(--zc-accent);background:var(--zc-accent);color:var(--zc-accent-contrast,#fff)}.market-button.danger{color:var(--zc-danger)}.market-button:disabled,.market-icon:disabled{cursor:wait;opacity:.6}.market-icon{width:36px;height:36px;display:inline-grid;place-items:center;padding:0}.market-notice{margin:0;border-left:3px solid var(--zc-accent);background:color-mix(in srgb,var(--zc-accent) 8%,transparent);padding:10px 12px;font-size:.8125rem}.market-notice.error{border-color:var(--zc-danger);color:var(--zc-danger)}.market-editor,.legacy-drafts{border:1px solid var(--zc-line);border-radius:8px;background:var(--zc-surface);padding:16px}.market-editor{display:grid;gap:12px}.market-editor label,.market-toolbar label,.market-quote-form label{display:grid;gap:6px;color:var(--zc-text-strong);font-size:.75rem;font-weight:650}.market-editor input,.market-editor textarea,.market-editor select,.market-toolbar input,.market-toolbar select,.market-message-form textarea,.market-quote-form input,.market-quote-form textarea{width:100%;border:1px solid var(--zc-line);border-radius:6px;background:var(--zc-bg);color:var(--zc-text);padding:9px 10px;font:inherit;outline:none}.market-editor :is(input,textarea,select):focus,.market-toolbar :is(input,select):focus,.market-message-form textarea:focus,.market-quote-form :is(input,textarea):focus{border-color:var(--zc-accent);box-shadow:0 0 0 2px color-mix(in srgb,var(--zc-accent) 20%,transparent)}.market-form-grid{display:grid;grid-template-columns:1fr 1fr;gap:12px}.market-editor-actions span{color:var(--zc-muted);font-size:.75rem}.legacy-drafts{display:flex;align-items:center;gap:8px;overflow-x:auto}.legacy-drafts>div{min-width:190px;display:grid}.legacy-drafts span{color:var(--zc-muted);font-size:.6875rem}.legacy-drafts button{white-space:nowrap;border:1px solid var(--zc-line);border-radius:6px;background:var(--zc-bg);color:var(--zc-text);padding:8px 10px}.market-tabs{display:flex;gap:4px;overflow-x:auto;border-bottom:1px solid var(--zc-line)}.market-tabs button{min-height:38px;display:inline-flex;align-items:center;gap:7px;border:0;border-bottom:2px solid transparent;border-radius:0;padding:0 12px;white-space:nowrap;background:transparent}.market-tabs button.active{border-bottom-color:var(--zc-accent);color:var(--zc-text-strong)}.market-tabs span,.market-tags span{border-radius:999px;background:var(--zc-bg);padding:2px 7px;font-size:.6875rem}.market-toolbar{display:grid;grid-template-columns:180px minmax(220px,1fr) auto auto;gap:8px;align-items:end}.market-search{position:relative}.market-search svg{position:absolute;left:10px;bottom:10px;color:var(--zc-muted)}.market-search input{padding-left:34px}.market-results{display:grid;grid-template-columns:1fr;gap:16px}.market-results.with-detail{grid-template-columns:minmax(0,1fr) minmax(280px,34%)}.market-list{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:10px;align-content:start}.market-item{min-width:0;display:grid;gap:10px;border:1px solid var(--zc-line);border-radius:8px;background:var(--zc-surface);padding:14px}.market-item.archived{opacity:.66}.market-category,.market-status{color:var(--zc-accent);font-size:.6875rem;font-weight:700}.market-status{color:var(--zc-muted)}.market-item-title{border:0;background:transparent;color:var(--zc-text-strong);padding:0;text-align:left;font:750 .9375rem/1.35 inherit;cursor:pointer}.market-item p,.market-detail p{margin:0;color:var(--zc-muted);font-size:.8125rem;line-height:1.6}.market-tags{display:flex;flex-wrap:wrap;gap:5px}.market-item footer{display:flex;justify-content:space-between;gap:8px;border-top:1px solid var(--zc-line);padding-top:10px;font-size:.75rem}.market-item footer span{color:var(--zc-muted);overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.market-item footer strong{color:var(--zc-text-strong);white-space:nowrap}.market-detail-link{width:fit-content;border:0;background:transparent;color:var(--zc-accent);padding:0;font-size:.75rem}.market-detail{position:sticky;top:72px;align-self:start;display:grid;gap:14px;border:1px solid var(--zc-line);border-radius:8px;background:var(--zc-surface);padding:16px}.market-detail dl{display:grid;grid-template-columns:90px 1fr;gap:8px;margin:0;font-size:.75rem}.market-detail dt{color:var(--zc-muted)}.market-detail dd{margin:0;color:var(--zc-text-strong)}.market-boundary{border-top:1px solid var(--zc-line);padding-top:12px}.load-more{grid-column:1/-1;justify-self:center}.market-empty{grid-column:1/-1;min-height:180px;display:grid;place-items:center;align-content:center;gap:8px;border:1px dashed var(--zc-line);border-radius:8px;color:var(--zc-muted);text-align:center;padding:20px}.market-empty.compact{min-height:120px}.market-empty p{margin:0;font-size:.8125rem}.market-inbox{min-height:480px;display:grid;grid-template-columns:260px minmax(0,1fr);border:1px solid var(--zc-line);border-radius:8px;overflow:hidden;background:var(--zc-surface)}.market-inbox>aside{display:grid;align-content:start;gap:6px;border-right:1px solid var(--zc-line);padding:12px}.market-thread{display:grid;gap:3px;padding:10px;text-align:left}.market-thread.active{border-color:var(--zc-accent);background:color-mix(in srgb,var(--zc-accent) 8%,var(--zc-surface))}.market-thread strong{overflow:hidden;text-overflow:ellipsis;white-space:nowrap;font-size:.8125rem}.market-thread small{color:var(--zc-muted);font-size:.6875rem}.market-conversation{min-width:0;display:flex;flex-direction:column;gap:10px;padding:16px}.market-conversation>header{display:flex;align-items:flex-start;justify-content:space-between;gap:10px}.market-conversation>header h2{margin-top:3px}.market-quote-form{display:grid;gap:10px;border-top:1px solid var(--zc-line);border-bottom:1px solid var(--zc-line);padding:12px 0}.market-quote-form>div{display:grid;grid-template-columns:2fr 1fr 100px;gap:8px}.market-quote-form>.market-button{justify-self:end}.market-messages{min-height:240px;flex:1;display:flex;flex-direction:column;gap:8px;overflow-y:auto;padding:4px}.market-messages>p{max-width:76%;display:grid;gap:3px;align-self:flex-start;margin:0}.market-messages>p.mine{align-self:flex-end;text-align:right}.market-messages span{border:1px solid var(--zc-line);border-radius:8px;background:var(--zc-bg);padding:9px 11px;color:var(--zc-text);font-size:.8125rem;white-space:pre-wrap;overflow-wrap:anywhere}.market-messages .mine span{border-color:var(--zc-accent)}.market-messages small{color:var(--zc-muted);font-size:.6875rem}.market-message-form{display:grid;gap:8px;border-top:1px solid var(--zc-line);padding-top:12px}.market-message-form span{color:var(--zc-danger);font-size:.6875rem}
@media(max-width:900px){.market-results.with-detail{grid-template-columns:1fr}.market-detail{position:static}.market-toolbar{grid-template-columns:1fr 1fr auto auto}}@media(max-width:680px){.market-head,.market-editor-actions{align-items:flex-start;flex-direction:column}.market-actions,.market-editor-actions .market-button{width:100%}.market-actions .market-button{flex:1}.market-form-grid,.market-list,.market-toolbar,.market-inbox,.market-quote-form>div{grid-template-columns:1fr}.market-inbox>aside{border-right:0;border-bottom:1px solid var(--zc-line)}.market-inbox{min-height:0}.market-conversation{min-height:460px}.market-toolbar .market-icon{width:100%}.market-messages>p{max-width:90%}}
</style>
