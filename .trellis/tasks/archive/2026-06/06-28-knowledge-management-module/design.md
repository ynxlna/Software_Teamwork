# Knowledge Go 微服务模块设计

## 1. Current State

`services/knowledge` 已迁回 Go 微服务形态，旧 Python/FastAPI 原型已移除。当前实现已超过最初 health/ready 骨架，已形成可继续扩展的 service-local module：

- `GET /healthz`
- `GET /readyz`
- 知识库 CRUD internal API。
- 文档处理状态和 chunk 只读 internal API。
- File -> Knowledge handoff 和 ingestion job internal API。
- 文本/Markdown parser、chunker 和同步 processing run。
- Embedding/vector 抽象、deterministic local embedding、memory vector index 的接入雏形。

当前 Knowledge service-local 能力已覆盖 metadata、handoff、同步处理、embedding/vector、Qdrant adapter、retrieval 和 P1 内部管理端点。仍未完成的主线是：gateway proxy/contract tests、真实异步 worker/queue、生产 embedding/rerank provider、File Service 正式 handoff 联调，以及更完整的 PostgreSQL migration/integration 验证。后续业务能力必须继续按 Go 服务的 vertical slice 迭代，不再把 Python 原型作为正式 runtime 或契约来源。

## 2. Design Goals

- 把 Knowledge 做成独立 Go 微服务，拥有知识库元数据、文档处理状态、chunks、embedding、Qdrant 索引和 retrieval。
- 保持前端公开契约只由 gateway 暴露，frontend 不能直接调用 `services/knowledge`。
- 用 PostgreSQL 保存 durable metadata 和处理状态，用 Qdrant 保存向量和检索 payload，用 Redis 作为短期队列或缓存而非业务事实来源。
- 把原文件上传和原始对象生命周期留给 File Service；Knowledge 只通过 handoff 接收文件引用和处理上下文。
- 为 QA 和 Document 提供稳定 retrieval HTTP 能力，避免它们直接读写 Qdrant。
- 先交付可验证 P0，再扩展重处理、运行时配置、统计监控和检索测试。

## 3. Non-Goals

- 不实现登录、session、RBAC 源数据；Knowledge 只消费 gateway 注入的用户上下文。
- 不接管 File Service 的 multipart upload、原始文件下载、MinIO object key 和 file-owned tags 更新。
- 不实现 QA 会话、LLM 回答生成、SSE 问答事件。
- 不实现报告模板、DOCX 导出或报告生成工作流。
- 不恢复旧 Python 原型里的调试字段、动作式路径或本地脚本作为稳定能力。

## 4. Ownership Boundaries

| Capability | Owner | Notes |
| --- | --- | --- |
| Public `/api/v1/**` routing and frontend envelope | `gateway` | Gateway 负责认证、上下文注入、错误归一化和公开 OpenAPI。 |
| Knowledge bases | `knowledge` | 创建、列表、详情、更新、删除，以及 chunk/retrieval strategy。 |
| Original file upload and object lifecycle | `file` | `POST /api/v1/knowledge-bases/{knowledgeBaseId}/documents` 是 file-owned。 |
| Document processing state | `knowledge` | `uploaded -> parsing -> chunking -> embedding -> ready | failed`。 |
| Chunks and embeddings | `knowledge` | PostgreSQL 保存 chunk metadata/text，Qdrant 保存 vector/payload。 |
| Retrieval query | `knowledge` | `knowledge-queries` 是资源化检索请求，不使用 `/search`。 |
| QA answer generation | `qa` | 通过 Knowledge retrieval HTTP 获取上下文。 |
| Report generation | `document` | 通过 Knowledge retrieval HTTP 获取引用材料。 |

## 5. Public Gateway Contract Mapping

Knowledge 服务实现必须对齐 `docs/api/gateway.openapi.yaml` 的 active operations。服务本地可以使用 `/internal/v1/**`，但公开路径仍由 gateway 暴露。

| Gateway Method | Gateway Path | Downstream owner | Knowledge responsibility |
| --- | --- | --- | --- |
| `GET` | `/api/v1/knowledge-bases` | `knowledge` | 查询知识库分页列表。 |
| `POST` | `/api/v1/knowledge-bases` | `knowledge` | 创建知识库。 |
| `GET` | `/api/v1/knowledge-bases/{knowledgeBaseId}` | `knowledge` | 查询知识库详情。 |
| `PATCH` | `/api/v1/knowledge-bases/{knowledgeBaseId}` | `knowledge` | 更新元数据、分段策略、检索策略。 |
| `DELETE` | `/api/v1/knowledge-bases/{knowledgeBaseId}` | `knowledge` | 删除业务状态、chunks 和 Qdrant points。 |
| `GET` | `/api/v1/knowledge-bases/{knowledgeBaseId}/documents` | `knowledge` | 查询文档处理状态列表。 |
| `GET` | `/api/v1/documents/{documentId}` | `knowledge` | 查询文档处理详情。 |
| `GET` | `/api/v1/documents/{documentId}/chunks` | `knowledge` | 查询文档切片详情。 |
| `POST` | `/api/v1/knowledge-queries` | `knowledge` | 创建一次检索请求并返回召回结果。 |

