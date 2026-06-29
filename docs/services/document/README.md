# Document 服务接口文档

本文档定义 `document` 服务在项目初期的职责边界、gateway 公开接口、报告生成工作流和实现约束。详细字段、状态码、response envelope 和 schema 以 [`docs/services/gateway/api/openapi.yaml`](../gateway/api/openapi.yaml) 为准；服务本地 OpenAPI 草案见 [`api/openapi.yaml`](api/openapi.yaml)。

前端、管理端、其他后端模块和 MCP 工具调用方只能通过 gateway `/api/v1/**` 访问报告生成能力，不能直接调用 `document` 服务内部地址。

RESTful 路径、统一响应和错误 envelope 以 [前后端集成契约](../../architecture/frontend-backend-contract.md) 为准。报告生成中的“生成、重新生成、导出、重试”统一建模为任务、任务尝试、章节版本和报告文件等资源创建，不在稳定 path 中使用 `generate`、`regenerate`、`export`、`retry`、`download` 等动作词。

## 配套文档

`README.md` 是服务入口，不替代所有细节文档。当前保留以下有独立维护价值的文档：

| 文档 | 说明 |
| --- | --- |
| [`../../architecture/technology-decisions.md`](../../architecture/technology-decisions.md) | 项目当前技术选型基线；`document` 服务实现和本文档约束必须与其保持一致。 |
| [`api/openapi.yaml`](api/openapi.yaml) | `document` 服务本地 OpenAPI 草案；公开稳定契约仍以 gateway OpenAPI 为准。 |
| [`docs/data-models.md`](docs/data-models.md) | 报告生成逻辑数据模型、实体关系、关键字段和存储约束。 |
| [`docs/frontend-api-design.md`](docs/frontend-api-design.md) | 前端 API 层、页面到接口映射和类型使用建议。 |
| [`docs/requirements.md`](docs/requirements.md) | 原始报告生成需求沉淀和验收点。 |

已被 gateway OpenAPI 和本文覆盖的重复 API 契约草稿不再作为当前阅读入口；数据模型、前端 API 设计和需求沉淀这类服务细节文档单独保留。

## 技术基线

`document` 服务实现必须遵循 [技术选型基线](../../architecture/technology-decisions.md)。本服务只补充报告生成特有约束：

- 代码落地时使用独立 Go module，服务代码放在 `services/document/`。
- 使用 `asynq` over Redis 执行大纲、正文、章节和 DOCX 创建任务；PostgreSQL 是任务业务状态权威。
- 由 Document worker 调用 Pandoc/LibreOffice 类工具链生成 DOCX；前端只提交结构化报告数据并下载结果文件。
- 报告生成链路是后续 OpenTelemetry tracing 重点。

## 职责边界

| 范围 | 说明 |
| --- | --- |
| 报告类型 | 维护固定报告类型、显示名称、启用状态和默认模板引用。首期覆盖迎峰度夏检查报告、煤库存审计报告。 |
| 报告模板 | 管理 DOCX 模板文件、模板元数据、启用状态、版本和模板结构配置。 |
| 报告素材 | 管理报告生成可引用的支撑材料业务资源；底层文件对象在服务边界内复用 `file`。 |
| 报告记录 | 保存报告草稿、报告基础信息、状态、创建人、来源和生成时间。 |
| 大纲与章节 | 保存报告大纲版本、章节树、章节正文、表格内容和章节版本。 |
| 生成任务 | 维护大纲生成、正文生成、章节重新生成、报告文件创建等异步任务和任务尝试。 |
| 报告文件 | 管理生成文件元数据和内容读取入口；底层 DOCX 对象在服务边界内复用 `file`。 |
| 统计和日志 | 提供报告统计、每日趋势和报告操作日志，用于管理端展示和排查。 |
| 报告配置 | 管理报告生成配置，包括 AI Gateway profile 引用、默认模板和文件生成默认值。 |

`document` 不负责用户登录、会话签发、RBAC 源数据、知识库索引、Qdrant 检索实现、provider API key 存储、MinIO object key 暴露、QA 聊天流程或前端页面。

## 接入模型

