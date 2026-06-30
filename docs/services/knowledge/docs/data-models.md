# Knowledge Service 数据模型文档

版本：v0.2
日期：2026-06-30
范围：`services/knowledge/` 负责的知识库、知识文档、处理任务、文档切片、向量索引和检索查询逻辑模型

## 1. 文档定位

本文定义 Knowledge Service 的服务内数据模型，用于指导 Go service、PostgreSQL goose migration、Qdrant payload、gateway 代理契约对齐和后续测试用例编写。

本文只描述 Knowledge Service 拥有的业务数据，不替代 gateway 公开 OpenAPI，也不定义 File Service、QA Service 或 Document Service 的内部模型。前端可见字段仍以 `docs/services/gateway/api/openapi.yaml` 为准；服务本地内部接口以 `services/knowledge/api/openapi.yaml` 和 Go DTO 为准。

## 2. 技术基线

本数据模型按 [`docs/architecture/technology-decisions.md`](../../../architecture/technology-decisions.md) 中的后端选型落地：

| 领域 | 数据模型约束 |
| --- | --- |
| PostgreSQL 访问 | 使用 `pgx` + `sqlc`；SQL 必须显式列名，用户输入必须通过参数绑定传入。 |
| ORM | 不使用 GORM/ent 等 ORM；schema、query 和事务边界由 migration、sqlc query 和 service/use-case 层维护。 |
| Migration | 使用 `goose`，迁移文件放在 `services/knowledge/migrations/` 并采用有序前缀。 |
| 事务 | 跨知识库、文档、job、chunk 的一致性由 service/use-case 层发起事务，repository 接收 `pgx.Tx` 或抽象后的 querier。 |
| Redis/asynq | Redis 只负责排队、调度、短期进度和并发协调；`processing_jobs` 表保存最终状态、失败摘要、重试次数和审计线索。 |
| Qdrant | Qdrant 只保存向量和最小 payload；展示正文、权限判断、状态判断必须回 PostgreSQL hydrate。 |

## 3. 边界原则

- PostgreSQL 是知识库元数据、文档状态、任务状态、切片正文和处理错误的事实来源。
- Qdrant 只保存向量和检索所需的最小 payload，不作为文档状态、任务状态或正文完整性的事实来源。
- 原始文件二进制、MinIO object key、文件下载权限和底层文件元数据归 File Service；Knowledge Service 只保存内部 `file_ref`，不把 file 服务内部 ID 暴露为公开文档字段。
- 文档解析运行时归 Parser Service；Knowledge 只消费 Parser 返回的规范化 parsed content，并负责后续切片、embedding、Qdrant 写入和状态推进。
- `knowledge-queries` 是检索请求资源。当前实现即时执行并返回结果，不持久化查询历史；如后续需要审计或调试留痕，应新增独立表。
- 删除采用软删除优先：知识库和文档通过 `deleted_at` 从常规列表中隐藏，任务和切片清理由后续 cleanup job 兜底。

## 4. 命名约定

| 层 | 命名风格 | 示例 |
| --- | --- | --- |
| Public/Internal HTTP JSON | camelCase | `knowledgeBaseId`, `chunkCount`, `createdAt` |
| PostgreSQL table/column | snake_case | `knowledge_base_id`, `chunk_count`, `created_at` |
| Go domain field | MixedCaps | `KnowledgeBaseID`, `ChunkCount`, `CreatedAt` |
| Qdrant payload | snake_case | `knowledge_base_id`, `document_id`, `chunk_id` |
| Public enum | lowercase snake/case word | `uploaded`, `delete_cleanup` |

## 5. 实体关系概览

```text
knowledge_bases 1 ── n knowledge_documents
knowledge_bases 1 ── n processing_jobs
knowledge_documents 1 ── n document_chunks
knowledge_documents 1 ── n processing_jobs
document_chunks 1 ── 1 Qdrant point

knowledge_query
  └── reads Qdrant hits
  └── hydrates document_chunks and knowledge_documents from PostgreSQL

parser service
  └── accepts raw bytes from Knowledge
  └── returns normalized parsed content
```

