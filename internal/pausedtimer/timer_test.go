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
