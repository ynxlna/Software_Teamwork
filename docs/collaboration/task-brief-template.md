# 标准任务书模板

本文档是团队公开的 GitHub Issue / Project 任务书模板，供协调人、组长或
AI 生成任务时使用。GitHub UI 中的
[Task Issue 模板](../../.github/ISSUE_TEMPLATE/issue.md) 是快速入口；需要派发正式任务、
补齐依赖关系或批量生成任务时，以本文档的完整任务书结构为准。任务创建、Project
同步和 View 归属流程见 [任务 Issue 与 Project 流程](task-issue-project-workflow.md)。

## 使用规则

- Issue 标题和正文都使用中文。
- 每个任务只设置 1 名主责人；认领前不预分配 Assignee。
- 每个任务必须分别写成一份完整 Issue 正文；不要把多个任务合并到一个 Issue，也不要用 Trellis PRD 代替 GitHub Issue 任务书。
- 任务书完成后必须先逐张校对，再直接上传或更新 GitHub Issue。
- 所有任务都必须同步到 GitHub Issue，并加入 GitHub Project `Software Teamwork`。
- `依赖任务`、`阻塞任务`、`并行任务` 在 issue 正文中必须写 GitHub issue
  引用，例如 `#118 #125`；没有则写“无”。任务尚未创建 issue 时可先写任务
  编号，同步或人工维护时再回填为 `#xx`。
- 验收标准必须可验证，不能只写“完成开发”“优化体验”“完善功能”。
- 一切冲突以 `docs/` 和 GitHub Issue / Project 当前状态为准；旧本地草稿不得覆盖公开契约或远端状态。

## 任务编号和分类

任务标题格式：

```text
A-001 中文任务标题
```

编号按字母组内部顺序递增。每个前缀都有独立序列，从 `001` 起三位补零。例如：

```text
A-001 Gateway 契约基线
A-002 Auth 会话 MVP
B-001 QA 服务骨架
```

| 前缀 | View | Project `Group` | 默认主责 |
| --- | --- | --- | --- |
| `A-*` | `Platform View` | `L1nggTeam` | 平台底座与知识管理 |
| `B-*` | `QA View` | `JerryTeam` | 智能问答 |
| `C-*` | `Report View` | `PrimeTeam` | 报告生成 |
| `F-*` | `Frontend View` | `Frontend` | 前端横向小队 |
| `S-*` | `Special View` | `Special` | OpenAPI、AI Gateway、联调、CI/CD、部署 |

创建新任务时，先查询同前缀已有任务编号，取该字母组当前最大序号加 1。不同字母组互不占号，例如 `A-003` 和 `B-003` 可以同时存在。

## 状态、优先级和 Risk

| 状态 | 含义 | Project `Risk` |
| --- | --- | --- |
| `Draft` | AI 刚生成，尚未由协调人确认。 | `Needs Decision` |
| `Ready` | 协调人确认，可以派发。 | `Normal` |
| `In Progress` | 已有人主责开发。 | `Normal` |
| `Blocked` | 依赖未满足或契约不清楚。 | `Blocked` |
| `Review` | 已提交 PR 或等待验收。 | `Normal` |
| `Done` | 代码、测试、文档回写都完成。 | `Normal` |

| 优先级 | 含义 |
| --- | --- |
| `P0` | 不做会导致当前功能无法开发、无法联调或文档与代码明显冲突。 |
| `P1` | 本轮业务闭环需要，但不阻塞其他人起步。 |
| `P2` | 演示质量、管理能力、体验增强或后续优化。 |

## 依赖字段

- `依赖任务`：当前任务开始前必须完成或至少稳定输出的任务。
- `阻塞任务`：当前任务完成后会解锁的下游任务。
- `并行任务`：可以并行推进但需要同步契约的任务。
- `依赖原因`：必须写具体接口、schema、数据结构、环境变量、服务能力或验收条件。

基础依赖方向：

```text
S 契约/专项
  -> A 平台底座
    -> B 智能问答
    -> C 报告生成
  -> F 前端横向
```

例外规则：

- `F-*` 可以和后端任务并行做 mock、页面骨架和类型占位，但真实联调仍写入 `依赖任务`。
- `B-*` 和 `C-*` 可以并行，只要它们不依赖同一个尚未稳定的 `A-*` 或 `S-*` 输出。
- 如果文档变更只影响某一组内部实现，不强行依赖其他类别。
- 如果出现循环依赖，先拆出更小的 `S-*` 契约确认任务。

## 上传前校对清单

每份任务书上传前必须逐项校对：

- [ ] 标题符合 `A/B/C/F/S-001 中文任务标题` 这类三位顺序编号格式。
- [ ] Issue 标题和正文都使用中文。
- [ ] `任务信息` 字段完整，状态、优先级、批次、模块、Risk 与本文档规则一致。
- [ ] 任务编号前缀和 `主责小组` 匹配。
- [ ] `依赖任务`、`阻塞任务`、`并行任务` 使用 GitHub issue 引用；尚未拿到编号时，先上传上游任务再回填。
- [ ] `权威依据` 指向具体公开文档、implementation 文档、GitHub issue 或 PR，不只写泛泛路径。
- [ ] `任务范围`、`交付物`、`验收标准` 一一对应，验收标准可被命令或手工步骤验证。
- [ ] `边界与不做内容` 明确，避免和相邻任务重复。
- [ ] 已对照 GitHub 当前状态基线，确认不是重复创建已完成、Review 中或已有人主责的同范围任务。

