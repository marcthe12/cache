package cache

import (
	"hash/fnv"
	"math"
	"time"
)

type pauseTimer struct {
	*time.Ticker
	duration time.Duration
}

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

func (t *pauseTimer) Reset(d time.Duration) {
	t.duration = d
	if t.duration == 0 {
		t.Stop()
	} else {
		t.Ticker.Reset(d)
	}
}

func (t *pauseTimer) Resume() {
	t.Reset(t.GetDuration())
}

func (t *pauseTimer) GetDuration() time.Duration {
	return t.duration
}

func zero[T any]() T {
	var ret T
	return ret
}

func hash(data []byte) uint64 {
	hasher := fnv.New64()
	if _, err := hasher.Write(data); err != nil {
		panic(err)
	}
	return hasher.Sum64()
}
