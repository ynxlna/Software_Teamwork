# Docker Build Optimization Research

## Repo Findings

- Go service Dockerfiles already use multi-stage builds and small Alpine runtime images.
- Go service builds repeat the same pattern seven times and currently use `go mod download` without BuildKit cache mounts, so cache reuse depends only on Docker layer cache and is fragile when source/module files change.
- `services/document/Dockerfile` and `services/ai-gateway/Dockerfile` do not use the same `CGO_ENABLED=0 GOOS=linux -trimpath -ldflags="-s -w"` build flags as most other Go services.
- Only Parser, QA, and Document have `.dockerignore`; Auth/File/Gateway/Knowledge/AI Gateway do not.
- Parser is the only Python runtime image. It uses `python:3.12-slim`, apt packages (`libglib2.0-0`, `libgl1`, `libgomp1`), uv, and PaddleOCR extras. This is intentionally heavier than Go services.
- Root Compose already pins infrastructure images. Upstream PR #340 aligned Redis and Alpine tags and documented the MinIO server/client exception.
- Compose infrastructure images are also part of the Docker source surface: PostgreSQL, Redis, Qdrant, MinIO server, and MinIO client should keep pinned defaults but allow full-image override variables.
- `services/qa/Dockerfile.host` is a Docker surface even though it builds from a precompiled host binary; it should support the same image registry prefix pattern for its `postgres:16-alpine` base.

## External Build Conventions

- Docker BuildKit cache mounts are the standard Dockerfile mechanism for persistent package manager caches without baking cache contents into final layers.
- Docker daemon registry mirrors accelerate image pulls at the daemon level and should be documented as local machine configuration, not hardcoded into `FROM` lines.
- Dockerfile `ARG` before `FROM` can parameterize a registry prefix while keeping default public images.
- uv supports Python package index configuration through environment/build arguments; this is the right level for PyPI/Tsinghua/Aliyun style mirror selection.
- Go checksum verification is part of build correctness. A Go module proxy may also proxy checksum database (`/sumdb/...`) requests when it advertises support. If that mirror has incomplete/broken sumdb paths, `go install` can fail even when module downloads appear to work.

## Go Proxy / SumDB Findings

- User-observed failure: `goproxy.cn` sumdb mirror returned bad/404 responses during `go install github.com/pressly/goose/v3/cmd/goose@v3.27.1`, causing migration image build failure.
- Local probes on 2026-06-30:
  - `https://goproxy.cn/sumdb/sum.golang.org/supported` returned `200`, so Go may use goproxy.cn as a checksum database proxy for `sum.golang.org`.
  - Specific lookup probes for `github.com/pressly/goose/v3@v3.27.1`, `github.com/tursodatabase/libsql-client-go@...`, and `modernc.org/libc@v1.72.1` returned `200` during this session, but that does not prove all required tiles/lookups are reliable.
  - `https://goproxy.cn/sumdb/sum.golang.google.cn/supported` returned `404`; this can make Go access `sum.golang.google.cn` directly instead of through goproxy.cn when `GOSUMDB=sum.golang.google.cn` is configured.
  - `https://proxy.golang.org` timed out from the current environment, so the official proxy is not a reliable sole default for this network.
- Do not set `GOSUMDB=off` as the normal fix; that trades build speed for weaker supply-chain verification. Use it only as an explicit last-resort local workaround.

## Docker Daemon Mirror Findings

- Local Docker daemon mirrors on 2026-06-30: `["https://docker.m.daocloud.io/"]`.
- `/etc/docker/daemon.json` also contained `https://docker.1panel.live/`, but the running daemon reported only `https://docker.m.daocloud.io/`; trust `docker info` for the active configuration.
- `DOCKER_BUILDKIT=1 docker build --file deploy/Dockerfile.migrate ... deploy` parsed the Dockerfile and resolved `golang:1.25-alpine`, then failed resolving `alpine:3.22` with:
  `unexpected status from HEAD request to https://docker.m.daocloud.io/v2/library/alpine/manifests/3.22?ns=docker.io: 401 Unauthorized`.
- `docker buildx build --check --file services/qa/Dockerfile.host services/qa` failed before Dockerfile logic ran because the same daemon mirror returned `401 Unauthorized` for `postgres:16-alpine`.
- Earlier attempts with `# syntax=docker/dockerfile:*` failed before repository Dockerfile logic ran because the same daemon mirror blocked the external Dockerfile frontend image. Removing syntax headers avoids that extra pull while keeping BuildKit cache mounts working on current Docker.
- `docker.1panel.live` was not a working replacement in this environment: explicit metadata probes for `library/alpine:3.22`, `library/golang:1.25-alpine`, `library/python:3.12-slim`, and `library/postgres:16-alpine` returned `403 Forbidden`.
- Explicit `docker.io/library/...` image names do not bypass a daemon mirror for Docker Hub; BuildKit still routed `docker.io` through the configured DaoCloud mirror and hit the same `?ns=docker.io` `401 Unauthorized` failure.
- Explicit `registry-1.docker.io/...` image names can bypass the DaoCloud mirror, and short metadata probes succeeded for `alpine`, `golang`, `python`, `postgres`, `qdrant`, and MinIO images. However, full Compose pull of the pinned infrastructure images timed out on `registry-1.docker.io`, and a single `deploy/Dockerfile.migrate --target build` check later timed out while resolving `registry-1.docker.io/library/golang:1.25-alpine`.
- This is an environment issue, not a service Dockerfile issue. If the project
  keeps Docker Hub short image names, the broken daemon mirror must be removed
  or replaced. If the project uses a complete explicit registry rewrite, the
  Docker Hub mirror path is bypassed for rewritten images.
