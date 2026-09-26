<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ArrowLeft, ArrowRight, BookOpen, Compass, Pause, Play, Sparkles, X } from '@lucide/vue'
import { useAuthStore } from '@/stores/auth'
import { useCityArrivalStore } from '@/stores/cityArrival'

const auth = useAuthStore()
const arrival = useCityArrivalStore()
const route = useRoute()
const router = useRouter()
const dialog = ref<HTMLDialogElement | null>(null)
const heading = ref<HTMLElement | null>(null)
const step = ref(0)
const paused = ref(false)
const navigationError = ref('')
const navigating = ref(false)
const name = computed(() => auth.user?.username?.trim() || '旅人')
const eligible = computed(() => auth.isAuthenticated && route.meta.requiresAuth === true && !route.path.startsWith('/admin'))
const visible = computed(() => eligible.value && arrival.showStory)
const scenes = [
  { image: '/brand/arrival/harbour.jpg', location: '蓝时海岸 · 零点灯塔', title: '欢迎来到零号城', line: '灯不是为完美抵达的人亮的。', story: '也为绕过路、停过步，仍然想再试一次的你。今晚，守灯人把光转向了岸边。' },
  { image: '/brand/arrival/night-shift.jpg', location: '未打烊巷 · 希望夜班员', title: '这座城，从一盏灯开始', line: '“这里有人可以接班。”', story: '断光那夜，人们把最后一盏灯留在商店门口。后来，有人修工具，有人留下故事，也有人为晚到的人添了一把椅子。零号城就这样慢慢长了出来。' },
  { image: '/brand/arrival/workshop.jpg', location: '重试工坊 · 下一页，由你写', title: '你想从哪里开始？', line: '带着一个还没完成的念头，也可以进城。', story: '' },
]
const scene = computed(() => scenes[step.value]!)
const destinations = [
  { name: '开始创作', detail: '带上一个想法，去工作台', path: '/operator', icon: Sparkles },
  { name: '寻找资源', detail: '挑选模型与共享池', path: '/account-square', icon: Compass },
  { name: '逛逛零号城', detail: '看看大家的故事与作品', path: '/community', icon: BookOpen },
]
let previousFocus: HTMLElement | null = null
let greetingTimer: ReturnType<typeof setTimeout> | undefined
let oldOverflow = ''
let ownsScrollLock = false
let openGeneration = 0

function restoreScroll() {
  if (ownsScrollLock) { document.body.style.overflow = oldOverflow; ownsScrollLock = false }
}

watch(() => auth.user?.id, (id, previous) => {
  if (id !== previous) arrival.resetSession()
}, { flush: 'sync' })
watch([eligible, () => auth.user?.id], ([allowed, id]) => {
  if (allowed && id) arrival.visit(id)
}, { immediate: true })

watch(visible, async (open) => {
  const generation = ++openGeneration
  if (open) {
    step.value = 0
    paused.value = false
    navigationError.value = ''
    previousFocus = document.activeElement as HTMLElement | null
    oldOverflow = document.body.style.overflow
    ownsScrollLock = true
    document.body.style.overflow = 'hidden'
    await nextTick()
    if (generation !== openGeneration || !visible.value) return
    if (dialog.value && !dialog.value.open) dialog.value.showModal()
    heading.value?.focus()
  } else {
    dialog.value?.close()
    restoreScroll()
    if (previousFocus?.isConnected) previousFocus.focus()
  }
}, { immediate: true })

function dismissGreeting() {
  clearTimeout(greetingTimer)
  arrival.showGreeting = false
}
function pauseGreetingTimer() { clearTimeout(greetingTimer) }
function resumeGreetingTimer() {
  clearTimeout(greetingTimer)
  greetingTimer = setTimeout(dismissGreeting, 9000)
}
watch(() => arrival.showGreeting && eligible.value, (show) => {
  clearTimeout(greetingTimer)
  if (show) resumeGreetingTimer()
}, { immediate: true })
async function changeStep(index: number) {
  step.value = Math.max(0, Math.min(2, index))
  await nextTick()
  heading.value?.focus()
}
async function go(path: string) {
  if (navigating.value) return
  navigating.value = true
  navigationError.value = ''
  try {
    const failure = await router.push(path)
    if (failure && route.fullPath !== path) throw new Error('navigation interrupted')
    arrival.finish()
  } catch {
    navigationError.value = '暂时没能打开目的地，请重试，或先留在当前页面。'
  } finally { navigating.value = false }
}
onBeforeUnmount(() => {
  openGeneration++
  clearTimeout(greetingTimer)
  dialog.value?.close()
  restoreScroll()
})
</script>

