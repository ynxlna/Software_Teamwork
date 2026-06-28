# 知识管理模块规划

## Goal

按 README 和 backend spec 规划的 Go 微服务方向搭建 `services/knowledge`，使知识管理模块成为智能问答和报告生成的数据底座：负责知识库元数据、文档处理状态、切片、embedding、Qdrant 索引和统一检索 API。

## Confirmed Decisions

- Knowledge 模块迁回 README 规划的 Go 微服务，不继续把现有 Python FastAPI 服务作为正式 MVP 技术栈。
- 旧 `services/knowledge` Python/FastAPI 原型已移除；正式实现应继续基于服务本地 Go module、`cmd/server`、`internal/*`、`api/openapi.yaml`、`migrations/`、Dockerfile 和测试。
- 当前稳定对外契约以 `docs/api/gateway.openapi.yaml` 和 `docs/services/knowledge.md` 为准。
- `docs/接口契约/知识管理-api契约.md` 和 `docs/接口契约/openapi/knowledge.openapi.yaml` 是早期分析产物，其中 `/api/v1/knowledge/...`、`:batch-delete`、`:retry` 等路径不作为当前稳定公开 API。
- File Service 拥有原始文件上传、原文件内容和 file-owned 标签/元数据更新；Knowledge Service 拥有知识库元数据、文档处理状态、chunks、embedding、Qdrant 索引和检索。
- 前端只通过 gateway 访问 `/api/v1/**`；Knowledge 服务提供 gateway/服务间调用的 HTTP 资源接口，不能让前端绕过 gateway。
- QA 和 Document 只能通过 Knowledge 检索接口复用知识能力，不能直接读写 Qdrant。

## Source Documents Reviewed

- `README.md`
- `docs/README.md`
- `docs/requirements/system.md`
- `docs/requirements/knowledge_management_system.md`
- `docs/需求分析/整体需求分析.md`
- `docs/architecture/service-boundaries.md`
- `docs/services/knowledge.md`
- `docs/services/knowledge-service-design.md`
- `docs/api/gateway.openapi.yaml`
- `docs/接口契约/知识管理-api契约.md`
- `docs/接口契约/openapi/knowledge.openapi.yaml`
- `services/knowledge/README.md`
- 旧 `services/knowledge/app/` Python 原型目录（现已移除）
- `.trellis/spec/backend/index.md`
- `.trellis/spec/backend/directory-structure.md`
- `services/file/README.md`

## Requirements

### P0 MVP

- Knowledge Go service skeleton: `services/knowledge/go.mod`、`cmd/server/main.go`、`internal/config`、`internal/http`、`internal/service`、`internal/repository`、`internal/platform`、`api/openapi.yaml`、`migrations/`、`Dockerfile` 和 `README.md`。
- 知识库管理：创建、列表、详情、更新、删除知识库；支持名称、描述、文档类型、分段策略、检索策略、文档数、切片数、创建人和时间字段。
- 文档入库状态：文档必须归属知识库；Knowledge 记录状态流转 `uploaded -> parsing -> chunking -> embedding -> ready | failed`，并保存失败原因、chunk 数、标签和解析信息。
- 文档列表和详情：按知识库分页查询文档，支持状态筛选；详情返回 processing details。
- 切片详情：就绪文档可以分页查看 chunks，包含章节路径、内容预览、token 数、embedding 元数据和向量点引用。
- 向量索引：Knowledge 是 Qdrant 唯一业务写入方；PostgreSQL 保存 durable metadata，Qdrant 保存向量和检索 payload。
- 知识检索：通过 `knowledge-queries` 创建检索请求，支持 query、knowledgeBaseIds、topK、scoreThreshold、tags、metadataFilter、rerank 开关和 rerankTopN。
- 权限边界：gateway 注入用户上下文；Knowledge 不拥有登录和 RBAC 源数据，但必须按传入上下文做资源访问校验。
- 文件边界：原始文件上传、原文件内容读取、file-owned 标签/元数据更新和原文件生命周期由 File Service 拥有；Knowledge 只接收 handoff 后的文件引用和处理上下文。
- 错误响应：使用统一错误码，不泄露 SQL、object key、原始向量、prompt、API key、token、堆栈和内部 URL。

### P1 Follow-up

