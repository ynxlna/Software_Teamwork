# Auth 服务接口文档

本文档定义 `auth` 服务在项目初期的职责边界和接口契约。当前仓库尚未落地 `services/auth/` 代码，因此本文档以现有 gateway OpenAPI、服务边界矩阵和前后端集成契约为准，用于指导后续 auth 服务实现与联调。

详细的前端公开路径以 [`docs/api/gateway.openapi.yaml`](api/gateway.openapi.yaml) 为准。前端不得直接调用 auth 服务内部地址，只能通过 gateway 暴露的 `/api/v1/**` 入口访问认证能力。公开和内部 HTTP API 都必须使用 RESTful 资源路径，用户创建使用 `users` 资源，会话创建、查询和删除使用 `sessions` 资源。

## 职责边界

| 范围 | 说明 |
| --- | --- |
| 用户身份 | 维护用户账号、用户 ID、用户名和用户基础状态。 |
| 凭证校验 | 负责用户创建、会话创建、密码校验和凭证安全策略。 |
| 会话 / 令牌 | 负责签发、校验、撤销 token 或 session，并返回 gateway 可缓存的会话身份。 |
| 角色权限 | 维护用户角色、权限集合，并为 gateway 提供会话上下文。 |
| 当前用户 | 根据认证凭据返回当前用户资料。 |
| 安全事件 | 记录会话创建失败、会话删除、令牌撤销等安全相关事件，不能记录明文密码或 token。 |

`auth` 不负责文件、知识库、问答、报告生成等业务资源，也不直接暴露给前端。gateway 负责公开 API 路由、统一响应 envelope、request id 和错误响应归一化。

## 接入模型

```text
frontend
   |
   v
gateway /api/v1/users, /api/v1/sessions, /api/v1/sessions/current, /api/v1/users/me
   |
   v
auth service
```

前端侧调用 gateway 公开接口；gateway 将认证请求转发给 auth。认证成功后，auth 返回完整会话身份，gateway 写入 Redis 会话缓存。后续业务请求优先由 gateway 从 Redis 获取身份上下文，再向其他下游服务注入认证 header。

Gateway 调用下游服务时应传递：

| Header | 说明 |
| --- | --- |
| `X-Request-Id` | 贯穿一次前端请求的 request id。 |
| `X-User-Id` | 已认证用户 ID。 |
| `X-User-Roles` | 逗号分隔的角色列表。 |
| `X-User-Permissions` | 逗号分隔的权限列表。 |
| `X-Forwarded-For` | 原始客户端地址链。 |
| `X-Forwarded-Proto` | 原始协议。 |

前端不得设置 `X-User-Id`、`X-User-Roles`、`X-User-Permissions`；这些字段只能由 gateway 在认证后注入。

## 公开接口总览

| Method | Gateway Path | Auth | Owner | 说明 |
| --- | --- | --- | --- | --- |
| `POST` | `/api/v1/users` | 不需要 | `auth` | 创建新用户并返回会话。 |
| `POST` | `/api/v1/sessions` | 不需要 | `auth` | 使用用户名和密码创建会话。 |
| `DELETE` | `/api/v1/sessions/current` | 需要 | `auth` | 删除当前登录会话。 |
| `GET` | `/api/v1/users/me` | 需要 | `auth` | 获取当前用户资料。 |

认证机制当前按公开契约使用 `bearerAuth`，即 `Authorization: Bearer <accessToken>`。Auth 服务签发 `sessionId` 和 `accessToken`，gateway 将 auth 返回的会话身份写入 Redis。后续请求由 gateway 基于 Redis 会话缓存完成身份读取和上下文注入。

## 通用响应结构

成功响应遵循 gateway 统一 envelope：

```json
{
  "data": {},
  "requestId": "req_123"
}
```

错误响应固定为：

```json
{
  "error": {
    "code": "validation_error",
    "message": "request validation failed",
    "requestId": "req_123",
    "fields": {
      "username": "is required"
    }
  }
}
```

前端和调用方应优先匹配 `error.code`，不要解析 `message` 文案。

## 会话缓存协作模型

Auth 服务是用户、角色、权限和会话签发的源服务；Gateway 是运行时会话缓存的使用方。二者协作方式如下：

