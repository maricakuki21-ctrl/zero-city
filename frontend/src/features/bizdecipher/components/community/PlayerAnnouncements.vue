<script setup lang="ts">
import { computed, onMounted, onScopeDispose, ref } from 'vue'
import { ArrowUpRight, Megaphone } from '@lucide/vue'
import { communityAPI, type CommunityPoll } from '@/features/bizdecipher/api/community'

const items = ref<CommunityPoll[]>([])
const error = ref(false)
const expanded = ref(false)
const visibleItems = computed(() => expanded.value ? items.value : items.value.slice(0, 2))
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
    <header><Megaphone :size="18" /><strong>玩家公告</strong><span>居民共同选出的声音</span><RouterLink class="proposal-link" to="/community?district=governance&channel=votes">参与城市议题<ArrowUpRight :size="15" /></RouterLink></header>
    <p v-if="error" role="status">玩家公告暂时无法读取。<button type="button" @click="load">重试</button></p>
    <article v-for="item in visibleItems" :key="item.id">
      <RouterLink :to="{ path: '/community', query: { district: 'governance', channel: 'votes', poll: item.id } }">{{ item.title }}</RouterLink>
      <p>{{ item.body }}</p>
      <small>{{ item.author }} · {{ item.total_votes }} 人参与 · 支持 {{ item.options.find(option => option.position === 1)?.vote_count || 0 }} 票 · 展示至 {{ item.expires_at ? new Date(item.expires_at).toLocaleDateString() : '' }}</small>
    </article>
    <button v-if="items.length > 2" class="show-more" type="button" :aria-expanded="expanded" @click="expanded = !expanded">{{ expanded ? '收起公告' : `查看全部 ${items.length} 条公告` }}</button>
  </section>
</template>

<style scoped>
.player-announcements { padding-block: 16px; border-block: 1px solid var(--zc-line); }
header { display: flex; align-items: center; flex-wrap: wrap; gap: 8px; color: var(--bd-accent-teal); }
header span, small { color: var(--bd-text-muted); font-size: 12px; }
.proposal-link { margin-left: auto; display: inline-flex; align-items: center; gap: 4px; font-size: 12px; }
article { padding-block: 12px; overflow-wrap: anywhere; }
article + article { border-top: 1px solid var(--zc-line); }
article a { font-weight: 650; color: var(--bd-text-primary); }
article p { display: -webkit-box; -webkit-line-clamp: 2; -webkit-box-orient: vertical; overflow: hidden; white-space: pre-wrap; color: var(--bd-text-secondary); font-size: 14px; line-height: 1.7; margin-block: 6px; }
button { text-decoration: underline; margin-inline: 8px; }
.show-more { color: var(--bd-accent-teal); font-size: 13px; margin: 8px 0 0; }
</style>