File-owned 相关公开路径：

| Gateway Method | Gateway Path | Owner | Knowledge relation |
| --- | --- | --- | --- |
| `POST` | `/api/v1/knowledge-bases/{knowledgeBaseId}/documents` | `file` | 上传成功后通过内部 handoff 创建 Knowledge document/job。 |
| `PATCH` | `/api/v1/documents/{documentId}` | `file` | 更新 file-owned tags；Knowledge 可通过协调机制同步 retrieval filter 字段。 |
| `DELETE` | `/api/v1/documents/{documentId}` | `file` | File 删除原文件后通知 Knowledge 清理 chunks/vector。 |
| `GET` | `/api/v1/documents/{documentId}/content` | `file` | Knowledge 处理时通过内部授权接口读取内容，不暴露 object key。 |

## 6. Service-Local API Design

服务内部 HTTP 资源统一放在 `/internal/v1/**`。这些接口给 gateway、file、qa、document 或内部 worker 使用，不直接给 browser。

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/internal/v1/knowledge-bases` | Gateway 列表代理。 |
| `POST` | `/internal/v1/knowledge-bases` | Gateway 创建代理。 |
| `GET` | `/internal/v1/knowledge-bases/{knowledgeBaseId}` | Gateway 详情代理。 |
| `PATCH` | `/internal/v1/knowledge-bases/{knowledgeBaseId}` | Gateway 更新代理。 |
| `DELETE` | `/internal/v1/knowledge-bases/{knowledgeBaseId}` | Gateway 删除代理。 |
| `GET` | `/internal/v1/knowledge-bases/{knowledgeBaseId}/documents` | Gateway 文档列表代理。 |
| `POST` | `/internal/v1/knowledge-bases/{knowledgeBaseId}/ingestion-jobs` | File -> Knowledge handoff，创建 document 和 ingestion job。 |
| `GET` | `/internal/v1/documents/{documentId}` | Gateway 文档详情代理。 |
| `GET` | `/internal/v1/documents/{documentId}/chunks` | Gateway chunks 代理。 |
| `GET` | `/internal/v1/jobs/{jobId}` | 查询 ingestion/reprocess/delete cleanup job。 |
| `POST` | `/internal/v1/jobs/{jobId}/processing-runs` | 触发当前 MVP 的同步 ingestion processing run；后续可替换为 worker/queue。 |
| `POST` | `/internal/v1/knowledge-queries` | Gateway、QA、Document 复用的检索入口。 |

内部服务仍使用统一成功和错误 envelope。Gateway 对外再按公开 OpenAPI 做最终归一化。

P1 扩展 API 进入公开契约前，先以内部资源形式设计和验证：

| Method | Path | Purpose |
| --- | --- | --- |
| `POST` | `/internal/v1/jobs` | 创建 `reprocess` 或 `delete_cleanup` job；body 声明资源 ID 和 job type。 |
| `GET` | `/internal/v1/runtime-config` | 查看脱敏后的 embedding、rerank、parser 和 retrieval runtime config。 |
| `PATCH` | `/internal/v1/runtime-config` | 更新非密钥运行参数；密钥只允许保存引用名。 |
| `GET` | `/internal/v1/knowledge-stats` | 查询知识库、文档、chunk、ready/failed 和上传趋势统计。 |

这些 P1 path 不是当前 gateway 公开 API。若需要暴露给前端，必须先进入 `docs/api/gateway.openapi.yaml` 并完成 gateway contract tests。

## 7. Domain Model

### 7.1 KnowledgeBase

主要字段：

- `id`
- `name`
- `description`
- `docType`
- `chunkStrategy`
- `retrievalStrategy`
- `createdBy`
- `createdAt`
- `updatedAt`
- `deletedAt`

`documentCount` 和 `chunkCount` 先从 repository 聚合返回，后续量级上来后再评估是否引入 denormalized counters。

### 7.2 KnowledgeDocument

主要字段：

- `id`
- `knowledgeBaseId`
- `fileId`
- `name`
- `contentType`
- `sizeBytes`
- `status`
- `errorCode`
- `errorMessage`
- `chunkCount`
- `tags`
- `parserBackend`
- `createdBy`
- `createdAt`
- `updatedAt`
- `deletedAt`
- `currentJobId`

公开 `status` 只允许：

```text
uploaded
parsing
chunking
embedding
ready
failed
```

内部阶段可以更细，但不能直接扩展公开 enum。

### 7.3 ProcessingJob

主要字段：

- `id`
- `knowledgeBaseId`
- `documentId`
- `jobType`: `ingest | reprocess | delete_cleanup`
- `status`: `queued | running | succeeded | failed | cancelled`
- `currentStage`: `handoff | parsing | chunking | embedding | indexing | finalizing`
- `progressPercent`
- `message`
- `errorCode`
- `errorMessage`
- `attempts`
- `maxAttempts`
- `startedAt`
- `finishedAt`
- `createdAt`
- `updatedAt`

P0 可以只提供 job 查询；job events、cancel 和 bulk job 放到 P1。

### 7.4 DocumentChunk

主要字段：

- `id`
- `knowledgeBaseId`
- `documentId`
- `chunkIndex`
- `sectionPath`
- `content`
- `tokenCount`
- `chunkType`
- `qdrantPointId`
- `embeddingProvider`
- `embeddingModel`
- `embeddingDimension`
- `metadata`
- `createdAt`

公开接口当前允许返回 `content`，但 retrieval result 应返回 `contentPreview`，避免检索列表泄露过多全文。

## 8. PostgreSQL Schema Plan

迁移放在 `services/knowledge/migrations/`，先使用显式 SQL，不引入 ORM。

建议首批表：

```text
knowledge_bases
knowledge_documents
processing_jobs
document_chunks
```

### 8.1 `knowledge_bases`

- `id text primary key`
- `name text not null`
- `description text not null default ''`
- `doc_type text not null`
- `chunk_strategy jsonb not null`
- `retrieval_strategy jsonb not null`
- `created_by text`
- `created_at timestamptz not null`
- `updated_at timestamptz not null`
- `deleted_at timestamptz`

Indexes:

- `idx_knowledge_bases_created_at`
- `idx_knowledge_bases_doc_type`
- `idx_knowledge_bases_deleted_at`

### 8.2 `knowledge_documents`

- `id text primary key`
- `knowledge_base_id text not null references knowledge_bases(id)`
- `file_id text not null`
- `name text not null`
- `content_type text`
- `size_bytes bigint`
- `status text not null`
- `error_code text`
- `error_message text`
- `tags jsonb not null default '[]'`
- `parser_backend text`
- `created_by text`
- `current_job_id text`
- `created_at timestamptz not null`
- `updated_at timestamptz not null`
- `deleted_at timestamptz`

Indexes:

- `idx_knowledge_documents_knowledge_base_id`
- `idx_knowledge_documents_status`
- `idx_knowledge_documents_file_id`
- `idx_knowledge_documents_created_at`

### 8.3 `processing_jobs`

- `id text primary key`
- `knowledge_base_id text not null`
- `document_id text`
- `job_type text not null`
- `status text not null`
- `current_stage text`
- `progress_percent integer not null default 0`
- `message text`
- `error_code text`
- `error_message text`
- `attempts integer not null default 0`
- `max_attempts integer not null default 3`
- `started_at timestamptz`
- `finished_at timestamptz`
- `created_at timestamptz not null`
- `updated_at timestamptz not null`

Indexes:

- `idx_processing_jobs_status_created_at`
- `idx_processing_jobs_document_id`
- `idx_processing_jobs_knowledge_base_id`

### 8.4 `document_chunks`

- `id text primary key`
- `knowledge_base_id text not null`
- `document_id text not null references knowledge_documents(id)`
- `chunk_index integer not null`
- `section_path text`
- `content text not null`
- `token_count integer not null`
- `chunk_type text`
- `qdrant_point_id text`
- `embedding_provider text`
- `embedding_model text`
- `embedding_dimension integer`
- `metadata jsonb not null default '{}'`
- `created_at timestamptz not null`

Indexes:

- `idx_document_chunks_document_id_chunk_index`
- `idx_document_chunks_knowledge_base_id`
- `idx_document_chunks_qdrant_point_id`
- unique `(document_id, chunk_index)`

## 9. Storage And Infrastructure Rules

- PostgreSQL 是 Knowledge metadata、document status、job status、chunk text 和 chunk metadata 的事实来源。
- Qdrant 只保存向量和最小 retrieval payload：`knowledge_base_id`、`document_id`、`chunk_id`、`chunk_index`、`section_path`、`tags`、`chunk_type`。
- Redis 只用于短期 job queue、rate limit 或 cache；不能作为唯一业务状态。
- MinIO 原始对象由 File Service 持有。Knowledge 不保存或暴露 raw object key。
- Embedding provider、parser backend、Qdrant client 统一通过 `internal/platform/` adapter 隔离。

## 10. Package Responsibilities

```text
internal/http
  handlers, route registration, JSON encoding, request validation, context header extraction

