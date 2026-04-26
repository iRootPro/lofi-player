# Ambient Mixer Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a 3-channel ambient mixer (rain / fire / white noise) that plays alongside the main station, with embedded loop files, an `x`-opened modal, and a compact active-channel indicator on the main view.

**Architecture:** New `internal/audio/ambient.go` package owns one `AmbientMixer` with three `AmbientChannel`s, each backed by an independent mpv subprocess (reusing existing `audio.NewPlayer` patterns minus stream/event translation). Loop files are embedded via `embed.FS` and extracted on first run to `~/.cache/lofi-player/ambient/`. State persists via a new `Ambient map[string]int` field in `state.State`. UI adds a new `viewMode` (`modeMixer`) plus a fresh `mixerModel` rendered as a centered card.

**Tech Stack:** Go 1.26, `embed.FS`, mpv JSON-IPC (existing IPC client in `internal/audio/mpv.go`), Bubble Tea, lipgloss.

**Design reference:** `plans/2026-04-25-ambient-mixer-design.md` (committed on main as `27e0a27`).

**Branch:** `ambient-mixer` (worktree at `.worktrees/ambient-mixer/`).

**Conventions (from CLAUDE.md / repo memory):**
- Commit messages in English. **No `Co-Authored-By: Claude` trailer, no `Generated with Claude Code` footer.**
- Comments explain WHY, not WHAT. Default to no comments.
- Match existing code style (value-receiver Bubble Tea models, `clampVolume`-style helpers, atomic temp+rename for file writes, etc.).
- Russian text only in user-facing strings if pre-existing; everything new in English.

---

## Task overview

| # | Task | Files | LOC est. |
|---|---|---|---|
| 1 | Scaffold ambient channel registry | `internal/audio/ambient.go` (new) + test | ~80 |
| 2 | Embed assets + extract-on-init | `internal/audio/ambient_assets/` + ambient.go | ~120 |
| 3 | Per-channel mpv lifecycle | `internal/audio/ambient_player.go` (new) + test | ~150 |
| 4 | Mixer SetVolume / pause-unpause | ambient.go + test | ~60 |
| 5 | Mixer Volumes / ActiveIDs / Close | ambient.go + test | ~50 |
| 6 | State extension | `internal/state/state.go` + test | ~30 |
| 7 | KeyMap: add MixerOpen / MixerClose | `internal/tui/keys.go` + test | ~30 |
| 8 | Model: viewMixer mode + open/close | `internal/tui/model.go`, `update.go` | ~60 |
| 9 | mixerModel struct | `internal/tui/mixer.go` (new) + test | ~80 |
| 10 | Mixer keybindings: nav + adjust | `internal/tui/mixer.go` + test | ~80 |
| 11 | Mixer view rendering | `internal/tui/mixer.go` + test | ~100 |
| 12 | Global keys disabled in mixer modal | already by Task 8 dispatch; tests | ~30 |
| 13 | Debounced state save | `internal/tui/commands.go`, model | ~50 |
| 14 | Ambient indicator on main view | `internal/tui/view.go` + test | ~50 |
| 15 | main.go wire-up | `main.go` | ~30 |
| 16 | Loop file placeholders + README | `internal/audio/ambient_assets/` | docs only |
| 17 | Manual verification | (no code) | — |

Total ~1000 lines including tests.

---

## Task 1: Scaffold ambient channel registry

Set up the type definitions and the static channel list. No mpv yet — we just want `NewAmbientMixer` to construct a struct with three channels in a fixed order with the right metadata.

**Files:**
- Create: `internal/audio/ambient.go`
- Create: `internal/audio/ambient_test.go`

### Step 1: Write the failing test

Append to `internal/audio/ambient_test.go`:

```go
package audio

import "testing"

func TestNewAmbientMixerHasThreeChannelsInFixedOrder(t *testing.T) {
	m := NewAmbientMixer()
	got := m.ChannelIDs()
	want := []string{"rain", "fire", "white_noise"}
	if len(got) != len(want) {
		t.Fatalf("ChannelIDs: got %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("ChannelIDs[%d]: got %q, want %q", i, got[i], want[i])
		}
	}
}

func TestAmbientChannelMetadata(t *testing.T) {
	m := NewAmbientMixer()
	cases := []struct {
		id    string
		label string
	}{
		{"rain", "rain"},
		{"fire", "fire"},
		{"white_noise", "white noise"},
	}
	for _, c := range cases {
		ch, ok := m.Channel(c.id)
		if !ok {
			t.Fatalf("Channel(%q): not found", c.id)
		}
		if ch.Label != c.label {
			t.Errorf("Channel(%q).Label: got %q, want %q", c.id, ch.Label, c.label)
		}
		if ch.Icon == "" {
			t.Errorf("Channel(%q).Icon: empty", c.id)
		}
	}
}
```

### Step 2: Run test, verify it fails

```
cd .worktrees/ambient-mixer
go test ./internal/audio/ -run TestNewAmbientMixer -v
go test ./internal/audio/ -run TestAmbientChannelMetadata -v
```

Expected: compile error (`undefined: NewAmbientMixer`).

### Step 3: Minimal implementation

Create `internal/audio/ambient.go`:

```go
// Package audio's ambient mixer plays N independent loop files alongside
// the main station. Each channel is an mpv subprocess with its own
// volume; the OS audio stack does the mixing.
//
// Channel metadata (id, label, icon, embedded asset path) is fixed at
// compile time. Adding a channel is a one-line edit to ambientChannels
// plus a new file in ambient_assets/.
package audio

// AmbientChannel is the static metadata for one ambient channel.
// Volume and runtime state live on the mixer, not here.
type AmbientChannel struct {
	ID    string
	Label string
	Icon  string
	asset string // path inside ambient_assets/
}

// ambientChannels is the fixed list. Order is significant — used by
// ChannelIDs and ActiveIDs and shown in the UI.
var ambientChannels = []AmbientChannel{
	{ID: "rain", Label: "rain", Icon: "🌧️", asset: "rain.opus"},
	{ID: "fire", Label: "fire", Icon: "🔥", asset: "fire.opus"},
	{ID: "white_noise", Label: "white noise", Icon: "⚪", asset: "white_noise.opus"},
}

// AmbientMixer is the public coordinator over all channels. Construct
// with NewAmbientMixer, then call Init to bring up mpv subprocesses.
type AmbientMixer struct {
	channels []AmbientChannel
}

func NewAmbientMixer() *AmbientMixer {
	return &AmbientMixer{channels: append([]AmbientChannel(nil), ambientChannels...)}
}

// ChannelIDs returns channel IDs in the canonical order.
func (m *AmbientMixer) ChannelIDs() []string {
	ids := make([]string, len(m.channels))
	for i, c := range m.channels {
		ids[i] = c.ID
	}
	return ids
}

// Channel returns the static metadata for id, or zero+false if unknown.
func (m *AmbientMixer) Channel(id string) (AmbientChannel, bool) {
	for _, c := range m.channels {
		if c.ID == id {
			return c, true
		}
	}
	return AmbientChannel{}, false
}
```

### Step 4: Run test, verify it passes

```
go test ./internal/audio/ -run TestNewAmbientMixer -v
go test ./internal/audio/ -run TestAmbientChannelMetadata -v
go test ./internal/audio/ -v
```

Expected: PASS, plus all pre-existing audio tests still PASS.

### Step 5: Commit

```bash
git add internal/audio/ambient.go internal/audio/ambient_test.go
git commit -m "audio: scaffold ambient channel registry"
```

---

## Task 2: Embed assets + extract-on-init with SHA-256 verify

Embed loop files via `embed.FS`. Implement `Init()` that resolves the cache dir under `os.UserCacheDir`, writes each embedded file to disk if absent or hash-mismatched, and stores the on-disk path on each channel.

