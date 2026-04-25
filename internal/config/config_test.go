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
}
