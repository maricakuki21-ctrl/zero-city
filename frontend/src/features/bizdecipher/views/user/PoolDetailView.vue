<template>
  <AppLayout>
    <div class="pd-page">

      <!-- BACK NAV -->
      <nav class="pd-back pd-space-nav">
        <RouterLink to="/account-square" class="pd-back-link pd-space-current">共享市场</RouterLink>
        <RouterLink to="/account-square/my" class="pd-back-link">用户中心</RouterLink>
        <RouterLink to="/account-square/owner" class="pd-back-link">池主中心</RouterLink>
      </nav>

      <!-- LOADING -->
      <div v-if="loading" class="pd-loading">
        <div class="pd-spinner" />
        <span>加载中…</span>
      </div>

      <!-- NOT FOUND -->
      <div v-else-if="!pool" class="pd-empty">
        <p>找不到该共享池，可能已下线或你没有访问权限。</p>
        <RouterLink to="/account-square" class="pd-btn pd-btn-primary">返回市场</RouterLink>
      </div>

      <template v-else>

        <!-- ─── HERO ─── -->
        <section class="pd-hero" :class="`pd-service-${serviceGrade.tone}`">
          <div class="pd-hero-grid">
            <div class="pd-hero-copy">
              <div class="pd-identity-row">
                <span class="pd-identity-chip">社区共享池</span>
                <span class="pd-service-grade" :class="`grade-${serviceGrade.tone}`">{{ serviceGrade.label }}</span>
                <span class="pd-status-chip" :class="`status-${pool.status}`">
                  <span class="pd-dot" />
                  {{ statusLabel }}
                </span>
              </div>

              <div class="pd-hero-left">
                <div class="pd-avatar">
                  <img v-if="pool.avatar_url" :src="pool.avatar_url" :alt="pool.name" class="pd-avatar-img" />
                  <span v-else class="pd-avatar-mark">{{ pool.name?.charAt(0) || '?' }}</span>
                </div>
                <div class="pd-hero-info">
                  <div class="pd-hero-name-row">
                    <h1 class="pd-pool-name">{{ pool.name }}</h1>
                  </div>
                  <p class="pd-pool-meta">池主：{{ pool.owner_label || '未公开' }} · 社区共享服务</p>
                  <div class="pd-tags">
                    <span v-if="joined" class="pd-tag pd-tag-joined">已加入</span>
                    <span v-if="observation" class="pd-tag pd-tag-watch">观察中 · 连续失败 {{ pool.consecutive_probe_failures || 0 }} 次</span>
                    <span class="pd-tag" :class="verificationTagClass">{{ verificationBadgeLabel }}</span>
                    <span v-if="cardPresentation" class="pd-tag pd-tag-card">
                      {{ cardPresentation.badgeLabel }} · {{ cardPresentation.name }}
                    </span>
                    <span v-if="pool.verification_mode === 'professional_review' && pool.verification_exemption_reason" class="pd-tag pd-tag-note">
                      {{ pool.verification_exemption_reason }}
                    </span>
                    <span v-for="m in (pool.models || []).slice(0,4)" :key="m" class="pd-tag">{{ m }}</span>
                  </div>
                </div>
              </div>

              <div class="pd-decision" :class="`decision-${serviceDecision.tone}`">
                <span class="pd-decision-kicker">平台综合判断</span>
                <h2>{{ serviceDecision.title }}</h2>
                <p>{{ serviceDecision.detail }}</p>
                <div class="pd-decision-facts" aria-label="判断依据">
                  <span><small>近7日可用率</small><strong>{{ sevenDayAvailabilityText }}</strong></span>
                  <span><small>平均延迟</small><strong>{{ latencyText }}</strong></span>
                  <span><small>剩余席位</small><strong>{{ remainingSeatsText }}</strong></span>
                </div>
              </div>

              <div class="pd-hero-actions">
                <button v-if="!joined" class="pd-btn pd-btn-primary" :disabled="acting" @click="handleJoin">
                  {{ acting ? '加入中…' : '加入共享池' }}
                </button>
                <button v-else class="pd-btn pd-btn-outline" :disabled="acting" @click="handleLeave">
                  {{ acting ? '退出中…' : '退出池子' }}
                </button>
                <button class="pd-btn pd-btn-outline" @click="goDiscussion">社区讨论</button>
              </div>
            </div>

            <aside class="pd-card-showcase" :class="{ 'has-card': cardPresentation && !cardImageFailed }">
              <template v-if="cardPresentation && !cardImageFailed">
                <img
                  class="pd-card-showcase-image"
                  :src="cardPresentation.imageUrl"
                  :alt="`${cardPresentation.name}收藏卡`"
                  @error="cardImageFailed = true"
                />
                <div class="pd-card-showcase-shade" />
                <div class="pd-card-showcase-top">
                  <span>{{ cardPresentation.badgeLabel }}</span>
                  <span>池主收藏身份</span>
                </div>
                <div class="pd-card-showcase-caption">
                  <small>{{ cardPresentation.faction || '零号城' }}</small>
                  <strong>{{ cardPresentation.name }}</strong>
                  <span>{{ cardPresentation.role || '收藏身份待补充' }}</span>
                </div>
              </template>
              <template v-else>
                <div class="pd-card-showcase-empty">
                  <img
                    class="pd-card-showcase-default-image"
                    src="/assets/zero-point-city/mascots/shared-pool-owner.png"
                    alt=""
                    aria-hidden="true"
                  />
                  <div class="pd-card-showcase-shade" />
                  <div class="pd-card-showcase-top">
                    <span>共享池公共主题</span>
                    <span>池主可替换</span>
                  </div>
                  <div class="pd-card-showcase-caption">
                    <small>零点城共享市场</small>
                    <strong>{{ pool.name }}</strong>
                    <span>获得收藏卡后，可在经营控制台更换专属背景</span>
                  </div>
                </div>
              </template>
            </aside>
          </div>
        </section>

        <div class="pd-detail-layout">
        <!-- ─── METRICS GRID ─── -->
        <section class="pd-section pd-section-metrics">
          <div class="pd-metrics-grid">
            <div class="pd-metric-card">
              <small>可用率</small>
              <strong :class="availClass">{{ formatPoolAvailability(pool.today_availability) }}</strong>
            </div>
            <div class="pd-metric-card">
              <small>7日可用率</small>
              <strong>{{ sevenDayAvailabilityText }}</strong>
            </div>
            <div class="pd-metric-card">
              <small>池级倍率</small>
              <strong>{{ formatSharedPoolMultiplier(pool.rate_multiplier) }}x</strong>
            </div>
            <div class="pd-metric-card">
              <small>延迟</small>
              <strong>{{ latencyText }}</strong>
            </div>
            <div class="pd-metric-card">
              <small>并发席位</small>
              <strong>{{ pool.current_users ?? 0 }}/{{ pool.max_users ?? '—' }}</strong>
            </div>
            <div class="pd-metric-card">
              <small>席位费</small>
              <strong>{{ pool.hourly_seat_fee ? `${pool.hourly_seat_fee}/小时` : '无席位费' }}</strong>
            </div>
            <div class="pd-metric-card">
              <small>免费门槛</small>
              <strong>{{ pool.hourly_min_usage_waiver ? pool.hourly_min_usage_waiver : '未开启' }}</strong>
            </div>
            <div class="pd-metric-card">
              <small>最低余额</small>
              <strong>{{ pool.min_balance_admission ?? '无要求' }}</strong>
            </div>
          </div>

          <!-- health bar -->
          <div class="pd-health">
            <div class="pd-health-head">
              <span>快速判断</span>
              <span :class="healthTone">{{ healthLabel }}</span>
            </div>
            <div class="pd-bar-track"><div class="pd-bar-fill" :class="barClass" :style="{ width: barWidth }" /></div>
            <p class="pd-health-note">{{ healthNote }}</p>
          </div>
        </section>

        <!-- ─── COLLECTIBLE CARD ARCHIVE ─── -->
        <section v-if="cardPresentation" class="pd-section pd-card-archive">
          <div class="pd-section-head">
            <div>
              <span class="pd-card-archive-kicker">零号城收藏身份</span>
              <h2 class="pd-section-title">收藏卡档案</h2>
            </div>
            <span class="pd-card-rarity-badge">{{ cardPresentation.badgeLabel }}</span>
          </div>
          <div class="pd-card-archive-grid">
            <dl class="pd-card-archive-facts">
              <div><dt>卡片名称</dt><dd>{{ cardPresentation.name }}</dd></div>
              <div><dt>所属阵营</dt><dd>{{ cardPresentation.faction || '零号城' }}</dd></div>
              <div><dt>卡片身份</dt><dd>{{ cardPresentation.role || '档案待补充' }}</dd></div>
              <div><dt>收藏等级</dt><dd>{{ cardPresentation.rarityLabel }}</dd></div>
            </dl>
            <div class="pd-card-archive-story">
              <blockquote>{{ cardPresentation.line || '这张收藏卡还没有公开台词。' }}</blockquote>
              <p>{{ cardPresentation.lore || '这张收藏卡的背景故事仍在整理中。' }}</p>
            </div>
          </div>
        </section>

        <!-- ─── MY KEY ─── -->
        <section class="pd-section pd-key-panel" v-if="joined">
          <h2 class="pd-section-title">共享池组合 Key</h2>
          <div v-if="poolKeys.length === 0" class="pd-empty-sm">
            此池尚未绑定到你的组合 Key。
            <button class="pd-btn pd-btn-sm" :disabled="creatingKey" @click="handleCreateKey">
              {{ creatingKey ? '绑定中…' : '绑定到组合 Key' }}
            </button>
          </div>
          <div v-else class="pd-key-list">
            <div v-for="k in poolKeys" :key="k.id" class="pd-key-row">
              <code class="pd-key-code">{{ k.key || k.key_preview }}</code>
              <span class="pd-key-models">{{ k.allowed_models?.join(', ') || '池内开放模型' }}</span>
              <button class="pd-btn pd-btn-sm" :disabled="!k.key" @click="copyKey(k)">复制完整 Key</button>
            </div>
            <RouterLink to="/account-square/my" class="pd-btn pd-btn-outline pd-btn-sm">
              管理组合 Key
            </RouterLink>
          </div>
        </section>

        <section v-else class="pd-section pd-key-panel pd-join-card">
          <h2 class="pd-section-title">加入后可绑定组合 Key</h2>
          <p>共享池组合 Key 与普通 API Key 分开管理；同一把 <code>sk-share</code> 可绑定多个已加入池，并按模型与服务状态选择命中池。</p>
          <button class="pd-btn pd-btn-primary" :disabled="acting" @click="handleJoin">
            {{ acting ? '加入中…' : '加入共享池' }}
          </button>
        </section>

        <!-- ─── MODELS & PRICING ─── -->
        <section class="pd-section">
          <div class="pd-section-head">
            <h2 class="pd-section-title">可用服务与实际价格</h2>
            <button class="pd-btn pd-btn-sm" type="button" :disabled="pricingLoading" @click="loadPricing">
              {{ pricingLoading ? '读取中…' : '刷新价格' }}
            </button>
          </div>
          <div class="pd-model-list">
            <div v-for="m in (pool.models || [])" :key="m" class="pd-model-chip">{{ m }}</div>
            <span v-if="!pool.models?.length" class="pd-empty-sm">暂无模型信息</span>
          </div>
          <div v-if="pricingLoading && modelPricing.length === 0" class="pd-empty-sm">正在核对每个服务的价格和检测状态…</div>
          <div v-else-if="pricingLoadFailed" class="pd-price-warning">
            当前价格暂时读取失败。为避免误导，页面不会把未知价格显示成免费。
            <button class="pd-btn pd-btn-sm" type="button" @click="loadPricing">重新读取</button>
          </div>
          <div v-else-if="modelPricing.length === 0" class="pd-price-warning">当前没有完整服务价格，系统不会把调用按 0 元放行。</div>
          <div v-else class="pd-price-list">
            <article v-for="item in modelPricing" :key="`${item.pool_model_id}:${item.endpoint_type}`" class="pd-price-row">
              <div class="pd-price-row-head">
                <div class="pd-price-name">
                  <strong>{{ item.display_name || item.model_name }}</strong>
                  <span>{{ endpointLabel(item.endpoint_type) }}</span>
                </div>
                <div class="pd-price-badges">
                  <span class="pd-endpoint-state" :class="`is-${endpointState(item).tone}`">{{ endpointState(item).label }}</span>
                  <span class="pd-price-source">{{ item.pricing_source === 'official_catalog' ? '平台官方价' : '池主自定义价' }}</span>
                </div>
              </div>
              <p class="pd-endpoint-reason">{{ endpointState(item).description }}</p>
              <template v-if="item.current_price && endpointHasCompatiblePrice(item)">
                <div class="pd-price-facts">
                  <div><small>基础单价</small><strong>{{ formatPriceComponents(item.current_price.base_price) }}</strong></div>
                  <div><small>池主倍率</small><strong>{{ formatSharedPoolMultiplier(item.current_price.multiplier) }} 倍</strong></div>
                  <div v-if="item.current_price.owner_price"><small>池主原价（含倍率）</small><strong>{{ formatPriceComponents(item.current_price.owner_price) }}</strong></div>
                  <div v-if="item.current_price.fee_mode === 'buyer_surcharge_v1'"><small>平台附加费</small><strong>原价 × {{ item.current_price.platform_fee_percent ?? 0 }}%</strong></div>
                  <div><small>你实际支付</small><strong>{{ formatPriceComponents(item.current_price.user_price) }}</strong></div>
                  <div><small>生效时间</small><strong>{{ formatPriceTime(item.current_price.effective_from) }}</strong></div>
                </div>
                <p class="pd-price-example">{{ pricingExampleLabel(item) }}，预计 {{ formatMoney(item.current_price.example_cost) }}。</p>
              </template>
              <div v-else class="pd-price-unavailable">价格还没补完整，这个服务暂不可调用，也不会按免费处理。</div>
            </article>
          </div>
          <div class="pd-pricing-note">
            <p v-if="pool.min_balance_admission">入池需余额 ≥ {{ pool.min_balance_admission }}</p>
            <p>应付价格已含池主倍率及平台附加费。平台费加在池主原价之上，不从池主收入中扣除；历史账单沿用原规则。</p>
            <p>平台官方图片价以 1K、视频价以 480p 作为页面浏览基准；请求使用其他规格时，系统会按真实规格冻结当次价格，不会拿默认规格冒充最终账单。</p>
            <p>席位费按整小时结算：{{ pool.hourly_seat_fee ? `${pool.hourly_seat_fee}/小时` : '无席位费' }}；API 调用仍按上方实际单价扣费。满 1 小时后入账，连续 2 小时无真实 API 调用会自动释放席位。</p>
            <p>共享池扣费请到「我的共享池 → 消费账本」查看；「使用记录」只展示普通网关请求，不含席位费。</p>
          </div>
        </section>

        <!-- ─── SEAT RULES ─── -->
        <section class="pd-section">
          <h2 class="pd-section-title">席位规则</h2>
          <div class="pd-rules-grid">
            <div class="pd-rule-item">
              <span>并发席位</span>
              <strong>{{ pool.max_users }}</strong>
            </div>
            <div class="pd-rule-item">
              <span>当前使用中</span>
              <strong>{{ pool.current_users }}</strong>
            </div>
            <div class="pd-rule-item">
              <span>席位费</span>
              <strong>{{ pool.hourly_seat_fee ? `${pool.hourly_seat_fee}/小时` : '无席位费（API 仍计费）' }}</strong>
            </div>
            <div class="pd-rule-item">
              <span>退出规则</span>
              <strong>随时退出，已产生费用按规则处理</strong>
            </div>
            <div class="pd-rule-item">
              <span>闲置释放</span>
              <strong>连续 2 小时无调用自动释放</strong>
            </div>
          </div>
        </section>

        <!-- ─── FULL CHECK REPORT ─── -->
        <section class="pd-section">
          <div class="pd-section-head">
            <h2 class="pd-section-title">满血检测报告</h2>
            <button class="pd-btn pd-btn-sm" type="button" :disabled="reportLoading" @click="loadFullCheckReport">
              {{ reportLoading ? '加载中…' : '刷新报告' }}
            </button>
          </div>
          <div v-if="reportLoading && !fullCheckReport" class="pd-empty-sm">正在加载满血检测报告…</div>
          <div v-else-if="reportLoadFailed" class="pd-empty-sm">
            满血检测报告暂时无法加载。
            <button class="pd-btn pd-btn-sm" type="button" @click="loadFullCheckReport">重试</button>
          </div>
          <div v-else-if="verificationMode === 'professional_review'" class="pd-empty-sm">
            此池按专业核验方式公开：{{ pool?.verification_exemption_reason || '已登记差异化模型专业核验说明' }}。
          </div>
          <div v-else-if="pool.account_mode_enabled && !fullCheckReport?.has_full_check" class="pd-empty-sm">
            多账号池按账号分别检测：当前通过 {{ pool.account_summary?.full_check_passed_accounts || 0 }}/{{ pool.account_summary?.full_check_total_accounts || 0 }} 个，
            {{ pool.account_summary?.schedulable_accounts || 0 }} 个可调度。只有通过检测且当前可用的账号会进入路由。
          </div>
          <div v-else-if="!fullCheckReport?.has_full_check" class="pd-empty-sm">
            此池尚未完成满血检测，或最近检测记录中没有完整报告。
          </div>
          <div v-else class="pd-report">
            <div class="pd-report-summary">
              <span><small>得分</small><strong>{{ Math.round(fullCheckReport.score || 0) }}</strong></span>
              <span><small>通过项</small><strong>{{ fullCheckReport.passed || 0 }}/{{ fullCheckReport.total || 0 }}</strong></span>
              <span><small>门槛</small><strong>{{ fullCheckReport.gate_passed ? '已通过' : '未通过' }}</strong></span>
              <span><small>模型</small><strong>{{ fullCheckReport.model || '—' }}</strong></span>
              <span><small>检测时间</small><strong>{{ formatReportTime(fullCheckReport.checked_at) }}</strong></span>
            </div>
            <div v-if="(fullCheckReport.checks || []).length" class="pd-check-list">
              <div
                v-for="item in fullCheckReport.checks"
                :key="item.id"
                class="pd-check-item"
                :class="item.success ? 'is-pass' : 'is-fail'"
              >
                <div class="pd-check-main">
                  <strong>{{ item.title || item.id }}</strong>
                  <span>{{ item.category || 'check' }}{{ item.required ? ' · 必检' : '' }}</span>
                </div>
                <div class="pd-check-meta">
                  <em>{{ item.success ? '通过' : '未通过' }}</em>
                  <span v-if="item.latency_ms">{{ item.latency_ms }}ms</span>
                </div>
              </div>
            </div>
          </div>
        </section>

        <!-- ─── COMMUNITY ─── -->
        <section class="pd-section">
          <h2 class="pd-section-title">社区讨论</h2>
          <div v-if="communityLoadFailed" class="pd-empty-sm">
            社区讨论暂时无法加载。
            <button class="pd-btn pd-btn-sm" @click="loadCommunity">重试</button>
            <button class="pd-btn pd-btn-sm" @click="goDiscussion">进入社区</button>
          </div>
          <div v-else-if="communitySummary.discussion_posts === 0" class="pd-empty-sm">
            还没有关于此池的讨论。
            <button class="pd-btn pd-btn-sm" @click="goDiscussion">发起讨论</button>
          </div>
          <div v-else class="pd-community-preview">
            <div class="pd-community-stats">
              <span>{{ communitySummary.discussion_posts }} 条讨论</span>
              <span>{{ communitySummary.feedback_posts }} 条反馈</span>
              <span v-if="communitySummary.risk_signals > 0" class="pd-risk">⚠ {{ communitySummary.risk_signals }} 条风险</span>
            </div>
            <p v-if="communitySummary.last_post_title" class="pd-latest-post">{{ communitySummary.last_post_title }}</p>
            <button class="pd-btn pd-btn-outline" @click="goDiscussion">查看全部讨论 →</button>
          </div>
        </section>

        <!-- ─── LIKE / REPORT ─── -->
        <div class="pd-footer-actions">
          <button
            class="pd-like-btn"
            type="button"
            :class="{ liked: liked }"
            :disabled="likeActing"
            @click="handleLike"
          >
            <span class="pd-heart" aria-hidden="true">♥</span>
            <span>{{ liked ? '已喜欢' : '喜欢' }}</span>
            <em>{{ likeCount }}</em>
          </button>
          <button class="pd-discuss-btn" type="button" @click="goDiscussion">
            <span>去讨论</span>
          </button>
          <button class="pd-report-btn" type="button" @click="handleReport">
            <span>投诉</span>
          </button>
        </div>
        </div>

      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter, RouterLink } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import {
  formatSharedPoolMultiplier,
  sharedPoolBillingExampleLabel,
  sharedPoolEndpointAvailability,
  sharedPoolEndpointHasCompatiblePrice,
  sharedPoolEndpointLabel,
} from '@/features/bizdecipher/components/shared-pool/sharedPoolPricing'
import {
  resolveSharedPoolCardPresentation,
  sharedPoolDecision,
  sharedPoolServiceGrade,
} from '@/features/bizdecipher/components/shared-pool/sharedPoolPresentation'
import { useAppStore } from '@/stores/app'
import { useClipboard } from '@/composables/useClipboard'
import {
  getSharedPool,
  getSharedPoolFullCheckReport,
  listSharedPoolModelPricing,
  joinSharedPool,
  leaveSharedPool,
  likeSharedPool,
  unlikeSharedPool,
  createSharedPoolAccessKey,
  listMySharedPoolAccessKeys,
  listMySeats,
  reportSharedPool,
  type SharedPool,
  type SharedPoolAccessKey,
  type SharedPoolFullCheckReport,
  type SharedPoolModelEndpointPricing,
  type SharedPoolPriceComponents,
} from '@/features/bizdecipher/api/bizdecipher'
import { communityAPI, type SharedPoolCommunitySummary } from '@/features/bizdecipher/api/community'
import { emptyPoolCommunitySummary } from '@/composables/useSharedPool'

