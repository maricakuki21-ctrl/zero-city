<template>
  <AppLayout>
    <div class="asow-page owner-experience" data-tour="shared-pool-owner">
      <header class="asow-header" data-tour="shared-pool-owner-header">
        <div>
          <h1 class="asow-title">池主中心</h1>
        </div>
        <div class="asow-header-actions">
          <RouterLink to="/account-square" class="asow-btn asow-btn-sm">共享市场</RouterLink>
          <RouterLink to="/account-square/my" class="asow-btn asow-btn-sm">我的资源</RouterLink>
          <SharedPoolGuideButton @click="replaySharedPoolGuide" />
          <button class="asow-btn asow-btn-primary" type="button" data-tour="shared-pool-owner-create" @click="toggleCreateForm">
            <Plus :size="16" />{{ showCreateForm ? '取消创建' : '创建共享池' }}
          </button>
        </div>
      </header>

      <section class="asow-overview" aria-label="池主经营概览">
        <article><span>我的共享池</span><b>{{ loading || listError ? '—' : myPools.length }}</b></article>
        <article><span>已上架</span><b>{{ loading || listError ? '—' : listedPoolCount }}</b></article>
        <article><span>配置中</span><b>{{ loading || listError ? '—' : myPools.length - listedPoolCount }}</b></article>
        <article><span>使用中的席位</span><b>{{ loading || listError ? '—' : totalSeatCount }}</b></article>
      </section>

      <nav class="owner-view-tabs" aria-label="池主中心分区">
        <button type="button" :class="{ active: ownerTab === 'pools' }" :aria-pressed="ownerTab === 'pools'" @click="ownerTab = 'pools'"><Network :size="16" />我的共享池</button>
        <RouterLink to="/wallet#shared-pool-earnings" class="asow-btn" data-testid="owner-wallet-link" data-tour="shared-pool-owner-wallet"><Wallet :size="16" />收益与账单<ArrowUpRight :size="14" /></RouterLink>
      </nav>

      <section v-if="ownerTab === 'pools' && showCreateForm" ref="createPanelRef" class="asow-create-panel">
        <div class="asow-section-head">
          <div>
            <h2>创建共享池</h2>
            <p>{{ legacyCompatibility ? '旧版兼容配置' : '先起个名字，接着连接你的资源。' }}</p>
          </div>
          <details class="owner-advanced"><summary>高级选项</summary><button class="asow-btn asow-btn-sm" type="button" @click="legacyCompatibility = !legacyCompatibility">{{ legacyCompatibility ? '返回简易创建' : '旧版兼容创建' }}</button></details>
        </div>

        <template v-if="!legacyCompatibility">
          <div class="owner-create-layout"><div>
          <div class="asow-form-grid">
            <label class="asow-field">
              <span>池子名称 <b>*</b></span>
              <input v-model="form.name" class="asow-input" placeholder="例：稳定 OpenAI 兼容资源池" maxlength="100" :disabled="creating" />
            </label>
            <label class="asow-field">
              <span>简介</span>
              <textarea v-model="form.description" class="asow-input owner-description" placeholder="一句话描述资源特点" maxlength="500" :disabled="creating" rows="3" />
            </label>
          </div>
          <div class="asow-form-actions">
            <span v-if="createError" class="asow-create-error" role="alert">{{ createError }}</span>
            <RouterLink v-if="createdNativeDraftId" :to="{ name: 'AccountSquareOwnerPoolDetail', params: { id: createdNativeDraftId }, query: { setup: 'connect' } }" class="asow-btn asow-btn-primary">继续已创建的草稿 #{{ createdNativeDraftId }}</RouterLink>
            <button v-if="createError && !createdNativeDraftId" class="asow-btn asow-btn-sm" type="button" :disabled="creating" @click="startNewNativeDraft">重新开始草稿</button>
            <button class="asow-btn asow-btn-sm" type="button" @click="showCreateForm = false">取消</button>
            <button v-if="!createdNativeDraftId" class="asow-btn asow-btn-primary" type="button" :disabled="!canCreate || creating" @click="createPool">
              {{ creating ? '创建中…' : '继续接入资源' }}<ArrowRight :size="16" />
            </button>
          </div>
          <p class="owner-form-note">保存为未发布草稿，可随时回来继续。</p>
          </div><aside><div class="owner-preview-label">市场卡片预览</div><PoolListingPreview :name="form.name" :description="form.description" /></aside></div>
        </template>
        <template v-else>
        <div class="asow-mode-picker">
          <button type="button" :class="['asow-mode-option', { 'asow-mode-option-on': !form.account_mode_enabled }]" @click="setAccountModeEnabled(false)">
            <b>单凭证模式</b>
            <span>只用池级 Base URL + Key。运行时不会再走账号列表，适合单一上游。</span>
          </button>
          <button type="button" :class="['asow-mode-option', { 'asow-mode-option-on': form.account_mode_enabled }]" @click="setAccountModeEnabled(true)">
            <b>多账号池模式</b>
            <span>先创建池控制面，再在详情导入 OAuth/API Key 账号；调度只走账号凭证。</span>
          </button>
        </div>
        <p class="asow-mode-mutex-note">
          {{ form.account_mode_enabled
            ? '多账号模式下，创建时不保存池级 Base URL / Key；账号、模型与检测在详情页完成，未通过检测前不会公开上架。'
            : '单凭证模式下，请只维护池级凭证。若之后改成多账号，系统会下架并要求逐账号重新检测。' }}
        </p>

        <div class="asow-form-grid">
          <label class="asow-field">
            <span>池子名称</span>
            <input v-model="form.name" class="asow-input" :placeholder="suggestedPoolName || '例：稳定 OpenAI 兼容资源池'" />
            <small v-if="!form.name.trim() && suggestedPoolName">留空将使用建议名称：{{ suggestedPoolName }}</small>
          </label>
          <label class="asow-field">
            <span>简介</span>
            <input v-model="form.description" class="asow-input" placeholder="一句话描述资源特点" />
          </label>
          <div class="asow-field asow-field-full">
            <span>供应商</span>
            <div class="asow-provider-grid">
              <button
                v-for="preset in providerPresets"
                :key="preset.id"
                type="button"
                class="asow-provider-card"
                :class="{ 'asow-provider-card-on': selectedProviderId === preset.id, limited: preset.supportLevel === 'limited' }"
                @click="selectProviderPreset(preset.id)"
              >
                <b>{{ preset.label }}</b>
                <small>{{ preset.hint }}</small>
              </button>
            </div>
            <label v-if="selectedProviderId === 'custom'" class="asow-field" style="margin-top:10px">
              <span>自定义供应商名</span>
              <input v-model="form.provider" class="asow-input" placeholder="my-provider" />
            </label>
            <small class="asow-help">主流接口请直接点选；只有自定义才需要手输名称。</small>
          </div>
          <label class="asow-field">
            <span>服务 Base URL <b v-if="!form.account_mode_enabled">*</b></span>
            <input v-model="form.upstream_base_url" class="asow-input" :disabled="form.account_mode_enabled" :placeholder="form.account_mode_enabled ? '多账号池创建后在账号详情配置' : 'https://api.openai.com/v1'" @input="resetCreateModelState" />
            <small v-if="form.account_mode_enabled">创建时不保存池级地址；每个账号单独配置。</small>
          </label>
          <label class="asow-field">
            <span>服务 API Key <b v-if="!form.account_mode_enabled">*</b></span>
            <input v-model="form.upstream_api_key" class="asow-input" type="password" :disabled="form.account_mode_enabled" :placeholder="form.account_mode_enabled ? '多账号池创建后在账号详情配置' : 'sk-...'" @input="resetCreateModelState" />
            <small v-if="form.account_mode_enabled">不会把池级 Key 作为多账号兜底。</small>
          </label>
        </div>

        <div class="asow-model-fetch-card">
          <div class="asow-model-fetch-head">
            <div>
              <h3>开放模型 <b v-if="!form.account_mode_enabled">*</b></h3>
              <p>{{ form.account_mode_enabled ? '多账号池创建后，从账号导入和检测结果确认开放模型。' : '点击按钮读取上游 /v1/models；不会要求手动输入逗号分隔模型。' }}</p>
            </div>
            <button v-if="!form.account_mode_enabled" class="asow-btn asow-btn-primary" type="button" :disabled="!canFetchModels || fetchingModels" @click="pullUpstreamModels">
              {{ fetchingModels ? '拉取中…' : '拉取模型' }}
            </button>
          </div>

          <p v-if="modelFetchError" class="asow-fetch-message asow-fetch-error">{{ modelFetchError }}</p>
          <p v-else-if="modelFetchMessage" class="asow-fetch-message asow-fetch-success">{{ modelFetchMessage }}</p>
          <p v-else-if="form.account_mode_enabled" class="asow-fetch-message">创建后进入经营控制台导入账号；模型、探针和上架检测会以账号为准。</p>
          <p v-else class="asow-fetch-message">填写 Base URL 和 API Key 后即可拉取；Key 不会在浏览器长期保存。</p>

          <div v-if="fetchedModels.length > 0" class="asow-model-picker">
            <div class="asow-model-toolbar">
              <span>已选择 {{ selectedModels.length }}/{{ fetchedModels.length }} 个模型</span>
              <div>
                <button class="asow-link-btn" type="button" @click="selectAllModels">全选</button>
                <button class="asow-link-btn" type="button" @click="clearSelectedModels">清空</button>
              </div>
            </div>
            <label v-if="selectedModels.length > 0" class="asow-field asow-primary-select">
              <span>主力模型</span>
              <select v-model="primaryModel" class="asow-input" @change="suggestPoolNameFromPrimary">
                <option v-for="model in selectedModels" :key="model" :value="model">{{ model }}</option>
              </select>
              <small>用于生成默认池名：供应商-主力模型。</small>
            </label>
            <label v-if="selectedModels.length > 0" class="asow-field asow-primary-select">
              <span>检测模型（探针） <b>*</b></span>
              <select v-model="probeModel" class="asow-input">
                <option disabled value="">请选择用于连通/满血检测的模型</option>
                <option v-for="model in selectedModels" :key="`probe-${model}`" :value="model">{{ model }}</option>
              </select>
              <small>必须从已开放模型中选择。未选检测模型不能创建，也无法完成上架检测。</small>
            </label>
            <div class="asow-model-grid">
              <div
                v-for="model in fetchedModels"
                :key="model"
                class="asow-model-card"
                :class="{ 'asow-model-card-on': selectedModelSet.has(model) }"
                role="button"
                tabindex="0"
                @click="toggleModel(model)"
                @keydown.enter.space.prevent="toggleModel(model)"
              >
                <div class="asow-model-card-top">
                  <span class="asow-model-check">{{ selectedModelSet.has(model) ? '✓' : '' }}</span>
                  <span>{{ model }}</span>
                </div>
                <div v-if="selectedModelSet.has(model)" class="asow-model-rate-row" @click.stop>
                  <span>倍率</span>
                  <input
                    :value="selectedModelRates[model] ?? form.rate_multiplier"
                    class="asow-model-rate-input"
                    type="number" min="0.0001" step="0.0001"
                    @input="setModelRate(model, $event)"
                  />
                </div>
              </div>
            </div>
          </div>
        </div>

        <div class="asow-form-grid">
          <label class="asow-field">
            <span>验证方式</span>
            <select v-model="form.verification_mode" class="asow-input">
              <option value="full_check">满血验证（主流模型必选）</option>
              <option value="professional_review">专业核验（差异化 / 自研模型）</option>
            </select>
            <small>主流模型必须走满血验证；差异化模型可走专业核验，但需要写明理由。</small>
          </label>
          <label class="asow-field">
            <span>专业核验说明</span>
            <input v-model="form.verification_exemption_reason" class="asow-input" :disabled="form.verification_mode !== 'professional_review'" placeholder="例如：自研差异化模型，以人工规则核验输出质量与稳定性" />
            <small>仅在“专业核验”时必填，公开市场会显示简化后的可信说明。</small>
          </label>
          <label class="asow-field">
            <span>调用倍率</span>
            <input v-model.number="form.rate_multiplier" class="asow-input" type="number" min="0.0001" step="0.0001" />
            <small>用户调用成本 = 平台模型价格 × 倍率。</small>
          </label>
          <label class="asow-field">
            <span>最多席位</span>
            <input v-model.number="form.max_users" class="asow-input" type="number" min="1" />
          </label>
          <label class="asow-field">
            <span>入场最低余额</span>
            <input v-model.number="form.min_balance_admission" class="asow-input" type="number" min="0" step="0.01" />
            <small>余额低于该值的用户不能加入。</small>
          </label>
          <label class="asow-field">
            <span>每小时席位费</span>
            <input v-model.number="form.hourly_seat_fee" class="asow-input" type="number" min="0" step="0.001" />
            <small>0 表示免费席位。</small>
          </label>
          <label class="asow-field">
            <span>席位费抵扣门槛（余额/时）</span>
            <input v-model.number="form.hourly_min_usage_waiver" class="asow-input" type="number" min="0" step="0.001" />
            <small>当小时 API 消耗会按比例抵扣席位费，达到门槛时全额抵扣。</small>
          </label>
          <label class="asow-field">
            <span>账号级并发</span>
            <input v-model.number="form.account_concurrency" class="asow-input" type="number" min="1" />
          </label>
          <label class="asow-field">
            <span>单用户并发</span>
            <input v-model.number="form.user_concurrency" class="asow-input" type="number" min="1" />
          </label>
          <label class="asow-field">
            <span>单模型并发</span>
            <input v-model.number="form.model_concurrency" class="asow-input" type="number" min="0" step="1" />
            <small>0 = 继承池级并发，不额外限制单个模型。</small>
          </label>
        </div>

        <div class="asow-rule-panel">
          <div class="asow-rule-item"><b>凭证保护</b><span>服务 Key 会加密保存，创建完成后不再显示明文。</span></div>
          <div class="asow-rule-item"><b>验证策略</b><span>{{ verificationPolicyHint }}</span></div>
          <div class="asow-rule-item"><b>当前平台规则</b><span>加入后 10 分钟内未产生首次调用，可按平台规则免费退出。</span></div>
          <div class="asow-rule-item"><b>当前平台规则</b><span>连续 2 小时无真实 API 调用，系统会自动释放席位并结算已产生的整小时席位费。</span></div>
          <div class="asow-rule-item"><b>本次配置</b><span>席位费按小时结算：{{ formatCredit(form.hourly_seat_fee) }}/时；后续修改从下一整点窗口生效。</span></div>
          <div class="asow-rule-item"><b>本次配置</b><span>当小时 API 消耗按比例抵扣席位费；达到 {{ formatCredit(form.hourly_min_usage_waiver) }} 时全额抵扣，已开始窗口不追溯。</span></div>
        </div>

        <div class="asow-form-actions">
          <span v-if="createError" class="asow-create-error">{{ createError }}</span>
          <button class="asow-btn asow-btn-sm" type="button" @click="showCreateForm = false">取消</button>
          <button class="asow-btn asow-btn-primary" type="button" :disabled="!canCreate || creating" @click="createPool">
            {{ creating ? '创建中…' : '确认创建' }}
          </button>
        </div>
        </template>
      </section>

      <section v-if="ownerTab === 'pools' && !showCreateForm" class="asow-section" data-tour="shared-pool-owner-pools">
        <div class="asow-section-head">
          <div>
            <h2>我的共享池</h2>
          </div>
          <button class="asow-btn asow-btn-sm owner-icon-button" type="button" :disabled="loading" aria-label="刷新池列表" title="刷新池列表" @click="loadPools"><RefreshCw :size="16" /></button>
        </div>
        <div class="owner-list-toolbar">
          <label><Search :size="16" /><input v-model="poolQuery" type="search" aria-label="搜索我的共享池" placeholder="搜索名称或模型" /></label>
          <select v-model="poolFilter" aria-label="筛选池状态"><option value="all">全部状态</option><option value="listed">已上架</option><option value="draft">配置中</option><option value="paused">已停用</option><option value="attention">需要处理</option></select>
        </div>
        <p v-if="listError" class="asow-load-error" role="alert">{{ listError }}</p>

        <div v-if="loading" class="asow-loading">
          <div class="asow-spinner"></div>
          <span>加载中…</span>
        </div>
        <div v-else-if="myPools.length === 0" class="asow-empty">
          <img :src="zeroCityMascots.sharedPoolOwner" alt="" width="88" height="88" />
          <p class="asow-empty-title"><span class="asow-keep-phrase">还没有共享池</span></p>
          <p class="asow-empty-sub">把你的模型资源分享给需要的人。</p>
          <button v-if="!showCreateForm" class="asow-btn asow-btn-primary" @click="toggleCreateForm"><Plus :size="16" />创建第一个池</button>
        </div>
        <div v-else-if="!visiblePools.length" class="asow-empty"><p>没有匹配的共享池</p><button class="asow-btn" @click="poolQuery = ''; poolFilter = 'all'">清除筛选</button></div>
        <div v-else class="asow-pool-list">
          <article v-for="pool in visiblePools" :key="pool.id" class="asow-pool-card" :class="{ 'asow-pool-card-has-skin': Boolean(pool.card_skin_key && pool.card_skin_rarity) }">
            <div
              v-if="pool.card_skin_key && pool.card_skin_rarity"
              class="asow-pool-card-skin"
              :style="poolCardStyle(pool.card_skin_key, pool.card_skin_rarity)"
              aria-hidden="true"
            ></div>
            <div class="asow-pool-card-overlay" aria-hidden="true"></div>
            <div class="asow-pool-card-body">
              <div class="asow-pool-card-head">
              <div class="asow-pool-avatar">{{ pool.name.slice(0, 1).toUpperCase() }}</div>
              <div class="asow-pool-meta">
                <b>{{ pool.name }}</b>
                <div class="asow-pool-status-row">
                  <span class="asow-status-chip" :class="`asow-s-${pool.status}`">{{ ownerPoolStatusLabel(pool) }}</span>
                  <span v-if="isObservationPool(pool)" class="asow-watch-chip">观察中</span>
                  <span :class="pool.listed ? 'asow-listed-chip' : 'asow-unlisted-chip'">{{ pool.listed ? '已上架' : '未上架' }}</span>
                </div>
              </div>
            </div>

            <div class="asow-pool-card-stats">
              <div class="asow-mini-stat"><span>成员</span><b>{{ pool.current_users || 0 }}/{{ pool.max_users || 0 }}</b></div>
              <div class="asow-mini-stat"><span>账号</span><b>{{ pool.account_summary?.total_accounts || 0 }}</b></div>
              <div class="asow-mini-stat"><span>检测</span><b :class="probeScoreClass(pool)">{{ formatProbeScore(pool) }}</b></div>
              <div class="asow-mini-stat"><span>调用</span><b>{{ pool.total_calls || 0 }}</b></div>
            </div>

              <div class="asow-pool-card-actions">
                <RouterLink :to="`/account-square/owner/pools/${pool.id}`" class="asow-btn asow-btn-sm asow-btn-primary">{{ pool.listed ? '管理' : '继续配置' }}<ArrowRight :size="14" /></RouterLink>
              <details class="owner-more"><summary aria-label="更多池操作" title="更多池操作"><MoreHorizontal :size="18" /></summary><div class="owner-more-actions">
              <button class="asow-btn asow-btn-sm" type="button" :class="pool.card_skin_key ? 'asow-btn-skin-active' : ''" @click="toggleSkinSelector(pool.id)">
                {{ pool.card_skin_key ? '更换背景' : '设置背景' }}
              </button>
              <button
                v-if="pool.listed && (!pool.native_onboarding_state || pool.native_onboarding_state === 'legacy_existing')"
                class="asow-btn asow-btn-sm"
                type="button"
                :disabled="governanceActingPoolId === pool.id || isObservationPool(pool)"
                @click="movePoolToObservation(pool)"
              >
                {{ governanceActingPoolId === pool.id && governanceAction === 'watch' ? '处理中…' : (isObservationPool(pool) ? '观察中' : '降级观察') }}
              </button>
              <button
                v-if="!pool.native_onboarding_state || pool.native_onboarding_state === 'legacy_existing'"
                class="asow-btn asow-btn-sm"
                type="button"
                :disabled="governanceActingPoolId === pool.id || !isObservationPool(pool)"
                @click="restorePoolPublic(pool)"
              >
                {{ governanceActingPoolId === pool.id && governanceAction === 'restore' ? '处理中…' : (pool.listed ? '恢复公开' : '重新检测') }}
              </button>
              <button
                class="asow-btn asow-btn-sm asow-btn-danger"
                type="button"
                :disabled="governanceActingPoolId === pool.id"
                @click="delistPool(pool)"
              >
                {{ governanceActingPoolId === pool.id && governanceAction === 'delist' ? '处理中…' : pool.owner_paused ? '恢复为待发布' : '停用共享池' }}
              </button>
              <button
                v-if="pool.card_skin_key"
                class="asow-btn asow-btn-sm"
                type="button"
                :disabled="clearingSkinPoolId === pool.id"
                @click="clearCardSkin(pool.id)"
              >
                {{ clearingSkinPoolId === pool.id ? '清除中…' : '清除背景' }}
              </button>
              <button
                class="asow-btn asow-btn-sm asow-btn-danger"
                type="button"
                :disabled="deletingPoolId === pool.id"
                @click="removePool(pool)"
              >
                {{ deletingPoolId === pool.id ? '删除中…' : '删除池子' }}
              </button>
              </div></details><span :class="['asow-status-mini', listingStatusChip(pool).cls]">{{ listingStatusChip(pool).label }}</span>
              </div>
              <div class="asow-pool-next-step">
                <b>下一步</b>
                <span>{{ poolNextStep(pool) }}</span>
              </div>
              <div v-if="skinSelectorPoolId === pool.id" class="asow-skin-panel">
                <div class="asow-skin-head">
                  <div>
                    <h3>选择收藏卡背景</h3>
                    <p>只保存你已持有的收藏卡引用。背景会自动透明化处理，保证市场卡片和详情页文字可读。</p>
                  </div>
                  <button class="asow-btn asow-btn-sm" type="button" @click="closeSkinSelector">关闭</button>
                </div>
                <div v-if="loadingCards" class="asow-loading asow-loading-inline">
                  <div class="asow-spinner"></div>
                  <span>加载收藏卡…</span>
                </div>
                <div v-else-if="availableCards.length === 0" class="asow-empty asow-empty-inline">
                  <p class="asow-empty-title">暂无收藏卡</p>
                  <p class="asow-empty-sub">先去每日礼盒或收藏卡册获得收藏卡，再回来设置共享池背景。</p>
                </div>
                <div v-else class="asow-skin-grid">
                <button
                  v-for="card in availableCards"
                  :key="`${pool.id}-${card.card_key}-${card.serial_no || 0}`"
                  class="asow-skin-card"
                  :class="{ 'is-selected': pool.card_skin_key === card.card_key }"
                  :disabled="settingSkinPoolId === pool.id"
                  @click="setCardSkin(pool.id, card.card_key, card.rarity)"
                >
                  <img :src="collectibleCardImage(card.card_key, card.rarity)" :alt="zeroCityCardDisplayName(card.card_key)" class="asow-skin-card-img asow-skin-card-img-compact" />
                  <span class="asow-skin-card-meta">
                    <b>{{ zeroCityCardDisplayName(card.card_key) }}</b>
                    <small>{{ zeroCityCardRarityDisplayName(card.rarity) }}</small>
                  </span>
                  </button>
                </div>
              </div>
            </div>
          </article>
        </div>
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, nextTick, ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import { ArrowRight, ArrowUpRight, MoreHorizontal, Network, Plus, RefreshCw, Search, Wallet } from '@lucide/vue'
import PoolListingPreview from '@/features/bizdecipher/components/shared-pool/PoolListingPreview.vue'
import { zeroCityMascots } from '@/features/bizdecipher/constants/zeroCityMascots'
import '@/features/bizdecipher/components/shared-pool/pool-owner-experience.css'
import SharedPoolGuideButton from '@/features/bizdecipher/components/shared-pool/SharedPoolGuideButton.vue'
import { parseSharedPoolMultiplier } from '@/features/bizdecipher/components/shared-pool/sharedPoolPricing'
import { useSharedPoolOnboarding } from '@/composables/useSharedPoolOnboarding'
import {
  createNativeSharedPoolDraft,
  createSharedPool,
  createSharedPoolNativeOperationID,
  deleteSharedPool,
  setSharedPoolOwnerPause,
  fetchSharedPoolUpstreamModels,
  listMySharedPools,
  updateSharedPool,
  type CreateSharedPoolPayload,
  type SharedPool,
  type SharedPoolModelConfig,
  type UpdateSharedPoolPayload,
} from '@/features/bizdecipher/api/bizdecipher'
import { usePoolCardSkin } from '@/composables/usePoolOwner'
import { zeroCityCardDisplayName, zeroCityCardRarityDisplayName } from '@/constants/zeroCityCardManifest'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import {
  SHARED_POOL_PROVIDER_PRESETS,
  getSharedPoolProviderPreset,
  normalizeOpenAICompatibleBaseURL,
  providerPresetLabel,
  type SharedPoolProviderPresetId
} from '@/features/shared-pool/providerPresets'

