package audio

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// translateOne is the only Player surface that's testable without an
// actual mpv subprocess — it's a pure function over an ipcEvent and the
// player's metadata-dedup state.

func TestTranslateOne_PauseToggle(t *testing.T) {
	p := &Player{}
	if e := p.translateOne(ipcEvent{Event: "property-change", Name: "pause", Data: json.RawMessage("true")}); e == nil {
		t.Error("pause=true: got nil, want PlaybackPaused")
	} else if _, ok := e.(PlaybackPaused); !ok {
		t.Errorf("pause=true: got %T, want PlaybackPaused", e)
	}
	if e := p.translateOne(ipcEvent{Event: "property-change", Name: "pause", Data: json.RawMessage("false")}); e == nil {
		t.Error("pause=false: got nil, want PlaybackStarted")
	} else if _, ok := e.(PlaybackStarted); !ok {
		t.Errorf("pause=false: got %T, want PlaybackStarted", e)
	}
}

func TestTranslateOne_MetadataDedup(t *testing.T) {
	p := &Player{}
	raw := ipcEvent{
		Event: "property-change",
		Name:  "metadata",
		Data:  json.RawMessage(`{"icy-title":"Hippie Sabotage - Snowflakes"}`),
	}
	first := p.translateOne(raw)
	md, ok := first.(MetadataChanged)
	if !ok {
		t.Fatalf("first call: got %T, want MetadataChanged", first)
	}
	if md.Title != "Snowflakes" || md.Artist != "Hippie Sabotage" {
		t.Errorf("first call: got %+v", md)
	}
	if e := p.translateOne(raw); e != nil {
		t.Errorf("duplicate metadata: got %v, want nil (dedup)", e)
	}

	// Different metadata should fire again.
	raw.Data = json.RawMessage(`{"icy-title":"Other - Track"}`)
	if e := p.translateOne(raw); e == nil {
		t.Error("changed metadata: got nil, want MetadataChanged")
	}
}

func TestTranslateOne_MetadataEmptyIgnored(t *testing.T) {
	p := &Player{}
	raw := ipcEvent{
		Event: "property-change",
		Name:  "metadata",
		Data:  json.RawMessage(`{}`),
	}
	if e := p.translateOne(raw); e != nil {
		t.Errorf("empty metadata: got %v, want nil", e)
	}
}

func TestTranslateOne_MediaTitleFallback(t *testing.T) {
	p := &Player{}
	raw := ipcEvent{
		Event: "property-change",
		Name:  "media-title",
		Data:  json.RawMessage(`"Stream Name"`),
	}
	first := p.translateOne(raw)
	md, ok := first.(MetadataChanged)
	if !ok {
		t.Fatalf("got %T, want MetadataChanged", first)
	}
	if md.Title != "Stream Name" || md.Artist != "" {
		t.Errorf("got %+v, want {Title:'Stream Name', Artist:''}", md)
	}
	// Same title — dedup.
	if e := p.translateOne(raw); e != nil {
		t.Errorf("duplicate media-title: got %v, want nil", e)
	}
}

func TestTranslateOne_MediaTitleURLLikeIgnored(t *testing.T) {
	p := &Player{}
	cases := []string{
		"watch?v=jfKfPfyJRdk",
		"https://example.com/stream.mp3",
		"http://radio.example.com",
		"rtmp://live.example.com/app",
		"https://www.youtube.com/watch?v=xyz",
	}
	for _, raw := range cases {
		t.Run(raw, func(t *testing.T) {
			payload, _ := json.Marshal(raw)
			evt := p.translateOne(ipcEvent{
				Event: "property-change",
				Name:  "media-title",
				Data:  payload,
			})
			if evt != nil {
				t.Errorf("URL-like media-title %q produced %v, want nil (suppressed)", raw, evt)
			}
		})
	}
}

func TestTranslateOne_MediaTitleSuppressedWhenArtistKnown(t *testing.T) {
	p := &Player{lastTitle: "Track", lastArtist: "Artist"}
	raw := ipcEvent{
		Event: "property-change",
		Name:  "media-title",
		Data:  json.RawMessage(`"Some Stream"`),
	}
	if e := p.translateOne(raw); e != nil {
		t.Errorf("media-title with prior artist: got %v, want nil", e)
	}
}

func TestTranslateOne_EndFileEOF(t *testing.T) {
	p := &Player{}
	if e := p.translateOne(ipcEvent{Event: "end-file", Reason: "eof"}); e == nil {
		t.Error("got nil, want EOF")
	} else if _, ok := e.(EOF); !ok {
		t.Errorf("got %T, want EOF", e)
	}
}

func TestTranslateOne_EndFileError(t *testing.T) {
	p := &Player{}
	e := p.translateOne(ipcEvent{Event: "end-file", Reason: "error"})
	pe, ok := e.(PlaybackError)
	if !ok {
		t.Fatalf("got %T, want PlaybackError", e)
	}
	if pe.Err == nil {
		t.Error("PlaybackError.Err is nil")
	}
}

func TestTranslateOne_PlaybackRestart(t *testing.T) {
	p := &Player{}
	e := p.translateOne(ipcEvent{Event: "playback-restart"})
	if _, ok := e.(PlaybackStarted); !ok {
		t.Errorf("playback-restart: got %T, want PlaybackStarted", e)
	}
}