- Explicit DaoCloud registry rewrite was validated in a clean shell-proxy
  environment on 2026-06-30:
  - `docker.m.daocloud.io/library/alpine:3.22`
  - `docker.m.daocloud.io/library/golang:1.25-alpine`
  - `docker.m.daocloud.io/library/python:3.12-slim`
  - `docker.m.daocloud.io/library/postgres:16-alpine`
  - `docker.m.daocloud.io/library/redis:7-alpine`
  - `docker.m.daocloud.io/qdrant/qdrant:v1.18.2`
  - `docker.m.daocloud.io/minio/minio:RELEASE.2025-09-07T16-13-09Z`
  - `docker.m.daocloud.io/minio/mc:RELEASE.2025-08-13T08-35-41Z`
  This validates the chosen mainland China path as explicit registry rewrite,
  not DaoCloud daemon mirror mode.

## Recommended Approach

1. Add BuildKit cache mounts without requiring an external Dockerfile frontend image:
   - Go: cache `/go/pkg/mod` and `/root/.cache/go-build`.
   - apk: cache `/var/cache/apk` only when installing packages.
   - Parser/uv: cache uv/Python package downloads in builder stage.
2. Add build args:
   - `IMAGE_REGISTRY_PREFIX` for `FROM` image mirror/registry overrides.
   - `GOPROXY`, with safe defaults and explicit domestic override examples.
   - `GOSUMDB`, defaulting to the Go toolchain default; domestic override examples can use `sum.golang.google.cn` to avoid a broken third-party sumdb mirror path while keeping verification enabled.
   - `ALPINE_MIRROR`, empty by default.
   - Parser-specific `APT_MIRROR`, `UV_DEFAULT_INDEX`, and optionally `UV_INDEX`.
3. Add Compose image override variables:
   - `POSTGRES_IMAGE`, `REDIS_IMAGE`, `QDRANT_IMAGE`, `MINIO_IMAGE`, `MINIO_MC_IMAGE`.
   - Defaults must remain pinned and must not become `latest`.
4. Add `.dockerignore` files for all Go service build contexts.
5. Keep Go and Parser runtime families separate:
   - Go: Alpine is small and compatible with static binaries.
   - Parser: Debian slim avoids Alpine/musl problems with Paddle/PaddleOCR native wheels.
6. Update CI to force BuildKit for Dockerfile builds.
7. Add a lightweight repository Docker policy checker before daemon-dependent
   builds. This should fail on `latest`, `GOSUMDB=off`, missing BuildKit cache
   mounts, missing pinned Compose override variables, Parser Docker runtime
   command drift, Parser runtime recursive `/app` chown, and missing
   `.dockerignore`.
8. Update docs with mirror examples and expectations.

## Risks And Mitigations

- BuildKit-only syntax can fail on old Docker engines. Mitigation: document Compose v2/BuildKit baseline and set `DOCKER_BUILDKIT=1` in CI build jobs.
- External Dockerfile frontend pulls can fail before build logic runs when a daemon registry mirror is broken. Mitigation: rely on the Docker engine's bundled frontend for current cache-mount syntax instead of adding `# syntax=docker/dockerfile:*` headers.
- Domestic mirror URLs can become unavailable or serve incomplete module/sumdb data. Mitigation: make mirror args optional and overridable, keep checksum verification enabled by default, and document tested combinations plus failure symptoms.
- Parser image may remain large because PaddleOCR dependencies dominate. Mitigation: keep cache out of runtime layers, use `COPY --chown` instead of a recursive runtime `chown -R /app`, and document that size is capability-driven.
- Registry prefix syntax must include a trailing slash when used. Mitigation: document examples like `--build-arg IMAGE_REGISTRY_PREFIX=registry.cn-hangzhou.aliyuncs.com/dockerhub-mirror/` or a team-approved mirror path.
- Compose `image:` overrides can accidentally drift to unpinned tags. Mitigation: keep pinned defaults in Compose and document `*_IMAGE` variables as full pinned image replacements only.
- CI and review can miss policy drift if Docker happens to build on one machine.
  Mitigation: run `python3 scripts/check_docker_policy.py` in
  `docker-deploy-checks.yml`, document it in `docs/testing/strategy.md`, and
  add unit tests for the checker.

