<template>
  <AppLayout>
    <TablePageLayout>
      <template #filters>
        <div class="flex flex-wrap items-center gap-3">
          <div>
            <h1 class="text-lg font-semibold text-gray-900 dark:text-white">
              {{ t('admin.promoCampaigns.title') }}
            </h1>
            <p class="mt-0.5 text-sm text-gray-500 dark:text-dark-400">
              {{ t('admin.promoCampaigns.description') }}
            </p>
          </div>

          <div class="flex flex-1 flex-wrap items-center justify-end gap-2">
            <button
              @click="loadCampaigns"
              :disabled="loading"
              class="btn btn-secondary"
              :title="t('common.refresh')"
            >
              <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
            </button>
            <button @click="openCreateDialog" class="btn btn-primary">
              <Icon name="plus" size="md" class="mr-1" />
              {{ t('admin.promoCampaigns.create') }}
            </button>
          </div>
        </div>
      </template>

      <template #table>
        <DataTable :columns="columns" :data="campaigns" :loading="loading">
          <template #cell-name="{ row }">
            <div class="min-w-0">
              <div class="flex items-center gap-2">
                <span class="truncate font-medium text-gray-900 dark:text-white">{{ row.title || row.name }}</span>
              </div>
              <div class="mt-1 flex items-center gap-2 text-xs text-gray-500 dark:text-dark-400">
                <span>#{{ row.id }}</span>
                <span class="text-gray-300 dark:text-dark-700">·</span>
                <span>{{ row.name }}</span>
              </div>
            </div>
          </template>

          <template #cell-credit_amount="{ value }">
            <span class="font-mono text-sm text-gray-900 dark:text-white">{{ value }}</span>
          </template>

          <template #cell-target="{ value }">
            <span class="badge badge-gray">
              {{ value === 'new_users' ? t('admin.promoCampaigns.targetNewUsers') : t('admin.promoCampaigns.targetAllUsers') }}
            </span>
          </template>

          <template #cell-window="{ row }">
            <div class="text-sm text-gray-600 dark:text-gray-300">
              <div>{{ row.start_at ? formatDateTime(row.start_at) : t('admin.promoCampaigns.noStart') }}</div>
              <div class="mt-0.5">{{ row.end_at ? formatDateTime(row.end_at) : t('admin.promoCampaigns.noEnd') }}</div>
            </div>
          </template>

          <template #cell-enabled="{ row }">
            <button
              type="button"
              class="badge"
              :class="row.enabled ? 'badge-success' : 'badge-gray'"
              :disabled="togglingId === row.id"
              @click="toggleEnabled(row)"
            >
              {{ row.enabled ? t('admin.promoCampaigns.enabled') : t('admin.promoCampaigns.disabled') }}
            </button>
          </template>

          <template #cell-actions="{ row }">
            <div class="flex items-center space-x-1">
              <button
                @click="openEditDialog(row)"
                class="flex flex-col items-center gap-0.5 rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-gray-100 hover:text-gray-700 dark:hover:bg-dark-600 dark:hover:text-gray-300"
                :title="t('common.edit')"
              >
                <Icon name="edit" size="sm" />
              </button>
            </div>
          </template>

          <template #empty>
            <EmptyState
              :title="t('empty.noData')"
              :description="t('admin.promoCampaigns.emptyDesc')"
              :action-text="t('admin.promoCampaigns.create')"
              @action="openCreateDialog"
            />
          </template>
        </DataTable>
      </template>
    </TablePageLayout>

    <!-- Create/Edit Dialog -->
    <BaseDialog
      :show="showEditDialog"
      :title="isEditing ? t('admin.promoCampaigns.edit') : t('admin.promoCampaigns.create')"
      width="wide"
      @close="closeEdit"
    >
      <form id="promo-campaign-form" @submit.prevent="handleSave" class="space-y-4">
        <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
          <div>
            <label class="input-label">{{ t('admin.promoCampaigns.form.name') }}</label>
            <input v-model="form.name" type="text" class="input" required :placeholder="t('admin.promoCampaigns.form.namePlaceholder')" />
            <p class="input-hint">{{ t('admin.promoCampaigns.form.nameHint') }}</p>
          </div>
          <div>
            <label class="input-label">{{ t('admin.promoCampaigns.form.title') }}</label>
            <input v-model="form.title" type="text" class="input" :placeholder="t('admin.promoCampaigns.form.titlePlaceholder')" />
          </div>
        </div>

        <div>
          <label class="input-label">{{ t('admin.promoCampaigns.form.descriptionLabel') }}</label>
          <textarea v-model="form.description" rows="3" class="input"></textarea>
        </div>

        <div class="grid grid-cols-1 gap-4 md:grid-cols-3">
          <div>
            <label class="input-label">{{ t('admin.promoCampaigns.form.creditAmount') }}</label>
            <input v-model.number="form.credit_amount" type="number" min="0" step="0.01" class="input" required />
          </div>
          <div>
            <label class="input-label">{{ t('admin.promoCampaigns.form.target') }}</label>
            <Select v-model="form.target" :options="targetOptions" />
          </div>
          <div>
            <label class="input-label">{{ t('admin.promoCampaigns.form.status') }}</label>
            <Select v-model="form.enabledStr" :options="enabledOptions" />
          </div>
        </div>

        <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
          <div>
            <label class="input-label">{{ t('admin.promoCampaigns.form.startAt') }}</label>
            <input v-model="form.start_at_str" type="datetime-local" class="input" />
            <p class="input-hint">{{ t('admin.promoCampaigns.form.startAtHint') }}</p>
          </div>
          <div>
            <label class="input-label">{{ t('admin.promoCampaigns.form.endAt') }}</label>
            <input v-model="form.end_at_str" type="datetime-local" class="input" />
            <p class="input-hint">{{ t('admin.promoCampaigns.form.endAtHint') }}</p>
          </div>
        </div>

        <label class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
          <input v-model="form.auto_hide" type="checkbox" class="checkbox" />
          {{ t('admin.promoCampaigns.form.autoHide') }}
        </label>
      </form>

      <template #footer>
        <div class="flex justify-end gap-3">
          <button type="button" @click="closeEdit" class="btn btn-secondary">
            {{ t('common.cancel') }}
          </button>
          <button type="submit" form="promo-campaign-form" :disabled="saving" class="btn btn-primary">
            {{ saving ? t('common.saving') : t('common.save') }}
          </button>
        </div>
      </template>
    </BaseDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { formatDateTime, formatDateTimeLocalInput, parseDateTimeLocalInput } from '@/utils/format'
