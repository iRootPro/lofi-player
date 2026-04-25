// Command lofi-player is a TUI player for lofi/chillhop/ambient internet
// radio streams. See the project plan in plans/lofi-player-plan.md.
package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/iRootPro/lofi-player/internal/config"
	"github.com/iRootPro/lofi-player/internal/tui"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "lofi-player: %v\n", err)
		os.Exit(1)
	}

	p := tea.NewProgram(tui.NewModel(cfg), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "lofi-player: %v\n", err)
		os.Exit(1)
	}
}
