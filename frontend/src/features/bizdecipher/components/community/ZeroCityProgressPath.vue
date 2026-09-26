<template>
  <section class="progress-path" aria-labelledby="city-progress-title">
    <header>
      <div><span>成长路径</span><h2 id="city-progress-title">资格状态待服务确认</h2></div>
      <p>当前前端只能展示真实社区历史，不能据此自动认定城市等级。</p>
    </header>
    <div class="history-strip">
      <span><b>{{ history.posts }}</b>我的帖子</span>
      <span><b>{{ history.accepted }}</b>已采纳</span>
      <span><b>{{ history.confirmed }}</b>已确认</span>
      <span><b>{{ history.assets }}</b>资产线索</span>
    </div>
    <ol>
      <li v-for="level in levels" :key="level.key">
        <span>{{ level.key }}</span>
        <div><strong>{{ level.title }}</strong><p>{{ level.access }}</p><small>{{ level.path }}</small></div>
      </li>
    </ol>
    <aside>
      <strong>双路径进入 L1</strong>
      <p>可以通过城内真实参与，或提交可验证的开源、艺术、视频、游戏等外部作品。验证接口尚未接入，本页不会显示“已通过”。</p>
    </aside>
  </section>
</template>

<script setup lang="ts">
defineProps<{ history: { posts: number; accepted: number; confirmed: number; assets: number } }>()

const levels = [
  { key: 'L0', title: '新来者', access: '阅读公开内容、点赞收藏，在闲聊广场发帖提问。', path: '轻量入城，仍受普通账号限制。' },
  { key: 'L1', title: '居民', access: '参与技术工坊与协作交流的发布和回复。', path: '城内成长，或可验证外部作品。' },
  { key: 'L2', title: '贡献者', access: '显示已确认的贡献身份与相关共创机会。', path: '来自采纳、真实复用、完成协作或有效主持反馈。' },
  { key: 'L3', title: '共建者', access: '持续贡献身份；组织与编辑权限仍需单独授权。', path: '长期贡献与承担责任，不由热度或消费直接授予。' },
] as const
</script>

<style scoped>
.progress-path { display:grid; gap:var(--bd-space-4); }
header { display:grid; grid-template-columns:minmax(0,.8fr) minmax(18rem,1.2fr); gap:var(--bd-space-4); align-items:end; padding-bottom:var(--bd-space-4); border-bottom:var(--bd-line-width) solid var(--zc-line); }
header span { color:var(--zc-accent); font-size:var(--bd-type-overline); font-weight:var(--bd-weight-emphasis); }
header h2 { color:var(--zc-text-strong); font-size:var(--zc-city-section); }
header p, li p, li small, aside p { color:var(--zc-muted); line-height:var(--bd-line-body-small); }
.history-strip { display:grid; grid-template-columns:repeat(4,minmax(0,1fr)); border-block:var(--bd-line-width) solid var(--zc-line); }
.history-strip span { display:grid; gap:var(--bd-space-1); padding:var(--bd-space-3); border-right:var(--bd-line-width) solid var(--zc-line); color:var(--zc-muted); font-size:var(--bd-type-overline); }
.history-strip span:last-child { border-right:0; }
.history-strip b { color:var(--zc-text-strong); font-size:var(--zc-city-section); }
ol { display:grid; grid-template-columns:repeat(4,minmax(0,1fr)); gap:var(--bd-space-3); }
li { display:grid; grid-template-columns:auto minmax(0,1fr); gap:var(--bd-space-3); padding-top:var(--bd-space-3); border-top:var(--bd-line-width) solid var(--zc-line); }
li > span { display:grid; width:2rem; height:2rem; place-items:center; background:var(--zc-city-level); color:var(--zc-accent); font-size:var(--bd-type-overline); font-weight:var(--bd-weight-emphasis); }
li div { display:grid; gap:var(--bd-space-1); }
li strong, aside strong { color:var(--zc-text-strong); }
li p, li small, aside p { font-size:var(--bd-type-overline); }
aside { padding:var(--bd-space-4); border-left:3px solid var(--zc-accent); background:color-mix(in srgb,var(--zc-accent) 7%,var(--zc-card)); }
aside p { margin-top:var(--bd-space-1); }
@media (max-width: 900px) { ol { grid-template-columns:repeat(2,minmax(0,1fr)); } }
@media (max-width: 620px) { header { grid-template-columns:1fr; } .history-strip, ol { grid-template-columns:1fr; } .history-strip span { border-right:0; border-bottom:var(--bd-line-width) solid var(--zc-line); } }
</style>
