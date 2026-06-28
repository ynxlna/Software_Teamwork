# Knowledge Go 微服务实施计划

## Execution Principle

按小切片推进，每个切片都要能独立运行、测试和回滚。优先完成 metadata、handoff、job、chunk、embedding、retrieval 这条主链路，再补 P1 的重处理、配置和统计能力。

## Task 0: Go Service Baseline

Status: done.

已完成：

- [x] 创建 `services/knowledge/go.mod`。
- [x] 添加 `cmd/server/main.go`、config、HTTP router、health/readiness handlers。
- [x] 添加 unified success/error envelope、request id handling 和基础测试。
- [x] 添加 README、Dockerfile、docker-compose、`api/openapi.yaml` baseline。
- [x] 移除旧 Python/FastAPI 原型、`requirements.txt` 和本地 ingest 脚本。
- [x] 更新实现说明，避免旧原型被误认为正式服务。

验证：

- [x] `go test ./...`
- [x] `go build ./cmd/server`
- [x] `docker compose config --quiet`
- [x] Docker image build
- [x] `git diff --check`

## Task 1: Module Design Freeze

Status: in progress.

目标：把 Knowledge 模块边界、数据模型、内部 API、状态机、存储规则和实施顺序固定到 Trellis task 文档。

交付：

- [x] 梳理需求和 gateway active public API。
- [x] 明确 File、Knowledge、QA、Document、Gateway 边界。
- [x] 设计 `/internal/v1/**` service-local API。
- [x] 设计 PostgreSQL tables、Qdrant payload、Redis/MinIO 使用边界。
- [x] 设计 ingestion job 状态机和 retrieval flow。
- [x] 拆分后续实施 task。

验证：

- [x] `git diff --check`

## Task 2: Metadata CRUD And PostgreSQL Foundation

Status: done.

目标：让 Knowledge Service 拥有可持久化的知识库元数据和基础文档状态查询能力。

范围：

- Domain types:
  - `KnowledgeBase`
  - `KnowledgeDocument`
  - `DocumentChunk`
  - `ProcessingJob`
- App errors:
  - `validation_error`
  - `unauthorized`
  - `forbidden`
  - `not_found`
  - `conflict`
  - `dependency_error`
  - `internal_error`
- Repository interfaces:
  - `KnowledgeBaseRepository`
  - `DocumentRepository`（Task 3）
  - `ChunkRepository`（Task 3）
  - `JobRepository`（Task 4）
- PostgreSQL migration:
  - `knowledge_bases`
  - `knowledge_documents`
  - `processing_jobs`
  - `document_chunks`
- API handlers:
  - `GET /internal/v1/knowledge-bases`
  - `POST /internal/v1/knowledge-bases`
  - `GET /internal/v1/knowledge-bases/{knowledgeBaseId}`
  - `PATCH /internal/v1/knowledge-bases/{knowledgeBaseId}`
  - `DELETE /internal/v1/knowledge-bases/{knowledgeBaseId}`

验收：

- [x] 知识库 CRUD 字段对齐 gateway `KnowledgeBaseSummary`。
- [x] 分页参数 `page`、`pageSize` 校验。
- [x] `docType`、`chunkStrategy`、`retrievalStrategy` 校验。
- [x] 普通用户只能访问 `createdBy` 等于 `X-User-Id` 的资源。
- [x] Not found、validation、forbidden、conflict 都有 handler tests。
- [x] Repository tests 覆盖 create/list/get/update/delete。

实现说明：

- `services/knowledge/internal/service/knowledge_base.go` 实现 KnowledgeBase domain、validator、owner filtering 和 CRUD use case。
- `services/knowledge/internal/repository/memory.go` 是默认本地 backend，供 tests 和 `KNOWLEDGE_STORAGE_BACKEND=memory` 使用。
- `services/knowledge/internal/repository/postgres.go` 提供 `database/sql` PostgreSQL repository，runtime driver 使用 Go 1.22 兼容的 `github.com/jackc/pgx/v4/stdlib`。
- `services/knowledge/migrations/0001_create_knowledge_core_tables.sql` 创建 `knowledge_bases`、`knowledge_documents`、`processing_jobs`、`document_chunks`。
- Task 2 只实现 `KnowledgeBaseRepository` 的 PostgreSQL 读写；Document/Chunk/Job repository 方法会在 Task 3/4 按对应 API 补齐。