type PoolCreateForm = {
  name: string
  description: string
  upstream_base_url: string
  upstream_api_key: string
  provider: string
  rate_multiplier: number
  max_users: number
  min_balance_admission: number
  hourly_seat_fee: number
  hourly_min_usage_waiver: number
  account_concurrency: number
  user_concurrency: number
  model_concurrency: number
  account_mode_enabled: boolean
  verification_mode: 'full_check' | 'professional_review'
  verification_exemption_reason: string
}

const router = useRouter()
const { replayGuide: replaySharedPoolGuide } = useSharedPoolOnboarding({ scope: 'owner' })

const appStore = useAppStore()
const loading = ref(false)
const creating = ref(false)
const fetchingModels = ref(false)
const showCreateForm = ref(false)
const legacyCompatibility = ref(false)
const createPanelRef = ref<HTMLElement | null>(null)
const myPools = ref<SharedPool[]>([])
const ownerTab = ref<'pools' | 'earnings'>('pools')
const poolQuery = ref('')
const poolFilter = ref('all')
const listError = ref('')
const visiblePools = computed(() => myPools.value.filter(pool => {
  const matches = `${pool.name} ${(pool.models || []).join(' ')}`.toLowerCase().includes(poolQuery.value.trim().toLowerCase())
  const needsAttention = pool.native_onboarding_state === 'supply_needs_attention' || pool.status === 'limited'
  return matches && (poolFilter.value === 'all' || (poolFilter.value === 'listed' && pool.listed) || (poolFilter.value === 'draft' && !pool.listed && !pool.owner_paused) || (poolFilter.value === 'paused' && pool.owner_paused) || (poolFilter.value === 'attention' && needsAttention && !pool.owner_paused))
}))
const form = ref<PoolCreateForm>(emptyForm())
const providerPresets = SHARED_POOL_PROVIDER_PRESETS
const selectedProviderId = ref<SharedPoolProviderPresetId>('openai_compatible')
const fetchedModels = ref<string[]>([])
const selectedModelNames = ref<string[]>([])
const primaryModel = ref('')
const modelFetchMessage = ref('')
const modelFetchError = ref('')
const createError = ref('')
const selectedModelRates = ref<Record<string, number>>({})
const nativeDraftOperationID = ref('')
const createdNativeDraftId = ref<number | null>(null)