- 处理任务资源：把 ingest/reprocess/delete 等长任务建模为 job 资源，支持状态查询、失败原因、尝试次数和后续事件流扩展。
- 重处理：知识库分段策略、检索策略、embedding 维度或解析后端变更时，触发就绪文档的后台重处理。
- 批量删除：不用动作式 `:batch-delete` 稳定路径；通过多次 DELETE、批处理资源或明确的 job 资源建模。
- 模型和解析配置：运行时配置 embedding、rerank、解析器后端、并发数、超时和供应商密钥引用；返回脱敏配置。
- 检索测试：管理员可测试召回效果，用于调参和验收。
- 统计监控：知识库数、文档数、切片数、ready/failed 文档数、近 30 天上传趋势。

### Out Of Scope For First Implementation Slice

- QA 会话、消息、SSE 回答生成和 LLM 答案生成。
- 报告模板、报告记录、DOCX 导出和报告生成工作流。
- 完整管理后台聚合指标，除非 gateway OpenAPI 明确接收。
- 组织、电厂、专业等复杂多维数据权限。
- OCR、多模态图片 chunk 和本地模型部署能力。
- 一次性恢复旧 Python 原型的所有本地调试能力；后续能力应按 Go 服务稳定契约分阶段重建。

## External API Surface To Provide

当前对外稳定口径以 `docs/api/gateway.openapi.yaml` 和 `docs/services/knowledge.md` 为准：

| Method | Path | Owner | Purpose |
| --- | --- | --- | --- |
| `GET` | `/api/v1/knowledge-bases` | `knowledge` | 分页查询知识库。 |
| `POST` | `/api/v1/knowledge-bases` | `knowledge` | 创建知识库。 |
| `GET` | `/api/v1/knowledge-bases/{knowledgeBaseId}` | `knowledge` | 查询知识库详情。 |
| `PATCH` | `/api/v1/knowledge-bases/{knowledgeBaseId}` | `knowledge` | 更新知识库元数据、分段策略或检索策略。 |
| `DELETE` | `/api/v1/knowledge-bases/{knowledgeBaseId}` | `knowledge` | 删除知识库业务状态、切片和向量索引。 |
| `GET` | `/api/v1/knowledge-bases/{knowledgeBaseId}/documents` | `knowledge` | 查询知识库内文档处理状态列表。 |
| `POST` | `/api/v1/knowledge-bases/{knowledgeBaseId}/documents` | `file` | 上传原始文件到知识库上下文，上传后交给 Knowledge 入库。 |
| `GET` | `/api/v1/documents/{documentId}` | `knowledge` | 查询文档处理详情。 |
| `PATCH` | `/api/v1/documents/{documentId}` | `file` | 更新 file-owned 文档标签等元数据。 |
| `DELETE` | `/api/v1/documents/{documentId}` | `file` | 删除 file-owned 文档记录和原始文件，并协调 Knowledge 清理索引。 |
| `GET` | `/api/v1/documents/{documentId}/chunks` | `knowledge` | 查询文档切片详情。 |
| `GET` | `/api/v1/documents/{documentId}/content` | `file` | 获取原始文件内容。 |
| `POST` | `/api/v1/knowledge-queries` | `knowledge` | 创建一次知识检索查询并返回召回结果。 |

服务内部后续需要补充的接口或资源：

- File -> Knowledge handoff：创建 ingestion job，输入 `fileId`、`knowledgeBaseId`、filename、mimeType、sizeBytes、tags 和 request/user context。
- Job 查询：`GET /internal/v1/jobs/{jobId}` 或稳定后进入 gateway 的 `/api/v1/jobs/{jobId}`。
- Reprocess job：用 job 资源表达，不使用 `/retry` 或 `/reprocess` 动作式稳定路径。
- Delete cleanup：File 删除后通知 Knowledge 清理 chunks 和 Qdrant points。

## Task Breakdown

### Task 1: Go Service Baseline And Contract Reconciliation

目标：建立 Knowledge Go 微服务骨架，把当前权威契约、历史契约和旧 Python 原型移除状态对齐，形成可执行实现基线。

交付：
- `services/knowledge` 切换为 Go module 布局。
- `GET /healthz`、`GET /readyz` 可运行并有测试。
- `api/openapi.yaml` 或服务接口文档明确内部接口和 gateway 对齐关系。
- 文档说明 Python 原型已移除，以及历史契约废弃路径。
- 形成 P0/P1 API 差距清单。