关系约束：

- `knowledge_documents.knowledge_base_id` 必须引用 `knowledge_bases.id`。
- `document_chunks.document_id` 必须引用 `knowledge_documents.id`。
- `document_chunks.knowledge_base_id` 必须与所属文档的 `knowledge_base_id` 一致。
- `processing_jobs.document_id` 可为空，用于知识库级重处理或清理任务。
- `knowledge_documents.current_job_id` 是便捷引用，不作为唯一任务来源；任务历史以 `processing_jobs` 为准。

## 6. 核心实体

### 6.1 KnowledgeBase

知识库是知识文档、切片策略和检索策略的聚合根。

| 逻辑字段 | HTTP 字段 | PostgreSQL 字段 | 类型 | 说明 |
| --- | --- | --- | --- | --- |
| `ID` | `id` | `id` | text | 业务 ID，建议使用 `kb_` 前缀。 |
| `Name` | `name` | `name` | text | 展示名，当前最大 120 字符。 |
| `Description` | `description` | `description` | text | 描述，当前最大 2000 字符。 |
| `DocType` | `docType` | `doc_type` | text | 文档领域类型，默认 `GENERAL`。 |
| `ChunkStrategy` | `chunkStrategy` | `chunk_strategy` | jsonb | 切片策略，默认语义文本切片。 |
| `RetrievalStrategy` | `retrievalStrategy` | `retrieval_strategy` | jsonb | 检索策略，默认向量检索。 |
| `DocumentCount` | `documentCount` | derived | integer | 从未删除文档聚合得出，不单独持久化。 |
| `ChunkCount` | `chunkCount` | derived | integer | 从切片聚合得出，不单独持久化。 |
| `CreatedBy` | `createdBy` | `created_by` | text | 创建人用户 ID，由 gateway/auth 上下文传入。 |
| `CreatedAt` | `createdAt` | `created_at` | timestamptz | 创建时间。 |
| `UpdatedAt` | `updatedAt` | `updated_at` | timestamptz | 最后更新时间。 |
| `DeletedAt` | hidden | `deleted_at` | timestamptz nullable | 软删除时间。 |

默认策略示例：

```json
{
  "chunkStrategy": {
    "type": "SEMANTIC_TEXT",
    "size": 1600,
    "overlap": 200
  },
  "retrievalStrategy": {
    "mode": "VECTOR",
    "topK": 10,
    "scoreThreshold": 0.35
  }
}
```

关键索引：

- `idx_knowledge_bases_created_at` 支持按创建时间倒序分页。
- `idx_knowledge_bases_doc_type` 支持按 `doc_type` 过滤未删除知识库。
- `idx_knowledge_bases_deleted_at` 支持软删除过滤和后台清理扫描。

### 6.2 KnowledgeDocument

知识文档是 Knowledge Service 拥有的可检索文档资源。它引用 File Service 中的原文件，并维护解析协调、切片、embedding 和错误状态；具体 OCR/PaddleOCR 运行时在 Parser Service 内。

