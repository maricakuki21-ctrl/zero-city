<template>
  <AppLayout>
    <div class="zero-governance-page ops-page space-y-5">
      <header class="ops-heading">
        <div class="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
          <div class="flex items-center gap-4">
            <div>
              <h1>共享池运营</h1>
              <p class="ops-meta">{{ governanceSummary.snapshot_at ? `数据更新 ${formatDateTime(governanceSummary.snapshot_at)}` : '等待运营数据' }}</p>
            </div>
          </div>
          <div class="flex flex-wrap gap-2">
            <button class="btn btn-primary" :disabled="loading || aggregationRunning" @click="runProbeAggregation">
              {{ aggregationRunning ? '监测中...' : '执行监测聚合' }}
            </button>
            <button class="btn btn-secondary" :disabled="loading" @click="loadPools">
              <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
              刷新
            </button>
          </div>
        </div>
      </header>

      <section class="ops-totals">
        <SummaryCard label="今日用户消耗" :value="summaryAvailable ? formatCurrency(governanceSummary.today_gross_charges) : '—'" hint="账本单位" />
        <SummaryCard label="今日池主收益" :value="summaryAvailable ? formatCurrency(governanceSummary.today_owner_payout) : '—'" tone="emerald" hint="已入账收益" />
        <SummaryCard label="今日平台收入" :value="summaryAvailable ? formatCurrency(governanceSummary.today_platform_fee) : '—'" tone="amber" :hint="platformFeeHint" />
        <SummaryCard label="今日调用" :value="summaryAvailable ? String(governanceSummary.today_usage_count) : '—'" tone="cyan" :hint="summaryAvailable ? `${governanceSummary.today_active_pools} 个活跃池` : '待载入'" />
      </section>

      <nav class="ops-tabs" aria-label="共享池运营分区">
        <button v-for="tab in workspaceTabs" :key="tab.key" :aria-pressed="activeTab === tab.key" type="button" @click="setWorkspaceTab(tab.key)">{{ tab.label }}</button>
      </nav>
      <SharedPoolOperationsPanel v-if="activeTab === 'operations'" :pools="allPools" :loading="loading" :error="poolsError" :selected-id="selectedPoolId" @refresh="loadPools" @select="selectPool" />

      <section v-if="activeTab === 'earnings'" class="ops-section" aria-labelledby="owner-earnings-ledger-title">
        <WithdrawalAdminPanel />
        <div class="flex flex-col gap-3 xl:flex-row xl:items-start xl:justify-between">
          <div>
            <h2 id="owner-earnings-ledger-title" class="text-lg font-black text-slate-950 dark:text-white">池主永久收益总账</h2>
            <p class="mt-1 text-sm font-semibold text-slate-500 dark:text-slate-400">按实际入账逐笔展示；池子归档或删除后，池名、模型和价格快照仍保留。</p>
          </div>
          <form class="grid gap-2 sm:grid-cols-2 xl:grid-cols-[140px_140px_160px_140px_auto]" @submit.prevent="loadOwnerEarnings()">
            <input v-model="ownerEarningsFilters.ownerId" class="input" inputmode="numeric" placeholder="池主 ID" aria-label="池主 ID" />
            <input v-model="ownerEarningsFilters.poolId" class="input" inputmode="numeric" placeholder="池子 ID" aria-label="池子 ID" />
            <select v-model="ownerEarningsFilters.kind" class="input" aria-label="收益来源">
              <option value="">全部来源</option>
              <option value="earning">全部收益</option>
              <option value="api">API 调用分润</option>
              <option value="seat">席位费分润</option>
              <option value="transfer">转入站内余额</option>
              <option value="adjustment">调整与冲正</option>
            </select>
            <select v-model="ownerEarningsFilters.status" class="input" aria-label="收益状态">
              <option value="">全部状态</option>
              <option value="available">可转余额</option>
              <option value="pending">待入账</option>
              <option value="settled">已转余额</option>
              <option value="reversed">已冲正</option>
            </select>
            <button class="btn btn-secondary" type="submit" :disabled="ownerEarningsLoading">
              <Icon name="search" size="sm" />
              查询
            </button>
          </form>
        </div>

        <p v-if="ownerEarningsError" role="alert" class="ops-load-error">{{ ownerEarningsError }}</p>
        <div v-if="!ownerEarningsError && !ownerEarningsLoading" class="mt-4 grid gap-2 sm:grid-cols-2 xl:grid-cols-5">
          <Metric label="匹配流水" :value="String(ownerEarningsPage.matching_entries || 0)" />
          <Metric label="匹配池主" :value="String(ownerEarningsPage.matching_owners || 0)" />
          <Metric label="用户实付" :value="formatCurrency(ownerEarningsPage.matching_gross_amount)" />
          <Metric label="池主实收" :value="formatCurrency(ownerEarningsPage.matching_net_amount)" tone="emerald" />
          <Metric label="平台服务费" :value="formatCurrency(ownerEarningsPage.matching_platform_fee)" tone="amber" />
        </div>

        <div class="mt-4 overflow-x-auto">
          <table class="min-w-[1040px] w-full text-left text-sm">
            <thead class="border-y border-slate-200 text-xs font-black text-slate-500 dark:border-dark-600 dark:text-slate-400">
              <tr>
                <th class="px-3 py-2">时间</th>
                <th class="px-3 py-2">池主</th>
                <th class="px-3 py-2">池 / 模型</th>
                <th class="px-3 py-2">来源</th>
                <th class="px-3 py-2 text-right">用户实付</th>
                <th class="px-3 py-2 text-right">平台服务费</th>
                <th class="px-3 py-2 text-right">池主实收</th>
                <th class="px-3 py-2">状态</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-slate-100 dark:divide-dark-700">
              <tr v-for="entry in ownerEarningsPage.items" :key="entry.id" class="text-slate-700 dark:text-slate-200">
                <td class="whitespace-nowrap px-3 py-3 text-xs">{{ formatDateTime(entry.posted_at || entry.created_at) }}</td>
                <td class="px-3 py-3">
                  <p class="font-black">{{ entry.owner_username || entry.owner_label_snapshot || `池主 #${entry.owner_id}` }}</p>
                  <p class="mt-0.5 text-xs text-slate-500">{{ entry.owner_email || `ID ${entry.owner_id}` }}</p>
                </td>
                <td class="px-3 py-3">
                  <p class="font-black">{{ entry.pool_name_snapshot || (entry.pool_id ? `池 #${entry.pool_id}` : '历史池') }}</p>
                  <p class="mt-0.5 text-xs text-slate-500">{{ entry.model_snapshot || '席位/钱包流水' }}</p>
                </td>
                <td class="px-3 py-3 font-semibold">{{ ownerEarningKindLabel(entry) }}</td>
                <td class="px-3 py-3 text-right font-mono">{{ formatCurrency(entry.gross_amount) }}</td>
                <td class="px-3 py-3 text-right font-mono text-amber-700 dark:text-amber-300">{{ formatCurrency(entry.platform_fee_amount) }}</td>
                <td class="px-3 py-3 text-right font-mono font-black text-emerald-700 dark:text-emerald-300">{{ formatSignedCurrency(entry.wallet_delta ?? entry.net_amount) }}</td>
                <td class="px-3 py-3">{{ ownerEarningStatusLabel(entry.status) }}</td>
              </tr>
              <tr v-if="!ownerEarningsError && !ownerEarningsLoading && !ownerEarningsPage.items.length">
                <td colspan="8" class="px-3 py-8 text-center font-semibold text-slate-500">当前筛选下没有收益流水</td>
              </tr>
            </tbody>
          </table>
        </div>
        <div class="mt-3 flex items-center justify-between gap-3">
          <p class="text-xs font-semibold text-slate-500">{{ ownerEarningsLoading ? '正在读取收益总账…' : '金额按账本原始精度显示，小额收益不会被伪装成 0。' }}</p>
          <button v-if="ownerEarningsPage.has_more" class="btn btn-secondary" type="button" :disabled="ownerEarningsLoading" @click="loadOwnerEarnings(true)">加载更多</button>
        </div>
      </section>

      <section v-if="activeTab === 'policy'" class="grid gap-3 lg:grid-cols-[1.2fr_0.8fr]">
        <div class="rounded-3xl border border-slate-200 bg-white p-4 shadow-sm dark:border-dark-600 dark:bg-dark-800">
          <div class="flex flex-wrap items-start justify-between gap-3">
            <div>
              <p class="text-sm font-black text-slate-950 dark:text-white">治理状态汇总</p>
              <p class="mt-1 text-xs text-slate-500 dark:text-slate-400">按池状态、投诉和探测聚合结果展示。</p>
            </div>
            <span class="rounded-full bg-slate-100 px-3 py-1 text-xs font-semibold text-slate-600 dark:bg-dark-700 dark:text-slate-300">
              {{ formatDateTime(governanceSummary.snapshot_at) }}
            </span>
          </div>
          <div class="mt-4 grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
            <Metric label="当前视图" :value="String(pools.length)" />
            <Metric label="已上架" :value="String(governanceSummary.listed_pools || 0)" tone="emerald" />
            <Metric label="平均可用率" :value="formatPercent(governanceSummary.average_availability || listView.avg_availability)" tone="cyan" />
            <Metric label="开放投诉" :value="String(governanceSummary.open_complaints || 0)" tone="red" />
            <Metric label="健康/受限" :value="`${governanceSummary.healthy_pools || listView.online}/${governanceSummary.limited_pools || listView.limited}`" tone="emerald" />
            <Metric label="下线/维护" :value="String(governanceSummary.offline_pools || 0)" tone="amber" />
            <Metric label="观察/降权" :value="`${governanceSummary.watch_pools || 0}/${governanceSummary.suppressed_pools || 0}`" tone="amber" />
            <Metric label="封禁" :value="String(governanceSummary.banned_pools || 0)" tone="red" />
          </div>
        </div>
        <div class="rounded-3xl border border-cyan-200 bg-cyan-50/70 p-4 shadow-sm dark:border-cyan-900/50 dark:bg-cyan-950/20">
          <p class="text-sm font-black text-cyan-900 dark:text-cyan-100">自动治理解释</p>
          <p class="mt-2 text-sm font-semibold leading-6 text-cyan-800 dark:text-cyan-200">
            {{ governanceSummary.auto_governance_summary || '探测聚合会按连续失败、可用率和错误类型自动调整池状态；管理员操作会写入治理日志。' }}
          </p>
          <div class="mt-4 grid grid-cols-3 gap-2 text-center text-xs font-semibold text-cyan-900 dark:text-cyan-100">
            <div class="rounded-2xl bg-white/70 p-3 dark:bg-dark-800/70">
              <p class="text-lg font-black">{{ governanceSuccessRate }}</p>
              <p class="mt-1 text-cyan-700 dark:text-cyan-300">调用成功率</p>
            </div>
            <div class="rounded-2xl bg-white/70 p-3 dark:bg-dark-800/70">
              <p class="text-lg font-black">{{ formatCompactNumber(governanceSummary.total_calls) }}</p>
              <p class="mt-1 text-cyan-700 dark:text-cyan-300">累计调用</p>
            </div>
            <div class="rounded-2xl bg-white/70 p-3 dark:bg-dark-800/70">
              <p class="text-lg font-black">{{ formatCompactNumber(governanceSummary.failed_calls) }}</p>
              <p class="mt-1 text-cyan-700 dark:text-cyan-300">失败调用</p>
            </div>
          </div>
        </div>
      </section>

      <section v-if="activeTab === 'history'" class="ops-section" aria-labelledby="shared-pool-usage-review-title">
        <div class="flex flex-col gap-4 xl:flex-row xl:items-start xl:justify-between">
          <div class="max-w-3xl">
            <p class="text-xs font-semibold text-amber-700 dark:text-amber-300">历史资金记录</p>
            <h2 id="shared-pool-usage-review-title" class="mt-1 text-lg font-semibold text-slate-950 dark:text-white">历史冻结与释放</h2>
            <p class="mt-2 text-sm font-semibold leading-6 text-amber-900 dark:text-amber-100">
              <strong>冻结不是扣款。</strong>明确的上游 HTTP 失败会立即释放；真正超时或断流才会短暂待定，低额项到期后自动释放。只有超过自动释放阈值、需要证据判断的少量项目才进入人工处理。
            </p>
            <p class="mt-2 text-xs font-black leading-5 text-amber-800 dark:text-amber-200">
              旧复核记录不能作为人工扣费依据；实际扣费只认 canonical usage。本页只能释放冻结，不会创建扣费。
            </p>
          </div>
          <button class="btn btn-secondary" type="button" :disabled="usageReviewsLoading || resolvingReviewId !== null" @click="loadUsageReviews()">
            <Icon name="refresh" size="sm" :class="usageReviewsLoading ? 'animate-spin' : ''" />
            刷新复核单
          </button>
        </div>

        <div class="mt-4 grid gap-3 xl:grid-cols-[1fr_1.15fr]">
          <div class="border-y border-amber-200 py-3 dark:border-amber-900/60">
            <div class="flex flex-wrap items-center justify-between gap-3">
              <div>
                <p class="text-sm font-black text-slate-900 dark:text-white">低额待定自动释放</p>
                <p class="mt-1 text-xs font-semibold leading-5 text-slate-500 dark:text-slate-400">默认等待 15 分钟，单笔冻结不超过 0.01 时自动退回用户；系统不会自动扣款。</p>
              </div>
              <label class="inline-flex items-center gap-2 text-xs font-black text-slate-700 dark:text-slate-200">
                <input v-model="reviewPolicy.auto_release_enabled" type="checkbox" :disabled="reviewPolicyLoading || reviewPolicySaving" />
                启用
              </label>
            </div>
            <div class="mt-3 grid gap-3 sm:grid-cols-[1fr_1fr_auto] sm:items-end">
              <label class="space-y-1 text-xs font-black text-slate-700 dark:text-slate-200">
                等待时间（分钟）
                <input v-model.number="reviewPolicy.auto_release_minutes" class="input" type="number" min="5" max="10080" step="1" :disabled="reviewPolicyLoading || reviewPolicySaving" />
              </label>
              <label class="space-y-1 text-xs font-black text-slate-700 dark:text-slate-200">
                单笔自动释放上限
                <input v-model.number="reviewPolicy.auto_release_max_hold" class="input" type="number" min="0" max="0.1" step="0.000001" :disabled="reviewPolicyLoading || reviewPolicySaving" />
              </label>
              <button class="btn btn-secondary" type="button" :disabled="reviewPolicyLoading || reviewPolicySaving" @click="saveUsageReviewPolicy">
                {{ reviewPolicySaving ? '保存中…' : '保存策略' }}
              </button>
            </div>
          </div>

          <div class="border-y border-amber-200 py-3 dark:border-amber-900/60">
            <div class="grid grid-cols-3 gap-2">
              <Metric label="待定总数" :value="String(usageReviewSummary.pending_count || 0)" tone="amber" />
              <Metric label="待定冻结总额" :value="formatReviewAmount(usageReviewSummary.pending_hold)" tone="amber" />
              <Metric label="最早待定" :value="formatDateTime(usageReviewSummary.oldest_reserved_at)" />
            </div>
            <div v-if="usageReviewSummary.groups?.length" class="mt-3 space-y-1" aria-label="待定复核聚合">
              <div v-for="group in usageReviewSummary.groups.slice(0, 6)" :key="`${group.pool_id}-${group.model}-${group.trigger_reason}`" class="grid gap-1 text-xs font-semibold text-slate-600 sm:grid-cols-[1fr_1fr_auto] dark:text-slate-300">
                <span class="truncate">{{ group.pool_name || `池 #${group.pool_id}` }} · {{ group.model || '未知模型' }}</span>
                <span class="truncate">{{ usageReviewReasonLabel(group.trigger_reason) }}</span>
                <span class="font-black text-amber-700 dark:text-amber-300">{{ group.pending_count }} 笔 / {{ formatReviewAmount(group.pending_hold) }}</span>
              </div>
            </div>
          </div>
        </div>

        <div class="mt-4 flex flex-wrap gap-2" aria-label="人工复核状态">
          <button
            v-for="option in usageReviewStateOptions"
            :key="option.key"
            class="rounded-full border px-3 py-2 text-xs font-black transition"
            :class="usageReviewState === option.key
              ? 'border-amber-500 bg-amber-500 text-white'
              : 'border-amber-200 bg-white text-amber-800 hover:border-amber-400 dark:border-amber-900/60 dark:bg-dark-800 dark:text-amber-200'"
            type="button"
            :disabled="usageReviewsLoading || resolvingReviewId !== null"
            @click="setUsageReviewState(option.key)"
          >
            {{ option.label }}
          </button>
        </div>

        <div v-if="usageReviewState === 'pending' && pendingReviewsOnPage.length" class="mt-3 grid gap-3 border-y border-amber-200 py-3 lg:grid-cols-[auto_1fr_auto] lg:items-end dark:border-amber-900/60">
          <label class="inline-flex items-center gap-2 pb-2 text-xs font-black text-slate-700 dark:text-slate-200">
            <input type="checkbox" :checked="allPendingReviewsSelected" :disabled="batchReviewRunning || resolvingReviewId !== null" @change="toggleAllPendingReviews" />
            选择本页待定（{{ selectedPendingReviewIds.length }}/{{ pendingReviewsOnPage.length }}）
          </label>
          <label class="space-y-1 text-xs font-black text-slate-700 dark:text-slate-200">
            统一处理依据（每笔写入独立审计）
            <input v-model="batchReviewNote" class="input" maxlength="500" placeholder="例如：已核对同批上游故障，全部未产生成功响应" :disabled="batchReviewRunning || resolvingReviewId !== null" />
          </label>
          <button class="btn btn-primary" type="button" :disabled="batchReviewRunning || resolvingReviewId !== null || !selectedPendingReviewIds.length" @click="batchResolveUsageReviews">
            {{ batchReviewRunning ? '批量处理中…' : `处理 ${selectedPendingReviewIds.length} 笔` }}
          </button>
        </div>

        <div v-if="usageReviewsLoading && !usageReviews.length" class="mt-4 rounded-2xl bg-white/80 py-8 text-center dark:bg-dark-800/80">
          <LoadingSpinner />
          <p class="mt-2 text-xs font-semibold text-slate-500 dark:text-slate-400">正在读取资金复核单…</p>
        </div>

        <div v-else-if="usageReviewsError && !usageReviews.length" class="mt-4 rounded-2xl border border-red-200 bg-red-50 p-4 text-sm text-red-800 dark:border-red-900/50 dark:bg-red-950/20 dark:text-red-200" role="alert">
          <p class="font-black">复核单读取失败</p>
          <p class="mt-1 text-xs font-semibold leading-5">{{ usageReviewsError }}</p>
          <button class="btn btn-secondary btn-sm mt-3" type="button" @click="loadUsageReviews()">重新加载</button>
        </div>

        <div v-else-if="!usageReviews.length" class="mt-4 rounded-2xl border border-dashed border-amber-300 bg-white/70 py-8 text-center text-sm font-semibold text-slate-500 dark:border-amber-900/60 dark:bg-dark-800/60 dark:text-slate-400">
          当前状态下没有人工复核单
        </div>

        <div v-else class="mt-4 space-y-3">
          <article
            v-for="review in usageReviews"
            :key="review.reservation_id"
            class="rounded-2xl border border-slate-200 bg-white p-4 dark:border-dark-600 dark:bg-dark-800"
          >
            <div class="flex flex-wrap items-start justify-between gap-3">
              <div class="flex items-start gap-3">
                <input
                  v-if="review.reservation_status === 'review_required'"
                  v-model="selectedUsageReviewIds"
                  class="mt-1"
                  type="checkbox"
                  :value="review.reservation_id"
                  :disabled="batchReviewRunning || resolvingReviewId !== null"
                  :aria-label="`选择复核单 ${review.reservation_id}`"
                />
                <div>
                <div class="flex flex-wrap items-center gap-2">
                  <p class="font-black text-slate-950 dark:text-white">{{ review.pool_name || `共享池 #${review.pool_id}` }}</p>
                  <span class="rounded-full px-2.5 py-1 text-[11px] font-black" :class="usageReviewStatusClass(review)">
                    {{ usageReviewStatusLabel(review) }}
                  </span>
                </div>
                <p class="mt-1 text-xs text-slate-500 dark:text-slate-400">复核单 #{{ review.reservation_id }} · 请求 {{ review.request_id || '-' }}</p>
                </div>
              </div>
              <div class="text-right">
                <p class="text-xs font-semibold text-slate-500 dark:text-slate-400">当前冻结额（尚未扣款）</p>
                <p class="mt-1 text-xl font-black text-amber-700 dark:text-amber-300">{{ formatReviewAmount(review.hold_amount) }}</p>
              </div>
            </div>

            <div class="mt-4 grid gap-3 sm:grid-cols-2 xl:grid-cols-5">
              <Metric label="池" :value="`${review.pool_name || '未命名'} (#${review.pool_id})`" />
              <Metric label="用户" :value="`${review.user_label || '未命名用户'} (#${review.user_id})`" />
              <Metric label="模型" :value="review.model || '-'" tone="cyan" />
              <Metric label="触发原因" :value="usageReviewReasonLabel(review.trigger_reason)" tone="amber" />
              <Metric label="冻结时间" :value="formatDateTime(review.reserved_at)" />
            </div>

            <div v-if="review.resolution_id" class="mt-3 rounded-2xl bg-slate-50 p-3 text-xs font-semibold leading-5 text-slate-600 dark:bg-dark-700 dark:text-slate-300">
              已登记处理：{{ usageReviewActionLabel(review.resolution_action) }}
              <span v-if="review.resolution_action !== 'release'"> {{ formatReviewAmount(review.resolution_amount) }}</span>
              · 备注：{{ review.resolution_note || '后端未返回备注' }}
              · {{ formatDateTime(review.resolved_at) }}
            </div>

            <form v-if="canResolveUsageReview(review)" class="mt-4 grid gap-3 rounded-2xl border border-amber-200 bg-amber-50/70 p-4 dark:border-amber-900/50 dark:bg-amber-950/20" @submit.prevent="resolveUsageReview(review)">
              <div class="grid gap-3 lg:grid-cols-[220px_1fr]">
                <div class="rounded-xl bg-white/80 px-3 py-2 text-xs font-semibold leading-5 text-slate-600 dark:bg-dark-800/70 dark:text-slate-300">
                  释放 {{ formatReviewAmount(review.hold_amount) }} 冻结资金，不会产生扣费。实际扣费由 canonical usage 自动入账。
                </div>
                <label class="space-y-1 text-xs font-black text-slate-700 dark:text-slate-200">
                  处理备注（必填，会进入审计记录）
                  <textarea
                    v-model="usageReviewForms[review.reservation_id].note"
                    class="input min-h-[76px]"
                    maxlength="500"
                    required
                    placeholder="写明核对了什么证据，以及为什么这样处理"
                    :disabled="resolvingReviewId !== null"
                  />
                </label>
              </div>
              <p v-if="usageReviewForms[review.reservation_id].error" class="rounded-xl bg-red-50 px-3 py-2 text-xs font-semibold leading-5 text-red-700 dark:bg-red-950/30 dark:text-red-200" role="alert">
                {{ usageReviewForms[review.reservation_id].error }} 同样内容再次提交时会沿用原操作编号，避免重复处理。
              </p>
              <div class="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
                <p class="text-xs font-semibold leading-5 text-amber-800 dark:text-amber-200">
                  点击后还会弹出二次确认；处理中会锁住全部复核按钮，防止重复提交。
                </p>
                <button class="btn btn-primary" type="submit" :disabled="resolvingReviewId !== null">
                  {{ resolvingReviewId === review.reservation_id ? '处理中，请勿重复点击…' : '核对后执行' }}
                </button>
              </div>
            </form>
          </article>

          <div v-if="usageReviewsHasMore" class="flex justify-center pt-1">
            <button class="btn btn-secondary" type="button" :disabled="usageReviewsLoading || resolvingReviewId !== null" @click="loadMoreUsageReviews">
              {{ usageReviewsLoading ? '加载中…' : '加载更多复核单' }}
            </button>
          </div>
        </div>
      </section>

      <section v-if="activeTab === 'policy'" class="ops-section">
        <div class="mb-4 space-y-3">
          <div class="flex flex-wrap gap-2" aria-label="共享池生命周期视图">
            <button
              v-for="option in lifecycleViewOptions"
              :key="option.key"
              class="inline-flex items-center gap-2 rounded-full border px-3 py-2 text-xs font-black transition"
              :class="lifecycleView === option.key
                ? 'border-cyan-500 bg-cyan-500 text-white shadow-sm'
                : 'border-slate-200 bg-slate-50 text-slate-600 hover:border-cyan-300 hover:text-cyan-700 dark:border-dark-600 dark:bg-dark-700 dark:text-slate-300'"
              type="button"
              @click="setLifecycleView(option.key)"
            >
              <span>{{ option.label }}</span>
              <span class="rounded-full bg-white/80 px-2 py-0.5 text-[10px] text-slate-700">{{ lifecycleCount(option.key) }}</span>
            </button>
          </div>
          <p class="text-xs font-semibold leading-5 text-slate-500 dark:text-slate-400">
            上方财务与治理汇总仍采用后端现有全局口径；标签页只对当前加载结果做安全分流。离线、维护或未上架只进入“待处理”，不会被猜成历史归档。
          </p>
        </div>
        <div class="grid gap-3 lg:grid-cols-[1fr_160px_160px_160px_auto]">
          <input v-model="filters.keyword" class="input" placeholder="搜索池名 / 池主 / 模型" @input="debounceLoad" />
          <select v-model="filters.status" class="input" @change="loadPools">
            <option value="all">全部状态</option>
            <option value="healthy">正常可用</option>
            <option value="limited">受限</option>
            <option value="offline">已下线</option>
            <option value="maintenance">维护中</option>
          </select>
          <select v-model="filters.sort" class="input" @change="loadPools">
            <option value="recommended">综合推荐</option>
            <option value="availability">可用率</option>
            <option value="latency">延迟</option>
            <option value="rate">价格</option>
            <option value="users">用户数</option>
            <option value="newest">最新</option>
          </select>
          <input v-model.number="filters.min_availability" class="input" type="number" min="0" max="100" placeholder="最低可用率" @change="loadPools" />
          <button class="btn btn-primary" :disabled="loading" @click="loadPools">查询</button>
        </div>
        <div class="mt-4 flex flex-col gap-3 rounded-2xl border border-amber-200 bg-amber-50/70 p-3 dark:border-amber-900/50 dark:bg-amber-950/20 lg:flex-row lg:items-center lg:justify-between">
          <div>
            <p class="text-sm font-black text-amber-900 dark:text-amber-100">批量调整平台附加费</p>
            <p class="mt-1 text-xs font-semibold text-amber-700 dark:text-amber-200">
              只修改当前筛选到的 {{ pools.length }} 个已有池，不影响新池默认。逐池保留治理日志与失败反馈。
            </p>
          </div>
          <div class="flex flex-wrap items-center gap-2">
            <button class="btn btn-secondary" type="button" @click="router.push('/admin/settings')">前往系统设置</button>
            <input v-model.number="bulkPlatformFeePercent" class="input w-32" type="number" min="0" max="100" step="0.1" placeholder="附加费 %" />
            <button class="btn btn-primary" type="button" :disabled="bulkSaving || loading || !pools.length" @click="applyBulkPlatformFee">
              {{ bulkSaving ? `批量保存中 ${bulkProgress.done}/${bulkProgress.total}` : '应用到当前结果' }}
            </button>
          </div>
        </div>
      </section>

      <section v-if="loading && activeTab !== 'operations'" class="ops-section py-12 text-center">
        <LoadingSpinner />
      </section>

      <section v-if="activeTab === 'operations' && selectedPools.length" class="ops-details space-y-4">
        <div class="ops-detail-heading"><h2>池详情</h2><button class="btn btn-secondary" type="button" @click="selectedPoolId = null">收起详情</button></div>
        <article
          v-for="pool in selectedPools"
          :key="pool.id"
          class="rounded-3xl border border-slate-200 bg-white p-5 shadow-sm dark:border-dark-600 dark:bg-dark-800"
        >
          <div class="flex flex-col gap-4 xl:flex-row xl:items-start xl:justify-between">
            <div class="min-w-0 flex-1">
              <div class="flex flex-wrap items-center gap-2">
                <img v-if="pool.avatar_url" :src="pool.avatar_url" class="h-10 w-10 rounded-2xl object-cover" alt="" />
                <div v-else class="flex h-10 w-10 items-center justify-center rounded-2xl bg-slate-100 text-sm font-bold text-slate-600 dark:bg-dark-700 dark:text-slate-200">
                  {{ pool.name.slice(0, 1).toUpperCase() }}
                </div>
                <div class="min-w-0">
                  <h2 class="truncate text-lg font-semibold text-slate-950 dark:text-white">{{ pool.name }}</h2>
                  <p class="text-xs text-slate-500 dark:text-slate-400">#{{ pool.id }} · {{ pool.owner_label }} · {{ pool.models?.slice(0, 4).join(', ') || '未配置模型' }}</p>
                </div>
                <span :class="statusClass(pool.status)" class="rounded-full px-2.5 py-1 text-xs font-semibold">{{ statusLabel(pool.status) }}</span>
                <span :class="governanceClass(pool.governance_status)" class="rounded-full px-2.5 py-1 text-xs font-semibold">{{ governanceLabel(pool.governance_status) }}</span>
                <span class="rounded-full px-2.5 py-1 text-xs font-semibold" :class="pool.listed ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-200' : 'bg-slate-100 text-slate-600 dark:bg-dark-700 dark:text-slate-300'">
                  {{ pool.listed ? '已上架' : '未上架' }}
                </span>
                <span v-if="isExplicitlyArchivedPool(pool)" class="rounded-full bg-violet-100 px-2.5 py-1 text-xs font-semibold text-violet-700 dark:bg-violet-900/30 dark:text-violet-200">
                  已归档
                </span>
                <span class="rounded-full bg-slate-100 px-2.5 py-1 text-xs font-semibold text-slate-600 dark:bg-dark-700 dark:text-slate-300">
                  {{ sharedPoolCompatibilitySummary(pool).label }}
                </span>
              </div>

              <div class="ops-pool-metrics mt-4 grid gap-3 sm:grid-cols-2 xl:grid-cols-7">
                <Metric label="综合分" :value="formatNumber(pool.market_score)" tone="cyan" />
                <Metric label="质量分" :value="formatNumber(pool.quality_score)" />
                <Metric label="可用率" :value="formatPercent(pool.today_availability)" tone="emerald" />
                <Metric label="7日可用" :value="formatPercent(pool.seven_day_availability)" />
                <Metric label="延迟" :value="`${pool.avg_latency_ms || 0}ms`" />
                <Metric label="倍率" :value="`${formatSharedPoolMultiplier(pool.rate_multiplier)}x`" tone="amber" />
                <Metric label="投诉" :value="String(pool.complaint_count || 0)" tone="red" />
              </div>
              <SharedPoolEndpointInspection :pool-id="pool.id" />

              <p v-if="pool.governance_note" class="mt-3 rounded-2xl bg-cyan-50 px-3 py-2 text-xs text-cyan-800 dark:bg-cyan-950/30 dark:text-cyan-200">
                前台说明：{{ pool.governance_note }}
              </p>
              <p v-if="pool.admin_note" class="mt-2 rounded-2xl bg-slate-50 px-3 py-2 text-xs text-slate-600 dark:bg-dark-700 dark:text-slate-300">
                后台备注：{{ pool.admin_note }}
              </p>
              <div v-if="isExplicitlyArchivedPool(pool)" class="mt-2 rounded-2xl border border-violet-200 bg-violet-50 px-3 py-3 text-xs text-violet-700 dark:border-violet-900/50 dark:bg-violet-950/30 dark:text-violet-200">
                <p>归档记录：{{ pool.archive_reason || '后端未提供归档原因' }} · {{ formatDateTime(pool.archived_at || undefined) }}</p>
                <p class="mt-2 font-semibold leading-5">
                  恢复只会把池子转为草稿：不会自动上架，也不会自动启用任何旧 Key。池主必须重新检查配置并通过满血检测后，才能再次申请上架。
                </p>
                <button
                  class="btn btn-secondary btn-sm mt-3"
                  type="button"
                  :disabled="restoringId === pool.id"
                  @click="restoreArchivedPool(pool)"
                >
                  {{ restoringId === pool.id ? '恢复中...' : '恢复为草稿' }}
                </button>
              </div>

              <div class="mt-3 rounded-2xl border p-3 text-xs" :class="communitySignalTone(pool)">
                <div class="flex flex-wrap items-start justify-between gap-2">
                  <div class="min-w-0">
                    <p class="font-black">社区反馈信号 · {{ communitySignals(pool).length }}</p>
                    <p class="mt-1 line-clamp-1 font-semibold">{{ communitySignalTitle(pool) }}</p>
                  </div>
                  <button class="btn btn-secondary btn-sm" type="button" @click="openCommunitySignals(pool)">查看主体页</button>
                </div>
                <p class="mt-2 line-clamp-2 opacity-80">{{ communitySignalHint(pool) }}</p>
              </div>

              <div class="mt-3 rounded-2xl border border-slate-200 bg-slate-50 p-3 text-xs dark:border-dark-600 dark:bg-dark-700">
                <div class="flex flex-wrap items-center justify-between gap-2">
                  <div>
                    <p class="font-semibold text-slate-700 dark:text-slate-100">最近探测</p>
                    <p class="mt-1 text-slate-500 dark:text-slate-400">{{ probeSummary(pool) }}</p>
                  </div>
                  <span class="rounded-full px-2.5 py-1 font-semibold" :class="probeBadgeClass(pool)">{{ probeBadge(pool) }}</span>
                </div>
                <p v-if="pool.last_probe_error_message" class="mt-2 line-clamp-2 text-red-600 dark:text-red-300">
                  {{ probeErrorLabel(pool.last_probe_error_type) }}：{{ pool.last_probe_error_message }}
                </p>
                <div class="mt-3 flex flex-wrap gap-2">
                  <button class="btn btn-secondary btn-sm" type="button" :disabled="detailLoadingId === pool.id" @click="toggleProbeHistories(pool)">
                    {{ expandedProbePoolId === pool.id ? '收起探测历史' : '查看探测历史' }}
                  </button>
                  <button class="btn btn-secondary btn-sm" type="button" :disabled="detailLoadingId === pool.id" @click="toggleGovernanceLogs(pool)">
                    {{ expandedLogPoolId === pool.id ? '收起治理日志' : '查看治理日志' }}
                  </button>
                </div>
              </div>

              <div v-if="expandedProbePoolId === pool.id" class="mt-3 rounded-2xl border border-slate-200 bg-white p-3 dark:border-dark-600 dark:bg-dark-800">
                <div class="mb-2 flex items-center justify-between gap-2">
                  <p class="text-xs font-semibold text-slate-700 dark:text-slate-200">探测历史</p>
                  <button class="text-xs font-medium text-cyan-600 dark:text-cyan-300" type="button" @click="loadProbeHistories(pool, true)">刷新</button>
                </div>
                <div v-if="!(probeHistories[pool.id]?.length)" class="rounded-xl bg-slate-50 p-3 text-xs text-slate-500 dark:bg-dark-700 dark:text-slate-400">暂无探测记录</div>
                <div v-else class="space-y-2">
                  <div v-for="item in probeHistories[pool.id]" :key="item.id" class="rounded-xl bg-slate-50 p-3 text-xs dark:bg-dark-700">
                    <div class="flex flex-wrap items-center justify-between gap-2">
                      <span class="font-semibold" :class="item.success ? 'text-emerald-600 dark:text-emerald-300' : 'text-red-600 dark:text-red-300'">{{ item.success ? '成功' : '失败' }} · {{ probeTypeLabel(item.probe_type) }}</span>
                      <span class="text-slate-500 dark:text-slate-400">{{ formatDateTime(item.checked_at) }}</span>
                    </div>
                    <p class="mt-1 text-slate-600 dark:text-slate-300">模型：{{ item.model_name || '-' }} <span v-if="item.upstream_model_name">→ {{ item.upstream_model_name }}</span> · HTTP {{ item.http_status || '-' }} · {{ item.latency_ms || 0 }}ms</p>
                    <p v-if="item.error_message" class="mt-1 text-red-600 dark:text-red-300">{{ probeErrorLabel(item.error_type) }}：{{ item.error_message }}</p>
                  </div>
                </div>
              </div>

              <div v-if="expandedLogPoolId === pool.id" class="mt-3 rounded-2xl border border-slate-200 bg-white p-3 dark:border-dark-600 dark:bg-dark-800">
                <div class="mb-2 flex items-center justify-between gap-2">
                  <p class="text-xs font-semibold text-slate-700 dark:text-slate-200">治理日志</p>
                  <button class="text-xs font-medium text-cyan-600 dark:text-cyan-300" type="button" @click="loadGovernanceLogs(pool, true)">刷新</button>
                </div>
                <div v-if="!(governanceLogs[pool.id]?.length)" class="rounded-xl bg-slate-50 p-3 text-xs text-slate-500 dark:bg-dark-700 dark:text-slate-400">暂无治理日志</div>
                <div v-else class="space-y-2">
                  <div v-for="item in governanceLogs[pool.id]" :key="item.id" class="rounded-xl bg-slate-50 p-3 text-xs dark:bg-dark-700">
                    <div class="flex flex-wrap items-center justify-between gap-2">
                      <span class="font-semibold text-slate-700 dark:text-slate-200">{{ governanceActionLabel(item.action) }}</span>
                      <span class="text-slate-500 dark:text-slate-400">{{ formatDateTime(item.created_at) }}</span>
                    </div>
                    <p class="mt-1 text-slate-500 dark:text-slate-400">管理员：{{ item.admin_user_id || '-' }} · 原因：{{ item.reason || '未填写' }}</p>
                  </div>
                </div>
              </div>
            </div>

            <form class="grid min-w-full gap-3 rounded-2xl bg-slate-50 p-4 dark:bg-dark-700 xl:min-w-[520px]" @submit.prevent="savePool(pool)">
              <div class="grid gap-3 sm:grid-cols-3">
                <label class="space-y-1 text-xs font-medium text-slate-600 dark:text-slate-300">
                  平台附加费 %（用户承担）
                  <input v-model.number="forms[pool.id].platform_fee_percent" class="input" type="number" min="0" max="100" step="0.1" />
                </label>
                <label class="space-y-1 text-xs font-medium text-slate-600 dark:text-slate-300">
                  精选分
                  <input v-model.number="forms[pool.id].featured_score" class="input" type="number" min="-1000" max="1000" step="1" />
                </label>
                <label class="space-y-1 text-xs font-medium text-slate-600 dark:text-slate-300">
                  奖励分
                  <input v-model.number="forms[pool.id].reward_score" class="input" type="number" min="0" max="1000" step="1" />
                </label>
                <label class="space-y-1 text-xs font-medium text-slate-600 dark:text-slate-300">
                  惩罚分
                  <input v-model.number="forms[pool.id].penalty_score" class="input" type="number" min="0" max="1000" step="1" />
                </label>
                <label class="space-y-1 text-xs font-medium text-slate-600 dark:text-slate-300">
                  治理状态
                  <select v-model="forms[pool.id].governance_status" class="input">
                    <option value="normal">正常</option>
                    <option value="boosted">加权推荐</option>
                    <option value="watch">观察</option>
                    <option value="suppressed">降权</option>
                    <option value="banned">封禁</option>
                  </select>
                </label>
                <label class="space-y-1 text-xs font-medium text-slate-600 dark:text-slate-300">
                  服务状态
                  <select v-model="forms[pool.id].status" class="input">
                    <option value="healthy">正常可用</option>
                    <option value="limited">受限</option>
                    <option value="offline">已下线</option>
                    <option value="maintenance">维护中</option>
                  </select>
                </label>
              </div>
              <label class="inline-flex items-center gap-2 text-xs font-semibold text-slate-700 dark:text-slate-200">
                <input v-model="forms[pool.id].listed" type="checkbox" />
                允许上架展示
              </label>
              <label class="space-y-1 text-xs font-medium text-slate-600 dark:text-slate-300">
                前台治理说明
                <textarea v-model="forms[pool.id].governance_note" class="input min-h-[68px]" placeholder="例如：该池正在观察期，低峰可用。" />
              </label>
              <label class="space-y-1 text-xs font-medium text-slate-600 dark:text-slate-300">
                后台备注
                <textarea v-model="forms[pool.id].admin_note" class="input min-h-[68px]" placeholder="仅管理员可见的处理备注。" />
              </label>
              <div class="flex items-center justify-between gap-3">
                <p class="text-xs text-slate-500 dark:text-slate-400">新调用的平台费按池主原价另加，由用户承担，不扣池主定价；历史账单不变。封禁会自动下架。</p>
                <button class="btn btn-primary" type="submit" :disabled="savingId === pool.id">
                  {{ savingId === pool.id ? '保存中...' : '保存治理' }}
                </button>
              </div>
            </form>
          </div>
        </article>

        <div v-if="!pools.length" class="rounded-3xl border border-dashed border-slate-300 bg-white py-12 text-center text-slate-500 dark:border-dark-600 dark:bg-dark-800 dark:text-slate-400">
          {{ emptyViewMessage }}
        </div>
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, defineComponent, h, onMounted, onBeforeUnmount, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores/app'
import SharedPoolOperationsPanel from './SharedPoolOperationsPanel.vue'
import WithdrawalAdminPanel from '../../components/wallet/WithdrawalAdminPanel.vue'
import SharedPoolEndpointInspection from './SharedPoolEndpointInspection.vue'
import { formatSharedPoolMultiplier } from '@/features/bizdecipher/components/shared-pool/sharedPoolPricing'
import {
  adminListSharedPoolOwnerEarnings,
  adminListCommunityPosts,
  adminListSharedPoolGovernanceLogs,
  adminListSharedPoolProbeHistories,
  adminListSharedPools,
  adminListSharedPoolUsageReviews,
  adminBatchResolveSharedPoolUsageReviews,
  adminGetSharedPoolUsageReviewPolicy,
  adminResolveSharedPoolUsageReview,
  adminRestoreSharedPool,
  adminRunSharedPoolProbeAggregation,
  adminUpdateSharedPoolUsageReviewPolicy,
  adminUpdateSharedPoolGovernance,
  type SharedPool,
  type AdminSharedPoolOwnerEarningsEntry,
  type AdminSharedPoolOwnerEarningsPage,
  type SharedPoolGovernanceLog,
  type SharedPoolGovernancePayload,
  type SharedPoolGovernanceSummary,
  type SharedPoolListView,
  type SharedPoolProbeHistory,
  type SharedPoolUsageReview,
  type SharedPoolUsageReviewAction,
  type SharedPoolUsageReviewPolicy,
  type SharedPoolUsageReviewQueueSummary,
  type SharedPoolUsageReviewState,
} from '@/features/bizdecipher/api/bizdecipher'
import type { CommunityPost } from '@/features/bizdecipher/api/community'
import {
  filterSharedPoolsByLifecycle,
  hasSharedPoolArchiveContract,
  isExplicitlyArchivedPool,
  sharedPoolCompatibilitySummary,
  type SharedPoolLifecycleView,
} from './sharedPoolGovernanceView'

