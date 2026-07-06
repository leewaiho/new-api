# 迁移到 Advanced Custom 渠道

> 适用版本：`origin/release/prod`（包含本仓库加的 `coding_plan_claude_and_openai` 模板）
> 适用场景：把"同供应商的 Claude 渠道 + OpenAI 渠道"合并成 1 个 Advanced Custom 渠道

## 何时做这个迁移

满足以下**任一**情况：

- 同一个供应商你配了 2 个渠道：1 个 Claude 渠道（type 14）+ 1 个 OpenAI 渠道（type 1）或其他变体
- 这两个渠道共享 baseURL 和 key（**或 baseURL 不同但有可推导关系**，比如 Coding Plan 的 Claude 端点和 OpenAI 端点同主机不同 path）
- 两个渠道的 Models 列表大部分重叠

**不要迁移**的情况：

- 供应商的 Claude 端点和 OpenAI 端点是**完全独立**的（比如一个在 AWS，一个在自建机房）
- 两侧配额 / 限速 / 计费比例需要**分别**控制
- 客户端行为依赖具体的 channel id（日志、计费归因）

## 5 步迁移清单

### Step 1：盘点现有渠道

对每个候选供应商，列出：

| 项 | Claude 渠道 A | OpenAI 渠道 B | 备注 |
|---|---|---|---|
| baseURL | | | |
| key | | | |
| Models | | | 取并集 |
| Group | | | 取并集或选一个 |
| Priority | | | |
| RateLimit | | | |
| Custom 字段（model_mapping 等） | | | 合并策略 |

### Step 2：建 1 个 Advanced Custom 渠道

- **Type** = `58`（Advanced Custom）
- **BaseURL**：填一个**真实可达**的 URL（必须以 `http://` 或 `https://` 开头）。注意：
  - 多数 Coding Plan 类的供应商需要填**完整 baseURL**（如 `https://ark.cn-beijing.volces.com`），而不是 `doubao-coding-plan` 那种特殊别名（特殊别名只对 type 45 有效）
  - 如果 Claude 端点和 OpenAI 端点的 baseURL 不同（如 Coding Plan 的 `/api/coding` 和 `/api/coding/v3`），需要在路由里写**完整 upstream_path** 来覆盖
- **Key**：填供应商 API key
- **Models**：Step 1 表里 Claude + OpenAI 渠道 Models 的**并集**
- **Group**：原值（去重）
- **Priority / RateLimit**：取你想要的合并值

### Step 3：配置 Advanced Custom 路由

打开 Advanced Custom 编辑器，按以下策略之一：

**策略 A：用现成模板**（推荐）

仓库已加 1 个组合模板 `coding_plan_claude_and_openai`，适用于"Claude + OpenAI Chat"双协议同 baseURL 场景。选模板后**手工调整** `upstream_path` 字段（如果需要重写 path）。

**策略 B：手动配**

典型 3 路由配置（Claude + OpenAI Chat + OpenAI Responses）：

```json
{
  "advanced_routes": [
    {
      "incoming_path": "/v1/messages",
      "upstream_path": "/v1/messages",
      "converter": "none",
      "auth": {
        "type": "header",
        "name": "x-api-key",
        "value": "{api_key}"
      }
    },
    {
      "incoming_path": "/v1/chat/completions",
      "upstream_path": "/v1/chat/completions",
      "converter": "none",
      "auth": {
        "type": "header",
        "name": "Authorization",
        "value": "Bearer {api_key}"
      }
    },
    {
      "incoming_path": "/v1/responses",
      "upstream_path": "/v1/responses",
      "converter": "none",
      "auth": {
        "type": "header",
        "name": "Authorization",
        "value": "Bearer {api_key}"
      }
    }
  ]
}
```

**`upstream_path` 三种写法**：

| 形式 | 例子 | 行为 |
|---|---|---|
| `/` 开头 path | `/v1/messages` | 拼到 `Channel.BaseURL` 上 → `BaseURL + /v1/messages` |
| 完整 URL | `https://api.example.com/v2/messages` | **完全覆盖** BaseURL 拼接 |
| 含 `{model}` | `/v1beta/models/{model}:generateContent` | 运行时替换成 `info.UpstreamModelName` |

### Step 4：端到端验证

跑这 6 步（**先小流量后大流量**）：

1. **保存**渠道成功（前端编辑器 Validate 通过）
2. `curl` Claude 路径到 new-api（用你的 access token）：
   ```bash
   curl -X POST http://localhost:3000/v1/messages \
     -H "Authorization: Bearer <new-api-token>" \
     -H "anthropic-version: 2023-06-01" \
     -H "Content-Type: application/json" \
     -d '{"model":"<渠道 Models 里的一个>","max_tokens":16,"messages":[{"role":"user","content":"hi"}]}'
   ```
3. `curl` OpenAI Chat / Responses 路径：
   ```bash
   curl -X POST http://localhost:3000/v1/chat/completions \
     -H "Authorization: Bearer <new-api-token>" \
     -H "Content-Type: application/json" \
     -d '{"model":"<渠道 Models 里的一个>","max_tokens":16,"messages":[{"role":"user","content":"hi"}]}'
   ```
