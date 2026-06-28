# 报告生成数据模型文档

## 1. 文档说明

本文定义报告生成模块的核心数据模型，用于支撑后端接口、MCP 工具、数据库持久化和 file 服务内部文件引用。

本文只描述逻辑数据模型，不提供具体 SQL 建表语句。数据库访问、迁移和 repository 通用规则以 [技术选型基线](../../../architecture/technology-decisions.md) 为准。

## 2. 存储实现补充

| 领域 | 约束 |
|---|---|
| SQL 文件 | 查询文件放在 `services/document/internal/repository/queries/`，生成代码放在 `internal/repository/sqlc/`。 |
| 事务 | 由 service 层发起并传入 repository；repository 不跨用例自行开启事务。 |
| SQL 安全 | 查询显式列名，不使用 `SELECT *`；所有用户输入通过参数绑定。 |
| 异步任务 | `ReportJob`、`ReportJobAttempt` 和 `ReportEvent` 是 PostgreSQL 中的业务状态权威；`asynq` over Redis 只负责排队和执行。 |
| 文件引用 | 数据库只保存 file 服务内部引用和展示元数据；公开 API 不返回 `file_ref`、object key、bucket、内部 URL 或 file 内部 ID。 |

## 3. 存储边界

### 3.1 数据库

数据库保存结构化业务数据：

- 报告基础信息。
- 报告大纲。
- 报告章节内容。
- 报告模板元数据。
- 素材元数据。
- 报告任务。
- 任务尝试记录。
- 报告事件记录。
- 报告文件记录。
- 操作日志。
- 统计聚合数据或查询视图。

### 3.2 文件对象

文件类对象通过 file 服务保存，file 服务负责与 MinIO 等对象存储交互：

- 报告模板文件。
- 专业素材文件。
- 导出的 DOCX 文件。
- 后续可能保存的生成结果快照文件。

`document` 数据库中只保存 file 服务返回的内部 `file_ref` 和展示所需元数据，不直接保存文件二进制内容，也不把 object key、bucket、内部 URL 或 file 内部 ID 返回给公开 API。报告模板、素材和导出文件的公开资源 ID 分别是 `reportTemplateId`、`materialId` 和 `reportFileId`；`file_ref` 只用于 document 服务边界内调用 file 服务。

## 4. 实体关系概览

```text
ReportType 1 ── N ReportTemplate
ReportType 1 ── N Report

ReportTemplate 1 ── N Report
ReportTemplate N ── N ReportMaterial

Report 1 ── N ReportOutline
Report 1 ── N ReportSection
ReportSection 1 ── N ReportSectionVersion
Report 1 ── N ReportJob
Report 1 ── N ReportFile
Report 1 ── N ReportEvent

ReportJob 1 ── N ReportJobAttempt
ReportJob 1 ── N ReportEvent
ReportJob 1 ── N OperationLog
Report 1 ── N OperationLog
```

## 5. 通用字段约定

| 字段 | 说明 |
|---|---|
| `id` | 主键，建议 UUID |
| `created_at` | 创建时间 |
| `updated_at` | 更新时间 |
| `deleted_at` | 软删除时间，可选 |
| `created_by` | 创建人标识，优先来自 gateway 注入的用户上下文或 MCP 调用上下文 |
| `updated_by` | 更新人标识，优先来自 gateway 注入的用户上下文或 MCP 调用上下文 |

本模块不做用户认证，用户相关字段只用于记录来源和追溯。
数据库字段使用 snake_case；公开 API 字段映射为 camelCase，并以 `docs/services/gateway/api/openapi.yaml` 为准。
当数据库内部字段和公开 API 字段不是简单大小写转换时，应在实体说明中显式写明映射关系。

## 6. 核心实体

### 6.1 ReportType

报告类型可以作为固定枚举，也可以落库方便后续扩展。

| 字段 | 类型建议 | 说明 |
|---|---|---|
| `code` | string | 类型编码，唯一 |
| `name` | string | 类型名称 |
| `description` | string | 类型描述 |
| `enabled` | boolean | 是否启用 |
| `default_template_id` | uuid | 默认模板 ID，可选 |

初始枚举：

