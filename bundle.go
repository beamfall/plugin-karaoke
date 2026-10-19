// Package karaoke is the Karaoke Domain Plugin: a bundle and scan hooks that
// turn a folder of karaoke songs into a sing-along Library.
//
// A karaoke song is one Media whose words arrive in one of four ways, and the
// conventional "class" Media field names which:
//
//   - "cdg" — MP3+G: an audio file with a .cdg CD+G graphics sidecar of the
//     same stem. The client draws the graphics.
//   - "video" — a karaoke video with the words in the picture.
//   - "ultrastar" — audio with an UltraStar .txt: timed, pitched syllables a
//     client can wipe, draw a pitch guide from, and score.
//   - "lrc" — audio with LRC timed lyrics (line- or word-timed), as a sidecar
//     or embedded in the file's lyrics tag.
//
// Audio with none of these is a "backing" track. The .cdg, .lrc, and .txt
// companions are declared InteractiveSidecars, so Core serves them to clients
// beside the Media; the parser reads them for metadata, and TimedLyrics
// reduces LRC/UltraStar to one neutral timeline for highlighting.
//
// The model is Media-field-first: the song's artist, disc ID, label, language,
// and vocal facts are typed Media fields the parser fills, because karaoke
// files are self-describing and no open catalog identifies a karaoke track
// (the backing is a re-recording, not the original release). The bundle
// therefore declares no network scrapers and no Entity Kinds.
package karaoke

import sdk "github.com/beamfall/plugin-sdk"

// PerformanceEventKind is the engagement event a client records when someone
// sings a song (as distinct from a play: a song can play to an empty room).
const PerformanceEventKind = "karaoke-performance"

