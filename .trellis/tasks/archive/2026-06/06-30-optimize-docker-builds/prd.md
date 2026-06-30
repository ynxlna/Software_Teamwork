# Optimize Docker Build Speed And Image Footprint

## Goal

Make the repository's Docker builds runnable first, then faster, smaller, and lighter for local and CI use. The work should address slow upstream pulls/downloads through configurable mirrors, reduce repeated dependency downloads with BuildKit caches, keep Go service images small, and make the heavier Parser image more deliberate without forcing every Dockerfile into one shared base.

## What I Already Know

- The user suspects slow Docker builds are caused by Docker/image/package sources and wants mirror support such as Aliyun/Tsinghua where appropriate.
- The user prioritizes Docker work in this order: runnable builds first, build speed second, small image size third, low memory use fourth, and low storage use fifth.
- The user does not require every Dockerfile to become one identical template, but wants the current Alpine/Debian split to be intentional and documented.
- The user observed `goproxy.cn` checksum database mirror failures during `go install github.com/pressly/goose/v3/cmd/goose@v3.27.1`, causing `migrate-file` to fail and the Parser build to be cancelled by Compose dependency failure.
- The user clarified that Docker build environment issues should be documented for other contributors, including the fastest safe setup and diagnostics for bad mirrors.
- The user clarified that the scope is all Docker-related surfaces, not only Go image/build mirrors. Compose infrastructure images, Parser Python/Debian sources, QA host Dockerfile, Docker daemon mirrors, and docs must be checked together.
- Upstream was updated before implementation. `upstream/develop` fast-forwarded to `a31b628`; PR #340 (`fix(docker): align local image versions`) is merged and includes commit `8455530`.
- PR #340 aligned local image versions but did not add BuildKit cache mounts, configurable image registry prefixes, Python/uv mirror controls, Parser multi-stage slimming, or per-service `.dockerignore` coverage.
- At task start, Go service Dockerfiles used `golang:1.25-alpine` build stages, `alpine:3.22` runtime stages, and `GOPROXY=https://goproxy.cn,direct`. The implementation now targets safer defaults with explicit domestic overrides.
- Current Parser Dockerfile uses `python:3.12-slim` and installs PaddleOCR extras in the same runtime stage.
- Existing docs already define pinned image tags: `postgres:16-alpine`, `redis:7-alpine`, `qdrant/qdrant:v1.18.2`, `golang:1.25-alpine`, `alpine:3.22`, MinIO server/client tags, and Parser's Debian/Python runtime exception.
- Follow-up audit after the first Docker commit rebased local work onto `upstream/develop@d003319` and added a machine-checkable Docker policy guard so docs, AI startup context, CI, and PR review guidance all point at the same fastest safe path.
- Clean-shell validation selected the mainland China path as explicit registry
  rewrite first: `deploy/.env.china.example` uses `docker.m.daocloud.io/...`
  full image names, Aliyun package mirrors, `goproxy.cn,direct`, and
  `sum.golang.google.cn`. The project-level root Compose build/start passed
  with these values and without shell proxy environment variables.
- Full project validation also found and fixed a Parser runtime Dockerfile
  issue: recursive `chown -R /app` was slow and failed because
  `/tmp/parser-cache` did not exist. Parser now creates the cache directory and
  uses `COPY --chown`; Docker policy tests block regressions.
- Local health/readiness validation must bypass shell proxies. A shell with
  `http_proxy`/`https_proxy` but no localhost `NO_PROXY` can return proxy-owned
  `503` responses for `localhost` even while all Compose containers are healthy.
  The diagnostic script now warns about this and the runbooks use
  `curl --noproxy '*'`/`NO_PROXY=localhost,127.0.0.1,::1`.

## Requirements

- Keep Go services independently buildable with service-local Dockerfiles.
- Preserve the current Go baseline of `golang:1.25-alpine` build stage and `alpine:3.22` runtime stage unless there is a measured reason to change it.
- Add configurable image registry prefix support for Dockerfile `FROM` lines so domestic mirrors can be used without hardcoding one vendor into repository defaults.
- Keep Compose infrastructure images pinned by default while allowing full-image overrides for local/enterprise registries.
- Default build settings must favor correctness/runnability over speed. A fast mirror may be documented as an opt-in override only if module checksum verification remains enabled and the failure mode is documented.
- Keep dependency mirrors configurable by build arguments/environment:
  - Go: default should not rely on a single third-party mirror with known sumdb issues; domestic speedups must be explicit build args.
  - Alpine apk: optional repository mirror override, disabled by default.
  - Debian apt and Python/uv for Parser: optional mirror/index overrides, disabled by default.
- Add BuildKit cache mounts for Go module cache/build cache and Parser package manager caches where practical.
- Add or update `.dockerignore` files for source-backed Docker build contexts so generated files, local caches, binaries, logs, and VCS metadata are excluded.
- Optimize Parser image shape while keeping Debian slim if required by PaddleOCR/native runtime dependencies.
- Update deploy/runbook/testing/architecture docs so the Docker baseline explains:
  - why Go services use Alpine while Parser uses Debian slim,
  - how to use mirrors,
  - how to use BuildKit/build cache,
  - how to diagnose broken Docker daemon registry mirrors and Go sumdb mirror paths,
  - which checks prove Docker changes are valid.