| code | name |
|---|---|
| `summer_peak_inspection` | 迎峰度夏检查报告 |
| `coal_inventory_audit` | 煤库存审计报告 |

### 6.2 Report

报告主记录。

| 字段 | 类型建议 | 说明 |
|---|---|---|
| `id` | uuid | 报告 ID |
| `report_name` | string | 报告名称 |
| `report_type` | string | 报告类型编码 |
| `template_id` | uuid | 当前使用模板 |
| `topic` | string | 报告主题 |
| `specialty` | string | 专业 |
| `plant_or_business_object` | string | 电厂或业务对象 |
| `year` | int | 年份 |
| `status` | string | 报告状态 |
| `extra_context_json` | json | 扩展上下文 |
| `creator_id` | string | 创建人标识 |
| `creator_name` | string | 创建人名称 |
| `source` | string | 来源，例如 `frontend`、`admin`、`mcp`、`backend` |
| `latest_job_id` | uuid | 最新报告任务 |
| `latest_report_file_id` | uuid | 最新报告文件 |
| `generated_at` | datetime | 正文生成完成时间 |
| `exported_at` | datetime | 最近导出完成时间 |
| `created_at` | datetime | 创建时间 |
| `updated_at` | datetime | 更新时间 |
| `deleted_at` | datetime | 软删除时间 |

公开 API 字段映射：

| 数据库字段 | 公开 API 字段 |
|---|---|
| `report_name` | `name` |
| `plant_or_business_object` | `businessObject` |

状态枚举建议：

| status | 说明 |
|---|---|
| `draft` | 草稿 |
| `outline_generating` | 大纲生成中 |
| `outline_generated` | 大纲已生成 |
| `content_generating` | 正文生成中 |
| `generated` | 正文已生成 |
| `exporting` | 导出中 |
| `exported` | 已导出 |
| `failed` | 失败 |
| `deleted` | 已删除 |

### 6.3 ReportOutline

报告大纲。

| 字段 | 类型建议 | 说明 |
|---|---|---|
| `id` | uuid | 大纲 ID |
| `report_id` | uuid | 所属报告 |
| `outline_json` | json | 多级大纲树 |
| `version` | int | 大纲版本 |
| `source_job_id` | uuid | 生成或重新生成任务 ID |
| `manual_edited` | boolean | 是否发生过手工编辑 |
| `created_at` | datetime | 创建时间 |
| `updated_at` | datetime | 更新时间 |

说明：

- AI 重新生成大纲时，应提升 `version`。
- 是否保留旧版本由后续实现决定，但生成任务中必须保留请求和响应快照。

### 6.4 ReportSection

报告章节内容。

| 字段 | 类型建议 | 说明 |
|---|---|---|
| `id` | uuid | 章节 ID |
| `report_id` | uuid | 所属报告 |
| `parent_id` | uuid | 父章节 ID，可空 |
| `outline_node_id` | string | 对应大纲节点 ID |
| `title` | string | 章节标题 |
| `level` | int | 层级 |
| `sort_order` | int | 排序 |
| `numbering` | string | 编号 |
| `section_type` | string | 章节类型 |
| `content` | text | 正文内容 |
| `tables_json` | json | 表格内容 |
| `images_json` | json | 图片引用，后续扩展 |
| `generation_status` | string | 章节生成状态 |
| `content_source` | string | 内容来源 |
| `manual_edited` | boolean | 是否手工编辑 |
| `version` | int | 内容版本 |
| `last_job_id` | uuid | 最近生成或重新生成任务 |
| `generated_at` | datetime | 生成时间 |
| `created_at` | datetime | 创建时间 |
| `updated_at` | datetime | 更新时间 |

章节类型枚举建议：

| section_type | 说明 |
|---|---|
| `text` | 正文 |
| `table` | 表格 |
| `image` | 图片 |
| `mixed` | 混合内容 |

内容来源枚举建议：

| content_source | 说明 |
|---|---|
| `ai` | AI 生成 |
| `manual` | 手工编辑 |
| `mixed` | AI 生成后手工编辑 |

### 6.5 ReportSectionVersion

报告章节版本，用于保存手工编辑版本、AI 重新生成版本和后续可能的版本追溯。

