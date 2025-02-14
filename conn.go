package cache

import (
	"errors"
	"os"
	"sync"
	"time"

	"github.com/vmihailenco/msgpack"
)

type DB[K any, V any] struct {
	file  *os.File
	Store Store
	stop  chan struct{}
	wg    sync.WaitGroup
}

func OpenFile[K any, V any](filename string) (*DB[K, V], error) {
	ret := &DB[K, V]{}
	file, err := os.OpenFile(filename, os.O_RDWR, 0)
	if errors.Is(err, os.ErrNotExist) {
		file, err := os.Create(filename)
		if err != nil {
			return nil, err
		}
		ret.file = file
		ret.SetStratergy(StrategyNone)
		ret.Clear()
		ret.Flush()
	} else if err == nil {
		ret.Clear()
		ret.file = file
		ret.Store.LoadSnapshot(ret.file)
	} else {
		return nil, err
	}
	ret.wg.Add(1)
	go ret.backgroundWorker()
	return ret, nil
}

func OpenMem[K any, V any]() *DB[K, V] {
	ret := &DB[K, V]{}
	ret.SetStratergy(StrategyNone)
	ret.Clear()
	return ret
}

func (d *DB[K, V]) SetStratergy(e EvictionPolicy) error {
	strategy, err := e.ToStratergy(&d.Store.evict)
	if err != nil {
		return err
	}
	d.Store.strategy = strategy
	return nil
}

func (d *DB[K, V]) SetMaxCost(e EvictionPolicy) {
	d.Store.max_cost = d.Store.max_cost
}

func (d *DB[K, V]) backgroundWorker() {
	defer d.wg.Done()
	for {
		select {
		case <-d.stop:
			return
			// TODO: Do house keeping
		}
	}
}

func (d *DB[K, V]) Close() {
	close(d.stop)
	d.wg.Wait()
	d.Flush()
	d.Clear()
	if d.file != nil {
		d.file.Close()
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
