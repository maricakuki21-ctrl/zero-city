<template>
  <AppLayout>
    <div class="aopd-page owner-experience">
      <div class="aopd-navigation">
        <RouterLink to="/account-square" class="aopd-back">共享市场</RouterLink>
        <RouterLink to="/account-square/my" class="aopd-back">我的资源</RouterLink>
        <RouterLink to="/account-square/owner" class="aopd-back aopd-back-current">池主中心</RouterLink>
      </div>

      <div v-if="loading" class="aopd-loading">
        <div class="aopd-spinner"></div>
        <span>加载中…</span>
      </div>
      <div v-else-if="!pool" class="aopd-empty">找不到该共享池，或当前账号不是池主。</div>

      <template v-else>
        <header
          class="aopd-header"
          :class="{
            'aopd-header-has-skin': Boolean(pool.card_skin_key && pool.card_skin_rarity),
            'aopd-header-default-skin': !pool.card_skin_key || !pool.card_skin_rarity,
          }"
          :style="headerCardStyle"
        >
          <PoolCardBackdrop :card-key="pool.card_skin_key" :rarity="pool.card_skin_rarity" />
          <div class="aopd-header-overlay" aria-hidden="true"></div>
          <div class="aopd-pool-identity">
            <div class="aopd-avatar">{{ pool.name.slice(0, 1).toUpperCase() }}</div>
            <div>
              <h1 class="aopd-title">{{ pool.name }}</h1>
              <div class="aopd-status-row">
                <span v-if="!isNativePool" class="aopd-status-chip" :class="`aopd-s-${pool.status}`">{{ poolStatusLabel(pool) }}</span>
                <span v-if="isObservationPool(pool)" class="aopd-watch-chip">观察中</span>
                <span :class="pool.listed ? 'aopd-listed-chip' : 'aopd-unlisted-chip'">{{ pool.owner_paused ? '已停用' : pool.listed ? '已上架' : '未上架' }}</span>
                <span v-if="!isNativePool" :class="verificationChipClass(pool)">{{ verificationBadgeLabel(pool) }}</span>
                <span v-if="fullCheckReady(pool)" class="aopd-passed-chip">{{ fullCheckBannerLabel(pool) }}</span>
              </div>
            </div>
          </div>
          <div class="aopd-header-actions">
            <button class="aopd-btn aopd-btn-sm" type="button" @click="activeTab = 'members'">管理成员（{{ pool.current_users || 0 }}）</button>
            <button class="aopd-btn aopd-btn-sm" type="button" :disabled="loading || publicationPending" @click="refreshAll">刷新</button>
            <button class="aopd-btn aopd-btn-sm" type="button" :disabled="pausingPool || publicationLocked" @click="toggleOwnerPause">{{ pausingPool ? '处理中…' : pool.owner_paused ? '恢复为待发布' : '停用此池' }}</button>
            <button
              class="aopd-btn aopd-btn-sm aopd-btn-danger"
              type="button"
              :disabled="deletingPool || publicationLocked"
              @click="removePool"
            >
              {{ deletingPool ? '删除中…' : '删除此池' }}
            </button>
          </div>
        </header>
        <p v-if="pool.owner_paused" class="aopd-message" role="status">已停用：不在市场展示，也不接受新调用。资源、成员和账单保留；恢复后需要重新检查并发布，检测成功不会自动替你启用。</p>

        <nav v-if="isNativePool" class="native-pool-tabs" aria-label="池主经营">
          <button type="button" :aria-current="nativeSetupVisible ? 'page' : undefined" @click="activeTab = 'accounts'">资源与发布</button>
          <button type="button" :aria-current="activeTab === 'members' ? 'page' : undefined" @click="activeTab = 'members'">成员管理</button>
          <button type="button" :aria-current="activeTab === 'revenue' ? 'page' : undefined" @click="activeTab = 'revenue'">收益与账单</button>
        </nav>
        <section v-if="isNativePool && nativeSetupVisible" class="aopd-section aopd-native-onboarding">
          <p v-if="nativeAccountError && !showNativeAccountForm" class="aopd-message aopd-msg-error" role="alert">{{ nativeAccountError }}</p>
          <div v-if="accountsError" class="aopd-message aopd-msg-error" role="alert" data-testid="native-accounts-load-error">
            {{ accountsError }}
            <button class="aopd-btn aopd-btn-xs" type="button" :disabled="accountsLoading" @click="loadAccounts">{{ accountsLoading ? '读取中…' : '重试读取资源' }}</button>
          </div>
          <PoolSetupWorkspace
            v-if="!accountsError"
            :pool="pool"
            :accounts="accounts"
            :readiness-pending-account-id="nativeReadinessPendingAccountId"
            :connecting="showNativeAccountForm"
            :pricing="modelPricing"
            :pricing-loading="pricingLoading"
            :pricing-error="pricingStatus === 'error' ? pricingMessage : ''"
            :pricing-message="pricingMessage"
            :pricing-status="pricingStatus"
            :pricing-drafts="pricingDrafts"
            :pricing-saving-key="savingPricingKey"
            :can-publish="canPublishNativePool && !nativeSetupBusy"
            :publishing="publicationPending"
            :unresolved-publication="publicationUnresolved"
            :write-locked="publicationLocked"
            :publication-message="publicationMessage"
            :publication-state="publicationState"
            @retry="retryNativeAccount"
            @readiness="verifyNativeAccountReadiness"
            @connect="openNativeConnection"
            @publish="publishNativePool"
            @refresh-pricing="loadModelPricing"
            @save-pricing="saveCustomPricing"
            @update-pricing-draft="updatePricingDraft"
          ><template #connection>
          <div class="aopd-section-head">
            <div>
              <h2>{{ nativeRepairTarget ? '修复资源连接' : '连接你的模型资源' }}</h2>
            </div>
            <button class="aopd-btn" type="button" :disabled="addingNativeAccount" @click="toggleNativeAccountForm">
              {{ showNativeAccountForm ? (nativeRepairTarget ? '取消修复' : '取消添加') : '添加账号' }}
            </button>
          </div>

          <div
            v-if="showNativeAccountForm"
            class="aopd-form-card"
            data-testid="native-account-form"
            :data-mode="nativeRepairTarget ? 'repair' : 'create'"
            :data-repair-account-id="nativeRepairTarget?.accountID"
            :data-native-operation-id="nativeRepairTarget?.operationID"
          >
            <div class="aopd-form-grid">
              <label class="aopd-field aopd-field-full"><span>接口类型</span><select v-model="nativeAccount.provider" class="aopd-input" data-testid="native-account-provider" :disabled="addingNativeAccount"><option value="openai">OpenAI / OpenAI 兼容接口</option><option value="anthropic">Anthropic</option><option value="gemini">Gemini</option><option v-if="!['openai', 'anthropic', 'gemini'].includes(nativeAccount.provider)" :value="nativeAccount.provider">{{ nativeAccount.provider }}</option></select></label>
              <label class="aopd-field aopd-field-full"><span>接口地址 <b>*</b></span><input v-model="nativeAccount.upstream_base_url" class="aopd-input" data-testid="native-account-url" placeholder="https://api.example.com/v1" :disabled="addingNativeAccount" /></label>
              <label class="aopd-field aopd-field-full">
                <span>{{ nativeAccount.auth_type === 'oauth' ? 'OAuth 凭证 JSON' : 'API Key' }} <b>*</b></span>
                <textarea v-if="nativeAccount.auth_type === 'oauth'" v-model="nativeAccount.secret" class="aopd-textarea" data-testid="native-account-secret" rows="4" placeholder="{&quot;access_token&quot;:&quot;...&quot;}" :disabled="addingNativeAccount" />
                <input v-else v-model="nativeAccount.secret" class="aopd-input" data-testid="native-account-secret" type="password" autocomplete="off" placeholder="粘贴资源的 API Key" :disabled="addingNativeAccount" />
              </label>
            </div>
            <details class="owner-advanced"><summary>账号备注与认证方式</summary><div class="aopd-form-grid">
              <label class="aopd-field"><span>账号名称</span><input v-model="nativeAccount.name" class="aopd-input" data-testid="native-account-name" placeholder="备注名" :disabled="addingNativeAccount" /></label>
              <label class="aopd-field"><span>认证方式</span><select v-model="nativeAccount.auth_type" class="aopd-input" data-testid="native-account-auth-type" :disabled="addingNativeAccount" @change="nativeAccount.secret = ''"><option value="api_key">API Key</option><option value="oauth">OAuth 凭证</option></select></label>
            </div></details>
            <p v-if="nativeAccountError" class="aopd-message aopd-msg-error" role="alert">{{ nativeAccountError }}</p>
            <div class="aopd-form-actions">
              <button class="aopd-btn aopd-btn-primary" type="button" data-testid="native-account-submit" :disabled="!canSubmitNativeAccount || addingNativeAccount" @click="submitNativeAccount">
                {{ addingNativeAccount ? '连接中…' : (nativeRepairTarget ? '保存修复' : '连接并拉取模型') }}
              </button>
            </div>
            <p class="owner-form-note">Key 加密保存，不会出现在公开页面。</p>
          </div>
          </template>
          <template #appearance>
            <div class="native-appearance">
              <div class="aopd-section-actions">
                <button class="aopd-btn" type="button" :disabled="settingSkin || clearingSkin" @click="toggleSkinPicker">{{ pool.card_skin_key ? '更换收藏卡背景' : '设置收藏卡背景' }}</button>
                <button v-if="pool.card_skin_key" class="aopd-btn" type="button" :disabled="settingSkin || clearingSkin" @click="clearSkin">{{ clearingSkin ? '清除中…' : '使用公共背景' }}</button>
                <RouterLink to="/zero-city/cards" class="aopd-btn">我的收藏卡</RouterLink>
              </div>
              <div v-if="showSkinPicker" class="native-skin-picker" aria-label="原生共享池收藏卡背景">
                <p v-if="skinLoading" role="status">加载收藏卡…</p>
                <p v-else-if="!skinCards.length">暂无可选收藏卡，可以继续使用公共背景。</p>
                <div v-else class="native-skin-grid">
                  <button v-for="card in skinCards" :key="`${card.card_key}-${card.serial_no || 0}`" type="button" :disabled="settingSkin || clearingSkin" :aria-pressed="pool.card_skin_key === card.card_key" @click="applySkin(card.card_key, card.rarity)">
                    <img :src="collectibleCardImage(card.card_key, card.rarity)" :alt="zeroCityCardDisplayName(card.card_key)" loading="lazy" />
                    <span>{{ zeroCityCardDisplayName(card.card_key) }}</span>
                  </button>
                </div>
              </div>
            </div>
          </template>
          </PoolSetupWorkspace>
        </section>

        <template v-if="!isNativePool">
        <section class="aopd-readiness" :class="`aopd-readiness-${ownerReadiness.tone}`">
          <div class="aopd-readiness-copy">
            <span class="aopd-readiness-kicker">上架准备度</span>
            <h2>{{ ownerReadiness.title }}</h2>
            <p>{{ ownerReadiness.description }}</p>
          </div>
          <div class="aopd-readiness-progress" aria-label="上架准备进度">
            <div class="aopd-readiness-bar"><span :style="{ width: `${ownerReadiness.progress}%` }"></span></div>
            <b>{{ ownerReadiness.progress }}%</b>
          </div>
          <div class="aopd-readiness-steps">
            <button
              v-for="step in ownerReadiness.steps"
              :key="step.key"
              type="button"
              class="aopd-readiness-step"
              :class="{ 'aopd-readiness-step-done': step.done }"
              @click="activeTab = step.tab"
            >
              <span>{{ step.done ? '✓' : step.index }}</span>
              <div><b>{{ step.label }}</b><small>{{ step.note }}</small></div>
            </button>
          </div>
        </section>

        <section class="aopd-stats-row">
          <div class="aopd-stat"><b>{{ pool.current_users || 0 }}/{{ pool.max_users || 0 }}</b><span>当前成员</span></div>
          <div class="aopd-stat"><b :class="probeToneClass(pool)">{{ formatProbeScore(pool) }}</b><span>{{ probeStateLabel(pool) }}</span></div>
          <div class="aopd-stat"><b>{{ availabilityText(pool) }}</b><span>今日可用率</span></div>
          <div class="aopd-stat"><b>{{ pool.total_calls || 0 }}</b><span>总调用</span></div>
          <div class="aopd-stat"><b>{{ pool.account_summary?.schedulable_accounts || (pool.account_mode_enabled ? 0 : 1) }}</b><span>可调度账号</span></div>
          <div class="aopd-stat"><b>{{ formatPlatformFee(pool.platform_fee_percent) }}</b><span>平台附加费（用户承担）</span></div>
        </section>

        <nav class="aopd-tabs" aria-label="池主管理分区">
          <button
            v-for="tab in tabs"
            :key="tab.key"
            class="aopd-tab"
            :class="{ 'aopd-tab-active': activeTab === tab.key }"
            type="button"
            @click="activeTab = tab.key"
          >
            {{ tab.label }}
          </button>
        </nav>

        <section v-if="activeTab === 'accounts'" class="aopd-section">
          <template v-if="!pool.account_mode_enabled">
            <div class="aopd-section-head">
              <div>
                <h2>池级凭证</h2>
                <p>当前共享池使用一套池级上游凭证，由系统直接承接模型拉取、健康检测和调用调度。</p>
              </div>
              <button class="aopd-btn" type="button" @click="activeTab = 'settings'">管理池级凭证</button>
            </div>
            <div class="aopd-routing-note">
              <div><b>{{ pool.has_upstream_key ? '凭证已保存' : '等待保存凭证' }}</b><span>{{ pool.has_upstream_key ? 'API Key 已加密保存，前端不会回显明文。' : '请在池子设置中保存 API Key，之后才能拉取模型和执行检测。' }}</span></div>
              <div><b>服务地址</b><span>{{ pool.upstream_base_url || '尚未配置 Base URL' }}</span></div>
              <div><b>开放模型</b><span>{{ pool.models?.length || 0 }} 个 · {{ pool.models?.join(', ') || '尚未选择模型' }}</span></div>
              <div><b>调度容量</b><span>账号并发 {{ pool.account_concurrency || 1 }} · 单用户并发 {{ pool.user_concurrency || 1 }}</span></div>
            </div>
            <div class="aopd-message aopd-msg-neutral">
              单凭证模式不需要添加账号。若后续需要多套上游账号、账号级检测和故障切换，可在池子设置中切换为多账号模式。
            </div>
          </template>

          <template v-else>
            <div class="aopd-section-head">
              <div>
                <h2>API 账号</h2>
                <p>上游 Key 只在创建时提交；列表仅展示脱敏状态和账号能力。</p>
              </div>
              <div class="aopd-section-actions">
                <button class="aopd-btn aopd-btn-primary" type="button" @click="showAddAccount = !showAddAccount">
                  {{ showAddAccount ? '取消添加' : '添加账号' }}
                </button>
                <button class="aopd-btn" type="button" @click="showBatchImport = !showBatchImport">
                  {{ showBatchImport ? '收起导入' : '一键导入账号' }}
                </button>
                <button class="aopd-btn" type="button" :disabled="accounts.length === 0 || probingAccounts || probeTaskRunning" @click="probeAllAccounts">
                  {{ probingAccounts ? '检测中…' : '检测全部账号' }}
                </button>
              </div>
            </div>

            <div v-if="showAddAccount" class="aopd-form-card" @mousedown.stop @click.stop>
              <h3>添加账号</h3>
              <div class="aopd-form-grid">
                <label class="aopd-field"><span>账号名称</span><input v-model="newAccount.name" class="aopd-input" placeholder="备注名" /></label>
                <div class="aopd-field aopd-field-full">
                  <span>供应商</span>
                  <div class="aopd-provider-grid">
                    <button
                      v-for="preset in providerPresets"
                      :key="preset.id"
                      type="button"
                      class="aopd-provider-card"
                      :class="{ on: accountProviderId === preset.id }"
                      @click="selectAccountProvider(preset.id)"
                    >
                      <b>{{ preset.label }}</b>
                      <small>{{ preset.supportLevel === 'primary' ? '推荐' : (preset.supportLevel === 'limited' ? '有限支持' : '自定义') }}</small>
                    </button>
                  </div>
                  <input v-if="accountProviderId === 'custom'" v-model="newAccount.provider" class="aopd-input" style="margin-top:8px" placeholder="自定义供应商名" />
                </div>
                <label class="aopd-field"><span>Base URL <b>*</b></span><input v-model="newAccount.upstream_base_url" class="aopd-input" placeholder="https://api.openai.com/v1" /></label>
                <label class="aopd-field"><span>API Key <b>*</b></span><input v-model="newAccount.upstream_api_key" class="aopd-input" type="password" placeholder="sk-..." /></label>
                <label class="aopd-field"><span>模型 <b>*</b></span><input v-model="newAccount.models_text" class="aopd-input" placeholder="gpt-4o-mini" /><small>新增账号的模型并发默认为 0，继承池级并发，不增加额外模型锁。</small></label>
                <label class="aopd-field"><span>账号 RPM</span><input v-model.number="newAccount.rpm_limit" class="aopd-input" type="number" min="0" /></label>
              </div>
              <div class="aopd-tool-row">
                <button class="aopd-btn aopd-btn-sm" type="button" :disabled="!canProbeNewAccountModels || probingNewAccountModels" @click="probeNewAccountModels">
                  {{ probingNewAccountModels ? '识别中…' : '识别上游模型' }}
                </button>
                <span v-if="modelProbeMessage" class="aopd-inline-message" :class="modelProbeStatus === 'error' ? 'aopd-msg-error' : 'aopd-msg-success'">{{ modelProbeMessage }}</span>
              </div>
              <div class="aopd-form-actions">
                <button class="aopd-btn aopd-btn-primary" type="button" :disabled="!canAddAccount || addingAccount" @click="addAccount">
                  {{ addingAccount ? '添加中…' : '确认添加' }}
                </button>
              </div>
            </div>

            <div v-if="showBatchImport" class="aopd-form-card" @mousedown.stop @click.stop>
              <div class="aopd-card-head">
                <div>
                  <h3>一键导入账号</h3>
                  <p class="aopd-muted">先点选供应商模板，再粘贴多行 Key / JSON。系统会自动补全 Base URL 并规范 /v1。</p>
                  <p>支持 JSON 数组，或每行：名称,Base URL,API Key,模型。模型可用 | 或 ; 分隔。</p>
                </div>
              </div>
              <textarea
                v-model="accountImportText"
                class="aopd-textarea"
                rows="6"
                placeholder="支持多行：
