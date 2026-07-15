# NewAPI Tool Compatibility Event + 管理后台交接

> 历史说明：本文保留 Compatibility Event / Admin 的早期设计与实施记录。当前状态、数据库迁移所有权和后续任务以 [NEWAPI_RESPONSES_TOOL_COMPATIBILITY_HANDOFF_20260715.md](./NEWAPI_RESPONSES_TOOL_COMPATIBILITY_HANDOFF_20260715.md) 为准。

更新时间：2026-07-15（UTC+8）
目标：记录 Advanced Custom 工具兼容事件、管理 API、管理后台配置与恢复指导的实现与后续验收要求。

> 本文覆盖数据库 Compatibility Event、管理 API、管理后台 Tool Handling / Model Tool Capabilities 及后续验收要求。核心模型级工具策略、运行时防误删代码、3011 E2E、3010 生产发布和智谱模型级策略迁移均已完成；当前 feature 仍未发布，禁止合并到其他分支。

## 1. 强制边界

称呼用户为“薯条🍟”，默认中文。

Git / 发布规则：

- NewAPI fork 使用 `main-local + release/test + release/prod`。
- feature 必须独立 merge 到各 release，禁止 release 分支互相 merge。
- 核心 Tool Policy feature 已分别合入 `release/test`、`release/prod`，并在生产验收后归档到 `main-local`。
- 后续 Compatibility Event 与管理后台必须创建新的独立 feature，不要继续向已验收的 `feature/202607-responses-tool-filter-fix` 追加实现。
- 新 feature 默认从当前 `main-local` 做基准判定；创建分支前必须重新 fetch 并遵守 `git-workflow`。
- Compatibility Event / 管理后台仍须先独立进入3011验证，未经新授权不得再次修改3010或 `release/prod`。
- 不要删除旧 feature、备份分支或数据库备份表，除非用户明确授权清理。

安全规则：

- 事件中禁止保存 API Key、请求正文、用户消息、工具参数、JSON Schema、Tool arguments、Authorization header。
- 错误内容必须 sanitize，长度必须设上限。
- 只记录 ToolType / ToolName 和必要的定位元数据。

数据库规则：

- 同时兼容 SQLite、MySQL、PostgreSQL。
- NewAPI 自带 schema 使用项目原生 GORM `DB.AutoMigrate(...)`，不要为 `ToolCompatibilityEvent` 再引入 `golang-migrate` 或第二套迁移所有权。
- 优先使用 GORM，不写数据库专属 UPSERT。
- JSON marshal/unmarshal 必须使用 `common/json.go` 包装。

## 2. 当前 Git、发布与环境状态（接手后必须重新验证）

### 已归档的核心 feature

```text
branch: feature/202607-responses-tool-filter-fix
worktree: /home/weihao/projects/new-api-worktree/feature-responses-tool-filter-fix
HEAD: 6e6a55cb fix: add model-aware Responses tool policies
remote: origin/feature/202607-responses-tool-filter-fix
working tree: clean
```

Rebase 后的4个提交：

```text
6e6a55cb fix: add model-aware Responses tool policies
11d8312e fix: preserve client-defined Responses tools
e9ab1889 fix: preserve function tools in Responses passthrough
76422f61 test: capture outbound Responses tools for AC passthrough
```

该 feature 已从干净基准 `fb9612c9` 重放，不包含此前14个旧归档/回滚污染提交。

### 长期分支

```text
main-local:   1e450f80 archive: model-aware Responses tool policies into main-local
release/test: 159c26dc merge: model-aware Responses tool policies into release/test
release/prod: 043e96e2 merge: model-aware Responses tool policies into release/prod
```

三条分支均已 push；feature 是分别独立 merge 到两个 release，release 分支没有互相 merge。

### 3011

```text
URL: http://192.168.200.10:3011
image: ghcr.io/leewaiho/new-api:test
image id: sha256:5fb51a25dfdce847a6d2bce8c5d36d77e23c51eb5ebcd275321ae1cf3559e2fe
status: running / healthy
DB: new-api-test-pg
Redis: new-api-test-redis
```