| 字段 | 类型建议 | 说明 |
|---|---|---|
| `id` | uuid | 章节版本 ID |
| `report_id` | uuid | 所属报告 |
| `section_id` | uuid | 所属章节 |
| `version` | int | 版本号 |
| `source` | string | 版本来源，例如 `manual`、`ai` |
| `content` | text | 该版本正文内容 |
| `tables_json` | json | 该版本表格内容 |
| `job_id` | uuid | 产出该版本的任务 ID，可空 |
| `requirements` | text | 本次生成或编辑要求摘要，可空 |
| `created_by` | string | 创建人或工具调用来源 |
| `created_at` | datetime | 创建时间 |

说明：

- AI 重新生成指定章节时，应创建新的 `ReportSectionVersion`。
- 当前生效版本可以通过 `ReportSection.version` 或后续实现中的 `current_version_id` 关联。

### 6.6 ReportTemplate

报告模板。

| 字段 | 类型建议 | 说明 |
|---|---|---|
| `id` | uuid | 模板 ID |
| `template_name` | string | 模板名称 |
| `report_type` | string | 绑定报告类型 |
| `version` | int | 模板版本 |
| `file_ref` | string | file 服务内部文件引用，不公开返回 |
| `filename` | string | 原始模板文件名快照；公开 API 字段为 `filename` |
| `file_size` | int64 | 模板文件大小；公开 API 字段为 `fileSize` |
| `structure_json` | json | 大纲结构配置 |
| `style_config_json` | json | DOCX 样式配置 |
| `description` | string | 描述 |
| `enabled` | boolean | 是否启用 |
| `created_by` | string | 创建人 |
| `created_at` | datetime | 创建时间 |
| `updated_at` | datetime | 更新时间 |
| `deleted_at` | datetime | 删除时间 |

公开 API 字段映射：

| 数据库字段 | 公开 API 字段 |
|---|---|
| `template_name` | `templateName` 或 `name`，以对应 OpenAPI 为准 |
| `filename` | `filename` |
| `file_size` | `fileSize` |
| `file_ref` | 不返回 |

### 6.7 ReportMaterial

专业素材。

| 字段 | 类型建议 | 说明 |
|---|---|---|
| `id` | uuid | 素材 ID |
| `material_name` | string | 素材名称 |
| `material_type` | string | 文件类型 |
| `category` | string | 分类 |
| `file_ref` | string | file 服务内部文件引用，不公开返回 |
| `filename` | string | 原始素材文件名快照；公开 API 字段为 `filename` |
| `file_size` | int64 | 素材文件大小；公开 API 字段为 `fileSize` |
| `description` | string | 描述 |
| `tags_json` | json | 标签 |
| `enabled` | boolean | 是否启用 |
| `created_by` | string | 创建人 |
| `created_at` | datetime | 创建时间 |
| `updated_at` | datetime | 更新时间 |
| `deleted_at` | datetime | 删除时间 |

公开 API 字段映射：

| 数据库字段 | 公开 API 字段 |
|---|---|
| `material_name` | `materialName` 或 `name`，以对应 OpenAPI 为准 |
| `filename` | `filename` |
| `file_size` | `fileSize` |
| `file_ref` | 不返回 |

### 6.8 TemplateMaterialLink

模板与素材的关联关系。

| 字段 | 类型建议 | 说明 |
|---|---|---|
| `id` | uuid | 关联 ID |
| `template_id` | uuid | 模板 ID |
| `material_id` | uuid | 素材 ID |
| `usage_type` | string | 用途，例如 `outline`、`content`、`export` |
| `created_at` | datetime | 创建时间 |

## 7. 任务与文件实体

### 7.1 ReportJob

报告任务记录，覆盖生成、重新生成和文件创建等长任务。任务重试或重新执行应通过 `ReportJobAttempt` 记录。

