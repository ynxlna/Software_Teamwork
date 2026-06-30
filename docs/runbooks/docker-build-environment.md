# Docker 构建环境与镜像源

日期：2026-06-30

本文记录本仓库 Docker 构建的本机环境配置。优先级固定为：

```text
能跑 > 构建快 > 镜像小 > 内存少 > 存储少
```

默认构建保持可移植，不绑定某一个国内镜像站。中国大陆网络的推荐路径是
`deploy/.env.china.example`，它用显式 registry rewrite 和包管理器镜像源覆盖默认值。
遇到慢或失败时，排查顺序固定为：

```text
registry rewrite > daemon mirror > proxy
```

镜像源、Go proxy、apt/apk/PyPI 源都可以按本机网络显式覆盖；覆盖后仍应保留 Go
module checksum verification。

## 推荐快速配置

先在仓库根目录诊断本机 Docker 路径。`--clean-env` 会移除当前 shell 的代理环境变量，
用于模拟“没有本机 shell 代理”的网络路径：

```bash
python3 scripts/check_docker_environment.py --profile all --clean-env
```

如果 `china explicit registry` 通过，面向中国大陆用户优先使用 overlay：

```bash
cd deploy
cp .env.example .env
cat .env.china.example >> .env
DOCKER_BUILDKIT=1 docker compose up -d --build
```

PowerShell：

```powershell
cd deploy
Copy-Item .env.example .env
Get-Content .env.china.example | Add-Content .env
$env:DOCKER_BUILDKIT = "1"
docker compose up -d --build
```

这个 overlay 当前固定为：

- 基础镜像和 Compose 基础设施镜像：显式使用 `docker.m.daocloud.io/...`。
- Alpine、Debian、PyPI/uv：使用阿里云镜像。
- Go module：`GOPROXY=https://goproxy.cn,direct`，`GOSUMDB=sum.golang.google.cn`。

它不依赖 Docker daemon registry mirror。显式 `docker.m.daocloud.io/...` 镜像名会直接
访问 DaoCloud registry；本机 daemon 的 Docker Hub mirror 只会影响未改写的
`docker.io`/短镜像名路径。

如果当前网络直连 Docker Hub 更稳定，也可以保留默认配置：

```bash
cd deploy
cp .env.example .env
DOCKER_BUILDKIT=1 docker compose up -d --build
```

`DOCKER_BUILDKIT` 必须是执行 `docker build` / `docker compose build` 的 shell 环境变量；
它不是 Compose build arg，写入 `deploy/.env` 不一定能控制 Docker builder。

不要把 `GOSUMDB=off` 当作普通修复。`goproxy.cn` 会声明支持代理 `sum.golang.org`
checksum database；当该代理路径上的 lookup/tile 返回异常 404 时，`go install
github.com/pressly/goose/v3/cmd/goose@v3.27.1` 会在校验阶段失败，进而让
`migrate-file` 失败并取消依赖它的 Parser/服务构建。使用
`sum.golang.google.cn` 可以绕开第三方 sumdb 代理路径，同时继续做 checksum
verification。

如需只构建镜像：

```bash
cd deploy
DOCKER_BUILDKIT=1 docker compose --env-file .env build
DOCKER_BUILDKIT=1 docker compose --env-file .env --profile ai build
```

Compose 基础设施镜像也可以按本机/企业 registry 覆盖，但默认必须继续是明确 tag：

```bash
export POSTGRES_IMAGE=postgres:16-alpine
export REDIS_IMAGE=redis:7-alpine
export QDRANT_IMAGE=qdrant/qdrant:v1.18.2
export MINIO_IMAGE=minio/minio:RELEASE.2025-09-07T16-13-09Z
export MINIO_MC_IMAGE=minio/mc:RELEASE.2025-08-13T08-35-41Z
```

如果企业 registry 使用重写后的完整镜像名，把这些变量设成完整目标镜像，同时给
Dockerfile `FROM` 路径设置 `DOCKER_IMAGE_REGISTRY_PREFIX`。Compose 的 `image:` 字段和
Dockerfile 的 `FROM` 字段是两条不同路径，必须一起覆盖才能避免某一类镜像仍走默认
Docker Hub 路径。

## 镜像源与网络路径选择

Docker daemon 的 registry mirror 是本机配置，不应写死到仓库 Dockerfile。排障时优先用
仓库诊断脚本，因为它会同时打印 shell proxy、Docker client proxy、daemon mirror 和三组
manifest 探测：

