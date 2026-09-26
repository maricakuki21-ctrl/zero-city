<template>
  <AppLayout>
    <div class="docs-page space-y-6">
      <section class="docs-hero relative overflow-hidden rounded-[2rem] p-6 lg:p-8">
        <div class="docs-hero-grid pointer-events-none absolute inset-0"></div>
        <div class="relative grid gap-8 lg:grid-cols-[1fr_0.85fr] lg:items-center">
          <div>
            <p class="docs-kicker text-xs font-bold uppercase tracking-[0.24em]">BizDecipher Docs</p>
            <h1 class="docs-hero-title mt-3 text-4xl font-bold tracking-[-0.04em] md:text-5xl">{{ t('docsPage.hero.title') }}</h1>
            <p class="docs-desc mt-4 max-w-2xl text-base leading-8">
              {{ t('docsPage.hero.description') }}
            </p>
            <div class="mt-6 flex flex-wrap gap-3">
              <router-link to="/keys" class="btn btn-primary">{{ t('docsPage.hero.createKey') }}</router-link>
              <router-link to="/account-square" class="btn btn-secondary">{{ t('docsPage.hero.accountSquare') }}</router-link>
            </div>
          </div>
          <div class="docs-code-card rounded-[1.5rem] p-5 font-mono text-sm">
            <div class="docs-code-label">Base URL</div>
            <div class="docs-code-accent mt-2">https://bizdecipher.com/v1</div>
            <div class="docs-code-label mt-5">Authorization</div>
            <div class="docs-code-text mt-2">Bearer sk-... / sk-share-...</div>
            <div class="docs-code-accent mt-5">{{ t('docsPage.hero.codeNote') }}</div>
          </div>
        </div>
      </section>

      <section class="grid gap-4 lg:grid-cols-4">
        <a v-for="item in toc" :key="item.href" :href="item.href" class="docs-toc-card rounded-3xl p-5 transition hover:-translate-y-0.5">
          <div class="docs-card-title text-sm font-bold">{{ item.title }}</div>
          <p class="docs-desc mt-2 text-sm leading-6">{{ item.desc }}</p>
        </a>
      </section>

      <section id="workspace" class="panel p-6 lg:p-8">
        <p class="docs-kicker text-xs font-bold uppercase tracking-[0.22em]">Workspace</p>
        <h2 class="docs-section-title mt-3 text-2xl font-bold">{{ t('docsPage.workspace.title') }}</h2>
        <div class="mt-5 grid gap-4 md:grid-cols-3">
          <article v-for="card in workspaceCards" :key="card.title" class="docs-info-card rounded-3xl p-5">
            <h3 class="docs-card-title font-semibold">{{ card.title }}</h3>
            <p class="docs-desc mt-2 text-sm leading-7">{{ card.desc }}</p>
          </article>
        </div>
      </section>

      <section id="service-entry" class="panel p-6 lg:p-8">
        <p class="docs-kicker text-xs font-bold uppercase tracking-[0.22em]">AI Gateway</p>
        <h2 class="docs-section-title mt-3 text-2xl font-bold">{{ t('docsPage.gateway.title') }}</h2>
        <p class="docs-desc mt-3 max-w-3xl leading-8">
          {{ t('docsPage.gateway.description') }}
        </p>
        <div class="mt-5 grid gap-4 lg:grid-cols-2">
          <div class="docs-info-card rounded-3xl p-5">
            <h3 class="docs-card-title font-semibold">{{ t('docsPage.gateway.normalKeyTitle') }}</h3>
            <ol class="docs-desc mt-3 list-decimal space-y-2 pl-5 text-sm leading-7">
              <li>{{ t('docsPage.gateway.stepCreatePrefix') }} <router-link class="docs-inline-link font-semibold" to="/keys">API Keys</router-link> {{ t('docsPage.gateway.stepCreateSuffix') }}</li>
              <li>{{ t('docsPage.gateway.stepBasePrefix') }} <code class="docs-inline-code rounded px-1 py-0.5">https://bizdecipher.com/v1</code>{{ t('docsPage.gateway.stepBaseSuffix') }}</li>
              <li>{{ t('docsPage.gateway.stepRequestPrefix') }} <code class="docs-inline-code rounded px-1 py-0.5">/chat/completions</code>{{ t('docsPage.gateway.stepRequestSuffix') }}</li>
            </ol>
          </div>
          <div class="docs-code-card rounded-3xl p-5 font-mono text-sm">
            <div class="docs-code-text">curl https://bizdecipher.com/v1/chat/completions \</div>
            <div class="docs-code-label mt-2">  -H "Authorization: Bearer $BIZDECIPHER_KEY" \</div>
            <div class="docs-code-label mt-2">  -H "Content-Type: application/json" \</div>
            <div class="docs-code-label mt-2">  -d '{"model":"gpt-4o-mini","messages":[{"role":"user","content":"Hello"}]}'</div>
          </div>
        </div>
      </section>

      <section id="pools" class="panel p-6 lg:p-8">
        <p class="docs-kicker text-xs font-bold uppercase tracking-[0.22em]">Shared Pools</p>
        <h2 class="docs-section-title mt-3 text-2xl font-bold">{{ t('docsPage.pools.title') }}</h2>
        <div class="mt-5 grid gap-4 md:grid-cols-3">
          <article v-for="step in poolSteps" :key="step.title" class="docs-info-card rounded-3xl p-5">
            <div class="docs-step-index mb-4 inline-flex h-8 w-8 items-center justify-center rounded-full text-xs font-bold">{{ step.index }}</div>
            <h3 class="docs-card-title font-semibold">{{ step.title }}</h3>
            <p class="docs-desc mt-2 text-sm leading-7">{{ step.desc }}</p>
          </article>
        </div>
      </section>

      <section id="credits" class="grid gap-4 lg:grid-cols-2">
        <article class="panel p-6 lg:p-8">
          <p class="docs-kicker text-xs font-bold uppercase tracking-[0.22em]">Account Balance</p>
          <h2 class="docs-section-title mt-3 text-2xl font-bold">{{ t('docsPage.credits.title') }}</h2>
          <p class="docs-desc mt-3 leading-8">{{ t('docsPage.credits.description') }}</p>
          <div class="mt-5 flex flex-wrap gap-3">
            <router-link to="/usage" class="btn btn-secondary">{{ t('docsPage.credits.viewUsage') }}</router-link>
            <router-link to="/redeem" class="btn btn-secondary">{{ t('docsPage.credits.redeem') }}</router-link>
          </div>
        </article>
        <article class="panel p-6 lg:p-8">
          <p class="docs-kicker text-xs font-bold uppercase tracking-[0.22em]">Service Health</p>
          <h2 class="docs-section-title mt-3 text-2xl font-bold">{{ t('docsPage.status.title') }}</h2>
          <p class="docs-desc mt-3 leading-8">{{ t('docsPage.status.description') }}</p>
          <div class="mt-5 flex flex-wrap gap-3">
            <router-link to="/monitor" class="btn btn-secondary">{{ t('docsPage.status.viewStatus') }}</router-link>
            <router-link to="/available-channels" class="btn btn-secondary">{{ t('docsPage.status.modelCatalog') }}</router-link>
          </div>
        </article>
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'