import {
  adminListPromos,
  adminCreatePromo,
  adminUpdatePromo,
  type PromoCampaign,
  type PromoCampaignPayload
} from '@/api/bizdecipher'
import type { Column } from '@/components/common/types'

import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Select from '@/components/common/Select.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import Icon from '@/components/icons/Icon.vue'

const { t } = useI18n()
const appStore = useAppStore()

const campaigns = ref<PromoCampaign[]>([])
const loading = ref(false)
const togglingId = ref<number | null>(null)

const columns = computed<Column[]>(() => [
  { key: 'name', label: t('admin.promoCampaigns.columns.name') },
  { key: 'credit_amount', label: t('admin.promoCampaigns.columns.creditAmount') },
  { key: 'target', label: t('admin.promoCampaigns.columns.target') },
  { key: 'window', label: t('admin.promoCampaigns.columns.window') },
  { key: 'enabled', label: t('admin.promoCampaigns.columns.status') },
  { key: 'actions', label: t('admin.promoCampaigns.columns.actions') }
])

const targetOptions = computed(() => [
  { value: 'new_users', label: t('admin.promoCampaigns.targetNewUsers') },
  { value: 'all_users', label: t('admin.promoCampaigns.targetAllUsers') }
])

const enabledOptions = computed(() => [
  { value: 'true', label: t('admin.promoCampaigns.enabled') },
  { value: 'false', label: t('admin.promoCampaigns.disabled') }
])