```text
frontend / admin / MCP caller
   |
   v
gateway /api/v1/report-templates
gateway /api/v1/report-materials
gateway /api/v1/reports
gateway /api/v1/report-jobs
gateway /api/v1/report-files
gateway /api/v1/report-settings
   |
   v
document service
   |
   +--> PostgreSQL report, template, section, job, file metadata, logs
   +--> asynq over Redis for async report jobs
   +--> File service base file APIs for template, material, generated DOCX bytes
   +--> Knowledge service retrieval when generation needs source material context
   +--> AI Gateway internal model profiles and OpenAI-compatible chat API
```

Gateway 调用 `document` 服务时应传递：

| Header | 说明 |
| --- | --- |
| `X-Request-Id` | 贯穿一次前端请求或工具调用的 request id。 |
| `X-User-Id` | 已认证用户 ID。 |
| `X-User-Roles` | 逗号分隔的角色列表。 |
| `X-User-Permissions` | 逗号分隔的权限列表。 |
| `X-Forwarded-For` | 原始客户端地址链。 |
| `X-Forwarded-Proto` | 原始协议。 |

前端不得设置 `X-User-Id`、`X-User-Roles`、`X-User-Permissions`；这些字段只能由 gateway 在认证后注入。`document` 只消费这些上下文做权限判断、审计、创建人记录和追踪，不负责登录态创建。

## 公开接口总览

以下路径均相对于 gateway `/api/v1`，稳定机器可读契约以 gateway OpenAPI active paths 为准。

| Method | Path | Owner | 说明 |
| --- | --- | --- | --- |
| `GET` | `/report-types` | `document` | 查询支持的报告类型。 |
| `GET/POST` | `/report-templates` | `document` | 查询模板列表，上传 DOCX 模板。 |
| `GET/PATCH/DELETE` | `/report-templates/{reportTemplateId}` | `document` | 查询、更新、删除或停用模板。 |
| `GET/PATCH` | `/report-templates/{reportTemplateId}/structure` | `document` | 查询或保存模板结构配置。 |
| `GET/POST` | `/report-materials` | `document` | 查询或上传报告素材。 |
| `GET/DELETE` | `/report-materials/{materialId}` | `document` | 查询或删除报告素材。 |
| `GET/POST` | `/reports` | `document` | 查询报告记录，创建报告草稿。 |
| `GET/PATCH/DELETE` | `/reports/{reportId}` | `document` | 查询、更新或删除报告记录。 |
| `GET/POST` | `/reports/{reportId}/outlines` | `document` | 查询大纲版本，创建或保存大纲版本。 |
| `GET/PATCH` | `/reports/{reportId}/outlines/{outlineId}` | `document` | 查询或编辑指定大纲。 |
| `DELETE` | `/reports/{reportId}/outlines/{outlineId}/sections/{sectionId}` | `document` | 删除大纲章节并由服务重新编号。 |
| `GET/POST` | `/reports/{reportId}/sections` | `document` | 查询章节内容，创建章节草稿或批量保存章节。 |
| `GET/PATCH` | `/reports/{reportId}/sections/{sectionId}` | `document` | 查询或更新章节正文、表格和元数据。 |
| `GET/POST` | `/reports/{reportId}/sections/{sectionId}/versions` | `document` | 查询章节版本或创建新版本，可用于单章重新生成。 |
| `GET/POST` | `/reports/{reportId}/jobs` | `document` | 查询报告任务列表，创建大纲生成、正文生成或文件创建任务。 |
| `GET` | `/report-jobs/{jobId}` | `document` | 查询任务状态、进度、结果和错误摘要。 |
| `GET/POST` | `/report-jobs/{jobId}/attempts` | `document` | 查询任务尝试记录，创建新的任务尝试。 |
| `GET` | `/reports/{reportId}/events` | `document` | 查询报告生成事件列表，用于轮询进度或审计。 |
| `GET/POST` | `/report-files` | `document` | 查询报告文件列表，创建生成文件资源。 |
| `GET` | `/report-files/{reportFileId}` | `document` | 查询报告文件元数据。 |
| `GET` | `/report-files/{reportFileId}/content` | `document` | 读取生成文件内容，成功时返回文件流。 |
| `GET` | `/report-statistics/overview` | `document` | 查询报告统计概览。 |
| `GET` | `/report-statistics/daily` | `document` | 查询每日报告趋势。 |
| `GET` | `/report-operation-logs` | `document` | 查询报告相关操作日志。 |
| `GET/PATCH` | `/report-settings` | `document` | 查询或更新报告生成配置。 |

