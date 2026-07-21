# NewAPI Responses Tool Compatibility 配置化交接

更新时间：2026-07-15（UTC+8）

## 0. 运行环境修正（2026-07-21）

本文中的 `3011` 固定指 homelab `192.168.200.10` 上的测试端点，而不是本地 WSL：

```text
3010（生产）：http://192.168.200.10:3010
3011（测试）：http://192.168.200.10:3011
测试目录：/opt/newapi-test（ssh homelab）
```

创建、重建或重新基线化 3011 时，必须先启动其独立 test stack，然后在 `/opt/newapi-test` 使用 3010 PostgreSQL 快照恢复 3011 测试库：

```bash
./sync-db-from-prod.sh
docker compose restart new-api-test
```

恢复会覆盖 3011 测试数据；之后的 E2E 配置和临时 Token 只能写入测试库，禁止直连、修改或复用 3010 生产数据库。完整流程见 [scripts/deploy/README.md](../../scripts/deploy/README.md)。

## 1. 当前 Git 状态

开发分支：

```text
feature/202607-tool-compatibility-config
```

worktree：

```text
/home/weihao/projects/new-api-worktree/feature-202607-tool-compatibility-config
```

基准与已有提交：

```text
main-local: 350e8963
feature HEAD: b0a43222 merge: compatibility admin onto current main-local
```

当前功能改动仍未 commit、未 push、未合入 `release/test` / `release/prod`，也未部署 3011/3010。

发布边界：未经新的明确授权，不得合入 release、部署、修改测试/生产数据库或删除分支/worktree。

## 2. 已完成实现

### 2.1 `responses_implicit_hosted_tools` 管理后台

后端原有 Route/模型级解析已接入 Default 前端：

```json
{
  "responses_implicit_hosted_tools": ["image_generation"],
  "responses_tool_model_overrides": [
    {
      "models": ["gpt-5.6-sol"],
      "responses_implicit_hosted_tools": ["image_generation"]
    }
  ]
}
```

管理后台当前提供受控 capability：

```text
image_generation
web_search
```

UI 展示有效来源：

```text
system default (none)
route
model
route + model
```

前后端均拒绝以下危险组合：

```text
responses_tool_conflict_policy = preserve
+
Route 或模型级 responses_implicit_hosted_tools 非空
```

隐式 hosted capability 必须配合 `deduplicate` 或 `reject`。

### 2.2 GLM `web_search` 参数兼容配置化

新增白名单配置：

```json
{
  "responses_tool_parameters": {
    "web_search": {
      "when_nested_options_missing": "populate_defaults",
      "defaults": {
        "enable": true,
        "search_result": true,
        "search_engine": "search_std"
      }
    }
  }
}
```

模型级覆盖：

```json
{
  "responses_tool_model_overrides": [
    {
      "models": ["glm-5.2"],
      "responses_tool_parameters": {
        "web_search": {
          "when_nested_options_missing": "populate_defaults",
          "defaults": {
            "enable": true,
            "search_result": true
          }
        }
      }
    }
  ]
}
```

行为约束：

- Route 支持 `preserve / populate_defaults`；
- 模型支持 `inherit / preserve / populate_defaults`；
- 只在嵌套 `web_search` 参数缺失或为空时补默认值；
- 客户端显式参数优先，不覆盖；
- 可配置字段仅限 `enable`、`search_result`、`search_engine`；
- 仅允许 `openai_responses_to_openai_chat_completions` converter 使用；
- 不支持任意 JSON Patch。

已移除基于 `strings.HasPrefix(model, "glm-")` 的硬编码补参。默认无配置时不修改请求。

### 2.3 Compatibility Event 数据库所有权

`ToolCompatibilityEvent` 使用 NewAPI 自身 GORM `DB.AutoMigrate(...)` 注册流程。

本 feature 删除了三份额外 SQL migration：

```text
bin/migration_tool_compatibility_events_mysql.sql
bin/migration_tool_compatibility_events_postgres.sql
bin/migration_tool_compatibility_events_sqlite.sql
```

不要再引入 `golang-migrate` 或第二套 schema 所有权。NewAPI 自带 schema 由 NewAPI 原生迁移机制负责。

### 2.4 OCI revision 标签

`.github/workflows/docker-build-ghcr.yml` 已增加：

```text
org.opencontainers.image.revision
org.opencontainers.image.source
org.opencontainers.image.created
```

当前文件对应 `release/prod` 的生产构建，可让 3010 镜像直接确认 Git revision。3011 使用的 `docker-build-ghcr-test.yml` 是 `release/test` 专属文件，不存在于本 feature 基准；获得测试发布授权后，必须在 `release/test` 独立补同样标签。当前尚未通过真实 GitHub Actions release 构建验证。