1. Gateway 接收前端用户创建或会话创建请求。
2. Gateway 调用 auth 服务内部接口。
3. Auth 校验凭证或创建用户后，返回 `UserSummary` 和 `SessionSummary`。
4. Gateway 将完整会话身份写入 Redis，缓存键使用 `gateway:session:<accessTokenHash>`。
5. 前端后续请求携带 `Authorization: Bearer <accessToken>`。
6. Gateway 用 token 派生 hash 查询 Redis，会话命中后向下游服务注入用户、角色、权限。
7. 当前会话删除、账号禁用、权限变更或安全事件发生时，auth 负责让会话失效，gateway 负责删除 Redis 中对应缓存。

Auth 返回给 gateway 的会话身份必须足以构造 Redis 缓存值：

| 字段 | 来源 | Gateway 用途 |
| --- | --- | --- |
| `session.sessionId` | auth 会话记录 | 关联撤销、审计和问题排查。 |
| `session.accessToken` | auth token 签发逻辑 | 返回给前端；gateway 对其做 hash 后作为 Redis key。 |
| `session.expiresAt` | auth 会话策略 | 设置 Redis TTL 和过期判断。 |
| `user.id` | auth 用户记录 | 写入 `X-User-Id`。 |
| `user.username` | auth 用户记录 | 用于审计和展示。 |
| `user.roles` | auth 角色关系 | 写入 `X-User-Roles`。 |
| `user.permissions` | auth 权限计算 | 写入 `X-User-Permissions`。 |

Redis 不是 auth 的持久化数据库。Auth 仍需在自己的 PostgreSQL 或其他持久化存储中维护用户、角色、权限、会话元数据和撤销状态。Gateway Redis 缓存只用于减少后续请求对 auth 的重复查询。

## 数据结构

### CreateUserRequest

```json
{
  "username": "alice",
  "password": "password"
}
```

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `username` | `string` | 是 | 登录用户名。 |
| `password` | `string` | 是 | 用户密码。请求、响应和日志中不得记录明文密码。 |

### CreateSessionRequest

```json
{
  "username": "alice",
  "password": "password"
}
```

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `username` | `string` | 是 | 登录用户名。 |
| `password` | `string` | 是 | 用户密码。请求、响应和日志中不得记录明文密码。 |

### UserSummary

```json
{
  "id": "usr_123",
  "username": "alice",
  "roles": ["admin"],
  "permissions": ["knowledge:read", "document:upload"]
}
```

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `id` | `string` | 是 | 用户公开 ID。 |
| `username` | `string` | 是 | 用户名。 |
| `roles` | `string[]` | 是 | 用户角色列表。 |
| `permissions` | `string[]` | 是 | 用户权限字符串列表，用于 gateway 写入 `X-User-Permissions`。 |

### SessionSummary

```json
{
  "sessionId": "sess_123",
  "accessToken": "eyJ...",
  "tokenType": "Bearer",
  "expiresAt": "2026-06-28T12:00:00Z"
}
```

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `sessionId` | `string` | 是 | Auth 服务签发的会话 ID。 |
| `accessToken` | `string` | 是 | 前端后续请求使用的 bearer 凭据。Gateway 写入 Redis 时只能派生哈希作为 key 或字段，不能把原始 token 写入日志。 |
| `tokenType` | `string` | 是 | 当前固定为 `Bearer`。 |
| `expiresAt` | `string(date-time)` | 是 | 会话过期时间。Gateway Redis TTL 必须与该时间对齐。 |

### SessionResponse

```json
{
  "data": {
    "user": {
      "id": "usr_123",
      "username": "alice",
      "roles": ["admin"],
      "permissions": ["knowledge:read", "document:upload"]
    },
    "session": {
      "sessionId": "sess_123",
      "accessToken": "eyJ...",
      "tokenType": "Bearer",
      "expiresAt": "2026-06-28T12:00:00Z"
    }
  },
  "requestId": "req_123"
}
```

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `data.user` | `UserSummary` | 是 | 当前认证用户。 |
| `data.session` | `SessionSummary` | 是 | 当前登录会话。Gateway 必须用它写入 Redis 会话缓存。 |
| `requestId` | `string` | 是 | 请求追踪 ID。 |

### UserResponse

```json
{
  "data": {
    "id": "usr_123",
    "username": "alice",
    "roles": ["admin"],
    "permissions": ["knowledge:read", "document:upload"]
  },
  "requestId": "req_123"
}
```

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `data` | `UserSummary` | 是 | 当前用户资料。 |
| `requestId` | `string` | 是 | 请求追踪 ID。 |

## Endpoint 详情

### POST /api/v1/users

创建新用户并返回会话。该接口不要求认证。

**Request**

```http
POST /api/v1/users
Content-Type: application/json
```

```json
{
  "username": "alice",
  "password": "password"
}
```

