<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { ArrowRight, Copy, Gift, QrCode, RefreshCw, Wallet } from '@lucide/vue'
import QRCode from 'qrcode'
import AppLayout from '@/components/layout/AppLayout.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import userAPI from '@/api/user'
import type { UserAffiliateDetail } from '@/types'
import { useAuthStore } from '@/stores/auth'
import { useClipboard } from '@/composables/useClipboard'
import { formatCurrency, formatDateTime } from '@/utils/format'
import { extractApiErrorMessage } from '@/utils/apiError'
import { useAffiliateDiscovery } from '@/features/bizdecipher/composables/useAffiliateDiscovery'

const authStore = useAuthStore()
const affiliateDiscovery = useAffiliateDiscovery(computed(() => Number(authStore.user?.id ?? 0)))
const { copyToClipboard } = useClipboard()
const detail = ref<UserAffiliateDetail | null>(null)
const loading = ref(false)
const loadError = ref('')
const transferring = ref(false)
const confirming = ref(false)
const transferMessage = ref('')
const transferSucceeded = ref(false)
const transferNeedsRefresh = ref(false)
const showQR = ref(false)
const qrImage = ref('')
const qrError = ref('')
const growth = computed(() => detail.value?.growth)
const tiers = computed(() => [...(growth.value?.tiers ?? [])].sort((a, b) => a.qualified_invitees - b.qualified_invitees))
const nextTier = computed(() => tiers.value.find(tier => tier.qualified_invitees > (growth.value?.qualified_paid_invitees ?? 0)))
const inviteLink = computed(() => detail.value?.aff_code
  ? `${window.location.origin}/register?aff=${encodeURIComponent(detail.value.aff_code)}` : '')
const canTransfer = computed(() => Boolean(detail.value && detail.value.aff_quota > 0 && !transferring.value && !loading.value && !transferNeedsRefresh.value && !loadError.value))
// Share copy stays honest: it repeats the server-declared rates and states the
// qualification rule instead of promising an unconditional payout.
const shareText = computed(() => {
  const lines: string[] = []
  if (detail.value?.aff_code) lines.push(`邀请码：${detail.value.aff_code}`)
  if (growth.value) {
    lines.push(`当前返利：直接邀请 ${growth.value.direct_percent}% · 间接邀请 ${growth.value.indirect_percent}%（合计上限 ${growth.value.max_combined_percent}%）`)
  }
  lines.push('好友完成符合条件的付费订单后才产生返利，注册本身不发放现金。')
  if (inviteLink.value) lines.push(inviteLink.value)
  return lines.join('\n')
})
const canShare = computed(() => Boolean(detail.value?.aff_code && inviteLink.value))

async function loadAffiliateDetail(): Promise<boolean> {
  if (loading.value) return false
  const accountId = Number(authStore.user?.id ?? 0)
  loading.value = true
  loadError.value = ''
  try {
    const response = await userAPI.getAffiliateDetail()
    if (accountId !== Number(authStore.user?.id ?? 0)) return false
    detail.value = response
    transferNeedsRefresh.value = false
    if (response.aff_code) affiliateDiscovery.markSeen(accountId)
    return true
  } catch (error) {
    loadError.value = extractApiErrorMessage(error, '邀请数据加载失败，请重试。')
    return false
  } finally {
    loading.value = false
  }
}

async function openQR(): Promise<void> {
  if (!inviteLink.value) return
  showQR.value = true
  qrImage.value = ''
  qrError.value = ''
  try {
    qrImage.value = await QRCode.toDataURL(inviteLink.value, { width: 256, margin: 2, errorCorrectionLevel: 'M' })
  } catch {
    qrError.value = '二维码生成失败，可继续复制邀请链接。'
  }
}

async function transferQuota(): Promise<void> {
  if (!confirming.value || !canTransfer.value) return
  transferring.value = true
  transferMessage.value = ''
  transferSucceeded.value = false
  try {
    const response = await userAPI.transferAffiliateQuota()
    transferSucceeded.value = true
    confirming.value = false
    transferNeedsRefresh.value = true
    // Successful writes stay successful even when their following reads fail.
    transferMessage.value = `已成功转入 ${formatCurrency(response.transferred_quota)}。`
    const [refreshed, account] = await Promise.all([
      loadAffiliateDetail(),
      authStore.refreshUser().then(() => true).catch(() => false),
    ])
    if (!refreshed || !account) transferMessage.value += '最新余额尚未完整同步，请刷新核对；无需重复转入。'
  } catch (error) {
    confirming.value = false
    transferNeedsRefresh.value = true
    transferMessage.value = `${extractApiErrorMessage(error, '未收到转入成功确认。')} 请先刷新核对记录与余额，再决定是否重试。`
  } finally {
    transferring.value = false
  }
}

