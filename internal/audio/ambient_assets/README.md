# ambient assets

Loop files embedded into the binary via `embed.FS` and unpacked at
runtime to `$XDG_CACHE_HOME/lofi-player/ambient/` (or
`~/.cache/lofi-player/ambient/` when `XDG_CACHE_HOME` is unset).

| file | source | author | license |
|---|---|---|---|
| rain.opus | [freesound.org/s/525046](https://freesound.org/s/525046/) | speakwithanimals | CC0 |
| fire.opus | [freesound.org/s/760474](https://freesound.org/s/760474/) | True_Killian | CC0 |
| white_noise.opus | [freesound.org/s/132275](https://freesound.org/s/132275/) | assett1 | CC0 |
| cafe.opus | [freesound.org/s/32910](https://freesound.org/s/32910/) | ToddBradley | CC0 |
| thunder.opus | [freesound.org/s/717890](https://freesound.org/s/717890/) | TRP | CC0 |

All five sources are CC0 (public domain) — no attribution legally
required, but credited above as a courtesy and so future contributors
can re-find them. Encoded to Opus 64 kbps stereo for size.

Source files are not tracked in this repo; the recipe below
reproduces the embedded `.opus` files from the originals downloaded
to `~/Downloads/`. All filters share the same shape: 2-second fade-in,
3-second fade-out, EBU R128 loudness normalization to -23 LUFS.

```sh
# rain — 4-minute slice from t=6:00 of the 13:52 source
ffmpeg -ss 360 -t 240 -i ~/Downloads/525046__speakwithanimals__rain-slowly-passing-treated-loop_edgewater_06192020.wav \
  -af "afade=t=in:st=0:d=2,afade=t=out:st=237:d=3,loudnorm=I=-23:LRA=7" \
  -c:a libopus -b:a 64k -ac 2 -y rain.opus

# fire — full 1:44 source (short, but a fireplace crackle's loop seam is
# texture-masked).
ffmpeg -i ~/Downloads/760474__true_killian__fireplace.m4a \
  -af "afade=t=in:st=0:d=2,afade=t=out:st=101:d=3,loudnorm=I=-23:LRA=7" \
  -c:a libopus -b:a 64k -ac 2 -y fire.opus

# white_noise — 4-minute slice from t=36:00 of the 74-minute source
# (middle is the most stable section).
ffmpeg -ss 2160 -t 240 -i ~/Downloads/132275__assett1__74-minutes-of-relaxing-soft-noise.mp3 \
  -map 0:a -af "afade=t=in:st=0:d=2,afade=t=out:st=237:d=3,loudnorm=I=-23:LRA=7" \
  -c:a libopus -b:a 64k -ac 2 -y white_noise.opus

# cafe — 4-minute slice from t=30s of the 5:35 source (mono input,
# upmixed to stereo by libopus).
ffmpeg -ss 30 -t 240 -i ~/Downloads/32910__toddbradley__general-ambience-from-bar.wav \
  -af "afade=t=in:st=0:d=2,afade=t=out:st=237:d=3,loudnorm=I=-23:LRA=7" \
  -c:a libopus -b:a 64k -ac 2 -y cafe.opus

# thunder — 4-minute slice from t=5s of the 4:07 source.
ffmpeg -ss 5 -t 240 -i ~/Downloads/717890__trp__230823-thunder-dry-distant-rolling-r-07-em272s-stratford-12pm.mp3 \
  -af "afade=t=in:st=0:d=2,afade=t=out:st=237:d=3,loudnorm=I=-23:LRA=7" \
  -c:a libopus -b:a 64k -ac 2 -y thunder.opus
```

To swap a file later: keep the same filename, drop the new `.opus` (or
re-encode from any source via the recipe above), `go build` — embed
picks up the new bytes, the on-disk cache auto-invalidates on
SHA-256 mismatch the next time the app starts.
