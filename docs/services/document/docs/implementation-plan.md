# 报告生成后端实现拆解文档

## 1. 文档说明

本文将报告生成需求、公开 API 契约和数据模型拆解为后端实现计划。实现代码应放在 `services/document` 下，gateway 只负责公开入口、认证上下文、统一响应 envelope、错误归一和路由转发，不承载报告生成业务逻辑。

相关文档：

- [技术选型基线](../../../architecture/technology-decisions.md)
- [报告生成需求文档](requirements.md)
- [报告生成数据模型文档](data-models.md)
- [Gateway OpenAPI](../../gateway/api/openapi.yaml)
- [服务边界文档](../../../architecture/service-boundaries.md)

## 2. 实现边界

### 2.1 服务归属

| 能力 | 归属 | 说明 |
|---|---|---|
| 公开 HTTP 入口 | `services/gateway` | 暴露 `/api/v1`，注入 request/user 上下文，统一 envelope 和错误结构 |
| 报告业务状态 | `services/document` | 报告、模板、素材、大纲、章节、任务、文件、日志和统计 |
| PostgreSQL 表 | `services/document/migrations` | document 服务拥有并写入报告生成相关表，迁移由 `goose` 管理 |
| 文件对象 | `services/file` | 保存模板、素材和导出文件的底层对象；document 服务只保存业务元数据和 file reference，公开 API 不返回 object key、file 内部 ID 或内部 URL |
| Redis 队列 | `asynq` over Redis | 执行大纲、正文、章节重新生成和 DOCX 创建任务；PostgreSQL 是任务业务状态权威 |
| DOCX worker | `services/document` | worker 调用 Pandoc/LibreOffice 类工具链生成 DOCX，再通过 file 服务保存 |
| MCP 工具适配 | `services/document` | MCP 工具实现放在 document 模块；工具如需发起 HTTP 调用，目标仍为 gateway `/api/v1` |

### 2.2 不做内容

- 不实现用户认证、登录、角色权限系统。
- 不实现前端页面或管理后台页面。
- 不实现模型服务配置管理。
- 不在 gateway 中实现报告生成业务规则、SQL、文件对象操作或 AI 编排。
- 不在 `document` 中重复实现 file 服务的基础对象存储语义；模板、素材和导出文件应通过 file 服务适配保存和读取底层对象。
- 第一阶段不做完整多人协同编辑、审核流、版本差异对比和回滚。

## 3. 建议目录结构

```text
services/document/
├── go.mod
├── Dockerfile
├── README.md
├── cmd/
│   ├── server/
│   │   └── main.go
│   └── mcp-server/
│       └── main.go
├── internal/
│   ├── config/
│   ├── http/
│   │   ├── middleware.go
│   │   ├── report_handler.go
│   │   ├── template_handler.go
│   │   ├── material_handler.go
│   │   ├── job_handler.go
│   │   ├── file_handler.go
│   │   ├── statistics_handler.go
│   │   └── log_handler.go
│   ├── service/
│   │   ├── report_service.go
│   │   ├── template_service.go
│   │   ├── material_service.go
│   │   ├── job_service.go
│   │   ├── file_service.go
│   │   ├── generation_service.go
│   │   └── audit_service.go
│   ├── repository/
│   │   ├── queries/
│   │   │   ├── reports.sql
│   │   │   ├── templates.sql
│   │   │   ├── jobs.sql
│   │   │   └── files.sql
│   │   ├── sqlc/
│   │   ├── report_repository.go
│   │   ├── template_repository.go
│   │   ├── material_repository.go
│   │   ├── job_repository.go
│   │   ├── file_repository.go
│   │   ├── statistics_repository.go
│   │   └── log_repository.go
│   ├── client/
│   │   ├── file/
│   │   ├── gateway/
│   │   ├── ai_gateway/
│   │   └── knowledge/
│   ├── worker/
│   │   ├── asynq.go
│   │   ├── outline_task.go
│   │   ├── content_task.go
│   │   └── report_file_task.go
│   ├── docx/
│   │   └── renderer.go
│   ├── platform/
│   │   └── model/
│   └── mcp/
│       ├── tools.go
│       └── gateway_client.go
├── api/
│   └── openapi.yaml
├── migrations/
│   └── 0001_create_document_tables.sql
└── sqlc.yaml
```

说明：

