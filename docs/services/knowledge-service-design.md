# Knowledge Service 实现说明

版本：v0.2
日期：2026-06-28
范围：`services/knowledge/` 本地服务实现、知识入库链路、Qdrant 检索、与上游契约的对齐说明

## 1. 文档定位

本文档不再单独定义一套公开 gateway 契约，也不再重复定义项目级开发规则。

本仓库已有上游规范文件，优先级如下：

| 类型 | 权威来源 | 本文档关系 |
| --- | --- | --- |
| 前端到 gateway 的稳定公开 API | [`docs/api/gateway.openapi.yaml`](../api/gateway.openapi.yaml) | 只能跟随，不能覆盖 |
| gateway 职责、RESTful 资源路径、response envelope | [`docs/services/gateway.md`](gateway.md)、[`docs/architecture/frontend-backend-contract.md`](../architecture/frontend-backend-contract.md) | 只能引用，不能另起规范 |
| 服务边界 | [`docs/architecture/service-boundaries.md`](../architecture/service-boundaries.md) | Knowledge Service 必须遵守 |
| File Service 上传和原文件边界 | [`docs/services/file.md`](file.md) | Knowledge Service 不抢原文件 owner |
| 知识管理需求 | [`docs/requirements/knowledge_management_system.md`](../requirements/knowledge_management_system.md) | 作为需求输入，不作为接口契约 |
| 代码目录和质量规则 | [`.trellis/spec/backend/`](../../.trellis/spec/backend/index.md) | 作为工程规范来源 |

因此，本文档只描述 Knowledge Service 当前本地实现和对齐状态，不决定 gateway 公开接口、不替代需求说明、不新增项目级开发规则。凡是本文档与上表文件冲突，以上游文件为准；需要进入前端稳定契约的内容，必须先由 gateway 相关文档和 `docs/api/gateway.openapi.yaml` 接收。

## 2. 当前结论

Knowledge Service 的第一阶段目标是把知识库组负责的链路跑通：

```text
文档进入知识库
  -> 解析
  -> 切片
  -> embedding
  -> 写入 Qdrant
  -> 查询文档状态和 chunks
  -> 通过 knowledge query 做向量召回
```

当前实现位于：

```text
services/knowledge/
```

本地 Docker Compose 也放在该目录下：

```text
services/knowledge/docker-compose.yml
```

根目录不再保留 `kb-service/`。旧运行数据如需保留，只能放在已忽略的运行时目录下，例如：

```text
services/knowledge/legacy-runtime-data/
```

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

本地 `services/knowledge` 为了独立闭环，也临时实现了同一路径的 multipart 文档接收能力。这是本地开发能力，不表示 Knowledge Service 在最终 gateway 公开契约中拥有原文件上传 owner。

后续正式联调建议采用以下任一 handoff 方式，具体以 gateway/file/knowledge 三方确认的内部 handoff 设计为准：

- File Service 保存原文件后，gateway 或 file 调用 Knowledge Service 的内部资源接口创建 ingestion job。
- File Service 保存原文件后，通过消息或事件通知 Knowledge Service 创建 ingestion job。

无论采用哪种方式，Knowledge Service 只处理 `fileId`、文档元数据、解析后的 chunks、向量和检索，不把 MinIO object key 暴露给前端作为权限依据。

## 4. RESTful 对齐原则

RESTful 规范由 `docs/api/gateway.openapi.yaml`、`docs/services/gateway.md`、`docs/architecture/frontend-backend-contract.md` 和 `docs/architecture/service-boundaries.md` 维护。Knowledge Service 只跟随这些规则：所有稳定 path 必须是资源路径，由 HTTP method 表达动作。

允许的资源建模示例：