已完成运行时 fake-upstream E2E：默认 Preserve、冲突去重、模型级 image drop、删空 fail-fast、无效 tool_choice fail-fast均通过。临时渠道和 fake upstream 已清理。

### 3010

```text
URL: http://192.168.200.10:3010
image: ghcr.io/leewaiho/new-api:latest
image id: sha256:07090abb24fc26b4f154332eb459c4c9e6aa768e9af1e5507a6db670317841cf
status: running / healthy
DB: newapi-postgres-1
Redis: newapi-redis-1
```

生产 `zhipu_ac` / `zhipu_gzh_ac` 已迁移为：

```text
Route: namespace=flatten，其他 ToolType 默认 Preserve
conflict: deduplicate
当前8个智谱模型: model-level image_generation=drop
未来新增模型: 默认 Preserve
```

生产与测试的两个智谱渠道 `settings::jsonb` 语义 hash 已对齐。

备份表：

```text
3011: channels_backup_before_model_tool_policy_20260715021439
3010: channels_backup_before_model_tool_policy_20260715022840
```

### 接手检查

```bash
REPO=/home/weihao/projects/new-api
MAIN_WT=/home/weihao/projects/new-api-worktree/main-local
FEATURE_WT=/home/weihao/projects/new-api-worktree/feature-responses-tool-filter-fix
TEST_WT=/home/weihao/projects/new-api-worktree/release-test
PROD_WT=/home/weihao/projects/new-api-worktree/release-prod

git -C "$REPO" fetch origin --prune
git -C "$MAIN_WT" status --short --branch
git -C "$FEATURE_WT" status --short --branch
git -C "$TEST_WT" status --short --branch
git -C "$PROD_WT" status --short --branch
```

## 3. 已完成并上线的核心能力

归档实现位置：

```text
/home/weihao/projects/new-api-worktree/main-local/dto/channel_settings.go
/home/weihao/projects/new-api-worktree/main-local/dto/advanced_custom_tool_policy.go
/home/weihao/projects/new-api-worktree/main-local/relay/channel/advancedcustom/adaptor.go
/home/weihao/projects/new-api-worktree/main-local/service/relayconvert/responses_request_to_chat.go
```

已完成：

- 未配置 Tool Policy 时默认 Preserve。
- `function` 固定 Preserve。
- Route ToolType policy。
- 精确模型 override，模型可匹配 requested model 或 upstream model。
- 模型 + ToolName override。
- 优先级：模型 ToolName > 模型 ToolType > Route > 系统 Preserve。
- 冲突策略：preserve / deduplicate / reject，默认 deduplicate。
- 先过滤明确不支持的 hosted tool，再做冲突去重。
- 所有工具被删除时 fail fast。
- `tool_choice` 指向已删除工具时 fail fast。
- 错误中已有脱敏 channel/route/model/decision 和后台路径提示。

已验证并在3010/3011上线：

```bash
/usr/local/go/bin/go test ./dto ./service/relayconvert ./relay/channel/advancedcustom -count=1
/usr/local/go/bin/go vet ./dto ./service/relayconvert ./relay/channel/advancedcustom
/usr/local/go/bin/go build ./relay/... ./service/...
```

尚未实现：

- Compatibility Event 数据模型、聚合写入和清理策略。
- Compatibility Event 管理 API。
- Tool Handling / Model Tool Capabilities 管理后台。
- Compatibility Issues 事件处理界面。
- 上游 unsupported 错误分类与配置建议闭环。

## 4. 最终产品原则

默认假设渠道和模型支持工具：

```text
function           preserve
namespace          preserve
custom             preserve
web_search         preserve
tool_search        preserve
image_generation   preserve
unknown            preserve
```

明确异常后，只配置：

```text
当前 Channel
+ 当前 AC Route
+ 当前 requested/upstream model
+ 当前 ToolType / ToolName
```

不得把某个模型的错误扩大成整个 Channel 的 Drop。