## GitHub Issue 正文模板

```markdown
## 认领规则

- 本任务为自领任务，当前不预分配 Assignee。
- 只允许 1 名主责人完成；认领前请在本 issue 评论 `认领：@你的 GitHub 用户名`，自动化会在校验通过后把评论者设为 Assignee。
- 可以请其他成员 review 或协助排障，但主责人只能有 1 个；如需转让，请在 issue 评论中交接清楚。
- 一切冲突以 `docs/` 为准；如果代码或旧本地草稿与 `docs/` 冲突，按 `docs/` 修改代码或同步公开文档。

## 任务信息

- 编号：`A/B/C/F/S-001`
- 状态：`Draft / Ready / In Progress / Blocked / Review / Done`
- 主责小组：`L1nggTeam / JerryTeam / PrimeTeam / Frontend / Special`
- View：`Platform / QA / Report / Frontend / Special`
- 优先级：`P0 / P1 / P2`
- 批次：`Batch 0 / Batch 1 / Batch 2 / Batch 3 / Batch 4`
- 模块：`gateway / auth / file / knowledge / qa / document / frontend / ai-gateway / openapi / deploy / ci`
- Risk：`Normal / Needs Decision / Blocked`
- 依赖任务：无 / #118 #125
- 阻塞任务：无 / #126 #127
- 并行任务：无 / #128
- 依赖原因：写清楚依赖的接口、schema、数据结构、环境变量、服务能力或验收条件。
- 建议分支：`group/type/short-title`
- GitHub Project：`Software Teamwork`
- Project sync：`pending / synced / blocked`

## 权威依据

- `docs/...`
- `docs/services/...`
- GitHub issue 或 PR 链接

## 任务范围

- ...
- ...
- ...

## 交付物

- ...
- ...
- ...

## 验收标准

- [ ] ...
- [ ] ...
- [ ] ...

## 边界与不做内容

- ...

## PR 要求

- PR 目标分支必须是主仓库 `develop`。
- Commit message 使用 Conventional Commits。
- PR 描述列出完成范围、验证命令、未完成风险和关联 issue。
```

## 可选增强字段

需要更细追踪时，可以在 `任务信息` 后追加：

````markdown
## 实现提示

- 主目录：
  - `services/...`
  - `apps/web/src/...`
  - `docs/...`
- 技术要求：
  - ...
- 验证命令：

```bash
# 写具体命令；没有自动化命令时写手工步骤。
```

## 完成记录

- GitHub Issue：pending
- GitHub Project item：pending
- PR：pending
- 完成人：pending
- 完成日期：pending
- 验收人：pending
````

## 最小合格示例

```markdown
## 认领规则

- 本任务为自领任务，当前不预分配 Assignee。
- 只允许 1 名主责人完成；认领前请在本 issue 评论 `认领：@你的 GitHub 用户名`，自动化会在校验通过后把评论者设为 Assignee。
- 可以请其他成员 review 或协助排障，但主责人只能有 1 个；如需转让，请在 issue 评论中交接清楚。
- 一切冲突以 `docs/` 为准；如果代码或旧本地草稿与 `docs/` 冲突，按 `docs/` 修改代码或同步公开文档。

## 任务信息

- 编号：`S-001`
- 状态：`Ready`
- 主责小组：`Special`
- View：`Special`
- 优先级：`P0`
- 批次：`Batch 0`
- 模块：`openapi`
- Risk：`Normal`
- 依赖任务：无
- 阻塞任务：#156 #161
- 并行任务：无
- 依赖原因：QA 后端和前端都需要稳定 citations schema，避免各自实现不同字段。
- 建议分支：`Special/docs/citation-schema`
- GitHub Project：`Software Teamwork`
- Project sync：`pending`

## 权威依据

- `docs/services/qa/README.md`
- `docs/services/qa/api/openapi.yaml`
- `docs/services/gateway/api/openapi.yaml`

## 任务范围

- 在 Gateway OpenAPI 中补齐 message detail response 的 citations schema。
- 明确 citation id、document id、chunk id、source title、score、snippet 字段。
- 明确字段缺失或来源不可用时的错误或降级策略。

## 交付物

- Gateway OpenAPI 中存在稳定 citations schema。
- QA 和前端任务能直接引用该 schema。

## 验收标准

- [ ] OpenAPI 中存在 message detail response 的 citations schema。
- [ ] 字段命名与现有 envelope、分页、错误规则一致。
- [ ] `B-001` 和 `F-001` 的依赖说明可追溯到本任务。

## 边界与不做内容

- 本任务只做契约对齐，不实现 QA 引用存储或前端引用展示。

## PR 要求

- PR 目标分支必须是主仓库 `develop`。
- Commit message 使用 Conventional Commits。
- PR 描述列出完成范围、验证命令、未完成风险和关联 issue。
```
