# Changelog

All notable user-facing changes are documented here. The project follows
[Semantic Versioning](https://semver.org/).

## [0.4.0] - 2026-11-18

### Added

- **Private local listening history**: the `H` dashboard shows today, the last
  seven days, or all retained listening time, top stations, and recent sessions.
  It keeps no stream URLs or ICY metadata, excludes pauses and buffering, and
  recovers bounded checkpoints after an unexpected exit.

### Changed

- Semantic hotkeys are now unambiguous: `c` shares a saved library station,
  `s` saves an active unsaved Radio Browser preview, `v` opens discovery
  details, and `C` clears listening history. Contextual navigation keys remain
  unchanged.

## [0.3.3] - 2026-11-17

### Changed

- Radio Browser discovery now uses the available terminal width more fully.
- Selected results show a compact metadata summary; press `d` for wrapped,
  scrollable full station details and tags.

## [0.3.2] - 2026-11-15

### Security

- Release binaries now build with Go 1.27.4, which includes the fix for
  CVE-2026-97054 in the standard library.

### Changed

- CI verifies the minimum supported patched Go release (1.26.11) and Go 1.27.4.
- Updated GitHub Actions checkout and Go setup actions to v7.

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
