<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ArrowLeft, ArrowRight, BookOpen, MapPin, Search, Users, X } from '@lucide/vue'
import { DialogClose, DialogContent, DialogDescription, DialogOverlay, DialogPortal, DialogRoot, DialogTitle, TabsContent, TabsList, TabsRoot, TabsTrigger } from 'reka-ui'
import { ZERO_CITY_CARD_MANIFEST, zeroCityCardCatalogRarity, zeroCityCardRarityDisplayName, type ZeroCityCardKey } from '@/constants/zeroCityCardManifest'
import { CHRONICLE_EDITION, chronicleCardLink, chronicleDistricts, chronicleEras, chronicleEvents, chroniclePortrait, chronicleResident, chronicleResidents, eventsForResident, filterChronicleEvents, type ChronicleEra } from '../../data/zeroCityChronicle'

const route = useRoute()
const router = useRouter()
const tab = ref('chronicle')
const query = ref('')
const era = ref<ChronicleEra | 'all'>('all')
const selectedID = ref(chronicleEvents[0].id)
const residentKey = ref<ZeroCityCardKey>()
const currentResident = computed(() => chronicleResident(residentKey.value))
const currentDefinition = computed(() => currentResident.value ? ZERO_CITY_CARD_MANIFEST[currentResident.value.key] : undefined)
const events = computed(() => filterChronicleEvents(query.value, era.value))
const selectedEvent = computed(() => events.value.find(event => event.id === selectedID.value) || events.value[0])
const selectedIndex = computed(() => events.value.findIndex(event => event.id === selectedEvent.value?.id))
const eraLabel = (key: ChronicleEra) => chronicleEras.find(item => item.id === key)?.name
const district = (id: string) => chronicleDistricts.find(item => item.id === id)
const residentsIn = (id: string) => chronicleResidents.filter(resident => resident.district === id)
const residentName = (key: ZeroCityCardKey) => ZERO_CITY_CARD_MANIFEST[key].name
const matches = (values: Array<string | undefined>) => !query.value.trim() || values.join(' ').toLocaleLowerCase().includes(query.value.trim().toLocaleLowerCase())
const filteredResidents = computed(() => chronicleResidents.filter(resident => matches([residentName(resident.key), ZERO_CITY_CARD_MANIFEST[resident.key].faction, resident.home, resident.routine, resident.wish, resident.fear, resident.secret, district(resident.district)?.name])))
const filteredDistricts = computed(() => chronicleDistricts.filter(item => matches([item.name, item.detail, item.everyday, ...residentsIn(item.id).map(resident => residentName(resident.key))])))

function resetFilters() { query.value = ''; era.value = 'all' }
function selectStory(id: string, reveal = false) {
  if (reveal) resetFilters()
  tab.value = 'chronicle'
  selectedID.value = id
  residentKey.value = undefined
  void router.replace({ query: { ...route.query, story: id, resident: undefined, view: undefined } })
}
function openResident(key: ZeroCityCardKey) {
  residentKey.value = key
  void router.replace({ query: { ...route.query, resident: key } })
}
function closeResident(open: boolean) {
  if (open) return
  residentKey.value = undefined
  if (route.query.resident) void router.replace({ query: { ...route.query, resident: undefined } })
}
function showDistrict(id: string) {
  query.value = district(id)?.name || ''
  tab.value = 'districts'
  residentKey.value = undefined
  void router.replace({ query: { ...route.query, view: 'districts', resident: undefined } })
}
function updateView(value: string | number) {
  tab.value = String(value)
  void router.replace({ query: { ...route.query, view: value === 'chronicle' ? undefined : String(value) } })
}
watch(() => [route.query.view, route.query.story, route.query.resident], () => {
  const view = route.query.view
  tab.value = view === 'districts' || view === 'residents' ? view : 'chronicle'
  const story = chronicleEvents.find(event => event.id === route.query.story)
  if (story && story.id !== selectedID.value) {
    resetFilters()
    selectedID.value = story.id
  }
  residentKey.value = typeof route.query.resident === 'string' ? chronicleResident(route.query.resident)?.key : undefined
}, { immediate: true })
</script>

