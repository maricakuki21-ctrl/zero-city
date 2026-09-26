<script setup lang="ts">
import { computed, onScopeDispose, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ArrowLeft, BookOpen, LockKeyhole, Pencil, Plus, RefreshCw, Save, Send, Settings, ShoppingCart, X } from '@lucide/vue'
import { useAuthStore } from '@/stores/auth'
import { extractActionableApiErrorMessage } from '@/utils/apiError'
import { columnsAPI, type ArticleInput, type ColumnArticle, type CreatorColumn, type ColumnCommercePolicy, type ColumnPurchase } from '@/features/bizdecipher/api/columns'

const auth = useAuthStore()
const route = useRoute()
const router = useRouter()
const admin = computed(() => auth.user?.role === 'admin')
const mode = ref<'browse' | 'mine' | 'admin'>('browse')
const columns = ref<CreatorColumn[]>([])
const column = ref<CreatorColumn | null>(null)
const articles = ref<ColumnArticle[]>([])
const article = ref<ColumnArticle | null>(null)
const cursor = ref<number | null>(null)
const articleCursor = ref<number | null>(null)
const loading = ref(false)
const busy = ref(false)
const error = ref('')
const notice = ref('')
const columnEditor = ref(false)
const creating = ref(false)
const writing = ref(false)
const editingID = ref<number | null>(null)
const title = ref('')
const description = ref('')
const reason = ref('')
const policy = ref<ColumnCommercePolicy>({ enabled: false, currency: 'USD', platform_fee: '0.00000000' })
const policyEnabled = ref(false)
const policyReason = ref('')
const pricingMode = ref<'free' | 'paid'>('free')
const price = ref('0')
const purchases = ref<ColumnPurchase[]>([])
const purchaseCursor = ref<number | null>(null)
const confirmingPurchase = ref(false)
const refundTarget = ref<ColumnPurchase | null>(null)
const refundReason = ref('')
const locked = computed(() => column.value?.mode === 'paid' && column.value.can_read !== true && !owned.value && !admin.value)
const money = (value?: string) => `${value?.replace(/(\.\d*?)0+$/, '$1').replace(/\.$/, '') || '0'} USD`
const draft = ref<ArticleInput>({ title: '', summary: '', body: '', status: 'draft' })
const owned = computed(() => column.value?.owner_user_id === auth.user?.id)
const canWrite = computed(() => owned.value && column.value?.status === 'active')
let generation = 0
let request = 0
const statusLabel = (status: ColumnArticle['status']) => ({ draft: '草稿', published: '已发布', archived: '已归档' }[status])
const date = (value: string) => Number.isNaN(Date.parse(value)) ? '' : new Date(value).toLocaleDateString('zh-CN')
function id(value: unknown): number | undefined {
  const number = Number(value)
  return Number.isSafeInteger(number) && number > 0 ? number : undefined
}

function location(columnID?: number, articleID?: number) {
  const query = { ...route.query, view: 'columns', column: columnID?.toString(), article: articleID?.toString() }
  return { path: '/community', query }
}
function navigate(columnID?: number, articleID?: number) {
  if (busy.value) return
  if ((writing.value || columnEditor.value) && !window.confirm('离开后，未保存的内容不会保留。继续吗？')) return
  writing.value = false
  columnEditor.value = false
  void router.push(location(columnID, articleID))
}
function city() {
  if (busy.value) return
  if ((writing.value || columnEditor.value) && !window.confirm('离开后，未保存的内容不会保留。继续吗？')) return
  const { view: _view, column: _column, article: _article, ...query } = route.query
  void router.push({ path: '/community', query })
}

