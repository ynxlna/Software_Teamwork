#!/usr/bin/env python3
"""Verify repository Dockerfiles and Compose files follow local build policy."""

from __future__ import annotations

import argparse
import re
import sys
from pathlib import Path


SCAN_ROOTS = ("deploy", "services")
EXPECTED_CHINA_ENV = {
    "DOCKER_IMAGE_REGISTRY_PREFIX": "docker.m.daocloud.io/library/",
    "POSTGRES_IMAGE": "docker.m.daocloud.io/library/postgres:16-alpine",
    "REDIS_IMAGE": "docker.m.daocloud.io/library/redis:7-alpine",
    "QDRANT_IMAGE": "docker.m.daocloud.io/qdrant/qdrant:v1.18.2",
    "MINIO_IMAGE": "docker.m.daocloud.io/minio/minio:RELEASE.2025-09-07T16-13-09Z",
    "MINIO_MC_IMAGE": "docker.m.daocloud.io/minio/mc:RELEASE.2025-08-13T08-35-41Z",
    "GO_DOCKER_GOPROXY": "https://goproxy.cn,direct",
    "GO_DOCKER_GOSUMDB": "sum.golang.google.cn",
    "ALPINE_MIRROR": "https://mirrors.aliyun.com/alpine",
    "DEBIAN_APT_MIRROR": "https://mirrors.aliyun.com/debian",
    "DEBIAN_SECURITY_APT_MIRROR": "https://mirrors.aliyun.com/debian-security",
    "PIP_INDEX_URL": "https://mirrors.aliyun.com/pypi/simple",
    "UV_DEFAULT_INDEX": "https://mirrors.aliyun.com/pypi/simple",
}
EXPECTED_IMAGE_DEFAULTS = {
    "POSTGRES_IMAGE": "postgres:16-alpine",
    "REDIS_IMAGE": "redis:7-alpine",
    "QDRANT_IMAGE": "qdrant/qdrant:v1.18.2",
    "MINIO_IMAGE": "minio/minio:RELEASE.2025-09-07T16-13-09Z",
    "MINIO_MC_IMAGE": "minio/mc:RELEASE.2025-08-13T08-35-41Z",
}
GO_PROXY_ARG = "ARG GOPROXY=https://proxy.golang.org,direct"
GO_SUMDB_ARG = "ARG GOSUMDB=sum.golang.org"
GO_PROXY_COMPOSE = "GOPROXY: ${GO_DOCKER_GOPROXY:-https://proxy.golang.org,direct}"
GO_SUMDB_COMPOSE = "GOSUMDB: ${GO_DOCKER_GOSUMDB:-sum.golang.org}"
IMAGE_REGISTRY_COMPOSE = "IMAGE_REGISTRY_PREFIX: ${DOCKER_IMAGE_REGISTRY_PREFIX:-}"
PARSER_COMPOSE_ARGS = (
    "APT_MIRROR: ${DEBIAN_APT_MIRROR:-}",
    "APT_SECURITY_MIRROR: ${DEBIAN_SECURITY_APT_MIRROR:-}",
    "PIP_INDEX_URL: ${PIP_INDEX_URL:-}",
    "UV_DEFAULT_INDEX: ${UV_DEFAULT_INDEX:-}",
    "UV_INDEX: ${UV_INDEX:-}",
)


def verify_docker_policy(root: Path) -> list[str]:
    issues: list[str] = []
    for dockerfile in discover_dockerfiles(root):
        issues.extend(validate_dockerfile(root, dockerfile))
    for compose_file in discover_compose_files(root):
        issues.extend(validate_compose_file(root, compose_file))
    issues.extend(validate_china_env(root))
    return issues


def discover_dockerfiles(root: Path) -> list[Path]:
    paths: list[Path] = []
    for scan_root in SCAN_ROOTS:
        directory = root / scan_root
        if not directory.exists():
            continue
        paths.extend(
            path
            for path in directory.rglob("Dockerfile*")
            if path.is_file() and ".git" not in path.parts
        )
    return sorted(paths)


def discover_compose_files(root: Path) -> list[Path]:
    paths: list[Path] = []
    for scan_root in SCAN_ROOTS:
        directory = root / scan_root
        if not directory.exists():
            continue
        paths.extend(
            path
            for path in directory.rglob("docker-compose*.yml")
            if path.is_file() and ".git" not in path.parts
        )
        paths.extend(
            path
            for path in directory.rglob("docker-compose*.yaml")
            if path.is_file() and ".git" not in path.parts
        )
    return sorted(paths)


