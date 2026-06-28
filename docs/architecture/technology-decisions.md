# 技术选型基线

本文记录当前团队确认的工程技术选型。后续服务实现、接口文档、CI 和部署文档应以本文为基线；如需偏离，必须在对应服务 README 说明原因并同步更新本文。

## 总体原则

- 后端服务继续采用独立 Go module 的微服务形态。
- 前端继续采用 `apps/web` 下的 Bun + Vite + React + TypeScript 应用。
- 服务间通信以 RESTful HTTP API 为主，公开接口以 gateway OpenAPI 为权威。
- 技术选型优先选择团队可维护、容易在 CI 中验证、和当前代码形态一致的方案。

## 服务文档使用方式

本文是项目技术选型的权威位置。`docs/services/<service>/README.md` 不应复制完整技术栈表；服务文档只记录：

- 本服务是否适用某项通用选型，例如是否拥有 PostgreSQL、Redis/asynq、MinIO、Qdrant 或模型调用。
- 本服务的服务内目录、迁移、任务、日志字段或测试重点。
- 与本文不同的明确偏离原因；偏离必须同时更新本文和对应服务文档。

如果只是说明 `pgx` + `sqlc`、`goose`、`net/http` / `ServeMux`、`slog`、`envconfig` 风格配置、统一测试工具或前端 OpenAPI 类型生成，应在服务文档中链接本文，不重复描述。

## 已确认选型

| 领域 | 选型 | 说明 |
| --- | --- | --- |
| PostgreSQL 访问 | `pgx` + `sqlc` | 不默认引入 ORM。SQL 写在服务本地 query 文件，由 `sqlc` 生成类型安全查询代码；运行时使用 `pgx`。 |
| ORM | 不使用 ORM | 禁止默认引入 GORM/ent 等 ORM。确有必要时必须单独记录设计理由。 |
| 数据库迁移 | `goose` | 每个服务维护自己的 `migrations/`，CI 后续用 `goose` 校验迁移可应用。 |
| 日志 | Go 标准库 `log/slog` | 所有后端服务使用结构化日志，生产默认 JSON 输出。 |
| HTTP 路由 | Go 1.22 `net/http` / `http.ServeMux` | 不默认引入 `gin`/`chi`。统一通过中间件补 request id、recover、timeout、auth、日志和 body limit。 |
| 配置读取 | `envconfig` 风格的环境变量结构化加载 | 每个服务在 `internal/config` 解析并校验配置。可以先手写实现，但行为必须等价于结构化 env config。 |
| Secret 管理 | 分阶段：本地 env，敏感 provider key 使用加密列或 secret ref | AI Gateway 不保存明文 API key。生产环境优先 secret manager；第一阶段可使用数据库加密列。 |
| Redis 队列 | `asynq` | Redis 作为队列后端；PostgreSQL 仍是可追溯业务状态的权威来源。 |
| Redis 缓存/会话 | `go-redis` | Gateway 会话缓存、短期缓存和 asynq 均走 Redis，但缓存不得作为长期业务真相。 |
| Qdrant 客户端 | 短期手写 HTTP client | 当前 Qdrant API 使用面较窄，先保留轻量 HTTP client；接口扩张后再评估官方 client 或生成 client。 |
| MinIO 客户端 | 官方 MinIO Go SDK | File service 封装对象存储，业务服务不得直接暴露 bucket/object key。 |
| 认证 token | Opaque Bearer token + Redis/session 表 | 公开请求仍使用 `Authorization: Bearer <accessToken>`，但 access token 是不可解析的随机 opaque token，不是 JWT。 |
| 密码哈希 | `argon2id` | Auth 服务保存密码凭证时使用 argon2id，并在 auth 设计中固定参数。 |
| 前端 API 类型 | `openapi-typescript` + typed fetch wrapper | OpenAPI 生成类型，手写轻量 fetch wrapper 负责 base URL、Bearer token、envelope 和错误归一化。 |
| 前端 SSE | `fetch` stream wrapper | QA 消息创建使用 POST + `text/event-stream`，不使用只支持 GET 的原生 `EventSource` 作为主实现。 |
| 前端测试 | Vitest + React Testing Library + Playwright | 单元/组件/关键流程 E2E 分层覆盖。 |
| 后端测试 | Go 标准 `testing` + `httptest` | 默认不引入 BDD 测试框架；需要断言辅助时再局部引入轻量库。 |
| CI | GitHub Actions + path filters | 前端、Go 服务、Docker 构建和部署分开，避免无关变更触发全量检查。 |
| 观测 | `slog` + Prometheus metrics，关键链路补 OpenTelemetry tracing | 第一阶段先保证结构化日志和指标；gateway、AI Gateway、QA、document 生成链路再接 tracing。 |
| DOCX 生成 | Document worker 调用 Pandoc/LibreOffice 类工具链 | 后端生成 DOCX，前端只提交结构化报告数据并下载结果文件。 |
| MCP 集成 | 成熟 SDK 或独立 MCP sidecar | QA 负责工具白名单、权限、参数校验和脱敏记录，不手写完整协议栈作为首选。 |

