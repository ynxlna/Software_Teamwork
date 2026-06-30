# Auth 服务实现说明

版本：v0.1
日期：2026-06-29
范围：`services/auth/` 当前实现、契约对齐、缺口和后续实现约束

## 1. 文档定位

本文档描述 `auth` 当前实现状态和后续实现约束。它只补充服务 README、OpenAPI、架构和技术选型文档，不覆盖这些上游契约。

权威来源：

| 类型 | 权威来源 | 本文档关系 |
| --- | --- | --- |
| 服务公开说明 | `docs/services/auth/README.md` | 只能补充，不能覆盖 |
| 服务 OpenAPI | `docs/services/auth/api/openapi.yaml` | 只能跟随，不能另起契约 |
| Gateway 公开契约 | `docs/services/gateway/api/openapi.yaml` | 前端稳定契约以 gateway 为准 |
| 服务边界 | `docs/architecture/service-boundaries.md` | 必须遵守 |
| 技术基线 | `docs/architecture/technology-decisions.md` | 必须跟随 |
| 代码实现 | `services/auth/` | 本文档记录当前状态和差距 |

凡是本文档与上表文件冲突，以上游文件为准；发现冲突时，在“文档与实现出入”中记录并生成回写或实现任务。

## 2. 当前结论

| 项目 | 状态 | 说明 |
| --- | --- | --- |
| 文档状态 | active | README、数据模型和内部 OpenAPI 存在。 |
| 代码状态 | implemented | Go service、PostgreSQL repository、migrations、用户/会话内部 API、argon2id、token hash 和测试已落地。 |
| 契约对齐 | aligned | Auth 内部 routes 与服务 OpenAPI 主体一致；Gateway 公开 auth routes 已有专门 handler。 |
| 数据持久化 | postgres | `AUTH_DATABASE_URL` 配置后使用 PostgreSQL；无 memory runtime。 |
| 测试状态 | covered | config、repository mapping、service crypto/session、HTTP handler 测试存在。 |
| 建议动作 | 补测试 / 回写文档 | 补真实 DB migration smoke 和初始化管理员账号说明；清理 README 中已过时的“代码未落地”描述。 |

## 3. 已实现

| 能力 | 代码位置 | 契约来源 | 验证方式 | 备注 |
| --- | --- | --- | --- | --- |
| 健康/就绪检查 | `services/auth/internal/http/server.go` | `docs/services/auth/api/openapi.yaml` | `cd services/auth && go test ./...` | `/readyz` 检查 PostgreSQL。 |
| 创建用户 | `services/auth/internal/http/auth_handlers.go`、`internal/service/auth.go` | Auth OpenAPI / Gateway OpenAPI | service/http tests | 成功返回 session response。 |
| 创建会话 | `services/auth/internal/service/auth.go` | Auth OpenAPI / Gateway OpenAPI | `TestCreateSessionRejectsWrongPasswordAndRecordsFailure` | 校验密码并签发 opaque token。 |
| 查询用户和权限 | `services/auth/internal/http/server.go`、`internal/repository/postgres.go` | Auth OpenAPI | repository/http tests | 支持 user summary 和 permissions。 |
| 查询/撤销会话 | `services/auth/internal/service/auth.go` | Auth OpenAPI | `TestRevokedTokenNoLongerReturnsActiveSession` | 只保存 token hash。 |
| 密码哈希 | `services/auth/internal/service/crypto.go` | `technology-decisions.md` | `TestPasswordHashUsesArgon2idV1PHC` | argon2id PHC 参数固定。 |
| PostgreSQL schema | `services/auth/migrations/0001_create_auth_core_tables.sql`、`0002_seed_auth_roles_permissions.sql` | Auth 数据模型 | goose apply 需手工 | seed roles/permissions 存在。 |
| 服务间 token | `services/auth/internal/http/server.go` | Auth README | handler tests | 配置后校验 `X-Service-Token`。 |

## 4. 未实现

| 缺口 | 文档来源 | 影响范围 | 建议任务 |
| --- | --- | --- | --- |
| 初始化管理员账号流程未形成公开 smoke | `docs/requirements-analysis/overall-requirements-analysis.md` | deploy / auth | 待确认：补 seed/admin bootstrap 文档或命令。 |
| 真实 PostgreSQL migration apply 测试未在本次证明 | `technology-decisions.md` | CI / DB | 待确认：补 CI 或本地脚本。 |
| 限流/风控不在当前代码范围 | Auth README 安全事件扩展 | security | 待确认：作为后续增强。 |

