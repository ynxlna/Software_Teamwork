# Knowledge 服务接口文档

本文档定义 `knowledge` 服务在项目初期的职责边界和 gateway 公开接口。详细字段、状态码、response envelope 和 schema 以 [`docs/services/gateway/api/openapi.yaml`](../gateway/api/openapi.yaml) 为准。前端不得直接调用 `services/knowledge`，只能通过 gateway 暴露的 `/api/v1/**` 入口访问知识库能力。

RESTful 路径、统一响应和错误 envelope 以 [前后端集成契约](../../architecture/frontend-backend-contract.md) 为准。知识检索使用 `knowledge-queries` 资源表示，不使用 `/search`、`/retrieval/search` 或其他动作式路径。

## 文档入口

| 文档 | 说明 |
| --- | --- |
| [API 契约](docs/api-contract.md) | 知识管理公开接口、权限、错误码和跨服务边界。 |
| [数据模型](docs/data-models.md) | Knowledge Service 拥有的 PostgreSQL、Qdrant 和运行时逻辑模型。 |
| [实现说明](docs/implementation.md) | 当前 Go service baseline、内部接口和本地运行说明。 |

## 技术基线

Knowledge Service 的工程选型以 [技术选型基线](../../architecture/technology-decisions.md) 为准。本服务只补充知识域特有约束：

- 文档入库、重处理和删除清理使用 `asynq` over Redis；PostgreSQL 仍是 job 状态、失败摘要和重试次数的事实来源。
- Qdrant 只保存向量和最小检索 payload；知识库、文档处理状态、chunks 和权限相关状态仍归 PostgreSQL 与 owner service。
- embedding、rerank 和后续 LLM 能力通过 AI Gateway 的 OpenAI-compatible profile 接入；Knowledge 不保存 provider API key 明文。

## 职责边界

| 范围 | 说明 |
| --- | --- |
| 知识库元数据 | 创建、查询、更新和删除知识库，维护文档类型、切片策略和检索策略。 |
| 文档处理状态 | 维护文档从 `uploaded` 到 `ready` 或 `failed` 的处理状态、错误摘要和统计字段。 |
| 文档解析与切片 | 对已进入知识库的文档做解析、语义切片和切片详情保存。 |
| 向量索引 | 生成 embedding，维护 Qdrant collection、point 和检索 payload。 |
| 检索查询 | 根据 query、知识库范围、Top K、阈值和标签过滤返回召回结果。 |

`knowledge` 不负责用户登录、RBAC 源数据、底层对象存储实现、LLM 回答生成或 DOCX 报告导出。知识库文档公开资源、处理状态和原始文件流入口由 `knowledge` 拥有；底层原始文件对象可在服务边界内复用 `file` 的基础能力。

## 接入模型

```text
frontend
   |
   v
gateway /api/v1/knowledge-bases
gateway /api/v1/knowledge-bases/{knowledgeBaseId}/documents
gateway /api/v1/documents/{documentId}/content
gateway /api/v1/documents/{documentId}/chunks
gateway /api/v1/knowledge-queries
   |
   v
knowledge service
   |
   +--> PostgreSQL metadata, document status, chunks
   +--> Qdrant vectors and retrieval payload
   +--> asynq over Redis for async ingestion/indexing workers
   +--> File service base file APIs for raw source bytes
```

Gateway 调用 knowledge 服务时应传递：

| Header | 说明 |
| --- | --- |
| `X-Request-Id` | 贯穿一次前端请求的 request id。 |
| `X-User-Id` | 已认证用户 ID。 |
| `X-User-Roles` | 逗号分隔的角色列表。 |
| `X-User-Permissions` | 逗号分隔的权限列表。 |
| `X-Forwarded-For` | 原始客户端地址链。 |
| `X-Forwarded-Proto` | 原始协议。 |

前端不得设置 `X-User-Id`、`X-User-Roles`、`X-User-Permissions`；这些字段只能由 gateway 在认证后注入。

## 公开接口总览

| Method | Gateway Path | Auth | Owner | 说明 |
| --- | --- | --- | --- | --- |
| `POST` | `/api/v1/knowledge-bases` | 需要 | `knowledge` | 创建知识库。 |
| `GET` | `/api/v1/knowledge-bases` | 需要 | `knowledge` | 分页查询知识库。 |
| `GET` | `/api/v1/knowledge-bases/{knowledgeBaseId}` | 需要 | `knowledge` | 查询知识库详情。 |
| `PATCH` | `/api/v1/knowledge-bases/{knowledgeBaseId}` | 需要 | `knowledge` | 更新知识库元数据、切片策略或检索策略。 |
| `DELETE` | `/api/v1/knowledge-bases/{knowledgeBaseId}` | 需要 | `knowledge` | 删除知识库业务状态、切片和向量索引。 |
| `POST` | `/api/v1/knowledge-bases/{knowledgeBaseId}/documents` | 需要 | `knowledge` | 上传原始文件并创建知识库文档资源；底层文件对象由 `knowledge` 在内部复用 `file` 保存。 |
| `GET` | `/api/v1/knowledge-bases/{knowledgeBaseId}/documents` | 需要 | `knowledge` | 查询知识库内文档处理状态列表。 |
| `GET` | `/api/v1/documents/{documentId}` | 需要 | `knowledge` | 查询文档处理详情。 |
| `PATCH` | `/api/v1/documents/{documentId}` | 需要 | `knowledge` | 更新知识库文档元数据，例如标签。 |
| `DELETE` | `/api/v1/documents/{documentId}` | 需要 | `knowledge` | 删除知识库文档资源，并协调切片、索引和底层 file 引用清理。 |
| `GET` | `/api/v1/documents/{documentId}/chunks` | 需要 | `knowledge` | 查询文档切片详情。 |
| `GET` | `/api/v1/documents/{documentId}/content` | 需要 | `knowledge` | 获取知识库原始文件流，路径使用 `documents/{documentId}/content` 子资源。 |
| `POST` | `/api/v1/knowledge-queries` | 需要 | `knowledge` | 创建一次知识检索查询并返回召回结果。 |