const appStore = useAppStore()
const router = useRouter()
type WorkspaceTab = 'operations' | 'earnings' | 'history' | 'policy'
const activeTab = ref<WorkspaceTab>('operations')
const workspaceTabs: { key: WorkspaceTab; label: string }[] = [
  { key: 'operations', label: '资源运营' },
  { key: 'earnings', label: '收益总账' },
  { key: 'history', label: '历史资金记录' },
  { key: 'policy', label: '运营策略' },
]
const selectedPoolId = ref<number | null>(null)
const selectedPools = computed(() => allPools.value.filter(pool => pool.id === selectedPoolId.value))
const poolsError = ref('')
let poolLoadSequence = 0

function selectPool(pool: SharedPool) {
  selectedPoolId.value = pool.id
}
function setWorkspaceTab(tab: WorkspaceTab) {
  activeTab.value = tab
  if (tab === 'earnings') void loadOwnerEarnings()
  if (tab === 'history') {
    void loadUsageReviews()
    void loadUsageReviewPolicy()
  }
}

const loading = ref(false)
const aggregationRunning = ref(false)
const savingId = ref<number | null>(null)
const restoringId = ref<number | null>(null)
const bulkSaving = ref(false)
const bulkPlatformFeePercent = ref<number | undefined>(undefined)
const bulkProgress = reactive({ done: 0, total: 0 })
const detailLoadingId = ref<number | null>(null)
const expandedProbePoolId = ref<number | null>(null)
const expandedLogPoolId = ref<number | null>(null)
const allPools = ref<SharedPool[]>([])
const lifecycleView = ref<SharedPoolLifecycleView>('current')
const lifecycleViewOptions: ReadonlyArray<{ key: SharedPoolLifecycleView; label: string }> = [
  { key: 'current', label: '当前池' },
  { key: 'attention', label: '待处理' },
  { key: 'archived', label: '历史归档' },
  { key: 'all', label: '全部' },
]
const pools = computed(() => filterSharedPoolsByLifecycle(allPools.value, lifecycleView.value))
const archiveContractAvailable = computed(() => hasSharedPoolArchiveContract(allPools.value))
const emptyViewMessage = computed(() => {
  if (lifecycleView.value === 'archived' && !archiveContractAvailable.value) {
    return '后端归档契约尚未接入；系统不会把离线池猜成历史池。'
  }
  if (lifecycleView.value === 'archived') return '当前加载结果中没有明确归档的共享池'
  if (lifecycleView.value === 'attention') return '当前加载结果中没有待处理共享池'
  if (lifecycleView.value === 'current') return '当前加载结果中没有正常运营的共享池'
  return '暂无共享池数据'
})
const emptyGovernanceSummary: SharedPoolGovernanceSummary = {
  snapshot_at: '',
  today_gross_charges: 0,
  today_owner_payout: 0,
  today_platform_fee: 0,
  today_usage_count: 0,
  today_active_pools: 0,
  open_complaints: 0,
  listed_pools: 0,
  healthy_pools: 0,
  limited_pools: 0,
  offline_pools: 0,
  watch_pools: 0,
  suppressed_pools: 0,
  banned_pools: 0,
  total_calls: 0,
  successful_calls: 0,
  failed_calls: 0,
  average_availability: 0,
  platform_fee_basis: 'share_pool_usage - owner_payout',
  auto_governance_summary: '',
}
const listView = reactive<SharedPoolListView>({ pools: [], total: 0, online: 0, limited: 0, avg_availability: 0, governance_summary: { ...emptyGovernanceSummary } })
const governanceSummary = computed(() => listView.governance_summary || emptyGovernanceSummary)
const summaryAvailable = computed(() => !loading.value && !poolsError.value && Boolean(listView.governance_summary?.snapshot_at))
const platformFeeHint = computed(() => '今日用户实际扣费扣除池主收益后的平台收入')
const emptyOwnerEarningsPage: AdminSharedPoolOwnerEarningsPage = {
  items: [],
  has_more: false,
  matching_entries: 0,
  matching_owners: 0,
  matching_gross_amount: 0,
  matching_platform_fee: 0,
  matching_net_amount: 0,
}
const ownerEarningsPage = reactive<AdminSharedPoolOwnerEarningsPage>({ ...emptyOwnerEarningsPage })
const ownerEarningsLoading = ref(false)
const ownerEarningsError = ref('')
let ownerEarningsSequence = 0
const ownerEarningsFilters = reactive({ ownerId: '', poolId: '', kind: '' as '' | 'earning' | 'api' | 'seat' | 'transfer' | 'adjustment', status: '' as '' | 'pending' | 'available' | 'settled' | 'reversed' })
const governanceSuccessRate = computed(() => {
  const total = Number(governanceSummary.value.total_calls || 0)
  if (total <= 0) return '0.0%'
  return formatPercent((Number(governanceSummary.value.successful_calls || 0) / total) * 100)
})
const filters = reactive({ keyword: '', status: 'all', sort: 'recommended', min_availability: undefined as number | undefined, limit: 100 })
const forms = reactive<Record<number, SharedPoolGovernancePayload>>({})
const probeHistories = reactive<Record<number, SharedPoolProbeHistory[]>>({})
const governanceLogs = reactive<Record<number, SharedPoolGovernanceLog[]>>({})
const communityFeedbackSignals = reactive<Record<number, CommunityPost[]>>({})

