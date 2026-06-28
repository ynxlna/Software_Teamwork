# Knowledge Service 实现说明

版本：v0.3
日期：2026-06-28
范围：`services/knowledge/` Go 微服务基线、旧 Python 原型移除状态、与上游契约的对齐说明

## 1. 文档定位

本文档描述 `services/knowledge/` 当前本地实现和迁移状态，不单独定义 gateway 公开契约，也不替代需求说明或 Trellis backend 规范。

权威来源如下：

| 类型 | 权威来源 | 本文档关系 |
| --- | --- | --- |
| 前端到 gateway 的稳定公开 API | [`docs/api/gateway.openapi.yaml`](../api/gateway.openapi.yaml) | 只能跟随，不能覆盖 |
| gateway 职责、RESTful 资源路径、response envelope | [`docs/services/gateway.md`](gateway.md)、[`docs/architecture/frontend-backend-contract.md`](../architecture/frontend-backend-contract.md) | 只能引用，不能另起规范 |
| 服务边界 | [`docs/architecture/service-boundaries.md`](../architecture/service-boundaries.md) | Knowledge Service 必须遵守 |
| File Service 上传和原文件边界 | [`docs/services/file.md`](file.md) | Knowledge Service 不抢原文件 owner |
| 知识管理需求 | [`docs/requirements/knowledge_management_system.md`](../requirements/knowledge_management_system.md) | 作为需求输入，不作为接口契约 |
| 代码目录和质量规则 | [`.trellis/spec/backend/`](../../.trellis/spec/backend/index.md) | 作为工程规范来源 |

凡是本文档与上表文件冲突，以上游文件为准；需要进入前端稳定契约的内容，必须先由 gateway 相关文档和 `docs/api/gateway.openapi.yaml` 接收。

当前 Go 模块详细设计和实施拆分记录在 [`.trellis/tasks/06-28-knowledge-management-module/design.md`](../../.trellis/tasks/06-28-knowledge-management-module/design.md) 与 [`.trellis/tasks/06-28-knowledge-management-module/implement.md`](../../.trellis/tasks/06-28-knowledge-management-module/implement.md)。团队级文档只保留稳定概览，避免重复维护两套细节。

## 2. 当前结论

Knowledge Service 已从 Python/FastAPI 原型迁回 README 规划的 Go 微服务方向，旧 Python 原型文件已从 `services/knowledge/` 移除。当前 Go 基线位于：

```text
services/knowledge/
├── go.mod
├── cmd/server/main.go
├── internal/config/
├── internal/http/
├── internal/service/
├── internal/repository/
├── internal/platform/
├── api/openapi.yaml
├── migrations/
├── Dockerfile
└── README.md
```

当前 Go 实现已超过最初运行骨架，服务本地已经具备以下内部能力：

```http
GET /healthz
GET /readyz
GET /internal/v1/knowledge-bases
POST /internal/v1/knowledge-bases
GET /internal/v1/knowledge-bases/{knowledgeBaseId}
PATCH /internal/v1/knowledge-bases/{knowledgeBaseId}
DELETE /internal/v1/knowledge-bases/{knowledgeBaseId}
GET /internal/v1/knowledge-bases/{knowledgeBaseId}/documents
POST /internal/v1/knowledge-bases/{knowledgeBaseId}/ingestion-jobs
GET /internal/v1/documents/{documentId}
GET /internal/v1/documents/{documentId}/chunks
GET /internal/v1/jobs/{jobId}
POST /internal/v1/jobs/{jobId}/processing-runs
POST /internal/v1/knowledge-queries
GET /internal/v1/runtime-config
PATCH /internal/v1/runtime-config
POST /internal/v1/knowledge-bases/{knowledgeBaseId}/jobs
GET /internal/v1/knowledge-stats
```

`services/knowledge/app/`、`requirements.txt` 和 `scripts/ingest_folder.sh` 已移除。旧 Python/FastAPI 原型不再作为 runtime 或接口契约来源；后续能力继续在 Go 的 `internal/` 分层内迭代。

## 3. 服务边界

### 3.1 Knowledge Service 负责

- 知识库元数据。
- 文档处理状态。
- 文档解析、切片、embedding。
- Qdrant collection 和 point 写入。
- chunk 查询。
- retrieval policy 和 retrieval query。
- 返回可溯源的检索结果。

### 3.2 Knowledge Service 不负责

- 用户登录、认证、RBAC。该部分归 `auth` 和 `gateway`。
- 原始文件上传、原文件对象生命周期、原文件内容读取。该部分归 `file`。
- QA 会话、LLM 回答生成、SSE 问答事件。该部分归 `qa`。
- 报告生成、DOCX 导出。该部分归 `document`。
- gateway response envelope 的最终公开归一化。该部分归 `gateway`。

### 3.3 File 与 Knowledge 的边界

上游当前已稳定的公开上传入口是：

```http
POST /api/v1/knowledge-bases/{knowledgeBaseId}/documents
```

在 gateway OpenAPI 中，该 `POST` 当前是 file-owned：File Service 负责保存原文件和 file-owned 元数据；Knowledge Service 负责后续入库状态、切片、向量索引和检索。

