// Command lofi-player is a TUI player for lofi/chillhop/ambient internet
// radio streams. See the project plan in plans/lofi-player-plan.md.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/iRootPro/lofi-player/internal/audio"
	"github.com/iRootPro/lofi-player/internal/config"
	"github.com/iRootPro/lofi-player/internal/doctor"
	listenhistory "github.com/iRootPro/lofi-player/internal/history"
	"github.com/iRootPro/lofi-player/internal/i18n"
	"github.com/iRootPro/lofi-player/internal/radiobrowser"
	sharepkg "github.com/iRootPro/lofi-player/internal/share"
	"github.com/iRootPro/lofi-player/internal/state"
	"github.com/iRootPro/lofi-player/internal/tui"
	"github.com/iRootPro/lofi-player/internal/updatecheck"
)

const mpvStartupTimeout = 5 * time.Second

// version is overridden at build time via -ldflags "-X main.version=...".
// Goreleaser injects the tag; `go install` and ad-hoc builds keep "dev".
var version = "dev"

func main() {
	locale := i18n.Resolve(config.LanguageChoice())
	if len(os.Args) >= 2 && os.Args[1] == "doctor" {
		if len(os.Args) > 2 {
			fmt.Fprintln(os.Stderr, i18n.T(locale, "usage_doctor"))
			os.Exit(2)
		}
		if err := runDoctor(os.Stdout, locale); err != nil {
			fmt.Fprintf(os.Stderr, "lofi-player: %v\n", err)
			os.Exit(1)
		}
		return
	}

	var (
		statusline    bool
		showVersion   bool
		checkUpdate   bool
		exportAll     bool
		exportStation string
		importPath    string
	)
	flag.BoolVar(&statusline, "statusline", false, i18n.T(locale, "flag_statusline"))
	flag.BoolVar(&showVersion, "version", false, i18n.T(locale, "flag_version"))
	flag.BoolVar(&showVersion, "v", false, i18n.T(locale, "flag_version_short"))
	flag.BoolVar(&checkUpdate, "check-update", false, i18n.T(locale, "flag_update"))
	flag.BoolVar(&exportAll, "export-all", false, i18n.T(locale, "flag_export_all"))
	flag.StringVar(&exportStation, "export-station", "", i18n.T(locale, "flag_export_station"))
	flag.StringVar(&importPath, "import", "", i18n.T(locale, "flag_import"))
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "%s:\n", i18n.T(locale, "usage"))
		flag.PrintDefaults()
	}
	flag.Parse()

	if showVersion {
		fmt.Println("lofi-player", version)
		return
	}
	if checkUpdate {
		if err := runCheckUpdate(os.Stdout, locale); err != nil {
			fmt.Fprintf(os.Stderr, "lofi-player: %s\n", i18n.T(locale, "update_check_failed"))
			os.Exit(1)
		}
		return
	}

	if statusline {
		if err := runStatusline(); err != nil {
			fmt.Fprintf(os.Stderr, "lofi-player: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if exportAll || exportStation != "" {
		if err := runExport(exportAll, exportStation, locale); err != nil {
			fmt.Fprintf(os.Stderr, "lofi-player: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if importPath != "" {
		if err := runImport(importPath, locale); err != nil {
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
	locale := i18n.Resolve(cfg.Language)

	if _, err := exec.LookPath("mpv"); err != nil {
		fmt.Fprint(os.Stderr, "\n", tui.RenderMissingDependency(
			locale, "mpv", i18n.T(locale, "mpv_role"),
			[]tui.InstallCmd{
				{Platform: "macOS", Cmd: "brew install mpv"},
				{Platform: "Linux", Cmd: "apt install mpv  ·  pacman -S mpv  ·  dnf install mpv"},
			}), "\n")
		os.Exit(1)
	}

	youtubeWarning := preflightYouTube(locale, cfg.Stations)

	st := state.Load()
	listeningHistory := listenhistory.Load()
	if recovered, changed := listeningHistory.Recover(); changed {
		listeningHistory = recovered
		// A prior process may have ended unexpectedly. Persist its bounded
		// checkpoint now, but do not prevent playback if the state directory
		// is temporarily unavailable.
		if err := listenhistory.Save(listeningHistory); err != nil {
			fmt.Fprintf(os.Stderr, "lofi-player: history recovery save failed: %v\n", err)
		}
	}
	opts := tui.Options{
		AppVersion:       version,
		Theme:            st.Theme,
		Volume:           st.Volume,
		AutoplayStation:  stationIndex(cfg.Stations, st.LastStationName),
		ShowStreamInfo:   st.ShowStreamInfo,
		StartupWarning:   youtubeWarning,
		YouTubeReady:     youtubeWarning == "",
		Discovery:        radiobrowser.New(radiobrowser.Options{}),
		ListeningHistory: listeningHistory,
		SaveListeningHistory: func(h listenhistory.History) error {
			return listenhistory.Save(h)
		},
		CheckForUpdate: func() (updatecheck.Result, error) {
			return updatecheck.New(version).Check(context.Background())
		},
	}
	effectiveVolume := cfg.Volume
	if opts.Volume > 0 {
		effectiveVolume = opts.Volume
	}

	ctx, cancel := context.WithTimeout(context.Background(), mpvStartupTimeout)
	defer cancel()
	player, err := audio.NewPlayer(ctx, audio.Options{
		InitialVolume:        effectiveVolume,
		BufferSeconds:        cfg.BufferSeconds,
		InitialBufferSeconds: cfg.InitialBufferSeconds,
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
	opts.SaveTheme = func(name string) error {
		current := state.Load()
		current.Theme = name
		return state.Save(current)
	}

	p := tea.NewProgram(tui.NewModel(cfg, player, mixer, opts), tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return err
	}

	if m, ok := finalModel.(tui.Model); ok {
		if err := listenhistory.Save(m.FinalListeningHistory(time.Now())); err != nil {
			fmt.Fprintf(os.Stderr, "lofi-player: history save failed: %v\n", err)
		}
		// Persistence is best-effort — write failure logs to stderr (now
		// that the alt-screen is restored) and never aborts shutdown.
		showInfo := m.ShowStreamInfo()
		lastStationName := m.LastStationName()
		if lastStationName == "" {
			// A Radio Browser preview is intentionally not persisted. Keep
			// the last saved station selected for the next launch instead.
			lastStationName = st.LastStationName
		}
		next := &state.State{
			Theme:           m.ThemeName(),
			Volume:          m.Volume(),
			LastStationName: lastStationName,
			Ambient:         mixer.Volumes(),
			ShowStreamInfo:  &showInfo,
		}
		if err := state.Save(next); err != nil {
			fmt.Fprintf(os.Stderr, "lofi-player: state save failed: %v\n", err)
		}
	}
	return nil
}

func runCheckUpdate(out io.Writer, locale i18n.Locale) error {
	result, err := updatecheck.New(version).CheckNow(context.Background())
	if errors.Is(err, updatecheck.ErrNonReleaseBuild) {
		_, writeErr := fmt.Fprintln(out, i18n.T(locale, "non_release"))
		return writeErr
	}
	if err != nil {
		return err
	}
	if result.Available {
		_, err = fmt.Fprintln(out, i18n.T(locale, "available_cli", result.LatestVersion, result.ReleaseURL))
		return err
	}
	_, err = fmt.Fprintln(out, i18n.T(locale, "up_to_date", version))
	return err
}

// runDoctor prints a read-only diagnostic report without loading config,
// starting mpv, or opening the TUI. Its timeout keeps it useful on blocked
// DNS/VPN networks while allowing each check to observe cancellation.
func runDoctor(out io.Writer, locale i18n.Locale) error {
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()
	report := doctor.Collect(ctx, doctor.Options{ApplicationVersion: version, Locale: locale})
	_, err := fmt.Fprintln(out, report.String())
	return err
}

func runExport(all bool, stationName string, locale i18n.Locale) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	var snippet string
	if all {
		snippet, err = sharepkg.MarshalStations(cfg.Stations)
	} else {
		st, ok := findStationByName(cfg.Stations, stationName)
		if !ok {
			return fmt.Errorf(i18n.T(locale, "station_not_found"), stationName)
		}
		snippet, err = sharepkg.MarshalStation(st)
	}
	if err != nil {
		return err
	}
	fmt.Print(snippet)
	return nil
}

func runImport(path string, locale i18n.Locale) error {
	var data []byte
	var err error
	if path == "-" {
		data, err = io.ReadAll(os.Stdin)
	} else {
		data, err = os.ReadFile(path)
	}
	if err != nil {
		return err
	}
	stations, err := sharepkg.Parse(string(data))
	if err != nil {
		return err
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	newStations, skipped := newStationsOnly(cfg.Stations, stations)
	if len(newStations) == 0 {
		fmt.Fprintf(os.Stderr, i18n.T(locale, "all_exist_cli"), skipped)
		return nil
	}
	cfg.Stations = append(cfg.Stations, newStations...)
	if err := config.Save(cfg); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, i18n.T(locale, "imported_cli"), len(newStations), skipped)
	return nil
}

func findStationByName(stations []config.Station, name string) (config.Station, bool) {
	for _, st := range stations {
		if st.Name == name {
			return st, true
		}
	}
	for _, st := range stations {
		if strings.EqualFold(st.Name, name) {
			return st, true
		}
	}
	return config.Station{}, false
}

func newStationsOnly(existing, incoming []config.Station) ([]config.Station, int) {
	seen := make(map[string]struct{}, len(existing)+len(incoming))
	for _, st := range existing {
		seen[st.URL] = struct{}{}
	}
	out := make([]config.Station, 0, len(incoming))
	skipped := 0
	for _, st := range incoming {
		if _, ok := seen[st.URL]; ok {
			skipped++
			continue
		}
		seen[st.URL] = struct{}{}
		out = append(out, st)
	}
	return out, skipped
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
func preflightYouTube(locale i18n.Locale, stations []config.Station) string {
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
		return i18n.T(locale, "youtube_missing") + ". install: brew install yt-dlp / pip install yt-dlp"
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
