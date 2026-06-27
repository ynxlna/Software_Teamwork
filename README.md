# Software Teamwork

A full-stack project managed with [Trellis](https://github.com/trellis-workflow).

## Getting Started

> Project setup instructions to be added.

## Conventions

### Collaboration

- Work in a personal fork, not directly in the main repository.
- Create a dedicated branch from the latest `upstream/develop`.
- Open PRs to `develop` only.
- Keep `main` for release merges from `develop`.

Full guide and GitHub CLI workflow: [CONTRIBUTING.md](CONTRIBUTING.md)

Repository guard settings: [docs/repository-settings.md](docs/repository-settings.md)

Auto label configuration: [.github/labeler.json](.github/labeler.json)

### Commit Messages

All commits follow [Conventional Commits](https://www.conventionalcommits.org/). Quick reference:

```
<type>(<scope>): <subject>
```

Common types: `feat`, `fix`, `refactor`, `test`, `docs`, `chore`, `perf`, `revert`

Full spec: [`.trellis/spec/guides/commit-convention.md`](.trellis/spec/guides/commit-convention.md)

### Code Style

- Backend: [`.trellis/spec/backend/`](.trellis/spec/backend/)
- Frontend: [`.trellis/spec/frontend/`](.trellis/spec/frontend/)

## Project Structure

```
.trellis/   # Workflow, tasks, and spec
```
