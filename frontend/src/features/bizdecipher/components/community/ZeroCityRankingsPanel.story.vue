<script setup lang="ts">
import { ref } from 'vue'
import ZeroCityRankingsPanel from './ZeroCityRankingsPanel.vue'
import { defaultZeroCityRankingConfig } from '@/features/bizdecipher/data/zeroCityRankings'
import type {
  ZeroCityRankingDefinition,
  ZeroCityRankingRow,
} from '@/features/bizdecipher/data/zeroCityRankings'

const definitions = defaultZeroCityRankingConfig.items

const rowsByKey = {
  contribution: [
    { rank: 1, label: '林默', value: '86 贡献', secondary: '4 条动态 · 18 回复 · 9 条证据' },
    { rank: 2, label: '池主观察站', value: '72 贡献', secondary: '3 条动态 · 12 回复 · 6 条证据' },
    { rank: 3, label: '低电量圣徒', value: '58 贡献', secondary: '2 条动态 · 8 回复 · 4 条证据' },
  ],
  answers: [
    { rank: 1, label: 'Mira Studio', value: '64 回答', secondary: '3 条动态 · 14 回复 · 7 条证据' },
    { rank: 2, label: '读表员', value: '49 回答', secondary: '2 条动态 · 9 回复 · 4 条证据' },
  ],
  assets: [
    { rank: 1, label: '林默', value: '41 复用', secondary: '2 条动态 · 7 回复 · 5 条证据' },
    { rank: 2, label: '夜航船', value: '33 复用', secondary: '2 条动态 · 6 回复 · 3 条证据' },
  ],
  tavern: [
    { rank: 1, label: '夜航船', value: '91 热度', secondary: '3 条动态 · 12 回复 · 4 条证据' },
    { rank: 2, label: '主持人值班台', value: '66 热度', secondary: '2 条动态 · 8 回复 · 3 条证据' },
  ],
} satisfies Readonly<Record<string, readonly ZeroCityRankingRow[]>>

const emptyRows = {
  contribution: [],
} satisfies Readonly<Record<string, readonly ZeroCityRankingRow[]>>

const eventLabel = ref('等待交互')

function handleSelect(row: ZeroCityRankingRow, definition: ZeroCityRankingDefinition | undefined): void {
  eventLabel.value = definition ? `${definition.title} / ${row.label}` : row.label
}

function handleOpen(definition: ZeroCityRankingDefinition): void {
  eventLabel.value = `打开 ${definition.title}`
}
</script>

<template>
  <Story title="Zero City / Rankings Panel">
    <Variant title="Top / compact">
      <div class="story-surface">
        <ZeroCityRankingsPanel
          variant="top"
          :definitions="definitions"
          :rows-by-key="rowsByKey"
          @select="handleSelect"
          @open="handleOpen"
        />
        <output class="story-event">{{ eventLabel }}</output>
      </div>
    </Variant>

    <Variant title="Side / full list">
      <div class="story-surface story-surface--narrow">
        <ZeroCityRankingsPanel
          :definitions="definitions"
          :rows-by-key="rowsByKey"
          @select="handleSelect"
          @open="handleOpen"
        />
      </div>
    </Variant>

    <Variant title="Empty state">
      <div class="story-surface story-surface--narrow">
        <ZeroCityRankingsPanel
          :definitions="definitions.slice(0, 1)"
          :rows-by-key="emptyRows"
        />
      </div>
    </Variant>
  </Story>
</template>

<style scoped>
.story-surface {
  min-width: min(100%, 620px);
  padding: var(--bd-space-6);
  border: 1px solid var(--zc-line);
  background: var(--zc-surface);
  color: var(--zc-text);
}

.story-surface--narrow {
  max-width: 520px;
}

.story-event {
  display: block;
  margin-top: var(--bd-space-4);
  color: var(--zc-muted);
  font-family: 'JetBrains Mono', SFMono-Regular, Consolas, monospace;
  font-size: 0.75rem;
}
</style>