onMounted(loadAffiliateDetail)
</script>

<template>
  <AppLayout>
    <main class="affiliate-page">
      <header class="aff-header">
        <div><span class="aff-eyebrow">激励中心</span><h1>邀请好友</h1></div>
        <div class="aff-actions">
          <RouterLink to="/wallet"><Wallet :size="16" />钱包</RouterLink>
          <button type="button" aria-label="刷新邀请数据" title="刷新邀请数据" :disabled="loading || transferring" @click="loadAffiliateDetail"><RefreshCw :size="16" /></button>
        </div>
      </header>
      <section class="aff-intro" aria-labelledby="aff-intro-title">
        <span class="aff-intro-icon" aria-hidden="true"><Gift :size="22" /></span>
        <div>
          <p class="aff-intro-kicker">为解决大家余额越用越少的问题</p>
          <h2 id="aff-intro-title">邀请好友，让返利补充你的站内余额</h2>
          <p>把零号城分享给有需要的朋友。好友通过你的链接注册并完成符合条件的付费订单后，你可获得返利，再转入站内余额使用。</p>
          <small>注册本身不发放现金；返利比例和到账情况以下方实际记录为准。</small>
        </div>
      </section>
      <p v-if="loadError" class="aff-notice aff-error" role="alert">{{ loadError }}<span v-if="detail"> 当前保留上次读取的数据。</span></p>
      <p v-if="loading && !detail" class="aff-empty" role="status">正在读取邀请数据…</p>
      <template v-if="detail">
        <section class="aff-share" aria-labelledby="aff-share-title">
          <div><h2 id="aff-share-title">我的邀请</h2><p>好友通过邀请链接注册，符合条件的付费订单产生返利。注册本身不发放现金。</p></div>
          <div class="aff-share-fields">
            <div class="aff-field"><span>邀请码</span><code>{{ detail.aff_code || '暂不可用' }}</code><button type="button" title="复制邀请码" aria-label="复制邀请码" :disabled="!detail.aff_code" @click="copyToClipboard(detail.aff_code, '邀请码已复制')"><Copy :size="16" /></button></div>
            <div class="aff-field aff-link"><span>邀请链接</span><code>{{ inviteLink || '暂不可用' }}</code><button type="button" title="复制邀请链接" aria-label="复制邀请链接" :disabled="!inviteLink" @click="copyToClipboard(inviteLink, '邀请链接已复制')"><Copy :size="16" /></button></div>
            <button type="button" :disabled="!canShare" title="复制邀请文案（含邀请码与链接）" @click="copyToClipboard(shareText, '邀请文案已复制')"><Copy :size="16" />复制邀请文案</button>
            <button type="button" :disabled="!inviteLink" @click="openQR"><QrCode :size="16" />分享二维码</button>
          </div>
        </section>
        <section class="aff-metrics" aria-label="邀请收益概览">
          <div><span>已邀请</span><strong>{{ detail.aff_count }}</strong><small>注册人数</small></div>
          <div><span>合格付费直邀</span><strong>{{ growth?.qualified_paid_invitees ?? '—' }}</strong><small>等级计算人数</small></div>
          <div><span>可转入收益</span><strong>{{ formatCurrency(detail.aff_quota) }}</strong><small>站内余额</small></div>
          <div><span>累计返利</span><strong>{{ formatCurrency(detail.aff_history_quota) }}</strong><small>历史收益</small></div>
        </section>
        <section class="aff-growth" aria-labelledby="aff-growth-title">
          <div class="aff-section-head"><h2 id="aff-growth-title">邀请等级</h2><strong v-if="growth">{{ growth.current_tier.name }}</strong></div>
          <template v-if="growth">
            <div class="aff-rates"><span>直接邀请 <b>{{ growth.direct_percent }}%</b></span><span>间接邀请 <b>{{ growth.indirect_percent }}%</b></span><span>合计上限 {{ growth.max_combined_percent }}%</span></div>
            <p>返利按符合条件的订单面额计算，不按净到账金额计算。赠送、兑换及旧版奖励另行记录，不计作注册现金奖励。</p>
            <div v-if="nextTier" class="aff-progress"><label for="aff-progress">距离 {{ nextTier.name }} 还差 {{ nextTier.qualified_invitees - growth.qualified_paid_invitees }} 位合格付费直邀</label><progress id="aff-progress" :value="growth.qualified_paid_invitees" :max="nextTier.qualified_invitees" /></div>
            <p v-else class="aff-current">已达到当前最高等级</p>
            <ol class="aff-tiers"><li v-for="tier in tiers" :key="tier.qualified_invitees" :class="{ current: tier.qualified_invitees === growth.current_tier.qualified_invitees }"><b>{{ tier.name }}</b><span>{{ tier.qualified_invitees }} 位合格付费直邀</span><small>直接 {{ tier.direct_percent }}% · 间接 {{ tier.indirect_percent }}%</small></li></ol>
          </template>
          <p v-else class="aff-notice">等级与返利比例暂不可用，等待服务端返回；不以历史比例推算当前收益。</p>
        </section>
        <section class="aff-transfer" aria-labelledby="aff-transfer-title">
          <div><h2 id="aff-transfer-title">收益转入站内余额</h2><p>转入全部可用收益，用于站内消费。这不是提现，不能转至银行卡或链上地址。</p><p v-if="detail.aff_frozen_quota > 0" class="aff-pending">历史待结算 {{ formatCurrency(detail.aff_frozen_quota) }}，暂不计入可转入金额。</p></div>
          <button type="button" class="aff-primary" data-testid="transfer-review" :disabled="!canTransfer" @click="confirming = true">核对转入 {{ formatCurrency(detail.aff_quota) }}<ArrowRight :size="16" /></button>
          <p v-if="transferMessage" class="aff-notice" :class="{ 'aff-error': !transferSucceeded }" role="status" data-testid="transfer-result">{{ transferMessage }}</p>
          <p v-if="transferNeedsRefresh" class="aff-pending">转入状态待核对，请先刷新邀请数据。</p>
        </section>
        <section aria-labelledby="aff-invitees-title">
          <div class="aff-section-head"><h2 id="aff-invitees-title">邀请记录</h2><span>已返回 {{ detail.invitees.length }} 条</span></div>
          <p v-if="!detail.invitees.length" class="aff-empty">暂无邀请记录</p>
          <div v-else class="aff-table-scroll"><table><thead><tr><th>好友</th><th>累计返利</th><th>注册时间</th></tr></thead><tbody><tr v-for="item in detail.invitees" :key="item.user_id"><td><b>{{ item.username || '未设置昵称' }}</b><small>{{ item.email || '未提供邮箱' }}</small></td><td>{{ formatCurrency(item.total_rebate) }}</td><td>{{ item.created_at ? formatDateTime(item.created_at) : '未记录' }}</td></tr></tbody></table></div>
        </section>
      </template>
      <p v-else-if="!loading" class="aff-empty">暂时无法读取邀请信息，请刷新重试。</p>
      <BaseDialog :show="showQR" title="邀请好友" width="narrow" @close="showQR = false">
        <div class="aff-qr"><img v-if="qrImage" :src="qrImage" width="256" height="256" alt="好友注册邀请二维码" /><p v-else role="status">{{ qrError || '正在生成二维码…' }}</p><b>邀请码 {{ detail?.aff_code }}</b><p>扫码注册 · 注册本身不发放现金</p><button type="button" @click="copyToClipboard(inviteLink, '邀请链接已复制')"><Copy :size="16" />复制邀请链接</button></div>
      </BaseDialog>
      <BaseDialog :show="confirming" title="确认转入站内余额" width="narrow" :show-close-button="!transferring" :close-on-escape="!transferring" @close="!transferring && (confirming = false)">
        <p>本次将全部可用收益 {{ formatCurrency(detail?.aff_quota ?? 0) }} 转入站内余额。实际金额以服务端执行结果为准。</p><p>此操作不是提现。</p>
        <template #footer><div class="aff-actions"><button type="button" :disabled="transferring" @click="confirming = false">取消</button><button type="button" class="aff-primary" data-testid="transfer-confirm" :disabled="transferring || !canTransfer" @click="transferQuota">{{ transferring ? '正在转入…' : '确认转入' }}</button></div></template>
      </BaseDialog>
    </main>
  </AppLayout>
