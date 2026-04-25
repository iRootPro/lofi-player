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

func TestMixerModelNavigationJK(t *testing.T) {
	mm := newMixerModel(audio.NewAmbientMixer())
	mm = mm.handle("j")
	if got := mm.Selected(); got != "fire" {
		t.Errorf("after j: got %q, want fire", got)
	}
	mm = mm.handle("j")
	if got := mm.Selected(); got != "white_noise" {
		t.Errorf("after jj: got %q, want white_noise", got)
	}
	mm = mm.handle("j") // clamp
	if got := mm.Selected(); got != "white_noise" {
		t.Errorf("after jjj clamp: got %q", got)
	}
	mm = mm.handle("k")
	if got := mm.Selected(); got != "fire" {
		t.Errorf("after k: got %q", got)
	}
	mm = mm.handle("k")
	mm = mm.handle("k") // clamp
	if got := mm.Selected(); got != "rain" {
		t.Errorf("after kkk clamp: got %q", got)
	}
}

func TestMixerModelVolumeAdjustHL(t *testing.T) {
	restore := audio.SetCacheDirForTest(t.TempDir())
	t.Cleanup(restore)
	am := audio.NewAmbientMixer()
	if err := am.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(func() { _ = am.Close() })
	mm := newMixerModel(am)

	mm = mm.handle("l")
	mm = mm.handle("l")
	if v := am.Volume("rain"); v != 10 {
		t.Errorf("after ll: got %d, want 10", v)
	}
	mm = mm.handle("L")
	if v := am.Volume("rain"); v != 35 {
		t.Errorf("after L: got %d, want 35", v)
	}
	mm = mm.handle("h")
	if v := am.Volume("rain"); v != 30 {
		t.Errorf("after h: got %d, want 30", v)
	}
	mm = mm.handle("H")
	if v := am.Volume("rain"); v != 5 {
		t.Errorf("after H: got %d, want 5", v)
	}
	mm = mm.handle("0")
	if v := am.Volume("rain"); v != 0 {
		t.Errorf("after 0: got %d, want 0", v)
	}
	mm = mm.handle("1")
	if v := am.Volume("rain"); v != 100 {
		t.Errorf("after 1: got %d, want 100", v)
	}
	_ = mm
}
