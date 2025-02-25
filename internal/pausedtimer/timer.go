package pausedtimer

import (
	"math"
	"time"
)

// PauseTimer is a struct that wraps a time.Ticker and provides additional functionality
// to pause and resume the ticker.
// If the duration is 0, the timer is created in a stopped state.
type PauseTimer struct {
	*time.Ticker
	duration time.Duration
}

// New creates a new pauseTimer with the specified duration.
func New(d time.Duration) *PauseTimer {
	ret := &PauseTimer{duration: d}
	if d != 0 {
		ret.Ticker = time.NewTicker(d)
	} else {
		ret.Ticker = time.NewTicker(math.MaxInt64)
		ret.Reset(0)
	}

	return ret
}

// NewStopped creates a new pauseTimer with the specified duration and stops it immediately.
func NewStopped(d time.Duration) *PauseTimer {
	ret := New(d)
	ret.Stop()

	return ret
}

// Reset sets the timer to the specified duration and starts it.
// If the duration is 0, the timer is stopped.
func (t *PauseTimer) Reset(d time.Duration) {
	t.duration = d
	if t.duration == 0 {
		t.Stop()
	} else {
		t.Ticker.Reset(d)
	}
}

// Resume resumes the timer with its last set duration.
func (t *PauseTimer) Resume() {
	t.Reset(t.GetDuration())
}

// GetDuration returns the current duration of the timer.
func (t *PauseTimer) GetDuration() time.Duration {
	return t.duration
}