| 逻辑字段 | HTTP 字段 | PostgreSQL 字段 | 类型 | 说明 |
| --- | --- | --- | --- | --- |
| `ID` | `id` | `id` | text | 业务 ID，建议使用 `doc_` 前缀。 |
| `KnowledgeBaseID` | `knowledgeBaseId` | `knowledge_base_id` | text | 所属知识库。 |
| `FileRef` | internal only | `file_ref` | text | File Service 内部文件引用，不作为前端权限依据，不返回给公开 API。 |
| `Name` | `name` | `name` | text | 文件名或用户定义文档名，当前最大 255 字符。 |
| `ContentType` | `contentType` | `content_type` | text nullable | MIME type。 |
| `SizeBytes` | `sizeBytes` | `size_bytes` | bigint nullable | 原文件大小。 |
| `Status` | `status` | `status` | text | 文档处理状态，见第 7.1 节。 |
| `ErrorCode` | `errorCode` | `error_code` | text nullable | 处理失败分类。 |
| `ErrorMessage` | `errorMessage` | `error_message` | text nullable | 可展示或可排查的错误摘要。 |
| `ParsedContent` | hidden | `parsed_content` | text nullable | 解析后的全文，主要用于调试、重切片和 fallback。 |
| `ChunkCount` | `chunkCount` | derived | integer | 从 `document_chunks` 聚合得出。 |
| `Tags` | `tags` | `tags` | jsonb | 文档标签，当前最多 32 个，每个最多 64 字符。 |
| `ParserBackend` | `parserBackend` | `parser_backend` | text nullable | 解析器标识，例如 `text`。 |
| `CreatedBy` | `createdBy` | `created_by` | text | 上传或 handoff 发起人。 |
| `CurrentJobID` | `jobId` | `current_job_id` | text nullable | 当前处理任务引用。 |
| `CreatedAt` | `createdAt` | `created_at` | timestamptz | 创建时间。 |
| `UpdatedAt` | `updatedAt` | `updated_at` | timestamptz | 状态或元数据更新时间。 |
| `DeletedAt` | hidden | `deleted_at` | timestamptz nullable | 软删除时间。 |

关键索引：

- `idx_knowledge_documents_knowledge_base_id` 支持按知识库列文档。
- `idx_knowledge_documents_status` 支持处理队列、管理台和状态过滤。
- `idx_knowledge_documents_file_ref` 仅用于内部清理和排查，不作为业务查询主路径。
- `idx_knowledge_documents_created_at` 支持上传记录倒序分页。

### 6.3 ProcessingJob

处理任务表示文档入库、知识库重处理或删除清理等异步工作。任务是状态机，不把动作词暴露为稳定 path。

| 逻辑字段 | HTTP 字段 | PostgreSQL 字段 | 类型 | 说明 |
| --- | --- | --- | --- | --- |
| `ID` | `id` | `id` | text | 业务 ID，建议使用 `job_` 前缀。 |
| `KnowledgeBaseID` | `knowledgeBaseId` | `knowledge_base_id` | text | 任务所属知识库。 |
| `DocumentID` | `documentId` | `document_id` | text nullable | 文档级任务引用；知识库级任务可为空。 |
| `JobType` | `jobType` | `job_type` | text | 任务类型，见第 7.2 节。 |
| `Status` | `status` | `status` | text | 任务状态，见第 7.3 节。 |
| `CurrentStage` | `currentStage` | `current_stage` | text nullable | 当前处理阶段，见第 7.4 节。 |
| `ProgressPercent` | `progressPercent` | `progress_percent` | integer | 0-100 的粗粒度进度。 |
| `Message` | `message` | `message` | text nullable | 当前状态说明。 |
| `ErrorCode` | `errorCode` | `error_code` | text nullable | 失败分类。 |
| `ErrorMessage` | `errorMessage` | `error_message` | text nullable | 失败摘要。 |
| `Attempts` | `attempts` | `attempts` | integer | 已执行次数。 |
| `MaxAttempts` | `maxAttempts` | `max_attempts` | integer | 最大执行次数，默认 3。 |
| `IdempotencyKey` | hidden | `idempotency_key` | text nullable | handoff 幂等键。 |
| `StartedAt` | `startedAt` | `started_at` | timestamptz nullable | 开始执行时间。 |
| `FinishedAt` | `finishedAt` | `finished_at` | timestamptz nullable | 结束时间。 |
| `CreatedAt` | `createdAt` | `created_at` | timestamptz | 创建时间。 |
| `UpdatedAt` | `updatedAt` | `updated_at` | timestamptz | 更新时间。 |

关键索引：

- `idx_processing_jobs_status_created_at` 支持 worker 获取队列任务和管理台查询。
- `idx_processing_jobs_document_id` 支持文档处理历史查询。
- `idx_processing_jobs_knowledge_base_id` 支持知识库级任务查询。
- `uniq_processing_jobs_idempotency_key` 防止重复 handoff 创建重复任务。

