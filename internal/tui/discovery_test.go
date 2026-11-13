package tui

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/iRootPro/lofi-player/internal/audio"
	"github.com/iRootPro/lofi-player/internal/config"
	"github.com/iRootPro/lofi-player/internal/radiobrowser"
)

type fakeDiscoveryClient struct {
	results radiobrowser.Results
	err     error
	clicked []string
	queries []string
}

func (f *fakeDiscoveryClient) Search(_ context.Context, query string) (radiobrowser.Results, error) {
	f.queries = append(f.queries, query)
	return f.results, f.err
}

func (f *fakeDiscoveryClient) RegisterClick(_ context.Context, uuid string) error {
	f.clicked = append(f.clicked, uuid)
	return nil
}

func discoveryFixture(client radiobrowser.Client) Model {
	m := fixture()
	m.discoveryClient = client
	return m
}

func discoveryKey(r rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
}

func openDiscovery(t *testing.T, m Model) Model {
	t.Helper()
	next, _ := m.Update(discoveryKey('/'))
	m = next.(Model)
	if m.mode != modeDiscovery {
		t.Fatalf("mode = %v, want modeDiscovery", m.mode)
	}
	return m
}

func typeDiscoveryQuery(t *testing.T, m Model, query string) Model {
	t.Helper()
	for _, r := range query {
		next, _ := m.Update(discoveryKey(r))
		m = next.(Model)
	}
	return m
}

func TestDiscoveryDebouncesAndDropsStaleResults(t *testing.T) {
	client := &fakeDiscoveryClient{results: radiobrowser.Results{Stations: []radiobrowser.Station{{Name: "Found", StreamURL: "https://found"}}}}
	m := openDiscovery(t, discoveryFixture(client))
	m = typeDiscoveryQuery(t, m, "ambient")
	if got, want := m.discovery.searchSeq, len("ambient"); got != want {
		t.Fatalf("searchSeq = %d, want %d", got, want)
	}
	if got := m.discovery.input.Value(); got != "ambient" {
		t.Fatalf("query = %q", got)
	}
	// A tick for an earlier debounce window is ignored, so only the final
	// value reaches the network client.
	next, cmd := m.Update(discoverySearchTickMsg{session: m.discovery.session, seq: m.discovery.searchSeq - 1, query: "ambien"})
	m = next.(Model)
	if cmd != nil || len(client.queries) != 0 {
		t.Fatal("stale debounce tick started a search")
	}
	next, cmd = m.Update(discoverySearchTickMsg{session: m.discovery.session, seq: m.discovery.searchSeq, query: "ambient"})
	m = next.(Model)
	result := cmd()
	next, _ = m.Update(result)
	m = next.(Model)
	if got := client.queries; len(got) != 1 || got[0] != "ambient" {
		t.Fatalf("search queries = %#v", got)
	}
	if len(m.discovery.results) != 1 || m.discovery.results[0].Name != "Found" {
		t.Fatalf("debounced result not applied: %#v", m.discovery.results)
	}

	old := discoverySearchResultMsg{session: m.discovery.session, seq: m.discovery.searchSeq - 1, results: radiobrowser.Results{Stations: []radiobrowser.Station{{Name: "Old", StreamURL: "https://old"}}}}
	next, _ = m.Update(old)
	m = next.(Model)
	if len(m.discovery.results) != 1 || m.discovery.results[0].Name != "Found" {
		t.Fatalf("stale response replaced results: %#v", m.discovery.results)
	}

	latest := discoverySearchResultMsg{session: m.discovery.session, seq: m.discovery.searchSeq, results: radiobrowser.Results{Cached: true, Stations: []radiobrowser.Station{{Name: "Fresh", StreamURL: "https://fresh"}}}}
	next, _ = m.Update(latest)
	m = next.(Model)
	if len(m.discovery.results) != 1 || m.discovery.results[0].Name != "Fresh" || !m.discovery.cached {
		t.Fatalf("latest response not applied: %#v", m.discovery)
	}
	if !strings.Contains(m.View(), "cached results") {
		t.Fatal("discovery view does not identify cached results")
	}
}

