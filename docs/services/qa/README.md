# QA 服务接口文档

本文档定义 `qa` 服务的前端接口、内部 Agent Host 设计目标和 gateway 适配约定。稳定公开契约以 [`Gateway OpenAPI 契约`](../gateway/api/openapi.yaml) 中的 active paths 为准；本文档解释字段来源、处理流程和内部边界。本文档基于以下来源整理：

- 外部前端接口文档《智能问答系统 — 前端接口文档》。
- [`技术选型基线`](../../architecture/technology-decisions.md)：`pgx` + `sqlc`、`goose`、`net/http` / `ServeMux`、`slog`、opaque Bearer token、`fetch` stream SSE、MCP SDK/sidecar 等实现约束。
- [`Gateway 服务规划`](../gateway/README.md)、[`前后端集成契约`](../../architecture/frontend-backend-contract.md)、[`服务边界矩阵`](../../architecture/service-boundaries.md) 和 [`Gateway OpenAPI 契约`](../gateway/api/openapi.yaml)。
- [`QA 数据模型文档`](docs/data-models.md)：`qa_config_versions`、`llm_config_versions`、`conversations`、`messages`、`response_runs`、`agent_model_invocations`、`agent_tool_calls`、`message_content_blocks`、`response_process_steps`、`response_stream_events`、`citations`、`retrieval_test_runs`、`retrieval_test_results`、`llm_connection_tests`、`admin_audit_logs`。
- [`QA 实现说明`](docs/implementation.md)：当前代码实现、契约对齐、缺口和最近检查记录。
- GitHub Discussion #65《请问能否重构AI问答模块接口契约？》。

> 当前状态：QA 会话、消息、非流式/流式回答、SSE 事件回放、回答运行、脱敏工具调用摘要、引用、配置、检索体验测试和统计接口已经进入 `docs/services/gateway/api/openapi.yaml` 的 active paths。MCP 原始 tool schema、完整工具参数/结果、内部审计和服务间私有接口仍不属于前端稳定公开契约。

## 设计目标

QA 不再按固定规则写死“意图识别 -> 检索 -> 生成”的单一路径，而是作为 Agent Host 运行一次可控的 ReAct 循环：

```text
frontend
   |
   v
gateway /api/v1/qa-sessions/**
   |
   v
qa service (Agent Host)
   |-- ReAct loop
   |-- MCP client manager
   |-- tool policy and permission checks
   |-- public SSE event projection
   |-- session, response run, model invocation, and tool-call state
   |
   |-- ai-gateway /internal/v1/chat/completions
   |     OpenAI-compatible function calling transport
   |
   +-- MCP client
         tools/list
         tools/call
         |
         +-- knowledge MCP server
         +-- document MCP server
         +-- future approved MCP servers
```

ReAct 在本文档中具体表示：

- Action：LLM 通过 OpenAI-compatible `tool_calls` 选择要调用的工具。
- Observation：QA 通过 MCP Client 执行 `tools/call` 后得到的结构化工具结果。
- Loop：QA 把 `role=tool` 的工具结果追加回模型上下文，继续下一轮模型调用，直到模型返回最终文本、达到终止条件或发生错误。

QA 只能保存和返回可向用户展示的处理摘要，不保存或返回模型原始 Thought、私有 chain-of-thought、完整 prompt、完整工具参数、内部 URL、原始文档全文、向量 payload 或 provider 原始错误。

## 职责边界

| 范围 | 说明 |
| --- | --- |
| 会话管理 | 维护用户的 QA 会话、会话标题、状态和消息顺序。 |
| 消息与回答运行 | 保存用户消息、助手消息、一次回答生成的运行状态、token 使用量、延迟和失败原因。 |
| Agent Host / ReAct Loop | 负责创建 response run、选择可用工具、调用模型、执行工具、循环终止和可展示步骤投影。 |
| MCP Client 编排 | 通过 MCP Client `tools/list` 发现工具，通过 `tools/call` 执行工具；QA 负责工具白名单、权限裁剪、参数校验和结果脱敏。 |
| 模型调用记录 | 记录每次模型调用的 iteration、模型、finish reason、token 用量、延迟和状态。 |
| 工具调用记录 | 记录每次工具调用的 tool call id、工具名、参数摘要、结果摘要、状态、延迟和错误码。 |
| SSE 事件 | 产生并短期保存前端可消费的流式事件，用于断线恢复或调试。 |
| 引用快照 | 保存回答中的引用编号、文档/chunk 外部 ID、引用文本、上下文和分数。 |
| QA 配置 | 管理问答检索参数、默认知识库选择和配置版本。 |
| LLM 配置 | 管理 AI Gateway profile 引用、模型名称、超时、生成参数和 Agent 终止策略。 |
| 检索体验测试 | 记录管理员发起的检索测试及结果快照。 |
| QA 统计 | 基于 `response_runs`、模型调用和工具调用记录聚合问答次数、延迟、工具使用和热门问题。 |

`qa` 不拥有用户、角色、权限、知识库主数据、文档原文件、文档解析、向量索引、报告记录、文件下载、provider API key 或 MCP server 的具体业务实现。相关能力由 `auth`、`knowledge`、`file`、`document`、`ai-gateway` 和 MCP server 拥有，gateway 只做公开入口、认证上下文和响应归一化。

## 接入模型

```text
frontend
   |
   v
gateway /api/v1/qa-sessions, /api/v1/qa-config-versions, /api/v1/retrieval-test-runs
   |
   v
qa service
   |
   +--> PostgreSQL conversations / messages / response_runs / agent_tool_calls / citations
   +--> ai-gateway for OpenAI-compatible model calls and function-calling transport
   +--> MCP Client for tools/list and tools/call
```

Gateway 调用 QA 服务时应传递：