const route = useRoute()
const router = useRouter()
const appStore = useAppStore()
const { copyToClipboard } = useClipboard()

const poolId = computed(() => Number(route.params.id))
const pool = ref<SharedPool | null>(null)
const loading = ref(true)
const acting = ref(false)
const creatingKey = ref(false)
const joined = ref(false)
const liked = ref(false)
const likeCount = ref(0)
const likeActing = ref(false)
const poolKeys = ref<SharedPoolAccessKey[]>([])
const modelPricing = ref<SharedPoolModelEndpointPricing[]>([])
const pricingLoading = ref(true)
const pricingLoadFailed = ref(false)
const communityLoadFailed = ref(false)
const communitySummary = ref<SharedPoolCommunitySummary>(emptyPoolCommunitySummary(poolId.value))
const fullCheckReport = ref<SharedPoolFullCheckReport | null>(null)
const reportLoading = ref(false)
const reportLoadFailed = ref(false)
const cardImageFailed = ref(false)

// ── Load ──────────────────────────────────────────────────────
onMounted(async () => {
  await Promise.all([loadPool(), loadSeatState(), loadKeys(), loadCommunity(), loadFullCheckReport(), loadPricing()])
})

async function loadPool(): Promise<void> {
  loading.value = true
  try {
    pool.value = await getSharedPool(poolId.value)
    cardImageFailed.value = false
    liked.value = Boolean(pool.value?.liked_by_me)
    likeCount.value = Number(pool.value?.like_count || 0)
  } catch { pool.value = null }
  finally { loading.value = false }
}