func TestDiscoveryPreviewDoesNotPersistStation(t *testing.T) {
	m := openDiscovery(t, discoveryFixture(&fakeDiscoveryClient{}))
	m.discovery.results = []radiobrowser.Station{{UUID: "preview-id", Name: "Preview", StreamURL: "https://preview"}}
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = next.(Model)
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)

	if m.previewStation == nil || m.previewStation.Name != "Preview" {
		t.Fatalf("preview station = %#v", m.previewStation)
	}
	if m.playingIdx != -1 || !m.playing || !m.loading {
		t.Fatalf("preview playback state: idx=%d playing=%v loading=%v", m.playingIdx, m.playing, m.loading)
	}
	if out := m.View(); !strings.Contains(out, "Loading Preview") || !strings.Contains(out, "not saved") || !strings.Contains(out, "enter play") {
		t.Fatalf("modal does not acknowledge starting transient playback:\n%s", out)
	}
	for _, station := range m.cfg.Stations {
		if station.URL == "https://preview" {
			t.Fatal("preview station was added to config")
		}
	}
	if !strings.Contains(m.View(), "Preview") {
		t.Fatal("now-playing card does not render preview station")
	}
	// Playback events must bypass the text-input modal so a preview can leave
	// the loading state and the long-lived audio subscription is re-armed.
	next, _ = m.Update(PlaybackStartedMsg{RequestID: m.playRequestID})
	m = next.(Model)
	if m.loading || !m.playing {
		t.Fatalf("preview start event was swallowed: loading=%v playing=%v", m.loading, m.playing)
	}
	if out := m.View(); !strings.Contains(out, "Playing Preview") || !strings.Contains(out, "not saved") || !strings.Contains(out, "playing") {
		t.Fatalf("modal does not show active playback and save affordance:\n%s", out)
	}

	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = next.(Model)
	if m.mode != modeFull || m.previewStation == nil {
		t.Fatalf("Esc changed preview playback: mode=%v preview=%#v", m.mode, m.previewStation)
	}
}

func TestDiscoveryLoadingKeepsSpinnerTicking(t *testing.T) {
	m := openDiscovery(t, discoveryFixture(&fakeDiscoveryClient{}))
	m.discovery.results = []radiobrowser.Station{{UUID: "preview-id", Name: "Preview", StreamURL: "https://preview"}}
	m.discovery = m.discovery.setFocus(discoveryResultsFocus)
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)

	updated, cmd := m.Update(m.spinner.Tick())
	m = updated.(Model)
	if !m.loading || cmd == nil {
		t.Fatalf("spinner tick was swallowed by discovery modal: loading=%v next=%v", m.loading, cmd)
	}
}

func TestDiscoveryArrowNavigationMovesBetweenInputAndResults(t *testing.T) {
	m := openDiscovery(t, discoveryFixture(&fakeDiscoveryClient{}))
	m.discovery.results = []radiobrowser.Station{
		{Name: "First", StreamURL: "https://first"},
		{Name: "Second", StreamURL: "https://second"},
	}

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = next.(Model)
	if m.discovery.focus != discoveryResultsFocus || m.discovery.cursor != 0 {
		t.Fatalf("Down from input = focus %v cursor %d, want results/0", m.discovery.focus, m.discovery.cursor)
	}
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = next.(Model)
	if m.discovery.cursor != 1 {
		t.Fatalf("Down in results = cursor %d, want 1", m.discovery.cursor)
	}
	next, _ = m.Update(discoveryKey('k'))
	m = next.(Model)
	if m.discovery.cursor != 0 {
		t.Fatalf("k in results = cursor %d, want 0", m.discovery.cursor)
	}
	next, _ = m.Update(discoveryKey('j'))
	m = next.(Model)
	if m.discovery.cursor != 1 {
		t.Fatalf("j in results = cursor %d, want 1", m.discovery.cursor)
	}
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = next.(Model)
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = next.(Model)
	if m.discovery.focus != discoveryInputFocus || m.discovery.cursor != 0 {
		t.Fatalf("Up from first result = focus %v cursor %d, want input/0", m.discovery.focus, m.discovery.cursor)
	}

	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = next.(Model)
	if m.discovery.focus != discoveryResultsFocus {
		t.Fatal("Tab did not provide alternate input-to-results focus switch")
	}
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	m = next.(Model)
	if m.discovery.focus != discoveryInputFocus {
		t.Fatal("Shift+Tab did not provide alternate results-to-input focus switch")
	}

	// `j` remains normal input while the text field is active.
	next, _ = m.Update(discoveryKey('j'))
	m = next.(Model)
	if got := m.discovery.input.Value(); got != "j" {
		t.Fatalf("j in input = %q, want query text", got)
	}
}