async function load(append = false) {
  const current = ++request
  const session = generation
  loading.value = true
  error.value = ''
  try {
    const nextPolicy = await columnsAPI.policy()
    const columnID = id(route.query.column)
    if (columnID) {
      const selected = await columnsAPI.get(columnID)
      const page = await columnsAPI.articles(columnID, selected.owner_user_id === auth.user?.id, append ? articleCursor.value ?? undefined : undefined)
      const articleID = id(route.query.article)
      const entitled = selected.mode !== 'paid' || selected.can_read === true || selected.owner_user_id === auth.user?.id || admin.value
      const selectedArticle = articleID && entitled ? await columnsAPI.article(columnID, articleID) : null
      const sales = auth.user ? await columnsAPI.purchases(columnID) : { items: [] }
      if (current !== request || session !== generation) return
      policy.value = nextPolicy
      policyEnabled.value = nextPolicy.enabled
      column.value = selected
      articles.value = append ? [...articles.value, ...page.items] : page.items
      articleCursor.value = page.next_cursor ?? null
      article.value = selectedArticle
      purchases.value = sales.items
      purchaseCursor.value = sales.next_cursor ?? null
    } else {
      const next = append ? cursor.value ?? undefined : undefined
      const page = mode.value === 'admin' && admin.value ? await columnsAPI.listAdmin(next) : await columnsAPI.list(mode.value === 'mine', next)
      if (current !== request || session !== generation) return
      policy.value = nextPolicy
      policyEnabled.value = nextPolicy.enabled
      columns.value = append ? [...columns.value, ...page.items] : page.items
      cursor.value = page.next_cursor ?? null
      column.value = null
      article.value = null
      articles.value = []
    }
  } catch (value) {
    if (current === request && session === generation) error.value = extractActionableApiErrorMessage(value, '专栏暂时无法加载，请重试。')
  } finally {
    if (current === request && session === generation) loading.value = false
  }
}

async function mutate(action: () => Promise<void>) {
  if (busy.value) return
  const session = generation
  busy.value = true
  error.value = ''
  notice.value = ''
  try { await action() }
  catch (value) {
    if (session === generation) error.value = extractActionableApiErrorMessage(value, '操作未确认成功，请刷新核对后重试。')
  } finally { if (session === generation) busy.value = false }
}

