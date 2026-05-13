package share

import (
	"strings"
	"testing"

	"github.com/iRootPro/lofi-player/internal/config"
)

func TestMarshalStationUsesCanonicalEnvelope(t *testing.T) {
	got, err := MarshalStation(config.Station{Name: "Soma", URL: "https://example.com/stream", Kind: config.KindStream})
	if err != nil {
		t.Fatalf("MarshalStation: %v", err)
	}
	if !strings.Contains(got, "stations:") || !strings.Contains(got, "name: Soma") || !strings.Contains(got, "url: https://example.com/stream") {
		t.Fatalf("snippet missing expected fields:\n%s", got)
	}
	if strings.Contains(got, "kind: stream") {
		t.Fatalf("direct stream kind should be omitted from snippets:\n%s", got)
	}
}

func TestMarshalStationKeepsYouTubeKind(t *testing.T) {
	got, err := MarshalStation(config.Station{Name: "Lofi Girl", URL: "https://www.youtube.com/watch?v=jfKfPfyJRdk", Kind: config.KindYouTube})
	if err != nil {
		t.Fatalf("MarshalStation: %v", err)
	}
	if !strings.Contains(got, "kind: youtube") {
		t.Fatalf("youtube kind missing from snippet:\n%s", got)
	}
}

func TestParseCanonicalDocument(t *testing.T) {
	got, err := Parse(`stations:
  - name: Soma
    url: https://example.com/stream
`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(got) != 1 || got[0].Name != "Soma" || got[0].URL != "https://example.com/stream" {
		t.Fatalf("unexpected stations: %+v", got)
	}
}

func TestParseSingleStationMapping(t *testing.T) {
	got, err := Parse(`name: Soma
url: https://example.com/stream
`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(got) != 1 || got[0].Name != "Soma" {
		t.Fatalf("unexpected stations: %+v", got)
	}
}

func TestParseBareList(t *testing.T) {
	got, err := Parse(`- name: A
  url: https://a
- name: B
  url: https://b
`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(got) != 2 || got[1].Name != "B" {
		t.Fatalf("unexpected stations: %+v", got)
	}
}

func TestParseMarkdownFenceAndDetectsYouTube(t *testing.T) {
	got, err := Parse("```yaml\nstations:\n  - name: Lofi Girl\n    url: https://youtu.be/jfKfPfyJRdk\n```")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got[0].Kind != config.KindYouTube {
		t.Fatalf("Kind = %q, want youtube", got[0].Kind)
	}
}

func TestParseBareURL(t *testing.T) {
	got, err := Parse("https://ice1.somafm.com/groovesalad-256-mp3")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(got) != 1 || got[0].Name != "ice1.somafm.com" || got[0].URL != "https://ice1.somafm.com/groovesalad-256-mp3" {
		t.Fatalf("unexpected stations: %+v", got)
	}
}

func TestParseRejectsEmpty(t *testing.T) {
	if _, err := Parse("stations: []\n"); err == nil {
		t.Fatal("expected error for empty snippet")
	}
}