NewAPI 可以生成配置建议，但不能自动应用。管理员必须确认。

## 5. Compatibility Event 数据设计

### 5.1 已实现模型

已新增：

```text
model/tool_compatibility_event.go
```

推荐字段：

```go
type ToolCompatibilityEvent struct {
    Id              int    `json:"id"`
    EventKey        string `json:"event_key"`        // SHA-256 hex，唯一索引
    ChannelId       int    `json:"channel_id"`       // index
    Route           string `json:"route"`
    RequestedModel  string `json:"requested_model"` // index
    UpstreamModel   string `json:"upstream_model"`
    ToolType        string `json:"tool_type"`
    ToolName        string `json:"tool_name"`
    EventType       string `json:"event_type"`       // index
    CurrentPolicy   string `json:"current_policy"`
    SuggestedPolicy string `json:"suggested_policy"`
    ErrorFingerprint string `json:"error_fingerprint"`
    SanitizedError  string `json:"sanitized_error"`
    OccurrenceCount int    `json:"occurrence_count"`
    FirstSeenAt     int64  `json:"first_seen_at"`
    LastSeenAt      int64  `json:"last_seen_at"`     // index
    ResolutionStatus string `json:"resolution_status"` // open/ignored/resolved
}
```

建议使用独立 `EventKey` 唯一索引，不使用超长复合唯一索引。`EventKey` 由以下字段归一化后计算：

```text
channel_id
route
requested_model
upstream_model
tool_type
tool_name
event_type
error_fingerprint
```

这样更容易兼容 MySQL 索引长度限制。

不要把 `SanitizedError` 放进唯一键。

### 5.2 事件类型

```text
upstream_unsupported
name_conflict
policy_drop
policy_reject
invalid_tool_schema
unclassified
accepted_definition
invoked
```

建议分阶段：

Phase A：先记录本地确定性事件。

- `policy_drop`
- `policy_reject`
- `name_conflict`
- `invalid_tool_schema`

Phase B：再接入上游错误分类。

- `upstream_unsupported`
- `unclassified`

Phase C：正向能力证据。

- `accepted_definition`
- `invoked`

不要一开始同时实现全部分类，避免错误归因。

### 5.3 建议生成规则

明确不支持：

```text
Unsupported tool type: image_generation
→ suggested_policy=drop
```

名称冲突：

```text
image_gen.imagegen conflicts with hosted image_gen
→ suggested conflict policy=deduplicate
```

本地 policy drop/reject：

```text
记录 current_policy
不重复生成“建议改成当前值”
```

模糊 400：

```text
event_type=unclassified
suggested_policy=""
```

禁止仅凭 HTTP 400、Bad Request、invalid request 就生成 Drop 建议。

### 5.4 聚合写入

不要做 provider-specific raw SQL UPSERT。

推荐：

1. 计算 `EventKey`。
2. 使用事务读取现有事件。
3. 存在：更新 `occurrence_count + 1`、`last_seen_at`、最新 sanitized error。
4. 不存在：Create。
5. 遇到唯一索引竞争：重新查询并更新一次。

需要并发测试，避免两个实例重复插入。

### 5.5 迁移入口与显式迁移文件

当前 GORM 迁移注册位置：

```text
model/main.go:migrateDB
model/main.go:migrateDBFast
```

必须同时检查普通迁移与 fast migration 注册列表。

当前 feature 已为 `tool_compatibility_events` 表落地显式迁移文件：

```text
bin/migration_tool_compatibility_events_mysql.sql
bin/migration_tool_compatibility_events_postgres.sql
bin/migration_tool_compatibility_events_sqlite.sql
```

这些 SQL 必须与 `model.ToolCompatibilityEvent` 的字段、索引和默认值保持同步。

不要把 Compatibility Event 放进 `LOG_DB`，除非先明确其生命周期和部署方式。当前放主数据库：它属于管理员配置恢复工作流，不只是请求日志。

## 6. 事件记录接入点

### 6.1 当前本地策略事件