## 3. Tool Alias Mapping 的准确状态

通用、管理员可编辑的 Tool Alias Mapping **尚未实现**。

当前 `image_gen.imagegen ↔ image_generation` 的处理依赖：

1. 内置 image capability canonicalization；
2. `responses_implicit_hosted_tools=["image_generation"]`；
3. `responses_tool_conflict_policy=deduplicate`。

因此当前后台能声明“上游隐式提供 image_generation”，但不能新增任意：

```text
function/namespace 名称 → hosted capability
```

通用 Alias Mapping 属于后续 P1，安全设计见 `NEWAPI_IMAGE_GEN_HANDOFF_20260715.md`。

## 4. 已完成验证

后端已通过：

```bash
/usr/local/go/bin/go test ./dto ./service/relayconvert ./relay/channel/advancedcustom -count=1
/usr/local/go/bin/go vet ./dto ./service/relayconvert ./relay/channel/advancedcustom
/usr/local/go/bin/go build ./relay/... ./service/...
```

前端配置单测当前通过：

```text
11 pass
0 fail
```

覆盖：

- 配置 JSON round-trip；
- Route/模型 implicit hosted tools；
- `responses_tool_parameters`；
- Compatibility mutation merge；
- native converter 拒绝参数规则；
- Route `preserve` 与模型级 implicit hosted tools 冲突校验；
- 客户端显式 `web_search` 参数不覆盖。

当前 worktree 安装锁定依赖后，`rsbuild build` 已成功。改动文件 oxlint 无 error，仅有旧的 `structuredClone` 建议。完整 TypeScript typecheck 存在任务开始前已有的无关基线错误：

```text
src/features/models/components/drawers/model-mutate-drawer.tsx
Property 'AutomaticDisableIgnoreKeywords' is missing in type ModelSettings
```

本地浏览器 smoke check 已确认静态 bundle 加载且无 JS/console error；由于临时 dev server 没有后台代理，`/api/status` 和 `/api/setup` 请求中止，页面无法进入登录后的管理后台。因此保存/重开交互仍属于 P0。

## 5. 发布前必须完成

### P0

1. 获得 `release/test` 修改授权后，为 `docker-build-ghcr-test.yml` 补齐同样的 OCI revision/source/created 标签。
2. 浏览器实际检查 Advanced Custom 编辑器：
   - Route implicit hosted tools 保存、刷新和重开；
   - 模型 override 保存、刷新和重开；
   - `web_search` defaults 编辑；
   - JSON/可视化模式切换不丢字段；
   - 控制台无错误。
3. 在发布 3011 前，为实际 GLM Route/模型准备明确配置；新代码不再自动按 `glm-*` 补参。
4. 获得用户明确授权后，才可独立 merge 到 `release/test` 并部署 3011。
5. 3011 真实 E2E：
   - `gpt-5.5`；
   - `gpt-5.6-sol`；
   - GLM `web_search` 空嵌套参数补全；
   - 客户端显式参数不覆盖；
   - 未命中模型保持原样。

### P1

1. 通用、严格精确匹配的 Tool Alias Mapping。
2. 参数类 Compatibility Event 分类和建议：

   ```text
   web_search 参数不能为空
   → 建议为 channel + route + model 配置 defaults
   ```

3. 管理员确认前只生成配置建议，不自动应用。

## 6. 主要改动文件

后端：

```text
dto/advanced_custom_tool_policy.go
dto/channel_settings.go
dto/channel_settings_test.go
relay/channel/advancedcustom/adaptor.go
relay/channel/advancedcustom/adaptor_test.go
service/relayconvert/responses_request_to_chat.go
service/relayconvert/responses_request_to_chat_test.go
```

前端：

```text
web/default/src/features/channels/types.ts
web/default/src/features/channels/lib/advanced-custom.ts
web/default/src/features/channels/lib/advanced-custom.test.ts
web/default/src/features/channels/components/dialogs/advanced-custom-editor-dialog.tsx
web/default/src/i18n/locales/en.json
web/default/src/i18n/locales/zh.json
```

CI / 数据库迁移清理：

```text
.github/workflows/docker-build-ghcr.yml
bin/migration_tool_compatibility_events_*.sql (deleted)
```

## 7. 接手命令

```bash
WT=/home/weihao/projects/new-api-worktree/feature-202607-tool-compatibility-config

git -C "$WT" status --short --branch
git -C "$WT" diff --stat
git -C "$WT" diff --check
```

测试使用固定本地工具路径：

```bash
/usr/local/go/bin/go
/home/weihao/.bun/bin/bun
```

禁止触碰仓库其他 worktree 中的 `.codegraph/`、`CONTEXT.md` 或用户未提交内容。
