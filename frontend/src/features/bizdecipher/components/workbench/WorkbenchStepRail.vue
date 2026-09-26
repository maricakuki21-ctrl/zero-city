<template>
  <ol class="wb-step-rail" aria-label="创作工作流进度">
    <li
      v-for="step in steps"
      :key="step.index"
      class="wb-step"
      :class="{ 'is-active': step.index <= activeIndex }"
    >
      <span class="wb-step-dot">{{ step.index }}</span>
      <div>
        <strong>{{ step.label }}</strong>
        <p>{{ step.description }}</p>
      </div>
    </li>
  </ol>
</template>

<script setup lang="ts">
const { activeIndex } = defineProps<{
  activeIndex: number
}>()

const steps = [
  { index: 1, label: '意图', description: '先固定目标、受众与输出格式。' },
  { index: 2, label: '能力', description: '根据预算与时延选择最合适的能力。' },
  { index: 3, label: '运行', description: '追踪排队、执行、取消与恢复状态。' },
  { index: 4, label: '工件', description: '检查产物、费用与失败原因。' },
  { index: 5, label: '保存 / 回放', description: '保存证据，随后一键复用。' },
]
</script>

<style scoped>
.wb-step-rail {
  display: grid;
  gap: 0.8rem;
  list-style: none;
  margin: 0;
  padding: 0;
}

.wb-step {
  display: grid;
  grid-template-columns: auto 1fr;
  gap: 0.75rem;
  align-items: start;
  color: var(--wb-muted);
}

.wb-step.is-active {
  color: var(--wb-ink);
}

.wb-step-dot {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 1.8rem;
  height: 1.8rem;
  border-radius: 999px;
  background: var(--wb-overlay, color-mix(in srgb, var(--module-panel-raised) 9%, transparent));
  border: 1px solid var(--wb-line);
  font-size: 0.8rem;
  font-weight: 800;
}

.wb-step.is-active .wb-step-dot {
  background: color-mix(in srgb, var(--wb-accent) 18%, transparent);
  color: var(--wb-ink-strong);
  border-color: color-mix(in srgb, var(--wb-accent) 40%, transparent);
}

.wb-step strong {
  display: block;
  color: inherit;
}

.wb-step p {
  margin: 0.2rem 0 0;
  font-size: 0.84rem;
  line-height: 1.5;
}
</style>
