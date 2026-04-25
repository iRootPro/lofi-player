package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLoadFromFile_FirstRunWritesDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	cfg, err := loadFromFile(path)
	if err != nil {
		t.Fatalf("loadFromFile: %v", err)
	}

	want := Defaults()
	if !reflect.DeepEqual(*cfg, want) {
		t.Errorf("loaded config does not equal Defaults()\n got:  %+v\n want: %+v", *cfg, want)
	}

	if _, err := os.Stat(path); err != nil {
		t.Errorf("expected config file written to %q, got %v", path, err)
	}
}

func TestLoadFromFile_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	original := &Config{
		Theme:  "tokyo-night",
		Volume: 42,
		Stations: []Station{
			{Name: "Test", URL: "https://example.com/stream"},
		},
		Pomodoro: PomodoroConfig{
			FocusMinutes:         50,
			ShortBreakMinutes:    10,
			LongBreakMinutes:     20,
			RoundsUntilLongBreak: 3,
			AutoPauseOnBreak:     true,
			AutoResumeOnFocus:    false,
			BreakStations:        []Station{},
		},
	}

	if err := saveToFile(path, original); err != nil {
		t.Fatalf("saveToFile: %v", err)
	}
	loaded, err := loadFromFile(path)
	if err != nil {
		t.Fatalf("loadFromFile: %v", err)
	}
	if !reflect.DeepEqual(*loaded, *original) {
		t.Errorf("round-trip mismatch\n got:  %+v\n want: %+v", *loaded, *original)
	}
}

func TestLoadFromFile_MissingFieldsKeepDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	// File present but missing theme and volume keys.
	yaml := []byte("stations:\n  - name: Only\n    url: http://x\n")
	if err := os.WriteFile(path, yaml, 0o644); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	cfg, err := loadFromFile(path)
	if err != nil {
		t.Fatalf("loadFromFile: %v", err)
	}
	defaults := Defaults()
	if cfg.Theme != defaults.Theme {
		t.Errorf("Theme = %q, want default %q", cfg.Theme, defaults.Theme)
	}
	if cfg.Volume != defaults.Volume {
		t.Errorf("Volume = %d, want default %d", cfg.Volume, defaults.Volume)
	}
	if len(cfg.Stations) != 1 || cfg.Stations[0].Name != "Only" {
		t.Errorf("Stations = %+v, want [{Only http://x}]", cfg.Stations)
	}
}

func TestLoadFromFile_InvalidYAMLReturnsError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("theme: [unterminated\n"), 0o644); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	if _, err := loadFromFile(path); err == nil {
		t.Errorf("expected error for invalid YAML, got nil")
	}
}

func TestLoadFromFile_EmptyStationsList(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	// Both `stations: []` and `stations:` (null) should yield "no stations" —
	// not the default station list. This is the behavior called out in plan §9.
	cases := map[string]string{
		"explicit empty": "theme: tokyo-night\nvolume: 60\nstations: []\n",
		"null value":     "theme: tokyo-night\nvolume: 60\nstations:\n",
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
				t.Fatalf("seed file: %v", err)
			}
			cfg, err := loadFromFile(path)
			if err != nil {
				t.Fatalf("loadFromFile: %v", err)
			}
			if len(cfg.Stations) != 0 {
				t.Errorf("Stations = %+v, want empty/nil", cfg.Stations)
			}
		})
	}
}

func TestDefaultsAreNonEmpty(t *testing.T) {
	d := Defaults()
	if d.Theme == "" {
		t.Error("Defaults().Theme is empty")
	}
	if d.Volume < 0 || d.Volume > 100 {
		t.Errorf("Defaults().Volume = %d, want 0..100", d.Volume)
	}
	if len(d.Stations) == 0 {
		t.Error("Defaults().Stations is empty")
	}
	for i, s := range d.Stations {
		if s.Name == "" || s.URL == "" {
			t.Errorf("Defaults().Stations[%d] = %+v has empty field", i, s)
		}
	}
	// Pomodoro defaults must be sensible.
	if d.Pomodoro.FocusMinutes != 25 || d.Pomodoro.ShortBreakMinutes != 5 ||
		d.Pomodoro.LongBreakMinutes != 15 || d.Pomodoro.RoundsUntilLongBreak != 4 {
		t.Errorf("Defaults().Pomodoro durations: %+v", d.Pomodoro)
	}
	if !d.Pomodoro.AutoPauseOnBreak || !d.Pomodoro.AutoResumeOnFocus {
		t.Errorf("Defaults().Pomodoro auto-* flags should default to true: %+v", d.Pomodoro)
	}
}

func TestLoadFromFile_PartialPomodoroKeepsDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	body := []byte(`
theme: tokyo-night
volume: 60
stations:
  - {name: A, url: http://x}
pomodoro:
  focus_minutes: 50
`)
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadFromFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Pomodoro.FocusMinutes != 50 {
		t.Errorf("FocusMinutes overridden = %d, want 50", cfg.Pomodoro.FocusMinutes)
	}
	if cfg.Pomodoro.ShortBreakMinutes != 5 {
		t.Errorf("ShortBreakMinutes default = %d, want 5", cfg.Pomodoro.ShortBreakMinutes)
	}
	if cfg.Pomodoro.RoundsUntilLongBreak != 4 {
		t.Errorf("RoundsUntilLongBreak default = %d, want 4", cfg.Pomodoro.RoundsUntilLongBreak)
	}
}

func TestStationKind(t *testing.T) {
	tests := []struct {
		station   Station
		wantKind  string
		wantYTube bool
	}{
		{Station{}, KindStream, false},
		{Station{Kind: ""}, KindStream, false},
		{Station{Kind: "stream"}, KindStream, false},
		{Station{Kind: "youtube"}, KindYouTube, true},
		// Unknown kinds aren't rejected here — they pass through and the
		// audio engine will surface whatever error mpv produces.
		{Station{Kind: "magnet"}, "magnet", false},
	}
	for _, tc := range tests {
		t.Run(tc.station.Kind, func(t *testing.T) {
			if got := tc.station.EffectiveKind(); got != tc.wantKind {
				t.Errorf("EffectiveKind() = %q, want %q", got, tc.wantKind)
			}
			if got := tc.station.IsYouTube(); got != tc.wantYTube {
				t.Errorf("IsYouTube() = %v, want %v", got, tc.wantYTube)
			}
		})
	}
}

func TestLoadFromFile_KindDefaultsToStreamForExistingConfigs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	// Mirrors a Phase 0-3 config with no Kind field per station.
	body := []byte(`
theme: tokyo-night
volume: 60
stations:
  - name: SomaFM
    url: https://ice1.somafm.com/groovesalad-256-mp3
`)
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadFromFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.Stations[0].EffectiveKind(); got != KindStream {
		t.Errorf("legacy station EffectiveKind() = %q, want %q", got, KindStream)
	}
}

func TestLoadFromFile_YouTubeKindParses(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	body := []byte(`
theme: tokyo-night
volume: 60
stations:
  - name: Lofi Girl
    url: https://www.youtube.com/watch?v=jfKfPfyJRdk
    kind: youtube
`)
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadFromFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Stations[0].IsYouTube() {
		t.Errorf("station IsYouTube() = false, want true")
	}
}

func TestLoadFromFile_NullPomodoroFallsBackToDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	body := []byte(`
theme: tokyo-night
volume: 60
stations: []
pomodoro:
`)
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadFromFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Pomodoro.FocusMinutes != 25 {
		t.Errorf("FocusMinutes = %d, want 25 after null pomodoro", cfg.Pomodoro.FocusMinutes)
	}
}
