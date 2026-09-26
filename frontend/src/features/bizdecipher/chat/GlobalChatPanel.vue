<template>
  <section class="global-chat-panel" :class="{ 'global-chat-panel--expanded': expanded }" aria-label="零号城聊天">
    <header class="global-chat-panel__head">
      <div class="global-chat-panel__title">
        <strong>{{ activeChannel?.title || '零号城' }}</strong>
        <span>{{ activeChannel?.description || '公共大厅' }}</span>
      </div>
      <div class="global-chat-panel__actions">
        <button type="button" :aria-label="expanded ? '收起聊天' : '放大聊天'" @click="emit('toggle-expand')">
          <Minimize2 v-if="expanded" :size="16" />
          <Maximize2 v-else :size="16" />
        </button>
        <button type="button" aria-label="关闭聊天" @click="emit('close')"><X :size="16" /></button>
      </div>
    </header>

    <nav v-if="channels.length > 1" class="global-chat-panel__channels" aria-label="频道">
      <button
        v-for="channel in channels"
        :key="channel.slug"
        type="button"
        :class="{ active: channel.slug === activeSlug }"
        :aria-pressed="channel.slug === activeSlug"
        @click="selectChannel(channel.slug)"
      >
        {{ channel.title }}
        <b v-if="unread[channel.slug]" :aria-label="`${unread[channel.slug]} 条未读消息`">{{ unread[channel.slug] > 99 ? '99+' : unread[channel.slug] }}</b>
      </button>
    </nav>

    <div ref="scrollRegion" class="global-chat-panel__messages" role="log" aria-live="polite">
      <p v-if="loading && !messages.length" class="global-chat-panel__status">正在同步消息…</p>
      <p v-else-if="!messages.length" class="global-chat-panel__status">还没有消息，先说一句。</p>
      <article
        v-for="message in messages"
        :key="message.client_message_id || message.id"
        class="global-chat-panel__message"
        :class="{ 'is-own': message.sender_user_id === me, 'is-failed': message.failed }"
      >
        <img v-if="message.sender_avatar_url" :src="message.sender_avatar_url" :alt="message.sender_name" loading="lazy" />
        <div class="global-chat-panel__bubble">
          <span class="global-chat-panel__meta">
            <b>{{ message.sender_user_id === me ? '我' : message.sender_name }}</b>
            <time :datetime="message.created_at">{{ formatTime(message.created_at) }}</time>
          </span>
          <TokenPacketCard v-if="message.token_packet_id" :key="`${me}-${message.token_packet_id}`" :packet-id="message.token_packet_id" />
          <p v-else>{{ message.body }}</p>
          <span v-if="message.pending" class="global-chat-panel__state">发送中…</span>
          <span v-else-if="message.failed" class="global-chat-panel__state is-error">
            发送失败
            <button type="button" @click="retry(message.client_message_id)">重试</button>
          </span>
        </div>
      </article>
    </div>

    <p v-if="error" class="global-chat-panel__error" role="alert">
      {{ error }}
      <button type="button" @click="retrySync">重新同步</button>
    </p>

    <TokenPacketComposer v-if="packetComposer && authStore.user?.role === 'admin'" :key="`${me}-${activeSlug}`" :channel="activeSlug" @close="packetComposer = false" @sent="onPacketSent" />
    <button v-else-if="authStore.user?.role === 'admin'" type="button" class="global-chat-panel__packet-launch" @click="packetComposer = true">＋ 发 Token 红包</button>
    <form class="global-chat-panel__composer" @submit.prevent="submit">
      <label class="sr-only" for="global-chat-input">输入消息</label>
      <textarea
        id="global-chat-input"
        v-model="draft"
        rows="2"
        maxlength="2000"
        placeholder="说点什么，Enter 发送，Shift+Enter 换行"
        @keydown.enter.exact="onEnter"
      />
      <button type="submit" :disabled="sending || !draft.trim()" aria-label="发送消息">
        <SendHorizonal :size="16" />
      </button>
    </form>
  </section>
