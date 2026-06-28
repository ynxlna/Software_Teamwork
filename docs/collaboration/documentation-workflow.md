# 文档维护工作流

本文定义 `docs/` 下文档的归属规则，避免把跨服务标准重复写进单个服务细则。

## 文档归属

| 内容类型 | 权威位置 | 服务文档中应如何处理 |
| --- | --- | --- |
| 服务职责、数据归属、公开能力边界 | `docs/architecture/service-boundaries.md` | 只写本服务负责/不负责的细化说明，并链接边界矩阵。 |
| 技术栈、数据库、迁移、日志、队列、测试和观测基线 | `docs/architecture/technology-decisions.md` | 只记录本服务的特殊约束或明确偏离原因，不重复完整技术栈表。 |
| RESTful 路径、OpenAPI、统一响应、分页、错误、SSE、上传和 request id | `docs/architecture/frontend-backend-contract.md` | 只列本服务资源路径、业务语义、状态枚举和服务特有错误场景。 |
| 分支、PR、提交、CI 和维护者设置 | `CONTRIBUTING.md`、`docs/collaboration/*.md` | 服务文档不得定义仓库级流程。需要补充时更新协作文档。 |
| 机器可读 API 契约 | `docs/services/gateway/api/openapi.yaml` 或对应内部服务 `api/openapi.yaml` | Markdown 只解释业务语义，不替代 OpenAPI schema。 |
| 服务内数据模型、流程、实现拆解 | `docs/services/<service>/README.md` 和 `docs/services/<service>/docs/*.md` | 保留服务特有内容，避免复制跨服务规则。 |

## 更新顺序

新增或修改公开接口时：

1. 先确认归属服务和边界，必要时更新 `docs/architecture/service-boundaries.md`。
2. 更新 `docs/services/gateway/api/openapi.yaml`，公开字段、状态码和错误码以 OpenAPI 为准。
3. 如果改动影响通用调用规则，更新 `docs/architecture/frontend-backend-contract.md`。
4. 更新对应服务文档，只补充服务业务语义、状态枚举、工作流和实现注意事项。
5. 如果涉及协作、CI、分支或提交要求，更新 `docs/collaboration/` 或 `CONTRIBUTING.md`。

新增或修改内部服务接口时：

1. 更新对应服务的 `api/openapi.yaml`。
2. 如果内部能力会影响公开 API、前端生成类型或服务边界，同步更新 gateway OpenAPI、架构契约和服务边界矩阵。
3. 服务 README 只记录内部接口的业务用途和调用边界，不复制项目统一 envelope、日志或技术栈基线。

## 服务文档检查清单

提交服务文档前，检查：

- 没有在服务 README 中重新定义仓库级分支、PR、提交或 CI 流程。
- 没有在服务 README 中重复完整技术选型表；仅保留服务特有约束。
- 统一响应、分页、错误 envelope、request id、SSE 和上传规则链接到前后端集成契约。
- RESTful 命名规则链接到前后端集成契约或服务边界矩阵；服务文档只给出本服务资源映射。
- Markdown 中的字段和状态没有与 OpenAPI 冲突；如冲突，以 OpenAPI 为准并修正文档。