func TestDiscoverySavePersistsAndPromotesActivePreview(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := openDiscovery(t, discoveryFixture(&fakeDiscoveryClient{}))
	m.discovery.results = []radiobrowser.Station{{UUID: "save-id", Name: "Save me", StreamURL: "https://save-me"}}
	m.previewStation = &config.Station{Name: "Save me", URL: "https://save-me", Kind: config.KindStream}
	m.previewUUID = "save-id"
	m.playing = true
	m.playingIdx = -1
	m.discovery = m.discovery.setFocus(discoveryResultsFocus)

	next, _ := m.Update(discoveryKey('s'))
	m = next.(Model)
	if len(m.cfg.Stations) != 4 || m.cfg.Stations[3].URL != "https://save-me" {
		t.Fatalf("saved stations = %#v", m.cfg.Stations)
	}
	if out := m.View(); !strings.Contains(out, "ok: Saved Save me to your library.") {
		t.Fatalf("save confirmation is not visible in discovery modal:\n%s", out)
	}
	if m.previewStation != nil || m.playingIdx != 3 {
		t.Fatalf("preview was not promoted: preview=%#v idx=%d", m.previewStation, m.playingIdx)
	}
	path := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "lofi-player", "config.yaml")
	data, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(data), "https://save-me") {
		t.Fatalf("saved config = %q, %v", data, err)
	}

	// Saving the same public result again must keep the URL unique.
	next, _ = m.Update(discoveryKey('s'))
	m = next.(Model)
	if len(m.cfg.Stations) != 4 || m.discovery.notice != "Already in your library: Save me" || m.discovery.noticeKind != ToastInfo {
		t.Fatalf("duplicate save state: stations=%d notice=%q kind=%v", len(m.cfg.Stations), m.discovery.notice, m.discovery.noticeKind)
	}
	if out := m.View(); !strings.Contains(out, "note: Already in your library: Save me") {
		t.Fatalf("duplicate confirmation is not visible in discovery modal:\n%s", out)
	}

	m.discovery = m.discovery.setFocus(discoveryInputFocus)
	next, _ = m.Update(discoveryKey('x'))
	m = next.(Model)
	if m.discovery.notice != "" {
		t.Fatalf("notice was not cleared after changing query: %q", m.discovery.notice)
	}
}

func TestDiscoverySaveFailureRendersInlineError(t *testing.T) {
	configRoot := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(configRoot, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", configRoot)

	m := openDiscovery(t, discoveryFixture(&fakeDiscoveryClient{}))
	m.discovery.results = []radiobrowser.Station{{Name: "Cannot save", StreamURL: "https://cannot-save"}}
	m.discovery = m.discovery.setFocus(discoveryResultsFocus)
	next, _ := m.Update(discoveryKey('s'))
	m = next.(Model)

	if len(m.cfg.Stations) != 3 || m.discovery.noticeKind != ToastError {
		t.Fatalf("failed save mutated library or missed error notice: stations=%d kind=%v", len(m.cfg.Stations), m.discovery.noticeKind)
	}
	if out := m.View(); !strings.Contains(out, "error: Could not save this station.") {
		t.Fatalf("save failure is not visible in discovery modal:\n%s", out)
	}
}

func TestDiscoverySearchErrorRendersInModal(t *testing.T) {
	m := openDiscovery(t, discoveryFixture(&fakeDiscoveryClient{}))
	m = typeDiscoveryQuery(t, m, "jazz")
	next, _ := m.Update(discoverySearchResultMsg{session: m.discovery.session, seq: m.discovery.searchSeq, err: errors.New("offline")})
	m = next.(Model)
	if out := m.View(); !strings.Contains(out, "Radio Browser is unavailable") || strings.Contains(out, "offline") {
		t.Fatalf("search error leaked internal details: %s", out)
	}
}

func TestDiscoverySessionRejectsResponseFromClosedModal(t *testing.T) {
	m := openDiscovery(t, discoveryFixture(&fakeDiscoveryClient{}))
	m = typeDiscoveryQuery(t, m, "jazz")
	oldSession, oldSeq := m.discovery.session, m.discovery.searchSeq
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = next.(Model)
	m = openDiscovery(t, m)
	m = typeDiscoveryQuery(t, m, "rock")

	next, _ = m.Update(discoverySearchResultMsg{
		session: oldSession,
		seq:     oldSeq,
		results: radiobrowser.Results{Stations: []radiobrowser.Station{{Name: "Old jazz", StreamURL: "https://old"}}},
	})
	m = next.(Model)
	if len(m.discovery.results) != 0 {
		t.Fatalf("closed-session result populated a new modal: %#v", m.discovery.results)
	}
}

func TestPreviewCanSwitchToSelectedLibraryStation(t *testing.T) {
	m := fixture()
	preview := config.Station{Name: "Preview", URL: "https://preview", Kind: config.KindStream}
	m.previewStation = &preview
	m.playing = true
	m.playingIdx = -1
	m.cursor = 1 // Station B is distinct from preview.

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}})
	m = next.(Model)
	if m.previewStation != nil || m.playingIdx != 1 || !m.loading {
		t.Fatalf("selected station did not replace preview: preview=%#v idx=%d loading=%v", m.previewStation, m.playingIdx, m.loading)
	}
}