```bash
python3 scripts/check_docker_environment.py --profile all --clean-env
```

也可以单独查看 daemon 实际配置：

```bash
docker info --format '{{json .RegistryConfig.Mirrors}}'
```

手工验证镜像源时，至少覆盖本仓库这些基础镜像：

```bash
docker manifest inspect alpine:3.22 >/tmp/alpine-manifest.json
docker manifest inspect golang:1.25-alpine >/tmp/golang-manifest.json
docker manifest inspect python:3.12-slim >/tmp/python-manifest.json
docker manifest inspect postgres:16-alpine >/tmp/postgres-manifest.json
docker manifest inspect redis:7-alpine >/tmp/redis-manifest.json
docker manifest inspect qdrant/qdrant:v1.18.2 >/tmp/qdrant-manifest.json
docker manifest inspect minio/minio:RELEASE.2025-09-07T16-13-09Z >/tmp/minio-manifest.json
docker manifest inspect minio/mc:RELEASE.2025-08-13T08-35-41Z >/tmp/minio-mc-manifest.json
```

本次排查中，`https://docker.m.daocloud.io/` 作为 daemon mirror 时对 `alpine:3.22` 和
`postgres:16-alpine` manifest 返回 `401 Unauthorized`，并且会让 BuildKit 在解析
`FROM alpine:3.22`、`FROM postgres:16-alpine` 或外部 Dockerfile frontend 时失败。
但同一个站点作为显式 registry rewrite 时，`docker.m.daocloud.io/library/alpine:3.22`
等完整镜像名可以正常返回 manifest。结论是：不要把“某 registry 可用”等同于“它作为
daemon mirror 一定可用”。

2026-06-30 的本机实测还说明：只是不使用 DaoCloud 不等于更快或更稳定。
`docker.1panel.live` 对 `alpine:3.22`、`golang:1.25-alpine`、`python:3.12-slim`、
`postgres:16-alpine` 返回 `403 Forbidden`。显式使用 `docker.io/library/...` 时，
BuildKit 仍会按 Docker daemon 的 `docker.io` mirror 规则转到 DaoCloud，并在
`?ns=docker.io` manifest 请求上返回 `401 Unauthorized`。显式使用
`registry-1.docker.io/...` 可以绕过 DaoCloud mirror，但当前网络在全量 Compose pull
多张基础设施镜像时出现 `i/o timeout`，单独解析
`registry-1.docker.io/library/golang:1.25-alpine` 也可能超时。

推荐路径按网络环境选择：

| 环境 | 推荐做法 | 判断标准 |
| --- | --- | --- |
| 没配 Docker mirror / proxy 的中国大陆网络 | 直接使用 `deploy/.env.china.example` | `python3 scripts/check_docker_environment.py --profile china --clean-env` 通过 |
| 已有 daemon mirror | 先跑 `--profile all --clean-env`；default 路径全通过才保留 mirror | default 失败但 china 通过时，用 `.env.china.example`，不要继续依赖短镜像名 |
| 已有 shell 代理 | 先用 `--clean-env` 验证不用 shell 代理也能跑 | 只有不加 `--clean-env` 才通过时，说明代理参与了 CLI/探测；Docker daemon pull/build 仍应单独配置 daemon proxy |
| 企业/团队内网 registry | 同时设置 `DOCKER_IMAGE_REGISTRY_PREFIX` 和 `POSTGRES_IMAGE` 等完整镜像名 | 目标 registry 必须覆盖 Go/Alpine/Python 基础镜像和 Compose 基础设施镜像 |
| Docker Hub 直连稳定 | 保留默认 `.env.example` | default 和 dockerhub-direct 探测、全量 pull/build 都通过 |

如果 daemon mirror 在 `?ns=docker.io` manifest 请求上返回 401/403/404，不需要为了使用
`.env.china.example` 先改 daemon 配置；显式 registry rewrite 会绕开未限定 Docker Hub
短名路径。只有当你想继续使用默认短镜像名时，才需要移除或替换坏的 daemon mirror。

`DOCKER_IMAGE_REGISTRY_PREFIX` 只用于企业/团队提供的显式 registry rewrite，例如：

```bash
export DOCKER_IMAGE_REGISTRY_PREFIX=registry.example.com/dockerhub/
```