| Header | 说明 |
| --- | --- |
| `X-Request-Id` | 贯穿一次前端请求的 request id。 |
| `X-User-Id` | 已认证用户 ID；映射到 QA 数据库中的 `external_user_id`。 |
| `X-User-Roles` | 逗号分隔的角色列表。 |
| `X-User-Permissions` | 逗号分隔的权限列表。 |
| `X-Forwarded-For` | 原始客户端地址链。 |
| `X-Forwarded-Proto` | 原始协议。 |

前端不得设置 `X-User-Id`、`X-User-Roles`、`X-User-Permissions`。QA 服务必须在自己的边界校验用户上下文和权限，不依赖前端传入身份字段。

## 技术落地基线

QA 服务实现必须对齐 [技术选型基线](../../architecture/technology-decisions.md)。本服务只补充 Agent Host 特有约束：

- QA 作为独立 Go module 微服务实现，公开前端入口仍经 gateway 暴露。
- 业务表结构以 `docs/services/qa/docs/data-models.md` 为逻辑来源；创建消息并启动 Agent Run、配置版本切换、引用快照落库等跨表写入必须由 service/use-case 层开启事务。
- Redis 只可用于短期 SSE 推送、运行中取消信号、短期锁或缓存；`response_runs`、`messages`、`response_stream_events` 和工具调用摘要仍以 PostgreSQL 为权威。
- 交互式 QA 回答主路径不默认投递后台队列，避免破坏 SSE 实时性。后续若增加离线评测、批量重放或清理任务，应使用 `asynq`，任务最终状态仍落 PostgreSQL。
- QA、AI Gateway、MCP tool call、knowledge/file/document 依赖调用属于重点 tracing 链路；第一阶段至少确保 request id 在日志、数据库记录和下游请求中贯穿。
- 消息创建使用 `POST` + `Accept: text/event-stream`；前端通过 `fetch` stream reader 和 `AbortController` 消费/取消，不以原生 `EventSource` 作为主实现。
- QA 负责工具白名单、权限裁剪、参数 schema 校验、超时、幂等和脱敏记录，不手写完整 MCP 协议栈作为首选。
- QA 不保存 provider API key、provider base URL、MinIO object key、数据库连接串或 secret ref 明文。模型 profile、provider 凭证和供应商适配由 AI Gateway 拥有。

建议的服务目录：

```text
services/qa/
  cmd/qa/
  internal/config/
  internal/http/
  internal/middleware/
  internal/service/
  internal/repository/
    queries/
    sqlc/
  internal/agent/
  internal/mcp/
  internal/aigateway/
  migrations/
  sqlc.yaml
```

## 与前端原始设计的映射

| 原始路径 | Gateway 草案路径 | Owner | 说明 |
| --- | --- | --- | --- |
| `POST /api/conversations` | `POST /api/v1/qa-sessions` | `qa` | 创建 QA 会话。 |
| `GET /api/conversations` | `GET /api/v1/qa-sessions` | `qa` | 查询当前用户会话列表。 |
| `GET /api/conversations/{conversation_id}` | `GET /api/v1/qa-sessions/{sessionId}` | `qa` | 查询会话详情。 |
| `PUT /api/conversations/{conversation_id}` | `PATCH /api/v1/qa-sessions/{sessionId}` | `qa` | 更新会话标题或状态。 |
| `DELETE /api/conversations/{conversation_id}` | `DELETE /api/v1/qa-sessions/{sessionId}` | `qa` | 归档或删除会话。 |
| `POST /api/chat/stream` | `POST /api/v1/qa-sessions/{sessionId}/messages` | `qa` | 创建用户消息并启动 Agent Run；可用 SSE 返回 Agent 状态和回答增量。 |
| `POST /api/rag/search` | `POST /api/v1/knowledge-queries` 或 MCP `search_knowledge` | `knowledge` / MCP | 正式检索查询由 `knowledge` 拥有；QA 通过 MCP 工具或检索测试间接调用。 |
| `POST /api/admin/rag/test` | `POST /api/v1/retrieval-test-runs` | `qa` | 管理员检索体验测试，等价于受控调用 `search_knowledge` 工具并保存结果快照。 |
| `GET /api/citations/{chunk_id}` | `GET /api/v1/citations/{citationId}` | `qa` / `knowledge` | QA 返回回答引用快照；原文详情可由知识服务补齐。 |
| `POST /api/citations/batch` | `POST /api/v1/citation-lookups` | `qa` | 批量引用详情查询；QA 可在内部调用 knowledge/file 补齐来源可用性。 |
| `GET/PUT /api/admin/knowledge-config` | `GET /api/v1/qa-config-versions/current`, `POST /api/v1/qa-config-versions` | `qa` | 配置采用版本化资源，并包含可用工具、默认知识库和检索参数。 |
| `GET/PUT /api/admin/llm-config` | `GET /api/v1/llm-config-versions/current`, `POST /api/v1/llm-config-versions` | `qa` | LLM 配置采用版本化资源，只保存 AI Gateway profile 引用和生成参数。 |
| `POST /api/admin/llm-config/test` | `POST /api/v1/llm-connection-tests` | `qa` | 通过 AI Gateway profile 创建一次 LLM 连接测试记录。 |
| `GET /api/admin/stats/*` | `GET /api/v1/qa-metrics/*` | `qa` | QA 指标由 `response_runs` 聚合。 |

## 通用响应结构

JSON 成功、分页和错误响应遵循 [前后端集成契约](../../architecture/frontend-backend-contract.md)。本文只保留外部旧数字码到项目错误码的迁移映射，便于前端和后端重构时对照。

错误码映射：

| 原始数字码 | 项目错误码 | HTTP status | 说明 |
| --- | --- | --- | --- |
| `40000` | `validation_error` | `400` | 请求参数错误。 |
| `40100` | `unauthorized` | `401` | 未登录或会话失效。 |
| `40300` | `forbidden` | `403` | 已登录但权限不足。 |
| `40400` | `not_found` | `404` | 会话、消息、引用或配置不存在。 |
| `50000` | `internal_error` | `500` | QA 服务未预期错误。 |
| `50100` | `dependency_error` | `502` | LLM 服务失败。 |
| `50200` | `dependency_error` | `502` | 知识检索或重排序依赖失败。 |
| `50300` | `dependency_error` | `502` | 文档处理或知识服务依赖失败。 |

