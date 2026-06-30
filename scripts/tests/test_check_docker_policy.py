import tempfile
import textwrap
import unittest
from pathlib import Path

from scripts.check_docker_policy import verify_docker_policy


VALID_GO_DOCKERFILE = textwrap.dedent(
    """
    ARG IMAGE_REGISTRY_PREFIX=
    ARG GO_VERSION=1.25
    ARG ALPINE_VERSION=3.22

    FROM ${IMAGE_REGISTRY_PREFIX}golang:${GO_VERSION}-alpine AS build
    ARG GOPROXY=https://proxy.golang.org,direct
    ARG GOSUMDB=sum.golang.org
    ARG ALPINE_MIRROR=
    ENV GOPROXY=${GOPROXY} GOSUMDB=${GOSUMDB}
    COPY go.mod go.sum ./
    RUN --mount=type=cache,target=/go/pkg/mod go mod download
    COPY . .
    RUN --mount=type=cache,target=/go/pkg/mod \\
        --mount=type=cache,target=/root/.cache/go-build \\
        CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/auth ./cmd/server

    FROM ${IMAGE_REGISTRY_PREFIX}alpine:${ALPINE_VERSION}
    ARG ALPINE_MIRROR=
    RUN --mount=type=cache,target=/var/cache/apk apk add --update-cache --cache-dir /var/cache/apk ca-certificates
    COPY --from=build /out/auth /usr/local/bin/auth
    ENTRYPOINT ["auth"]
    """
)


VALID_PARSER_DOCKERFILE = textwrap.dedent(
    """
    ARG IMAGE_REGISTRY_PREFIX=
    ARG PYTHON_VERSION=3.12
    ARG DEBIAN_VARIANT=slim
    ARG UV_VERSION=0.11.6

    FROM ${IMAGE_REGISTRY_PREFIX}python:${PYTHON_VERSION}-${DEBIAN_VARIANT} AS builder
    ARG APT_MIRROR=
    ARG APT_SECURITY_MIRROR=
    ARG UV_DEFAULT_INDEX=
    ARG UV_INDEX=
    ARG PIP_INDEX_URL=
    RUN --mount=type=cache,target=/var/cache/apt,sharing=locked apt-get update
    RUN --mount=type=cache,target=/root/.cache/pip python -m pip install uv==${UV_VERSION}
    RUN --mount=type=cache,target=/root/.cache/uv uv sync --frozen --no-dev --extra paddleocr

    FROM ${IMAGE_REGISTRY_PREFIX}python:${PYTHON_VERSION}-${DEBIAN_VARIANT} AS runtime
    ARG APT_MIRROR=
    ARG APT_SECURITY_MIRROR=
    RUN --mount=type=cache,target=/var/cache/apt,sharing=locked apt-get update
    RUN useradd --system --uid 10001 --create-home parser \\
        && mkdir -p /tmp/parser-cache \\
        && chown parser:parser /tmp/parser-cache
    COPY --from=builder --chown=parser:parser /app /app
    CMD ["parser-service"]
    """
)


VALID_COMPOSE = textwrap.dedent(
    """
    services:
      postgres:
        image: ${POSTGRES_IMAGE:-postgres:16-alpine}
      redis:
        image: ${REDIS_IMAGE:-redis:7-alpine}
      parser:
        build:
          context: ../services/parser
          dockerfile: Dockerfile
          args:
            IMAGE_REGISTRY_PREFIX: ${DOCKER_IMAGE_REGISTRY_PREFIX:-}
            APT_MIRROR: ${DEBIAN_APT_MIRROR:-}
            APT_SECURITY_MIRROR: ${DEBIAN_SECURITY_APT_MIRROR:-}
            PIP_INDEX_URL: ${PIP_INDEX_URL:-}
            UV_DEFAULT_INDEX: ${UV_DEFAULT_INDEX:-}
            UV_INDEX: ${UV_INDEX:-}
      auth:
        build:
          context: ../services/auth
          dockerfile: Dockerfile
          args:
            IMAGE_REGISTRY_PREFIX: ${DOCKER_IMAGE_REGISTRY_PREFIX:-}
            GOPROXY: ${GO_DOCKER_GOPROXY:-https://proxy.golang.org,direct}
            GOSUMDB: ${GO_DOCKER_GOSUMDB:-sum.golang.org}
            ALPINE_MIRROR: ${ALPINE_MIRROR:-}
    """
)