async function loadKeys(): Promise<void> {
  try {
    const all = await listMySharedPoolAccessKeys()
    poolKeys.value = all.filter(k => k.pool_id === poolId.value)
  } catch { poolKeys.value = [] }
}

async function loadPricing(): Promise<void> {
  pricingLoading.value = true
  pricingLoadFailed.value = false
  try {
    modelPricing.value = await listSharedPoolModelPricing(poolId.value)
  } catch {
    pricingLoadFailed.value = true
    modelPricing.value = []
  } finally {
    pricingLoading.value = false
  }
}

async function loadSeatState(): Promise<void> {
  try {
    const seats = await listMySeats()
    joined.value = seats.some((seat) => seat.pool_id === poolId.value && ['active', 'held'].includes(String(seat.status).toLowerCase()))
  } catch {
    joined.value = false
  }
}

async function loadCommunity(): Promise<void> {
  try {
    communityLoadFailed.value = false
    const res = await communityAPI.listSharedPoolSummaries([poolId.value], 5)
    const found = res.items?.find(s => s.pool_id === poolId.value)
    communitySummary.value = found || emptyPoolCommunitySummary(poolId.value)
  } catch {
    communityLoadFailed.value = true
    communitySummary.value = emptyPoolCommunitySummary(poolId.value)
  }
}

async function loadFullCheckReport(): Promise<void> {
  reportLoading.value = true
  reportLoadFailed.value = false
  try {
    fullCheckReport.value = await getSharedPoolFullCheckReport(poolId.value)
  } catch {
    reportLoadFailed.value = true
    fullCheckReport.value = null
  } finally {
    reportLoading.value = false
  }
}

// ── Actions ──────────────────────────────────────────────────
async function handleJoin(): Promise<void> {
  const current = pool.value
  const fee = Number(current?.hourly_seat_fee || 0)
  const waiver = Number(current?.hourly_min_usage_waiver || 0)
  const feeText = fee > 0 ? `${fee.toFixed(4)}/小时` : '0（无席位费；API 调用仍单独扣费）'
  const waiverText = waiver > 0 ? `；当小时 API 消费达到 ${waiver.toFixed(4)} 可全额抵扣席位费` : ''
  const ok = window.confirm(
    `确认加入「${current?.name || '共享池'}」？

` +
    `1) 席位费按整小时结算：${feeText}${waiverText}
` +
    `2) 加入后至少满 1 小时才会结算一笔席位费；未调用 API 也可能产生席位费
` +
    `3) 席位费与共享池 API 扣费记在「共享市场 → 我的共享池 → 消费账本」，不会出现在「使用记录」页
` +
    `4) 连续 2 小时无真实 API 调用，系统会自动释放席位
` +
    `5) 若不想继续占用席位，请及时退出，避免空占扣费`
  )
  if (!ok) return
  acting.value = true
  try {
    await joinSharedPool(poolId.value)
    joined.value = true
    await Promise.all([loadSeatState(), loadKeys(), loadPool()])
    appStore.showSuccess('已加入共享池。席位费与共享池扣费请到「我的共享池 → 消费账本」查看。')
  } catch (e: unknown) {
    appStore.showError((e as {message?:string})?.message || '加入失败')
  } finally { acting.value = false }
}

