# AI Gateway 实现说明

版本：v0.1
日期：2026-06-30
范围：`services/ai-gateway/` 当前实现、契约对齐、缺口和后续实现约束

## 1. 文档定位

本文档描述 `ai-gateway` 当前实现状态和后续实现约束。它只补充服务 README、OpenAPI、架构和技术选型文档，不覆盖这些上游契约。

权威来源：

| 类型 | 权威来源 | 本文档关系 |
| --- | --- | --- |
| 服务公开说明 | `docs/services/ai-gateway/README.md` | 只能补充，不能覆盖 |
| 服务 OpenAPI | `docs/services/ai-gateway/api/openapi.yaml` | 只能跟随，不能另起契约 |
| Gateway 公开契约 | `docs/services/gateway/api/openapi.yaml` | 前端稳定契约以 gateway 为准 |
| 服务边界 | `docs/architecture/service-boundaries.md` | 必须遵守 |
| 技术基线 | `docs/architecture/technology-decisions.md` | 必须跟随 |
| 代码实现 | `services/ai-gateway/` | 本文档记录当前状态和差距 |

凡是本文档与上表文件冲突，以上游文件为准；发现冲突时，在“文档与实现出入”中记录并生成回写或实现任务。

## 2. 当前结论

| 项目 | 状态 | 说明 |
| --- | --- | --- |
| 文档状态 | active | README、数据模型和内部 OpenAPI 存在。 |
| 代码状态 | partial | Go service、PostgreSQL repository、model profile CRUD、credential encryption、service-token auth、chat completions、embeddings、rerankings、provider invocation 记录和 usage aggregate 已实现；provider adapter 已有受控 fake 回归样本和默认跳过的真实 provider smoke 入口，DB smoke 仍缺。 |
| 契约对齐 | partial | `/internal/v1/model-profiles/**`、`/internal/v1/chat/completions`、`/internal/v1/embeddings` 和 `/internal/v1/rerankings` 已按当前 OpenAPI 落地；前端仍不得直接调用本服务。 |
| 数据持久化 | postgres | runtime 使用 PostgreSQL 和 AES-GCM 加密列保存 provider credentials；migrations `0001`-`0006` 覆盖 profile、credential、revision、invocation、attempt 和 usage aggregate。 |
| 测试状态 | partial | config/service/http/middleware/provider tests 覆盖 profile、安全、非流式 chat、流式 chat、embedding、rerank、受控 fake provider 成功路径、脱敏、响应映射和失败记录；真实 provider smoke 有 env-gated 入口但缺实际凭证运行记录，DB smoke 仍缺。 |
| 建议动作 | 集成验证 / 回写文档 | 补真实 provider smoke 运行记录、Knowledge/QA/Document 跨服务接入验证、配额/指标和 profile 运维文档。 |

## 3. 已实现

| 能力 | 代码位置 | 契约来源 | 验证方式 | 备注 |
| --- | --- | --- | --- | --- |
| 健康/就绪检查 | `services/ai-gateway/internal/http/server.go` | AI Gateway OpenAPI | `cd services/ai-gateway && go test ./...` | ready 检查 repo/profile 状态。 |
| model profile CRUD | `internal/http/server.go`、`internal/service/service.go` | AI Gateway OpenAPI / Gateway admin model profiles | HTTP/service tests | 支持 list/create/get/update/delete。 |
| provider credential 加密 | `internal/service/crypto.go` | AI Gateway README | `TestCreateModelProfileDoesNotReturnAPIKey` | 响应只返回 `apiKeyConfigured`。 |
| sensitive default parameter validation | `internal/service/service.go` | AI Gateway README | service tests | 拒绝敏感参数进入 profile defaults。 |
| service-token/caller-service auth | `internal/middleware/auth.go`、`internal/http/server.go` | AI Gateway README | auth/http tests | 要求 token hash 和 `X-Caller-Service`。 |
| OpenAI-compatible chat completions | `internal/http/server.go`、`internal/service/chat.go`、`internal/provider/chat_http.go` | AI Gateway OpenAPI | fake provider HTTP tests | 支持非流式和流式请求，转发 function-calling 字段。 |
| OpenAI-compatible embeddings | `internal/http/server.go`、`internal/service/invocations.go`、`internal/provider/client.go` | AI Gateway OpenAPI | HTTP/service tests；受控 fake provider smoke | 支持 profile model exact-match、dimensions fallback、input count/index 校验和 OpenAI-compatible list 回归样本。 |
| OpenAI-style rerankings | `internal/http/server.go`、`internal/service/invocations.go`、`internal/provider/client.go` | AI Gateway OpenAPI | HTTP/service tests；受控 fake provider smoke | 使用 `/rerank` 兼容路径，校验 index、document_id、score、`data[]`/`results[]` 归一和 topN。 |
| provider invocation 记录 | `migrations/0004`-`0006`、`internal/service/chat.go`、`internal/service/invocations.go`、`internal/repository/postgres.go` | 数据模型 / observability | service/http tests | 记录 provider、model、status、token usage、duration、input count、embedding dimensions、rerank topN 和错误归一摘要，不保存 prompt/API key/input 文本。 |
| usage aggregate | `migrations/0006_create_model_usage_aggregates.sql`、`internal/repository/postgres.go` | 数据模型 / observability | repository path via service tests | 按小时、caller、profile 和 operation 聚合 request/success/failure/token/duration。 |
| PostgreSQL schema/repository | `migrations/0001`-`0006`、`internal/repository/postgres.go` | 数据模型 | service tests / migration CI | runtime 使用 `pgx/v5`。 |

