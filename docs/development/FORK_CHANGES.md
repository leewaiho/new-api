# Fork 改动清单

> 范围：`origin/release/prod` 相比 `upstream/main` 的**独有 commit**。
> merge-base：`0977965d933f599b0bbed3ca501b67abce6ce712`

## 概览

| Commit | 作者 | 类型 | 摘要 |
|---|---|---|---|
| `884b5bbc` | weihao.li | feat | 同模型名多协议路由 |
| `780ec185` | weihao.li | ci   | GHCR 构建 release/prod 镜像 |

两个 commit 都是 AI 辅助生成（commit message 含 `Co-Authored-By: Claude / Happy`）。

---

## ① `884b5bbc` feat: route requests to channels matching client API protocol

**目标**：同一个模型名可以同时挂 OpenAI 协议渠道和 Claude 协议渠道；请求到达时按客户端协议（路径 + UA）推断期望 API 类型，优先选原生协议匹配的渠道。顺便兼容上游 baseURL 已带 `/v2`、`/v3` 等版本前缀的情况。

**改动面**：9 个文件，+284 / -23 行。

### 新增文件

- `relay/common/api_type_resolver.go`（+89 行）
  - 抽象 `RequestSignature{Path, UserAgent, Headers}` 便于离线推断和单测
  - 规则链 `ExpectedAPITypeChain`（`PathAPITypeRule` → `UserAgentAPITypeRule`）
  - 全局变量 `CurrentAPITypeInferenceChain` 可被替换 / 扩展

### 改动文件

| 文件 | 关键改动 |
|---|---|
| `model/ability.go` | `GetChannel` 增 `expectedAPIType *int` 参数；新增 `filterAbilitiesByExpectedAPIType`（Advanced Custom 渠道永远保留；无匹配时回退原列表） |
| `model/channel_cache.go` | `GetRandomSatisfiedChannel` 同步增参；新增 `filterChannelsByExpectedAPIType`（内存缓存路径） |
| `service/channel_select.go` | `RetryParam` 增 `ExpectedAPIType *int`；新增 `InferExpectedAPITypeFromContext(c)` |
| `controller/relay.go` | `Relay` / `RelayTask` 入口填入 `ExpectedAPIType` |
| `middleware/distributor.go` | `Distribute` 入口填入 `ExpectedAPIType` |
| `relay/constant/relay_mode.go` | `Path2RelayMode` 入口先 `normalizeVersionPrefix`；支持 `/v2`、`/v3` 路径 |
| `relay/common/relay_utils.go` | 新增 `BaseUrlHasVersionPrefix`、`StripVersionPrefix`；`GetFullRequestURL` 处理 baseURL 已含版本前缀且 requestURL 是 `/v1` 开头时去掉 `/v1` |
| `relay/channel/openai/adaptor.go` | Azure OpenAI 任务路径用 `StripVersionPrefix`（原硬编码 `/v1`）；Claude/Gemini 走 OpenAI 适配器时若 baseURL 已带版本前缀则返回 `baseURL/chat/completions` 不再附加 `/v1` |

### 行为保证

- `expectedAPIType == nil` 时，**与原版完全等价**（调用方没传 → 推断失败 → 走原逻辑）
- 没有匹配渠道时**回退到原候选列表**，不会"突然无可用渠道"
- Advanced Custom 渠道**永远保留**（可配多协议，不能被过滤掉）
- 没有附带单元测试 — `api_type_resolver` 的规则链是天然可单测的，未来补测试时优先补

### 影响面（要小心的区域）

✅ **未触碰**：
- 计费 / 配额（`pkg/billingexpr`、`service/quota*`、`model/ability` 计费相关字段）
- token 鉴权 / 用户 / 分组（`service/authz`、`middleware/auth*`）
- 日志 / 审计
- 前端

⚠️ **触碰但语义未变**：
- `model/ability.go` — 选 channel 多了过滤步骤，但入参为 `nil` 时等价原版
- `model/channel_cache.go` — 同上
- `relay/common/relay_utils.go` — `GetFullRequestURL` 行为微调（baseURL 带版本前缀时不重复加 `/v1`）

🚧 **行为变更**（需重点回归）：
- **同模型名多协议渠道混挂**的场景：以前随机选一个；现在按请求协议优先选。属于新能力引入，但若运营侧一直在依赖"完全随机"，需要确认
- **baseURL 自带 `/v2` / `/v3` 的上游**：以前会拼出 `baseURL/v1/chat/completions`（重复）；现在拼出 `baseURL/chat/completions`。如果某些上游其实要求 `/v1`，就会出错。已知安全的上游：`/v2`/`/v3` 的 Anthropic、部分自部署网关

---

## ② `780ec185` ci: add GHCR build workflow for release/prod

**目标**：`release/prod` push 或手动 `workflow_dispatch` 时构建 `linux/amd64` 镜像推到 `ghcr.io/leewaiho/new-api:<tag>`。

**新增文件**：

- `.github/workflows/docker-build-ghcr.yml`（+69 行）
  - 触发：`push` 到 `release/prod` / `workflow_dispatch`（可指定 tag）
  - 镜像：`ghcr.io/leewaiho/new-api`，默认 `latest`
  - runner：`ubuntu-24.04`
  - 缓存：`type=gha` + `mode=max`
  - 权限：`packages: write`（推 GHCR 必需），用默认 `GITHUB_TOKEN`，**无需额外 secrets**

**没有**：
- 没有 multi-arch（仅 amd64；arm 机器要跑要本地 build）
- 没有 SBOM / cosign 签名
- 没有触发 tag 自动递增（每次 push 都覆盖 `latest`）
- 没有 nightly / release-please 类自动发版

---

## 整体风险评估

| 维度 | 状态 |
|---|---|
| 与 upstream 的同步友好性 | ✅ 高（diff 集中在 9 个文件 + 1 个 CI 文件） |
| 回归测试覆盖 | ⚠️ 无新增测试；建议补 `relay/common/api_type_resolver_test.go` |
| 行为兼容性 | ✅ `expectedAPIType == nil` 路径完全等价 |
| 文档 | ⚠️ 本文是首批；尚无对运营侧的"如何混挂多协议渠道"操作文档 |
| 发布可追溯性 | ⚠️ 无 tag；建议至少打 `vYYYY.MM.DD-<short-sha` 形式 |
| 镜像产物 | ⚠️ 单 arch、无签名，发布面有限（个人 / 内部） |
