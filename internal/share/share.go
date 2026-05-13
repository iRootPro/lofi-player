// Package share serializes and parses small station snippets that users can
// paste into chats, READMEs, gists, or back into lofi-player.
package share

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/iRootPro/lofi-player/internal/config"
)

// Document is the canonical exchange envelope. Keeping the top-level key as
// `stations` makes snippets directly copy-pasteable into config.yaml.
type Document struct {
	Stations []config.Station `yaml:"stations"`
}

// MarshalStation returns a YAML snippet containing one station.
func MarshalStation(st config.Station) (string, error) {
	return MarshalStations([]config.Station{st})
}

// MarshalStations returns a YAML snippet containing stations.
func MarshalStations(stations []config.Station) (string, error) {
	if len(stations) == 0 {
		return "", errors.New("no stations to share")
	}
	normalized := make([]config.Station, 0, len(stations))
	for _, st := range stations {
		st = normalizeForShare(st)
		if strings.TrimSpace(st.Name) == "" || strings.TrimSpace(st.URL) == "" {
			return "", errors.New("station name and url are required")
		}
		normalized = append(normalized, st)
	}
	data, err := yaml.Marshal(Document{Stations: normalized})
	if err != nil {
		return "", fmt.Errorf("marshaling station snippet: %w", err)
	}
	return string(data), nil
}

// Parse accepts the canonical `stations:` document, a single station mapping,
// or a bare YAML list of stations. Markdown fenced code blocks are accepted so
// users can copy directly from Telegram/Slack/Discord messages.
func Parse(input string) ([]config.Station, error) {
	body := strings.TrimSpace(stripMarkdownFence(input))
	if body == "" {
		return nil, errors.New("empty station snippet")
	}

	var doc Document
	if err := yaml.Unmarshal([]byte(body), &doc); err == nil && len(doc.Stations) > 0 {
		return normalizeParsed(doc.Stations)
	}

	var one config.Station
	if err := yaml.Unmarshal([]byte(body), &one); err == nil && strings.TrimSpace(one.URL) != "" {
		return normalizeParsed([]config.Station{one})
	}

	var list []config.Station
	if err := yaml.Unmarshal([]byte(body), &list); err == nil && len(list) > 0 {
		return normalizeParsed(list)
	}

	if st, ok := stationFromBareURL(body); ok {
		return normalizeParsed([]config.Station{st})
	}

	return nil, errors.New("no stations found in snippet")
}

func normalizeForShare(st config.Station) config.Station {
	st.Name = strings.TrimSpace(st.Name)
	st.URL = strings.TrimSpace(st.URL)
	st.Kind = strings.TrimSpace(st.Kind)
	// Direct streams are the default; omit kind to keep chat snippets short.
	if st.Kind == config.KindStream {
		st.Kind = ""
	}
	return st
}

func normalizeParsed(stations []config.Station) ([]config.Station, error) {
	out := make([]config.Station, 0, len(stations))
	for i, st := range stations {
		st.Name = strings.TrimSpace(st.Name)
		st.URL = strings.TrimSpace(st.URL)
		st.Kind = strings.TrimSpace(st.Kind)
		if st.Name == "" {
			return nil, fmt.Errorf("station %d: name is required", i+1)
		}
		if st.URL == "" {
			return nil, fmt.Errorf("station %d: url is required", i+1)
		}
		if st.Kind == "" && looksYouTube(st.URL) {
			st.Kind = config.KindYouTube
		}
		out = append(out, st)
	}
	return out, nil
}

func looksYouTube(url string) bool {
	u := strings.ToLower(url)
	return strings.Contains(u, "youtube.com") || strings.Contains(u, "youtu.be")
}

func stationFromBareURL(input string) (config.Station, bool) {
	if strings.ContainsAny(input, " \t\n\r") {
		return config.Station{}, false
	}
	parsed, err := url.Parse(input)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return config.Station{}, false
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return config.Station{}, false
	}
	name := parsed.Hostname()
	name = strings.TrimPrefix(name, "www.")
	if name == "" {
		name = "Pasted station"
	}
	return config.Station{Name: name, URL: input}, true
}

func stripMarkdownFence(input string) string {
	s := strings.TrimSpace(input)
	if !strings.HasPrefix(s, "```") {
		return s
	}
	lines := strings.Split(s, "\n")
	if len(lines) < 2 {
		return s
	}
	// Drop the opening fence, including an optional language tag (`yaml`).
	lines = lines[1:]
	for i, line := range lines {
		if strings.TrimSpace(line) == "```" {
			return strings.Join(lines[:i], "\n")
		}
	}
	return strings.Join(lines, "\n")
}