internal/service
  domain types, use cases, state machine, app errors, idempotency decisions

internal/repository
  repository interfaces plus postgres implementation; tests may use memory fake

internal/platform/fileclient
  internal File Service client for content handoff/read

internal/platform/parser
  parser adapter interface and initial text/md implementation

internal/platform/embedding
  embedding provider interface and deterministic local adapter for tests

internal/platform/qdrant
  vector index interface and Qdrant implementation

internal/platform/queue
  Redis-backed or in-process queue adapter, introduced only when pipeline needs it
```

Handlers must not call PostgreSQL, Qdrant, Redis, MinIO, parser, or embedding clients directly.

## 11. File -> Knowledge Handoff

推荐 P0 内部 handoff resource：

```http
POST /internal/v1/knowledge-bases/{knowledgeBaseId}/ingestion-jobs
```

Request body:

```json
{
  "fileId": "file_123",
  "name": "manual.pdf",
  "contentType": "application/pdf",
  "sizeBytes": 123456,
  "tags": ["规程", "汽机"],
  "createdBy": "user_123",
  "idempotencyKey": "optional-upload-request-id"
}
```

Response body:

```json
{
  "data": {
    "documentId": "doc_123",
    "jobId": "job_123",
    "status": "uploaded"
  },
  "requestId": "req_123"
}
```

Rules:

- Handoff 在一个 PostgreSQL transaction 中创建 `knowledge_documents` 和 `processing_jobs`。
- `idempotencyKey` 存在时，同一 key 的重复请求应返回同一 document/job 或明确 `conflict`，不能创建重复入库任务。
- Knowledge 读取文件内容时调用 File Service 内部授权接口；不得接收或暴露 MinIO object key。
- File 删除文档后应通过后续 `delete_cleanup` job 或内部协调资源触发 Knowledge 清理 chunks 和 Qdrant points。

## 12. Ingestion Pipeline

状态流：

```text
uploaded -> parsing -> chunking -> embedding -> ready
                                      |
                                      v
                                    failed