该值会直接改写 Dockerfile `FROM` 镜像名，必须包含末尾 `/`，并且目标 registry 必须
提供 `golang:1.25-alpine`、`alpine:3.22`、`python:3.12-slim` 等同名路径。Compose
基础设施镜像不使用这个前缀，必须通过 `POSTGRES_IMAGE`、`REDIS_IMAGE`、
`QDRANT_IMAGE`、`MINIO_IMAGE` 和 `MINIO_MC_IMAGE` 单独设置完整镜像名。

## Go 构建

默认值：

```text
GO_DOCKER_GOPROXY=https://proxy.golang.org,direct
GO_DOCKER_GOSUMDB=sum.golang.org
```

国内推荐显式覆盖：

```bash
export GO_DOCKER_GOPROXY=https://goproxy.cn,direct
export GO_DOCKER_GOSUMDB=sum.golang.google.cn
```

验证 goose 依赖下载和校验：

```bash
docker run --rm golang:1.25-alpine sh -c \
  'GOMODCACHE=/tmp/modcache GOCACHE=/tmp/gocache GOPROXY=https://goproxy.cn,direct GOSUMDB=sum.golang.google.cn go install github.com/pressly/goose/v3/cmd/goose@v3.27.1'
```

如果这条命令失败，先修正 Go proxy/sumdb 或本机网络，再构建 migration 镜像。

## Alpine 与 Debian

Go 服务继续使用：

```text
builder: golang:1.25-alpine
runtime: alpine:3.22
```

原因是 Go 服务可以产出静态小二进制，Alpine runtime 镜像小，适合当前服务边界。

Parser 继续使用：

```text
python:3.12-slim
```

原因是 Parser 依赖 PaddleOCR/PaddlePaddle/native Python wheels 和系统库。为了省体积强行切到 Alpine/musl 会优先破坏“能跑”。Parser 的优化策略是 Debian slim、多阶段构建、BuildKit cache 和避免把包管理器缓存带进 runtime。

## Parser 构建

Parser 可选镜像源示例：

```bash
DOCKER_BUILDKIT=1 docker build \
  --build-arg APT_MIRROR=https://mirrors.tuna.tsinghua.edu.cn/debian \
  --build-arg APT_SECURITY_MIRROR=https://mirrors.tuna.tsinghua.edu.cn/debian-security \
  --build-arg PIP_INDEX_URL=https://pypi.tuna.tsinghua.edu.cn/simple \
  --build-arg UV_DEFAULT_INDEX=https://pypi.tuna.tsinghua.edu.cn/simple \
  -t software-teamwork-parser:local \
  services/parser
```

Parser 是当前最大的镜像和最高内存服务。默认本地 Compose 保持：

```text
PARSER_LOAD_BACKEND_ON_STARTUP=false
PARSER_MAX_CONCURRENCY=1
```

这会减少启动时常驻模型内存，优先保证 16 GB 级别开发机可运行。真实 OCR 第一次调用仍可能下载/加载模型，构建和运行时间都明显高于 Go 服务。

Parser runtime stage 不应对 `/app` 做递归 `chown -R`。本次全量构建实测中，旧写法先对
Python 虚拟环境递归 chown 约 206 秒，随后又因 `/tmp/parser-cache` 不存在而失败。当前
Dockerfile 先创建 `/tmp/parser-cache`，并通过 `COPY --chown=parser:parser /app /app`
设置 `/app` ownership；`scripts/check_docker_policy.py` 会阻止退回递归 chown。

## Compose 基础设施镜像

本地联调依赖这些 pinned 镜像：

| 组件 | 默认镜像 | 覆盖变量 |
| --- | --- | --- |
| PostgreSQL | `postgres:16-alpine` | `POSTGRES_IMAGE` |
| Redis | `redis:7-alpine` | `REDIS_IMAGE` |
| Qdrant | `qdrant/qdrant:v1.18.2` | `QDRANT_IMAGE` |
| MinIO server | `minio/minio:RELEASE.2025-09-07T16-13-09Z` | `MINIO_IMAGE` |
| MinIO client | `minio/mc:RELEASE.2025-08-13T08-35-41Z` | `MINIO_MC_IMAGE` |

不要把这些变量改成 `latest`。如果某个镜像在当前网络下拉取慢或失败，优先使用
`deploy/.env.china.example` 或等价的显式 registry rewrite；daemon mirror 只有在
manifest 探测和真实 pull 都通过时才作为第二选择；Docker daemon proxy 是最后选择。

