#!/usr/bin/env python3
"""Warn agents before they run `git commit`.

This hook is intentionally advisory. Agent platforms can inject the message
before shell execution, but they cannot prove what the model has read. The
repository policy lives in the referenced docs; this hook creates a mandatory
prompt checkpoint right before a commit command is attempted.
"""

from __future__ import annotations

import json
import os
import shlex
import sys
from pathlib import Path
from typing import Any


GIT_OPTIONS_WITH_VALUE = {
    "-C",
    "-c",
    "--git-dir",
    "--work-tree",
    "--namespace",
    "--config-env",
    "--exec-path",
}

REQUIRED_DOCS = (
    "CONTRIBUTING.md",
    ".trellis/spec/guides/commit-convention.md",
    ".agents/git-commit-checklist.md",
)


def _string_value(value: Any) -> str | None:
    if isinstance(value, str):
        stripped = value.strip()
        return stripped or None
    return None


def _load_input() -> dict[str, Any]:
    try:
        data = json.loads(sys.stdin.read() or "{}")
    except (json.JSONDecodeError, ValueError):
        return {}
    return data if isinstance(data, dict) else {}


def _extract_command_candidates(data: Any) -> list[str]:
    candidates: list[str] = []
    if isinstance(data, dict):
        for key, value in data.items():
            if key in {"command", "cmd", "shell_command", "shellCommand", "script"}:
                text = _string_value(value)
                if text:
                    candidates.append(text)
            elif key in {"arguments", "args"}:
                text = _string_value(value)
                if text:
                    candidates.append(text)
                    try:
                        nested = json.loads(text)
                    except (json.JSONDecodeError, ValueError):
                        nested = None
                    if nested is not None:
                        candidates.extend(_extract_command_candidates(nested))
                else:
                    candidates.extend(_extract_command_candidates(value))
            elif isinstance(value, (dict, list)):
                candidates.extend(_extract_command_candidates(value))
    elif isinstance(data, list):
        for item in data:
            candidates.extend(_extract_command_candidates(item))
    return candidates


def _token_is_git(token: str) -> bool:
    cleaned = token.strip("\"'")
    return Path(cleaned).name == "git"


def _is_option_with_inline_value(token: str) -> bool:
    return any(token.startswith(f"{option}=") for option in GIT_OPTIONS_WITH_VALUE)


def _tokens_contain_git_commit(tokens: list[str]) -> bool:
    for index, token in enumerate(tokens):
        if not _token_is_git(token):
            continue

        cursor = index + 1
        while cursor < len(tokens):
            current = tokens[cursor]
            if current == "--":
                cursor += 1
                break
            if current in GIT_OPTIONS_WITH_VALUE:
                cursor += 2
                continue
            if _is_option_with_inline_value(current):
                cursor += 1
                continue
            if current.startswith("-"):
                cursor += 1
                continue
            break

        if cursor < len(tokens) and tokens[cursor] == "commit":
            return True
    return False


def _command_is_git_commit(command: str) -> bool:
    try:
        tokens = shlex.split(command, posix=os.name != "nt")
    except ValueError:
        return "git commit" in command
    return _tokens_contain_git_commit(tokens)


def _find_repo_root(start: Path) -> Path:
    current = start.resolve()
    while current != current.parent:
        if (current / ".git").exists():
            return current
        current = current.parent
    return start.resolve()


def _build_notice(root: Path) -> str:
    missing = [path for path in REQUIRED_DOCS if not (root / path).is_file()]
    missing_line = (
        "\nMissing expected docs: " + ", ".join(missing)
        if missing
        else ""
    )
    docs = "\n".join(f"- `{path}`" for path in REQUIRED_DOCS)
    return (
        "<agent-git-commit-reminder>\n"
        "Before running `git commit` in this repository, confirm you have read "
        "these files in the current session:\n"
        f"{docs}\n"
        "Then verify the branch/PR target, staged files, quality checks, and "
        "Conventional Commit message before committing."
        f"{missing_line}\n"
        "</agent-git-commit-reminder>"
    )


def main() -> int:
    if os.environ.get("TRELLIS_HOOKS") == "0" or os.environ.get("TRELLIS_DISABLE_HOOKS") == "1":
        return 0

    hook_input = _load_input()
    commands = _extract_command_candidates(hook_input)
    if not any(_command_is_git_commit(command) for command in commands):
        return 0

    cwd = Path(_string_value(hook_input.get("cwd")) or os.getcwd())
    root = _find_repo_root(cwd)
    notice = _build_notice(root)
    output = {
        "hookSpecificOutput": {
            "hookEventName": "PreToolUse",
            "permissionDecision": "allow",
            "additionalContext": notice,
        },
        "permission": "allow",
        "message": notice,
        "additional_context": notice,
    }
    print(json.dumps(output, ensure_ascii=False))
    return 0


if __name__ == "__main__":
    sys.exit(main())