</template>

<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { Maximize2, Minimize2, SendHorizonal, X } from '@lucide/vue'
import { useAuthStore } from '@/stores/auth'
import { globalChat } from './globalChat'
import TokenPacketCard from './TokenPacketCard.vue'
import TokenPacketComposer from './TokenPacketComposer.vue'
const packetComposer = ref(false)
async function onPacketSent() {
  packetComposer.value = false
  await globalChat.loadHistory(globalChat.state.activeSlug)
}

defineProps<{ expanded: boolean }>()
const emit = defineEmits<{ close: []; 'toggle-expand': [] }>()

const authStore = useAuthStore()
const scrollRegion = ref<HTMLElement | null>(null)
const draft = ref(globalChat.draftFor(globalChat.state.activeSlug))

const me = computed(() => Number(authStore.user?.id) || 0)
const channels = computed(() => globalChat.state.channels)
const activeSlug = computed(() => globalChat.state.activeSlug)
const activeChannel = computed(() => globalChat.activeChannel.value)
const messages = computed(() => globalChat.activeMessages.value)
const loading = computed(() => globalChat.state.loading)
const sending = computed(() => globalChat.state.sending)
const error = computed(() => globalChat.state.error)
const unread = computed(() => globalChat.state.unread)

function formatTime(value: string): string {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''
  return date.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })
}

function scrollToBottom(): void {
  void nextTick(() => {
    const region = scrollRegion.value
    if (region) region.scrollTop = region.scrollHeight
  })
}

async function selectChannel(slug: string): Promise<void> {
  globalChat.setActiveChannel(slug)
  draft.value = globalChat.draftFor(slug)
  try {
    await globalChat.loadHistory(slug)
  } catch {
    // The shared error bar reports the failure.
  }
  scrollToBottom()
}

async function submit(): Promise<void> {
  const body = draft.value
  if (!body.trim() || sending.value) return
  const slug = activeSlug.value
  const userId = me.value
  draft.value = ''
  scrollToBottom()
  await globalChat.send(slug, body)
  if (activeSlug.value === slug && me.value === userId && !draft.value) {
    draft.value = globalChat.draftFor(slug)
  }
  scrollToBottom()
}

function onEnter(event: KeyboardEvent): void {
  if (event.isComposing || event.keyCode === 229) return
  event.preventDefault()
  void submit()
}

function retry(clientMessageID: string): void {
  void globalChat.retry(activeSlug.value, clientMessageID).then(scrollToBottom)
}

async function retrySync(): Promise<void> {
  globalChat.clearError()
  try {
    if (!globalChat.state.loaded) await globalChat.loadChannels()
    await globalChat.loadHistory(activeSlug.value)
  } catch {
    // The shared error bar remains available for another retry.
  }
}

watch(activeSlug, (slug) => {
  draft.value = globalChat.draftFor(slug)
  scrollToBottom()
})
watch(() => messages.value.length, scrollToBottom)
watch(draft, (value) => globalChat.setDraft(activeSlug.value, value))
</script>