async function toggleCreateForm(): Promise<void> {
  ownerTab.value = 'pools'
  showCreateForm.value = !showCreateForm.value
  if (!showCreateForm.value) return
  if (!legacyCompatibility.value) nativeDraftOperationID.value = readNativeDraftOperationID()
  await nextTick()
  createPanelRef.value?.scrollIntoView?.({ behavior: 'smooth', block: 'start' })
  createPanelRef.value?.querySelector<HTMLInputElement>('input:not([disabled])')?.focus({ preventScroll: true })
}

const probeModel = ref<string>('')
const deletingPoolId = ref<number | null>(null)
const governanceActingPoolId = ref<number | null>(null)
const governanceAction = ref<'watch' | 'restore' | 'delist' | ''>('')
const {
  skinSelectorPoolId,
  availableCards,
  loadingCards,
  settingSkinPoolId,
  clearingSkinPoolId,
  setCardSkin,
  clearCardSkin,
  openSkinSelector,
  closeSkinSelector,
} = usePoolCardSkin(appStore, loadPools)
const selectedModels = computed(() => uniqueModels(selectedModelNames.value))
const selectedModelSet = computed(() => new Set(selectedModels.value))
const listedPoolCount = computed(() => myPools.value.filter((pool) => pool.listed).length)
const totalSeatCount = computed(() => myPools.value.reduce((sum, pool) => sum + Number(pool.current_users || 0), 0))
const canFetchModels = computed(() => Boolean(
  !form.value.account_mode_enabled &&
  form.value.upstream_base_url.trim() &&
  form.value.upstream_api_key.trim()
))
const suggestedPoolName = computed(() => {
  const model = primaryModel.value || selectedModels.value[0] || ''
  if (!model) return ''
  return `${providerLabel(form.value.provider)}-${model}`
})
const canCreate = computed(() => {
  if (!legacyCompatibility.value) return Boolean(form.value.name.trim())
  const hasName = Boolean(form.value.name.trim() || suggestedPoolName.value)
  if (!hasName) return false
  if (form.value.verification_mode === 'professional_review' && !form.value.verification_exemption_reason.trim()) {
    return false
  }
  if (form.value.account_mode_enabled) return true
  return Boolean(
    form.value.upstream_base_url.trim() &&
    form.value.upstream_api_key.trim() &&
    selectedModels.value.length > 0 &&
    probeModel.value &&
    selectedModels.value.includes(probeModel.value)
  )
})

