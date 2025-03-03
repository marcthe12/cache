package cache

import (
	"bytes"
	"sync"
	"time"

	"github.com/marcthe12/cache/internal/pausedtimer"
)

const (
	initialBucketSize uint64  = 8
	loadFactor        float64 = 0.75
)

// node represents an entry in the cache with metadata for eviction and expiration.
type node struct {
	Hash       uint64
	Key        []byte
	Value      []byte
	Expiration time.Time
	Access     uint64

	HashNext  *node
	HashPrev  *node
	EvictNext *node
	EvictPrev *node
}

func (n *node) UnlinkHash() {
	n.HashNext.HashPrev = n.HashPrev
	n.HashPrev.HashNext = n.HashNext
	n.HashNext = nil
	n.HashPrev = nil
}

func (n *node) UnlinkEvict() {
	n.EvictNext.EvictPrev = n.EvictPrev
	n.EvictPrev.EvictNext = n.EvictNext
	n.EvictNext = nil
	n.EvictPrev = nil
}

// IsValid checks if the node is still valid based on its expiration time.
func (n *node) IsValid() bool {
	return n.Expiration.IsZero() || n.Expiration.After(time.Now())
}

// TTL returns the time-to-live of the node.
func (n *node) TTL() time.Duration {
	if n.Expiration.IsZero() {
		return 0
	} else {
		return time.Until(n.Expiration)
	}
}

func (n *node) Cost() uint64 {
	return uint64(len(n.Key) + len(n.Value))
}

// store represents the in-memory cache with eviction policies and periodic tasks.
type store struct {
	Bucket         []node
	Length         uint64
	Cost           uint64
	EvictList      node
	MaxCost        uint64
	SnapshotTicker *pausedtimer.PauseTimer
	CleanupTicker  *pausedtimer.PauseTimer
	Policy         evictionPolicy

	Lock      sync.RWMutex
	EvictLock sync.RWMutex
}

// Init initializes the store with default settings.
func (s *store) Init() {
	s.Clear()
	s.Policy = evictionPolicy{
		ListLock: &s.EvictLock,
		Sentinel: &s.EvictList,
	}
	s.SnapshotTicker = pausedtimer.NewStopped(0)
	s.CleanupTicker = pausedtimer.NewStopped(10 * time.Second)

	if err := s.Policy.SetPolicy(PolicyNone); err != nil {
		panic(err)
	}
}

// Clear removes all entries from the store.
func (s *store) Clear() {
	s.Lock.Lock()
	defer s.Lock.Unlock()

	s.Bucket = make([]node, initialBucketSize)
	s.Length = 0
	s.Cost = 0

	s.EvictList.EvictNext = &s.EvictList
	s.EvictList.EvictPrev = &s.EvictList
}

// lookupIdx calculates the hash and index for a given key.
func lookupIdx(s *store, key []byte) (uint64, uint64) {
	hash := hash(key)

	return hash % uint64(len(s.Bucket)), hash
}

// lazyInitBucket initializes the hash bucket if it hasn't been initialized yet.
func lazyInitBucket(n *node) {
	if n.HashNext == nil {
		n.HashNext = n
		n.HashPrev = n
	}
}

// lookup finds a node in the store by key.
func (s *store) lookup(key []byte) (*node, uint64, uint64) {
	idx, hash := lookupIdx(s, key)

	bucket := &s.Bucket[idx]

	lazyInitBucket(bucket)

	for v := bucket.HashNext; v != bucket; v = v.HashNext {
		if bytes.Equal(key, v.Key) {
			return v, idx, hash
		}
	}

	return nil, idx, hash
}

// Get retrieves a value from the store by key with locking.
func (s *store) Get(key []byte) ([]byte, time.Duration, bool) {
	s.Lock.RLock()
	defer s.Lock.RUnlock()

	v, _, _ := s.lookup(key)
	if v != nil {
		if !v.IsValid() {
			//deleteNode(s, v)

			return nil, 0, false
		}

		s.Policy.OnAccess(v)

		return v.Value, v.TTL(), true
	}

	return nil, 0, false
}

// resize doubles the size of the hash table and rehashes all entries.
func (s *store) Resize() {
	bucket := make([]node, 2*len(s.Bucket))

	for i := range s.Bucket {
		sentinel := &s.Bucket[i]
		if sentinel.HashNext == nil {
			continue
		}

		var order []*node
		for v := sentinel.HashNext; v != sentinel; v = v.HashNext {
			order = append(order, v)
		}

		for _, v := range order {
			idx := v.Hash % uint64(len(bucket))

			n := &bucket[idx]
			lazyInitBucket(n)

			v.HashPrev = n
			v.HashNext = v.HashPrev.HashNext
			v.HashNext.HashPrev = v
			v.HashPrev.HashNext = v
		}
	}

	s.Bucket = bucket
}