```text
POST   /api/v1/knowledge-bases
GET    /api/v1/knowledge-bases
GET    /api/v1/knowledge-bases/{knowledgeBaseId}
PATCH  /api/v1/knowledge-bases/{knowledgeBaseId}
DELETE /api/v1/knowledge-bases/{knowledgeBaseId}

POST   /api/v1/knowledge-bases/{knowledgeBaseId}/documents
GET    /api/v1/knowledge-bases/{knowledgeBaseId}/documents

GET    /api/v1/documents/{documentId}
PATCH  /api/v1/documents/{documentId}
DELETE /api/v1/documents/{documentId}
GET    /api/v1/documents/{documentId}/chunks

GET    /api/v1/jobs/{jobId}
POST   /api/v1/knowledge-queries
GET    /api/v1/admin-overview
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

## 5. 当前本地实现接口

以下接口是 `services/knowledge` 当前本地实现，用于 curl、Swagger 和后端联调验证。

注意：`docs/api/gateway.openapi.yaml` 已将 knowledge-owned 知识库、文档处理详情、chunks 和 `knowledge-queries` 提升为 active operations。当前本地实现用于验证这些能力，字段和状态应持续向 gateway OpenAPI 对齐；本地 multipart 上传、job 查询和 `admin-overview` 仍不等同于 knowledge-owned 的前端稳定公开契约。

### 5.1 健康检查

```http
GET /healthz
GET /readyz
```

### 5.2 知识库

```http
POST   /api/v1/knowledge-bases
GET    /api/v1/knowledge-bases
GET    /api/v1/knowledge-bases/{knowledgeBaseId}
PATCH  /api/v1/knowledge-bases/{knowledgeBaseId}
DELETE /api/v1/knowledge-bases/{knowledgeBaseId}
```

创建请求示例：

```json
{
  "id": "kb_linux",
  "name": "Linux Knowledge Base",
  "description": "Linux source and documentation test set",
  "docType": "GENERAL",
  "chunkStrategy": {
    "type": "SEMANTIC_TEXT",
    "chunkSize": 1600,
    "overlap": 200
  },
  "retrievalStrategy": {
    "mode": "VECTOR",
    "topK": 10,
    "scoreThreshold": 0
  }
}
```

### 5.3 文档

```http
POST   /api/v1/knowledge-bases/{knowledgeBaseId}/documents
GET    /api/v1/knowledge-bases/{knowledgeBaseId}/documents
GET    /api/v1/documents/{documentId}
PATCH  /api/v1/documents/{documentId}
DELETE /api/v1/documents/{documentId}
GET    /api/v1/documents/{documentId}/chunks
```

本地 multipart 上传字段：

| Field | Type | 说明 |
| --- | --- | --- |
| `file` | binary | 本地待入库文件 |
| `tags` | JSON string array | 例如 `["linux","local-test"]` |

`tags` 本地也兼容 JSON object，用于元数据过滤实验；但上游当前 file contract 是 `string[]`。

### 5.4 任务

```http
GET /api/v1/jobs/{jobId}
```

当前只实现 job 查询。`GET /api/v1/jobs/{jobId}/events` 只是按 RESTful 资源建模列出的未来方向，当前未实现；是否进入公开契约以后续 gateway OpenAPI 为准。

### 5.5 检索

```http
POST /api/v1/knowledge-queries
```

请求示例：

```json
{
  "query": "parser backend semantic vector indexing",
  "knowledgeBaseIds": ["kb_linux"],
  "topK": 5,
  "scoreThreshold": 0,
  "tags": ["linux"],
  "metadataFilter": {
    "source": "local"
  },
  "rerank": false,
  "rerankTopN": null
}
```

当前实现只执行 embedding + Qdrant vector retrieval + threshold filtering。`rerank` 字段只保留为 trace 和未来扩展点。

### 5.6 本地统计

```http
GET /api/v1/admin-overview
```

该接口只用于本地联调和观察已入库数据。上游当前把 `admin-overview` 标记为 missing contract，正式公开字段以后应由 gateway 聚合契约决定。

## 6. 响应格式

Knowledge Service 当前本地响应尽量跟随 gateway envelope，方便后续代理。

单资源响应：

```json
{
  "data": {
    "id": "kb_linux"
  },
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
    "total": 0
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
      "file": "is required"
    }
  }
}
```

本地调试响应可能包含：

```json
{
  "_fieldDescriptions": {
    "id": "字段中文说明"
  }
}
```

该字段只用于本地调试和后端检查，前端稳定契约不应依赖它。

## 7. 字段命名约定

| 层 | 命名风格 | 示例 |
| --- | --- | --- |
| Public HTTP JSON | camelCase | `knowledgeBaseId`, `chunkCount`, `createdAt` |
| Query parameter | camelCase | `pageSize`, `topK`, `scoreThreshold` |
| PostgreSQL table/column | snake_case | `kb_id`, `chunk_count`, `created_at` |
| Python variable/function | snake_case | `knowledge_base_id`, `chunk_count` |
| Qdrant payload | snake_case | `kb_id`, `document_id`, `chunk_id` |
| Public error code | lower_snake_case | `validation_error`, `not_found` |
| Public document status | lowercase enum | `uploaded`, `ready`, `failed` |

## 8. 状态约定

### 8.1 Public DocumentStatus

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

### 8.2 Internal JobStatus

当前本地实现：

```text
running
succeeded
failed
```

未来异步队列可扩展：

```text
queued
canceled
```

### 8.3 Internal JobStage

当前本地实现：

```text
upload
parsing
chunking
embedding
indexing
done
failed
```

`indexing` 是 job stage，不是当前 public document status。

## 9. 存储模型

### 9.1 PostgreSQL

当前本地服务使用以下表：

```text
knowledge_bases
documents
ingest_jobs
document_chunks
```

PostgreSQL 是业务元数据和处理状态的事实来源。

### 9.2 Qdrant

默认 collection：

```text
knowledge_chunks
```

Qdrant point ID 使用 Qdrant 支持的 UUID。业务 ID 继续使用 `chunk_xxx`、`doc_xxx`、`kb_xxx`。

Qdrant payload 只保留检索和引用溯源需要的最小字段：

```json
{
  "kb_id": "kb_linux",
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

OCR 和视觉多模态后续应沿用同一条边界：解析器产出带 `chunk_type` 的 chunk，PostgreSQL 保存完整文本、版面或视觉元数据，Qdrant payload 只保存检索和溯源需要的最小字段。图片 OCR 可使用 `image_ocr`，图表或截图类多模态索引可在进入公开契约前扩展新的 `chunk_type` 和 metadata schema。

## 10. 本地 Docker Compose

启动目录：

```bash
cd services/knowledge
docker compose up -d --build
```

本地端口：

| Service | Port | 用途 |
| --- | ---: | --- |
| `knowledge-api` | 8000 | Swagger / API |
| `knowledge-worker` | 无公开端口 | 后续异步 worker |
| `postgres` | 5432 | 元数据 |
| `redis` | 6379 | 后续队列/事件 |
| `qdrant` | 6333 / 6334 | 向量库 |
| `minio` | 9000 / 9001 | 本地对象存储 |
| `adminer` | 8080 | PostgreSQL 管理 |
| `redis-commander` | 8081 | Redis 管理 |

当前 Docker Compose 是 knowledge 组本地开发拓扑，不放在仓库根目录，避免影响其他组。

## 11. 本地 folder ingest 脚本

脚本位置：

```text
services/knowledge/scripts/ingest_folder.sh
```

职责：

- 扫描目录。
- 过滤当前基础管线支持的文件后缀。
- 调用 Knowledge Service API。
- 不在 shell 脚本中实现解析、切片、embedding 或 Qdrant 写入。

示例：

```bash
cd services/knowledge

scripts/ingest_folder.sh \
  --dir /home/bao/projects/linux \
  --recursive \
  --upload \
  --kb-id kb_linux \
  --kb-name "Linux Knowledge Base" \
  --tags '["linux","local-test"]' \
  --max-files 20 \
  --show-excluded
```

## 12. 当前支持和暂不支持

当前支持：

- 创建和查询知识库。
- 上传 TXT、MD、PDF、DOCX、XLSX、CSV、JSON、YAML、HTML、XML、常见代码和配置文本。
- 基础语义切片。
- `local_hashing` 离线 embedding，用于验证管线。
- Qdrant upsert 和 vector retrieval。
- 查询文档详情、chunks、job。
- 通过 `_fieldDescriptions` 查看中文字段说明。

暂不支持：

- OCR 和视觉多模态 embedding。
- PPT/PPTX 可靠解析。
- 异步队列 worker。
- job events SSE。
- retry job 资源。
- rerank API 调用。
- 模型配置运行时变更接口。
- 与 gateway active OpenAPI 的逐项契约测试。

## 13. 与需求文档的关系

[`docs/requirements/knowledge_management_system.md`](../requirements/knowledge_management_system.md) 是需求说明，列出了完整目标能力，例如批量删除、失败重试、OCR、PPTX、运行时模型配置、近 30 天趋势图等。

本文档不与 `knowledge_management_system.md` 平级争夺“接口和开发规则”定义权：需求范围以需求说明为准，公开接口和开发规则以上游 gateway / backend 规范为准。本文档只记录当前 Knowledge Service 本地实现。需求中尚未实现或尚未进入 gateway active OpenAPI 的能力，一律作为后续迭代处理，不在本文档中伪装成稳定接口。

## 14. 后续对齐步骤

Knowledge 相关公开契约已进入 gateway OpenAPI。后续接入实现按以下顺序推进：

1. 以 `docs/api/gateway.openapi.yaml` 的 active knowledge operations 作为前端稳定契约。
2. 明确 `POST /api/v1/knowledge-bases/{knowledgeBaseId}/documents` 的 file -> knowledge 内部 handoff。
3. 为 Knowledge Service 增加服务本地 `api/openapi.yaml` 或等价内部接口文档。
4. Gateway 只做路由、鉴权上下文传递和 envelope 归一化，不实现解析、切片、Qdrant 检索。
5. 前端只消费 gateway active OpenAPI，不直接调用 `services/knowledge`。
6. 用契约测试逐项校验本地 Knowledge Service 响应字段与 gateway schema 的差异。

## 15. 验收口径

第一阶段当前验收只覆盖已实现能力：

- `docker compose up -d --build` 后 `knowledge-api` healthy。
- `GET /healthz`、`GET /readyz` 正常。
- 能创建知识库。
- 能上传一个基础文本类文档。
- 文档状态最终为 `ready`。
- 能查询文档 chunks。
- Qdrant 中能看到 `knowledge_chunks` points。
- `POST /api/v1/knowledge-queries` 能召回 chunk。
- 返回字段使用 camelCase envelope。
- 本地 README 和 curl 示例可运行。

第二阶段再验收：

- File Service handoff。
- 异步 worker。
- job events。
- retry job。
- OCR。
- rerank。
- 运行时模型配置。
- gateway 代理实现、契约测试和前端类型生成。
