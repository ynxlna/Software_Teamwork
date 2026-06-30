# Document 服务实现说明

版本：v0.1
日期：2026-06-30
范围：`services/document/` 当前实现、契约对齐、缺口和后续实现约束

## 1. 文档定位

本文档描述 `document` 当前实现状态和后续实现约束。它只补充服务 README、OpenAPI、架构和技术选型文档，不覆盖这些上游契约。

权威来源：

| 类型 | 权威来源 | 本文档关系 |
| --- | --- | --- |
| 服务公开说明 | `docs/services/document/README.md` | 只能补充，不能覆盖 |
| 服务 OpenAPI | `docs/services/document/api/openapi.yaml` | 只能跟随，不能另起契约 |
| Gateway 公开契约 | `docs/services/gateway/api/openapi.yaml` | 前端稳定契约以 gateway 为准 |
| 服务边界 | `docs/architecture/service-boundaries.md` | 必须遵守 |
| 技术基线 | `docs/architecture/technology-decisions.md` | 必须跟随 |
| 代码实现 | `services/document/` | 本文档记录当前状态和差距 |

凡是本文档与上表文件冲突，以上游文件为准；发现冲突时，在“文档与实现出入”中记录并生成回写或实现任务。

## 2. 当前结论

| 项目 | 状态 | 说明 |
| --- | --- | --- |
| 文档状态 | active | README、需求、数据模型、前端 API 设计和 OpenAPI 存在。 |
| 代码状态 | partial | Go service、PostgreSQL repository、模板/材料/报告/大纲/章节 API、report jobs/attempts/events、asynq worker 状态机、report settings、statistics 和 operation logs 已实现；文件生成、Document MCP tools 和真实 AI/DOCX 生成仍未闭环。 |
| 契约对齐 | partial | Gateway active document paths 有 43 个；除 report files/content 外，当前 Document active routes 已由服务处理。 |
| 数据持久化 | postgres | runtime 使用 PostgreSQL；模板/材料底层文件通过 File Service client。 |
| 测试状态 | partial | service、HTTP、repository tests 存在；集成测试依赖 `DOCUMENT_TEST_DATABASE_URL`。 |
| 建议动作 | 补实现 / 回写文档 | 优先实现 report files/content、Document MCP tools 和真实 AI/Pandoc/DOCX 生成；C-08 合入验证后继续回写状态。 |

## 3. 已实现

| 能力 | 代码位置 | 契约来源 | 验证方式 | 备注 |
| --- | --- | --- | --- | --- |
| 健康/就绪检查 | `services/document/internal/http/server.go` | Document OpenAPI | `cd services/document && go test ./...` | `/readyz` 检查 repository。 |
| 报告类型 | `internal/service/document.go`、`internal/http/types_handlers.go` | Gateway / Document OpenAPI | HTTP tests | `GET /report-types`。 |
| 报告模板 CRUD 和结构 | `internal/http/template_handlers.go`、`internal/service/document.go` | Document README | HTTP/service tests | 创建模板时调用 File Service 保存文件。 |
| 报告材料 CRUD | `internal/http/material_handlers.go`、`internal/service/document.go` | Document README | HTTP/service tests | 创建材料时调用 File Service 保存文件。 |
| 报告记录 CRUD | `internal/http/reports.go`、`internal/service/report_service.go` | Gateway / Document OpenAPI | `TestCreateReportThenGetByOwner` 等 | 含权限和软删除规则。 |
| 大纲和章节 | `internal/service/report_service.go`、`internal/service/outline.go` | Document README | outline/report service tests | 支持大纲版本、章节树、编号、章节版本。 |
| report jobs / attempts / events | `internal/http/job_handlers.go`、`internal/service/job_service.go` | Gateway / Document OpenAPI | job service/http tests | 支持创建任务、查询任务、重试、列出尝试和事件。 |
| asynq client / worker 状态机 | `internal/worker/client.go`、`internal/worker/worker.go`、`cmd/server/main.go` | 技术基线 / Document README | worker/job tests | 创建任务时入队，worker 更新 job/attempt running/succeeded/failed；当前 worker 只完成状态流转，不执行真实生成。 |
| report settings | `internal/http/admin_handlers.go`、`internal/service/admin_service.go`、`internal/repository/admin.go` | Gateway / Document OpenAPI | HTTP/service/repository tests | 持久化 AI Gateway profile 引用、默认模板和文件默认值；`PATCH` 仅 admin/super_admin。 |
| statistics / operation logs | `internal/http/admin_handlers.go`、`internal/service/admin_service.go`、`internal/repository/admin.go` | Gateway / Document OpenAPI | HTTP/service/repository tests | 支持概览、每日趋势和操作日志过滤；日志写入路径做敏感字段脱敏。 |
| AI Gateway profile client | `internal/platform/aigateway/profile_client.go`、`cmd/server/main.go` | AI Gateway internal API | client/config tests | Document 只校验并引用 profile，不保存 provider key。 |
| PostgreSQL repository | `internal/repository`、`migrations/0001_create_report_generation_tables.sql` | 数据模型 | repository tests | runtime 使用 `pgx/v5`。 |
| File Service client | `internal/platform/fileclient` | File/Document 边界 | fileclient tests | multipart 创建 file，delete cleanup。 |

