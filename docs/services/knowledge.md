# Knowledge 服务接口文档

本文档定义 `knowledge` 服务在项目初期的职责边界和 gateway 公开接口。详细字段、状态码、response envelope 和 schema 以 [`docs/api/gateway.openapi.yaml`](../api/gateway.openapi.yaml) 为准。前端不得直接调用 `services/knowledge`，只能通过 gateway 暴露的 `/api/v1/**` 入口访问知识库能力。

公开和内部 HTTP API 都必须使用 RESTful 资源路径，由 HTTP method 表达动作。知识检索使用 `knowledge-queries` 资源表示，不使用 `/search`、`/retrieval/search` 或其他动作式路径。

## 职责边界

| 范围 | 说明 |
| --- | --- |
| 知识库元数据 | 创建、查询、更新和删除知识库，维护文档类型、切片策略和检索策略。 |
| 文档处理状态 | 维护文档从 `uploaded` 到 `ready` 或 `failed` 的处理状态、错误摘要和统计字段。 |
| 文档解析与切片 | 对已进入知识库的文档做解析、语义切片和切片详情保存。 |
| 向量索引 | 生成 embedding，维护 Qdrant collection、point 和检索 payload。 |
| 检索查询 | 根据 query、知识库范围、Top K、阈值和标签过滤返回召回结果。 |

`knowledge` 不负责用户登录、RBAC 源数据、原始文件对象生命周期、原文件内容读取、LLM 回答生成或 DOCX 报告导出。

## 接入模型

```text
frontend
   |
   v
gateway /api/v1/knowledge-bases
gateway /api/v1/documents/{documentId}/chunks
gateway /api/v1/knowledge-queries
   |
   v
knowledge service
   |
   +--> PostgreSQL metadata, document status, chunks
   +--> Qdrant vectors and retrieval payload
   +--> Redis job/event coordination when async workers are enabled
   +--> File service handoff context for uploaded source files
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
| `GET` | `/api/v1/knowledge-bases/{knowledgeBaseId}/documents` | 需要 | `knowledge` | 查询知识库内文档处理状态列表。 |
| `GET` | `/api/v1/documents/{documentId}` | 需要 | `knowledge` | 查询文档处理详情。 |
| `GET` | `/api/v1/documents/{documentId}/chunks` | 需要 | `knowledge` | 查询文档切片详情。 |
| `POST` | `/api/v1/knowledge-queries` | 需要 | `knowledge` | 创建一次知识检索查询并返回召回结果。 |

相关但非 knowledge-owned 的公开接口：

| Method | Gateway Path | Owner | 说明 |
| --- | --- | --- | --- |
| `POST` | `/api/v1/knowledge-bases/{knowledgeBaseId}/documents` | `file` | 上传原始文件到知识库上下文。File 保存原文件和 file-owned 元数据；Knowledge 拥有后续入库状态、切片和向量索引。 |
| `PATCH` | `/api/v1/documents/{documentId}` | `file` | 更新 file-owned 文档标签等元数据。 |
| `DELETE` | `/api/v1/documents/{documentId}` | `file` | 删除 file-owned 文档记录和原始文件；Knowledge 索引清理需通过内部协调完成。 |
| `GET` | `/api/v1/documents/{documentId}/content` | `file` | 获取原始文件内容。 |

## 数据结构

公开响应统一使用 gateway envelope：

```json
{
  "data": {},
  "requestId": "req_123"
}
```

分页响应：

```json
{
  "data": [],
  "page": {
    "page": 1,
    "pageSize": 20,
    "total": 100
  },
  "requestId": "req_123"
}
```

错误响应：

```json
{
  "error": {
    "code": "validation_error",
    "message": "request validation failed",
    "requestId": "req_123",
    "fields": {
      "query": "is required"
    }
  }
}
```

核心公开 schema：

| Schema | 说明 |
| --- | --- |
| `KnowledgeBaseSummary` | 知识库 ID、名称、描述、文档类型、切片策略、检索策略、文档数、切片数和时间字段。 |
| `DocumentSummary` | 文档 ID、知识库 ID、文件名、处理状态、错误摘要、切片数、标签和解析信息。 |
| `DocumentChunk` | 切片 ID、章节路径、切片文本、token 数、embedding 元数据和 Qdrant point ID。 |
| `KnowledgeQueryRequest` | query、knowledgeBaseIds、topK、scoreThreshold、tags、metadataFilter、rerank 配置。 |
| `KnowledgeQuerySummary` | 检索请求 ID、原始 query、召回结果列表和 trace。 |

字段详情以 [`docs/api/gateway.openapi.yaml`](../api/gateway.openapi.yaml) 为准，不在本文档重复定义完整 schema。

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

该接口由 `file` 拥有。File Service 负责接收 multipart、保存原始文件和 file-owned 元数据；Knowledge Service 负责文档进入知识库后的处理状态、chunks、embedding、Qdrant 索引和检索。File 与 Knowledge 的内部 handoff 可以通过内部 HTTP 资源接口、消息或任务队列实现，但不能让 gateway 直接解析文件或操作 Qdrant。

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
| `dependency_error` | `502` | PostgreSQL、Qdrant、Redis、embedding provider 或其他依赖失败。 |
| `internal_error` | `500` | 未预期服务端错误。 |

错误响应不得包含 SQL、object key、MinIO 内部路径、原始向量、prompt、API key、token 或堆栈。

## 后续实现建议

后续落地 gateway 代理和 Knowledge Service 内部接口时，需要确认：

- Gateway 到 Knowledge Service 的内部 base URL、超时、重试和错误映射。
- `POST /api/v1/knowledge-bases/{knowledgeBaseId}/documents` 上传成功后的 file -> knowledge handoff 方式。
- 知识库策略变更后是否自动创建 reprocessing job，以及 job 资源是否进入 gateway OpenAPI。
- OCR、PPT/PPTX、视觉多模态 chunk、rerank、运行时模型配置等能力进入公开 API 的版本策略。
- 契约测试覆盖 active knowledge operations 的 response envelope、字段命名、错误码和 request id。
