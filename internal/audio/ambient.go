package audio

import (
	"bytes"
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
	filePath string
}

type AmbientMixer struct {
	channels []AmbientChannel
	runtime  []*runtimeChannel
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
			return fmt.Errorf("materialize %s: %w", c.ID, err)
		}
		m.runtime[i] = &runtimeChannel{meta: c, filePath: path}
	}
	return nil
}

func (m *AmbientMixer) Close() error { return nil }

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
		return "", err
	}

	tmp, err := os.CreateTemp(dir, ".ambient-*.opus")
	if err != nil {
		return "", err
	}
	tmpName := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpName) }
	if _, err := io.Copy(tmp, bytes.NewReader(want)); err != nil {
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