## 4. 未实现

| 缺口 | 文档来源 | 影响范围 | 建议任务 |
| --- | --- | --- | --- |
| report files / content | Gateway OpenAPI | API / file / export | 实现 DOCX 文件创建和内容读取；#223 合入验证前仍标未闭环。 |
| Document MCP tools | Document README / requirements | QA / tool integration | 注册工具、权限校验、参数校验、脱敏输出和调用链路仍未实现。 |
| 真实生成逻辑 | Document README | worker / report content | worker 当前只推进状态，尚未调用 AI Gateway 填充大纲/章节内容；需要真实生成闭环任务。 |
| AI Gateway / Pandoc / LibreOffice generation | Document README | AI provider / DOCX | 实现生成编排和导出工具链；落地前不得承诺 DOCX 可生成。 |

## 5. 文档与实现出入

| 出入点 | 文档要求 | 当前实现 | 风险 | 建议处理 |
| --- | --- | --- | --- | --- |
| Active document paths | Gateway OpenAPI 将 jobs/files/statistics/logs/settings 设为 active | jobs/attempts/events、settings/statistics/logs 已实现；report files/content 仍为 scaffold routes 返回 501 | 前端可创建并观察任务、读取配置/统计/日志，但无法得到真实生成文件 | 补 report files/content，或在契约/owner map 标注阶段性不可调用。 |
| Redis/asynq | README 要求使用 asynq over Redis 执行报告任务 | `cmd/server` 已创建 asynq client/worker，任务创建会入队并持久化 task id | 运行时需要 Redis；worker 目前只更新状态，不执行真实生成 | 补真实生成 handler 和 Redis smoke。 |
| AI Gateway/Pandoc/LibreOffice | README 描述生成和导出依赖 | config 校验相关 env/path，但服务未调用 AI Gateway 或工具链 | 部署方配置后仍不会生成 DOCX | 在 implementation 中标为未实现；补 worker 后更新。 |
| Document MCP tools | README/requirements 描述后续可注册 Document MCP 工具 | 当前没有 Document MCP tool registry、handler 或 QA 调用链路 | 后续排期容易漏掉 MCP tools，或误以为 README 中的工具已可用 | 在本文未实现任务表单列；拆实现任务。 |
| Service path prefix | Gateway public paths 是 `/api/v1/report-*` | Document service 本地 routes 无 `/internal/v1` 前缀，gateway 默认剥离 `/api/v1` | 这与 gateway proxy 逻辑一致但易误解 | README/implementation 明确 document local path 形态。 |

## 6. MVP / mock / memory backend / 占位

| 项目 | 当前用途 | 退出条件 | 关联任务 |
| --- | --- | --- | --- |
| `handleNotImplemented` scaffold | 为剩余 active but pending document operations 返回稳定 501 | report files/content 实现并通过契约测试 | report files follow-up |
| worker success placeholder | 让 report job 队列和状态机先闭环 | worker 执行真实大纲/章节/文件生成；`job succeeded` 对应真实内容产出 | Document 真实生成闭环任务 |
| fake repositories in tests | service/http 单元测试 | 保留测试用 | 无 |
| env-gated repository integration tests | 无 DB 环境跳过 | CI 提供 `DOCUMENT_TEST_DATABASE_URL` | testing required checks 分阶段升级任务 |

