# lofi-player

A compact, polished TUI player for lofi / chillhop / ambient internet
radio. Built for focused work — keyboard-driven, Tokyo-Night by default,
designed to live happily in a tmux pane.

> **Status: Phase 2 (UX polish).** Real audio plays through `mpv`;
> themes are switchable with `t`, the layout has a compact mini mode
> (`m`), state survives restarts, and there's a `--statusline` mode for
> tmux integration. Pomodoro support arrives in Phase 3 — see
> [`plans/lofi-player-plan.md`](plans/lofi-player-plan.md).

## Requirements

- Go **1.26** or newer (build only)
- `mpv` on `$PATH` (`brew install mpv` / `apt install mpv`)
- A terminal that handles 256 colors and Unicode block characters

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
| `t`            | Cycle theme                       |
| `m`            | Toggle mini mode                  |
| `?`            | Toggle compact / full help card   |
| `q` / `ctrl+c` | Quit                              |

## Configuration

`$XDG_CONFIG_HOME/lofi-player/config.yaml` — usually
`~/.config/lofi-player/config.yaml` on both Linux and macOS. Created
with sensible defaults on first run; documented example at
[`configs/lofi-player.example.yaml`](configs/lofi-player.example.yaml).

```yaml
theme: tokyo-night
volume: 60
stations:
  - name: SomaFM Groove Salad
    url: https://ice1.somafm.com/groovesalad-256-mp3
```

Available themes: `tokyo-night` (default), `catppuccin-mocha`,
`gruvbox-dark`, `rose-pine`. Cycle live with `t`.

## State

`$XDG_STATE_HOME/lofi-player/state.json` remembers the last theme,
volume, and station between sessions. Persistence is best-effort — a
write failure logs to stderr but never aborts shutdown.

## Project layout

```
main.go                      entry point: load config + state, start mpv, run TUI
internal/
  audio/                     mpv subprocess + JSON-IPC client
  config/                    YAML config + XDG paths + defaults
  state/                     state.json — last-session persistence
  theme/                     palettes (tokyo-night, catppuccin-mocha, gruvbox-dark, rose-pine)
  tui/                       Bubble Tea model / update / view / keys / styles / mini / toast
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
