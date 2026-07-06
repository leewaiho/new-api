# 同模型名多协议路由机制

> 适用版本：`origin/release/prod` HEAD（`780ec185`）
> 引入 commit：`884b5bbc` feat: route requests to channels matching client API protocol
> 相关源文件：`relay/common/api_type_resolver.go`、`model/ability.go`、`model/channel_cache.go`、`service/channel_select.go`、`relay/constant/relay_mode.go`、`relay/common/relay_utils.go`、`relay/channel/openai/adaptor.go`、`controller/relay.go`、`middleware/distributor.go`

## 1. 问题与目标

**问题**：new-api 一个模型名（如 `claude-sonnet-4-5`）下挂的渠道只能绑一个原生协议。运营侧常见需求：

- 一个模型名既要有 Anthropic 协议的官方渠道，又要有 OpenAI 协议的中转渠道
- 上游 baseURL 形如 `https://api.example.com/v2`（已带版本前缀），再拼 `/v1/...` 会变成 `/v2/v1/...` 而报错

**目标**：
1. 同一个模型名下不同渠道可以有不同原生协议（OpenAI / Anthropic / Gemini / …）
2. 请求到达时按**客户端协议**（路径 + UA）推断期望 API 类型，**优先选原生协议匹配的渠道**
3. 兼容 baseURL 自带版本前缀（`/v2`、`/v3` …）的上游

## 2. 全链路概览

```
HTTP 请求
  │
  ▼
router/api-router.go（路径分派）
  │
  ▼
controller/relay.go: Relay()               ◀── 填 ExpectedAPIType
  │                                          │
  ▼                                          │
service/channel_select.go:                   │
  CacheGetRandomSatisfiedChannel(RetryParam) │
  │                                          │
  ├─ InferExpectedAPITypeFromContext(c) ─────┘
  │   └─ relay/common/api_type_resolver.go: ResolveExpectedAPIType
  │       ├─ PathAPITypeRule   (路径前缀 → APIType)
  │       └─ UserAgentAPITypeRule (UA 包含 → APIType)
  │
  ▼
model/channel_cache.go: GetRandomSatisfiedChannel
  │  （或 model/ability.go: GetChannel，缓存关闭时）
  │
  ├─ filterChannelsByExpectedAPIType          ◀── 新增过滤步骤
  │   ├─ 若 expectedAPIType == nil → 原列表
  │   ├─ 跳到 channel type == Advanced Custom → 永远保留
  │   ├─ 跳到 ChannelType2APIType(ch.Type) == expectedAPIType → 保留
  │   └─ 过滤后空 → 回退原列表（保证有可用渠道）
  │
  ▼
返回 *model.Channel
  │
  ▼
relay/channel/<provider>/adaptor.go: GetRequestURL
  │  GetFullRequestURL(baseURL, requestURL, channelType)
  │   ├─ 若 baseURL 已含版本前缀 & requestURL 以 /v1 开头 → 去掉 /v1
  │   ├─ Cloudflare gateway 特殊处理（trim /v1 / /openai/deployments）
  │   └─ 其它 → baseURL + requestURL
```

## 3. 协议推断规则（`relay/common/api_type_resolver.go`）

```go
var CurrentAPITypeInferenceChain ExpectedAPITypeChain = DefaultAPITypeInferenceChain()

func DefaultAPITypeInferenceChain() ExpectedAPITypeChain {
    return ExpectedAPITypeChain{
        PathAPITypeRule{},
        UserAgentAPITypeRule{},
    }
}
```

两条规则，**先路径后 UA**，短路返回：

### `PathAPITypeRule`（精确匹配）

| 路径前缀 | APIType |
|---|---|
| `/v1/messages` | `APITypeAnthropic` |
| `/v1/chat/completions` | `APITypeOpenAI` |
| `/v1/completions` | `APITypeOpenAI` |
| `/v1/responses` | `APITypeOpenAI` |
| `/v1/embeddings` | `APITypeOpenAI` |
| `/v1/rerank` | `APITypeOpenAI` |
| `/v1beta/models/` | `APITypeGemini` |

其它路径返回 `(_, false)`，交给下一条规则。

### `UserAgentAPITypeRule`（包含匹配，兜底）

| UA 包含 | APIType |
|---|---|
| `claude-cli` 或 `anthropic` | `APITypeAnthropic` |
| `codex` 或 `openai` | `APITypeOpenAI` |

兜底是为了**客户端不带显式协议版本路径**时也能识别（如某些 CLI 直接打 `/messages` 配自定义 UA）。

> **注意**：`/v1beta/models/...` 只识别 Gemini，UA 规则里**没有** Gemini 兜底（UA 难以稳定识别 Gemini）。如果需要，加一条 `UserAgentAPITypeRule` 子规则或加路径前缀。

## 4. 渠道过滤

两个等价实现（一个走内存缓存，一个走 DB）：

| 缓存开 | 调用 | 函数 |
|---|---|---|
| `MemoryCacheEnabled = true` | `model/channel_cache.go` | `filterChannelsByExpectedAPIType` |
| `MemoryCacheEnabled = false` | `model/ability.go` | `filterAbilitiesByExpectedAPIType` |

逻辑相同：

```text
if expectedAPIType == nil:
    return 原列表
matched := []
for ch in 原列表:
    if ch.Type == ChannelTypeAdvancedCustom:
        matched.append(ch)        # Advanced Custom 永远保留
    elif ChannelType2APIType(ch.Type) == *expectedAPIType:
        matched.append(ch)
if matched.isEmpty():
    return 原列表                  # 兜底：避免"无渠道可用"
return matched
```