// Bundle returns the Karaoke plugin's declarative contribution.
func Bundle() sdk.Bundle {
	resume := false
	return sdk.Bundle{
		ID:          PluginID,
		Version:     "0.1.0",
		SDKFloor:    "0.3.1",
		SDKCeiling:  "0.3.2",
		DisplayName: sdk.LocaleString{"en": "Karaoke", "fr": "Karaoké"},
		// A karaoke track is one Media — "Song" is a display label only.
		MediaLabel:   sdk.LocaleString{"en": "Song", "fr": "Chanson"},
		MediaClasses: []string{"audio", "video"},
		MediaClassLabels: map[string]sdk.LocaleString{
			ClassCDG:       {"en": "Karaoke (CD+G)", "fr": "Karaoké (CD+G)"},
			ClassVideo:     {"en": "Karaoke Video", "fr": "Vidéo karaoké"},
			ClassUltraStar: {"en": "Sing-Along (UltraStar)", "fr": "Chant (UltraStar)"},
			ClassLRC:       {"en": "Karaoke Track", "fr": "Piste karaoké"},
			ClassBacking:   {"en": "Backing Track", "fr": "Accompagnement"},
		},
		// A karaoke song always starts from the top — including videos, whose
		// class default would resume.
		Engagement: sdk.Engagement{
			ResumeMedia: &resume,
			EventKinds:  []string{PerformanceEventKind},
		},
		InteractiveSidecars: []string{extCDG, extLRC, extTXT},
		MediaFields: []sdk.FieldSpec{
			{Name: "class", Type: "text", Filterable: true,
				Label: sdk.LocaleString{"en": "Format", "fr": "Format"}},
			// The original artist the backing imitates — a flat field, so the
			// songbook sorts and searches by it without entity hydration.
			{Name: "artist", Type: "text", Searchable: true, Filterable: true, Sortable: true,
				Label: sdk.LocaleString{"en": "Artist", "fr": "Artiste"}},
			// The label's disc ID ("SC8125"): how a KJ's printed songbook
			// addresses a track.
			{Name: "disc-id", Type: "text", Searchable: true, Filterable: true, Sortable: true,
				Label: sdk.LocaleString{"en": "Disc", "fr": "Disque"}},
			{Name: "brand", Type: "text", Filterable: true,
				Label: sdk.LocaleString{"en": "Label", "fr": "Label"}},
			{Name: "language", Type: "text", Filterable: true,
				Label: sdk.LocaleString{"en": "Language", "fr": "Langue"}},
			{Name: "genre", Type: "text", Filterable: true,
				Label: sdk.LocaleString{"en": "Genre", "fr": "Genre"}},
			// "guide" when the track carries a guide vocal.
			{Name: "vocals", Type: "text", Filterable: true,
				Label: sdk.LocaleString{"en": "Vocals", "fr": "Voix"}},
			// "duet" for a two-voice song.
			{Name: "singers", Type: "text", Filterable: true,
				Label: sdk.LocaleString{"en": "Singers", "fr": "Chanteurs"}},
			// How the client highlights the words: line, word, note,
			// graphics, or video (the Sync* constants).
			{Name: "lyrics-sync", Type: "text",
				Label: sdk.LocaleString{"en": "Lyric Sync", "fr": "Synchronisation des paroles"}},
		},
		LockableMediaFields: []string{"title", "media.artist", "media.disc-id", "media.language"},
		Layouts: []sdk.LayoutSpec{{ID: "home", Role: "browse", Sections: []sdk.SectionSpec{
			{ID: "recently-sung", Kind: "shelf",
				Title:       sdk.LocaleString{"en": "Recently Sung", "fr": "Chantées récemment"},
				ArtworkSlot: "thumb", Source: sdk.SourceSpec{Kind: "recently-played"}},
			{ID: "most-sung", Kind: "shelf",
				Title:       sdk.LocaleString{"en": "Crowd Favorites", "fr": "Les plus chantées"},
				ArtworkSlot: "thumb", Source: sdk.SourceSpec{Kind: "most-played"}},
			{ID: "recently-added", Kind: "shelf",
				Title:       sdk.LocaleString{"en": "Recently Added", "fr": "Ajouts récents"},
				ArtworkSlot: "thumb", Source: sdk.SourceSpec{Kind: "recently-added"}},
			{ID: "surprise-me", Kind: "shelf",
				Title:       sdk.LocaleString{"en": "Surprise Me", "fr": "Au hasard"},
				ArtworkSlot: "thumb", Source: sdk.SourceSpec{Kind: "random", Seed: PluginID}},
			{ID: "duets", Kind: "shelf",
				Title:       sdk.LocaleString{"en": "Duets", "fr": "Duos"},
				ArtworkSlot: "thumb",
				Source:      sdk.SourceSpec{Kind: "all-items", FieldFilters: map[string]any{"singers": SingersDuet}}},
			// Narrows the adjacent songbook grid by format.
			{ID: "format", Kind: "filter-bar",
				Title:  sdk.LocaleString{"en": "Format", "fr": "Format"},
				Source: sdk.SourceSpec{Kind: "all-items"},
				FilterOptions: []sdk.FilterOption{
					{Field: "class", Value: ClassCDG, Label: sdk.LocaleString{"en": "CD+G", "fr": "CD+G"}},
					{Field: "class", Value: ClassVideo, Label: sdk.LocaleString{"en": "Video", "fr": "Vidéo"}},
					{Field: "class", Value: ClassUltraStar, Label: sdk.LocaleString{"en": "UltraStar", "fr": "UltraStar"}},
					{Field: "class", Value: ClassLRC, Label: sdk.LocaleString{"en": "Lyrics", "fr": "Paroles"}},
					{Field: "class", Value: ClassBacking, Label: sdk.LocaleString{"en": "Backing", "fr": "Accompagnement"}},
				}},
			{ID: "songbook", Kind: "grid",
				Title:       sdk.LocaleString{"en": "Songbook", "fr": "Répertoire"},
				ArtworkSlot: "thumb", Source: sdk.SourceSpec{Kind: "all-items"}},
		}}},
	}
}