function emptyForm(): PoolCreateForm {
  return {
    name: '',
    description: '',
    upstream_base_url: '',
    upstream_api_key: '',
    provider: 'openai_compatible',
    rate_multiplier: 1,
    max_users: 20,
    min_balance_admission: 0,
    hourly_seat_fee: 0,
    hourly_min_usage_waiver: 0,
    account_concurrency: 1,
    user_concurrency: 1,
    model_concurrency: 0,
    account_mode_enabled: false,
    verification_mode: 'full_check',
    verification_exemption_reason: '',
  }
}

function uniqueModels(models: string[]): string[] {
  const seen = new Set<string>()
  const result: string[] = []
  for (const raw of models) {
    const model = raw.trim()
    const key = model.toLowerCase()
    if (!model || seen.has(key)) continue
    seen.add(key)
    result.push(model)
  }
  return result
}

function providerLabel(provider: string): string {
  return providerPresetLabel(provider.trim() || 'openai_compatible')
}

function selectProviderPreset(id: SharedPoolProviderPresetId) {
  selectedProviderId.value = id
  const preset = getSharedPoolProviderPreset(id)
  if (id !== 'custom') {
    form.value.provider = preset.providerValue
  } else if (!form.value.provider || form.value.provider === 'openai' || form.value.provider === 'openai_compatible') {
    form.value.provider = 'custom'
  }
  if (form.value.account_mode_enabled) {
    resetCreateModelState()
    return
  }
  if (!form.value.upstream_base_url.trim() || selectedProviderId.value !== 'custom') {
    // only auto-fill when empty or switching non-custom templates
    if (preset.defaultBaseURL) {
      if (id !== 'custom') form.value.upstream_base_url = preset.defaultBaseURL
    }
  } else if (id !== 'custom' && preset.defaultBaseURL) {
    form.value.upstream_base_url = preset.defaultBaseURL
  }
  resetCreateModelState()
}

function setAccountModeEnabled(enabled: boolean): void {
  if (form.value.account_mode_enabled === enabled) return
  form.value.account_mode_enabled = enabled
  if (enabled) {
    form.value.upstream_base_url = ''
    form.value.upstream_api_key = ''
  } else {
    const preset = getSharedPoolProviderPreset(selectedProviderId.value)
    if (preset.defaultBaseURL) {
      form.value.upstream_base_url = preset.defaultBaseURL
    }
  }
  resetCreateModelState()
}

function normalizeFormBaseURL() {
  if (form.value.account_mode_enabled) return
  const preset = getSharedPoolProviderPreset(selectedProviderId.value)
  form.value.upstream_base_url = normalizeOpenAICompatibleBaseURL(form.value.upstream_base_url, preset.autoNormalizeV1)
}

function modelConfigs(models: string[], provider: string): SharedPoolModelConfig[] {
  const rateMultiplier = parseSharedPoolMultiplier(form.value.rate_multiplier) ?? 1
  const maxConcurrency = Math.max(0, Math.floor(Number(form.value.model_concurrency) || 0))
  return models.map((model) => ({
    provider: providerLabel(provider),
    model_name: model,
    upstream_model_name: model,
    rate_multiplier: selectedModelRates.value[model] ?? rateMultiplier,
    five_hour_protection_percent: 100,
    seven_day_protection_percent: 100,
    daily_protection_percent: 100,
    max_concurrency: maxConcurrency,
    model_open: true,
  }))
}

function setModelRate(model: string, event: Event): void {
  const val = parseFloat((event.target as HTMLInputElement).value)
  if (parseSharedPoolMultiplier(val) == null) return
  selectedModelRates.value = { ...selectedModelRates.value, [model]: val }
}

function syncPrimaryModel(): void {
  const models = selectedModels.value
  if (models.length === 0) {
    primaryModel.value = ''
    probeModel.value = ''
    return
  }
  if (!models.includes(primaryModel.value)) {
    primaryModel.value = models[0]
  }
  ensureProbeModelValid()
}

function selectAllModels(): void {
  selectedModelNames.value = [...fetchedModels.value]
  syncPrimaryModel()
}

function clearSelectedModels(): void {
  selectedModelNames.value = []
  probeModel.value = ''
  selectedModelRates.value = {}
  syncPrimaryModel()
}

function toggleModel(model: string): void {
  if (selectedModelSet.value.has(model)) {
    selectedModelNames.value = selectedModelNames.value.filter((item) => item !== model)
  } else {
    selectedModelNames.value = uniqueModels([...selectedModelNames.value, model])
  }
  syncPrimaryModel()
}

function suggestPoolNameFromPrimary(): void {
  createError.value = ''
}

const verificationPolicyHint = computed(() => {
  if (form.value.verification_mode === 'professional_review') {
    return form.value.verification_exemption_reason.trim()
      ? `当前按专业核验处理：${form.value.verification_exemption_reason.trim()}`
      : '差异化 / 自研模型可走专业核验，但必须写明理由并接受人工/规则核验。'
  }
  return '主流模型默认走满血验证；检测分达标后才会以“满血验证”标识公开展示。'
})

function formatCredit(value: number): string {
  const amount = Number(value) || 0
  return amount > 0 ? amount.toFixed(4) : '0'
}

function sanitizeSecretMessage(message: string): string {
  let sanitized = message.trim()
  const secret = form.value.upstream_api_key.trim()
  if (!secret) return sanitized
  sanitized = sanitized.split(secret).join('[redacted]')
  if (secret.length >= 8) {
    sanitized = sanitized.split(secret.slice(0, 4)).join('[redacted]')
    sanitized = sanitized.split(secret.slice(-4)).join('[redacted]')
  }
  return sanitized
}

function errorMessage(error: unknown, fallback: string): string {
  return sanitizeSecretMessage(extractApiErrorMessage(error, fallback))
}

function nativeDraftStorageKey(): string {
  return 'shared-pool-native-draft-operation'
}

function readNativeDraftOperationID(): string {
  const operationID = createSharedPoolNativeOperationID()
  try {
    if (typeof window === 'undefined') return operationID
    const existing = window.sessionStorage.getItem(nativeDraftStorageKey())
    if (existing) return existing
    window.sessionStorage.setItem(nativeDraftStorageKey(), operationID)
  } catch {
    // Keep retries stable in this component even when browser storage is blocked.
  }
  return operationID
}