推荐验证：

```bash
cd services/knowledge
go test ./...
go build ./cmd/server
```

验证：

- [x] `docker run --rm -e GOPROXY=https://goproxy.cn,direct -v "$PWD/services/knowledge":/src -w /src golang:1.22-alpine sh -c 'go test ./... && go build ./cmd/server'`

## Task 3: Document State And Chunk Read APIs

Status: done.

目标：补齐文档列表、文档详情和 chunk 只读能力，为 ingestion pipeline 做承接。

范围：

- `GET /internal/v1/knowledge-bases/{knowledgeBaseId}/documents`
- `GET /internal/v1/documents/{documentId}`
- `GET /internal/v1/documents/{documentId}/chunks`
- `DocumentStatus` enum:
  - `uploaded`
  - `parsing`
  - `chunking`
  - `embedding`
  - `ready`
  - `failed`
- Chunk pagination and ordering by `chunkIndex`。

验收：

- [x] 文档列表支持 `status` filter。
- [x] 文档详情返回 `jobId`、`errorCode`、`errorMessage`、`chunkCount`。
- [x] Chunk 列表返回 gateway `DocumentChunk` schema 字段。
- [x] Failed document 不泄露内部错误、object key 或 stack trace。
- [x] Deleted/hidden resource 对无权限用户返回 `not_found` 或 `forbidden`，不泄露存在性。

实现说明：

- `services/knowledge/internal/service/document.go` 定义 `DocumentStatus`、`KnowledgeDocument`、`DocumentChunk` 和只读 use cases。
- Memory/PostgreSQL repositories 均实现 `ListDocuments`、`FindDocumentByID`、`ListChunks`。
- HTTP 增加 `GET /internal/v1/knowledge-bases/{knowledgeBaseId}/documents`、`GET /internal/v1/documents/{documentId}`、`GET /internal/v1/documents/{documentId}/chunks`。
- 当前 Task 3 只实现 Knowledge-owned read side。File-owned `PATCH /documents/{documentId}`、`DELETE /documents/{documentId}` 仍由 File Service 拥有；Knowledge cleanup 会在 Task 4/后续 delete coordination 处理。

推荐验证：

```bash
cd services/knowledge
go test ./...
go build ./cmd/server
```

验证：

- [x] `docker run --rm -e GOPROXY=https://goproxy.cn,direct -v "$PWD/services/knowledge":/src -w /src golang:1.22-alpine sh -c 'gofmt -w ./cmd ./internal && go test ./... && go build ./cmd/server'`

## Task 4: File Handoff And ProcessingJob

Status: done.

目标：把 File Service 上传后的文件引用转换为 Knowledge document 和 ingestion job。

范围：

- `POST /internal/v1/knowledge-bases/{knowledgeBaseId}/ingestion-jobs`
- `GET /internal/v1/jobs/{jobId}`
- Handoff request:
  - `fileId`
  - `name`
  - `contentType`
  - `sizeBytes`
  - `tags`
  - `createdBy`
  - `idempotencyKey`
- Transaction:
  - create/update document
  - create processing job
  - set document `uploaded`
- Job state machine:
  - `queued`
  - `running`
  - `succeeded`
  - `failed`
  - `cancelled`

验收：

- [x] 同一 `idempotencyKey` 重复 handoff 不创建重复 active document/job。
- [x] 不存在的 knowledge base 返回 `not_found`。
- [x] 无权限 knowledge base 返回 `forbidden` 或隐藏为 `not_found`。
- [x] Job 查询返回 status、stage、progress、attempt、错误摘要。
- [x] Handoff 不接收、不保存、不返回 MinIO object key。

实现说明：

- `services/knowledge/internal/service/job.go` 定义 `ProcessingJob`、handoff input/result、job 状态和 idempotency 逻辑。
- HTTP 增加 `POST /internal/v1/knowledge-bases/{knowledgeBaseId}/ingestion-jobs` 和 `GET /internal/v1/jobs/{jobId}`。
- Migration 在 `processing_jobs` 上增加 `idempotency_key` 和唯一索引。
- Handoff 只保存 `fileId`、文档元数据和 job 状态，不接收 MinIO object key；实际读取 File Service 内容放在 Task 5 pipeline。

推荐验证：

