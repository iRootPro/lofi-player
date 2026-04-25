package tui

import (
	"math"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/harmonica"
)

const (
	// volSpringFrequency and volSpringDamping settle the volume bar at
	// the new target in roughly 150 ms (plan §5.4) without overshoot.
	volSpringFrequency = 8.0
	volSpringDamping   = 1.0

	// volTickInterval is the animation frame rate. ~60 Hz feels smooth
	// for a small bar without burning real CPU when the spring is at rest.
	volTickInterval = 16 * time.Millisecond

	// volSettledEpsilon is how close the spring must be to its target
	// to count as "settled" — at which point the tick loop stops.
	volSettledEpsilon = 0.5
)

// volTickMsg drives one frame of the volume-bar spring animation.
type volTickMsg struct{}

// volSpring is the singleton spring used for the volume bar.
var volSpring = harmonica.NewSpring(harmonica.FPS(60), volSpringFrequency, volSpringDamping)

// tickVolAnim schedules the next animation frame.
func tickVolAnim() tea.Cmd {
	return tea.Tick(volTickInterval, func(time.Time) tea.Msg {
		return volTickMsg{}
	})
}

// stepVolume samples the spring once and returns the next displayed
// value plus its velocity. The third return value is true once the
// spring has settled close enough to the target that further ticks
// would be visually pointless — the caller should stop the tick loop.
func stepVolume(displayed, velocity, target float64) (next, nextVel float64, settled bool) {
	next, nextVel = volSpring.Update(displayed, velocity, target)
	if math.Abs(next-target) < volSettledEpsilon && math.Abs(nextVel) < volSettledEpsilon {
		return target, 0, true
	}
	return next, nextVel, false
}