</template>

<style scoped>
.affiliate-page { max-width:1120px; margin:0 auto; display:grid; gap:24px; color:var(--bd-text-primary); font-size:13px; }
.affiliate-page h1 { font-size:24px; margin:4px 0 0; }.affiliate-page h2 { font-size:16px; margin:0; }.affiliate-page p { color:var(--bd-text-secondary); line-height:1.7; margin:8px 0; }
.aff-header,.aff-section-head,.aff-actions { display:flex; align-items:center; justify-content:space-between; gap:12px; }.aff-header { border-bottom:1px solid var(--bd-ui-line); padding-bottom:20px; }.aff-eyebrow { color:var(--bd-accent-teal); font-size:11px; }.aff-actions { justify-content:flex-end; flex-wrap:wrap; }
.aff-intro { display:flex; align-items:flex-start; gap:16px; padding:20px; border:1px solid color-mix(in srgb,var(--bd-accent-teal) 20%,var(--bd-ui-line)); border-radius:12px; background:color-mix(in srgb,var(--bd-accent-teal) 6%,var(--bd-surface)); }
.aff-intro>div { min-width:0; }.aff-intro-icon { display:grid; place-items:center; flex:0 0 42px; width:42px; height:42px; border-radius:12px; background:color-mix(in srgb,var(--bd-accent-teal) 12%,var(--bd-surface)); color:var(--bd-accent-teal); }
.affiliate-page .aff-intro-kicker { margin:0 0 6px; color:var(--bd-accent-teal); font-size:12px; }.aff-intro h2 { font-size:20px; line-height:1.5; }.aff-intro small { color:var(--bd-text-secondary); font-size:11px; line-height:1.6; }
.affiliate-page button,.affiliate-page a,.aff-qr button,.aff-actions button { display:inline-flex; align-items:center; justify-content:center; gap:8px; min-height:36px; padding:7px 12px; border:1px solid var(--bd-ui-line); border-radius:6px; background:var(--bd-surface); color:var(--bd-text-primary); text-decoration:none; font:inherit; cursor:pointer; }.affiliate-page button:disabled,.aff-actions button:disabled { opacity:.5; cursor:not-allowed; }.affiliate-page :is(button,a):focus-visible { outline:2px solid var(--bd-accent-teal); outline-offset:3px; }
.aff-share-fields { display:flex; gap:12px; align-items:stretch; flex-wrap:wrap; margin-top:16px; }.aff-field { display:flex; align-items:center; gap:12px; min-width:0; border-bottom:1px solid var(--bd-ui-line); padding:4px 0; }.aff-field>span { color:var(--bd-text-secondary); flex-shrink:0; }.aff-field code { overflow-wrap:anywhere; min-width:0; }.aff-field button { flex-shrink:0; }.aff-link { flex:1 1 380px; }
.aff-metrics { display:grid; grid-template-columns:repeat(4,minmax(0,1fr)); gap:20px; padding:20px 0; border-block:1px solid var(--bd-ui-line); }.aff-metrics>div { display:grid; gap:7px; min-width:0; }.aff-metrics span,.aff-metrics small { color:var(--bd-text-secondary); }.aff-metrics strong { font-size:22px; overflow-wrap:anywhere; font-variant-numeric:tabular-nums; }
.aff-growth,.aff-transfer { padding-bottom:24px; border-bottom:1px solid var(--bd-ui-line); }.aff-rates { display:flex; align-items:baseline; flex-wrap:wrap; gap:24px; margin-top:16px; }.aff-rates b { font-size:20px; color:var(--bd-accent-teal); }.aff-progress { display:grid; gap:10px; margin:18px 0; }progress { width:100%; height:8px; accent-color:var(--bd-accent-teal); }.aff-tiers { list-style:none; display:grid; grid-template-columns:repeat(3,minmax(0,1fr)); gap:16px; padding:0; margin:20px 0 0; }.aff-tiers li { display:grid; gap:6px; padding:12px 0; border-top:2px solid var(--bd-ui-line); }.aff-tiers .current { border-color:var(--bd-accent-teal); }.aff-tiers :is(span,small) { color:var(--bd-text-secondary); }
.aff-transfer { display:flex; align-items:center; flex-wrap:wrap; gap:16px; }.aff-transfer>div { flex:1 1 420px; }.aff-primary { background:var(--bd-accent-teal)!important; color:white!important; border-color:transparent!important; }.aff-notice { width:100%; padding:12px; background:var(--bd-surface-raised); border-left:3px solid var(--bd-accent-teal); }.affiliate-page .aff-error { color:var(--bd-status-danger); border-color:var(--bd-status-danger); }.affiliate-page .aff-pending { color:var(--bd-text-secondary); width:100%; }.aff-empty { padding:24px 0; }
.aff-table-scroll { overflow:auto; margin-top:14px; }table { width:100%; border-collapse:collapse; text-align:left; }th,td { padding:12px 14px; border-bottom:1px solid var(--bd-ui-line); }th { color:var(--bd-text-secondary); font-weight:500; }td { overflow-wrap:anywhere; }td small { display:block; color:var(--bd-text-secondary); margin-top:4px; }.aff-qr { display:grid; justify-items:center; gap:12px; text-align:center; }.aff-qr img { width:256px; max-width:100%; height:auto; }
@media(max-width:640px) { .aff-metrics { grid-template-columns:repeat(2,minmax(0,1fr)); }.aff-share-fields { display:grid; grid-template-columns:minmax(0,1fr); }.aff-tiers { gap:10px; font-size:11px; }.aff-tiers b { overflow-wrap:anywhere; }.aff-header { align-items:flex-start; }.aff-transfer>button { width:100%; }th,td { padding:10px 8px; } }
@media(max-width:640px) { .aff-intro { padding:16px; gap:12px; }.aff-intro-icon { flex-basis:32px; width:32px; height:32px; border-radius:9px; }.aff-intro h2 { font-size:18px; } }
</style>
