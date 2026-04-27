// Command lofi-player is a TUI player for lofi/chillhop/ambient internet
// radio streams. See the project plan in plans/lofi-player-plan.md.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/iRootPro/lofi-player/internal/audio"
	"github.com/iRootPro/lofi-player/internal/config"
	"github.com/iRootPro/lofi-player/internal/state"
	"github.com/iRootPro/lofi-player/internal/tui"
)

const mpvStartupTimeout = 5 * time.Second

// version is overridden at build time via -ldflags "-X main.version=...".
// Goreleaser injects the tag; `go install` and ad-hoc builds keep "dev".
var version = "dev"

func main() {
	var (
		statusline  bool
		showVersion bool
	)
	flag.BoolVar(&statusline, "statusline", false, "print one status-line snapshot to stdout and exit (no TUI)")
	flag.BoolVar(&showVersion, "version", false, "print version and exit")
	flag.BoolVar(&showVersion, "v", false, "print version and exit (shorthand)")
	flag.Parse()

	if showVersion {
		fmt.Println("lofi-player", version)
		return
	}

	if statusline {
		if err := runStatusline(); err != nil {
			fmt.Fprintf(os.Stderr, "lofi-player: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "lofi-player: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	if _, err := exec.LookPath("mpv"); err != nil {
		fmt.Fprint(os.Stderr, "\n", tui.RenderMissingDependency(
			"mpv", "audio engine, required for all playback",
			[]tui.InstallCmd{
				{Platform: "macOS", Cmd: "brew install mpv"},
				{Platform: "Linux", Cmd: "apt install mpv  ·  pacman -S mpv  ·  dnf install mpv"},
			}), "\n")
		os.Exit(1)
	}

	youtubeWarning := preflightYouTube(cfg.Stations)

	st := state.Load()
	opts := tui.Options{
		Theme:           st.Theme,
		Volume:          st.Volume,
		AutoplayStation: stationIndex(cfg.Stations, st.LastStationName),
		ShowStreamInfo:  st.ShowStreamInfo,
		StartupWarning:  youtubeWarning,
		YouTubeReady:    youtubeWarning == "",
	}
	effectiveVolume := cfg.Volume
	if opts.Volume > 0 {
		effectiveVolume = opts.Volume
	}

	ctx, cancel := context.WithTimeout(context.Background(), mpvStartupTimeout)
	defer cancel()
	player, err := audio.NewPlayer(ctx, audio.Options{
		InitialVolume: effectiveVolume,
	})
	if err != nil {
		return fmt.Errorf("starting mpv: %w", err)
	}
	defer player.Close()

	mixer := audio.NewAmbientMixer()
	if err := mixer.Init(); err != nil {
		// Init failure is non-fatal: the main station keeps working,
		// the mixer modal just renders its rows as 'unavailable'.
		fmt.Fprintf(os.Stderr, "lofi-player: ambient mixer init failed: %v\n", err)
	} else {
		for id, v := range st.Ambient {
			_ = mixer.SetVolume(id, v)
		}
	}
	defer mixer.Close()

	opts.SaveAmbient = func(snap map[string]int) error {
		current := state.Load()
		current.Ambient = snap
		return state.Save(current)
	}

	p := tea.NewProgram(tui.NewModel(cfg, player, mixer, opts), tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return err
	}

	if m, ok := finalModel.(tui.Model); ok {
		// Persistence is best-effort — write failure logs to stderr (now
		// that the alt-screen is restored) and never aborts shutdown.
		showInfo := m.ShowStreamInfo()
		next := &state.State{
			Theme:           m.ThemeName(),
			Volume:          m.Volume(),
			LastStationName: m.LastStationName(),
			Ambient:         mixer.Volumes(),
			ShowStreamInfo:  &showInfo,
		}
		if err := state.Save(next); err != nil {
			fmt.Fprintf(os.Stderr, "lofi-player: state save failed: %v\n", err)
		}
	}
	return nil
}

// runStatusline produces a single colored line and exits. Designed for
// tmux's status-right and similar integrations: configure tmux to run
// `lofi-player --statusline` periodically and embed the output.
func runStatusline() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	st := state.Load()

	themeName := cfg.Theme
	if st.Theme != "" {
		themeName = st.Theme
	}
	volume := cfg.Volume
	if st.Volume > 0 {
		volume = st.Volume
	}

	fmt.Println(tui.StatusLine(themeName, st.LastStationName, fmt.Sprintf("%d%%", volume), volume))
	return nil
}

// preflightYouTube checks whether yt-dlp is on $PATH when the config
// contains YouTube stations. Returns the empty string when YouTube is
// either unused or fully wired; otherwise returns a one-line warning
// suitable for an in-app startup toast. The TUI then renders YouTube
// stations as unavailable and refuses to play them, but the rest of
// the app keeps working — losing one source kind shouldn't sink the
// whole player.
func preflightYouTube(stations []config.Station) string {
	hasYouTube := false
	for _, s := range stations {
		if s.IsYouTube() {
			hasYouTube = true
			break
		}
	}
	if !hasYouTube {
		return ""
	}
	if _, err := exec.LookPath("yt-dlp"); err != nil {
		return "yt-dlp not found — YouTube stations unavailable. install: brew install yt-dlp / pip install yt-dlp"
	}
	return ""
}

// stationIndex returns the index in stations matching name, or -1 if
// not found. Used to map the persisted LastStationName back to a cursor
// position so renaming a station doesn't break autoplay.
func stationIndex(stations []config.Station, name string) int {
	if name == "" {
		return -1
	}
	for i, s := range stations {
		if s.Name == name {
			return i
		}
	}
	return -1
}
