// Package config loads and persists the user's YAML configuration.
//
// Config files live at $XDG_CONFIG_HOME/lofi-player/config.yaml — typically
// ~/.config/lofi-player/config.yaml on both Linux and macOS, since adrg/xdg
// applies XDG conventions on macOS too. On a fresh install the file does
// not exist; Load writes Defaults() into it, then returns those defaults.
//
// Subsequent reads pre-fill a Config with Defaults() and unmarshal the
// file on top, so missing keys keep their default values while present
// keys (including an explicit empty stations list) are honored as-is.
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/adrg/xdg"
	"gopkg.in/yaml.v3"
)

// Config is the deserialized user configuration.
type Config struct {
	// Theme names a built-in palette (see internal/theme).
	Theme string `yaml:"theme"`
	// Volume is the initial playback volume, 0–100.
	Volume int `yaml:"volume"`
	// Stations is the user's list of internet-radio stations.
	Stations []Station `yaml:"stations"`
	// Pomodoro tunes the focus-timer in internal/pomodoro.
	Pomodoro PomodoroConfig `yaml:"pomodoro"`
}

// Station is a single internet-radio entry.
type Station struct {
	// Name is the display name shown in the station list.
	Name string `yaml:"name"`
	// URL is the stream URL passed to the audio engine.
	URL string `yaml:"url"`
}

// PomodoroConfig mirrors the YAML keys described in plan §6 Phase 3.
type PomodoroConfig struct {
	FocusMinutes         int       `yaml:"focus_minutes"`
	ShortBreakMinutes    int       `yaml:"short_break_minutes"`
	LongBreakMinutes     int       `yaml:"long_break_minutes"`
	RoundsUntilLongBreak int       `yaml:"rounds_until_long_break"`
	AutoPauseOnBreak     bool      `yaml:"auto_pause_on_break"`
	AutoResumeOnFocus    bool      `yaml:"auto_resume_on_focus"`
	BreakStations        []Station `yaml:"break_stations"`
}

// defaultPomodoro returns the canonical pomodoro configuration.
func defaultPomodoro() PomodoroConfig {
	return PomodoroConfig{
		FocusMinutes:         25,
		ShortBreakMinutes:    5,
		LongBreakMinutes:     15,
		RoundsUntilLongBreak: 4,
		AutoPauseOnBreak:     true,
		AutoResumeOnFocus:    true,
	}
}

// Defaults returns the canonical default configuration written to disk on
// first run. The station list comes from §6 Phase 0 of the project plan
// and was chosen for stability across networks and metadata correctness.
func Defaults() Config {
	return Config{
		Theme:  "tokyo-night",
		Volume: 60,
		Stations: []Station{
			{Name: "SomaFM Groove Salad", URL: "https://ice1.somafm.com/groovesalad-256-mp3"},
			{Name: "SomaFM Drone Zone", URL: "https://ice1.somafm.com/dronezone-256-mp3"},
			{Name: "SomaFM Deep Space One", URL: "https://ice1.somafm.com/deepspaceone-128-mp3"},
			{Name: "Radio Paradise Mellow", URL: "https://stream.radioparadise.com/mellow-128"},
		},
		Pomodoro: defaultPomodoro(),
	}
}

// Path returns the canonical XDG path to the config file, creating any
// missing parent directories along the way.
func Path() (string, error) {
	p, err := xdg.ConfigFile("lofi-player/config.yaml")
	if err != nil {
		return "", fmt.Errorf("resolving XDG config path: %w", err)
	}
	return p, nil
}

// Load reads the user's configuration from the canonical XDG path.
// On first run the file is created with Defaults() and those defaults
// are returned. See package documentation for the merge semantics.
func Load() (*Config, error) {
	p, err := Path()
	if err != nil {
		return nil, err
	}
	return loadFromFile(p)
}

// Save writes cfg to the canonical XDG path.
func Save(cfg *Config) error {
	p, err := Path()
	if err != nil {
		return err
	}
	return saveToFile(p, cfg)
}

func loadFromFile(path string) (*Config, error) {
	cfg := Defaults()
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		if err := saveToFile(path, &cfg); err != nil {
			return nil, fmt.Errorf("writing default config to %q: %w", path, err)
		}
		return &cfg, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading %q: %w", path, err)
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing %q: %w", path, err)
	}
	cfg.sanitize()
	return &cfg, nil
}

// sanitize replaces invalid (≤0) pomodoro durations with their defaults.
// This catches the case where a user wrote `pomodoro:` (null) or
// `pomodoro: {}` and ended up with zeroed numeric fields. Booleans
// can't be distinguished from "unset" so they keep whatever the
// unmarshal produced.
func (c *Config) sanitize() {
	d := defaultPomodoro()
	if c.Pomodoro.FocusMinutes <= 0 {
		c.Pomodoro.FocusMinutes = d.FocusMinutes
	}
	if c.Pomodoro.ShortBreakMinutes <= 0 {
		c.Pomodoro.ShortBreakMinutes = d.ShortBreakMinutes
	}
	if c.Pomodoro.LongBreakMinutes <= 0 {
		c.Pomodoro.LongBreakMinutes = d.LongBreakMinutes
	}
	if c.Pomodoro.RoundsUntilLongBreak <= 0 {
		c.Pomodoro.RoundsUntilLongBreak = d.RoundsUntilLongBreak
	}
}

func saveToFile(path string, cfg *Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("writing %q: %w", path, err)
	}
	return nil
}