function editColumn(isNew: boolean) {
  creating.value = isNew
  title.value = isNew ? '' : column.value?.title ?? ''
  description.value = isNew ? '' : column.value?.description ?? ''
  pricingMode.value = column.value?.mode ?? 'free'
  price.value = column.value?.price ?? '0'
  columnEditor.value = true
  writing.value = false
  notice.value = ''
}
async function saveColumn() {
  const session = generation
  const input = { title: title.value.trim(), description: description.value.trim() }
  if (!input.title) { error.value = '请填写专栏名称。'; return }
  await mutate(async () => {
    const result = creating.value ? await columnsAPI.create(input) : await columnsAPI.update(column.value!.id, input)
    if (session !== generation) return
    columnEditor.value = false
    notice.value = creating.value ? '专栏已开通。' : '专栏设置已保存。'
    if (id(route.query.column) !== result.id) await router.push(location(result.id))
    else await load()
  })
}
function write(existing?: ColumnArticle) {
  if (!canWrite.value) return
  editingID.value = existing?.id ?? null
  draft.value = existing ? { title: existing.title, summary: existing.summary, body: existing.body ?? '', status: existing.status } : { title: '', summary: '', body: '', status: 'draft' }
  writing.value = true
  columnEditor.value = false
  notice.value = ''
}
async function saveArticle(status: ColumnArticle['status']) {
  if (!column.value || !canWrite.value) return
  if (!draft.value.title.trim() || !draft.value.body.trim()) { error.value = '请填写文章标题和正文。'; return }
  const session = generation
  const columnID = column.value.id
  const input = { ...draft.value, title: draft.value.title.trim(), status }
  await mutate(async () => {
    const result = editingID.value
      ? await columnsAPI.updateArticle(columnID, editingID.value, input)
      : await columnsAPI.createArticle(columnID, input)
    if (session !== generation) return
    writing.value = false
    notice.value = status === 'published' ? '文章已发布。' : status === 'archived' ? '文章已归档。' : '草稿已保存。'
    if (id(route.query.article) !== result.id) await router.push(location(columnID, result.id))
    else await load()
  })
}
async function moderate() {
  if (!admin.value || !column.value || !reason.value.trim()) return
  const session = generation
  const target = column.value
  await mutate(async () => {
    await columnsAPI.moderate(target.id, target.status === 'active' ? 'suspended' : 'active', reason.value.trim())
    if (session !== generation) return
    reason.value = ''
    notice.value = '专栏状态已更新。'
    await load()
  })
}
async function savePricing() {
  if (!canWrite.value || !column.value) return
  const session = generation
  const columnID = column.value.id
  await mutate(async () => {
    await columnsAPI.setPricing(columnID, pricingMode.value, pricingMode.value === 'free' ? '0' : price.value.trim())
    if (session !== generation) return
    notice.value = '价格已保存，已有购买权限保持有效。'
    await load()
  })
}
function durableOperation(key: string, payload: string) {
  const stored = sessionStorage.getItem(key)
  if (stored) {
    const previous = JSON.parse(stored) as { operation: string; payload: string }
    if (previous.payload === payload) return previous.operation
  }
  const operation = crypto.randomUUID()
  sessionStorage.setItem(key, JSON.stringify({ operation, payload }))
  return operation
}
async function buy() {
  if (!column.value || !auth.user || !locked.value || !policy.value.enabled) return
  const session = generation
  const target = column.value
  await mutate(async () => {
    const amount = target.price ?? ''
    const key = `column-purchase:${auth.user!.id}:${target.id}`
    const result = await columnsAPI.purchase(target.id, durableOperation(key, amount), amount)
    if (session !== generation) return
    sessionStorage.removeItem(key)
    confirmingPurchase.value = false
    notice.value = result.status === 'active' ? '购买已确认，专栏已解锁。' : '此笔购买已退款，未重复扣款。'
    await load()
  })
}
async function setPolicy() {
  if (!admin.value || !policyReason.value.trim()) return
  const session = generation
  await mutate(async () => {
    const result = await columnsAPI.setPolicy(policyEnabled.value, policyReason.value.trim())
    if (session !== generation) return
    policy.value = result
    policyReason.value = ''
    notice.value = result.enabled ? '付费购买已开放。' : '新购买已关闭，已购买权限不受影响。'
  })
}
async function refund() {
  if (!admin.value || !refundTarget.value || !refundReason.value.trim() || !column.value) return
  const session = generation
  const target = refundTarget.value
  const columnID = column.value.id
  await mutate(async () => {
    const key = `column-refund:${auth.user!.id}:${target.id}`
    const text = refundReason.value.trim()
    await columnsAPI.refund(columnID, target.id, durableOperation(key, text), text)
    if (session !== generation) return
    sessionStorage.removeItem(key)
    refundTarget.value = null
    refundReason.value = ''
    notice.value = '已退回买家站点余额，并撤销本次购买权限。'
    await load()
  })
}
async function morePurchases() {
  if (!column.value || !purchaseCursor.value) return
  const session = generation
  const columnID = column.value.id
  await mutate(async () => {
    const page = await columnsAPI.purchases(columnID, purchaseCursor.value!)
    if (session !== generation) return
    purchases.value.push(...page.items)
    purchaseCursor.value = page.next_cursor ?? null
  })
}
function switchMode(next: typeof mode.value) {
  if (loading.value || busy.value || mode.value === next) return
  mode.value = next
  void load()
}
watch(() => [route.query.column, route.query.article, auth.user?.id, auth.user?.role], (_next, previous) => {
  generation++
  request++
  busy.value = false
  writing.value = false
  columnEditor.value = false
  column.value = null
  article.value = null
  articles.value = []
  reason.value = ''
  confirmingPurchase.value = false
  refundTarget.value = null
  refundReason.value = ''
  policyReason.value = ''
  purchases.value = []
  if (previous && previous[2] !== auth.user?.id) { columns.value = []; mode.value = 'browse'; notice.value = '' }
  void load()
}, { immediate: true })
onScopeDispose(() => { generation++; request++ })
</script>

