package cache

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewPauseTimer(t *testing.T) {
	d := 1 * time.Second
	timer := newPauseTimer(d)
	assert.Equal(t, d, timer.duration)
	assert.NotNil(t, timer.Ticker)
}

func TestPauseTimerReset(t *testing.T) {
	d := 1 * time.Second
	timer := newPauseTimer(d)
	newD := 2 * time.Second
	timer.Reset(newD)
	assert.Equal(t, newD, timer.duration)
}

func TestPauseTimerResume(t *testing.T) {
	d := 1 * time.Second
	timer := newPauseTimerStopped(d)
	timer.Resume()
	assert.Equal(t, d, timer.duration)
}

func TestPauseTimerGetDuration(t *testing.T) {
	d := 1 * time.Second
	timer := newPauseTimer(d)
	assert.Equal(t, d, timer.GetDuration())
}
