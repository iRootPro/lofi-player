package theme

import "testing"

func TestLookup(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantName  string
		wantFound bool
	}{
		{"tokyo-night", "tokyo-night", "tokyo-night", true},
		{"catppuccin-mocha", "catppuccin-mocha", "catppuccin-mocha", true},
		{"gruvbox-dark", "gruvbox-dark", "gruvbox-dark", true},
		{"rose-pine", "rose-pine", "rose-pine", true},
		{"catppuccin-latte", "catppuccin-latte", "catppuccin-latte", true},
		{"rose-pine-dawn", "rose-pine-dawn", "rose-pine-dawn", true},
		{"solarized-light", "solarized-light", "solarized-light", true},
		{"paper", "paper", "paper", true},

		{"empty string falls back to default", "", "tokyo-night", false},
		{"unknown name falls back", "monokai", "tokyo-night", false},
		{"case sensitive — wrong case is unknown", "Tokyo-Night", "tokyo-night", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := Lookup(tc.input)
			if ok != tc.wantFound {
				t.Errorf("Lookup(%q) ok = %v, want %v", tc.input, ok, tc.wantFound)
			}
			if got.Name != tc.wantName {
				t.Errorf("Lookup(%q) Name = %q, want %q", tc.input, got.Name, tc.wantName)
			}
		})
	}
}

func TestAllRegisteredThemesHaveAllRoles(t *testing.T) {
	for _, name := range Names() {
		t.Run(name, func(t *testing.T) {
			th, ok := Lookup(name)
			if !ok {
				t.Fatalf("Names() lists %q but Lookup says it's unknown", name)
			}
			if th.Name != name {
				t.Errorf("loaded theme Name = %q, want %q", th.Name, name)
			}
			roles := map[string]string{
				"Background": string(th.Background),
				"Foreground": string(th.Foreground),
				"Muted":      string(th.Muted),
				"Subtle":     string(th.Subtle),
				"Primary":    string(th.Primary),
				"Secondary":  string(th.Secondary),
				"Accent":     string(th.Accent),
				"Success":    string(th.Success),
				"Warning":    string(th.Warning),
				"Info":       string(th.Info),
			}
			for role, value := range roles {
				if value == "" {
					t.Errorf("role %s is empty", role)
				}
			}
		})
	}
}

func TestNamesStartsWithTokyoNight(t *testing.T) {
	names := Names()
	if len(names) == 0 || names[0] != "tokyo-night" {
		t.Errorf("Names() should start with tokyo-night, got %v", names)
	}
}

func TestThemeInfosMatchNames(t *testing.T) {
	infos := Infos()
	names := Names()
	if len(infos) != len(names) {
		t.Fatalf("Infos length = %d, Names length = %d", len(infos), len(names))
	}
	for i, info := range infos {
		if info.Name != names[i] {
			t.Errorf("Infos()[%d].Name = %q, want %q", i, info.Name, names[i])
		}
		if info.DisplayName == "" {
			t.Errorf("Info %q has empty DisplayName", info.Name)
		}
		if info.Description == "" {
			t.Errorf("Info %q has empty Description", info.Name)
		}
		got, ok := InfoFor(info.Name)
		if !ok || got != info {
			t.Errorf("InfoFor(%q) = %+v, %v; want %+v, true", info.Name, got, ok, info)
		}
	}

	fallback, ok := InfoFor("missing")
	if ok || fallback.Name != "tokyo-night" {
		t.Errorf("InfoFor(missing) = %+v, %v; want tokyo-night fallback, false", fallback, ok)
	}
}

func TestNext(t *testing.T) {
	names := Names()
	if len(names) < 2 {
		t.Skip("need at least 2 themes to test Next")
	}

	tests := []struct {
		from, want string
	}{
		{names[0], names[1]},
		{names[len(names)-1], names[0]}, // wrap around
		{"unknown-theme", names[0]},     // unknown cycles back to first
		{"", names[0]},                  // empty cycles to first
	}
	for _, tc := range tests {
		if got := Next(tc.from); got != tc.want {
			t.Errorf("Next(%q) = %q, want %q", tc.from, got, tc.want)
		}
	}
}