核心运行时已经能产生：

```go
[]relayconvert.ResponsesToolPolicyDecision
```

位置：

```text
relay/channel/advancedcustom/adaptor.go
service/relayconvert/responses_request_to_chat.go
```

建议新增 service 层：

```text
service/tool_compatibility_event.go
```

Adaptor 不直接调用 GORM。Adaptor 只构造脱敏事件 DTO，service 异步或同步写入。

错误路径不能因事件写入失败而吞掉原始 relay 错误：

```text
relay error 优先
事件写入失败只写内部 logger
```

但不要静默假装事件成功。

### 6.2 上游错误分类

先定位 Advanced Custom 上游非 2xx 响应转换入口，再接分类器。不要仅在 adaptor request conversion 阶段猜上游能力。

分类器输入只能使用：

- HTTP status。
- 已 sanitize 且截断的上游 error message。
- 当前 tool decisions/definitions 的 ToolType/ToolName 摘要。

禁止把完整上游响应或请求 body写入事件。

### 6.3 正向能力证据

`accepted_definition`：上游接受带该工具定义的请求，不等于模型真正调用工具。

`invoked`：响应中实际产生对应 tool call，证据更强。

UI 必须区分这两种证据，不能统一显示“支持”。

## 7. 管理 API 设计

推荐新增：

```text
controller/tool_compatibility_event.go
router/api-router.go
```

建议路由组：

```text
GET    /api/tool-compatibility/events
PATCH  /api/tool-compatibility/events/:id/status
POST   /api/tool-compatibility/events/:id/apply-suggestion
POST   /api/tool-compatibility/events/:id/restore-default
```

权限：

- 查询：`AdminAuth`。
- 修改事件状态、应用建议、修改渠道配置：`RootAuth`。

查询参数：

```text
channel_id
route
requested_model
upstream_model
tool_type
event_type
resolution_status
page
page_size
```

写操作要求：

- 重新从数据库读取 Channel，不使用前端提交的完整旧 settings。
- 解析最新 `ChannelOtherSettings.AdvancedCustom`。
- 通过 `incoming_path` 精确定位 Route。
- 只修改目标模型的目标 ToolType/ToolName override。
- 调用现有 `AdvancedCustomConfig.Validate()`。
- 只保存需要变化的 settings 字段。
- 防止覆盖其他管理员刚保存的渠道配置。
- 返回最终有效策略和策略来源。

“应用到整个 Route”属于高级操作，第一版可以不做；默认只能应用当前模型。

## 8. 管理后台设计

### 8.1 当前真实扩展文件

```text
web/default/src/features/channels/types.ts
web/default/src/features/channels/lib/advanced-custom.ts
web/default/src/features/channels/components/dialogs/advanced-custom-editor-dialog.tsx
web/default/src/features/channels/api.ts
web/default/src/i18n/locales/*.json
```

注意：当前 feature 已新增后端 JSON 字段，但前端 TypeScript 类型和 normalizer 尚未同步。

需要新增类型：

```text
responses_tool_conflict_policy
responses_tool_model_overrides
responses_tool_names
```

### 8.2 Tool Handling

AC `/v1/responses` Route 增加 Tool Handling 区域：

- Route ToolType 默认策略。
- Conflict Policy。
- 模型级 override。
- ToolName 级 override。

交互约束：

- Function 显示为锁定 Preserve。
- `converter=none` 不显示/不允许 namespace Flatten。
- `openai_responses_to_openai_chat_completions` 可允许 namespace Flatten。
- `converter=none` 不允许 `responses_drop_fields`。
- 模型 override 只存与 Route 默认不同的字段。
- 同一模型不能出现在多个 override。
- 第一版精确模型名，不做 glob/regex。

### 8.3 Model Tool Capabilities

展示矩阵：

```text
Model × ToolType
```

每格显示：

- 有效策略。
- 策略来源：system / route / model type / model tool name / protected。
- 最近正向证据。
- 最近错误。
- open/ignored/resolved 状态。