For the test, we ship tiny placeholder bytes for the assets so the test runs offline. The real `.opus` files are added in Task 16; for now just three small dummy files (1-2 bytes each) are enough to validate the extract logic.

**Files:**
- Create: `internal/audio/ambient_assets/rain.opus` (placeholder bytes)
- Create: `internal/audio/ambient_assets/fire.opus` (placeholder bytes)
- Create: `internal/audio/ambient_assets/white_noise.opus` (placeholder bytes)
- Create: `internal/audio/ambient_assets/README.md`
- Modify: `internal/audio/ambient.go`
- Modify: `internal/audio/ambient_test.go`

### Step 1: Create placeholder asset files

**Important:** We use distinct, recognizable bytes so the SHA-256 mismatch test has something to flip.

```bash
printf 'rain-placeholder-v1\n' > internal/audio/ambient_assets/rain.opus
printf 'fire-placeholder-v1\n' > internal/audio/ambient_assets/fire.opus
printf 'wn-placeholder-v1\n'   > internal/audio/ambient_assets/white_noise.opus
```

Create `internal/audio/ambient_assets/README.md`:

```markdown
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
```

### Step 2: Write the failing test

Append to `internal/audio/ambient_test.go`:

```go
import (
	"crypto/sha256"
	"os"
	"path/filepath"
)

func TestMixerExtractsEmbedToCache(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	m := NewAmbientMixer()
	if err := m.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer m.Close()

	cacheDir, _ := m.cacheDir()
	for _, id := range []string{"rain", "fire", "white_noise"} {
		path := filepath.Join(cacheDir, id+".opus")
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected %s on disk: %v", path, err)
		}
	}
}

func TestMixerSkipsExtractIfHashMatches(t *testing.T) {
	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)

	m1 := NewAmbientMixer()
	if err := m1.Init(); err != nil {
		t.Fatalf("Init #1: %v", err)
	}
	m1.Close()

	target := filepath.Join(cache, "lofi-player", "ambient", "rain.opus")
	info1, err := os.Stat(target)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	mtime1 := info1.ModTime()

	// Wait a bit so a rewrite would change mtime.
	if err := os.Chtimes(target, mtime1.Add(-time.Hour), mtime1.Add(-time.Hour)); err != nil {
		t.Fatalf("chtimes: %v", err)
	}
	mtimeBackdated, _ := os.Stat(target)

	m2 := NewAmbientMixer()
	if err := m2.Init(); err != nil {
		t.Fatalf("Init #2: %v", err)
	}
	m2.Close()

	info2, _ := os.Stat(target)
	if !info2.ModTime().Equal(mtimeBackdated.ModTime()) {
		t.Errorf("file was rewritten despite matching hash (mtime changed)")
	}
}

func TestMixerOverwritesIfHashMismatch(t *testing.T) {
	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)

	dir := filepath.Join(cache, "lofi-player", "ambient")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	target := filepath.Join(dir, "rain.opus")
	bogus := []byte("not the real bytes")
	if err := os.WriteFile(target, bogus, 0o644); err != nil {
		t.Fatalf("write bogus: %v", err)
	}

	m := NewAmbientMixer()
	if err := m.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer m.Close()

	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read after init: %v", err)
	}
	if sha256.Sum256(got) == sha256.Sum256(bogus) {
		t.Errorf("file still has bogus content: %q", got)
	}
}
```

(Remember to add `"time"` to the imports.)

### Step 3: Run test, verify it fails

```
go test ./internal/audio/ -run TestMixerExtracts -v
```

Expected: compile error (`m.Init undefined`, `m.Close undefined`, `m.cacheDir undefined`).

### Step 4: Implementation

Modify `internal/audio/ambient.go`:

```go
package audio

import (
	"crypto/sha256"
	"embed"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

//go:embed ambient_assets/*.opus
var ambientFS embed.FS

// (keep AmbientChannel / ambientChannels / AmbientMixer / NewAmbientMixer
// from Task 1)

// runtimeChannel pairs static metadata with on-disk path. Lives in
// AmbientMixer.runtime; channels exposed via Channel() show only the
// static half.
type runtimeChannel struct {
	meta     AmbientChannel
	filePath string
	disabled bool
}

// Mutate AmbientMixer to hold runtime state:
type AmbientMixer struct {
	channels []AmbientChannel  // (existing, keep)
	runtime  []*runtimeChannel // populated by Init
}

// Init extracts embedded loop files to the OS cache dir and prepares
// per-channel runtime state. mpv subprocesses are NOT started here —
// that's Task 3.
func (m *AmbientMixer) Init() error {
	dir, err := m.cacheDir()
	if err != nil {
		return fmt.Errorf("ambient cache dir: %w", err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}
	m.runtime = make([]*runtimeChannel, len(m.channels))
	for i, c := range m.channels {
		path, err := m.materialize(dir, c)
		if err != nil {
			return fmt.Errorf("materialize %s: %w", c.ID, err)
		}
		m.runtime[i] = &runtimeChannel{meta: c, filePath: path}
	}
	return nil
}

// Close releases resources. With no mpv yet (Task 3), this is a no-op
// stub — keep it so tests already call it and we don't churn signatures.
func (m *AmbientMixer) Close() error { return nil }

func (m *AmbientMixer) cacheDir() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "lofi-player", "ambient"), nil
}

// materialize ensures the embedded asset for c is present on disk under
// dir. Returns the absolute file path. Existing on-disk files with a
// matching SHA-256 are left untouched.
func (m *AmbientMixer) materialize(dir string, c AmbientChannel) (string, error) {
	embedPath := filepath.Join("ambient_assets", c.asset)
	want, err := ambientFS.ReadFile(embedPath)
	if err != nil {
		return "", fmt.Errorf("read embed %s: %w", embedPath, err)
	}
	wantSum := sha256.Sum256(want)

	target := filepath.Join(dir, c.asset)
	if existing, err := os.ReadFile(target); err == nil {
		if sha256.Sum256(existing) == wantSum {
			return target, nil
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}

	tmp, err := os.CreateTemp(dir, ".ambient-*.opus")
	if err != nil {
		return "", err
	}
	tmpName := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpName) }
	if _, err := io.Copy(tmp, bytesReader(want)); err != nil {
		_ = tmp.Close()
		cleanup()
		return "", err
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return "", err
	}
	if err := os.Rename(tmpName, target); err != nil {
		cleanup()
		return "", err
	}
	return target, nil
}

// bytesReader is a tiny helper so the materialize body stays uniform —
// using bytes.NewReader directly works just as well; kept inline only
// to avoid an extra import in this snippet.
func bytesReader(b []byte) io.Reader { return &readerFromBytes{b: b} }

type readerFromBytes struct{ b []byte; off int }

func (r *readerFromBytes) Read(p []byte) (int, error) {
	if r.off >= len(r.b) {
		return 0, io.EOF
	}
	n := copy(p, r.b[r.off:])
	r.off += n
	return n, nil
}
```

**Note on `bytesReader`:** swap for `bytes.NewReader(want)` from `bytes` package — cleaner. Use the standard library form, the snippet above shows the shape.

### Step 5: Run test, verify it passes

```
go test ./internal/audio/ -v
```

Expected: PASS.

### Step 6: Commit

```bash
git add internal/audio/ambient.go internal/audio/ambient_test.go internal/audio/ambient_assets/
git commit -m "audio: embed ambient loop files and extract on init"
```

---

## Task 3: Per-channel mpv subprocess lifecycle

Bring up an mpv subprocess for each channel during `Init()`, configured with `--loop-file=inf` and `--idle=yes`, paused at volume 0. Add `Close()` that terminates all subprocesses in parallel. Add graceful degradation: if one mpv fails to start, mark the channel `disabled` and continue with the rest.

**Files:**
- Create: `internal/audio/ambient_player.go`
- Modify: `internal/audio/ambient.go` (Init, Close)
- Modify: `internal/audio/ambient_test.go`