4. **看日志 / 数据库**确认 `ChannelId` 命中新建的 Advanced Custom 渠道（不是原两个旧渠道）
5. **故意访问未配的 path**（如 `/v1/embeddings`），看是否报 `advanced custom channel does not support request path`（确认报错信息是 Advanced Custom 而不是"无匹配渠道"）
6. **限速/配额验证**：把新渠道的 `RateLimit` 调小到 1，触发限速后**两个 path 都受限**（验证共享）

### Step 5：下线旧渠道

- **保留 1-2 周观察期**：旧渠道先 disable（不要删），方便回滚
- 观察期无异常后再**物理删除**旧渠道
- 在内部 wiki / 变更日志记一笔

## 路由约束速查

`dto/channel_settings.go` 的 `Validate()` 关键约束：

| 字段 | 规则 |
|---|---|
| `incoming_path` | 必填、以 `/` 开头、不含 `?`、**全局唯一** |
| `upstream_path` | 必填、是完整 URL 或 `/` 开头路径 |
| `converter` | 必须在白名单内；`converter=none` 不校验 path；其他 converter 强约束 path 形态 |
| `auth` | 可选；type ∈ {none, header, query}；header 时 `name`/`value` 必填 |

`converter` 与 `incoming_path` 的配对：

| converter | incoming_path 必须 |
|---|---|
| `none` | 任意 |
| `anthropic_messages_to_openai_chat_completions` | `/v1/messages` |
| `openai_chat_completions_to_anthropic_messages` | `/v1/chat/completions` |
| `openai_chat_completions_to_openai_responses` | `/v1/chat/completions` |
| `openai_responses_to_openai_chat_completions` | `/v1/responses` |
| `gemini_generate_content_to_openai_chat_completions` | 含 `:generateContent` / `:streamGenerateContent` |
| `openai_chat_completions_to_gemini_generate_content` | `/v1/chat/completions` |

> **如果需要把 Claude 端点当 OpenAI Responses 用**（即"客户端发 Responses 走 Claude 协议的上游"），目前 **没有** `anthropic_messages_to_openai_responses` 这种 converter，只能在路由里直接配 `incoming_path=/v1/responses`、`converter=none`，让上游 Claude 端点直接吃 Responses 格式。这只在你的上游 Claude 端点真的支持 Responses 格式时才行。

## 常见坑

1. **`auth.value` 里的 `{api_key}` 是模板占位符**，会被替换成 `Channel.Key`。**不要**把真 key 写进去。
2. **IncominPath 不可重复**。两条 `/v1/messages` 会 Validate 拒。
3. **Claude 路径不写 auth = 默认 Bearer = 上游拒**。Claude 端点必须显式 `x-api-key`。
4. **Models 字段是所有路由共享的**。只配 `claude-sonnet-4-5` 时，Responses 路径用 `gpt-4o` 会在**能力查询**阶段被过滤掉，根本到不了 Advanced Custom。要把用得到的所有模型名都写进 Models。
5. **baseURL 字段在 Advanced Custom 里仍然是必填**（前端校验），但**实际请求时**：
   - 如果 `upstream_path` 是完整 URL，baseURL **不会被使用**——可以填任意有效 URL 满足校验
   - 如果 `upstream_path` 是 `/` 开头 path，baseURL 会被拼上
6. **Advanced Custom 渠道不会被 fork 改造的"协议过滤"误伤**：`filterChannelsByExpectedAPIType` 对 `ChannelTypeAdvancedCustom` **永远保留**。

## 已知不能做（缺口）

| 能力 | 状态 | 影响 |
|---|---|---|
| 路由级配额拆分（"Claude 端点用 60%、OpenAI 端点用 40%"） | ❌ | 只能渠道级共享配额 |
| 路由级限速 | ❌ | 同上 |
| `anthropic_messages_to_openai_responses` 协议转换 | ❌ | 上游 Claude 端点不支持 Responses 时无法直转 |
| Claude 端点用 OpenAI Responses 走 OpenAI Chat 协议再转 | ❌ | 同上 |
| 路由级日志分流统计 | ❌ | 只能到 Channel 维度 |

如果上面任一项是运营侧的硬需求，回到 [CHANNEL_PROTOCOLS.md](./CHANNEL_PROTOCOLS.md) 看「同模型多渠道」方案；不推荐为了这些去 fork Advanced Custom。

---

## 真实案例：火山引擎 Coding Plan

> 本节是上面通用清单的**具体应用**。

### 现状

你之前有 2 个 VolcEngine 渠道（type 45）：

| 渠道 | baseURL 实际值 | 走的协议 | 实际请求 URL |
|---|---|---|---|
| Claude 渠道 | `https://ark.cn-beijing.volces.com/api/coding` | Anthropic Messages | `POST /v1/messages` |
| OpenAI 渠道 | `https://ark.cn-beijing.volces.com/api/coding/v3` | OpenAI Chat Completions | `POST /chat/completions` |

### 方式 A：用 Quick Setup UI（推荐，本仓库新加）

