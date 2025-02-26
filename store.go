package cache

import (
	"bytes"
	"sync"
	"time"

	"github.com/marcthe12/cache/internal/pausedtimer"
)

const initialBucketSize uint64 = 8

// node represents an entry in the cache with metadata for eviction and expiration.
type node struct {
	Hash       uint64
	Expiration time.Time
	Access     uint64
	Key        []byte
	Value      []byte

	HashNext  *node
	HashPrev  *node
	EvictNext *node
	EvictPrev *node
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

	mu sync.Mutex
}

// Init initializes the store with default settings.
func (s *store) Init() {
	s.Clear()
	s.Policy.evict = &s.EvictList
	s.SnapshotTicker = pausedtimer.NewStopped(0)
	s.CleanupTicker = pausedtimer.NewStopped(10 * time.Second)

	if err := s.Policy.SetPolicy(PolicyNone); err != nil {
		panic(err)
	}
}

// Clear removes all entries from the store.
func (s *store) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Bucket = make([]node, initialBucketSize)
	s.Length = 0
	s.Cost = 0

	s.EvictList.EvictNext = &s.EvictList
	s.EvictList.EvictPrev = &s.EvictList
}

// lookup calculates the hash and index for a given key.
func lookup(s *store, key []byte) (uint64, uint64) {
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
	idx, hash := lookup(s, key)

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
	s.mu.Lock()
	defer s.mu.Unlock()

	v, _, _ := s.lookup(key)
	if v != nil {
		if !v.IsValid() {
			deleteNode(s, v)

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

	for v := s.EvictList.EvictNext; v != &s.EvictList; v = v.EvictNext {
		idx := v.Hash % uint64(len(bucket))

		n := &bucket[idx]
		lazyInitBucket(n)

		v.HashPrev = n
		v.HashNext = v.HashPrev.HashNext
		v.HashNext.HashPrev = v
		v.HashPrev.HashNext = v
	}

	s.Bucket = bucket
}

// cleanup removes expired entries from the store.
func (s *store) Cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for v := s.EvictList.EvictNext; v != &s.EvictList; v = v.EvictNext {
		if !v.IsValid() {
			deleteNode(s, v)
		}
	}
}

// evict removes entries from the store based on the eviction policy.
func (s *store) Evict() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	for s.MaxCost != 0 && s.MaxCost < s.Cost {
		n := s.Policy.Evict()
		if n == nil {
			break
		}

		deleteNode(s, n)
	}

	return true
}

// Set adds or updates a key-value pair in the store with locking.
func (s *store) Set(key []byte, value []byte, ttl time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	v, idx, hash := s.lookup(key)
	if v != nil {
		s.Cost = s.Cost + uint64(len(value)) - uint64(len(v.Value))
		v.Value = value
		v.Expiration = time.Now().Add(ttl)
		s.Policy.OnUpdate(v)
	}

	bucket := &s.Bucket[idx]

	if float64(s.Length)/float64(len(s.Bucket)) > 0.75 {
		s.Resize()
		// resize may invidate pointer to bucket
		_, idx, _ := s.lookup(key)
		bucket = &s.Bucket[idx]
		lazyInitBucket(bucket)
	}

	node := &node{
		Hash:  hash,
		Key:   key,
		Value: value,
	}

	if ttl != 0 {
		node.Expiration = time.Now().Add(ttl)
	}

	node.HashPrev = bucket
	node.HashNext = node.HashPrev.HashNext
	node.HashNext.HashPrev = node
	node.HashPrev.HashNext = node

	s.Policy.OnInsert(node)

	s.Cost = s.Cost + uint64(len(key)) + uint64(len(value))
	s.Length = s.Length + 1
}

// deleteNode removes a node from the store.
func deleteNode(s *store, v *node) {
	v.HashNext.HashPrev = v.HashPrev
	v.HashPrev.HashNext = v.HashNext
	v.HashNext = nil
	v.HashPrev = nil

	v.EvictNext.EvictPrev = v.EvictPrev
	v.EvictPrev.EvictNext = v.EvictNext
	v.EvictNext = nil
	v.EvictPrev = nil

	s.Cost = s.Cost - (uint64(len(v.Key)) + uint64(len(v.Value)))
	s.Length = s.Length - 1
}

// Delete removes a key-value pair from the store with locking.
func (s *store) Delete(key []byte) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	v, _, _ := s.lookup(key)
	if v != nil {
		deleteNode(s, v)

		return true
	}

	return false
}
