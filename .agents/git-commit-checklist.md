# Agent Git Commit Checklist

This file is for AI agents working in this repository. Read it before running
`git commit`.

## Required Reading

Before committing, read these files in the current session:

- [CONTRIBUTING.md](../CONTRIBUTING.md)
- [.trellis/spec/guides/commit-convention.md](../.trellis/spec/guides/commit-convention.md)

## Commit Gate

Do not run `git commit` until you have checked:

1. The work is on a dedicated branch based on the latest `develop`. Never
   create a routine PR to `main`.
2. The commit includes only files related to the current task. Do not stage
   private local notes, Trellis runtime files, or unrelated user changes.
3. Quality checks requested by the task or relevant docs have run, or the final
   response clearly states why they were not run.
4. The commit message follows Conventional Commits:

   ```text
   <type>(<scope>): <subject>
   ```

5. The subject is imperative, lower-case after the type, has no trailing
   period, and keeps the first line at 72 characters or less.

## Good Messages

```text
docs(workflow): add agent commit checklist
chore(hooks): remind agents before git commit
docs(workflow): merge cli workflow into contributing
```

## Stop Conditions

Stop and inspect the repo instead of committing when:

- `git status` shows unrelated modified files.
- The branch was not created from the latest `develop`.
- The PR target branch is unclear.
- The only possible commit message would be vague, such as `update`, `wip`,
  `fix bug`, or `changes`.
