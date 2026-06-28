# Gateway 服务规划

本文档定义 `gateway` 服务在项目初期的职责边界和基础契约。目标是让前端只依赖一个稳定入口，同时让 `auth`、`file`、`knowledge`、`qa`、`document`、`ai-gateway` 等服务可以按清晰边界并行开发。

相关文档：

- [Gateway 数据模型文档](docs/data-models.md)
- [Gateway OpenAPI 契约](api/openapi.yaml)
- [技术选型基线](../../architecture/technology-decisions.md)

## 设计原则

- `gateway` 是面向前端、管理端、其他后端模块和工具调用方的后端统一入口，不是业务大单体。
- 所有公开业务请求都必须先进入 `gateway` 暴露的 `/api/v1/**` 接口，不直接调用内部服务。
- `gateway` 通过 HTTP/REST 调用内部服务，不 import 其他服务的 Go `internal/` 包。
- AI Gateway 是内部模型服务；前端不得直接调用其 `/internal/v1/**` 或 OpenAI-compatible endpoint。
- 稳定 API 的 RESTful 命名、统一 envelope、错误和 request id 规则以 [前后端集成契约](../../architecture/frontend-backend-contract.md) 为准。
- 领域业务规则尽量留在拥有该领域数据和流程的服务中。
- 跨服务聚合接口必须有明确前端场景，不能把所有服务编排都放进 `gateway`。
- OpenAPI 契约先行，代码实现必须跟随契约变更。

## 技术选型落地约束

Gateway 后续实现必须遵循 [技术选型基线](../../architecture/technology-decisions.md)。本服务特有约束只包括：

- 代码落地时使用独立 Go module，目录建议为 `services/gateway/`。
- 首期不拥有业务数据库，因此不维护 `sqlc.yaml` 或 `migrations/`；若后续新增自有持久化模型，需回到技术选型基线补齐 `pgx` + `sqlc` 和 `goose`。
- Redis 只保存会话缓存、短期缓存和运行时状态，不作为长期业务真相。
- 路由、中间件、认证缓存、错误映射和 SSE 转发是 Gateway 测试重点。
- 本目录 OpenAPI 是前端 `openapi-typescript` 类型生成的 public gateway 权威契约；内部服务 OpenAPI 不生成到前端。

## Gateway 应负责

| 能力 | 说明 |
| --- | --- |
| Public API surface | 暴露前端、管理端、其他后端模块和工具调用方使用的 `/api/v1/**` HTTP API。 |
| Routing | 将已确定的公开请求转发到 `auth`、`file`、`knowledge`、`qa`、`document`、`ai-gateway` 等内部服务；未定下游服务只保留缺失占位。 |
| Auth context | 基于 Redis 会话缓存读取用户身份，并向下游传递用户、角色、权限和 request id。 |
| Session cache | 用户或会话创建成功后缓存 auth 返回的会话身份信息，后续请求优先从 Redis 获取会话上下文。 |
| Response contract | 对前端保持统一成功响应、分页响应和错误响应结构。 |
| Request correlation | 生成或透传 `X-Request-Id`，并要求下游服务保留该 request id。 |
| Admin runtime configuration entrypoint | 暴露模型 profile 和文档解析器配置的管理入口；模型配置转发给 `ai-gateway`，解析器配置转发给 `knowledge`。 |
| Cross-service aggregation | 仅在前后端契约明确后提供聚合读接口；本轮管理后台概览暂标缺失。 |
| Streaming entrypoint | 问答通过 `POST /api/v1/qa-sessions/{sessionId}/messages` 提供 `text/event-stream` 响应，并通过 `/api/v1/qa-sessions/{sessionId}/events` 提供短期事件回放；报告生成当前提供事件列表资源，后续如需 SSE 需先补 OpenAPI 契约。 |
| Edge policy | 集中处理 CORS、基础请求头、请求大小原则、健康检查和公开 API 命名。 |

## Gateway 不应负责