func TestPreviewIgnoresTerminalEventsFromPreviousRequest(t *testing.T) {
	m := fixture()
	preview := config.Station{Name: "Preview", URL: "https://preview", Kind: config.KindStream}
	m.previewStation = &preview
	m.playing = true
	m.loading = true
	m.playRequestID = 2

	// A queued lifecycle event from request 1 must not erase preview from
	// request 2, irrespective of whether request 2 has reached start-file.
	next, _ := m.Update(EOFMsg{RequestID: 1})
	m = next.(Model)
	if m.previewStation == nil {
		t.Fatal("old EOF cleared preview before start-file")
	}
	next, _ = m.Update(PlaybackErrorMsg{Err: errors.New("old source"), RequestID: 1})
	m = next.(Model)
	if m.previewStation == nil {
		t.Fatal("old error cleared preview before start-file")
	}

	next, _ = m.Update(PlaybackLoadStartedMsg{RequestID: 1})
	m = next.(Model)
	if m.previewStation == nil {
		t.Fatal("old start-file cleared current preview")
	}
	next, _ = m.Update(PlaybackErrorMsg{Err: errors.New("new source"), RequestID: 2})
	m = next.(Model)
	if m.previewStation != nil || m.playing {
		t.Fatalf("new-source error did not stop preview: preview=%#v playing=%v", m.previewStation, m.playing)
	}
}

func TestPreviewPlayErrorForCurrentRequestStopsPlayback(t *testing.T) {
	m := fixture()
	preview := config.Station{Name: "Preview", URL: "https://preview", Kind: config.KindStream}
	m.previewStation = &preview
	m.playing = true
	m.loading = true
	m.playRequestID = 2

	next, _ := m.Update(PlaybackErrorMsg{Err: errors.New("IPC failed"), RequestID: 2})
	m = next.(Model)
	if m.previewStation != nil || m.toast == nil || m.toast.Message != "Could not play this station. Try another one." {
		t.Fatalf("current-request failure was ignored: preview=%#v toast=%#v", m.previewStation, m.toast)
	}
}

func TestPreviewIgnoresOldMetadataAndStreamState(t *testing.T) {
	m := fixture()
	preview := config.Station{Name: "Preview", URL: "https://preview", Kind: config.KindStream}
	m.previewStation = &preview
	m.playRequestID = 2
	m.currentTrack = Track{Title: "new track"}
	m.streamInfo = audio.StreamInfoChanged{Codec: "opus"}
	m.cacheSeconds = 5

	for _, msg := range []tea.Msg{
		MetadataChangedMsg{Title: "old track", RequestID: 1},
		StreamInfoChangedMsg{Codec: "mp3", RequestID: 1},
		CacheStateChangedMsg{Seconds: 1, RequestID: 1},
		BufferingChangedMsg{Stalled: true, RequestID: 1},
	} {
		next, _ := m.Update(msg)
		m = next.(Model)
	}
	if m.currentTrack.Title != "new track" || m.streamInfo.Codec != "opus" || m.cacheSeconds != 5 || m.bufferingStalled {
		t.Fatalf("old stream state leaked into preview: track=%#v info=%#v cache=%v stalled=%v", m.currentTrack, m.streamInfo, m.cacheSeconds, m.bufferingStalled)
	}
}