Owner 权限语义：

- `GET/PATCH/DELETE /api/v1/qa-sessions/{sessionId}` 和会话消息列表/创建只允许当前用户访问。目标会话存在且属于其他用户时返回 `403 forbidden`；会话不存在或已软删除时返回 `404 not_found`。
- `GET` message、response run、citation 子资源始终带当前用户 owner 过滤。不存在或不属于当前用户时返回 `404 not_found`，不通过单资源响应泄露其他用户数据；批量 citation lookup 只返回当前用户可见的记录，不披露被省略 ID 的存在性。
- 当前未实现管理员跨用户访问能力；即使调用方带管理员角色，也不能绕过 QA owner 检查。

## 公开接口总览

| Method | Gateway Path | Auth | Owner | 说明 |
| --- | --- | --- | --- | --- |
| `POST` | `/api/v1/qa-sessions` | 需要 | `qa` | 创建 QA 会话。 |
| `GET` | `/api/v1/qa-sessions` | 需要 | `qa` | 查询当前用户 QA 会话列表。 |
| `GET` | `/api/v1/qa-sessions/{sessionId}` | 需要 | `qa` | 查询会话详情和消息摘要。 |
| `PATCH` | `/api/v1/qa-sessions/{sessionId}` | 需要 | `qa` | 更新会话标题或归档状态。 |
| `DELETE` | `/api/v1/qa-sessions/{sessionId}` | 需要 | `qa` | 删除或软删除会话。 |
| `GET` | `/api/v1/qa-sessions/{sessionId}/messages` | 需要 | `qa` | 查询会话消息列表。 |
| `POST` | `/api/v1/qa-sessions/{sessionId}/messages` | 需要 | `qa` | 创建用户消息并触发 Agent Run；可返回 SSE。 |
| `GET` | `/api/v1/qa-sessions/{sessionId}/events` | 需要 | `qa` | 按 `responseRunId` 读取短期保存的 Agent/SSE 事件。 |
| `GET` | `/api/v1/messages/{messageId}/citations` | 需要 | `qa` | 查询某条助手消息的引用列表。 |
| `GET` | `/api/v1/citations/{citationId}` | 需要 | `qa` | 查询引用快照详情。 |
| `POST` | `/api/v1/citation-lookups` | 需要 | `qa` | 批量查询引用详情；QA 可在内部调用 knowledge/file 补齐来源可用性。 |
| `GET` | `/api/v1/qa-config-versions/current` | 需要 | `qa` | 查询当前生效 QA 配置。 |
| `POST` | `/api/v1/qa-config-versions` | 需要 | `qa` | 创建新的 QA 配置版本并可设置为生效。 |
| `GET` | `/api/v1/llm-config-versions/current` | 需要 | `qa` | 查询当前生效 LLM 配置。 |
| `POST` | `/api/v1/llm-config-versions` | 需要 | `qa` | 创建新的 LLM 配置版本并可设置为生效。 |
| `POST` | `/api/v1/llm-connection-tests` | 需要 | `qa` | 测试一次 LLM 配置连接。 |
| `POST` | `/api/v1/retrieval-test-runs` | 需要 | `qa` | 创建一次检索体验测试。 |
| `GET` | `/api/v1/retrieval-test-runs/{testRunId}` | 需要 | `qa` | 查询检索体验测试结果。 |
| `GET` | `/api/v1/qa-metrics/overview` | 需要 | `qa` | 查询 QA 统计概览。 |
| `GET` | `/api/v1/qa-metrics/trend` | 需要 | `qa` | 查询问答趋势。 |
| `GET` | `/api/v1/qa-metrics/top-queries` | 需要 | `qa` | 查询热门问题。 |
| `GET` | `/api/v1/qa-metrics/intent-distribution` | 需要 | `qa` | 查询意图分布。 |

相关但非 QA-owned 的公开接口：

| Method | Gateway Path | Owner | 说明 |
| --- | --- | --- | --- |
| `POST` | `/api/v1/knowledge-queries` | `knowledge` | 语义检索、跨知识库联合检索和重排序结果。QA 可通过 MCP 工具或内部客户端间接调用，但不拥有知识索引。 |
| `GET/POST/PATCH/DELETE` | `/api/v1/knowledge-bases/**` | `knowledge` | 知识库和知识库内文档状态。 |
| `GET` | `/api/v1/documents/{documentId}/content` | `knowledge` | 下载或读取知识库原始文件内容；底层文件对象可由 knowledge 在内部复用 file。 |
| `GET/POST/PATCH/DELETE` | `/api/v1/users`, `/api/v1/sessions`, `/api/v1/users/me` | `auth` | 用户和会话。 |

## 会话与消息数据结构

### QASession

```json
{
  "id": "qa_sess_123",
  "title": "变压器巡检要点",
  "status": "active",
  "messageCount": 5,
  "lastMessagePreview": "根据知识库检索结果...",
  "createdAt": "2026-06-27T10:30:00Z",
  "updatedAt": "2026-06-27T11:00:00Z"
}
```

| 字段 | 类型 | 必填 | 来源 |
| --- | --- | --- | --- |
| `id` | `string` | 是 | `conversations.id` 的公开 ID。 |
| `title` | `string` | 否 | `conversations.title`。 |
| `status` | `active \| archived` | 是 | `conversations.status`。 |
| `messageCount` | `number` | 否 | 基于 `messages` 聚合。 |
| `lastMessagePreview` | `string` | 否 | 最近一条可展示消息摘要。 |
| `createdAt` | `string(date-time)` | 是 | `conversations.created_at`。 |
| `updatedAt` | `string(date-time)` | 是 | `conversations.updated_at`。 |