| 领域 | 归属服务 | Gateway 不做什么 |
| --- | --- | --- |
| 用户、密码、会话、角色权限源数据 | `auth` | 不保存密码，不维护用户表，不实现 RBAC 持久化；只在 Redis 保存运行时会话缓存。 |
| 文件对象、基础 file 元数据、对象存储协调 | `file` | 不直接操作 MinIO，不生成业务 object key。 |
| 知识库、文档切片、向量索引、检索策略 | `knowledge` | 不执行切片、嵌入、Qdrant 查询或重排序。 |
| 问答 Agent、MCP 工具编排、LLM 调用 | `qa` | 不拼 prompt，不执行 ReAct loop，不执行 MCP 工具，不保存对话业务状态。 |
| 报告大纲、章节生成、DOCX 导出 | `document` | 不生成报告内容，不操作报告模板业务规则。 |
| 模型 provider 配置、API key、chat/embedding/rerank 调用 | `ai-gateway` | 不保存 provider API key，不直连 OpenAI-compatible 或 SiliconFlow-compatible provider，不把内部模型调用接口暴露给前端；只通过 admin model-profile 资源转发配置管理请求。 |
| 服务数据库迁移 | 各领域服务 | 不拥有其他服务的 migrations。 |

## Public API 命名

第一版公开 API 使用统一版本前缀：

```text
/api/v1
```

健康检查接口不带版本前缀：

```text
/healthz
/readyz
```

当前已确定路径分组：

| Gateway path | 初始 owner | 说明 |
| --- | --- | --- |
| `/healthz` | `gateway` | 进程存活检查。 |
| `/readyz` | `gateway` | 就绪检查。 |
| `/api/v1/users` | `auth` | 创建用户。 |
| `/api/v1/sessions` | `auth` | 创建登录会话。 |
| `/api/v1/sessions/current` | `auth` | 删除当前登录会话。 |
| `/api/v1/users/me` | `auth` | 获取当前用户。 |
| `/api/v1/knowledge-bases` | `knowledge` | 创建知识库、分页查询知识库。 |
| `/api/v1/knowledge-bases/{knowledgeBaseId}` | `knowledge` | 查询、更新、删除知识库。 |
| `POST /api/v1/knowledge-bases/{knowledgeBaseId}/documents` | `knowledge` | 知识库文档上传入口。Knowledge 创建文档资源、保存知识库归属和处理状态，并在内部复用 file 保存底层原文件对象。 |
| `GET /api/v1/knowledge-bases/{knowledgeBaseId}/documents` | `knowledge` | 查询知识库内文档列表和处理状态。 |
| `GET /api/v1/documents/{documentId}` | `knowledge` | 查询文档处理详情。 |
| `PATCH/DELETE /api/v1/documents/{documentId}` | `knowledge` | 更新或删除知识库文档资源，并由 Knowledge 协调切片、索引和底层 file 引用。 |
| `/api/v1/documents/{documentId}/chunks` | `knowledge` | 查询文档切片详情。 |
| `/api/v1/documents/{documentId}/content` | `knowledge` | 获取知识库原始文件内容；底层 bytes 可由 Knowledge 从 file 服务读取。 |
| `/api/v1/knowledge-queries` | `knowledge` | 创建一次知识检索查询，返回召回结果和 trace。 |
| `/api/v1/report-types` | `document` | 查询报告类型。 |
| `/api/v1/report-templates`、`/api/v1/report-templates/{reportTemplateId}`、`/api/v1/report-templates/{reportTemplateId}/structure` | `document` | 报告模板上传、查询、更新、删除和结构配置。 |
| `/api/v1/report-materials`、`/api/v1/report-materials/{materialId}` | `document` | 报告素材上传、查询和删除。素材是 document-owned 独立资源，底层文件对象复用 file 服务。 |
| `/api/v1/reports`、`/api/v1/reports/{reportId}` | `document` | 报告草稿、记录、详情、基础信息更新和删除。 |
| `/api/v1/reports/{reportId}/outlines`、`/api/v1/reports/{reportId}/outlines/{outlineId}`、`/api/v1/reports/{reportId}/outlines/{outlineId}/sections/{sectionId}` | `document` | 报告大纲版本查询、保存、编辑和章节删除。 |
| `/api/v1/reports/{reportId}/sections`、`/api/v1/reports/{reportId}/sections/{sectionId}`、`/api/v1/reports/{reportId}/sections/{sectionId}/versions` | `document` | 报告章节查询、编辑和章节版本创建。 |
| `/api/v1/reports/{reportId}/jobs`、`/api/v1/report-jobs/{jobId}`、`/api/v1/report-jobs/{jobId}/attempts` | `document` | 报告生成、重新生成、文件创建等长任务资源及任务尝试记录。 |
| `/api/v1/reports/{reportId}/events` | `document` | 报告生成事件列表，用于轮询进度或审计。 |
| `/api/v1/report-files`、`/api/v1/report-files/{reportFileId}`、`/api/v1/report-files/{reportFileId}/content` | `document` | 报告文件创建、元数据查询和生成文件内容读取。生成文件是 document-owned 业务资源，底层对象存取复用 file 服务。 |
| `/api/v1/report-statistics/overview`、`/api/v1/report-statistics/daily` | `document` | 报告统计概览和每日趋势。 |
| `/api/v1/report-operation-logs` | `document` | 报告相关操作日志查询。 |
| `/api/v1/report-settings` | `document` | 管理端报告生成配置，包含 AI Gateway profile 引用、默认模板和文件生成默认值。 |
| `/api/v1/admin/model-profiles`、`/api/v1/admin/model-profiles/{profileId}` | `ai-gateway` | 管理端运行时模型配置入口，覆盖 chat、embedding、rerank profile；Gateway 做管理员鉴权、响应归一化和密钥脱敏转发，不保存 API key。 |
| `/api/v1/admin/parser-configs`、`/api/v1/admin/parser-configs/{parserConfigId}` | `knowledge` | 管理端文档解析器配置入口，覆盖解析后端选择、并发数和默认参数；Knowledge 拥有解析行为。 |
| `/api/v1/qa-sessions`、`/api/v1/qa-sessions/{sessionId}`、`/api/v1/qa-sessions/{sessionId}/messages`、`/api/v1/qa-sessions/{sessionId}/events` | `qa` | 智能问答会话、消息、非流式/流式回答和短期 SSE 事件回放。 |
| `/api/v1/response-runs/{responseRunId}`、`/api/v1/response-runs/{responseRunId}/tool-calls` | `qa` | 回答运行状态、取消和脱敏工具调用摘要。 |
| `/api/v1/messages/{messageId}/citations`、`/api/v1/citations/{citationId}`、`/api/v1/citation-lookups` | `qa` | 回答引用快照、引用详情和批量引用详情查询。 |
| `/api/v1/qa-config-versions/current`、`/api/v1/qa-config-versions`、`/api/v1/llm-config-versions/current`、`/api/v1/llm-config-versions`、`/api/v1/llm-connection-tests` | `qa` | 问答运行配置、AI Gateway profile 引用和连接测试；provider base URL 与 API key 仍由 AI Gateway 管理。 |
| `/api/v1/retrieval-test-runs`、`/api/v1/retrieval-test-runs/{testRunId}`、`/api/v1/qa-metrics/overview`、`/api/v1/qa-metrics/trend`、`/api/v1/qa-metrics/top-queries`、`/api/v1/qa-metrics/intent-distribution` | `qa` | 检索体验测试和问答统计。 |