- `cmd/mcp-server` 是否第一阶段独立启动可以后续确认，但 MCP 相关代码仍放在 `services/document` 模块内。
- `internal/http` 基于 Go 1.22 `http.ServeMux`，只做请求解析、认证上下文读取、响应映射和错误转换，不写 SQL、不直接调用文件存储。
- `internal/service` 负责状态流转、事务边界、任务入队、任务编排和审计日志。
- `internal/repository` 使用 `pgx` + `sqlc`；手写 SQL 放在 `internal/repository/queries/`，生成代码放在 `internal/repository/sqlc/`，只由 repository 适配层使用。
- `internal/client/file` 封装对 file 服务的对象写入、读取和删除调用。
- `internal/client/ai_gateway` 封装 AI Gateway OpenAI-compatible chat API 调用，业务层只传 profile、模型名和超时，不接触 provider API key。
- `internal/worker` 使用 `asynq` 处理异步任务；任务 payload 使用 JSON 并包含 `requestId`、`jobId`、`userId`。
- `internal/docx` 封装 Pandoc/LibreOffice 类工具链调用，HTTP handler 不直接执行本地命令。
- `internal/config` 使用 `envconfig` 风格结构化环境变量加载并校验必填项。

## 4. 数据库实现拆解

第一阶段建议创建以下表，字段以 [数据模型文档](data-models.md) 为准：

| 顺序 | 表 | 用途 |
|---:|---|---|
| 1 | `report_types` | 两类固定报告类型，可先用种子数据初始化 |
| 2 | `report_templates` | 模板元数据和 file 服务文件引用 |
| 3 | `report_materials` | 素材元数据和 file 服务文件引用 |
| 4 | `report_template_materials` | 模板与素材关联 |
| 5 | `reports` | 报告主记录 |
| 6 | `report_outlines` | 大纲版本 |
| 7 | `report_sections` | 当前章节内容 |
| 8 | `report_section_versions` | 章节版本和 AI 重新生成记录 |
| 9 | `report_jobs` | 生成、重新生成、文件创建等任务 |
| 10 | `report_job_attempts` | 任务重试或重新执行记录 |
| 11 | `report_events` | 任务进度、状态变化和轮询事件 |
| 12 | `report_files` | 导出文件元数据和 file 服务文件引用 |
| 13 | `report_operation_logs` | 操作日志和 MCP 调用审计 |

实现约束：

- 所有表由 `services/document/migrations` 管理，迁移工具固定为 `goose`。
- 迁移文件使用有序前缀，例如 `0001_create_document_tables.sql`、`0002_add_report_events.sql`。
- 服务目录维护 `sqlc.yaml`；SQL 查询文件放在 `internal/repository/queries/`，由 `sqlc` 生成类型安全查询代码。
- Repository 运行时使用 `pgx`/`pgxpool`；事务由 service 层开启并传递给 repository，repository 不自行决定跨资源事务边界。
- SQL 查询必须显式列名，不使用 `SELECT *`；用户输入只能通过参数绑定传入 SQL。
- 不默认引入 GORM、ent 或其他 ORM；确有必要时必须先更新技术决策和本文档。
- PostgreSQL 字段使用 snake_case。
- 公开 API 字段以 `docs/services/gateway/api/openapi.yaml` 的 camelCase schema 为准。
- `file_ref` 或等价 file 服务文件引用只在服务内部使用，不返回给公开 API；如实现中暂存 object key，也必须视为内部实现细节并在接入 file 服务后迁移。
- 任务、文件、事件和日志要带 `request_id`，便于 gateway、document 和 MCP 调用串联。

## 5. HTTP 接口实现拆解

### 5.1 Gateway 到 Document

Gateway 的公开路径已经登记在 `docs/services/gateway/api/openapi.yaml`。实现时建议 gateway 将以下路径转发到 document 服务内部 HTTP 路由，保持公开 response envelope 不变：

| 公开资源 | Document 处理模块 |
|---|---|
| `/api/v1/report-types` | `template_handler` 或 `report_handler` |
| `/api/v1/report-templates/**` | `template_handler` |
| `/api/v1/report-materials/**` | `material_handler` |
| `/api/v1/reports/**` | `report_handler`、`job_handler`、`file_handler` |
| `/api/v1/report-jobs/**` | `job_handler` |
| `/api/v1/report-files/**` | `file_handler` |
| `/api/v1/report-statistics/**` | `statistics_handler` |
| `/api/v1/report-operation-logs` | `log_handler` |

Gateway 转发时必须携带：

- `X-Request-Id`
- `X-User-Id`
- `X-User-Roles`
- `X-User-Permissions`
- `X-Forwarded-For`
- `X-Forwarded-Proto`

### 5.2 Document 内部 HTTP 约定

