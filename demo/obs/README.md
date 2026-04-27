# OBS setup for the lofi-player demo

Imports a Scene Collection that captures the kitty window plus
system audio in one source (macOS 13+ ScreenCaptureKit), so the
recording catches both the TUI and the music coming out of mpv.

## 1. Permissions (one time)

System Settings → Privacy & Security → grant OBS:

- **Screen & System Audio Recording** — required.
- **Microphone** — required for the system-audio capture path on
  macOS (ScreenCaptureKit asks for it even though no mic is used).

Quit and reopen OBS after granting.

## 2. Import the scene

OBS → menu **Scene Collection** → **Import…** → select
`demo/obs/lofi-player-demo.scene.json` → Import → switch to it via
Scene Collection → `lofi-player demo`.

If kitty isn't shown by ID under macOS Screen Capture properties:
right-click the source → Properties → set **Method** to "Application"
and pick kitty from the list. The bundle id `net.kovidgoyal.kitty`
is what's hardcoded in the JSON.

## 3. Profile / output settings (manual, ~5 clicks)

Settings (⌘ ,) → set the values below. OBS profiles are `.ini` and
fiddly to ship as a file; clicking through is faster.

**Output**
- Output Mode: `Advanced`
- Recording → Type: `Standard`
- Recording Path: anywhere; default is `~/Movies`
- Recording Format: `mp4`
- Audio Track: ✓ `1`
- Encoder: `x264` (or `Apple VT H.264 Hardware Encoder` if you want
  it cooler / faster on Apple Silicon)
- Rate Control: `CRF`, CRF `22`
- Keyframe Interval: `2`
- Preset: `medium`

**Audio**
- Sample Rate: `48 kHz`
- Channels: `Stereo`

**Video**
- Base (Canvas) Resolution: `1280x800`
- Output (Scaled) Resolution: `1280x800`
- Common FPS: `30`

**Hotkeys**
- Start Recording: `F9`
- Stop Recording: `F10`

Apply, OK.

## 4. Sanity check

- Open the Audio Mixer panel (View → Docks → Audio Mixer if hidden).
- The `kitty (window + audio)` row should show meter movement when
  music plays in lofi-player.
- If `Desktop Audio` shows alongside it, mute it (the speaker icon)
  to avoid double-capture / echo.

## 5. Recording session

1. In kitty, run `lofi-player`.
2. F9 to start.
3. Walk through the demo (see below).
4. F10 to stop. File lands in your recording folder.

## Suggested ~35-second script

| t (s) | action | what's heard / seen |
|---|---|---|
| 0 | type `lofi-player`, Enter | UI paints |
| 3 | `j j j` (300 ms apart) | cursor walks the list |
| 6 | `space` on Lofi Girl 24/7 | spinner → ●, music starts |
| 10 | hold | bitrate / uptime / buffer fill in |
| 14 | `x`, `j j`, `l l l l l l` | rain layers under the lofi |
| 21 | `j`, `l l l l` | cafe joins in |
| 25 | `esc` | back to main view |
| 27 | `t t t t` | cycle the four themes |
| 32 | `m`, hold, `m` | mini mode toggle |
| 36 | `q` | clean exit |

## 6. Post-processing

```sh
# light, README-friendly MP4 with sound
ffmpeg -i ~/Movies/<recording>.mov \
  -ss 0:00 -to 0:36 \
  -vf "scale=1200:-2" \
  -c:v libx264 -preset slow -crf 26 \
  -c:a aac -b:a 96k \
  ../lofi-player.mp4

# silent GIF for the README hero
ffmpeg -i ~/Movies/<recording>.mov \
  -vf "fps=15,scale=1200:-1:flags=lanczos,split[s0][s1];[s0]palettegen[p];[s1][p]paletteuse" \
  -loop 0 ../lofi-player.gif
gifsicle -O3 --lossy=80 -o ../lofi-player.gif ../lofi-player.gif
```

## 7. Embedding the MP4 in the README

GitHub markdown blocks `<video>`. Workaround: open a draft Issue on
the repo, drag-drop the MP4 in, GitHub uploads it and replaces the
drop with a `https://github.com/user-attachments/assets/...` URL.
Paste that URL into README — it renders as an inline player with
sound. Don't submit the issue, just close the tab; the asset stays
hosted.
