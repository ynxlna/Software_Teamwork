# 前后端集成契约

本文档定义 frontend 与 gateway 的基础集成约定。详细 endpoint 以 [`docs/api/gateway.openapi.yaml`](../api/gateway.openapi.yaml) 为准。

## API 入口

前端只调用 gateway：

```text
/api/v1
```

前端不得直接调用 `auth`、`file`、`knowledge`、`qa`、`document` 的内部地址。内部服务地址只应存在于 gateway 或部署配置中。

## OpenAPI 作为协作源

- `docs/api/gateway.openapi.yaml` 是前端与 gateway 的第一版契约源。
- 前端可以基于 OpenAPI 生成类型、mock server 或手写 API client。
- 后端实现 endpoint 前，应先更新 OpenAPI。
- 破坏性字段变更必须同步更新 OpenAPI 和本契约文档。
- 所有前端到 gateway、gateway 到下游服务的 HTTP API 必须使用 RESTful 资源路径，由 HTTP method 表达动作；健康检查是唯一已允许的非 `/api/v1` 例外。
- 本轮把 gateway 健康检查、auth、file-owned 接口和 knowledge-owned 知识库/文档处理/切片/检索接口列为已确定契约；`qa`、`document` 和管理后台聚合接口暂缺，见 OpenAPI 顶层 `x-missing-contracts`。

## 认证约定

- 用户创建和会话创建接口不要求认证。
- 业务接口默认要求认证，OpenAPI 中使用 `bearerAuth` 标记。
- 用户创建或会话创建成功后，前端从响应的 `data.session.accessToken` 读取访问令牌。
- 前端后续请求使用 `Authorization: Bearer <accessToken>`。
- 前端只发送认证凭据，不发送 `X-User-Id`、`X-User-Roles`、`X-User-Permissions`。
- 用户身份、角色和权限由 gateway 从 Redis 会话缓存读取后传递给下游服务。
- Redis 会话缓存由 gateway 在 auth 返回身份/会话信息后写入；前端不直接访问 Redis 或 auth 内部地址。
- `401 unauthorized` 表示未登录或认证失效；前端应回到登录流程。
- `403 forbidden` 表示已登录但权限不足；前端应展示权限不足状态。

## 请求约定

| 项目 | 约定 |
| --- | --- |
| JSON request | `Content-Type: application/json` |
| JSON response | `Content-Type: application/json` |
| File upload | `multipart/form-data` |
| Streaming response | `text/event-stream` |
| Timestamp | RFC 3339 / OpenAPI `date-time` |
| ID | Public API 使用 string ID |
| Page index | `page` 从 1 开始 |
| Page size | `pageSize`，默认值和上限后续由 endpoint 细化 |

## 成功响应

单资源响应：

```json
{
  "data": {
    "id": "kb_123"
  },
  "requestId": "req_123"
}
```

列表响应：

```json
{
  "data": [],
  "page": {
    "page": 1,
    "pageSize": 20,
    "total": 0
  },
  "requestId": "req_123"
}
```

前端应从 `data` 读取业务数据，不依赖响应中的内部服务字段。

## 错误响应

错误响应固定为：

```json
{
  "error": {
    "code": "validation_error",
    "message": "request validation failed",
    "requestId": "req_123",
    "fields": {
      "name": "is required"
    }
  }
}
```

前端逻辑应优先匹配 `error.code`，不要解析 `message` 文案。

| Code | Frontend behavior |
| --- | --- |
| `validation_error` | 显示字段错误或表单级错误。 |
| `unauthorized` | 清理本地登录态并进入登录流程。 |
| `forbidden` | 展示权限不足。 |
| `not_found` | 展示资源不存在或已删除。 |
| `conflict` | 展示状态冲突并刷新当前数据。 |
| `rate_limited` | 展示稍后重试。 |
| `dependency_error` | 展示服务暂不可用。 |
| `internal_error` | 展示通用系统错误。 |