## 存储与缓存

Dockerfiles 使用 BuildKit cache mount 复用：

- Go module cache: `/go/pkg/mod`
- Go build cache: `/root/.cache/go-build`
- apk cache: `/var/cache/apk`
- apt cache: `/var/cache/apt`、`/var/lib/apt/lists`
- pip/uv cache: `/root/.cache/pip`、`/root/.cache/uv`

这些 cache 不会进入最终 runtime layer，但会占用本机 builder 存储。查看和清理：

```bash
docker system df
docker builder prune
```

清理会让下一次构建重新下载依赖；只在磁盘压力明显时执行。

## 验证清单

策略层验证：

```bash
python3 scripts/check_docker_policy.py
```

这条命令不依赖 Docker daemon，可先挡住明显回退：`latest`、`GOSUMDB=off`、丢失
BuildKit cache mount、Compose pinned 默认值漂移、Parser 容器入口回退、缺少
`.dockerignore` 等。除非同步更新本 runbook 和 Trellis/CI 规范，否则不要绕过它。

配置层验证：

```bash
docker compose -f deploy/docker-compose.yml --env-file deploy/.env.example config --quiet
docker compose -f deploy/docker-compose.yml --env-file deploy/.env.example --env-file deploy/.env.china.example config --quiet
docker compose -f deploy/docker-compose.yml --env-file deploy/.env.example --profile ai config --quiet
docker compose -f services/qa/docker-compose.yml config --quiet
docker compose -f services/qa/docker-compose.db.yml config --quiet
docker compose -f services/document/docker-compose.yml config --quiet
```

环境层验证：

```bash
python3 scripts/check_docker_environment.py --skip-network --clean-env
python3 scripts/check_docker_environment.py --profile china --clean-env
```

2026-06-30 本机实测中，`--profile china --clean-env` 的显式 DaoCloud registry rewrite
通过了 Alpine、Go、Python、PostgreSQL、Redis、Qdrant、MinIO server 和 MinIO client
manifest 探测；默认 daemon mirror 路径仍会在 `?ns=docker.io` 请求上遇到 DaoCloud
`401 Unauthorized`。

构建层验证：

```bash
DOCKER_BUILDKIT=1 docker build -f deploy/Dockerfile.migrate deploy
DOCKER_BUILDKIT=1 docker build services/auth
DOCKER_BUILDKIT=1 docker build services/parser
DOCKER_BUILDKIT=1 docker build -f services/qa/Dockerfile.host services/qa
```

如果构建在 `FROM alpine:3.22`、`FROM golang:1.25-alpine`、`FROM python:3.12-slim`、
`FROM postgres:16-alpine` 或 Compose `image:` 的 metadata/pull 阶段失败，先用
`scripts/check_docker_environment.py` 判断是默认 Docker Hub 路径、显式 registry rewrite、
daemon mirror 还是 proxy 问题；这类失败发生在仓库 Dockerfile 逻辑执行之前。

项目级启动验证：

```bash
cd deploy
cp .env.example .env
cat .env.china.example >> .env
DOCKER_BUILDKIT=1 docker compose up -d --build
docker compose ps
curl --noproxy '*' -fsS http://localhost:8080/healthz
curl --noproxy '*' -fsS http://localhost:8080/readyz
```

如果当前 shell 设置了 `HTTP_PROXY`/`HTTPS_PROXY`/`http_proxy`/`https_proxy`，本地
health/readiness 请求必须绕过代理。推荐设置
`NO_PROXY=localhost,127.0.0.1,::1`，或像上面的命令一样使用 `curl --noproxy '*'`。
否则代理可能返回自己的 `503` 或超时，让本地服务被误判为不可用。

本机在移除 shell 代理环境变量后，用 `.env.china.example` 等价变量完成了根级
`docker compose up -d --build`。所有本地服务镜像、migration 镜像和 Parser 镜像构建
完成；`migrate-auth`、`migrate-file`、`migrate-knowledge`、`migrate-qa`、
`migrate-document`、`minio-init` 和 `seed-local` 均退出 `0`；PostgreSQL、Redis、
Qdrant、MinIO、Parser、Auth、File、Knowledge、QA、Document 和 Gateway 容器均为
`healthy`；Gateway `/healthz`、Gateway `/readyz` 以及各服务 `/readyz` 均返回 `200`。