| 字段 | 类型建议 | 说明 |
|---|---|---|
| `id` | uuid | 任务 ID |
| `request_id` | string | 请求标识 |
| `source` | string | 来源，例如 `api`、`mcp` |
| `job_type` | string | 任务类型 |
| `target_type` | string | 目标类型 |
| `target_id` | string | 目标 ID |
| `asynq_task_id` | string | Redis/asynq 任务 ID，可空，用于排队状态与 worker 日志关联 |
| `queue_name` | string | asynq 队列名，例如 `document` |
| `report_id` | uuid | 报告 ID |
| `template_id` | uuid | 模板 ID |
| `request_payload_json` | json | 请求快照 |
| `response_payload_json` | json | 响应快照 |
| `input_snapshot_json` | json | 生成前报告状态快照 |
| `status` | string | 任务状态 |
| `progress_json` | json | 进度 |
| `error_code` | string | 错误码 |
| `error_message` | string | 错误信息 |
| `retry_count` | int | 已自动重试次数 |
| `max_attempts` | int | 自动重试上限，首期固定为 3 |
| `started_at` | datetime | 开始时间 |
| `finished_at` | datetime | 结束时间 |
| `created_at` | datetime | 创建时间 |

任务类型枚举建议：

| job_type | 说明 |
|---|---|
| `outline_generation` | 首次生成大纲 |
| `outline_regeneration` | 重新生成大纲 |
| `content_generation` | 首次生成完整正文 |
| `content_regeneration` | 重新生成完整正文 |
| `section_regeneration` | 重新生成指定章节 |
| `report_file_creation` | 创建报告文件 |

任务状态枚举建议：

| status | 说明 |
|---|---|
| `pending` | 待执行 |
| `running` | 执行中 |
| `succeeded` | 成功 |
| `partial_succeeded` | 部分成功 |
| `failed` | 失败 |
| `canceled` | 已取消 |

### 7.2 ReportJobAttempt

任务尝试记录，用于保留失败任务重试、人工重新触发或系统恢复执行的历史。

| 字段 | 类型建议 | 说明 |
|---|---|---|
| `id` | uuid | 尝试记录 ID |
| `job_id` | uuid | 所属任务 ID |
| `attempt_number` | int | 第几次尝试，从 1 开始 |
| `asynq_task_id` | string | 本次执行对应的 asynq 任务 ID，可空 |
| `trigger_source` | string | 触发来源，例如 `api`、`mcp`、`system` |
| `reason` | string | 触发原因或备注 |
| `request_payload_json` | json | 本次尝试请求快照 |
| `status` | string | 本次尝试状态 |
| `error_code` | string | 错误码 |
| `error_message` | string | 错误信息 |
| `started_at` | datetime | 开始时间 |
| `finished_at` | datetime | 结束时间 |
| `created_at` | datetime | 创建时间 |

说明：

- `POST /api/v1/report-jobs/{jobId}/attempts` 应创建新的 `ReportJobAttempt`，并投递新的 asynq task。
- 原 `ReportJob` 不应被删除或覆盖，便于审计失败原因和重试历史。

### 7.3 ReportEvent

报告事件记录，用于支撑 `GET /api/v1/reports/{reportId}/events` 的轮询进度、状态变化和审计信息。

| 字段 | 类型建议 | 说明 |
|---|---|---|
| `id` | uuid | 事件 ID |
| `report_id` | uuid | 所属报告 |
| `job_id` | uuid | 关联任务 ID，可空 |
| `event_type` | string | 事件类型 |
| `message` | string | 事件说明 |
| `payload_json` | json | 事件附加数据 |
| `created_at` | datetime | 创建时间 |

说明：

- 事件用于对外展示进度和状态，不应包含 prompt、MinIO object key、file 内部 ID、内部 URL 或敏感配置。
- 稳定 SSE 契约未确定前，事件模型先支撑列表轮询。

### 7.4 ReportFile

报告文件记录。

| 字段 | 类型建议 | 说明 |
|---|---|---|
| `id` | uuid | 报告文件 ID |
| `report_id` | uuid | 报告 ID |
| `job_id` | uuid | 文件创建任务 ID |
| `filename` | string | 文件名快照；公开 API 字段为 `filename` |
| `file_type` | string | 文件类型，例如 `docx` |
| `file_ref` | string | file 服务内部文件引用，不公开返回 |
| `file_size` | int64 | 文件大小；公开 API 字段为 `fileSize` |
| `file_status` | string | 文件状态 |
| `created_by` | string | 创建人 |
| `created_at` | datetime | 创建时间 |

公开 API 字段映射：