仍暂缺的下游接口：

| Placeholder | 预期 owner | 状态 |
| --- | --- | --- |
| `GET /api/v1/admin-overview`、`GET /api/v1/admin-metrics` | `gateway` + domain services | 缺失：概览/指标聚合来源和展示字段未定。模型 profile 和解析器配置管理不属于该缺失范围，已在 active paths 中定义。 |

当某个 endpoint 涉及两个服务时，文档必须显式标注 workflow owner。默认规则是：拥有核心业务状态的服务拥有流程，gateway 只做入口和上下文传递。若流程需要模型能力，领域服务应通过 [AI Gateway 服务接口文档](../ai-gateway/README.md) 和 [AI Gateway OpenAPI 契约](../ai-gateway/api/openapi.yaml) 调用内部模型接口，不能让 public gateway 直接拼 prompt 或直连 provider。

## 认证与上下文传递

认证机制初期采用 opaque bearer token + Redis 会话缓存。Auth 服务负责认证、签发不透明随机 access token、维护会话身份和撤销会话；Gateway 负责在用户或会话创建成功后写入 Redis，并在后续请求中从 Redis 读取会话上下文。

前端请求：

- 登录类接口不要求认证。
- 业务接口必须携带认证凭据。
- 前端不直接设置用户身份 header，用户身份由 gateway 认证后注入。
- 后续请求使用 `Authorization: Bearer <accessToken>` 携带 gateway 返回的访问令牌；该令牌不可被前端或 Gateway 当作 JWT 解析。

会话缓存流程：

