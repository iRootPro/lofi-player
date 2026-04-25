package tui

import (
	"testing"

	"github.com/iRootPro/lofi-player/internal/config"
)

func TestDetectKind(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"https://www.youtube.com/watch?v=jfKfPfyJRdk", config.KindYouTube},
		{"https://youtu.be/dQw4w9WgXcQ", config.KindYouTube},
		{"https://YOUTUBE.com/watch?v=abc", config.KindYouTube},
		{"https://music.youtube.com/playlist?list=foo", config.KindYouTube},
		{"https://ice1.somafm.com/groovesalad-256-mp3", config.KindStream},
		{"https://stream.radioparadise.com/mellow-128", config.KindStream},
		{"http://example.com/audio.mp3", config.KindStream},
		{"", config.KindStream},
	}
	for _, tc := range tests {
		t.Run(tc.url, func(t *testing.T) {
			if got := detectKind(tc.url); got != tc.want {
				t.Errorf("detectKind(%q) = %q, want %q", tc.url, got, tc.want)
			}
		})
	}
}
