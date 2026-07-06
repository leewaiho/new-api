# CLAUDE.md — Project Conventions for new-api

@AGENTS.md

## Branch Policy (fork-specific)

**release/test is a mandatory gate for release/prod.**

- All changes destined for `release/prod` MUST be merged into `release/test`
  first and verified in the :3011 test environment.
- Do NOT push directly to `release/prod` unless the same changes have already
  been tested on `release/test` and the user has explicitly authorized a
  production deploy.
- Hotfix workflow: branch off `release/test`, merge to `release/test`, deploy
  to :3011, verify, then fast-forward or merge `release/test` into
  `release/prod`.

### CI mapping

| Branch          | GHCR tag   | Environment | Port |
|-----------------|------------|-------------|------|
| `release/prod`  | `:latest`  | Production  | 3010 |
| `release/test`  | `:test`    | Test        | 3011 |

See `scripts/deploy/README.md` for the deployment and database-copy workflow.

## Claude Code

- Follow the shared project instructions imported from `AGENTS.md`.