## 4. 未实现

| 缺口 | 文档来源 | 影响范围 | 建议任务 |
| --- | --- | --- | --- |
| 真实 provider smoke 缺少运行记录 | AI Gateway README / provider adapter 约束 | API / observability | 已有 `TestRealProviderSmoke_ExplicitEnvOnly` env-gated 入口；需要有凭证环境时记录 chat、embedding、rerank 实际 provider 结果。 |
| 跨服务接入未闭环 | 服务边界 / Knowledge、QA、Document requirements | Knowledge indexing/retrieval、QA、Document generation | 补 Knowledge embedding/rerank、QA/Document chat 调用和 Gateway admin model profile smoke。 |
| 配额、限流、指标和 tracing 未落地 | 数据模型 / 技术基线 | observability / admin | 后续实现 caller/user/profile 级配额、Prometheus metrics 和 OpenTelemetry tracing。 |
| profile 生命周期运维文档不足 | README / local integration | deploy / ops | 补默认 profile seed、token hash 生成、provider credential rotation 和真实环境 smoke 手册。 |
| provider-specific adapter 差异未展开 | Provider adapter 说明 | 多 provider 兼容性 | 新增 provider 前补特异路径、响应 shape、错误映射和 fake tests。 |

## 5. 文档与实现出入

| 出入点 | 文档要求 | 当前实现 | 风险 | 建议处理 |
| --- | --- | --- | --- | --- |
| README 状态记录 | README 曾称当前文档不落地 `services/ai-gateway/` 代码 | 实际已有 Go module、migrations、repository、profile API；本次已回写 README | 后续若重复写实现状态，容易再次漂移 | README 只链接 implementation，当前状态在本文维护。 |
| Model invocation API | OpenAPI 声明 chat/embeddings/rerankings | 三类 invocation endpoint 均已实现 provider adapter 和调用摘要记录 | 下游服务尚未全部接入，容易把 AI Gateway 可用误读为 Knowledge/QA/Document 闭环已完成 | 在能力矩阵和本地联调文档中继续区分服务内能力与跨服务闭环。 |
| Provider adapter 行为 | README 只描述接口形态 | 代码还包含 model exact-match、embedding count/index 校验、rerank document_id 校验和 usage aggregate | 不写清会导致下游绕过 profile 或误存敏感 payload | 新增 `provider-adapters.md`，本文链接到该细则。 |
| Migration 编号 | 数据模型早期把 usage aggregate 规划为 `0005` | 当前 `0005` 是 invocation attempts，usage aggregate 是 `0006` | reviewer 按旧编号找不到表 | 本次回写 data-models 和 implementation。 |
| Service token 格式 | README 要求 `X-Service-Token`，配置使用 token hashes | 代码读取 `AI_GATEWAY_SERVICE_TOKEN_HASHES`，不接受明文配置 | 安全部署需要先生成 hash | README 保留 hash 格式说明，补生成示例。 |
| pgx version | 技术基线目标为 `pgx/v5@v5.7.6` | AI Gateway 使用 `pgx/v5` | 无直接出入 | 保持随技术基线升级；不得降回 `pgx/v4`。 |

## 6. MVP / mock / memory backend / 占位

| 项目 | 当前用途 | 退出条件 | 关联任务 |
| --- | --- | --- | --- |
| fake provider tests | 验证 provider adapter、错误归一化、成功响应 shape 和脱敏 | 保留测试用；真实 provider smoke 通过 env-gated 入口另行记录 | #287 |
| memory repository in tests | profile、invocation 和 HTTP 单元测试 | 保留测试用；repository 行为由 migration/DB smoke 补充 | 无 |
| OpenAI-compatible shared HTTP adapter | 统一处理 `openai_compatible`、`siliconflow`、`local_compatible` 的兼容路径 | 出现 provider 特异差异时拆分 adapter 并补测试 | provider-specific adapter follow-up |