type UsageReviewForm = {
  note: string
  operationId: string
  operationSignature: string
  error: string
}

const usageReviewState = ref<SharedPoolUsageReviewState>('pending')
const usageReviewStateOptions: ReadonlyArray<{ key: SharedPoolUsageReviewState; label: string }> = [
  { key: 'pending', label: '待复核' },
  { key: 'processing', label: '结算重试中' },
  { key: 'resolved', label: '已处理' },
  { key: 'all', label: '全部' },
]
const usageReviews = ref<SharedPoolUsageReview[]>([])
const usageReviewsLoading = ref(false)
const usageReviewsError = ref('')
const usageReviewsHasMore = ref(false)
const usageReviewsNextBeforeId = ref(0)
const emptyUsageReviewSummary: SharedPoolUsageReviewQueueSummary = {
  pending_count: 0,
  pending_hold: 0,
  groups: [],
}
const usageReviewSummary = ref<SharedPoolUsageReviewQueueSummary>({ ...emptyUsageReviewSummary })
const usageReviewForms = reactive<Record<number, UsageReviewForm>>({})
const resolvingReviewId = ref<number | null>(null)
const selectedUsageReviewIds = ref<number[]>([])
const batchReviewNote = ref('')
const batchReviewRunning = ref(false)
const reviewPolicy = reactive<SharedPoolUsageReviewPolicy>({
  auto_release_enabled: true,
  auto_release_minutes: 15,
  auto_release_max_hold: 0.01,
})
const reviewPolicyLoading = ref(false)
const reviewPolicySaving = ref(false)
const pendingReviewsOnPage = computed(() => usageReviews.value.filter(review => review.reservation_status === 'review_required'))
const selectedPendingReviewIds = computed(() => {
  const pending = new Set(pendingReviewsOnPage.value.map(review => review.reservation_id))
  return selectedUsageReviewIds.value.filter(id => pending.has(id))
})
const allPendingReviewsSelected = computed(() => pendingReviewsOnPage.value.length > 0 && selectedPendingReviewIds.value.length === pendingReviewsOnPage.value.length)
let usageReviewLoadSequence = 0
let debounceTimer: ReturnType<typeof setTimeout> | null = null