<template>
  <section class="city-columns" aria-label="创作者专栏" :aria-busy="loading">
    <header class="columns-header">
      <div class="heading"><button class="icon-button" type="button" :title="column ? '返回专栏目录' : '返回全城动态'" :aria-label="column ? '返回专栏目录' : '返回全城动态'" :disabled="busy" @click="column ? navigate() : city()"><ArrowLeft :size="18" /></button><BookOpen :size="22" /><h1>{{ column?.title || '创作者专栏' }}</h1></div>
      <div class="actions">
        <button class="icon-button" type="button" title="刷新" aria-label="刷新专栏" :disabled="busy || loading || writing || columnEditor" @click="load()"><RefreshCw :size="17" /></button>
        <button v-if="!column" class="primary" type="button" :disabled="busy" @click="editColumn(true)"><Plus :size="16" />开通专栏</button>
        <button v-if="owned" type="button" :disabled="busy" @click="editColumn(false)"><Settings :size="16" />设置</button>
        <button v-if="canWrite" class="primary" type="button" :disabled="busy" @click="write()"><Pencil :size="16" />写文章</button>
      </div>
    </header>
    <p v-if="error" class="message error" role="alert">{{ error }}<button type="button" :disabled="busy || loading" @click="load()">重试加载</button></p>
    <p v-if="notice" class="message" role="status">{{ notice }}</p>
    <p v-if="loading" class="empty" role="status">正在读取专栏…</p>

    <form v-if="columnEditor" class="editor" aria-label="专栏设置" @submit.prevent="saveColumn">
      <div class="section-heading"><h2>{{ creating ? '开通专栏' : '专栏设置' }}</h2><button class="icon-button" type="button" title="关闭设置" aria-label="关闭设置" :disabled="busy" @click="columnEditor = false"><X :size="18" /></button></div>
      <label>专栏名称<input v-model="title" maxlength="80" required :disabled="busy" /></label>
      <label>简介<textarea v-model="description" maxlength="1000" rows="3" :disabled="busy" /></label>
      <div class="actions"><span v-if="creating" class="meta">新专栏默认免费</span><button class="primary" type="submit" :disabled="busy"><Save :size="16" />{{ busy ? '正在保存…' : creating ? '确认开通' : '保存设置' }}</button></div>
    </form>
    <form v-if="columnEditor && !creating && canWrite" class="pricing-editor editor" aria-label="专栏定价" @submit.prevent="savePricing">
      <h2>专栏定价</h2>
      <label>计费方式<select v-model="pricingMode" :disabled="busy"><option value="free">免费</option><option value="paid">一次购买，永久解锁</option></select></label>
      <label v-if="pricingMode === 'paid'">价格（USD）<input v-model="price" inputmode="decimal" pattern="(0|[1-9][0-9]{0,11})(\.[0-9]{1,8})?" required :disabled="busy" /></label>
      <p class="meta">平台费 {{ money(policy.platform_fee) }}。收入进入创作者收益钱包。{{ policy.enabled ? '' : '当前尚未开放付费购买。' }}</p>
      <button class="primary" type="submit" :disabled="busy"><Save :size="16" />保存定价</button>
    </form>

    <template v-if="!column && !loading">
      <nav class="mode-tabs" aria-label="专栏范围"><button type="button" :aria-pressed="mode === 'browse'" :disabled="busy" @click="switchMode('browse')">发现专栏</button><button type="button" :aria-pressed="mode === 'mine'" :disabled="busy" @click="switchMode('mine')">我的专栏</button><button v-if="admin" type="button" :aria-pressed="mode === 'admin'" :disabled="busy" @click="switchMode('admin')">专栏管理</button></nav>
      <form v-if="mode === 'admin' && admin" class="commerce-policy editor" aria-label="付费购买开关" @submit.prevent="setPolicy">
        <label class="checkbox-label"><input v-model="policyEnabled" type="checkbox" :disabled="busy" />开放付费专栏购买</label>
        <p class="meta">站点余额结算（USD），平台费 {{ money(policy.platform_fee) }}；关闭仅阻止新购买。</p>
        <label>调整原因<input v-model="policyReason" maxlength="1000" required :disabled="busy" /></label>
        <button type="submit" :disabled="busy || !policyReason.trim()"><Save :size="16" />保存购买开关</button>
      </form>
      <div class="column-grid">
        <article v-for="item in columns" :key="item.id" class="column-item">
          <div class="item-top"><BookOpen :size="19" /><span>{{ item.status === 'suspended' ? '已停用' : item.mode === 'paid' ? money(item.price) : '免费阅读' }}</span></div>
          <h2><button class="text-button" type="button" @click="navigate(item.id)">{{ item.title }}</button></h2>
          <p>{{ item.description || '作者尚未填写简介。' }}</p>
          <footer><span>{{ item.author_name }}</span><span>{{ item.article_count }} 篇文章</span></footer>
        </article>
      </div>
      <p v-if="!columns.length && !error" class="empty">{{ mode === 'mine' ? '你还没有专栏。' : '暂时没有可见专栏。' }}</p>
      <button v-if="cursor" type="button" :disabled="busy || loading" @click="load(true)">加载更多专栏</button>
    </template>

    <template v-if="column && !loading">
      <div class="column-intro"><p>{{ column.description }}</p><div class="meta"><span>{{ column.author_name }}</span><span>{{ column.mode === 'paid' ? `${money(column.price)} · 一次购买` : '免费专栏' }}</span><span v-if="column.viewer_purchase_id">已购买</span><span>{{ column.article_count }} 篇文章</span></div></div>
      <p v-if="column.status === 'suspended'" class="message error">此专栏已停用。{{ column.moderation_reason }}</p>
      <div v-if="locked && column.status === 'active'" class="paywall">
        <div class="section-heading"><h2><LockKeyhole :size="18" />解锁专栏</h2><strong>{{ money(column.price) }}</strong></div>
        <p>从站点余额扣款，一次购买永久解锁本专栏已发布内容，不自动续费。平台费 {{ money(policy.platform_fee) }}。</p>
        <p class="meta">不承诺自动退款；管理退款须经核实且创作者余额充足。专栏停用期间内容不可见。</p>
        <p v-if="!auth.user">登录后可购买。</p>
        <p v-else-if="!policy.enabled" role="status">付费购买尚未开放。</p>
        <button v-else-if="!confirmingPurchase" class="primary" type="button" :disabled="busy" @click="confirmingPurchase = true"><ShoppingCart :size="16" />购买专栏</button>
        <div v-else class="actions" role="group" aria-label="购买确认"><button class="primary" type="button" :disabled="busy" @click="buy">确认扣款 {{ money(column.price) }}</button><button type="button" :disabled="busy" @click="confirmingPurchase = false">取消</button></div>
      </div>
      <form v-if="admin" class="moderation" @submit.prevent="moderate"><label>管理操作原因<input v-model="reason" maxlength="500" required :disabled="busy" /></label><button type="submit" :disabled="busy || !reason.trim()">{{ column.status === 'active' ? '停用专栏' : '恢复专栏' }}</button></form>
      <details v-if="purchases.length || owned || admin" class="purchase-history">
        <summary>{{ owned || admin ? '销售与退款记录' : '我的购买记录' }}</summary>
        <p v-if="!purchases.length" class="meta">暂无购买记录。</p>
        <div v-for="purchase in purchases" :key="purchase.id" class="purchase-row">
          <div><strong>#{{ purchase.id }} · {{ money(purchase.amount) }}</strong><div class="meta"><span>{{ date(purchase.created_at) }}</span><span>{{ purchase.status === 'active' ? '已解锁' : '已退款' }}</span><span v-if="owned || admin">买家 #{{ purchase.buyer_user_id }} · 创作者收入 {{ money(purchase.creator_amount) }}</span></div><p v-if="purchase.refund_reason">{{ purchase.refund_reason }}</p></div>
          <button v-if="admin && purchase.status === 'active'" type="button" :disabled="busy" @click="refundTarget = purchase; refundReason = ''">管理退款</button>
        </div>
        <button v-if="purchaseCursor" type="button" :disabled="busy" @click="morePurchases">更多购买记录</button>
      </details>
      <form v-if="admin && refundTarget" class="refund-editor editor" aria-label="管理退款" @submit.prevent="refund">
        <div class="section-heading"><h2>退款 #{{ refundTarget.id }}</h2><button class="icon-button" type="button" aria-label="取消退款" :disabled="busy" @click="refundTarget = null"><X :size="18" /></button></div>
        <p>退款 {{ money(refundTarget.amount) }} 到买家站点余额，同时撤销购买权限。创作者可用余额不足时不会执行退款。</p>
        <label>退款原因<textarea v-model="refundReason" maxlength="1000" required :disabled="busy" rows="2" /></label>
        <button type="submit" :disabled="busy || !refundReason.trim()">确认退款并撤权</button>
      </form>
      <form v-if="writing" class="editor article-editor" aria-label="文章编辑" @submit.prevent="saveArticle('draft')">
        <div class="section-heading"><h2>{{ editingID ? '编辑文章' : '新文章' }}</h2><button class="icon-button" type="button" title="关闭编辑" aria-label="关闭编辑" :disabled="busy" @click="writing = false"><X :size="18" /></button></div>
        <label>文章标题<input v-model="draft.title" maxlength="160" required :disabled="busy" /></label>
        <label>摘要<textarea v-model="draft.summary" maxlength="500" rows="2" :disabled="busy" /></label>
        <label>正文<textarea v-model="draft.body" maxlength="100000" rows="16" required :disabled="busy" /></label>
        <div class="actions"><button type="submit" :disabled="busy"><Save :size="16" />保存草稿</button><button v-if="editingID" type="button" :disabled="busy" @click="saveArticle('archived')">归档</button><button class="primary" type="button" :disabled="busy" @click="saveArticle('published')"><Send :size="16" />{{ busy ? '正在保存…' : '发布文章' }}</button></div>
      </form>
      <article v-else-if="article" class="reader">
        <div class="section-heading"><button class="text-button" type="button" @click="navigate(column.id)"><ArrowLeft :size="16" />全部文章</button><button v-if="canWrite" type="button" @click="write(article)"><Pencil :size="16" />编辑</button></div>
        <h2>{{ article.title }}</h2><div class="meta"><span>{{ column.author_name }}</span><time :datetime="article.updated_at">{{ date(article.updated_at) }}</time><span v-if="owned">{{ statusLabel(article.status) }}</span></div>
        <p v-if="article.summary" class="article-summary">{{ article.summary }}</p>
        <div class="article-body">{{ article.body }}</div>
      </article>
      <div v-else class="article-list">
        <h2>{{ owned ? '我的文章' : '全部文章' }}</h2>
        <article v-for="item in articles" :key="item.id"><div class="meta"><time :datetime="item.updated_at">{{ date(item.updated_at) }}</time><span v-if="owned">{{ statusLabel(item.status) }}</span></div><h3><button class="text-button" type="button" @click="navigate(column.id, item.id)">{{ item.title }}</button></h3><p>{{ item.summary }}</p></article>
        <p v-if="!articles.length && !error" class="empty">{{ owned ? '从第一篇文章开始。' : '作者还没有发布文章。' }}</p>
        <button v-if="articleCursor" type="button" :disabled="busy || loading" @click="load(true)">加载更多文章</button>
      </div>
    </template>
  </section>