```

执行流程：

1. Handoff 创建 document 和 queued job。
2. Worker 领取 job，标记 job `running`，document `parsing`。
3. 通过 File Service 读取源文件内容或流。
4. Parser 产出文本和基础结构信息。
5. Chunker 按 knowledge base 的 `chunkStrategy` 产出 chunks。
6. Embedding adapter 对 chunks 批量向量化，document `embedding`。
7. Qdrant adapter upsert points，point ID 应可由 `chunkId` 稳定推导或映射。
8. PostgreSQL transaction 写入 chunks、更新 document `ready`、job `succeeded`。

Consistency rules:

- 外部 HTTP 调用和 Qdrant upsert 不放在 PostgreSQL transaction 内。
- Reprocess 前先按 document ID 清理旧 chunks/points，或用版本字段确保新旧 point 不混用。
- Retry 必须幂等；同一 document 重试不能产生重复 active chunks。
- 失败时 document 进入 `failed`，job 进入 `failed`，`errorMessage` 必须脱敏。

## 13. Retrieval Flow

`POST /internal/v1/knowledge-queries` 执行流程：

1. 校验 `query`、`topK`、`scoreThreshold`、`knowledgeBaseIds`、`tags`、`metadataFilter`。
2. 结合 gateway 传入的 user context 解析可访问 knowledge base 范围。
3. 使用 embedding adapter 生成 query vector。
4. Qdrant search，payload filter 包含 knowledge base、tags、metadata 条件。
5. 用 PostgreSQL hydrate document/chunk，过滤 deleted、failed、隐藏或无权限资源。
6. 如 `rerank=true` 且 provider 已配置，执行 rerank；否则 trace 中标明未执行或 fallback。
7. 返回 `KnowledgeQuerySummary`，包含 `id`、`query`、`results` 和 `trace`。

Response must not include:

- raw embedding vector
- full Qdrant payload
- SQL details
- object key / MinIO path
- prompts
- API key / token
- internal service URLs

## 14. Authorization Model For MVP

MVP 不引入完整组织权限模型，先遵循以下规则：

- Gateway 必须注入 `X-User-Id`，否则 Knowledge 返回 `unauthorized`。
- 创建知识库和文档时记录 `createdBy`。
- 普通用户只能访问自己创建的知识库和文档。
- 如果 gateway 传入 `X-User-Permissions` 包含后续约定的管理权限，例如 `knowledge:read:any`、`knowledge:write:any`，Knowledge 可以放宽 owner 限制。
- 所有权限拒绝统一返回 `forbidden` 或对隐藏资源返回 `not_found`，不泄露资源是否存在。

该规则后续可被 Auth/Gateway 提供的组织、角色、资源权限模型替换。

## 15. Error And Logging Rules

Stable error codes:

```text
validation_error
unauthorized
forbidden
not_found
conflict
rate_limited
dependency_error
internal_error
```

Logging:

- 每个请求记录 `requestId`、method、path、status、duration。
- ingestion/retrieval 记录 documentId、jobId、knowledgeBaseId、provider、duration、result count。
- 不记录 raw file content、full chunk content、raw vectors、prompt、token、API key、object key。
- 依赖失败在 HTTP boundary 记录一次，service/repository 层只包装错误上下文。

## 16. Phased Implementation Plan

### Phase 0: Baseline

已完成：Go module、config、HTTP server、health/ready、Dockerfile、Compose、README、service-local OpenAPI、旧 Python 原型移除。

### Phase 1: Metadata CRUD

实现 domain model、migration、repository port、PostgreSQL implementation、knowledge base CRUD、document list/detail skeleton、chunk list skeleton。先让 gateway active schemas 的字段都能由服务返回。

### Phase 2: Handoff And Job State

实现 File -> Knowledge handoff、document/job transaction、job 查询、状态机和 idempotency。此阶段可以先用同步或 in-process worker，保证业务状态可测。

### Phase 3: Parser And Chunker

实现 text/markdown 的最小 parser/chunker，建立可测试的 chunk persistence。PDF/DOCX/PPTX/XLSX/OCR 后续通过 parser adapter 扩展。

### Phase 4: Embedding And Qdrant

接入 deterministic test embedding、真实 embedding provider adapter、Qdrant upsert/delete/search。保证重试和 reprocess 不产生重复 active vectors。当前已完成 embedding/vector interface、本地确定性 embedding 和 memory vector index；下一步需要补齐 Qdrant HTTP adapter、config gate 和 dependency failure tests。

### Phase 5: Retrieval

实现 `knowledge-queries`，支持 knowledgeBaseIds、topK、scoreThreshold、tags、metadataFilter、trace 和安全字段过滤。

### Phase 6: Gateway Integration

实现 gateway -> Knowledge client/proxy、上下文 headers、错误映射和 contract tests。前端仍只调用 gateway。

### Phase 7: P1 Extensions

实现 reprocessing job、runtime config、admin stats、retrieval testing、job events 和 batch cleanup 资源。

## 17. Validation Strategy

每个实现 slice 至少需要：

- `cd services/knowledge && go test ./...`
- `cd services/knowledge && go build ./cmd/server`
- 涉及 migration 时解析 SQL 并用本地 PostgreSQL 或容器跑迁移 smoke test。
- 涉及 Qdrant 时跑 Qdrant adapter integration test，或显式 gate 外部依赖测试。
- 涉及 gateway 时跑 gateway handler/client tests，并校验 `docs/api/gateway.openapi.yaml` schema。
- 文档改动至少跑 `git diff --check`。

## 18. Risks And Mitigations

| Risk | Impact | Mitigation |
| --- | --- | --- |
| Python 原型移除后 ingestion 暂时缺口 | 本地上传解析能力短期不可用 | 按 handoff -> parser -> chunker -> embedding vertical slice 重建。 |
| Gateway/File/Knowledge handoff 未定 | 上传后状态可能断裂 | 先实现 `/internal/v1/.../ingestion-jobs` 资源，后续可替换为消息队列。 |
| Qdrant 和 PostgreSQL 一致性 | 检索命中 stale vectors | point ID 稳定、按 document 清理、retry 幂等、hydrate 后过滤 deleted/failed。 |
| 权限模型不足 | 多用户数据隔离风险 | MVP 强制 `X-User-Id` + createdBy owner 过滤；后续接入 Auth 权限。 |
| 公开 enum 过早扩展 | 前端和 OpenAPI 不兼容 | 内部 stage 与公开 `DocumentStatus` 分离。 |