## 7. 运行与配置

| 项目 | 当前状态 | 缺口 |
| --- | --- | --- |
| 启动命令 | `cd services/ai-gateway && go run ./cmd/server` | 需要 PostgreSQL、token hashes、credential encryption key。 |
| 环境变量 | `AI_GATEWAY_DATABASE_URL`、`AI_GATEWAY_SERVICE_TOKEN_HASHES`、`AI_GATEWAY_CREDENTIAL_ENCRYPTION_KEY_REF`、`AI_GATEWAY_CREDENTIAL_ENCRYPTION_KEY`、timeouts/body size | 需要 provider-specific runtime smoke 和 profile seed。 |
| PostgreSQL / migration | `migrations/0001`-`0006`，`sqlc.yaml`，runtime repository | 需要本地 DB smoke。 |
| Redis / queue | 当前不使用 | 后续配额/熔断按需。 |
| Object storage / vector store / AI provider | provider profiles 已存储；chat、embedding、rerank provider HTTP adapters 已实现；受控 provider 回归样本和 env-gated 真实 provider smoke 入口已存在 | 真实 provider 凭证环境运行记录；Knowledge/QA/Document 接入验证。 |

## 8. 测试与验证

| 验证项 | 命令或步骤 | 当前结果 | 缺口 |
| --- | --- | --- | --- |
| 单元测试 | `cd services/ai-gateway && go test ./...` | pass（本次执行） | 真实 provider smoke 默认跳过。 |
| Provider adapter 受控回归 | `cd services/ai-gateway && go test ./internal/http -run 'Test(ChatSmoke|ChatStreamSmoke|EmbeddingSmoke|RerankSmoke)' -count=1` | pass（本次执行） | 使用 `httptest.Server`，不外联。 |
| 真实 provider smoke | `AI_GATEWAY_REAL_PROVIDER_SMOKE=1 ... go test ./internal/http -run TestRealProviderSmoke_ExplicitEnvOnly -count=1 -v` | available / not run by default | 需要真实 provider base URL、API key 和至少一个模型环境变量。 |
| 集成测试 | goose apply + profile CRUD against DB | missing | 需要 PostgreSQL。 |
| 契约测试 | HTTP tests for profile、chat completion、streaming、embedding、rerank routes | partial | 未从 OpenAPI 自动校验完整 schema。 |
| 手工 smoke | Gateway admin model profile CRUD + chat/embedding/rerank model calls | not run | 需要 gateway/auth/Redis/ai-gateway/provider。 |

## 9. 建议任务

| 任务 | 类型 | 优先级 | 依据 | 说明 |
| --- | --- | --- | --- | --- |
| 记录真实 provider smoke 结果 | 新任务 | P1 | 错误归一和脱敏要求 | 在有凭证环境时运行 env-gated smoke，补实际 provider chat/embedding/rerank 结果和 provider-specific 差异。 |
| 接入 Knowledge embedding/rerank | 新任务 | P0 | Knowledge indexing/retrieval 依赖 | 使用 AI Gateway embedding/rerank endpoint，Knowledge 仍负责 chunk/vector/Qdrant。 |
| 接入 QA/Document 真实模型调用 smoke | 新任务 | P1 | 跨服务联调 | 验证 caller service、request id、profile id 和错误归一化。 |
| 补 profile seed 和 token hash 运行手册 | 回写文档 | P1 | 部署可操作性 | 避免明文 token 配置误用，降低本地 smoke 成本。 |
| 增加配额、指标和 tracing | 新任务 | P2 | observability | 基于 `provider_invocations` 和 `model_usage_aggregates` 扩展。 |

## 10. 最近检查记录

| 日期 | 检查人/工具 | 代码基准 | 结论 |
| --- | --- | --- | --- |
| 2026-06-29 | Codex after #225 rebase | `51045a1` + docs branch | AI Gateway 已实现模型配置、凭据安全、chat、embedding、rerank、调用摘要和 usage aggregate；主要缺口转为真实 provider smoke 与跨服务接入验证。 |
| 2026-06-29 | Codex docs PR | `065b3f4` rebased on `51045a1` | 本文从旧实现状态更新到 #225 后的当前能力，并新增 provider adapter、本地联调、能力矩阵和测试策略文档。 |
| 2026-06-30 | Codex issue #287 | `b02de67` | 增加 embedding/rerank 受控 fake provider 成功回归样本、env-gated 真实 provider smoke 入口和 seed runbook；普通 CI 不依赖外部模型服务。 |