- Document 服务可以复用公开资源路径作为内部路由，但内部地址不对调用方暴露。
- Document 返回稳定 JSON 成功或错误结构；gateway 可以透传或规范化。
- 二进制文件内容由 `GET /api/v1/report-files/{reportFileId}/content` 经 gateway 输出。
- 内部错误不得包含 SQL、MinIO object key、file 内部 ID、prompt、内部 URL 或密钥。
- 访问日志和业务日志使用 `log/slog` 结构化输出，至少包含 `service`、`request_id`、`operation`、`status`；有用户上下文时可包含 `user_id`。
- 服务第一阶段暴露 Prometheus 风格指标；报告生成、AI Gateway 调用、file 保存和 DOCX 创建链路后续接 OpenTelemetry tracing。

## 6. 核心工作流

### 6.1 创建报告草稿

1. Handler 校验 `name`、`reportType`、`templateId`、`topic`。
2. Service 校验报告类型和模板启用状态。
3. Repository 写入 `reports`。
4. Audit 写入 `report_operation_logs`。
5. 返回 `ReportResponse`。

### 6.2 上传模板或素材

1. Handler 接收 `multipart/form-data`。
2. Service 校验文件类型、大小、报告类型或分类。
3. Document file client 调用 file 服务保存底层对象并获得内部 file reference。
4. Repository 保存业务元数据、文件名、文件大小和 file reference。
5. 返回模板或素材元数据，不返回 file reference、object key 或内部 URL。

### 6.3 生成或重新生成大纲

1. 调用方创建 `ReportJob`，`jobType=outline_generation` 或 `outline_regeneration`。
2. Service 在同一事务内写入 `report_jobs` 和初始 `report_events`。
3. Service 投递 `asynq` task，payload 至少包含 `requestId`、`jobId`、`userId`。
4. Worker 领取任务后创建 `report_job_attempts`，读取报告、模板结构和素材引用。
5. Worker 通过 AI Gateway OpenAI-compatible API 调用模型生成大纲。
6. 成功后写入新的 `report_outlines` 版本，更新 `reports.latest_job_id` 和状态。
7. 失败时记录 `error_code`、`error_message`、attempt 和事件；不删除报告基础信息。

### 6.4 生成或重新生成正文

1. 创建 `ReportJob`，`jobType=content_generation`、`content_regeneration` 或 `section_regeneration`。
2. Service 写入任务和事件后投递 `asynq` task。
3. Worker 按当前大纲逐章节生成正文。
4. 每个章节生成后更新 `report_sections`，并创建 `report_section_versions`。
5. 部分失败时保留成功章节，任务可进入 `partial_succeeded`。
6. 重新生成指定章节时必须保留历史版本，不覆盖审计记录。

### 6.5 创建报告文件

1. `POST /api/v1/report-files` 创建报告文件资源。
2. Service 创建 `ReportJob`，`jobType=report_file_creation`，并投递 `asynq` task。
3. 读取当前报告、章节和模板样式。
4. Worker 调用 Pandoc/LibreOffice 类工具链生成 DOCX，通过 file 服务保存底层对象。
5. 保存 `report_files.file_ref`、`filename`、`file_size`、`status`。
6. 返回 `reportFileId`、`status`、`contentPath`，不返回 file reference、object key 或内部 URL。

### 6.6 查询统计和操作日志

- 统计第一阶段可以实时聚合 `reports`、`report_jobs`、`report_files`。
- 操作日志支持按 `targetType`、`targetId`、`operationType`、`requestId`、`requestSource`、`toolName` 过滤。
- `parameter_summary_json` 和 `metadata_json` 必须脱敏，不保存密钥、完整 prompt、MinIO object key、file 内部 ID 或完整文档内容。

## 7. MCP 实现计划

MCP 工具可以保留动词型工具名，但工具内部映射到 RESTful 资源能力：

| MCP 工具 | 内部处理 |
|---|---|
| `generate_report_outline` | 创建报告或使用已有报告后，创建 `outline_generation` job |
| `regenerate_report_outline` | 创建 `outline_regeneration` job |
| `generate_report_text` | 创建 `content_generation` job |
| `regenerate_report_text` | 创建 `content_regeneration` job |
| `regenerate_report_section` | 创建 `section_regeneration` job 或章节版本 |
| `get_generation_status` | 查询 `GET /report-jobs/{jobId}` |
| `get_report_result` | 查询 `GET /reports/{reportId}` |
| `export_report_docx` | 创建 `POST /report-files` |
| `get_template_schema` | 查询 `GET /report-templates/{reportTemplateId}/structure` |

实现约束：

- MCP 工具暴露在 `services/document` 模块内。
- MCP 工具如需走 HTTP，必须调用 gateway `/api/v1`，不直连 document 内部 HTTP 地址。
- MCP 工具调用必须写入 `report_operation_logs`，记录 `request_id`、`tool_name`、`parameter_summary_json`、`operation_result` 和错误信息。
- MCP 输出不返回 MinIO object key、file 内部 ID、内部 URL、模型密钥或完整 prompt。

