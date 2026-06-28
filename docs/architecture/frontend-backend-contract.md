# 前后端集成契约

本文档定义 frontend 与 gateway 的基础集成约定。详细 endpoint 以 [`docs/services/gateway/api/openapi.yaml`](../services/gateway/api/openapi.yaml) 为准。

## API 入口

前端只调用 gateway：

```text
/api/v1
```

前端不得直接调用 `auth`、`file`、`knowledge`、`qa`、`document`、`ai-gateway` 的内部地址。内部服务地址只应存在于 gateway、领域服务或部署配置中。

管理端、其他后端模块和 MCP 工具等 HTTP 调用方同样必须通过 gateway `/api/v1` 访问公开业务接口，不得绕过 gateway 直连内部服务。

AI Gateway 是内部模型服务，只提供 `/internal/v1/**` 给 `qa`、`knowledge`、`document` 和 public `gateway` 等后端服务使用。前端即使需要问答、报告生成或模型配置能力，也必须先调用 public gateway 的 `/api/v1/**`，不能直接调用 AI Gateway 的 OpenAI-compatible endpoint 或内部配置 endpoint。

## OpenAPI 作为协作源

- `docs/services/gateway/api/openapi.yaml` 是前端与 gateway 的第一版契约源。
- `docs/services/ai-gateway/api/openapi.yaml` 是 AI Gateway 内部服务契约源，不生成前端 API client。
- 前端统一使用 `openapi-typescript` 从 gateway OpenAPI 生成类型，并通过项目封装的 typed fetch wrapper 调用 gateway；不得继续扩展旧的手写 `{ code, message, data }` API client。
- 后端实现 endpoint 前，应先更新 OpenAPI。
- 破坏性字段变更必须同步更新 OpenAPI 和本契约文档。
- 所有前端到 gateway、gateway 到下游服务的 HTTP API 必须使用 RESTful 资源路径，由 HTTP method 表达动作；健康检查是唯一已允许的非 `/api/v1` 例外。
- 本轮把 gateway 健康检查、auth、knowledge-owned 知识库/文档上传/文档处理/原文件内容/切片/检索接口、`document` 拥有的报告生成接口、`qa` 拥有的会话/消息/SSE/引用/配置/检索体验测试/统计接口，以及 admin-facing runtime model/parser configuration 接口列为已确定公开契约；`file` 只作为后端内部基础文件能力，不直接拥有前端公开 API。管理后台概览/指标聚合接口暂缺，见 OpenAPI 顶层 `x-missing-contracts`。
- AI Gateway 的 chat、Function Calling 透传、embedding 和 rerank 契约已经作为内部服务契约补齐，但不改变前端只能调用 gateway 的约束。

## 接口文档编写标准

本文是 RESTful 路径、OpenAPI 协作、请求/响应 envelope、分页、错误、SSE、上传和 request id 的权威位置。服务 README 只应解释该服务拥有的资源、字段业务含义、状态枚举、工作流和特殊错误场景，不应重复定义通用标准。

服务文档中如需提到通用规则，使用链接而不是复制：

| 通用规则 | 权威位置 |
| --- | --- |
| RESTful 资源路径、动作词限制和 owner service | 本文、[`service-boundaries.md`](service-boundaries.md) |
| 成功响应、分页响应和错误响应 envelope | 本文的“成功响应”“错误响应”章节 |
| `Authorization`、用户上下文 header 和 request id | 本文的“认证约定”“Request ID”章节 |
| OpenAPI 先行、前端类型生成和缺失契约处理 | 本文的“OpenAPI 作为协作源”“Mock 与并行开发”章节 |
| 技术栈、日志、测试、数据库和队列选型 | [`technology-decisions.md`](technology-decisions.md) |
| 文档维护和归属规则 | [`../collaboration/documentation-workflow.md`](../collaboration/documentation-workflow.md) |

## 认证约定