function debounceLoad() {
  if (debounceTimer) clearTimeout(debounceTimer)
  debounceTimer = setTimeout(() => loadPools(), 300)
}

function positiveFilterID(value: string): number | undefined {
  const parsed = Number(value.trim())
  return Number.isSafeInteger(parsed) && parsed > 0 ? parsed : undefined
}

async function loadOwnerEarnings(append = false): Promise<void> {
  const sequence = ++ownerEarningsSequence
  ownerEarningsLoading.value = true
  ownerEarningsError.value = ''
  try {
    const page = await adminListSharedPoolOwnerEarnings({
      owner_id: positiveFilterID(ownerEarningsFilters.ownerId),
      pool_id: positiveFilterID(ownerEarningsFilters.poolId),
      before_id: append ? ownerEarningsPage.next_before_id : undefined,
      kind: ownerEarningsFilters.kind,
      status: ownerEarningsFilters.status,
      limit: 50,
    })
    if (sequence !== ownerEarningsSequence) return
    ownerEarningsPage.items = append ? [...ownerEarningsPage.items, ...(page.items || [])] : (page.items || [])
    ownerEarningsPage.next_before_id = page.next_before_id
    ownerEarningsPage.has_more = Boolean(page.has_more)
    ownerEarningsPage.matching_entries = Number(page.matching_entries || 0)
    ownerEarningsPage.matching_owners = Number(page.matching_owners || 0)
    ownerEarningsPage.matching_gross_amount = Number(page.matching_gross_amount || 0)
    ownerEarningsPage.matching_platform_fee = Number(page.matching_platform_fee || 0)
    ownerEarningsPage.matching_net_amount = Number(page.matching_net_amount || 0)
  } catch (err: unknown) {
    if (sequence !== ownerEarningsSequence) return
    ownerEarningsError.value = err instanceof Error ? err.message : '加载池主收益总账失败'
    appStore.showError(ownerEarningsError.value)
  } finally {
    if (sequence === ownerEarningsSequence) ownerEarningsLoading.value = false
  }
}

