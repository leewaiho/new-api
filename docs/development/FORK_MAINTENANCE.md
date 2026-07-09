# Fork 维护指南

> 适用：跟进 upstream `QuantumNous/new-api:main`、处理 rebase 冲突、GHCR 发版。

## 1. 当前分叉状态

```bash
$ git remote -v
origin    https://github.com/leewaiho/new-api.git (fetch/push)
upstream  https://github.com/QuantumNous/new-api.git (fetch/push)

$ git rev-parse origin/release/prod upstream/main merge-base
780ec185...   # release/prod HEAD
fc26b88f...   # upstream/main HEAD
0977965d...   # merge-base
```

- **独有 fork commit 数**：2
- **upstream 领先数**（写本文时）：31，**会持续增长**
- 复现命令：

```bash
git log --oneline origin/release/prod ^upstream/main
git log --oneline upstream/main ^origin/release/prod | wc -l
```

## 2. 同步策略

**默认推荐：merge，谨慎 rebase。**

| 策略 | 优点 | 缺点 | 何时用 |
|---|---|---|---|
| `git merge upstream/main` | 不重写历史；冲突一次性解决；CI/GHCR 友好 | 分支图不线性 | **绝大多数情况**，尤其是 release/prod 已发版过 |
| `git rebase upstream/main` | 线性历史；本地开发清爽 | 重写已发布 commit SHA；rebase 中解决冲突要每个 commit 一次 | 仅在 release/prod **没被下游消费过** 时 |
| cherry-pick 单个 upstream commit | 精准引入 | 后续 merge/rebase 还是会冲突 | 只想拿某个具体修复 |

### 2.1 标准 merge 流程

```bash
# 1. 同步 upstream 引用
git fetch upstream

# 2. 切到本地分支（默认就是 release/prod）
git checkout release/prod

# 3. merge
git merge upstream/main --no-ff -m "merge: sync upstream main to release/prod"

# 4. 如有冲突：见 §3
# 5. 跑构建 / 测试
go build ./...
cd web/default && bun install && bun run build

# 6. 推 origin
git push origin release/prod
```

`--no-ff` 保证后续能看到 merge point，回滚方便。push 后 GHCR workflow 自动构建 `latest`。

### 2.2 标准 rebase 流程（仅在 release/prod 还没人拉过时用）

```bash
git checkout release/prod
git rebase upstream/main
# 解决冲突（见 §3）
git push --force-with-lease origin release/prod
```

`--force-with-lease` 避免误覆盖他人提交。

## 3. 冲突高危点（重点 review）

按风险从高到低：

### 🔴 高危：upstream 改了 routing / channel select

我们 fork 的核心就是这一块，upstream 大概率会动到：

- `model/ability.go` 的 `GetChannel` 签名
- `model/channel_cache.go` 的 `GetRandomSatisfiedChannel` 签名
- `service/channel_select.go` 的 `RetryParam`
- `controller/relay.go` 的 `Relay` / `RelayTask` 调用点
- `middleware/distributor.go` 的 `Distribute` 调用点

处理思路：
1. 先按 upstream 改完 `GetChannel` 签名
2. **保留**我们的 `expectedAPIType` 参数与 `filterAbilitiesByExpectedAPIType` 调用
3. 如果 upstream 引入了新的过滤步骤，把 `filterAbilitiesByExpectedAPIType` 串到对应位置

### 🟡 中危：upstream 改了 URL 拼接

- `relay/common/relay_utils.go`（`GetFullRequestURL`）
- `relay/channel/openai/adaptor.go`（`GetRequestURL`）
- `relay/constant/relay_mode.go`（`Path2RelayMode`）

冲突时：
- 保留我们新加的 `BaseUrlHasVersionPrefix` / `StripVersionPrefix` / `normalizeVersionPrefix` 调用
- 在 upstream 改动的合适位置保留这些调用

### 🟢 低危：上游常规迭代

- 计费（`pkg/billingexpr`）
- UI / 前端
- 日志 / 审计
- token 鉴权

这些区域我们没动，merge 时直接采用 upstream 版本即可。

## 4. CI / GHCR 发版

### 4.1 自动触发

`release/prod` 任何 push 都会触发 `.github/workflows/docker-build-ghcr.yml`，镜像推到 `ghcr.io/leewaiho/new-api:latest`。

```bash
# 改动 → 提交 → 推 release/prod → GHCR:latest 自动更新
git push origin release/prod
```

### 4.2 手动指定 tag

GitHub Actions → "Build and push to GHCR" → Run workflow → 输入 tag（如 `v0.1.0-rc.1`），产物：`ghcr.io/leewaiho/new-api:v0.1.0-rc.1`。

### 4.3 拉镜像

```bash
docker pull ghcr.io/leewaiho/new-api:latest
```

### 4.4 发版前 checklist

- [ ] 跑过本地构建 `go build ./...`
- [ ] 前端构建过 `cd web/default && bun install && bun run build`
- [ ] 关键路径单测（如已补 `api_type_resolver_test.go`）通过
- [ ] CHANGELOG / 内部 wiki 记一笔
- [ ] 决定是否打 git tag（见 §5）

## 5. 关于打 git tag

**当前状态：未打 tag**。GHCR 用 `latest` mutable ref。

建议**至少**给每次正式发版打 immutable git tag，便于回滚：

```bash
git tag -a v0.1.0-rc.1 -m "release: v0.1.0-rc.1"
git push origin v0.1.0-rc.1
```

但要注意：`tag` 推送**不会**自动触发 GHCR workflow（只 push 分支会）。打 tag 之后**单独**用 `workflow_dispatch` 触发并指定 tag 重新构建一遍：

```bash
# 在 GitHub UI 或 gh CLI
gh workflow run "Build and push to GHCR" -f tag=v0.1.0-rc.1
```

或考虑加一条 on: push: tags 触发（**改动 CI 文件本身要小心，见 §6**）。

## 6. 改 CI 时的注意点

`.github/workflows/docker-build-ghcr.yml` 是 fork 独有的二开 CI。

- 改 workflow 后**先在分支上测试**，不要直接 push 到 `release/prod` 然后试错（会反复触发构建，污染 GHCR `latest`）
- 如果要加 multi-arch：runner 时间会增加 ~3-4x，建议先用 self-hosted runner
- 如果要加签名：用 `docker/build-push-action` 的 `provenance` 或 `sigstore/cosign-installer`

## 7. 本地开发环境

### 7.1 后端

需要：Go 1.22+、SQLite（默认）或 MySQL/PostgreSQL、Redis（可选，开了内存缓存就不强需）。

```bash
# 拷贝配置
cp .env.example .env

# 跑（首次会自动迁移 + 提示建管理员）
go run .

# 跑单测
go test ./relay/common/... ./model/...
```

### 7.2 前端

需要：Bun（首选）。

```bash
cd web/default
bun install
bun run dev       # 开发
bun run build     # 生产构建
bun run i18n:sync # 同步 i18n 键
```

详细前端约定见 `web/default/AGENTS.md`（项目自带的 skill 之一）。

### 7.3 完整 docker

```bash
docker compose -f docker-compose.dev.yml up
```

## 8. 不在本文档范围

- 项目通用约定（代码风格、JSON、DB 兼容、PR 流程）→ 仓库根 [AGENTS.md](../../AGENTS.md)
- 用户侧文档（安装、配置、面板）→ 仓库根 [README.md](../../README.md) 和 [docs/installation](../../docs/installation/)
- 渠道配置说明 → [docs/channel](../../docs/channel/)
- OpenAPI 规范 → [docs/openapi/](../../docs/openapi/)