### QAMessage

```json
{
  "id": "msg_001",
  "sessionId": "qa_sess_123",
  "role": "assistant",
  "sequenceNo": 2,
  "status": "completed",
  "content": "根据知识库检索结果...",
  "thinking": [
    {
      "type": "tool_call",
      "label": "检索知识库",
      "status": "done",
      "detail": "命中 5 条结果"
    }
  ],
  "citations": [],
  "createdAt": "2026-06-27T10:30:15Z",
  "completedAt": "2026-06-27T10:30:18Z"
}
```

| 字段 | 类型 | 必填 | 来源 |
| --- | --- | --- | --- |
| `id` | `string` | 是 | `messages.id`。 |
| `sessionId` | `string` | 是 | `messages.conversation_id`。 |
| `role` | `user \| assistant \| system` | 是 | `messages.role`。 |
| `sequenceNo` | `number` | 是 | `messages.sequence_no`。 |
| `status` | `streaming \| completed \| stopped \| failed` | 是 | `messages.status`。 |
| `content` | `string` | 是 | 合并 `message_content_blocks` 中可展示文本块。 |
| `thinking` | `ThinkingStep[]` | 否 | `response_process_steps` 中可向用户展示的步骤摘要。不得包含私有思维链、完整 prompt 或完整工具参数。 |
| `citations` | `Citation[]` | 否 | `citations`。 |
| `createdAt` | `string(date-time)` | 是 | `messages.created_at`。 |
| `completedAt` | `string(date-time)` | 否 | `messages.completed_at`。 |

### ThinkingStep

```json
{
  "type": "tool_call",
  "label": "检索知识库",
  "status": "done",
  "detail": "命中 5 条结果，耗时 42ms"
}
```

允许的 `type` 初始值：`agent_iteration`、`tool_call`、`tool_result`、`generation`、`citation`、`verify`。

允许的 `status` 初始值：`pending`、`running`、`done`、`failed`。

### AgentModelInvocation

每次 QA 调用 AI Gateway 都应形成一条模型调用记录，用于调试、统计和成本分析。该记录是内部/管理侧数据，不作为第一版前端消息接口的稳定字段直接暴露。

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `responseRunId` | `string` | 所属 Agent Run。 |
| `iterationNo` | `number` | ReAct 第几轮，从 1 开始。 |
| `modelName` | `string` | 脱敏后的模型名或 AI Gateway alias。 |
| `finishReason` | `stop \| length \| content_filter \| tool_calls \| error` | AI Gateway 归一化后的结束原因。 |
| `tokenUsage` | `object` | prompt/completion/total token 摘要。 |
| `latencyMs` | `number` | 本次模型调用耗时。 |
| `status` | `running \| completed \| failed \| cancelled` | 调用状态。 |
| `startedAt` / `finishedAt` | `string(date-time)` | 开始和结束时间。 |

### AgentToolCall

每次模型返回 `tool_calls` 并由 QA 通过 MCP Client 执行工具时，应保存一条工具调用记录。

```json
{
  "responseRunId": "run_123",
  "modelInvocationId": "inv_001",
  "iterationNo": 2,
  "toolCallId": "call_001",
  "toolName": "search_knowledge",
  "argumentsSummary": {
    "knowledgeBaseCount": 2,
    "queryPreview": "变压器巡检..."
  },
  "resultSummary": {
    "hitCount": 8,
    "citationCount": 3
  },
  "status": "completed",
  "latencyMs": 420,
  "startedAt": "2026-06-27T10:30:16Z",
  "finishedAt": "2026-06-27T10:30:17Z"
}
```

`argumentsSummary` 和 `resultSummary` 必须是脱敏摘要，不得保存完整工具参数、内部 URL、object key、原始文档全文、provider token、prompt 或其他秘密。需要完整审计能力时，应先定义受限访问的内部审计契约，不能复用前端公开字段。

## Endpoint 详情

### POST /api/v1/qa-sessions

创建当前用户的 QA 会话。

请求：

```json
{
  "title": "新对话"
}
```

响应 `201`：

```json
{
  "data": {
    "id": "qa_sess_123",
    "title": "新对话",
    "status": "active",
    "messageCount": 0,
    "createdAt": "2026-06-27T10:30:00Z",
    "updatedAt": "2026-06-27T10:30:00Z"
  },
  "requestId": "req_123"
}
```

### GET /api/v1/qa-sessions

查询当前用户的 QA 会话列表。

查询参数：

| 参数 | 类型 | 必填 | 默认值 | 说明 |
| --- | --- | --- | --- | --- |
| `page` | `number` | 否 | `1` | 页码，从 1 开始。 |
| `pageSize` | `number` | 否 | `20` | 每页数量。 |
| `status` | `string` | 否 | `active` | 可选：`active`、`archived`。 |
| `sort` | `string` | 否 | `-updatedAt` | 排序字段。 |

响应 `200`：

```json
{
  "data": [
    {
      "id": "qa_sess_123",
      "title": "变压器巡检要点",
      "status": "active",
      "messageCount": 5,
      "lastMessagePreview": "根据知识库检索结果...",
      "createdAt": "2026-06-27T10:30:00Z",
      "updatedAt": "2026-06-27T11:00:00Z"
    }
  ],
  "page": {
    "page": 1,
    "pageSize": 20,
    "total": 12
  },
  "requestId": "req_123"
}
```

### GET /api/v1/qa-sessions/{sessionId}

查询会话详情。默认返回最近消息摘要；完整消息列表由 `/messages` 子资源查询。

当前用户访问其他用户的有效会话时返回 `403 forbidden`；会话不存在或已软删除时返回 `404 not_found`。

响应 `200`：

```json
{
  "data": {
    "id": "qa_sess_123",
    "title": "变压器巡检要点",
    "status": "active",
    "messageCount": 5,
    "createdAt": "2026-06-27T10:30:00Z",
    "updatedAt": "2026-06-27T11:00:00Z"
  },
  "requestId": "req_123"
}
```