function ownerEarningKindLabel(entry: AdminSharedPoolOwnerEarningsEntry): string {
  const source = String(entry.metadata?.source_type || '')
  if (entry.event_type === 'transfer_to_balance') return '转入站内余额'
  if (entry.event_type === 'reversal') return '收益冲正'
  if (entry.event_type === 'adjustment') return '人工调整'
  if (source === 'pool_owner_payout') return '席位费分润'
  if (source === 'share_pool_payout') return 'API 调用分润'
  return entry.event_type || '收益'
}

function ownerEarningStatusLabel(status: string): string {
  if (status === 'available') return '可转余额'
  if (status === 'pending') return '待入账'
  if (status === 'settled') return '已转余额'
  if (status === 'reversed') return '已冲正'
  return status || '-'
}

function lifecycleCount(view: SharedPoolLifecycleView): number {
  return filterSharedPoolsByLifecycle(allPools.value, view).length
}

function setLifecycleView(view: SharedPoolLifecycleView) {
  if (lifecycleView.value === view) return
  lifecycleView.value = view
}

function ensureForm(pool: SharedPool) {
  forms[pool.id] = {
    platform_fee_percent: Number(pool.platform_fee_percent ?? Math.max(0, 100 - Number(pool.owner_share_percent || 90))),
    featured_score: Number(pool.featured_score || 0),
    reward_score: Number(pool.reward_score || 0),
    penalty_score: Number(pool.penalty_score || 0),
    governance_status: (pool.governance_status || 'normal') as SharedPoolGovernancePayload['governance_status'],
    governance_note: pool.governance_note || '',
    admin_note: pool.admin_note || '',
    listed: Boolean(pool.listed),
    status: (pool.status || 'healthy') as SharedPoolGovernancePayload['status'],
  }
}

