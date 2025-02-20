package cache

import (
	"hash/fnv"
	"math"
	"time"
)

// pauseTimer is a struct that wraps a time.Ticker and provides additional functionality
// to pause and resume the ticker.
// If the duration is 0, the timer is created in a stopped state.
type pauseTimer struct {
	*time.Ticker
	duration time.Duration
}

// newPauseTimer creates a new pauseTimer with the specified duration.
func newPauseTimer(d time.Duration) *pauseTimer {
	ret := &pauseTimer{duration: d}
	if d != 0 {
		ret.Ticker = time.NewTicker(d)
	} else {
		ret.Ticker = time.NewTicker(math.MaxInt64)
		ret.Reset(0)
	}
	return ret
}

// newPauseTimerStopped creates a new pauseTimer with the specified duration and stops it immediately.
func newPauseTimerStopped(d time.Duration) *pauseTimer {
	ret := newPauseTimer(d)
	ret.Stop()
	return ret
}

// Reset sets the timer to the specified duration and starts it.
// If the duration is 0, the timer is stopped.
func (t *pauseTimer) Reset(d time.Duration) {
	t.duration = d
	if t.duration == 0 {
		t.Stop()
	} else {
		t.Ticker.Reset(d)
	}
}

// Resume resumes the timer with its last set duration.
func (t *pauseTimer) Resume() {
	t.Reset(t.GetDuration())
}

// GetDuration returns the current duration of the timer.
func (t *pauseTimer) GetDuration() time.Duration {
	return t.duration
}

// zero returns the zero value for the specified type.
func zero[T any]() T {
	var ret T
	return ret
}

// hash computes the 64-bit FNV-1a hash of the provided data.
func hash(data []byte) uint64 {
	hasher := fnv.New64()
	if _, err := hasher.Write(data); err != nil {
		panic(err)
	}
	return hasher.Sum64()
}