后续正式联调建议采用以下任一 handoff 方式，具体以 gateway/file/knowledge 三方确认的内部 handoff 设计为准：

- File Service 保存原文件后，gateway 或 file 调用 Knowledge Service 的内部资源接口创建 ingestion job。
- File Service 保存原文件后，通过消息或事件通知 Knowledge Service 创建 ingestion job。

无论采用哪种方式，Knowledge Service 只处理 `fileId`、文档元数据、解析后的 chunks、向量和检索，不把 MinIO object key 暴露给前端作为权限依据。

## 4. 当前 Go 实现

### 4.1 运维接口

```http
GET /healthz
GET /readyz
```

成功响应使用统一 envelope：

```json
{
  "data": {
    "service": "knowledge",
    "status": "ready"
  },
  "requestId": "req_123"
}
```

错误响应使用统一 error envelope：

```json
{
  "error": {
    "code": "validation_error",
    "message": "request validation failed",
    "requestId": "req_123"
  }
}
```

当前 readiness 返回本地配置摘要，包括 service version、environment、storage backend、embedding provider、embedding dimension 和 Qdrant collection 名称。它不检查 PostgreSQL、Redis、Qdrant 或 MinIO 连通性；这些会在对应 platform client 接入后逐步增强。

### 4.2 配置

| Variable | Default | Description |
| --- | --- | --- |
| `KNOWLEDGE_HTTP_ADDR` | `:8000` | HTTP listen address. |
| `KNOWLEDGE_SERVICE_VERSION` | `0.3.0` | Service version shown by readiness. |
| `KNOWLEDGE_ENV` | `local` | Runtime environment label. |
| `KNOWLEDGE_STORAGE_BACKEND` | `memory` | Metadata backend. Supported values: `memory`, `postgres`. |
| `DATABASE_URL` | unset | PostgreSQL connection string required when `KNOWLEDGE_STORAGE_BACKEND=postgres`. |
| `FILE_SERVICE_BASE_URL` | unset | Optional File Service base URL used by ingestion pipeline source reads. |
| `KNOWLEDGE_SHUTDOWN_TIMEOUT` | `10s` | Graceful shutdown timeout. |
| `EMBEDDING_PROVIDER` | `local_hashing` | Embedding provider label. |
| `EMBEDDING_MODEL` | `local_hashing` | Embedding model label. |
| `EMBEDDING_DIMENSION` | `384` | Embedding vector dimension. |
| `QDRANT_URL` | unset | Optional Qdrant REST base URL. When unset, service uses an in-memory vector index. |
| `QDRANT_COLLECTION` | `knowledge_chunks` | Qdrant collection name for vector indexing and retrieval. |

## 5. RESTful 对齐原则

RESTful 规范由 `docs/api/gateway.openapi.yaml`、`docs/services/gateway.md`、`docs/architecture/frontend-backend-contract.md` 和 `docs/architecture/service-boundaries.md` 维护。Knowledge Service 只跟随这些规则：所有稳定 path 必须是资源路径，由 HTTP method 表达动作。

当前已稳定的 gateway 公开 Knowledge 路径包括：

```text
GET    /api/v1/knowledge-bases
POST   /api/v1/knowledge-bases
GET    /api/v1/knowledge-bases/{knowledgeBaseId}
PATCH  /api/v1/knowledge-bases/{knowledgeBaseId}
DELETE /api/v1/knowledge-bases/{knowledgeBaseId}
GET    /api/v1/knowledge-bases/{knowledgeBaseId}/documents
GET    /api/v1/documents/{documentId}
GET    /api/v1/documents/{documentId}/chunks
POST   /api/v1/knowledge-queries
```

File-owned related paths include：

```text
POST   /api/v1/knowledge-bases/{knowledgeBaseId}/documents
PATCH  /api/v1/documents/{documentId}
DELETE /api/v1/documents/{documentId}
GET    /api/v1/documents/{documentId}/content
```

禁止把动作词放进稳定 path：

```text
/search
/upload
/retry
/batch-delete
/generate
/export
/chat/stream
```

如果未来需要重试、批量删除、事件流，应建模为资源：

| 需求 | RESTful 建议 | 说明 |
| --- | --- | --- |
| 重试处理 | `POST /api/v1/jobs` | body 中声明 `documentId` 和 `jobType` |
| 批量删除 | 多次 `DELETE /api/v1/{resource}/{id}`，或设计批处理资源 | 不使用 `batch-delete` path |
| 处理事件流 | `GET /api/v1/jobs/{jobId}/events` | `events` 是 job 的子资源 |
| QA 流式回答 | `GET /api/v1/qa-sessions/{sessionId}/events` | 归 `qa`，不归 `knowledge` |

## 6. 字段命名约定