<style scoped>
.global-chat-panel { display: flex; flex-direction: column; width: min(400px, calc(100vw - 40px)); height: min(560px, calc(100dvh - 40px)); overflow: hidden; border: 1px solid var(--bd-ui-line, rgba(148, 163, 184, 0.28)); border-radius: 8px; background: var(--bd-surface, #fff); color: var(--bd-text-primary, #0f172a); box-shadow: 0 24px 60px rgba(8, 20, 32, 0.32); }
.global-chat-panel--expanded { width: min(560px, calc(100vw - 40px)); height: min(760px, calc(100vh - 40px)); }
.global-chat-panel__head { display: flex; align-items: center; justify-content: space-between; gap: 12px; padding: 12px 14px; border-bottom: 1px solid var(--bd-ui-line, rgba(148, 163, 184, 0.22)); }
.global-chat-panel__title { display: flex; flex-direction: column; min-width: 0; }
.global-chat-panel__title strong { font-size: 14px; }
.global-chat-panel__title span { font-size: 11px; color: var(--bd-text-secondary, #64748b); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.global-chat-panel__actions { display: flex; gap: 4px; }
.global-chat-panel__actions button { display: grid; place-items: center; width: 30px; height: 30px; border-radius: 6px; color: var(--bd-text-secondary, #475569); }
.global-chat-panel__channels { display: flex; gap: 6px; padding: 8px 12px 0; overflow-x: auto; }
.global-chat-panel__channels button { flex: none; padding: 5px 10px; border-radius: 999px; font-size: 12px; color: var(--bd-text-secondary, #475569); background: var(--bd-ui-soft, rgba(148, 163, 184, 0.14)); }
.global-chat-panel__channels button.active { background: var(--bd-accent-teal, #0f766e); color: #fff; }
.global-chat-panel__channels b { display: inline-block; min-width: 16px; margin-left: 4px; padding: 0 4px; border-radius: 999px; background: #dc2626; color: #fff; font-size: 10px; line-height: 16px; text-align: center; }
.global-chat-panel__messages { flex: 1; min-height: 0; overflow-y: auto; padding: 12px 14px; display: flex; flex-direction: column; gap: 10px; }
.global-chat-panel__status { margin: auto; font-size: 12px; color: var(--bd-text-secondary, #64748b); }
.global-chat-panel__message { display: flex; gap: 8px; align-items: flex-start; max-width: 100%; }
.global-chat-panel__message img { width: 28px; height: 28px; border-radius: 50%; object-fit: cover; }
.global-chat-panel__bubble { min-width: 0; max-width: 82%; padding: 7px 10px; border-radius: 10px; background: var(--bd-ui-soft, rgba(148, 163, 184, 0.16)); }
.global-chat-panel__message.is-own { flex-direction: row-reverse; }
.global-chat-panel__message.is-own .global-chat-panel__bubble { background: color-mix(in srgb, var(--bd-accent-teal, #0f766e) 16%, transparent); }
.global-chat-panel__message.is-failed .global-chat-panel__bubble { outline: 1px solid #b91c1c; }
.global-chat-panel__meta { display: flex; gap: 8px; align-items: baseline; font-size: 11px; color: var(--bd-text-secondary, #64748b); }
.global-chat-panel__meta b { font-weight: 500; }
.global-chat-panel__bubble p { margin-top: 2px; font-size: 13px; line-height: 1.6; white-space: pre-wrap; overflow-wrap: anywhere; }
.global-chat-panel__state { display: inline-flex; gap: 6px; margin-top: 4px; font-size: 11px; color: var(--bd-text-secondary, #64748b); }
.global-chat-panel__state.is-error { color: #b91c1c; }
.global-chat-panel__error { display: flex; justify-content: space-between; gap: 10px; padding: 8px 14px; font-size: 12px; color: #b91c1c; border-top: 1px solid rgba(185, 28, 28, 0.24); }
.global-chat-panel__composer { display: flex; gap: 8px; align-items: flex-end; padding: 10px 12px; border-top: 1px solid var(--bd-ui-line, rgba(148, 163, 184, 0.22)); }
.global-chat-panel__composer textarea { flex: 1; min-height: 44px; max-height: 120px; resize: vertical; padding: 10px 12px; border-radius: 8px; border: 1px solid var(--bd-ui-line, rgba(148, 163, 184, 0.32)); background: transparent; font: inherit; font-size: 13px; color: var(--bd-text-primary, #0f172a); }
.global-chat-panel__composer button { display: grid; place-items: center; width: 40px; height: 40px; border-radius: 8px; background: var(--bd-accent-teal, #0f766e); color: #fff; }
.global-chat-panel__composer button:disabled { opacity: 0.45; }
@media (max-width: 640px) { .global-chat-panel, .global-chat-panel--expanded { width: 100vw; height: 100dvh; border-radius: 0; padding-bottom: env(safe-area-inset-bottom); } }
</style>
