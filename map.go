package cache

import (
	"bytes"
	"sync"
	"time"
)

type node struct {
	Hash       uint64
	Expiration time.Time
	Access     uint64
	Key        []byte
	Value      []byte
	HashNext   *node
	HashPrev   *node
	EvictNext  *node
	EvictPrev  *node
}

func (n *node) IsValid() bool {
	return n.Expiration.IsZero() || n.Expiration.After(time.Now())
}

func (n *node) TTL() time.Duration {
	if n.Expiration.IsZero() {
		return 0
	} else {
		return time.Until(n.Expiration)
	}
}

type store struct {
	Bucket  []node
	Lenght  uint64
	Cost    uint64
	Evict   node
	MaxCost uint64
	Policy  evictionPolicy
	mu      sync.Mutex
}

func (s *store) Init() {
	s.Clear()
	s.Policy.evict = &s.Evict
	s.Policy.SetPolicy(PolicyNone)
}

func (s *store) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Bucket = make([]node, 8)
	s.Lenght = 0
	s.Cost = 0

	s.Evict.EvictNext = &s.Evict
	s.Evict.EvictPrev = &s.Evict
}

func lookup(s *store, key []byte) (uint64, uint64) {
	hash := hash(key)
	return hash % uint64(len(s.Bucket)), hash
}

func lazyInitBucket(n *node) {
	if n.HashNext == nil {
		n.HashNext = n
		n.HashPrev = n
	}
}

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

func (s *store) get(key []byte) ([]byte, time.Duration, bool) {
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

func (s *store) Get(key []byte) ([]byte, time.Duration, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.get(key)
}

func resize(s *store) {
	bucket := make([]node, 2*len(s.Bucket))

	for v := s.Evict.EvictNext; v != &s.Evict; v = v.EvictNext {
		if !v.IsValid() {
			continue
		}
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

func cleanup(s *store) {
	for v := s.Evict.EvictNext; v != &s.Evict; v = v.EvictNext {
		if !v.IsValid() {
			deleteNode(s, v)
		}
	}
}

func evict(s *store) bool {
	for s.MaxCost != 0 && s.MaxCost < s.Cost {
		n := s.Policy.Evict()
		if n == nil {
			break
		}
		deleteNode(s, n)
	}
	return true
}

func (s *store) set(key []byte, value []byte, ttl time.Duration) {
	v, idx, hash := s.lookup(key)
	if v != nil {
		s.Cost = s.Cost + uint64(len(value)) - uint64(len(v.Value))
		v.Value = value
		v.Expiration = time.Now().Add(ttl)
		s.Policy.OnAccess(v)
	}

	bucket := &s.Bucket[idx]
	if float64(s.Lenght)/float64(len(s.Bucket)) > 0.75 {
		resize(s)
		//resize may invidate pointer to bucket
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
	s.Lenght = s.Lenght + 1
}

func (s *store) Set(key []byte, value []byte, ttl time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.set(key, value, ttl)
}

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
	s.Lenght = s.Lenght - 1
}

func (s *store) delete(key []byte) bool {
	v, _, _ := s.lookup(key)
	if v != nil {
		deleteNode(s, v)
		return true
	}

	return false
}

func (s *store) Delete(key []byte) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.delete(key)
}
