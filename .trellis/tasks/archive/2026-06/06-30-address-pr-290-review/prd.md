# Address PR 290 Review

## Background

PR #290 received Codex PR Review feedback for F-014 report frontend integration.
This follow-up keeps the same PR branch and addresses review findings without
changing backend or Gateway OpenAPI contracts.

## Requirements

- If report draft creation succeeds but report job creation fails, do not leave
  the UI in a state where retrying creates another draft by default.
- Allow users to retry job creation against the existing server report draft.
- Keep Gateway errors visible, including `message` and `requestId`.
- Make report deletion and template deletion mutation failures visible to users.
- Re-check the Frontend CI/format concern and document the actual state in the
  PR if no code change is required.

## Scope

- Frontend report pages under `apps/web/src/pages/reports`.
- Existing report feature hooks and tests under `apps/web/src/features/reports`.
- PR #290 body may be updated if verification/risk notes change.

## Out Of Scope

- Backend implementation.
- Gateway OpenAPI changes.
- Mass-formatting unrelated frontend files unless CI proves it is required.

## Acceptance Criteria

- Retrying after job creation failure reuses the existing draft report where
  possible.
- Delete failures show formatted Gateway errors and do not silently disappear.
- Unit/component tests cover the changed states.
- Relevant frontend checks pass, or any baseline failure is explicitly explained.