## 三选一决策记录

| 领域 | 备选 1 | 备选 2 | 备选 3 | 当前决定 |
| --- | --- | --- | --- | --- |
| 数据库访问 | `pgx` + 手写 SQL | `pgx` + `sqlc` | GORM/ent ORM | `pgx` + `sqlc` |
| 数据库迁移 | `goose` | `golang-migrate` | Atlas | `goose` |
| 日志 | `slog` | `zap` | `zerolog` | `slog` |
| HTTP 路由 | 标准库 `ServeMux` | `chi` | `gin` | 标准库 `ServeMux` |
| 配置 | 手写 `os.Getenv` | `envconfig` | `viper` | `envconfig` 风格结构化加载 |
| 队列 | Redis Streams 手写 | `asynq` | PostgreSQL queue | `asynq` |
| 前端 API client | 手写 fetch 类型 | `openapi-typescript` + wrapper | Orval | `openapi-typescript` + wrapper |
| 认证 token | Opaque token | JWT access + refresh | HttpOnly cookie session | Opaque Bearer token |
| DOCX 生成 | Go DOCX 库 | Pandoc/LibreOffice | 独立模板服务 | Document worker + Pandoc/LibreOffice 类工具链 |

## 后端落地约定

### PostgreSQL、sqlc 和 pgx

- 每个需要 PostgreSQL 的服务在服务目录维护 `sqlc.yaml`。
- SQL 查询文件放在服务本地目录，例如：

```text
services/<service>/
  internal/repository/queries/
  internal/repository/sqlc/
  migrations/
  sqlc.yaml
```

- `sqlc` 生成代码只能被服务本地 repository 适配层使用，不直接泄露到 HTTP handler。
- 事务由 service/use-case 层发起；repository 接收 `pgx.Tx` 或抽象后的 querier。
- 查询必须显式列名，不使用 `SELECT *`。
- 用户输入只能通过参数绑定传入 SQL。

### goose 迁移

- 迁移文件继续放在 `services/<service>/migrations/`。
- 文件名使用有序前缀，例如 `0001_create_users.sql`。
- 首期允许 forward-only migration；如果写 down migration，必须能在本地和 CI 验证。
- CI 后续应对变更服务执行迁移 apply 校验。

### Redis 和 asynq

- Redis 队列统一使用 `asynq`。
- 任务类型命名使用 `<service>:<resource>:<action>`，例如 `knowledge:document:ingest`。
- 任务 payload 必须是 JSON，并包含可追踪字段：

```json
{
  "requestId": "req_123",
  "jobId": "job_123",
  "userId": "usr_123"
}
```

- 业务状态、任务最终状态、失败摘要和重试次数以 PostgreSQL 为权威；asynq 只负责排队、调度和执行。
- handler 不直接执行长任务，只创建业务 job 记录并投递 asynq task。

### 日志、指标和追踪

- 后端服务默认初始化 `slog.NewJSONHandler(os.Stdout, ...)`。
- 日志字段至少包含 `service`、`request_id`、`operation`、`status`；有用户上下文时可包含 `user_id`。
- 不记录密码、token、API key、数据库连接串、MinIO secret、prompt 全文、文档全文、object key 或 provider 原始响应体。
- 指标第一阶段采用 Prometheus 风格 endpoint；指标 label 不得包含用户输入正文、prompt、token、object key 或 API key 指纹。

## 前端落地约定

- OpenAPI 类型生成统一使用 `openapi-typescript`。
- 生成目录为 `apps/web/src/api/generated/`，生成文件不得手工修改。
- `apps/web/src/api/client.ts` 只负责 transport：base URL、认证头、request id、JSON/form/SSE 处理、envelope 和错误归一化。
- 前端不得继续依赖旧 `{ code, message, data }` envelope；gateway 项目自有 JSON 接口使用 `{ data, requestId }` 成功响应和 `{ error }` 错误响应。
- SSE 通过 `fetch` + stream reader 处理，必须支持 `AbortController` 取消。

## 安全落地约定

- `Authorization: Bearer <accessToken>` 中的 access token 是 opaque token。
- Auth 服务保存 token hash，不保存明文 access token。
- Gateway Redis 会话缓存 key 使用 token hash，不使用原始 token。
- 密码哈希使用 `argon2id`；参数、salt 长度和升级策略由 auth 服务实现文档固定。
- AI Gateway provider API key 不进入日志、响应、指标 label 或前端缓存。

## 后续需要同步的实现任务

- 为每个 Go 服务补充或迁移 `sqlc.yaml`、query 文件和 `pgx` repository。
- 为每个有数据库的服务接入 `goose` 迁移命令和 CI 校验。
- 为需要异步任务的服务接入 asynq client/worker，并把任务状态落 PostgreSQL。
- 前端替换当前手写旧 envelope API client，接入 `openapi-typescript`。