**Success**

| Status | Body |
| --- | --- |
| `201 Created` | `SessionResponse` |

**Error**

当前 OpenAPI 已声明：

| Status | Code | 场景 |
| --- | --- | --- |
| `400` | `validation_error` | 请求体缺失、字段类型错误、用户名或密码不满足规则。 |

后续实现可补充：

| Status | Code | 场景 |
| --- | --- | --- |
| `409` | `conflict` | 用户名已存在等状态冲突。 |
| `429` | `rate_limited` | 注册频率限制。 |

上述补充状态码加入 OpenAPI 前，前端不得作为公开契约依赖。非预期服务端错误仍按项目统一错误响应处理为 `internal_error`。

### POST /api/v1/sessions

使用用户名和密码创建会话。该接口不要求认证。

**Request**

```http
POST /api/v1/sessions
Content-Type: application/json
```

```json
{
  "username": "alice",
  "password": "password"
}
```

**Success**

| Status | Body |
| --- | --- |
| `200 OK` | `SessionResponse` |

`data.session.accessToken` 返回访问令牌，前端后续请求使用：

```http
Authorization: Bearer <accessToken>
```

会话创建成功后，gateway 必须把 `data.user` 与 `data.session` 一起写入 Redis。

**Error**

当前 OpenAPI 已声明：

| Status | Code | 场景 |
| --- | --- | --- |
| `400` | `validation_error` | 请求体缺失、字段类型错误、用户名或密码为空。 |
| `401` | `unauthorized` | 用户名或密码错误、账号不可用、凭证无效。 |

后续实现可补充：

| Status | Code | 场景 |
| --- | --- | --- |
| `429` | `rate_limited` | 会话创建失败频率限制。 |

上述补充状态码加入 OpenAPI 前，前端不得作为公开契约依赖。非预期服务端错误仍按项目统一错误响应处理为 `internal_error`。

会话创建失败响应不得区分“用户名不存在”和“密码错误”，避免泄露账号枚举信息。

### DELETE /api/v1/sessions/current

删除当前登录会话。该接口要求认证。

**Request**

```http
DELETE /api/v1/sessions/current
Authorization: Bearer <accessToken>
```

**Success**

| Status | Body |
| --- | --- |
| `204 No Content` | 无响应体。 |

**Error**

当前 OpenAPI 已声明：

| Status | Code | 场景 |
| --- | --- | --- |
| `401` | `unauthorized` | 缺少认证凭据、token 无效或登录态已失效。 |

非预期服务端错误仍按项目统一错误响应处理为 `internal_error`。

采用 token 机制时，auth 服务应明确 token 撤销策略，例如服务端 denylist、短期 access token 配合 refresh token，或其他可审计方案。当前会话删除成功后，gateway 必须删除 Redis 中的对应会话缓存。

### GET /api/v1/users/me

获取当前用户资料。该接口要求认证。

**Request**

```http
GET /api/v1/users/me
Authorization: Bearer <accessToken>
```

**Success**

| Status | Body |
| --- | --- |
| `200 OK` | `UserResponse` |

**Error**

当前 OpenAPI 已声明：

| Status | Code | 场景 |
| --- | --- | --- |
| `401` | `unauthorized` | 缺少认证凭据、token 无效或登录态已失效。 |

非预期服务端错误仍按项目统一错误响应处理为 `internal_error`。

## 权限与上下文输出

Auth 服务需要为 gateway 提供足够的信息，用于构造下游服务的认证上下文：

| 输出 | 来源 | 用途 |
| --- | --- | --- |
| `user.id` | 用户身份记录 | 写入 `X-User-Id`。 |
| `user.roles` | 用户角色关系 | 写入 `X-User-Roles`，也可用于 gateway 粗粒度路由保护。 |
| `permissions` | 角色权限映射或用户权限策略 | 写入 `X-User-Permissions`，字段细节后续由 auth 实现契约细化。 |

下游服务仍需在自己的边界做权限校验，不能只依赖前端传参。Gateway 可以做认证和基础路由保护，但不应持久化用户、角色或权限数据。

## 内部 Auth Service 接口初稿

公开契约由 gateway OpenAPI 决定。后续落地 `services/auth/` 时，gateway 可通过内部 HTTP API 与 auth 服务协作。内部 API 也应使用统一 JSON error shape，并保留 `X-Request-Id`。

