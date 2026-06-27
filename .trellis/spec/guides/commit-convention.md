# Commit Convention

> Follow [Conventional Commits](https://www.conventionalcommits.org/) for all commits.

---

## Format

```
<type>(<scope>): <subject>

[optional body]

[optional footer(s)]
```

- **type**: lowercase, from the list below
- **scope**: optional, the module/layer affected (e.g. `auth`, `api`, `ui`)
- **subject**: imperative mood, no capital first letter, no period at end, ≤ 72 chars total for the first line

---

## Before Commit

Before running `git commit`, read the repository workflow and confirm the branch
and PR rules still match your current work:

- [CONTRIBUTING.md](../../../CONTRIBUTING.md)
- [.agents/git-commit-checklist.md](../../../.agents/git-commit-checklist.md)

Do not commit unrelated user changes, private local notes, or Trellis runtime
files. If checks cannot be run, record that explicitly in the final response.

---

## Types

| Type | When to use |
|------|-------------|
| `feat` | New feature |
| `fix` | Bug fix |
| `refactor` | Code change that neither fixes a bug nor adds a feature |
| `test` | Adding or updating tests |
| `docs` | Documentation only |
| `chore` | Build process, tooling, config, CI/CD |
| `perf` | Performance improvement |
| `style` | Formatting, white-space (no logic change) |
| `revert` | Revert a previous commit |

---

## Rules

1. Use present tense, imperative: "add feature" not "added feature"
2. First line ≤ 72 characters
3. Separate body from subject with a blank line
4. Breaking changes: append `!` after type/scope, e.g. `feat(api)!: ...`, and add `BREAKING CHANGE:` footer
5. Reference issues/PRs in footer: `Closes #123`

---

## Examples

```
feat(auth): add JWT refresh token endpoint

fix(db): handle null pointer in user query

chore: add GitHub Actions CI workflow

feat(api)!: rename /users to /accounts

BREAKING CHANGE: all clients must update endpoint URL
Closes #42
```

---

## Bad Examples (avoid)

```
# Too vague
fix: stuff

# Wrong tense
feat: added login page

# Capital subject / period
Fix: Resolve login bug.

# No type
update config
```