<template>
  <section class="city-chronicle" aria-labelledby="chronicle-title">
    <header class="chronicle-heading">
      <div><p class="chronicle-eyebrow">零号城正史 · {{ CHRONICLE_EDITION }}</p><h1 id="chronicle-title">城市编年史</h1><p class="chronicle-intro">地图少了一条路，值班表少了一页。桥的另一头，还有人不肯回来。</p></div>
      <RouterLink class="chronicle-link" to="/zero-city/cards"><BookOpen :size="16" aria-hidden="true" />收藏卡册<ArrowRight :size="16" aria-hidden="true" /></RouterLink>
    </header>

    <TabsRoot :model-value="tab" class="chronicle-tabs" @update:model-value="updateView">
      <div class="chronicle-toolbar">
        <TabsList class="chronicle-tab-list" aria-label="世界观档案">
          <TabsTrigger value="chronicle"><BookOpen :size="16" aria-hidden="true" />编年史</TabsTrigger>
          <TabsTrigger value="districts"><MapPin :size="16" aria-hidden="true" />城区</TabsTrigger>
          <TabsTrigger value="residents"><Users :size="16" aria-hidden="true" />居民</TabsTrigger>
        </TabsList>
        <label class="chronicle-search"><Search :size="16" aria-hidden="true" /><input v-model="query" type="search" aria-label="搜索城市故事、城区与居民" placeholder="故事、城区、居民" /><button v-if="query" type="button" aria-label="清除搜索" title="清除搜索" @click="query = ''"><X :size="15" aria-hidden="true" /></button></label>
      </div>

      <TabsContent value="chronicle">
        <div class="chronicle-filter"><label for="chronicle-era">时代</label><select id="chronicle-era" v-model="era"><option value="all">全部时代</option><option v-for="item in chronicleEras" :key="item.id" :value="item.id">{{ item.name }} · {{ item.years }}</option></select><span role="status">{{ events.length }} 篇故事</span></div>
        <div v-if="selectedEvent" class="chronicle-reading-layout">
          <nav class="chronicle-timeline" aria-label="故事目录">
            <button v-for="event in events" :key="event.id" type="button" :aria-current="event.id === selectedEvent.id ? 'true' : undefined" @click="selectStory(event.id)"><span class="timeline-point" aria-hidden="true"></span><small>{{ event.date }}</small><strong>{{ event.title }}</strong></button>
          </nav>
          <article class="chronicle-story" aria-labelledby="chronicle-story-title">
            <div class="story-meta"><span>{{ eraLabel(selectedEvent.era) }}</span><span>{{ selectedEvent.date }}</span></div>
            <h2 id="chronicle-story-title">{{ selectedEvent.title }}</h2>
            <p class="story-lede">{{ selectedEvent.summary }}</p>
            <div class="story-cast" aria-label="故事中的居民"><button v-for="key in selectedEvent.residents" :key="key" type="button" @click="openResident(key)"><img :src="chroniclePortrait(key)" :alt="residentName(key)" width="76" height="90" /><span>{{ residentName(key) }}</span></button></div>
            <div class="story-prose"><p v-for="paragraph in selectedEvent.paragraphs" :key="paragraph">{{ paragraph }}</p></div>
            <aside class="story-trace"><span>留在现场的证据</span><p>{{ selectedEvent.trace }}</p><button class="chronicle-link" type="button" @click="showDistrict(selectedEvent.district)"><MapPin :size="14" aria-hidden="true" />{{ district(selectedEvent.district)?.name }}<ArrowRight :size="14" aria-hidden="true" /></button></aside>
            <footer class="story-pagination"><button type="button" :disabled="selectedIndex <= 0" @click="selectStory(events[selectedIndex - 1].id)"><ArrowLeft :size="16" aria-hidden="true" />上一篇</button><span>{{ selectedIndex + 1 }} / {{ events.length }}</span><button type="button" :disabled="selectedIndex >= events.length - 1" @click="selectStory(events[selectedIndex + 1].id)">下一篇<ArrowRight :size="16" aria-hidden="true" /></button></footer>
          </article>
        </div>
        <div v-else class="chronicle-empty" role="status"><BookOpen :size="28" aria-hidden="true" /><h2>没有找到这段故事</h2><p>这一卷里暂时没有匹配的记录。</p><button type="button" @click="resetFilters">清除筛选</button></div>
      </TabsContent>

      <TabsContent value="districts">
        <p class="chronicle-result" role="status">{{ filteredDistricts.length }} 处城区 · 城市世界观设定</p>
        <div class="chronicle-districts"><article v-for="item in filteredDistricts" :key="item.id" class="chronicle-district"><div class="district-marker" aria-hidden="true"><MapPin :size="20" /></div><div><span class="chronicle-eyebrow">{{ item.address }}</span><h2>{{ item.name }}</h2><p>{{ item.detail }}</p><p class="district-everyday">{{ item.everyday }}</p><small class="district-landmark">{{ item.landmark }}</small><div class="district-residents"><button v-for="resident in residentsIn(item.id)" :key="resident.key" type="button" @click="openResident(resident.key)"><img :src="chroniclePortrait(resident.key)" :alt="residentName(resident.key)" loading="lazy" width="48" height="56" /><span>{{ residentName(resident.key) }}</span><ArrowRight :size="14" aria-hidden="true" /></button></div></div></article></div>
        <div v-if="!filteredDistricts.length" class="chronicle-empty" role="status"><h2>没有找到这个城区</h2><button type="button" @click="query = ''">清除搜索</button></div>
      </TabsContent>

      <TabsContent value="residents">
        <p class="chronicle-result" role="status">{{ filteredResidents.length }} 位出场角色 · 收藏卡角色档案</p>
        <div class="chronicle-residents"><button v-for="resident in filteredResidents" :key="resident.key" class="resident-entry" type="button" @click="openResident(resident.key)"><img :src="chroniclePortrait(resident.key)" :alt="residentName(resident.key)" width="112" height="140" loading="lazy" /><span class="resident-entry-copy"><small>{{ ZERO_CITY_CARD_MANIFEST[resident.key].faction }}</small><strong>{{ residentName(resident.key) }}</strong><span>{{ ZERO_CITY_CARD_MANIFEST[resident.key].line }}</span><small class="resident-home"><MapPin :size="13" aria-hidden="true" />{{ district(resident.district)?.name }}</small></span><ArrowRight :size="16" aria-hidden="true" /></button></div>
        <div v-if="!filteredResidents.length" class="chronicle-empty" role="status"><h2>没有找到这位居民</h2><button type="button" @click="query = ''">清除搜索</button></div>
      </TabsContent>
    </TabsRoot>
    <p class="chronicle-colophon">官方世界观作品 · 零历为虚构纪年。角色故事不代表真实用户行为、平台规则或资产权益。</p>

    <DialogRoot :open="Boolean(currentResident)" @update:open="closeResident">
      <DialogPortal><DialogOverlay class="chronicle-dialog-overlay" /><DialogContent v-if="currentResident && currentDefinition" class="chronicle-resident-dialog">
        <div class="resident-dialog-heading"><span>居民档案 · {{ currentDefinition.code }}</span><DialogClose class="chronicle-icon-button" aria-label="关闭居民档案" title="关闭居民档案"><X :size="20" /></DialogClose></div>
        <div class="resident-dialog-identity"><img :src="chroniclePortrait(currentResident.key)" :alt="currentDefinition.name" width="128" height="160" /><div><span class="resident-rarity">{{ zeroCityCardRarityDisplayName(zeroCityCardCatalogRarity(currentResident.key)) }} · {{ currentDefinition.faction }}</span><DialogTitle as="h2">{{ currentDefinition.name }}</DialogTitle><DialogDescription>{{ currentDefinition.role }}</DialogDescription><blockquote>{{ currentDefinition.line }}</blockquote></div></div>
        <div class="resident-dialog-body"><section><h3>住处</h3><p>{{ currentResident.home }}</p><button class="chronicle-link" type="button" @click="showDistrict(currentResident.district)">走进{{ district(currentResident.district)?.name }}<ArrowRight :size="14" aria-hidden="true" /></button></section><section><h3>没有任务的时候</h3><p>{{ currentResident.routine }}</p></section><section><h3>真正想要的</h3><p>{{ currentResident.wish }}</p></section><section><h3>最怕发生的</h3><p>{{ currentResident.fear }}</p></section><section><h3>没有写进正式档案</h3><p>{{ currentResident.secret }}</p></section><section><h3>欠下的一件事</h3><p>{{ currentResident.relationship.detail }}</p><button class="chronicle-link" type="button" @click="openResident(currentResident.relationship.key)">{{ residentName(currentResident.relationship.key) }}<ArrowRight :size="14" aria-hidden="true" /></button></section><section><h3>出场故事</h3><div class="resident-stories"><button v-for="event in eventsForResident(currentResident.key)" :key="event.id" type="button" @click="selectStory(event.id, true)"><span>{{ event.title }}</span><ArrowRight :size="16" aria-hidden="true" /></button></div></section></div>
        <footer class="resident-dialog-footer"><span>故事档案不要求持有收藏卡。</span><RouterLink class="chronicle-link" :to="chronicleCardLink(currentResident.key)"><BookOpen :size="16" aria-hidden="true" />查看收藏卡<ArrowRight :size="16" aria-hidden="true" /></RouterLink></footer>
      </DialogContent></DialogPortal>
    </DialogRoot>
  </section>