## RESTful 建模规则

| 业务动作 | 稳定资源建模 |
| --- | --- |
| 生成大纲 | `POST /api/v1/reports/{reportId}/jobs`，`jobType=outline_generation`。 |
| 重新生成大纲 | `POST /api/v1/reports/{reportId}/jobs`，`jobType=outline_regeneration`。 |
| 生成正文 | `POST /api/v1/reports/{reportId}/jobs`，`jobType=content_generation`。 |
| 重新生成正文 | `POST /api/v1/reports/{reportId}/jobs`，`jobType=content_regeneration`。 |
| 重新生成指定章节 | `POST /api/v1/reports/{reportId}/sections/{sectionId}/versions`。 |
| 重试失败任务 | `POST /api/v1/report-jobs/{jobId}/attempts`。 |
| 导出 DOCX | `POST /api/v1/report-files`。 |
| 获取导出文件内容 | `GET /api/v1/report-files/{reportFileId}/content`。 |

后续如需报告 SSE，只能先补 gateway OpenAPI active path；当前公开能力通过 `GET /api/v1/reports/{reportId}/events` 轮询事件列表。

## 通用响应结构

JSON 成功、分页和错误响应遵循 [前后端集成契约](../../architecture/frontend-backend-contract.md)。文件内容接口成功时返回二进制流，不包裹 JSON envelope；失败时仍返回统一错误响应。调用方应优先匹配 `error.code`，不要解析 `message` 文案。

## 核心工作流

### 创建报告并生成大纲

1. 调用方查询 `GET /api/v1/report-types` 和 `GET /api/v1/report-templates`。
2. 调用方通过 `POST /api/v1/reports` 创建报告草稿，传入报告类型、模板、主题、专业、业务对象、年份和补充上下文。
3. 调用方通过 `POST /api/v1/reports/{reportId}/jobs` 创建 `outline_generation` 任务。
4. `document` 根据报告类型、模板结构、主题、上下文和材料引用生成大纲版本。
5. 调用方通过 `GET /api/v1/reports/{reportId}/outlines` 或 `GET /api/v1/reports/{reportId}/outlines/{outlineId}` 获取大纲。

### 编辑大纲和章节

1. 调用方通过 `PATCH /api/v1/reports/{reportId}/outlines/{outlineId}` 修改章节标题、顺序和层级。
2. 删除大纲章节使用 `DELETE /api/v1/reports/{reportId}/outlines/{outlineId}/sections/{sectionId}`。
3. 服务必须保持章节树合法并重新计算展示编号。
4. 当前大纲版本是后续正文生成的输入；重新生成时应创建新任务或新章节版本，不隐式覆盖用户编辑。

### 生成正文和重新生成

1. 调用方通过 `POST /api/v1/reports/{reportId}/jobs` 创建 `content_generation` 或 `content_regeneration` 任务。
2. `document` 逐章节生成正文，保存章节内容、结构化表格、引用快照和任务进度。
3. 部分章节失败时，已成功章节不得丢失；任务可进入 `partial_succeeded` 或 `failed`，具体枚举以 OpenAPI 为准。
4. 单章重新生成通过 `POST /api/v1/reports/{reportId}/sections/{sectionId}/versions` 创建新章节版本。`preserveUserEdits` 默认应为 `true`，只有调用方显式传 `false` 才覆盖用户编辑内容。

### 创建报告文件

1. 调用方通过 `POST /api/v1/report-files` 创建报告文件资源，首期格式为 DOCX。
2. `document` 使用最终保存的报告、大纲、章节和样式配置生成文件，不重新执行 AI 生成。
3. 底层文件对象由 `document` 在服务边界内调用 `file` 保存。
4. 调用方通过 `GET /api/v1/report-files/{reportFileId}` 查询元数据，通过 `GET /api/v1/report-files/{reportFileId}/content` 读取内容。

## 核心数据模型

数据库字段建议使用 `snake_case`，公开 API 字段使用 `camelCase`，并以 gateway OpenAPI 为准。