- 用户创建和会话创建接口不要求认证。
- 业务接口默认要求认证，OpenAPI 中使用 `bearerAuth` 标记。
- 用户创建或会话创建成功后，前端从响应的 `data.session.accessToken` 读取访问令牌。该 token 是 opaque Bearer token，不是 JWT，前端不得解析其内容。
- 前端后续请求使用 `Authorization: Bearer <accessToken>`。
- 前端只发送认证凭据，不发送 `X-User-Id`、`X-User-Roles`、`X-User-Permissions`。
- 用户身份、角色和权限由 gateway 从 Redis 会话缓存读取后传递给下游服务。
- Redis 会话缓存由 gateway 在 auth 返回身份/会话信息后写入；前端不直接访问 Redis 或 auth 内部地址。
- `401 unauthorized` 表示未登录或认证失效；前端应回到登录流程。
- `403 forbidden` 表示已登录但权限不足；前端应展示权限不足状态。

## 请求约定

| 项目 | 约定 |
| --- | --- |
| JSON request | `Content-Type: application/json` |
| JSON response | `Content-Type: application/json` |
| File upload | `multipart/form-data` |
| Streaming response | `text/event-stream` |
| Timestamp | RFC 3339 / OpenAPI `date-time` |
| ID | Public API 使用 string ID |
| Page index | `page` 从 1 开始 |
| Page size | `pageSize`，默认值和上限后续由 endpoint 细化 |

## 成功响应

单资源响应：

```json
{
  "data": {
    "id": "kb_123"
  },
  "requestId": "req_123"
}
```

列表响应：

```json
{
  "data": [],
  "page": {
    "page": 1,
    "pageSize": 20,
    "total": 0
  },
  "requestId": "req_123"
}
```

前端应从 `data` 读取业务数据，不依赖响应中的内部服务字段。

## 错误响应

错误响应固定为：

```json
{
  "error": {
    "code": "validation_error",
    "message": "request validation failed",
    "requestId": "req_123",
    "fields": {
      "name": "is required"
    }
  }
}
```

前端逻辑应优先匹配 `error.code`，不要解析 `message` 文案。

| Code | Frontend behavior |
| --- | --- |
| `validation_error` | 显示字段错误或表单级错误。 |
| `unauthorized` | 清理本地登录态并进入登录流程。 |
| `forbidden` | 展示权限不足。 |
| `not_found` | 展示资源不存在或已删除。 |
| `conflict` | 展示状态冲突并刷新当前数据。 |
| `rate_limited` | 展示稍后重试。 |
| `dependency_error` | 展示服务暂不可用。 |
| `internal_error` | 展示通用系统错误。 |

## 分页、过滤和查询

分页、过滤和查询属于下游服务契约的一部分。Knowledge 相关列表和检索参数已经进入 OpenAPI；其他下游接口后续补齐时优先使用以下约定：

```text
?page=1&pageSize=20&keyword=xxx&status=ready
```

约定：

- `keyword` 表示模糊查询关键词。
- 多值过滤可使用逗号分隔字符串，具体字段由 OpenAPI endpoint 定义。
- 排序参数后续统一为 `sort`，例如 `sort=-createdAt`，本轮只保留扩展空间。
- 在对应 OpenAPI path 补齐前，前端不得依赖管理后台概览/指标聚合接口。知识库、问答、报告生成和 admin runtime configuration 接口以当前 OpenAPI active paths 为准。

## Knowledge 接口

知识库管理、文档处理状态、切片详情和知识检索已经进入 gateway OpenAPI。前端应只调用 gateway 暴露的以下资源路径，不能直接调用 `services/knowledge`：

| Method | Path | 说明 |
| --- | --- | --- |
| `GET/POST` | `/api/v1/knowledge-bases` | 分页查询知识库、创建知识库。 |
| `GET/PATCH/DELETE` | `/api/v1/knowledge-bases/{knowledgeBaseId}` | 查询、更新、删除知识库。 |
| `GET` | `/api/v1/knowledge-bases/{knowledgeBaseId}/documents` | 查询知识库内文档处理状态列表。 |
| `POST` | `/api/v1/knowledge-bases/{knowledgeBaseId}/documents` | 上传原始文件并创建知识库文档资源；底层文件对象由 knowledge 在内部复用 file 保存。 |
| `GET` | `/api/v1/documents/{documentId}` | 查询文档处理详情。 |
| `PATCH/DELETE` | `/api/v1/documents/{documentId}` | 更新或删除知识库文档资源，由 knowledge 协调切片、索引和底层 file 引用。 |
| `GET` | `/api/v1/documents/{documentId}/chunks` | 查询文档切片详情。 |
| `POST` | `/api/v1/knowledge-queries` | 创建一次知识检索查询并返回召回结果。 |