### 6.4 DocumentChunk

文档切片是检索命中的最小可引用单元。PostgreSQL 保存正文和元数据，Qdrant 保存向量和最小检索 payload。

| 逻辑字段 | HTTP 字段 | PostgreSQL 字段 | 类型 | 说明 |
| --- | --- | --- | --- | --- |
| `ID` | `id` | `id` | text | 业务 ID，建议使用 `chunk_` 前缀。 |
| `KnowledgeBaseID` | `knowledgeBaseId` | `knowledge_base_id` | text | 所属知识库。 |
| `DocumentID` | `documentId` | `document_id` | text | 所属文档。 |
| `ChunkIndex` | `chunkIndex` | `chunk_index` | integer | 文档内从 0 开始的切片序号。 |
| `SectionPath` | `sectionPath` | `section_path` | text nullable | 章节路径或解析器提供的位置。 |
| `Content` | `content` | `content` | text | 切片正文。 |
| `TokenCount` | `tokenCount` | `token_count` | integer | 估算 token 数或切片器计数。 |
| `ChunkType` | `chunkType` | `chunk_type` | text nullable | 例如 `text`、`table`。 |
| `QdrantPointID` | `qdrantPointId` | `qdrant_point_id` | text nullable | 对应 Qdrant point ID。 |
| `EmbeddingProvider` | `embeddingProvider` | `embedding_provider` | text nullable | embedding provider 标识。 |
| `EmbeddingModel` | internal only | `embedding_model` | text nullable | embedding 模型标识。 |
| `EmbeddingDimension` | `embeddingDimension` | `embedding_dimension` | integer nullable | 向量维度。 |
| `Metadata` | `metadata` | `metadata` | jsonb | 切片器产生的结构化元数据。 |
| `CreatedAt` | `createdAt` | `created_at` | timestamptz | 创建时间。 |

约束与索引：

- `UNIQUE (document_id, chunk_index)` 保证同一文档内切片顺序唯一。
- `idx_document_chunks_document_id_chunk_index` 支持按文档顺序列出 chunks。
- `idx_document_chunks_knowledge_base_id` 支持知识库级聚合。
- `idx_document_chunks_qdrant_point_id` 支持向量点反查。

### 6.5 KnowledgeQuery

检索查询当前不持久化，但作为资源建模以保持 RESTful 契约稳定。

请求字段：

| 逻辑字段 | HTTP 字段 | 类型 | 说明 |
| --- | --- | --- | --- |
| `Query` | `query` | string | 查询文本，当前最大 2000 字符。 |
| `KnowledgeBaseIDs` | `knowledgeBaseIds` | string[] | 限定知识库；为空时按用户权限检索可访问知识库。 |
| `TopK` | `topK` | integer | 返回候选数，默认 runtime config，范围 1-100。 |
| `ScoreThreshold` | `scoreThreshold` | number | 相似度阈值，默认 runtime config。 |
| `Tags` | `tags` | string[] | 标签过滤。 |
| `MetadataFilter` | `metadataFilter` | object | 切片 metadata 等值过滤。 |
| `Rerank` | `rerank` | boolean | 是否请求 rerank。当前 trace 记录该请求。 |
| `RerankTopN` | `rerankTopN` | integer | rerank 后截断数，必须小于等于 `topK`。 |

响应字段：

| 逻辑字段 | HTTP 字段 | 类型 | 说明 |
| --- | --- | --- | --- |
| `ID` | `id` | string | 查询资源 ID，建议使用 `kq_` 前缀。 |
| `Results` | `results` | array | 检索命中列表。 |
| `Trace` | `trace` | object | embedding、collection、topK、阈值和命中数。 |

检索结果从 Qdrant 命中 payload 取 `chunk_id`，再回 PostgreSQL hydrate `document_chunks` 和 `knowledge_documents`。只有文档状态为 `ready` 且调用方有权限访问的结果才会返回。