### Step 1: Write the failing test

The test must be skippable when mpv is not on `$PATH` so CI without mpv still works. Use the same skip pattern as `internal/audio/player_test.go` would (read its existing skip pattern; if none, add `if _, err := exec.LookPath("mpv"); err != nil { t.Skip("mpv not installed") }`).

Append to `internal/audio/ambient_test.go`:

```go
import "os/exec"

func TestMixerSpawnsMpvSubprocesses(t *testing.T) {
	if _, err := exec.LookPath("mpv"); err != nil {
		t.Skip("mpv not installed")
	}
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	m := NewAmbientMixer()
	if err := m.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer m.Close()

	for _, id := range m.ChannelIDs() {
		if m.Disabled(id) {
			t.Errorf("channel %s unexpectedly disabled", id)
		}
	}
}

func TestMixerCloseTerminatesSubprocesses(t *testing.T) {
	if _, err := exec.LookPath("mpv"); err != nil {
		t.Skip("mpv not installed")
	}
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	m := NewAmbientMixer()
	if err := m.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := m.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	// Idempotency:
	if err := m.Close(); err != nil {
		t.Errorf("second Close: %v", err)
	}
}
```

### Step 2: Run test, verify it fails

```
go test ./internal/audio/ -run TestMixerSpawns -v
```

Expected: compile error (`m.Disabled undefined`).

### Step 3: Implementation

Create `internal/audio/ambient_player.go`. Reuse helpers from `player.go` — `waitForSocketOrExit`, `dialIPC`, `clampVolume` already exist.

```go
package audio

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

// ambientPlayer is a thin per-channel wrapper. Unlike Player it has no
// event translation, no metadata observation, no playback state machine
// — just spawn, set pause, set volume, quit.
type ambientPlayer struct {
	cmd       *exec.Cmd
	ipc       *ipcClient
	socketDir string
	closeOnce sync.Once
}

// newAmbientPlayer spawns mpv looping the given file at volume 0,
// paused. ctx bounds startup only.
func newAmbientPlayer(ctx context.Context, filePath string) (*ambientPlayer, error) {
	socketDir, err := os.MkdirTemp("/tmp", "lofi-ambient-*")
	if err != nil {
		socketDir, err = os.MkdirTemp("", "lofi-ambient-*")
		if err != nil {
			return nil, fmt.Errorf("create socket dir: %w", err)
		}
	}
	socketPath := filepath.Join(socketDir, "mpv.sock")

	cmd := exec.Command("mpv",
		"--idle=no",
		"--no-video",
		"--no-terminal",
		"--really-quiet",
		"--loop-file=inf",
		"--volume=0",
		"--pause=yes",
		"--input-ipc-server="+socketPath,
		filePath,
	)
	if err := cmd.Start(); err != nil {
		os.RemoveAll(socketDir)
		return nil, fmt.Errorf("start mpv: %w", err)
	}

	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()

	if err := waitForSocketOrExit(ctx, socketPath, 5*time.Second, exited); err != nil {
		select {
		case <-exited:
		default:
			_ = cmd.Process.Kill()
			<-exited
		}
		os.RemoveAll(socketDir)
		return nil, fmt.Errorf("mpv socket did not appear: %w", err)
	}

	ipc, err := dialIPC(socketPath)
	if err != nil {
		select {
		case <-exited:
		default:
			_ = cmd.Process.Kill()
			<-exited
		}
		os.RemoveAll(socketDir)
		return nil, fmt.Errorf("dial mpv: %w", err)
	}
	return &ambientPlayer{cmd: cmd, ipc: ipc, socketDir: socketDir}, nil
}

func (p *ambientPlayer) setVolume(v int) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := p.ipc.command(ctx, "set_property", "volume", clampVolume(v))
	return err
}

func (p *ambientPlayer) setPaused(paused bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := p.ipc.command(ctx, "set_property", "pause", paused)
	return err
}

func (p *ambientPlayer) close() {
	p.closeOnce.Do(func() {
		if p.ipc != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
			_, _ = p.ipc.command(ctx, "quit")
			cancel()
			_ = p.ipc.close()
		}
		if p.cmd != nil && p.cmd.Process != nil {
			done := make(chan struct{})
			go func() { _ = p.cmd.Wait(); close(done) }()
			select {
			case <-done:
			case <-time.After(2 * time.Second):
				_ = p.cmd.Process.Kill()
				<-done
			}
		}
		if p.socketDir != "" {
			_ = os.RemoveAll(p.socketDir)
		}
	})
}
```

Update `internal/audio/ambient.go`:

```go
// Add field on runtimeChannel:
type runtimeChannel struct {
	meta     AmbientChannel
	filePath string
	player   *ambientPlayer
	disabled bool
}

// Add field on AmbientMixer:
type AmbientMixer struct {
	channels []AmbientChannel
	runtime  []*runtimeChannel
	closed   bool
	mu       sync.Mutex
}

// Init: after materialize, spawn mpv per channel. On failure mark
// disabled and continue.
func (m *AmbientMixer) Init() error {
	// (existing dir + MkdirAll)
	m.runtime = make([]*runtimeChannel, len(m.channels))
	for i, c := range m.channels {
		path, err := m.materialize(dir, c)
		if err != nil {
			return fmt.Errorf("materialize %s: %w", c.ID, err)
		}
		rc := &runtimeChannel{meta: c, filePath: path}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		p, err := newAmbientPlayer(ctx, path)
		cancel()
		if err != nil {
			rc.disabled = true
		} else {
			rc.player = p
		}
		m.runtime[i] = rc
	}
	return nil
}

// Close terminates every running subprocess in parallel.
func (m *AmbientMixer) Close() error {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return nil
	}
	m.closed = true
	rt := m.runtime
	m.mu.Unlock()

	var wg sync.WaitGroup
	for _, rc := range rt {
		if rc == nil || rc.player == nil {
			continue
		}
		wg.Add(1)
		go func(p *ambientPlayer) {
			defer wg.Done()
			p.close()
		}(rc.player)
	}
	wg.Wait()
	return nil
}

// Disabled returns true if the channel exists but its mpv didn't come
// up. Used by the UI to render the slider as "unavailable".
func (m *AmbientMixer) Disabled(id string) bool {
	for _, rc := range m.runtime {
		if rc != nil && rc.meta.ID == id {
			return rc.disabled
		}
	}
	return false
}
```

(Adjust imports: add `context`, `sync`.)

### Step 4: Run test, verify it passes

```
go test ./internal/audio/ -v
```

Expected: PASS (TestMixerSpawnsMpvSubprocesses runs if mpv on PATH, otherwise SKIP).

### Step 5: Commit

```bash
git add internal/audio/ambient.go internal/audio/ambient_player.go internal/audio/ambient_test.go
git commit -m "audio: spawn mpv per ambient channel on init"
```

---

## Task 4: Mixer SetVolume API with pause/unpause semantics

Add `(*AmbientMixer).SetVolume(id string, v int) error`. Volume 0 → set volume + paused. Volume > 0 → set volume + unpaused. Disabled or unknown channels return a sentinel error so the UI can ignore them silently.

**Files:**
- Modify: `internal/audio/ambient.go`
- Modify: `internal/audio/ambient_test.go`

### Step 1: Failing tests

Append to `internal/audio/ambient_test.go`:

