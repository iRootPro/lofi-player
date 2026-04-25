package tui

import (
	"testing"

	"github.com/iRootPro/lofi-player/internal/audio"
)

func TestMixerModelDefaultsToFirstChannel(t *testing.T) {
	mm := newMixerModel(audio.NewAmbientMixer())
	if got := mm.Selected(); got != "rain" {
		t.Errorf("Selected default: got %q, want %q", got, "rain")
	}
}

func TestMixerModelNilMixerReturnsEmpty(t *testing.T) {
	mm := newMixerModel(nil)
	if got := mm.Selected(); got != "" {
		t.Errorf("Selected nil: got %q, want empty", got)
	}
}
