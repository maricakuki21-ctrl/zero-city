# Harness 创作执行适配器

状态（2026-09-08 21:33）：服务器入口已恢复，前端与接口实测通过；
真实付费模型未验证。先读仓库根 `FRONTEND_EXECUTION.md`。

## 执行路径

Vue `/operator` → 现有 Go JWT 路由 → loopback Node 服务 → 官方 SDK →
本站 `/v1` 网关。JWT 用于服务端查询当前用户及该用户拥有的 Key。
前端只提交 Key ID。规划模型与图片模型可以使用不同 Key。
用户余额和网关用量继续由原系统处理，本服务不维护第二份账本。

每个任务创建独立 Harness home/session，并关闭官方 SDK 的子进程。
技能为受控平台注册项：商品图片设计师、短视频分镜师。
`generate_image` 每任务至多尝试一次。当前只支持网关返回的图片 URL，
不支持纯 base64 返回、图片编辑参考素材和视频生成。

## 当前限制

- 对话界面当前是一任务一运行，不保留跨任务模型上下文。
- 浏览器断开会取消运行；尚未提供后台任务恢复与事件重放。
- 历史草稿存在当前浏览器；SDK 会话日志存在 `.runtime`，不对公网开放。
- 原生插件部署由平台管理，未开放用户代码上传。
- 技能免费，真实模型费用按所选网关资源；没有独立工具收费。
- 尚未验证真实双用户运行隔离和平台付费生图。
- 不使用原装 Web 登录作为本站用户鉴权。

## 测试

`node --test test/*.test.mjs`：目录选择、媒体请求、失败结果。
`node test/runtime-smoke.mjs`：真正启动官方 SDK/代理循环，
对本地模拟模型网关执行技能加载和图片调用，检查工具集合。
Windows smoke 禁用文件会话持久化以避开未编译的 fs-ext；
Linux smoke 保留真实文件会话持久化，需构建 fs-ext。

`test/live-probe.mjs` 仅在已授权测试服务器运行，读取既有测试凭据，
输出脱敏的资源列表，不执行付费调用。
`test/session-login.mjs` 只供 QA 父进程内存接收凭据，禁止把 stdout
记录到部署日志、聊天或提交文件。

## 部署

安装锁定依赖后，在 Linux 执行 `npm rebuild fs-ext --foreground-scripts`。
`deploy.service` 以专用低权限用户启动，仅开放 loopback 18082，
通过现有 18081 服务转发，不另开公网端口。
Go 服务必须带 `-tags embed` 构建，否则网页不可用。
使用 `outputs/recover-harness-web.sh` 将嵌入体积限制为启动 HTML，
完整静态文件保持在现有 `backend/data/public`。脚本与前端发布共用锁。
通过 systemd scope 限制编译内存；后端和前端构建串行运行。

任何重新发布前，先确认没有上一次尚未结束的构建。
