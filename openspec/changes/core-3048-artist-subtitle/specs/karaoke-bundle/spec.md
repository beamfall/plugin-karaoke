## ADDED Requirements

### Requirement: Song cards name the artist

The bundle SHALL declare a MediaSubtitle that renders the `artist` Media field, so a host shows a
song's artist under its title. A song with no artist SHALL render no subtitle.

#### Scenario: A song with an artist

- **WHEN** a host renders the subtitle of a song whose `artist` field is set
- **THEN** the subtitle is the artist's name

#### Scenario: A song with no artist

- **WHEN** a host renders the subtitle of a song with no `artist` field
- **THEN** the subtitle is empty