<template>
  <Teleport to="body">
    <dialog ref="dialog" class="city-arrival" :class="{ 'is-paused': paused }" aria-labelledby="arrival-title" aria-describedby="arrival-story" @cancel.prevent="arrival.finish()">
      <template v-if="visible">
        <div class="arrival-visual" aria-hidden="true">
          <Transition name="arrival-scene">
            <img :key="scene.image" class="arrival-art" :src="scene.image" alt="" />
          </Transition>
          <div class="arrival-shade"></div>
        </div>
        <header class="arrival-header">
          <span class="arrival-brand"><img src="/brand/zero-mark.svg" alt="" />零号城 <small>入城序章</small></span>
          <div class="arrival-controls">
            <button type="button" class="arrival-icon" :aria-label="paused ? '播放画面' : '暂停画面'" :title="paused ? '播放画面' : '暂停画面'" @click="paused = !paused"><Play v-if="paused" :size="18" /><Pause v-else :size="18" /></button>
            <button type="button" class="arrival-skip" @click="arrival.finish()">跳过序章<X :size="17" /></button>
          </div>
        </header>
        <main class="arrival-content">
          <p class="arrival-place">{{ scene.location }}</p>
          <h1 id="arrival-title" ref="heading" tabindex="-1">{{ scene.title }}</h1>
          <p v-if="step === 0" class="arrival-name">{{ name }}，你到了。</p>
          <p class="arrival-line">{{ scene.line }}</p>
          <p id="arrival-story" class="arrival-story">{{ scene.story || '选择第一站，或者留在刚刚打开的页面。' }}</p>
          <div v-if="step === 2" class="arrival-destinations">
            <button v-for="destination in destinations" :key="destination.path" type="button" :disabled="navigating" @click="go(destination.path)">
              <component :is="destination.icon" :size="21" />
              <span><strong>{{ destination.name }}</strong><small>{{ destination.detail }}</small></span>
              <ArrowRight :size="20" />
            </button>
          </div>
          <p v-if="navigationError" class="arrival-error" role="alert">{{ navigationError }}</p>
          <div class="arrival-actions">
            <button v-if="step < 2" type="button" class="arrival-primary" @click="changeStep(step + 1)">{{ step === 0 ? '听听这座城的故事' : '翻开我的这一页' }}<ArrowRight :size="18" /></button>
            <button v-else type="button" class="arrival-primary" @click="arrival.finish()">留在当前页面<ArrowRight :size="18" /></button>
            <button v-if="step === 2" type="button" class="arrival-text" :disabled="navigating" @click="go('/zero-city/chronicle')">阅读完整编年史<BookOpen :size="16" /></button>
          </div>
        </main>
        <footer class="arrival-footer">
          <nav aria-label="入城故事章节">
            <button v-for="(label, index) in ['抵达', '留灯的人', '你的第一站']" :key="label" type="button" :aria-current="step === index ? 'step' : undefined" @click="changeStep(index)"><span>0{{ index + 1 }}</span>{{ label }}</button>
          </nav>
          <button v-if="step > 0" type="button" class="arrival-icon" aria-label="上一幕" title="上一幕" @click="changeStep(step - 1)"><ArrowLeft :size="20" /></button>
          <span v-else class="arrival-edition">第一卷 · 留灯的人</span>
        </footer>
      </template>
    </dialog>
    <Transition name="arrival-greeting">
      <aside v-if="eligible && arrival.showGreeting && !arrival.showStory" class="city-return" aria-label="回城问候" @mouseenter="pauseGreetingTimer" @mouseleave="resumeGreetingTimer" @focusin="pauseGreetingTimer" @focusout="resumeGreetingTimer">
        <img src="/brand/arrival/harbour.jpg" alt="" />
        <div><p role="status">欢迎回到零号城，{{ name }}。</p><span>灯还亮着。今天的故事，慢慢续。</span><button type="button" @click="arrival.replay(auth.user!.id)">重看入城故事<ArrowRight :size="14" /></button></div>
        <button class="return-close" type="button" aria-label="关闭欢迎" title="关闭欢迎" @click="dismissGreeting"><X :size="16" /></button>
      </aside>
    </Transition>
  </Teleport>