### 6.6 RuntimeConfig

运行配置当前保存在进程内存中，用于管理台和本地开发。后续如果需要跨实例生效，应新增 `runtime_configs` 或配置中心集成，不能把配置散落到业务表。

| 字段 | HTTP 字段 | 类型 | 说明 |
| --- | --- | --- | --- |
| `EmbeddingProvider` | `embeddingProvider` | string | embedding provider。 |
| `EmbeddingModel` | `embeddingModel` | string | embedding 模型。 |
| `EmbeddingDimension` | `embeddingDimension` | integer | 向量维度。 |
| `QdrantCollection` | `qdrantCollection` | string | Qdrant collection。 |
| `ParserBackend` | `parserBackend` | string | 默认解析器。 |
| `RerankProvider` | `rerankProvider` | string | rerank provider。 |
| `RerankModel` | `rerankModel` | string | rerank 模型。 |
| `RetrievalTopK` | `retrievalTopK` | integer | 默认 topK。 |
| `ScoreThreshold` | `scoreThreshold` | number | 默认分数阈值。 |
| `MaxConcurrentJobs` | `maxConcurrentJobs` | integer | 最大并发任务数。 |
| `ProcessingTimeoutSec` | `processingTimeoutSec` | integer | 处理超时秒数。 |
| `SecretRefs` | `secretRefs` | object | 外部密钥引用名，不保存密钥明文。 |

### 6.7 RetrievalContractFixture

检索实现和契约测试依赖的是持久化数据契约，而不是 A-11 worker
进程是否已经真实跑通。A-12 和 A-14 可以在测试中 seed 以下最小数据：

| Fixture | 必填字段 | 说明 |
| --- | --- | --- |
| `knowledge_bases` | `id`、`created_by`、`deleted_at IS NULL` | 作为权限和范围过滤根。 |
| `knowledge_documents` | `id`、`knowledge_base_id`、`name`、`status='ready'`、`tags`、`created_by`、`deleted_at IS NULL` | 只有 `ready` 且未删除文档可进入检索结果。 |
| `document_chunks` | `id`、`knowledge_base_id`、`document_id`、`chunk_index`、`content`、`token_count`、`metadata`、`qdrant_point_id` | 作为检索结果 hydrate 的正文和展示事实来源。 |
| vector hit | `pointId`、`score`、payload.`knowledge_base_id`、payload.`document_id`、payload.`chunk_id` | 可由 fake Qdrant adapter 直接返回；payload 只用于定位和过滤。 |

fixture 规则：

- `knowledge_documents.status != 'ready'`、`deleted_at IS NOT NULL`、缺少访问权限、低于阈值或 tag/metadata 不匹配的命中必须被过滤。
- Qdrant hit 找不到对应 `document_chunks` 或 hydrate 后文档不可见时，跳过该 hit，不把内部不一致暴露给前端。
- 无有效命中时返回 `results: []` 和 `trace.hitCount: 0`，不得返回 500。
- fake embedding/rerank adapter 可以返回固定向量、固定 score 或固定重排序顺序，但公开响应和 trace 字段必须符合 gateway OpenAPI。
- 真实 A-11 worker 只负责生产同样形态的数据；不能要求 A-12/A-14 在单元或契约测试中启动 Parser service、真实 embedding provider 或真实 Qdrant。

## 7. 枚举与状态机

### 7.1 DocumentStatus

| 状态 | 说明 |
| --- | --- |
| `uploaded` | 文档资源已创建，原文件引用已保存，等待处理。 |
| `parsing` | 正在读取原文件并解析。 |
| `chunking` | 正在生成切片。 |
| `embedding` | 正在生成 embedding 并准备向量索引。 |
| `ready` | 切片和索引可用于检索。 |
| `failed` | 处理失败，查看 `errorCode` 和 `errorMessage`。 |

状态流转：

```text
uploaded -> parsing -> chunking -> embedding -> ready
                      \-> failed
parsing/chunking/embedding -> failed
failed -> parsing    # retry/reprocess 时
```

