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

func main() {
	var statusline bool
	flag.BoolVar(&statusline, "statusline", false, "print one status-line snapshot to stdout and exit (no TUI)")
	flag.Parse()

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
		return fmt.Errorf("mpv not found on $PATH; install with `brew install mpv` (macOS) or `apt install mpv` (Debian/Ubuntu)")
	}

	if err := preflightYouTube(cfg.Stations); err != nil {
		return err
	}

	st := state.Load()
	opts := tui.Options{
		Theme:           st.Theme,
		Volume:          st.Volume,
		AutoplayStation: stationIndex(cfg.Stations, st.LastStationName),
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

	p := tea.NewProgram(tui.NewModel(cfg, player, mixer, opts), tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return err
	}

	if m, ok := finalModel.(tui.Model); ok {
		// Persistence is best-effort — write failure logs to stderr (now
		// that the alt-screen is restored) and never aborts shutdown.
		next := &state.State{
			Theme:           m.ThemeName(),
			Volume:          m.Volume(),
			LastStationName: m.LastStationName(),
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

// preflightYouTube verifies yt-dlp is on $PATH whenever the config
// contains at least one YouTube-kind station. Hard-fails with a clear
// install hint so the user isn't left guessing why YouTube playback
// produces a generic "stream load failed" toast at runtime.
func preflightYouTube(stations []config.Station) error {
	var youtubeNames []string
	for _, s := range stations {
		if s.IsYouTube() {
			youtubeNames = append(youtubeNames, s.Name)
		}
	}
	if len(youtubeNames) == 0 {
		return nil
	}
	if _, err := exec.LookPath("yt-dlp"); err != nil {
		return fmt.Errorf("yt-dlp not found on $PATH but config has %d YouTube station(s) (%v); install with `brew install yt-dlp` (macOS) or `pip install yt-dlp`",
			len(youtubeNames), youtubeNames)
	}
	return nil
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