</template>

<style scoped>
.city-arrival{position:fixed;inset:0;width:100%;max-width:none;height:100dvh;max-height:none;margin:0;padding:0;border:0;background:#142622;color:#fff;overflow-x:hidden;overflow-y:auto;letter-spacing:0}
.city-arrival[open]{display:flex;flex-direction:column;isolation:isolate}
.city-arrival::backdrop{background:#142622}
.arrival-visual{position:fixed;inset:0;overflow:hidden;z-index:-1;pointer-events:none}
.arrival-art,.arrival-shade{position:absolute;inset:0;width:100%;height:100%;min-height:100%;pointer-events:none}
.arrival-art{object-fit:cover;object-position:center 38%;animation:arrival-drift 20s ease-in-out infinite alternate}
.arrival-shade{background:linear-gradient(90deg,rgba(8,23,22,.87),rgba(8,23,22,.65) 36%,rgba(8,23,22,.06) 75%),linear-gradient(0deg,rgba(8,23,22,.75),transparent 55%,rgba(8,23,22,.2))}
.arrival-header,.arrival-footer{display:flex;align-items:center;justify-content:space-between;gap:16px;padding:24px 48px;flex-shrink:0}
.arrival-brand{display:flex;align-items:center;gap:10px;font-size:20px;font-weight:650}.arrival-brand img{width:34px;height:34px}.arrival-brand small{font-size:12px;font-weight:400;margin-left:14px;color:#d4ded7}
.arrival-controls{display:flex;align-items:center;gap:16px}.arrival-icon{width:40px;height:40px;display:grid;place-items:center;border:1px solid #ffffff50;border-radius:50%;color:#fff;background:#10252060}
.arrival-skip,.arrival-text{display:inline-flex;align-items:center;gap:8px;font-size:13px;color:#f0f5f1;min-height:40px}
.arrival-content{flex:1;display:flex;flex-direction:column;justify-content:center;align-items:flex-start;padding:36px 72px;max-width:790px}
.arrival-place{font-size:12px;color:#c4e5d8;margin-bottom:22px}
.arrival-content h1{font-size:48px;font-weight:600;line-height:1.22;margin:0 0 22px;text-wrap:balance;outline:none}
.arrival-name{font-size:17px;margin:0 0 24px;max-width:100%;overflow-wrap:anywhere;color:#d8e8e1}
.arrival-line{font-size:22px;line-height:1.6;margin:0 0 14px;text-wrap:balance}
.arrival-story{font-size:15px;line-height:1.95;max-width:480px;color:#dce7e1;margin:0;min-height:56px}
.arrival-actions{display:flex;flex-wrap:wrap;gap:12px 24px;margin-top:30px}
.arrival-primary{display:flex;align-items:center;gap:22px;min-height:46px;padding:12px 20px;border-radius:6px;background:#dcf5ba;color:#183b2b;font-size:14px;font-weight:600}
.arrival-primary:hover{background:#edfbdc}.arrival-text:hover,.arrival-skip:hover{text-decoration:underline}
.arrival-footer nav{display:flex;gap:24px}.arrival-footer nav button{display:flex;align-items:center;gap:9px;padding:12px 0;border-top:2px solid #ffffff35;color:#c2d1c9;font-size:12px;min-width:100px}
.arrival-footer nav button[aria-current]{border-color:#dcf5ba;color:#fff}.arrival-footer nav span{font-size:11px;color:#c4e5d8}
.arrival-edition{font-size:12px;color:#cedbd3}
.arrival-destinations{width:100%;max-width:470px;margin-top:12px;border-top:1px solid #ffffff40}
.arrival-destinations>button{display:flex;align-items:center;gap:16px;width:100%;text-align:left;padding:14px 4px;border-bottom:1px solid #ffffff40;color:#fff;min-height:70px}
.arrival-destinations>button:hover{background:#ffffff12}.arrival-destinations span{flex:1;min-width:0}.arrival-destinations strong{display:block;font-size:15px;font-weight:600}.arrival-destinations small{display:block;font-size:12px;color:#d0ded6;margin-top:5px}
.arrival-error{font-size:13px;color:#ffe0d6;margin-top:12px}
.city-arrival button:focus-visible,.city-return button:focus-visible{outline:2px solid #c1ec83;outline-offset:5px}
.city-arrival button:disabled{opacity:.6;cursor:wait}
.arrival-scene-enter-active,.arrival-scene-leave-active{transition:opacity .65s ease}.arrival-scene-enter-from,.arrival-scene-leave-to{opacity:0}
.is-paused .arrival-art{animation-play-state:paused}
.city-return{position:fixed;top:74px;right:24px;z-index:45;display:flex;gap:14px;align-items:center;width:400px;max-width:calc(100% - 24px);padding:16px 40px 16px 16px;background:var(--bd-surface,#fff);border:1px solid var(--bd-ui-line,#dce3de);border-radius:8px;box-shadow:0 10px 35px #14262218;color:var(--bd-text-primary,#183b2b)}
.city-return>img{width:54px;height:72px;object-fit:cover;border-radius:4px;flex-shrink:0}.city-return>div{min-width:0}.city-return p{font-size:14px;font-weight:600;line-height:1.6;overflow-wrap:anywhere;margin:0 0 4px}.city-return span{font-size:12px;color:var(--bd-text-secondary,#52685f)}
.city-return div button{display:flex;gap:8px;align-items:center;font-size:12px;color:var(--bd-accent-teal,#067f70);margin-top:8px;min-height:24px}
.return-close{position:absolute;right:10px;top:10px;width:28px;height:28px;display:grid;place-items:center}
.arrival-greeting-enter-active,.arrival-greeting-leave-active{transition:opacity .3s,transform .3s}.arrival-greeting-enter-from,.arrival-greeting-leave-to{opacity:0;transform:translateY(-8px)}
@keyframes arrival-drift{from{transform:scale(1.02) translateX(-.3%)}to{transform:scale(1.09) translateX(.3%)}}
@media(min-width:1700px){.arrival-content{padding-left:120px;max-width:900px}.arrival-content h1{font-size:56px}}
@media(max-width:767px){
  .arrival-header,.arrival-footer{padding:18px 22px}.arrival-brand{font-size:17px}.arrival-brand small{display:none}.arrival-controls{gap:12px}
  .arrival-content{padding:38px 24px 20px;justify-content:flex-end;max-width:100%}.arrival-content h1{font-size:32px;margin-bottom:18px}.arrival-line{font-size:19px}.arrival-place{margin-bottom:14px}.arrival-name{font-size:15px;margin-bottom:18px}
  .arrival-story{font-size:14px;max-width:100%}.arrival-art{object-position:58% center}.arrival-shade{background:linear-gradient(0deg,rgba(8,23,22,.96),rgba(8,23,22,.85) 35%,rgba(8,23,22,.12) 86%,rgba(8,23,22,.5))}
  .arrival-footer nav{gap:16px;flex:1}.arrival-footer nav button{min-width:0;gap:5px;flex:1;font-size:11px;white-space:nowrap}.arrival-edition{display:none}.arrival-footer>.arrival-icon{width:32px;height:32px;flex-shrink:0}
  .arrival-actions{margin-top:22px;gap:12px}.arrival-destinations>button{min-height:62px;padding:12px 0}.city-return{right:12px}
}
@media(max-height:640px){.arrival-content{padding-top:16px;padding-bottom:16px}.arrival-header,.arrival-footer{padding-top:12px;padding-bottom:12px}.arrival-content h1{font-size:30px}.arrival-art,.arrival-shade{position:fixed}.arrival-actions{margin-top:16px}}
@media(prefers-reduced-motion:reduce){.arrival-art{animation:none}.arrival-scene-enter-active,.arrival-scene-leave-active,.arrival-greeting-enter-active,.arrival-greeting-leave-active{transition:none}}
</style>