sk-xxx
sk-yyy|https://sub.example.com/v1
名称,https://sub.example.com/v1,sk-xxx,gpt-4o-mini|gpt-4.1
或 JSON 数组"
              ></textarea>
              <div class="aopd-form-actions">
                <button class="aopd-btn aopd-btn-primary" type="button" :disabled="!accountImportText.trim() || importingAccounts" @click="submitAccountImport">
                  {{ importingAccounts ? '导入中…' : '开始导入' }}
                </button>
              </div>
              <div class="aopd-oauth-import">
                <div class="aopd-oauth-head">
                  <div>
                    <b>Sub2API OAuth 账号包</b>
                    <span>支持 sub2api-data JSON。凭证将加密保存，不会在页面中回显。</span>
                  </div>
                  <label class="aopd-btn aopd-btn-sm aopd-file-btn">
                    选择 JSON 文件
                    <input type="file" accept="application/json,.json" @change="handleOAuthPackageFile" />
                  </label>
                </div>
                <span v-if="oauthImportFileName" class="aopd-file-name">已选择：{{ oauthImportFileName }}</span>
                <textarea v-model="oauthImportText" class="aopd-textarea" rows="4" placeholder="也可直接粘贴 sub2api_data_package.json 内容"></textarea>
                <label class="aopd-check-row">
                  <input v-model="oauthUpdateExisting" type="checkbox" />
                  <span>匹配到已有账号时更新凭证</span>
                </label>
                <p class="aopd-oauth-warning">缺少 refresh_token 的账号只能使用到当前 access_token 到期；到期后会停止调度，请重新取得凭证并更新已有账号。</p>
                <div class="aopd-form-actions">
                  <button class="aopd-btn aopd-btn-primary" type="button" :disabled="!oauthImportText.trim() || importingOAuthAccounts" @click="submitOAuthPackageImport">
                    {{ importingOAuthAccounts ? '导入中…' : '导入 OAuth 账号包' }}
                  </button>
                </div>
                <div v-if="oauthImportResult" class="aopd-message" :class="oauthImportResult.failed > 0 ? 'aopd-msg-error' : 'aopd-msg-success'">
                  <span>处理 {{ oauthImportResult.total }} 个：新建 {{ oauthImportResult.created }}，更新 {{ oauthImportResult.updated }}，跳过 {{ oauthImportResult.skipped }}，失败 {{ oauthImportResult.failed }}</span>
                  <ul v-if="oauthImportResult.items.length" class="aopd-error-list">
                    <li v-for="item in oauthImportResult.items.slice(0, 12)" :key="`${item.index}-${item.action}`">{{ item.name || `第 ${item.index + 1} 项` }}：{{ item.action }}<template v-if="item.message"> · {{ item.message }}</template></li>
                  </ul>
                </div>
              </div>
              <div v-if="accountImportError" class="aopd-message aopd-msg-error">{{ accountImportError }}</div>
              <div v-else-if="accountImportResult" class="aopd-message" :class="accountImportResult.failed > 0 ? 'aopd-msg-error' : 'aopd-msg-success'">
                <span>导入完成：成功 {{ accountImportResult.created }} / {{ accountImportResult.total }}，失败 {{ accountImportResult.failed }}</span>
                <ul v-if="failedImportItems.length" class="aopd-error-list">
                  <li v-for="item in failedImportItems" :key="item.index">第 {{ item.index }} 项：{{ item.error || '导入失败' }}</li>
                </ul>
              </div>
            </div>

            <div v-if="accountProbeMessage" class="aopd-message" :class="accountProbeStatus === 'error' ? 'aopd-msg-error' : 'aopd-msg-success'">
              {{ accountProbeMessage }}
            </div>
            <div v-if="probeTaskRunning && activeProbeJob && activeProbeScope === 'account'" class="aopd-message aopd-msg-neutral">
              {{ activeProbeJobSummary }}
            </div>

            <div v-if="accountsLoading" class="aopd-muted">加载账号…</div>
            <div v-else-if="accounts.length === 0" class="aopd-empty-sm">还没有上游账号。添加 API Key 账号或导入 OAuth 账号包，完成检测后才会进入共享池调度。</div>
            <div v-else class="aopd-account-list">
              <article v-for="account in accounts" :key="account.id" class="aopd-account-row">
                <div class="aopd-account-info">
                  <b>{{ account.name || account.provider || '未命名账号' }} <small class="aopd-auth-badge">{{ account.auth_type === 'oauth' ? 'OAuth' : 'API Key' }}</small></b>
                  <span class="aopd-account-meta">{{ account.auth_type === 'oauth' ? 'OpenAI Codex Responses' : (account.upstream_base_url || '默认 Base URL') }} · 成功率 {{ formatSuccessRate(account.successful_calls, account.total_calls) }}</span>
                  <span class="aopd-account-meta">模型 {{ account.model_configs?.map((model) => model.model_name).join(', ') || '未配置' }} · {{ accountRoutingLabel(account) }}<template v-if="account.expires_at"> · 到期 {{ new Date(account.expires_at).toLocaleString() }}</template></span>
                </div>
                <span class="aopd-acc-status" :class="account.status === 'active' ? 'aopd-acc-active' : 'aopd-acc-disabled'">{{ account.status }}</span>
                <button class="aopd-btn aopd-btn-xs" type="button" :disabled="probeTaskRunning || probingAccounts || probingAccountId === account.id" @click="probeAccount(account.id)">
                  {{ probingAccountId === account.id ? '检测中…' : '检测' }}
                </button>
                <button class="aopd-btn aopd-btn-xs aopd-btn-danger" type="button" :disabled="deletingAccountId === account.id" @click="deleteAccount(account.id)">
                  {{ deletingAccountId === account.id ? '删除中…' : '删除' }}
                </button>
              </article>
            </div>
          </template>
        </section>

        <section v-if="activeTab === 'models'" class="aopd-section">
          <div class="aopd-section-head">
            <div>
              <h2>模型与路由范围</h2>
              <p>模型拉取只确认上游返回的目录；是否可公开上架仍以满血检测证据为准。</p>
            </div>
            <div class="aopd-section-actions">
              <button class="aopd-btn" type="button" :disabled="syncingPoolModels || pool.account_mode_enabled" @click="pullSavedPoolModels">
                {{ syncingPoolModels ? '拉取中…' : '用已保存凭证拉取模型' }}
              </button>
              <button class="aopd-btn aopd-btn-primary" type="button" :disabled="savingPoolModels || selectedPoolModels.length === 0" @click="savePoolModelSelection">
                {{ savingPoolModels ? '保存中…' : '保存开放模型' }}
              </button>
            </div>
          </div>
          <div v-if="pool.account_mode_enabled" class="aopd-message aopd-msg-neutral">
            当前为多账号池。池级模型范围来自账号能力覆盖；请先在「账号」分区为各账号拉取模型并完成检测，再在此确认对外开放范围。
          </div>
          <div v-if="poolModelMessage" class="aopd-message" :class="poolModelStatus === 'error' ? 'aopd-msg-error' : 'aopd-msg-success'">{{ poolModelMessage }}</div>
          <div class="aopd-model-toolbar">
            <span>候选 {{ effectivePoolModelCandidates.length }} 个 · 已开放 {{ selectedPoolModels.length }} 个</span>
            <div>
              <button class="aopd-link-btn" type="button" @click="selectAllPoolModels">全选</button>
              <button class="aopd-link-btn" type="button" @click="clearPoolModels">清空</button>
              <button class="aopd-link-btn" type="button" @click="resetPoolModelSelection">恢复已保存</button>
            </div>
          </div>
          <div v-if="effectivePoolModelCandidates.length === 0" class="aopd-empty-sm">
            暂无模型候选。单凭证池可使用已保存凭证拉取；多账号池请先添加账号并识别模型。
          </div>
          <div v-else class="aopd-model-grid">
            <article v-for="modelName in effectivePoolModelCandidates" :key="modelName" :class="{ selected: selectedPoolModelSet.has(modelName) }" @click="togglePoolModel(modelName)">
              <div><span class="aopd-model-check">{{ selectedPoolModelSet.has(modelName) ? '✓' : '' }}</span><b>{{ modelName }}</b></div>
              <label v-if="selectedPoolModelSet.has(modelName)" @click.stop>
                <span>倍率</span>
                <input :value="poolModelRates[modelName] ?? pool.rate_multiplier ?? 1" type="number" min="0.0001" step="0.0001" @input="setPoolModelRate(modelName, $event)" />
              </label>
              <label v-if="selectedPoolModelSet.has(modelName)" @click.stop>
                <span>模型并发（0 = 继承池级）</span>
                <input :value="poolModelConcurrency[modelName] ?? 0" type="number" min="0" step="1" @input="setPoolModelConcurrency(modelName, $event)" />
              </label>
            </article>
          </div>
          <div class="aopd-pricing-head">
            <div>
              <h3>服务能力与模型价格</h3>
              <p>文字、图片、视频分别检查。只有“已启用、满血检测通过、价格完整”三项同时满足，用户才真的能调用。</p>
            </div>
            <button class="aopd-btn aopd-btn-xs" type="button" :disabled="pricingLoading" @click="loadModelPricing">
              {{ pricingLoading ? '刷新中…' : '刷新价格' }}
            </button>
          </div>
          <div v-if="!pricingLoading || modelPricing.length > 0" class="aopd-capability-grid">
            <article v-for="capability in endpointCapabilityRows" :key="capability.endpoint" :class="`is-${capability.tone}`">
              <div>
                <b>{{ capability.label }}</b>
                <span>{{ capability.description }}</span>
              </div>
              <strong>{{ capability.status }}</strong>
              <small>{{ capability.detail }}</small>
            </article>
          </div>
          <div class="aopd-capability-rule">
            “价格已设置”不等于“已经可用”。系统会逐个服务检查开关、检测证据和价格，任何一项不完整都会拒绝调用，不会偷偷按 0 元放行。平台官方图片价以 1K、视频价以 480p 作为页面浏览基准；实际规格会在调用时冻结当次价格。
          </div>
          <div v-if="pricingMessage" class="aopd-message" :class="pricingStatus === 'error' ? 'aopd-msg-error' : 'aopd-msg-success'">{{ pricingMessage }}</div>
          <div v-if="pricingLoading && modelPricing.length === 0" class="aopd-empty-sm">正在读取模型价格…</div>
          <div v-else-if="modelPricing.length === 0" class="aopd-empty-sm">还没有可配置的服务。先保存开放模型；图片或视频端点只有后端真正返回后才会出现在这里。</div>
          <div v-else class="aopd-pricing-list">
            <article v-for="item in modelPricing" :key="pricingKey(item)" class="aopd-pricing-item">
              <div class="aopd-pricing-summary">
                <div>
                  <b>{{ item.display_name || item.model_name }}</b>
                  <span>{{ endpointLabel(item.endpoint_type) }}</span>
                </div>
                <div class="aopd-pricing-badges">
                  <span class="aopd-endpoint-status" :class="`is-${endpointAvailability(item).tone}`">{{ endpointAvailability(item).label }}</span>
                  <span class="aopd-pricing-source" :class="item.pricing_source === 'official_catalog' ? 'official' : 'custom'">
                    {{ item.pricing_source === 'official_catalog' ? '平台官方价' : '池主自定义价' }}
                  </span>
                </div>
              </div>
              <p class="aopd-endpoint-explanation">{{ endpointAvailability(item).description }}</p>
              <template v-if="item.current_price && endpointHasCompatiblePrice(item)">
                <div class="aopd-price-facts">
                  <div><small>基础单价</small><strong>{{ formatPriceComponents(item.current_price.base_price) }}</strong></div>
                  <div><small>加价倍率</small><strong>{{ formatMultiplier(item.current_price.multiplier) }}</strong></div>
                  <div><small>用户实际单价</small><strong>{{ formatPriceComponents(item.current_price.user_price) }}</strong></div>
                  <div><small>开始生效</small><strong>{{ formatPricingTime(item.current_price.effective_from) }}</strong></div>
                </div>
                <p class="aopd-price-example">
                  {{ pricingExampleLabel(item) }}，用户约支付 {{ formatMoney(item.current_price.example_cost) }}。
                </p>
              </template>
              <div v-else class="aopd-price-pending">价格还没补完整，系统会拒绝这个模型的调用，不会按 0 元放行。</div>

              <div v-if="item.pricing_source === 'owner_custom' && canEditPricing(item) && pricingDrafts[pricingKey(item)]" class="aopd-price-editor">
                <div class="aopd-price-editor-title">
                  <b>{{ item.current_price ? '发布新价格' : '补齐价格后启用' }}</b>
                  <span>每次保存都会生成一个新版本；过去的调用仍按当时价格结算。</span>
                </div>
                <div class="aopd-price-fields">
                  <label class="aopd-field">
                    <span>收费方式</span>
                    <select v-model="pricingDrafts[pricingKey(item)].billing_mode" class="aopd-input">
                      <option v-for="option in billingOptions(item.endpoint_type)" :key="option.value" :value="option.value">{{ option.label }}</option>
                    </select>
                  </label>
                  <template v-if="pricingDrafts[pricingKey(item)].billing_mode === 'token'">
                    <label class="aopd-field"><span>每百万输入 token（美元）</span><input v-model.number="pricingDrafts[pricingKey(item)].input_per_million" class="aopd-input" type="number" min="0" step="0.01" /></label>
                    <label class="aopd-field"><span>每百万输出 token（美元）</span><input v-model.number="pricingDrafts[pricingKey(item)].output_per_million" class="aopd-input" type="number" min="0" step="0.01" /></label>
                    <label class="aopd-field"><span>每百万缓存读取（可不填）</span><input v-model.number="pricingDrafts[pricingKey(item)].cache_read_per_million" class="aopd-input" type="number" min="0" step="0.01" /></label>
                    <label class="aopd-field"><span>每百万缓存写入（可不填）</span><input v-model.number="pricingDrafts[pricingKey(item)].cache_write_per_million" class="aopd-input" type="number" min="0" step="0.01" /></label>
                  </template>
                  <label v-else-if="pricingDrafts[pricingKey(item)].billing_mode === 'image'" class="aopd-field"><span>每张图片（美元）</span><input v-model.number="pricingDrafts[pricingKey(item)].image_item_price" class="aopd-input" type="number" min="0" step="0.001" /></label>
                  <label v-else-if="pricingDrafts[pricingKey(item)].billing_mode === 'video'" class="aopd-field"><span>每秒视频（美元）</span><input v-model.number="pricingDrafts[pricingKey(item)].video_second_price" class="aopd-input" type="number" min="0" step="0.001" /></label>
                  <label v-else class="aopd-field"><span>每次请求（美元）</span><input v-model.number="pricingDrafts[pricingKey(item)].per_request_price" class="aopd-input" type="number" min="0" step="0.001" /></label>
                  <label class="aopd-field"><span>用户价格倍率</span><input v-model.number="pricingDrafts[pricingKey(item)].multiplier" class="aopd-input" type="number" min="0.0001" step="0.0001" /></label>
                  <label class="aopd-field"><span>最低收费（可选）</span><input v-model.number="pricingDrafts[pricingKey(item)].minimum_charge" class="aopd-input" type="number" min="0" step="0.001" /></label>
                  <label class="aopd-field"><span>最高收费（可选）</span><input v-model.number="pricingDrafts[pricingKey(item)].maximum_charge" class="aopd-input" type="number" min="0" step="0.001" /></label>
                </div>
                <div class="aopd-form-actions">
                  <button class="aopd-btn aopd-btn-primary" type="button" :disabled="savingPricingKey === pricingKey(item)" @click="saveCustomPricing(item)">
                    {{ savingPricingKey === pricingKey(item) ? '发布中…' : '发布新价格版本' }}
                  </button>
                </div>
              </div>
              <div v-else-if="item.pricing_source === 'owner_custom' && !canEditPricing(item)" class="aopd-price-pending">
                这个服务类型尚未接入安全计价，页面不会提供一个看似能保存、实际不能调用的按钮。
              </div>
            </article>
          </div>
          <div class="aopd-routing-note">
            <div><b>池级开放范围</b><span>只有这里保存为开放的模型才会进入市场和组合 Key 路由范围。</span></div>
            <div><b>账号级能力</b><span>多账号模式下，实际调度还必须命中具备该模型、检测通过且状态可调度的账号。</span></div>
            <div><b>失败兜底</b><span>只在已绑定、席位有效且价格规则一致的候选池中选择；费用只归属最终成功命中的池。</span></div>
          </div>
        </section>

        <section v-if="activeTab === 'detection'" class="aopd-section">
          <div class="aopd-section-head">
            <div>
              <h2>健康检测</h2>
              <p>上架/手动检测走满血检测；定时任务只做基础测活。多账号模式会自动命中一个可调度账号，不再回退池级空凭证。</p>
            </div>
            <label class="aopd-field aopd-probe-model-field">
              <span>检测模型（探针）</span>
              <select v-model="poolProbeModel" class="aopd-input">
                <option disabled value="">请选择检测模型</option>
                <option v-for="model in openPoolModels" :key="`probe-${model}`" :value="model">{{ model }}</option>
              </select>
              <small>必须指定模型；多账号模式会自动选择一个具备该模型、可调度且凭证有效的账号执行满血检测。</small>
            </label>
            <button
              class="aopd-btn aopd-btn-primary"
              type="button"
              :disabled="probingAll || probeTaskRunning || !poolProbeModel || openPoolModels.length === 0"
              :title="!poolProbeModel ? '请先选择检测模型（探针）' : (probingAll ? '检测进行中' : (pool.account_mode_enabled ? '触发账号级满血检测' : '触发池级满血检测'))"
              @click="probeAll"
            >
              {{ probingAll ? '检测中…' : (pool.account_mode_enabled ? '触发账号级满血检测' : '触发池级满血检测') }}
            </button>
          </div>
          <div v-if="!poolProbeModel && openPoolModels.length === 0" class="aopd-message aopd-msg-error">
            暂无可用检测模型。请先在「模型」页开放至少一个模型，再回来触发满血检测。
          </div>
          <div v-else-if="!poolProbeModel" class="aopd-message aopd-msg-error">
            请先在上方选择检测模型（探针），否则满血检测不会启动。
          </div>
          <div v-if="probeAllMessage" class="aopd-message" :class="probeAllStatus === 'error' ? 'aopd-msg-error' : 'aopd-msg-success'">
            {{ probeAllMessage }}
          </div>
          <div v-if="probeTaskRunning && activeProbeJob && activeProbeScope === 'pool'" class="aopd-message aopd-msg-neutral">
            {{ activeProbeJobSummary }}
          </div>
          <div v-if="isObservationPool(pool)" class="aopd-passed-banner aopd-passed-banner-watch">
            观察中 · 连续失败 {{ pool.consecutive_probe_failures || 0 }} 次
            <span class="aopd-hint">{{ pool.status_note || pool.governance_note || '请尽快修复上游后重新检测；连续失败达到 9 次将自动下架。' }}</span>
          </div>
          <div class="aopd-governance-strip">
            <article>
              <b>当前治理状态</b>
              <span>{{ isObservationPool(pool) ? '已进入观察期，暂不建议继续对外加入。' : '当前为正常经营态；若异常持续，先切观察再决定下架。' }}</span>
            </article>
            <article>
              <b>恢复公开</b>
              <span>{{ fullCheckReady(pool) ? (pool.account_mode_enabled ? '已有通过检测且当前可调度的账号；其余账号仍可继续复测。' : '满血检测达标后可恢复公开展示。') : '先修复上游，再补一轮手动满血检测。' }}</span>
            </article>
            <article>
              <b>下架策略</b>
              <span>建议先观察，再决定下架；池主可在经营总览先做观察/恢复/下架，治理端再查看日志与异常证据。</span>
            </article>
          </div>
          <div v-if="fullCheckReady(pool)" class="aopd-passed-banner">
            {{ fullCheckBannerLabel(pool) }} {{ fullCheckProgressText(pool) }} · {{ formatProbeScore(pool) }}
            <span v-if="fullCheckBannerHint(pool)" class="aopd-hint">{{ fullCheckBannerHint(pool) }}</span>
          </div>
          <div v-else-if="pool.verification_mode === 'professional_review'" class="aopd-passed-banner aopd-passed-banner-review">
            专业核验池：{{ pool.verification_exemption_reason || '已登记差异化模型专业核验说明' }}
            <span class="aopd-hint">请确保至少保留一个可调度账号，公开页会展示“专业核验”标识。</span>
          </div>
          <div v-if="probeHistories.length === 0" class="aopd-empty-sm">暂无检测记录。</div>
          <div v-else class="aopd-probe-list">
            <article v-for="history in probeHistories" :key="history.id" class="aopd-probe-row-wrap">
              <button class="aopd-probe-row" type="button" @click="toggleProbeDetail(history.id)">
                <span class="aopd-probe-type" :class="history.success ? 'aopd-probe-ok' : 'aopd-probe-fail'">{{ history.success ? '✓' : '✗' }}</span>
                <div class="aopd-probe-info">
                  <b>{{ probeTypeLabel(history.probe_type) }} · {{ history.model_name || '未指定模型' }}</b>
                  <span>{{ formatProbeHistoryScore(history) }} {{ history.latency_ms || 0 }}ms · {{ probeHistorySummary(history) }}</span>
                </div>
                <span class="aopd-probe-time">{{ formatDateTime(history.checked_at) }}</span>
                <span class="aopd-probe-expand">{{ expandedProbeId === history.id ? '收起' : '证据' }}</span>
              </button>
              <div v-if="expandedProbeId === history.id" class="aopd-probe-detail">
                <div v-if="!history.metadata?.checks?.length" class="aopd-muted">该记录没有逐项能力证据。</div>
                <div v-else class="aopd-check-grid">
                  <article v-for="check in history.metadata.checks" :key="check.id" :class="check.success ? 'aopd-check-ok' : 'aopd-check-fail'">
                    <div><b>{{ check.title }}</b><span>{{ check.category }} · {{ check.required ? '上架必需' : '扩展能力' }}</span></div>
                    <strong>{{ check.success ? '通过' : '未通过' }}</strong>
                    <small>{{ check.evidence || check.error_message || `HTTP ${check.http_status || '—'} · ${check.latency_ms || 0}ms` }}</small>
                  </article>
                </div>
              </div>
            </article>
          </div>
        </section>

        </template>
        <section v-if="activeTab === 'members'" class="aopd-section">
          <div class="aopd-section-head">
            <div>
              <h2>席位成员</h2>
              <span class="aopd-muted">{{ memberView === 'active' ? `${members.length} 个使用中席位` : `${members.length} 条历史席位` }}</span>
            </div>
            <div class="aopd-member-filter" aria-label="席位视图">
              <button type="button" :class="{ active: memberView === 'active' }" @click="setMemberView('active')">使用中</button>
              <button type="button" :class="{ active: memberView === 'released' }" @click="setMemberView('released')">退出历史</button>
            </div>
          </div>
          <div v-if="membersLoading" class="aopd-muted">加载成员…</div>
          <div v-else-if="membersError" class="aopd-message aopd-msg-error" role="alert">{{ membersError }} <button class="aopd-btn aopd-btn-xs" type="button" @click="loadMembers">重试</button></div>
          <div v-else-if="members.length === 0" class="aopd-empty-sm">
            {{ memberView === 'active' ? '当前没有用户占用席位。' : '暂无已退出的历史席位。' }}
          </div>
          <div v-else class="aopd-member-list">
            <article v-for="member in members" :key="member.id" class="aopd-member-row" :class="{ 'is-released': !isActiveMember(member) }">
              <div class="aopd-member-info">
                <b>用户 #{{ member.user_id }}</b>
                <span>{{ isActiveMember(member) ? '使用中' : '已退出' }} · 席位费 {{ formatSeatFee(member.hourly_seat_fee) }}</span>
                <small v-if="!isActiveMember(member)">{{ releaseReasonLabel(member.release_reason) }}</small>
              </div>
              <span class="aopd-member-time">
                <template v-if="isActiveMember(member)">加入 {{ formatDate(member.joined_at) }}</template>
                <template v-else>退出 {{ formatDate(member.released_at || member.last_activity_at || member.joined_at) }}</template>
              </span>
              <button v-if="isActiveMember(member)" class="aopd-btn aopd-btn-xs aopd-btn-danger" type="button" :disabled="removingSeatId !== null" @click="removeMember(member.id)">
                {{ removingSeatId === member.id ? '移除中…' : '移除' }}
              </button>
            </article>
          </div>
          <p v-if="memberActionMessage" class="aopd-message" role="status">{{ memberActionMessage }}</p>
        </section>

        <section v-if="activeTab === 'revenue'" class="aopd-section">
          <div class="aopd-settlement-grid">
            <article><span>池主原价收入</span><b>100%</b><small>新调用按池主原价入账；历史账单不变</small></article>
            <article><span>平台附加费</span><b>{{ formatPlatformFee(pool.platform_fee_percent) }}</b><small>加在调用原价之上，由用户承担，不扣池主收益</small></article>
            <article><span>当前席位费</span><b>{{ formatSeatFee(pool.hourly_seat_fee || 0) }}</b><small>已开始的结算窗口沿用当时规则</small></article>
            <article><span>下一规则生效</span><b>{{ pendingRuleEffectiveText(pool) }}</b><small>{{ pendingRuleSummary(pool) }}</small></article>
          </div>
          <RouterLink :to="{ path: '/wallet', query: { pool_id: poolId }, hash: '#shared-pool-earnings' }" class="aopd-btn">在钱包查看本池收益与账单</RouterLink>
        </section>

        <template v-if="!isNativePool">
        <section v-if="activeTab === 'settings'" class="aopd-section">
          <div class="aopd-section-head">
            <div>
              <h2>池子设置</h2>
              <p>这里承载池展示、背景设置和经营信息维护；观察/恢复/下架动作放在池主经营总览统一处理。API Key 留空则保持不变。</p>
            </div>
            <div class="aopd-section-actions">
              <button class="aopd-btn aopd-btn-sm" type="button" :class="pool.card_skin_key ? 'aopd-btn-skin-active' : ''" @click="toggleSkinPicker()">
                {{ pool.card_skin_key ? '更换收藏卡背景' : '设置收藏卡背景' }}
              </button>
              <button v-if="pool.card_skin_key" class="aopd-btn aopd-btn-sm" type="button" :disabled="clearingSkin" @click="clearSkin">
                {{ clearingSkin ? '清除中…' : '清除背景' }}
              </button>
              <button class="aopd-btn aopd-btn-sm" type="button" @click="initEditForm">重置</button>
            </div>
          </div>
          <div v-if="showSkinPicker" class="aopd-form-card aopd-skin-card-panel">
            <div class="aopd-card-head">
              <div>
                <h3>选择收藏卡背景</h3>
                <p class="aopd-muted">只保存你已经拥有的收藏卡引用；系统会自动叠加磨砂遮罩，避免文字被卡面吞掉。</p>
              </div>
              <button class="aopd-btn aopd-btn-sm" type="button" @click="showSkinPicker = false">收起</button>
            </div>
            <div v-if="skinLoading" class="aopd-muted">加载收藏卡…</div>
            <div v-else-if="skinCards.length === 0" class="aopd-empty-sm">当前使用共享池公共背景，不影响上架和调用。先去每日礼盒或卡册获得收藏卡，之后可在这里一键替换专属背景。</div>
            <div v-else class="aopd-skin-grid aopd-skin-grid-compact">
              <button
                v-for="card in skinCards"
                :key="`${card.card_key}-${card.serial_no || 0}`"
                class="aopd-skin-option"
                :class="{ 'aopd-skin-option-active': pool.card_skin_key === card.card_key }"
                :disabled="settingSkin"
                @click="applySkin(card.card_key, card.rarity)"
              >
                <img :src="collectibleCardImage(card.card_key, card.rarity)" :alt="zeroCityCardDisplayName(card.card_key)" class="aopd-skin-option-img" />
                <div class="aopd-skin-option-meta">
                  <b>{{ zeroCityCardDisplayName(card.card_key) }}</b>
                  <small>{{ zeroCityCardRarityDisplayName(card.rarity) }}</small>
                </div>
              </button>
            </div>
          </div>
          <div v-if="!editForm" class="aopd-empty-sm">加载中…</div>
          <div v-else class="aopd-form-card" @mousedown.stop @click.stop>
            <div class="aopd-form-grid">
              <label class="aopd-field"><span>池子名称</span><input v-model="editForm.name" class="aopd-input" /></label>
              <label class="aopd-field"><span>简介</span><input v-model="editForm.description" class="aopd-input" /></label>
              <label class="aopd-field"><span>服务 Base URL</span><input v-model="editForm.upstream_base_url" class="aopd-input" /></label>
              <label class="aopd-field"><span>API Key（留空保持不变）</span><input v-model="editForm.upstream_api_key" class="aopd-input" type="password" placeholder="留空保持不变" /></label>
              <label class="aopd-field"><span>凭证模式</span><select v-model="editForm.account_mode_enabled" class="aopd-input"><option :value="false">单凭证模式（只用池级 Key）</option><option :value="true">多账号模式（禁止回退池级 Key）</option></select></label>
              <label class="aopd-field"><span>验证方式</span><select v-model="editForm.verification_mode" class="aopd-input"><option value="full_check">满血验证</option><option value="professional_review">专业核验</option></select></label>
              <label class="aopd-field aopd-field-full"><span>专业核验说明</span><input v-model="editForm.verification_exemption_reason" class="aopd-input" :disabled="editForm.verification_mode !== 'professional_review'" placeholder="例如：差异化模型，已人工核验稳定性与输出质量" /></label>
              <label class="aopd-field">
                <span>池级调用倍率</span>
                <input v-model.number="editForm.rate_multiplier" class="aopd-input" type="number" min="0.0001" step="0.0001" />
                <small>最低 0.0001；保存后同步所有已启用模型，池主之后仍可随时修改。</small>
              </label>
              <label class="aopd-field"><span>最多席位</span><input v-model.number="editForm.max_users" class="aopd-input" type="number" min="1" /></label>
              <label class="aopd-field"><span>入场最低余额</span><input v-model.number="editForm.min_balance_admission" class="aopd-input" type="number" min="0" step="0.01" /></label>
              <label class="aopd-field"><span>每小时席位费</span><input v-model.number="editForm.hourly_seat_fee" class="aopd-input" type="number" min="0" step="0.001" /></label>
              <label class="aopd-field"><span>席位费抵扣门槛</span><input v-model.number="editForm.hourly_min_usage_waiver" class="aopd-input" type="number" min="0" step="0.001" /></label>
              <label class="aopd-field"><span>账号级并发</span><input v-model.number="editForm.account_concurrency" class="aopd-input" type="number" min="1" /></label>
              <label class="aopd-field"><span>单用户并发</span><input v-model.number="editForm.user_concurrency" class="aopd-input" type="number" min="1" /></label>
            </div>
            <p class="aopd-rule-note">切换凭证模式后，单凭证池使用池级 Base URL 与 Key；多账号池只调度账号列表中已配置且通过门槛的账号。主流模型必须走满血验证；差异化模型可切到专业核验，但必须填写说明。席位费与抵扣门槛从下一整点结算窗口生效；平台抽水由平台治理统一维护。</p>
            <p v-if="pool.pending_settlement_rule_effective_from" class="aopd-rule-note">
              待生效规则：{{ formatSeatFee(pool.pending_hourly_seat_fee || 0) }}，消费达到
              {{ formatSeatFee(pool.pending_hourly_min_usage_waiver || 0) }} 时全额抵扣；
              将于 {{ formatDate(pool.pending_settlement_rule_effective_from) }} 生效。
            </p>
            <div v-if="savePoolMessage" class="aopd-message" :class="savePoolStatus === 'error' ? 'aopd-msg-error' : 'aopd-msg-success'">{{ savePoolMessage }}</div>
            <div class="aopd-form-actions">
              <button class="aopd-btn aopd-btn-primary" type="button" :disabled="savingPool" @click="savePoolSettings">
                {{ savingPool ? '保存中…' : '保存设置' }}
              </button>
            </div>
          </div>
        </section>
        </template>
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import { useAppStore } from '@/stores/app'
import AppLayout from '@/components/layout/AppLayout.vue'
import PoolSetupWorkspace from '@/features/bizdecipher/components/shared-pool/PoolSetupWorkspace.vue'
import { useNativePoolPublication } from '@/features/bizdecipher/components/shared-pool/useNativePoolPublication'
import '@/features/bizdecipher/components/shared-pool/pool-owner-experience.css'
import {
  defaultSharedPoolBillingMode,
  formatSharedPoolMultiplier,
  parseSharedPoolMultiplier,
  sharedPoolBillingExampleLabel,
  sharedPoolBillingModeAllowed,
  sharedPoolBillingOptions,
  sharedPoolEndpointAvailability,
  sharedPoolEndpointDescription,
  sharedPoolEndpointGateSatisfied,
  sharedPoolEndpointHasCompatiblePrice,
  sharedPoolEndpointLabel,
  sharedPoolEndpointOrder,
  sharedPoolEndpointRequiresMediaGate,
} from '@/features/bizdecipher/components/shared-pool/sharedPoolPricing'
import {
  SHARED_POOL_PROVIDER_PRESETS,
  getSharedPoolProviderPreset,
  normalizeOpenAICompatibleBaseURL,
  type SharedPoolProviderPresetId
} from '@/features/shared-pool/providerPresets'
import {
  createNativeSharedPoolAccount,
  createSharedPoolAccount,
  createSharedPoolNativeOperationID,
  createSharedPoolProbeOperationID,
  deleteSharedPool,
  setSharedPoolOwnerPause,
  deleteSharedPoolAccount,
  fetchSharedPoolUpstreamModels,
  getSharedPoolProbeJob,
  importSharedPoolAccounts,
  importSharedPoolOAuthPackage,
  clearPoolCardSkin,
  isSharedPoolProbeJobTerminal,
  listMySharedPools,
  listSharedPoolModelPricing,
  listSharedPoolAccounts,
  listSharedPoolMembers,
  listSharedPoolProbeHistories,
  probeSharedPoolUpstream,
  removeSharedPoolMember,
  repairSharedPoolNativeAccount,
  saveSharedPoolCustomPrice,
  type PoolSeat,
  type SharedPool,
  type SharedPoolAccount,
  type SharedPoolAccountImportResult,
  type SharedPoolAccountPayload,
  type CreateNativeSharedPoolAccountPayload,
  type RepairNativeSharedPoolAccountPayload,
  type SharedPoolOAuthDataPackage,
  type SharedPoolOAuthImportResult,
  type SharedPoolModelConfig,
  type SharedPoolModelEndpointPricing,
  type SharedPoolBillingMode,
  type SharedPoolPriceComponents,
  updateSharedPool,
  verifySharedPoolNativeReadiness,
  type SharedPoolProbeHistory,
  type SharedPoolProbeJob,
  type SharedPoolUpstreamProbePayload,
  setPoolCardSkin,
  waitForSharedPoolProbeJob,
} from '@/features/bizdecipher/api/bizdecipher'
import { getCheckinCards } from '@/api/user'
import { zeroCityCardDisplayName, zeroCityCardRarityDisplayName } from '@/constants/zeroCityCardManifest'
import PoolCardBackdrop from '@/features/bizdecipher/components/shared-pool/PoolCardBackdrop.vue'
import type { CheckinCollectibleCard } from '@/types'
import {
  persistSharedPoolSettings,
  SharedPoolSettingsSaveError,
  type SharedPoolSettingsDraft,
} from './sharedPoolSettings'

