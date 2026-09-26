<script setup lang="ts">
import { onScopeDispose, ref, watch } from 'vue'
import { Send } from '@lucide/vue'
import { useAuthStore } from '@/stores/auth'
import { communityAPI, type CommunityComment } from '@/features/bizdecipher/api/community'
import { extractActionableApiErrorMessage } from '@/utils/apiError'
const props = defineProps<{ postId: number }>()
const auth = useAuthStore()
const comments = ref<CommunityComment[]>([])
const draft = ref('')
const error = ref('')
const loading = ref(false)
const saving = ref(false)
let epoch = 0
async function load() {
  const request = ++epoch
  loading.value = true
  error.value = ''
  try {
    const result = await communityAPI.listComments(props.postId, 100)
    if (request === epoch) comments.value = result.items
  } catch (value) {
    if (request === epoch) error.value = extractActionableApiErrorMessage(value, '讨论加载失败，请重试。')
  } finally { if (request === epoch) loading.value = false }
}
async function send() {
  if (!auth.isAuthenticated || !draft.value.trim() || saving.value) return
  const request = epoch
  saving.value = true
  error.value = ''
  try {
    const result = await communityAPI.createComment(props.postId, { body: draft.value.trim(), helper_role: 'resident' })
    if (request === epoch) { comments.value.push(result); draft.value = '' }
  } catch (value) {
    if (request === epoch) error.value = extractActionableApiErrorMessage(value, '回复失败，请重试。')
  } finally { if (request === epoch) saving.value = false }
}
watch(() => [props.postId, auth.user?.id], () => { comments.value = []; draft.value = ''; saving.value = false; void load() }, { immediate: true })
onScopeDispose(() => { epoch++ })
</script>

<template>
  <section class="poll-discussion" aria-label="议题讨论">
    <p v-if="loading" role="status">正在读取讨论…</p>
    <p v-if="error" role="alert">{{ error }} <button type="button" :disabled="saving" @click="load">重试</button></p>
    <article v-for="comment in comments" :key="comment.id"><strong>{{ comment.author }}</strong><p>{{ comment.body }}</p></article>
    <p v-if="!loading && !error && !comments.length">还没有讨论，来说说你的理由。</p>
    <form v-if="auth.isAuthenticated" @submit.prevent="send">
      <label :for="`poll-discussion-${postId}`">参与讨论</label>
      <textarea :id="`poll-discussion-${postId}`" v-model="draft" maxlength="3000" :disabled="saving" rows="3"></textarea>
      <button class="city-activity-primary" type="submit" :disabled="loading || saving || !draft.trim()"><Send :size="16" />{{ saving ? '发送中…' : '回复' }}</button>
    </form>
  </section>
</template>

<style scoped>
.poll-discussion { padding-top: 12px; color: var(--bd-text-secondary); font-size: 14px; }
article { border-bottom: 1px solid var(--zc-line); padding-block: 12px; }
article p { white-space: pre-wrap; overflow-wrap: anywhere; }
form { margin-top: 12px; }
textarea { width: 100%; border: 1px solid var(--zc-line); background: var(--bd-surface); color: var(--bd-text-primary); border-radius: 6px; padding: 10px; resize: vertical; }
form button { justify-self: end; }
</style>
