# 零号城 · Zero City

AI 创作、共享资源和社区共建。这个仓库是 **2026-09-26 正式运行版本的整理基线**，不是 7 月的旧仓库。

## 参与共建

1. 在 [提案区](../../issues/new/choose) 描述问题、截图和希望发生的变化。
2. 会写代码：Fork → 创建分支 → 提交 Pull Request，描述“如何验证”。
3. 不写代码：体验测试站，在对应 PR 下反馈复现步骤、截图与测试结果。
4. PR 会自动运行检查。满足范围要求的文档改进可自动合并；行为、资金、鉴权、依赖、迁移和部署修改进入维护者验收。

**不要提交用户信息、真实密钥或数据库文件。** 测试环境不承诺真实模型和支付可用，也不使用正式用户数据。

## 测试入口

见 [TESTING.md](TESTING.md)。测试服只是体验环境，不是正式服务，不要往里面充值。

## 源码与正式版本

| 部分 | 基线 |
|---|---|
| 后端 | 正式 `city-arrival-20260926` 构建源，保留生产热修复和原迁移文件 |
| 前端 | 对应同日进城首页、共享市场与分组改版 |
| Harness | 正式运行容器的 `src` 与依赖清单 |
| 用户数据、密钥、支付配置、运维备份 | **不在仓库内** |

历史项目许可见 [LICENSE](LICENSE)。原有版权与来源说明保留；仓库内品牌与角色素材不表示可以冒充官方运营。

## 开发

```bash
corepack enable
cd frontend
pnpm install --frozen-lockfile
pnpm typecheck
pnpm build
cd ../backend
go build -buildvcs=false -tags embed -o sub2api ./cmd/server
```

Node 版本见 `.node-version`，Go 版本见 `backend/go.mod`。后端第一次启动需要独立 PostgreSQL、Redis 和初始化配置。
也可以使用 `deploy/compose.demo.yml` 启动一套独立演示环境；不要把生产配置复制进演示环境。

## 自动化边界

- PR 检查用 GitHub 托管临时 runner，不使用带生产凭据的自托管 runner。
- 有权限的自动合并流程**不检出、不运行贡献者代码**，只检查 GitHub 返回的检查结果和完整改动清单。
- 目前自动合并仅限 `docs/community/*.md` 的新增或修改；工作流、测试报告和支付逻辑不能通过改名混入。
- 样式也可能隐藏价格或确认提示，因此不会仅凭 `.css` 扩展名就自动上线。
- 自动合并不等于自动正式发布。正式发布需同一份已验收构建产物、迁移兼容及回退条件；未接通的自动生产发布不会假装已启用。

详细规则见 [CONTRIBUTING.md](CONTRIBUTING.md)。
