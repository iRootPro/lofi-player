package audio

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Property IDs handed to mpv's observe_property. The numeric values are
// internal — mpv echoes them back on each property-change event so the
// translator can route by id without parsing names.
const (
	propIDPause      = 1
	propIDVolume     = 2
	propIDMetadata   = 3
	propIDMediaTitle = 4
	propIDIdleActive = 5
)

// Event is the sealed interface implemented by every value emitted on
// (*Player).Events. Use a type switch to handle each variant.
type Event interface{ isEvent() }

// MetadataChanged is delivered whenever ICY (or media-title) metadata
// resolves to a track that differs from the previous one. Title or
// Artist (but not both) may be empty when the stream omits the field.
type MetadataChanged struct {
	Title  string
	Artist string
}

// PlaybackStarted fires when mpv transitions out of the paused state —
// after a fresh loadfile finishes buffering, or after an explicit Resume.
type PlaybackStarted struct{}

// PlaybackPaused fires when mpv enters the paused state.
type PlaybackPaused struct{}

// PlaybackError is delivered when mpv signals end-file with an error
// reason (network drop, 404, decoder failure). The TUI surfaces this as
// a transient toast (plan §6 Phase 1).
type PlaybackError struct {
	Err error
}

// EOF is delivered when mpv signals end-file with reason=eof. Live
// streams normally don't reach EOF except on graceful server shutdown.
type EOF struct{}

func (MetadataChanged) isEvent() {}
func (PlaybackStarted) isEvent() {}
func (PlaybackPaused) isEvent() {}
func (PlaybackError) isEvent()  {}
func (EOF) isEvent()            {}

// Options controls Player startup.
type Options struct {
	// MPVPath overrides the default "mpv" lookup on $PATH.
	MPVPath string
	// InitialVolume is the volume (0..100) applied right after the IPC
	// handshake. Values outside the range are clamped.
	InitialVolume int
}

// Player owns an mpv subprocess and a JSON-IPC connection to it,
// translating low-level mpv events into typed Event values.
//
// All public methods are safe to call concurrently. The Events channel
// is closed exactly once, when (*Player).Close is invoked or the mpv
// process exits unexpectedly.
type Player struct {
	cmd       *exec.Cmd
	ipc       *ipcClient
	socketDir string
	events    chan Event

	closeOnce sync.Once
	closeErr  error

	mu         sync.Mutex
	lastTitle  string
	lastArtist string
}

// NewPlayer spawns mpv in idle mode and establishes a JSON-IPC connection
// to it. The returned Player is ready to accept Play/Pause/Resume calls.
//
// ctx bounds the startup handshake (socket appearance + observe_property
// calls). Cancelling it after NewPlayer returns has no effect on
// subsequent operations — those use their own short timeouts.
func NewPlayer(ctx context.Context, opts Options) (*Player, error) {
	mpvPath := opts.MPVPath
	if mpvPath == "" {
		mpvPath = "mpv"
	}

	// Use a short tempdir to keep the socket path under macOS's
	// sockaddr_un limit (~104 bytes). /tmp is shorter than the default
	// /var/folders/.../T tempdir on macOS.
	socketDir, err := os.MkdirTemp("/tmp", "lofi-player-*")
	if err != nil {
		// Fall back to the default tempdir if /tmp isn't writable.
		socketDir, err = os.MkdirTemp("", "lofi-player-*")
		if err != nil {
			return nil, fmt.Errorf("create socket dir: %w", err)
		}
	}
	socketPath := filepath.Join(socketDir, "mpv.sock")

	var stderr bytes.Buffer
	cmd := exec.Command(mpvPath,
		"--idle=yes",
		"--no-video",
		"--no-terminal",
		"--really-quiet",
		"--input-ipc-server="+socketPath,
	)
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		os.RemoveAll(socketDir)
		return nil, fmt.Errorf("start mpv: %w", err)
	}

	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()

	if err := waitForSocketOrExit(ctx, socketPath, 5*time.Second, exited); err != nil {
		// If mpv is still alive, terminate it; otherwise it already exited.
		select {
		case <-exited:
		default:
			_ = cmd.Process.Kill()
			<-exited
		}
		os.RemoveAll(socketDir)
		stderrSnippet := strings.TrimSpace(stderr.String())
		if stderrSnippet != "" {
			return nil, fmt.Errorf("mpv did not open IPC socket: %w; mpv stderr: %s", err, stderrSnippet)
		}
		return nil, fmt.Errorf("mpv did not open IPC socket: %w", err)
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

	p := &Player{
		cmd:       cmd,
		ipc:       ipc,
		socketDir: socketDir,
		events:    make(chan Event, 32),
	}

	properties := []struct {
		id   int
		name string
	}{
		{propIDPause, "pause"},
		{propIDVolume, "volume"},
		{propIDMetadata, "metadata"},
		{propIDMediaTitle, "media-title"},
		{propIDIdleActive, "idle-active"},
	}
	for _, prop := range properties {
		if err := ipc.observe(ctx, prop.id, prop.name); err != nil {
			_ = p.Close()
			return nil, fmt.Errorf("observe %s: %w", prop.name, err)
		}
	}

	if _, err := ipc.command(ctx, "set_property", "volume", clampVolume(opts.InitialVolume)); err != nil {
		_ = p.Close()
		return nil, fmt.Errorf("set initial volume: %w", err)
	}

	go p.translate()
	return p, nil
}

