# 开发文档（fork 专属）

本目录是 `leewaiho/new-api`（`release/prod` 分支）的**开发者文档**，补充上游 QuantumNous/new-api 自带的项目约定（见仓库根 `AGENTS.md`）。

## 阅读顺序

按下面的顺序读，能最快建立对这个 fork 的整体认知：

1. [FORK_CHANGES.md](./FORK_CHANGES.md) — 我们的 2 个 commit 在做什么、动过哪些文件、风险面在哪
2. [CHANNEL_PROTOCOLS.md](./CHANNEL_PROTOCOLS.md) — 二开核心：「同模型名多协议路由」机制与扩展点
3. [FORK_MAINTENANCE.md](./FORK_MAINTENANCE.md) — 怎么从 upstream 同步、rebase 冲突处理、GHCR 发版
4. [MIGRATE_TO_ADVANCED_CUSTOM.md](./MIGRATE_TO_ADVANCED_CUSTOM.md) — 把"同供应商 Claude 渠道 + OpenAI 渠道"合并成 1 个 Advanced Custom 渠道

## 何时看哪份

| 你要做什么 | 先看 |
|---|---|
| 改 / 加一个渠道（channel） | FORK_CHANGES → CHANNEL_PROTOCOLS |
| 改 / 加一个模型价格或计费规则 | FORK_CHANGES（确认没碰计费路径） |
| 修请求路由 / URL 拼接 bug | CHANNEL_PROTOCOLS |
| 跟 upstream 同步、rebase | FORK_MAINTENANCE |
| 发版、打 tag、推 GHCR | FORK_MAINTENANCE |
| 加新 API 协议类型 | CHANNEL_PROTOCOLS（扩展点章节） |
| 把 N 个 Claude/OpenAI 渠道合并成 1 个 Advanced Custom | [MIGRATE_TO_ADVANCED_CUSTOM.md](./MIGRATE_TO_ADVANCED_CUSTOM.md) |
| 看上游通用约定（代码风格、JSON、DB、i18n、PR 流程） | 仓库根 [AGENTS.md](../../AGENTS.md) |

## 关键事实速记

- **分叉点（merge-base）**：`0977965d933f599b0bbed3ca501b67abce6ce712`（upstream main 上的 "fix: handle ollama non-stream tool calls #5865"）
- **upstream 远程**：`https://github.com/QuantumNous/new-api.git`（已添加为 `upstream`）
- **origin 远程**：`https://github.com/leewaiho/new-api.git`
- **HEAD**：`780ec185` — `ci: add GHCR build workflow for release/prod`
- **二开 commit 数**：2（均作者 weihao.li，均为 Claude Code 生成）
- **未打 tag** — 发布依赖 GHCR `latest` mutable ref
- **没有附带测试文件** — `api_type_resolver` 的规则链是可单测的，未来补测试时优先补这里

## 不要做的事

- 不要修改仓库根 `AGENTS.md` 和 `CLAUDE.md`（受项目保护）
- 不要把 fork 里的 `QuantumNous/new-api` import path 改成 `leewaiho/new-api`（会破坏与 upstream 的 diff 友好性）
- 不要把 `release/prod` 直接 merge 到 upstream main（这是个人发布分支）
- 不要在没有经过测试的情况下手动 rebase 跨大版本（见 FORK_MAINTENANCE 的 rebase 策略）

## 本地工作流小贴士

```bash
# 确认你处在 release/prod
git branch --show-current

# 看 fork 与 upstream 的差异
git fetch upstream
git log --oneline origin/release/prod ^upstream/main
git log --oneline upstream/main ^origin/release/prod

# 跑后端（默认 SQLite 即可）
go run .

# 跑前端
cd web/default && bun install && bun run dev
```

完整命令与依赖请见 [FORK_MAINTENANCE.md](./FORK_MAINTENANCE.md) 与仓库根 `README.md`。