function clearNativeDraftOperationID(): void {
  try {
    if (typeof window !== 'undefined') window.sessionStorage.removeItem(nativeDraftStorageKey())
  } catch {
    // A saved draft must remain accessible even without browser storage.
  }
  nativeDraftOperationID.value = ''
}

function startNewNativeDraft(): void {
  if (creating.value) return
  clearNativeDraftOperationID()
  createdNativeDraftId.value = null
  form.value = emptyForm()
  createError.value = ''
}

function resetCreateModelState(): void {
  fetchedModels.value = []
  selectedModelNames.value = []
  selectedModelRates.value = {}
  primaryModel.value = ''
  probeModel.value = ''
  modelFetchMessage.value = ''
  modelFetchError.value = ''
  createError.value = ''
}

function collectibleCardImage(cardKey: string, rarity: string): string {
  return `/assets/zero-point-city/cards/collectible/${rarity}/${cardKey}.png`
}

function poolCardStyle(cardKey: string, rarity: string): Record<string, string> {
  return {
    '--asow-card-skin-url': `url(${collectibleCardImage(cardKey, rarity)})`,
  }
}

function toggleSkinSelector(poolId: number): void {
  if (skinSelectorPoolId.value === poolId) {
    closeSkinSelector()
    return
  }
  openSkinSelector(poolId)
}

function hasProbeEvidence(pool: SharedPool): boolean {
  return Boolean(
    pool.last_probe_at ||
    pool.last_successful_probe_at ||
    (pool.last_probe_full_check_total ?? 0) > 0 ||
    (pool.consecutive_probe_failures ?? 0) > 0 ||
    (pool.account_summary?.full_check_total_accounts ?? 0) > 0
  )
}

function probeScore(pool: SharedPool): number | null {
  return pool.account_mode_enabled
    ? (pool.account_summary?.average_full_check_score ?? null)
    : (pool.last_probe_full_check_score ?? pool.account_summary?.average_full_check_score ?? null)
}

function formatProbeScore(pool: SharedPool): string {
  if (!hasProbeEvidence(pool)) return '未检测'
  const score = probeScore(pool)
  return score == null ? '未检测' : `${score.toFixed(0)}%`
}

function probeScoreClass(pool: SharedPool): string {
  const score = probeScore(pool)
  const hasEvidence = Boolean(
    pool.last_probe_at ||
    pool.last_successful_probe_at ||
    (pool.last_probe_full_check_total ?? 0) > 0 ||
    (pool.account_summary?.full_check_total_accounts ?? 0) > 0
  )
  if (!hasEvidence || score == null) return 'clr-pending'
  if (score >= 70) return 'clr-good'
  if (score >= 50) return 'clr-warn'
  return 'clr-bad'
}

function isObservationPool(pool: SharedPool): boolean {
  return (String(pool.governance_status || '') === 'watch' && Number(pool.consecutive_probe_failures || 0) >= 3)
    || String(pool.status || '') === 'limited'
    || String(pool.status_note || '').includes('观察')
}

function ownerGovernancePayload(pool: SharedPool, patch: Partial<UpdateSharedPoolPayload>): UpdateSharedPoolPayload {
  return {
    expected_config_version: Number(pool.config_version || 0),
    ...patch,
  }
}

async function updateOwnerGovernance(pool: SharedPool, patch: Partial<UpdateSharedPoolPayload>, successMessage: string, action: 'watch' | 'restore' | 'delist'): Promise<void> {
  governanceActingPoolId.value = pool.id
  governanceAction.value = action
  try {
    const updated = await updateSharedPool(pool.id, ownerGovernancePayload(pool, patch))
    const index = myPools.value.findIndex((item) => item.id === pool.id)
    if (index >= 0) myPools.value[index] = updated
    appStore.showSuccess(successMessage)
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, '治理动作执行失败'))
  } finally {
    governanceActingPoolId.value = null
    governanceAction.value = ''
  }
}

async function movePoolToObservation(pool: SharedPool): Promise<void> {
  await updateOwnerGovernance(pool, {
    status_note: '池主手动降级观察，暂停继续公开加入。',
    status: 'limited',
    listed: true,
  }, '共享池已转入观察期', 'watch')
}

async function restorePoolPublic(pool: SharedPool): Promise<void> {
  if (!pool.listed) {
    appStore.showInfo('已下架的池需要重新满血检测；达标后会自动上架')
    await router.push({ name: 'AccountSquareOwnerPoolDetail', params: { id: pool.id }, query: { tab: 'detection' } })
    return
  }
  await updateOwnerGovernance(pool, {
    status_note: '',
    status: 'healthy',
    listed: true,
  }, '共享池已恢复公开状态', 'restore')
}

async function delistPool(pool: SharedPool): Promise<void> {
  if (!window.confirm(pool.owner_paused ? '恢复为待发布？检查资源后再发布，不会自动营业。' : '停用共享池？退出市场并停止新调用，保留成员、资源和历史账单。')) return
  governanceActingPoolId.value = pool.id
  governanceAction.value = 'delist'
  try {
    await setSharedPoolOwnerPause(pool.id, !pool.owner_paused, pool.config_version || 0)
    await loadPools()
  } catch (error) { createError.value = errorMessage(error, '未能确认停用状态，请刷新后重试') }
  finally { governanceActingPoolId.value = null }
}

function ownerPoolStatusLabel(pool: SharedPool): string {
  if (pool.owner_paused) return '已停用'
  if (pool.native_onboarding_state === 'draft') return '草稿'
  if (isObservationPool(pool)) return '观察中'
  const labels: Record<string, string> = {
    healthy: '健康', limited: '受限', offline: '离线', maintenance: '维护中',
  }
  return labels[pool.status] || pool.status || '未知'
}

function hasProfessionalReviewReadyEvidence(pool: SharedPool): boolean {
  const schedulable = Number(pool.account_summary?.schedulable_accounts || 0)
  return pool.account_mode_enabled
    ? schedulable > 0
    : schedulable > 0 || Boolean(pool.last_probe_success)
}

function listingStatusChip(pool: SharedPool): { label: string; cls: string } {
  if (pool.owner_paused) return { label: '已停用', cls: 'asow-chip-pending' }
  if (pool.native_onboarding_state && pool.native_onboarding_state !== 'legacy_existing') {
    const label = pool.native_onboarding_state === 'draft' ? '等待接入资源' : pool.native_onboarding_state === 'supply_ready_billing_blocked' ? '等待平台开通结算' : pool.native_onboarding_state === 'supply_needs_attention' ? '资源需要处理' : '资源配置中'
    return { label, cls: 'asow-chip-pending' }
  }
  if (isObservationPool(pool)) {
    const failures = Number(pool.consecutive_probe_failures || 0)
    if (!pool.listed) return { label: `连续失败 ${failures} 次·已下架`, cls: 'asow-chip-blocked' }
    return { label: `连续失败 ${failures} 次·观察中`, cls: 'asow-chip-pending' }
  }
  if (pool.listed) {
    return {
      label: pool.verification_mode === 'professional_review'
        ? '专业核验已上架'
        : (pool.account_mode_enabled ? '账号池可调度·已上架' : '满血验证已上架'),
      cls: 'asow-chip-listed'
    }
  }
  if (!hasProbeEvidence(pool)) {
    return { label: pool.verification_mode === 'professional_review' ? '等待专业核验' : '等待首次检测', cls: 'asow-chip-pending' }
  }
  const score = probeScore(pool)
  if (pool.verification_mode === 'professional_review') {
    if (hasProfessionalReviewReadyEvidence(pool)) {
      return { label: '专业核验可上架', cls: 'asow-chip-ready' }
    }
    return { label: '专业核验未完成', cls: 'asow-chip-blocked' }
  }
  if (pool.account_mode_enabled) {
    const passed = pool.account_summary?.full_check_passed_accounts || 0
    const total = pool.account_summary?.full_check_total_accounts || 0
    const schedulable = pool.account_summary?.schedulable_accounts || 0
    if (total <= 0) return { label: '等待账号满血检测', cls: 'asow-chip-pending' }
    if (passed > 0 && schedulable > 0) return { label: `账号检测 ${passed}/${total}·自动上架中`, cls: 'asow-chip-ready' }
    return { label: `账号检测 ${passed}/${total}·未达门槛`, cls: 'asow-chip-blocked' }
  }
  if ((pool.last_probe_full_check_total ?? 0) <= 0 && pool.last_probe_success) return { label: '基础可用·待满血检测', cls: 'asow-chip-ready' }
  if ((score ?? 0) >= 70) return { label: '达标·自动上架中', cls: 'asow-chip-ready' }
  return { label: '检测未达上架门槛', cls: 'asow-chip-blocked' }
}