async function handleLeave(): Promise<void> {
  acting.value = true
  try {
    await leaveSharedPool(poolId.value)
    joined.value = false
    poolKeys.value = []
    await Promise.all([loadSeatState(), loadPool()])
    appStore.showSuccess('已退出')
  } catch (e: unknown) {
    appStore.showError((e as {message?:string})?.message || '退出失败')
  } finally { acting.value = false }
}

async function handleCreateKey(): Promise<void> {
  creatingKey.value = true
  try {
    const result = await createSharedPoolAccessKey(poolId.value, `${pool.value?.name || '共享池'} 专属 Key`)
    poolKeys.value = [result.access_key, ...poolKeys.value]
    joined.value = true
    appStore.showSuccess('Key 已生成')
  } catch (e: unknown) {
    appStore.showError((e as {message?:string})?.message || '生成失败')
  } finally { creatingKey.value = false }
}

async function copyKey(k: SharedPoolAccessKey): Promise<void> {
  if (!k.key) return
  await copyToClipboard(k.key)
  appStore.showSuccess('Key 已复制')
}

async function handleReport(): Promise<void> {
  const reason = window.prompt('请描述投诉原因（可选）：')
  if (reason === null) return
  try {
    await reportSharedPool(poolId.value, reason || undefined)
    appStore.showSuccess('投诉已提交')
  } catch { appStore.showError('提交失败') }
}

async function handleLike(): Promise<void> {
  if (likeActing.value) return
  likeActing.value = true
  try {
    const updated = liked.value
      ? await unlikeSharedPool(poolId.value)
      : await likeSharedPool(poolId.value)
    liked.value = Boolean(updated.liked_by_me)
    likeCount.value = Number(updated.like_count || 0)
    if (pool.value) {
      pool.value.liked_by_me = liked.value
      pool.value.like_count = likeCount.value
    }
    appStore.showSuccess(liked.value ? '已喜欢该共享池' : '已取消喜欢')
  } catch (e: unknown) {
    appStore.showError((e as { message?: string })?.message || '操作失败')
  } finally {
    likeActing.value = false
  }
}

function goDiscussion(): void {
  router.push(`/community?source_type=shared_pool&source_id=${poolId.value}&subject_type=shared_pool&subject_id=${poolId.value}`)
}

function endpointLabel(endpoint: string): string {
  return sharedPoolEndpointLabel(endpoint)
}

function endpointState(item: SharedPoolModelEndpointPricing) {
  return sharedPoolEndpointAvailability(item)
}

function endpointHasCompatiblePrice(item: SharedPoolModelEndpointPricing): boolean {
  return sharedPoolEndpointHasCompatiblePrice(item)
}

function billingExampleLabel(billingMode: string): string {
  return sharedPoolBillingExampleLabel(billingMode)
}

function pricingExampleLabel(item: SharedPoolModelEndpointPricing): string {
  const billingMode = item.current_price?.base_price.billing_mode || ''
  if (item.pricing_source === 'official_catalog' && billingMode === 'image') return '示例基准：1K 图片 1 张'
  if (item.pricing_source === 'official_catalog' && billingMode === 'video') return '示例基准：480p、默认 8 秒视频'
  return billingExampleLabel(billingMode)
}

function formatMoney(value: number | null | undefined): string {
  const amount = Number(value || 0)
  if (amount === 0) return '$0.00'
  if (amount < 0.0001) return `$${amount.toFixed(8)}`
  if (amount < 0.01) return `$${amount.toFixed(5)}`
  return `$${amount.toFixed(4)}`
}

function formatPriceComponents(price: SharedPoolPriceComponents): string {
  if (price.billing_mode === 'per_request') return `${formatMoney(price.per_request_price)} / 次`
  if (price.billing_mode === 'image') return `${formatMoney(price.image_item_price)} / 张`
  if (price.billing_mode === 'video') return `${formatMoney(price.video_second_price)} / 秒`
  const input = formatMoney(Number(price.input_price || 0) * 1_000_000)
  const output = formatMoney(Number(price.output_price || 0) * 1_000_000)
  return `输入 ${input} / 百万 token · 输出 ${output} / 百万 token`
}

function formatPriceTime(value: string): string {
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '待生效' : date.toLocaleString('zh-CN', { hour12: false })
}

function formatReportTime(value?: string): string {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString()
}

// ── Computed ──────────────────────────────────────────────────
const observation = computed(() => String(pool.value?.governance_status || '') === 'watch' && Number(pool.value?.consecutive_probe_failures || 0) >= 3)

const cardPresentation = computed(() => resolveSharedPoolCardPresentation({
  cardKey: pool.value?.card_skin_key,
  cardRarity: pool.value?.card_skin_rarity,
  fallbackCardKey: pool.value?.owner_card_asset?.featured_card_key,
  fallbackCardRarity: pool.value?.owner_card_asset?.featured_card_rarity,
}))

const hasServiceEvidence = computed(() => Boolean(
  pool.value?.last_probe_at ||
  pool.value?.last_successful_probe_at ||
  Number(pool.value?.last_probe_full_check_total || 0) > 0 ||
  Number(pool.value?.account_summary?.total_calls || 0) > 0 ||
  Number(pool.value?.seven_day_availability || 0) > 0
))

const serviceGrade = computed(() => sharedPoolServiceGrade({
  status: pool.value?.status,
  observation: observation.value,
  todayAvailability: pool.value?.today_availability,
  sevenDayAvailability: pool.value?.seven_day_availability,
  avgLatencyMs: pool.value?.avg_latency_ms,
  hasEvidence: hasServiceEvidence.value,
}))

const serviceDecision = computed(() => sharedPoolDecision({
  status: pool.value?.status,
  observation: observation.value,
  todayAvailability: pool.value?.today_availability,
  sevenDayAvailability: pool.value?.seven_day_availability,
  avgLatencyMs: pool.value?.avg_latency_ms,
  hasEvidence: hasServiceEvidence.value,
}))

const statusLabel = computed(() => {
  if (observation.value) return '观察中'
  return ({
    healthy: '当前可用', limited: '受限', offline: '已下线', maintenance: '维护中',
  } as Record<string, string>)[pool.value?.status ?? ''] ?? pool.value?.status ?? ''
})

function formatPoolAvailability(value?: number | null): string {
  if (value == null || Number.isNaN(Number(value))) return '—'
  const normalized = Number(value)
  const percent = normalized <= 1 ? normalized * 100 : normalized
  return `${Math.max(0, Math.min(100, percent)).toFixed(1)}%`
}

const availClass = computed(() => {
  const raw = pool.value?.today_availability ?? 0
  const v = raw <= 1 ? raw * 100 : raw
  if (v >= 99) return 'good'
  if (v >= 95) return 'warn'
  return 'bad'
})

const latencyText = computed(() => {
  const l = pool.value?.avg_latency_ms
  if (!l) return '—'
  return l < 1000 ? `${Math.round(l)}ms` : `${(l/1000).toFixed(1)}s`
})

const sevenDayAvailabilityText = computed(() => formatPoolAvailability(pool.value?.seven_day_availability))
const remainingSeatsText = computed(() => {
  const maximum = Number(pool.value?.max_users || 0)
  if (maximum <= 0) return '不限'
  return `${Math.max(0, maximum - Number(pool.value?.current_users || 0))} 席`
})

const verificationMode = computed(() => String(pool.value?.verification_mode || '').trim() || 'full_check')

function accountPoolFullCheckReady(target: SharedPool | null | undefined): boolean {
  if (!target?.account_mode_enabled) return false
  const summary = target.account_summary
  return (summary?.full_check_total_accounts ?? 0) > 0
    && (summary?.full_check_passed_accounts ?? 0) > 0
    && (summary?.schedulable_accounts ?? 0) > 0
}

function fullCheckReadyForDisplay(target: SharedPool | null | undefined): boolean {
  if (!target) return false
  if (target.verification_mode === 'professional_review') {
    return target.account_mode_enabled
      ? (target.account_summary?.schedulable_accounts ?? 0) > 0
      : Boolean(target.last_probe_gate_passed || target.last_probe_success)
  }
  if (target.account_mode_enabled) return accountPoolFullCheckReady(target)
  return Boolean(target.last_probe_gate_passed)
    && (target.last_probe_full_check_score ?? 0) >= 70
    && (target.last_probe_full_check_total ?? 0) > 0
}

function fullCheckScoreForDisplay(target: SharedPool | null | undefined): number {
  if (!target) return 0
  return target.account_mode_enabled
    ? Number(target.account_summary?.average_full_check_score || 0)
    : Number(target.last_probe_full_check_score || 0)
}

function fullCheckPassedForDisplay(target: SharedPool | null | undefined): number {
  if (!target) return 0
  return target.account_mode_enabled
    ? Number(target.account_summary?.full_check_passed_accounts || 0)
    : Number(target.last_probe_full_check_passed || 0)
}

