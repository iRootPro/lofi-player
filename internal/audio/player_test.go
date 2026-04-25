package audio

import (
	"encoding/json"
	"testing"
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

func TestTranslateOne_UnknownEventDropped(t *testing.T) {
	p := &Player{}
	if e := p.translateOne(ipcEvent{Event: "playback-restart"}); e != nil {
		t.Errorf("playback-restart: got %v, want nil", e)
	}
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