function poolNextStep(pool: SharedPool): string {
  if (pool.native_onboarding_state === 'draft') return '接入 API Key 或账号，再查看实际支持的模型。'
  if (pool.native_onboarding_state === 'supply_ready_billing_blocked') return '资源已保存；平台计费上架接通后可继续发布，无需重复创建。'
  if (pool.native_onboarding_state === 'supply_needs_attention') return '打开池子，处理失败的资源并重新检查。'
  if (isObservationPool(pool)) {
    const failures = Number(pool.consecutive_probe_failures || 0)
    const remaining = Math.max(0, 9 - failures)
    if (!pool.listed) return '已自动下架。修复上游后重新执行满血检测，达标后可恢复上架。'
    return `请尽快修复上游并复测；当前连续失败 ${failures} 次，再失败 ${remaining} 次将自动下架。`
  }
  if (pool.listed) return '关注账号可调度状态、成员席位、收益账本和治理风险。'
  if (pool.account_mode_enabled && (pool.account_summary?.configured_accounts || 0) <= 0) return '添加至少一个上游账号，保存凭证并拉取模型。'
  if (!pool.models?.length && !pool.model_configs?.length) return '拉取上游模型并确认公开模型和倍率。'
  if (pool.verification_mode === 'professional_review') {
    if (!pool.verification_exemption_reason?.trim()) return '补充专业核验说明，说明差异化模型为何采用豁免上架。'
    if (!hasProfessionalReviewReadyEvidence(pool)) return '至少保留一个可调度账号，并补充基础连通证据。'
    return '专业核验说明已齐，确认可调度账号后即可公开展示。'
  }
  if (pool.account_mode_enabled) {
    const passed = pool.account_summary?.full_check_passed_accounts || 0
    const total = pool.account_summary?.full_check_total_accounts || 0
    const schedulable = pool.account_summary?.schedulable_accounts || 0
    if (total <= 0) return '对至少一个账号执行满血检测；通过且当前可用的账号才会进入路由。'
    if (passed <= 0 || schedulable <= 0) return `当前账号检测 ${passed}/${total}，请修复失败账号并重新检测。`
    return `账号检测已达标（${passed}/${total}，当前 ${schedulable} 个可调度），刷新确认自动上架状态。`
  }
  if (!hasProbeEvidence(pool)) return '执行首次满血检测，系统会记录逐项能力证据。'
  if ((pool.last_probe_full_check_total ?? 0) <= 0 && pool.last_probe_success) return '基础连通已通过，继续执行满血检测。'
  if ((probeScore(pool) ?? 0) < 70) return '查看检测失败项，修正上游模型能力或账号配置后复测。'
  return '检测已达标，刷新确认自动上架状态。'
}

async function loadPools(): Promise<void> {
  loading.value = true
  listError.value = ''
  try {
    myPools.value = await listMySharedPools()
  } catch {
    listError.value = '池列表暂时无法刷新，请重试。'
  } finally {
    loading.value = false
  }
}

async function pullUpstreamModels(): Promise<void> {
  if (form.value.account_mode_enabled || !canFetchModels.value) return
  fetchingModels.value = true
  modelFetchMessage.value = ''
  modelFetchError.value = ''
  createError.value = ''
  try {
    const result = await fetchSharedPoolUpstreamModels({
      upstream_base_url: form.value.upstream_base_url.trim(),
      upstream_api_key: form.value.upstream_api_key.trim(),
    })
    const models = uniqueModels(result.models || [])
    fetchedModels.value = models
    if (!probeModel.value || !models.includes(probeModel.value)) {
      probeModel.value = primaryModel.value || models[0] || ''
    }
    selectedModelNames.value = models
    syncPrimaryModel()
    if (models.length === 0) {
      modelFetchError.value = '上游 /v1/models 未返回可用模型，请确认账号权限。'
      return
    }
    const checkedAt = result.checked_at ? new Date(result.checked_at).toLocaleString() : '刚刚'
    modelFetchMessage.value = `已拉取 ${models.length} 个模型 · HTTP ${result.http_status || 200} · ${checkedAt}`
  } catch (error) {
    fetchedModels.value = []
    selectedModelNames.value = []
    primaryModel.value = ''
    modelFetchError.value = errorMessage(error, '拉取模型失败，请确认 Base URL、API Key 或代理可访问。')
  } finally {
    fetchingModels.value = false
  }
}

function ensureProbeModelValid(): void {
  if (!selectedModels.value.includes(probeModel.value)) {
    probeModel.value = primaryModel.value || selectedModels.value[0] || ''
  }
}

async function createPool(): Promise<void> {
  if (creating.value || createdNativeDraftId.value) return
  if (!legacyCompatibility.value) {
    if (!form.value.name.trim()) {
      createError.value = '请先填写池子名称。'
      return
    }
    creating.value = true
    createError.value = ''
    const operationID = nativeDraftOperationID.value || readNativeDraftOperationID()
    nativeDraftOperationID.value = operationID
    try {
      const created = await createNativeSharedPoolDraft({
        name: form.value.name.trim(),
        ...(form.value.description.trim() ? { description: form.value.description.trim() } : {}),
        operation_id: operationID,
      })
      createdNativeDraftId.value = created.id
      clearNativeDraftOperationID()
      await router.push({ name: 'AccountSquareOwnerPoolDetail', params: { id: created.id }, query: { setup: 'connect' } })
    } catch (error) {
      createError.value = createdNativeDraftId.value
        ? '草稿已保存，但详情页未能打开。请继续已创建的草稿，不必重复创建。'
        : errorMessage(error, '创建原生共享池草稿失败；可使用相同操作重试。')
    } finally {
      creating.value = false
    }
    return
  }
  if (!form.value.account_mode_enabled) {
    ensureProbeModelValid()
  }
  if (!canCreate.value) {
    createError.value = form.value.account_mode_enabled
      ? '请先填写池子名称，再创建多账号池控制面。'
      : '请先拉取模型，并选择检测模型（探针）后再创建。'
    return
  }
  normalizeFormBaseURL()
  const rateMultiplier = parseSharedPoolMultiplier(form.value.rate_multiplier)
  if (rateMultiplier == null) {
    createError.value = '调用倍率必须是有限数字，且不能小于 0.0001。'
    return
  }
  creating.value = true
  createError.value = ''
  const models = form.value.account_mode_enabled ? [] : selectedModels.value
  const payload: CreateSharedPoolPayload = {
    name: form.value.name.trim() || suggestedPoolName.value,
    description: form.value.description.trim(),
    upstream_base_url: form.value.account_mode_enabled ? '' : form.value.upstream_base_url.trim(),
    upstream_api_key: form.value.account_mode_enabled ? '' : form.value.upstream_api_key.trim(),
    models,
    model_configs: form.value.account_mode_enabled ? [] : modelConfigs(models, form.value.provider),
    rate_multiplier: rateMultiplier,
    max_users: Number(form.value.max_users) || 20,
    min_balance_admission: Math.max(0, Number(form.value.min_balance_admission) || 0),
    hourly_seat_fee: Math.max(0, Number(form.value.hourly_seat_fee) || 0),
    hourly_min_usage_waiver: Math.max(0, Number(form.value.hourly_min_usage_waiver) || 0),
    account_concurrency: Math.max(1, Number(form.value.account_concurrency) || 1),
    user_concurrency: Math.max(1, Number(form.value.user_concurrency) || 1),
    account_mode_enabled: form.value.account_mode_enabled,
    oauth_provider: form.value.provider,
    verification_mode: form.value.verification_mode,
    verification_exemption_reason: form.value.verification_exemption_reason.trim(),
    probe_model: form.value.account_mode_enabled ? '' : probeModel.value,
    status: 'healthy',
    listed: false,
  }

  try {
    const created = await createSharedPool(payload)
    form.value = emptyForm()
    resetCreateModelState()
    showCreateForm.value = false
    await router.push({ name: 'AccountSquareOwnerPoolDetail', params: { id: created.id } })
  } catch (error) {
    createError.value = errorMessage(error, '创建共享池失败')
  } finally {
    creating.value = false
  }
}

async function removePool(pool: SharedPool): Promise<void> {
  if (!window.confirm(`删除共享池「${pool.name}」？它将从经营列表移除，旧授权失效；历史账单和收益仍保留。临时不营业请使用“停用”。服务器会检查活跃成员与在途用量。`)) return
  deletingPoolId.value = pool.id
  try {
    await deleteSharedPool(pool.id)
    myPools.value = myPools.value.filter((item) => item.id !== pool.id)
    await loadPools()
  } catch (error) {
    createError.value = errorMessage(error, '删除共享池失败')
  } finally {
    deletingPoolId.value = null
  }
}

onMounted(loadPools)
</script>

<style scoped>
.asow-page {
  --bg: var(--module-panel, var(--zc-surface));
  --nd: color-mix(in srgb, var(--zc-shadow-dark) 24%, transparent);
  --nl: color-mix(in srgb, var(--zc-shadow-light) 58%, transparent);
  --text: var(--module-ink-strong, var(--zc-text-strong));
  --muted: var(--module-muted, var(--zc-muted));
  --teal: var(--module-accent-2, var(--zc-accent));
  --blue: var(--module-accent, var(--zc-accent-2));
  --gold: var(--zc-warning);
  --danger: var(--zc-danger);
  --raise-sm: 0 0 0 1px var(--bd-ui-line);
  --inset-sm: inset 0 0 0 1px var(--bd-ui-line);
  max-width: 960px;
  padding: 0 0 80px;
  display: flex;
  flex-direction: column;
  gap: 24px;
}