async function loadPools() {
  const sequence = ++poolLoadSequence
  loading.value = true
  poolsError.value = ''
  try {
    const data = await adminListSharedPools({
      keyword: filters.keyword || undefined,
      status: filters.status,
      lifecycle: 'all',
      sort: filters.sort,
      min_availability: filters.min_availability || undefined,
      limit: filters.limit,
    })
    if (sequence !== poolLoadSequence) return
    listView.pools = data.pools || []
    listView.total = data.total || 0
    listView.online = data.online || 0
    listView.limited = data.limited || 0
    listView.avg_availability = data.avg_availability || 0
    listView.governance_summary = data.governance_summary || { ...emptyGovernanceSummary }
    allPools.value = listView.pools
    for (const pool of allPools.value) ensureForm(pool)
    await loadCommunityFeedbackSignals(pools.value.map(pool => pool.id))
  } catch (err: unknown) {
    if (sequence !== poolLoadSequence) return
    poolsError.value = err instanceof Error ? err.message : '加载共享池失败'
    appStore.showError(poolsError.value)
  } finally {
    if (sequence === poolLoadSequence) loading.value = false
  }
}

function usageReviewSignature(review: SharedPoolUsageReview, form: UsageReviewForm): string {
  return JSON.stringify({
    reservation_id: review.reservation_id,
    action: 'release',
    note: form.note.trim(),
  })
}

function ensureUsageReviewForm(review: SharedPoolUsageReview): UsageReviewForm {
  const existing = usageReviewForms[review.reservation_id]
  if (existing) return existing

  const form: UsageReviewForm = {
    note: review.resolution_note || '',
    operationId: review.resolution_operation_id || '',
    operationSignature: '',
    error: '',
  }
  if (form.operationId) form.operationSignature = usageReviewSignature(review, form)
  usageReviewForms[review.reservation_id] = form
  return form
}

async function loadUsageReviews(options: { append?: boolean } = {}): Promise<void> {
  if (usageReviewsLoading.value && options.append) return
  const append = Boolean(options.append)
  const state = usageReviewState.value
  const sequence = ++usageReviewLoadSequence
  usageReviewsLoading.value = true
  usageReviewsError.value = ''
  try {
    const page = await adminListSharedPoolUsageReviews({
      state,
      before_id: append ? usageReviewsNextBeforeId.value : undefined,
      limit: 20,
    })
    if (sequence !== usageReviewLoadSequence || state !== usageReviewState.value) return

    const incoming = page.items || []
    for (const review of incoming) ensureUsageReviewForm(review)
    if (append) {
      const merged = new Map(usageReviews.value.map(review => [review.reservation_id, review]))
      for (const review of incoming) merged.set(review.reservation_id, review)
      usageReviews.value = Array.from(merged.values())
    } else {
      usageReviews.value = incoming
      selectedUsageReviewIds.value = []
    }
    usageReviewSummary.value = page.summary || { ...emptyUsageReviewSummary }
    usageReviewsHasMore.value = Boolean(page.has_more)
    usageReviewsNextBeforeId.value = Number(page.next_before_id || 0)
  } catch (err: unknown) {
    if (sequence !== usageReviewLoadSequence) return
    usageReviewsError.value = err instanceof Error ? err.message : '加载人工复核单失败，请重试'
  } finally {
    if (sequence === usageReviewLoadSequence) usageReviewsLoading.value = false
  }
}

async function loadUsageReviewPolicy(): Promise<void> {
  reviewPolicyLoading.value = true
  try {
    Object.assign(reviewPolicy, await adminGetSharedPoolUsageReviewPolicy())
  } catch (err: unknown) {
    appStore.showError(err instanceof Error ? err.message : '加载自动释放策略失败')
  } finally {
    reviewPolicyLoading.value = false
  }
}

async function saveUsageReviewPolicy(): Promise<void> {
  const minutes = Number(reviewPolicy.auto_release_minutes)
  const maxHold = Number(reviewPolicy.auto_release_max_hold)
  if (!Number.isInteger(minutes) || minutes < 5 || minutes > 10080) {
    appStore.showError('自动释放等待时间必须是 5 到 10080 分钟的整数')
    return
  }
  if (!Number.isFinite(maxHold) || maxHold < 0 || maxHold > 0.1) {
    appStore.showError('自动释放金额阈值必须在 0 到 0.10 之间')
    return
  }
  reviewPolicySaving.value = true
  try {
    Object.assign(reviewPolicy, await adminUpdateSharedPoolUsageReviewPolicy({
      auto_release_enabled: Boolean(reviewPolicy.auto_release_enabled),
      auto_release_minutes: minutes,
      auto_release_max_hold: maxHold,
    }))
    appStore.showSuccess('自动释放策略已保存')
  } catch (err: unknown) {
    appStore.showError(err instanceof Error ? err.message : '保存自动释放策略失败')
  } finally {
    reviewPolicySaving.value = false
  }
}

function toggleAllPendingReviews(): void {
  if (allPendingReviewsSelected.value) {
    const pending = new Set(pendingReviewsOnPage.value.map(review => review.reservation_id))
    selectedUsageReviewIds.value = selectedUsageReviewIds.value.filter(id => !pending.has(id))
    return
  }
  selectedUsageReviewIds.value = Array.from(new Set([
    ...selectedUsageReviewIds.value,
    ...pendingReviewsOnPage.value.map(review => review.reservation_id),
  ]))
}

async function batchResolveUsageReviews(): Promise<void> {
  const ids = selectedPendingReviewIds.value
  const note = batchReviewNote.value.trim()
  if (!ids.length) {
    appStore.showError('请先选择要处理的待复核单')
    return
  }
  if (!note) {
    appStore.showError('请填写本批次统一处理依据')
    return
  }
  const actionText = `释放 ${ids.length} 笔冻结，不扣用户余额`
  if (!window.confirm(`${actionText}。\n\n处理依据：${note}\n\n每笔都会写入独立审计记录，是否继续？`)) return

  batchReviewRunning.value = true
  try {
    const operationId = `shared-pool-review-batch-${Date.now()}-${Math.random().toString(16).slice(2)}`
    const result = await adminBatchResolveSharedPoolUsageReviews({
      reservation_ids: ids,
      action: 'release',
      note,
      operation_id: operationId,
    })
    if (result.failed > 0) {
      appStore.showError(`批量处理完成 ${result.succeeded} 笔，${result.failed} 笔未处理；失败项仍保留在队列中。`)
    } else {
      appStore.showSuccess(`已处理 ${result.succeeded} 笔复核单`)
    }
    selectedUsageReviewIds.value = []
    await loadUsageReviews()
  } catch (err: unknown) {
    appStore.showError(err instanceof Error ? err.message : '批量处理失败，未确认的复核单仍保持冻结')
  } finally {
    batchReviewRunning.value = false
  }
}

function setUsageReviewState(state: SharedPoolUsageReviewState): void {
  if (state === usageReviewState.value) return
  usageReviewState.value = state
  usageReviews.value = []
  usageReviewsHasMore.value = false
  usageReviewsNextBeforeId.value = 0
  usageReviewsError.value = ''
  void loadUsageReviews()
}

function loadMoreUsageReviews(): void {
  if (!usageReviewsHasMore.value || usageReviewsNextBeforeId.value <= 0) return
  void loadUsageReviews({ append: true })
}

function canResolveUsageReview(review: SharedPoolUsageReview): boolean {
	return review.reservation_status === 'review_required'
}

function validateUsageReview(form: UsageReviewForm): string {
  const note = form.note.trim()
  if (!note) return '请先填写处理备注，说明核对依据。'
  if (Array.from(note).length > 500) return '处理备注不能超过 500 个字。'
  return ''
}

function createUsageReviewOperationId(reviewId: number): string {
  const randomPart = typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function'
    ? crypto.randomUUID()
    : `${Date.now()}-${Math.random().toString(16).slice(2)}`
  return `shared-pool-usage-review-${reviewId}-${randomPart}`
}

function usageReviewConfirmMessage(review: SharedPoolUsageReview, form: UsageReviewForm): string {
  const holdAmount = formatReviewAmount(review.hold_amount)
  const actionMessage = `释放全部冻结额 ${holdAmount}，不会产生扣款，用户可用余额会恢复。`
  return `请再次确认人工复核结果：\n\n${actionMessage}\n\n处理备注：${form.note.trim()}\n\n确认后会写入永久审计记录。是否继续？`
}

async function resolveUsageReview(review: SharedPoolUsageReview): Promise<void> {
  if (resolvingReviewId.value !== null || !canResolveUsageReview(review)) return
  const form = ensureUsageReviewForm(review)
  const validationError = validateUsageReview(form)
  if (validationError) {
    form.error = validationError
    return
  }
  if (!window.confirm(usageReviewConfirmMessage(review, form))) return

  const signature = usageReviewSignature(review, form)
  if (!form.operationId || form.operationSignature !== signature) {
    form.operationId = createUsageReviewOperationId(review.reservation_id)
    form.operationSignature = signature
  }
  form.error = ''
  resolvingReviewId.value = review.reservation_id
  try {
    await adminResolveSharedPoolUsageReview(review.reservation_id, {
      action: 'release',
      note: form.note.trim(),
      operation_id: form.operationId,
    })
    appStore.showSuccess(`${usageReviewActionLabel('release')}已提交并写入审计记录`)
    await loadUsageReviews()
  } catch (err: unknown) {
    form.error = err instanceof Error ? err.message : '处理失败，请使用同一操作编号重试'
    appStore.showError('人工复核处理未完成；本页已保留同一操作编号，可安全重试。')
  } finally {
    resolvingReviewId.value = null
  }
}

function usageReviewActionLabel(action?: SharedPoolUsageReviewAction): string {
  if (action === 'capture_hold') return '按全部冻结额扣款'
  if (action === 'settle_amount') return '按明确金额扣款'
  return '释放冻结（不扣款）'
}

function usageReviewStatusLabel(review: SharedPoolUsageReview): string {
  if (review.reservation_status === 'review_required') return '等待人工复核'
  if (review.reservation_status === 'settlement_pending') return '结算已登记，等待重试'
  if (review.reservation_status === 'released') return '已释放，未扣款'
  if (review.reservation_status === 'settled') return '已完成扣款'
  return review.reservation_status || '未知状态'
}

function usageReviewStatusClass(review: SharedPoolUsageReview): string {
  if (review.reservation_status === 'released') return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-200'
  if (review.reservation_status === 'settled') return 'bg-cyan-100 text-cyan-700 dark:bg-cyan-900/30 dark:text-cyan-200'
  if (review.reservation_status === 'settlement_pending') return 'bg-orange-100 text-orange-700 dark:bg-orange-900/30 dark:text-orange-200'
  return 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-200'
}

