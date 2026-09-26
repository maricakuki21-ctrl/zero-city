<template>
  <section class="native-onboarding" :data-state="state" data-testid="pool-onboarding-status" aria-live="polite">
    <div class="native-onboarding__summary">
      <div>
        <p class="native-onboarding__eyebrow">资源状态</p>
        <h2 data-testid="pool-onboarding-readiness">{{ readinessCopy }}</h2>
      </div>
      <span class="native-onboarding__state">{{ stateLabel }}</span>
    </div>

    <ul v-if="accounts.length" class="native-onboarding__accounts" data-testid="pool-onboarding-evidence">
      <li v-for="account in accounts" :key="account.id" class="native-onboarding__account">
        <div class="native-onboarding__account-copy">
          <strong>{{ account.name }}</strong>
          <span class="native-onboarding__connection-summary">{{ connectionStatusLabel(account.native_connection_status) }}</span>
          <div v-if="account.native_models?.length" class="native-onboarding__models">
            <span v-for="model in account.native_models.slice(0, 3)" :key="model">{{ model }}</span>
            <details v-if="account.native_models.length > 3" class="native-onboarding__model-overflow">
              <summary>其余 {{ account.native_models.length - 3 }} 个模型</summary>
              <span v-for="model in account.native_models.slice(3)" :key="model">{{ model }}</span>
            </details>
          </div>
          <details class="native-onboarding__details"><summary>检测详情</summary><span
            class="native-onboarding__connection"
            data-testid="native-account-connection"
            :data-account-id="account.id"
            :data-connection-status="connectionStatus(account.native_connection_status)"
          >
            连通性：{{ connectionStatusLabel(account.native_connection_status) }}
          </span>
          <span
            data-testid="native-account-connection-verified-at"
            :data-account-id="account.id"
            :data-has-timestamp="Boolean(account.native_connection_verified_at)"
          >
            连通性验证：{{ account.native_connection_verified_at || '暂无服务器验证时间' }}
          </span>
          <span
            data-testid="native-account-models-verified-at"
            :data-account-id="account.id"
            :data-has-timestamp="Boolean(account.native_models_verified_at)"
          >
            目录验证：{{ account.native_models_verified_at || '暂无服务器验证时间' }}
          </span>
          <span
            v-if="account.native_evidence_stale"
            class="native-onboarding__stale"
            data-testid="native-account-evidence-stale"
            :data-account-id="account.id"
            data-stale="true"
          >
            服务器证据可能已过期
          </span>
          <span v-if="account.native_error_code" class="native-onboarding__error-code">
            {{ account.native_error_code }}
          </span>
          </details>
        </div>
        <button
          v-if="account.native_binding_state === 'needs_attention'"
          class="native-onboarding__retry"
          type="button"
          data-testid="native-account-retry"
          :data-account-id="account.id"
          @click="$emit('retry', account)"
        >
          修复连接
        </button>
        <button
          class="native-onboarding__retry"
          type="button"
          data-testid="native-account-readiness"
          :data-account-id="account.id"
          :disabled="readinessPendingAccountId != null"
          @click="$emit('readiness', account)"
        >
          {{ readinessPendingAccountId === account.id ? '检查中…' : '检查并拉取模型' }}
        </button>
      </li>
    </ul>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type {
  SharedPoolAccount,
  SharedPoolNativeOnboardingState,
} from '@/features/bizdecipher/api/bizdecipher'

const onboardingStates = ['draft', 'supply_configuring', 'supply_needs_attention', 'supply_ready_billing_blocked', 'billing_active', 'legacy_existing'] as const
const connectionStates = ['pending', 'verifying', 'verified', 'authenticated_metadata_reachable', 'failed', 'unverified'] as const

type NativeOnboardingDisplayState = SharedPoolNativeOnboardingState | 'unknown'
type NativeConnectionDisplayState = (typeof connectionStates)[number] | 'unknown'

const props = defineProps<{
  state: string
  accounts: SharedPoolAccount[]
  readinessPendingAccountId?: number | null
}>()

defineEmits<{
  retry: [account: SharedPoolAccount]
  readiness: [account: SharedPoolAccount]
}>()

const state = computed<NativeOnboardingDisplayState>(() => isOneOf(onboardingStates, props.state)
  ? props.state
  : 'unknown')

const stateLabelByState = {
  draft: '草稿',
  supply_configuring: '配置中',
  supply_needs_attention: '需要处理',
  supply_ready_billing_blocked: '供给就绪',
  legacy_existing: '历史池',
  billing_active: '计费已接通',
  unknown: '未知状态',
} satisfies Record<NativeOnboardingDisplayState, string>

const readinessCopyByState = {
  draft: '连接资源后，这里会出现支持的模型',
  supply_configuring: '正在连接资源',
  supply_needs_attention: '部分资源需要重新连接',
  supply_ready_billing_blocked: '资源已连接，待开通计费与上架',
  legacy_existing: '这是历史兼容共享池',
  billing_active: '计费已接通；模型可用性以检测结果为准',
  unknown: '服务器返回了尚未识别的供给状态',
} satisfies Record<NativeOnboardingDisplayState, string>

