# Upstream PR Plan

This document tracks the upstream contribution plan for the NewAPI and CC Switch
changes developed in the local fork. Only PR #1 is ready to submit immediately;
all other items should stay local until the release/test environment has run
stably for a longer period.

| Order | Project | Usage scenario | Proposed PR scope | Submit now? | Notes |
| ---: | --- | --- | --- | --- | --- |
| 1 | NewAPI | Prevent channel validation from panicking when a nil channel reaches `validateChannel` | `fix(channel): guard nil channel in validateChannel` | Yes | Small isolated bugfix. Submit first. |
| 2 | NewAPI | Let clients discover model capabilities from `/v1/models` and `/api/pricing` | `feat(model): expose model metadata in pricing and model list APIs` | Not yet | Includes `input_modalities`, `output_modalities`, `capabilities`, `context_length`, and `max_output_tokens`. Keep the OpenAI-compatible DTO narrow and expose NewAPI metadata through a separate anti-corruption DTO. |
| 3 | NewAPI | Allow admins to configure model input/output/capability metadata in the model management UI | Same PR as #2 | Not yet | The API output needs a configuration surface, so DB fields, model CRUD, and UI should move together. |
| 4 | NewAPI | Fetch upstream model lists for Advanced Custom channels | `feat(channel): support model fetching for Advanced Custom channels` | Not yet | Generic Advanced Custom capability. Keep separate from quick setup templates. |
| 5 | NewAPI | Provide reusable route templates for Advanced Custom setup | `feat(channel): add Advanced Custom route quick setup templates` | Not yet | Present as route templates / quick setup. Avoid coding-plan-specific wording or provider-specific assumptions. |
| 6 | CC Switch | Propagate upstream model input capabilities into Codex catalog | `feat(codex): propagate model input modalities into Codex catalog` | Not yet | Combine upstream capability parsing, Codex catalog image-input writing, manual fallback selection, and auto-fill after model fetching into one scenario-level PR. |
| - | CC Switch | Tray build diagnostics | No upstream PR | No | Build hostname, CST timestamp, and click-to-copy version info are fork-only diagnostics. |
| - | NewAPI | Homelab / fork deployment | No upstream PR | No | `Dockerfile.test`, `docker-compose.test.yml`, GHCR workflows, local DB sync scripts, and fork maintenance docs stay in the fork. |

## Current immediate upstream target

Submit only PR #1 first. Do not submit the model metadata, Advanced Custom, or CC
Switch Codex catalog changes until they have been exercised locally and in the
`:3011` test environment.

