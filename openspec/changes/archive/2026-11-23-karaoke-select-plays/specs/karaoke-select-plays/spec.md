## ADDED Requirements

### Requirement: Karaoke songs play on Select

The karaoke bundle SHALL declare `Engagement.SelectPlays` so hosts tell clients that a karaoke song,
audio or video, starts when it is selected.

#### Scenario: Bundle declares the flag

- **WHEN** the karaoke bundle is built
- **THEN** its `Engagement.SelectPlays` is true