def validate_dockerfile(root: Path, dockerfile: Path) -> list[str]:
    rel = dockerfile.relative_to(root).as_posix()
    content = dockerfile.read_text(encoding="utf-8")
    issues: list[str] = []

    if re.search(r"(?m)^\s*#\s*syntax=", content):
        issues.append(
            f"{rel}: do not require an external Dockerfile frontend; broken daemon mirrors can fail before local Dockerfile logic runs"
        )

    if "GOSUMDB=off" in content or re.search(r"GOSUMDB\s*[:=]\s*off\b", content):
        issues.append(f"{rel}: must not disable Go checksum verification with GOSUMDB=off")

    from_images = collect_base_from_images(content)
    if from_images and "ARG IMAGE_REGISTRY_PREFIX=" not in content:
        issues.append(f"{rel}: Dockerfiles with external FROM images must define ARG IMAGE_REGISTRY_PREFIX=")

    for line_no, image in from_images:
        if image == "scratch":
            continue
        if "${IMAGE_REGISTRY_PREFIX}" not in image:
            issues.append(f"{rel}:{line_no}: base image `{image}` must use ${{IMAGE_REGISTRY_PREFIX}}")
        image_without_prefix = image.replace("${IMAGE_REGISTRY_PREFIX}", "")
        if uses_latest_tag(image_without_prefix):
            issues.append(f"{rel}:{line_no}: base image `{image}` must not use latest")
        if not has_explicit_tag_or_digest(image_without_prefix):
            issues.append(f"{rel}:{line_no}: base image `{image}` must use an explicit tag or digest")

    if is_go_dockerfile(content):
        issues.extend(validate_go_dockerfile(rel, content))
    if is_parser_dockerfile(rel, content):
        issues.extend(validate_parser_dockerfile(rel, content))
    if dockerfile.name == "Dockerfile.host" and "ARG POSTGRES_VERSION=16-alpine" not in content:
        issues.append(f"{rel}: QA host Dockerfile must keep ARG POSTGRES_VERSION=16-alpine")

    if not (dockerfile.parent / ".dockerignore").exists():
        issues.append(f"{rel}: Docker build context must have a sibling .dockerignore")

    return issues


def collect_base_from_images(content: str) -> list[tuple[int, str]]:
    stage_aliases: set[str] = set()
    from_images: list[tuple[int, str]] = []
    for line_no, line in enumerate(content.splitlines(), start=1):
        stripped = line.strip()
        if not stripped or stripped.startswith("#"):
            continue
        tokens = stripped.split()
        if not tokens or tokens[0].upper() != "FROM":
            continue

        image_index = 1
        while image_index < len(tokens) and tokens[image_index].startswith("--"):
            image_index += 1
        if image_index >= len(tokens):
            continue

        image = tokens[image_index]
        if image.lower() not in stage_aliases:
            from_images.append((line_no, image))

        for index, token in enumerate(tokens):
            if token.upper() == "AS" and index + 1 < len(tokens):
                stage_aliases.add(tokens[index + 1].lower())
                break
    return from_images


def is_go_dockerfile(content: str) -> bool:
    return "golang:" in content or "GOPROXY" in content or "go build" in content


def is_parser_dockerfile(rel: str, content: str) -> bool:
    return rel == "services/parser/Dockerfile" or "uv sync" in content or "parser-service" in content


def validate_go_dockerfile(rel: str, content: str) -> list[str]:
    issues: list[str] = []
    required = (
        GO_PROXY_ARG,
        GO_SUMDB_ARG,
        "ARG ALPINE_MIRROR=",
        "--mount=type=cache,target=/go/pkg/mod",
        "--mount=type=cache,target=/root/.cache/go-build",
    )
    for needle in required:
        if needle not in content:
            issues.append(f"{rel}: missing Go Docker build policy `{needle}`")
    if "apk add" in content and "--mount=type=cache,target=/var/cache/apk" not in content:
        issues.append(f"{rel}: apk installs must use a BuildKit cache mount for /var/cache/apk")
    return issues


def validate_parser_dockerfile(rel: str, content: str) -> list[str]:
    issues: list[str] = []
    required = (
        "ARG APT_MIRROR=",
        "ARG APT_SECURITY_MIRROR=",
        "ARG UV_DEFAULT_INDEX=",
        "ARG PIP_INDEX_URL=",
        "--mount=type=cache,target=/var/cache/apt",
        "--mount=type=cache,target=/root/.cache/uv",
        "--mount=type=cache,target=/root/.cache/pip",
        "mkdir -p /tmp/parser-cache",
        "COPY --from=builder --chown=parser:parser /app /app",
        'CMD ["parser-service"]',
    )
    for needle in required:
        if needle not in content:
            issues.append(f"{rel}: missing Parser Docker build policy `{needle}`")
    if re.search(r'(?m)^\s*(CMD|ENTRYPOINT)\s+\["uv"', content):
        issues.append(f"{rel}: runtime image must call parser-service directly, not uv run")
    if re.search(r"chown\s+-R\s+\S+\s+/app\b", content):
        issues.append(
            f"{rel}: runtime image must use COPY --chown for /app instead of recursive chown"
        )
    return issues


