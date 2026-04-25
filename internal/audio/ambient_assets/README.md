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

The current placeholders are 1/2/3-second silent Opus streams generated
so mpv can decode them in tests; distinct durations keep distinct
SHA-256s. Recipe to regenerate:

```sh
ffmpeg -f lavfi -i "anullsrc=r=44100:cl=stereo" -t 1 -c:a libopus -b:a 32k -y rain.opus
ffmpeg -f lavfi -i "anullsrc=r=44100:cl=stereo" -t 2 -c:a libopus -b:a 32k -y fire.opus
ffmpeg -f lavfi -i "anullsrc=r=44100:cl=stereo" -t 3 -c:a libopus -b:a 32k -y white_noise.opus
```
