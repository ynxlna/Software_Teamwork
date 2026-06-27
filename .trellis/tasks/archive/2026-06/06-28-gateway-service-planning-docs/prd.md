# Gateway Service Planning Docs

## Goal

明确 gateway 服务在电力行业知识管理系统中的职责边界，并规划项目初期应产出的文档包，让 frontend、gateway、auth、file、knowledge、qa、document 各方向可以围绕稳定契约并行开发，减少接口等待和职责漂移。

## What I already know

* 用户要求先阅读现有文档，思考 gateway 服务该做哪些事情，以及项目初期应出什么文档方便并行开发。
* 仓库处于架构与工程规范建设阶段，服务目录多数仍是 `.gitkeep` 占位。
* README 将系统定义为以 gateway 为入口的微服务架构：frontend -> gateway -> auth/file/qa/knowledge/document。
* README 已写明 gateway 职责是后端统一入口，负责路由、鉴权上下文传递、聚合接口和跨服务请求协调。
* 后端规范要求每个 Go 服务独立维护 `go.mod`，禁止跨服务 import 其他服务的 `internal/` 包，跨服务通信走 HTTP API 契约。
* 后端规范明确提醒不要把共享业务逻辑放进 `services/gateway/`，gateway 不能演化成业务大单体。
* 错误处理规范要求 gateway 和内部服务使用统一 JSON 错误响应形状，并在跨服务调用时将下游错误转换为调用方自己的错误码。
* 日志规范要求 HTTP middleware 生成/传递 request ID，并在边界日志中包含 service、request_id、operation、dependency、duration 等字段。
* 产品需求包含认证、知识库/文档管理、智能问答 SSE、报告内容流式生成、报告 DOCX 导出、后台配置和统计监控。

## Assumptions (temporary)

* 当前只需要服务规划与文档产出建议，不立即实现 gateway 代码。
* 项目早期更需要 contract-first 文档和服务边界，而不是先把复杂网关能力一次性实现完。
* 初期只有一个 React Web 前端，因此不需要多个 BFF；一个 gateway API 足够。

## Requirements

* 给出 gateway 应负责的事情和明确不应负责的事情。
* 第一批文档采用基础契约包范围，目标是尽快解锁并行开发。
* 文档方案要匹配当前仓库微服务划分和 Trellis 后端规范。
* 方案应帮助前端可基于契约开发，后端各服务可独立实现自己的 API。
* 采用 contract-first thin gateway 方向：先写边界文档、frontend-facing OpenAPI、服务边界矩阵和前后端契约约定，再进入实现。
* gateway 初期保持薄网关，不承接领域业务；仅对跨服务页面级读模型做明确标注的聚合接口。
* 本轮应产出或规划以下文档：
  * `docs/gateway.md`
  * `docs/api/gateway.openapi.yaml`
  * `docs/service-boundaries.md`
  * `docs/frontend-backend-contract.md`
* `docs/api/internal-services.md` 可以作为后续服务契约细化文档，除非实现时发现必须提前落地。

## Acceptance Criteria

* [x] PRD 记录现有文档依据、gateway 职责边界和待决策问题。
* [x] 研究文件记录 API Gateway / OpenAPI / BFF 相关模式，并映射到本仓库。
* [x] 向用户提出 2-3 个可选方案，并给出推荐方案。
* [x] 用户确认采用 contract-first thin gateway。
* [x] 用户确认 MVP 第一批文档采用基础契约包。
* [x] PRD 收敛到可执行的文档产出计划。
* [x] 如进入实施阶段，新增/更新基础契约文档并保持 README 中架构描述一致。
* [x] 如进入实施阶段，文档路径、标题和交叉链接清晰可读。

## Definition of Done (team quality bar)

* Tests added/updated when code changes are introduced.
* Lint / typecheck / CI green when code changes are introduced.
* Docs/notes updated if behavior or architecture changes.
* Rollout/rollback considered if risky.

## Out of Scope (explicit)

* 本轮不默认实现 `services/gateway/` 代码。
* 本轮不选择具体 Go router/middleware 库，除非后续决定进入实现阶段。
* 本轮不设计所有业务接口的完整字段细节，只先定义 OpenAPI 骨架、公共 envelope 和主要资源路径。
* 本轮不展开完整失败策略包；超时、错误翻译、SSE 断线、上传限制、鉴权失败只在基础文档中保留占位或简短原则。
* 本轮不展开完整架构包；版本化、限流、审计、多端/BFF 预留和 ADR 放到后续任务。

## Research References

* [`research/gateway-patterns-and-contracts.md`](research/gateway-patterns-and-contracts.md) — API Gateway 常见职责是 routing、aggregation、offloading；项目初期建议采用 OpenAPI 契约优先。

## Research Notes

### What similar tools do

* API Gateway 通常作为客户端统一入口，做请求路由、跨服务聚合和横切能力集中处理。
* OpenAPI 可作为 HTTP API 的机器可读契约，便于 frontend、backend、mock、测试和文档并行推进。
* BFF 适合多端差异明显的场景；当前只有 Web 前端，不应过早引入多个 BFF。

### Constraints from our repo/project

* 微服务按 `services/<service>/` 独立 Go module 组织。
* gateway 只能通过 HTTP 调用内部服务，不能 import 其他服务内部包。
* gateway 和内部服务需要一致的 JSON 错误响应形状。
* gateway 是最适合统一 CORS、request ID、认证上下文传递、超时和依赖错误翻译的位置。

### Feasible approaches here

**Approach A: Contract-first thin gateway** (Recommended)

