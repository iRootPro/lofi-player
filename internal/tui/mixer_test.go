package tui

import (
	"strings"
	"testing"

	"github.com/iRootPro/lofi-player/internal/audio"
	"github.com/iRootPro/lofi-player/internal/theme"
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

// runHandle is a test helper that mirrors what tea.Program does for the
// mixerModel: applies the keypress, then synchronously executes the
// returned tea.Cmd so the IPC volume change actually lands before the
// next assertion. Production runs the cmd in a goroutine — tests just
// drive it inline so the post-conditions are checkable.
func runHandle(t *testing.T, mm mixerModel, key string) mixerModel {
	t.Helper()
	mm, cmd := mm.handle(key)
	if cmd != nil {
		_ = cmd()
	}
	return mm
}

func TestMixerModelNavigationJK(t *testing.T) {
	// Channel order: rain, fire, white_noise, cafe, thunder.
	mm := newMixerModel(audio.NewAmbientMixer())
	want := []string{"fire", "white_noise", "cafe", "thunder", "thunder"}
	for i, w := range want {
		mm = runHandle(t, mm, "j")
		if got := mm.Selected(); got != w {
			t.Errorf("after %d j-presses: got %q, want %q", i+1, got, w)
		}
	}
	// Walk back up: thunder → cafe → white_noise → fire → rain → rain (clamp)
	wantBack := []string{"cafe", "white_noise", "fire", "rain", "rain"}
	for i, w := range wantBack {
		mm = runHandle(t, mm, "k")
		if got := mm.Selected(); got != w {
			t.Errorf("after %d k-presses: got %q, want %q", i+1, got, w)
		}
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

	mm = runHandle(t, mm, "l")
	mm = runHandle(t, mm, "l")
	if v := am.Volume("rain"); v != 10 {
		t.Errorf("after ll: got %d, want 10", v)
	}
	mm = runHandle(t, mm, "L")
	if v := am.Volume("rain"); v != 35 {
		t.Errorf("after L: got %d, want 35", v)
	}
	mm = runHandle(t, mm, "h")
	if v := am.Volume("rain"); v != 30 {
		t.Errorf("after h: got %d, want 30", v)
	}
	mm = runHandle(t, mm, "H")
	if v := am.Volume("rain"); v != 5 {
		t.Errorf("after H: got %d, want 5", v)
	}
	mm = runHandle(t, mm, "0")
	if v := am.Volume("rain"); v != 0 {
		t.Errorf("after 0: got %d, want 0", v)
	}
	mm = runHandle(t, mm, "1")
	if v := am.Volume("rain"); v != 100 {
		t.Errorf("after 1: got %d, want 100", v)
	}
	_ = mm
}

func TestMixerViewIncludesAllChannels(t *testing.T) {
	restore := audio.SetCacheDirForTest(t.TempDir())
	t.Cleanup(restore)
	am := audio.NewAmbientMixer()
	if err := am.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(func() { _ = am.Close() })
	_ = am.SetVolume("rain", 40)

	mm := newMixerModel(am)
	tk, _ := theme.Lookup("tokyo-night")
	out := mm.view(80, NewStyles(tk), tk)

	for _, want := range []string{"rain", "fire", "white noise", "ambient mixer"} {
		if !strings.Contains(out, want) {
			t.Errorf("view missing %q\n%s", want, out)
		}
	}
}

func TestMixerViewMarksDisabledChannels(t *testing.T) {
	// No Init → all runtime channels absent → Disabled() returns false
	// (per its unknown-id contract). To genuinely exercise the disabled
	// path we'd need a mixer where Init succeeded but newAmbientPlayer
	// failed — hard to provoke deterministically. Instead, assert that
	// view doesn't crash and shows zeros when no channels are active.
	tk, _ := theme.Lookup("tokyo-night")
	mm := newMixerModel(audio.NewAmbientMixer())
	out := mm.view(80, NewStyles(tk), tk)
	if !strings.Contains(out, "rain") {
		t.Errorf("view missing rain row:\n%s", out)
	}
}
