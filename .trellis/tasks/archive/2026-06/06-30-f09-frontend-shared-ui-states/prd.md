# F-09 Frontend Shared UI States

## Goal

Implement issue #116 by consolidating repeated frontend state UI into shared components and applying them across knowledge, QA, and report pages so the management workspace has consistent loading, empty, error, progress, confirmation, and notification patterns.

## Requirements

- Base work on the latest `upstream/develop`; branch: `Frontend/refactor/shared-ui-states`.
- Keep frontend code under `apps/web/src/`.
- Create shared components under `apps/web/src/components/common/` and avoid imports from feature modules.
- Reuse shared state components in at least one knowledge page, one QA page, and one report page.
- Cover loading, empty, error, forbidden-ready primitives where applicable, plus partial/progress and destructive confirmation.
- Keep the UI dense and operational, using existing shadcn/Radix/lucide/Tailwind patterns.
- Preserve existing feature behavior and API ownership; do not move server state into Zustand.
- Improve responsive robustness so text does not overflow or overlap on mobile and desktop widths.

## Acceptance Criteria

- [ ] `bun run --cwd apps/web check` passes.
- [ ] `bun run --cwd apps/web build` passes.
- [ ] `git diff --check` passes.
- [ ] Shared state components are used by knowledge, QA, and report surfaces.
- [ ] Shared UI does not import feature modules.
- [ ] Destructive confirmation and progress/partial status have consistent styling and copy structure.

## Definition of Done

- Focused frontend changes only.
- Required frontend checks run and reported.
- PR can target `Sakayori-Iroha-168/Software_Teamwork:develop`.
- Issue #116 can be linked with `Closes #116`.

## Technical Approach

Create a small shared state toolkit in `components/common`: page/section state panel, inline alert, loading rows/skeleton helpers, progress summary, and confirm dialog. Replace local one-off state markup in `KnowledgeDocumentsPage`, `KnowledgeSearchPage`, chat components/page where useful, and `ReportRecordsPage` or `ReportGeneratePage` without changing backend contracts.

## Out of Scope

- No marketing or landing page work.
- No backend API changes.
- No generated OpenAPI edits.
- No broad redesign of navigation or product flow beyond consistency fixes.

## Technical Notes

- Issue: https://github.com/Sakayori-Iroha-168/Software_Teamwork/issues/116
- Authority: `.trellis/spec/frontend/index.md`, `component-guidelines.md`, `directory-structure.md`, `quality-guidelines.md`, `state-management.md`.
- Project docs search found no additional F-09-specific doc under `docs/`.
- Existing `apps/web/src/components/common`, `data-table`, `file-upload`, `markdown`, `rich-editor`, and `charts` are mostly placeholders; state UI currently lives in pages/components.