```bash
cd services/knowledge
go test ./...
go build ./cmd/server
```

验证：

- [x] `docker run --rm -e GOPROXY=https://goproxy.cn,direct -v "$PWD/services/knowledge":/src -w /src golang:1.22-alpine sh -c 'gofmt -w ./cmd ./internal && go test ./... && go build ./cmd/server'`

如同步 File Service client：

```bash
cd services/file
go test ./...
```

## Task 5: Parser And Chunker Pipeline

Status: done.

目标：先支持最小可测的文本入库链路，建立 Go pipeline 的扩展点。

范围：

- `internal/platform/fileclient`：读取 File Service source content。
- `internal/platform/parser`：先支持 `text/plain`、`text/markdown`。
- Chunker：
  - 按 `chunkStrategy` 切分。
  - 记录 `chunkIndex`、`sectionPath`、`tokenCount`、`chunkType`。
- Pipeline updates:
  - `parsing`
  - `chunking`
  - `failed`
- Chunk persistence。

验收：

- [x] 文本或 Markdown 文件可以从 handoff 进入 parser/chunker。
- [x] 成功后 PostgreSQL 里有 document chunks。
- [x] 解析失败时 document `failed`，job `failed`，错误脱敏。
- [x] Retry 同一 document 不产生重复 active chunks。
- [x] Parser adapter tests 不依赖外部服务。

实现说明：

- `services/knowledge/internal/platform/parser` 提供 text/markdown parser 和 fixed chunker。
- `services/knowledge/internal/platform/source` 提供可替换 SourceReader：本地 memory reader 和 File Service HTTP reader。
- `ProcessIngestionJob` 通过 `POST /internal/v1/jobs/{jobId}/processing-runs` 触发同步处理运行，完成 `uploaded -> parsing -> chunking -> ready|failed`。
- Task 5 不做 embedding/Qdrant；Task 6 会把 ready 前的最终阶段扩展为 embedding/indexing。

推荐验证：

```bash
cd services/knowledge
go test ./...
go build ./cmd/server
```

验证：

- [x] `docker run --rm -e GOPROXY=https://goproxy.cn,direct -v "$PWD/services/knowledge":/src -w /src golang:1.22-alpine sh -c 'gofmt -w ./cmd ./internal && go test ./... && go build ./cmd/server'`

## Task 6: Embedding And Qdrant Indexing

Status: done.

目标：把 chunks 写入向量索引，并保证重试和删除可控。

范围：

- `internal/platform/embedding`:
  - deterministic local adapter for tests
  - configurable provider interface
- `internal/platform/qdrant`:
  - ensure collection
  - upsert points
  - delete by document/knowledgeBase
  - search
- Pipeline updates:
  - `embedding`
  - `indexing` internal stage
  - final `ready`
- Qdrant payload shape:
  - `knowledge_base_id`
  - `document_id`
  - `chunk_id`
  - `chunk_index`
  - `section_path`
  - `tags`
  - `chunk_type`

验收：

- [x] Ready document 对应 chunks 有 `qdrantPointId`。
- [x] Qdrant payload 不包含 full SQL row、raw object key、API token 或 prompt。
- [x] Re-run pipeline 前能清理旧 points 或用版本隔离。
- [x] Qdrant dependency failure 映射为 `dependency_error`，HTTP response 脱敏。
- [x] 外部依赖 integration tests 有 env gate，不影响默认 `go test ./...`。

实现说明：

- `service.VectorIndex` 支持 `Upsert`、`DeleteByDocument` 和 `Search`，后续 retrieval 复用同一接口。
- Pipeline 在 embedding 后进入 internal `indexing` job stage，写入 Qdrant-compatible stable point ID，并把 tags/metadata 放进最小检索 payload。
- `internal/platform/vector.NewMemoryIndex` 作为默认本地/test vector backend，支持 cosine search、knowledgeBaseIds、tags 和 metadata filter。
- `internal/platform/vector.NewQdrantClient` 提供 Qdrant REST adapter，支持 ensure collection、upsert、delete by document 和 search；默认测试使用 mocked HTTP server，不要求真实 Qdrant。
- `QDRANT_URL` 为空时 runtime 使用 memory vector index；配置 `QDRANT_URL` 时 startup 会 ensure collection。

推荐验证：

