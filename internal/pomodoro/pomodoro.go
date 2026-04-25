// Package pomodoro implements the focus-timer state machine.
//
// The package is intentionally pure: there are no goroutines, no timers,
// no I/O. Callers tick the machine forward by calling Tick with the
// current wall-clock time, and react to the returned Transition. Because
// the machine stores absolute end-times rather than countdowns, it
// survives laptop sleep / suspend without drifting (plan §6 Phase 3
// pitfall).
package pomodoro

import "time"

// Phase identifies what the timer is currently doing.
type Phase int

const (
	// PhaseIdle means no session is running.
	PhaseIdle Phase = iota
	// PhaseFocus is the productive segment of a pomodoro.
	PhaseFocus
	// PhaseShortBreak is the small rest after a focus block.
	PhaseShortBreak
	// PhaseLongBreak is the longer rest after RoundsUntilLongBreak focus blocks.
	PhaseLongBreak
)

// Config controls phase durations and the long-break cadence. Zero
// values are permitted but unhelpful — callers should prefer Defaults.
type Config struct {
	FocusDuration        time.Duration
	ShortBreakDuration   time.Duration
	LongBreakDuration    time.Duration
	RoundsUntilLongBreak int
}

// Defaults returns the canonical pomodoro durations from plan §6
// (25 / 5 / 15 minutes, long break after every 4th focus session).
func Defaults() Config {
	return Config{
		FocusDuration:        25 * time.Minute,
		ShortBreakDuration:   5 * time.Minute,
		LongBreakDuration:    15 * time.Minute,
		RoundsUntilLongBreak: 4,
	}
}

// Session is the current state of the timer.
type Session struct {
	// Phase is what the timer is doing right now.
	Phase Phase
	// EndsAt is the absolute time at which the current Phase ends.
	// Zero when Phase == PhaseIdle.
	EndsAt time.Time
	// Round counts completed focus sessions inside the current cycle.
	// Resets to 0 after a long break.
	Round int
}

// Transition is what changed between two Tick calls (or after Start /
// Stop). NoChange is the common case.
type Transition int

const (
	// NoChange means the call did not advance the state machine.
	NoChange Transition = iota
	// StartedFocus means the machine entered (or re-entered) PhaseFocus.
	StartedFocus
	// StartedShortBreak means the machine entered PhaseShortBreak.
	StartedShortBreak
	// StartedLongBreak means the machine entered PhaseLongBreak.
	StartedLongBreak
	// Stopped means the machine returned to PhaseIdle.
	Stopped
)

// New returns an idle Session.
func New() Session {
	return Session{Phase: PhaseIdle}
}

// Start begins a new focus block, regardless of the current phase. If a
// session was already running, its in-progress phase is discarded.
// Round count carries over so the long-break cadence is preserved.
func Start(s Session, now time.Time, cfg Config) (Session, Transition) {
	s.Phase = PhaseFocus
	s.EndsAt = now.Add(cfg.FocusDuration)
	return s, StartedFocus
}

// Stop returns the machine to idle and clears EndsAt. Round count is
// preserved so a Start later picks up the cadence.
func Stop(s Session) (Session, Transition) {
	if s.Phase == PhaseIdle {
		return s, NoChange
	}
	s.Phase = PhaseIdle
	s.EndsAt = time.Time{}
	return s, Stopped
}

// Tick advances the machine if the current phase has ended. Calling
// Tick before EndsAt is a no-op. After a focus phase ends Tick advances
// to the appropriate break (short or long depending on Round count);
// after a break ends Tick advances back to focus. The Transition return
// describes the change so callers can fire side effects (notifications,
// music auto-pause, stat increments).
//
// Tick advances exactly one phase per call even if "now" is far past
// EndsAt — sleep/suspend doesn't compound transitions. Callers that
// want to catch up multiple phases must loop until NoChange.
func Tick(s Session, now time.Time, cfg Config) (Session, Transition) {
	if s.Phase == PhaseIdle {
		return s, NoChange
	}
	if now.Before(s.EndsAt) {
		return s, NoChange
	}

	switch s.Phase {
	case PhaseFocus:
		s.Round++
		if cfg.RoundsUntilLongBreak > 0 && s.Round%cfg.RoundsUntilLongBreak == 0 {
			s.Phase = PhaseLongBreak
			s.EndsAt = now.Add(cfg.LongBreakDuration)
			return s, StartedLongBreak
		}
		s.Phase = PhaseShortBreak
		s.EndsAt = now.Add(cfg.ShortBreakDuration)
		return s, StartedShortBreak

	case PhaseShortBreak, PhaseLongBreak:
		s.Phase = PhaseFocus
		s.EndsAt = now.Add(cfg.FocusDuration)
		return s, StartedFocus
	}
	return s, NoChange
}

// Remaining returns the time left in the current phase. Returns 0 when
// idle or when EndsAt is in the past (Tick should be called next).
func Remaining(s Session, now time.Time) time.Duration {
	if s.Phase == PhaseIdle || s.EndsAt.IsZero() {
		return 0
	}
	d := s.EndsAt.Sub(now)
	if d < 0 {
		return 0
	}
	return d
}

// String returns the human-readable phase name. Used for toasts and
// notifications.
func (p Phase) String() string {
	switch p {
	case PhaseFocus:
		return "focus"
	case PhaseShortBreak:
		return "short break"
	case PhaseLongBreak:
		return "long break"
	default:
		return "idle"
	}
}