## 分页、过滤和查询

分页、过滤和查询属于下游服务契约的一部分。Knowledge 相关列表和检索参数已经进入 OpenAPI；其他下游接口后续补齐时优先使用以下约定：

```text
?page=1&pageSize=20&keyword=xxx&status=ready
```

约定：

- `keyword` 表示模糊查询关键词。
- 多值过滤可使用逗号分隔字符串，具体字段由 OpenAPI endpoint 定义。
- 排序参数后续统一为 `sort`，例如 `sort=-createdAt`，本轮只保留扩展空间。
- 在对应 OpenAPI path 补齐前，前端不得依赖聊天、报告或管理后台聚合接口。

## Knowledge 接口

知识库管理、文档处理状态、切片详情和知识检索已经进入 gateway OpenAPI。前端应只调用 gateway 暴露的以下资源路径，不能直接调用 `services/knowledge`：

| Method | Path | 说明 |
| --- | --- | --- |
| `GET/POST` | `/api/v1/knowledge-bases` | 分页查询知识库、创建知识库。 |
| `GET/PATCH/DELETE` | `/api/v1/knowledge-bases/{knowledgeBaseId}` | 查询、更新、删除知识库。 |
| `GET` | `/api/v1/knowledge-bases/{knowledgeBaseId}/documents` | 查询知识库内文档处理状态列表。 |
| `GET` | `/api/v1/documents/{documentId}` | 查询文档处理详情。 |
| `GET` | `/api/v1/documents/{documentId}/chunks` | 查询文档切片详情。 |
| `POST` | `/api/v1/knowledge-queries` | 创建一次知识检索查询并返回召回结果。 |

检索使用 `knowledge-queries` 资源，不使用 `/search`、`/retrieval/search` 或其他动作路径。返回字段、分页结构和错误响应以 [`docs/api/gateway.openapi.yaml`](../api/gateway.openapi.yaml) 为准。

## SSE 与流式 UI

问答和报告生成的流式接口暂缺，当前 OpenAPI 不提供稳定 SSE endpoint。后续补齐时，前端处理原则如下：

- 根据 `Content-Type: text/event-stream` 进入流式读取。
- `message` 事件用于文本增量。
- `progress` 事件用于阶段或百分比。
- `citation` 事件用于问答引用。
- `done` 事件表示完成。
- `error` 事件表示本次流式任务失败。

断线重连、幂等恢复和任务恢复 ID 后续单独细化；在 OpenAPI 补齐前，前端不应实现依赖 gateway SSE 路径的稳定调用。

## 文件上传与内容读取

- 上传使用 `multipart/form-data`。
- 上传 endpoint 由 gateway 暴露，实际文件对象归 `file` 服务管理。
- 文档处理状态、知识库列表、切片详情和知识检索归 `knowledge` 服务并已进入 gateway OpenAPI；当前原始文件上传和原文件内容读取仍由 `file` 服务拥有。
- 前端读取原文件内容时，只使用 gateway 提供的 `GET /api/v1/documents/{documentId}/content`。
- 生成报告和报告文件内容接口暂缺，后续由 `document` 契约补齐。
- 前端不得依赖 MinIO object key 或内部存储路径。

## Request ID

- 前端可以在请求头中传递 `X-Request-Id`，不传时由 gateway 生成。
- Gateway 应在响应头和响应体中返回 request id。
- 用户反馈问题时，前端可展示或复制 request id 便于排查。

## Mock 与并行开发

并行开发时：

- 前端以 OpenAPI 中已存在的 active paths 为准，不等待所有内部服务完成。
- OpenAPI `x-missing-contracts` 中列出的范围只能作为待办，不应生成可调用 API client 方法。
- 各后端服务以 gateway OpenAPI 和服务边界矩阵确认自己需要提供的能力。
- 如果实现发现契约不合理，先更新 OpenAPI 和相关文档，再改代码。