| 数据库字段 | 公开 API 字段 |
|---|---|
| `file_type` | `format` |
| `file_status` | `status` |
| `filename` | `filename` |
| `file_size` | `fileSize` |
| `file_ref` | 不返回；公开接口返回 `id`、`contentPath` 或通过 content 接口获取文件内容 |

文件状态枚举建议：

| file_status | 说明 |
|---|---|
| `pending` | 待创建 |
| `running` | 创建中 |
| `succeeded` | 创建成功 |
| `failed` | 创建失败 |

### 7.5 OperationLog

操作日志。

| 字段 | 类型建议 | 说明 |
|---|---|---|
| `id` | uuid | 日志 ID |
| `operator_id` | string | 操作者 ID |
| `operator_name` | string | 操作者名称 |
| `operation_type` | string | 操作类型 |
| `target_type` | string | 目标类型 |
| `target_id` | string | 目标 ID |
| `request_id` | string | 请求链路 ID |
| `request_source` | string | 请求来源 |
| `tool_name` | string | MCP 工具名，可空 |
| `parameter_summary_json` | json | MCP 或接口调用参数摘要，可空 |
| `operation_result` | string | 操作结果 |
| `error_message` | string | 错误信息 |
| `metadata_json` | json | 其他审计扩展信息 |
| `created_at` | datetime | 创建时间 |

公开 API 字段映射：

| 数据库字段 | 公开 API 字段 |
|---|---|
| `request_id` | `requestId` |
| `request_source` | `requestSource` |
| `tool_name` | `toolName` |
| `parameter_summary_json` | `parameterSummary` |
| `metadata_json` | `metadata` |

操作类型建议：

- `create_report`
- `update_report`
- `outline_generation`
- `outline_regeneration`
- `save_outline`
- `content_generation`
- `content_regeneration`
- `section_regeneration`
- `update_section`
- `report_file_creation`
- `upload_template`
- `update_template`
- `upload_material`
- `delete_material`
- `mcp_call`

## 8. 统计数据

统计数据可通过实时聚合查询，也可以后续增加聚合表。

### 8.1 ReportDailyStatistic

可选聚合模型。

| 字段 | 类型建议 | 说明 |
|---|---|---|
| `stat_date` | date | 日期 |
| `report_type` | string | 报告类型 |
| `created_count` | int | 新建报告数 |
| `generated_count` | int | 生成成功数 |
| `failed_count` | int | 生成失败数 |
| `exported_count` | int | 导出成功数 |
| `updated_at` | datetime | 更新时间 |

第一阶段可以不建聚合表，直接从报告、任务和导出记录聚合。

## 9. 关键约束

- `Report.report_type` 必须是支持的报告类型。
- `Report.template_id` 应引用启用状态的模板。
- `ReportOutline.report_id` 与 `ReportSection.report_id` 必须属于同一报告。
- AI 重新生成必须创建新的 `ReportJob`。
- 任务重试必须创建新的 `ReportJobAttempt`，不得覆盖原任务失败记录。
- `asynq` task payload 必须是 JSON，并至少包含 `requestId`、`jobId`、`userId`；任务类型命名使用 `document:report:<action>`。
- Redis/asynq 状态不得作为业务真相；任务最终状态、失败摘要、重试次数和事件必须落 PostgreSQL。
- AI 重新生成指定章节时必须创建新的 `ReportSectionVersion`，并更新 `ReportSection.version` 或当前版本引用。
- `ReportEvent` 只保存可对外展示的进度和状态摘要，不得保存 prompt、内部 URL、MinIO object key、file 内部 ID 或敏感配置。
- 重新生成不得删除报告基础信息。
- 重新生成正文或章节时，应更新对应章节的 `last_job_id`。
- 删除模板时，如果已有报告使用该模板，建议只允许停用或软删除。
- 删除素材时，如果已有任务引用该素材，建议只允许软删除。
- 导出文件的 `file_ref` 必须能通过 file 服务定位并读取底层文件对象。
- `file_ref` 只作为服务内部存储引用，不得作为公开 API 字段返回；公开接口应返回文件 ID 或 content 接口路径。
- 操作日志不得记录密钥、完整下载签名等敏感信息。