.asow-header { display: flex; align-items: flex-start; justify-content: space-between; gap: 16px; padding: 28px 0 8px; }
.asow-header-actions { display: flex; align-items: center; justify-content: flex-end; gap: 8px; flex-wrap: wrap; }
.asow-eyebrow { margin: 0; font-size: 11px; font-weight: 900; letter-spacing: .16em; text-transform: uppercase; color: var(--gold); }
.asow-title { margin: 10px 0 0; font-size: clamp(24px, 3vw, 36px); font-weight: 950; color: var(--text); }
.asow-subtitle { margin: 8px 0 0; font-size: 14px; color: var(--muted); line-height: 1.65; }

.asow-flow-strip { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 10px; padding: 20px; border-radius: 18px; background: var(--bg); box-shadow: var(--raise-sm); }
.asow-overview { display: grid; grid-template-columns: repeat(5, minmax(0, 1fr)); gap: 10px; }
.asow-overview article { min-width: 0; padding: 15px; border-radius: 16px; background: var(--bg); box-shadow: var(--raise-sm); display: grid; gap: 4px; }
.asow-overview span { color: var(--muted); font-size: 10px; font-weight: 850; }
.asow-overview b { color: var(--text); font-size: 22px; font-weight: 950; }
.asow-overview small { color: var(--muted); font-size: 10px; line-height: 1.45; }
.asow-flow-strip article { padding: 16px; border-radius: 14px; background: rgba(47, 154, 154, .07); display: flex; flex-direction: column; gap: 6px; }
.asow-flow-strip article:nth-child(2) { background: rgba(63, 127, 217, .07); }
.asow-flow-strip article:nth-child(3) { background: rgba(176, 125, 42, .08); }
.asow-flow-strip span { font-size: 10px; font-weight: 950; letter-spacing: .15em; color: var(--teal); }
.asow-flow-strip b { font-size: 14px; font-weight: 950; color: var(--text); }
.asow-flow-strip p { margin: 0; font-size: 12px; color: var(--muted); line-height: 1.6; }

.asow-create-panel { padding: 24px; border-radius: 18px; background: var(--bg); box-shadow: var(--raise-sm); display: flex; flex-direction: column; gap: 16px; }
.asow-section { display: flex; flex-direction: column; gap: 14px; }
.asow-section-head { display: flex; align-items: flex-start; justify-content: space-between; gap: 12px; flex-wrap: wrap; }
.asow-section-head h2 { margin: 0 0 4px; font-size: 18px; font-weight: 900; color: var(--text); }
.asow-section-head p { margin: 0; font-size: 13px; color: var(--muted); }
.asow-form-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 12px; }
.asow-mode-picker { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 12px; }
.asow-mode-option { padding: 16px; border: 0; border-radius: 16px; background: var(--bg); box-shadow: var(--raise-sm); color: var(--text); text-align: left; cursor: pointer; display: flex; flex-direction: column; gap: 7px; }
.asow-mode-option b { font-size: 14px; }
.asow-mode-option span { color: var(--muted); font-size: 12px; line-height: 1.6; }
.asow-mode-option-on { box-shadow: var(--inset-sm); color: var(--teal); }
.asow-field { display: flex; flex-direction: column; gap: 6px; }
.asow-field span { font-size: 12px; font-weight: 800; color: var(--text); }
.asow-field b { color: var(--danger); }
.asow-field small { color: var(--muted); font-size: 11px; line-height: 1.5; }
.asow-input { min-height: 40px; padding: 0 14px; border: none; border-radius: 12px; background: var(--bg); box-shadow: var(--inset-sm); font-size: 13px; color: var(--text); outline: none; }
.asow-primary-select { max-width: 360px; }
.asow-model-fetch-card { display: flex; flex-direction: column; gap: 12px; padding: 18px; border-radius: 16px; background: rgba(255, 255, 255, .26); box-shadow: var(--inset-sm); }
.asow-model-fetch-head { display: flex; align-items: flex-start; justify-content: space-between; gap: 12px; flex-wrap: wrap; }
.asow-model-fetch-head h3 { margin: 0 0 4px; font-size: 15px; font-weight: 950; color: var(--text); }
.asow-model-fetch-head h3 b { color: var(--danger); }
.asow-model-fetch-head p { margin: 0; font-size: 12px; color: var(--muted); line-height: 1.6; }
.asow-fetch-message { margin: 0; font-size: 12px; color: var(--muted); line-height: 1.6; }
.asow-fetch-success { color: var(--teal); font-weight: 800; }
.asow-fetch-error, .asow-create-error, .asow-load-error { color: var(--danger); font-weight: 800; }
.asow-model-picker { display: flex; flex-direction: column; gap: 12px; }
.asow-model-toolbar { display: flex; align-items: center; justify-content: space-between; gap: 10px; flex-wrap: wrap; font-size: 12px; font-weight: 800; color: var(--muted); }
.asow-model-toolbar div { display: flex; gap: 8px; }
.asow-link-btn { border: none; background: transparent; color: var(--teal); font-weight: 900; cursor: pointer; padding: 0; }
.asow-model-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 10px; max-height: 300px; overflow: auto; padding: 3px; }
.asow-model-card-top { display: flex; align-items: center; gap: 9px; }
.asow-model-rate-row { display: flex; align-items: center; gap: 6px; margin-top: 6px; font-size: 11px; color: var(--muted); font-weight: 700; }
.asow-model-rate-input { width: 58px; min-height: 26px; padding: 0 6px; border: none; border-radius: 8px; background: var(--bg); box-shadow: var(--inset-sm); font-size: 12px; color: var(--teal); outline: none; }
.asow-status-mini { font-size: 10px; font-weight: 800; text-transform: uppercase; letter-spacing: .06em; border-radius: 999px; padding: 2px 8px; }
.asow-chip-listed { background: rgba(47,154,154,.1); color: var(--teal); }
.asow-chip-ready { background: rgba(63,127,217,.1); color: var(--blue); }
.asow-chip-pending { background: rgba(101,115,134,.1); color: var(--muted); }
.asow-chip-blocked { background: rgba(185,64,64,.08); color: var(--danger); }
.asow-model-card { min-height: 44px; padding: 8px 10px; border: none; border-radius: 13px; background: var(--bg); box-shadow: var(--raise-sm); color: var(--text); cursor: pointer; display: flex; align-items: center; gap: 9px; text-align: left; font-size: 12px; font-weight: 800; }
.asow-model-card-on { color: var(--teal); box-shadow: var(--inset-sm); }
.asow-model-check { width: 18px; height: 18px; border-radius: 7px; background: rgba(47, 154, 154, .12); color: var(--teal); display: grid; place-items: center; flex-shrink: 0; font-size: 12px; }
.asow-rule-panel { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 10px; }
.asow-rule-item { padding: 12px; border-radius: 14px; background: rgba(63, 127, 217, .07); display: flex; flex-direction: column; gap: 5px; }
.asow-rule-item b { font-size: 11px; color: var(--blue); font-weight: 950; letter-spacing: .04em; }
.asow-rule-item span { font-size: 12px; line-height: 1.55; color: var(--muted); }
.asow-form-actions { display: flex; justify-content: flex-end; align-items: center; gap: 8px; flex-wrap: wrap; }
.asow-create-error { margin-right: auto; font-size: 12px; }
.asow-keep-phrase { white-space: nowrap; }

.asow-loading { display: flex; align-items: center; gap: 12px; padding: 40px; color: var(--muted); }
.asow-spinner { width: 20px; height: 20px; border-radius: 999px; border: 2px solid rgba(47, 154, 154, .15); border-top-color: var(--teal); animation: spin .8s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }
.asow-empty { display: flex; flex-direction: column; align-items: center; gap: 10px; padding: 56px 24px; text-align: center; border-radius: 18px; background: var(--bg); box-shadow: var(--inset-sm); }
.asow-empty-icon { font-size: 13px; font-weight: 900; color: var(--teal); }
.asow-empty-title { margin: 0; font-size: 16px; font-weight: 800; color: var(--text); }
.asow-empty-sub { margin: 0; font-size: 13px; color: var(--muted); max-width: 32rem; line-height: 1.65; }

