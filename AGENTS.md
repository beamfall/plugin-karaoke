# AGENTS.md

## Corvint context

Before editing, read `corvint context "<task>"` or `corvint impact <path>...`; before choosing tests,
run `corvint affected --base <origin/main sha>`. **Verify:** `script/gate.sh`.

## Spec-driven changes (OpenSpec)

New or changed behaviour needs an OpenSpec change in `openspec/changes/<id>/` committed before the
code: `proposal.md` (naming its `CORE` ticket), the spec delta and `tasks.md`. A fix that restores an
existing requirement or ADR cites it instead. Tickets stay the work queue; `tasks.md` is a
checklist. Existing ADRs and `docs/` stay canonical; do not backfill specs. Run
`openspec validate --all --strict` before review and `openspec archive <id>` after landing, before
completing the ticket. Agent commands are generated per checkout with `openspec update`. The
workflow is specified in beamfall/core, `openspec/changes/adopt-openspec/`.