1. 前端调用 `/api/v1/sessions` 或 `/api/v1/users`。
2. Gateway 将请求转发给 auth 服务。
3. Auth 服务校验凭证，返回用户身份、角色、权限、`sessionId`、不透明 `accessToken` 和 `expiresAt`。
4. Gateway 将完整会话身份写入 Redis，缓存键使用 `gateway:session:<accessTokenHash>`，TTL 与 `expiresAt` 对齐。
5. 前端后续请求携带 `Authorization: Bearer <accessToken>`。
6. Gateway 从 Redis 查询会话；命中且未过期时，不需要每次调用 auth 服务。
7. Gateway 基于缓存的会话身份向下游服务注入 `X-User-Id`、`X-User-Roles`、`X-User-Permissions` 和 `X-Request-Id`。
8. 当前会话删除时 Gateway 调用 auth 删除会话，并删除 Redis 中的对应缓存。

Redis 会话缓存值应至少包含：

| 字段 | 说明 |
| --- | --- |
| `sessionId` | Auth 服务签发的会话 ID。 |
| `userId` | 已认证用户 ID。 |
| `username` | 用户名，用于审计和调试，不作为权限判断唯一依据。 |
| `roles` | 角色列表。 |
| `permissions` | 权限字符串列表。 |
| `accessTokenHash` | 访问令牌哈希，避免把原始 token 当作可读缓存字段。 |
| `expiresAt` | 会话过期时间，使用 RFC 3339 / OpenAPI `date-time`。 |
| `issuedAt` | 会话签发时间。 |

缓存规则：

- Redis 是运行时会话缓存，不是用户、角色、权限的持久化源数据。
- 每条会话缓存必须设置明确 TTL。
- Gateway 日志和错误响应不得输出原始 token、session secret 或 Redis 连接信息。
- Redis 未命中、会话过期或缓存内容无效时，Gateway 返回 `401 unauthorized`，前端回到登录流程。
- Redis 不可用时，业务接口返回 `502 dependency_error`；登录、注册和登出等 auth 流程可以按实现策略选择失败或降级，但必须保持错误 envelope 一致。
- 权限变更、用户禁用或安全事件需要让旧会话失效时，auth 服务应提供撤销能力，Gateway 删除对应 Redis 会话缓存。

Gateway 调用下游服务时应传递：

| Header | 说明 |
| --- | --- |
| `X-Request-Id` | 贯穿一次前端请求的 request id。 |
| `X-User-Id` | 已认证用户 ID。 |
| `X-User-Roles` | 逗号分隔的角色列表。 |
| `X-User-Permissions` | 逗号分隔的权限列表，字段细节由 auth 契约细化。 |
| `X-Forwarded-For` | 原始客户端地址链。 |
| `X-Forwarded-Proto` | 原始协议。 |

下游服务仍需在自己的边界做权限校验，不能只依赖前端传参。

## Gateway User / Session 接口

Gateway 对前端暴露 auth 相关公开接口，具体 schema 以 [`docs/services/gateway/api/openapi.yaml`](api/openapi.yaml) 为准。

| Method | Path | Auth | Gateway 行为 | Auth service 行为 |
| --- | --- | --- | --- | --- |
| `POST` | `/api/v1/users` | 不需要 | 转发用户创建请求，成功后写入 Redis 会话缓存并返回统一 envelope。 | 创建用户、计算角色权限、签发会话身份。 |
| `POST` | `/api/v1/sessions` | 不需要 | 转发会话创建请求，成功后写入 Redis 会话缓存并返回统一 envelope。 | 校验凭证、计算角色权限、签发会话身份。 |
| `DELETE` | `/api/v1/sessions/current` | 需要 | 从 Redis 定位当前会话，调用 auth 删除会话，删除 Redis 缓存。 | 删除会话或令牌，记录安全事件。 |
| `GET` | `/api/v1/users/me` | 需要 | 从 Redis 会话缓存读取当前用户并返回 `UserResponse`。 | 拥有用户和权限源数据；默认不参与每次 `/me` 查询。 |

用户或会话创建成功响应包含：

```json
{
  "data": {
    "user": {
      "id": "usr_123",
      "username": "alice",
      "roles": ["admin"],
      "permissions": ["knowledge:read", "document:upload"]
    },
    "session": {
      "sessionId": "sess_123",
      "accessToken": "tok_8Ywq7T2n4pQ9mR3xV6sL",
      "tokenType": "Bearer",
      "expiresAt": "2026-06-28T12:00:00Z"
    }
  },
  "requestId": "req_123"
}
```

