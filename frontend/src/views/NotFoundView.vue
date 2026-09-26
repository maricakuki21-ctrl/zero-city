<template>
  <div class="not-found-page">
    <div class="zero-grid" aria-hidden="true"></div>

    <main class="not-found-card">
      <div class="mascot-shell">
        <span class="orbit orbit-a"></span>
        <span class="orbit orbit-b"></span>
        <img class="mascot" :src="zeroCityMascots.repairSupport" alt="Zero City Repair Support" />
      </div>

      <div class="copy">
        <p class="eyebrow">Zero City · 404</p>
        <h1>{{ t('errors.pageNotFound') }}</h1>
        <p class="description">这条巷子还没有开放，小维修员已经在查线路。你可以返回上一页，或回到工作台继续使用主产品。</p>
        <div class="actions">
          <button class="btn-secondary" type="button" @click="goBack">
            <Icon name="arrowLeft" size="md" class="mr-2" />
            Go Back
          </button>
          <router-link to="/dashboard" class="btn-primary">
            <Icon name="home" size="md" class="mr-2" />
            Go to Dashboard
          </router-link>
        </div>
      </div>
    </main>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import Icon from '@/components/icons/Icon.vue'
import { zeroCityMascots } from '@/constants/zeroCityMascots'

const { t } = useI18n()
const router = useRouter()

function goBack(): void {
  router.back()
}
</script>

<style scoped>
.not-found-page {
  position: relative;
  display: flex;
  min-height: 100vh;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  background: var(--bd-canvas, #f4f6f8);
  padding: 1.5rem;
  color: var(--bd-text-primary, #20262e);
}

.zero-grid {
  position: absolute;
  inset: 0;
  background:
    linear-gradient(90deg, color-mix(in srgb, var(--bd-text-primary, #20262e) 6%, transparent) 1px, transparent 1px),
    linear-gradient(180deg, color-mix(in srgb, var(--bd-text-primary, #20262e) 4%, transparent) 1px, transparent 1px),
    radial-gradient(circle at 1px 1px, color-mix(in srgb, var(--bd-text-primary, #20262e) 8%, transparent) 1px, transparent 0);
  background-size: 54px 54px, 54px 54px, 18px 18px;
  opacity: 0.9;
}

.not-found-card {
  position: relative;
  z-index: 1;
  display: grid;
  width: min(100%, 980px);
  grid-template-columns: 0.88fr 1.12fr;
  gap: 1.5rem;
  align-items: center;
  border: 1px solid var(--bd-ui-line, #dce1e6);
  border-radius: 12px;
  background: var(--bd-surface, #ffffff);
  padding: 1.35rem;
}

.mascot-shell {
  position: relative;
  display: grid;
  min-height: 360px;
  place-items: center;
  overflow: hidden;
  border: 1px solid var(--bd-ui-line, #dce1e6);
  border-radius: 8px;
  background: var(--bd-surface-raised, #ffffff);
}

.orbit {
  position: absolute;
  border: 1px dashed color-mix(in srgb, var(--bd-accent-teal, #147d70) 28%, transparent);
  border-radius: 999px;
}

.orbit-a {
  height: 74%;
  width: 74%;
  transform: rotate(-14deg);
}

.orbit-b {
  height: 46%;
  width: 92%;
  transform: rotate(18deg);
  border-color: color-mix(in srgb, var(--bd-accent-blue, #356aca) 30%, transparent);
}

.mascot {
  position: relative;
  z-index: 1;
  width: min(78%, 330px);
  max-height: 330px;
  object-fit: contain;
  filter: drop-shadow(0 12px 20px color-mix(in srgb, var(--bd-text-primary, #20262e) 24%, transparent));
}

.copy {
  padding: 2rem 1.5rem;
}

.eyebrow {
  display: inline-flex;
  border-radius: 999px;
  background: color-mix(in srgb, var(--bd-accent-teal, #147d70) 14%, transparent);
  padding: 0.45rem 0.75rem;
  color: var(--bd-accent-teal, #147d70);
  font-size: 0.75rem;
  font-weight: 800;
  letter-spacing: 0.16em;
  text-transform: uppercase;
}

h1 {
  margin-top: 1.2rem;
  color: var(--bd-text-primary, #20262e);
  font-size: clamp(2.6rem, 7vw, 5.8rem);
  font-weight: 900;
  letter-spacing: 0;
  line-height: 0.92;
}

.description {
  margin-top: 1.15rem;
  max-width: 34rem;
  color: var(--bd-text-secondary, #586473);
  font-size: 1rem;
  font-weight: 500;
  line-height: 1.85;
}

.actions {
  margin-top: 1.75rem;
  display: flex;
  flex-wrap: wrap;
  gap: 0.75rem;
}

.btn-primary,
.btn-secondary {
  display: inline-flex;
  min-height: 2.85rem;
  align-items: center;
  justify-content: center;
  border-radius: 8px;
  padding: 0 1rem;
  font-size: 0.9rem;
  font-weight: 800;
  transition: transform 0.18s ease, box-shadow 0.18s ease;
}

.btn-primary:hover,
.btn-secondary:hover {
  transform: translateY(-1px);
}

.btn-primary {
  background: var(--bd-accent-teal, #147d70);
  color: #ffffff;
  box-shadow: 0 10px 24px color-mix(in srgb, var(--bd-accent-teal, #147d70) 28%, transparent);
}

.btn-secondary {
  border: 1px solid var(--bd-ui-line, #dce1e6);
  background: var(--bd-surface-raised, #ffffff);
  color: var(--bd-text-primary, #20262e);
}

@media (max-width: 840px) {
  .not-found-card {
    grid-template-columns: 1fr;
  }

  .mascot-shell {
    min-height: 280px;
  }

  .copy {
    padding: 1.25rem 0.5rem 0.75rem;
  }
}
</style>
