# karaoke-mic-icon Specification

## Purpose
The icon hosts draw for a karaoke Library and for a karaoke song without artwork.

## Requirements

### Requirement: Karaoke declares the mic icon

The karaoke bundle SHALL declare `LibraryDefaults.Icon` as `mic`, so hosts draw a microphone for a
karaoke Library by default and as the placeholder for a karaoke song without artwork.

#### Scenario: Bundle declares the icon

- **WHEN** the karaoke bundle is built
- **THEN** its `LibraryDefaults.Icon` is `mic`
