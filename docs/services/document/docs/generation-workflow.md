# Document 生成工作流

日期：2026-06-30

本文说明 `document` 服务报告生成链路的目标工作流、当前已实现部分和未实现缺口。它用于帮助前端、后端和部署同学判断“现在能联调什么”，不是替代 README、OpenAPI 或 implementation。

## 当前结论

| 范围 | 当前状态 | 说明 |
| --- | --- | --- |
| 报告记录、大纲、章节 | 已实现 | 可以创建报告、保存大纲、维护章节树和章节版本。 |
| Report job / attempt / event | 已实现 | 可以创建 job、查询 job、重试、查询 attempts/events。 |
| asynq worker | 部分实现 | worker 会把 job/attempt 从 pending 推进到 running/succeeded/failed；`report_file_creation` 会执行基础 DOCX 导出，其他生成类 job 仍是状态占位。 |
| 真实大纲/正文生成 | 未实现 | worker 当前不调用 AI Gateway，也不写入 AI 生成的大纲或章节正文。 |
| 报告文件 / DOCX 导出 | 已实现基础闭环 | `POST /report-files` 创建元数据和任务；worker 使用内置 `SimpleDOCXGenerator` 从已保存章节生成基础 DOCX 并通过 File Service 保存，content endpoint 只读取已成功文件。 |
| settings / statistics / operation logs | 已实现 | 支持 settings 持久化、AI Gateway profile 校验、统计查询和脱敏操作日志读写。 |

## 核心资源

| 资源 | 用途 | 当前状态 |
| --- | --- | --- |
| `Report` | 报告草稿、基础信息、生命周期状态。 | 已实现。 |
| `ReportOutline` | 报告大纲版本和章节结构。 | 已实现。 |
| `ReportSection` | 当前章节内容、结构化表格和元数据。 | 已实现。 |
| `ReportSectionVersion` | 单章历史版本和重新生成结果。 | 已实现。 |
| `ReportJob` | 大纲、正文、章节、文件生成等异步任务。 | 已实现状态机。 |
| `ReportJobAttempt` | 每次执行或重试。 | 已实现。 |
| `ReportEvent` | 进度、状态和审计事件。 | 已实现列表读取。 |
| `ReportFile` | 生成文件业务资源。 | 已实现基础闭环；文件未完成或缺少 File Service 内容时，content endpoint 会返回未就绪错误。 |
| `ReportSettings` | AI Gateway profile、默认模板和导出配置。 | 已实现。 |

## Job 类型

当前服务接受以下 `jobType`：

| jobType | 目标语义 | 当前 worker 行为 |
| --- | --- | --- |
| `outline_generation` | 根据报告、模板、材料和上下文生成新大纲。 | 只推进任务状态。 |
| `outline_regeneration` | 基于现有报告重新生成大纲版本。 | 只推进任务状态。 |
| `content_generation` | 根据当前大纲逐章生成正文。 | 只推进任务状态。 |
| `content_regeneration` | 重新生成全文正文。 | 只推进任务状态。 |
| `section_regeneration` | 重新生成指定章节版本。 | 只推进任务状态；单章版本资源本身已实现。 |
| `report_file_creation` | 根据最终报告内容创建 DOCX 文件。 | 读取报告和已保存章节，调用内置 `SimpleDOCXGenerator` 生成基础 DOCX，上传 File Service 后更新文件状态和报告导出元数据。 |

Redis/asynq 只负责排队和执行协调。PostgreSQL 的 `report_jobs`、`report_job_attempts` 和 `report_events` 是业务状态权威。

## 目标工作流

### 1. 创建报告

调用方通过 Gateway 创建报告草稿：

```text
POST /api/v1/reports
```

Document 保存 `Report`，记录创建人、报告类型、模板、主题、业务对象、年份和上下文。此阶段不触发 AI 调用。

### 2. 创建大纲任务

调用方创建任务：

```text
POST /api/v1/reports/{reportId}/jobs
jobType=outline_generation
```

当前实现会：

1. 校验报告访问权限。
2. 创建 `ReportJob(status=pending)`。
3. 创建第 1 次 `ReportJobAttempt(status=pending)`。
4. 投递 asynq task，并回写 `asynqTaskId`。
5. Worker 消费后更新 job/attempt 状态。

目标实现还需要：

1. 读取报告、模板结构、材料摘要和已保存的报告配置。
2. 使用 `ReportSettings` 中的 profile 引用调用 AI Gateway chat completion。
3. 校验模型输出为合法章节树。
4. 写入新的 `ReportOutline` 和对应事件。

