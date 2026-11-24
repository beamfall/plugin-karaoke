# Karaoke offers the party and names its parts

Ticket: CORE-3487 (`queue:core:main`). Builds on plugin-sdk v0.4.16 `FeatureParty` and
`Bundle.PartyTerms` (CORE-3486, ADR-0237).

## Why

Core's party session is generic (ADR-0237), but its hosts still hard-code "Karaoke party" and singer
wording. Karaoke is the plugin that offers a party, so it should say so and supply the words.

## What Changes

- Require plugin-sdk v0.4.16.
- Declare `party` in `FeatureEligibility`.
- Declare `PartyTerms` in English and French: Karaoke party, singer(s), song(s), stage.

## Failure modes

- A core that predates the fields ignores them and keeps its current party wording and eligibility.
- `sdk.Version` stays 0.4.7 through v0.4.16, so the bundle cannot floor on party support (CORE-0086).

## Acceptance evidence

Bundle tests for the eligibility and every term in en and fr; `script/gate.sh` on the branch head.

## Rollback

Revert the commit; the SDK bump is additive.
