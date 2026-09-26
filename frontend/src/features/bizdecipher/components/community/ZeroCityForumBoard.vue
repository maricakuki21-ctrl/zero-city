<script setup lang="ts" generic="T extends ForumTopic">
import { computed, nextTick, ref } from 'vue'
import { ArrowLeft, MessageSquare, Search, SquarePen, RotateCw, Pin, X } from '@lucide/vue'
import type { ForumTopic } from './forumTopic'

const props = defineProps<{
  title: string
  district: string
  posts: T[]
  loading: boolean
  error: boolean
  excerpts?: boolean
  channels?: readonly { key: string; label: string }[]
  activeChannel?: string
  participationNote?: string
  canCreate?: boolean
}>()
const emit = defineEmits<{
  create: []
  retry: []
  open: [post: T]
  profile: [userId: number]
  channel: [key: string]
}>()
defineSlots<{ detail(props: { post: T }): unknown }>()

const search = ref('')
const sort = ref('latest')
const selectedKey = ref<string | null>(null)
const root = ref<HTMLElement>()
const modes = [
  { key: 'latest', label: '最新' },
  { key: 'hot', label: '热议' },
  { key: 'unanswered', label: '未回复' },
]
const keyOf = (post: T) => String(post.id ?? `${post.title}:${post.time}`)
const selectedPost = computed(() => props.posts.find(post => keyOf(post) === selectedKey.value))
const results = computed(() => {
  const query = search.value.trim().toLocaleLowerCase()
  return props.posts.filter(post =>
    (sort.value !== 'unanswered' || post.replies === 0) &&
    (!query || [post.title, post.card, post.body || post.excerpt, ...(post.tags || [])]
      .join(' ').toLocaleLowerCase().includes(query)),
  ).slice().sort((a, b) => {
    if (Boolean(a.pinned) !== Boolean(b.pinned)) return a.pinned ? -1 : 1
    if (sort.value === 'hot' && a.replies !== b.replies) return b.replies - a.replies
    return (Date.parse(b.created_at || '') || 0) - (Date.parse(a.created_at || '') || 0)
  })
})

async function open(post: T) {
  selectedKey.value = keyOf(post)
  emit('open', post)
  await nextTick()
  root.value?.querySelector<HTMLElement>('.city-forum-detail-title')?.focus()
}

async function back() {
  const previous = selectedKey.value
  selectedKey.value = null
  await nextTick()
  const buttons = root.value?.querySelectorAll<HTMLButtonElement>('[data-topic]')
  const target = Array.from(buttons || []).find(button => button.dataset.topic === previous)
  target?.focus()
}

function changeChannel(event: Event) {
  if (event.target instanceof HTMLSelectElement) emit('channel', event.target.value)
}
</script>

<template>
  <section ref="root" class="city-forum" :aria-label="title">
    <template v-if="selectedKey !== null">
      <div class="city-forum-detail-bar">
        <button class="city-forum-back" type="button" @click="back"><ArrowLeft :size="18" /> 返回{{ title }}</button>
        <span>{{ district }}</span>
      </div>
      <template v-if="selectedPost">
        <h1 class="city-forum-detail-title" tabindex="-1">{{ selectedPost.title }}</h1>
        <slot name="detail" :post="selectedPost" />
      </template>
      <div v-else class="city-forum-state" role="status">这条帖子不在当前列表中。返回列表后可重新加载。</div>
    </template>
    <template v-else>
      <header class="city-forum-header">
        <div>
          <p class="city-forum-breadcrumb">零号城<span v-if="district !== title"> / {{ district }}</span></p>
          <h1>{{ title }}</h1>
        </div>
        <button class="city-forum-primary" type="button" @click="emit('create')"><SquarePen :size="18" /> {{ canCreate === false ? '参与资格' : '发帖' }}</button>
      </header>
      <p v-if="participationNote" class="city-forum-participation" role="status">{{ participationNote }}</p>
      <div class="city-forum-toolbar">
        <div class="city-forum-modes" role="group" aria-label="帖子排序">
          <button v-for="mode in modes" :key="mode.key" type="button" :aria-pressed="sort === mode.key" @click="sort = mode.key">{{ mode.label }}</button>
        </div>
        <select v-if="channels?.length" class="city-forum-channels" aria-label="广场频道" :value="activeChannel" @change="changeChannel">
          <option v-for="channel in channels" :key="channel.key" :value="channel.key">{{ channel.label }}</option>
        </select>
        <label class="city-forum-search">
          <Search :size="16" aria-hidden="true" />
          <input v-model="search" type="search" aria-label="搜索已加载帖子" placeholder="搜索本页帖子" />
        </label>
        <button class="city-forum-icon" type="button" aria-label="刷新帖子" title="刷新帖子" :disabled="loading" @click="emit('retry')"><RotateCw :size="18" /></button>
      </div>
      <div v-if="loading" class="city-forum-state" role="status">正在加载帖子…</div>
      <div v-else-if="error" class="city-forum-state" role="alert">
        <MessageSquare :size="28" /><h2>帖子暂时没有加载成功</h2>
        <button class="city-forum-back" type="button" @click="emit('retry')">重新加载</button>
      </div>
      <div v-else-if="!posts.length" class="city-forum-state">
        <MessageSquare :size="28" /><h2>还没有帖子</h2><p>{{ canCreate === false ? '这里的讨论即将开始。你也可以先去闲聊广场认识其他居民。' : '聊个近况，分享发现，或者问一个问题。' }}</p>
        <button class="city-forum-primary" type="button" @click="emit('create')"><SquarePen :size="16" /> {{ canCreate === false ? '查看参与资格' : '发第一帖' }}</button>
      </div>
      <div v-else-if="!results.length" class="city-forum-state" role="status">
        <h2>{{ search ? '没有找到匹配的帖子' : '当前没有未回复的帖子' }}</h2>
        <button class="city-forum-back" type="button" @click="search = ''; sort = 'latest'"><X :size="16" /> 清除筛选</button>
      </div>
      <template v-else>
        <div class="city-forum-columns"><span>主题</span><span>回复 / 浏览</span></div>
        <div class="city-forum-topics">
          <article v-for="post in results" :key="keyOf(post)" class="city-forum-row">
            <div class="city-forum-avatar" aria-hidden="true">{{ Array.from(post.card || '居民')[0] }}</div>
            <div class="city-forum-topic">
              <button class="city-forum-topic-title" :data-topic="keyOf(post)" type="button" @click="open(post)">
                <Pin v-if="post.pinned" :size="14" aria-label="置顶" /><span>{{ post.title }}</span>
              </button>
              <p v-if="excerpts" class="city-forum-excerpt">{{ post.excerpt }}</p>
              <div class="city-forum-meta">
                <button v-if="post.user_id" type="button" @click="emit('profile', post.user_id)">{{ post.card || '居民' }}</button>
                <span v-else>{{ post.card || '居民' }}</span>
                <time :datetime="post.created_at">{{ post.time }}</time>
                <span v-for="tag in (post.tags || []).filter(tag => tag !== title).slice(0, 2)" :key="tag" class="city-forum-tag">{{ tag }}</span>
              </div>
            </div>
            <div class="city-forum-stats"><strong>{{ post.replies }}<small> 回复</small></strong><span>{{ post.views }} 浏览</span></div>
          </article>
        </div>
        <p class="city-forum-count">当前加载 {{ posts.length }} 帖<span v-if="results.length !== posts.length"> · 显示 {{ results.length }} 帖</span></p>
      </template>
    </template>
  </section>
</template>

<style src="./zero-city-forum.css"></style>
