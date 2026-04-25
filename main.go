// Command lofi-player is a TUI player for lofi/chillhop/ambient internet
// radio streams. See the project plan in plans/lofi-player-plan.md.
package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/iRootPro/lofi-player/internal/audio"
	"github.com/iRootPro/lofi-player/internal/config"
	"github.com/iRootPro/lofi-player/internal/tui"
)

const mpvStartupTimeout = 5 * time.Second

func main() {
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

	ctx, cancel := context.WithTimeout(context.Background(), mpvStartupTimeout)
	defer cancel()
	player, err := audio.NewPlayer(ctx, audio.Options{
		InitialVolume: cfg.Volume,
	})
	if err != nil {
		return fmt.Errorf("starting mpv: %w", err)
	}
	defer player.Close()

	p := tea.NewProgram(tui.NewModel(cfg, player), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return err
	}
	return nil
}