不在 Channel.Models 中但仍存在 override 的模型显示 stale warning，不自动删除配置。

### 8.4 Compatibility Issues

展示：

```text
Channel
Route
Requested Model
Upstream Model
ToolType / ToolName
错误次数
最近发生时间
当前策略
建议策略
状态
```

操作：

```text
应用到当前模型
应用到选定模型
忽略建议
标记已解决
恢复当前模型默认支持
恢复 Route 安全默认值
```

默认操作范围必须是当前模型。应用整个 Route 必须明确标为高级操作，并要求二次确认。

### 8.5 对话框上下文问题

当前 `AdvancedCustomEditorDialogProps` 只有：

```text
open
value
onOpenChange
onSave
```

Compatibility Issues 查询需要 `channelId`，模型矩阵需要 Channel.Models。后续应从父级传入：

```text
channelId?: number
channelModels: string[]
```

不要从 JSON value 里猜 channel ID。

## 9. 前端 normalizer 与验证

`advanced-custom.ts` 必须：

- 保留未知未来字段，或至少不要无意删除后端新增字段。
- normalize model names、去空、去重。
- normalize ToolType/ToolName policy。
- JSON raw mode与 visual mode 往返不能丢 override。
- 校验逻辑与后端一致。
- 不把后端 stale model warning 当作保存错误。

需要特别做 round-trip 测试：

```text
parse → visual edit → stringify → parse
```

确认模型 override、ToolName policy、冲突策略不丢失。

## 10. 推荐实施顺序

### Phase 1：后端事件契约

1. Event model + constants。
2. GORM migration 注册。
3. EventKey / ErrorFingerprint / sanitizer。
4. 聚合写入 repository/service。
5. SQLite/MySQL/PostgreSQL 兼容单测。

### Phase 2：本地确定性事件

1. `policy_drop`。
2. `policy_reject`。
3. `name_conflict`。
4. `invalid_tool_schema`。
5. 确认事件写失败不改变 relay 原错误。

### Phase 3：管理 API

1. list/filter/pagination。
2. ignore/resolved。
3. apply suggestion to current model。
4. restore model default。
5. 权限测试与并发更新测试。

### Phase 4：AC Tool Handling UI

1. TypeScript types。
2. normalizer/validator。
3. Route default policy UI。
4. model override UI。
5. Function locked Preserve。
6. stale warning。

### Phase 5：Compatibility Issues UI

1. issue list。
2. model matrix。
3. apply/ignore/resolve/restore actions。
4. i18n。
5. visual verification。

### Phase 6：上游错误分类

只在本地事件/UI稳定后做，避免把模糊错误错误归类为工具不支持。

## 11. 验证计划

### Backend

```bash
/usr/local/go/bin/go test ./model/... ./service/... ./controller/... ./dto/... ./relay/channel/advancedcustom/... -count=1
/usr/local/go/bin/go vet ./model/... ./service/... ./controller/... ./dto/... ./relay/channel/advancedcustom/...
/usr/local/go/bin/go build ./relay/... ./service/... ./controller/... ./model/...
```

必须覆盖：

- EventKey 稳定性。
- 同事件聚合计数。
- 不同模型不聚合。
- 并发插入不产生重复。
- sanitizer 不泄露 secret/body/schema/arguments。
- 模糊 400 不产生 Drop 建议。
- 应用建议只改当前模型。
- Function 不能配置 Drop/Reject。
- restore 不影响同 Channel 其他模型。
- SQLite/MySQL/PostgreSQL migration 字段与索引兼容。

### Frontend

先读取 `web/default/package.json` 获取真实 scripts，不猜命令。至少执行：

- TypeScript typecheck。
- lint。
- build。
- 相关前端单测（如果当前项目有对应 test script）。
- `bun run i18n:sync` 或项目当前 i18n 检查命令。

实际浏览器验证：

- visual/raw mode round-trip。
- 添加模型 override。
- Function 锁定。
- 应用建议。
- 忽略/解决。
- restore model default。
- stale model warning。
- 没有水平滚动、遮挡或弹窗越界。