```bash
cd services/knowledge
go test ./...
go build ./cmd/server
```

如启用本地依赖：

```bash
cd services/knowledge
docker compose up -d postgres qdrant
go test ./...
```

验证：

- [x] `docker run --rm -e GOPROXY=https://goproxy.cn,direct -v "$PWD/services/knowledge":/src -w /src golang:1.22-alpine sh -c 'gofmt -w ./cmd ./internal && go test ./... && go build ./cmd/server'`

## Task 7: Retrieval API

Status: done.

目标：实现可被前台检索、QA 和 Document 复用的 `knowledge-queries`。

范围：

- `POST /internal/v1/knowledge-queries`
- Request:
  - `query`
  - `knowledgeBaseIds`
  - `topK`
  - `scoreThreshold`
  - `tags`
  - `metadataFilter`
  - `rerank`
  - `rerankTopN`
- Flow:
  - validate request
  - resolve accessible knowledge bases
  - embed query
  - Qdrant search
  - hydrate chunks/documents from PostgreSQL
  - filter deleted/failed/hidden data
  - optional rerank
  - return trace

验收：

- [x] Query 为空返回 `validation_error`。
- [x] `topK` 超限返回 `validation_error`。
- [x] 无权访问的 knowledge base 不参与检索。
- [x] 返回结果字段对齐 gateway `KnowledgeQueryResult`。
- [x] Trace 字段对齐 `KnowledgeQueryTrace`。
- [x] Response 不包含 raw vector、full Qdrant payload、object key、prompt、token、内部 URL。

实现说明：

- 新增 `POST /internal/v1/knowledge-queries`。
- Retrieval flow：校验请求 -> 解析可访问 knowledge bases -> query embedding -> vector search -> chunk/document hydrate -> 权限和 ready 状态过滤 -> response DTO。
- 支持 `knowledgeBaseIds`、`topK`、`scoreThreshold`、`tags`、`metadataFilter`、`rerank`、`rerankTopN`。
- `rerank` 当前只进入 trace 和 `rerankTopN` 截断；真实 rerank provider 留给后续 adapter。
- `FindChunksByIDs` 已加入 memory/PostgreSQL repository，用于根据 vector hit hydrate chunk。
- 默认 memory vector index 已支持 cosine search 和 filters；Qdrant adapter 已支持 search。

推荐验证：

```bash
cd services/knowledge
go test ./...
go build ./cmd/server
```

验证：

- [x] `docker run --rm -e GOPROXY=https://goproxy.cn,direct -v "$PWD/services/knowledge":/src -w /src golang:1.22-alpine sh -c 'gofmt -w ./cmd ./internal && go test ./... && go build ./cmd/server'`

## Task 8: Gateway Proxy And Contract Tests

Status: blocked (outside current ownership scope).

目标：让 frontend 只通过 gateway 访问 Knowledge 能力，并验证公开契约。

范围：

- Gateway knowledge client。
- Gateway routes for active Knowledge operations。
- Context headers:
  - `X-Request-Id`
  - `X-User-Id`
  - `X-User-Roles`
  - `X-User-Permissions`
  - `X-Forwarded-For`
  - `X-Forwarded-Proto`
- Error mapping:
  - downstream domain errors -> gateway envelope。
  - dependency failures -> `dependency_error`。
- Contract tests against `docs/api/gateway.openapi.yaml` schemas。

验收：

- [ ] Gateway 不直接调用 Qdrant、PostgreSQL、MinIO 或 parser。
- [ ] Frontend-facing paths 全部仍是 `/api/v1/**`。
- [ ] Gateway tests 覆盖 success envelope、paginated envelope、error envelope。
- [ ] Mock downstream tests 断言 context headers 被传递。
- [ ] OpenAPI active operations 与 handler/client 保持一致。

当前处理：

- `services/gateway` 不属于当前负责范围。
- 已撤回本次误加入的 gateway Go module、routes、client、tests 和 API 文件。
- `services/gateway` 保持原有占位状态，只保留既有 `.gitkeep`。
- Knowledge Service 继续保留 `/internal/v1/**` 能力，等待 gateway owner 后续接入。

推荐验证：

```bash
cd services/gateway
go test ./...
go build ./cmd/server
```

验证：

- [x] `git status --short services/gateway`

如涉及前端类型或调用：

```bash
bun install
bun run lint
bun run test
```