| 实体 | 说明 | 关键字段 |
| --- | --- | --- |
| `ReportType` | 固定报告类型配置。 | `code`、`name`、`enabled`、`defaultTemplateId`。 |
| `ReportTemplate` | DOCX 模板业务资源。 | `id`、`templateName`、`reportType`、`version`、`enabled`、`fileName`、`fileSize`。 |
| `ReportTemplateStructure` | 模板大纲、材料映射和样式配置。 | `outlineSchema`、`materialMappings`、`styleConfig`。 |
| `ReportMaterial` | 报告支撑材料业务资源。 | `id`、`materialName`、`materialType`、`category`、`tags`、`enabled`。 |
| `Report` | 报告草稿和报告记录。 | `id`、`name`、`reportType`、`templateId`、`topic`、`status`、`creatorId`。 |
| `ReportOutline` | 报告大纲版本。 | `id`、`reportId`、`version`、`outlineSchema`、`source`、`isCurrent`。 |
| `ReportSection` | 当前章节内容。 | `id`、`reportId`、`outlineId`、`sectionPath`、`title`、`content`、`tableData`。 |
| `ReportSectionVersion` | 章节历史版本和 AI 生成版本。 | `id`、`sectionId`、`version`、`source`、`content`、`createdBy`。 |
| `ReportJob` | 异步生成任务。 | `id`、`reportId`、`jobType`、`status`、`progress`、`error`。 |
| `ReportJobAttempt` | 任务执行尝试。 | `id`、`jobId`、`attemptNumber`、`status`、`error`。 |
| `ReportEvent` | 任务进度和审计事件。 | `id`、`reportId`、`jobId`、`eventType`、`payload`。 |
| `ReportFile` | 生成文件业务资源。 | `id`、`reportId`、`format`、`status`、`contentPath`、`fileSize`。 |
| `ReportOperationLog` | 报告相关操作日志。 | `id`、`operatorId`、`operationType`、`targetType`、`targetId`、`operationResult`。 |

## 状态约定

`ReportStatus` 公开枚举以 gateway OpenAPI 为准，当前覆盖：

```text
draft | outline_generating | outline_generated | content_generating | generated | exporting | exported | failed | deleted
```

`ReportJobStatus` 公开枚举以 gateway OpenAPI 为准，当前覆盖：

```text
pending | running | succeeded | partial_succeeded | failed | canceled
```

`ReportJobType` 当前覆盖：

```text
outline_generation | outline_regeneration | content_generation | content_regeneration | section_regeneration | report_file_creation
```

长任务失败优先体现在 `ReportJob.status=failed` 和 `ReportJob.error` 中；只有同步创建任务失败时才直接返回 HTTP 错误。

## 报告配置

管理端报告生成配置通过 gateway active path 暴露：

```http
GET /api/v1/report-settings
PATCH /api/v1/report-settings
```

配置范围：

| 字段 | 说明 |
| --- | --- |
| `llm.provider` | 当前固定为 `ai-gateway`。 |
| `llm.profileId` | AI Gateway chat model profile id。 |
| `llm.model` | 上游模型名或业务显示名。 |
| `llm.timeoutSeconds` | 生成调用超时时间。 |
| `defaultTemplates` | `reportType -> reportTemplateId` 默认模板映射。 |
| `file.defaultFormat` | 首期固定为 `docx`。 |
| `file.defaultNumberingMode` | 首期支持 `global`；`by_chapter` 字段保留但不作为首期验收。 |
| `file.defaultStyleProfileId` | 默认 DOCX 样式配置引用。 |

`document` 不保存 provider `baseUrl` 或 `apiKey`。这些敏感配置只由 AI Gateway model profile 拥有，公开响应和日志不得包含。

## 与其他服务的边界

### File Service

报告模板、报告素材和报告文件由 `document` 拥有业务状态。`document` 在内部调用 `file` 的基础文件能力保存和读取底层对象。公开响应不得暴露 file 内部 ID、bucket、object key、MinIO URL、签名 URL 或存储凭据。

### Knowledge Service

报告生成需要材料上下文时，应通过报告素材业务资源和 knowledge 检索能力获取。`document` 不直接访问 Qdrant，不保存知识库切片索引状态，也不绕过 `knowledge` 读取受控知识库数据。

