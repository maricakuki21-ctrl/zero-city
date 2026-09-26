<script setup lang="ts">
import { reactive, watch } from 'vue'
import Icon from '@/components/icons/Icon.vue'
import type { CapabilityAsset, CapabilityAssetPayload } from '@/features/bizdecipher/api/bizdecipher'
import { assetDraftPayload, createAssetDraftForm, type AssetEditorMode } from './assetDraft'

const props = defineProps<{
  open: boolean
  mode: AssetEditorMode
  asset?: CapabilityAsset | null
  saving: boolean
  notice?: string
  noticeTone?: 'success' | 'warning' | 'error'
}>()

const emit = defineEmits<{
  close: []
  save: [payload: CapabilityAssetPayload]
}>()

const form = reactive(createAssetDraftForm())

watch(
  () => [props.open, props.mode, props.asset] as const,
  ([open]) => {
    if (open) Object.assign(form, createAssetDraftForm(props.mode === 'edit' ? props.asset ?? undefined : undefined))
  },
  { immediate: true },
)

const typeOptions = [
  ['product_app', '产品 / 应用'], ['game', '游戏'], ['workflow', '工作流'], ['agent', 'Agent'],
  ['api_tool', 'API / 工具'], ['plugin_template', '插件 / 模板'], ['prompt_solution', '提示词 / 方案'],
  ['dataset_report', '数据集 / 报告'], ['other', '其他'],
]
const actionOptions = [
  ['view_detail', '查看详情'], ['visit_product', '访问产品'], ['open_demo', '在线体验'],
  ['view_workflow', '查看工作流'], ['open_agent', '打开 Agent'], ['view_docs', '查看文档'],
  ['copy_prompt', '复制提示词'], ['view_report', '查看报告'], ['download_template', '下载模板'],
  ['contact_author', '联系作者'],
]

function submit() {
  emit('save', assetDraftPayload(form, props.mode))
}
</script>

<template>
  <Transition name="asset-editor-fade">
    <div v-if="open" class="asset-editor-overlay" @click.self="emit('close')">
      <section class="asset-editor" role="dialog" aria-modal="true" :aria-labelledby="`asset-editor-title-${mode}`">
        <header>
          <div>
            <span>{{ mode === 'edit' ? `草稿 #${asset?.id}` : '新资产' }}</span>
            <h2 :id="`asset-editor-title-${mode}`">{{ mode === 'edit' ? '编辑能力资产' : '提交能力资产' }}</h2>
          </div>
          <button type="button" class="asset-editor-icon" aria-label="关闭编辑器" :disabled="saving" @click="emit('close')">
            <Icon name="x" size="sm" />
          </button>
        </header>

        <form @submit.prevent="submit">
          <p v-if="notice" class="asset-editor-notice" :data-tone="noticeTone" role="status">{{ notice }}</p>

          <fieldset class="asset-editor-fields" :disabled="saving">
            <label class="asset-editor-wide">资产标题 <span>*</span><input v-model="form.title" required maxlength="160" /></label>
            <label class="asset-editor-wide">一句话简介 <span>*</span><input v-model="form.summary" required maxlength="360" /></label>
            <label>资产类型 <select v-model="form.assetType"><option v-for="option in typeOptions" :key="option[0]" :value="option[0]">{{ option[1] }}</option></select></label>
            <label>定价方式 <select v-model="form.pricingType"><option value="free">免费</option><option value="open_source">开源</option><option value="paid">付费</option><option value="contact">联系作者</option></select></label>
            <label class="asset-editor-wide">详细介绍 <span>*</span><textarea v-model="form.description" required rows="5" /></label>
            <label>标签<input v-model="form.tags" placeholder="AI, 客服, 自动化" /></label>
            <label>适用场景<input v-model="form.scenarioTags" placeholder="企业服务, 个人效率" /></label>
            <label>集成能力<input v-model="form.integrationTags" placeholder="OpenAI, Slack" /></label>
            <label>主操作<select v-model="form.primaryActionType"><option v-for="option in actionOptions" :key="option[0]" :value="option[0]">{{ option[1] }}</option></select></label>
            <label class="asset-editor-wide">封面图 URL<input v-model="form.coverURL" type="url" placeholder="https://..." /></label>
            <label class="asset-editor-wide">截图 URL（逗号分隔）<input v-model="form.screenshotURLs" placeholder="https://..., https://..." /></label>
            <label>演示链接<input v-model="form.demoURL" type="url" placeholder="https://..." /></label>
            <label>视频链接<input v-model="form.videoURL" type="url" placeholder="https://..." /></label>
            <label>文档链接<input v-model="form.docURL" type="url" placeholder="https://..." /></label>
            <label>源码链接<input v-model="form.sourceURL" type="url" placeholder="https://..." /></label>
            <label class="asset-editor-wide">模板链接<input v-model="form.templateURL" type="url" placeholder="https://..." /></label>
            <label class="asset-editor-check"><input v-model="form.contactEnabled" type="checkbox" />允许他人联系我</label>
          </fieldset>

          <footer>
            <div v-if="mode === 'create'" class="asset-editor-status" aria-label="提交状态">
              <label><input v-model="form.status" type="radio" value="draft" :disabled="saving" />保存草稿</label>
              <label><input v-model="form.status" type="radio" value="pending" :disabled="saving" />提交审核</label>
            </div>
            <span v-else>仅草稿可编辑，保存不会提交审核。</span>
            <button type="submit" :disabled="saving">
              <Icon name="edit" size="sm" />{{ saving ? '保存中…' : mode === 'edit' ? '保存草稿' : '确认提交' }}
            </button>
          </footer>
        </form>
      </section>
    </div>
  </Transition>
</template>

<style scoped src="./asset-editor.css"></style>
