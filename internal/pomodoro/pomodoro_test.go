package pomodoro

import (
	"testing"
	"time"
)

var testCfg = Config{
	FocusDuration:        25 * time.Minute,
	ShortBreakDuration:   5 * time.Minute,
	LongBreakDuration:    15 * time.Minute,
	RoundsUntilLongBreak: 4,
}

func TestNew_IsIdle(t *testing.T) {
	s := New()
	if s.Phase != PhaseIdle {
		t.Errorf("New().Phase = %v, want PhaseIdle", s.Phase)
	}
	if !s.EndsAt.IsZero() {
		t.Errorf("New().EndsAt = %v, want zero", s.EndsAt)
	}
}

func TestStart_BeginsFocus(t *testing.T) {
	now := time.Now()
	s, tr := Start(New(), now, testCfg)
	if tr != StartedFocus {
		t.Errorf("transition = %v, want StartedFocus", tr)
	}
	if s.Phase != PhaseFocus {
		t.Errorf("Phase = %v, want PhaseFocus", s.Phase)
	}
	want := now.Add(testCfg.FocusDuration)
	if !s.EndsAt.Equal(want) {
		t.Errorf("EndsAt = %v, want %v", s.EndsAt, want)
	}
}

func TestStop_ReturnsToIdle(t *testing.T) {
	now := time.Now()
	s, _ := Start(New(), now, testCfg)
	s, tr := Stop(s)
	if tr != Stopped {
		t.Errorf("transition = %v, want Stopped", tr)
	}
	if s.Phase != PhaseIdle {
		t.Errorf("Phase = %v, want PhaseIdle", s.Phase)
	}
}

func TestStop_FromIdleIsNoChange(t *testing.T) {
	s, tr := Stop(New())
	if tr != NoChange {
		t.Errorf("transition = %v, want NoChange", tr)
	}
	if s.Phase != PhaseIdle {
		t.Errorf("Phase = %v, want PhaseIdle", s.Phase)
	}
}

func TestTick_BeforeEndsAtIsNoChange(t *testing.T) {
	now := time.Now()
	s, _ := Start(New(), now, testCfg)
	got, tr := Tick(s, now.Add(time.Minute), testCfg)
	if tr != NoChange {
		t.Errorf("transition = %v, want NoChange", tr)
	}
	if got.Phase != PhaseFocus {
		t.Errorf("Phase = %v, want PhaseFocus", got.Phase)
	}
}

func TestTick_FocusEndsToShortBreak(t *testing.T) {
	now := time.Now()
	s, _ := Start(New(), now, testCfg)
	got, tr := Tick(s, s.EndsAt, testCfg)
	if tr != StartedShortBreak {
		t.Errorf("transition = %v, want StartedShortBreak", tr)
	}
	if got.Phase != PhaseShortBreak {
		t.Errorf("Phase = %v, want PhaseShortBreak", got.Phase)
	}
	if got.Round != 1 {
		t.Errorf("Round = %d, want 1", got.Round)
	}
}

func TestTick_FocusToLongBreakEveryNthRound(t *testing.T) {
	now := time.Now()
	s := New()
	// Walk through 4 full focus + break cycles and verify the 4th break is long.
	for i := 1; i <= 4; i++ {
		var tr Transition
		s, _ = Start(s, now, testCfg)
		s, tr = Tick(s, s.EndsAt, testCfg)
		if i < 4 {
			if tr != StartedShortBreak {
				t.Errorf("round %d: transition = %v, want StartedShortBreak", i, tr)
			}
		} else if tr != StartedLongBreak {
			t.Errorf("round %d: transition = %v, want StartedLongBreak", i, tr)
		}
		// Advance through the break.
		s, _ = Tick(s, s.EndsAt, testCfg)
	}
}

func TestTick_FarFutureAdvancesOnePhase(t *testing.T) {
	// Simulates a laptop suspend: "now" jumps far past EndsAt. The
	// machine must advance exactly one phase, not many.
	now := time.Now()
	s, _ := Start(New(), now, testCfg)
	farFuture := s.EndsAt.Add(2 * time.Hour)
	got, tr := Tick(s, farFuture, testCfg)
	if tr != StartedShortBreak {
		t.Errorf("transition = %v, want StartedShortBreak", tr)
	}
	if got.Phase != PhaseShortBreak {
		t.Errorf("Phase = %v, want PhaseShortBreak", got.Phase)
	}
	// EndsAt should be set relative to the new "now", not the old EndsAt.
	wantEndsAt := farFuture.Add(testCfg.ShortBreakDuration)
	if !got.EndsAt.Equal(wantEndsAt) {
		t.Errorf("EndsAt = %v, want %v", got.EndsAt, wantEndsAt)
	}
}

func TestTick_BreakEndsToFocus(t *testing.T) {
	now := time.Now()
	s, _ := Start(New(), now, testCfg)
	s, _ = Tick(s, s.EndsAt, testCfg) // focus → short break
	breakEndsAt := s.EndsAt
	got, tr := Tick(s, breakEndsAt, testCfg)
	if tr != StartedFocus {
		t.Errorf("transition = %v, want StartedFocus", tr)
	}
	if got.Phase != PhaseFocus {
		t.Errorf("Phase = %v, want PhaseFocus", got.Phase)
	}
}

func TestRemaining(t *testing.T) {
	now := time.Now()
	if r := Remaining(New(), now); r != 0 {
		t.Errorf("idle session: Remaining = %v, want 0", r)
	}

	s, _ := Start(New(), now, testCfg)
	if r := Remaining(s, now); r != testCfg.FocusDuration {
		t.Errorf("just-started focus: Remaining = %v, want %v", r, testCfg.FocusDuration)
	}
	if r := Remaining(s, now.Add(10*time.Minute)); r != 15*time.Minute {
		t.Errorf("10 min in: Remaining = %v, want 15m", r)
	}
	if r := Remaining(s, s.EndsAt.Add(time.Minute)); r != 0 {
		t.Errorf("past EndsAt: Remaining = %v, want 0", r)
	}
}

func TestPhaseString(t *testing.T) {
	tests := []struct {
		phase Phase
		want  string
	}{
		{PhaseIdle, "idle"},
		{PhaseFocus, "focus"},
		{PhaseShortBreak, "short break"},
		{PhaseLongBreak, "long break"},
	}
	for _, tc := range tests {
		if got := tc.phase.String(); got != tc.want {
			t.Errorf("%d.String() = %q, want %q", tc.phase, got, tc.want)
		}
	}
}