```go
func TestMixerSetVolumeSkippedWhenDisabled(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	m := NewAmbientMixer()
	if err := m.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer m.Close()

	// On systems without mpv, every channel is disabled — SetVolume
	// must not error, just skip silently.
	for _, id := range m.ChannelIDs() {
		if !m.Disabled(id) {
			continue
		}
		if err := m.SetVolume(id, 50); err != nil {
			t.Errorf("SetVolume on disabled %s: %v, want nil", id, err)
		}
	}
}

func TestMixerSetVolumeUnknownChannel(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	m := NewAmbientMixer()
	if err := m.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer m.Close()

	if err := m.SetVolume("does_not_exist", 50); err == nil {
		t.Error("SetVolume(unknown): nil error, want error")
	}
}

// Real-mpv test (skip if no mpv).
func TestMixerSetVolumeUnpauses(t *testing.T) {
	if _, err := exec.LookPath("mpv"); err != nil {
		t.Skip("mpv not installed")
	}
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	m := NewAmbientMixer()
	if err := m.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer m.Close()

	if err := m.SetVolume("rain", 30); err != nil {
		t.Fatalf("SetVolume: %v", err)
	}
	if v := m.Volume("rain"); v != 30 {
		t.Errorf("Volume: got %d, want 30", v)
	}
}
```

### Step 2: Run, verify failing

```
go test ./internal/audio/ -run TestMixerSetVolume -v
```

Expected: compile error (`m.SetVolume undefined`, `m.Volume undefined`).

### Step 3: Implementation

Add to `internal/audio/ambient.go`:

```go
// errUnknownChannel is returned by SetVolume / Volume when id doesn't
// match any registered channel. UI code should treat this as a bug.
var errUnknownChannel = errors.New("unknown ambient channel")

// SetVolume updates the volume of a single channel and pauses or
// unpauses it accordingly. Disabled channels (mpv didn't start)
// silently no-op so the UI can keep showing the slider without
// special-casing.
func (m *AmbientMixer) SetVolume(id string, v int) error {
	rc := m.find(id)
	if rc == nil {
		return errUnknownChannel
	}
	v = clampVolume(v)
	rc.volume = v
	if rc.disabled || rc.player == nil {
		return nil
	}
	if err := rc.player.setVolume(v); err != nil {
		return fmt.Errorf("set %s volume: %w", id, err)
	}
	if err := rc.player.setPaused(v == 0); err != nil {
		return fmt.Errorf("set %s pause: %w", id, err)
	}
	return nil
}

// Volume returns the current volume of id, or 0 for unknown ids.
func (m *AmbientMixer) Volume(id string) int {
	if rc := m.find(id); rc != nil {
		return rc.volume
	}
	return 0
}

func (m *AmbientMixer) find(id string) *runtimeChannel {
	for _, rc := range m.runtime {
		if rc != nil && rc.meta.ID == id {
			return rc
		}
	}
	return nil
}
```

Add `volume int` field to `runtimeChannel`.

### Step 4: Run, verify passing

```
go test ./internal/audio/ -v
```

### Step 5: Commit

```bash
git add internal/audio/ambient.go internal/audio/ambient_test.go
git commit -m "audio: ambient mixer set-volume with pause-on-zero"
```

---

## Task 5: Volumes / ActiveIDs snapshot APIs

**Files:**
- Modify: `internal/audio/ambient.go`
- Modify: `internal/audio/ambient_test.go`

### Step 1: Failing tests

```go
func TestMixerVolumesSnapshot(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	m := NewAmbientMixer()
	_ = m.Init()
	defer m.Close()

	// All zero by default.
	got := m.Volumes()
	for _, id := range m.ChannelIDs() {
		if v := got[id]; v != 0 {
			t.Errorf("default Volumes[%s]: got %d, want 0", id, v)
		}
	}

	_ = m.SetVolume("rain", 40)
	_ = m.SetVolume("white_noise", 25)

	got = m.Volumes()
	if got["rain"] != 40 || got["white_noise"] != 25 || got["fire"] != 0 {
		t.Errorf("Volumes after sets: %+v", got)
	}
}

func TestMixerActiveIDsOrder(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	m := NewAmbientMixer()
	_ = m.Init()
	defer m.Close()

	if got := m.ActiveIDs(); len(got) != 0 {
		t.Errorf("default ActiveIDs: %v, want empty", got)
	}

	_ = m.SetVolume("white_noise", 10) // last in canonical order
	_ = m.SetVolume("rain", 5)         // first in canonical order

	got := m.ActiveIDs()
	want := []string{"rain", "white_noise"} // fixed order
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("ActiveIDs: got %v, want %v", got, want)
	}
}
```

### Step 2: Run, verify failing → Step 3: Implementation

```go
// Volumes returns a snapshot of every channel's current volume keyed by
// id. Disabled channels show 0 (their stored volume) — they're still
// part of the registry, just not playing.
func (m *AmbientMixer) Volumes() map[string]int {
	out := make(map[string]int, len(m.runtime))
	for _, rc := range m.runtime {
		if rc == nil {
			continue
		}
		out[rc.meta.ID] = rc.volume
	}
	return out
}

// ActiveIDs returns the IDs of channels whose volume > 0, in canonical
// (registry) order. Used by the main-view indicator and to choose
// initial state on Init.
func (m *AmbientMixer) ActiveIDs() []string {
	var out []string
	for _, rc := range m.runtime {
		if rc != nil && rc.volume > 0 {
			out = append(out, rc.meta.ID)
		}
	}
	return out
}
```

### Step 4: Run, verify passing → Step 5: Commit

```bash
git add internal/audio/ambient.go internal/audio/ambient_test.go
git commit -m "audio: ambient mixer Volumes and ActiveIDs snapshots"
```

---

## Task 6: state.State.Ambient extension

**Files:**
- Modify: `internal/state/state.go`
- Modify: `internal/state/state_test.go`

### Step 1: Failing tests

Append to `internal/state/state_test.go`:

```go
func TestRoundtripStateWithAmbient(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	in := &State{
		Theme:           "tokyo-night",
		Volume:          70,
		LastStationName: "SomaFM Drone Zone",
		Ambient:         map[string]int{"rain": 40, "fire": 0, "white_noise": 25},
	}
	if err := Save(in); err != nil {
		t.Fatalf("Save: %v", err)
	}
	out := Load()
	if out.Ambient["rain"] != 40 || out.Ambient["fire"] != 0 || out.Ambient["white_noise"] != 25 {
		t.Errorf("Ambient roundtrip: %+v", out.Ambient)
	}
}

func TestLoadStateWithoutAmbientField(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	p, _ := Path()
	_ = os.MkdirAll(filepath.Dir(p), 0o755)
	old := []byte(`{"theme":"tokyo-night","volume":60,"last_station_name":"x"}`)
	if err := os.WriteFile(p, old, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	out := Load()
	if out.Theme != "tokyo-night" || out.Volume != 60 {
		t.Errorf("classic fields lost: %+v", out)
	}
	if out.Ambient != nil {
		t.Errorf("Ambient on legacy file: %+v, want nil", out.Ambient)
	}
}

func TestLoadStateUnknownAmbientKeyPreserved(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	p, _ := Path()
	_ = os.MkdirAll(filepath.Dir(p), 0o755)
	future := []byte(`{"ambient":{"rain":10,"cafe":50}}`)
	if err := os.WriteFile(p, future, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	out := Load()
	if out.Ambient["rain"] != 10 {
		t.Errorf("known key dropped: %+v", out.Ambient)
	}
	if v, ok := out.Ambient["cafe"]; !ok || v != 50 {
		t.Errorf("unknown key dropped: %+v", out.Ambient)
	}
}
```

### Step 2: Run, verify failing → Step 3: Implementation

`internal/state/state.go`:

```go
type State struct {
	Theme            string         `json:"theme,omitempty"`
	Volume           int            `json:"volume,omitempty"`
	LastStationName  string         `json:"last_station_name,omitempty"`
	Ambient          map[string]int `json:"ambient,omitempty"`
}
```

### Step 4: Run, verify passing → Step 5: Commit

```bash
git add internal/state/state.go internal/state/state_test.go
git commit -m "state: persist ambient channel volumes"
```

---

## Task 7: KeyMap entries for the mixer modal