function fullCheckTotalForDisplay(target: SharedPool | null | undefined): number {
  if (!target) return 0
  return target.account_mode_enabled
    ? Number(target.account_summary?.full_check_total_accounts || 0)
    : Number(target.last_probe_full_check_total || 0)
}

const verificationBadgeLabel = computed(() => {
  if (verificationMode.value === 'professional_review') return '专业核验'
  return pool.value?.account_mode_enabled
    ? (accountPoolFullCheckReady(pool.value) ? '账号池可调度' : '待账号检测')
    : (fullCheckReadyForDisplay(pool.value) ? '满血验证' : '待满血验证')
})
const verificationTagClass = computed(() => {
  if (verificationMode.value === 'professional_review') return 'pd-tag-professional'
  return fullCheckReadyForDisplay(pool.value) ? 'pd-tag-verified' : 'pd-tag-pending'
})

const healthNote = computed(() => {
  if (!pool.value) return ''
  if (verificationMode.value === 'professional_review') {
    return pool.value.verification_exemption_reason || '差异化模型采用专业核验后上架'
  }
  if (pool.value.account_mode_enabled) {
    const p = fullCheckPassedForDisplay(pool.value)
    const t = fullCheckTotalForDisplay(pool.value)
    const schedulable = pool.value.account_summary?.schedulable_accounts ?? 0
    if (t > 0) {
      return accountPoolFullCheckReady(pool.value)
        ? `账号池检测通过 · ${p}/${t} 个账号通过，${schedulable} 个当前可调度`
        : `账号池检测未达到公开门槛 · ${p}/${t} 个账号通过，${schedulable} 个当前可调度`
    }
    return schedulable > 0 ? `当前有 ${schedulable} 个账号可调度，等待满血检测证据` : '等待账号满血检测；未通过检测的账号不会进入路由。'
  }
  const p = fullCheckPassedForDisplay(pool.value)
  const t = fullCheckTotalForDisplay(pool.value)
  if (t > 0) {
    return pool.value.last_probe_gate_passed
      ? `满血检测已通过发布闸门 · ${p}/${t} 项通过`
      : `满血检测未通过发布闸门 · ${p}/${t} 项通过`
  }
  if (pool.value.last_probe_success) return '基础连通已通过；尚未完成满血检测，不能当作满血通过。'
  if ((pool.value.consecutive_probe_failures ?? 0) > 0) return `连续 ${pool.value.consecutive_probe_failures} 次检测失败`
  return '等待检测；当前没有可用于判断满血能力的证据。'
})

const healthTone = computed(() => {
  if (verificationMode.value === 'professional_review') return fullCheckReadyForDisplay(pool.value) ? 'tone-good' : 'tone-warn'
  if (pool.value?.account_mode_enabled) {
    if (accountPoolFullCheckReady(pool.value)) return 'tone-good'
    return fullCheckTotalForDisplay(pool.value) > 0 ? 'tone-bad' : 'tone-pending'
  }
  const score = fullCheckScoreForDisplay(pool.value)
  const total = fullCheckTotalForDisplay(pool.value)
  if (total > 0) {
    if (fullCheckReadyForDisplay(pool.value)) return 'tone-good'
    return score >= 60 ? 'tone-warn' : 'tone-bad'
  }
  if (pool.value?.last_probe_success) return 'tone-warn'
  return (pool.value?.consecutive_probe_failures ?? 0) > 0 ? 'tone-bad' : 'tone-pending'
})

const healthLabel = computed(() => {
  if (verificationMode.value === 'professional_review') return fullCheckReadyForDisplay(pool.value) ? '专业核验通过' : '等待专业核验'
  if (pool.value?.account_mode_enabled) {
    const total = fullCheckTotalForDisplay(pool.value)
    if (total > 0) return accountPoolFullCheckReady(pool.value) ? '账号池可调度' : '账号检测未达标'
    return (pool.value.account_summary?.schedulable_accounts ?? 0) > 0 ? '基础可调度' : '等待账号检测'
  }
  const score = fullCheckScoreForDisplay(pool.value)
  const total = fullCheckTotalForDisplay(pool.value)
  if (total > 0) return fullCheckReadyForDisplay(pool.value) ? '满血检测通过' : (score >= 60 ? '满血部分通过' : '满血检测未通过')
  if (pool.value?.last_probe_success) return '基础连通'
  if ((pool.value?.consecutive_probe_failures ?? 0) > 0) return '检测未通过'
  return '等待检测'
})

const barWidth = computed(() => {
  if (verificationMode.value === 'professional_review') return fullCheckReadyForDisplay(pool.value) ? '100%' : '50%'
  const score = fullCheckScoreForDisplay(pool.value)
  const total = fullCheckTotalForDisplay(pool.value)
  if (total > 0) return `${Math.min(100, Math.max(0, score))}%`
  return pool.value?.last_probe_success ? '35%' : '0%'
})

const barClass = computed(() => {
  if (verificationMode.value === 'professional_review') return fullCheckReadyForDisplay(pool.value) ? 'fill-good' : 'fill-warn'
  const score = fullCheckScoreForDisplay(pool.value)
  const total = fullCheckTotalForDisplay(pool.value)
  if (total > 0) {
    if (fullCheckReadyForDisplay(pool.value)) return 'fill-good'
    return score >= 60 ? 'fill-warn' : 'fill-bad'
  }
  if (pool.value?.last_probe_success) return 'fill-warn'
  return (pool.value?.consecutive_probe_failures ?? 0) > 0 ? 'fill-bad' : 'fill-pending'
})

</script>

<style scoped>
.pd-page {
  --bg: var(--module-canvas, var(--zc-bg));
  --nd: var(--module-shadow-dark, var(--zc-shadow-dark));
  --nl: var(--module-shadow-light, var(--zc-shadow-light));
  --text: var(--module-text-strong, var(--zc-text-strong));
  --muted: var(--module-muted, var(--zc-muted));
  --teal: var(--module-accent, var(--zc-accent));
  --gold: var(--module-accent-2, var(--zc-accent-2));
  --raise: 0 18px 44px color-mix(in srgb, var(--nd) 42%, transparent), inset 0 1px 0 color-mix(in srgb, var(--nl) 70%, transparent);
  --raise-sm: 0 10px 24px color-mix(in srgb, var(--nd) 32%, transparent), inset 0 1px 0 color-mix(in srgb, var(--nl) 66%, transparent);
  --inset: inset 0 3px 12px color-mix(in srgb, var(--nd) 34%, transparent), inset 0 -1px 0 color-mix(in srgb, var(--nl) 46%, transparent);
  --inset-sm: inset 0 2px 8px color-mix(in srgb, var(--nd) 30%, transparent), inset 0 -1px 0 color-mix(in srgb, var(--nl) 42%, transparent);
  max-width: 960px; margin: 0 auto; padding: 24px 24px 80px;
  display: flex; flex-direction: column; gap: 20px; color: var(--text);
}
:root.theme-daylight .pd-page {
  --bg: var(--module-canvas, var(--zc-bg));
  --nd: var(--module-shadow-dark, var(--zc-shadow-dark));
  --nl: var(--module-shadow-light, var(--zc-shadow-light));
  --text: var(--module-text-strong, var(--zc-text-strong));
  --muted: var(--module-muted, var(--zc-muted));
  --teal: var(--module-accent, var(--zc-accent));
  --gold: var(--module-accent-2, var(--zc-accent-2));
}

/* BACK */
.pd-back-link {
  font-size: 13px; font-weight: 700; color: var(--muted);
  text-decoration: none; transition: color .15s;
}
.pd-space-nav { display: flex; flex-wrap: wrap; gap: 8px; margin-bottom: 14px; }
.pd-space-nav .pd-back-link { min-height: 34px; display: inline-flex; align-items: center; padding: 0 12px; border-radius: 10px; background: var(--bg); box-shadow: var(--inset); }
.pd-space-nav .pd-space-current { color: var(--teal); box-shadow: 0 0 0 2px color-mix(in srgb, var(--teal) 24%, transparent), var(--inset); }
.pd-back-link:hover { color: var(--text); }

/* LOADING / EMPTY */
.pd-loading { display: flex; align-items: center; gap: 10px; padding: 60px; justify-content: center; color: var(--muted); }
.pd-spinner { width: 24px; height: 24px; border-radius: 999px; border: 2px solid var(--nd); border-top-color: var(--teal); animation: spin .8s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }
.pd-empty { text-align: center; padding: 60px 24px; color: var(--muted); display: flex; flex-direction: column; align-items: center; gap: 16px; }
.pd-empty-sm { font-size: 13px; color: var(--muted); padding: 16px 0; display: flex; align-items: center; gap: 10px; flex-wrap: wrap; }