## 7. 运行与配置

| 项目 | 当前状态 | 缺口 |
| --- | --- | --- |
| 启动命令 | `cd services/document && go run ./cmd/server` | 需要 PostgreSQL、Redis、File Service 和多个预留 env。 |
| 环境变量 | `DOCUMENT_DATABASE_URL`、`DOCUMENT_REDIS_ADDR`、`DOCUMENT_FILE_SERVICE_URL`、`DOCUMENT_AI_GATEWAY_URL`、`DOCUMENT_AI_GATEWAY_PROFILE_ID`、可选 `DOCUMENT_AI_GATEWAY_SERVICE_TOKEN` / `INTERNAL_SERVICE_TOKEN`、Pandoc/LibreOffice paths | Redis 已用于 asynq；AI Gateway profile client 已用于 settings 校验；AI/Pandoc 生成当前未实际使用。 |
| PostgreSQL / migration | `migrations/0001_create_report_generation_tables.sql`，`sqlc.yaml`，runtime repository | 需要 migration CI/smoke。 |
| Redis / queue | asynq client/worker 已接入 report job enqueue/status lifecycle | 需要 Redis smoke 和真实生成任务。 |
| Object storage / vector store / AI provider | 模板/材料通过 File Service；AI provider 未调用 | report files、DOCX export、AI generation 未实现。 |

## 8. 测试与验证

| 验证项 | 命令或步骤 | 当前结果 | 缺口 |
| --- | --- | --- | --- |
| 单元测试 | `cd services/document && go test ./...` | pass（本次执行） | 真实 DB tests 可能被 env gate 跳过。 |
| 集成测试 | `DOCUMENT_TEST_DATABASE_URL=... go test ./internal/repository` | not run | 需要 PostgreSQL。 |
| 契约测试 | route coverage tests + gateway route matrix | partial | report files/content scaffold routes 仍返回 501。 |
| 手工 smoke | 创建模板/报告/大纲/章节 through gateway | not run | 需要 gateway/auth/file/document。 |

## 9. 建议任务

| 任务 | 类型 | 优先级 | 依据 | 说明 |
| --- | --- | --- | --- | --- |
| 实现 report files/content | 新任务 | P0 | DOCX 导出核心 | 调 File Service 保存并读取生成文件。 |
| 实现 worker 真实生成步骤 | 新任务 | P0 | 报告生成闭环核心 | 在当前 job/attempt/asynq 状态机内调用 AI Gateway、更新大纲/章节和事件。 |
| 实现 Document MCP tools | 新任务 | P0 | README / requirements 已保留工具目标 | 注册 `generate_report_outline`、`generate_report_text`、`get_generation_status`、`get_report_result`、`export_report_docx` 等工具，并覆盖权限和脱敏输出。 |
| C-08 合入后回写状态文档 | 回写文档 | P1 | open PR 不提前算 develop 能力 | 合入并验证后更新能力矩阵、generation workflow 和本文。 |
| 回写预留配置状态 | 回写文档 | P1 | AI/Pandoc env 当前要求但未使用 | 防部署误判。 |

## 10. 最近检查记录

| 日期 | 检查人/工具 | 代码基准 | 结论 |
| --- | --- | --- | --- |
| 2026-06-30 | Codex C-08 redo | `31711d9` + working tree | Document 已补 report settings、statistics、operation logs、AI Gateway profile validation 和日志脱敏写入；report files、Document MCP tools 和真实 AI/Pandoc/DOCX 生成仍未闭环。 |
| 2026-06-29 | Codex after proxy rebase | `0e402ca` + working tree | Document 已补 report jobs/attempts/events 和 asynq worker 状态机；报告文件、统计/settings、真实 AI/Pandoc/DOCX 生成仍是主要缺口。 |
| 2026-06-29 | Codex goal | `eddf917` + working tree | Document 已有模板、材料、报告、大纲、章节基础能力；当时生成任务、报告文件、统计/settings/worker 仍是主要缺口，后续 `develop` 已补 jobs/worker 状态机。 |
