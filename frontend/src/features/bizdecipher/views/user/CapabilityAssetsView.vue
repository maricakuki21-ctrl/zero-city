<template>
  <AppLayout>
    <div class="capability-assets-page">
      <section class="assets-hero">
        <div class="assets-hero-copy">
          <h1>能力资产</h1>
        </div>
          <div class="assets-hero-actions">
            <button type="button" class="assets-primary-button" @click="openSubmitForm">
              <Icon name="plus" size="sm" />
              发布作品
            </button>
            <button type="button" class="assets-secondary-button" @click="refreshAll">
              <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
              刷新
            </button>
          </div>
      </section>

      <!-- Tab switch -->
      <section class="assets-tabs">
        <button :class="['assets-tab', activeTab === 'public' && 'active']" @click="switchTab('public')">公开资产</button>
        <button :class="['assets-tab', activeTab === 'mine' && 'active']" @click="switchTab('mine')">我的资产</button>
      </section>

      <!-- Filter bar (public only) -->
      <section v-if="activeTab === 'public'" class="assets-filter-bar">
        <div class="assets-search">
          <Icon name="search" size="sm" />
          <input v-model="keyword" type="search" placeholder="搜索产品、工作流、Agent、场景标签" @keyup.enter="loadAssets" />
        </div>
        <select v-model="activeType" class="assets-select" @change="loadAssets">
          <option v-for="option in typeOptionsWithAll" :key="option.value" :value="option.value">{{ option.label }}</option>
        </select>
        <select v-model="activeSort" class="assets-select" @change="loadAssets">
          <option value="featured">推荐优先</option>
          <option value="latest">最新发布</option>
          <option value="popular">热度优先</option>
          <option value="updated">最近更新</option>
        </select>
        <label class="assets-featured-toggle">
          <input v-model="featuredOnly" type="checkbox" @change="loadAssets" />
          只看推荐
        </label>
      </section>

      <!-- Public assets -->
      <section v-if="activeTab === 'public'" class="assets-shell">
        <main class="assets-catalog">
          <div class="assets-section-head">
            <span>当前显示 {{ assets.length }} 件作品</span>
          </div>

          <div v-if="loading" class="assets-grid">
            <article v-for="i in 6" :key="i" class="asset-card asset-card-loading">
              <span></span>
              <strong></strong>
              <p></p>
            </article>
          </div>

          <div v-else-if="assets.length === 0" class="assets-empty">
            <Icon name="grid" size="xl" />
            <h3>还没有符合条件的能力资产</h3>
            <p>可以先提交你的产品或工作流，审核通过后会出现在这里。</p>
          </div>

          <div v-else class="assets-grid">
            <article v-for="asset in assets" :key="asset.id" class="asset-card" @click="openPublicDetail(asset)">
              <div class="asset-cover">
                <img v-if="asset.cover_url" :src="asset.cover_url" :alt="asset.title" />
                <span v-else class="asset-cover-placeholder">{{ typeLabel(asset.asset_type).slice(0, 2) }}</span>
              </div>
              <div class="asset-card-body">
                <div class="asset-card-topline">
                  <span class="asset-type-pill">{{ typeLabel(asset.asset_type) }}</span>
                  <span v-if="asset.is_featured" class="asset-featured-pill">推荐</span>
                </div>
                <h3>{{ asset.title }}</h3>
                <p>{{ asset.summary }}</p>
                <div class="asset-tags">
                  <span v-for="tag in compactTags(asset)" :key="tag" class="asset-tag">{{ tag }}</span>
                </div>
                <div class="asset-card-footer">
                  <span class="asset-author">{{ asset.author }}</span>
                  <span class="asset-pricing-pill" :data-pricing="asset.pricing_type">{{ pricingLabel(asset.pricing_type) }}</span>
                </div>
              </div>
            </article>
          </div>

          <StarterTemplates class="assets-starter-templates" />
        </main>
      </section>

      <!-- My assets -->
      <section v-if="activeTab === 'mine'" class="assets-shell">
        <main class="assets-catalog">
          <div class="assets-section-head">
            <div>
              <span class="assets-kicker">My Submissions</span>
              <h2>我的资产</h2>
            </div>
            <button type="button" class="assets-primary-button assets-primary-button--sm" @click="openSubmitForm">
              <Icon name="plus" size="sm" />
              新建资产
            </button>
          </div>

          <div v-if="loadingMine" class="assets-grid">
            <article v-for="i in 3" :key="i" class="asset-card asset-card-loading">
              <span></span>
              <strong></strong>
              <p></p>
            </article>
          </div>

          <div v-else-if="myAssets.length === 0" class="assets-empty">
            <Icon name="grid" size="xl" />
            <h3>你还没有提交过资产</h3>
            <p>把你的产品、工作流或工具发布上来，让更多人发现。</p>
            <button type="button" class="assets-primary-button" @click="openSubmitForm">提交第一个资产</button>
          </div>

          <div v-else class="my-assets-list">
            <article v-for="asset in myAssets" :key="asset.id" class="my-asset-row" @click="openPublicDetail(asset)">
              <div class="my-asset-cover">
                <img v-if="asset.cover_url" :src="asset.cover_url" :alt="asset.title" />
                <span v-else class="asset-cover-placeholder">{{ typeLabel(asset.asset_type).slice(0, 2) }}</span>
              </div>
              <div class="my-asset-info">
                <div class="my-asset-topline">
                  <span class="asset-type-pill">{{ typeLabel(asset.asset_type) }}</span>
                  <span class="my-asset-status-pill" :data-status="asset.status">{{ statusLabel(asset.status) }}</span>
                </div>
                <h3>{{ asset.title }}</h3>
                <p>{{ asset.summary }}</p>
                <div class="my-asset-stats" @click.stop>
                  <span><strong>{{ myAssetStat(asset.id)?.view_count ?? asset.view_count }}</strong>浏览</span>
                  <span><strong>{{ myAssetStat(asset.id)?.like_count ?? asset.like_count }}</strong>点赞</span>
                  <span><strong>{{ myAssetStat(asset.id)?.favorite_count ?? asset.favorite_count }}</strong>收藏</span>
                  <span><strong>{{ myAssetStat(asset.id)?.download_count ?? asset.download_count }}</strong>下载</span>
                  <span><strong>{{ myAssetStat(asset.id)?.use_count ?? asset.use_count }}</strong>运行</span>
                  <span><strong>{{ myAssetStat(asset.id)?.unique_users ?? 0 }}</strong>独立用户</span>
                  <span class="my-asset-revenue" :data-connected="myAssetStat(asset.id)?.revenue_connected ?? false">
                    {{ myAssetStat(asset.id)?.revenue_connected ? '收益已接通' : '收益结算未接通' }}
                  </span>
                </div>
              </div>
              <div class="my-asset-meta">
                <span v-if="asset.review_note && asset.status === 'rejected'" class="my-asset-review-note">审核备注: {{ asset.review_note }}</span>
                <span class="my-asset-date">{{ assetDateLabel(asset) }} {{ formatDate(assetDate(asset)) }}</span>
                <button type="button" class="my-asset-package" @click.stop="openPackageDialog(asset)">
                  <Icon name="document" size="sm" />版本与交付包
                </button>
                <button v-if="asset.status === 'draft'" type="button" class="my-asset-edit" :disabled="savingAssetId === asset.id" @click.stop="openEditForm(asset)">
                  <Icon name="edit" size="sm" />{{ savingAssetId === asset.id ? '保存中…' : '编辑草稿' }}
                </button>
              </div>
            </article>
          </div>
        </main>
      </section>

      <!-- Detail drawer -->
      <Transition name="drawer-slide">
        <div v-if="detailVisible" class="asset-detail-overlay" @click.self="closeDetail">
          <aside class="asset-detail-panel">
            <header class="detail-header">
              <div class="detail-header-titles">
                <div class="detail-header-pills">
                  <span class="asset-type-pill">{{ typeLabel(detailAsset?.asset_type || 'other') }}</span>
                  <span v-if="detailAsset?.is_featured" class="asset-featured-pill">推荐</span>
                  <span class="asset-pricing-pill" :data-pricing="detailAsset?.pricing_type">{{ pricingLabel(detailAsset?.pricing_type || 'free') }}</span>
                </div>
                <h2>{{ detailAsset?.title }}</h2>
                <p class="detail-author">作者: {{ detailAsset?.author }}</p>
              </div>
              <button type="button" class="detail-close" @click="closeDetail">
                <Icon name="x" size="sm" />
              </button>
            </header>

            <div v-if="detailAsset" class="detail-body">
              <AssetCommercePanel :asset-id="detailAsset.id" />
              <div v-if="detailAsset.cover_url" class="detail-cover">
                <img :src="detailAsset.cover_url" :alt="detailAsset.title" />
              </div>

              <p v-if="detailAsset.summary" class="detail-summary">{{ detailAsset.summary }}</p>

              <div v-if="detailAsset.description" class="detail-description">
                <h4>详细介绍</h4>
                <pre>{{ detailAsset.description }}</pre>
              </div>

              <div v-if="detailAsset.tags?.length || detailAsset.scenario_tags?.length || detailAsset.integration_tags?.length" class="detail-tags-section">
                <div v-if="detailAsset.tags?.length" class="detail-tag-group">
                  <span class="detail-tag-label">标签</span>
                  <div class="detail-tags">
                    <span v-for="tag in detailAsset.tags" :key="tag" class="asset-tag">{{ tag }}</span>
                  </div>
                </div>
                <div v-if="detailAsset.scenario_tags?.length" class="detail-tag-group">
                  <span class="detail-tag-label">适用场景</span>
                  <div class="detail-tags">
                    <span v-for="tag in detailAsset.scenario_tags" :key="tag" class="asset-tag">{{ tag }}</span>
                  </div>
                </div>
                <div v-if="detailAsset.integration_tags?.length" class="detail-tag-group">
                  <span class="detail-tag-label">集成能力</span>
                  <div class="detail-tags">
                    <span v-for="tag in detailAsset.integration_tags" :key="tag" class="asset-tag">{{ tag }}</span>
                  </div>
                </div>
              </div>

              <div class="detail-links">
                <a v-if="detailAsset.demo_url" :href="detailAsset.demo_url" target="_blank" rel="noopener" class="detail-link-button">
                  <Icon name="externalLink" size="sm" /> 在线体验
                </a>
                <a v-if="detailAsset.doc_url" :href="detailAsset.doc_url" target="_blank" rel="noopener" class="detail-link-button">
                  <Icon name="externalLink" size="sm" /> 查看文档
                </a>
                <a v-if="detailAsset.source_url" :href="detailAsset.source_url" target="_blank" rel="noopener" class="detail-link-button">
                  <Icon name="externalLink" size="sm" /> 源码
                </a>
                <a v-if="detailAsset.template_url" :href="detailAsset.template_url" target="_blank" rel="noopener" class="detail-link-button">
                  <Icon name="externalLink" size="sm" /> 模板
                </a>
              </div>

              <div class="detail-actions">
                <button
                  type="button"
                  class="detail-action-button"
                  :class="{ active: detailAsset.liked_by_me }"
                  :disabled="activityBusy"
                  @click="toggleLike"
                >
                  <Icon name="heart" size="sm" />
                  {{ detailAsset.liked_by_me ? '已点赞' : '点赞' }}
                </button>
                <button
                  type="button"
                  class="detail-action-button"
                  :class="{ active: detailAsset.favorited_by_me }"
                  :disabled="activityBusy"
                  @click="toggleFavorite"
                >
                  <Icon name="bookmark" size="sm" />
                  {{ detailAsset.favorited_by_me ? '已收藏' : '收藏' }}
                </button>
              </div>

              <div class="detail-stats">
                <div class="detail-stat"><strong>{{ detailAsset.view_count }}</strong><span>浏览</span></div>
                <div class="detail-stat"><strong>{{ detailAsset.like_count }}</strong><span>点赞</span></div>
                <div class="detail-stat"><strong>{{ detailAsset.favorite_count }}</strong><span>收藏</span></div>
                <div class="detail-stat"><strong>{{ detailAsset.download_count }}</strong><span>下载</span></div>
                <div class="detail-stat"><strong>{{ detailAsset.use_count }}</strong><span>运行</span></div>
                <div class="detail-stat"><strong>{{ formatDate(assetDate(detailAsset)) }}</strong><span>{{ assetDateLabel(detailAsset) }}</span></div>
              </div>
            </div>
          </aside>
        </div>
      </Transition>

      <AssetEditorDialog
        :open="submitFormVisible"
        :mode="editorMode"
        :asset="editingAsset"
        :saving="submitting"
        :notice="editorNotice"
        :notice-tone="editorNoticeTone"
        @close="closeSubmitForm"
        @save="handleSubmit"
      />

      <AssetPackageDialog
        :open="packageVisible"
        :asset="packageAsset"
        @close="closePackageDialog"
      />
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import StarterTemplates from '@/features/bizdecipher/components/assets/StarterTemplates.vue'
import AssetEditorDialog from '@/features/bizdecipher/components/assets/AssetEditorDialog.vue'
import AssetPackageDialog from '@/features/bizdecipher/components/assets/AssetPackageDialog.vue'
import AssetCommercePanel from '@/features/bizdecipher/components/assets/AssetCommercePanel.vue'
import { onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores/app'
import {
  getCapabilityAsset,
  getMyCapabilityAssetStats,
  listCapabilityAssets,
  listMyCapabilityAssets,
  createCapabilityAsset,
  setCapabilityAssetFavorite,
  setCapabilityAssetLike,
  updateCapabilityAsset,
  type CapabilityAsset,
  type CapabilityAssetStats,
  type CapabilityAssetPayload,
} from '@/features/bizdecipher/api/bizdecipher'
import { extractActionableApiErrorMessage } from '@/utils/apiError'

const appStore = useAppStore()
const route = useRoute()

// --- tabs ---
const activeTab = ref<'public' | 'mine'>('public')

function switchTab(tab: 'public' | 'mine') {
  activeTab.value = tab
  if (tab === 'mine') loadMyAssets()
}

// --- public assets ---
const loading = ref(false)
const assets = ref<CapabilityAsset[]>([])
const keyword = ref('')
const activeType = ref<string>('all')
const activeSort = ref<string>('featured')
const featuredOnly = ref(false)

const typeOptions = [
  { value: 'product_app', label: '产品 / 应用' },
  { value: 'game', label: '游戏' },
  { value: 'workflow', label: '工作流' },
  { value: 'agent', label: 'Agent' },
  { value: 'api_tool', label: 'API / 工具' },
  { value: 'plugin_template', label: '插件 / 模板' },
  { value: 'prompt_solution', label: '提示词 / 方案' },
  { value: 'dataset_report', label: '数据集 / 报告' },
  { value: 'other', label: '其他' },
]
const typeOptionsWithAll = [{ value: 'all', label: '全部类型' }, ...typeOptions]

async function loadAssets() {
  loading.value = true
  try {
    assets.value = await listCapabilityAssets({
      keyword: keyword.value || undefined,
      asset_type: activeType.value === 'all' ? undefined : activeType.value,
      sort: activeSort.value,
      featured: featuredOnly.value || undefined,
      limit: 60,
    })
  } catch {
    appStore.showError('加载资产列表失败')
  } finally {
    loading.value = false
  }
}

// --- my assets ---
const loadingMine = ref(false)
const myAssets = ref<CapabilityAsset[]>([])
const myAssetStats = ref<CapabilityAssetStats[]>([])

function myAssetStat(assetID: number): CapabilityAssetStats | undefined {
  return myAssetStats.value.find(item => item.asset_id === assetID)
}

async function loadMyAssets(reportError = true): Promise<boolean> {
  loadingMine.value = true
  try {
    const [assetsResult, statsResult] = await Promise.allSettled([
      listMyCapabilityAssets({ limit: 50 }),
      getMyCapabilityAssetStats(),
    ])
    if (assetsResult.status === 'rejected') {
      if (reportError) appStore.showError('加载我的资产失败')
      return false
    }
    myAssets.value = assetsResult.value
    myAssetStats.value = statsResult.status === 'fulfilled' ? statsResult.value : []
    if (reportError && statsResult.status === 'rejected') {
      appStore.showWarning('资产已加载，使用统计暂时不可用')
    }
    return true
  } catch {
    if (reportError) appStore.showError('加载我的资产失败')
    return false
  } finally {
    loadingMine.value = false
  }
}

async function refreshAll() {
  await loadAssets()
  if (activeTab.value === 'mine') await loadMyAssets()
}

// --- detail drawer ---
const detailVisible = ref(false)
const detailAsset = ref<CapabilityAsset | null>(null)
const activityBusy = ref(false)

async function openPublicDetail(asset: CapabilityAsset) {
  detailAsset.value = asset
  detailVisible.value = true
  try {
    const fresh = await getCapabilityAsset(asset.id)
    if (detailAsset.value?.id !== asset.id) return
    applyAssetUpdate(fresh)
  } catch {
    appStore.showError('资产详情刷新失败，当前显示列表中的信息')
  }
}
function closeDetail() {
  detailVisible.value = false
  detailAsset.value = null
}

function applyAssetUpdate(updated: CapabilityAsset) {
  assets.value = assets.value.map(asset => asset.id === updated.id ? updated : asset)
  myAssets.value = myAssets.value.map(asset => asset.id === updated.id ? updated : asset)
  if (detailAsset.value?.id === updated.id) detailAsset.value = updated
  const stat = myAssetStat(updated.id)
  if (stat) {
    stat.view_count = updated.view_count
    stat.like_count = updated.like_count
    stat.favorite_count = updated.favorite_count
    stat.download_count = updated.download_count
    stat.use_count = updated.use_count
  }
}

async function toggleLike() {
  if (!detailAsset.value || activityBusy.value) return
  activityBusy.value = true
  try {
    const updated = await setCapabilityAssetLike(detailAsset.value.id, !detailAsset.value.liked_by_me)
    applyAssetUpdate(updated)
  } catch {
    appStore.showError('点赞操作失败，请稍后重试')
  } finally {
    activityBusy.value = false
  }
}

async function toggleFavorite() {
  if (!detailAsset.value || activityBusy.value) return
  activityBusy.value = true
  try {
    const updated = await setCapabilityAssetFavorite(detailAsset.value.id, !detailAsset.value.favorited_by_me)
    applyAssetUpdate(updated)
  } catch {
    appStore.showError('收藏操作失败，请稍后重试')
  } finally {
    activityBusy.value = false
  }
}

// --- submit form ---
const submitFormVisible = ref(false)
const submitting = ref(false)
const editorMode = ref<'create' | 'edit'>('create')
const editingAsset = ref<CapabilityAsset | null>(null)
const savingAssetId = ref<number | null>(null)
const editorNotice = ref('')
const editorNoticeTone = ref<'success' | 'warning' | 'error'>('success')

// --- versioned delivery packages ---
const packageVisible = ref(false)
const packageAsset = ref<CapabilityAsset | null>(null)

function openPackageDialog(asset: CapabilityAsset) {
  packageAsset.value = asset
  packageVisible.value = true
}

function closePackageDialog() {
  packageVisible.value = false
  packageAsset.value = null
}

function openSubmitForm() {
  editorMode.value = 'create'
  editingAsset.value = null
  editorNotice.value = ''
  submitFormVisible.value = true
}

function openEditForm(asset: CapabilityAsset) {
  if (asset.status !== 'draft' || submitting.value) return
  editorMode.value = 'edit'
  editingAsset.value = asset
  editorNotice.value = ''
  submitFormVisible.value = true
}

function closeSubmitForm() {
  if (submitting.value) return
  submitFormVisible.value = false
}

function replaceAsset(updated: CapabilityAsset) {
  applyAssetUpdate(updated)
  editingAsset.value = updated
}

async function handleSubmit(payload: CapabilityAssetPayload) {
  if (submitting.value) return
  submitting.value = true
  editorNotice.value = ''
  const editedId = editingAsset.value?.id ?? null
  savingAssetId.value = editedId
  try {
    if (editorMode.value === 'edit') {
      if (!editedId) throw new Error('缺少草稿编号，无法保存')
      const updated = await updateCapabilityAsset(editedId, payload)
      replaceAsset(updated)
      editorNotice.value = `草稿 #${updated.id} 已保存，正在重新读取…`
      editorNoticeTone.value = 'success'
      const refreshedOK = await loadMyAssets(false)
      if (refreshedOK) {
        const refreshed = myAssets.value.find(asset => asset.id === updated.id)
        if (refreshed) replaceAsset(refreshed)
        editorNotice.value = `草稿 #${updated.id} 已保存并重新读取。`
        appStore.showSuccess('草稿修改已保存')
      } else {
        replaceAsset(updated)
        editorNotice.value = `草稿 #${updated.id} 已保存，但列表重新读取失败。你可以继续编辑或稍后刷新。`
        editorNoticeTone.value = 'warning'
        appStore.showError('草稿已保存，但列表刷新失败')
      }
      return
    }

    await createCapabilityAsset(payload)
    appStore.showSuccess(payload.status === 'draft' ? '草稿已保存' : '已提交，等待审核')
    submitFormVisible.value = false
    await loadMyAssets()
    if (activeTab.value === 'public') await loadAssets()
  } catch (error) {
    const message = extractActionableApiErrorMessage(error, editorMode.value === 'edit' ? '保存失败，请稍后重试' : '提交失败，请稍后重试')
    editorNotice.value = message
    editorNoticeTone.value = 'error'
    appStore.showError(message)
  } finally {
    submitting.value = false
    savingAssetId.value = null
  }
}

// --- helpers ---
function typeLabel(type: string): string {
  const found = typeOptions.find(o => o.value === type)
  return found?.label ?? '其他'
}
function pricingLabel(type: string): string {
  const map: Record<string, string> = { free: '免费', open_source: '开源', paid: '付费', contact: '联系作者' }
  return map[type] ?? '免费'
}
function statusLabel(status: string): string {
  const map: Record<string, string> = {
    draft: '草稿',
    pending: '待审核',
    listed: '已上架',
    rejected: '已拒绝',
    archived: '已归档',
    delisted: '已下架',
  }
  return map[status] ?? status
}
function compactTags(asset: CapabilityAsset): string[] {
  return [...(asset.tags || []), ...(asset.scenario_tags || [])].slice(0, 4)
}
function formatDate(dateStr?: string | null): string {
  if (!dateStr) return '--'
  const d = new Date(dateStr)
  if (isNaN(d.getTime())) return '--'
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
}

function assetDate(asset: CapabilityAsset): string | null | undefined {
  if (asset.status === 'listed') return asset.published_at
  return asset.updated_at || asset.created_at
}

function assetDateLabel(asset: CapabilityAsset): string {
  if (asset.status === 'listed') return '发布'
  if (asset.status === 'draft') return '更新'
  return '提交'
}

onMounted(async () => {
  await loadAssets()
  if (route.query.tab === 'mine') {
    activeTab.value = 'mine'
    await loadMyAssets()
    const requestedAssetId = Number(route.query.asset)
    if (Number.isSafeInteger(requestedAssetId) && requestedAssetId > 0) {
      const requestedAsset = myAssets.value.find(asset => asset.id === requestedAssetId)
      if (requestedAsset) openPublicDetail(requestedAsset)
    }
  }
  if (route.query.action === 'create') {
    openSubmitForm()
  }
})
</script>

<style scoped>
.capability-assets-page {
  --color-text-primary: var(--zc-text-strong);
  --color-text-secondary: var(--zc-muted);
  --color-text-tertiary: var(--zc-subtle);
  --color-primary: var(--zc-accent);
  --color-border: var(--zc-line);
  --color-bg-primary: var(--zc-bg);
  --color-bg-secondary: var(--zc-bg);
  --color-bg-card: var(--zc-bg);
  --color-bg-hover: color-mix(in srgb, var(--zc-bg) 82%, var(--zc-shadow-dark));
  --color-bg-skeleton: color-mix(in srgb, var(--zc-shadow-dark) 30%, var(--zc-bg));
  --color-bg-tag: var(--zc-bg);
  max-width: 1200px;
  margin: 0 auto;
  padding: 24px 20px 60px;
}

/* Hero */
.assets-hero {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
  margin-bottom: 18px;
  padding-bottom: 16px;
  border-bottom: 1px solid var(--bd-ui-line);
}
.assets-hero-copy h1 {
  font-size: 22px;
  font-weight: 600;
  margin: 0;
}
.assets-hero-copy p {
  font-size: 14px;
  color: var(--color-text-secondary, #8b95a7);
  max-width: 560px;
  line-height: 1.6;
}
.assets-kicker {
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: var(--color-primary, #6366f1);
}
.assets-hero-actions {
  display: flex;
  gap: 10px;
  margin-top: 16px;
}
.assets-primary-button {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 8px 18px;
  border-radius: 8px;
  font-size: 13px;
  font-weight: 600;
  background: var(--color-primary, #6366f1);
  color: var(--zc-bg);
  border: none;
  cursor: pointer;
  transition: opacity 0.15s;
}
.assets-primary-button:hover { opacity: 0.88; }
.assets-primary-button:disabled { opacity: 0.5; cursor: not-allowed; }
.assets-primary-button--sm { padding: 6px 14px; font-size: 12px; }
.assets-secondary-button {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 8px 16px;
  border-radius: 8px;
  font-size: 13px;
  font-weight: 500;
  background: transparent;
  color: var(--color-text-primary, #e5e7eb);
  border: 1px solid var(--color-border, #2d3348);
  cursor: pointer;
}
.assets-secondary-button:hover { border-color: var(--color-primary, #6366f1); }
.assets-hero-board {
  display: flex;
  gap: 20px;
  flex-shrink: 0;
}
.hero-signal {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 2px;
}
.hero-signal span { font-size: 11px; color: var(--color-text-tertiary, #6b7280); }
.hero-signal strong { font-size: 22px; font-weight: 700; }

/* Tabs */
.assets-tabs {
  display: flex;
  gap: 4px;
  margin-bottom: 20px;
  border-bottom: 1px solid var(--color-border, #2d3348);
}
.assets-tab {
  padding: 8px 20px;
  font-size: 13px;
  font-weight: 500;
  background: transparent;
  border: none;
  border-bottom: 2px solid transparent;
  color: var(--color-text-secondary, #8b95a7);
  cursor: pointer;
  transition: all 0.15s;
}
.assets-tab:hover { color: var(--color-text-primary, #e5e7eb); }
.assets-tab.active {
  color: var(--color-primary, #6366f1);
  border-bottom-color: var(--color-primary, #6366f1);
}

/* Filter bar */
.assets-filter-bar {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 20px;
  flex-wrap: wrap;
}
.assets-search {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 12px;
  border: 1px solid var(--color-border, #2d3348);
  border-radius: 8px;
  flex: 1;
  min-width: 240px;
}
.assets-search input {
  background: transparent;
  border: none;
  outline: none;
  color: var(--color-text-primary, #e5e7eb);
  font-size: 13px;
  width: 100%;
}
.assets-select {
  padding: 6px 12px;
  border: 1px solid var(--color-border, #2d3348);
  border-radius: 8px;
  background: var(--color-bg-secondary, #1a1f2e);
  color: var(--color-text-primary, #e5e7eb);
  font-size: 13px;
  cursor: pointer;
}
.assets-featured-toggle {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  color: var(--color-text-secondary, #8b95a7);
  cursor: pointer;
  white-space: nowrap;
}

/* Grid */
.assets-shell { min-height: 400px; }
.assets-catalog { width: 100%; }
.assets-section-head {
  display: flex;
  justify-content: space-between;
  align-items: flex-end;
  margin-bottom: 16px;
}
.assets-section-head h2 { font-size: 18px; font-weight: 600; margin-top: 2px; }
.assets-section-head span { font-size: 12px; color: var(--color-text-secondary, #8b95a7); }
.assets-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 16px;
}
.asset-card {
  display: flex;
  flex-direction: column;
  border: 1px solid var(--color-border, #2d3348);
  border-radius: 12px;
  overflow: hidden;
  cursor: pointer;
  transition: border-color 0.15s, transform 0.15s;
  background: var(--color-bg-card, #1a1f2e);
}
.asset-card:hover {
  border-color: var(--color-primary, #6366f1);
  transform: translateY(-2px);
}
.asset-card-loading { pointer-events: none; }
.asset-card-loading span,
.asset-card-loading strong,
.asset-card-loading p {
  display: block;
  height: 12px;
  margin: 8px;
  border-radius: 4px;
  background: var(--color-bg-skeleton, #2a2f3e);
  animation: pulse 1.5s infinite;
}
.asset-card-loading strong { width: 60%; }
.asset-card-loading p { width: 80%; }
@keyframes pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.5; }
}
.asset-cover {
  width: 100%;
  height: 140px;
  overflow: hidden;
  background: var(--color-bg-skeleton, #2a2f3e);
  display: flex;
  align-items: center;
  justify-content: center;
}
.asset-cover img { width: 100%; height: 100%; object-fit: cover; }
.asset-cover-placeholder { font-size: 28px; font-weight: 700; color: var(--color-text-tertiary, #6b7280); }
.asset-card-body { padding: 12px 14px; display: flex; flex-direction: column; gap: 6px; }
.asset-card-topline { display: flex; gap: 6px; flex-wrap: wrap; }
.asset-type-pill {
  font-size: 10px;
  font-weight: 600;
  padding: 2px 8px;
  border-radius: 4px;
  background: var(--color-bg-tag, #2d3348);
  color: var(--color-text-secondary, #8b95a7);
}
.asset-featured-pill {
  font-size: 10px;
  font-weight: 600;
  padding: 2px 8px;
  border-radius: 4px;
  background: rgba(99, 102, 241, 0.15);
  color: var(--color-primary, #6366f1);
}
.asset-card-body h3 { font-size: 14px; font-weight: 600; line-height: 1.4; }
.asset-card-body p { font-size: 12px; color: var(--color-text-secondary, #8b95a7); line-height: 1.5; display: -webkit-box; -webkit-line-clamp: 2; -webkit-box-orient: vertical; overflow: hidden; }
.asset-tags { display: flex; gap: 4px; flex-wrap: wrap; }
.asset-tag {
  font-size: 10px;
  padding: 1px 6px;
  border-radius: 3px;
  background: var(--color-bg-tag, #2d3348);
  color: var(--color-text-tertiary, #6b7280);
}
.asset-card-footer { display: flex; justify-content: space-between; align-items: center; margin-top: 4px; }
.asset-author { font-size: 11px; color: var(--color-text-tertiary, #6b7280); }
.asset-pricing-pill {
  font-size: 10px;
  font-weight: 600;
  padding: 2px 8px;
  border-radius: 4px;
}
.asset-pricing-pill[data-pricing="free"] { background: color-mix(in srgb, var(--zc-success) 12%, var(--zc-bg)); color: var(--zc-success); }
.asset-pricing-pill[data-pricing="open_source"] { background: color-mix(in srgb, var(--zc-accent-2) 12%, var(--zc-bg)); color: var(--zc-accent-2); }
.asset-pricing-pill[data-pricing="paid"] { background: color-mix(in srgb, var(--zc-warning) 12%, var(--zc-bg)); color: var(--zc-warning); }
.asset-pricing-pill[data-pricing="contact"] { background: color-mix(in srgb, var(--zc-accent) 12%, var(--zc-bg)); color: var(--zc-accent); }

/* Empty */
.assets-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  padding: 60px 20px;
  text-align: center;
  color: var(--color-text-secondary, #8b95a7);
}
.assets-empty h3 { font-size: 16px; font-weight: 600; color: var(--color-text-primary, #e5e7eb); }
.assets-empty p { font-size: 13px; }

/* My assets list */
.my-assets-list { display: flex; flex-direction: column; gap: 10px; }
.my-asset-row {
  display: flex;
  gap: 14px;
  padding: 12px;
  border: 1px solid var(--color-border, #2d3348);
  border-radius: 10px;
  cursor: pointer;
  transition: border-color 0.15s;
  background: var(--color-bg-card, #1a1f2e);
}
.my-asset-row:hover { border-color: var(--color-primary, #6366f1); }
.my-asset-cover {
  width: 64px;
  height: 64px;
  border-radius: 8px;
  overflow: hidden;
  flex-shrink: 0;
  background: var(--color-bg-skeleton, #2a2f3e);
  display: flex;
  align-items: center;
  justify-content: center;
}
.my-asset-cover img { width: 100%; height: 100%; object-fit: cover; }
.my-asset-info { flex: 1; display: flex; flex-direction: column; gap: 4px; }
.my-asset-topline { display: flex; gap: 6px; align-items: center; }
.my-asset-status-pill {
  font-size: 10px;
  font-weight: 600;
  padding: 2px 8px;
  border-radius: 4px;
}
.my-asset-status-pill[data-status="draft"] { background: var(--color-bg-tag, #2d3348); color: var(--color-text-secondary, #8b95a7); }
.my-asset-status-pill[data-status="pending"] { background: color-mix(in srgb, var(--zc-warning) 12%, var(--zc-bg)); color: var(--zc-warning); }
.my-asset-status-pill[data-status="listed"] { background: color-mix(in srgb, var(--zc-success) 12%, var(--zc-bg)); color: var(--zc-success); }
.my-asset-status-pill[data-status="rejected"] { background: color-mix(in srgb, var(--zc-danger) 12%, var(--zc-bg)); color: var(--zc-danger); }
.my-asset-status-pill[data-status="archived"] { background: var(--color-bg-tag, #2d3348); color: var(--color-text-tertiary, #6b7280); }
.my-asset-status-pill[data-status="delisted"] { background: color-mix(in srgb, var(--zc-danger) 12%, var(--zc-bg)); color: var(--zc-danger); }
.my-asset-info h3 { font-size: 14px; font-weight: 600; }
.my-asset-info p { font-size: 12px; color: var(--color-text-secondary, #8b95a7); }
.my-asset-stats {
  display: flex;
  flex-wrap: wrap;
  gap: 6px 14px;
  margin-top: 4px;
  font-size: 11px;
  color: var(--color-text-tertiary, #6b7280);
}
.my-asset-stats strong {
  margin-right: 3px;
  color: var(--color-text-primary, #e5e7eb);
  font-size: 12px;
}
.my-asset-revenue {
  color: var(--zc-warning);
}
.my-asset-revenue[data-connected="true"] {
  color: var(--zc-success);
}
.my-asset-meta { display: flex; flex-direction: column; gap: 4px; align-items: flex-end; justify-content: center; }
.my-asset-review-note { font-size: 11px; color: var(--zc-danger); max-width: 200px; text-align: right; }
.my-asset-date { font-size: 11px; color: var(--color-text-tertiary, #6b7280); }

/* Detail drawer */
.asset-detail-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  justify-content: flex-end;
  z-index: 1000;
}
.asset-detail-panel {
  width: min(560px, 90vw);
  height: 100%;
  background: var(--color-bg-primary, #131826);
  border-left: 1px solid var(--color-border, #2d3348);
  display: flex;
  flex-direction: column;
  overflow-y: auto;
}
.detail-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  padding: 20px 24px;
  border-bottom: 1px solid var(--color-border, #2d3348);
  position: sticky;
  top: 0;
  background: var(--color-bg-primary, #131826);
  z-index: 1;
}
.detail-header-pills { display: flex; gap: 6px; margin-bottom: 8px; }
.detail-header h2 { font-size: 20px; font-weight: 700; }
.detail-author { font-size: 12px; color: var(--color-text-secondary, #8b95a7); margin-top: 4px; }
.detail-close {
  background: transparent;
  border: none;
  color: var(--color-text-secondary, #8b95a7);
  cursor: pointer;
  padding: 4px;
  border-radius: 6px;
  display: flex;
  align-items: center;
  justify-content: center;
}
.detail-close:hover { background: var(--color-bg-hover, #1e2436); color: var(--color-text-primary, #e5e7eb); }
.detail-body { padding: 20px 24px; display: flex; flex-direction: column; gap: 20px; }
.detail-cover { width: 100%; border-radius: 10px; overflow: hidden; }
.detail-cover img { width: 100%; display: block; }
.detail-summary { font-size: 14px; color: var(--color-text-secondary, #8b95a7); line-height: 1.6; }
.detail-description h4 { font-size: 13px; font-weight: 600; margin-bottom: 8px; }
.detail-description pre {
  font-family: inherit;
  font-size: 13px;
  line-height: 1.6;
  color: var(--color-text-primary, #e5e7eb);
  white-space: pre-wrap;
  word-break: break-word;
  margin: 0;
}
.detail-tags-section { display: flex; flex-direction: column; gap: 12px; }
.detail-tag-group { display: flex; flex-direction: column; gap: 6px; }
.detail-tag-label { font-size: 12px; font-weight: 600; color: var(--color-text-secondary, #8b95a7); }
.detail-tags { display: flex; gap: 6px; flex-wrap: wrap; }
.detail-links { display: flex; gap: 8px; flex-wrap: wrap; }
.detail-link-button {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 8px 14px;
  border-radius: 8px;
  font-size: 12px;
  font-weight: 500;
  background: var(--color-bg-secondary, #1a1f2e);
  color: var(--color-text-primary, #e5e7eb);
  border: 1px solid var(--color-border, #2d3348);
  text-decoration: none;
  transition: border-color 0.15s;
}
.detail-link-button:hover { border-color: var(--color-primary, #6366f1); }
.detail-actions { display: flex; gap: 8px; flex-wrap: wrap; }
.detail-action-button {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 8px 14px;
  border-radius: 8px;
  border: 1px solid var(--color-border, #2d3348);
  background: var(--color-bg-secondary, #1a1f2e);
  color: var(--color-text-primary, #e5e7eb);
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
}
.detail-action-button:hover:not(:disabled) { border-color: var(--color-primary, #6366f1); }
.detail-action-button.active {
  border-color: var(--color-primary, #6366f1);
  background: color-mix(in srgb, var(--color-primary, #6366f1) 14%, var(--color-bg-secondary, #1a1f2e));
  color: var(--color-primary, #6366f1);
}
.detail-action-button:disabled { opacity: 0.5; cursor: wait; }
.detail-stats { display: flex; flex-wrap: wrap; gap: 16px 20px; padding-top: 12px; border-top: 1px solid var(--color-border, #2d3348); }
.detail-stat { display: flex; flex-direction: column; gap: 2px; }
.detail-stat strong { font-size: 16px; font-weight: 700; }
.detail-stat span { font-size: 11px; color: var(--color-text-tertiary, #6b7280); }

/* Submit form */
.submit-form-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
  padding: 20px;
}
.submit-form-panel {
  width: min(680px, 95vw);
  max-height: 90vh;
  background: var(--color-bg-primary, #131826);
  border: 1px solid var(--color-border, #2d3348);
  border-radius: 12px;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}
.submit-form-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 16px 24px;
  border-bottom: 1px solid var(--color-border, #2d3348);
}
.submit-form-header h2 { font-size: 16px; font-weight: 600; }
.submit-form-body { padding: 20px 24px; overflow-y: auto; display: flex; flex-direction: column; gap: 14px; }
.form-row { display: flex; flex-direction: column; gap: 6px; }
.form-row-2col { flex-direction: row; gap: 12px; }
.form-row-2col > div { flex: 1; display: flex; flex-direction: column; gap: 6px; }
.form-row label { font-size: 12px; font-weight: 500; color: var(--color-text-secondary, #8b95a7); }
.required { color: var(--zc-danger); }
.form-row input[type="text"],
.form-row input[type="url"],
.form-row textarea,
.form-row select {
  padding: 8px 12px;
  border: 1px solid var(--color-border, #2d3348);
  border-radius: 8px;
  background: var(--color-bg-secondary, #1a1f2e);
  color: var(--color-text-primary, #e5e7eb);
  font-size: 13px;
  outline: none;
  transition: border-color 0.15s;
}
.form-row input:focus,
.form-row textarea:focus,
.form-row select:focus { border-color: var(--color-primary, #6366f1); }
.form-row textarea { resize: vertical; font-family: inherit; }
.form-checkbox-label,
.form-radio-label {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  color: var(--color-text-primary, #e5e7eb);
  cursor: pointer;
}
.form-submit-row {
  flex-direction: row;
  align-items: center;
  gap: 16px;
  padding-top: 8px;
  border-top: 1px solid var(--color-border, #2d3348);
}

/* Transitions */
.drawer-slide-enter-active,
.drawer-slide-leave-active { transition: opacity 0.2s; }
.drawer-slide-enter-active .asset-detail-panel,
.drawer-slide-leave-active .asset-detail-panel { transition: transform 0.25s ease; }
.drawer-slide-enter-from,
.drawer-slide-leave-to { opacity: 0; }
.drawer-slide-enter-from .asset-detail-panel,
.drawer-slide-leave-to .asset-detail-panel { transform: translateX(100%); }

.modal-fade-enter-active,
.modal-fade-leave-active { transition: opacity 0.2s; }
.modal-fade-enter-from,
.modal-fade-leave-to { opacity: 0; }

.animate-spin { animation: spin 1s linear infinite; }
@keyframes spin { from { transform: rotate(0deg); } to { transform: rotate(360deg); } }

@media (max-width: 768px) {
  .assets-hero { flex-direction: column; align-items: flex-start; }
  .assets-hero-board { width: 100%; justify-content: space-around; }
  .assets-filter-bar { flex-direction: column; }
  .assets-search { width: 100%; }
  .form-row-2col { flex-direction: column; }
}

/* ── Capability asset material system ─────────────────────────────── */
.capability-assets-page {
  color: var(--module-text, var(--zc-text));
}

/* Page background: single flat bg, no fills */
.assets-hero-copy h1 { color: var(--module-text-strong, var(--zc-text-strong)); }
.assets-hero-copy p  { color: var(--module-muted, var(--zc-muted)); }
.assets-kicker       { color: var(--module-accent, var(--zc-accent)); }
.hero-signal span    { color: var(--module-muted, var(--zc-muted)); }
.hero-signal strong  { color: var(--module-text-strong, var(--zc-text-strong)); }

/* Tabs */
.assets-tabs { border-bottom-color: var(--module-line, var(--zc-line)); }
.assets-tab  { color: var(--module-muted, var(--zc-muted)); background: transparent; }
.assets-tab:hover { color: var(--module-text-strong, var(--zc-text-strong)); }
.assets-tab.active { color: var(--module-accent, var(--zc-accent)); border-bottom-color: var(--module-accent, var(--zc-accent)); }

/* Filter bar */
.assets-search {
  background: var(--module-canvas, var(--zc-bg));
  border-color: var(--module-line, var(--zc-line));
  box-shadow: inset 0 2px 8px color-mix(in srgb, var(--module-shadow-dark, var(--zc-shadow-dark)) 30%, transparent);
}
.assets-search input { color: var(--module-text, var(--zc-text)); }

.assets-select {
  background: var(--module-canvas, var(--zc-bg));
  border-color: var(--module-line, var(--zc-line));
  color: var(--module-text, var(--zc-text));
  box-shadow: 0 8px 18px color-mix(in srgb, var(--module-shadow-dark, var(--zc-shadow-dark)) 24%, transparent);
}
.assets-featured-toggle { color: var(--module-muted, var(--zc-muted)); }

/* Cards */
.asset-card {
  background: var(--module-canvas, var(--zc-bg));
  border-color: var(--module-line, var(--zc-line));
  box-shadow: 0 14px 34px color-mix(in srgb, var(--module-shadow-dark, var(--zc-shadow-dark)) 28%, transparent), inset 0 1px 0 color-mix(in srgb, var(--module-shadow-light, var(--zc-shadow-light)) 64%, transparent);
}
.asset-card:hover {
  border-color: var(--module-line-strong, var(--zc-line-strong));
  box-shadow: 0 18px 40px color-mix(in srgb, var(--module-shadow-dark, var(--zc-shadow-dark)) 34%, transparent), inset 0 1px 0 color-mix(in srgb, var(--module-shadow-light, var(--zc-shadow-light)) 70%, transparent);
}
.asset-cover { background: var(--module-canvas, var(--zc-bg)); }
.asset-cover-placeholder { color: var(--module-subtle, var(--zc-subtle)); }
.asset-type-pill { background: var(--module-canvas, var(--zc-bg)); color: var(--module-muted, var(--zc-muted)); box-shadow: inset 0 1px 5px color-mix(in srgb, var(--module-shadow-dark, var(--zc-shadow-dark)) 22%, transparent); }
.asset-card-body h3 { color: var(--module-text-strong, var(--zc-text-strong)); }
.asset-card-body p  { color: var(--module-muted, var(--zc-muted)); }
.asset-tag { background: var(--module-canvas, var(--zc-bg)); color: var(--module-muted, var(--zc-muted)); box-shadow: inset 0 1px 5px color-mix(in srgb, var(--module-shadow-dark, var(--zc-shadow-dark)) 20%, transparent); }
.asset-author { color: var(--module-subtle, var(--zc-subtle)); }

/* My-assets list rows */
.my-asset-row {
  background: var(--module-canvas, var(--zc-bg));
  border-color: var(--module-line, var(--zc-line));
  box-shadow: 0 12px 28px color-mix(in srgb, var(--module-shadow-dark, var(--zc-shadow-dark)) 26%, transparent), inset 0 1px 0 color-mix(in srgb, var(--module-shadow-light, var(--zc-shadow-light)) 60%, transparent);
}
.my-asset-row:hover { box-shadow: 0 16px 34px color-mix(in srgb, var(--module-shadow-dark, var(--zc-shadow-dark)) 32%, transparent); }
.my-asset-cover  { background: var(--module-canvas, var(--zc-bg)); }
.my-asset-info h3 { color: var(--module-text-strong, var(--zc-text-strong)); }
.my-asset-info p  { color: var(--module-muted, var(--zc-muted)); }
.my-asset-date    { color: var(--module-subtle, var(--zc-subtle)); }
.my-asset-edit {
  display: inline-flex;
  align-items: center;
  gap: var(--bd-space-1);
  min-height: 32px;
  padding: 0 var(--bd-space-3);
  color: var(--bd-accent-teal);
  background: transparent;
  border: 1px solid var(--bd-ui-line);
  border-radius: var(--bd-radius-sm);
  font-size: var(--bd-type-caption);
  font-weight: 700;
}
.my-asset-edit:hover { border-color: var(--bd-accent-teal); }
.my-asset-edit:disabled { opacity: .55; cursor: wait; }
.my-asset-package {
  display: inline-flex;
  align-items: center;
  gap: var(--bd-space-1);
  min-height: 32px;
  padding: 0 var(--bd-space-3);
  color: var(--module-text, var(--zc-text));
  background: transparent;
  border: 1px solid var(--module-line, var(--zc-line));
  border-radius: var(--bd-radius-sm);
  font-size: var(--bd-type-caption);
  font-weight: 700;
}
.my-asset-package:hover { color: var(--module-accent, var(--zc-accent)); border-color: var(--module-accent, var(--zc-accent)); }
.assets-starter-templates { margin-top: var(--bd-space-10); }

@media (max-width: 640px) {
  .my-asset-row { align-items: flex-start; flex-wrap: wrap; }
  .my-asset-info { min-width: calc(100% - 82px); }
  .my-asset-meta { width: 100%; flex-direction: row; align-items: center; justify-content: space-between; }
}

/* Empty state */
.assets-empty { color: var(--module-muted, var(--zc-muted)); }
.assets-empty h3 { color: var(--module-text-strong, var(--zc-text-strong)); }

/* Section head */
.assets-section-head h2 { color: var(--module-text-strong, var(--zc-text-strong)); }
.assets-section-head > span { color: var(--module-muted, var(--zc-muted)); }

/* Buttons */
.assets-primary-button {
  background: linear-gradient(135deg, var(--module-accent, var(--zc-accent)), var(--module-accent-2, var(--zc-accent-2)));
  color: var(--zc-accent-ink, #fff);
  border: none;
  box-shadow: 0 10px 24px color-mix(in srgb, var(--module-accent, var(--zc-accent)) 28%, transparent);
}
.assets-secondary-button {
  background: var(--module-canvas, var(--zc-bg));
  color: var(--module-text, var(--zc-text));
  border-color: var(--module-line, var(--zc-line));
  box-shadow: 0 8px 18px color-mix(in srgb, var(--module-shadow-dark, var(--zc-shadow-dark)) 24%, transparent);
}
.assets-secondary-button:hover { border-color: var(--module-accent, var(--zc-accent)); }

/* Detail panel */
.asset-detail-overlay { background: rgba(12, 24, 42, 0.48); }
.asset-detail-panel {
  background: var(--module-canvas, var(--zc-bg));
  border-left-color: var(--module-line, var(--zc-line));
  box-shadow: -18px 0 54px color-mix(in srgb, #08111f 38%, transparent);
}
.detail-header {
  background: var(--module-canvas, var(--zc-bg));
  border-bottom-color: var(--module-line, var(--zc-line));
}
.detail-header h2 { color: var(--module-text-strong, var(--zc-text-strong)); }
.detail-author    { color: var(--module-muted, var(--zc-muted)); }
.detail-close { color: var(--module-muted, var(--zc-muted)); }
.detail-close:hover { background: color-mix(in srgb, var(--module-canvas, var(--zc-bg)) 80%, var(--module-shadow-dark, var(--zc-shadow-dark))); color: var(--module-text-strong, var(--zc-text-strong)); }
.detail-summary { color: var(--module-muted, var(--zc-muted)); }
.detail-description pre { color: var(--module-text, var(--zc-text)); }
.detail-tag-label { color: var(--module-muted, var(--zc-muted)); }
.detail-link-button {
  background: var(--module-canvas, var(--zc-bg));
  color: var(--module-text, var(--zc-text));
  border-color: var(--module-line, var(--zc-line));
  box-shadow: 0 8px 18px color-mix(in srgb, var(--module-shadow-dark, var(--zc-shadow-dark)) 24%, transparent);
}
.detail-link-button:hover { border-color: var(--module-accent, var(--zc-accent)); }
.detail-stats { border-top-color: var(--module-line, var(--zc-line)); }
.detail-stat strong { color: var(--module-text-strong, var(--zc-text-strong)); }
.detail-stat span   { color: var(--module-subtle, var(--zc-subtle)); }

/* Submit form */
.submit-form-overlay { background: rgba(12, 24, 42, 0.48); }
.submit-form-panel {
  background: var(--module-canvas, var(--zc-bg));
  border-color: var(--module-line, var(--zc-line));
  box-shadow: 0 26px 72px rgba(8, 17, 31, 0.34), inset 0 1px 0 color-mix(in srgb, var(--module-shadow-light, var(--zc-shadow-light)) 52%, transparent);
}
.submit-form-header { border-bottom-color: var(--module-line, var(--zc-line)); }
.submit-form-header h2 { color: var(--module-text-strong, var(--zc-text-strong)); }
.form-row label { color: var(--module-muted, var(--zc-muted)); }
.form-row input[type="text"],
.form-row input[type="url"],
.form-row textarea,
.form-row select {
  background: var(--module-canvas, var(--zc-bg));
  border-color: var(--module-line, var(--zc-line));
  color: var(--module-text, var(--zc-text));
  box-shadow: inset 0 2px 8px color-mix(in srgb, var(--module-shadow-dark, var(--zc-shadow-dark)) 28%, transparent);
}
.form-row input:focus,
.form-row textarea:focus,
.form-row select:focus { border-color: var(--module-accent, var(--zc-accent)); }
.form-checkbox-label,
.form-radio-label { color: var(--module-text, var(--zc-text)); }
.form-submit-row { border-top-color: var(--module-line, var(--zc-line)); }

/* loading skeleton */
.asset-card-loading span,
.asset-card-loading strong,
.asset-card-loading p {
  background: color-mix(in srgb, var(--module-shadow-dark, var(--zc-shadow-dark)) 30%, var(--module-canvas, var(--zc-bg)));
}
</style>
