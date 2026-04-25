# lofi-player

A compact, polished TUI player for lofi / chillhop / ambient internet
radio. Built for focused work — keyboard-driven, Tokyo-Night by default,
designed to live happily in a tmux pane.

> **Status: Phase 0 (UI scaffold).** The interface is wired up and the
> station list is keyboard-navigable, but audio playback isn't connected
> yet — that arrives in Phase 1 once the `mpv` IPC bridge lands.
> See [`plans/lofi-player-plan.md`](plans/lofi-player-plan.md) for the
> full roadmap.

## Requirements

- Go **1.26** or newer (build only)
- A terminal that handles 256 colors and Unicode block characters
- _From Phase 1 onward:_ `mpv` on `$PATH` (`brew install mpv` /
  `apt install mpv`). Not required for Phase 0.

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

## Keybindings (Phase 0)

| Key            | Action                            |
|----------------|-----------------------------------|
| `j` / `↓`      | Move cursor down                  |
| `k` / `↑`      | Move cursor up                    |
| `space`        | Play / pause selected station     |
| `+` / `=`      | Volume up (5%)                    |
| `-` / `_`      | Volume down (5%)                  |
| `?`            | Toggle compact / full help        |
| `q` / `ctrl+c` | Quit                              |

Audio doesn't actually play in Phase 0 — `space` only flips the
"live" / "paused" status text and the `♪` marker in the station list.

## Configuration

The config file lives at `$XDG_CONFIG_HOME/lofi-player/config.yaml`,
which is `~/.config/lofi-player/config.yaml` on both Linux and macOS
(the `adrg/xdg` library applies XDG conventions to macOS too — this is
intentional and matches expectations for terminal tools).

It is created with sensible defaults on first run. A documented example
lives in [`configs/lofi-player.example.yaml`](configs/lofi-player.example.yaml).

Schema (Phase 0):

```yaml
theme: tokyo-night     # only theme shipped in Phase 0
volume: 60             # initial volume, 0–100
stations:
  - name: SomaFM Groove Salad
    url: https://ice1.somafm.com/groovesalad-256-mp3
  # ...
```

Missing keys fall back to defaults; an explicit `stations: []` is
honored as "no stations" (the list is _not_ refilled with defaults
once the config file exists).

## Project layout

```
main.go                      entry point: load config, start Bubble Tea
internal/
  config/                    YAML + XDG, defaults on first run
  theme/                     Tokyo Night palette (more themes in Phase 2)
  tui/                       Bubble Tea model / update / view / keys / styles
configs/
  lofi-player.example.yaml   documented example config
plans/
  lofi-player-plan.md        the roadmap (single source of truth)
  lofi-player-preview.html   Tokyo-Night visual reference for the layout
```

## Tests

```sh
go test ./...
go vet  ./...
```

The strategy is "test the core, not the chrome" — `internal/config` and
`internal/theme` have unit tests; the TUI rendering is verified by eye.
