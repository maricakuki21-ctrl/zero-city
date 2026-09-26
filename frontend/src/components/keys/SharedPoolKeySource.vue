<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { RefreshCw } from '@lucide/vue'
import { listMySeats, type PoolSeat } from '@/features/bizdecipher/api/bizdecipher'
import { extractActionableApiErrorMessage } from '@/utils/apiError'

const props = defineProps<{ modelValue: number | null; busy?: boolean }>()
const emit = defineEmits<{ 'update:modelValue': [id: number | null] }>()
const seats = ref<PoolSeat[]>([])
const loading = ref(false)
const error = ref('')
const query = ref('')
const visible = computed(() => seats.value.filter(s => s.pool_name.toLocaleLowerCase().includes(query.value.trim().toLocaleLowerCase())))
async function load() {
  loading.value = true
  error.value = ''
  try {
    seats.value = (await listMySeats()).filter(s => s.status === 'active')
    if (!seats.value.some(s => s.pool_id === props.modelValue)) emit('update:modelValue', null)
  } catch (cause) {
    seats.value = []
    emit('update:modelValue', null)
    error.value = extractActionableApiErrorMessage(cause, '共享池加载失败')
  } finally {
    loading.value = false
  }
}
onMounted(load)
</script>

<template>
  <section class="space-y-3" aria-label="共享池资源">
    <div class="flex items-center gap-2">
      <input v-model="query" type="search" class="input min-w-0 flex-1" placeholder="搜索已加入的共享池" aria-label="搜索已加入的共享池" />
      <button type="button" class="btn btn-secondary shrink-0" :disabled="loading || busy" aria-label="刷新共享池" title="刷新共享池" @click="load"><RefreshCw :size="16" /></button>
    </div>
    <p v-if="loading" role="status" class="input-hint">正在读取共享池…</p>
    <p v-else-if="error" role="alert" class="text-sm text-red-600">{{ error }}</p>
    <fieldset v-else class="max-h-64 space-y-1 overflow-y-auto" :disabled="busy">
      <legend class="sr-only">选择共享池</legend>
      <label v-for="seat in visible" :key="seat.id" class="flex cursor-pointer items-center gap-3 rounded-md border px-3 py-3" :class="modelValue === seat.pool_id ? 'border-primary-500 bg-primary-50 dark:bg-primary-950' : 'border-gray-200 dark:border-gray-700'">
        <input type="radio" name="shared-pool-source" :value="seat.pool_id" :checked="modelValue === seat.pool_id" @change="emit('update:modelValue', seat.pool_id)" />
        <span class="min-w-0 flex-1 break-words text-sm font-medium">{{ seat.pool_name }}</span>
        <span class="shrink-0 text-xs text-gray-500">已加入</span>
      </label>
      <p v-if="!visible.length" class="py-4 text-sm text-gray-500">{{ seats.length ? '没有匹配的共享池' : '还没有已加入的共享池' }}</p>
    </fieldset>
    <RouterLink to="/account-square" class="inline-block text-sm text-primary-600">浏览共享市场</RouterLink>
  </section>
</template>