function usageReviewReasonLabel(reason?: string): string {
  if (reason?.startsWith('upstream_http_')) return '上游明确返回失败（已自动释放）'
  if (reason?.startsWith('auto_released_low_value_after_grace:')) return '低额待定到期自动释放'
  if (reason?.startsWith('auto_release_blocked_balance_mismatch:')) return '冻结账不一致，需人工核账'
  switch (reason) {
    case 'upstream_timeout':
    case 'upstream_timeout_result_unknown': return '上游响应超时，结果待定'
    case 'upstream_error': return '上游调用异常'
    case 'client_disconnect':
    case 'upstream_canceled_result_unknown': return '连接取消，结果待定'
    case 'upstream_connection_lost_result_unknown': return '上游连接中断，结果待定'
    case 'upstream_partial_response_result_unknown': return '已返回部分内容，用量待定'
    case 'upstream_result_unknown': return '上游结果待定'
    case 'forward_result_missing_after_timeout': return '转发超时且缺少结果'
    case 'settlement_failed': return '自动结算失败'
    case 'usage_unknown': return '实际用量无法确认'
    default: return reason || '后端未提供原因'
  }
}

async function savePool(pool: SharedPool) {
  savingId.value = pool.id
  try {
    const updated = await adminUpdateSharedPoolGovernance(pool.id, forms[pool.id])
    const index = allPools.value.findIndex(item => item.id === pool.id)
    if (index >= 0) allPools.value[index] = updated
    ensureForm(updated)
    if (expandedLogPoolId.value === pool.id) await loadGovernanceLogs(updated, true)
    appStore.showSuccess('共享池治理配置已保存')
  } catch (err: unknown) {
    appStore.showError(err instanceof Error ? err.message : '保存共享池治理配置失败')
  } finally {
    savingId.value = null
  }
}

async function restoreArchivedPool(pool: SharedPool) {
  if (!isExplicitlyArchivedPool(pool) || restoringId.value !== null) return
  const confirmed = window.confirm(
    `确认将“${pool.name}”恢复为草稿？\n\n恢复后不会自动上架，也不会自动启用任何旧 Key；池主需要重新配置并通过满血检测后才能再次上架。`,
  )
  if (!confirmed) return

  restoringId.value = pool.id
  try {
    const operationId = `shared-pool-restore-${pool.id}-${crypto.randomUUID()}`
    await adminRestoreSharedPool(pool.id, {
      reason: `治理后台将“${pool.name}”恢复为草稿，需重新配置并通过满血检测后上架`,
      operation_id: operationId,
    })
    appStore.showSuccess('已恢复为草稿；未自动上架，旧 Key 仍不可用。请重新配置并完成满血检测。')
    await loadPools()
  } catch (err: unknown) {
    appStore.showError(err instanceof Error ? err.message : '恢复共享池草稿失败')
  } finally {
    restoringId.value = null
  }
}

async function applyBulkPlatformFee() {
  const nextFee = Number(bulkPlatformFeePercent.value)
  if (!Number.isFinite(nextFee) || nextFee < 0 || nextFee > 100) {
    appStore.showError('平台附加费必须在 0 到 100 之间')
    return
  }
  if (!pools.value.length || bulkSaving.value) return
  const targets = [...pools.value]
  if (!window.confirm(`将当前策略筛选的 ${targets.length} 个池的平台附加费改为 ${nextFee}%？由用户在池主原价之外支付。\n\n池编号：${targets.map(pool => pool.id).join(', ')}\n不影响新池默认规则。`)) return
  bulkSaving.value = true
  bulkProgress.done = 0
  bulkProgress.total = targets.length
  try {
    for (const pool of targets) {
      const payload: SharedPoolGovernancePayload = { platform_fee_percent: nextFee }
      const updated = await adminUpdateSharedPoolGovernance(pool.id, payload)
      const index = allPools.value.findIndex(item => item.id === pool.id)
      if (index >= 0) allPools.value[index] = updated
      ensureForm(updated)
      bulkProgress.done += 1
    }
    appStore.showSuccess(`已批量更新 ${bulkProgress.done} 个共享池的平台附加费`)
    await loadPools()
  } catch (err: unknown) {
    appStore.showError(err instanceof Error ? err.message : '批量调整平台附加费失败')
  } finally {
    bulkSaving.value = false
  }
}

async function runProbeAggregation() {
  aggregationRunning.value = true
  try {
    const summary = await adminRunSharedPoolProbeAggregation(50)
    if (summary.skipped_cooldown || (summary.pools_checked || 0) === 0) {
      appStore.showInfo(summary.message || '当前没有可探测目标：候选均在 5 分钟冷却内，或尚无上架可调度账号')
    } else {
      appStore.showSuccess(
        summary.message ||
          `监测完成：检查 ${summary.pools_checked} 个（账号 ${summary.accounts_checked || 0}，池级 ${summary.pool_level_checked || 0}），成功 ${summary.succeeded}，失败 ${summary.failed}`,
      )
    }
    await loadPools()
  } catch (err: unknown) {
    appStore.showError(err instanceof Error ? err.message : '执行共享池监测失败')
  } finally {
    aggregationRunning.value = false
  }
}

async function toggleProbeHistories(pool: SharedPool) {
  if (expandedProbePoolId.value === pool.id) {
    expandedProbePoolId.value = null
    return
  }
  expandedProbePoolId.value = pool.id
  await loadProbeHistories(pool)
}

async function loadProbeHistories(pool: SharedPool, force = false) {
  if (!force && probeHistories[pool.id]?.length) return
  detailLoadingId.value = pool.id
  try {
    probeHistories[pool.id] = await adminListSharedPoolProbeHistories(pool.id, { limit: 20 })
  } catch (err: unknown) {
    appStore.showError(err instanceof Error ? err.message : '加载探测历史失败')
  } finally {
    detailLoadingId.value = null
  }
}

async function toggleGovernanceLogs(pool: SharedPool) {
  if (expandedLogPoolId.value === pool.id) {
    expandedLogPoolId.value = null
    return
  }
  expandedLogPoolId.value = pool.id
  await loadGovernanceLogs(pool)
}

async function loadGovernanceLogs(pool: SharedPool, force = false) {
  if (!force && governanceLogs[pool.id]?.length) return
  detailLoadingId.value = pool.id
  try {
    governanceLogs[pool.id] = await adminListSharedPoolGovernanceLogs(pool.id, 20)
  } catch (err: unknown) {
    appStore.showError(err instanceof Error ? err.message : '加载治理日志失败')
  } finally {
    detailLoadingId.value = null
  }
}

async function loadCommunityFeedbackSignals(poolIds: number[]): Promise<void> {
  const ids = Array.from(new Set(poolIds.filter((id) => Number.isFinite(id) && id > 0)))
  for (const poolId of ids) communityFeedbackSignals[poolId] = []
  if (!ids.length) return
  try {
    const posts = await adminListCommunityPosts({
      subject_type: 'shared_pool',
      scenario: 'feedback_triage',
      action_type: 'report',
      limit: 100,
    })
    const visiblePoolIds = new Set(ids)
    for (const post of posts) {
      const poolId = communityPostPoolId(post)
      if (!poolId || !visiblePoolIds.has(poolId)) continue
      if (!communityFeedbackSignals[poolId]) communityFeedbackSignals[poolId] = []
      communityFeedbackSignals[poolId].push(post)
    }
    for (const poolId of ids) {
      communityFeedbackSignals[poolId] = [...(communityFeedbackSignals[poolId] || [])]
        .sort((left, right) => new Date(right.created_at || '').getTime() - new Date(left.created_at || '').getTime())
        .slice(0, 6)
    }
  } catch (err: unknown) {
    for (const poolId of ids) communityFeedbackSignals[poolId] = []
    appStore.showError(err instanceof Error ? err.message : '加载社区反馈信号失败')
  }
}

function communityPostPoolId(post: CommunityPost): number | null {
  const rawId = post.subject_type === 'shared_pool' && post.subject_id ? post.subject_id : post.source_id
  const parsed = Number(rawId)
  return Number.isFinite(parsed) && parsed > 0 ? parsed : null
}

function communitySignals(pool: SharedPool): CommunityPost[] {
  return communityFeedbackSignals[pool.id] || []
}

function communitySignalTitle(pool: SharedPool): string {
  const latest = communitySignals(pool)[0]
  return latest?.title || '暂无用户反馈线索'
}

function communitySignalHint(pool: SharedPool): string {
  const signals = communitySignals(pool)
  if (!signals.length) return '未收到 feedback_triage/report 信号；仍以官方探测、账本和治理日志为准。'
  const latest = signals[0]
  return `${formatDateTime(latest.created_at)} · ${latest.author || '用户'} · ${latest.body || latest.title || '用户反馈'}`
}

function communitySignalTone(pool: SharedPool): string {
  const count = communitySignals(pool).length
  if (count >= 3) return 'border-red-200 bg-red-50 text-red-800 dark:border-red-900/50 dark:bg-red-950/20 dark:text-red-100'
  if (count > 0) return 'border-amber-200 bg-amber-50 text-amber-900 dark:border-amber-900/50 dark:bg-amber-950/20 dark:text-amber-100'
  return 'border-slate-200 bg-slate-50 text-slate-600 dark:border-dark-600 dark:bg-dark-700 dark:text-slate-300'
}

function openCommunitySignals(pool: SharedPool) {
  router.push({
    path: '/community',
    query: {
      source_type: 'shared_pool',
      source_id: String(pool.id),
      subject_type: 'shared_pool',
      subject_id: String(pool.id),
    },
  })
}

function formatNumber(value?: number) {
  return Number(value || 0).toFixed(0)
}

function formatPercent(value?: number) {
  return `${Number(value || 0).toFixed(1)}%`
}

function formatCurrency(value?: number) {
  if (value == null || !Number.isFinite(Number(value))) return '—'
  const amount = Number(value)
  const absolute = Math.abs(amount)
  const maximumFractionDigits = absolute > 0 && absolute < 0.000001 ? 12 : absolute > 0 && absolute < 0.01 ? 8 : 2
  return amount.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits })
}

function formatSignedCurrency(value?: number) {
  if (value == null || !Number.isFinite(Number(value))) return '—'
  const amount = Number(value)
  const formatted = formatCurrency(Math.abs(amount))
  return amount < 0 ? `-${formatted}` : formatted
}

function formatReviewAmount(value?: number | null) {
  if (value == null || !Number.isFinite(Number(value))) return '—'
  const amount = Number(value)
  return amount.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 6 })
}

function formatCompactNumber(value?: number) {
  const num = Number(value || 0)
  if (Math.abs(num) >= 1000000) return `${(num / 1000000).toFixed(1)}M`
  if (Math.abs(num) >= 1000) return `${(num / 1000).toFixed(1)}K`
  return String(Math.round(num))
}