### Task 2: Knowledge Metadata Persistence And Public DTO Alignment

目标：补齐 KnowledgeBase、KnowledgeDocument、DocumentChunk、ProcessingJob 的 PostgreSQL schema、repository 和 DTO 对齐。

交付：
- 数据表/迁移或初始化脚本可追踪。
- 知识库 CRUD、文档列表/详情、chunk 列表字段对齐 gateway OpenAPI。
- 文档状态枚举严格使用 `uploaded | parsing | chunking | embedding | ready | failed`。
- 测试覆盖分页、过滤、not found、validation error 和 envelope。

### Task 3: File Handoff And Ingestion Job Pipeline

目标：把“上传原文件”和“知识入库处理”拆开，建立 File -> Knowledge handoff 和 job 状态。

交付：
- 内部 handoff 接口或消息契约。
- 创建 ingestion job 后能持久化状态，并驱动解析、切片、embedding、Qdrant 写入。
- 失败原因、attempt、progress/currentStage 可查询。
- 不重新引入 Python/FastAPI runtime；parser/ingest 能力后续按 Go adapter 或 worker 边界重建，不影响 Go API 契约。

### Task 4: Retrieval API And Qdrant Contract Hardening

目标：把 `POST /api/v1/knowledge-queries` 做成可被前台检索、QA 和报告生成复用的稳定检索能力。

交付：
- 支持 knowledgeBaseIds、topK、scoreThreshold、tags、metadataFilter。
- 返回可溯源结果：knowledgeBaseId、documentId、chunkId、documentName、sectionPath、score、contentPreview。
- 不返回原始向量、Qdrant payload、object key、prompt 或内部 URL。
- rerank 字段先保留 trace 或接入可配置 rerank provider，按当前实现能力决定。

### Task 5: Gateway Proxy And Contract Tests

目标：让前端只通过 gateway 访问 Knowledge 能力，并验证网关公开契约。

交付：
- Gateway 代理 active knowledge operations。
- 注入 `X-Request-Id`、`X-User-Id`、`X-User-Roles`、`X-User-Permissions`。
- 错误码和 response envelope 与 `docs/api/gateway.openapi.yaml` 一致。
- 增加契约测试或集成测试，覆盖知识库、文档详情、chunk、query 核心路径。

### Task 6: Reprocessing, Configuration, And Admin Observability

目标：补齐 P1 能力，支持策略变更重处理、运行时配置和统计监控。

交付：
- 策略变更触发 reprocessing job。
- embedding/rerank/parser 配置读取、更新和脱敏展示。
- 知识库、文档、切片、ready/failed 文档和上传趋势统计。
- 检索测试或管理端调参接口按稳定契约进入 OpenAPI。

## Acceptance Criteria

- [x] 当前任务 `prd.md` 记录已确认文档、需求、API 和 task 拆分。
- [x] 已确认 Knowledge 正式实现迁回 README 规划的 Go 微服务。
- [x] 后续实现任务可以按 Task 1 -> Task 6 顺序推进，每个 task 都有清晰边界和交付物。
- [x] 对外 API 只采用当前稳定 gateway 路径，不把历史动作式路径误认为当前公开契约。
- [x] 明确 File、Knowledge、QA、Document 的边界，尤其是原文件和 Qdrant 所有权。
- [x] Task 1 Go service baseline 已在当前任务中完成：Go module、health/ready handlers、tests、Dockerfile、Compose、service-local OpenAPI 和实现说明已对齐。

## Technical Notes

- 当前 `services/knowledge/` 已建立 Go module；旧 Python 原型文件已从服务目录移除。
- `services/file/` 可作为当前仓库内 Go 服务布局、README 和内部 RESTful 口径的参考。
- 后续进入实现前需要读取 `.trellis/spec/backend/index.md`、`directory-structure.md`、`database-guidelines.md`、`api-contracts.md`、`error-handling.md`、`logging-guidelines.md`、`quality-guidelines.md`。
- 若前端开始接入，需要同时加载项目级 `frontend-workflow` skill，并遵循 `.trellis/spec/frontend/index.md`。
