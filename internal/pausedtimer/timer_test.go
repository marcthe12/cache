package pausedtimer

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	d := 1 * time.Second
	timer := New(d)
	assert.Equal(t, d, timer.duration)
	assert.NotNil(t, timer.Ticker)
func TestPauseTimerPauseAndResume(t *testing.T) {
    d := 1 * time.Second
    timer := New(d)
    timer.Stop() // Simulate pause
    time.Sleep(500 * time.Millisecond)
    timer.Resume()

    select {
    case <-timer.C:
        // Timer should not have fired yet
        t.Fatal("Timer fired too early")
    case <-time.After(600 * time.Millisecond):
        // Timer should fire after resuming
    }
}

func TestPauseTimerReset(t *testing.T) {
	d := 1 * time.Second
	timer := New(d)
	newD := 2 * time.Second
	timer.Reset(newD)
	assert.Equal(t, newD, timer.duration)
}

func TestPauseTimerResume(t *testing.T) {
	d := 1 * time.Second
	timer := NewStopped(d)
	timer.Resume()
	assert.Equal(t, d, timer.duration)
}

func TestPauseTimerGetDuration(t *testing.T) {
	d := 1 * time.Second
	timer := New(d)
	assert.Equal(t, d, timer.GetDuration())
}
