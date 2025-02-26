package cache

import (
	"errors"
	"io"
	"log"
	"os"
	"sync"
	"time"

	"github.com/rogpeppe/go-internal/lockedfile"
	"github.com/vmihailenco/msgpack/v5"
)

// db represents a cache database with file-backed storage and in-memory operation.
type db struct {
	File  io.WriteSeeker
	Store store
	Stop  chan struct{}
	wg    sync.WaitGroup
}

// Option is a function type for configuring the db.
type Option func(*db) error

// openFile opens a file-backed cache database with the given options.
func openFile(filename string, options ...Option) (*db, error) {
	ret, err := openMem(options...)
	if err != nil {
		return nil, err
	}

	file, err := lockedfile.OpenFile(filename, os.O_RDWR|os.O_CREATE, 0o666)
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

// openMem initializes an in-memory cache database with the given options.
func openMem(options ...Option) (*db, error) {
	ret := &db{}
	ret.Store.Init()
	if err := ret.SetConfig(options...); err != nil {
		return nil, err
	}

	return ret, nil
}

// Start begins the background worker for periodic tasks.
func (d *db) Start() {
	d.Stop = make(chan struct{})
	d.wg.Add(1)

	go d.backgroundWorker()
}

// SetConfig applies configuration options to the db.
func (d *db) SetConfig(options ...Option) error {
	d.Store.mu.Lock()
	defer d.Store.mu.Unlock()

	for _, opt := range options {
		if err := opt(d); err != nil {
			return err
		}
	}

	return nil
}

// WithPolicy sets the eviction policy for the cache.
func WithPolicy(e EvictionPolicyType) Option {
	return func(d *db) error {
		return d.Store.Policy.SetPolicy(e)
	}
}

// WithMaxCost sets the maximum cost for the cache.
func WithMaxCost(maxCost uint64) Option {
	return func(d *db) error {
		d.Store.MaxCost = maxCost

		return nil
	}
}

// SetSnapshotTime sets the interval for taking snapshots of the cache.
func SetSnapshotTime(t time.Duration) Option {
	return func(d *db) error {
		d.Store.SnapshotTicker.Reset(t)

		return nil
	}
}

// SetCleanupTime sets the interval for cleaning up expired entries.
func SetCleanupTime(t time.Duration) Option {
	return func(d *db) error {
		d.Store.CleanupTicker.Reset(t)

		return nil
	}
}

// backgroundWorker performs periodic tasks such as snapshotting and cleanup.
func (d *db) backgroundWorker() {
	defer d.wg.Done()

	defer func() {
		if r := recover(); r != nil {
			log.Printf("Recovered from panic in background worker: %v", r)
		}
	}()

	d.Store.SnapshotTicker.Resume()
	defer d.Store.SnapshotTicker.Stop()

	d.Store.CleanupTicker.Resume()
	defer d.Store.CleanupTicker.Stop()

	for {
		select {
		case <-d.Stop:
			return
		case <-d.Store.SnapshotTicker.C:
			d.Flush()
		case <-d.Store.CleanupTicker.C:
			d.Store.Cleanup()
			d.Store.Evict()
		}
	}
}

// Close stops the background worker and cleans up resources.
func (d *db) Close() error {
	close(d.Stop)
	d.wg.Wait()
	err := d.Flush()
	d.Clear()

	var err1 error
	if d.File != nil {
		closer, ok := d.File.(io.Closer)
		if ok {
			err1 = closer.Close()
		}
	}
	if err != nil {
		return err
	}
	return err1
}

// Flush writes the current state of the store to the file.
func (d *db) Flush() error {
	if d.File != nil {
		return d.Store.Snapshot(d.File)
	}

	return nil
}

// Clear removes all entries from the in-memory store.
func (d *db) Clear() {
	d.Store.Clear()
}

var ErrKeyNotFound = errors.New("key not found") // ErrKeyNotFound is returned when a key is not found in the cache.

// The Cache database. Can be initialized by either OpenFile or OpenMem. Uses per DB Locks.
// DB represents a generic cache database with key-value pairs.
type DB[K any, V any] struct {
	*db
}

// OpenFile opens a file-backed cache database with the specified options.
func OpenFile[K any, V any](filename string, options ...Option) (DB[K, V], error) {
	ret, err := openFile(filename, options...)
	if err != nil {
		return zero[DB[K, V]](), err
	}

	ret.Start()

	return DB[K, V]{db: ret}, nil
}

// OpenMem initializes an in-memory cache database with the specified options.
func OpenMem[K any, V any](options ...Option) (DB[K, V], error) {
	ret, err := openMem(options...)
	if err != nil {
		return zero[DB[K, V]](), err
	}

	ret.Start()

	return DB[K, V]{db: ret}, nil
}

// marshal serializes a value using msgpack.
func marshal[T any](v T) ([]byte, error) {
	return msgpack.Marshal(v)
}

// unmarshal deserializes data into a value using msgpack.
func unmarshal[T any](data []byte, v *T) error {
	return msgpack.Unmarshal(data, v)
}

// Get retrieves a value from the cache by key and returns its TTL.
func (h *DB[K, V]) Get(key K, value *V) (time.Duration, error) {
	keyData, err := marshal(key)
	if err != nil {
		return 0, err
	}

	v, ttl, ok := h.Store.Get(keyData)
	if !ok {
		return 0, ErrKeyNotFound
	}

	if v != nil {
		if err = unmarshal(v, value); err != nil {
			return 0, err
		}
	}

	return ttl, err
}

// GetValue retrieves a value from the cache by key and returns the value and its TTL.
func (h *DB[K, V]) GetValue(key K) (V, time.Duration, error) {
	value := zero[V]()
	ttl, err := h.Get(key, &value)

	return value, ttl, err
}

// Set adds a key-value pair to the cache with a specified TTL.
func (h *DB[K, V]) Set(key K, value V, ttl time.Duration) error {
	keyData, err := marshal(key)
	if err != nil {
		return err
	}

	valueData, err := marshal(value)
	if err != nil {
		return err
	}

	h.Store.Set(keyData, valueData, ttl)

	return nil
}

// Delete removes a key-value pair from the cache.
func (h *DB[K, V]) Delete(key K) error {
	keyData, err := marshal(key)
	if err != nil {
		return err
	}

	ok := h.Store.Delete(keyData)
	if !ok {
		return ErrKeyNotFound
	}

	return nil
}

// UpdateInPlace retrieves a value from the cache, processes it using the provided function,
// and then sets the result back into the cache with the same key.
func (h *DB[K, V]) UpdateInPlace(key K, processFunc func(V) (V, error), ttl time.Duration) error {
	keyData, err := marshal(key)
	if err != nil {
		return err
	}

	return h.Store.UpdateInPlace(keyData, func(data []byte) ([]byte, error) {
		var value V
		if err := unmarshal(data, &value); err != nil {
			return nil, err
		}

		processedValue, err := processFunc(value)
		if err != nil {
			return nil, err
		}

		return marshal(processedValue)
	}, ttl)
}

// Memoize attempts to retrieve a value from the cache. If the retrieval fails,
// it sets the result of the factory function into the cache and returns that result.
func (h *DB[K, V]) Memoize(key K, factoryFunc func() (V, error), ttl time.Duration) (V, error) {
	keyData, err := marshal(key)
	if err != nil {
		return zero[V](), err
	}

	data, err := h.Store.Memoize(keyData, func() ([]byte, error) {
		value, err := factoryFunc()
		if err != nil {
			return nil, err
		}

		return marshal(value)
	}, ttl)

	if err != nil {
		return zero[V](), err
	}

	var value V
	if err := unmarshal(data, &value); err != nil {
		return zero[V](), err
	}

	return value, nil
}
