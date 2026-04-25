package theme

import "testing"

func TestLookup(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantName  string
		wantFound bool
	}{
		{"known theme", "tokyo-night", "tokyo-night", true},
		{"empty string falls back", "", "tokyo-night", false},
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

func TestTokyoNightHasAllRoles(t *testing.T) {
	tn := TokyoNight()
	roles := map[string]string{
		"Background": string(tn.Background),
		"Foreground": string(tn.Foreground),
		"Muted":      string(tn.Muted),
		"Subtle":     string(tn.Subtle),
		"Primary":    string(tn.Primary),
		"Secondary":  string(tn.Secondary),
		"Accent":     string(tn.Accent),
		"Success":    string(tn.Success),
		"Warning":    string(tn.Warning),
		"Info":       string(tn.Info),
	}
	for role, value := range roles {
		if value == "" {
			t.Errorf("Tokyo Night role %s is empty", role)
		}
	}
	if tn.Name != "tokyo-night" {
		t.Errorf("Tokyo Night Name = %q, want %q", tn.Name, "tokyo-night")
	}
}
