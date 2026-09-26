<template>
  <div class="harness-markdown" v-html="rendered" />
</template>

<script setup lang="ts">
import { computed } from 'vue'
import DOMPurify from 'dompurify'
import { marked } from 'marked'

const props = defineProps<{ content: string }>()

const rendered = computed(() => DOMPurify.sanitize(marked.parse(props.content, {
  breaks: true,
  gfm: true,
}) as string))
</script>

<style scoped>
.harness-markdown{padding-left:28px;color:var(--bd-text-primary);font-size:14px;line-height:1.75;overflow-wrap:anywhere}.harness-markdown :deep(> :first-child){margin-top:0}.harness-markdown :deep(> :last-child){margin-bottom:0}.harness-markdown :deep(p),.harness-markdown :deep(ul),.harness-markdown :deep(ol),.harness-markdown :deep(pre),.harness-markdown :deep(blockquote){margin:0 0 14px}.harness-markdown :deep(ul),.harness-markdown :deep(ol){padding-left:22px}.harness-markdown :deep(pre){max-width:100%;overflow:auto;padding:14px;border:1px solid var(--bd-ui-line);border-radius:6px;background:var(--bd-canvas);font:12px/1.65 "JetBrains Mono",SFMono-Regular,Consolas,monospace;tab-size:2}.harness-markdown :deep(code){font:12px/1.65 "JetBrains Mono",SFMono-Regular,Consolas,monospace}.harness-markdown :deep(:not(pre)>code){padding:2px 5px;border-radius:4px;background:var(--bd-canvas)}.harness-markdown :deep(blockquote){padding-left:12px;border-left:2px solid var(--bd-accent-teal);color:var(--bd-text-secondary)}.harness-markdown :deep(a){color:var(--bd-accent-teal);text-decoration:underline;text-underline-offset:2px}.harness-markdown :deep(img){max-width:100%;height:auto}
@media(max-width:767px){.harness-markdown{padding-left:0}.harness-markdown :deep(pre){padding:12px}}
</style>