</template>

<style scoped>
.city-chronicle { --chronicle-text:var(--zc-text,#202428); --chronicle-muted:var(--zc-muted,#606a75); --chronicle-line:var(--zc-line,#dee3e8); --chronicle-surface:var(--zc-card,#fff); --chronicle-accent:var(--zc-accent,#087f70); max-width:1200px; margin:0 auto; color:var(--chronicle-text); letter-spacing:0; }
.chronicle-heading { display:flex; align-items:center; justify-content:space-between; gap:24px; padding:8px 0 24px; }
.chronicle-heading h1 { margin:6px 0; font-size:24px; font-weight:600; }
.chronicle-eyebrow,.chronicle-result,.chronicle-colophon { color:var(--chronicle-muted); font-size:12px; line-height:1.65; }
.chronicle-intro { font-size:14px; color:var(--chronicle-muted); line-height:1.7; }
.chronicle-link { display:inline-flex; align-items:center; gap:7px; color:var(--chronicle-accent); font-size:13px; line-height:1.6; text-align:left; }
.chronicle-heading>.chronicle-link { flex-shrink:0; }
.chronicle-toolbar { display:flex; justify-content:space-between; align-items:center; gap:16px; border-bottom:1px solid var(--chronicle-line); }
.chronicle-tab-list { display:flex; gap:16px; }
.chronicle-tab-list button { display:flex; align-items:center; justify-content:center; gap:7px; min-height:48px; padding:0 3px; color:var(--chronicle-muted); font-size:14px; border-bottom:2px solid transparent; }
.chronicle-tab-list button[data-state="active"] { color:var(--chronicle-accent); border-bottom-color:var(--chronicle-accent); }
.chronicle-search { display:flex; align-items:center; gap:8px; width:240px; max-width:100%; padding:7px 10px; margin:6px 0; border:1px solid var(--chronicle-line); border-radius:6px; color:var(--chronicle-muted); background:var(--chronicle-surface); }
.chronicle-search input { min-width:0; width:100%; background:transparent; color:var(--chronicle-text); font-size:13px; outline:0; }
.chronicle-search button { display:grid; place-items:center; width:24px; height:24px; flex-shrink:0; }
.chronicle-filter { display:flex; align-items:center; gap:12px; padding:18px 0; font-size:12px; color:var(--chronicle-muted); }
.chronicle-filter select { min-height:36px; padding:4px 28px 4px 10px; max-width:100%; background:var(--chronicle-surface); border:1px solid var(--chronicle-line); border-radius:6px; font-size:13px; color:var(--chronicle-text); }
.chronicle-filter>span { margin-left:auto; white-space:nowrap; }
.chronicle-reading-layout { display:grid; grid-template-columns:215px minmax(0,1fr); gap:32px; border-top:1px solid var(--chronicle-line); }
.chronicle-timeline { padding-top:24px; align-self:start; display:grid; }
.chronicle-timeline button { position:relative; display:grid; gap:5px; padding:12px 10px 12px 26px; text-align:left; border-left:1px solid var(--chronicle-line); min-width:0; }
.timeline-point { width:7px; height:7px; position:absolute; top:19px; left:-4px; border-radius:50%; background:var(--chronicle-line); }
.chronicle-timeline small { color:var(--chronicle-muted); font-size:11px; line-height:1.5; }
.chronicle-timeline strong { font-size:13px; font-weight:500; line-height:1.7; overflow-wrap:anywhere; }
.chronicle-timeline [aria-current="true"] { background:color-mix(in srgb,var(--chronicle-accent) 6%,transparent); color:var(--chronicle-accent); }
.chronicle-timeline [aria-current="true"] .timeline-point { background:var(--chronicle-accent); }
.chronicle-story { min-width:0; max-width:780px; padding:28px 0 0; }
.story-meta { display:flex; gap:12px; flex-wrap:wrap; color:var(--chronicle-muted); font-size:12px; }
.story-meta span:first-child { color:var(--chronicle-accent); }
.chronicle-story h2 { margin:12px 0; font-size:24px; font-weight:600; line-height:1.5; overflow-wrap:anywhere; }
.story-lede { color:var(--chronicle-muted); font-size:14px; line-height:1.85; }
.story-cast { display:flex; flex-wrap:wrap; gap:12px; padding:24px 0; }
.story-cast button { display:flex; flex-direction:column; gap:8px; width:104px; text-align:center; align-items:center; font-size:11px; color:var(--chronicle-muted); }
.story-cast img { width:76px; height:90px; object-fit:cover; border-radius:6px; }
.story-cast button:hover { color:var(--chronicle-accent); }
.story-prose p { margin-bottom:20px; font-size:15px; line-height:2; text-align:justify; overflow-wrap:anywhere; }
.story-trace { margin-top:26px; padding:16px 0 16px 18px; border-left:3px solid color-mix(in srgb,var(--chronicle-accent) 55%,var(--chronicle-line)); }
.story-trace>span { color:var(--chronicle-muted); font-size:11px; }
.story-trace p { font-size:13px; line-height:1.9; margin:6px 0 10px; }
.story-pagination { display:flex; align-items:center; justify-content:space-between; gap:12px; border-top:1px solid var(--chronicle-line); margin-top:28px; padding:18px 0; color:var(--chronicle-muted); font-size:12px; }
.story-pagination button { display:flex; align-items:center; gap:8px; min-height:36px; }
.story-pagination button:disabled { opacity:.4; cursor:default; }
.chronicle-result { padding:18px 0; }
.chronicle-district { display:grid; grid-template-columns:40px minmax(0,1fr); gap:16px; padding:24px 0; border-bottom:1px solid var(--chronicle-line); }
.district-marker { width:36px; height:36px; display:grid; place-items:center; border:1px solid var(--chronicle-line); color:var(--chronicle-accent); border-radius:6px; }
.chronicle-district h2 { font-size:20px; font-weight:600; margin:6px 0 12px; }
.chronicle-district p { max-width:760px; font-size:14px; line-height:1.9; }
.district-everyday { margin-top:10px; color:var(--chronicle-muted); }
.district-landmark { display:block; font-size:12px; color:var(--chronicle-accent); margin-top:14px; }
.district-residents { display:flex; flex-wrap:wrap; gap:16px; margin-top:18px; }
.district-residents button { display:flex; align-items:center; gap:9px; min-height:56px; font-size:12px; }
.district-residents img { width:48px; height:56px; object-fit:cover; border-radius:4px; }
.chronicle-residents { display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); gap:0 28px; }
.resident-entry { display:grid; grid-template-columns:96px minmax(0,1fr) 16px; align-items:center; gap:16px; padding:20px 0; text-align:left; border-bottom:1px solid var(--chronicle-line); }
.resident-entry>img { width:96px; height:120px; object-fit:cover; border-radius:6px; }
.resident-entry-copy { display:grid; gap:7px; min-width:0; }
.resident-entry-copy small { color:var(--chronicle-muted); font-size:11px; }
.resident-entry-copy strong { font-size:16px; font-weight:600; line-height:1.6; }
.resident-entry-copy>span { font-size:13px; line-height:1.8; }
.resident-home { display:flex; gap:5px; align-items:center; }
.chronicle-empty { min-height:240px; display:flex; flex-direction:column; justify-content:center; align-items:center; gap:12px; color:var(--chronicle-muted); text-align:center; }
.chronicle-empty h2 { font-size:18px; color:var(--chronicle-text); }
.chronicle-empty p,.chronicle-empty button { font-size:13px; }
.chronicle-empty button { color:var(--chronicle-accent); min-height:36px; }
.chronicle-colophon { padding:24px 0 8px; border-top:1px solid var(--chronicle-line); margin-top:16px; font-size:11px; }
.city-chronicle button:focus-visible,.city-chronicle a:focus-visible,.city-chronicle select:focus-visible,.chronicle-search:focus-within { outline:2px solid var(--chronicle-accent); outline-offset:3px; }
.chronicle-dialog-overlay { position:fixed; inset:0; z-index:250; background:rgb(20 24 28 / 50%); }
.chronicle-resident-dialog { position:fixed; inset:50% auto auto 50%; z-index:251; width:min(620px,calc(100vw - 32px)); max-height:calc(100dvh - 32px); overflow:auto; transform:translate(-50%,-50%); background:var(--bd-surface,#fff); color:var(--bd-text-primary,#202428); border:1px solid var(--bd-ui-line,#dee3e8); border-radius:8px; letter-spacing:0; }
.resident-dialog-heading { display:flex; align-items:center; justify-content:space-between; padding:14px 20px; border-bottom:1px solid var(--bd-ui-line,#dee3e8); color:var(--bd-text-secondary,#606a75); font-size:12px; }
.chronicle-icon-button { display:grid; place-items:center; width:36px; height:36px; border-radius:6px; flex-shrink:0; }
.resident-dialog-identity { display:grid; grid-template-columns:112px minmax(0,1fr); gap:20px; align-items:center; padding:24px; }
.resident-dialog-identity img { width:112px; height:140px; object-fit:cover; border-radius:6px; }
.resident-dialog-identity h2 { font-size:22px; line-height:1.5; font-weight:600; margin:6px 0; overflow-wrap:anywhere; }
.resident-dialog-identity p,.resident-rarity { font-size:12px; color:var(--bd-text-secondary,#606a75); line-height:1.6; }
.resident-dialog-identity blockquote { margin-top:12px; font-size:13px; line-height:1.8; }
.resident-dialog-body { padding:0 24px; }
.resident-dialog-body section { padding:16px 0; border-top:1px solid var(--bd-ui-line,#dee3e8); }
.resident-dialog-body h3 { font-size:12px; color:var(--bd-text-secondary,#606a75); margin-bottom:8px; }
.resident-dialog-body p { font-size:14px; line-height:1.9; }
.resident-dialog-body .chronicle-link { margin-top:10px; color:var(--bd-accent-teal,#087f70); }
.resident-stories { display:grid; gap:2px; }
.resident-stories button { display:flex; align-items:center; justify-content:space-between; gap:16px; min-height:40px; text-align:left; font-size:13px; }
.resident-dialog-footer { display:flex; align-items:center; justify-content:space-between; gap:16px; padding:20px 24px; border-top:1px solid var(--bd-ui-line,#dee3e8); }
.resident-dialog-footer>span { font-size:11px; color:var(--bd-text-secondary,#606a75); }
.resident-dialog-footer .chronicle-link { color:var(--bd-accent-teal,#087f70); flex-shrink:0; }
.chronicle-resident-dialog button:focus-visible,.chronicle-resident-dialog a:focus-visible { outline:2px solid var(--bd-accent-teal,#087f70); outline-offset:2px; }
@media(max-width:900px) { .chronicle-reading-layout { grid-template-columns:180px minmax(0,1fr); gap:24px; } .chronicle-residents { grid-template-columns:minmax(0,1fr); } }
@media(max-width:640px) { .chronicle-heading { align-items:flex-start; gap:12px; } .chronicle-heading>.chronicle-link { font-size:12px; margin-top:8px; } .chronicle-intro { max-width:30em; font-size:13px; } .chronicle-toolbar { flex-wrap:wrap; gap:4px; padding-bottom:10px; } .chronicle-tab-list { gap:24px; } .chronicle-search { width:100%; margin:0; } .chronicle-reading-layout { display:block; } .chronicle-timeline { display:flex; overflow:auto; padding:16px 0 0; gap:0; } .chronicle-timeline button { flex:0 0 172px; padding:10px 14px; border-left:0; border-bottom:2px solid var(--chronicle-line); } .chronicle-timeline [aria-current="true"] { border-bottom-color:var(--chronicle-accent); } .timeline-point { display:none; } .chronicle-story { padding-top:24px; } .chronicle-story h2 { font-size:22px; } .story-prose p { font-size:14px; line-height:1.95; text-align:left; } .story-cast { gap:6px; } .story-cast button { width:84px; } .story-cast img { width:64px; height:78px; } .chronicle-district { grid-template-columns:28px minmax(0,1fr); gap:12px; } .district-marker { width:28px; height:32px; } .chronicle-filter { gap:8px; } .chronicle-filter select { min-width:0; flex:1; width:0; } .resident-dialog-identity { gap:14px; padding:18px; grid-template-columns:88px minmax(0,1fr); } .resident-dialog-identity img { width:88px; height:112px; } .resident-dialog-identity h2 { font-size:19px; } .resident-dialog-body { padding:0 18px; } .resident-dialog-footer { padding:18px; flex-wrap:wrap; } .resident-entry { gap:12px; grid-template-columns:80px minmax(0,1fr) 16px; } .resident-entry>img { width:80px; height:104px; } }
</style>
