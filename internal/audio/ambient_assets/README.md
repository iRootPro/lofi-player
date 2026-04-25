# ambient assets

Loop files embedded into the binary via `embed.FS` and unpacked at
runtime to `$XDG_CACHE_HOME/lofi-player/ambient/` (or
`~/.cache/lofi-player/ambient/` when `XDG_CACHE_HOME` is unset).

| file | source | author | license |
|---|---|---|---|
| rain.opus | placeholder | — | — |
| fire.opus | placeholder | — | — |
| white_noise.opus | placeholder | — | — |

Replace placeholders with real loops before tagging v0.4.0. Targets:
- Format: Opus in OGG, ~64 kbps stereo
- Length: 3–5 minutes per loop
- License: prefer CC0; CC-BY acceptable with attribution here and in
  root `ATTRIBUTIONS.md`
- Smooth start/end so the loop seam is inaudible (use ffmpeg `afade` or
  Audacity crossfade)