检索使用 `knowledge-queries` 资源，不使用 `/search`、`/retrieval/search` 或其他动作路径。返回字段、分页结构和错误响应以 [`docs/services/gateway/api/openapi.yaml`](../services/gateway/api/openapi.yaml) 为准。

## QA 接口

智能问答会话、消息、回答运行、引用、配置、检索体验测试和统计已经进入 gateway OpenAPI。前端应只调用 gateway 暴露的以下资源路径，不能直接调用 `services/qa` 或 AI Gateway：

| Method | Path | 说明 |
| --- | --- | --- |
| `GET/POST` | `/api/v1/qa-sessions` | 查询当前用户会话、创建会话。 |
| `GET/PATCH/DELETE` | `/api/v1/qa-sessions/{sessionId}` | 查询、更新、软删除会话。 |
| `GET/POST` | `/api/v1/qa-sessions/{sessionId}/messages` | 查询消息；创建用户消息并触发非流式或流式回答。 |
| `GET` | `/api/v1/qa-sessions/{sessionId}/events` | 按 `responseRunId` 查询短期保存的 SSE 事件。 |
| `GET/PATCH` | `/api/v1/response-runs/{responseRunId}` | 查询回答运行状态或取消运行。 |
| `GET` | `/api/v1/response-runs/{responseRunId}/tool-calls` | 查询脱敏后的工具调用摘要。 |
| `GET/POST` | `/api/v1/messages/{messageId}/citations`、`/api/v1/citations/{citationId}`、`/api/v1/citation-lookups` | 查询回答引用列表、引用详情和批量引用详情。 |
| `GET/POST` | `/api/v1/qa-config-versions/current`、`/api/v1/qa-config-versions`、`/api/v1/llm-config-versions/current`、`/api/v1/llm-config-versions`、`/api/v1/llm-connection-tests` | 查询或创建问答/LLM 配置版本，测试 AI Gateway profile 连接。 |
| `GET/POST` | `/api/v1/retrieval-test-runs`、`/api/v1/retrieval-test-runs/{testRunId}` | 创建或查询检索体验测试。 |
| `GET` | `/api/v1/qa-metrics/**` | 查询问答统计、趋势、热门问题和意图分布。 |

前端只展示 QA 返回的 `thinking` / `reasoning.step` 安全摘要和 tool-call summary，不展示或缓存完整 prompt、私有 chain-of-thought、MCP 原始参数/结果、内部 URL、provider 原始错误或存储 object key。

## Admin Runtime Configuration 接口

模型配置管理由 public `gateway` 暴露给前端，实际 provider 配置、API key 写入状态和模型 profile 校验仍由 `ai-gateway` 拥有。文档解析器配置由 `knowledge` 拥有，gateway 只提供统一公开入口、管理员鉴权和响应归一化。

| Method | Path | Owner | 说明 |
| --- | --- | --- | --- |
| `GET/POST` | `/api/v1/admin/model-profiles` | `ai-gateway` | 查询或创建 chat、embedding、rerank 模型 profile。 |
| `GET/PATCH/DELETE` | `/api/v1/admin/model-profiles/{profileId}` | `ai-gateway` | 查询、更新、删除或停用模型 profile。 |
| `GET/POST` | `/api/v1/admin/parser-configs` | `knowledge` | 查询或创建文档解析器配置。 |
| `GET/PATCH/DELETE` | `/api/v1/admin/parser-configs/{parserConfigId}` | `knowledge` | 查询、更新、删除或停用文档解析器配置。 |

`apiKey` 是 write-only 字段，只允许在创建或更新模型 profile 时发送；任何响应、日志、错误文案和前端缓存都不得包含明文 key。前端只能依赖 `apiKeyConfigured` 判断是否已配置密钥。模型调用能力仍由后端领域服务通过 AI Gateway 内部调用完成，前端不得用这些 admin 配置接口发起 chat、embedding 或 rerank 请求。