async function loadCampaigns() {
  loading.value = true
  try {
    const res = await adminListPromos()
    campaigns.value = res.items || []
  } catch (error: any) {
    appStore.showError(error?.response?.data?.message || t('admin.promoCampaigns.loadFailed'))
  } finally {
    loading.value = false
  }
}

// ===== Create/Edit dialog =====
const showEditDialog = ref(false)
const saving = ref(false)
const editing = ref<PromoCampaign | null>(null)
const isEditing = computed(() => !!editing.value)

const form = reactive({
  name: '',
  title: '',
  description: '',
  credit_amount: 0,
  target: 'new_users',
  enabledStr: 'true',
  start_at_str: '',
  end_at_str: '',
  auto_hide: true
})

function resetForm() {
  form.name = ''
  form.title = ''
  form.description = ''
  form.credit_amount = 0
  form.target = 'new_users'
  form.enabledStr = 'true'
  form.start_at_str = ''
  form.end_at_str = ''
  form.auto_hide = true
}

function fillForm(c: PromoCampaign) {
  form.name = c.name
  form.title = c.title
  form.description = c.description
  form.credit_amount = c.credit_amount
  form.target = c.target || 'new_users'
  form.enabledStr = c.enabled ? 'true' : 'false'
  form.start_at_str = c.start_at ? formatDateTimeLocalInput(Math.floor(new Date(c.start_at).getTime() / 1000)) : ''
  form.end_at_str = c.end_at ? formatDateTimeLocalInput(Math.floor(new Date(c.end_at).getTime() / 1000)) : ''
  form.auto_hide = c.auto_hide
}

function openCreateDialog() {
  editing.value = null
  resetForm()
  showEditDialog.value = true
}

function openEditDialog(row: PromoCampaign) {
  editing.value = row
  fillForm(row)
  showEditDialog.value = true
}

function closeEdit() {
  showEditDialog.value = false
  editing.value = null
}

function toRFC3339(localStr: string): string | undefined {
  const secs = parseDateTimeLocalInput(localStr)
  if (secs === null) return undefined
  return new Date(secs * 1000).toISOString()
}

function buildPayload(): PromoCampaignPayload {
  return {
    name: form.name.trim(),
    title: form.title.trim(),
    description: form.description.trim(),
    enabled: form.enabledStr === 'true',
    credit_amount: Number(form.credit_amount) || 0,
    target: form.target,
    auto_hide: form.auto_hide,
    start_at: toRFC3339(form.start_at_str),
    end_at: toRFC3339(form.end_at_str)
  }
}

async function handleSave() {
  if (!form.name.trim()) {
    appStore.showError(t('admin.promoCampaigns.form.nameRequired'))
    return
  }
  saving.value = true
  try {
    const payload = buildPayload()
    if (editing.value) {
      await adminUpdatePromo(editing.value.id, payload)
    } else {
      await adminCreatePromo(payload)
    }
    appStore.showSuccess(t('common.success'))
    closeEdit()
    await loadCampaigns()
  } catch (error: any) {
    appStore.showError(error?.response?.data?.message || t('admin.promoCampaigns.saveFailed'))
  } finally {
    saving.value = false
  }
}

// ===== Quick enable/disable toggle =====
async function toggleEnabled(row: PromoCampaign) {
  togglingId.value = row.id
  try {
    await adminUpdatePromo(row.id, {
      name: row.name,
      title: row.title,
      description: row.description,
      enabled: !row.enabled,
      credit_amount: row.credit_amount,
      target: row.target,
      auto_hide: row.auto_hide,
      start_at: row.start_at || undefined,
      end_at: row.end_at || undefined
    })
    row.enabled = !row.enabled
    appStore.showSuccess(t('common.success'))
  } catch (error: any) {
    appStore.showError(error?.response?.data?.message || t('admin.promoCampaigns.saveFailed'))
  } finally {
    togglingId.value = null
  }
}

onMounted(loadCampaigns)
</script>