## 数据结构

公开响应统一使用 gateway envelope；格式、分页和错误响应见 [前后端集成契约](../../architecture/frontend-backend-contract.md)。

核心公开 schema：

| Schema | 说明 |
| --- | --- |
| `KnowledgeBaseSummary` | 知识库 ID、名称、描述、文档类型、切片策略、检索策略、文档数、切片数和时间字段。 |
| `DocumentSummary` | 文档 ID、知识库 ID、文件名、处理状态、错误摘要、切片数、标签和解析信息。 |
| `DocumentChunk` | 切片 ID、章节路径、切片文本、token 数、embedding 元数据和 Qdrant point ID。 |
| `KnowledgeQueryRequest` | query、knowledgeBaseIds、topK、scoreThreshold、tags、metadataFilter、rerank 配置。 |
| `KnowledgeQuerySummary` | 检索请求 ID、原始 query、召回结果列表和 trace。 |

字段详情以 [`docs/services/gateway/api/openapi.yaml`](../gateway/api/openapi.yaml) 为准，不在本文档重复定义完整 schema。

## 状态约定

`DocumentStatus` 公开枚举：

```text
uploaded | parsing | chunking | embedding | ready | failed
```

`indexing`、`reprocessing`、`deleted` 等内部阶段或扩展状态进入公开 API 前，必须先更新 gateway OpenAPI、前后端契约和本文档。

## 检索约定

知识检索使用：

```http
POST /api/v1/knowledge-queries
```

请求语义：

| 字段 | 说明 |
| --- | --- |
| `query` | 用户检索问题或关键词。 |
| `knowledgeBaseIds` | 可选知识库范围；空数组表示由权限和默认策略决定范围。 |
| `topK` | 向量召回数量上限。 |
| `scoreThreshold` | 相似度阈值，低于阈值的结果应过滤。 |
| `tags` | 标签过滤条件。 |
| `metadataFilter` | 扩展元数据过滤条件。 |
| `rerank` | 是否请求重排序；具体重排序实现由 knowledge 服务决定。 |
| `rerankTopN` | 重排序后保留数量。 |

响应必须返回可溯源字段，例如 `knowledgeBaseId`、`documentId`、`chunkId`、`documentName`、`sectionPath`、`score` 和 `contentPreview`。不要向前端返回原始向量、完整 Qdrant payload、内部 object key、prompt 或下游服务 URL。

## 与 File Service 的边界

当前公开上传入口：

```http
POST /api/v1/knowledge-bases/{knowledgeBaseId}/documents
```

该接口由 `knowledge` 拥有。Knowledge Service 负责接收 gateway 转发的 multipart、创建知识库文档资源、保存内部 file reference、维护处理状态、chunks、embedding、Qdrant 索引和检索。Knowledge 可在服务边界内调用 File Service 的 `/internal/v1/files/**` 基础接口保存和读取底层原始文件对象；File Service 不保存 `knowledgeBaseId`、文档处理状态、chunks 或索引状态。gateway 不能直接解析文件或操作 Qdrant。

## 错误码约定

Knowledge 相关接口使用项目统一错误码：

| Code | HTTP status | Knowledge 场景 |
| --- | --- | --- |
| `validation_error` | `400` | 请求体或查询参数格式错误，例如 `query` 为空、`topK` 超出范围、策略配置非法。 |
| `unauthorized` | `401` | 缺少认证凭据或 gateway 未注入有效用户上下文。 |
| `forbidden` | `403` | 已认证但无权访问目标知识库、文档或检索范围。 |
| `not_found` | `404` | 知识库、文档或切片不存在，或对当前用户隐藏。 |
| `conflict` | `409` | 当前资源状态不允许修改、删除或重新处理。 |
| `rate_limited` | `429` | 检索、上传或处理任务超过配额。 |
| `dependency_error` | `502` | PostgreSQL、Qdrant、Redis、AI Gateway 或其他依赖失败。 |
| `internal_error` | `500` | 未预期服务端错误。 |

错误响应不得包含 SQL、object key、MinIO 内部路径、原始向量、prompt、API key、token 或堆栈。

## 后续实现建议

后续落地 gateway 代理和 Knowledge Service 内部接口时，需要确认：

- Gateway 到 Knowledge Service 的内部 base URL、超时、重试和错误映射。
- `POST /api/v1/knowledge-bases/{knowledgeBaseId}/documents` 上传成功后，Knowledge 如何保存 file reference、创建 ingestion job 和处理 file 写入失败补偿。
- 知识库策略变更后是否自动创建 reprocessing job，以及 job 资源是否进入 gateway OpenAPI。
- OCR、PPT/PPTX、视觉多模态 chunk、rerank、运行时模型配置等能力进入公开 API 的版本策略。
- 契约测试覆盖 active knowledge operations 的 response envelope、字段命名、错误码和 request id。