### AI Gateway

`document` 通过 AI Gateway 内部 OpenAI-compatible API 调用模型。业务配置只引用 AI Gateway `profileId`、模型名和业务超时参数；provider 配置、API key 写入状态和密钥轮换仍由 AI Gateway 管理。

### QA 与 MCP 工具

QA 后续可注册 Document MCP 工具，例如：

| MCP 工具 | Gateway 资源 |
| --- | --- |
| `generate_report_outline` | `POST /reports/{reportId}/jobs`，`jobType=outline_generation` |
| `regenerate_report_outline` | `POST /reports/{reportId}/jobs`，`jobType=outline_regeneration` |
| `generate_report_text` | `POST /reports/{reportId}/jobs`，`jobType=content_generation` |
| `regenerate_report_text` | `POST /reports/{reportId}/jobs`，`jobType=content_regeneration` |
| `regenerate_report_section` | `POST /reports/{reportId}/sections/{sectionId}/versions` |
| `get_generation_status` | `GET /report-jobs/{jobId}` |
| `get_template_schema` | `GET /report-templates/{reportTemplateId}/structure` |
| `export_report_docx` | `POST /report-files` |
| `get_report_result` | `GET /reports/{reportId}`、`GET /report-files/{reportFileId}` |

工具内部不得绕过 gateway 权限边界直连数据库、MinIO、Qdrant 或 provider。工具响应只返回安全摘要、业务资源 ID、进度和错误码，不返回 prompt、provider 原始错误、内部 URL、对象存储路径或完整工具私有参数。

## 错误码约定

Document 相关接口使用项目统一错误码：

| Code | HTTP status | Document 场景 |
| --- | --- | --- |
| `validation_error` | `400` | 请求体或查询参数格式错误，例如缺少 `reportType`、`templateId` 或 `jobType`。 |
| `unauthorized` | `401` | 缺少认证凭据或 gateway 未注入有效用户上下文。 |
| `forbidden` | `403` | 已认证但无权访问目标报告、模板、素材或配置。 |
| `not_found` | `404` | 报告、模板、素材、任务、章节或文件不存在，或对当前用户隐藏。 |
| `conflict` | `409` | 当前资源状态不允许修改、删除、导出或重试。 |
| `rate_limited` | `429` | 生成、导出、上传或查询超过配额。 |
| `dependency_error` | `502` | PostgreSQL、Redis、file、knowledge、AI Gateway 或对象存储失败。 |
| `internal_error` | `500` | 未预期服务端错误。 |

错误响应、任务错误、操作日志和工具输出不得包含 SQL、MinIO object key、内部文件路径、prompt、API key、token、provider 原始响应或堆栈。

## 实现与验证要求

- 服务代码放在 `services/document/`，使用独立 Go module；通用数据库、迁移、HTTP、配置、日志、测试和观测规则见 [技术选型基线](../../architecture/technology-decisions.md)。
- 启动时必须校验 PostgreSQL、Redis、file、AI Gateway、DOCX 工具链和监听地址。
- Gateway 只做公开入口、认证上下文、统一 envelope、错误归一化和路由转发，不承载报告生成业务逻辑。
- Redis 只通过 `asynq` 承载任务队列和短期协调；PostgreSQL 中的 `ReportJob`、`ReportJobAttempt`、`ReportEvent` 是权威业务状态。
- 任务最多自动重试 3 次，失败后保留最近尝试摘要；手动重试通过 `report-jobs/{jobId}/attempts` 创建新资源。
- DOCX 创建由 worker 调用 Pandoc/LibreOffice 类工具链完成，生成后通过 file 服务保存底层对象。
- 服务日志和指标不得记录 prompt 全文、文档全文、file reference、object key、token、API key 或 provider 原始响应体。
- 模板文件首期限定 DOCX；模板结构、默认章节和材料映射以数据库配置为权威，不从 DOCX 自动解析。
- 首期大纲生成可使用模板模式；AI 生成能力进入公开语义前必须确保 OpenAPI、状态枚举和错误处理已同步。
- 契约测试应覆盖 active document operations 的 response envelope、字段命名、错误码、request id、权限边界和文件内容接口。