| 层 | 命名风格 | 示例 |
| --- | --- | --- |
| Public HTTP JSON | camelCase | `knowledgeBaseId`, `chunkCount`, `createdAt` |
| Query parameter | camelCase | `pageSize`, `topK`, `scoreThreshold` |
| PostgreSQL table/column | snake_case | `knowledge_base_id`, `chunk_count`, `created_at` |
| Go variable/function | mixedCaps | `knowledgeBaseID`, `chunkCount` |
| Qdrant payload | snake_case | `knowledge_base_id`, `document_id`, `chunk_id` |
| Public error code | lower_snake_case | `validation_error`, `not_found` |
| Public document status | lowercase enum | `uploaded`, `ready`, `failed` |

## 7. 状态约定

### 7.1 Public DocumentStatus

当前应与 gateway OpenAPI 中 `DocumentStatus` 对齐：

```text
uploaded
parsing
chunking
embedding
ready
failed
```

`indexing`、`reprocessing`、`deleted` 不进入当前 public `DocumentStatus`。如果后续确需公开，必须先更新 `docs/api/gateway.openapi.yaml`、`docs/architecture/frontend-backend-contract.md` 和对应服务文档。

### 7.2 Future JobStatus

后续异步任务建议使用：

```text
queued
running
succeeded
failed
cancelled
```

## 8. 存储模型方向

### 8.1 PostgreSQL

后续 Go 服务应通过 `internal/repository/` 拥有自己的 PostgreSQL schema。预期核心表包括：

```text
knowledge_bases
knowledge_documents
processing_jobs
document_chunks
```

PostgreSQL 是业务元数据和处理状态的事实来源。

### 8.2 Qdrant

默认 collection：

```text
knowledge_chunks
```

Qdrant point ID 使用 Qdrant 支持的 UUID 或无符号整数。业务 ID 继续使用 `chunk_xxx`、`doc_xxx`、`kb_xxx`。

Qdrant payload 只保留检索和引用溯源需要的最小字段，例如：

```json
{
  "knowledge_base_id": "kb_linux",
  "document_id": "doc_123",
  "chunk_id": "chunk_123",
  "filename": "README.md",
  "section_path": "root",
  "tags": ["linux", "local-test"],
  "chunk_index": 0,
  "chunk_type": "text"
}
```

完整文本、错误原因、文档状态、任务状态必须以 PostgreSQL 为准。

## 9. 本地 Docker Compose

启动目录：

```bash
cd services/knowledge
docker compose up -d --build
```

本地端口：

| Service | Port | 用途 |
| --- | ---: | --- |
| `knowledge-api` | 8000 | Go Knowledge Service baseline |
| `postgres` | 5432 | Future metadata database |
| `redis` | 6379 | Future queue/event backend |
| `qdrant` | 6333 / 6334 | Future vector database |
| `minio` | 9000 / 9001 | Future local object storage |
| `adminer` | 8080 | PostgreSQL management |
| `redis-commander` | 8081 | Redis management |

当前 Docker Compose 是 knowledge 组本地开发拓扑，不放在仓库根目录，避免影响其他组。

## 10. 旧 Python 原型移除状态

旧 Python 原型曾经实现本地 multipart 上传、解析、切片、embedding、Qdrant 写入、job 查询、`admin-overview` 和 folder ingest。这些能力已随原型移除，暂不作为当前 Go baseline 的已实现能力。

后续重建原则：

- 先以 Go 服务稳定工程骨架、HTTP envelope、错误码和配置入口。
- 再迁移知识库 metadata、文档状态、chunks、job 和 retrieval 的 vertical slice。
- 不允许重新引入 Python/FastAPI runtime 作为正式服务入口。
- 不允许前端或 gateway 依赖旧 Python 原型的临时调试字段，例如 `_fieldDescriptions`。

## 11. 后续对齐步骤

Knowledge 相关公开契约已进入 gateway OpenAPI。后续接入实现按以下顺序推进：

1. 以 `docs/api/gateway.openapi.yaml` 的 active knowledge operations 作为前端稳定契约。
2. 明确 `POST /api/v1/knowledge-bases/{knowledgeBaseId}/documents` 的 file -> knowledge 内部 handoff。
3. `services/knowledge/api/openapi.yaml` 维护服务本地 `/internal/v1/**` 接口，不作为 browser-facing 契约。
4. Gateway 只做路由、鉴权上下文传递和 envelope 归一化，不实现解析、切片、Qdrant 检索。
5. 前端只消费 gateway active OpenAPI，不直接调用 `services/knowledge`。
6. 用契约测试逐项校验 Knowledge Service 响应字段与 gateway schema 的差异。

## 12. 当前验收口径

当前 Go service-local 验收：

- `go test ./...` 通过。
- `go build ./cmd/server` 通过。
- Docker image 能 build。
- `docker compose config --quiet` 通过。
- `GET /healthz`、`GET /readyz` 和 `/internal/v1/**` 返回统一 envelope。
- KnowledgeBase metadata CRUD、File handoff、ProcessingJob 状态、Document chunks、Qdrant/memory vector indexing、`knowledge-queries` retrieval 和内部 admin endpoints 已有 service-local tests。
- Gateway 代理实现、契约测试和前端类型生成不属于当前 `services/knowledge` owner 范围，需由 gateway owner 接入。