* How it works: 先产出 `docs/gateway.md`、`docs/api/gateway.openapi.yaml`、服务边界矩阵和前后端契约约定，再由各服务按契约并行实现。
* Pros: 最利于并行开发，能控制 gateway 边界，减少接口猜测。
* Cons: 需要维护契约与实现同步，早期字段可能迭代。

**Approach B: Feature-slice gateway docs**

* How it works: 按登录、知识库、文件、问答、报告等功能切片分别定义 gateway API、下游服务和数据流。
* Pros: 更贴近页面和业务工作流，方便功能小组阅读。
* Cons: 横切规则容易散落重复，需要额外维护统一政策文档。

**Approach C: Implementation-first gateway skeleton**

* How it works: 先搭建 `services/gateway/` 代码骨架、health/config/router，再从代码反推文档。
* Pros: 很快能跑通工程链路。
* Cons: 项目初期对并行开发帮助较弱，容易提前固化不成熟接口。

## Decision (ADR-lite)

**Context**: gateway 是 frontend 访问后端能力的统一入口，但当前仓库仍处于架构和规范建设阶段，各微服务目录尚未实现完整代码。若先写 gateway 代码，容易在领域服务契约未定前固化接口；若只按业务功能分散写文档，横切规则容易重复和漂移。

**Decision**: 采用 contract-first thin gateway。第一阶段先产出 gateway 边界文档、frontend-facing OpenAPI、服务边界矩阵、内部服务契约索引和前后端集成约定；gateway 只承接路由、认证上下文传递、错误/日志/超时/SSE 等横切能力，以及明确标注的前端聚合接口。

**Consequences**: 前端可以先基于 OpenAPI/mock 开发，后端服务可以独立实现各自 API；代价是需要维护契约与实现同步，并在字段细节随业务发现变化时更新文档。

## Initial Gateway Boundary Draft

### Gateway should own

* 面向 frontend 的 public HTTP API surface 和版本化路径。
* 路由到 auth/file/knowledge/qa/document 等内部服务。
* 用户认证检查入口，以及 user/role/permission/request_id 等上下文向下游传递。
* 前端友好的聚合接口，尤其是跨多个服务才能完成的页面级读模型。
* 统一错误响应 envelope、下游错误翻译和依赖失败处理。
* CORS、请求体大小限制、超时、有限重试、熔断/降级策略、访问日志。
* SSE/流式响应的代理与连接生命周期策略。
* `/healthz`、`/readyz` 等运行状态接口。

### Gateway should not own

* 用户、密码、会话、角色权限数据的持久化；这属于 auth。
* 文件元数据、MinIO object key、解析/切片/向量化流程；这属于 file/knowledge。
* RAG、意图识别、检索、重排序、LLM 调用；这属于 qa/knowledge。
* 报告大纲、内容生成、DOCX 导出业务流程；这属于 document。
* 其他服务数据库 migration。
* 可被放到某个领域服务内的业务规则。

## Initial Document Package Draft

* `docs/gateway.md` — gateway 职责、非职责、路由表、认证上下文、错误策略、SSE 策略、超时/重试/日志策略。
* `docs/api/gateway.openapi.yaml` — frontend-facing API 契约，包括 DTO、错误 envelope、认证要求、分页/过滤、上传限制、SSE endpoint。
* `docs/service-boundaries.md` — gateway/auth/file/knowledge/qa/document 职责矩阵，避免业务逻辑漂移。
* `docs/frontend-backend-contract.md` — 前端集成约定，包括 base path、认证流、错误处理、分页/filter、loading/retry、SSE 断线行为。
* Later optional: `docs/api/internal-services.md` — 下游服务 API 索引、服务 owner、base URL 环境变量、gateway 传递的 headers、依赖关系。
* Later optional: `deploy/.env.example` — gateway 下游服务 URL、端口、运行模式、CORS origin 等非 secret 变量名。
* Later optional: `docs/adr/0001-gateway-boundary.md` — 团队确认 gateway 风格后记录 ADR-lite。

## Technical Approach

采用 contract-first thin gateway：

* 先用 `docs/gateway.md` 定义 gateway 的公共职责、禁止承接的业务职责、路由分组和上下文传递规则。
* 用 `docs/api/gateway.openapi.yaml` 提供前端可对接的 API 骨架，至少覆盖认证、知识库、文档、问答、报告和后台统计的主要路径分组、通用响应 envelope、错误 envelope、分页结构和 SSE endpoint 标记。
* 用 `docs/service-boundaries.md` 建立服务职责矩阵，明确每个资源和工作流归属哪个服务。
* 用 `docs/frontend-backend-contract.md` 固化前端调用约定，避免各页面独立发明错误处理、分页、过滤、认证和流式请求约定。

## Implementation Plan (small PRs)

* PR1: Gateway boundary docs
  * Add `docs/gateway.md`.
  * Add `docs/service-boundaries.md`.
  * Cross-link from README or existing docs if appropriate.
* PR2: Frontend-facing API contract
  * Add `docs/api/gateway.openapi.yaml`.
  * Define common schemas, error envelope, pagination envelope, auth requirement, and major endpoint groups.
* PR3: Frontend-backend integration conventions
  * Add `docs/frontend-backend-contract.md`.
  * Align naming/path conventions with OpenAPI.
  * Leave advanced failure strategy and deployment env docs as explicit follow-up.

## Technical Notes

* Inspected `README.md`, `docs/system.md`, `docs/knowledge_management_system.md`, `docs/smart_quote_system.md`, `docs/report_generation_system.md`.
* Inspected `.trellis/spec/backend/index.md`, `directory-structure.md`, `error-handling.md`, `logging-guidelines.md`, `quality-guidelines.md`, and `.trellis/spec/cicd.md`.
* Current service directories under `services/` contain placeholders only, so this is a planning/documentation task rather than a code modification task.
