# Changelog

All notable user-facing changes are documented here. The project follows
[Semantic Versioning](https://semver.org/).

## [0.3.1] - 2026-11-13

### Changed

- The TUI title bar now shows the running version when terminal width permits;
  Playback Doctor also displays the application version explicitly.

## [0.3.0] - 2026-11-13

### Added

- **Playback Doctor**: read-only TUI modal (`D`) and `lofi-player doctor`
  command with safe mpv, dependency, config, DNS, and Radio Browser API
  diagnostics.
- Safe clipboard actions for a sanitised support report and the active stream
  hostname only; reports never include stream paths, query strings, or tokens.

## [0.2.0] - 2026-11-13

### Added

- **Radio Browser discovery**: search the public station directory directly
  from the TUI with `/`.
- Automatic search after a 350 ms pause in typing, supporting plain text plus
  `tag:`, `country:`, and `lang:` filters.
- Visible playback feedback in discovery: `Loading …` immediately after
  `enter`, then `Playing … · not saved` once the stream starts.
- Inline save feedback in discovery, including success, duplicate, and
  save-failure messages.
- A persistent `Radio Browser · not saved · s save` action after leaving
  discovery, so an active unsaved station can always be added to the library.
- Cached search results as an offline fallback when Radio Browser cannot be
  reached.

### Changed

- `enter` in discovery is labelled **play**. It starts a station without
  modifying `config.yaml`; `s` is the explicit save action.
- While an unsaved Radio Browser station is playing outside discovery, `s`
  saves that active station. Once saved, `s` returns to sharing the selected
  library station.
- Metadata, stream information, and cache status now refresh reliably after
  switching between stations, even when both streams report identical values.
- TUI errors use short English messages instead of Go, mpv, or HTTP internals.

## Earlier releases

Changes before this file was introduced are available in the
[Git history](https://github.com/iRootPro/lofi-player/commits/main) and
[GitHub Releases](https://github.com/iRootPro/lofi-player/releases).