Gateway 必须只把 `data.session.accessToken` 返回给前端，不得把 Redis key、token hash、内部 auth URL 或 session secret 暴露给前端。

## Gateway Knowledge 接口

Gateway 对前端暴露 knowledge 相关公开接口，具体 schema 以 [`docs/services/gateway/api/openapi.yaml`](api/openapi.yaml) 为准。Gateway 只负责鉴权上下文传递、路由和响应归一化，不执行解析、切片、embedding、Qdrant 检索或重排序。

| Method | Path | Auth | Gateway 行为 | Knowledge service 行为 |
| --- | --- | --- | --- | --- |
| `POST` | `/api/v1/knowledge-bases` | 需要 | 转发知识库创建请求并返回统一 envelope。 | 创建知识库元数据、保存切片和检索策略。 |
| `GET` | `/api/v1/knowledge-bases` | 需要 | 转发分页查询参数并返回统一分页 envelope。 | 返回用户可访问的知识库列表和统计字段。 |
| `GET` | `/api/v1/knowledge-bases/{knowledgeBaseId}` | 需要 | 转发知识库详情查询。 | 返回知识库元数据、文档数、切片数和策略配置。 |
| `PATCH` | `/api/v1/knowledge-bases/{knowledgeBaseId}` | 需要 | 转发局部更新请求。 | 更新知识库元数据、分段策略或检索策略；必要时触发后续重处理流程。 |
| `DELETE` | `/api/v1/knowledge-bases/{knowledgeBaseId}` | 需要 | 转发删除请求。 | 删除知识库业务状态、切片和向量索引，或按实现策略标记删除并异步清理。 |
| `GET` | `/api/v1/knowledge-bases/{knowledgeBaseId}/documents` | 需要 | 转发分页和状态过滤参数。 | 返回知识库内文档处理状态列表。 |
| `GET` | `/api/v1/documents/{documentId}` | 需要 | 转发文档详情查询。 | 返回文档处理状态、错误摘要、切片数量和解析信息。 |
| `GET` | `/api/v1/documents/{documentId}/chunks` | 需要 | 转发分页参数。 | 返回文档切片、章节路径、embedding 元数据和 Qdrant point ID。 |
| `POST` | `/api/v1/knowledge-queries` | 需要 | 转发检索请求并返回统一 envelope。 | 执行向量召回、过滤、可选重排序预留，并返回命中文档、分数、摘要和 trace。 |

检索被建模为 `knowledge-queries` 资源创建，因此公开路径使用 `POST /api/v1/knowledge-queries`，不使用 `/search` 或 `/retrieval/search`。

知识库文档公开资源统一由 `knowledge` 拥有：`POST /api/v1/knowledge-bases/{knowledgeBaseId}/documents` 创建文档资源并保存底层 file reference，`GET /api/v1/knowledge-bases/{knowledgeBaseId}/documents` 返回处理状态列表，`PATCH/DELETE /api/v1/documents/{documentId}` 更新或删除知识库文档资源，`GET /api/v1/documents/{documentId}/content` 使用 `documents/{documentId}/content` 子资源返回原文件流。Gateway 不直接解析文件、操作 MinIO 或操作 Qdrant。

报告素材、模板和导出文件不得复用知识库文档上传路径建模。它们的公开资源由 `document` 拥有，`document` 在内部通过 file 服务保存、读取或删除底层文件对象；Gateway 只做入口、认证上下文传递和响应归一化。

## Gateway QA 接口

Gateway 对前端暴露智能问答相关公开接口，具体 schema 以 [`docs/services/gateway/api/openapi.yaml`](api/openapi.yaml) 为准。Gateway 只负责认证上下文、统一 envelope、SSE 转发和错误归一化；`qa` 服务拥有会话、消息、回答运行、Agent/ReAct 循环、MCP 工具编排、引用快照、配置版本、检索体验测试和问答统计。

