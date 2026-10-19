<p align="center">
  <img src=".github/brand/beamfall-logo.png" alt="Beamfall" width="96" height="96">
</p>

# Beamfall Karaoke

**Your karaoke collection, turned into a sing-along songbook, without anything leaving your server.**
Beamfall Karaoke is the plugin that gives [Beamfall Core](https://github.com/beamfall/core) its
karaoke experience. Point a Beamfall library at your folder of MP3+G discs, karaoke videos,
UltraStar songs and LRC tracks, and Karaoke works out what each song is, who it is by, and how its
words should light up on screen. It works offline, needs no configuration, and calls no outside
service.

It is free and open source under [AGPL-3.0](LICENSE).

## What you get

- **Every common karaoke format, recognized automatically.** Each song is sorted by where its
  words come from:

  | Format | Files | Words |
  | --- | --- | --- |
  | CD+G (MP3+G) | audio plus a `.cdg` of the same name | CD+G graphics |
  | Karaoke video | an MP4, MKV or other video | burned into the picture |
  | UltraStar | audio plus an UltraStar `.txt` | timed, pitched syllables, solo or duet |
  | LRC | audio plus an `.lrc`, or LRC text in the file's lyrics tag | line- or word-timed text |
  | Backing track | audio alone | none |

- **Words that light up as you sing.** LRC and UltraStar lyrics are reduced to one timed
  timeline of lines and syllables, with the pitch of every UltraStar note and which singer each
  line belongs to, so a client can wipe words, draw a pitch guide and score the singer.
- **Every song plays on every screen.** For a song with no picture of its own, the plugin tells
  the server how to make one: the `.cdg` graphics for CD+G, or a generated subtitle track for
  UltraStar and LRC, with lines that appear ahead of time on alternating rows, syllables that fill
  as they are sung, a different color for each duet voice, and a title card over a long intro.
- **A songbook that reads your file names.** Disc IDs, track numbers, artists and titles come from
  names such as `SC8125-03 - Artist - Title`, `03 - Artist - Title` inside an `SC8125/` disc
  folder, or plain `Artist - Title`. Markers such as `(Karaoke Version)`, `[Vocal Guide]`,
  `(Duet)` and `(In the Style of Artist)` are understood and cleaned out of the title. UltraStar
  headers, the file's tags and LRC `[ti:]`/`[ar:]` tags take priority when present.
- **Search and filter like a KJ.** Artist, disc ID, label (Sound Choice, Sunfly, Chartbuster and
  other well-known prefixes), language, genre, guide vocals, duets and lyric sync are all fields
  you can filter, and artist and disc ID are searchable and sortable.
- **A home screen built for a party.** Recently Sung, Crowd Favorites, Recently Added, Surprise
  Me, Duets, a format filter and the full Songbook.
- **Always from the top.** Songs never resume halfway, even videos, and clients can record a
  performance separately from a play, because a song can play to an empty room.
- **Kind to old files.** Sidecars are decoded from UTF-8, UTF-16 with a byte-order mark, or a
  legacy Windows code page. The LRC and UltraStar readers handle the conventions found in real
  collections: repeated timestamps, offsets, instrumental breaks, translation lines, duet
  prefixes, sustained `~` notes, relative beats and comma-decimal numbers.
- **Careful with your files.** Every sidecar read is size-bounded, the folder scan for UltraStar
  files is capped, and the file-name, LRC and UltraStar readers are fuzz tested. A broken sidecar
  is simply not used.
- **English and French** labels out of the box.

## How it fits into Beamfall

Karaoke ships with [Beamfall Core](https://github.com/beamfall/core), the self-hosted media server
for the library you already own. You do not install it separately: Core loads it, and your karaoke
libraries use it.

The plugin is built on the [Beamfall plugin SDK](https://github.com/beamfall/plugin-sdk). Its
shape is declarative: `Bundle()` in [`bundle.go`](bundle.go) describes the song fields, format
labels, interactive sidecars, playback behavior and home layout, and Core does the rest. It
declares no network scrapers and no entity kinds, because karaoke files describe themselves and a
karaoke backing is a re-recording that no open catalog identifies.

The compiled code is the reading of songs:

| File | Purpose |
| --- | --- |
| [`parse.go`](parse.go) | The scan parser: format, title, and fields, plus the optional `TimedLyrics` and `ComposedPicture` hooks. |
| [`sidecar.go`](sidecar.go) | Bounded discovery and reading of `.cdg`, `.lrc` and UltraStar `.txt` companions. |
| [`lrc.go`](lrc.go) | `ParseLRC`: plain and enhanced (word-timed) LRC. |
| [`ultrastar.go`](ultrastar.go) | `ParseUltraStar`: legacy, relative and 1.x UltraStar files, solo and duet. |
| [`lyrics.go`](lyrics.go) | The neutral `Lyrics` timeline: lines, syllables, pitch and singer. |
| [`ass.go`](ass.go) | `RenderASS`: the on-screen subtitle track for composed karaoke pictures. |
| [`text.go`](text.go) | Sidecar text decoding: byte-order marks, UTF-16 and legacy code pages. |

### Not supported yet

- **MIDI/KAR** songs: Beamfall plays media through FFmpeg-based pipelines with no MIDI
  synthesizer, so a `.kar` file is not playable today.
- **ASS/SSA karaoke subtitles** and **KaraFun `.kfn`** files as song sources.
- **Double-byte legacy encodings** (Shift-JIS, GBK, Big5) in sidecars; they decode as
  Windows-1252.
- Drawing CD+G graphics, syllable wipes, pitch guides and key or tempo changes happens in the
  Beamfall clients, not in this plugin.

## Develop

You need Go (the version in [`go.mod`](go.mod)) and [golangci-lint](https://golangci-lint.run/).

```sh
go test ./...          # unit tests and the SDK conformance harness
script/gate.sh         # gofmt, go vet, tests with a coverage floor, golangci-lint
script/gate.sh race    # the test suite under the race detector
script/gate.sh fuzz    # a bounded fuzz run of the file-name, LRC and UltraStar readers
```

[`conformance_test.go`](conformance_test.go) runs the SDK's `sdktest` conformance harness against
`Bundle()` and the parser. CI runs the same checks on every push and pull request; see
[`.github/workflows/conformance.yml`](.github/workflows/conformance.yml).

## Acknowledgements

The LRC and UltraStar conventions handled here were cross-checked against the readers in
[KaraokeMachine](https://github.com/rrgmc/karaokemachine) (MIT / Apache-2.0). No code was copied;
this is an independent Go implementation.

## Contributing

Contributions need a signed Contributor License Agreement. See [CONTRIBUTING.md](CONTRIBUTING.md)
and [CLA.md](CLA.md).

## Security

Please report vulnerabilities privately. See [SECURITY.md](SECURITY.md).

## License

Beamfall Karaoke is licensed under the GNU Affero General Public License v3. See [LICENSE](LICENSE).