### PATCH /api/v1/qa-sessions/{sessionId}

更新会话标题或状态。只允许当前用户更新自己的会话。

其他用户的有效会话返回 `403 forbidden`；不存在或已软删除的会话返回 `404 not_found`。

请求：

```json
{
  "title": "变压器巡检与维护要点",
  "status": "active"
}
```

响应 `200`：返回更新后的 `QASession`。

### DELETE /api/v1/qa-sessions/{sessionId}

删除或软删除会话。实现可将 `conversations.deleted_at` 置为当前时间，并隐藏该会话及其消息。

其他用户的有效会话返回 `403 forbidden`；不存在或已软删除的会话返回 `404 not_found`。

响应 `204`：无响应体。

### GET /api/v1/qa-sessions/{sessionId}/messages

查询会话消息列表。

其他用户的有效会话返回 `403 forbidden`；不存在或已软删除的会话返回 `404 not_found`。

查询参数：

| 参数 | 类型 | 必填 | 默认值 | 说明 |
| --- | --- | --- | --- | --- |
| `page` | `number` | 否 | `1` | 页码。 |
| `pageSize` | `number` | 否 | `50` | 每页消息数。 |
| `includeThinking` | `boolean` | 否 | `true` | 是否返回可展示处理步骤。 |
| `includeCitations` | `boolean` | 否 | `true` | 是否返回引用摘要。 |

响应 `200`：分页返回 `QAMessage[]`。

### POST /api/v1/qa-sessions/{sessionId}/messages

创建一条用户消息并触发 Agent Run。请求头 `Accept: text/event-stream` 时，gateway 以 SSE 流式返回 Agent 状态和回答增量；请求完成后，用户消息、助手消息、回答运行、模型调用摘要、工具调用摘要、处理步骤和引用快照均应持久化。

请求：

```json
{
  "message": "变压器巡检有哪些要点？",
  "knowledgeBaseIds": ["kb_power_standard", "kb_device_manual"],
  "params": {
    "topK": 5,
    "similarityThreshold": 0.7,
    "useRerank": true,
    "rerankThreshold": 0.5,
    "rerankTopN": 3
  },
  "agent": {
    "enabledToolNames": ["search_knowledge", "get_citation_source"],
    "maxIterations": 5
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `message` | `string` | 是 | 用户输入内容。 |
| `knowledgeBaseIds` | `string[]` | 否 | 指定知识库 ID；为空时使用当前 QA 配置。 |
| `params.topK` | `number` | 否 | 覆盖当前配置的检索返回数。 |
| `params.similarityThreshold` | `number` | 否 | 覆盖当前配置的相似度阈值。 |
| `params.useRerank` | `boolean` | 否 | 是否启用重排序。 |
| `params.rerankThreshold` | `number` | 否 | 重排序阈值。 |
| `params.rerankTopN` | `number` | 否 | 重排序后保留数。 |
| `agent.enabledToolNames` | `string[]` | 否 | 在当前用户权限和 QA 配置允许范围内进一步收窄可用工具。 |
| `agent.maxIterations` | `number` | 否 | 覆盖本次 Agent Run 最大迭代次数；默认和上限由当前 QA 配置决定。 |

非流式响应 `202` 草案：

```json
{
  "data": {
    "responseRunId": "run_123",
    "userMessageId": "msg_user_001",
    "assistantMessageId": "msg_assistant_002",
    "status": "running"
  },
  "requestId": "req_123"
}
```

流式响应使用 `Content-Type: text/event-stream`，事件结构见下一节。

## SSE 事件协议

SSE 通用格式：

```text
event: <event_type>
id: <event_seq>
data: <json_payload>
```

事件类型：

| 事件类型 | 触发时机 | 用途 | 数据库来源 |
| --- | --- | --- | --- |
| `message.created` | 用户消息已保存，Agent Run 已创建 | 前端创建消息占位和运行状态。 | `messages`、`response_runs`。 |
| `agent.iteration.started` | 新一轮 ReAct 迭代开始 | 展示 Agent 正在规划或调用模型。 | `response_runs.current_iteration`、`agent_model_invocations`。 |
| `reasoning.step` | 可展示步骤变化 | 更新前端处理步骤列表。 | `response_process_steps`。 |
| `tool.started` | 工具调用开始 | 展示正在执行的工具摘要。 | `agent_tool_calls`。 |
| `tool.completed` | 工具调用成功 | 展示工具结果摘要。 | `agent_tool_calls`。 |
| `tool.failed` | 工具调用失败 | 展示工具失败摘要，可继续或终止。 | `agent_tool_calls`。 |
| `answer.delta` | 最终回答生成文本增量 | 流式渲染回答。 | `response_stream_events`，最终合并入 `message_content_blocks`。 |
| `citation.delta` | 引用产生或确认 | 展示引用标注。 | `citations`。 |
| `answer.completed` | 回答完成 | 关闭流式状态。 | `response_runs`、`messages`。 |
| `error` | 任何环节失败 | 展示错误并决定是否终止流。 | `response_runs.error`、`messages.error_code`。 |
| `heartbeat` | 空闲保活 | 防止代理超时。 | 可不持久化。 |

> 历史事件名 `intent`、`step`、`token`、`citation`、`done` 可以作为迁移前的兼容别名，但新 Agent 契约应优先使用上表事件。`heartbeat` 是传输层事件，不要求持久化。

示例：

```text
event: message.created
id: 1
data: {"responseRunId":"run_123","userMessageId":"msg_user_001","status":"running"}

event: agent.iteration.started
id: 2
data: {"responseRunId":"run_123","iterationNo":1}

event: tool.started
id: 3
data: {"toolCallId":"call_001","tool":"search_knowledge","summary":"正在检索 2 个知识库"}

event: tool.completed
id: 4
data: {"toolCallId":"call_001","tool":"search_knowledge","summary":"检索 2 个知识库，命中 8 个片段"}