const connectionLabelByState = {
  pending: '等待验证',
  verifying: '验证中',
  verified: '历史连接验证（不代表推理可用）',
  authenticated_metadata_reachable: '认证元数据可达（不代表推理可用）',
  failed: '验证失败',
  unverified: '未验证',
  unknown: '未知状态',
} satisfies Record<NativeConnectionDisplayState, string>

const stateLabel = computed(() => stateLabelByState[state.value])
const readinessCopy = computed(() => readinessCopyByState[state.value])

function connectionStatus(value: string | undefined): NativeConnectionDisplayState {
  return value && isOneOf(connectionStates, value)
    ? value
    : 'unknown'
}

function connectionStatusLabel(value: string | undefined): string {
  return connectionLabelByState[connectionStatus(value)]
}

function isOneOf<const T extends string>(values: readonly T[], value: string): value is T {
  return values.some((item) => item === value)
}
</script>

<style scoped>
.native-onboarding { display: grid; gap: var(--bd-space-4); padding: var(--bd-space-5); border: var(--bd-line-width) solid color-mix(in srgb, var(--bd-accent-blue) 24%, transparent); border-radius: var(--bd-radius-md); background: color-mix(in srgb, var(--bd-surface) 86%, var(--bd-accent-blue)); color: var(--bd-text-primary); }
.native-onboarding__summary { display: flex; align-items: start; justify-content: space-between; gap: var(--bd-space-3); }
.native-onboarding__eyebrow { margin: 0 0 var(--bd-space-1); color: var(--bd-text-secondary); font-size: var(--bd-type-overline); font-weight: var(--bd-weight-overline); }
.native-onboarding h2 { margin: 0; font-size: 1rem; line-height: var(--bd-line-body-small); }
.native-onboarding__state { flex: 0 0 auto; padding: 4px 8px; border-radius: var(--bd-radius-pill); color: var(--bd-accent-blue); background: color-mix(in srgb, var(--bd-accent-blue) 12%, transparent); font-size: var(--bd-type-caption); font-weight: var(--bd-weight-emphasis); }
.native-onboarding__notice { margin: 0; color: var(--bd-accent-gold); font-size: var(--bd-type-caption); line-height: var(--bd-line-body-small); }
.native-onboarding__accounts { display: grid; gap: var(--bd-space-2); margin: 0; padding: 0; list-style: none; }
.native-onboarding__account { display: flex; align-items: center; justify-content: space-between; gap: var(--bd-space-3); padding: var(--bd-space-3); border-radius: var(--bd-radius-sm); background: color-mix(in srgb, var(--bd-surface-raised) 84%, transparent); }
.native-onboarding__account-copy { display: grid; min-width: 0; gap: 2px; }
.native-onboarding__account-copy strong { overflow-wrap: anywhere; font-size: .875rem; }
.native-onboarding__account-copy span { color: var(--bd-text-secondary); font-size: var(--bd-type-caption); line-height: var(--bd-line-caption); overflow-wrap: anywhere; }
.native-onboarding__models { color: var(--bd-accent-teal) !important; }
.native-onboarding__models { display: flex; flex-wrap: wrap; align-items: baseline; gap: 6px 12px; margin-top: 8px; }
.native-onboarding__models>span { max-width: 100%; font-family: Consolas, monospace; }
.native-onboarding__model-overflow { flex-basis: 100%; font-size: 12px; }
.native-onboarding__model-overflow summary { cursor: pointer; min-height: 28px; }
.native-onboarding__model-overflow span { display: block; margin: 4px 0; overflow-wrap: anywhere; }
.native-onboarding__connection { color: var(--bd-accent-blue) !important; }
.native-onboarding__stale { color: var(--bd-accent-gold) !important; }
.native-onboarding__error-code { color: var(--bd-status-danger) !important; font-family: JetBrains Mono, SFMono-Regular, Consolas, monospace; }
.native-onboarding__retry { flex: 0 0 auto; min-height: 32px; padding: 0 10px; border: 0; border-radius: var(--bd-radius-sm); background: var(--bd-accent-blue); color: #fff; font: inherit; font-size: var(--bd-type-caption); font-weight: var(--bd-weight-emphasis); cursor: pointer; }
.native-onboarding__retry:focus-visible { outline: var(--bd-focus-ring-width) solid var(--bd-accent-teal); outline-offset: 2px; }
.native-onboarding { padding: 0; background: transparent; border: 0; border-radius: 0; }
.native-onboarding__account { padding: 16px 0; background: transparent; border-radius: 0; border-bottom: 1px solid var(--bd-ui-line); flex-wrap: wrap; }
.native-onboarding__account-copy { flex: 1; flex-basis: 100%; }
.native-onboarding__account-copy strong { font-weight: 600; }
.native-onboarding__details { font-size: 12px; margin-top: 8px; color: var(--bd-text-secondary); }
.native-onboarding__details summary { cursor: pointer; padding: 4px 0; }
.native-onboarding__details span { display: block; margin-top: 4px; }
.native-onboarding__retry { border: 1px solid var(--bd-ui-line); background: var(--bd-surface); color: var(--bd-accent-teal); min-height: 36px; }
@media (max-width: 375px) {
  .native-onboarding__summary, .native-onboarding__account { align-items: stretch; flex-direction: column; flex-wrap: nowrap; }
  .native-onboarding__account-copy { flex: 0 1 auto; }
  .native-onboarding__retry { width: 100%; }
}
</style>