## Task 9: P1 Reprocessing, Runtime Config, Stats

Status: done.

目标：补齐需求文档中的 P1 管理能力。

范围：

- Reprocessing job:
  - knowledge base strategy changes trigger reprocess。
  - manual retry/reprocess modeled as job resource, not `/retry` path。
- Runtime config:
  - embedding model/provider
  - rerank model/provider
  - parser backend
  - max concurrent jobs
  - timeouts
  - secret references only, no secret values in response。
- Admin observability:
  - knowledge base count
  - document count
  - chunk count
  - ready/failed counts
  - 30-day upload trend
- Retrieval testing endpoint or resource, subject to gateway OpenAPI acceptance。

验收：

- [x] Strategy update 不阻塞 HTTP request 等待所有文档重处理完成。
- [x] Runtime config response 脱敏。
- [x] Stats 查询不扫描大表到不可接受的程度，有索引或聚合策略。
- [x] 未新增 gateway 公开 API；后续若要公开，必须先进入 `docs/api/gateway.openapi.yaml`。
- [x] 不引入动作式稳定 path，例如 `/retry`、`/batch-delete`、`/search`。

实现说明：

- 新增内部 P1 endpoints：
  - `GET /internal/v1/runtime-config`
  - `PATCH /internal/v1/runtime-config`
  - `POST /internal/v1/knowledge-bases/{knowledgeBaseId}/jobs`，当前支持 `jobType=reprocess`
  - `GET /internal/v1/knowledge-stats`
- Runtime config 只返回 provider/model/parser/rerank/retrieval 参数和 secret reference，不返回 secret value。
- Reprocess 建模为 job resource，不使用 `/retry` 或 `/reprocess` 动作式稳定 path。
- Stats 当前通过现有 repository 分页聚合，满足 MVP；大规模数据后应替换为 repository 专用 SQL 聚合。
- 这些 P1 endpoints 仅为 service-local internal API，未进入 gateway public OpenAPI。

验证：

- [x] `docker run --rm -e GOPROXY=https://goproxy.cn,direct -v "$PWD/services/knowledge":/src -w /src golang:1.22-alpine sh -c 'gofmt -w ./cmd ./internal && go test ./... && go build ./cmd/server'`

## Suggested Immediate Next Slice

下一步建议进入 **Knowledge hardening and integration handoff**：

1. 由 gateway owner 接入 Knowledge 的 `/internal/v1/**` routes，并完成 gateway proxy/contract tests。
2. 为 PostgreSQL migration 引入真实数据库集成测试或 migration smoke test。
3. 把同步 `processing-runs` 替换为 worker/queue 执行模型。
4. 接入生产 embedding provider、rerank provider 和 File Service handoff。
5. 为 Qdrant/PostgreSQL 一致性补偿增加后台清理或重建索引任务。

## General Validation Commands

文档设计阶段：

```bash
git diff --check
```

Knowledge service 实现阶段：

```bash
cd services/knowledge
go test ./...
go build ./cmd/server
```

Gateway/File 交互阶段：

```bash
cd services/gateway
go test ./...

cd ../file
go test ./...
```

OpenAPI 或文档契约变更阶段：

```bash
git diff --check
```

如果项目引入 OpenAPI lint 或 schema ref resolver，应在改动 `docs/api/gateway.openapi.yaml` 时同步运行。

## Risky Files

- `services/knowledge/`：旧 Python 原型已移除，后续能力必须按 Go vertical slice 重建。
- `docs/api/gateway.openapi.yaml`：公开契约改动必须谨慎；active paths 不能出现历史动作式路径。
- `docs/services/knowledge.md`：必须和 gateway OpenAPI 保持一致。
- `services/file/`：上传 ownership 在 File，handoff 不能让 Knowledge 接管原文件对象。
- `services/gateway/`：只能代理和归一化，不能实现 Qdrant、parser、embedding 业务逻辑。

## Rollback Points

- Task 2 可回滚到 health/ready-only baseline，不影响其他服务。
- Task 4 handoff 可先在 internal route 里迭代，公开 gateway upload path 不变。
- Task 6 Qdrant indexing 可通过 feature/config gate 控制，避免未稳定时影响 metadata CRUD。
- Task 8 gateway proxy 上线前必须用 mock downstream 和 service-local tests 验证。
