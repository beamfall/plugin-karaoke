# Karaoke declares the mic icon

Ticket: CORE-3336 (`queue:core:main`). Builds on plugin-sdk v0.4.14 `LibraryDefaults.Icon`.

## Why

Karaoke songs without artwork show the generic music note, and a karaoke Library gets the generic
"layers" icon. Hosts can now draw a plugin-declared icon for both.

## What Changes

- Require plugin-sdk v0.4.14 and declare `LibraryDefaults.Icon: "mic"`.

## Failure modes

- A host that predates the field ignores it and keeps its own default icon and placeholder.

## Acceptance evidence

A bundle test that the icon is declared; `script/gate.sh` on the branch head.

## Rollback

Revert the commit; the SDK bump is additive.
