package cache

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/rogpeppe/go-internal/lockedfile"
	"github.com/vmihailenco/msgpack/v5"
)

// cache represents a cache database with file-backed storage and in-memory operation.
type cache struct {
	File  io.WriteSeeker
	Store store
	Stop  chan struct{}
	wg    sync.WaitGroup
	err   error
}

// Option is a function type for configuring the cache.
type Option func(*cache) error

// open opens a file-backed cache database with the given options.
func open(filename string, options ...Option) (*cache, error) {
	ret := &cache{}
	ret.Store.Init()

	if err := ret.SetConfig(options...); err != nil {
		return nil, err
	}

	if filename == "" {
		return ret, nil
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
		if err := ret.Flush(); err != nil {
			return nil, err
		}
	} else {
		err := ret.Store.LoadSnapshot(file)
		if err != nil {
			return nil, err
		}

		ret.File = file
	}

	return ret, nil
}

// start begins the background worker for periodic tasks.
func (c *cache) start() {
	c.Stop = make(chan struct{})

	c.wg.Add(1)

	go c.backgroundWorker()
}

// SetConfig applies configuration options to the cache.
func (c *cache) SetConfig(options ...Option) error {
	c.Store.Lock.Lock()
	defer c.Store.Lock.Unlock()

	for _, opt := range options {
		if err := opt(c); err != nil {
			return err
		}
	}

	return nil
}

// WithPolicy sets the eviction policy for the cache.
func WithPolicy(e EvictionPolicyType) Option {
	return func(d *cache) error {
		return d.Store.Policy.SetPolicy(e)
	}
}

// WithMaxCost sets the maximum cost for the cache.
func WithMaxCost(maxCost uint64) Option {
	return func(d *cache) error {
		d.Store.MaxCost = maxCost

		return nil
	}
}

// SetSnapshotTime sets the interval for taking snapshots of the cache.
func SetSnapshotTime(t time.Duration) Option {
	return func(d *cache) error {
		d.Store.SnapshotTicker.Reset(t)

		return nil
	}
}

// SetCleanupTime sets the interval for cleaning up expired entries.
func SetCleanupTime(t time.Duration) Option {
	return func(d *cache) error {
		d.Store.CleanupTicker.Reset(t)

		return nil
	}
}

// backgroundWorker performs periodic tasks such as snapshotting and cleanup.
func (c *cache) backgroundWorker() {
	defer c.wg.Done()

	defer func() {
		if r := recover(); r != nil {
			c.err = fmt.Errorf("panic occurred: %v", r)
		}
	}()

	c.Store.SnapshotTicker.Resume()
	defer c.Store.SnapshotTicker.Stop()

	c.Store.CleanupTicker.Resume()
	defer c.Store.CleanupTicker.Stop()

	c.Store.Cleanup()
	c.Store.Evict()

	for {
		select {
		case <-c.Stop:
			return
		case <-c.Store.SnapshotTicker.C:
			if err := c.Flush(); err != nil {
				c.err = err
			}
		case <-c.Store.CleanupTicker.C:
			c.Store.Cleanup()
			c.Store.Evict()
		}
	}
}

func (c *cache) Error() error {
	return c.err
}

func (c *cache) Cost() uint64 {
	return c.Store.Cost
}

