# okiscraper

Scrapes [okizeme.gg](https://okizeme.gg) for a Tekken 8 character's move videos and
frame data, then builds an Anki deck (video on the front, frame data on the back).

## Prerequisites

- **Go** — see `go.mod` for the version.
- **Chromium** — downloaded automatically on first run by go-rod into
  `%APPDATA%\rod\browser`. (Antivirus sometimes quarantines rod's `leakless.exe`
  helper; the launcher runs with `Leakless(false)` so that is avoided.)
- **ffmpeg** on your `PATH` — used to convert `.mp4` -> `.webm` for Anki.
- **Anki** running, with the [AnkiConnect](https://ankiweb.net/shared/info/2055492159)
  add-on installed (listens on `http://localhost:8765`).

## Run

```bash
go run .
# or build:
go build -o okiscraper.exe .
./okiscraper.exe
```

Enter a character name as it appears in the okizeme.gg URL (lowercase, e.g. `kaz`,
`victor`), then choose actions:

1. **Download Videos** — saves each move clip as `<key>.mp4`.
2. **Get Move Frame Data** — saves `frame_data.json`.
3. **Create Anki Deck** — creates the deck + note type, converts videos to `.webm`,
   and adds one note per move.

For the full pipeline, select all three.

## Output layout

```
scraper_output/<character>/
├── <key>.mp4                 # downloaded clips (key = lossless-encoded move command)
├── frame_data.json           # [{ key, move_name, startup, hit_level, frames_on_block, frames_on_hit, notes }, ...]
└── webm/
    └── <character>_<key>.webm
```

`key` is `models.EncodeMoveName(move)` — a **lossless**, filesystem-safe encoding of the
move command (e.g. `1+2` -> `1%2B2`, which stays distinct from `1,2` -> `1%2C2`). Frame
data and videos are joined on this key, so `+` (simultaneous) and `,` (sequence) never
collide, and illegal-in-filename characters are handled safely.

## Architecture

```
main.go                 thin entrypoint: configure -> app.Run -> exit
internal/
├── app/                orchestrator: runs stages in order; deck setup; webm conversion; card-building join
├── anki/               AnkiConnect client (createDeck / createModel / addNote) + card templates
├── cli/                interactive config (huh multi-select) -> character + AppOptions
├── media/              ffmpeg mp4 -> webm conversion
├── models/             shared `Move` type, lossless `EncodeMoveName`, `Capitalize`
└── scraper/            okizeme.gg browser scraping (videos + frame data) via go-rod
```

Pipeline stages (each runs against `scraper_output/<character>/`):

1. Scrape videos -> `*.mp4`
2. Scrape frame data -> `frame_data.json`
3. Convert media -> `webm/*.webm`
4. Anki setup -> create deck + note type
5. Build cards -> join `frame_data.json` <-> `webm/` by `key`, add notes

## Troubleshooting

- **`leakless.exe ... contains a virus`** — antivirus false positive. rod's leakless
  helper is disabled in `internal/scraper`, so this should not recur; otherwise exclude
  rod's folders from your antivirus.
- **`could not reach AnkiConnect ...`** — open Anki and confirm the AnkiConnect add-on
  is installed.
- **No cards created** — run a video + frame-data scrape first (so `webm/` and
  `frame_data.json` exist) and make sure `ffmpeg` is on `PATH`.
