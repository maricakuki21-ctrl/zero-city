<template>
  <Teleport to="body">
  <div v-if="userId" class="global-chat" :class="{ 'global-chat--open': open }">
    <component :is="GlobalChatPanel" v-if="open" :expanded="expanded" @close="close" @toggle-expand="expanded = !expanded" />
    <button
      v-else
      class="global-chat__launcher"
      type="button"
      :aria-label="unreadCount ? `打开零号城聊天，${unreadCount} 条未读消息` : '打开零号城聊天'"
      :aria-expanded="open"
      :title="unreadCount ? `${unreadCount} 条未读消息` : '打开零号城聊天'"
      @click="openChat"
    >
      <MessagesSquare :size="20" />
      <span class="global-chat__label">零号城</span>
      <span v-if="unreadCount" class="global-chat__unread-dot" aria-hidden="true" />
    </button>
  </div>
  </Teleport>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { MessagesSquare } from '@lucide/vue'
import { useRoute } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { globalChat } from './globalChat'

import GlobalChatPanel from './GlobalChatPanel.vue'

const authStore = useAuthStore()
const route = useRoute()
const open = ref(false)
const expanded = ref(false)
let pollTimer: ReturnType<typeof setInterval> | undefined
let syncing = false
let lastChannelSync = 0

const userId = computed(() => Number(authStore.user?.id) || 0)
const unreadCount = computed(() => globalChat.totalUnread.value)

function openChat(): void {
  open.value = true
  updateVisibility()
  void sync()
}

function close(): void {
  open.value = false
  expanded.value = false
  updateVisibility()
}

async function sync(): Promise<void> {
  if (syncing || !userId.value || document.visibilityState === 'hidden') return
  syncing = true
  try {
    if (!globalChat.state.loaded || Date.now() - lastChannelSync > 20000) {
      await globalChat.loadChannels()
      lastChannelSync = Date.now()
    }
    if (open.value) await globalChat.refreshActive()
    await globalChat.refreshUnread()
  } catch {
    // Keep the panel available when synchronization fails.
  } finally {
    syncing = false
  }
}

function updateVisibility(): void {
  globalChat.setVisible(open.value && document.visibilityState !== 'hidden')
}

function onVisibilityChange(): void {
  updateVisibility()
  void sync()
}

function startPolling(): void {
  stopPolling()
  pollTimer = setInterval(() => void sync(), open.value ? 4000 : 10000)
}

function stopPolling(): void {
  if (pollTimer) clearInterval(pollTimer)
  pollTimer = undefined
}

watch(userId, (id) => {
  if (id) {
    close()
    globalChat.init(id)
    lastChannelSync = 0
    void sync()
  } else {
    globalChat.reset()
    close()
  }
}, { immediate: true })

watch(open, () => startPolling())

watch(() => route.query.chat, (value) => {
  if (value && userId.value) openChat()
})

function onKeydown(event: KeyboardEvent): void {
  if (event.key === 'Escape' && open.value && expanded.value) expanded.value = false
}

onMounted(() => {
  document.addEventListener('keydown', onKeydown)
  document.addEventListener('visibilitychange', onVisibilityChange)
  window.addEventListener('online', onVisibilityChange)
  startPolling()
  if (route.query.chat && userId.value) openChat()
})

onBeforeUnmount(() => {
  document.removeEventListener('keydown', onKeydown)
  document.removeEventListener('visibilitychange', onVisibilityChange)
  window.removeEventListener('online', onVisibilityChange)
  globalChat.setVisible(false)
  stopPolling()
})
</script>

<style scoped>
.global-chat { position: fixed; right: 20px; bottom: 20px; z-index: 60; }
.global-chat__launcher { position: relative; display: inline-flex; align-items: center; gap: 8px; min-height: 44px; padding: 0 16px; border-radius: 999px; background: var(--bd-accent-teal, #0f766e); color: #fff; font-size: 13px; box-shadow: 0 12px 30px rgba(8, 20, 32, 0.28); }
.global-chat__launcher:focus-visible { outline: 2px solid var(--bd-accent-teal, #0f766e); outline-offset: 3px; }
.global-chat__unread-dot { position: absolute; top: -2px; right: 0; display: block; width: 12px; height: 12px; border: 2px solid var(--bd-surface, #fff); border-radius: 50%; background: #dc2626; box-shadow: 0 1px 4px rgba(127, 29, 29, 0.25); pointer-events: none; }
@media (max-width: 640px) { .global-chat { right: 12px; bottom: 12px; } .global-chat--open { right: 0; bottom: 0; } .global-chat__launcher .global-chat__label { display: none; } }
</style>
