# Karaoke songs play on Select

Ticket: CORE-3307 (`queue:core:main`). Builds on plugin-sdk v0.4.13 `Engagement.SelectPlays`.

## Why

People pick a karaoke song by title to sing it. TV clients today guess songs from artless audio, so
video songs and songs with thumbnails open a detail page instead of playing.

## What Changes

- Require plugin-sdk v0.4.13 and declare `Engagement.SelectPlays: true`.

## Failure modes

- A host that predates the flag ignores it and clients keep their own Select behaviour.

## Acceptance evidence

A bundle test that the flag is declared; `script/gate.sh` on the branch head.

## Rollback

Revert the commit; the SDK bump is additive.