event: answer.delta
id: 5
data: {"text":"根","index":0}

event: citation.delta
id: 6
data: {"citation":{"id":"cit_1","citationNo":1,"docId":"DOC-001","docName":"电力变压器巡检手册.pdf","chunkId":"chunk_abc","text":"变压器外壳应保持清洁...","score":0.96}}

event: answer.completed
id: 7
data: {"responseRunId":"run_123","messageId":"msg_assistant_002","totalTokens":256,"latencyMs":3200}
```

SSE 不得返回完整工具参数、完整 MCP tool result、内部 URL、原始文档全文或私有 chain-of-thought。需要展示推理过程时，只返回 `reasoning.step` 的安全摘要。

### GET /api/v1/qa-sessions/{sessionId}/events

读取短期保存的 SSE 事件，主要用于断线后恢复 UI 或调试。

查询参数：

| 参数 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `responseRunId` | `string` | 是 | 回答运行 ID。 |
| `afterEventSeq` | `number` | 否 | 只返回指定序号之后的事件。 |

响应 `200`：

```json
{
  "data": [
    {
      "eventSeq": 4,
      "eventType": "answer.delta",
      "payload": {
        "text": "根",
        "index": 0
      },
      "createdAt": "2026-06-27T10:30:16Z"
    }
  ],
  "requestId": "req_123"
}
```

## 引用溯源

### Citation

```json
{
  "id": "cit_123",
  "messageId": "msg_assistant_002",
  "citationNo": 1,
  "docId": "DOC-001",
  "docName": "电力变压器巡检手册.pdf",
  "chunkId": "chunk_abc",
  "text": "变压器外壳应保持清洁，无渗漏油现象...",
  "context": "第三章 巡检项目\n3.1 外观检查\n...",
  "pageNumber": 12,
  "score": 0.96,
  "metadata": {}
}
```

QA 返回的是回答时保存的引用快照，字段对应 `citations.external_doc_id`、`external_chunk_id`、`quote_text`、`context`、`page_number` 和 `score`。如果前端需要原文件内容，应使用 file-owned `GET /api/v1/documents/{documentId}/content`。

### GET /api/v1/messages/{messageId}/citations

查询助手消息的引用列表。

响应 `200`：

```json
{
  "data": [
    {
      "id": "cit_123",
      "messageId": "msg_assistant_002",
      "citationNo": 1,
      "docId": "DOC-001",
      "docName": "电力变压器巡检手册.pdf",
      "chunkId": "chunk_abc",
      "text": "变压器外壳应保持清洁...",
      "pageNumber": 12,
      "score": 0.96
    }
  ],
  "requestId": "req_123"
}
```

### GET /api/v1/citations/{citationId}

查询单条引用快照详情。

响应 `200`：返回 `Citation`。

### POST /api/v1/citation-lookups

批量查询引用详情草案。若需要实时从知识服务补齐 chunk 上下文，QA 应通过内部客户端调用 `knowledge`，并由 gateway 保持对前端的统一 envelope。

请求：

```json
{
  "citationIds": ["cit_123", "cit_456"]
}
```

响应 `200`：

```json
{
  "data": [
    {
      "id": "cit_123",
      "messageId": "msg_assistant_002",
      "citationNo": 1,
      "docId": "DOC-001",
      "docName": "电力变压器巡检手册.pdf",
      "chunkId": "chunk_abc",
      "text": "变压器外壳应保持清洁...",
      "context": "第三章 巡检项目\n3.1 外观检查\n...",
      "pageNumber": 12,
      "score": 0.96
    }
  ],
  "requestId": "req_123"
}
```

## QA 配置接口

### QAConfigVersion

```json
{
  "id": "qa_cfg_1",
  "versionNo": 1,
  "topK": 5,
  "similarityThreshold": 0.7,
  "useRerank": false,
  "rerankThreshold": null,
  "rerankTopN": null,
  "maxIterations": 5,
  "toolTimeoutSeconds": 10,
  "modelTimeoutSeconds": 60,
  "overallTimeoutSeconds": 120,
  "enabledToolNames": ["search_knowledge", "get_citation_source"],
  "knowledgeBases": [
    {
      "id": "kb_power_standard",
      "type": "technical_supervision",
      "displayName": "电力标准规范库",
      "sortOrder": 0
    }
  ],
  "isActive": true,
  "createdAt": "2026-06-27T09:00:00Z"
}
```

### GET /api/v1/qa-config-versions/current

查询当前生效 QA 配置。

响应 `200`：返回 `QAConfigVersion`。

### POST /api/v1/qa-config-versions

创建新的 QA 配置版本。为避免原地修改配置导致历史回答不可追溯，更新配置时创建新版本；`activate` 为 `true` 时将新版本设为当前生效版本。

请求：

```json
{
  "topK": 5,
  "similarityThreshold": 0.65,
  "useRerank": true,
  "rerankThreshold": 0.5,
  "rerankTopN": 3,
  "maxIterations": 5,
  "toolTimeoutSeconds": 10,
  "modelTimeoutSeconds": 60,
  "overallTimeoutSeconds": 120,
  "enabledToolNames": ["search_knowledge", "get_citation_source"],
  "knowledgeBases": [
    {
      "id": "kb_power_standard",
      "type": "technical_supervision",
      "displayName": "电力标准规范库",
      "sortOrder": 0
    }
  ],
  "activate": true
}
```

响应 `201`：返回新建的 `QAConfigVersion`。

QA 配置中的 Agent 控制字段含义：

| 字段 | 默认建议 | 说明 |
| --- | --- | --- |
| `maxIterations` | `5` | 单次 Agent Run 允许的最大 ReAct 迭代次数。达到上限时应以 `termination_reason=max_iterations` 结束。 |
| `toolTimeoutSeconds` | `10` | 单次 MCP 工具调用超时。 |
| `modelTimeoutSeconds` | `60` | 单次 AI Gateway 模型调用超时。 |
| `overallTimeoutSeconds` | `120` | 单次 Agent Run 总超时。 |
| `enabledToolNames` | `search_knowledge`、`get_citation_source` | 当前配置允许暴露给模型的工具白名单。实际可用工具还要按用户权限和请求参数裁剪。 |

首期工具建议：

| 工具 | Owner | 用途 |
| --- | --- | --- |
| `search_knowledge` | `knowledge` MCP server | 在用户可访问的知识库内执行检索、rerank 和结果摘要。 |
| `get_citation_source` | `knowledge` 或 `file` MCP server | 按引用 ID 或 chunk ID 查询可展示的引用来源。 |

报告生成阶段可继续注册 `generate_report_outline`、`generate_report_text`、`get_generation_status`、`get_report_result`、`export_report_docx` 等 Document MCP 工具。工具内部仍应通过 Gateway `/api/v1/**` 或受控内部契约调用业务能力，不能绕过权限边界直连数据库、MinIO、Qdrant 或领域服务私有实现。

## LLM 配置接口

### LLMConfigVersion

```json
{
  "id": "llm_cfg_1",
  "versionNo": 1,
  "provider": "ai-gateway",
  "profileId": "mp_chat_default",
  "modelName": "gpt-4o-mini",
  "timeoutSeconds": 60,
  "temperature": 0.7,
  "maxTokens": 4096,
  "isActive": true,
  "createdAt": "2026-06-27T09:00:00Z"
}
```

QA 不得保存或向前端返回 provider API key。Provider `baseUrl` 和密钥由 AI Gateway 的 model profile 管理，QA 只保存 `profileId`、模型名、超时和生成参数。

### GET /api/v1/llm-config-versions/current

查询当前生效 LLM 配置。

响应 `200`：返回 `LLMConfigVersion`。

### POST /api/v1/llm-config-versions

创建新的 LLM 配置版本。

请求：

```json
{
  "provider": "ai-gateway",
  "profileId": "mp_chat_default",
  "modelName": "Qwen/Qwen2.5-7B-Instruct",
  "timeoutSeconds": 30,
  "temperature": 0.1,
  "maxTokens": 2048,
  "activate": true
}
```

响应 `201`：返回脱敏后的 `LLMConfigVersion`。

### POST /api/v1/llm-connection-tests

通过 AI Gateway profile 创建一次 LLM 连接测试记录。该接口不接收 provider API key，也不应把 prompt、provider 原始错误或内部 URL 写入日志和响应。

请求：

```json
{
  "provider": "ai-gateway",
  "profileId": "mp_chat_default",
  "modelName": "Qwen/Qwen2.5-7B-Instruct",
  "timeoutSeconds": 30
}
```

响应 `201`：

```json
{
  "data": {
    "id": "llm_test_123",
    "success": true,
    "latencyMs": 520,
    "modelName": "Qwen/Qwen2.5-7B-Instruct",
    "testedAt": "2026-06-27T11:00:00Z"
  },
  "requestId": "req_123"
}
```

## 检索体验测试

正式知识检索由 `knowledge` 服务拥有。QA 的检索体验测试只记录管理员使用当前 QA 配置发起的一次测试运行及结果快照，对应 `retrieval_test_runs` 和 `retrieval_test_results`。

### POST /api/v1/retrieval-test-runs

请求：

```json
{
  "query": "变压器巡检要点",
  "knowledgeBaseIds": ["kb_power_standard"],
  "overrides": {
    "topK": 5,
    "similarityThreshold": 0.7,
    "useRerank": true,
    "rerankThreshold": 0.5,
    "rerankTopN": 3
  }
}
```

响应 `201`：

```json
{
  "data": {
    "id": "retrieval_run_123",
    "query": "变压器巡检要点",
    "status": "completed",
    "resultCount": 2,
    "latencyMs": 42,
    "results": [
      {
        "rankNo": 1,
        "knowledgeBaseId": "kb_power_standard",
        "docId": "DOC-001",
        "docName": "电力变压器巡检手册.pdf",
        "chunkId": "chunk_abc",
        "text": "变压器外壳应保持清洁，无渗漏油现象...",
        "vectorScore": 0.92,
        "rerankScore": 0.96,
        "metadata": {
          "pageNumber": 12,
          "chunkIndex": 5
        }
      }
    ],
    "createdAt": "2026-06-27T11:00:00Z",
    "finishedAt": "2026-06-27T11:00:01Z"
  },
  "requestId": "req_123"
}
```

### GET /api/v1/retrieval-test-runs/{testRunId}

查询检索体验测试结果。

响应 `200`：返回与创建接口一致的测试运行详情。

## QA 统计接口

统计接口基于 `response_runs`、`messages` 和 `citations` 聚合，不新增重复统计事实表。

### GET /api/v1/qa-metrics/overview

查询参数：

| 参数 | 类型 | 必填 | 默认值 | 说明 |
| --- | --- | --- | --- | --- |
| `days` | `number` | 否 | `1` | 统计窗口。 |

响应 `200`：

```json
{
  "data": {
    "totalQaCount": 12850,
    "todayQaCount": 127,
    "avgLatencyMs": 3200,
    "activeUsersToday": 23,
    "knowledgeBaseCount": 5,
    "documentCount": 78
  },
  "requestId": "req_123"
}
```

`knowledgeBaseCount` 和 `documentCount` 来源于 `knowledge` 服务或聚合缓存；QA 服务不能把知识库主数据复制为自己的源数据。

### GET /api/v1/qa-metrics/trend

查询参数：`days`，默认 `30`。

响应 `200`：

```json
{
  "data": {
    "days": 30,
    "points": [
      {
        "date": "2026-06-27",
        "count": 127
      }
    ]
  },
  "requestId": "req_123"
}
```

### GET /api/v1/qa-metrics/top-queries

查询参数：

| 参数 | 类型 | 必填 | 默认值 |
| --- | --- | --- | --- |
| `limit` | `number` | 否 | `10` |
| `days` | `number` | 否 | `7` |

响应 `200`：

```json
{
  "data": [
    {
      "query": "变压器巡检有哪些要点",
      "count": 86,
      "avgLatencyMs": 3100,
      "lastAskedAt": "2026-06-27T10:30:00Z"
    }
  ],
  "requestId": "req_123"
}
```

### GET /api/v1/qa-metrics/intent-distribution

查询参数：`days`，默认 `7`。

响应 `200`：

```json
{
  "data": [
    {
      "intent": "knowledge_qa",
      "label": "知识问答",
      "count": 650,
      "percent": 72.2
    },
    {
      "intent": "general_chat",
      "label": "一般对话",
      "count": 180,
      "percent": 20.0
    }
  ],
  "requestId": "req_123"
}
```

## Agent Loop 处理约定

前端只需要创建消息并消费 SSE 事件。QA 服务内部按以下顺序执行：

```text
user message
  -> create response_run
  -> load active QA / LLM config
  -> MCP Client tools/list
  -> filter tools by config, whitelist, user permission, and request overrides
  -> convert MCP tool schemas to OpenAI-compatible tools
  -> for iteration < maxIterations:
       call AI Gateway chat/completions(messages, tools)
       persist agent_model_invocation
       if assistant returns tool_calls:
         validate tool name, JSON schema, permissions, timeout, and idempotency
         execute MCP tools/call through MCP Client
         persist agent_tool_calls
         append role=tool results to model messages
         emit sanitized tool events
         continue
       if assistant returns final text:
         persist assistant message, content blocks, process steps, citations, and response run
         emit answer.completed
         stop
  -> if limit/timeout/cancel/error:
       persist termination_reason and safe error summary
```

旧意图值可保留为统计标签或首轮提示上下文，但不再作为固定后端编排分支：

| 历史意图 | Agent Host 处理方式 |
| --- | --- |
| `knowledge_qa` | 模型可选择 `search_knowledge`，QA 从工具结构化结果生成引用快照。 |
| `general_chat` | 模型不调用工具，直接返回最终文本。 |
| `report_generation` | 后续注册 Document MCP 工具后由模型选择对应工具；首期未注册时返回安全的不支持提示。 |
| `data_analysis` | 首期不注册数据分析工具，返回 `unsupported_intent` 或普通回答，不执行未授权工具。 |

终止原因初始值：

| termination_reason | 说明 |
| --- | --- |
| `completed` | 模型返回最终文本并完成持久化。 |
| `max_iterations` | 达到配置的最大迭代次数。 |
| `timeout` | 单次模型、单次工具或整体运行超时。 |
| `cancelled` | 用户或上游请求取消。 |
| `tool_error` | 工具调用失败且无法恢复。 |
| `model_error` | AI Gateway 或 provider 调用失败。 |
| `policy_denied` | 工具、参数或用户权限校验失败。 |

安全规则：

- MCP server 和工具必须在白名单内。
- 每次工具调用必须校验 JSON Schema。
- 根据用户权限裁剪可用工具，不把未授权工具暴露给模型。
- 工具结果必须限制长度和条数，并在进入模型上下文、日志、SSE 或数据库前脱敏。
- 只读工具失败可以重试一次；写操作必须使用幂等键，不能自动盲目重试。
- 工具结果中的 prompt injection 文本不能提升工具权限、改变系统策略或启用未授权工具。
- 远程 MCP 必须使用 HTTPS、受限出站访问和独立凭证，避免 SSRF 与 token passthrough。
- QA 服务只能保存可向用户展示的处理步骤。不得保存或返回模型私有思维链、完整 prompt、内部工具参数、向量 payload、下游内部 URL、原始 token 或完整 API key。

## 内部服务接口草案

以下接口为 gateway 到 QA 的内部草案，不直接暴露给前端；公开路径仍以 `/api/v1/**` 为准。

| Method | Internal Path | 说明 |
| --- | --- | --- |
| `POST` | `/internal/qa-sessions` | 创建会话。 |
| `GET` | `/internal/qa-sessions` | 按 `X-User-Id` 查询会话列表。 |
| `POST` | `/internal/qa-sessions/{sessionId}/messages` | 创建消息并触发回答。 |
| `GET` | `/internal/qa-sessions/{sessionId}/events` | 查询短期流式事件。 |
| `GET` | `/internal/messages/{messageId}/citations` | 查询消息引用。 |
| `GET` | `/internal/qa-config-versions/current` | 查询当前 QA 配置。 |
| `POST` | `/internal/qa-config-versions` | 创建配置版本。 |
| `GET` | `/internal/response-runs/{responseRunId}` | 查询 Agent Run 内部状态。 |
| `GET` | `/internal/response-runs/{responseRunId}/tool-calls` | 查询脱敏后的工具调用摘要。 |
| `POST` | `/internal/retrieval-test-runs` | 创建检索体验测试。 |

内部服务错误也必须被 gateway 归一化为公开错误 envelope，不能把 SQL 错误、AI Gateway 或 provider 原始错误、MCP 原始错误、密钥引用、prompt、完整工具参数、工具原始结果或内部 URL 直接返回给前端。

## OpenAPI 维护清单

调整 QA 公开接口时，需要同步：

- 更新 `docs/services/gateway/api/openapi.yaml` 中对应 active `paths`、schemas、tags、`operationId`、错误响应和 `x-owner-service`。
- 仅在确有未定公开接口时，才把该范围登记到 `x-missing-contracts`；MCP 原始 tool schema、完整工具参数/结果和内部审计不应作为前端缺失契约登记。
- 更新 [`前后端集成契约`](../../architecture/frontend-backend-contract.md) 中 SSE、Agent Run 和 QA 路径说明。
- 更新 [`服务边界矩阵`](../../architecture/service-boundaries.md) 中 QA 契约状态。
- 前端只从 gateway OpenAPI active paths 生成或实现可调用 client。