// Play loads url and starts (or resumes) playback. Any currently-playing
// stream is replaced. Metadata state is reset so the next ICY update
// emits a fresh MetadataChanged.
func (p *Player) Play(url string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	p.mu.Lock()
	p.lastTitle = ""
	p.lastArtist = ""
	p.mu.Unlock()

	if _, err := p.ipc.command(ctx, "loadfile", url, "replace"); err != nil {
		return fmt.Errorf("loadfile: %w", err)
	}
	if _, err := p.ipc.command(ctx, "set_property", "pause", false); err != nil {
		return fmt.Errorf("unpause after loadfile: %w", err)
	}
	return nil
}

// Pause halts playback. The stream stays loaded so Resume picks up
// without re-buffering (where the protocol allows).
func (p *Player) Pause() error {
	return p.setProperty("pause", true)
}

// Resume reverses a previous Pause.
func (p *Player) Resume() error {
	return p.setProperty("pause", false)
}

// SetVolume adjusts playback volume. percent is clamped to 0..100.
func (p *Player) SetVolume(percent int) error {
	return p.setProperty("volume", clampVolume(percent))
}

func (p *Player) setProperty(name string, value any) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := p.ipc.command(ctx, "set_property", name, value)
	if err != nil {
		return fmt.Errorf("set %s: %w", name, err)
	}
	return nil
}

// Events returns the receive-only channel of typed playback events.
// The channel is closed when the Player shuts down.
func (p *Player) Events() <-chan Event {
	return p.events
}

// Close shuts down the mpv subprocess and releases all resources. It is
// safe to call concurrently and more than once; only the first call has
// effect. Close blocks for at most ~2.5s while waiting for mpv to exit
// gracefully before falling back to SIGKILL.
func (p *Player) Close() error {
	p.closeOnce.Do(func() {
		if p.ipc != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
			_, _ = p.ipc.command(ctx, "quit")
			cancel()
			_ = p.ipc.close()
		}
		if p.cmd != nil && p.cmd.Process != nil {
			done := make(chan struct{})
			go func() {
				_ = p.cmd.Wait()
				close(done)
			}()
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
	return p.closeErr
}

func (p *Player) translate() {
	defer close(p.events)
	for raw := range p.ipc.Events() {
		evt := p.translateOne(raw)
		if evt == nil {
			continue
		}
		p.events <- evt
	}
}

func (p *Player) translateOne(raw ipcEvent) Event {
	switch raw.Event {
	case "property-change":
		return p.translatePropertyChange(raw)
	case "end-file":
		switch raw.Reason {
		case "eof":
			return EOF{}
		case "error":
			return PlaybackError{Err: errors.New("playback ended with error")}
		}
	}
	return nil
}

func (p *Player) translatePropertyChange(raw ipcEvent) Event {
	switch raw.Name {
	case "pause":
		var paused bool
		if err := json.Unmarshal(raw.Data, &paused); err == nil {
			if paused {
				return PlaybackPaused{}
			}
			return PlaybackStarted{}
		}
	case "metadata":
		var meta map[string]string
		if err := json.Unmarshal(raw.Data, &meta); err == nil && len(meta) > 0 {
			return p.maybeMetadata(ParseMetadata(meta))
		}
	case "media-title":
		// media-title is the fallback when no ICY metadata is present.
		// Only treat it as a title if the metadata channel hasn't already
		// produced an artist (which would mean we have richer info), and
		// the value isn't an obvious URL fragment — mpv/ytdl_hook briefly
		// reports the URL tail as media-title before the real video title
		// resolves, and "watch?v=jfKfPfyJRdk" is not what we want to show.
		var title string
		if err := json.Unmarshal(raw.Data, &title); err == nil && title != "" && !looksLikeURLFragment(title) {
			p.mu.Lock()
			defer p.mu.Unlock()
			if p.lastArtist == "" && title != p.lastTitle {
				p.lastTitle = title
				return MetadataChanged{Title: title}
			}
		}
	}
	return nil
}

// looksLikeURLFragment returns true for strings that look like raw
// URLs or query fragments rather than human-readable titles. The TUI
// keeps showing its "…" placeholder while these surface, instead of
// flashing them at the user.
func looksLikeURLFragment(s string) bool {
	if strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") {
		return true
	}
	if strings.Contains(s, "://") {
		return true
	}
	// "watch?v=..." (YouTube), "v?id=..." and similar bare query forms.
	if strings.Contains(s, "?v=") || strings.Contains(s, "?id=") {
		return true
	}
	return false
}

func (p *Player) maybeMetadata(title, artist string) Event {
	if title == "" && artist == "" {
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if title == p.lastTitle && artist == p.lastArtist {
		return nil
	}
	p.lastTitle = title
	p.lastArtist = artist
	return MetadataChanged{Title: title, Artist: artist}
}

func clampVolume(v int) int {
	switch {
	case v < 0:
		return 0
	case v > 100:
		return 100
	default:
		return v
	}
}

// waitForSocketOrExit polls for the socket file to appear, returning
// early if mpv exits beforehand (in which case the error wraps mpv's
// exit status — usually a nil exit means "exited cleanly without error
// but never opened the socket", which still counts as failure here).
func waitForSocketOrExit(ctx context.Context, path string, timeout time.Duration, exited <-chan error) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return nil
		}
		select {
		case waitErr := <-exited:
			if waitErr != nil {
				return fmt.Errorf("mpv exited prematurely: %w", waitErr)
			}
			return errors.New("mpv exited prematurely with status 0")
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(20 * time.Millisecond):
		}
	}
	return errors.New("timeout waiting for socket file")
}
