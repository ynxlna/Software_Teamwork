# Knowledge Go 微服务实施计划

## Ordered Checklist

1. Task 1: 建立 Go 服务骨架。
   - [x] 创建 `services/knowledge/go.mod`。
   - [x] 添加 `cmd/server/main.go`、config、HTTP router、health/readiness handlers。
   - [x] 添加 README、Dockerfile、`api/openapi.yaml` baseline。
   - [x] 保留 Python 原型作为迁移参考，并更新实现说明，避免它继续被误认为正式服务。

2. Task 2: 元数据和 DTO。
   - 设计 KnowledgeBase、KnowledgeDocument、DocumentChunk、ProcessingJob domain model。
   - 添加 repository port 和初期 memory/PostgreSQL 实现策略。
   - 添加 migrations。
   - 实现知识库 CRUD、文档列表/详情、chunk 列表。

3. Task 3: Handoff 和 ingestion job。
   - 定义 File -> Knowledge 内部 handoff request/response。
   - 创建 ingestion job 状态机。
   - 接入 parser/chunk/embed/index pipeline 的第一条可验证链路。

4. Task 4: Retrieval。
   - 封装 Qdrant client port。
   - 实现 `knowledge-queries` use case。
   - 加强返回字段和敏感信息过滤。

5. Task 5: Gateway。
   - 增加 gateway -> knowledge client/proxy。
   - 添加 contract tests，验证 envelope、error code、context headers。

6. Task 6: P1 能力。
   - Reprocessing job。
   - Runtime config。
   - Stats and retrieval testing.

## Validation Commands

每个 Go 服务实现 task 至少运行：

```bash
cd services/knowledge
go test ./...
go build ./cmd/server
```

涉及 gateway 或 file handoff 时，还需要运行对应服务的本地测试：

```bash
cd services/file
go test ./...
```

如涉及前端接入，按项目级 frontend workflow 运行 Bun lint/test/build。

## Risky Files

- `services/knowledge/`：当前已有 Python 原型，迁移时不能静默删除有参考价值的逻辑；需要明确迁移/保留策略。
- `docs/api/gateway.openapi.yaml`：公开契约改动必须谨慎，不能让历史动作式路径重新进入稳定 API。
- `docs/services/knowledge.md`：服务接口文档必须和 gateway OpenAPI 保持一致。

## Rollback Points

- Task 1 只建立 Go skeleton 和文档基线，风险较低。
- 迁移 ingestion/retrieval 前保留 Python 原型可读状态，直到 Go pipeline 覆盖相同核心能力。
- Gateway proxy 上线前先用 service-local tests 和 contract tests 验证字段与错误响应。