VALID_CHINA_ENV = textwrap.dedent(
    """
    DOCKER_IMAGE_REGISTRY_PREFIX=docker.m.daocloud.io/library/
    POSTGRES_IMAGE=docker.m.daocloud.io/library/postgres:16-alpine
    REDIS_IMAGE=docker.m.daocloud.io/library/redis:7-alpine
    QDRANT_IMAGE=docker.m.daocloud.io/qdrant/qdrant:v1.18.2
    MINIO_IMAGE=docker.m.daocloud.io/minio/minio:RELEASE.2025-09-07T16-13-09Z
    MINIO_MC_IMAGE=docker.m.daocloud.io/minio/mc:RELEASE.2025-08-13T08-35-41Z
    GO_DOCKER_GOPROXY=https://goproxy.cn,direct
    GO_DOCKER_GOSUMDB=sum.golang.google.cn
    ALPINE_MIRROR=https://mirrors.aliyun.com/alpine
    DEBIAN_APT_MIRROR=https://mirrors.aliyun.com/debian
    DEBIAN_SECURITY_APT_MIRROR=https://mirrors.aliyun.com/debian-security
    PIP_INDEX_URL=https://mirrors.aliyun.com/pypi/simple
    UV_DEFAULT_INDEX=https://mirrors.aliyun.com/pypi/simple
    """
)


class DockerPolicyTests(unittest.TestCase):
    def test_valid_policy_files_have_no_issues(self) -> None:
        issues = self.verify(
            files={
                "services/auth/Dockerfile": VALID_GO_DOCKERFILE,
                "services/auth/.dockerignore": ".git\nbin/\n",
                "services/parser/Dockerfile": VALID_PARSER_DOCKERFILE,
                "services/parser/.dockerignore": ".venv/\n",
                "deploy/docker-compose.yml": VALID_COMPOSE,
                "deploy/.env.china.example": VALID_CHINA_ENV,
            }
        )

        self.assertEqual([], issues)

    def test_dockerfile_regressions_are_reported(self) -> None:
        dockerfile = VALID_GO_DOCKERFILE.replace(
            "FROM ${IMAGE_REGISTRY_PREFIX}alpine:${ALPINE_VERSION}",
            "FROM alpine:latest",
        ).replace("ARG GOSUMDB=sum.golang.org", "ARG GOSUMDB=off")

        issues = self.verify(
            files={
                "services/auth/Dockerfile": "# syntax=docker/dockerfile:1\n" + dockerfile,
            }
        )

        self.assertIssueContains(issues, "external Dockerfile frontend")
        self.assertIssueContains(issues, "GOSUMDB=off")
        self.assertIssueContains(issues, "base image `alpine:latest` must use ${IMAGE_REGISTRY_PREFIX}")
        self.assertIssueContains(issues, "must not use latest")
        self.assertIssueContains(issues, "sibling .dockerignore")

    def test_compose_regressions_are_reported(self) -> None:
        compose = VALID_COMPOSE.replace(
            "image: ${POSTGRES_IMAGE:-postgres:16-alpine}",
            "image: postgres:latest",
        ).replace(
            "GOSUMDB: ${GO_DOCKER_GOSUMDB:-sum.golang.org}",
            "GOSUMDB: ${GO_DOCKER_GOSUMDB:-off}",
        )

        issues = self.verify(files={"deploy/docker-compose.yml": compose})

        self.assertIssueContains(issues, "must not use latest")
        self.assertIssueContains(issues, "GOSUMDB=off")
        self.assertIssueContains(issues, "Go build args must default")

    def test_parser_recursive_chown_regression_is_reported(self) -> None:
        dockerfile = VALID_PARSER_DOCKERFILE.replace(
            "COPY --from=builder --chown=parser:parser /app /app",
            "COPY --from=builder /app /app",
        ).replace(
            "&& chown parser:parser /tmp/parser-cache",
            "&& chown -R parser:parser /app /tmp/parser-cache",
        )

        issues = self.verify(
            files={
                "services/parser/Dockerfile": dockerfile,
                "services/parser/.dockerignore": ".venv/\n",
            }
        )

        self.assertIssueContains(issues, "COPY --chown for /app")

    def test_china_env_regressions_are_reported(self) -> None:
        china_env = VALID_CHINA_ENV.replace(
            "GO_DOCKER_GOSUMDB=sum.golang.google.cn",
            "GO_DOCKER_GOSUMDB=off",
        ).replace(
            "POSTGRES_IMAGE=docker.m.daocloud.io/library/postgres:16-alpine",
            "POSTGRES_IMAGE=postgres:latest",
        )

        issues = self.verify(files={"deploy/.env.china.example": china_env})

        self.assertIssueContains(issues, "POSTGRES_IMAGE")
        self.assertIssueContains(issues, "must not use latest")
        self.assertIssueContains(issues, "must not disable Go checksum verification")

    def verify(self, *, files: dict[str, str]) -> list[str]:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            for relative, content in files.items():
                path = root / relative
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_text(content, encoding="utf-8")
            return verify_docker_policy(root)

    def assertIssueContains(self, issues: list[str], expected: str) -> None:
        self.assertTrue(
            any(expected in issue for issue in issues),
            f"Expected issue containing {expected!r}, got: {issues!r}",
        )


if __name__ == "__main__":
    unittest.main()