// Close stops the background worker and cleans up resources.
func (c *cache) Close() error {
	close(c.Stop)
	c.wg.Wait()

	err := c.Flush()
	c.Clear()

	var err1 error

	if c.File != nil {
		closer, ok := c.File.(io.Closer)
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
func (c *cache) Flush() error {
	if c.File != nil {
		return c.Store.Snapshot(c.File)
	}

	return nil
}

// Clear removes all entries from the in-memory store.
func (c *cache) Clear() {
	c.Store.Clear()
}

var ErrKeyNotFound = errors.New("key not found") // ErrKeyNotFound is returned when a key is not found in the cache.

// Get retrieves a value from the cache by key and returns its TTL.
func (c *cache) Get(key []byte, value *[]byte) (time.Duration, error) {
	v, ttl, err := c.GetValue(key)
	*value = v

	return ttl, err
}

// GetValue retrieves a value from the cache by key and returns the value and its TTL.
func (c *cache) GetValue(key []byte) ([]byte, time.Duration, error) {
	if err := c.err; err != nil {
		return zero[[]byte](), 0, err
	}

	v, ttl, ok := c.Store.Get(key)
	if !ok {
		return v, 0, ErrKeyNotFound
	}

	return v, ttl, nil
}

// Set adds a key-value pair to the cache with a specified TTL.
func (c *cache) Set(key, value []byte, ttl time.Duration) error {
	if err := c.err; err != nil {
		return err
	}

	c.Store.Set(key, value, ttl)

	return nil
}

// Delete removes a key-value pair from the cache.
func (c *cache) Delete(key []byte) error {
	ok := c.Store.Delete(key)
	if !ok {
		return ErrKeyNotFound
	}

	return nil
}

// UpdateInPlace retrieves a value from the cache, processes it using the provided function,
// and then sets the result back into the cache with the same key.
func (c *cache) UpdateInPlace(key []byte, processFunc func([]byte) ([]byte, error), ttl time.Duration) error {
	if err := c.err; err != nil {
		return err
	}

	return c.Store.UpdateInPlace(key, processFunc, ttl)
}

// Memorize attempts to retrieve a value from the cache. If the retrieval fails,
// it sets the result of the factory function into the cache and returns that result.
func (c *cache) Memorize(key []byte, factoryFunc func() ([]byte, error), ttl time.Duration) ([]byte, error) {
	if err := c.err; err != nil {
		return []byte{}, err
	}

	return c.Store.Memorize(key, factoryFunc, ttl)
}

// The Cache database. Can be initialized by either Open or OpenFile or OpenMem. Uses per Cache Locks.
// Cache represents a generic cache database with key-value pairs.
type Cache[K any, V any] struct {
	*cache
}

// The CacheRaw database. Can be initialized by either OpenRaw or OpenRawFile or OpenRawMem. Uses per Cache Locks.
// CacheRaw represents a binary cache database with key-value pairs.
type CacheRaw struct {
	*cache
}

// OpenRaw opens a binary cache database with the specified options. If filename is empty then in-memory otherwise file backed.
func OpenRaw(filename string, options ...Option) (CacheRaw, error) {
	ret, err := open(filename, options...)
	if err != nil {
		return zero[CacheRaw](), err
	}

	ret.start()

	return CacheRaw{cache: ret}, nil
}

var ErrEmptyFilename = errors.New("cannot open empty filename")

// OpenRawFile opens a binary file-backed cache database with the specified options.
func OpenRawFile(filename string, options ...Option) (CacheRaw, error) {
	if filename == "" {
		return zero[CacheRaw](), ErrEmptyFilename
	}

	return OpenRaw(filename, options...)
}

// OpenRawMem initializes a binary in-memory cache database with the specified options.
func OpenRawMem(options ...Option) (CacheRaw, error) {
	return OpenRaw("", options...)
}

// Open opens a cache database with the specified options. If filename is empty then in-memory otherwise file backed.
func Open[K, V any](filename string, options ...Option) (Cache[K, V], error) {
	ret, err := OpenRaw(filename, options...)
	if err != nil {
		return zero[Cache[K, V]](), err
	}

	return Cache[K, V]{cache: ret.cache}, nil
}

// OpenFile opens a file-backed cache database with the specified options.
func OpenFile[K, V any](filename string, options ...Option) (Cache[K, V], error) {
	if filename == "" {
		return zero[Cache[K, V]](), ErrEmptyFilename
	}

	return Open[K, V](filename, options...)
}

// OpenMem initializes an in-memory cache database with the specified options.
func OpenMem[K, V any](options ...Option) (Cache[K, V], error) {
	return Open[K, V]("", options...)
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
func (c Cache[K, V]) Get(key K, value *V) (time.Duration, error) {
	keyData, err := marshal(key)
	if err != nil {
		return 0, err
	}

	v, ttl, err := c.cache.GetValue(keyData)
	if err != nil {
		return 0, err
	}

	if v != nil {
		if err = unmarshal(v, value); err != nil {
			return 0, err
		}
	}

	return ttl, err
}

// GetValue retrieves a value from the cache by key and returns the value and its TTL.
func (c Cache[K, V]) GetValue(key K) (V, time.Duration, error) {
	value := zero[V]()
	ttl, err := c.Get(key, &value)

	return value, ttl, err
}

// Set adds a key-value pair to the cache with a specified TTL.
func (c Cache[K, V]) Set(key K, value V, ttl time.Duration) error {
	keyData, err := marshal(key)
	if err != nil {
		return err
	}

	valueData, err := marshal(value)
	if err != nil {
		return err
	}

	return c.cache.Set(keyData, valueData, ttl)
}

// Delete removes a key-value pair from the cache.
func (c Cache[K, V]) Delete(key K) error {
	keyData, err := marshal(key)
	if err != nil {
		return err
	}

	return c.cache.Delete(keyData)
}

// UpdateInPlace retrieves a value from the cache, processes it using the provided function,
// and then sets the result back into the cache with the same key.
func (c Cache[K, V]) UpdateInPlace(key K, processFunc func(V) (V, error), ttl time.Duration) error {
	keyData, err := marshal(key)
	if err != nil {
		return err
	}

	return c.cache.UpdateInPlace(keyData, func(data []byte) ([]byte, error) {
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

// Memorize attempts to retrieve a value from the cache. If the retrieval fails,
// it sets the result of the factory function into the cache and returns that result.
func (c Cache[K, V]) Memorize(key K, factoryFunc func() (V, error), ttl time.Duration) (V, error) {
	keyData, err := marshal(key)
	if err != nil {
		return zero[V](), err
	}

	data, err := c.cache.Memorize(keyData, func() ([]byte, error) {
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

type Cacher[K any, V any] interface {
	Clear()
	Close() error
	Cost() uint64
	Delete(key K) error
	Error() error
	Flush() error
	Get(key K, value *V) (time.Duration, error)
	GetValue(key K) (V, time.Duration, error)
	Memorize(key K, factoryFunc func() (V, error), ttl time.Duration) (V, error)
	Set(key K, value V, ttl time.Duration) error
	SetConfig(options ...Option) error
	UpdateInPlace(key K, processFunc func(V) (V, error), ttl time.Duration) error
}
