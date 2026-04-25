package audio

import "testing"

func TestParseMetadata(t *testing.T) {
	tests := []struct {
		name       string
		input      map[string]string
		wantTitle  string
		wantArtist string
	}{
		{
			name:       "standard Artist - Title",
			input:      map[string]string{"icy-title": "Hippie Sabotage - Snowflakes"},
			wantTitle:  "Snowflakes",
			wantArtist: "Hippie Sabotage",
		},
		{
			name:       "title only, no dash",
			input:      map[string]string{"icy-title": "Lofi Chillhop Mix"},
			wantTitle:  "Lofi Chillhop Mix",
			wantArtist: "",
		},
		{
			name:       "multi-dash splits on first only",
			input:      map[string]string{"icy-title": "Foo - Bar - Remix"},
			wantTitle:  "Bar - Remix",
			wantArtist: "Foo",
		},
		{
			name:       "leading and trailing whitespace trimmed",
			input:      map[string]string{"icy-title": "  A  -  B  "},
			wantTitle:  "B",
			wantArtist: "A",
		},
		{
			name:       "unicode artist and title",
			input:      map[string]string{"icy-title": "Чайф - Поплачь о нём"},
			wantTitle:  "Поплачь о нём",
			wantArtist: "Чайф",
		},
		{
			name:       "fallback to title key",
			input:      map[string]string{"title": "Some Track"},
			wantTitle:  "Some Track",
			wantArtist: "",
		},
		{
			name:       "fallback to media-title",
			input:      map[string]string{"media-title": "Stream Name"},
			wantTitle:  "Stream Name",
			wantArtist: "",
		},
		{
			name: "icy-title wins over title and media-title",
			input: map[string]string{
				"icy-title":   "Real - Track",
				"title":       "Fallback",
				"media-title": "Stream",
			},
			wantTitle:  "Track",
			wantArtist: "Real",
		},
		{
			name:       "icy-title empty string falls through to next key",
			input:      map[string]string{"icy-title": "  ", "title": "Backup"},
			wantTitle:  "Backup",
			wantArtist: "",
		},
		{
			name:       "all empty input",
			input:      map[string]string{},
			wantTitle:  "",
			wantArtist: "",
		},
		{
			name:       "nil map",
			input:      nil,
			wantTitle:  "",
			wantArtist: "",
		},
		{
			name:       "dash with no spaces is not a separator",
			input:      map[string]string{"icy-title": "Foo-Bar"},
			wantTitle:  "Foo-Bar",
			wantArtist: "",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotTitle, gotArtist := ParseMetadata(tc.input)
			if gotTitle != tc.wantTitle {
				t.Errorf("title: got %q, want %q", gotTitle, tc.wantTitle)
			}
			if gotArtist != tc.wantArtist {
				t.Errorf("artist: got %q, want %q", gotArtist, tc.wantArtist)
			}
		})
	}
}