## 5. 文档与实现出入

| 出入点 | 文档要求 | 当前实现 | 风险 | 建议处理 |
| --- | --- | --- | --- | --- |
| README 状态记录 | README 曾称 `services/auth/` 代码尚未落地 | 实际已有 Go module、migrations、repository、HTTP routes；本次已回写 README | 后续若重复写实现状态，容易再次漂移 | README 只链接 implementation，当前状态在本文维护。 |
| 技术选型 pgx 版本 | 技术基线已统一目标为 `pgx/v5@v5.7.6` | Auth 仍使用 `pgx/v4`，其他 PostgreSQL 服务多已使用 `pgx/v5` | Auth 与目标基线不一致，后续迁移时可能触及 repository、tests 和 go.mod | 将 Auth 迁移到 `pgx/v5`；迁移完成前不得把 Auth 的 v4 用法复制到新服务。 |
| 无 DB 时 runtime | README 允许无 `AUTH_DATABASE_URL` 启动但 ready 503 | 当前 handlers 无 auth service 时业务 routes 会依赖缺失服务 | 本地误以为可用 | README/implementation 说明无 DB 仅用于进程启动检查。 |

## 6. MVP / mock / memory backend / 占位

| 项目 | 当前用途 | 退出条件 | 关联任务 |
| --- | --- | --- | --- |
| 无 DB 启动模式 | 本地验证 health 和配置默认值 | 真实业务联调必须配置 PostgreSQL | 无 |
| repository row tests | 不依赖真实 PostgreSQL 的 SQL mapping 校验 | 保留，同时补 migration smoke | 待确认 |

## 7. 运行与配置

| 项目 | 当前状态 | 缺口 |
| --- | --- | --- |
| 启动命令 | `cd services/auth && AUTH_HTTP_ADDR=:8001 go run ./cmd/server` | 业务可用需配置 DB、token secret、service token。 |
| 环境变量 | `AUTH_DATABASE_URL`、`AUTH_INTERNAL_SERVICE_TOKEN`、`AUTH_TOKEN_HASH_SECRET`、session TTL、default role、timeouts | 需要部署 secret 注入说明。 |
| PostgreSQL / migration | `migrations/0001` + `0002`，`sqlc.yaml`，runtime repository | 需 migration CI/smoke 证据。 |
| Redis / queue | Auth 不使用 Redis；Gateway 使用 Redis session cache | 无。 |
| Object storage / vector store / AI provider | 不涉及 | 无。 |

## 8. 测试与验证

| 验证项 | 命令或步骤 | 当前结果 | 缺口 |
| --- | --- | --- | --- |
| 单元测试 | `cd services/auth && go test ./...` | pass（本次执行） | 无真实 DB apply。 |
| 集成测试 | goose apply + create user/session smoke | missing | 需要 PostgreSQL 环境。 |
| 契约测试 | HTTP handler tests + Gateway auth proxy tests | partial | 未从 OpenAPI 自动生成校验。 |
| 手工 smoke | 注册、登录、`/users/me` through gateway | not run | 需要 gateway + Redis + auth。 |

## 9. 建议任务

| 任务 | 类型 | 优先级 | 依据 | 说明 |
| --- | --- | --- | --- | --- |
| 补 Auth DB migration smoke | 新任务 | P0 | Auth 是 gateway 鉴权上游 | 验证 migrations、seed roles、create session。 |
| 补管理员账号初始化说明 | 新任务 | P1 | 管理端需要管理员身份 | 形成本地和演示环境 bootstrap。 |
| 迁移 Auth 到 pgx/v5 | 实现任务 | P1 | 技术基线已统一为 `pgx/v5@v5.7.6` | 更新 `services/auth/go.mod`、repository/import、测试和迁移 smoke。 |

## 10. 最近检查记录

| 日期 | 检查人/工具 | 代码基准 | 结论 |
| --- | --- | --- | --- |
| 2026-06-29 | Codex goal | `eddf917` + working tree | Auth 实现已落地且基本对齐契约；主要剩余是 DB smoke、管理员初始化和 README 状态回写。 |