## 8. 第一阶段开发顺序

### Step 1 服务骨架

- 创建 `services/document/go.mod`。
- 创建 `cmd/server/main.go`。
- 创建 `internal/config`，按 `envconfig` 风格校验 PostgreSQL、Redis、file 服务、AI Gateway、DOCX 工具链和监听地址等配置。
- 使用 Go 1.22 `http.ServeMux` 创建健康检查、基础路由和中间件。
- 初始化 `log/slog` JSON 日志、request id、recover、timeout、auth context、body limit 和 Prometheus 风格指标入口。
- 创建 Dockerfile。

### Step 2 数据库与基础资源

- 添加 `goose` migration：报告类型、模板、素材、报告主表。
- 添加 `sqlc.yaml` 和 `internal/repository/queries/`。
- 运行 `sqlc generate` 生成 repository 查询代码。
- 实现 repository 和 service。
- 实现报告类型、模板、素材、报告草稿 CRUD。
- 增加 handler/service/repository 单元测试。

### Step 3 大纲、章节和任务

- 添加大纲、章节、章节版本、任务、任务尝试、事件表 migration 和 sqlc query。
- 实现 `ReportJob` 状态机和任务查询。
- 接入 `asynq` client/worker，任务类型命名使用 `document:report:<action>`，payload 包含 `requestId`、`jobId`、`userId`。
- 实现手工大纲保存、章节编辑和章节版本创建。
- 先用可替换的 generation adapter 接入 AI Gateway；如模型端未就绪，可保留 mock adapter，但接口形态仍按 AI Gateway OpenAI-compatible API 设计。

### Step 4 文件导出与 file 服务

- 添加 `report_files` 表。
- 实现 DOCX 文件创建任务框架。
- 实现 Pandoc/LibreOffice 类 DOCX renderer。
- 实现 file 服务 client、文件元数据保存和 content 接口。
- 第一阶段 DOCX 样式可先使用统一默认样式。

### Step 5 统计、日志和 MCP

- 添加操作日志表和统计查询。
- 实现操作日志过滤字段。
- 实现 MCP 工具 adapter 和 gateway client。
- 为 MCP 调用添加审计日志和脱敏参数摘要。

## 9. 测试与验证

实现代码后，每个阶段至少运行：

```bash
cd services/document
sqlc generate
go test ./...
go build ./cmd/server
```

有可用测试数据库时继续验证：

```bash
goose -dir migrations postgres "$DATABASE_URL" up
```

建议测试覆盖：

| 层级 | 测试重点 |
|---|---|
| Handler | 状态码、请求校验、响应 envelope、错误码 |
| Service | 状态流转、任务创建、重试、章节版本、文件创建 |
| Repository | SQL 参数化、分页、软删除、事务一致性 |
| File client | file reference/object key 不外露、上传失败处理 |
| Worker | asynq payload 校验、任务状态落库、自动重试和部分成功 |
| DOCX renderer | 工具链失败、样式默认值、输出文件可读取 |
| MCP | 工具输入校验、gateway 路径映射、审计日志脱敏 |

文档或契约变更后继续运行：

- OpenAPI YAML 解析。
- `$ref` 目标解析。
- `/api/v1/**` 前缀检查。
- 禁止动作词路径检查。

## 10. 风险与待确认

| 风险 | 处理建议 |
|---|---|
| MCP 工具是否独立进程启动未最终确认 | 先保留 `cmd/mcp-server` 规划，第一阶段可只实现内部 adapter |
| AI Gateway profile 或模型能力未就绪 | `generation_service` 以 AI Gateway OpenAI-compatible API 为目标接口，第一阶段可用 mock adapter 保持边界不变 |
| DOCX 样式复杂度可能超出第一阶段 | 先实现统一默认样式，复杂样式后续增强 |
| Redis/asynq 运维依赖影响任务执行 | PostgreSQL 先落 `ReportJob` 权威状态；asynq 投递失败返回同步错误，worker 失败通过 attempt 和 event 追踪 |
| 删除策略未完全确认 | 模板、素材、报告优先软删除，保留审计链路 |

## 11. 交付建议

建议按小 PR 或小提交拆分：

1. `docs(report-generation): prepare document implementation plan`
2. `feat(document): scaffold document service`
3. `feat(document): add report template material records`
4. `feat(document): add report jobs outlines and sections`
5. `feat(document): add report files logs and mcp adapter`

本仓库协作中不要自动 push 或创建 PR，提交和 PR 由用户确认后再执行。