</template>

<style scoped>
.city-columns{color:var(--bd-text-primary);display:grid;gap:20px;min-width:0;letter-spacing:0}
.columns-header,.heading,.actions,.section-heading,.meta,.item-top,.column-item footer{display:flex;align-items:center;gap:10px;flex-wrap:wrap}
.columns-header,.section-heading,.item-top,.column-item footer{justify-content:space-between}
.heading{min-width:0;flex:1}.heading h1{font-size:22px;margin:0;overflow-wrap:anywhere}h2{font-size:18px;margin:0}h3{font-size:16px;margin:8px 0}
button{display:inline-flex;align-items:center;justify-content:center;gap:7px;min-height:36px;padding:8px 12px;border:1px solid var(--bd-ui-line);border-radius:6px;background:var(--bd-surface);color:var(--bd-text-primary);cursor:pointer;font-size:13px}
button:disabled{opacity:.5;cursor:not-allowed}.icon-button{width:36px;flex:0 0 36px;padding:0}.primary{background:var(--bd-accent-teal);color:var(--bd-surface);border-color:var(--bd-accent-teal)}.text-button{padding:0;min-height:28px;background:none;border:0;text-align:left;justify-content:flex-start;font-size:inherit;font-weight:600;overflow-wrap:anywhere}
button:focus-visible,input:focus-visible,textarea:focus-visible{outline:2px solid var(--bd-accent-teal);outline-offset:3px}
.mode-tabs{display:flex;gap:8px;flex-wrap:wrap;border-bottom:1px solid var(--bd-ui-line);padding-bottom:12px}.mode-tabs button[aria-pressed=true]{color:var(--bd-accent-teal);border-color:var(--bd-accent-teal)}
.column-grid{display:grid;grid-template-columns:repeat(auto-fill,minmax(min(100%,260px),1fr));gap:16px}.column-item{border:1px solid var(--bd-ui-line);border-radius:6px;padding:20px;min-width:0;display:flex;flex-direction:column;gap:16px;background:var(--bd-surface)}.column-item p{flex:1;margin:0;line-height:1.7;color:var(--bd-text-secondary);overflow-wrap:anywhere}.column-item footer,.item-top{font-size:12px;color:var(--bd-text-secondary)}.item-top svg{color:var(--bd-accent-teal)}
.empty{padding:28px 0;color:var(--bd-text-secondary)}.message{margin:0;padding:12px;border-left:3px solid var(--bd-accent-teal);background:var(--bd-surface);overflow-wrap:anywhere;display:flex;gap:10px;align-items:center;flex-wrap:wrap}.error{border-left-color:var(--bd-accent-gold)}
.editor{display:grid;gap:16px;padding:20px 0;border-block:1px solid var(--bd-ui-line)}label{display:grid;gap:7px;font-size:13px;min-width:0}input,textarea{width:100%;box-sizing:border-box;max-width:100%;padding:10px;border:1px solid var(--bd-ui-line);border-radius:6px;background:var(--bd-surface);color:var(--bd-text-primary);font:inherit}textarea{resize:vertical;line-height:1.8}
.column-intro p{margin:0 0 12px;white-space:pre-wrap;overflow-wrap:anywhere;line-height:1.7}.meta{font-size:12px;color:var(--bd-text-secondary)}.moderation{display:flex;gap:12px;align-items:end;flex-wrap:wrap;padding-block:12px;border-block:1px solid var(--bd-ui-line)}.moderation label{flex:1;min-width:min(100%,200px)}
.reader{max-width:860px;min-width:0;width:100%;margin-inline:auto;display:grid;gap:20px;padding:12px 0}.reader h2{font-size:26px;line-height:1.5;overflow-wrap:anywhere}.article-summary{border-left:3px solid var(--bd-accent-gold);padding-left:16px;margin:0;color:var(--bd-text-secondary);line-height:1.8;white-space:pre-wrap;overflow-wrap:anywhere}.article-body{white-space:pre-wrap;overflow-wrap:anywhere;line-height:1.9;font-size:15px}.article-list>article{padding:22px 0;border-bottom:1px solid var(--bd-ui-line)}.article-list p{color:var(--bd-text-secondary);line-height:1.7;overflow-wrap:anywhere;margin-bottom:0}
.paywall{padding:20px 0;border-block:1px solid var(--bd-ui-line)}.paywall h2{display:flex;gap:8px;align-items:center}.paywall p,.refund-editor p{line-height:1.7;overflow-wrap:anywhere}.purchase-history{padding:16px 0;border-block:1px solid var(--bd-ui-line)}summary{cursor:pointer;font-weight:600}.purchase-row{display:flex;justify-content:space-between;align-items:center;gap:12px;flex-wrap:wrap;padding:14px 0;border-bottom:1px solid var(--bd-ui-line)}.purchase-row p{overflow-wrap:anywhere}.checkbox-label{display:flex;align-items:center;gap:10px}.checkbox-label input{width:auto}select{padding:10px;border:1px solid var(--bd-ui-line);border-radius:6px;background:var(--bd-surface);color:var(--bd-text-primary);max-width:100%}
@media(max-width:600px){.columns-header{align-items:flex-start}.columns-header>.actions{width:100%}.heading h1{font-size:20px}.reader h2{font-size:22px}}
</style>