## Validation Notes

- Compose config validation passed for:
  - `deploy/docker-compose.yml` default profile
  - `deploy/docker-compose.yml --profile ai`
  - `services/qa/docker-compose.yml`
  - `services/qa/docker-compose.db.yml`
  - `services/document/docker-compose.yml`
- `git diff --check` passed.
- `python3 scripts/check_docker_policy.py` passed.
- `python3 -m unittest scripts.tests.test_check_docker_policy` passed.
- `.github/workflows/docker-deploy-checks.yml` parsed successfully via Node's
  `yaml` package.
- `docker buildx build --check --target build` passed for every changed Go service Dockerfile and migration Dockerfile.
- `docker buildx build --check` passed for `services/parser/Dockerfile`.
- `services/qa/Dockerfile.host` supports `IMAGE_REGISTRY_PREFIX` and
  `POSTGRES_VERSION`; it should be validated with a real build when that Docker
  surface changes.
- After rebasing onto `upstream/develop@d003319`, a full `buildx --check` loop
  passed through deploy migration, all Go service Dockerfiles, service migration
  Dockerfiles, and Parser; it stopped only at `services/qa/Dockerfile.host`
  because the configured daemon mirror `https://docker.m.daocloud.io/` returned
  `401 Unauthorized` for `postgres:16-alpine` metadata before Dockerfile logic
  executed.
- Representative BuildKit build-stage checks passed:
  - `services/auth/Dockerfile --target build` with `GOPROXY=https://goproxy.cn,direct`, `GOSUMDB=sum.golang.google.cn`, and Tsinghua Alpine mirror.
  - `deploy/Dockerfile.migrate --target build` with the same Go/sumdb settings; this validates the goose dependency path that previously failed.
- The default Docker Hub short-name path remains blocked in the current local
  environment by Docker daemon mirror `https://docker.m.daocloud.io/` returning
  `401 Unauthorized` for `alpine:3.22` metadata. Use the mainland China overlay
  or fix the daemon mirror before relying on default short image names.
- Failed project-level attempts documented why explicit registry rewrite is the
  selected path:
  - With the default active daemon mirror, `alpine:3.22` fails through DaoCloud with `401 Unauthorized`.
  - With explicit `docker.io/library/...` overrides, BuildKit still routes Docker Hub through the daemon mirror and fails with the same DaoCloud `401`.
  - With explicit `registry-1.docker.io/...` overrides, DaoCloud is bypassed, but full infrastructure pulls and a representative Go build-stage metadata resolve timed out against Docker Hub direct.
- A full root Compose run passed in a clean shell-proxy environment using the
  chosen mainland China overlay values:
  `DOCKER_IMAGE_REGISTRY_PREFIX=docker.m.daocloud.io/library/`, DaoCloud full
  image overrides for PostgreSQL/Redis/Qdrant/MinIO, Aliyun Alpine/Debian/PyPI
  mirrors, `GOPROXY=https://goproxy.cn,direct`, and
  `GOSUMDB=sum.golang.google.cn`.
  - Command shape:
    `env -u http_proxy -u https_proxy -u HTTP_PROXY -u HTTPS_PROXY -u ALL_PROXY -u all_proxy DOCKER_BUILDKIT=1 ... docker compose -f deploy/docker-compose.yml --env-file deploy/.env.example up -d --build`
  - Build result: all local Go service, migration, and Parser images built.
  - Runtime result: PostgreSQL, Redis, Qdrant, MinIO, Parser, Auth, File,
    Knowledge, QA, Document, and Gateway containers were `healthy`.
  - One-shot jobs `migrate-auth`, `migrate-file`, `migrate-knowledge`,
    `migrate-qa`, `migrate-document`, `minio-init`, and `seed-local` exited `0`.
  - HTTP checks passed: gateway `/healthz` and `/readyz` returned `200`; service
    `/readyz` returned `200` for Auth, File, Knowledge, QA, Document, and Parser.
- A later direct `curl http://localhost:...` check from a shell with
  `http_proxy`/`https_proxy` but no localhost `NO_PROXY` returned proxy-owned
  `503` responses with `Proxy-Connection` headers. Re-running the same checks
  with `curl --noproxy '*'` returned `200` for Gateway and all service readiness
  endpoints. `scripts/check_docker_environment.py` now warns when shell proxy is
  set without `NO_PROXY=localhost,127.0.0.1,::1`.
- An initial full Compose run exposed a Parser Dockerfile bug unrelated to
  mirrors: the old runtime stage ran `chown -R parser:parser /app
  /tmp/parser-cache`; it spent about 206 seconds recursively chowning `/app` and
  then failed because `/tmp/parser-cache` did not exist. The Dockerfile now
  creates `/tmp/parser-cache` before `USER parser` and uses `COPY --chown` for
  `/app`; `scripts/check_docker_policy.py` blocks a regression to recursive
  `/app` chown.