### 3. 编辑大纲和章节

大纲和章节编辑已可作为同步资源操作联调：

```text
GET/POST /api/v1/reports/{reportId}/outlines
GET/PATCH /api/v1/reports/{reportId}/outlines/{outlineId}
GET/POST /api/v1/reports/{reportId}/sections
GET/PATCH /api/v1/reports/{reportId}/sections/{sectionId}
GET/POST /api/v1/reports/{reportId}/sections/{sectionId}/versions
```

服务负责维护章节树合法性、章节路径、版本和权限；不依赖 worker 才能保存用户编辑。

### 4. 创建正文任务

目标正文生成通过同一个 jobs 资源建模：

```text
POST /api/v1/reports/{reportId}/jobs
jobType=content_generation
```

当前 worker 只更新状态。目标实现需要逐章节调用 AI Gateway，保存章节正文、结构化表格、引用摘要和事件。部分章节失败时，已成功章节不得被回滚；job 应进入 `partial_succeeded` 或 `failed`，具体枚举以 OpenAPI 为准。

### 5. 创建报告文件

DOCX 创建通过文件资源建模：

```text
POST /api/v1/report-files
GET /api/v1/report-files/{reportFileId}
GET /api/v1/report-files/{reportFileId}/content
```

当前基础实现会：

1. `POST /report-files` 校验报告访问权限，创建 `ReportFile(status=pending)`、`ReportJob` 和 `ReportJobAttempt`。
2. 投递 `report_file_creation` asynq task。
3. Worker 读取 `Report` 和已保存章节，调用内置 `SimpleDOCXGenerator` 生成基础 DOCX package。
4. 通过 File Service 保存底层 bytes，并在 Document 回写 `ReportFile(fileRef, fileSize, status=succeeded)` 和报告导出元数据。
5. `GET /report-files/{reportFileId}/content` 只在文件 `succeeded` 且 File Service 可返回内容时流式读取文件。

后续富 DOCX 实现还需要读取当前大纲、样式配置和模板能力，并固定 Pandoc/LibreOffice worker 镜像或 CLI 版本。不要把非文件类 report job succeeded 解读为已有 AI 大纲、AI 正文或富 DOCX。

## 下游服务边界

| 下游 | Document 应做 | Document 不应做 |
| --- | --- | --- |
| File Service | 保存模板、材料和生成文件 bytes；读取文件内容。 | 暴露 bucket、object key、内部 URL、MinIO 凭据。 |
| AI Gateway | 通过 profile 调用 chat completion；记录 request id。 | 保存 provider base URL/API key，直连 OpenAI/SiliconFlow provider。 |
| Knowledge | 在需要材料上下文时使用受控检索结果。 | 直接访问 Qdrant、绕过权限读取知识库 chunk。 |
| Gateway | 接收公开 `/api/v1/report-*` 请求并注入认证上下文。 | 在 Gateway 实现报告生成业务逻辑。 |

## 事件和进度

当前公开轮询入口：

```text
GET /api/v1/reports/{reportId}/events
```

现阶段报告生成没有 SSE public contract。后续如果需要报告 SSE，必须先补 Gateway OpenAPI active path、前后端集成契约和 Document 实现，不要复用 QA SSE 事件名。

## 验收建议

| 阶段 | 最小验收 |
| --- | --- |
| Job 状态机 | 创建 job 后能查到 job、attempt 和事件；worker 能推进 pending/running/succeeded/failed；重试不会双重 claim。 |
| 大纲生成 | job succeeded 后产生合法 `ReportOutline`；失败时错误摘要脱敏；用户编辑不被隐式覆盖。 |
| 正文生成 | 按章节保存内容和版本；部分失败保留已生成章节；引用和材料摘要不泄露内部 object key。 |
| DOCX 创建 | 基础路径已实现：`POST /report-files` 生成元数据并入队，`report_file_creation` worker 生成基础 DOCX，content endpoint 能返回已成功文件流；后续仍需跨服务 smoke 和富 DOCX 工具链。 |
| 配置/统计/日志 | settings 可持久化 AI Gateway profile 引用；statistics 和 operation logs 不读取 provider/API key 等敏感字段。当前已完成服务端基础闭环。 |

## 当前不可承诺事项

- 不能承诺未配置 PostgreSQL、Redis、File Service 和 document worker 的一键本地环境可以生成 DOCX。
- 不能承诺非文件类 report job succeeded 后已有大纲、正文或文件内容。
- 不能承诺 Document 已经调用 AI Gateway。
- 不能承诺 Pandoc/LibreOffice 富 DOCX 工具链已经落地。
