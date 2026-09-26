<script setup lang="ts">
import { computed, ref } from 'vue'
import { Award, ArrowRight, ClipboardList, RefreshCw, ScrollText, Vote } from '@lucide/vue'
import type { ForumTopic } from './forumTopic'

type ActivityKind = 'votes' | 'badges' | 'rules'

const props = defineProps<{
  kind: ActivityKind
  title: string
  posts: readonly ForumTopic[]
  loading: boolean
  error: boolean
}>()

const emit = defineEmits<{
  create: []
  retry: []
  cards: []
}>()

const copy = {
  votes: {
    eyebrow: '城市共识',
    description: '把需要大家决定的事情放在这里，先看议题，再留下你的理由。',
    emptyTitle: '暂无开放议题',
    emptyNote: '真正的投票接口接入后，议题、选项和截止时间会出现在这里。',
    action: '提交议题',
    icon: Vote,
  },
  badges: {
    eyebrow: '身份与贡献',
    description: '勋章记录的是已经发生的贡献，不是预先填满的装饰墙。',
    emptyTitle: '还没有可验证的授予记录',
    emptyNote: '勋章授予服务接入后，这里会显示获得原因、时间和对应经历。',
    action: '查看收藏卡册',
    icon: Award,
  },
  rules: {
    eyebrow: '公开档案',
    description: '城里的约定、变更和处理结果，按时间留下可回看的记录。',
    emptyTitle: '暂无已发布公示',
    emptyNote: '没有真实公示时保持空白，不用普通帖子占位。',
    action: '',
    icon: ScrollText,
  },
} as const

const current = () => copy[props.kind]
const keyword = ref('')
const visiblePosts = computed(() => {
  const query = keyword.value.trim().toLocaleLowerCase()
  return props.posts.filter(post => !query || [post.title, post.card, post.body, post.excerpt, ...(post.tags || [])]
    .join(' ').toLocaleLowerCase().includes(query))
})

function publishedAt(post: ForumTopic): string {
  if (!post.created_at) return post.time || '时间未记录'
  const date = new Date(post.created_at)
  return Number.isNaN(date.getTime()) ? '时间未记录' : date.toLocaleString('zh-CN', { hour12: false })
}
</script>

<template>
  <section class="city-activity-board" :class="`city-activity-board--${kind}`" :aria-label="title">
    <header class="city-activity-header">
      <div class="city-activity-heading">
        <span class="city-activity-icon" aria-hidden="true"><component :is="current().icon" :size="22" /></span>
        <div>
          <p class="city-activity-eyebrow">{{ current().eyebrow }}</p>
          <h1>{{ title }}</h1>
          <p>{{ current().description }}</p>
        </div>
      </div>
      <div class="city-activity-actions">
        <button class="city-activity-refresh" type="button" aria-label="刷新城市活动" title="刷新" :disabled="loading" @click="emit('retry')"><RefreshCw :size="18" /></button>
        <button v-if="kind === 'votes'" class="city-activity-primary" type="button" @click="emit('create')"><ClipboardList :size="17" /> {{ current().action }}</button>
        <button v-else-if="kind === 'badges'" class="city-activity-secondary" type="button" @click="emit('cards')">{{ current().action }} <ArrowRight :size="16" /></button>
      </div>
    </header>

    <div v-if="loading" class="city-activity-state" role="status">正在读取真实记录…</div>
    <div v-else-if="error" class="city-activity-state city-activity-state--error" role="alert">
      <h2>记录暂时没有加载成功</h2>
      <button class="city-activity-secondary" type="button" @click="emit('retry')">重新读取</button>
    </div>
    <div v-else-if="posts.length" class="city-activity-records">
      <p class="city-activity-records-label">{{ kind === 'badges' ? '关联贡献记录' : kind === 'rules' ? '已发布记录' : '开放议题' }}</p>
      <p v-if="kind === 'badges'" class="activity-record-boundary">以下为贡献记录，不代表已获勋章或已开通权限。</p>
      <label class="activity-record-search">查找记录<input v-model="keyword" type="search" placeholder="标题、发布者或关键词" /></label>
      <p class="activity-record-count" role="status">当前已加载 {{ posts.length }} 条，匹配 {{ visiblePosts.length }} 条</p>
      <div v-if="!visiblePosts.length" class="city-activity-state">
        <h2>没有匹配的记录</h2><button class="city-activity-secondary" type="button" @click="keyword = ''">清空查找</button>
      </div>
      <details v-for="post in visiblePosts" :key="post.id || post.title" class="city-activity-record">
        <summary>
          <span class="city-activity-record-mark" aria-hidden="true"><component :is="current().icon" :size="17" /></span>
          <span class="city-activity-record-main"><strong>{{ post.title }}</strong><small>{{ post.card }} · {{ post.time }}</small></span>
          <span class="city-activity-record-meta">{{ post.replies }} 回复</span>
        </summary>
        <div class="city-activity-record-body">
          <dl class="activity-record-source">
            <div><dt>发布者</dt><dd>{{ post.card || '未记录' }}</dd></div>
            <div><dt>发布时间</dt><dd><time>{{ publishedAt(post) }}</time></dd></div>
            <div v-if="post.id"><dt>来源记录</dt><dd>社区记录 #{{ post.id }}</dd></div>
          </dl>
          <p>{{ post.body || post.excerpt }}</p>
          <div v-if="post.tags?.length" class="city-activity-tags">
            <span v-for="tag in post.tags.slice(0, 4)" :key="tag">{{ tag }}</span>
          </div>
        </div>
      </details>
    </div>
    <div v-else class="city-activity-state">
      <component :is="current().icon" :size="30" aria-hidden="true" />
      <h2>{{ current().emptyTitle }}</h2>
      <p>{{ current().emptyNote }}</p>
      <button v-if="kind === 'votes'" class="city-activity-primary" type="button" @click="emit('create')"><ClipboardList :size="17" /> {{ current().action }}</button>
      <button v-else-if="kind === 'badges'" class="city-activity-secondary" type="button" @click="emit('cards')">{{ current().action }} <ArrowRight :size="16" /></button>
    </div>
  </section>
</template>

<style src="./zero-city-activity.css"></style>
<style scoped>
.activity-record-search { display: grid; gap: 8px; margin: 16px 0 8px; font-size: 13px; color: var(--bd-text-secondary); }
.activity-record-search input { width: 100%; min-height: 40px; padding: 8px 12px; border: 1px solid var(--bd-ui-line); border-radius: 6px; color: var(--bd-text-primary); background: var(--bd-surface); }
.activity-record-search input:focus-visible { outline: 2px solid var(--bd-accent-teal); outline-offset: 2px; }
.activity-record-count, .activity-record-boundary { font-size: 12px; line-height: 1.7; color: var(--bd-text-secondary); }
.activity-record-source { display: flex; flex-wrap: wrap; gap: 12px 24px; margin: 0 0 16px; padding-bottom: 12px; border-bottom: 1px solid var(--bd-ui-line); font-size: 12px; }
.activity-record-source div { min-width: 0; display: grid; gap: 4px; }
.activity-record-source dt { color: var(--bd-text-secondary); }
.activity-record-source dd { margin: 0; overflow-wrap: anywhere; }
</style>