const { t } = useI18n()

const toc = computed(() => [
  { href: '#workspace', title: t('docsPage.toc.workspace.title'), desc: t('docsPage.toc.workspace.desc') },
  { href: '#service-entry', title: t('docsPage.toc.gateway.title'), desc: t('docsPage.toc.gateway.desc') },
  { href: '#pools', title: t('docsPage.toc.pools.title'), desc: t('docsPage.toc.pools.desc') },
  { href: '#credits', title: t('docsPage.toc.credits.title'), desc: t('docsPage.toc.credits.desc') }
])

const workspaceCards = computed(() => [
  { title: t('docsPage.workspace.cards.dashboard.title'), desc: t('docsPage.workspace.cards.dashboard.desc') },
  { title: t('docsPage.workspace.cards.gateway.title'), desc: t('docsPage.workspace.cards.gateway.desc') },
  { title: t('docsPage.workspace.cards.future.title'), desc: t('docsPage.workspace.cards.future.desc') }
])

const poolSteps = computed(() => [
  { index: '1', title: t('docsPage.pools.steps.choose.title'), desc: t('docsPage.pools.steps.choose.desc') },
  { index: '2', title: t('docsPage.pools.steps.join.title'), desc: t('docsPage.pools.steps.join.desc') },
  { index: '3', title: t('docsPage.pools.steps.call.title'), desc: t('docsPage.pools.steps.call.desc') }
])
</script>

<style scoped>
.docs-page {
  color: var(--zc-text);
}

.docs-hero {
  background: var(--zc-bg);
  box-shadow:
    12px 12px 28px var(--zc-shadow-dark),
    -12px -12px 28px var(--zc-shadow-light);
  isolation: isolate;
}

.docs-hero-grid {
  background:
    linear-gradient(90deg, var(--zc-grid) 1px, transparent 1px),
    linear-gradient(180deg, var(--zc-grid) 1px, transparent 1px),
    radial-gradient(circle at 18px 18px, color-mix(in srgb, var(--zc-accent) 18%, transparent) 1px, transparent 1px);
  background-size: 42px 42px, 42px 42px, 18px 18px;
  opacity: 0.62;
}

.docs-kicker {
  color: var(--zc-subtle);
}

.docs-hero-title,
.docs-section-title,
.docs-card-title {
  color: var(--zc-text-strong);
}

.docs-desc,
.docs-code-label {
  color: var(--zc-muted);
}

.docs-code-text {
  color: var(--zc-text);
}

.docs-code-accent,
.docs-inline-link {
  color: var(--zc-accent);
}

.docs-code-card,
.docs-info-card,
.docs-toc-card {
  background: var(--zc-bg);
  box-shadow:
    8px 8px 18px var(--zc-shadow-dark),
    -8px -8px 18px var(--zc-shadow-light);
}

.docs-code-card {
  box-shadow:
    inset 6px 6px 14px var(--zc-shadow-dark),
    inset -6px -6px 14px var(--zc-shadow-light);
}

.docs-toc-card:hover,
.docs-info-card:hover {
  box-shadow:
    5px 5px 12px var(--zc-shadow-dark),
    -5px -5px 12px var(--zc-shadow-light);
}

.docs-inline-code,
.docs-step-index {
  background: var(--zc-bg);
  color: var(--zc-accent);
  box-shadow:
    inset 3px 3px 7px var(--zc-shadow-dark),
    inset -3px -3px 7px var(--zc-shadow-light);
}
</style>