## SSE 与流式 UI

问答的公开流式接口已经进入 gateway OpenAPI。前端通过 `POST /api/v1/qa-sessions/{sessionId}/messages` 创建消息；当请求头包含 `Accept: text/event-stream` 时，gateway 返回 QA SSE 流。`GET /api/v1/qa-sessions/{sessionId}/events?responseRunId=...` 用于短期事件回放和断线恢复。AI Gateway 内部 `POST /internal/v1/chat/completions` 支持 OpenAI-compatible streaming chunk 和 tool-call delta，但该能力只供 `qa`、`document` 等后端领域服务使用，不等同于前端可直接调用的 gateway SSE contract。报告生成当前可使用 `GET /api/v1/reports/{reportId}/events` 轮询事件列表；后续如需报告 SSE，必须先补 OpenAPI 契约。QA SSE 前端处理原则如下：

- 根据 `Content-Type: text/event-stream` 进入流式读取。
- `message.created` 事件用于创建消息和运行占位。
- `agent.iteration.started` 事件用于展示 Agent 正在进入下一轮模型/工具循环。
- `reasoning.step` 事件用于展示安全的处理步骤摘要，不展示私有 chain-of-thought。
- `tool.started`、`tool.completed`、`tool.failed` 事件用于展示脱敏后的工具调用状态。
- `answer.delta` 事件用于最终回答文本增量。
- `citation.delta` 事件用于问答引用。
- `answer.completed` 事件表示回答完成。
- `error` 事件表示本次流式任务失败。

QA SSE 不得返回完整工具参数、MCP 原始响应、内部 URL、原始文档全文、prompt、provider 原始错误或存储 object key。断线重连时，前端应使用当前 OpenAPI 中的事件回放资源，而不是直接调用内部 QA 或 AI Gateway 地址。

## 文件上传与内容读取

- 上传使用 `multipart/form-data`。
- 上传 endpoint 由 gateway 暴露，知识库文档公开资源归 `knowledge` 服务管理；底层原始文件对象由 `knowledge` 在服务边界内复用 `file` 服务保存和读取。
- 文档处理状态、知识库列表、切片详情、原文件内容入口和知识检索归 `knowledge` 服务并已进入 gateway OpenAPI。
- 前端读取原文件内容时，只使用 gateway 提供的 `GET /api/v1/documents/{documentId}/content`。
- 生成报告和报告文件内容接口由 `document` 契约提供；前端只通过 `POST /api/v1/reports/{reportId}/jobs` 创建生成类任务，通过 `GET /api/v1/report-files/{reportFileId}/content` 获取生成文件内容。
- 前端不得依赖 file 内部 ID、MinIO object key、内部 URL 或内部存储路径。

## Request ID

- 前端可以在请求头中传递 `X-Request-Id`，不传时由 gateway 生成。
- Gateway 应在响应头和响应体中返回 request id。
- 用户反馈问题时，前端可展示或复制 request id 便于排查。

## Mock 与并行开发

并行开发时：

- 前端以 OpenAPI 中已存在的 active paths 为准，不等待所有内部服务完成。
- OpenAPI `x-missing-contracts` 中列出的范围只能作为待办，不应生成可调用 API client 方法；QA 和 admin model/parser configuration 不在缺失范围内，应按 active paths 生成或手写 client。
- 各后端服务以 gateway OpenAPI 和服务边界矩阵确认自己需要提供的能力。
- 领域服务需要模型能力时，以 [AI Gateway 服务接口文档](../services/ai-gateway/README.md) 和 [AI Gateway OpenAPI 契约](../services/ai-gateway/api/openapi.yaml) 为准，不把 provider 细节暴露给前端。QA 需要工具能力时，应通过自己的 Agent Host 和 MCP Client 契约暴露安全摘要，而不是把 MCP server、tool schema 或工具原始结果直接暴露给前端。
- 如果实现发现契约不合理，先更新 OpenAPI 和相关文档，再改代码。