function governanceLabel(status?: string) {
  switch (status) {
    case 'boosted': return '加权推荐'
    case 'watch': return '观察'
    case 'suppressed': return '降权'
    case 'banned': return '封禁'
    default: return '正常'
  }
}

function governanceClass(status?: string) {
  switch (status) {
    case 'boosted': return 'bg-cyan-100 text-cyan-700 dark:bg-cyan-900/30 dark:text-cyan-200'
    case 'watch': return 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-200'
    case 'suppressed': return 'bg-orange-100 text-orange-700 dark:bg-orange-900/30 dark:text-orange-200'
    case 'banned': return 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-200'
    default: return 'bg-slate-100 text-slate-600 dark:bg-dark-700 dark:text-slate-300'
  }
}

function statusLabel(status?: string) {
  switch (status) {
    case 'healthy': return '正常可用'
    case 'limited': return '受限'
    case 'offline': return '已下线'
    case 'maintenance': return '维护中'
    default: return status || '未知状态'
  }
}

function statusClass(status?: string) {
  switch (status) {
    case 'healthy': return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-200'
    case 'limited': return 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-200'
    case 'maintenance': return 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-200'
    default: return 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-200'
  }
}

function probeBadge(pool: SharedPool) {
  if (!pool.last_probe_at) return '未探测'
  if (pool.last_probe_success) return '探测成功'
  return `失败 ${pool.consecutive_probe_failures || 1} 次`
}

function probeBadgeClass(pool: SharedPool) {
  if (!pool.last_probe_at) return 'bg-slate-100 text-slate-600 dark:bg-dark-700 dark:text-slate-300'
  if (pool.last_probe_success) return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-200'
  return 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-200'
}

function probeSummary(pool: SharedPool) {
  if (!pool.last_probe_at) return '该共享池还没有探测历史，建议先执行监测聚合。'
  const time = formatDateTime(pool.last_probe_at)
  const success = pool.last_probe_success ? '成功' : '失败'
  const latency = pool.avg_latency_ms ? ` · ${pool.avg_latency_ms}ms` : ''
  const failures = pool.consecutive_probe_failures ? ` · 连续失败 ${pool.consecutive_probe_failures} 次` : ''
  return `${time} · ${success}${latency}${failures}`
}

function probeTypeLabel(type?: string) {
  switch (type) {
    case 'publish_gate': return '上架校验'
    case 'scheduled_full': return '定时满血检测'
    case 'scheduled': return '定时监测'
    case 'manual': return '手动探测'
    default: return type || '-'
  }
}

function probeErrorLabel(type?: string) {
  switch (type) {
    case 'timeout': return '超时'
    case 'auth_error': return '鉴权失败'
    case 'rate_limited': return '限流'
    case 'model_error': return '模型不可用'
    case 'upstream_5xx': return '上游服务异常'
    case 'upstream_4xx': return '上游请求异常'
    case 'invalid_response': return '响应异常'
    case 'network_error': return '网络异常'
    case 'upstream_error': return '上游错误'
    default: return type || '未知错误'
  }
}

function governanceActionLabel(action?: string) {
  switch (action) {
    case 'update_governance': return '更新治理配置'
    default: return action || '治理操作'
  }
}

function formatDateTime(value?: string) {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString()
}

const SummaryCard = defineComponent({
  name: 'SummaryCard',
  props: {
    label: { type: String, required: true },
    value: { type: String, required: true },
    hint: { type: String, default: '' },
    tone: { type: String, default: 'slate' },
  },
  setup(props) {
    return () => h('div', { class: 'ops-summary' }, [
      h('p', { class: 'text-xs font-semibold text-slate-500 dark:text-slate-400' }, props.label),
      h('p', { class: `mt-1 text-2xl font-black ${toneClass(props.tone)}` }, props.value),
      props.hint ? h('p', { class: 'mt-2 line-clamp-1 text-xs text-slate-400 dark:text-slate-500' }, props.hint) : null,
    ])
  },
})

const Metric = defineComponent({
  name: 'Metric',
  props: {
    label: { type: String, required: true },
    value: { type: String, required: true },
    tone: { type: String, default: 'slate' },
  },
  setup(props) {
    return () => h('div', { class: 'rounded-2xl bg-slate-50 p-3 dark:bg-dark-700' }, [
      h('p', { class: 'text-xs text-slate-500 dark:text-slate-400' }, props.label),
      h('p', { class: `mt-1 text-lg font-bold ${toneClass(props.tone)}` }, props.value),
    ])
  },
})

function toneClass(tone: string) {
  switch (tone) {
    case 'cyan': return 'text-cyan-600 dark:text-cyan-300'
    case 'emerald': return 'text-emerald-600 dark:text-emerald-300'
    case 'amber': return 'text-amber-600 dark:text-amber-300'
    case 'red': return 'text-red-600 dark:text-red-300'
    default: return 'text-slate-950 dark:text-white'
  }
}

onMounted(() => {
  void loadPools()
})
onBeforeUnmount(() => {
  if (debounceTimer) clearTimeout(debounceTimer)
  poolLoadSequence += 1
  ownerEarningsSequence += 1
  usageReviewLoadSequence += 1
})
</script>

<style scoped>
.ops-page{max-width:1600px;width:100%;min-width:0;color:var(--bd-text-primary);letter-spacing:0}
.ops-heading{padding:0 0 18px;border-bottom:1px solid var(--bd-ui-line)}.ops-heading h1{font-size:24px;font-weight:600;line-height:1.4}.ops-meta{font-size:12px;color:var(--bd-text-secondary);margin-top:4px}
.ops-tabs{display:flex;gap:24px;overflow:auto;border-bottom:1px solid var(--bd-ui-line)}.ops-tabs button{padding:12px 0;font-size:13px;border-bottom:2px solid transparent;white-space:nowrap;color:var(--bd-text-secondary)}
.ops-tabs button[aria-pressed=true]{color:var(--bd-accent-teal);border-color:var(--bd-accent-teal)}.ops-totals{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:16px;padding-bottom:16px}
.ops-totals :deep(article){border:0;box-shadow:none;padding:0;background:transparent}.ops-section{min-width:0;padding:8px 0}.ops-detail-heading{display:flex;align-items:center;justify-content:space-between;border-top:1px solid var(--bd-ui-line);padding-top:20px}.ops-detail-heading h2{font-size:18px;font-weight:600}
.ops-load-error{color:var(--bd-status-danger);padding:12px 0;font-size:13px}.ops-details>article{padding:0;border:0;box-shadow:none;background:transparent}
.ops-page :deep(.rounded-3xl),.ops-page :deep(.rounded-2xl),.ops-page :deep(.rounded-xl){border-radius:8px}
.ops-page :deep(.font-black){font-weight:600}.ops-page :deep([class*="tracking-"]){letter-spacing:0}
.ops-page :deep(.btn){min-height:36px;border-radius:6px;box-shadow:none;font-size:12px}
.ops-page :deep(form){min-width:0;max-width:100%}.ops-page :deep(.input){width:100%;min-width:0}
.ops-page :deep(button:focus-visible){outline:2px solid var(--bd-accent-teal);outline-offset:2px}
@media(max-width:767px){.ops-totals{grid-template-columns:repeat(2,minmax(0,1fr))}.ops-tabs{gap:20px}.ops-details :deep(.grid){min-width:0}.ops-page :deep(p){overflow-wrap:anywhere}}
.zero-governance-page {
  color: var(--module-ink, var(--zc-text));
}

.zero-governance-kicker {
  color: var(--module-accent, var(--zc-accent));
}

.zero-governance-title {
  color: var(--module-ink-strong, var(--zc-text-strong));
}

.zero-governance-copy {
  color: var(--module-muted, var(--zc-muted));
}

.zero-governance-page :deep(.btn-primary) {
  border-color: var(--module-accent, var(--zc-accent));
  background: var(--module-accent, var(--zc-accent));
  color: var(--zc-accent-ink, #ffffff);
  box-shadow: 0 10px 24px color-mix(in srgb, var(--module-accent, var(--zc-accent)) 22%, transparent);
}

.zero-governance-page :deep(.btn-secondary) {
  border-color: var(--module-line-strong, var(--zc-line-strong));
  background: var(--module-card, var(--zc-card-strong));
  color: var(--module-ink-strong, var(--zc-text-strong));
}

.zero-governance-page :deep(.input) {
  border-color: var(--module-line, var(--zc-line));
  background: var(--module-panel, var(--zc-surface));
  color: var(--module-ink-strong, var(--zc-text-strong));
}

.zero-governance-page :deep(.input:focus) {
  border-color: color-mix(in srgb, var(--module-accent, var(--zc-accent)) 58%, transparent);
  box-shadow: 0 0 0 4px color-mix(in srgb, var(--module-accent, var(--zc-accent)) 12%, transparent);
}

.zero-governance-hero {
  border: 1px solid var(--module-line, var(--zc-line));
  background:
    radial-gradient(circle at 92% 12%, color-mix(in srgb, var(--module-accent, var(--zc-accent)) 14%, transparent), transparent 28%),
    radial-gradient(circle at 20% 88%, color-mix(in srgb, var(--module-accent-2, var(--zc-accent-2)) 10%, transparent), transparent 30%),
    var(--module-panel-raised, var(--zc-surface-raised));
  box-shadow: var(--module-shadow-lift, var(--zc-shadow-lift));
}

.zero-governance-mascot-shell {
  display: grid;
  height: 4.9rem;
  width: 4.9rem;
  flex: 0 0 4.9rem;
  place-items: center;
  overflow: hidden;
  border: 1px solid var(--module-line, var(--zc-line));
  border-radius: 1.5rem;
  background:
    radial-gradient(circle at 50% 78%, color-mix(in srgb, var(--module-accent, var(--zc-accent)) 18%, transparent), transparent 48%),
    var(--module-card, var(--zc-card-strong));
  box-shadow: var(--module-shadow, var(--zc-shadow-soft));
}

.zero-governance-mascot {
  width: 4.5rem;
  height: 4.5rem;
  object-fit: contain;
  filter: drop-shadow(0 12px 14px color-mix(in srgb, var(--module-ink-strong, var(--zc-text-strong)) 20%, transparent));
}
.ops-page :deep(.ops-summary){padding:4px 0;border:0;box-shadow:none;background:transparent}
.ops-page :deep(.btn-primary){background:var(--bd-accent-teal);border-color:var(--bd-accent-teal);color:var(--bd-on-action);box-shadow:none}
.ops-page .ops-details :deep(.input){border-radius:6px;background:var(--bd-surface);color:var(--bd-text-primary);box-shadow:none}
.ops-page .ops-pool-metrics{grid-template-columns:repeat(4,minmax(0,1fr))}
.ops-page .ops-pool-metrics :deep(>div){padding:8px 0;background:transparent}
@media(max-width:767px){.ops-page .ops-pool-metrics{grid-template-columns:repeat(2,minmax(0,1fr))}}
</style>