.asow-pool-list { display: flex; flex-direction: column; gap: 12px; }
.asow-pool-card {
  position: relative;
  padding: 18px;
  border-radius: 18px;
  background: var(--bg);
  box-shadow: var(--raise-sm);
  display: flex;
  flex-direction: column;
  gap: 14px;
  overflow: hidden;
}
.asow-pool-card-skin {
  position: absolute;
  inset: 0;
  background-image: var(--asow-card-skin-url);
  background-size: cover;
  background-position: center;
  opacity: .30;
  filter: saturate(.94) blur(1px);
  transform: scale(1.02);
}
.asow-pool-card-overlay {
  position: absolute;
  inset: 0;
  pointer-events: none;
  background:
    linear-gradient(180deg, color-mix(in srgb, var(--bg) 28%, transparent) 0%, color-mix(in srgb, var(--bg) 64%, transparent) 100%),
    linear-gradient(130deg, color-mix(in srgb, var(--teal) 8%, transparent) 0%, transparent 46%, color-mix(in srgb, var(--blue) 8%, transparent) 100%);
  backdrop-filter: blur(2px);
}
.asow-pool-card-has-skin {
  border: 1px solid color-mix(in srgb, var(--teal) 16%, var(--module-line, var(--zc-line)));
}
.asow-pool-card-body {
  position: relative;
  z-index: 1;
  display: flex;
  flex-direction: column;
  gap: 14px;
}
.asow-pool-card-head { display: flex; align-items: center; gap: 12px; }
.asow-pool-avatar { width: 40px; height: 40px; border-radius: 12px; flex-shrink: 0; background: rgba(47, 154, 154, .12); color: var(--teal); display: grid; place-items: center; font-weight: 900; font-size: 15px; }
.asow-pool-meta { display: flex; flex-direction: column; gap: 4px; min-width: 0; }
.asow-pool-meta b { font-size: 15px; font-weight: 800; color: var(--text); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.asow-pool-status-row { display: flex; gap: 6px; flex-wrap: wrap; }
.asow-status-chip, .asow-listed-chip, .asow-unlisted-chip, .asow-ready-chip, .asow-watch-chip { font-size: 10px; font-weight: 800; text-transform: uppercase; letter-spacing: .06em; border-radius: 999px; padding: 2px 8px; }
.asow-watch-chip { background: rgba(176, 125, 42, .14); color: var(--gold); }
.asow-s-healthy { background: rgba(47, 154, 154, .1); color: var(--teal); }
.asow-s-limited, .asow-s-maintenance { background: rgba(176, 125, 42, .12); color: var(--gold); }
.asow-s-offline { background: rgba(185, 64, 64, .1); color: var(--danger); }
.asow-listed-chip, .asow-ready-chip { background: rgba(47, 154, 154, .1); color: var(--teal); }
.asow-unlisted-chip { background: rgba(107, 98, 90, .1); color: var(--muted); }
.asow-pool-card-stats { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 12px; }
.asow-mini-stat { display: flex; flex-direction: column; gap: 2px; }
.asow-mini-stat span { font-size: 10px; color: var(--muted); font-weight: 700; }
.asow-mini-stat b { font-size: 16px; font-weight: 900; color: var(--text); }
.clr-good { color: var(--teal) !important; }
.clr-warn { color: var(--gold) !important; }
.clr-bad  { color: var(--danger) !important; }
.clr-pending { color: var(--muted) !important; }
.asow-pool-card-actions { display: flex; gap: 8px; align-items: center; flex-wrap: wrap; }
.asow-pool-next-step { display: flex; gap: 8px; align-items: flex-start; padding: 10px 12px; border-radius: 12px; background: color-mix(in srgb, var(--blue) 7%, transparent); }
.asow-pool-next-step b { flex: 0 0 auto; color: var(--blue); font-size: 11px; }
.asow-pool-next-step span { color: var(--muted); font-size: 11px; line-height: 1.5; }

.asow-btn { display: inline-flex; align-items: center; justify-content: center; min-height: 36px; padding: 0 16px; border: none; border-radius: 11px; font-size: 13px; font-weight: 800; cursor: pointer; text-decoration: none; background: var(--bg); box-shadow: var(--raise-sm); color: var(--text); white-space: nowrap; }
.asow-btn:disabled { opacity: .5; cursor: not-allowed; }
.asow-btn-primary { background: var(--teal); color: var(--zc-accent-ink); box-shadow: 0 10px 24px color-mix(in srgb, var(--teal) 24%, transparent), inset 0 1px 0 rgba(255, 255, 255, .2); }
.asow-btn-sm { min-height: 30px; padding: 0 12px; font-size: 12px; }
.asow-btn-skin-active {
  color: var(--teal);
  box-shadow: var(--inset-sm);
}
.asow-skin-panel {
  margin-top: 2px;
  padding: 16px;
  border-radius: 16px;
  background: color-mix(in srgb, var(--bg) 86%, transparent);
  box-shadow: var(--inset-sm);
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.asow-skin-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}
.asow-skin-head h3 {
  margin: 0 0 4px;
  font-size: 15px;
  font-weight: 900;
  color: var(--text);
}
.asow-skin-head p {
  margin: 0;
  color: var(--muted);
  font-size: 12px;
  line-height: 1.6;
}
.asow-skin-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(112px, 132px));
  justify-content: flex-start;
  gap: 10px;
}
.asow-skin-card {
  border: none;
  border-radius: 16px;
  padding: 8px;
  background: var(--bg);
  box-shadow: var(--raise-sm);
  display: flex;
  flex-direction: column;
  gap: 8px;
  text-align: left;
  cursor: pointer;
  color: var(--text);
}
.asow-skin-card.is-selected {
  box-shadow: var(--inset-sm);
}
.asow-skin-card-img {
  width: 100%;
  aspect-ratio: 2 / 3;
  object-fit: cover;
  border-radius: 12px;
}
.asow-skin-card-img-compact {
  aspect-ratio: 0.72;
}
.asow-skin-card-meta {
  display: flex;
  justify-content: space-between;
  gap: 10px;
  align-items: center;
}
.asow-skin-card-meta b {
  font-size: 11px;
  font-weight: 900;
  color: var(--text);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.asow-skin-card-meta small {
  font-size: 10px;
  font-weight: 800;
  color: var(--muted);
  text-transform: uppercase;
}
.asow-loading-inline {
  padding: 16px 0;
}
.asow-empty-inline {
  padding: 24px 16px;
}

@media (max-width: 920px) {
  .asow-overview { grid-template-columns: repeat(3, minmax(0, 1fr)); }
}
@media (max-width: 720px) {
  .asow-header { flex-direction: column; }
  .asow-header-actions { width: 100%; justify-content: flex-start; }
  .asow-flow-strip, .asow-form-grid, .asow-pool-card-stats, .asow-model-grid, .asow-rule-panel, .asow-overview { grid-template-columns: 1fr; }
}

.asow-provider-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(160px, 1fr));
  gap: 10px;
}
.asow-provider-card {
  text-align: left;
  border-radius: 14px;
  border: 1px solid rgba(20,34,56,.12);
  background: #f7f9fc;
  padding: 12px;
  min-height: 92px;
  box-shadow: 4px 4px 10px rgba(163,177,198,.25), -4px -4px 10px rgba(255,255,255,.8);
}
.asow-provider-card b { display:block; color:#0c1524; margin-bottom:6px; font-size:13px; }
.asow-provider-card small { color:#667286; line-height:1.4; display:block; font-size:11px; }
.asow-provider-card-on {
  border-color: rgba(43,143,138,.45);
  box-shadow: inset 2px 2px 5px rgba(163,177,198,.35), inset -2px -2px 5px rgba(255,255,255,.9);
}
.asow-provider-card.limited { opacity: .88; }
.asow-help { color:#667286; font-size:12px; margin-top:8px; display:block; }
.asow-field-full { grid-column: 1 / -1; }
.asow-mode-mutex-note {
  margin: 0 0 12px;
  padding: 10px 12px;
  border-radius: 12px;
  background: color-mix(in srgb, var(--blue, #2f6fed) 8%, transparent);
  color: var(--muted);
  font-size: 12px;
  line-height: 1.5;
}
.asow-btn-danger {
  color: #b42318;
  border: 1px solid color-mix(in srgb, #b42318 25%, transparent);
}
.asow-btn-danger:disabled {
  opacity: .55;
}
.owner-experience :is(.asow-flow-strip, .asow-create-panel, .asow-empty) {
  border-radius: 0;
  box-shadow: none;
  border-block: 1px solid var(--bd-ui-line);
}
.owner-experience :is(.asow-overview article, .asow-flow-strip article,
  .asow-mode-option, .asow-input, .asow-model-fetch-card, .asow-model-card,
  .asow-rule-item, .asow-pool-card, .asow-pool-next-step, .asow-btn) {
  border-radius: 6px;
}
.owner-experience :is(.asow-eyebrow, .asow-flow-strip span, .asow-status-mini,
  .asow-rule-item b, .asow-status-chip, .asow-listed-chip, .asow-unlisted-chip,
  .asow-ready-chip, .asow-watch-chip) {
  letter-spacing: 0;
  font-weight: 600;
}
.owner-experience .asow-btn {
  font-weight: 600;
  white-space: normal;
  overflow-wrap: anywhere;
  gap: 6px;
}
.owner-experience .asow-btn-primary {
  box-shadow: none;
}
.owner-experience :is(button, input, select, a):focus-visible {
  outline: 2px solid var(--bd-accent-teal);
  outline-offset: 3px;
}
.owner-experience .asow-mode-option-on,
.owner-experience .asow-model-card-on {
  box-shadow: inset 0 0 0 2px var(--bd-accent-teal);
  background: color-mix(in srgb, var(--bd-accent-teal) 6%, var(--bg));
}
@media (prefers-reduced-motion: reduce) {
  .owner-experience *, .owner-experience *::before, .owner-experience *::after {
    animation-duration: 0.01ms;
    transition-duration: 0.01ms;
  }
}
</style>
