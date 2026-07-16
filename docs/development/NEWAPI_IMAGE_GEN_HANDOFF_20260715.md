# NewAPI Responses image_gen 兼容修复状态快照

更新时间：2026-07-15（UTC+8）

## 1. 当前结论

- `image_gen.imagegen` 与上游 hosted `image_generation` 的冲突修复已进入测试线、生产线，并已归档到 `main-local`。
- `main-local` 归档提交：`350e8963`（`archive: Responses image-gen semantic dedupe into main-local`）。
- hotfix：`hotfix/202607-responses-image-gen-alias`，最终实现提交：`07e53f99`。
- 用户确认 3010 当前运行暂无异常。本快照不重新读取生产请求正文、凭据或数据库配置。
- hotfix 分支和 worktree 当前仍保留；除非用户明确授权，不要删除。

## 2. 根因

Codex Desktop 可能发送 namespace/function 形式的图像工具：

```text
namespace image_gen
→ 展平后 function image_gen.imagegen
```

部分 OpenAI/ChatGPT 上游模型会隐式注册 hosted capability：

```text
image_generation
```

当 NewAPI 不知道该 hosted capability 隐式存在时，function 与 hosted 工具会同时到达上游，上游拒绝请求：

```text
Function 'image_gen.imagegen' conflicts with a hosted tool in the same request.
```

## 3. 当前修复机制

当前不是通用的管理员 Tool Alias Mapping，而是三层组合：

1. 内置 capability canonicalization：

   ```text
   image_gen / image_generation → image_generation
   web_search / web_search_preview → web_search
   ```

2. Route 或模型配置声明隐式 hosted capability：

   ```json
   {
     "responses_implicit_hosted_tools": ["image_generation"]
   }
   ```

3. 冲突策略：

   ```json
   {
     "responses_tool_conflict_policy": "deduplicate"
   }
   ```

命中后：保留 hosted `image_generation`，删除语义冲突的 `image_gen.imagegen` function。没有对应显式或隐式 hosted capability 时，function 必须保留。

### 模型级示例

```json
{
  "responses_tool_conflict_policy": "deduplicate",
  "responses_tool_model_overrides": [
    {
      "models": ["gpt-5.5", "gpt-5.6-sol"],
      "responses_implicit_hosted_tools": ["image_generation"]
    }
  ]
}
```

## 4. Tool Alias Mapping 的边界

当前**不支持**管理员任意配置：

```text
function/namespace 名称 → hosted capability
```

例如下面的映射目前不能仅靠后台新增：

```text
provider_search.query → web_search
vendor_image.create → image_generation
```

`image_gen.imagegen ↔ image_generation` 能工作，是因为代码已有 image capability canonicalization，再结合 `responses_implicit_hosted_tools` 和 `deduplicate`。

未来如果实现通用 Alias Mapping，最低安全要求：

- 精确 function/namespace 名称匹配；
- 精确 hosted capability；
- 禁止 wildcard、regex 和模糊前缀删除；
- 只有 hosted capability 显式或隐式存在时才执行去重；
- 无对应 hosted capability 时保留 function；
- Route 默认和模型覆盖均需可审计。

## 5. 已验证范围

历史发布验证覆盖：

- Go 单测：`dto`、`service/relayconvert`、`relay/channel/advancedcustom`；
- 3011 Responses 请求：namespace/function 形式、多个受影响模型、SSE 完成事件；
- 3010 发布后 API 验证；
- Compatibility Event 将该行为记录为 `name_conflict / deduplicate`，不是工具不支持导致的 `policy_drop`。

当前用户反馈：3010 无异常。若后续再出现冲突，优先检查：

1. 请求中的真实 ToolType/ToolName；
2. requested/upstream model；
3. Route/模型的 `responses_implicit_hosted_tools`；
4. `responses_tool_conflict_policy`；
5. OCI image revision 和实际容器版本。

## 6. 发布与回滚边界

- 禁止 `release/test → release/prod` 链式 merge。
- 同一 feature/hotfix 必须独立 merge 到每条 release 分支。
- 本 hotfix 已归档到 `main-local`，当前无需重复归档。
- 没有新的失败证据时不要回滚生产。
- 不在文档、日志或回复中记录 API Token、Authorization、DSN 或请求正文。