func TestTranslateOne_PausedForCache(t *testing.T) {
	p := &Player{}
	e := p.translateOne(ipcEvent{
		Event: "property-change",
		Name:  "paused-for-cache",
		Data:  json.RawMessage(`true`),
	})
	bc, ok := e.(BufferingChanged)
	if !ok {
		t.Fatalf("paused-for-cache: got %T, want BufferingChanged", e)
	}
	if !bc.Stalled {
		t.Fatal("paused-for-cache true produced Stalled=false")
	}
}

func TestTranslateOne_UnknownEventDropped(t *testing.T) {
	p := &Player{}
	if e := p.translateOne(ipcEvent{Event: "property-change", Name: "idle-active", Data: json.RawMessage("true")}); e != nil {
		t.Errorf("idle-active: got %v, want nil", e)
	}
}

func TestClampVolume(t *testing.T) {
	tests := []struct {
		in, want int
	}{
		{-10, 0},
		{0, 0},
		{50, 50},
		{100, 100},
		{150, 100},
	}
	for _, tc := range tests {
		if got := clampVolume(tc.in); got != tc.want {
			t.Errorf("clampVolume(%d) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestMPVArgsDisableUserConfig(t *testing.T) {
	mainArgs := mainMPVArgs("/tmp/lofi-player-test.sock")
	if !hasArg(mainArgs, "--no-config") {
		t.Fatalf("main mpv args %v do not disable user config", mainArgs)
	}
	if hasArg(mainArgs, "--really-quiet") {
		t.Fatalf("main mpv args %v suppress startup diagnostics", mainArgs)
	}
	if !hasArg(mainArgs, "--input-ipc-server=/tmp/lofi-player-test.sock") {
		t.Fatalf("main mpv args %v do not configure IPC socket", mainArgs)
	}

	ambientArgs := ambientMPVArgs("/tmp/lofi-ambient-test.sock", "/tmp/rain.opus")
	if !hasArg(ambientArgs, "--no-config") {
		t.Fatalf("ambient mpv args %v do not disable user config", ambientArgs)
	}
	if hasArg(ambientArgs, "--really-quiet") {
		t.Fatalf("ambient mpv args %v suppress startup diagnostics", ambientArgs)
	}
	if !hasArg(ambientArgs, "/tmp/rain.opus") {
		t.Fatalf("ambient mpv args %v do not include file path", ambientArgs)
	}
}

func TestDarwinMPVArgsWireMediaKeys(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("media-key args are macOS-only")
	}
	mainArgs := mainMPVArgs("/tmp/lofi-player-test.sock")
	if !hasArg(mainArgs, "--input-media-keys=yes") {
		t.Fatalf("main mpv args %v do not enable media keys", mainArgs)
	}
	if !hasArg(mainArgs, "--input-conf=/tmp/input.conf") {
		t.Fatalf("main mpv args %v do not include media-key input.conf", mainArgs)
	}

	ambientArgs := ambientMPVArgs("/tmp/lofi-ambient-test.sock", "/tmp/rain.opus")
	if !hasArg(ambientArgs, "--input-media-keys=no") {
		t.Fatalf("ambient mpv args %v do not disable media keys", ambientArgs)
	}
}

func TestWriteMainInputConf(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("media-key input.conf is macOS-only")
	}
	socketPath := filepath.Join(t.TempDir(), "mpv.sock")
	if err := writeMainInputConf(socketPath); err != nil {
		t.Fatalf("writeMainInputConf: %v", err)
	}
	data, err := os.ReadFile(mainInputConfPath(socketPath))
	if err != nil {
		t.Fatalf("read input.conf: %v", err)
	}
	if got := string(data); got != mainInputConf {
		t.Fatalf("input.conf = %q, want %q", got, mainInputConf)
	}
}

func TestFormatMPVStartupErrorIncludesDiagnostics(t *testing.T) {
	err := formatMPVStartupError(context.DeadlineExceeded, "fatal: bad option\n", []string{"mpv", "--no-config", "--input-ipc-server=/tmp/lofi.sock"})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("formatted error does not wrap deadline: %v", err)
	}
	msg := err.Error()
	for _, want := range []string{"mpv did not open IPC socket", "fatal: bad option", "--no-config", "--input-ipc-server=/tmp/lofi.sock", "hint:"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("formatted error %q does not contain %q", msg, want)
		}
	}
}

func TestWaitForSocketOrExitContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := waitForSocketOrExit(ctx, "/tmp/lofi-player-test-missing.sock", time.Second, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("waitForSocketOrExit() = %v, want context.Canceled", err)
	}
}

func TestTerminateMPVProcessDoesNotBlockAfterProcessAlreadyExited(t *testing.T) {
	cmd := exec.Command("sh", "-c", "exit 0")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start helper process: %v", err)
	}
	exited := newProcessWaiter(cmd)
	select {
	case <-exited.Done():
		if err := exited.Err(); err != nil {
			t.Fatalf("helper process exited with error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("helper process did not exit")
	}

	done := make(chan struct{})
	go func() {
		terminateMPVProcess(cmd, exited)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("terminateMPVProcess blocked after process had already exited")
	}
}

func hasArg(args []string, want string) bool {
	for _, arg := range args {
		if arg == want {
			return true
		}
	}
	return false
}
