package cache

import (
	"errors"
	"io"
	"os"
	"sync"
	"time"

	"github.com/vmihailenco/msgpack"
)

type DB[K any, V any] struct {
	file           io.WriteSeeker
	Store          Store
	snapshotTicker *pauseTimer
	cleanupTicker *pauseTimer
	stop           chan struct{}
	wg             sync.WaitGroup
}

func OpenFile[K any, V any](filename string) (*DB[K, V], error) {
	ret := OpenMem[K, V]()
	file, err := os.OpenFile(filename, os.O_RDWR, 0)
	if errors.Is(err, os.ErrNotExist) {
		file, err := os.Create(filename)
		if err != nil {
			return nil, err
		}
		ret.file = file
		ret.Flush()
	} else if err == nil {
		ret.Store.LoadSnapshot(file)
		ret.file = file
	} else {
		return nil, err
	}

	return ret, nil
}

func OpenMem[K any, V any]() *DB[K, V] {
	ret := &DB[K, V]{
		snapshotTicker: newPauseTimer(0),
	}
	ret.snapshotTicker.Stop()
	ret.Clear()
	ret.Store.strategy.evict = &ret.Store.evict
	ret.SetStratergy(StrategyNone)
	return ret
}

func (d *DB[K, V]) Start() {
	d.stop = make(chan struct{})
	d.wg.Add(1)
	go d.backgroundWorker()
}

func (d *DB[K, V]) SetStratergy(e EvictionPolicyType) error {
	return d.Store.strategy.SetPolicy(e)
}

func (d *DB[K, V]) SetMaxCost(e EvictionPolicyType) {
	d.Store.maxCost = d.Store.maxCost
}

func (d *DB[K, V]) SetSnapshotTime(t time.Duration) {
	d.snapshotTicker.Reset(t)
}

func (d *DB[K, V]) backgroundWorker() {
	defer d.wg.Done()

	d.snapshotTicker.Resume()
	defer d.snapshotTicker.Stop()

	for {
		select {
		case <-d.stop:
			return
		case <-d.snapshotTicker.C:
			d.Flush()
		case <-d.snapshotTicker.C:
			d.Flush()
		}
	}
}

func (d *DB[K, V]) Close() {
	close(d.stop)
	d.wg.Wait()
	d.Flush()
	d.Clear()
	if d.file != nil {
		closer, ok := d.file.(io.Closer)
		if ok {
			closer.Close()
		}
	}
}

func (d *DB[K, V]) Flush() error {
	if d.file != nil {
		return d.Store.Snapshot(d.file)
	}
	return nil
}

func (d *DB[K, V]) Clear() {
	d.Store.Clear()
}

var ErrKeyNotFound = errors.New("key not found")

func (h *DB[K, V]) Get(key K) (V, time.Duration, error) {
	keyData, err := msgpack.Marshal(key)
	if err != nil {
		return zero[V](), 0, err
	}

	v, ttl, ok := h.Store.Get(keyData)
	if !ok {
		return zero[V](), 0, ErrKeyNotFound
	}

	var ret V
	if err = msgpack.Unmarshal(v, &ret); err != nil {
		return zero[V](), 0, err
	}
	return ret, ttl, err
}

func (h *DB[K, V]) Set(key K, value V, ttl time.Duration) error {
	keyData, err := msgpack.Marshal(key)
	if err != nil {
		return err
	}
	valueData, err := msgpack.Marshal(value)
	if err != nil {
		return err
	}
	h.Store.Set(keyData, valueData, ttl)
	return nil
}

func (h *DB[K, V]) Delete(key K) error {
	keyData, err := msgpack.Marshal(key)
	if err != nil {
		return err
	}
	ok := h.Store.Delete(keyData)
	if !ok {
		return ErrKeyNotFound
	}
	return nil
}
