package audio

import (
	"bytes"
	"context"
	"crypto/sha256"
	"embed"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

//go:embed ambient_assets/*.opus
var ambientFS embed.FS

// AmbientChannel is the static metadata for one ambient channel.
type AmbientChannel struct {
	ID    string
	Label string
	Icon  string
	asset string
}

var ambientChannels = []AmbientChannel{
	{ID: "rain", Label: "rain", Icon: "🌧️", asset: "rain.opus"},
	{ID: "fire", Label: "fire", Icon: "🔥", asset: "fire.opus"},
	{ID: "white_noise", Label: "white noise", Icon: "⚪", asset: "white_noise.opus"},
}

type runtimeChannel struct {
	meta     AmbientChannel
	volume   int
	filePath string
	player   *ambientPlayer
	disabled bool
}

var errUnknownChannel = errors.New("unknown ambient channel")

type AmbientMixer struct {
	channels []AmbientChannel
	runtime  []*runtimeChannel
	closed   bool
	mu       sync.Mutex
}

// cacheDirFn lets tests redirect the cache root because os.UserCacheDir
// ignores XDG_CACHE_HOME on darwin and would otherwise touch the real
// user cache.
var cacheDirFn = defaultCacheDir

func defaultCacheDir() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "lofi-player", "ambient"), nil
}

func NewAmbientMixer() *AmbientMixer {
	return &AmbientMixer{channels: append([]AmbientChannel(nil), ambientChannels...)}
}

func (m *AmbientMixer) ChannelIDs() []string {
	ids := make([]string, len(m.channels))
	for i, c := range m.channels {
		ids[i] = c.ID
	}
	return ids
}

func (m *AmbientMixer) Channel(id string) (AmbientChannel, bool) {
	for _, c := range m.channels {
		if c.ID == id {
			return c, true
		}
	}
	return AmbientChannel{}, false
}

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
			m.closeSpawned()
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

// closeSpawned tears down any mpv subprocesses already started by Init.
// Used on Init's own failure path so the caller never has to choose
// between calling Close on a half-built mixer and leaking subprocesses.
func (m *AmbientMixer) closeSpawned() {
	for _, rc := range m.runtime {
		if rc != nil && rc.player != nil {
			rc.player.close()
		}
	}
	m.runtime = nil
}

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

func (m *AmbientMixer) Disabled(id string) bool {
	for _, rc := range m.runtime {
		if rc != nil && rc.meta.ID == id {
			return rc.disabled
		}
	}
	return false
}

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

func (m *AmbientMixer) Volume(id string) int {
	if rc := m.find(id); rc != nil {
		return rc.volume
	}
	return 0
}

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

func (m *AmbientMixer) ActiveIDs() []string {
	var out []string
	for _, rc := range m.runtime {
		if rc != nil && rc.volume > 0 {
			out = append(out, rc.meta.ID)
		}
	}
	return out
}

func (m *AmbientMixer) find(id string) *runtimeChannel {
	for _, rc := range m.runtime {
		if rc != nil && rc.meta.ID == id {
			return rc
		}
	}
	return nil
}

func (m *AmbientMixer) cacheDir() (string, error) {
	return cacheDirFn()
}

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
		return "", fmt.Errorf("read existing %s: %w", target, err)
	}

	tmp, err := os.CreateTemp(dir, ".ambient-*.opus")
	if err != nil {
		return "", fmt.Errorf("create temp in %s: %w", dir, err)
	}
	tmpName := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpName) }
	if _, err := io.Copy(tmp, bytes.NewReader(want)); err != nil {
		_ = tmp.Close()
		cleanup()
		return "", fmt.Errorf("copy %s: %w", c.ID, err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		cleanup()
		return "", fmt.Errorf("sync %s: %w", c.ID, err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return "", fmt.Errorf("close temp: %w", err)
	}
	if err := os.Rename(tmpName, target); err != nil {
		cleanup()
		return "", fmt.Errorf("rename %s: %w", target, err)
	}
	return target, nil
}
