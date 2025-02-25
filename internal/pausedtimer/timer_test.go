package pausedtimer

import (
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	d := 1 * time.Second
	timer := New(d)
	if timer.duration != d {
		t.Errorf("expected duration %#v, got %v", d, timer.duration)
	}
	if timer.Ticker == nil {
		t.Error("expected Ticker to be non-nil")
	}
}

func TestPauseTimerZeroDuration(t *testing.T) {
	timer := New(0)
	if timer.GetDuration() != 0 {
		t.Errorf("expected duration %v, got %v", time.Duration(0), timer.GetDuration())
	}
	if timer.Ticker == nil {
		t.Error("expected Ticker to be non-nil")
	}
}

func TestPauseTimerResetToZero(t *testing.T) {
	timer := New(1 * time.Second)
	timer.Reset(0)
	if timer.GetDuration() != 0 {
		t.Errorf("expected duration %v, got %v", time.Duration(0), timer.GetDuration())
	}
}

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
	got := 2 * time.Second
	timer.Reset(got)
	if timer.duration != got {
		t.Errorf("expected duration %v, got %v", got, timer.duration)
	}
}

func TestPauseTimerResume(t *testing.T) {
	d := 1 * time.Second
	timer := NewStopped(d)
	timer.Resume()
	if timer.duration != d {
		t.Errorf("expected duration %v, got %v", d, timer.duration)
	}
}

func TestPauseTimerGetDuration(t *testing.T) {
	d := 1 * time.Second
	timer := New(d)
	if timer.GetDuration() != d {
		t.Errorf("expected duration %v, got %v", d, timer.GetDuration())
	}
}