- Update CI Docker build invocation when needed to keep BuildKit-only syntax working.
- Add a CI policy guard that blocks obvious Docker regressions before daemon-dependent builds run, including `latest`, `GOSUMDB=off`, missing BuildKit cache mounts, missing pinned Compose overrides, Parser runtime command drift, and missing `.dockerignore`.
- Do not introduce production secrets, unpinned `latest` tags, or runtime-only assumptions into build layers.

## Acceptance Criteria

- [x] Go service Dockerfiles support configurable registry prefix and build args without changing default public image names.
- [x] Go service Dockerfiles use BuildKit cache mounts for module/build caches.
- [x] Parser Dockerfile supports configurable registry prefix plus apt/uv/Python index mirrors and avoids carrying unnecessary build cache into runtime.
- [x] Compose infrastructure images support explicit pinned defaults plus override variables for PostgreSQL, Redis, Qdrant, MinIO server, and MinIO client.
- [x] QA host-binary Dockerfile supports configurable registry prefix/postgres version without changing the pinned default.
- [x] Source-backed Docker contexts have `.dockerignore` coverage for local artifacts.
- [x] `docker compose` config validation passes for root default profile, root `ai` profile, QA compose, QA DB compose, and Document compose.
- [x] Changed Dockerfiles build at least far enough to validate Dockerfile syntax and build-arg wiring; if large Parser dependencies make a full build impractical, record the exact skipped runtime validation.
- [x] CI includes `scripts/check_docker_policy.py` so Docker policy regressions are caught without relying on the local daemon mirror.
- [x] `git diff --check` passes.
- [x] Documentation points to the new mirror/cache/environment workflow and remains aligned with pinned image baseline.

## Definition Of Done

- Dockerfiles, workflow, and docs are updated together.
- Checks are run and recorded in the final answer.
- Remaining image-size or full-runtime validation risks are explicitly called out.
- Trellis task remains traceable with research notes under `research/`.

## Technical Approach

Use a conservative two-family Docker baseline:

- Go services: keep Alpine runtime for small images, static Go binaries, and existing health probes. Add BuildKit cache mounts and optional mirror/build args to reduce repeated downloads, but keep checksum verification enabled.
- Parser: keep Debian slim because PaddleOCR/Paddle/native OCR dependencies are more compatible with Debian wheels and system packages than Alpine musl. Convert to a builder/runtime pattern if feasible, keep cache directories out of final layers, and expose mirror args for apt and uv/Python indexes.

Mirror behavior should be opt-in or overridable. Repository defaults stay
portable: public images and official package indexes remain the documented
neutral baseline. For China mainland users, the documented fastest safe path is
`deploy/.env.china.example` explicit registry/package-source rewrite. Docker
daemon mirrors and proxies are secondary paths that must pass
`scripts/check_docker_environment.py` diagnostics before being trusted.

## Decision (ADR-lite)

**Context**: Builds are slow and Dockerfiles are similar but not identical. Go services and Parser have different runtime dependency profiles. Hardcoding one domestic mirror would speed up one environment but make CI and international contributors more fragile. `goproxy.cn` can also proxy the Go checksum database and has been observed to fail during goose dependency verification, so mirror selection can break "can run" even when it improves download speed.

**Decision**: Standardize build mechanics and override points, not all images.
Keep Go on Alpine, Parser on Debian slim, keep Compose infrastructure pinned,
add configurable mirror/image arguments, add BuildKit cache mounts, add
`.dockerignore` coverage, and document Docker daemon registry mirrors separately
from Dockerfile package mirrors and Compose image overrides. For China mainland
usage, choose explicit registry rewrite over daemon mirror mode because the same
registry can work as a full image name while failing as a Docker Hub mirror
through `?ns=docker.io`. Keep checksum verification enabled and avoid known
broken third-party sumdb paths. Avoid an external Dockerfile frontend image
requirement because a broken daemon mirror can fail before the repository
Dockerfile is parsed.

**Consequences**: Default builds remain portable and safer. China mainland builds
have a documented one-file overlay that was validated with full root Compose
startup. BuildKit becomes the expected Docker builder. Parser image will still
be the largest image because PaddleOCR/Paddle dependencies dominate, but cache,
layer shape, and runtime ownership handling are now guarded.

## Out Of Scope

- Replacing PaddleOCR/Paddle dependencies or changing Parser OCR capability.
- Building and publishing shared organization base images.
- Introducing Kubernetes, production registry publishing, or multi-arch release automation.
- Collapsing all service Dockerfiles into one generated template.
- Changing application runtime behavior unrelated to Docker build/deploy.

## Technical Notes

- Relevant upstream PR: #340 `fix(docker): align local image versions`, merged 2026-06-30, now in local `develop`.
- Relevant files inspected:
  - `services/*/Dockerfile`
  - `services/parser/Dockerfile`
  - `deploy/Dockerfile.migrate`
  - `deploy/docker-compose.yml`
  - `services/qa/docker-compose.yml`
  - `services/document/docker-compose.yml`
  - `.github/workflows/docker-deploy-checks.yml`
  - `deploy/README.md`
  - `docs/runbooks/local-integration.md`
  - `docs/architecture/technology-decisions.md`
  - `.trellis/spec/backend/quality-guidelines.md`
  - `.trellis/spec/cicd.md`

## Research References

- [`research/docker-build-optimization.md`](research/docker-build-optimization.md) - Summary of local repo findings and recommended Docker build strategy.