// cleanup removes expired entries from the store.
func (s *store) Cleanup() {
	s.Lock.Lock()
	defer s.Lock.Unlock()

	s.EvictLock.Lock()
	defer s.EvictLock.Unlock()

	for v := s.EvictList.EvictNext; v != &s.EvictList; {
		n := v.EvictNext
		if !v.IsValid() {
			deleteNode(s, v)
		}
		v = n
	}
}

// evict removes entries from the store based on the eviction policy.
func (s *store) Evict() bool {
	s.Lock.Lock()
	defer s.Lock.Unlock()

	s.EvictLock.Lock()
	defer s.EvictLock.Unlock()

	if s.MaxCost == 0 {
		return true
	}

	for s.MaxCost < s.Cost {
		n := s.Policy.Evict()
		if n == nil {
			break
		}
		deleteNode(s, n)
	}

	return true
}

// insert adds a new key-value pair to the store.
func (s *store) insert(key []byte, value []byte, ttl time.Duration) {
	idx, hash := lookupIdx(s, key)
	bucket := &s.Bucket[idx]

	if float64(s.Length)/float64(len(s.Bucket)) > float64(loadFactor) {
		s.Resize()
		// resize may invalidate pointer to bucket
		idx, _ = lookupIdx(s, key)
		bucket = &s.Bucket[idx]
		lazyInitBucket(bucket)
	}

	v := &node{
		Hash:  hash,
		Key:   key,
		Value: value,
	}

	if ttl != 0 {
		v.Expiration = time.Now().Add(ttl)
	} else {
		v.Expiration = zero[time.Time]()
	}

	v.HashPrev = bucket
	v.HashNext = v.HashPrev.HashNext
	v.HashNext.HashPrev = v
	v.HashPrev.HashNext = v

	s.Policy.OnInsert(v)

	s.Cost = s.Cost + v.Cost()
	s.Length = s.Length + 1
}

// Set adds or updates a key-value pair in the store with locking.
func (s *store) Set(key []byte, value []byte, ttl time.Duration) {
	s.Lock.Lock()
	defer s.Lock.Unlock()

	v, _, _ := s.lookup(key)
	if v != nil {
		cost := v.Cost()
		v.Value = value
		if ttl != 0 {
			v.Expiration = time.Now().Add(ttl)
		} else {
			v.Expiration = zero[time.Time]()
		}
		s.Cost = s.Cost + v.Cost() - cost
		s.Policy.OnUpdate(v)
		return
	}

	s.insert(key, value, ttl)
}

// deleteNode removes a node from the store.
func deleteNode(s *store, v *node) {
	v.UnlinkEvict()
	v.UnlinkHash()

	s.Cost = s.Cost - v.Cost()
	s.Length = s.Length - 1
}

// Delete removes a key-value pair from the store with locking.
func (s *store) Delete(key []byte) bool {
	s.Lock.Lock()
	defer s.Lock.Unlock()

	v, _, _ := s.lookup(key)
	if v != nil {
		deleteNode(s, v)

		return true
	}

	return false
}

// UpdateInPlace retrieves a value from the store, processes it using the provided function,
// and then sets the result back into the store with the same key.
func (s *store) UpdateInPlace(key []byte, processFunc func([]byte) ([]byte, error), ttl time.Duration) error {
	s.Lock.Lock()
	defer s.Lock.Unlock()

	v, _, _ := s.lookup(key)
	if v == nil {
		return ErrKeyNotFound
	}

	if !v.IsValid() {
		deleteNode(s, v)
		return ErrKeyNotFound
	}

	value, err := processFunc(v.Value)
	if err != nil {
		return err
	}

	cost := v.Cost()
	v.Value = value
	if ttl != 0 {
		v.Expiration = time.Now().Add(ttl)
	} else {
		v.Expiration = zero[time.Time]()
	}
	s.Cost = s.Cost + v.Cost() - cost
	s.Policy.OnUpdate(v)

	return nil
}

// Memorize attempts to retrieve a value from the store. If the retrieval fails,
// it sets the result of the factory function into the store and returns that result.
func (s *store) Memorize(key []byte, factory func() ([]byte, error), ttl time.Duration) ([]byte, error) {
	s.Lock.Lock()
	defer s.Lock.Unlock()

	v, _, _ := s.lookup(key)
	if v != nil && v.IsValid() {
		s.Policy.OnAccess(v)
		return v.Value, nil
	}

	value, err := factory()
	if err != nil {
		return nil, err
	}

	s.insert(key, value, ttl)
	return value, nil
}
