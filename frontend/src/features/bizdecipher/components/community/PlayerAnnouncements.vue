<script setup lang="ts">
import { onMounted, onScopeDispose, ref } from 'vue'
import { Megaphone } from '@lucide/vue'
import { communityAPI, type CommunityPoll } from '@/features/bizdecipher/api/community'

const items = ref<CommunityPoll[]>([])
const error = ref(false)
let alive = true
let timer: ReturnType<typeof setInterval> | undefined
async function load() {
  try {
    const response = await communityAPI.listPlayerAnnouncements()
    if (alive) { items.value = response.items; error.value = false }
  } catch { if (alive) { items.value = []; error.value = true } }
}
onMounted(() => { void load(); timer = setInterval(() => void load(), 60000) })
onScopeDispose(() => { alive = false; if (timer) clearInterval(timer) })
</script>

<template>
  <section v-if="items.length || error" class="player-announcements" aria-label="玩家公告">
    <header><Megaphone :size="18" /><strong>玩家公告</strong><span>居民共同选出的声音</span></header>
    <p v-if="error" role="status">玩家公告暂时无法读取。<button type="button" @click="load">重试</button></p>
    <article v-for="item in items" :key="item.id">
      <RouterLink :to="{ path: '/community', query: { district: 'governance', channel: 'votes', poll: item.id } }">{{ item.title }}</RouterLink>
      <p>{{ item.body }}</p>
      <small>{{ item.author }} · {{ item.total_votes }} 人参与 · 支持 {{ item.options.find(option => option.position === 1)?.vote_count || 0 }} 票 · 展示至 {{ item.expires_at ? new Date(item.expires_at).toLocaleDateString() : '' }}</small>
    </article>
  </section>
</template>

<style scoped>
.player-announcements { padding-block: 16px; border-block: 1px solid var(--zc-line); }
header { display: flex; align-items: center; flex-wrap: wrap; gap: 8px; color: var(--bd-accent-teal); }
header span, small { color: var(--bd-text-muted); font-size: 12px; }
article { padding-top: 16px; overflow-wrap: anywhere; }
article a { font-weight: 650; color: var(--bd-text-primary); }
article p { white-space: pre-wrap; color: var(--bd-text-secondary); font-size: 14px; line-height: 1.7; margin-block: 6px; }
button { text-decoration: underline; margin-inline: 8px; }
</style>
