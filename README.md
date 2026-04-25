# lofi-player

A compact, polished TUI player for lofi / chillhop / ambient internet
radio. Built for focused work — keyboard-driven, Tokyo-Night by default,
designed to live happily in a tmux pane.

> **Status: Phase 4a (YouTube).** Real audio through `mpv`; switchable
> themes (`t`); compact mini mode (`m`); state survives restarts;
> `--statusline` for tmux; YouTube live streams and videos via mpv's
> ytdl_hook (Lofi Girl 24/7 etc.); add stations from the TUI (`a`).
> Remaining Phase 4 power features (local files, ambient mixer, MPRIS,
> Discord) and Phase 5 distribution still ahead — see
> [`plans/lofi-player-plan.md`](plans/lofi-player-plan.md).

## Requirements

- Go **1.26** or newer (build only)
- `mpv` on `$PATH` (`brew install mpv` / `apt install mpv`)
- `yt-dlp` on `$PATH` (`brew install yt-dlp` / `pip install yt-dlp`) —
  only if your config has YouTube-kind stations; the app refuses to
  start with a clear hint if it's missing
- A terminal that handles 256 colors and Unicode block characters
- A **Nerd Font** for the section/volume/logo icons (FontAwesome subset).
  Without one, those icons render as tofu boxes (`􂁒` etc.); the rest of
  the UI keeps working. JetBrains Mono Nerd Font and FiraCode Nerd Font
  are popular choices. Ghostty / WezTerm / Kitty / iTerm2 all support
  loading a Nerd Font as the terminal font.

## Build & run

```sh
go build -o lofi-player .
./lofi-player
```

Or, during development:

```sh
go run .
```

Quit with `q` or `ctrl+c`. The alt-screen is restored on exit.

### Tmux statusbar

```sh
./lofi-player --statusline
# ♪ SomaFM Drone Zone  ▰▰▰▱▱▱  60%
```

This reads the last persisted state and prints one colored line, then
exits cleanly. Drop it into your `status-right` to show what's playing
in the bar:

```tmux
set -g status-interval 5
set -g status-right '#(lofi-player --statusline)'
```

## Keybindings

| Key            | Action                            |
|----------------|-----------------------------------|
| `j` / `↓`      | Move cursor down                  |
| `k` / `↑`      | Move cursor up                    |
| `space`        | Play / pause selected station     |
| `+` / `=`      | Volume up (5%, spring-animated)   |
| `-` / `_`      | Volume down (5%, spring-animated) |
| `a`            | Add station (modal form)          |
| `t`            | Cycle theme                       |
| `m`            | Toggle mini mode                  |
| `?`            | Toggle compact / full help card   |
| `q` / `ctrl+c` | Quit                              |

Adding stations: press `a` to open a modal form (name + URL). Tab cycles
fields, Enter saves to `config.yaml`, Esc cancels. The `kind` is
auto-detected from the URL (`youtube.com` / `youtu.be` → `youtube`,
otherwise `stream`).

## Configuration

`$XDG_CONFIG_HOME/lofi-player/config.yaml` — defaults to
`~/.config/lofi-player/config.yaml` on both Linux and macOS (the
macOS-native `~/Library/Application Support` is intentionally _not_
used; terminal users expect the XDG-style path). Created automatically
on first run with sensible defaults — no manual setup needed. A
documented example lives at
[`configs/lofi-player.example.yaml`](configs/lofi-player.example.yaml).

```yaml
theme: tokyo-night
volume: 60
stations:
  - name: SomaFM Groove Salad
    url: https://ice1.somafm.com/groovesalad-256-mp3

  # YouTube (any URL mpv's ytdl_hook accepts — videos, live streams, etc.):
  - name: Lofi Girl 24/7
    url: https://www.youtube.com/watch?v=jfKfPfyJRdk
    kind: youtube
```

Station `kind` defaults to `stream` (a direct HTTP/Icecast URL passed
to mpv as-is). Set it to `youtube` to route through mpv's ytdl_hook
(requires `yt-dlp` on `$PATH`).

Available themes: `tokyo-night` (default), `catppuccin-mocha`,
`gruvbox-dark`, `rose-pine`. Cycle live with `t`.

## State

`$XDG_STATE_HOME/lofi-player/state.json` — defaults to
`~/.local/state/lofi-player/state.json` on both Linux and macOS.
Remembers the last theme, volume, and station between sessions.
Persistence is best-effort — a write failure logs to stderr but never
aborts shutdown.

## Project layout

```
main.go                      entry point: load config + state, start mpv, run TUI
internal/
  audio/                     mpv subprocess + JSON-IPC client
  config/                    YAML config + XDG paths + defaults
  state/                     state.json — last-session persistence
  theme/                     palettes (tokyo-night, catppuccin-mocha, gruvbox-dark, rose-pine)
  tui/                       Bubble Tea model / update / view / keys / styles / mini / toast / anim
configs/
  lofi-player.example.yaml   documented example config
plans/
  lofi-player-plan.md        the roadmap (single source of truth)
  lofi-player-preview.html   Tokyo-Night visual reference
```

## Tests

```sh
go test ./...
go vet  ./...
```

Strategy is "test the core, not the chrome": `internal/audio`,
`internal/config`, `internal/state`, and `internal/theme` have unit
tests; the TUI rendering is verified by eye against
[`plans/lofi-player-preview.html`](plans/lofi-player-preview.html).
