## ADDED Requirements

### Requirement: Karaoke offers the party session

The karaoke bundle SHALL list `party` in `FeatureEligibility`, so core may run a party session over
karaoke Libraries.

#### Scenario: Bundle offers the party

- **WHEN** the karaoke bundle is built
- **THEN** its `FeatureEligibility` contains `party`

### Requirement: Karaoke names the party's parts

The karaoke bundle SHALL declare `PartyTerms` with English and French values for the party (Karaoke
party), participant and participants (singer), request and requests (song) and stage.

#### Scenario: Every term in both locales

- **WHEN** the karaoke bundle is built
- **THEN** each `PartyTerms` term has a non-empty `en` and `fr` value and the bundle validates
