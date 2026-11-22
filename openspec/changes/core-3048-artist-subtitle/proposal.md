## Why

Ticket: CORE-3048 in `queue:core:main`. A Karaoke Library stores each song's artist in the flat
`artist` Media field, but the bundle declares no MediaSubtitle, so song cards and party rotation
lists show the title with no artist.

## What Changes

- The bundle declares `MediaSubtitle` as the `{artist}` field, so a host renders the artist under
  each song's title. A song with no artist renders no subtitle.
- The artist stays a flat field: the scan still links no artist Entity (the ticket's first
  acceptance line asked for one; the flat field is the plugin's documented design).
