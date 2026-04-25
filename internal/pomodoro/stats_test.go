package pomodoro

import (
	"encoding/json"
	"testing"
	"time"
)

func mustParseDate(t *testing.T, s string) time.Time {
	t.Helper()
	tm, err := time.Parse("2006-01-02 15:04:05", s)
	if err != nil {
		t.Fatalf("bad date %q: %v", s, err)
	}
	return tm
}

func TestStatsJSON_RoundTrip(t *testing.T) {
	original := Stats{
		ListenedToday:  90 * time.Minute,
		LastListenDate: "2026-04-25",
		Streak:         7,
		LastFocusDate:  "2026-04-25",
	}
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	// Wire format must use seconds, not nanoseconds, for human readability.
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("re-decode: %v", err)
	}
	if sec, ok := raw["listened_today_seconds"].(float64); !ok || int64(sec) != int64(original.ListenedToday/time.Second) {
		t.Errorf("listened_today_seconds in JSON = %v, want %d", raw["listened_today_seconds"], int64(original.ListenedToday/time.Second))
	}

	var got Stats
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got != original {
		t.Errorf("round-trip mismatch\n got:  %+v\n want: %+v", got, original)
	}
}

func TestTickListening_AccumulatesSameDay(t *testing.T) {
	now := mustParseDate(t, "2026-04-25 10:00:00")
	s := TickListening(Stats{}, now, time.Second)
	s = TickListening(s, now.Add(time.Second), time.Second)
	s = TickListening(s, now.Add(2*time.Second), time.Second)
	if s.ListenedToday != 3*time.Second {
		t.Errorf("ListenedToday = %v, want 3s", s.ListenedToday)
	}
	if s.LastListenDate != "2026-04-25" {
		t.Errorf("LastListenDate = %q, want 2026-04-25", s.LastListenDate)
	}
}

func TestTickListening_ResetsOnMidnightRollover(t *testing.T) {
	day1 := mustParseDate(t, "2026-04-25 23:59:59")
	day2 := mustParseDate(t, "2026-04-26 00:00:01")
	s := TickListening(Stats{}, day1, 30*time.Minute)
	if s.ListenedToday != 30*time.Minute {
		t.Fatalf("day1: ListenedToday = %v, want 30m", s.ListenedToday)
	}
	s = TickListening(s, day2, time.Second)
	if s.ListenedToday != time.Second {
		t.Errorf("day2: ListenedToday = %v, want 1s after rollover", s.ListenedToday)
	}
	if s.LastListenDate != "2026-04-26" {
		t.Errorf("LastListenDate = %q, want 2026-04-26", s.LastListenDate)
	}
}

func TestRegisterFocusStart_FirstEverIsStreak1(t *testing.T) {
	now := mustParseDate(t, "2026-04-25 10:00:00")
	s := RegisterFocusStart(Stats{}, now)
	if s.Streak != 1 {
		t.Errorf("Streak = %d, want 1", s.Streak)
	}
	if s.LastFocusDate != "2026-04-25" {
		t.Errorf("LastFocusDate = %q, want 2026-04-25", s.LastFocusDate)
	}
}

func TestRegisterFocusStart_SameDayIsNoOp(t *testing.T) {
	now := mustParseDate(t, "2026-04-25 10:00:00")
	s := RegisterFocusStart(Stats{}, now)
	s = RegisterFocusStart(s, now.Add(2*time.Hour))
	if s.Streak != 1 {
		t.Errorf("Streak = %d after second start same day, want still 1", s.Streak)
	}
}

func TestRegisterFocusStart_ConsecutiveDaysExtendStreak(t *testing.T) {
	day1 := mustParseDate(t, "2026-04-25 10:00:00")
	day2 := mustParseDate(t, "2026-04-26 10:00:00")
	day3 := mustParseDate(t, "2026-04-27 10:00:00")

	s := RegisterFocusStart(Stats{}, day1)
	s = RegisterFocusStart(s, day2)
	s = RegisterFocusStart(s, day3)

	if s.Streak != 3 {
		t.Errorf("Streak after 3 consecutive days = %d, want 3", s.Streak)
	}
}

func TestRegisterFocusStart_GapResetsStreak(t *testing.T) {
	day1 := mustParseDate(t, "2026-04-25 10:00:00")
	day3 := mustParseDate(t, "2026-04-27 10:00:00") // skipped 26th

	s := RegisterFocusStart(Stats{}, day1)
	s.Streak = 5 // pretend a longer history
	s = RegisterFocusStart(s, day3)

	if s.Streak != 1 {
		t.Errorf("Streak after 1-day gap = %d, want 1 (reset)", s.Streak)
	}
}