Add `MixerOpen` (`x`), `MixerClose` (`x` / `esc`) to `internal/tui/keys.go`. Wire `MixerOpen` into the FullHelp grouping.

**Files:**
- Modify: `internal/tui/keys.go`
- Modify: `internal/tui/model_test.go` (or new `keys_test.go` if no existing keymap test)

### Step 1: Failing test

Add to `internal/tui/model_test.go`:

```go
func TestKeyMapHasMixerOpen(t *testing.T) {
	km := DefaultKeyMap()
	if len(km.MixerOpen.Keys()) == 0 {
		t.Error("MixerOpen has no keys bound")
	}
	for _, k := range km.MixerOpen.Keys() {
		if k == "x" {
			return
		}
	}
	t.Error("MixerOpen does not include 'x'")
}
```

### Step 2: Run, verify failing → Step 3: Implementation

Edit `internal/tui/keys.go`:

```go
type KeyMap struct {
	Up         key.Binding
	Down       key.Binding
	PlayPause  key.Binding
	VolUp      key.Binding
	VolDown    key.Binding
	ThemeCycle key.Binding
	Mini       key.Binding
	AddStation key.Binding
	MixerOpen  key.Binding // new
	Help       key.Binding
	Quit       key.Binding
}

// In DefaultKeyMap, after AddStation:
MixerOpen: key.NewBinding(
	key.WithKeys("x"),
	key.WithHelp("x", "mixer"),
),

// In FullHelp, add MixerOpen to the first group (alongside AddStation):
{k.Up, k.Down, k.AddStation, k.MixerOpen},
```

### Step 4: Run, verify passing → Step 5: Commit

```bash
git add internal/tui/keys.go internal/tui/model_test.go
git commit -m "tui: bind 'x' to open ambient mixer"
```

---

## Task 8: Model — viewMixer mode + open/close transitions

Add `modeMixer` to `viewMode`. Wire `x` (when in `modeFull`/`modeMini`) to switch into `modeMixer`. While in `modeMixer`, route all messages to a new `updateMixer` method (Task 10). `esc` and `x` close back to `modePrev`.

**Files:**
- Modify: `internal/tui/model.go`
- Modify: `internal/tui/update.go`
- Modify: `internal/tui/model_test.go`

### Step 1: Failing test

```go
func TestPressXOpensMixer(t *testing.T) {
	m := newTestModel(t)  // existing helper that builds Model with sample cfg
	m, _ = updateKey(m, "x").(Model), nil
	if m.mode != modeMixer {
		t.Errorf("mode after x: got %v, want modeMixer", m.mode)
	}
}

func TestPressEscClosesMixer(t *testing.T) {
	m := newTestModel(t)
	m, _ = updateKey(m, "x").(Model), nil
	m, _ = updateKey(m, "esc").(Model), nil
	if m.mode != modeFull {
		t.Errorf("mode after esc: got %v, want modeFull", m.mode)
	}
}
```

(Inspect `internal/tui/model_test.go` for the existing helpers — `newTestModel`, `updateKey` — and use whichever names already exist. If none, write a 5-line helper that builds a Model with a 1-station cfg and dispatches `tea.KeyMsg{Runes: []rune(s)}`.)

### Step 2: Run, verify failing → Step 3: Implementation

`model.go`:

```go
const (
	modeFull viewMode = iota
	modeMini
	modeAddStation
	modeMixer
)
```

Add field on `Model`:

```go
mixer    *audio.AmbientMixer  // injected by main; never nil after NewModel
mixerUI  mixerModel           // see Task 9
```

Update `NewModel` signature to accept the mixer:

```go
func NewModel(cfg *config.Config, player *audio.Player, mixer *audio.AmbientMixer, opts Options) Model {
	// ... existing body ...
	mm := newMixerModel(mixer)  // see Task 9
	return Model{
		// ... existing fields ...
		mixer:   mixer,
		mixerUI: mm,
	}
}
```

`update.go` — extend the leading dispatch and `handleKey`:

```go
// At top of Update:
if m.mode == modeMixer {
	return m.updateMixer(msg)
}

// In handleKey, before Help:
case key.Matches(msg, m.keys.MixerOpen):
	m.modePrev = m.mode
	m.mode = modeMixer
	return m, nil
```

Stub `updateMixer` (real logic in Task 10):

```go
func (m Model) updateMixer(msg tea.Msg) (tea.Model, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.String() {
		case "esc", "x":
			m.mode = m.modePrev
			return m, nil
		}
	}
	return m, nil
}
```

### Step 4: Run, verify passing

```
go test ./internal/tui/ -v
```

### Step 5: Commit

```bash
git add internal/tui/model.go internal/tui/update.go internal/tui/model_test.go
git commit -m "tui: add modeMixer and open/close transitions"
```

---

## Task 9: mixerModel struct

Holds `selectedChannel int`, exposes `Selected() string`, and is constructed from the audio mixer to know channel order. Pure UI state — actual volumes live on `audio.AmbientMixer`.

**Files:**
- Create: `internal/tui/mixer.go`
- Create: `internal/tui/mixer_test.go`

### Step 1: Failing test

```go
package tui

import (
	"testing"

	"github.com/iRootPro/lofi-player/internal/audio"
)

func TestMixerModelDefaultsToFirstChannel(t *testing.T) {
	mm := newMixerModel(audio.NewAmbientMixer())
	if got := mm.Selected(); got != "rain" {
		t.Errorf("Selected default: got %q, want %q", got, "rain")
	}
}
```

### Step 2: Run, verify failing → Step 3: Implementation

```go
package tui

import "github.com/iRootPro/lofi-player/internal/audio"

// mixerModel holds the modal's transient UI state. The single source of
// truth for volumes is audio.AmbientMixer; mixerModel only knows which
// channel the user has selected with j/k.
type mixerModel struct {
	mixer    *audio.AmbientMixer
	selected int
}

func newMixerModel(am *audio.AmbientMixer) mixerModel {
	return mixerModel{mixer: am}
}

func (m mixerModel) Selected() string {
	if m.mixer == nil {
		return ""
	}
	ids := m.mixer.ChannelIDs()
	if m.selected < 0 || m.selected >= len(ids) {
		return ""
	}
	return ids[m.selected]
}
```

### Step 4: Run, verify passing → Step 5: Commit

```bash
git add internal/tui/mixer.go internal/tui/mixer_test.go
git commit -m "tui: introduce mixerModel for modal state"
```

---

## Task 10: Mixer keybindings — navigation, volume adjust, 0/1 shortcuts

Wire j/k navigation, h/l ±5, H/L ±25, 0 → 0, 1 → 100. Each volume change calls `audio.AmbientMixer.SetVolume`.

**Files:**
- Modify: `internal/tui/mixer.go`
- Modify: `internal/tui/update.go` (replace stub `updateMixer`)
- Modify: `internal/tui/mixer_test.go`

### Step 1: Failing tests

```go
func TestMixerNavigationJK(t *testing.T) {
	mm := newMixerModel(audio.NewAmbientMixer())
	mm = mm.handle("j")
	if got := mm.Selected(); got != "fire" {
		t.Errorf("after j: %q, want fire", got)
	}
	mm = mm.handle("j")
	if got := mm.Selected(); got != "white_noise" {
		t.Errorf("after jj: %q", got)
	}
	mm = mm.handle("j") // clamp at last
	if got := mm.Selected(); got != "white_noise" {
		t.Errorf("after jjj clamp: %q", got)
	}
	mm = mm.handle("k")
	if got := mm.Selected(); got != "fire" {
		t.Errorf("after k: %q", got)
	}
}

func TestMixerVolumeAdjustHL(t *testing.T) {
	am := audio.NewAmbientMixer()
	_ = am.Init() // disabled channels are fine; SetVolume still records value
	mm := newMixerModel(am)
	mm = mm.handle("l")
	mm = mm.handle("l")
	if v := am.Volume("rain"); v != 10 {
		t.Errorf("after ll: %d, want 10", v)
	}
	mm = mm.handle("L")
	if v := am.Volume("rain"); v != 35 {
		t.Errorf("after L: %d, want 35", v)
	}
	mm = mm.handle("0")
	if v := am.Volume("rain"); v != 0 {
		t.Errorf("after 0: %d", v)
	}
	mm = mm.handle("1")
	if v := am.Volume("rain"); v != 100 {
		t.Errorf("after 1: %d", v)
	}
}
```