def validate_compose_file(root: Path, compose_file: Path) -> list[str]:
    rel = compose_file.relative_to(root).as_posix()
    content = compose_file.read_text(encoding="utf-8")
    issues: list[str] = []

    for line_no, image in collect_compose_images(content):
        if uses_latest_tag(image):
            issues.append(f"{rel}:{line_no}: Compose image `{image}` must not use latest")
        issues.extend(validate_compose_image_default(rel, line_no, image))

    if "GOSUMDB=off" in content or re.search(r"GOSUMDB\s*:\s*(?:\$\{[^}]*:-)?off\b", content):
        issues.append(f"{rel}: must not disable Go checksum verification with GOSUMDB=off")

    if "build:" in content and IMAGE_REGISTRY_COMPOSE not in content:
        issues.append(f"{rel}: Compose builds must pass `{IMAGE_REGISTRY_COMPOSE}`")

    if "GOPROXY:" in content and GO_PROXY_COMPOSE not in content:
        issues.append(f"{rel}: Go build args must default to `{GO_PROXY_COMPOSE}`")
    if "GOSUMDB:" in content and GO_SUMDB_COMPOSE not in content:
        issues.append(f"{rel}: Go build args must default to `{GO_SUMDB_COMPOSE}`")

    if is_parser_compose_file(content):
        for needle in PARSER_COMPOSE_ARGS:
            if needle not in content:
                issues.append(f"{rel}: Parser Compose build must pass `{needle}`")

    return issues


def collect_compose_images(content: str) -> list[tuple[int, str]]:
    images: list[tuple[int, str]] = []
    for line_no, line in enumerate(content.splitlines(), start=1):
        match = re.match(r"^\s*image:\s*(.+?)\s*(?:#.*)?$", line)
        if not match:
            continue
        image = match.group(1).strip().strip("'\"")
        images.append((line_no, image))
    return images


def validate_compose_image_default(rel: str, line_no: int, image: str) -> list[str]:
    issues: list[str] = []
    for variable, expected_default in EXPECTED_IMAGE_DEFAULTS.items():
        expected_expr = f"${{{variable}:-{expected_default}}}"
        if variable in image:
            if image != expected_expr:
                issues.append(
                    f"{rel}:{line_no}: `{variable}` must default to pinned `{expected_default}`"
                )
            return issues
        if image == expected_default:
            issues.append(
                f"{rel}:{line_no}: `{expected_default}` must be exposed through `{expected_expr}`"
            )
            return issues
    if re.match(r"^[^$].*:[^@]+$", image) and not image.startswith("${"):
        issues.append(
            f"{rel}:{line_no}: Compose image `{image}` must be exposed through a pinned override variable"
        )
    return issues


def is_parser_compose_file(content: str) -> bool:
    return "services/parser" in content or "PARSER_" in content


def validate_china_env(root: Path) -> list[str]:
    env_file = root / "deploy" / ".env.china.example"
    if not env_file.exists():
        return []

    content = env_file.read_text(encoding="utf-8")
    values = parse_env_file(content)
    issues: list[str] = []

    for key, expected in EXPECTED_CHINA_ENV.items():
        actual = values.get(key)
        if actual != expected:
            issues.append(
                f"deploy/.env.china.example: `{key}` must stay `{expected}` for the documented mainland China Docker path"
            )

    for key, value in values.items():
        if key.endswith("_IMAGE") and uses_latest_tag(value):
            issues.append(f"deploy/.env.china.example: `{key}` must not use latest")
    if values.get("GO_DOCKER_GOSUMDB") == "off":
        issues.append("deploy/.env.china.example: must not disable Go checksum verification")

    return issues


def parse_env_file(content: str) -> dict[str, str]:
    values: dict[str, str] = {}
    for line in content.splitlines():
        stripped = line.strip()
        if not stripped or stripped.startswith("#") or "=" not in stripped:
            continue
        key, value = stripped.split("=", 1)
        values[key.strip()] = value.strip().strip("'\"")
    return values


def uses_latest_tag(image: str) -> bool:
    return ":latest" in image or ":-latest" in image or image.endswith(":latest")


def has_explicit_tag_or_digest(image: str) -> bool:
    if "@" in image:
        return True
    last_component = image.rsplit("/", maxsplit=1)[-1]
    return ":" in last_component


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--root",
        type=Path,
        default=Path.cwd(),
        help="repository root; defaults to current working directory",
    )
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    root = args.root.resolve()
    issues = verify_docker_policy(root)
    if issues:
        for issue in issues:
            print(f"- {issue}", file=sys.stderr)
        return 1
    print("Docker policy checks passed.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