### 7.2 JobType

| 类型 | 说明 |
| --- | --- |
| `ingest` | 新文档入库处理。 |
| `reprocess` | 知识库或文档重处理。 |
| `delete_cleanup` | 删除后的切片、向量和外部引用清理。 |

### 7.3 JobStatus

| 状态 | 说明 |
| --- | --- |
| `queued` | 已创建，等待 worker 执行。 |
| `running` | 正在执行。 |
| `succeeded` | 执行成功。 |
| `failed` | 执行失败，可按策略重试。 |
| `cancelled` | 已取消。 |

### 7.4 JobStage

当前实现会使用以下阶段值：

```text
handoff
parsing
chunking
embedding
indexing
reprocessing
```

`currentStage` 是诊断字段，不应作为前端强状态机判断依据；前端稳定判断应使用 `status` 和 `progressPercent`。

## 8. Qdrant 模型

默认 collection 名称：

```text
knowledge_chunks
```

Point ID 使用由 `chunk_id` 稳定派生的 UUID 字符串。向量维度必须与 runtime config 的 `embeddingDimension` 一致。

Payload 示例：

```json
{
  "knowledge_base_id": "kb_123",
  "document_id": "doc_123",
  "chunk_id": "chunk_123",
  "chunk_index": 0,
  "chunk_type": "text",
  "section_path": "root/introduction",
  "tags": ["linux", "deployment"],
  "metadata": {
    "heading": "Introduction"
  }
}
```

Payload 规则：

- 必须包含 `knowledge_base_id`、`document_id`、`chunk_id`，用于权限过滤和 PostgreSQL hydrate。
- 可包含 `tags` 和 `metadata` 支持过滤。
- 不保存完整文档状态、任务状态或错误详情。
- 不依赖 Qdrant payload 作为最终展示内容来源；展示正文从 PostgreSQL 的 `document_chunks.content` 读取。
- 契约测试可以用等价的 fake vector hit 替代真实 Qdrant hit；只要 payload 字段和 score 语义一致，A-12/A-14 不需要等待 A-11 worker 完成真实索引写入。

## 9. PostgreSQL 当前表

当前 goose migration 位于 `services/knowledge/migrations/0001_create_knowledge_core_tables.sql`，包含：

```text
knowledge_bases
knowledge_documents
processing_jobs
document_chunks
```

实现注意事项：

- SQL query 文件应放在服务本地 repository query 目录，并由 `sqlc` 生成类型安全查询代码。
- `sqlc` 生成代码只能被 repository 适配层使用，不直接泄露到 HTTP handler 或 gateway DTO。
- repository 层负责把 PostgreSQL `jsonb` 转换为 Go `map[string]any`、`[]string` 或具体策略结构。
- `created_at`、`updated_at` 统一使用 UTC。
- 列表查询默认排除 `deleted_at IS NOT NULL` 的知识库和文档。
- 删除知识库时应先阻止或清理关联文档、切片和 Qdrant points；当前文档定义目标模型，具体事务顺序由 repository/service 实现保证。

## 10. 公开字段与内部字段

不得向前端暴露为权限依据的字段：

- `file_ref`
- `fileId`
- MinIO bucket、object key、presigned URL
- `parsedContent`
- Parser 内部文件路径、OCR debug output、PaddleOCR 原始 provider body
- `idempotencyKey`
- provider secret 或 secret 明文

可返回用于诊断或展示的字段：

- 文档 `status`、`errorCode`、`errorMessage`
- chunk `qdrantPointId`、`embeddingProvider`、`embeddingDimension`
- query `trace`

如果 gateway 公开契约没有这些字段，服务本地可先保留内部字段，但 browser-facing API 必须等 gateway OpenAPI 接收后再暴露。

当前 Go baseline 和早期 migration 中如果仍存在 `file_id` 命名，应视为兼容期实现细节；目标数据模型统一使用 `file_ref` 表达不透明的内部 file reference。
