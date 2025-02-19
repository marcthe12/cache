package cache

import (
	"errors"
	"io"
	"os"
	"sync"
	"time"

	"github.com/vmihailenco/msgpack"
)

type db struct {
	File           io.WriteSeeker
	Store          store
	SnapshotTicker *pauseTimer
	CleanupTicker  *pauseTimer
	Stop           chan struct{}
	wg             sync.WaitGroup
}

type Option func(*db) error

func openFile(filename string, options ...Option) (*db, error) {
	ret, err := openMem(options...)
	if err != nil {
		return nil, err
	}
	file, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		return nil, err
	}
	fileInfo, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if fileInfo.Size() == 0 {
		ret.File = file
		ret.Flush()
	} else {
		err := ret.Store.LoadSnapshot(file)
		if err != nil {
			return nil, err
		}
		ret.File = file
	}

	return ret, nil
}

func openMem(options ...Option) (*db, error) {
	ret := &db{
		SnapshotTicker: newPauseTimerStopped(0),
		CleanupTicker:  newPauseTimerStopped(10 * time.Second),
	}
	ret.Store.Init()
	ret.SetConfig(options...)
	return ret, nil
}

func (d *db) Start() {
	d.Stop = make(chan struct{})
	d.wg.Add(1)
	go d.backgroundWorker()
}

func (d *db) SetConfig(options ...Option) error {
	for _, opt := range options {
		if err := opt(d); err != nil {
			return err
		}
	}
	return nil
}

func WithPolicy(e EvictionPolicyType) Option {
	return func(d *db) error {
		return d.Store.Policy.SetPolicy(e)
	}
}

func WithMaxCost(maxCost uint64) Option {
	return func(d *db) error {
		d.Store.MaxCost = maxCost
		return nil
	}
}

func SetSnapshotTime(t time.Duration) Option {
	return func(d *db) error {
		d.SnapshotTicker.Reset(t)
		return nil
	}
}

func SetCleanupTime(t time.Duration) Option {
	return func(d *db) error {
		d.CleanupTicker.Reset(t)
		return nil
	}
}

func (d *db) backgroundWorker() {
	defer d.wg.Done()

	d.SnapshotTicker.Resume()
	defer d.SnapshotTicker.Stop()

	d.CleanupTicker.Resume()
	defer d.CleanupTicker.Stop()

	for {
		select {
		case <-d.Stop:
			return
		case <-d.SnapshotTicker.C:
			d.Flush()
		case <-d.CleanupTicker.C:
			cleanup(&d.Store)
			evict(&d.Store)
		}
	}
}

func (d *db) Close() {
	close(d.Stop)
	d.wg.Wait()
	d.Flush()
	d.Clear()
	if d.File != nil {
		closer, ok := d.File.(io.Closer)
		if ok {
			closer.Close()
		}
	}
}

func (d *db) Flush() error {
	if d.File != nil {
		return d.Store.Snapshot(d.File)
	}
	return nil
}

func (d *db) Clear() {
	d.Store.Clear()
}

var ErrKeyNotFound = errors.New("key not found")

// The Cache database. Can be initialized by either OpenFile or OpenMem. Uses per DB Locks.
type DB[K any, V any] struct {
	*db
}

func OpenFile[K any, V any](filename string, options ...Option) (DB[K, V], error) {
	ret, err := openFile(filename, options...)
	if err != nil {
		return zero[DB[K, V]](), err
	}
	ret.Start()
	return DB[K, V]{db: ret}, nil
}

func OpenMem[K any, V any](filename string, options ...Option) (DB[K, V], error) {
	ret, err := openMem(options...)
	if err != nil {
		return zero[DB[K, V]](), err
	}
	ret.Start()
	return DB[K, V]{db: ret}, nil
}

func (h *DB[K, V]) Get(key K, value V) (V, time.Duration, error) {
	keyData, err := msgpack.Marshal(key)
	if err != nil {
		return value, 0, err
	}

	v, ttl, ok := h.Store.Get(keyData)
	if !ok {
		return value, 0, ErrKeyNotFound
	}

	if err = msgpack.Unmarshal(v, value); err != nil {
		return value, 0, err
	}
	return value, ttl, err
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