`ChannelType2APIType` 映射见 `common/api_type.go`。**不在映射表中的 channel type**（含未识别的自定义类型）会被过滤掉；如果全部被过滤，会回退到原列表。

## 5. baseURL 版本前缀处理

`relay/common/relay_utils.go` 新增两个工具：

```go
// baseURL 的 path 段里是否含 vN 形式（如 /v2、/v3）
func BaseUrlHasVersionPrefix(baseURL string) bool

// 把 /vN/xxx 剥成 /xxx（只处理单段前缀，且 v 后必须是数字）
func StripVersionPrefix(path string) string
```

应用点：

- `GetFullRequestURL`：baseURL 含版本前缀 **且** requestURL 以 `/v1` 开头时，剥掉 `/v1` 再拼。**避免** `https://api.example.com/v2` + `/v1/chat/completions` → `/v2/v1/chat/completions`
- `relay/channel/openai/adaptor.go` Azure 任务路径：用 `StripVersionPrefix` 替掉原来硬编码的 `strings.TrimPrefix(..., "/v1/")`
- `relay/constant/relay_mode.go` `Path2RelayMode`：入口先 `normalizeVersionPrefix` 把 `/v2`、`/v3` 路径归一为 `/v1`，让上游的 relayMode 推断保持兼容

## 6. 调用方与参数传递

`service.RetryParam` 新增字段：

```go
type RetryParam struct {
    Ctx             *gin.Context
    TokenGroup      string
    ModelName       string
    RequestPath     string
    ExpectedAPIType *int   // 新增：可空，nil 时等价原行为
    Retry           *int
    resetNextTry    bool
}
```

在两个入口处填充：

- `controller/relay.go: Relay`、`RelayTask`
- `middleware/distributor.go: Distribute`

填充逻辑（同一份）：

```go
ExpectedAPIType: service.InferExpectedAPITypeFromContext(c)
```

`InferExpectedAPITypeFromContext` 推断失败返回 `nil`，**不阻断路由**。

## 7. 扩展点

### 7.1 加新的 API 协议类型

场景：要把 Gemini 加入"自动按请求协议优选"机制。

1. 在 `constant/api_type.go` 增加 `APITypeXxx`（如已有 `APITypeGemini`）
2. `relay/common/api_type_resolver.go`：
   - `PathAPITypeRule` 加前缀匹配分支
   - 可选：在 `UserAgentAPITypeRule` 加 UA 关键字
3. `common/api_type.go` 的 `ChannelType2APIType` 加映射（如有新的 channel type）
4. 写测试：参考 §8

### 7.2 替换 / 追加推断规则

`CurrentAPITypeInferenceChain` 是包级变量，可以：

- 测试时整体替换为 mock chain
- 启动时 `append` 自定义规则

```go
relaycommon.CurrentAPITypeInferenceChain = append(
    relaycommon.DefaultAPITypeInferenceChain(),
    MyCustomRule{},
)
```

### 7.3 修改 baseURL 拼接规则

如果某个上游要求即使 baseURL 含 `/v2`，也要再拼 `/v1`：

- 不要直接改 `BaseUrlHasVersionPrefix`（会影响所有调用方）
- 改在该渠道的 `adaptor.GetRequestURL` 里覆盖（参考 `relay/channel/openai/adaptor.go` 的处理方式）

## 8. 测试建议

`api_type_resolver.go` 没附带测试。建议补 `relay/common/api_type_resolver_test.go`，覆盖：

- `PathAPITypeRule`：6 个正例 + 1 个 Gemini + 3 个负例（未知路径、只有 `/v1`、大小写）
- `UserAgentAPITypeRule`：4 个正例 + 3 个负例
- `Chain.Infer`：路径命中时跳过 UA；都未命中返回 `(0, false)`
- `BaseUrlHasVersionPrefix`：`https://api.example.com` / `https://api.example.com/` / `https://api.example.com/v2` / `https://api.example.com/v` / `https://api.example.com/vertex` / `https://api.example.com/v2beta`
- `StripVersionPrefix`：`/v1/chat/completions` / `/v2/messages` / `/v1`（无后续） / `/chat/completions`（非版本） / `/v2beta/...`（非纯数字）

按 `AGENTS.md` 测试规范使用 `testify/require` + `testify/assert`，避免无意义 coverage 测试。

## 9. 排查清单

| 现象 | 优先检查 |
|---|---|
| 同模型名请求被路由到错的协议渠道 | `expectedAPIType` 推断是否对：`PathAPITypeRule` 路径前缀、`UserAgentAPITypeRule` UA；查看 `service.InferExpectedAPITypeFromContext` 返回 |
| 过滤后无渠道（"无可用渠道"） | 已有兜底，正常不应出现；若出现：检查 `ChannelType2APIType` 是否覆盖该 channel type |
| baseURL 是 `/v2` 仍报 `/v1` 找不到 | `BaseUrlHasVersionPrefix` 解析是否成功（`url.Parse` 失败时返回 false）；检查 baseURL 字符串是否含 scheme |
| `Path2RelayMode` 把 `/v2/...` 认成 Unknown | `normalizeVersionPrefix` 只处理单段 `/vN` 且 N 是数字；`/v2beta` 不会被归一（设计上要分别处理） |