type TabKey = 'accounts' | 'models' | 'detection' | 'members' | 'revenue' | 'settings'

type AccountForm = {
  name: string
  provider: string
  upstream_base_url: string
  upstream_api_key: string
  models_text: string
  rpm_limit: number
}

type NativeAccountForm = {
  name: string
  provider: string
  auth_type: 'api_key' | 'oauth'
  upstream_base_url: string
  secret: string
}

type MessageStatus = 'success' | 'error'

type PricingDraft = {
  billing_mode: SharedPoolBillingMode
  input_per_million: number | null
  output_per_million: number | null
  cache_read_per_million: number | null
  cache_write_per_million: number | null
  image_item_price: number | null
  video_second_price: number | null
  per_request_price: number | null
  multiplier: number
  minimum_charge: number | null
  maximum_charge: number | null
}

type SanitizedImportFailure = {
  index: number
  error: string
}

type ProbeTaskScope = 'pool' | 'account'

type PersistedProbeTask = {
  version: 1
  pool_id: number
  scope: ProbeTaskScope
  operation_id: string
  payload: SharedPoolUpstreamProbePayload
  job_id?: string
  created_at: number
}

const DEFAULT_PROBE_MODEL = 'gpt-4o-mini'
const PROBE_TASK_MAX_AGE_MS = 24 * 60 * 60 * 1000

const route = useRoute()
const router = useRouter()
const appStore = useAppStore()
const poolId = Number(route.params.id || 0)

const loading = ref(false)
const pool = ref<SharedPool | null>(null)
const activeTab = ref<TabKey>('accounts')
const tabs: Array<{ key: TabKey; label: string }> = [
  { key: 'accounts', label: '账号' },
  { key: 'models', label: '模型与路由' },
  { key: 'detection', label: '检测' },
  { key: 'members', label: '成员管理' },
  { key: 'revenue', label: '收益' },
  { key: 'settings', label: '设置' },
]
const requestedTab = String(route.query.tab || '')
if (tabs.some((tab) => tab.key === requestedTab)) activeTab.value = requestedTab as TabKey

const accounts = ref<SharedPoolAccount[]>([])
const isNativePool = computed(() => Boolean(pool.value?.native_onboarding_state && pool.value.native_onboarding_state !== 'legacy_existing'))
const nativeSetupVisible = computed(() => activeTab.value !== 'members' && activeTab.value !== 'revenue')
const {
  state: publicationState,
  message: publicationMessage,
  pending: publicationPending,
  unresolved: publicationUnresolved,
  canPublish: canPublishNativePool,
  publish: publishNativePool,
} = useNativePoolPublication(pool)
const publicationLocked = computed(() => publicationPending.value || publicationUnresolved.value)
const nativeSetupBusy = computed(() => addingNativeAccount.value || nativeReadinessPendingAccountId.value !== null || settingSkin.value || clearingSkin.value || Boolean(savingPricingKey.value))
const accountsLoading = ref(false)
const accountsError = ref('')
const showNativeAccountForm = ref(false)
const addingNativeAccount = ref(false)
const nativeAccountError = ref('')
const nativeAccountOperationID = ref('')
const nativeRepairOperationID = ref('')
const nativeReadinessPendingAccountId = ref<number | null>(null)
const nativeAccount = ref<NativeAccountForm>(emptyNativeAccountForm())
const showAddAccount = ref(false)
const showBatchImport = ref(false)
const addingAccount = ref(false)
const deletingAccountId = ref<number | null>(null)
const probingAccountId = ref<number | null>(null)
const probingAccounts = ref(false)
const probingNewAccountModels = ref(false)
const syncingPoolModels = ref(false)
const savingPoolModels = ref(false)
const upstreamPoolModels = ref<string[]>([])
const selectedPoolModels = ref<string[]>([])
const poolModelRates = ref<Record<string, number>>({})
const poolModelConcurrency = ref<Record<string, number>>({})
const poolModelMessage = ref('')
const poolModelStatus = ref<MessageStatus>('success')
const modelPricing = ref<SharedPoolModelEndpointPricing[]>([])
const pricingDrafts = ref<Record<string, PricingDraft>>({})
const pricingOperations = new Map<string, { payload: string; operationId: string }>()
const pricingLoading = ref(false)
const savingPricingKey = ref('')
const pricingMessage = ref('')
const pricingStatus = ref<MessageStatus>('success')
const endpointCapabilityRows = computed(() => sharedPoolEndpointOrder.map((endpoint) => {
  const entries = modelPricing.value.filter((item) => item.endpoint_type === endpoint)
  const ready = entries.filter((item) => endpointAvailability(item).callable).length
  if (entries.length === 0) {
    return {
      endpoint,
      label: endpointLabel(endpoint),
      description: sharedPoolEndpointDescription(endpoint),
      status: '尚未配置',
      detail: '当前没有模型提供这个服务，用户不能调用。',
      tone: 'muted',
    }
  }
  if (ready === entries.length) {
    return {
      endpoint,
      label: endpointLabel(endpoint),
      description: sharedPoolEndpointDescription(endpoint),
      status: '全部可调用',
      detail: `${ready}/${entries.length} 个模型已通过全部条件。`,
      tone: 'ready',
    }
  }
  if (ready > 0) {
    return {
      endpoint,
      label: endpointLabel(endpoint),
      description: sharedPoolEndpointDescription(endpoint),
      status: '部分可调用',
      detail: `${ready}/${entries.length} 个模型可用，其余请看下方原因。`,
      tone: 'warning',
    }
  }
  const firstState = endpointAvailability(entries[0])
  return {
    endpoint,
    label: endpointLabel(endpoint),
    description: sharedPoolEndpointDescription(endpoint),
    status: firstState.label,
    detail: `${entries.length} 个模型目前都不能调用，请按下方提示处理。`,
    tone: firstState.tone,
  }
}))
const newAccount = ref<AccountForm>(emptyAccountForm())
const providerPresets = SHARED_POOL_PROVIDER_PRESETS
const accountProviderId = ref<SharedPoolProviderPresetId>('openai_compatible')
const modelProbeMessage = ref('')
const modelProbeStatus = ref<MessageStatus>('success')
const accountImportText = ref('')
const importingAccounts = ref(false)
const accountImportResult = ref<SharedPoolAccountImportResult | null>(null)
const oauthImportText = ref('')
const oauthImportFileName = ref('')
const oauthUpdateExisting = ref(true)
const oauthImportResult = ref<SharedPoolOAuthImportResult | null>(null)
const importingOAuthAccounts = ref(false)
const accountImportError = ref('')
const accountProbeMessage = ref('')
const accountProbeStatus = ref<MessageStatus>('success')

const probeHistories = ref<SharedPoolProbeHistory[]>([])
const deletingPool = ref(false)
const pausingPool = ref(false)
const poolProbeModel = ref('')
const probingAll = ref(false)
const probeAllMessage = ref('')
const probeAllStatus = ref<MessageStatus>('success')
const expandedProbeId = ref<number | null>(null)
const activeProbeJob = ref<SharedPoolProbeJob | null>(null)
const activeProbeScope = ref<ProbeTaskScope | null>(null)
const probeTaskRunning = ref(false)
let probePollController: AbortController | null = null

const members = ref<PoolSeat[]>([])
const membersLoading = ref(false)
const membersError = ref('')
const memberActionMessage = ref('')
const removingSeatId = ref<number | null>(null)
const memberView = ref<'active' | 'released'>('active')
let memberLoadSequence = 0

const showSkinPicker = ref(false)
const skinLoading = ref(false)
const skinCards = ref<CheckinCollectibleCard[]>([])
const settingSkin = ref(false)
const clearingSkin = ref(false)

const editForm = ref<SharedPoolSettingsDraft | null>(null)
const savingPool = ref(false)
const savePoolMessage = ref('')
const savePoolStatus = ref<MessageStatus>('success')

