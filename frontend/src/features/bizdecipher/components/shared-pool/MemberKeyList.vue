<template>
  <div class="member-key-list">
    <article v-for="entry in grouped" :key="entry.key.api_key_id" class="member-key-row">
      <header><KeyRound :size="18" /><strong>{{ entry.key.name || '共享池 Key' }}</strong><span>{{ entry.key.status === 'active' ? '可用' : entry.key.status === 'disabled' ? '已停用' : '状态待确认' }} · {{ entry.pools.length }} 个池</span></header>
      <div class="member-key-secret">
        <code>{{ revealed.has(entry.key.api_key_id) && entry.key.key ? entry.key.key : entry.key.key_preview && entry.key.key_preview !== entry.key.key ? entry.key.key_preview : '••••••••' }}</code>
        <button :disabled="!entry.key.key" :aria-label="revealed.has(entry.key.api_key_id) ? '隐藏 Key' : '显示 Key'" :title="revealed.has(entry.key.api_key_id) ? '隐藏 Key' : '显示 Key'" @click="toggle(entry.key.api_key_id)"><EyeOff v-if="revealed.has(entry.key.api_key_id)" :size="16" /><Eye v-else :size="16" /></button>
        <button :disabled="!entry.key.key" aria-label="复制完整 Key" title="复制完整 Key" @click="emit('copy', entry.key.key || '')"><Copy :size="16" /></button>
        <button :disabled="deletingId === entry.key.api_key_id" aria-label="删除 Key" title="删除 Key" class="danger" @click="emit('delete', entry.key)"><Trash2 :size="16" /></button>
      </div>
      <p>{{ entry.pools.join(' · ') }}</p>
      <small v-if="!entry.key.key">当前接口仅返回脱敏值，不能复制为调用凭证。</small>
    </article>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { Copy, Eye, EyeOff, KeyRound, Trash2 } from '@lucide/vue'
import type { SharedPoolAccessKey } from '@/features/bizdecipher/api/bizdecipher'
const props = defineProps<{ keys: SharedPoolAccessKey[]; deletingId: number | null }>()
const emit = defineEmits<{ copy: [key: string]; delete: [key: SharedPoolAccessKey] }>()
const revealed = ref(new Set<number>())
watch(() => props.keys, () => { revealed.value = new Set() })
const grouped = computed(() => {
  const groups = new Map<number, { key: SharedPoolAccessKey; pools: string[] }>()
  for (const key of props.keys) {
    const label = key.pool_name || `池 #${key.pool_id}`
    const existing = groups.get(key.api_key_id)
    if (!existing) groups.set(key.api_key_id, { key, pools: [label] })
    else {
      if (!existing.pools.includes(label)) existing.pools.push(label)
      if (!existing.key.key && key.key) existing.key = key
    }
  }
  return [...groups.values()]
})
function toggle(id: number) { const next = new Set(revealed.value); if (next.has(id)) next.delete(id); else next.add(id); revealed.value = next }
</script>

<style scoped>
.member-key-list { display: grid; gap: 16px; }
.member-key-row { padding: 20px; border: 1px solid var(--bd-ui-line); border-radius: 8px; background: var(--bd-surface); min-width: 0; }
header { display: flex; align-items: center; gap: 10px; font-size: 14px; }header strong { flex: 1; overflow-wrap: anywhere; font-weight: 600; }header span { font-size: 12px; color: var(--bd-text-secondary); flex-shrink: 0; }
.member-key-secret { display: flex; align-items: center; gap: 8px; padding: 12px; margin-top: 16px; border-radius: 6px; background: var(--bd-canvas); }
code { flex: 1; min-width: 0; overflow-wrap: anywhere; font-size: 13px; }
button { width: 32px; height: 32px; display: grid; place-items: center; flex-shrink: 0; border-radius: 4px; }button:hover { background: var(--bd-surface); }button:disabled { opacity: .4; cursor: not-allowed; }button:focus-visible { outline: 2px solid var(--bd-accent-teal); }.danger { color: var(--bd-status-danger); }
p,small { color: var(--bd-text-secondary); font-size: 12px; line-height: 1.6; margin: 12px 0 0; overflow-wrap: anywhere; }
@media(max-width: 480px) { .member-key-row { padding: 14px; }.member-key-secret { padding: 8px; gap: 4px; } }
</style>
