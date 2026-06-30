<!-- TRELLIS:START -->
# Trellis Instructions

These instructions are for AI assistants working in this project.

This project is managed by Trellis. The working knowledge you need lives under `.trellis/`:

- `.trellis/workflow.md` — development phases, when to create tasks, skill routing
- `.trellis/spec/` — package- and layer-scoped coding guidelines (read before writing code in a given layer)
- `.trellis/workspace/` — per-developer journals and session traces
- `.trellis/tasks/` — active and archived tasks (PRDs, research, jsonl context)

If a Trellis command is available on your platform (e.g. `/trellis:finish-work`, `/trellis:continue`), prefer it over manual steps. Not every platform exposes every command.

If you're using Codex or another agent-capable tool, additional project-scoped helpers may live in:
- `.agents/skills/` — reusable Trellis skills
- `.codex/agents/` — optional custom subagents

Managed by Trellis. Edits outside this block are preserved; edits inside may be overwritten by a future `trellis update`.

<!-- TRELLIS:END -->

## 项目前端说明

- 前端源码放在 `apps/web/src/`。
- 分支、PR、提交和合并策略以 `CONTRIBUTING.md` 为准；当前前端工作默认通过个人 fork 向主仓库 `develop` 发起 PR。
- 面向团队成员的协作流程见 `docs/collaboration/frontend-workflow.md`。
- 面向 agent 的前端规范见 `.trellis/spec/frontend/index.md`。
- 涉及前端开发、分支、PR、Lint 或 CI 时，优先加载项目级 `frontend-workflow` skill。

## Docker 与本地启动说明

- 本地联调入口见 `README.md`、`deploy/README.md` 和 `docs/runbooks/local-integration.md`。
- Docker 构建、镜像源、BuildKit cache、Go sumdb、Alpine/Debian/PyPI/uv 镜像和 daemon mirror 排障见 `docs/runbooks/docker-build-environment.md`。
- 面向中国大陆网络的默认推荐路径是 `deploy/.env.china.example` 显式 registry rewrite；优先级为 `registry rewrite > daemon mirror > proxy`。已有 daemon mirror 或代理时，先跑 `python3 scripts/check_docker_environment.py --profile all --clean-env` 判定当前路径是否可用。
- Docker 优先级固定为：能跑 > 构建快 > 镜像小 > 内存少 > 存储少。
- 改 Dockerfile、Compose、镜像 tag、镜像源、Go proxy/sumdb、apk/apt/PyPI/uv 源、Docker 环境诊断或 `.dockerignore` 时，必须运行 `python3 scripts/check_docker_policy.py` 和相关单元测试，并按变更范围运行 Compose config / Docker build 检查。
- 不要把正常路径改成 `GOSUMDB=off` 或 `latest` 镜像；遇到镜像源异常时先按 Docker runbook 排查并记录环境阻断。

## 本地私有说明

- Agent 可在存在时读取 `.agents/local/AGENTS.local.md` 作为本机私有补充说明。
- 该文件应保持 Git 忽略状态，用于记录机器特定的代理、账号或凭据相关要求。