1. 新建渠道，Type 选 `58`（Advanced Custom）
2. BaseURL 填 `https://ark.cn-beijing.volces.com`（仅满足前端校验，**实际请求会被路由的 upstream_path 覆盖**）
3. Key 填火山引擎 API key
4. Models 填原 Claude + OpenAI 渠道模型并集
5. 打开 Advanced Custom 编辑器，点 toolbar 上的 **Quick Setup** 按钮
6. 在弹出表单里填：

   | 字段 | 值 |
   |---|---|
   | Anthropic-compatible Base URL | `https://ark.cn-beijing.volces.com/api/coding` |
   | OpenAI-compatible Base URL | `https://ark.cn-beijing.volces.com/api/coding/v3` |
   | Endpoints | Claude Messages ✓ / OpenAI Chat ✓ / OpenAI Responses ✓（如果你当前 OpenAI 渠道已验证可用） |
   | Anthropic auth | Bearer（火山引擎 Coding Plan 走 OpenAI 兼容鉴权，**不是** Anthropic 标准 x-api-key） |
   | OpenAI auth | Bearer |

7. 点 **Apply** — 前端自动生成 3 条路由（Claude Messages + OpenAI Chat + OpenAI Responses），填进编辑器
8. 保存渠道

**生成结果**等价于手写下面这段 JSON（你可以在 JSON Text 模式里看到）：

```json
{
  "advanced_routes": [
    {
      "incoming_path": "/v1/messages",
      "upstream_path": "https://ark.cn-beijing.volces.com/api/coding/v1/messages",
      "converter": "none",
      "auth": {
        "type": "header",
        "name": "Authorization",
        "value": "Bearer {api_key}"
      }
    },
    {
      "incoming_path": "/v1/chat/completions",
      "upstream_path": "https://ark.cn-beijing.volces.com/api/coding/v3/chat/completions",
      "converter": "none",
      "auth": {
        "type": "header",
        "name": "Authorization",
        "value": "Bearer {api_key}"
      }
    },
    {
      "incoming_path": "/v1/responses",
      "upstream_path": "https://ark.cn-beijing.volces.com/api/coding/v3/responses",
      "converter": "none",
      "auth": {
        "type": "header",
        "name": "Authorization",
        "value": "Bearer {api_key}"
      }
    }
  ]
}
```

### 方式 B：手写 JSON（不推荐，能用但繁琐）

如果 Quick Setup UI 不可用，或者你想精确控制路径后缀，编辑器的 JSON Text 模式里直接粘下面这段：

```json
{
  "advanced_routes": [
    {
      "incoming_path": "/v1/messages",
      "upstream_path": "https://ark.cn-beijing.volces.com/api/coding/v1/messages",
      "converter": "none",
      "auth": {
        "type": "header",
        "name": "Authorization",
        "value": "Bearer {api_key}"
      }
    },
    {
      "incoming_path": "/v1/chat/completions",
      "upstream_path": "https://ark.cn-beijing.volces.com/api/coding/v3/chat/completions",
      "converter": "none",
      "auth": {
        "type": "header",
        "name": "Authorization",
        "value": "Bearer {api_key}"
      }
    },
    {
      "incoming_path": "/v1/responses",
      "upstream_path": "https://ark.cn-beijing.volces.com/api/coding/v3/responses",
      "converter": "none",
      "auth": {
        "type": "header",
        "name": "Authorization",
        "value": "Bearer {api_key}"
      }
    }
  ]
}
```

### 关键点说明

1. **为什么这些 upstream_path 都不拼 Channel.BaseURL？** —— 用 Quick Setup 时 `upstream_path` 是完整 URL，绕过了 Advanced Custom 的 baseURL 拼接逻辑，让两条路由各自指向正确端点。
2. **为什么鉴权都是 Bearer？** —— 火山引擎 Coding Plan 走的是 OpenAI 兼容鉴权（`Authorization: Bearer <key>`），即使 Claude 端点也用 Bearer，**不是** Anthropic 默认的 `x-api-key` 头。
3. **Channel.BaseURL 实际不会被使用**（因为 upstream_path 是完整 URL），填 `https://ark.cn-beijing.volces.com` 仅为满足前端校验。
4. **要不要加 `/v1/responses` 路由？** —— VolcEngine adaptor 已有 `RelayModeResponses` 路径，会把 Responses 请求体原样透传到上游；如果你当前的火山 Coding Plan OpenAI 渠道已经能调用 Responses API，迁移到 Advanced Custom 时就应该加这条路由：
   - **Quick Setup 方式**：勾选 "OpenAI Responses" checkbox，UI 会自动生成第 3 条路由
   - **手写方式**：自己加第 3 条路由
   - **前提**：OpenAI-compatible Base URL 指向 Coding Plan OpenAI 端点（例如 `https://ark.cn-beijing.volces.com/api/coding/v3`），生成的上游 URL 会是 `https://ark.cn-beijing.volces.com/api/coding/v3/responses`
5. **下线路由**：迁移完成后**不要立即删**原 2 个 VolcEngine 渠道，保留 1-2 周 disable 状态作为回滚。