### 3011 E2E

只有再次确认该 feature 已允许合入 test 后执行：

1. 使用独立测试数据库，不能连接 3010 生产数据库。
2. 部署 3011。
3. 验证 migration。
4. 使用测试账号/Token。
5. 验证火山、智谱、ChatGPTPlus AC Route。
6. 触发明确 unsupported 错误，检查事件和建议。
7. 后台应用到当前模型。
8. 重试请求验证恢复。
9. 确认同渠道其他模型未变化。
10. 检查日志无 secret、panic、fatal。

## 12. 当前已上线渠道策略与后续目标

### huoshan_ac

```text
converter=none
Route 默认 Preserve
conflict=deduplicate
明确异常后添加模型 override
```

### zhipu_ac / zhipu_gzh_ac

```text
converter=openai_responses_to_openai_chat_completions
namespace=flatten
其他默认 Preserve
明确不支持 image generation 的模型单独 image_generation=drop
```

不要继续使用 route-wide：

```text
custom=drop
web_search=drop
tool_search=drop
image_generation=drop
unknown=drop
```

### CliProxyAPI_ac / mycyjg_ac

```text
converter=none
Route 默认 Preserve
conflict=deduplicate
明确异常后增加模型 override
```

## 13. 尚未获得用户决策的事项

不要自行决定：

1. Compatibility Event 保留周期和自动清理策略。
2. `SanitizedError` 最大长度，建议先提 1–4 KiB 方案供用户确认。
3. 是否在第一版记录 `accepted_definition` / `invoked` 正向事件。
4. Compatibility Issues 放在 AC 编辑器内部还是独立管理页面。
5. Classic 前端是否同步实现完整 UI，还是只保证 JSON 配置兼容。
6. 是否允许“应用到整个 Route”的高级操作。
7. 是否需要导出 Compatibility Event CSV。

## 14. Suggested skills

接手时建议加载：

- `git-workflow`
  - 必须同时读取 `references/newapi-fork-development.md`。
- `diagnosing-bugs`
  - 上游错误分类与真实不支持判断。
- `codebase-design`
  - Event repository/service/API 边界。
- `domain-modeling`
  - 固化 ToolType、ToolName、Capability Evidence、Suggestion、Resolution 等术语。
- `vercel:react-best-practices`
  - 修改多个 TSX 组件后检查前端质量。
- `vercel:agent-browser-verify` 或 `vercel:verification`
  - 3011 管理后台与 API 完整流程验证。

## 15. 下一会话推荐起步

```bash
# 1. 只读核对当前状态
REPO=/home/weihao/projects/new-api
MAIN_WT=/home/weihao/projects/new-api-worktree/main-local

git -C "$REPO" fetch origin --prune
git -C "$MAIN_WT" status --short --branch
git -C "$MAIN_WT" log -8 --oneline --decorate

# 2. 读取已归档核心实现，不重复开发 Tool Policy
sed -n '1,360p' "$MAIN_WT/dto/advanced_custom_tool_policy.go"
rg -n 'ResponsesToolPolicyDecision|ApplyResponsesToolPolicies|advancedCustomResponsesToolPolicyError' \
  "$MAIN_WT/relay/channel/advancedcustom" \
  "$MAIN_WT/service/relayconvert"

# 3. CodeGraph 定位 migration、model/service、API、UI 调用链
# 使用 codegraph_context，任务聚焦 ToolCompatibilityEvent + AdvancedCustom admin UI。
```

新的 Compatibility Event / 管理后台工作必须使用新的 feature 分支。创建前先按 `git-workflow` 展示基准判定，建议目标名称：

```text
feature/202607-tool-compatibility-events-admin
```

第一项实际开发工作只做：

```text
Event model
+ sanitizer
+ EventKey
+ 聚合 service
+ 跨 SQLite/MySQL/PostgreSQL 单测
```

不要在第一阶段同时启动 API、Default UI、Classic UI 和上游错误分类。
