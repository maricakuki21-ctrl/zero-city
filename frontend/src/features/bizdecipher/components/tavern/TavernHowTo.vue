<template>
  <details class="tavern-how-to">
    <summary>第一次来？一分钟看懂酒馆玩法</summary>
    <div class="tavern-guide-grid">
      <section><h2>一个人，马上玩</h2><p>选择下方初始故事和角色，开始免费本地体验。每幕阅读剧情、查看线索，再做一次决定；行动会留下回应，关键选择会改变结局。进度自动保存在当前浏览器，可导出复盘。</p><small>不联网调用 AI，不扣积分和余额；不是多人联机房间。</small></section>
      <section><h2>和朋友，开一局</h2><p>选择公开剧本 → 创建房间 → 开放房间 → 成员加入 → 房主开始 → 进入舞台记录行动 → 房主结束。入场费用以房间显示为准，付费入场须先确认。</p><small>舞台通过刷新读取回合，不是实时语音游戏。AI 主持需要房主选择可用模型资源，实际调用另行计费；免费本地故事不受影响。</small></section>
      <section><h2>做自己的故事</h2><p>在「投稿与我的剧本」填写故事、开场和角色信息，绑定并发布有效的声明式游戏包，再从上架剧本开房。新房间固定使用创建时的包版本。</p><small>目前不是任意代码插件执行器。参与说明、包格式和公开测试账号可从共建仓库查看。</small><a href="https://github.com/maricakuki21-ctrl/zero-city/blob/main/docs/community/tavern-play-guide.md" target="_blank" rel="noopener noreferrer">查看玩法与共建说明 ↗</a></section>
    </div>
  </details>
</template>
<style scoped>
.tavern-how-to { border: 1px solid var(--bd-ui-line); border-radius: 10px; padding: 14px 18px; background: var(--bd-surface); color: var(--bd-text-primary); }
summary { cursor: pointer; font-size: 14px; font-weight: 650; color: var(--bd-accent-teal); }
.tavern-guide-grid { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 24px; padding-top: 18px; }
h2 { font-size: 15px; font-weight: 650; margin-bottom: 8px; } p { font-size: 13px; line-height: 1.9; color: var(--bd-text-secondary); }
small { display: block; font-size: 12px; line-height: 1.8; color: var(--bd-text-muted); margin-top: 10px; } a { display: inline-block; margin-top: 10px; font-size: 13px; color: var(--bd-accent-teal); }
@media(max-width: 760px) { .tavern-guide-grid { grid-template-columns: 1fr; gap: 18px; } }
</style>