const accountModels = computed(() => parseModels(newAccount.value.models_text))
const effectivePoolModelCandidates = computed(() => uniqueModels([
  ...upstreamPoolModels.value,
  ...(pool.value?.models || []),
  ...((pool.value?.model_configs || []).map((item) => item.model_name)),
  ...accounts.value.flatMap((account) => (account.model_configs || []).map((item) => item.model_name)),
]))
const selectedPoolModelSet = computed(() => new Set(selectedPoolModels.value))
const canAddAccount = computed(() => Boolean(
  newAccount.value.upstream_base_url.trim() &&
  newAccount.value.upstream_api_key.trim() &&
  accountModels.value.length > 0
))
const canSubmitNativeAccount = computed(() => Boolean(
  nativeAccount.value.name.trim()
  && nativeAccount.value.provider.trim()
  && nativeAccount.value.upstream_base_url.trim()
  && nativeAccount.value.secret.trim(),
))
const canProbeNewAccountModels = computed(() => Boolean(
  newAccount.value.upstream_base_url.trim() &&
  newAccount.value.upstream_api_key.trim()
))
const failedImportItems = computed<SanitizedImportFailure[]>(() => {
  return (accountImportResult.value?.items || [])
    .filter((item) => !item.created)
    .slice(0, 6)
    .map((item) => ({
      index: item.index + 1,
      error: sanitizeSecretMessage(item.error || '导入失败'),
    }))
})
const activeProbeJobSummary = computed(() => {
  const job = activeProbeJob.value
  if (!job) return ''
  const stage = job.status === 'queued'
    ? '后台排队中'
    : job.status === 'running'
      ? '后台检测中'
      : '正在读取结果'
  const attempt = job.max_attempts > 1
    ? ` · 尝试 ${Math.max(1, job.attempt)}/${job.max_attempts}`
    : ''
  return `${stage} · ${job.model_name || '探针模型'}${attempt} · 任务 ${job.id.slice(0, 8)}。可以离开本页，回来后会自动继续显示。`
})

const ownerReadiness = computed(() => {
  const target = pool.value
  if (!target) {
    return { title: '等待加载', description: '正在读取池状态。', progress: 0, tone: 'pending', steps: [] as Array<{ key: string; index: string; label: string; note: string; done: boolean; tab: TabKey }> }
  }
  const accountDone = target.account_mode_enabled
    ? (target.account_summary?.configured_accounts || accounts.value.filter((item) => item.has_upstream_key || item.has_oauth_credentials).length) > 0
    : Boolean(target.has_upstream_key || target.upstream_base_url)
  const modelDone = (target.model_configs || []).some((item) => item.model_open !== false && item.enabled !== false) || target.models.length > 0
  const enabledEndpoints = modelPricing.value.filter((item) => item.enabled)
  const pricedEndpoints = enabledEndpoints.filter((item) => endpointHasCompatiblePrice(item))
  const mediaGateEndpoints = enabledEndpoints.filter((item) => sharedPoolEndpointRequiresMediaGate(item.endpoint_type))
  const passedMediaGateEndpoints = mediaGateEndpoints.filter((item) => sharedPoolEndpointGateSatisfied(item))
  const pricingDone = modelDone && enabledEndpoints.length > 0 && pricedEndpoints.length === enabledEndpoints.length
  const endpointDetectionDone = passedMediaGateEndpoints.length === mediaGateEndpoints.length
  const professionalReview = (target.verification_mode || 'full_check') === 'professional_review'
  const poolDetectionDone = professionalReview
    ? Boolean(target.verification_exemption_reason?.trim()) && ((target.account_summary?.schedulable_accounts || 0) > 0 || Boolean(target.last_probe_success))
    : fullCheckReady(target)
  // The top progress is the listing contract. Media probes are a separate
  // service-readiness gate and must not turn a passed pool-level full check
  // back into an apparent 80% result after the backend has listed the pool.
  const detectionDone = poolDetectionDone
  const allServiceReady = poolDetectionDone && endpointDetectionDone
  const mediaGateNote = mediaGateEndpoints.length > 0
    ? `媒体服务 ${passedMediaGateEndpoints.length}/${mediaGateEndpoints.length} 检测通过`
    : '文字服务不需要单独的图片/视频检测'
  const detectionNote = detectionDone
    ? (professionalReview ? `专业核验说明已齐；${mediaGateNote}` : `池级满血检测已通过；${mediaGateNote}`)
    : (professionalReview ? '待补齐专业核验说明与可调度账号' : probeStateDescription(target))
  const listedDone = Boolean(target.listed)
  const steps: Array<{ key: string; index: string; label: string; note: string; done: boolean; tab: TabKey }> = [
    { key: 'credential', index: '01', label: '上游凭证', note: target.account_mode_enabled ? '至少一个账号已保存凭证' : '池级 Base URL 与 Key 已保存', done: accountDone, tab: 'accounts' },
    { key: 'models', index: '02', label: '开放模型', note: modelDone ? `${target.models.length || target.model_configs?.length || 0} 个模型已配置` : '拉取并选择实际开放模型', done: modelDone, tab: 'models' },
    { key: 'pricing', index: '03', label: '服务价格', note: pricingDone ? `${pricedEndpoints.length}/${enabledEndpoints.length} 个服务价格完整` : (enabledEndpoints.length === 0 ? '等待后端返回实际服务端点' : `${pricedEndpoints.length}/${enabledEndpoints.length} 个服务价格完整`), done: pricingDone, tab: 'models' },
    { key: 'detection', index: '04', label: professionalReview ? '专业核验与媒体检测' : '满血检测', note: detectionNote, done: detectionDone, tab: 'detection' },
    { key: 'listing', index: '05', label: '公开上架', note: listedDone ? (pricingDone && detectionDone ? (allServiceReady ? '市场用户可浏览并调用全部已配置服务' : '池已公开；媒体未验证服务仍受保护') : '池已公开，未就绪服务仍会被拒绝') : (professionalReview ? '专业核验完成后可公开展示' : '价格与池级检测达标后自动上架'), done: listedDone, tab: 'detection' },
  ]
  const progress = Math.round((steps.filter((step) => step.done).length / steps.length) * 100)
  if (listedDone && pricingDone && detectionDone && allServiceReady) return { title: '池子已公开运营', description: '价格与服务检测证据完整；继续关注成员席位、服务健康和真实收益账本。', progress, tone: 'ready', steps }
  if (listedDone && pricingDone && detectionDone) return { title: '池子已公开运营，媒体服务待验证', description: `池级满血检测已经通过；${mediaGateNote}。图片或视频服务在验证前继续受保护，不影响文字服务的上架进度。`, progress, tone: 'attention', steps }
  if (listedDone) return { title: '池子已公开，但部分服务仍受保护', description: '市场页面可以看到池子；价格或检测未完成的服务仍会被拒绝，请按下方未完成步骤处理。', progress, tone: 'attention', steps }
  if (!accountDone) return { title: '先完成上游凭证配置', description: '凭证只加密保存，普通用户不可见。', progress, tone: 'pending', steps }
  if (!modelDone) return { title: '请拉取并确认开放模型', description: '模型拉取与能力检测分开，拉取成功不代表已经通过满血检测。', progress, tone: 'pending', steps }
  if (!pricingDone) return { title: '下一步：补齐每个服务的价格', description: enabledEndpoints.length === 0 ? '保存开放模型后，等待系统生成真实服务端点；没有端点时不能假装支持图片或视频。' : `还有 ${enabledEndpoints.length - pricedEndpoints.length} 个服务价格不完整。文字、图片、视频必须按自己的单位定价。`, progress, tone: 'attention', steps }
  if (detectionDone) {
    return {
      title: professionalReview ? '专业核验已完成，等待公开展示' : '检测已达标，等待自动上架',
      description: professionalReview ? `无需补跑满血检测，但要保持专业核验说明与可调度账号真实可用；${mediaGateNote}。` : `无需重复提交凭证；刷新后确认公开状态。${mediaGateEndpoints.length > 0 ? ` ${mediaGateNote}，媒体服务会继续独立受保护。` : ''}`,
      progress,
      tone: 'ready',
      steps
    }
  }
  if (professionalReview && !poolDetectionDone) {
    return { title: '下一步：补齐专业核验说明', description: '写清差异化模型的豁免原因，并确保至少一个账号可调度。', progress, tone: 'attention', steps }
  }
  return { title: '下一步：执行满血检测', description: '检测会产生逐项证据，并决定是否满足公开上架门槛。', progress, tone: 'attention', steps }
})

function togglePoolModel(modelName: string): void {
  selectedPoolModels.value = selectedPoolModelSet.value.has(modelName)
    ? selectedPoolModels.value.filter((item) => item !== modelName)
    : uniqueModels([...selectedPoolModels.value, modelName])
}

function selectAllPoolModels(): void {
  selectedPoolModels.value = [...effectivePoolModelCandidates.value]
}

function clearPoolModels(): void {
  selectedPoolModels.value = []
}

function resetPoolModelSelection(): void {
  const target = pool.value
  if (!target) return
  const configs = target.model_configs || []
  selectedPoolModels.value = uniqueModels(configs.length > 0
    ? configs.filter((item) => item.model_open !== false && item.enabled !== false).map((item) => item.model_name)
    : target.models || [])
  poolModelRates.value = Object.fromEntries(configs.map((item) => [item.model_name, parseSharedPoolMultiplier(item.rate_multiplier) ?? parseSharedPoolMultiplier(target.rate_multiplier) ?? 1]))
  poolModelConcurrency.value = Object.fromEntries(configs.map((item) => [item.model_name, Math.max(0, Math.floor(Number(item.max_concurrency) || 0))]))
  poolModelMessage.value = ''
}

function setPoolModelRate(modelName: string, event: Event): void {
  const value = Number((event.target as HTMLInputElement).value)
  if (parseSharedPoolMultiplier(value) == null) return
  poolModelRates.value = { ...poolModelRates.value, [modelName]: value }
}

function setPoolModelConcurrency(modelName: string, event: Event): void {
  const value = Number((event.target as HTMLInputElement).value)
  if (!Number.isFinite(value) || value < 0) return
  poolModelConcurrency.value = { ...poolModelConcurrency.value, [modelName]: Math.floor(value) }
}

async function pullSavedPoolModels(): Promise<void> {
  if (!pool.value || pool.value.account_mode_enabled) return
  syncingPoolModels.value = true
  poolModelMessage.value = ''
  poolModelStatus.value = 'success'
  try {
    const result = await fetchSharedPoolUpstreamModels({ pool_id: poolId })
    upstreamPoolModels.value = uniqueModels(result.models || [])
    if (upstreamPoolModels.value.length === 0) {
      poolModelStatus.value = 'error'
      poolModelMessage.value = '已连接 /v1/models，但上游没有返回可用模型。'
    } else {
      poolModelMessage.value = `已从已保存凭证拉取 ${upstreamPoolModels.value.length} 个模型 · HTTP ${result.http_status || 200}`
    }
  } catch (error) {
    poolModelStatus.value = 'error'
    poolModelMessage.value = errorMessage(error, '拉取模型失败，请检查池级凭证和 Base URL。')
  } finally {
    syncingPoolModels.value = false
  }
}

async function savePoolModelSelection(): Promise<void> {
  if (!pool.value || selectedPoolModels.value.length === 0) return
  savingPoolModels.value = true
  poolModelMessage.value = ''
  const existing = new Map((pool.value.model_configs || []).map((item) => [item.model_name, item]))
  const configs = selectedPoolModels.value.map((modelName) => {
    const current = existing.get(modelName)
    return {
      provider: current?.provider || 'openai',
      model_name: modelName,
      upstream_model_name: current?.upstream_model_name || modelName,
      display_name: current?.display_name,
      aliases: current?.aliases,
      rate_multiplier: poolModelRates.value[modelName] ?? current?.rate_multiplier ?? pool.value?.rate_multiplier ?? 1,
      rank_weight: current?.rank_weight,
      five_hour_protection_percent: current?.five_hour_protection_percent ?? 100,
      seven_day_protection_percent: current?.seven_day_protection_percent ?? 100,
      daily_protection_percent: current?.daily_protection_percent ?? 100,
      min_balance_admission: current?.min_balance_admission,
      hourly_seat_fee: current?.hourly_seat_fee,
      hourly_min_usage_waiver: current?.hourly_min_usage_waiver,
      max_concurrency: poolModelConcurrency.value[modelName] ?? current?.max_concurrency ?? 0,
      enabled: true,
      model_open: true,
      tags: current?.tags,
      pricing: current?.pricing,
    }
  })
  try {
    const updated = await updateSharedPool(poolId, {
      expected_config_version: Number(pool.value.config_version || 0),
      models: [...selectedPoolModels.value],
      model_configs: configs,
      probe_model: poolProbeModel.value || pool.value.models?.[0] || '',
      listed: false,
    })
    pool.value = updated
    resetPoolModelSelection()
    poolModelStatus.value = 'success'
    poolModelMessage.value = '开放模型已保存。模型范围变化后需重新完成满血检测，达标后自动上架。'
    await loadModelPricing()
  } catch (error) {
    poolModelStatus.value = 'error'
    poolModelMessage.value = errorMessage(error, '保存开放模型失败')
  } finally {
    savingPoolModels.value = false
  }
}

function pricingKey(item: Pick<SharedPoolModelEndpointPricing, 'pool_model_id' | 'endpoint_type'>): string {
  return `${item.pool_model_id}:${item.endpoint_type}`
}

function endpointLabel(endpoint: string): string {
  return sharedPoolEndpointLabel(endpoint)
}

function endpointAvailability(item: SharedPoolModelEndpointPricing) {
  return sharedPoolEndpointAvailability(item)
}

function endpointHasCompatiblePrice(item: SharedPoolModelEndpointPricing): boolean {
  return sharedPoolEndpointHasCompatiblePrice(item)
}

