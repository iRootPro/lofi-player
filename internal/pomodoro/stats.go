package pomodoro

import (
	"encoding/json"
	"time"
)

const dateFormat = "2006-01-02"

// Stats is the per-user persisted history of pomodoro and listening
// activity. It serializes as compact JSON so the state file stays
// human-readable.
type Stats struct {
	// ListenedToday is total focused-listening time since the day rolled over.
	ListenedToday time.Duration
	// LastListenDate is the YYYY-MM-DD of the last TickListening call;
	// used to detect midnight rollover.
	LastListenDate string
	// Streak is the count of consecutive days with at least one focus
	// session, including today.
	Streak int
	// LastFocusDate is the YYYY-MM-DD of the most recent focus session
	// start, used by the streak update logic.
	LastFocusDate string
}

// statsJSON is the on-disk representation. ListenedToday is stored as
// integer seconds rather than nanoseconds for readability.
type statsJSON struct {
	ListenedTodaySec int64  `json:"listened_today_seconds,omitempty"`
	LastListenDate   string `json:"last_listen_date,omitempty"`
	Streak           int    `json:"streak,omitempty"`
	LastFocusDate    string `json:"last_focus_date,omitempty"`
}

// MarshalJSON encodes Stats with ListenedToday as integer seconds.
func (s Stats) MarshalJSON() ([]byte, error) {
	return json.Marshal(statsJSON{
		ListenedTodaySec: int64(s.ListenedToday / time.Second),
		LastListenDate:   s.LastListenDate,
		Streak:           s.Streak,
		LastFocusDate:    s.LastFocusDate,
	})
}

// UnmarshalJSON parses the integer-seconds shape back into time.Duration.
func (s *Stats) UnmarshalJSON(data []byte) error {
	var j statsJSON
	if err := json.Unmarshal(data, &j); err != nil {
		return err
	}
	s.ListenedToday = time.Duration(j.ListenedTodaySec) * time.Second
	s.LastListenDate = j.LastListenDate
	s.Streak = j.Streak
	s.LastFocusDate = j.LastFocusDate
	return nil
}

// TickListening increments ListenedToday by dt. Crossing midnight
// resets the running total and stamps the new date.
func TickListening(s Stats, now time.Time, dt time.Duration) Stats {
	today := now.Format(dateFormat)
	if today != s.LastListenDate {
		s.ListenedToday = 0
		s.LastListenDate = today
	}
	s.ListenedToday += dt
	return s
}

// RegisterFocusStart bumps the streak when appropriate. Multiple starts
// on the same day are no-ops; a start on the day after LastFocusDate
// extends the streak by 1; a longer gap resets it to 1.
func RegisterFocusStart(s Stats, now time.Time) Stats {
	today := now.Format(dateFormat)
	if s.LastFocusDate == today {
		return s
	}
	yesterday := now.AddDate(0, 0, -1).Format(dateFormat)
	if s.LastFocusDate == yesterday {
		s.Streak++
	} else {
		s.Streak = 1
	}
	s.LastFocusDate = today
	return s
}
