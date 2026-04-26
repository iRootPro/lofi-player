# ambient assets

Loop files embedded into the binary via `embed.FS` and unpacked at
runtime to `$XDG_CACHE_HOME/lofi-player/ambient/` (or
`~/.cache/lofi-player/ambient/` when `XDG_CACHE_HOME` is unset).

| file | source | author | license |
|---|---|---|---|
| rain.opus | synthesized (ffmpeg) | this project | original (no third-party content) |
| fire.opus | synthesized (ffmpeg) | this project | original (no third-party content) |
| white_noise.opus | synthesized (ffmpeg) | this project | original (no third-party content) |

The current loops are synthesized from ffmpeg's `anoisesrc` generator —
filtered noise textures rather than field recordings. They sound
recognizably distinct (rain-like vs. low rumble vs. clean static) and
loop seamlessly because the underlying noise is statistically uniform,
but they are not literal recordings of rain or fire. For v1.0 release
consider replacing with real CC0/CC-BY recordings from freesound.org —
but the synthesized versions are good enough for shipping a first
release (similar to what mynoise.net and Brain.fm ship as backgrounds).

Recipe to regenerate (4 minutes each, ~1.4 MB at 64 kbps Opus stereo):

```sh
# rain — pink noise, high-pass + lowpass to feel like falling water,
# light tremolo for organic "drops" texture.
ffmpeg -f lavfi -i "anoisesrc=color=pink:duration=240:sample_rate=44100:amplitude=0.45" \
  -af "highpass=f=500, lowpass=f=7000, tremolo=f=6:d=0.15, afade=t=in:st=0:d=2, afade=t=out:st=237:d=3, loudnorm=I=-23:LRA=7" \
  -c:a libopus -b:a 64k -ac 2 -y rain.opus

# fire — brown noise, low-passed and slow tremolo for a warm rumble.
ffmpeg -f lavfi -i "anoisesrc=color=brown:duration=240:sample_rate=44100:amplitude=0.7" \
  -af "lowpass=f=1500, tremolo=f=0.7:d=0.45, afade=t=in:st=0:d=2, afade=t=out:st=237:d=3, loudnorm=I=-23:LRA=7" \
  -c:a libopus -b:a 64k -ac 2 -y fire.opus

# white noise — brown noise with rumble removed (high-pass at 80 Hz),
# clean focus background.
ffmpeg -f lavfi -i "anoisesrc=color=brown:duration=240:sample_rate=44100:amplitude=0.5" \
  -af "highpass=f=80, afade=t=in:st=0:d=2, afade=t=out:st=237:d=3, loudnorm=I=-23:LRA=7" \
  -c:a libopus -b:a 64k -ac 2 -y white_noise.opus
```

If you swap any file for a real CC-BY recording later, add the source
URL, author, and license to the table above and to a root
`ATTRIBUTIONS.md`.