### Step 2: Run, verify failing → Step 3: Implementation

`internal/tui/mixer.go`:

```go
const (
	mixerStepFine   = 5
	mixerStepCoarse = 25
)

// handle applies a single key string to the mixer model. Returns the
// updated model. Volume changes are pushed straight to the audio
// mixer; the model itself doesn't cache them.
func (m mixerModel) handle(key string) mixerModel {
	if m.mixer == nil {
		return m
	}
	ids := m.mixer.ChannelIDs()
	switch key {
	case "j", "down":
		if m.selected < len(ids)-1 {
			m.selected++
		}
	case "k", "up":
		if m.selected > 0 {
			m.selected--
		}
	case "l", "right":
		m.adjust(mixerStepFine)
	case "h", "left":
		m.adjust(-mixerStepFine)
	case "L":
		m.adjust(mixerStepCoarse)
	case "H":
		m.adjust(-mixerStepCoarse)
	case "0":
		m.set(0)
	case "1":
		m.set(100)
	}
	return m
}

func (m mixerModel) adjust(delta int) {
	id := m.Selected()
	if id == "" {
		return
	}
	v := m.mixer.Volume(id) + delta
	_ = m.mixer.SetVolume(id, v)
}

func (m mixerModel) set(v int) {
	id := m.Selected()
	if id == "" {
		return
	}
	_ = m.mixer.SetVolume(id, v)
}
```

`update.go` — replace the Task 8 stub:

```go
func (m Model) updateMixer(msg tea.Msg) (tea.Model, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch km.String() {
	case "esc", "x":
		m.mode = m.modePrev
		return m, scheduleAmbientSave()  // see Task 13
	case "q", "ctrl+c":
		return m, tea.Quit
	case "?":
		m.showFullHelp = !m.showFullHelp
		return m, nil
	}
	m.mixerUI = m.mixerUI.handle(km.String())
	return m, scheduleAmbientSave()  // Task 13 wire-up; for now keep a noop cmd
}
```

For now, define `scheduleAmbientSave` as a placeholder returning `nil`; Task 13 fills it in.

### Step 4: Run, verify passing → Step 5: Commit

```bash
git add internal/tui/mixer.go internal/tui/update.go internal/tui/mixer_test.go
git commit -m "tui: ambient mixer keybindings (jk/hl/HL/0/1)"
```

---

## Task 11: Mixer view rendering

Render the modal as a centered card: title, three rows of `icon · label · bar · value`, hint bar. Selected channel highlighted in Primary; muted channels (volume == 0 and not selected) in Subtle; disabled channels show `unavailable`.

**Files:**
- Modify: `internal/tui/mixer.go` (add `view` method)
- Modify: `internal/tui/view.go` (route `modeMixer` to mixer.view)
- Modify: `internal/tui/mixer_test.go`

### Step 1: Failing test

Snapshot/substring style — keep tests resilient to small style tweaks.

```go
func TestMixerViewIncludesAllChannels(t *testing.T) {
	am := audio.NewAmbientMixer()
	_ = am.Init()
	_ = am.SetVolume("rain", 40)
	mm := newMixerModel(am)
	out := mm.view(80, NewStyles(theme.Default()), theme.Default())

	for _, want := range []string{"rain", "fire", "white noise", "ambient mixer"} {
		if !strings.Contains(out, want) {
			t.Errorf("view missing %q\n%s", want, out)
		}
	}
}

func TestMixerViewMarksDisabledChannels(t *testing.T) {
	am := audio.NewAmbientMixer()
	_ = am.Init()
	mm := newMixerModel(am)
	out := mm.view(80, NewStyles(theme.Default()), theme.Default())
	// On a CI without mpv every channel is disabled, so "unavailable" must
	// appear at least once.
	if !strings.Contains(out, "unavailable") {
		t.Errorf("expected 'unavailable' marker\n%s", out)
	}
}
```