function billingOptions(endpoint: string) {
  return sharedPoolBillingOptions(endpoint)
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

function canEditPricing(item: SharedPoolModelEndpointPricing): boolean {
  return sharedPoolEndpointOrder.includes(item.endpoint_type)
}

function nullablePrice(value: number | null | undefined, factor = 1): number | null {
  return value == null || !Number.isFinite(Number(value)) ? null : Number(value) * factor
}

function draftFromPricing(item: SharedPoolModelEndpointPricing): PricingDraft {
  const base = item.current_price?.base_price
  const billingMode = base?.billing_mode && sharedPoolBillingModeAllowed(item.endpoint_type, base.billing_mode)
    ? base.billing_mode
    : defaultSharedPoolBillingMode(item.endpoint_type)
  return {
    billing_mode: billingMode,
    input_per_million: nullablePrice(base?.input_price, 1_000_000),
    output_per_million: nullablePrice(base?.output_price, 1_000_000),
    cache_read_per_million: nullablePrice(base?.cache_read_price, 1_000_000),
    cache_write_per_million: nullablePrice(base?.cache_write_price, 1_000_000),
    image_item_price: nullablePrice(base?.image_item_price),
    video_second_price: nullablePrice(base?.video_second_price),
    per_request_price: nullablePrice(base?.per_request_price),
    multiplier: parseSharedPoolMultiplier(item.current_price?.multiplier)
      ?? parseSharedPoolMultiplier(poolModelRates.value[item.model_name])
      ?? parseSharedPoolMultiplier(pool.value?.rate_multiplier)
      ?? 1,
    minimum_charge: nullablePrice(base?.minimum_charge),
    maximum_charge: nullablePrice(base?.maximum_charge),
  }
}

async function loadModelPricing(): Promise<void> {
  if (!poolId) return
  pricingLoading.value = true
  pricingMessage.value = ''
  pricingStatus.value = 'success'
  try {
    modelPricing.value = await listSharedPoolModelPricing(poolId)
    const drafts = { ...pricingDrafts.value }
    for (const item of modelPricing.value) {
      const key = pricingKey(item)
      if (!drafts[key] || item.current_price) drafts[key] = draftFromPricing(item)
    }
    pricingDrafts.value = drafts
  } catch (error) {
    pricingStatus.value = 'error'
    pricingMessage.value = errorMessage(error, '读取模型价格失败，请稍后重试')
  } finally {
    pricingLoading.value = false
  }
}

function finiteNonNegative(value: number | null): number | null {
  if (value == null || value === ('' as unknown as number)) return null
  const parsed = Number(value)
  return Number.isFinite(parsed) && parsed >= 0 ? parsed : null
}

function createPricingOperationID(key: string): string {
  const random = typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function'
    ? crypto.randomUUID()
    : `${Date.now()}-${Math.random().toString(16).slice(2)}`
  return `shared-price:${poolId}:${key}:${random}`.slice(0, 128)
}

async function saveCustomPricing(item: SharedPoolModelEndpointPricing): Promise<void> {
  if (savingPricingKey.value || publicationLocked.value || item.pricing_source !== 'owner_custom') return
  const key = pricingKey(item)
  const draft = pricingDrafts.value[key]
  if (!draft) return
  if (!canEditPricing(item) || !sharedPoolBillingModeAllowed(item.endpoint_type, draft.billing_mode)) {
    pricingStatus.value = 'error'
    pricingMessage.value = `${endpointLabel(item.endpoint_type)} 暂不支持这种收费方式，原价格没有改变。`
    return
  }
  const inputPerMillion = finiteNonNegative(draft.input_per_million)
  const outputPerMillion = finiteNonNegative(draft.output_per_million)
  const imageItemPrice = finiteNonNegative(draft.image_item_price)
  const videoSecondPrice = finiteNonNegative(draft.video_second_price)
  const perRequest = finiteNonNegative(draft.per_request_price)
  const minimum = finiteNonNegative(draft.minimum_charge)
  const maximum = finiteNonNegative(draft.maximum_charge)
  const optionalValues = [
    draft.minimum_charge,
    draft.maximum_charge,
    ...(draft.billing_mode === 'token' ? [draft.cache_read_per_million, draft.cache_write_per_million] : []),
  ]
  if (optionalValues.some(value => value != null && value !== ('' as unknown as number) && finiteNonNegative(value) == null)) {
    pricingStatus.value = 'error'
    pricingMessage.value = '可选价格也必须是非负的有限数字，留空才表示不设置。'
    return
  }
  if (draft.billing_mode === 'token' && (inputPerMillion == null || outputPerMillion == null)) {
    pricingStatus.value = 'error'
    pricingMessage.value = '按 token 收费时，输入价和输出价都必须填写，且不能小于 0。'
    return
  }
  if (draft.billing_mode === 'per_request' && perRequest == null) {
    pricingStatus.value = 'error'
    pricingMessage.value = '按请求收费时，请填写每次请求的价格，且不能小于 0。'
    return
  }
  if (draft.billing_mode === 'image' && imageItemPrice == null) {
    pricingStatus.value = 'error'
    pricingMessage.value = '按图片张数收费时，请填写每张图片的价格，且不能小于 0。'
    return
  }
  if (draft.billing_mode === 'video' && videoSecondPrice == null) {
    pricingStatus.value = 'error'
    pricingMessage.value = '按视频时长收费时，请填写每秒视频的价格，且不能小于 0。'
    return
  }
  if (minimum != null && maximum != null && minimum > maximum) {
    pricingStatus.value = 'error'
    pricingMessage.value = '单次最低收费不能高于单次最高收费。'
    return
  }
  const multiplier = Number(draft.multiplier)
  if (parseSharedPoolMultiplier(multiplier) == null) {
    pricingStatus.value = 'error'
    pricingMessage.value = '用户价格倍率必须是有限数字，且不能小于 0.0001。'
    return
  }
  const payload = {
    model_name: item.model_name,
    endpoint_type: item.endpoint_type,
    operation_id: '',
    billing_mode: draft.billing_mode,
    input_price: draft.billing_mode === 'token' ? Number(inputPerMillion) / 1_000_000 : null,
    output_price: draft.billing_mode === 'token' ? Number(outputPerMillion) / 1_000_000 : null,
    cache_read_price: draft.billing_mode === 'token' && finiteNonNegative(draft.cache_read_per_million) != null ? Number(draft.cache_read_per_million) / 1_000_000 : null,
    cache_write_price: draft.billing_mode === 'token' && finiteNonNegative(draft.cache_write_per_million) != null ? Number(draft.cache_write_per_million) / 1_000_000 : null,
    image_item_price: draft.billing_mode === 'image' ? imageItemPrice : null,
    video_second_price: draft.billing_mode === 'video' ? videoSecondPrice : null,
    per_request_price: draft.billing_mode === 'per_request' ? perRequest : null,
    multiplier,
    minimum_charge: minimum,
    maximum_charge: maximum,
  }
  const payloadIdentity = JSON.stringify(payload)
  const previousOperation = pricingOperations.get(key)
  const operationId = previousOperation?.payload === payloadIdentity ? previousOperation.operationId : createPricingOperationID(key)
  pricingOperations.set(key, { payload: payloadIdentity, operationId })
  const submittedPricingOperation = { payload: payloadIdentity, operationId }
  savingPricingKey.value = key
  pricingMessage.value = ''
  pricingStatus.value = 'success'
  try {
    const quote = await saveSharedPoolCustomPrice(poolId, { ...payload, operation_id: operationId })
    if (!quote || quote.pool_id !== poolId || quote.pool_model_id !== item.pool_model_id
      || quote.endpoint_id !== item.endpoint_id || !Number.isSafeInteger(quote.price_version_id) || quote.price_version_id <= 0) {
      throw new Error('Unconfirmed price publication')
    }
    await loadModelPricing()
    const refreshFailed = (pricingStatus.value as MessageStatus) === 'error'
    if (refreshFailed) pricingOperations.set(key, submittedPricingOperation)
    else pricingOperations.delete(key)
    pricingMessage.value = `${item.model_name} 的 ${endpointLabel(item.endpoint_type)} 新价格已发布，旧调用仍保留原价格。${refreshFailed ? '报价列表刷新失败，请重试读取，无需重复发布。' : ''}`
  } catch {
    pricingStatus.value = 'error'
    pricingMessage.value = '未能确认价格发布结果，请保留相同内容重试确认。'
  } finally {
    savingPricingKey.value = ''
  }
}

function updatePricingDraft(item: SharedPoolModelEndpointPricing, draft: PricingDraft): void {
  if (savingPricingKey.value || publicationLocked.value) return
  pricingDrafts.value = { ...pricingDrafts.value, [pricingKey(item)]: draft }
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
  return `输入 ${input} / 百万 · 输出 ${output} / 百万`
}

function formatMultiplier(value: number): string {
  return `${formatSharedPoolMultiplier(value)} 倍`
}

function formatPricingTime(value: string): string {
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '待生效' : date.toLocaleString('zh-CN', { hour12: false })
}

function emptyAccountForm(): AccountForm {
  return {
    name: '',
    provider: 'openai_compatible',
    upstream_base_url: '',
    upstream_api_key: '',
    models_text: DEFAULT_PROBE_MODEL,
    rpm_limit: 0,
  }
}

function parseModels(value: string): string[] {
  return uniqueModels(value.split(/[,|;\n]+/).map((model) => model.trim()).filter(Boolean))
}

function uniqueModels(models: string[]): string[] {
  const seen = new Set<string>()
  const result: string[] = []
  for (const model of models) {
    if (seen.has(model)) continue
    seen.add(model)
    result.push(model)
  }
  return result
}

function selectAccountProvider(id: SharedPoolProviderPresetId) {
  accountProviderId.value = id
  const preset = getSharedPoolProviderPreset(id)
  if (id !== 'custom') newAccount.value.provider = preset.providerValue
  if (preset.defaultBaseURL && id !== 'custom') {
    newAccount.value.upstream_base_url = preset.defaultBaseURL
  }
}

function normalizeAccountBaseURL(baseURL: string, provider = newAccount.value.provider): string {
  const preset = SHARED_POOL_PROVIDER_PRESETS.find((item) => item.providerValue === provider) || getSharedPoolProviderPreset(accountProviderId.value)
  return normalizeOpenAICompatibleBaseURL(baseURL, preset.autoNormalizeV1)
}

function defaultModelConfig(modelName: string, provider: string): SharedPoolModelConfig {
  return {
    provider: provider.trim() || 'openai',
    model_name: modelName,
    rate_multiplier: parseSharedPoolMultiplier(pool.value?.rate_multiplier) ?? 1,
    five_hour_protection_percent: 100,
    seven_day_protection_percent: 100,
    daily_protection_percent: 100,
    max_concurrency: 0,
    model_open: true,
  }
}

function recordOf(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null
}

function stringField(record: Record<string, unknown>, keys: string[]): string {
  for (const key of keys) {
    const value = record[key]
    if (typeof value === 'string' && value.trim()) return value.trim()
  }
  return ''
}

function numberField(record: Record<string, unknown>, key: string): number | undefined {
  const value = record[key]
  if (typeof value === 'number' && Number.isFinite(value)) return value
  if (typeof value === 'string' && value.trim()) {
    const parsed = Number(value)
    if (Number.isFinite(parsed)) return parsed
  }
  return undefined
}

function modelsFromUnknown(value: unknown): string[] {
  if (typeof value === 'string') return parseModels(value)
  if (!Array.isArray(value)) return []

  return uniqueModels(value.flatMap((item) => {
    if (typeof item === 'string') return parseModels(item)
    if (!recordOf(item)) return []
    return parseModels(stringField(item, ['model_name', 'name', 'id']))
  }))
}

function accountPayloadFromRecord(record: Record<string, unknown>, index: number): SharedPoolAccountPayload {
  const provider = stringField(record, ['provider']) || 'openai'
  const upstreamBaseUrl = stringField(record, ['upstream_base_url', 'base_url', 'baseUrl'])
  const upstreamApiKey = stringField(record, ['upstream_api_key', 'api_key', 'apiKey', 'key'])
  const explicitModels = modelsFromUnknown(record.models)
  const models = explicitModels.length > 0 ? explicitModels : modelsFromUnknown(record.model_configs)

  if (!upstreamBaseUrl || !upstreamApiKey) {
    throw new Error(`第 ${index + 1} 项缺少 Base URL 或 API Key`)
  }

  const resolvedProvider = provider || getSharedPoolProviderPreset(accountProviderId.value).providerValue
  return {
    name: stringField(record, ['name']) || `共享池账号 ${index + 1}`,
    provider: resolvedProvider,
    auth_type: 'api_key',
    upstream_base_url: normalizeAccountBaseURL(upstreamBaseUrl, resolvedProvider),
    upstream_api_key: upstreamApiKey,
    status: 'active',
    rpm_limit: numberField(record, 'rpm_limit') ?? 0,
    model_configs: (models.length > 0 ? models : [DEFAULT_PROBE_MODEL]).map((model) => defaultModelConfig(model, resolvedProvider)),
  }
}

function accountPayloadFromLine(line: string, index: number): SharedPoolAccountPayload {
  const preset = getSharedPoolProviderPreset(accountProviderId.value)
  const provider = preset.id === 'custom'
    ? (newAccount.value.provider.trim() || 'custom')
    : preset.providerValue
  const defaultBase = normalizeAccountBaseURL(
    newAccount.value.upstream_base_url.trim() || preset.defaultBaseURL || '',
    provider
  )

  // formats:
  // 1) sk-xxx
  // 2) sk-xxx|https://host/v1
  // 3) name,base,key,models
  // 4) key|base|name
  let name = `共享池账号 ${index + 1}`
  let upstreamBaseUrl = defaultBase
  let upstreamApiKey = ''
  let modelsText = DEFAULT_PROBE_MODEL

  if (line.includes(',') && !line.trim().startsWith('sk-')) {
    const [n = '', base = '', key = '', ...modelParts] = line.split(',').map((part) => part.trim())
    name = n || name
    upstreamBaseUrl = base || defaultBase
    upstreamApiKey = key
    modelsText = modelParts.join(',') || DEFAULT_PROBE_MODEL
  } else if (line.includes('|')) {
    const parts = line.split('|').map((part) => part.trim()).filter(Boolean)
    if (parts[0]?.startsWith('sk-') || parts[0]?.length > 20) {
      upstreamApiKey = parts[0] || ''
      upstreamBaseUrl = parts[1] || defaultBase
      if (parts[2]) name = parts[2]
      if (parts[3]) modelsText = parts[3]
    } else {
      name = parts[0] || name
      upstreamBaseUrl = parts[1] || defaultBase
      upstreamApiKey = parts[2] || ''
      modelsText = parts[3] || DEFAULT_PROBE_MODEL
    }
  } else {
    upstreamApiKey = line.trim()
  }

  if (!upstreamApiKey) {
    throw new Error(`第 ${index + 1} 行缺少 API Key`)
  }
  if (!upstreamBaseUrl) {
    throw new Error(`第 ${index + 1} 行缺少 Base URL；请先点选供应商或填写默认 Base URL`)
  }

  const models = parseModels(modelsText || DEFAULT_PROBE_MODEL)
  return {
    name,
    provider,
    auth_type: 'api_key',
    upstream_base_url: normalizeAccountBaseURL(upstreamBaseUrl, provider),
    upstream_api_key: upstreamApiKey,
    status: 'active',
    rpm_limit: 0,
    model_configs: models.map((model) => defaultModelConfig(model, provider)),
  }
}

function parseAccountImportItems(text: string): SharedPoolAccountPayload[] {
  const trimmed = text.trim()
  if (!trimmed) return []

  if (trimmed.startsWith('[')) {
    const parsed: unknown = JSON.parse(trimmed)
    if (!Array.isArray(parsed)) throw new Error('JSON 必须是账号对象数组')
    return parsed.map((item, index) => {
      if (!recordOf(item)) throw new Error(`第 ${index + 1} 项必须是对象`)
      return accountPayloadFromRecord(item, index)
    })
  }

  return trimmed
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter(Boolean)
    .map((line, index) => accountPayloadFromLine(line, index))
}

function sanitizeSecretMessage(message: string, extraSecrets: string[] = []): string {
  let sanitized = message.replace(/\bsk-[A-Za-z0-9._-]{4,}\b/g, 'sk-***')
  sanitized = sanitized.replace(/((?:api[_-]?key|upstream_api_key)["'\s:=]+)[^"'\s,}]+/gi, '$1***')
  const secretCandidates = [newAccount.value.upstream_api_key.trim(), ...extraSecrets]
  for (const secret of secretCandidates) {
    if (secret.trim().length >= 4) sanitized = sanitized.split(secret).join('***')
  }
  return sanitized
}

function errorMessage(error: unknown, fallback: string, extraSecrets: string[] = []): string {
  const errorRecord = recordOf(error) ? error : null
  const response = errorRecord?.response
  const responseRecord = recordOf(response) ? response : null
  const data = responseRecord?.data
  const dataRecord = recordOf(data) ? data : null
  const candidates = [dataRecord?.message, dataRecord?.detail, dataRecord?.error, errorRecord?.message]
  const message = candidates.find((candidate): candidate is string => typeof candidate === 'string' && candidate.trim().length > 0)
  return sanitizeSecretMessage(message || fallback, extraSecrets)
}

function emptyNativeAccountForm(): NativeAccountForm {
  return {
    name: '',
    provider: 'openai',
    auth_type: 'api_key',
    upstream_base_url: '',
    secret: '',
  }
}

function nativeAccountOperationStorageKey(name: string): string {
  return `shared-pool-native-account:${poolId}:${name.trim().toLowerCase()}`
}

function readNativeAccountOperationID(name: string): string {
  const key = nativeAccountOperationStorageKey(name)
  if (typeof window === 'undefined') return createSharedPoolNativeOperationID()
  const existing = window.sessionStorage.getItem(key)
  if (existing) return existing
  const operationID = createSharedPoolNativeOperationID()
  window.sessionStorage.setItem(key, operationID)
  return operationID
}

function clearNativeAccountOperationID(name: string): void {
  if (typeof window !== 'undefined') window.sessionStorage.removeItem(nativeAccountOperationStorageKey(name))
  nativeAccountOperationID.value = ''
}

function nativeRequestStatus(error: unknown): number | null {
  if (!recordOf(error)) return null
  if (typeof error.status === 'number') return error.status
  if (!recordOf(error.response)) return null
  return typeof error.response.status === 'number' ? error.response.status : null
}

type NativeRepairTarget = {
  readonly accountID: number
  readonly operationID: string
}

const nativeRepairTarget = ref<NativeRepairTarget | null>(null)

function toggleNativeAccountForm(): void {
  if (addingNativeAccount.value) return
  if (showNativeAccountForm.value) {
    nativeRepairTarget.value = null
    nativeAccountError.value = ''
    nativeRepairOperationID.value = ''
    nativeAccount.value = emptyNativeAccountForm()
    showNativeAccountForm.value = false
    return
  }
  nativeRepairTarget.value = null
  nativeAccountError.value = ''
  nativeRepairOperationID.value = ''
  nativeAccount.value = emptyNativeAccountForm()
  nativeAccount.value.name = `${pool.value?.name || '共享池'} · ${accounts.value.length + 1}`
  showNativeAccountForm.value = true
}

function openNativeConnection(): void {
  if (!showNativeAccountForm.value) toggleNativeAccountForm()
}

function retryNativeAccount(account: SharedPoolAccount): void {
  nativeAccount.value = {
    name: account.name,
    provider: account.provider || 'openai',
    auth_type: account.auth_type === 'oauth' ? 'oauth' : 'api_key',
    upstream_base_url: account.upstream_base_url || '',
    secret: '',
  }
  nativeRepairTarget.value = {
    accountID: account.id,
    operationID: account.native_operation_id || '',
  }
  nativeAccountOperationID.value = ''
  nativeRepairOperationID.value = createSharedPoolNativeOperationID()
  nativeAccountError.value = ''
  showNativeAccountForm.value = true
}

async function submitNativeAccount(): Promise<void> {
  if (!canSubmitNativeAccount.value || addingNativeAccount.value) return
  addingNativeAccount.value = true
  nativeAccountError.value = ''
  const current = nativeAccount.value
  if (nativeRepairTarget.value) {
    const repairTarget = nativeRepairTarget.value
    if (!repairTarget.operationID) {
      nativeAccountError.value = '服务器未返回该账号的原生操作标识，请刷新后重新提交。'
      nativeAccount.value.secret = ''
      addingNativeAccount.value = false
      return
    }
    const expectedConfigVersion = Number(pool.value?.config_version || 0)
    if (expectedConfigVersion <= 0) {
      nativeAccountError.value = '服务器配置版本不可用，请刷新后重新提交。'
      nativeAccount.value.secret = ''
      addingNativeAccount.value = false
      return
    }
    const repairOperationID = nativeRepairOperationID.value || createSharedPoolNativeOperationID()
    nativeRepairOperationID.value = repairOperationID
    try {
      const payload: RepairNativeSharedPoolAccountPayload = {
        repair_operation_id: repairOperationID,
        expected_config_version: expectedConfigVersion,
        provider: current.provider.trim(),
        auth_type: current.auth_type,
        upstream_base_url: current.upstream_base_url.trim(),
      }
      if (current.auth_type === 'oauth') {
        const credentials: unknown = JSON.parse(current.secret)
        if (!recordOf(credentials)) throw new Error('OAuth credentials must be an object')
        payload.credentials = credentials
      } else {
        payload.upstream_api_key = current.secret
      }
      await repairSharedPoolNativeAccount(poolId, repairTarget.accountID, payload)
      nativeRepairOperationID.value = ''
      nativeRepairTarget.value = null
      nativeAccount.value = emptyNativeAccountForm()
      showNativeAccountForm.value = false
      await Promise.all([loadPool(), loadAccounts()])
    } catch (error) {
      if (nativeRequestStatus(error) === 409) {
        await Promise.all([loadPool(), loadAccounts()])
        nativeRepairOperationID.value = ''
        nativeAccountError.value = '账号配置已变化，已刷新服务器版本，请重新明确提交修复。'
      } else {
        nativeAccountError.value = '修复提交失败，请检查账号配置后重新提交。'
      }
    } finally {
      nativeAccount.value.secret = ''
      addingNativeAccount.value = false
    }
    return
  }
  const operationID = nativeAccountOperationID.value || readNativeAccountOperationID(current.name)
  nativeAccountOperationID.value = operationID
  let accountSaved = false
  try {
    const payload: CreateNativeSharedPoolAccountPayload = {
      operation_id: operationID,
      name: current.name.trim(),
      provider: current.provider.trim(),
      auth_type: current.auth_type,
      upstream_base_url: current.upstream_base_url.trim(),
    }
    if (current.auth_type === 'oauth') {
      const credentials: unknown = JSON.parse(current.secret)
      if (!recordOf(credentials)) throw new Error('OAuth credentials must be an object')
      payload.credentials = credentials
    } else {
      payload.upstream_api_key = current.secret
    }
    const created = await createNativeSharedPoolAccount(poolId, payload)
    accountSaved = true
    clearNativeAccountOperationID(current.name)
    nativeAccount.value = emptyNativeAccountForm()
    showNativeAccountForm.value = false
    await Promise.all([loadPool(), loadAccounts()])
    await verifyNativeAccountReadiness(created)
  } catch {
    if (accountSaved) {
      showNativeAccountForm.value = false
      nativeAccountError.value = '资源已保存，模型状态刷新失败。请刷新页面后继续检查，无需再次提交 Key。'
    } else {
      nativeAccountError.value = '提交失败，请检查账号配置后使用相同操作重试。'
    }
  } finally {
    nativeAccount.value.secret = ''
    addingNativeAccount.value = false
  }
}

async function verifyNativeAccountReadiness(account: SharedPoolAccount): Promise<void> {
  if (nativeReadinessPendingAccountId.value !== null) return
  nativeAccountError.value = ''
  const expectedConfigVersion = Number(pool.value?.config_version || 0)
  if (expectedConfigVersion <= 0) {
    nativeAccountError.value = '服务器配置版本不可用，请刷新后重试。'
    return
  }
  nativeReadinessPendingAccountId.value = account.id
  try {
    await verifySharedPoolNativeReadiness(poolId, account.id, { expected_config_version: expectedConfigVersion })
    await Promise.all([loadPool(), loadAccounts()])
  } catch (error) {
    if (nativeRequestStatus(error) === 409) {
      await Promise.all([loadPool(), loadAccounts()])
      nativeAccountError.value = '账号配置已变化，已刷新服务器版本，请重新点击可用检查。'
    } else {
      nativeAccountError.value = '可用检查失败，请稍后重新发起。'
    }
  } finally {
    nativeReadinessPendingAccountId.value = null
  }
}

function importSecrets(items: SharedPoolAccountPayload[]): string[] {
  return items
    .map((item) => item.upstream_api_key)
    .filter((key): key is string => typeof key === 'string' && key.trim().length >= 4)
}

function sanitizedImportResult(result: SharedPoolAccountImportResult, secrets: string[]): SharedPoolAccountImportResult {
  return {
    ...result,
    items: result.items.map((item) => ({
      ...item,
      error: item.error ? sanitizeSecretMessage(item.error, secrets) : item.error,
    })),
  }
}

const openPoolModels = computed(() => {
  const models = pool.value?.models || []
  return Array.from(new Set(models.map((item) => String(item || '').trim()).filter(Boolean)))
})

function ensurePoolProbeModel(): void {
  const models = openPoolModels.value
  if (!models.includes(poolProbeModel.value)) {
    poolProbeModel.value = models[0] || ''
  }
}

function fullCheckReady(target: SharedPool): boolean {
  if (target.account_mode_enabled) {
    const summary = target.account_summary
    return (summary?.full_check_total_accounts ?? 0) > 0
      && (summary?.full_check_passed_accounts ?? 0) > 0
      && (summary?.schedulable_accounts ?? 0) > 0
  }
  return Boolean(target.last_probe_gate_passed) && (target.last_probe_full_check_score ?? 0) >= 70 && (target.last_probe_full_check_total ?? 0) > 0
}

function fullCheckBannerLabel(target: SharedPool): string {
  return target.account_mode_enabled ? '账号池可调度' : '满血检测通过'
}

function fullCheckBannerHint(target: SharedPool): string {
  if (!target.account_mode_enabled) {
    return target.listed ? '' : '检测门槛已达标；若仍未上架，请检查维护与治理状态'
  }
  const passed = target.account_summary?.full_check_passed_accounts ?? 0
  const total = target.account_summary?.full_check_total_accounts ?? 0
  if (total > passed) return `当前只调度通过检测的账号，仍有 ${total - passed} 个账号待复测`
  return '至少一个账号已通过检测；系统只会调度当前可用账号'
}

function verificationBadgeLabel(target: SharedPool): string {
  if ((target.verification_mode || 'full_check') === 'professional_review') return '专业核验'
  if (target.account_mode_enabled) return fullCheckReady(target) ? '账号池可调度' : '待账号检测'
  return fullCheckReady(target) ? '满血验证' : '待满血验证'
}

function verificationChipClass(target: SharedPool): string {
  if ((target.verification_mode || 'full_check') === 'professional_review') return 'aopd-review-chip'
  return fullCheckReady(target) ? 'aopd-passed-chip' : 'aopd-unlisted-chip'
}

function hasProbeEvidence(target: SharedPool): boolean {
  return Boolean(
    target.last_probe_at ||
    target.last_successful_probe_at ||
    (target.last_probe_full_check_total ?? 0) > 0 ||
    (target.consecutive_probe_failures ?? 0) > 0 ||
    (target.account_summary?.full_check_total_accounts ?? 0) > 0
  )
}

function probeStateLabel(target: SharedPool): string {
  if ((target.verification_mode || 'full_check') === 'professional_review') {
    return target.listed ? '专业核验已上架' : '专业核验中'
  }
  if (!hasProbeEvidence(target)) return '等待检测'
  if ((target.last_probe_full_check_total ?? 0) > 0 || (target.account_summary?.full_check_total_accounts ?? 0) > 0) {
    return fullCheckReady(target)
      ? (target.account_mode_enabled ? '账号池可调度' : '满血检测通过')
      : '能力检测结果'
  }
  return target.last_probe_success ? '基础可用' : '检测未通过'
}

function probeStateDescription(target: SharedPool): string {
  if ((target.verification_mode || 'full_check') === 'professional_review') {
    return target.verification_exemption_reason || '差异化模型已登记专业核验说明'
  }
  if (!hasProbeEvidence(target)) return '尚无检测记录，不代表不可用'
  if (target.account_mode_enabled && (target.account_summary?.full_check_total_accounts ?? 0) > 0) {
    return `账号检测 ${fullCheckProgressText(target)}，${formatProbeScore(target)}`
  }
  const total = target.last_probe_full_check_total ?? 0
  const passed = target.last_probe_full_check_passed ?? 0
  if (total > 0) return `能力检测 ${passed}/${total} 项通过，当前 ${formatProbeScore(target)}`
  if (target.last_probe_success) return '基础连通成功，仍需完成满血检测'
  return target.last_probe_error_message || '最近检测未通过，请查看检测记录'
}

function probeToneClass(target: SharedPool): string {
  if (!hasProbeEvidence(target)) return 'aopd-tone-pending'
  if (target.account_mode_enabled) {
    if ((target.account_summary?.full_check_passed_accounts ?? 0) > 0) return 'aopd-tone-good'
    return (target.account_summary?.full_check_total_accounts ?? 0) > 0 ? 'aopd-tone-danger' : 'aopd-tone-pending'
  }
  const score = target.last_probe_full_check_score
  if ((target.last_probe_full_check_total ?? 0) <= 0) return target.last_probe_success ? 'aopd-tone-basic' : 'aopd-tone-danger'
  if ((score ?? 0) >= 70) return 'aopd-tone-good'
  if ((score ?? 0) >= 60) return 'aopd-tone-warn'
  return 'aopd-tone-danger'
}

function formatProbeScore(target: SharedPool): string {
  if (!hasProbeEvidence(target)) return '待检测'
  if (target.account_mode_enabled) {
    const totalAccounts = target.account_summary?.full_check_total_accounts ?? 0
    const score = target.account_summary?.average_full_check_score
    if (totalAccounts <= 0 || score == null) return target.last_probe_success ? '基础可用' : '未通过'
    return `平均 ${score.toFixed(0)}%`
  }
  const score = target.last_probe_full_check_score ?? target.account_summary?.average_full_check_score
  if (score == null || (target.last_probe_full_check_total ?? 0) <= 0) return target.last_probe_success ? '基础可用' : '未通过'
  return `${score.toFixed(0)}%`
}

function fullCheckProgressText(target: SharedPool): string {
  if (target.account_mode_enabled) {
    const passed = target.account_summary?.full_check_passed_accounts ?? 0
    const total = target.account_summary?.full_check_total_accounts ?? 0
    return `账号 ${passed}/${total}`
  }
  return `${target.last_probe_full_check_passed ?? 0}/${target.last_probe_full_check_total ?? 0}`
}

function availabilityText(target: SharedPool): string {
  const hasTrafficEvidence = (target.total_calls ?? 0) > 0 || Boolean(target.last_successful_probe_at)
  if (!hasTrafficEvidence && Number(target.today_availability || 0) === 0) return '暂无样本'
  return formatPercent(target.today_availability)
}

function formatPlatformFee(value?: number): string {
  const percent = Number(value)
  return Number.isFinite(percent) && percent >= 0 ? `${percent.toFixed(2)}%` : '待平台规则'
}

function pendingRuleEffectiveText(target: SharedPool): string {
  return target.pending_settlement_rule_effective_from ? formatDateTime(target.pending_settlement_rule_effective_from) : '无待生效变更'
}

function pendingRuleSummary(target: SharedPool): string {
  if (!target.pending_settlement_rule_effective_from) return '当前没有下一窗口规则变更'
  return `席位费 ${formatSeatFee(target.pending_hourly_seat_fee || 0)} · 全免消费线 ${Number(target.pending_hourly_min_usage_waiver || 0).toFixed(4)}/h`
}

function formatPercent(value: number): string {
  if (value == null || Number.isNaN(Number(value))) return '—'
  const normalized = Number(value)
  const percent = normalized <= 1 ? normalized * 100 : normalized
  return `${Math.max(0, Math.min(100, percent)).toFixed(1)}%`
}

function formatSeatFee(value: number): string {
  return value > 0 ? `${Number(value).toFixed(4)}/h` : '免费'
}

function formatDate(value: string): string {
  return value ? new Date(value).toLocaleDateString() : '—'
}

function formatDateTime(value: string): string {
  return value ? new Date(value).toLocaleString() : '—'
}

function formatSuccessRate(successful: number, total: number): string {
  return total > 0 ? `${((successful / total) * 100).toFixed(0)}%` : '—'
}

function accountRoutingLabel(account: SharedPoolAccount): string {
  if (!account.schedulable) return '已停用调度'
  if (account.status !== 'active' || (account.gate_required && !account.gate_passed)) return '等待检测后调度'
  return '参与调度'
}

function formatProbeHistoryScore(history: SharedPoolProbeHistory): string {
  const score = history.metadata?.full_check_score
  return score == null ? '' : `满血分 ${score.toFixed(0)}% ·`
}

function probeTypeLabel(type: string): string {
  const labels: Record<string, string> = {
    manual: '手动满血检测',
    publish_gate: '发布闸门检测',
    scheduled: '定时基础检测',
    scheduled_full: '定时满血检测',
  }
  return labels[type] || type || '检测'
}

function probeHistorySummary(history: SharedPoolProbeHistory): string {
  const passed = history.metadata?.full_check_passed
  const total = history.metadata?.full_check_total
  if (typeof total === 'number' && total > 0) return `${passed || 0}/${total} 项通过`
  return history.success ? '基础连通通过' : (history.error_message || history.error_type || '检测未通过')
}

function toggleProbeDetail(id: number): void {
  expandedProbeId.value = expandedProbeId.value === id ? null : id
}

function collectibleCardImage(cardKey: string, rarity: string): string {
  return `/assets/zero-point-city/cards/collectible/${rarity}/${cardKey}.png`
}

async function loadSkinCards(): Promise<void> {
  skinLoading.value = true
  try {
    skinCards.value = await getCheckinCards()
  } catch {
    skinCards.value = []
  } finally {
    skinLoading.value = false
  }
}

function toggleSkinPicker(): void {
  showSkinPicker.value = !showSkinPicker.value
  if (showSkinPicker.value && skinCards.value.length === 0) {
    void loadSkinCards()
  }
}

async function applySkin(cardKey: string, rarity: string): Promise<void> {
  if (!pool.value) return
  settingSkin.value = true
  try {
    await setPoolCardSkin(poolId, cardKey, rarity)
    await Promise.all([loadPool(), loadMembers(), loadProbeHistories()])
    appStore.showSuccess('共享池背景已更新')
    showSkinPicker.value = false
  } catch (error) {
    appStore.showError(errorMessage(error, '设置共享池背景失败'))
  } finally {
    settingSkin.value = false
  }
}

async function clearSkin(): Promise<void> {
  if (!pool.value) return
  clearingSkin.value = true
  try {
    await clearPoolCardSkin(poolId)
    await Promise.all([loadPool(), loadMembers(), loadProbeHistories()])
    appStore.showSuccess('共享池背景已清除')
  } catch (error) {
    appStore.showError(errorMessage(error, '清除共享池背景失败'))
  } finally {
    clearingSkin.value = false
  }
}

const headerCardStyle = computed(() => {
  const skin = pool.value?.card_skin_key
  const rarity = pool.value?.card_skin_rarity
  if (!skin || !rarity) {
    return {
      backgroundImage: `linear-gradient(90deg, color-mix(in srgb, var(--bg) 96%, transparent) 0%, color-mix(in srgb, var(--bg) 74%, transparent) 58%, transparent 100%), url(/assets/zero-point-city/mascots/shared-pool-owner.png)`,
      backgroundSize: 'cover, auto 220%',
      backgroundPosition: 'center, right -20px center',
      backgroundRepeat: 'no-repeat',
    }
  }
  return {}
})

async function loadPool(): Promise<void> {
  const pools = await listMySharedPools()
  pool.value = pools.find((item) => item.id === poolId) || null
  initEditForm()
  resetPoolModelSelection()
}

async function loadAccounts(): Promise<void> {
  accountsLoading.value = true
  try {
    accounts.value = await listSharedPoolAccounts(poolId)
    accountsError.value = ''
  } catch {
    accountsError.value = '资源列表读取失败，请重试；无需重新提交已有凭证。'
  } finally {
    accountsLoading.value = false
  }
}

async function loadProbeHistories(): Promise<void> {
  try {
    probeHistories.value = await listSharedPoolProbeHistories(poolId, { limit: 20 })
  } catch {
    probeHistories.value = []
  }
}

async function loadMembers(): Promise<void> {
  const requestedView = memberView.value
  const sequence = ++memberLoadSequence
  membersLoading.value = true
  membersError.value = ''
  try {
    const loadedMembers = await listSharedPoolMembers(poolId, requestedView)
    if (sequence !== memberLoadSequence || requestedView !== memberView.value) return
    members.value = loadedMembers.filter(member => (
      requestedView === 'active' ? isActiveMember(member) : !isActiveMember(member)
    ))
  } catch {
    if (sequence === memberLoadSequence) membersError.value = '成员列表加载失败，请重试。'
  } finally {
    if (sequence === memberLoadSequence) membersLoading.value = false
  }
}

async function setMemberView(view: 'active' | 'released'): Promise<void> {
  if (memberView.value === view && !membersLoading.value) return
  memberView.value = view
  await loadMembers()
}

function isActiveMember(member: PoolSeat): boolean {
  const status = String(member.status || '').trim().toLowerCase()
  return status === 'active' || status === 'held'
}

function releaseReasonLabel(reason?: string): string {
  const labels: Record<string, string> = {
    user_leave: '用户主动退出',
    owner_removed: '池主移除',
    idle_timeout: '连续 2 小时无调用，系统自动释放',
    insufficient_balance: '余额不足，系统停止席位',
    pool_unavailable: '池已下架或暂不可用',
    pool_archived: '池已归档',
    admin_release: '管理员释放',
    legacy_release: '历史退出记录',
  }
  return labels[String(reason || '')] || '席位已结束'
}

async function addAccount(): Promise<void> {
  if (!canAddAccount.value) return
  addingAccount.value = true
  const models = accountModels.value
  const payload: SharedPoolAccountPayload = {
    name: newAccount.value.name.trim() || '共享池账号',
    provider: newAccount.value.provider.trim() || 'openai_compatible',
    auth_type: 'api_key',
    upstream_base_url: normalizeAccountBaseURL(newAccount.value.upstream_base_url.trim(), newAccount.value.provider),
    upstream_api_key: newAccount.value.upstream_api_key.trim(),
    status: 'active',
    rpm_limit: Number(newAccount.value.rpm_limit) || 0,
    model_configs: models.map((model) => defaultModelConfig(model, newAccount.value.provider)),
  }

  try {
    await createSharedPoolAccount(poolId, payload)
    newAccount.value = emptyAccountForm()
    showAddAccount.value = false
    await loadAccounts()
    await loadPool()
  } finally {
    addingAccount.value = false
  }
}

async function probeNewAccountModels(): Promise<void> {
  if (!canProbeNewAccountModels.value) return
  probingNewAccountModels.value = true
  modelProbeMessage.value = ''
  modelProbeStatus.value = 'success'
  try {
    const result = await fetchSharedPoolUpstreamModels({
      upstream_base_url: newAccount.value.upstream_base_url.trim(),
      upstream_api_key: newAccount.value.upstream_api_key.trim(),
    })
    const models = uniqueModels((result.models || []).filter(Boolean))
    if (models.length > 0) {
      newAccount.value.models_text = models.join(', ')
      const checkedAt = result.checked_at ? formatDateTime(result.checked_at) : '刚刚'
      modelProbeMessage.value = `已从 /v1/models 拉取 ${models.length} 个模型 · HTTP ${result.http_status || 200} · ${checkedAt}`
    } else {
      modelProbeMessage.value = '上游 /v1/models 已响应，但没有返回可用模型，请确认账号的模型权限。'
      modelProbeStatus.value = 'error'
    }
  } catch (error) {
    modelProbeStatus.value = 'error'
    modelProbeMessage.value = errorMessage(error, '识别上游模型失败')
  } finally {
    probingNewAccountModels.value = false
  }
}

async function handleOAuthPackageFile(event: Event): Promise<void> {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  accountImportError.value = ''
  oauthImportResult.value = null
  try {
    oauthImportText.value = await file.text()
    oauthImportFileName.value = file.name
  } catch (error) {
    accountImportError.value = errorMessage(error, '无法读取账号包')
  } finally {
    input.value = ''
  }
}

function parseOAuthPackage(text: string): SharedPoolOAuthDataPackage {
  const parsed = JSON.parse(text) as SharedPoolOAuthDataPackage
  if (!parsed || !Array.isArray(parsed.accounts)) {
    throw new Error('账号包缺少 accounts 数组')
  }
  if (parsed.accounts.length > 100) {
    throw new Error('单次最多导入 100 个账号')
  }
  return parsed
}

async function submitOAuthPackageImport(): Promise<void> {
  accountImportError.value = ''
  oauthImportResult.value = null
  let dataPackage: SharedPoolOAuthDataPackage
  try {
    dataPackage = parseOAuthPackage(oauthImportText.value)
  } catch (error) {
    accountImportError.value = errorMessage(error, '解析 OAuth 账号包失败')
    return
  }
  importingOAuthAccounts.value = true
  try {
    oauthImportResult.value = await importSharedPoolOAuthPackage(poolId, dataPackage, oauthUpdateExisting.value)
    if ((oauthImportResult.value.created || 0) + (oauthImportResult.value.updated || 0) > 0) {
      oauthImportText.value = ''
      oauthImportFileName.value = ''
      await Promise.all([loadAccounts(), loadPool()])
    }
  } catch (error) {
    accountImportError.value = errorMessage(error, 'OAuth 账号包导入失败')
  } finally {
    importingOAuthAccounts.value = false
  }
}

async function submitAccountImport(): Promise<void> {
  accountImportError.value = ''
  accountImportResult.value = null

  let items: SharedPoolAccountPayload[]
  try {
    items = parseAccountImportItems(accountImportText.value)
  } catch (error) {
    accountImportError.value = errorMessage(error, '解析导入内容失败')
    return
  }

  if (items.length === 0) {
    accountImportError.value = '请先粘贴要导入的账号'
    return
  }

  importingAccounts.value = true
  const secrets = importSecrets(items)
  try {
    const result = await importSharedPoolAccounts(poolId, items)
    accountImportResult.value = sanitizedImportResult(result, secrets)
    if (result.created > 0) {
      accountImportText.value = ''
      await Promise.all([loadAccounts(), loadPool()])
    }
  } catch (error) {
    accountImportError.value = errorMessage(error, '批量导入失败', secrets)
  } finally {
    importingAccounts.value = false
  }
}

async function deleteAccount(accountId: number): Promise<void> {
  deletingAccountId.value = accountId
  try {
    await deleteSharedPoolAccount(poolId, accountId)
    await loadAccounts()
    await loadPool()
  } finally {
    deletingAccountId.value = null
  }
}

function probeTaskStorageKey(): string {
  return `shared-pool-probe-task:${poolId}`
}

function readPersistedProbeTask(): PersistedProbeTask | null {
  if (typeof window === 'undefined') return null
  const raw = window.sessionStorage.getItem(probeTaskStorageKey())
  if (!raw) return null
  try {
    const task = JSON.parse(raw) as PersistedProbeTask
    if (
      task.version !== 1
      || task.pool_id !== poolId
      || !task.operation_id
      || !task.payload
      || Date.now() - Number(task.created_at || 0) > PROBE_TASK_MAX_AGE_MS
    ) {
      window.sessionStorage.removeItem(probeTaskStorageKey())
      return null
    }
    return task
  } catch {
    window.sessionStorage.removeItem(probeTaskStorageKey())
    return null
  }
}

function persistProbeTask(task: PersistedProbeTask): void {
  if (typeof window === 'undefined') return
  window.sessionStorage.setItem(probeTaskStorageKey(), JSON.stringify(task))
}

function clearPersistedProbeTask(operationID?: string): void {
  if (typeof window === 'undefined') return
  if (operationID) {
    const current = readPersistedProbeTask()
    if (current && current.operation_id !== operationID) return
  }
  window.sessionStorage.removeItem(probeTaskStorageKey())
}

function createProbeTask(payload: SharedPoolUpstreamProbePayload, scope: ProbeTaskScope): PersistedProbeTask {
  const operationID = createSharedPoolProbeOperationID()
  return {
    version: 1,
    pool_id: poolId,
    scope,
    operation_id: operationID,
    payload: {
      ...payload,
      pool_id: poolId,
      operation_id: operationID,
    },
    created_at: Date.now(),
  }
}

function isProbeAbortError(error: unknown): boolean {
  return (error as { name?: string })?.name === 'AbortError'
}

function shouldDiscardProbeTask(error: unknown): boolean {
  const status = Number((error as { status?: number })?.status || 0)
  return status >= 400 && status < 500 && status !== 408 && status !== 429
}

function updateProbeProgress(job: SharedPoolProbeJob, scope: ProbeTaskScope): void {
  activeProbeJob.value = job
  activeProbeScope.value = scope
  if (isSharedPoolProbeJobTerminal(job)) return
  const stage = job.status === 'queued' ? '已进入后台队列' : '正在服务器后台执行'
  const message = `${stage}，可以离开本页；回来后会自动继续显示任务 ${job.id.slice(0, 8)} 的结果。`
  if (scope === 'pool') {
    probeAllStatus.value = 'success'
    probeAllMessage.value = message
  } else {
    accountProbeStatus.value = 'success'
    accountProbeMessage.value = message
  }
}

async function executeProbeTask(task: PersistedProbeTask): Promise<SharedPoolProbeJob> {
  if (probeTaskRunning.value) {
    throw new Error('已有检测任务正在运行，请等待当前任务完成')
  }
  probeTaskRunning.value = true
  probePollController?.abort()
  const controller = new AbortController()
  probePollController = controller

  try {
    let job: SharedPoolProbeJob
    if (task.job_id) {
      job = await getSharedPoolProbeJob(task.job_id)
    } else {
      persistProbeTask(task)
      job = await probeSharedPoolUpstream(task.payload)
      task.job_id = job.id
      persistProbeTask(task)
    }
    if (controller.signal.aborted) {
      throw new DOMException('Probe polling aborted', 'AbortError')
    }
    updateProbeProgress(job, task.scope)
    if (!isSharedPoolProbeJobTerminal(job)) {
      job = await waitForSharedPoolProbeJob(job.id, {
        signal: controller.signal,
        onUpdate: (nextJob) => updateProbeProgress(nextJob, task.scope),
      })
    }
    activeProbeJob.value = job
    clearPersistedProbeTask(task.operation_id)
    return job
  } catch (error) {
    if (shouldDiscardProbeTask(error)) {
      clearPersistedProbeTask(task.operation_id)
    }
    throw error
  } finally {
    if (probePollController === controller) probePollController = null
    probeTaskRunning.value = false
  }
}

async function refreshProbeData(): Promise<void> {
  await Promise.all([loadAccounts(), loadProbeHistories(), loadPool()])
}

function probeJobFailureMessage(job: SharedPoolProbeJob, fallback: string): string {
  if (job.status === 'stale') return '检测已完成，但检测期间池子配置发生变化；旧结果没有用于上架，请按当前配置重新检测。'
  if (job.status === 'timed_out') return '本次检测在服务器端超时，任务已经结束；请检查上游响应速度后重试。'
  if (job.status === 'cancelled') return '本次检测任务已取消。'
  return job.error_message
    || job.result?.error_message
    || job.result?.message
    || fallback
}

async function applyAccountProbeOutcome(job: SharedPoolProbeJob): Promise<void> {
  await refreshProbeData()
  if (job.status === 'succeeded' && job.result?.ok) {
    accountProbeStatus.value = 'success'
    accountProbeMessage.value = `账号检测通过：${job.result.model || job.model_name || '探针模型'}`
    appStore.showSuccess(accountProbeMessage.value)
    return
  }
  accountProbeStatus.value = 'error'
  accountProbeMessage.value = probeJobFailureMessage(job, '账号检测未通过，请检查凭证、模型权限和上游状态。')
  appStore.showError(accountProbeMessage.value)
}

async function probeAccount(accountId: number): Promise<void> {
  if (readPersistedProbeTask()) {
    await resumePersistedProbeTask()
    return
  }
  probingAccountId.value = accountId
  accountProbeMessage.value = ''
  accountProbeStatus.value = 'success'
  try {
    const task = createProbeTask({ account_id: accountId, probe_type: 'manual' }, 'account')
    const job = await executeProbeTask(task)
    await applyAccountProbeOutcome(job)
  } catch (error) {
    if (isProbeAbortError(error)) return
    accountProbeStatus.value = 'error'
    accountProbeMessage.value = errorMessage(error, '暂时无法读取账号检测进度；已提交的任务仍会在服务器后台继续')
    appStore.showError(accountProbeMessage.value)
  } finally {
    probingAccountId.value = null
  }
}

async function probeAllAccounts(): Promise<void> {
  if (accounts.value.length === 0 || probingAccounts.value) return
  if (readPersistedProbeTask()) {
    await resumePersistedProbeTask()
    return
  }
  probingAccounts.value = true
  accountProbeMessage.value = ''
  accountProbeStatus.value = 'success'
  let succeeded = 0
  let failed = 0
  let firstFailure = ''

  try {
    for (const account of accounts.value) {
      try {
        const task = createProbeTask({ account_id: account.id, probe_type: 'manual' }, 'account')
        const job = await executeProbeTask(task)
        if (job.status === 'succeeded' && job.result?.ok) {
          succeeded += 1
        } else {
          failed += 1
          if (!firstFailure) {
            firstFailure = `${account.name || `账号 #${account.id}`}: ${probeJobFailureMessage(job, '检测未通过')}`
          }
        }
      } catch (error) {
        if (isProbeAbortError(error)) return
        failed += 1
        if (!firstFailure) {
          firstFailure = `${account.name || `账号 #${account.id}`}: ${errorMessage(error, '检测进度暂时不可用，后台任务未取消')}`
        }
        break
      }
    }
    await refreshProbeData()
    accountProbeStatus.value = failed > 0 ? 'error' : 'success'
    accountProbeMessage.value = failed > 0
      ? `账号检测完成：成功 ${succeeded}，失败 ${failed}${firstFailure ? ` · ${firstFailure}` : ''}`
      : `账号检测完成：成功 ${succeeded}，失败 ${failed}`
  } finally {
    probingAccounts.value = false
  }
}

async function removePool(): Promise<void> {
  if (!pool.value) return
  if (!window.confirm(`删除共享池「${pool.value.name}」？它将从经营列表移除，旧授权失效；历史账单和收益仍保留。临时不营业请使用“停用”。若仍有活跃成员或在途用量，服务器会阻止删除。`)) return
  deletingPool.value = true
  try {
    await deleteSharedPool(pool.value.id)
    appStore.showSuccess('共享池已删除，历史账本仍保留')
    await router.push('/account-square/owner')
  } catch (error) {
    appStore.showError(errorMessage(error, '删除共享池失败'))
  } finally {
    deletingPool.value = false
  }
}

async function toggleOwnerPause(): Promise<void> {
  const current = pool.value
  if (!current || pausingPool.value) return
  if (!window.confirm(current.owner_paused ? '恢复为待发布？不会自动上架，检查资源和报价后再发布。' : '停用后立即退出市场并停止新调用；成员、资源和历史账单保留。继续？')) return
  pausingPool.value = true
  try {
    pool.value = await setSharedPoolOwnerPause(current.id, !current.owner_paused, current.config_version || 0)
    appStore.showSuccess(current.owner_paused ? '已恢复为待发布，请检查资源后重新发布' : '已停用，成员和历史账单保留')
    await refreshAll()
  } catch (error) {
    appStore.showError(errorMessage(error, '未能确认停用状态，请刷新后重试'))
  } finally { pausingPool.value = false }
}

function isObservationPool(target: SharedPool | null | undefined): boolean {
  if (!target) return false
  return (String(target.governance_status || '') === 'watch' && Number(target.consecutive_probe_failures || 0) >= 3)
    || String(target.status || '') === 'limited'
    || String(target.status_note || '').includes('观察')
}

function poolStatusLabel(target: SharedPool): string {
  if (isObservationPool(target)) return '观察中'
  const labels: Record<string, string> = {
    healthy: '健康', limited: '受限', offline: '离线', maintenance: '维护中', testing: '检测中',
  }
  return labels[target.status] || target.status || '未知'
}

async function applyPoolProbeOutcome(job: SharedPoolProbeJob): Promise<void> {
  await refreshProbeData()
  if (job.status !== 'succeeded') {
    probeAllStatus.value = 'error'
    probeAllMessage.value = probeJobFailureMessage(job, '满血检测未通过，请查看下方检测记录与证据。')
    appStore.showError(probeAllMessage.value)
    return
  }

  const result = job.result
  const latest = pool.value
  const passed = result?.full_check_passed ?? latest?.last_probe_full_check_passed ?? 0
  const total = result?.full_check_total ?? latest?.last_probe_full_check_total ?? 0
  const score = result?.full_check_score ?? latest?.last_probe_full_check_score ?? 0
  let gatePassed = Boolean(latest && fullCheckReady(latest))
  if (result) {
    // A terminal job result is authoritative. Do not let an older successful
    // pool snapshot turn an explicitly failed probe into a false success.
    gatePassed = typeof result.gate_passed === 'boolean' ? result.gate_passed : result.ok === true
  }
  if (!gatePassed) {
    probeAllStatus.value = 'error'
    probeAllMessage.value = `满血检测已完成，但未达到上架门槛：通过 ${passed}/${total} · 分数 ${Number(score).toFixed(0)}%。请展开下方证据修复失败项。`
    appStore.showError(probeAllMessage.value)
    return
  }

  probeAllStatus.value = 'success'
  probeAllMessage.value = `满血检测完成：通过 ${passed}/${total} · 分数 ${Number(score).toFixed(0)}%`
    + (latest?.listed ? ' · 已自动上架' : ' · 已达标，当前治理状态暂未自动上架')
  appStore.showSuccess(probeAllMessage.value)
}

async function probeAll(): Promise<void> {
  if (readPersistedProbeTask()) {
    await resumePersistedProbeTask()
    return
  }
  ensurePoolProbeModel()
  if (!poolProbeModel.value) {
    probeAllStatus.value = 'error'
    probeAllMessage.value = '请先选择检测模型（探针），否则满血检测不会启动。'
    appStore.showError('请先选择检测模型（探针）')
    return
  }
  if (openPoolModels.value.length === 0) {
    probeAllStatus.value = 'error'
    probeAllMessage.value = '暂无可用检测模型。请先开放至少一个模型。'
    appStore.showError('暂无可用检测模型')
    return
  }
  const accountMode = Boolean(pool.value?.account_mode_enabled)
  probingAll.value = true
  probeAllMessage.value = accountMode
    ? `正在用可调度账号对模型 ${poolProbeModel.value} 执行满血检测，系统会自动避开空凭证账号，请勿重复点击…`
    : `正在对模型 ${poolProbeModel.value} 执行满血检测，通常需要数十秒，请勿重复点击…`
  probeAllStatus.value = 'success'
  appStore.showSuccess(accountMode ? '账号级满血检测已提交到后台' : '满血检测已提交到后台')
  try {
    const task = createProbeTask({
      probe_type: 'manual',
      probe_model: poolProbeModel.value,
    }, 'pool')
    const job = await executeProbeTask(task)
    await applyPoolProbeOutcome(job)
  } catch (error) {
    if (isProbeAbortError(error)) return
    probeAllStatus.value = 'error'
    try {
      await refreshProbeData()
    } catch {
      // ignore refresh failures and keep the original probe error message
    }
    const latest = pool.value
    probeAllMessage.value = latest?.last_probe_error_message
      || latest?.status_note
      || errorMessage(error, '暂时无法读取检测进度；已提交的任务仍会在服务器后台继续，刷新页面可恢复查看')
    appStore.showError(probeAllMessage.value)
  } finally {
    probingAll.value = false
  }
}

async function removeMember(seatId: number): Promise<void> {
  if (removingSeatId.value !== null) return
  removingSeatId.value = seatId
  memberActionMessage.value = ''
  let removed = false
  try {
    await removeSharedPoolMember(poolId, seatId)
    removed = true
    memberActionMessage.value = '成员已移除。'
    await loadMembers()
    await loadPool()
    if (membersError.value) memberActionMessage.value = '成员已移除，但列表刷新失败，请重试读取。'
  } catch {
    memberActionMessage.value = removed
      ? '成员已移除，但池子状态刷新失败，请刷新页面。'
      : '未能确认移除结果，请刷新成员列表后重试。'
  } finally {
    removingSeatId.value = null
  }
}

function initEditForm(): void {
  if (!pool.value) return
  editForm.value = {
    name: pool.value.name || '',
    description: pool.value.description || '',
    upstream_base_url: pool.value.upstream_base_url || '',
    upstream_api_key: '',
    account_mode_enabled: Boolean(pool.value.account_mode_enabled),
    rate_multiplier: parseSharedPoolMultiplier(pool.value.rate_multiplier) ?? 1,
    max_users: pool.value.max_users || 20,
    min_balance_admission: pool.value.min_balance_admission || 0,
    hourly_seat_fee: pool.value.pending_hourly_seat_fee ?? pool.value.hourly_seat_fee ?? 0,
    hourly_min_usage_waiver: pool.value.pending_hourly_min_usage_waiver ?? pool.value.hourly_min_usage_waiver ?? 0,
    account_concurrency: pool.value.account_concurrency || 1,
    user_concurrency: pool.value.user_concurrency || 1,
    verification_mode: (pool.value.verification_mode as 'full_check' | 'professional_review') || 'full_check',
    verification_exemption_reason: pool.value.verification_exemption_reason || '',
  }
  savePoolMessage.value = ''
}

async function savePoolSettings(): Promise<void> {
  if (!editForm.value || !pool.value) return
  savingPool.value = true
  savePoolMessage.value = ''
  try {
    const { updated, message } = await persistSharedPoolSettings({
      pool: pool.value,
      draft: editForm.value,
      updatePool: updateSharedPool,
      formatError: errorMessage,
      showError: (message) => appStore.showError(message),
      formatEffectiveAt: formatDate,
    })
    pool.value = updated
    initEditForm()
    savePoolStatus.value = 'success'
    savePoolMessage.value = message
    if (editForm.value) editForm.value.upstream_api_key = ''
  } catch (error) {
    savePoolStatus.value = 'error'
    savePoolMessage.value = error instanceof SharedPoolSettingsSaveError
      ? error.message
      : errorMessage(error, '保存失败，请重试')
  } finally {
    savingPool.value = false
  }
}

async function refreshAll(): Promise<void> {
  if (!poolId) return
  loading.value = true
  try {
    await loadPool()
    await Promise.all([loadAccounts(), loadProbeHistories(), loadMembers(), loadModelPricing()])
  } catch {
    pool.value = null
  } finally {
    loading.value = false
  }
}

async function resumePersistedProbeTask(): Promise<void> {
  const task = readPersistedProbeTask()
  if (!task || probeTaskRunning.value) return

  if (task.scope === 'pool') {
    probingAll.value = true
    probeAllStatus.value = 'success'
    probeAllMessage.value = '正在恢复上次未完成的后台满血检测…'
  } else {
    probingAccountId.value = task.payload.account_id || null
    accountProbeStatus.value = 'success'
    accountProbeMessage.value = '正在恢复上次未完成的后台账号检测…'
  }

  try {
    const job = await executeProbeTask(task)
    if (task.scope === 'pool') {
      await applyPoolProbeOutcome(job)
    } else {
      await applyAccountProbeOutcome(job)
    }
  } catch (error) {
    if (isProbeAbortError(error)) return
    const message = errorMessage(error, '暂时无法恢复检测进度；服务器任务不会因离开页面而取消')
    if (task.scope === 'pool') {
      probeAllStatus.value = 'error'
      probeAllMessage.value = message
    } else {
      accountProbeStatus.value = 'error'
      accountProbeMessage.value = message
    }
  } finally {
    if (task.scope === 'pool') {
      probingAll.value = false
    } else {
      probingAccountId.value = null
    }
  }
}

onMounted(async () => {
  await refreshAll()
  if (isNativePool.value && !accountsError.value && (route.query.setup === 'connect' || accounts.value.length === 0)) openNativeConnection()
  void resumePersistedProbeTask()
})

onBeforeUnmount(() => {
  probePollController?.abort()
  probePollController = null
})
</script>

<style scoped>
.native-pool-tabs { display: flex; gap: 24px; border-bottom: 1px solid var(--bd-ui-line); }
.native-pool-tabs button { min-height: 44px; padding: 10px 0; font-size: 14px; color: var(--bd-text-secondary); border-bottom: 2px solid transparent; }
.native-pool-tabs button[aria-current] { color: var(--bd-accent-teal); border-color: var(--bd-accent-teal); }
.native-pool-tabs button:focus-visible { outline: 2px solid var(--bd-accent-teal); outline-offset: 3px; }
.native-appearance { margin-bottom: 24px; }
.native-skin-picker { margin-top: 16px; }
.native-skin-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(95px, 1fr)); gap: 12px; }
.native-skin-grid button { min-width: 0; padding: 6px; border: 1px solid var(--bd-ui-line); border-radius: 6px; color: var(--bd-text-primary); background: var(--bd-surface); }
.native-skin-grid button[aria-pressed="true"] { border-color: var(--bd-accent-teal); }
.native-skin-grid img { display: block; width: 100%; aspect-ratio: 2 / 3; object-fit: cover; border-radius: 4px; }
.native-skin-grid span { display: block; font-size: 11px; margin-top: 6px; overflow-wrap: anywhere; }
.aopd-page {
  --bg: var(--module-panel, var(--zc-surface));
  --nd: color-mix(in srgb, var(--zc-shadow-dark) 24%, transparent);
  --nl: color-mix(in srgb, var(--zc-shadow-light) 58%, transparent);
  --text: var(--module-ink-strong, var(--zc-text-strong));
  --muted: var(--module-muted, var(--zc-muted));
  --teal: var(--module-accent-2, var(--zc-accent));
  --blue: var(--module-accent, var(--zc-accent-2));
  --gold: var(--zc-warning);
  --danger: var(--zc-danger);
  --raise-sm: var(--module-shadow, 0 16px 40px rgba(72, 92, 128, .1)), inset 0 1px 0 color-mix(in srgb, var(--nl) 72%, transparent);
  --inset-sm: inset 0 0 0 1px var(--module-line, var(--zc-line)), inset 0 3px 10px var(--nd);
  max-width: 960px;
  padding: 16px 0 80px;
  display: flex;
  flex-direction: column;
  gap: 20px;
}
.aopd-navigation { display: flex; flex-wrap: wrap; gap: 8px; margin-bottom: 14px; }
.aopd-back { display: inline-flex; align-items: center; min-height: 34px; padding: 0 12px; border-radius: 10px; font-size: 13px; font-weight: 800; color: var(--teal); text-decoration: none; background: var(--bg); box-shadow: var(--inset-sm); }
.aopd-back-current { box-shadow: 0 0 0 2px color-mix(in srgb, var(--teal) 24%, transparent), var(--inset-sm); }
.aopd-header {
  position: relative;
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  flex-wrap: wrap;
  overflow: hidden;
  border-radius: 22px;
  padding: 18px;
  background: var(--bg);
  box-shadow: var(--raise-sm);
}
.aopd-header-has-skin {
  border: 1px solid color-mix(in srgb, var(--teal) 16%, var(--module-line, var(--zc-line)));
}
.aopd-header-default-skin {
  border: 1px solid color-mix(in srgb, var(--teal) 12%, var(--module-line, var(--zc-line)));
}
.aopd-header-overlay {
  position: absolute;
  inset: 0;
  pointer-events: none;
  background:
    linear-gradient(90deg, color-mix(in srgb, var(--bg) 94%, transparent) 0%, color-mix(in srgb, var(--bg) 64%, transparent) 48%, color-mix(in srgb, var(--bg) 22%, transparent) 100%),
    linear-gradient(135deg, color-mix(in srgb, var(--teal) 8%, transparent), color-mix(in srgb, var(--blue) 7%, transparent));
  backdrop-filter: saturate(.95);
}
.aopd-header-default-skin .aopd-header-overlay {
  background: linear-gradient(90deg, color-mix(in srgb, var(--bg) 14%, transparent), color-mix(in srgb, var(--bg) 48%, transparent));
  backdrop-filter: none;
}
.aopd-pool-identity { position: relative; z-index: 1; display: flex; align-items: center; gap: 14px; min-width: 0; }
.aopd-avatar { width: 44px; height: 44px; border-radius: 13px; flex-shrink: 0; display: grid; place-items: center; font-size: 18px; font-weight: 900; color: var(--teal); background: rgba(47, 154, 154, .12); }
.aopd-title { margin: 0; font-size: clamp(22px, 3vw, 32px); font-weight: 950; color: var(--text); }
.aopd-status-row { display: flex; gap: 6px; flex-wrap: wrap; margin-top: 6px; }
.aopd-header-actions { position: relative; z-index: 1; display: flex; gap: 10px; flex-wrap: wrap; }
.aopd-status-chip, .aopd-listed-chip, .aopd-unlisted-chip, .aopd-passed-chip, .aopd-watch-chip { font-size: 10px; font-weight: 800; text-transform: uppercase; letter-spacing: .06em; border-radius: 999px; padding: 2px 9px; }
.aopd-s-healthy { background: rgba(47, 154, 154, .1); color: var(--teal); }
.aopd-s-limited, .aopd-s-maintenance { background: rgba(176, 125, 42, .12); color: var(--gold); }
.aopd-s-offline { background: rgba(192, 57, 43, .1); color: var(--danger); }
.aopd-listed-chip, .aopd-passed-chip { background: rgba(47, 154, 154, .1); color: var(--teal); }
.aopd-review-chip { background: rgba(176, 125, 42, .14); color: var(--gold); }
.aopd-watch-chip { background: rgba(217, 119, 6, .14); color: #b45309; }
.aopd-unlisted-chip { background: rgba(101, 115, 134, .1); color: var(--muted); }
.aopd-stats-row { display: grid; grid-template-columns: repeat(6, minmax(0, 1fr)); gap: 12px; }
.aopd-readiness { padding: 20px; border-radius: 20px; background: var(--bg); box-shadow: var(--raise-sm); display: grid; grid-template-columns: minmax(0, 1fr) auto; gap: 16px; align-items: center; }
.aopd-readiness-copy { min-width: 0; }
.aopd-readiness-kicker { display: block; margin-bottom: 5px; color: var(--blue); font-size: 10px; font-weight: 950; letter-spacing: .15em; text-transform: uppercase; }
.aopd-readiness h2 { margin: 0; color: var(--text); font-size: 18px; font-weight: 950; }
.aopd-readiness p { margin: 6px 0 0; color: var(--muted); font-size: 12px; line-height: 1.6; }
.aopd-readiness-progress { display: flex; align-items: center; gap: 10px; min-width: 180px; }
.aopd-readiness-bar { width: 130px; height: 8px; overflow: hidden; border-radius: 999px; background: color-mix(in srgb, var(--muted) 14%, transparent); box-shadow: var(--inset-sm); }
.aopd-readiness-bar span { display: block; height: 100%; border-radius: inherit; background: linear-gradient(90deg, var(--teal), var(--blue)); transition: width .3s ease; }
.aopd-readiness-progress b { color: var(--teal); font-size: 16px; }
.aopd-readiness-steps { grid-column: 1 / -1; display: grid; grid-template-columns: repeat(5, minmax(0, 1fr)); gap: 8px; }
.aopd-readiness-step { min-width: 0; padding: 11px; border: 0; border-radius: 14px; background: var(--bg); box-shadow: var(--inset-sm); color: var(--muted); text-align: left; cursor: pointer; display: flex; gap: 9px; align-items: flex-start; }
.aopd-readiness-step > span { width: 22px; height: 22px; flex: 0 0 22px; border-radius: 8px; display: grid; place-items: center; background: color-mix(in srgb, var(--muted) 10%, transparent); font-size: 9px; font-weight: 950; }
.aopd-readiness-step div { min-width: 0; display: grid; gap: 3px; }
.aopd-readiness-step b { color: var(--text); font-size: 12px; }
.aopd-readiness-step small { color: var(--muted); font-size: 10px; line-height: 1.45; }
.aopd-readiness-step-done > span { color: var(--zc-accent-ink); background: var(--teal); }
.aopd-readiness-attention .aopd-readiness-kicker { color: var(--gold); }
.aopd-readiness-ready .aopd-readiness-kicker { color: var(--teal); }
.aopd-tone-pending { color: var(--muted) !important; }
.aopd-tone-basic { color: var(--blue) !important; }
.aopd-tone-good { color: var(--teal) !important; }
.aopd-tone-warn { color: var(--gold) !important; }
.aopd-tone-danger { color: var(--danger) !important; }
.aopd-stat { display: flex; flex-direction: column; gap: 3px; padding: 14px 20px; border-radius: 16px; background: var(--bg); box-shadow: var(--raise-sm); min-width: 0; }
.aopd-stat b { font-size: 20px; font-weight: 950; color: var(--teal); }
.aopd-stat span { font-size: 11px; color: var(--muted); font-weight: 700; }
.aopd-tabs { display: flex; gap: 6px; background: var(--bg); box-shadow: var(--inset-sm); border-radius: 999px; padding: 5px; width: fit-content; }
.aopd-tab { min-height: 36px; padding: 0 18px; border: none; border-radius: 999px; font-size: 13px; font-weight: 800; cursor: pointer; color: var(--muted); background: transparent; }
.aopd-tab-active { background: var(--teal); color: var(--zc-accent-ink); box-shadow: 0 8px 18px color-mix(in srgb, var(--teal) 24%, transparent), inset 0 1px 0 rgba(255, 255, 255, .2); }
.aopd-section { display: flex; flex-direction: column; gap: 14px; }
.aopd-section-head { display: flex; align-items: flex-start; justify-content: space-between; gap: 12px; flex-wrap: wrap; }
.aopd-section-head h2 { margin: 0; font-size: 18px; font-weight: 900; color: var(--text); }
.aopd-section-head p { margin: 4px 0 0; color: var(--muted); font-size: 12px; line-height: 1.6; }
.aopd-section-actions { display: flex; align-items: center; justify-content: flex-end; gap: 8px; flex-wrap: wrap; }
.aopd-form-card { padding: 20px; border-radius: 18px; background: var(--bg); box-shadow: var(--raise-sm); display: flex; flex-direction: column; gap: 14px; }
.aopd-form-card h3 { margin: 0; font-size: 16px; font-weight: 900; color: var(--text); }
.aopd-card-head { display: flex; align-items: flex-start; justify-content: space-between; gap: 12px; }
.aopd-card-head p { margin: 4px 0 0; color: var(--muted); font-size: 12px; line-height: 1.6; }
.aopd-form-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 12px; }
.aopd-field { display: flex; flex-direction: column; gap: 5px; }
.aopd-field span { font-size: 12px; font-weight: 800; color: var(--text); }
.aopd-field b { color: var(--danger); }
.aopd-input { min-height: 38px; padding: 0 12px; border: none; border-radius: 10px; background: var(--bg); box-shadow: var(--inset-sm); font-size: 13px; color: var(--text); outline: none; }
.aopd-textarea { width: 100%; min-height: 132px; padding: 12px; border: none; border-radius: 12px; background: var(--bg); box-shadow: var(--inset-sm); font-size: 13px; line-height: 1.5; color: var(--text); outline: none; resize: vertical; font-family: inherit; box-sizing: border-box; }
.aopd-tool-row { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; }
.aopd-form-actions { display: flex; justify-content: flex-end; gap: 8px; }
.aopd-btn-skin-active { color: var(--teal); box-shadow: var(--inset-sm); }
.aopd-rule-note { margin: 0; padding: 10px 12px; border-radius: 12px; background: rgba(47, 154, 154, .07); color: var(--muted); font-size: 12px; line-height: 1.6; }
.aopd-skin-card-panel { background: color-mix(in srgb, var(--bg) 88%, transparent); }
.aopd-skin-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(150px, 1fr)); gap: 10px; }
.aopd-skin-grid-compact { grid-template-columns: repeat(auto-fit, minmax(112px, 132px)); justify-content: flex-start; }
.aopd-skin-option {
  border: 0;
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
.aopd-skin-option-active { box-shadow: var(--inset-sm); }
.aopd-skin-option-img { width: 100%; aspect-ratio: 0.72; object-fit: cover; border-radius: 12px; }
.aopd-skin-option-meta { display: flex; align-items: center; justify-content: space-between; gap: 10px; }
.aopd-skin-option-meta b { font-size: 11px; font-weight: 900; color: var(--text); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.aopd-skin-option-meta small { color: var(--muted); font-size: 10px; font-weight: 800; text-transform: uppercase; }
.aopd-oauth-import { display: grid; gap: 10px; padding: 14px; border-radius: 15px; background: color-mix(in srgb, var(--blue) 5%, var(--bg)); box-shadow: var(--inset-sm); }
.aopd-oauth-head { display: flex; align-items: center; justify-content: space-between; gap: 12px; flex-wrap: wrap; }
.aopd-oauth-head > div { display: grid; gap: 3px; }
.aopd-oauth-head b { color: var(--text); font-size: 13px; }
.aopd-oauth-head span, .aopd-file-name { color: var(--muted); font-size: 11px; }
.aopd-file-btn { position: relative; overflow: hidden; }
.aopd-file-btn input { position: absolute; inline-size: 1px; block-size: 1px; opacity: 0; }
.aopd-check-row { display: flex; align-items: center; gap: 8px; color: var(--text); font-size: 12px; font-weight: 750; }
.aopd-check-row input { accent-color: var(--teal); }
.aopd-oauth-warning { margin: 0; padding: 9px 11px; border-radius: 11px; color: var(--gold); background: color-mix(in srgb, var(--gold) 8%, transparent); font-size: 11px; line-height: 1.6; }
.aopd-auth-badge { display: inline-flex; margin-left: 5px; padding: 2px 7px; border-radius: 999px; color: var(--blue); background: color-mix(in srgb, var(--blue) 9%, transparent); font-size: 9px; font-weight: 900; vertical-align: middle; }
.aopd-inline-message { font-size: 12px; font-weight: 800; }
.aopd-message { padding: 10px 12px; border-radius: 12px; font-size: 12px; font-weight: 800; line-height: 1.5; }
.aopd-msg-success { background: rgba(47, 154, 154, .08); color: var(--teal); }
.aopd-msg-error { background: rgba(192, 57, 43, .08); color: var(--danger); }
.aopd-msg-neutral { background: color-mix(in srgb, var(--blue) 7%, transparent); color: var(--muted); }
.aopd-model-toolbar { display: flex; align-items: center; justify-content: space-between; gap: 10px; flex-wrap: wrap; color: var(--muted); font-size: 12px; font-weight: 800; }
.aopd-model-toolbar > div { display: flex; gap: 10px; }
.aopd-link-btn { padding: 0; border: 0; background: transparent; color: var(--teal); font-size: 12px; font-weight: 900; cursor: pointer; }
.aopd-model-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 9px; max-height: 420px; overflow: auto; padding: 3px; }
.aopd-model-grid article { min-width: 0; padding: 11px; border-radius: 14px; background: var(--bg); box-shadow: var(--raise-sm); cursor: pointer; display: grid; gap: 8px; }
.aopd-model-grid article.selected { box-shadow: var(--inset-sm); }
.aopd-model-grid article > div { min-width: 0; display: flex; gap: 8px; align-items: center; }
.aopd-model-grid article b { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; color: var(--text); font-size: 12px; }
.aopd-model-check { width: 19px; height: 19px; flex: 0 0 19px; border-radius: 7px; display: grid; place-items: center; color: var(--teal); background: color-mix(in srgb, var(--teal) 12%, transparent); font-size: 11px; font-weight: 950; }
.aopd-model-grid label { display: flex; align-items: center; gap: 8px; color: var(--muted); font-size: 10px; font-weight: 800; }
.aopd-model-grid input { width: 70px; min-height: 27px; border: 0; border-radius: 8px; padding: 0 7px; color: var(--teal); background: var(--bg); box-shadow: var(--inset-sm); outline: none; }
.aopd-pricing-head { display: flex; align-items: flex-end; justify-content: space-between; gap: 12px; padding-top: 8px; border-top: 1px solid color-mix(in srgb, var(--muted) 18%, transparent); }
.aopd-pricing-head h3 { margin: 0; color: var(--text); font-size: 15px; }
.aopd-pricing-head p { margin: 4px 0 0; color: var(--muted); font-size: 11px; line-height: 1.55; }
.aopd-capability-grid { display: grid; grid-template-columns: repeat(5, minmax(0, 1fr)); gap: 8px; }
.aopd-capability-grid article { display: grid; align-content: start; gap: 7px; min-width: 0; padding: 12px; border: 1px solid color-mix(in srgb, var(--muted) 18%, transparent); border-radius: 8px; background: color-mix(in srgb, var(--bg) 78%, transparent); }
.aopd-capability-grid article > div { display: grid; gap: 4px; }
.aopd-capability-grid b { color: var(--text); font-size: 11px; }
.aopd-capability-grid span, .aopd-capability-grid small { color: var(--muted); font-size: 9px; line-height: 1.5; }
.aopd-capability-grid strong { font-size: 11px; }
.aopd-capability-grid .is-ready strong { color: var(--teal); }
.aopd-capability-grid .is-warning strong { color: var(--gold); }
.aopd-capability-grid .is-danger strong { color: var(--danger); }
.aopd-capability-grid .is-muted strong { color: var(--muted); }
.aopd-capability-rule { padding: 10px 12px; border-radius: 7px; color: var(--blue); background: color-mix(in srgb, var(--blue) 7%, transparent); font-size: 10px; line-height: 1.6; }
.aopd-pricing-list { display: grid; gap: 10px; }
.aopd-pricing-item { display: grid; gap: 12px; min-width: 0; padding: 14px; border: 1px solid color-mix(in srgb, var(--muted) 18%, transparent); border-radius: 8px; background: color-mix(in srgb, var(--bg) 76%, transparent); }
.aopd-pricing-summary { display: flex; align-items: center; justify-content: space-between; gap: 10px; min-width: 0; }
.aopd-pricing-summary > div { display: flex; align-items: baseline; gap: 8px; min-width: 0; }
.aopd-pricing-summary b { overflow: hidden; color: var(--text); font-size: 13px; text-overflow: ellipsis; white-space: nowrap; }
.aopd-pricing-summary span { color: var(--muted); font-size: 10px; }
.aopd-pricing-badges { display: flex; align-items: center; justify-content: flex-end; gap: 6px; flex-wrap: wrap; }
.aopd-endpoint-status { flex: 0 0 auto; padding: 4px 7px; border-radius: 6px; font-size: 10px; font-weight: 850; }
.aopd-endpoint-status.is-ready { color: var(--teal); background: color-mix(in srgb, var(--teal) 11%, transparent); }
.aopd-endpoint-status.is-warning { color: var(--gold); background: color-mix(in srgb, var(--gold) 11%, transparent); }
.aopd-endpoint-status.is-danger { color: var(--danger); background: color-mix(in srgb, var(--danger) 10%, transparent); }
.aopd-endpoint-status.is-muted { color: var(--muted); background: color-mix(in srgb, var(--muted) 10%, transparent); }
.aopd-endpoint-explanation { margin: 0; color: var(--muted); font-size: 10px; line-height: 1.55; }
.aopd-pricing-source { flex: 0 0 auto; padding: 4px 7px; border-radius: 6px; font-weight: 800; }
.aopd-pricing-source.official { color: var(--blue); background: color-mix(in srgb, var(--blue) 10%, transparent); }
.aopd-pricing-source.custom { color: var(--teal); background: color-mix(in srgb, var(--teal) 10%, transparent); }
.aopd-price-facts { display: grid; grid-template-columns: 1.4fr .55fr 1.4fr .8fr; gap: 8px; }
.aopd-price-facts > div { display: grid; align-content: start; gap: 5px; min-width: 0; }
.aopd-price-facts small { color: var(--muted); font-size: 9px; }
.aopd-price-facts strong { overflow-wrap: anywhere; color: var(--text); font-size: 11px; line-height: 1.45; }
.aopd-price-example, .aopd-price-pending { margin: 0; padding: 9px 10px; border-radius: 6px; font-size: 10px; line-height: 1.55; }
.aopd-price-example { color: var(--blue); background: color-mix(in srgb, var(--blue) 7%, transparent); }
.aopd-price-pending { color: var(--danger); background: color-mix(in srgb, var(--danger) 8%, transparent); }
.aopd-price-editor { display: grid; gap: 10px; padding-top: 12px; border-top: 1px solid color-mix(in srgb, var(--muted) 18%, transparent); }
.aopd-price-editor-title { display: grid; gap: 3px; }
.aopd-price-editor-title b { color: var(--text); font-size: 12px; }
.aopd-price-editor-title span { color: var(--muted); font-size: 10px; line-height: 1.5; }
.aopd-price-fields { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 9px; }
.aopd-routing-note { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 8px; }
.aopd-routing-note div { min-width: 0; display: grid; gap: 4px; padding: 12px; border-radius: 13px; background: color-mix(in srgb, var(--blue) 6%, transparent); }
.aopd-routing-note b { color: var(--blue); font-size: 11px; }
.aopd-routing-note span { color: var(--muted); font-size: 10px; line-height: 1.5; }
.aopd-governance-strip { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 8px; }
.aopd-governance-strip article { min-width: 0; display: grid; gap: 4px; padding: 12px; border-radius: 13px; background: color-mix(in srgb, var(--gold) 8%, transparent); }
.aopd-governance-strip b { color: var(--gold); font-size: 11px; }
.aopd-governance-strip span { color: var(--muted); font-size: 10px; line-height: 1.5; }
.aopd-error-list { margin: 6px 0 0; padding-left: 18px; font-weight: 700; }
.aopd-member-filter { display: inline-grid; grid-template-columns: repeat(2, minmax(76px, 1fr)); gap: 3px; padding: 3px; border-radius: 8px; background: var(--bg); box-shadow: var(--inset-sm); }
.aopd-member-filter button { min-height: 30px; padding: 0 10px; border: 0; border-radius: 6px; background: transparent; color: var(--muted); font-size: 12px; font-weight: 850; cursor: pointer; }
.aopd-member-filter button.active { background: color-mix(in srgb, var(--teal) 12%, var(--bg)); color: var(--teal); box-shadow: var(--raise-sm); }
.aopd-account-list, .aopd-probe-list, .aopd-member-list, .aopd-ledger-list { display: flex; flex-direction: column; gap: 8px; }
.aopd-probe-row-wrap { border-radius: 14px; background: var(--bg); box-shadow: var(--raise-sm); overflow: hidden; }
.aopd-account-row, .aopd-probe-row, .aopd-member-row, .aopd-ledger-row { display: flex; align-items: center; gap: 10px; padding: 12px 16px; border-radius: 14px; background: var(--bg); box-shadow: var(--raise-sm); flex-wrap: wrap; }
.aopd-member-row.is-released { box-shadow: var(--inset-sm); }
.aopd-probe-row { width: 100%; border: 0; box-shadow: none; color: inherit; text-align: left; cursor: pointer; }
.aopd-probe-expand { color: var(--blue); font-size: 11px; font-weight: 850; }
.aopd-probe-detail { padding: 0 14px 14px; }
.aopd-check-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 8px; }
.aopd-check-grid article { min-width: 0; padding: 11px; border-radius: 12px; box-shadow: var(--inset-sm); display: grid; grid-template-columns: minmax(0, 1fr) auto; gap: 5px 8px; }
.aopd-check-grid article div { min-width: 0; display: grid; gap: 2px; }
.aopd-check-grid b { color: var(--text); font-size: 11px; }
.aopd-check-grid span, .aopd-check-grid small { color: var(--muted); font-size: 9px; line-height: 1.45; }
.aopd-check-grid small { grid-column: 1 / -1; overflow-wrap: anywhere; }
.aopd-check-grid strong { font-size: 10px; }
.aopd-check-ok strong { color: var(--teal); }
.aopd-check-fail strong { color: var(--danger); }
.aopd-account-info, .aopd-probe-info, .aopd-member-info { flex: 1; min-width: 180px; display: flex; flex-direction: column; gap: 3px; }
.aopd-account-info b, .aopd-probe-info b, .aopd-member-info b { font-size: 14px; font-weight: 800; color: var(--text); }
.aopd-account-meta, .aopd-probe-info span, .aopd-member-info span, .aopd-probe-time, .aopd-member-time, .aopd-ledger-time { font-size: 12px; color: var(--muted); }
.aopd-member-info small { color: var(--blue); font-size: 11px; font-weight: 750; }
.aopd-acc-status { font-size: 11px; font-weight: 700; border-radius: 999px; padding: 2px 9px; }
.aopd-acc-active { background: rgba(47, 154, 154, .1); color: var(--teal); }
.aopd-acc-disabled { background: rgba(192, 57, 43, .1); color: var(--danger); }
.aopd-probe-type { font-size: 16px; font-weight: 900; }
.aopd-probe-ok { color: var(--teal); }
.aopd-probe-fail { color: var(--danger); }
.aopd-passed-banner { padding: 14px 18px; border-radius: 14px; background: rgba(47, 154, 154, .08); color: var(--teal); font-size: 13px; font-weight: 800; display: flex; flex-wrap: wrap; gap: 8px; align-items: center; }
.aopd-passed-banner-review { background: rgba(176, 125, 42, .14); color: var(--gold); }
.aopd-passed-banner-watch { background: rgba(217, 119, 6, .12); color: #b45309; }
.aopd-hint { font-weight: 700; color: var(--blue); font-size: 12px; }
.aopd-ledger-summary { display: flex; gap: 12px; margin-bottom: 12px; }
.aopd-settlement-grid { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 10px; }
.aopd-settlement-grid article { min-width: 0; padding: 14px; border-radius: 15px; background: var(--bg); box-shadow: var(--raise-sm); display: grid; gap: 5px; }
.aopd-settlement-grid span { color: var(--muted); font-size: 10px; font-weight: 800; }
.aopd-settlement-grid b { color: var(--text); font-size: 15px; font-weight: 950; overflow-wrap: anywhere; }
.aopd-settlement-grid small { color: var(--muted); font-size: 10px; line-height: 1.5; }
.aopd-ledger-copy { flex: 1; min-width: 180px; display: grid; gap: 3px; }
.aopd-ledger-copy small { color: var(--muted); font-size: 10px; }
.aopd-ledger-note { flex: 1; font-size: 13px; color: var(--text); }
.aopd-ledger-amount { font-size: 14px; font-weight: 900; }
.aopd-amount-pos { color: var(--teal); }
.aopd-amount-neg { color: var(--danger); }
.aopd-loading { display: flex; align-items: center; gap: 12px; padding: 40px; color: var(--muted); font-weight: 700; }
.aopd-spinner { width: 20px; height: 20px; border-radius: 999px; border: 2px solid rgba(47, 154, 154, .15); border-top-color: var(--teal); animation: spin .8s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }
.aopd-muted { color: var(--muted); font-size: 13px; font-weight: 600; }
.aopd-empty { padding: 48px; text-align: center; color: var(--muted); font-weight: 700; }
.aopd-empty-sm { padding: 24px; text-align: center; color: var(--muted); font-size: 13px; font-weight: 600; background: var(--bg); box-shadow: var(--inset-sm); border-radius: 14px; }
.aopd-btn { display: inline-flex; align-items: center; justify-content: center; min-height: 34px; padding: 0 14px; border: none; border-radius: 10px; font-size: 13px; font-weight: 800; cursor: pointer; text-decoration: none; background: var(--bg); box-shadow: var(--raise-sm); color: var(--text); white-space: nowrap; }
.aopd-btn:disabled { opacity: .5; cursor: not-allowed; }
.aopd-btn-primary { background: var(--teal); color: var(--zc-accent-ink); box-shadow: 0 10px 24px color-mix(in srgb, var(--teal) 24%, transparent), inset 0 1px 0 rgba(255, 255, 255, .2); }
.aopd-btn-danger { color: var(--danger); }
.aopd-btn-sm { min-height: 30px; padding: 0 12px; font-size: 12px; }
.aopd-btn-xs { min-height: 26px; padding: 0 9px; font-size: 11px; }

@media (max-width: 960px) {
  .aopd-stats-row { grid-template-columns: repeat(3, minmax(0, 1fr)); }
  .aopd-settlement-grid, .aopd-readiness-steps { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .aopd-capability-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }
}
@media (max-width: 720px) {
  .aopd-form-grid, .aopd-stats-row, .aopd-settlement-grid, .aopd-readiness-steps, .aopd-check-grid, .aopd-model-grid, .aopd-routing-note, .aopd-price-facts, .aopd-price-fields, .aopd-capability-grid { grid-template-columns: 1fr; }
  .aopd-readiness { grid-template-columns: 1fr; }
  .aopd-readiness-progress { min-width: 0; width: 100%; }
  .aopd-readiness-bar { width: auto; flex: 1; }
  .aopd-section-actions, .aopd-form-actions { width: 100%; justify-content: stretch; }
  .aopd-section-actions .aopd-btn, .aopd-form-actions .aopd-btn { flex: 1 1 140px; }
  .aopd-tabs { width: 100%; overflow-x: auto; }
  .aopd-ledger-summary { flex-direction: column; }
}

.aopd-provider-grid { display:grid; grid-template-columns: repeat(auto-fit,minmax(140px,1fr)); gap:8px; }
.aopd-provider-card { text-align:left; border-radius:12px; border:1px solid rgba(20,34,56,.12); background:#f7f9fc; padding:10px; }
.aopd-provider-card b{display:block;font-size:12px;margin-bottom:4px;color:#0c1524}
.aopd-provider-card small{color:#667286;font-size:11px}
.aopd-provider-card.on{border-color:rgba(43,143,138,.45); box-shadow: inset 2px 2px 5px rgba(163,177,198,.28)}
.aopd-field-full{grid-column:1/-1}
</style>
