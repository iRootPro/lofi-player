# lofi-player

**English** · [Русский](README.ru.md)

A keyboard-driven TUI for lofi, chillhop and ambient internet radio —
built to live in a tmux pane while you work.

[![ci](https://github.com/iRootPro/lofi-player/actions/workflows/ci.yml/badge.svg)](https://github.com/iRootPro/lofi-player/actions/workflows/ci.yml)
[![release](https://img.shields.io/github/v/release/iRootPro/lofi-player?display_name=tag&sort=semver)](https://github.com/iRootPro/lofi-player/releases)
[![go version](https://img.shields.io/github/go-mod/go-version/iRootPro/lofi-player)](go.mod)
[![license](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

<p align="center">
  <img src="demo/lofi-player.gif" alt="lofi-player demo" width="800">
</p>

## What it is

A small Go TUI that wraps `mpv` and turns it into a comfortable lofi
station picker. Pick a stream, hit space, leave it running. Layer in
some rain or a crackling fire from the ambient mixer. Drop the window
into a tmux pane and forget it's there.

It is **not** a music library manager, not a downloader, not a Spotify
client. The only thing it plays is internet radio (Icecast/Shoutcast,
direct HTTP streams, and YouTube live/videos via `yt-dlp`).

## Features

- **Internet radio** — any URL `mpv` can open: Icecast, Shoutcast, raw
  HTTP/HTTPS streams. SomaFM and Radio Paradise ship in the defaults.
- **YouTube streams** — Lofi Girl 24/7 and friends, routed through
  mpv's `ytdl_hook` (needs `yt-dlp`).
- **Ambient mixer** — five looped beds (rain, fire, cafe, white noise,
  thunder), CC0 sources, embedded in the binary. Each channel has its
  own volume; mixes layer underneath the main station.
- **Four themes** — Tokyo Night, Catppuccin Mocha, Gruvbox Dark, Rose
  Pine. Cycle with `t`.
- **Mini mode** — collapses to ~6 lines for a tmux pane (`m`).
- **Tmux statusline** — `lofi-player --statusline` prints one colored
  line for `status-right`.
- **State persistence** — last station, volume, theme, ambient levels
  survive restarts (XDG state dir).
- **Manage stations from the TUI** — `a` adds, `e` edits, `d` deletes
  (with confirmation). Station kind is auto-detected from the URL.
- **ICY metadata** — title/artist updates from the stream are
  surfaced in the now-playing card.
- **Stream info row** — bitrate, codec, sample rate, session uptime,
  and a buffer-health bar live under the now-playing card. Toggle
  visibility with `i`; the choice persists across sessions.
- **Friendly first run** — missing `mpv` shows a styled install hint
  instead of a raw error; missing `yt-dlp` degrades to a startup
  warning and an `unavailable` tag on YouTube stations, so the rest
  of your library keeps playing.

## Install

Supported platforms: `linux/amd64`, `linux/arm64`, `darwin/amd64`,
`darwin/arm64`. Windows is not supported.

### Homebrew (macOS, Linux) — recommended

```sh
brew install iRootPro/tap/lofi-player
```

The formula pulls in `mpv` automatically and recommends `yt-dlp` for
YouTube playback — one command and you're done.

### One-line installer (macOS, Linux)

```sh
curl -fsSL https://raw.githubusercontent.com/iRootPro/lofi-player/main/scripts/install.sh | sh
```

Auto-detects OS/arch, pulls the matching tarball from the latest
release, drops the binary into `~/.local/bin`. Override with
`INSTALL_DIR=/usr/local/bin` or pin to a tag with `VERSION=v0.1.1`.

You'll also need `mpv` (and optionally `yt-dlp`) — see
[runtime dependencies](#runtime-dependencies) below. The installer
prints a hint if `mpv` isn't on `$PATH`.

### From source (Go 1.26+)

```sh
go install github.com/iRootPro/lofi-player@latest
```

### Pre-built binaries

Grab the archive for your OS/arch from the
[releases page](https://github.com/iRootPro/lofi-player/releases),
extract, drop `lofi-player` somewhere on `$PATH`.

### Runtime dependencies

Homebrew handles these for you. The other install paths require you
to install them separately.

| dependency | required for | install |
|---|---|---|
| `mpv` | all playback | `brew install mpv` · `apt install mpv` · `pacman -S mpv` · `dnf install mpv` |
| `yt-dlp` | YouTube stations only | `brew install yt-dlp` · `pip install yt-dlp` |
| Nerd Font | section/volume/mixer icons | [JetBrains Mono](https://github.com/ryanoasis/nerd-fonts/releases) or [FiraCode](https://github.com/ryanoasis/nerd-fonts/releases) Nerd Font |

If `mpv` isn't on `$PATH`, the app prints a styled "can't start" card
with platform-specific install commands and exits — there's nothing
to play without the engine. If `yt-dlp` is missing but your config
has YouTube stations, the app starts normally with a warning toast,
marks YouTube rows as `unavailable`, and refuses to autoplay them;
direct streams keep working. Without a Nerd Font, the icons render
as tofu boxes; the rest of the UI keeps working.

## Quick start

```sh
lofi-player
```

On first run, a default config with four SomaFM/Radio Paradise stations
is written to `~/.config/lofi-player/config.yaml`. Pick a station with
`j`/`k`, hit `space`, drop the window into a tmux pane, get back to
work.

Quit with `q` or `ctrl+c`.

## Keybindings

### Global

| key | action |
|---|---|
| `j` / `↓` | move cursor down |
| `k` / `↑` | move cursor up |
| `space` | play / pause selected station |
| `+` / `=` | volume up (5%) |
| `-` / `_` | volume down (5%) |
| `t` | cycle theme |
| `m` | toggle mini mode |
| `a` | add station (modal) |
| `e` | edit selected station (modal) |
| `d` | delete selected station (with confirmation) |
| `x` | open ambient mixer (modal) |
| `i` | toggle stream-info row |
| `?` | toggle full help card |
| `q` / `ctrl+c` | quit |

### Ambient mixer (after `x`)

| key | action |
|---|---|
| `j` / `↓` · `k` / `↑` | select channel |
| `h` / `←` · `l` / `→` | volume ±5% (fine) |
| `H` · `L` | volume ±25% (coarse) |
| `0` | mute channel |
| `1` | channel to 100% |
| `esc` / `x` | close mixer (state autosaves) |

### Add / edit station (after `a` or `e`)

| key | action |
|---|---|
| `tab` / `shift+tab` | next / previous field |
| `enter` | save (writes to `config.yaml`) |
| `esc` | cancel |

`e` pre-fills the form with the selected station's name and URL;
`enter` updates it in-place. `kind` is auto-detected from the URL:
`youtube.com` / `youtu.be` → `youtube`, anything else → `stream`.

### Delete confirmation (after `d`)

| key | action |
|---|---|
| `y` / `enter` | confirm delete |
| `n` / `esc` | cancel |

Deleting the currently-playing station pauses playback and clears the
now-playing card. The change is written to `config.yaml` immediately.

## Configuration

Lives at `$XDG_CONFIG_HOME/lofi-player/config.yaml` — i.e.
`~/.config/lofi-player/config.yaml` on both Linux and macOS. Created
on first run with sensible defaults; a documented example sits at
[`configs/lofi-player.example.yaml`](configs/lofi-player.example.yaml).

```yaml
theme: tokyo-night        # tokyo-night | catppuccin-mocha | gruvbox-dark | rose-pine
volume: 60                # initial volume, 0–100

stations:
  - name: SomaFM Groove Salad
    url: https://ice1.somafm.com/groovesalad-256-mp3

  - name: Lofi Girl 24/7
    url: https://www.youtube.com/watch?v=jfKfPfyJRdk
    kind: youtube         # only needed for YouTube URLs
```

Want more stations out of the box? A bundled preset of ~200 chillout /
jazz / classical / trance streams from
[radiopotok.ru](https://radiopotok.ru) lives at
[`configs/radiopotok.yaml`](configs/radiopotok.yaml) — copy the
entries you like into your own `config.yaml`. Regenerate from
upstream with `./scripts/fetch-radiopotok.py`. ~20% of these
third-party streams are flaky at any moment; pick another if one
fails.

## Themes

Four palettes ship in the binary:

- **Tokyo Night** (default) — cool, neon-on-deep-blue.
- **Catppuccin Mocha** — pastel-on-warm-charcoal.
- **Gruvbox Dark** — earthy, high-contrast.
- **Rose Pine** — muted, soft mauve.

Cycle live with `t`. The choice is persisted to state and reapplied on
the next launch.

## Mini mode and tmux

`m` collapses the UI to the now-playing card alone — about six lines.
Drop the window into a small tmux pane and you have a permanent
"what's playing" surface.

For an even smaller footprint, the `--statusline` mode prints a single
colored line and exits, suitable for `status-right`:

```sh
lofi-player --statusline
# ♪ SomaFM Drone Zone  ▰▰▰▱▱▱  60%
```

```tmux
set -g status-interval 5
set -g status-right '#(lofi-player --statusline)'
```

## State

`$XDG_STATE_HOME/lofi-player/state.json` —
`~/.local/state/lofi-player/state.json` on both Linux and macOS.
Stores last theme, volume, station name, and per-channel ambient
volumes. Best-effort: a write failure logs to stderr but never aborts
shutdown.

## Project layout

```
main.go                      entry: load config + state, start mpv, run TUI
internal/
  audio/                     mpv subprocess + JSON-IPC client + ambient mixer
  config/                    YAML config + XDG paths + defaults
  state/                     state.json — last-session persistence
  theme/                     color palettes
  tui/                       Bubble Tea model / update / view / keys / styles
configs/
  lofi-player.example.yaml   documented example config
  radiopotok.yaml            ~200-station preset
scripts/
  fetch-radiopotok.py        regenerates configs/radiopotok.yaml
demo/
  lofi-player.tape           vhs script for the README GIF
plans/
  lofi-player-plan.md        roadmap (single source of truth)
```

## Building and testing

```sh
go build -o lofi-player .
go test  ./...
go vet   ./...
```

The Makefile-free workflow on purpose. Releases are cut locally:
`git tag -a vX.Y.Z -m "..."` then `goreleaser release --clean` —
goreleaser uploads the binaries to GitHub Releases and pushes the
updated formula to [iRootPro/homebrew-tap](https://github.com/iRootPro/homebrew-tap)
in one shot. CI on `main` runs `vet` + `test` + `build` on every push.

## Credits

Ambient samples are CC0 (public domain) — credited here as a courtesy
and so the originals can be re-found.

| channel | source | author |
|---|---|---|
| rain | [freesound.org/s/525046](https://freesound.org/s/525046/) | speakwithanimals |
| fire | [freesound.org/s/760474](https://freesound.org/s/760474/) | True_Killian |
| white noise | [freesound.org/s/132275](https://freesound.org/s/132275/) | assett1 |
| cafe | [freesound.org/s/32910](https://freesound.org/s/32910/) | ToddBradley |
| thunder | [freesound.org/s/717890](https://freesound.org/s/717890/) | TRP |

Built on [Bubble Tea](https://github.com/charmbracelet/bubbletea),
[Lipgloss](https://github.com/charmbracelet/lipgloss),
[Bubbles](https://github.com/charmbracelet/bubbles),
and [mpv](https://mpv.io/).

## License

MIT — see [LICENSE](LICENSE).