(Use whatever the existing API for `theme.Default()` is — check `internal/theme/theme.go`. Most likely it's `theme.Lookup("tokyo-night")`.)

### Step 2: Run, verify failing → Step 3: Implementation

`mixer.go`:

```go
import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/iRootPro/lofi-player/internal/theme"
)

const mixerBarWidth = 14

func (m mixerModel) view(width int, styles Styles, t theme.Theme) string {
	if m.mixer == nil {
		return ""
	}
	var inner strings.Builder
	inner.WriteString(styles.SectionHeader.Render("─── ambient mixer ───"))
	inner.WriteString("\n\n")

	ids := m.mixer.ChannelIDs()
	for i, id := range ids {
		ch, _ := m.mixer.Channel(id)
		v := m.mixer.Volume(id)
		disabled := m.mixer.Disabled(id)
		selected := i == m.selected

		row := m.renderRow(ch, v, disabled, selected, styles)
		inner.WriteString(row)
		if i < len(ids)-1 {
			inner.WriteString("\n\n")
		}
	}
	inner.WriteString("\n\n")
	inner.WriteString(m.renderHint(styles))

	card := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Muted).
		Padding(1, 3).
		Render(inner.String())

	return lipgloss.PlaceHorizontal(width, lipgloss.Center, card)
}

func (m mixerModel) renderRow(ch audio.AmbientChannel, v int, disabled, selected bool, styles Styles) string {
	cursor := "  "
	if selected {
		cursor = styles.Cursor.Render("> ")
	}
	label := fmt.Sprintf("%-12s", ch.Label)

	if disabled {
		return cursor + ch.Icon + "  " + styles.Hint.Render(label) + styles.Hint.Render("unavailable")
	}

	fill := v * mixerBarWidth / 100
	bar := styles.VolFill.Render(strings.Repeat("▰", fill)) +
		styles.VolEmpty.Render(strings.Repeat("▱", mixerBarWidth-fill))

	value := fmt.Sprintf("%3d", v)
	switch {
	case selected:
		return cursor + ch.Icon + "  " + styles.Cursor.Render(label) + bar + "  " + styles.Cursor.Render(value)
	case v == 0:
		return cursor + ch.Icon + "  " + styles.Hint.Render(label) + bar + "  " + styles.Hint.Render(value)
	default:
		return cursor + ch.Icon + "  " + styles.StationItem.Render(label) + bar + "  " + styles.StationItem.Render(value)
	}
}

func (m mixerModel) renderHint(styles Styles) string {
	pair := func(k, d string) string {
		return styles.HelpKey.Render(k) + " " + styles.HelpDesc.Render(d)
	}
	sep := "  " + styles.HelpSep.Render("·") + "  "
	return pair("j/k", "select") + sep +
		pair("h/l", "±5") + sep +
		pair("0", "mute") + sep +
		pair("x", "close")
}
```

`view.go` — add a route in `View()`'s switch:

```go
case modeMixer:
	content = inner.viewMixer()
```

And the helper:

```go
func (m Model) viewMixer() string {
	// Render the previous layout as the backdrop so the user keeps
	// visual context (matches addform pattern).
	var backdrop string
	if m.modePrev == modeMini {
		backdrop = m.viewMini()
	} else {
		backdrop = m.viewFull()
	}
	card := m.mixerUI.view(m.width, m.styles, m.theme)
	return backdrop + "\n\n" + card
}
```

### Step 4: Run, verify passing → Step 5: Commit

```bash
git add internal/tui/mixer.go internal/tui/view.go internal/tui/mixer_test.go
git commit -m "tui: render ambient mixer modal card"
```

---

## Task 12: Confirm global keys are inert in mixer modal

The Task 8 dispatch already routes everything in `modeMixer` through `updateMixer` (which only handles a tight allowlist), so global bindings are inherently dead. Add a regression test so future refactors don't break this.

**Files:**
- Modify: `internal/tui/model_test.go`

### Step 1: Failing test

```go
func TestGlobalKeysDisabledInMixerModal(t *testing.T) {
	m := newTestModel(t)
	m, _ = updateKey(m, "x").(Model), nil // open mixer
	originalVolume := m.volume
	m, _ = updateKey(m, "+").(Model), nil // would normally bump volume
	if m.volume != originalVolume {
		t.Errorf("global volume key fired in modal: %d → %d", originalVolume, m.volume)
	}
	originalTheme := m.theme.Name
	m, _ = updateKey(m, "t").(Model), nil
	if m.theme.Name != originalTheme {
		t.Errorf("theme key fired in modal: %s → %s", originalTheme, m.theme.Name)
	}
}
```

### Step 2: Run

If the test passes immediately (it should — the dispatch is already in place), commit. If it fails, adjust `updateMixer` to ignore unknown keys.

### Step 3: Commit

```bash
git add internal/tui/model_test.go
git commit -m "tui: regression test for inert global keys in mixer modal"
```

---

## Task 13: Debounced state save on volume change

Adding a 500ms debounce: after a volume change, schedule a `tea.Tick`; if a new volume change arrives before the tick fires, replace it with a fresh tick. When the tick finally fires, call a save callback.

To keep this simple in a value-receiver Bubble Tea model, store a monotonic `ambientSaveSeq int` on the model. Each volume change bumps it and schedules a `tea.Tick(500ms)` carrying that seq. When the tick message arrives, if the carried seq still matches `m.ambientSaveSeq`, save; otherwise drop.

**Files:**
- Modify: `internal/tui/model.go` (add `ambientSaveSeq` field, `AmbientSnapshot` method)
- Modify: `internal/tui/messages.go` (new tick message type)
- Modify: `internal/tui/commands.go` (new tick command)
- Modify: `internal/tui/update.go` (handle tick message; replace `scheduleAmbientSave` placeholder)
- Modify: `main.go` (wire save callback through Options or direct field)

### Step 1: Decide save plumbing

`state.Save` is package-level. The model can build an `*state.State` from its own fields plus `m.mixer.Volumes()`, but it doesn't know `LastStationName` constants or wherever else state lives. Two choices:

- **A.** Inject a save callback into `Model`: `saveAmbient func(map[string]int)`. main.go sets it to `func(v map[string]int) { state.Save(snapshot.With(Ambient: v)) }`.
- **B.** Have the model expose `AmbientSnapshot() map[string]int`; main.go listens to a custom `tea.Msg` produced by a channel. Heavier.

Pick **A** — minimal surface.

Actually simplest: the model already exposes `ThemeName()`, `Volume()`, `LastStationName()` — main.go can read `Volumes()` from the mixer at any time. So the save can happen on-tick by calling a callback that builds the full state and calls `state.Save`. We don't need to thread anything new — just the callback.

### Step 2: Failing test

```go
func TestAmbientSaveDebounceTick(t *testing.T) {
	saved := 0
	m := newTestModel(t)
	m.saveAmbient = func() { saved++ }

	// Two volume changes within debounce.
	m, _ = updateKey(m, "x").(Model), nil
	m, _ = updateKey(m, "l").(Model), nil
	m, _ = updateKey(m, "l").(Model), nil

	// Simulate the first scheduled tick firing — its seq is stale.
	m1, _ := m.Update(ambientSaveTickMsg{seq: 1})
	if saved != 0 {
		t.Errorf("stale tick fired save: %d", saved)
	}
	// Latest tick (seq matches m.ambientSaveSeq).
	m = m1.(Model)
	m, _ = m.Update(ambientSaveTickMsg{seq: m.ambientSaveSeq})
	if saved != 1 {
		t.Errorf("save count after fresh tick: %d, want 1", saved)
	}
}
```

### Step 3: Implementation

`messages.go`:

```go
type ambientSaveTickMsg struct{ seq int }
```

`commands.go`:

```go
func ambientSaveTick(seq int) tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(time.Time) tea.Msg {
		return ambientSaveTickMsg{seq: seq}
	})
}
```

`model.go`:

```go
type Model struct {
	// ... existing ...
	ambientSaveSeq int
	saveAmbient    func()
}
```

`update.go`:

```go
// Replace placeholder scheduleAmbientSave with the real one:
func (m Model) scheduleAmbientSave() (Model, tea.Cmd) {
	m.ambientSaveSeq++
	return m, ambientSaveTick(m.ambientSaveSeq)
}

// Handle in updateMixer's volume-change branches AND on close.
// Then in the top-level Update, add the message case:
case ambientSaveTickMsg:
	if msg.seq == m.ambientSaveSeq && m.saveAmbient != nil {
		m.saveAmbient()
	}
	return m, nil
```

Refactor `updateMixer` to call `scheduleAmbientSave` on each volume change and on close:

```go
func (m Model) updateMixer(msg tea.Msg) (tea.Model, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch km.String() {
	case "esc", "x":
		m.mode = m.modePrev
		nm, cmd := m.scheduleAmbientSave()
		return nm, cmd
	case "q", "ctrl+c":
		return m, tea.Quit
	case "?":
		m.showFullHelp = !m.showFullHelp
		return m, nil
	}
	m.mixerUI = m.mixerUI.handle(km.String())
	nm, cmd := m.scheduleAmbientSave()
	return nm, cmd
}
```

### Step 4: Run, verify passing → Step 5: Commit

```bash
git add internal/tui/model.go internal/tui/messages.go internal/tui/commands.go internal/tui/update.go internal/tui/model_test.go
git commit -m "tui: debounce ambient state save on volume change"
```

---

## Task 14: Active-channel indicator on the main view

Render `· 🌧️🔥` (Primary color) right after the station kind tag in `renderNowPlaying`. Pull from `m.mixer.ActiveIDs()` to know order and presence.

**Files:**
- Modify: `internal/tui/view.go`
- Modify: `internal/tui/view_test.go` (if exists; otherwise extend `model_test.go`)

### Step 1: Failing test

```go
func TestStationLineShowsActiveAmbient(t *testing.T) {
	m := newTestModel(t)
	_ = m.mixer.SetVolume("rain", 40)
	m.playingIdx = 0

	out := m.renderNowPlaying()
	if !strings.Contains(out, "🌧️") {
		t.Errorf("expected rain icon in station line:\n%s", out)
	}
}

func TestStationLineHidesIndicatorWhenSilent(t *testing.T) {
	m := newTestModel(t)
	m.playingIdx = 0
	out := m.renderNowPlaying()
	if strings.Contains(out, "🌧️") || strings.Contains(out, "🔥") || strings.Contains(out, "⚪") {
		t.Errorf("ambient icons leaked to silent station line:\n%s", out)
	}
}
```

### Step 2: Run, verify failing → Step 3: Implementation

In `view.go`, append to `renderNowPlaying` after the existing kind icon:

```go
if indicator := m.renderAmbientIndicator(); indicator != "" {
	stationLine += "  " + indicator
}
```

Add helper:

```go
func (m Model) renderAmbientIndicator() string {
	if m.mixer == nil {
		return ""
	}
	ids := m.mixer.ActiveIDs()
	if len(ids) == 0 {
		return ""
	}
	var icons []string
	for _, id := range ids {
		ch, ok := m.mixer.Channel(id)
		if !ok {
			continue
		}
		icons = append(icons, ch.Icon)
	}
	return m.styles.SectionHeader.Render("· ") +
		m.styles.AppTitle.Render(strings.Join(icons, ""))
}
```

### Step 4: Run, verify passing → Step 5: Commit

```bash
git add internal/tui/view.go internal/tui/model_test.go
git commit -m "tui: show active ambient channels next to station"
```

---

## Task 15: main.go wire-up

Construct the mixer at startup, restore volumes from `state.Ambient`, pass it into `tui.NewModel`, register the save callback, and `Close()` on shutdown.

**Files:**
- Modify: `main.go`

### Step 1: No new tests — manual run is the validation here.

### Step 2: Implementation

```go
// In run(), after `defer player.Close()`:
mixer := audio.NewAmbientMixer()
if err := mixer.Init(); err != nil {
	fmt.Fprintf(os.Stderr, "lofi-player: ambient mixer init: %v\n", err)
	// Continue running without ambient — feature degrades gracefully.
}
defer mixer.Close()

// Restore persisted volumes.
for id, v := range st.Ambient {
	_ = mixer.SetVolume(id, v)
}

// (existing) p := tea.NewProgram(tui.NewModel(cfg, player, mixer, opts), ...)

// Register save callback. The model already calls it on debounce.
modelWithSave := tui.NewModel(cfg, player, mixer, opts)
modelWithSave = modelWithSave.WithSaveAmbient(func() {
	next := &state.State{
		Theme:           modelWithSave.ThemeName(),
		Volume:          modelWithSave.Volume(),
		LastStationName: modelWithSave.LastStationName(),
		Ambient:         mixer.Volumes(),
	}
	_ = state.Save(next)
})
p := tea.NewProgram(modelWithSave, tea.WithAltScreen())
```

Wait — `Model` is value-receiver and `tea.NewProgram` consumes it. `WithSaveAmbient` returning a new value works, but the closure captures `modelWithSave` which is then handed to `NewProgram`. That's fine because Bubble Tea internally clones models on every Update; the closure just needs to read snapshots from the *original* — which is wrong, since the live model is inside Bubble Tea.

**Better approach:** the callback should read from `mixer` (live state) and the *fields* needed for non-ambient state come from the *final* model returned by `p.Run()` — but on debounce we save before exit. We need access to the live model.

Simpler: store a pointer to the latest model snapshot via a goroutine-safe atomic. Or have the model emit a save message containing the ambient map, and main.go writes the file.

Cleanest with the existing patterns: change `saveAmbient` to take the snapshot as argument:

```go
saveAmbient    func(map[string]int)
```

And the tick handler reads from `m.mixer.Volumes()` and passes it in. main.go composes the full state from constants captured at startup plus the live ambient map. But `Theme` and `Volume` are not constants — they change at runtime.

OK, the pragmatic fix: on debounce, save *only* the ambient slice and delegate the merge to the state package. Or simplest: write a partial save that loads-modify-saves. Since the file is small:

```go
saveAmbient: func(volumes map[string]int) {
	current := state.Load()
	current.Ambient = volumes
	_ = state.Save(current)
},
```

And in the tick handler:

```go
case ambientSaveTickMsg:
	if msg.seq == m.ambientSaveSeq && m.saveAmbient != nil && m.mixer != nil {
		m.saveAmbient(m.mixer.Volumes())
	}
	return m, nil
```

Update `Model.saveAmbient` field to `func(map[string]int)`, and the test in Task 13 to match.

Also: at clean shutdown the existing block at the bottom of `run()` writes `state.Save(next)` with theme/volume/last_station — append `Ambient: mixer.Volumes()` to that. So the final on-quit save captures everything; the debounce only catches mid-session loss on `kill -9`.

Add `WithSaveAmbient` — actually skip the helper, just expose the field via NewModel option. Modify `tui.Options`:

```go
type Options struct {
	Theme           string
	Volume          int
	AutoplayStation int
	SaveAmbient     func(map[string]int)
}
```

`NewModel` plugs `opts.SaveAmbient` into `m.saveAmbient`.

### Step 3: Apply changes

(Adjust Task 13 test to match new signature; revisit if needed.)

In `run()` final block:

```go
if m, ok := finalModel.(tui.Model); ok {
	next := &state.State{
		Theme:           m.ThemeName(),
		Volume:          m.Volume(),
		LastStationName: m.LastStationName(),
		Ambient:         mixer.Volumes(),
	}
	if err := state.Save(next); err != nil {
		fmt.Fprintf(os.Stderr, "lofi-player: state save failed: %v\n", err)
	}
}
```

### Step 4: Run baseline + manual

```
go test ./... -v
go build ./...
./lofi-player
# press x — modal opens with three rows
# press l a few times — bar moves; rain icon appears next to station
# press esc — back to main; rain icon stays
# quit, restart — rain still set, icon still there
```

### Step 5: Commit

```bash
git add main.go internal/tui/model.go internal/tui/update.go internal/tui/model_test.go
git commit -m "main: wire ambient mixer into runtime and persistence"
```

---

## Task 16: Replace placeholders with real loop files

This task is *separate* from code work and can happen on the same branch or after merge.

**Files:**
- Modify: `internal/audio/ambient_assets/rain.opus`
- Modify: `internal/audio/ambient_assets/fire.opus`
- Modify: `internal/audio/ambient_assets/white_noise.opus`
- Modify: `internal/audio/ambient_assets/README.md`
- Maybe create: `ATTRIBUTIONS.md` (if any file is CC-BY)

### Steps

1. Source candidates from freesound.org (CC0 priority).
2. Pick best-fitting 3–5 minute segment for each channel.
3. ffmpeg pipeline per file:
   ```
   ffmpeg -i source.wav \
     -af "afade=t=in:st=0:d=2,afade=t=out:st=180:d=3,loudnorm=I=-23:LRA=7" \
     -c:a libopus -b:a 64k -ac 2 \
     internal/audio/ambient_assets/rain.opus
   ```
4. Listen to the loop seam. If audible, adjust fade timings. Audacity crossfade is easier for tricky cases.
5. Verify total bundle size: `ls -lh internal/audio/ambient_assets/*.opus` — target <12 MB combined.
6. Update README with source links, authors, licenses.
7. Run app manually; verify each channel sounds right at 30 / 60 / 100 volume.
8. Commit:
   ```bash
   git add internal/audio/ambient_assets/
   git commit -m "audio: replace placeholder ambient loops with real CC0 audio"
   ```

**This is a content task — block on it before tagging v0.4.0, but code can ship without.**

---

## Task 17: Manual end-to-end verification

Before opening the PR / merging:

1. `go test ./...` — all green.
2. `go vet ./...` — no warnings.
3. `go build ./...` — clean build.
4. `./lofi-player` (with mpv on PATH):
   - Press `x` → modal opens, three rows visible.
   - `j`/`k` move selection; bar of selected row recolors.
   - `l` × 8 → rain at 40, audible (if real loop files exist).
   - Indicator on main view shows 🌧️ rain icon next to station.
   - `0` → mute; indicator hides.
   - `1` → max; loud rain.
   - `esc` → back to main.
   - Quit (`q`) and reopen — last volumes restored.
5. `./lofi-player` without mpv on PATH (rename `mpv` binary briefly):
   - App fails fast with the existing mpv-not-found error from `run()` (this happens before mixer init, so mixer never tries to spawn).
6. Edge case: corrupt cache file:
   - `echo garbage > ~/.cache/lofi-player/ambient/rain.opus`
   - Restart — file rehydrated from embed; rain works again.

If anything fails, file an issue at the relevant Task and fix before merging.

---

## After this plan

When all tasks are complete:

1. **REQUIRED SUB-SKILL:** Use `superpowers:finishing-a-development-branch` to decide on merge/PR/cleanup.
2. Tag the design as the source of truth for any post-v1 follow-ups (named presets, custom paths, more channels).