| Method | Auth Service Path | 说明 |
| --- | --- | --- |
| `GET` | `/healthz` | auth 进程存活检查。 |
| `GET` | `/readyz` | auth 就绪检查，应覆盖 PostgreSQL 等关键依赖。 |
| `POST` | `/internal/v1/users` | 创建用户，返回用户身份和会话身份。 |
| `POST` | `/internal/v1/sessions` | 校验用户名密码，返回用户身份和会话身份。 |
| `DELETE` | `/internal/v1/sessions/current` | 删除当前会话。 |
| `GET` | `/internal/v1/sessions/{sessionId}` | 查询会话身份，用于缓存修复或调试，不作为 gateway 每次请求的默认路径。 |
| `DELETE` | `/internal/v1/sessions/{sessionId}` | 删除指定会话，用于账号禁用、权限变更或安全事件。 |

### 内部 User/Session 成功响应

Auth 服务返回给 gateway 的成功响应应与公开 `SessionResponse` 的 `data` 对齐：

```json
{
  "data": {
    "user": {
      "id": "usr_123",
      "username": "alice",
      "roles": ["admin"],
      "permissions": ["knowledge:read", "document:upload"]
    },
    "session": {
      "sessionId": "sess_123",
      "accessToken": "eyJ...",
      "tokenType": "Bearer",
      "expiresAt": "2026-06-28T12:00:00Z"
    }
  },
  "requestId": "req_123"
}
```

Gateway 收到该响应后负责：

- 将 `session.accessToken` 返回给前端。
- 使用访问令牌派生不可逆 hash，写入 Redis key `gateway:session:<accessTokenHash>`。
- 将 `user.id`、`user.username`、`user.roles`、`user.permissions`、`session.sessionId`、`session.expiresAt` 缓存为会话值。
- 将 Redis TTL 设置到不晚于 `session.expiresAt`。

Auth 服务不得要求 gateway 在后续每个业务请求中回源查询 auth；Redis 会话命中是 gateway 的默认认证路径。

## 错误码约定

Auth 相关接口使用项目统一错误码：

| Code | HTTP status | Auth 场景 |
| --- | --- | --- |
| `validation_error` | `400` | 请求体格式错误、必填字段缺失、字段不满足规则。 |
| `unauthorized` | `401` | 未登录、凭证无效、登录失败、认证过期。 |
| `forbidden` | `403` | 已认证但缺少访问某能力的权限。当前四个 auth 公开接口暂未定义 `403`。 |
| `not_found` | `404` | 当前四个 auth 公开接口暂未定义；用户隐藏策略不应泄露账号存在性。 |
| `conflict` | `409` | 用户名已存在等状态冲突，需先补充 OpenAPI 后作为公开契约。 |
| `rate_limited` | `429` | 注册、登录、验证码等频率限制，需先补充 OpenAPI 后作为公开契约。 |
| `dependency_error` | `502` | auth 依赖数据库、Redis 等基础设施失败并由 gateway 归一化。 |
| `internal_error` | `500` | 未预期服务端错误。 |

## 安全与日志要求

- 不得在日志、错误响应或追踪字段中记录明文密码、token、API key、数据库连接串或 session secret。
- 登录失败可以记录安全事件，但日志只应包含 `service`、`request_id`、`operation`、`status`、必要的用户标识或风险标签，不记录密码。
- 密码存储必须使用安全哈希算法；具体算法和参数在 auth 服务实现设计中单独确定。
- 认证失败信息应保持稳定且模糊，不暴露账号是否存在。
- 所有跨服务 HTTP client 后续实现必须设置超时，并传递 `context.Context`。

## 后续实现建议

后续落地 `services/auth/` 时，建议服务本地维护：

```text
services/auth/
├── api/
│   └── openapi.yaml
├── cmd/server/
├── internal/
│   ├── config/
│   ├── http/
│   ├── service/
│   ├── repository/
│   └── platform/
├── migrations/
└── README.md
```

实现前需要补齐或确认：

- 用户名、密码长度和复杂度规则。
- token/session 机制、过期时间、刷新策略和撤销策略。
- 角色与权限模型，以及 `X-User-Permissions` 的权限字符串命名规则。
- 注册是否开放、是否需要管理员创建用户、是否需要验证码或邀请机制。
- 用户状态模型，例如 active、disabled、locked。
- 登录失败限流、账号锁定和审计事件保留策略。

如果上述决策影响公开字段、错误码或状态码，必须同步更新：

- [`docs/api/gateway.openapi.yaml`](api/gateway.openapi.yaml)
- [`docs/frontend-backend-contract.md`](frontend-backend-contract.md)
- [`docs/service-boundaries.md`](service-boundaries.md)
- 本文档