| Method | Path | Auth | Gateway 行为 | QA service 行为 |
| --- | --- | --- | --- | --- |
| `GET/POST` | `/api/v1/qa-sessions` | 需要 | 转发当前用户会话查询或创建请求。 | 保存和查询 QA 会话。 |
| `GET/PATCH/DELETE` | `/api/v1/qa-sessions/{sessionId}` | 需要 | 转发会话详情、标题/状态更新和软删除请求。 | 校验会话归属并维护会话生命周期。 |
| `GET/POST` | `/api/v1/qa-sessions/{sessionId}/messages` | 需要 | 转发消息列表查询；当请求 `Accept: text/event-stream` 时转发 SSE 流。 | 创建用户消息，执行 Agent Run，保存助手消息、处理步骤、引用和事件。 |
| `GET` | `/api/v1/qa-sessions/{sessionId}/events` | 需要 | 转发短期事件回放查询。 | 返回指定 `responseRunId` 的已保存 SSE 事件。 |
| `GET/PATCH` | `/api/v1/response-runs/{responseRunId}` | 需要 | 转发运行状态查询和取消请求。 | 维护回答运行状态并尽力取消 LLM/MCP 下游请求。 |
| `GET` | `/api/v1/response-runs/{responseRunId}/tool-calls` | 需要 | 转发工具调用摘要查询。 | 返回脱敏后的工具调用摘要，不返回 MCP 原始参数或结果。 |
| `GET/POST` | `/api/v1/messages/{messageId}/citations`、`/api/v1/citations/{citationId}`、`/api/v1/citation-lookups` | 需要 | 转发引用列表、详情和批量详情查询。 | 返回回答生成时保存的引用快照和可展示来源信息。 |
| `GET/POST` | `/api/v1/qa-config-versions/current`、`/api/v1/qa-config-versions`、`/api/v1/llm-config-versions/current`、`/api/v1/llm-config-versions`、`/api/v1/llm-connection-tests` | 需要 | 转发配置读取、创建和连接测试。 | 保存 QA 配置版本和 AI Gateway profile 引用，不保存 provider API key。 |
| `GET/POST` | `/api/v1/retrieval-test-runs`、`/api/v1/retrieval-test-runs/{testRunId}` | 需要 | 转发检索体验测试创建和查询。 | 调用 knowledge 检索能力并保存测试结果快照。 |
| `GET` | `/api/v1/qa-metrics/**` | 需要 | 转发问答统计查询。 | 基于 response runs、messages 和 citations 聚合统计。 |

QA SSE 事件类型包括 `message.created`、`agent.iteration.started`、`reasoning.step`、`tool.started`、`tool.completed`、`tool.failed`、`answer.delta`、`citation.delta`、`answer.completed`、`error` 和可选 `heartbeat`。SSE 事件、工具摘要和错误响应不得包含完整工具参数、MCP 原始响应、内部 URL、原始文档全文、prompt、provider 原始错误或存储 object key。

## 响应约定

Gateway 负责对前端保持统一成功响应、分页响应和错误响应结构。通用 envelope、错误码和前端处理规则见 [前后端集成契约](../../architecture/frontend-backend-contract.md)。本节不再重复定义格式，避免与 OpenAPI 和架构契约漂移。

Gateway 可透传或映射 owner service 的服务特有错误码，但任何稳定公开错误都必须先进入 [`api/openapi.yaml`](api/openapi.yaml)。

## 缺失下游接口

管理后台概览/指标聚合的前后端接口尚未完全确定。当前 OpenAPI 只在顶层 `x-missing-contracts` 标记这些缺失范围，不把这些 endpoint 作为可依赖的公开契约。QA 会话、消息、SSE、引用、配置、检索体验测试和统计已经进入 active paths。

AI Gateway 的内部模型调用接口已经有独立契约：[`docs/services/ai-gateway/api/openapi.yaml`](../ai-gateway/api/openapi.yaml)。该契约不属于前端可调用的 gateway OpenAPI，也不应生成到前端 API client。前端需要管理运行时模型配置时，只能使用 gateway OpenAPI 中的 `/api/v1/admin/model-profiles` 资源；gateway 再调用 AI Gateway 内部 `/internal/v1/model-profiles`。

后续补齐任一缺失接口时，需要同步更新：

- `docs/services/gateway/api/openapi.yaml`
- `docs/architecture/frontend-backend-contract.md`
- `docs/architecture/service-boundaries.md`
- 对应服务接口文档

## 健康检查

| Endpoint | 说明 |
| --- | --- |
| `GET /healthz` | 进程存活检查，只表示 gateway 进程可响应。 |
| `GET /readyz` | 就绪检查，后续可包含关键下游依赖状态。 |

## 后续扩展

本轮只定义基础契约包。以下内容后续单独细化：

- 下游服务完整内部 API 索引。
- 超时、重试、熔断和断线重连策略。
- API 版本兼容策略。
- 限流、审计和安全事件记录。
- 多端 BFF 拆分条件。