/* HERO */
.pd-hero {
  border-radius: 28px; padding: 28px;
  background: var(--bg); box-shadow: var(--raise);
  position: relative; overflow: hidden;
}
.pd-hero::before {
  content: '';
  position: absolute;
  inset: 0;
  background:
    linear-gradient(180deg, rgba(15, 18, 24, 0.38) 0%, rgba(15, 18, 24, 0.72) 100%),
    linear-gradient(135deg, rgba(42, 126, 126, 0.10), rgba(57, 96, 168, 0.08));
  backdrop-filter: blur(10px) saturate(.92);
}
:root.theme-daylight .pd-hero::before {
  background:
    linear-gradient(180deg, rgba(236, 241, 247, 0.36) 0%, rgba(236, 241, 247, 0.82) 100%),
    linear-gradient(135deg, rgba(42, 126, 126, 0.08), rgba(57, 96, 168, 0.06));
}
.pd-hero-inner { position: relative; z-index: 1; display: flex; align-items: flex-start; justify-content: space-between; gap: 20px; flex-wrap: wrap; }
.pd-hero-left { display: flex; align-items: flex-start; gap: 16px; flex: 1; min-width: 0; }
.pd-avatar { width: 56px; height: 56px; border-radius: 18px; background: var(--bg); box-shadow: var(--raise-sm); display: grid; place-items: center; overflow: hidden; flex-shrink: 0; }
.pd-avatar-img { width: 100%; height: 100%; object-fit: cover; }
.pd-avatar-mark { font-size: 22px; font-weight: 800; color: var(--text); }
.pd-hero-info { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: 6px; }
.pd-hero-name-row { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; }
.pd-pool-name { margin: 0; font-size: clamp(22px,3vw,32px); font-weight: 900; letter-spacing: -.05em; color: var(--text); }
.pd-status-chip { display: inline-flex; align-items: center; gap: 5px; font-size: 11px; font-weight: 700; border-radius: 999px; padding: 4px 11px; background: var(--bg); box-shadow: var(--inset-sm); }
.pd-dot { width: 6px; height: 6px; border-radius: 999px; flex-shrink: 0; }
.status-healthy .pd-dot { background: var(--teal); }
.status-healthy { color: var(--teal); }
.status-limited .pd-dot  { background: var(--zc-warning); }
.status-limited { color: var(--zc-warning); }
.status-offline .pd-dot  { background: var(--zc-danger); }
.status-offline { color: var(--zc-danger); }
.pd-pool-meta { margin: 0; font-size: 13px; color: var(--muted); }
.pd-tags { display: flex; gap: 6px; flex-wrap: wrap; }
.pd-tag { font-size: 11px; font-weight: 700; border-radius: 999px; padding: 3px 10px; background: var(--bg); box-shadow: var(--inset-sm); color: var(--muted); }
.pd-tag-joined { color: var(--teal); background: rgba(90,167,164,.1); }
.pd-tag-verified { color: var(--teal); background: rgba(90,167,164,.12); }
.pd-tag-professional { color: var(--gold); background: rgba(176,125,42,.14); }
.pd-tag-watch { color: #b45309; background: rgba(217,119,6,.14); }
.pd-tag-pending { color: var(--muted); }
.pd-tag-note { max-width: 100%; line-height: 1.45; }
.pd-hero-actions { display: flex; gap: 10px; flex-wrap: wrap; flex-shrink: 0; }

/* SECTION */
.pd-section { background: var(--bg); box-shadow: var(--raise-sm); border-radius: 22px; padding: 22px; display: flex; flex-direction: column; gap: 14px; }
.pd-section-title { margin: 0; font-size: 16px; font-weight: 800; letter-spacing: -.03em; color: var(--text); }

/* METRICS */
.pd-metrics-grid { display: grid; grid-template-columns: repeat(4,1fr); gap: 8px; }
.pd-metric-card { background: var(--bg); box-shadow: var(--inset-sm); border-radius: 16px; padding: 14px 12px; text-align: center; display: flex; flex-direction: column; gap: 4px; }
.pd-metric-card small { font-size: 11px; color: var(--muted); font-weight: 600; }
.pd-metric-card strong { font-size: 17px; font-weight: 900; letter-spacing: -.04em; color: var(--text); }
.good { color: var(--teal) !important; }
.warn { color: var(--zc-warning) !important; }
.bad  { color: var(--zc-danger) !important; }

/* HEALTH BAR */
.pd-health { display: flex; flex-direction: column; gap: 7px; }
.pd-health-head { display: flex; align-items: center; justify-content: space-between; font-size: 12px; font-weight: 700; color: var(--muted); }
.tone-good { color: var(--teal) !important; }
.tone-warn { color: var(--zc-warning) !important; }
.tone-bad  { color: var(--zc-danger) !important; }
.tone-pending { color: var(--muted) !important; }
.pd-bar-track { height: 7px; border-radius: 999px; background: var(--bg); box-shadow: var(--inset-sm); overflow: hidden; }
.pd-bar-fill { height: 100%; border-radius: 999px; transition: width .4s ease; }
.fill-good { background: var(--teal); }
.fill-warn { background: var(--zc-warning); }
.fill-bad  { background: var(--zc-danger); }
.fill-pending { background: var(--muted); }
.pd-health-note { font-size: 12px; color: var(--muted); margin: 0; }

/* KEYS */
.pd-key-list { display: flex; flex-direction: column; gap: 10px; }
.pd-key-row { display: flex; align-items: center; gap: 10px; padding: 12px; background: var(--bg); box-shadow: var(--inset-sm); border-radius: 14px; }
.pd-key-code { flex: 1; font-family: "JetBrains Mono",monospace; font-size: 13px; color: var(--teal); letter-spacing: .04em; }
.pd-key-models { font-size: 11px; color: var(--muted); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; max-width: 200px; }

/* MODELS */
.pd-model-list { display: flex; gap: 8px; flex-wrap: wrap; }
.pd-model-chip { font-size: 12px; font-weight: 700; padding: 4px 12px; border-radius: 999px; background: var(--bg); box-shadow: var(--raise-sm); color: var(--text); }
.pd-pricing-note { display: flex; flex-direction: column; gap: 4px; }
.pd-pricing-note p { margin: 0; font-size: 13px; color: var(--muted); }
.pd-pricing-note strong { color: var(--text); }
.pd-price-list { display: grid; gap: 10px; }
.pd-price-row { display: grid; gap: 10px; min-width: 0; padding: 14px; border: 1px solid color-mix(in srgb, var(--muted) 18%, transparent); border-radius: 14px; background: color-mix(in srgb, var(--bg) 82%, transparent); }
.pd-price-row-head { display: flex; align-items: flex-start; justify-content: space-between; gap: 12px; min-width: 0; }
.pd-price-name { display: grid; gap: 3px; min-width: 0; }
.pd-price-name strong { overflow-wrap: anywhere; color: var(--text); font-size: 13px; }
.pd-price-name span { color: var(--muted); font-size: 11px; }
.pd-price-badges { display: flex; justify-content: flex-end; gap: 6px; flex-wrap: wrap; }
.pd-price-badges > span { padding: 4px 8px; border-radius: 999px; font-size: 10px; font-weight: 850; }
.pd-price-source { color: var(--teal); background: color-mix(in srgb, var(--teal) 9%, transparent); }
.pd-endpoint-state.is-ready { color: var(--teal); background: color-mix(in srgb, var(--teal) 11%, transparent); }
.pd-endpoint-state.is-warning { color: var(--zc-warning); background: color-mix(in srgb, var(--zc-warning) 11%, transparent); }
.pd-endpoint-state.is-danger { color: var(--zc-danger); background: color-mix(in srgb, var(--zc-danger) 10%, transparent); }
.pd-endpoint-state.is-muted { color: var(--muted); background: color-mix(in srgb, var(--muted) 10%, transparent); }
.pd-endpoint-reason { margin: 0; color: var(--muted); font-size: 11px; line-height: 1.55; }
.pd-price-facts { display: grid; grid-template-columns: 1.25fr .55fr 1.25fr .8fr; gap: 8px; }
.pd-price-facts > div { display: grid; align-content: start; gap: 4px; min-width: 0; padding: 9px; border-radius: 9px; background: var(--bg); box-shadow: var(--inset-sm); }
.pd-price-facts small { color: var(--muted); font-size: 9px; }
.pd-price-facts strong { overflow-wrap: anywhere; color: var(--text); font-size: 11px; line-height: 1.45; }
.pd-price-example { margin: 0; padding: 8px 10px; border-radius: 8px; color: var(--teal); background: color-mix(in srgb, var(--teal) 7%, transparent); font-size: 10px; line-height: 1.5; }
.pd-price-warning, .pd-price-unavailable { padding: 10px 12px; border-radius: 9px; color: var(--zc-danger); background: color-mix(in srgb, var(--zc-danger) 8%, transparent); font-size: 11px; line-height: 1.55; }
.pd-price-warning { display: flex; align-items: center; justify-content: space-between; gap: 10px; flex-wrap: wrap; }

/* SEAT RULES */
.pd-rules-grid { display: grid; grid-template-columns: repeat(2,1fr); gap: 8px; }
.pd-rule-item { display: flex; align-items: center; justify-content: space-between; padding: 12px 14px; background: var(--bg); box-shadow: var(--inset-sm); border-radius: 14px; font-size: 13px; }
.pd-rule-item span { color: var(--muted); font-weight: 600; }
.pd-rule-item strong { color: var(--text); font-weight: 800; }

/* COMMUNITY */
.pd-community-preview { display: flex; flex-direction: column; gap: 10px; }
.pd-community-stats { display: flex; gap: 12px; font-size: 12px; color: var(--muted); font-weight: 600; }
.pd-risk { color: var(--zc-danger) !important; }
.pd-latest-post { margin: 0; font-size: 13px; color: var(--text); font-weight: 600; padding: 10px 14px; background: var(--bg); box-shadow: var(--inset-sm); border-radius: 12px; }

/* FOOTER ACTIONS */
.pd-footer-actions { display: flex; gap: 10px; flex-wrap: wrap; }
.pd-like-btn, .pd-discuss-btn, .pd-report-btn {
  display: inline-flex; align-items: center; gap: 7px;
  min-height: 42px; padding: 0 20px; border-radius: 999px;
  font-size: 13px; font-weight: 700; cursor: pointer; border: none;
  background: var(--bg); box-shadow: var(--raise-sm); color: var(--muted);
  transition: box-shadow .18s, color .15s;
}
.pd-like-btn em { font-style: normal; font-weight: 900; }
.pd-like-btn:hover, .pd-discuss-btn:hover, .pd-report-btn:hover { box-shadow: 0 8px 18px color-mix(in srgb, var(--nd) 30%, transparent); }
.pd-heart { color: #ef4444; font-size: 16px; line-height: 1; transform: translateY(-.5px); }
.pd-like-btn.liked { color: #dc2626; background: color-mix(in srgb, #ef4444 9%, var(--zc-surface)); box-shadow: var(--inset-sm); }
.pd-like-btn:disabled { opacity: .55; cursor: not-allowed; }
.pd-discuss-btn:hover { color: var(--teal); }
.pd-report-btn:hover { color: var(--zc-danger); }

.pd-section-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.pd-report {
  display: grid;
  gap: 12px;
}

.pd-report-summary {
  display: grid;
  grid-template-columns: repeat(5, minmax(0, 1fr));
  gap: 8px;
}

.pd-report-summary span {
  display: flex;
  flex-direction: column;
  gap: 4px;
  border-radius: 14px;
  background: var(--bg);
  box-shadow: var(--inset-sm);
  padding: 12px;
}

.pd-report-summary small {
  color: var(--muted);
  font-size: 11px;
  font-weight: 700;
}

.pd-report-summary strong {
  color: var(--text);
  font-size: 14px;
  font-weight: 900;
}

.pd-check-list {
  display: grid;
  gap: 8px;
}

.pd-check-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  border-radius: 14px;
  background: var(--bg);
  box-shadow: var(--inset-sm);
  padding: 12px 14px;
}

.pd-check-main {
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 3px;
}

.pd-check-main strong {
  color: var(--text);
  font-size: 13px;
  font-weight: 850;
}

.pd-check-main span,
.pd-check-meta span {
  color: var(--muted);
  font-size: 11px;
  font-weight: 700;
}

.pd-check-meta {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 3px;
}

.pd-check-meta em {
  font-style: normal;
  font-size: 12px;
  font-weight: 900;
}

.pd-check-item.is-pass .pd-check-meta em { color: var(--teal); }
.pd-check-item.is-fail .pd-check-meta em { color: var(--zc-danger); }

@media (max-width: 900px) {
  .pd-report-summary {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

/* BUTTONS */
.pd-btn {
  display: inline-flex; align-items: center; justify-content: center;
  min-height: 40px; padding: 0 18px; border: none; border-radius: 14px;
  font-size: 14px; font-weight: 700; cursor: pointer;
  background: var(--bg); box-shadow: var(--raise-sm); color: var(--text);
  text-decoration: none; transition: box-shadow .18s;
}
.pd-btn:hover:not(:disabled) { box-shadow: 0 8px 18px color-mix(in srgb, var(--nd) 30%, transparent); }
.pd-btn:active:not(:disabled) { box-shadow: var(--inset-sm); }
.pd-btn:disabled { opacity: .5; cursor: not-allowed; }
.pd-btn-primary {
  color: var(--zc-accent-ink, #fff);
  background: linear-gradient(135deg, var(--teal), color-mix(in srgb, var(--gold) 42%, var(--teal)));
  box-shadow: 0 10px 24px color-mix(in srgb, var(--teal) 28%, transparent);
  border-radius: 999px;
}
.pd-btn-outline { background: var(--bg); box-shadow: var(--raise-sm); border-radius: 999px; }
.pd-btn-sm { min-height: 32px; padding: 0 14px; font-size: 12px; border-radius: 12px; }

@media (max-width: 700px) {
  .pd-metrics-grid { grid-template-columns: repeat(2,1fr); }
  .pd-rules-grid { grid-template-columns: 1fr; }
  .pd-price-facts { grid-template-columns: 1fr; }
  .pd-price-row-head { align-items: stretch; flex-direction: column; }
  .pd-price-badges { justify-content: flex-start; }
  .pd-hero-inner { flex-direction: column; }
  .pd-hero-actions { width: 100%; }
}

/* Shared-pool detail refresh: make the second-level page readable at a glance. */
.pd-page {
  max-width: 1180px;
  padding: 6px 0 80px;
}

:root.theme-noir .pd-page {
  --bg: var(--module-canvas, var(--zc-bg));
  --nd: var(--module-shadow-dark, var(--zc-shadow-dark));
  --nl: var(--module-shadow-light, var(--zc-shadow-light));
  --text: var(--module-text-strong, var(--zc-text-strong));
  --muted: var(--module-muted, var(--zc-muted));
  --teal: var(--module-accent, var(--zc-accent));
  --gold: var(--module-accent-2, var(--zc-accent-2));
  --raise: 0 20px 48px color-mix(in srgb, var(--nd) 52%, transparent), inset 0 1px 0 color-mix(in srgb, var(--nl) 26%, transparent);
  --raise-sm: 0 12px 28px color-mix(in srgb, var(--nd) 44%, transparent), inset 0 1px 0 color-mix(in srgb, var(--nl) 22%, transparent);
  --inset: inset 0 3px 14px color-mix(in srgb, #000 38%, transparent), inset 0 -1px 0 color-mix(in srgb, var(--nl) 18%, transparent);
  --inset-sm: inset 0 2px 9px color-mix(in srgb, #000 34%, transparent), inset 0 -1px 0 color-mix(in srgb, var(--nl) 16%, transparent);
}

.pd-hero,
.pd-overview-card,
.pd-section,
.pd-like-btn,
.pd-discuss-btn,
.pd-report-btn {
  border: 0;
  background: var(--bg);
}

.pd-hero {
  background: var(--bg);
}

.pd-hero::before {
  background:
    radial-gradient(circle at 15% 0%, color-mix(in srgb, var(--gold) 11%, transparent), transparent 34%),
    radial-gradient(circle at 86% 8%, color-mix(in srgb, var(--teal) 10%, transparent), transparent 32%),
    color-mix(in srgb, var(--bg) 92%, white);
  backdrop-filter: none;
}

:root.theme-noir .pd-hero::before {
  background: rgba(28,30,34,.78);
}

.pd-overview-card {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 20px;
  border-radius: 26px;
  padding: 22px;
  box-shadow: var(--raise-sm);
}

.pd-overview-card span:first-child {
  color: var(--gold);
  font-size: 11px;
  font-weight: 950;
  letter-spacing: 0.18em;
  text-transform: uppercase;
}

.pd-overview-card h2 {
  margin: 6px 0 0;
  color: var(--text);
  font-size: 22px;
  font-weight: 950;
  letter-spacing: -0.045em;
}

.pd-overview-card p {
  max-width: 46rem;
  margin: 8px 0 0;
  color: var(--muted);
  font-size: 14px;
  font-weight: 700;
  line-height: 1.7;
}

.pd-overview-steps {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.pd-overview-steps span {
  border-radius: 999px;
  border: 0;
  background: var(--bg);
  color: var(--text) !important;
  padding: 7px 12px;
  font-size: 12px !important;
  letter-spacing: 0 !important;
  text-transform: none !important;
  box-shadow: var(--inset-sm);
}

.pd-detail-layout {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(300px, 360px);
  gap: 18px;
  align-items: start;
}

.pd-section-metrics,
.pd-section:not(.pd-key-panel),
.pd-footer-actions {
  grid-column: 1;
}

.pd-key-panel {
  position: sticky;
  top: 5.25rem;
  grid-column: 2;
  grid-row: 1 / span 3;
}

.pd-footer-actions {
  margin-top: -4px;
}

.pd-section,
.pd-metric-card,
.pd-key-row,
.pd-model-chip,
.pd-rule-item,
.pd-latest-post {
  border: 0;
}

.pd-metric-card,
.pd-key-row,
.pd-model-chip,
.pd-rule-item,
.pd-latest-post {
  background: var(--bg);
}

.pd-btn-primary {
  background: linear-gradient(135deg, var(--teal), color-mix(in srgb, var(--gold) 42%, var(--teal)));
  color: var(--zc-accent-ink, #fff);
  box-shadow: 0 10px 24px color-mix(in srgb, var(--teal) 28%, transparent);
}

.pd-btn-primary:hover:not(:disabled) {
  color: var(--zc-accent-ink, #fff);
  box-shadow: 0 14px 30px color-mix(in srgb, var(--teal) 34%, transparent);
}

.pd-btn-outline,
.pd-btn-sm {
  background: var(--bg);
  box-shadow: var(--raise-sm);
}

.pd-join-card p {
  margin: 0;
  color: var(--muted);
  font-size: 13px;
  font-weight: 700;
  line-height: 1.7;
}

@media (max-width: 960px) {
  .pd-detail-layout {
    grid-template-columns: 1fr;
  }

  .pd-section-metrics,
  .pd-section:not(.pd-key-panel),
  .pd-footer-actions,
  .pd-key-panel {
    grid-column: 1;
  }

  .pd-key-panel {
    position: static;
    grid-row: auto;
  }

  .pd-overview-card {
    align-items: flex-start;
    flex-direction: column;
  }
}

@media (max-width: 700px) {
  .pd-page {
    padding: 0 0 60px;
  }

  .pd-hero,
  .pd-overview-card,
  .pd-section {
    border-radius: 22px;
    padding: 18px;
  }

  .pd-hero-left {
    width: 100%;
  }

  .pd-hero-actions,
  .pd-footer-actions {
    display: grid;
    grid-template-columns: 1fr;
    width: 100%;
  }

  .pd-key-row {
    align-items: stretch;
    flex-direction: column;
  }

  .pd-key-models {
    max-width: none;
    white-space: normal;
  }
}

/* 详情首屏与市场卡共用收藏身份，同时把服务判断独立呈现。 */
.pd-hero {
  padding: 0;
  border: 1px solid color-mix(in srgb, var(--gold) 24%, transparent);
  background: var(--bg);
}
.pd-hero::before { display: none; }
.pd-hero-grid {
  display: grid;
  grid-template-columns: minmax(0, 1.32fr) minmax(280px, .68fr);
  min-height: 390px;
}
.pd-hero-copy {
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 18px;
  padding: 28px;
}
.pd-identity-row {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}
.pd-identity-chip,
.pd-service-grade {
  display: inline-flex;
  min-height: 27px;
  align-items: center;
  padding: 0 10px;
  border-radius: 999px;
  font-size: 11px;
  font-weight: 900;
}
.pd-identity-chip {
  background: color-mix(in srgb, var(--teal) 10%, var(--bg));
  color: var(--teal);
}
.pd-service-grade { background: color-mix(in srgb, currentColor 9%, var(--bg)); }
.grade-excellent { color: #087f75; }
.grade-good { color: #2f7dd1; }
.grade-watch { color: #b56a00; }
.grade-risk,
.grade-offline { color: #c93838; }
.grade-pending { color: var(--muted); }
.pd-tag-card {
  color: var(--gold);
  background: color-mix(in srgb, var(--gold) 10%, var(--bg));
}
.pd-decision {
  display: grid;
  gap: 8px;
  padding: 18px;
  border-left: 4px solid currentColor;
  border-radius: 16px;
  background: color-mix(in srgb, currentColor 6%, var(--bg));
  box-shadow: var(--inset-sm);
}
.decision-excellent { color: #087f75; }
.decision-good { color: #2f7dd1; }
.decision-watch { color: #b56a00; }
.decision-risk,
.decision-offline { color: #c93838; }
.decision-pending { color: var(--muted); }
.pd-decision-kicker {
  color: currentColor;
  font-size: 10px;
  font-weight: 950;
}
.pd-decision h2 {
  margin: 0;
  color: var(--text);
  font-size: 18px;
  font-weight: 950;
  line-height: 1.35;
}
.pd-decision p {
  margin: 0;
  color: var(--muted);
  font-size: 12px;
  line-height: 1.65;
}
.pd-decision-facts {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 8px;
}
.pd-decision-facts > span {
  display: grid;
  min-width: 0;
  gap: 2px;
  padding: 9px 10px;
  border-radius: 11px;
  background: color-mix(in srgb, var(--bg) 88%, transparent);
}
.pd-decision-facts small { color: var(--muted); font-size: 9px; font-weight: 800; }
.pd-decision-facts strong { color: var(--text); font-size: 13px; font-weight: 950; overflow-wrap: anywhere; }

.pd-card-showcase {
  position: relative;
  min-width: 0;
  min-height: 390px;
  overflow: hidden;
  background: color-mix(in srgb, var(--bg) 88%, var(--gold));
}
.pd-card-showcase-image {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  object-fit: cover;
  object-position: center;
  filter: saturate(1.07) contrast(1.02);
}
.pd-card-showcase-shade {
  position: absolute;
  inset: 0;
  background:
    linear-gradient(180deg, rgba(10, 15, 24, .08) 0%, rgba(10, 15, 24, .12) 52%, rgba(10, 15, 24, .78) 100%),
    linear-gradient(90deg, color-mix(in srgb, var(--bg) 44%, transparent) 0%, transparent 38%);
}
.pd-card-showcase-top {
  position: absolute;
  inset: 18px 18px auto;
  display: flex;
  justify-content: space-between;
  gap: 8px;
}
.pd-card-showcase-top span {
  display: inline-flex;
  min-height: 26px;
  align-items: center;
  padding: 0 9px;
  border-radius: 999px;
  background: rgba(16, 24, 36, .58);
  color: #fff;
  font-size: 10px;
  font-weight: 900;
  backdrop-filter: blur(7px);
}
.pd-card-showcase-caption {
  position: absolute;
  inset: auto 20px 20px;
  display: grid;
  gap: 3px;
  color: #fff;
  text-shadow: 0 2px 12px rgba(0, 0, 0, .45);
}
.pd-card-showcase-caption small,
.pd-card-showcase-caption span { color: rgba(255, 255, 255, .82); font-size: 11px; font-weight: 800; }
.pd-card-showcase-caption strong { font-size: 21px; font-weight: 950; }
.pd-card-showcase-empty {
  position: relative;
  height: 100%;
  min-height: 390px;
  overflow: hidden;
  background:
    radial-gradient(circle at 72% 26%, color-mix(in srgb, var(--teal) 24%, transparent), transparent 38%),
    linear-gradient(145deg, color-mix(in srgb, var(--bg) 92%, var(--gold)), color-mix(in srgb, var(--bg) 82%, var(--teal)));
}
.pd-card-showcase-default-image {
  position: absolute;
  right: -10%;
  bottom: -7%;
  width: 116%;
  height: 116%;
  object-fit: contain;
  object-position: right bottom;
  filter: saturate(1.04) contrast(1.02) drop-shadow(0 24px 32px rgba(0, 0, 0, .2));
}

.pd-card-archive { gap: 18px; }
.pd-card-archive-kicker {
  color: var(--gold);
  font-size: 10px;
  font-weight: 950;
}
.pd-card-rarity-badge {
  display: inline-flex;
  min-height: 28px;
  align-items: center;
  padding: 0 10px;
  border-radius: 999px;
  background: color-mix(in srgb, var(--gold) 11%, var(--bg));
  color: var(--gold);
  font-size: 11px;
  font-weight: 900;
}
.pd-card-archive-grid {
  display: grid;
  grid-template-columns: minmax(220px, .72fr) minmax(0, 1.28fr);
  gap: 14px;
}
.pd-card-archive-facts {
  display: grid;
  gap: 8px;
  margin: 0;
}
.pd-card-archive-facts > div {
  display: flex;
  justify-content: space-between;
  gap: 14px;
  padding: 10px 12px;
  border-radius: 11px;
  background: color-mix(in srgb, var(--bg) 90%, var(--gold));
}
.pd-card-archive-facts dt { color: var(--muted); font-size: 11px; font-weight: 800; }
.pd-card-archive-facts dd { margin: 0; color: var(--text); font-size: 12px; font-weight: 900; text-align: right; }
.pd-card-archive-story {
  display: grid;
  align-content: start;
  gap: 10px;
  padding: 16px;
  border-radius: 14px;
  background: color-mix(in srgb, var(--gold) 6%, var(--bg));
}
.pd-card-archive-story blockquote {
  margin: 0;
  color: var(--text);
  font-size: 15px;
  font-weight: 850;
  line-height: 1.65;
}
.pd-card-archive-story p { margin: 0; color: var(--muted); font-size: 12px; line-height: 1.75; }

@media (max-width: 820px) {
  .pd-hero-grid { grid-template-columns: 1fr; }
  .pd-card-showcase { min-height: 320px; order: -1; }
  .pd-card-showcase-empty { min-height: 320px; }
}

@media (max-width: 700px) {
  .pd-hero { padding: 0; }
  .pd-hero-copy { padding: 18px; }
  .pd-decision-facts { grid-template-columns: repeat(3, minmax(0, 1fr)); }
  .pd-card-archive-grid { grid-template-columns: 1fr; }
  .pd-card-showcase { min-height: 286px; }
  .pd-card-showcase-empty { min-height: 286px; }
}
</style>
