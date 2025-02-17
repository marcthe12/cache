package cache

import (
	"bytes"
	"iter"
	"sync"
	"time"
)

type Map[K any, V any] interface {
	Set(key K, value V, ttl time.Duration) error
	Get(key K) (V, time.Duration, error)
	Delete(key K) error
	Clear() iter.Seq2[K, V]
}

type Node struct {
	Hash       uint64
	Expiration time.Time
	Access     uint64
	Key        []byte
	Value      []byte
	HashNext   *Node
	HashPrev   *Node
	EvictNext  *Node
	EvictPrev  *Node
}

func (n *Node) IsValid() bool {
	return n.Expiration.IsZero() || n.Expiration.After(time.Now())
}

func (n *Node) Detach() {
}

type Store struct {
	bucket   []Node
	lenght   uint64
	cost     uint64
	evict    Node
	maxCost  uint64
	strategy EvictionPolicy
	mu       sync.RWMutex
}

func (s *Store) Init() {
	s.Clear()
	s.strategy.evict = &s.evict
	s.strategy.SetPolicy(StrategyNone)
}

func (s *Store) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.bucket = make([]Node, 8)
	s.lenght = 0
	s.cost = 0

	s.evict.EvictNext = &s.evict
	s.evict.EvictPrev = &s.evict
}

func lookup(s *Store, key []byte) (uint64, uint64) {
	hash := hash(key)
	return hash % uint64(len(s.bucket)), hash
}

func lazyInitBucket(n *Node) {
	if n.HashNext == nil {
		n.HashNext = n
		n.HashPrev = n
	}
}

func (s *Store) Get(key []byte) ([]byte, time.Duration, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	idx, _ := lookup(s, key)

	bucket := &s.bucket[idx]

	lazyInitBucket(bucket)

	for v := bucket.HashNext; v != bucket; v = v.HashNext {
		if bytes.Equal(key, v.Key) {
			if !v.IsValid() {
				return nil, 0, false
			}
			s.strategy.OnAccess(v)
			return v.Value, time.Until(v.Expiration), true
		}
	}

	return nil, 0, false
}

func resize(s *Store) {
	bucket := make([]Node, 2*len(s.bucket))

	for v := s.evict.EvictNext; v != &s.evict; v = v.EvictNext {
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

	s.bucket = bucket
}

func cleanup(s *Store) {
	for v := s.evict.EvictNext; v != &s.evict; v = v.EvictNext {
		if !v.IsValid() {
			deleteNode(s, v)
		}
	}
}

func evict(s *Store) bool {
	n := s.strategy.Evict()
	if n == nil {
		return false
	}
	deleteNode(s, n)
	return true
}

func (s *Store) Set(key []byte, value []byte, ttl time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	idx, hash := lookup(s, key)
	bucket := &s.bucket[idx]

	lazyInitBucket(bucket)

	for v := bucket.HashNext; v != bucket; v = v.HashNext {
		if bytes.Equal(key, v.Key) {
			s.cost = s.cost + uint64(len(value)) - uint64(len(v.Value))
			v.Value = value
			v.Expiration = time.Now().Add(ttl)
			s.strategy.OnAccess(v)
		}
	}

	if float64(s.lenght)/float64(len(s.bucket)) > 0.75 {
		resize(s)
		//resize may invidate pointer to bucket
		bucket = &s.bucket[idx]
		lazyInitBucket(bucket)
	}

	node := &Node{
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

	s.strategy.OnInsert(node)
	s.strategy.OnAccess(node)

	s.cost = s.cost + uint64(len(key)) + uint64(len(value))
	s.lenght = s.lenght + 1
}

func deleteNode(s *Store, v *Node) {
	v.HashNext.HashPrev = v.HashPrev
	v.HashPrev.HashNext = v.HashNext
	v.HashNext = nil
	v.HashPrev = nil

	v.EvictNext.EvictPrev = v.EvictPrev
	v.EvictPrev.EvictNext = v.EvictNext
	v.EvictNext = nil
	v.EvictPrev = nil

	s.cost = s.cost - (uint64(len(v.Key)) + uint64(len(v.Value)))
	s.lenght = s.lenght - 1
}

func (s *Store) Delete(key []byte) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	idx, _ := lookup(s, key)

	bucket := &s.bucket[idx]

	lazyInitBucket(bucket)

	for v := bucket.HashNext; v != bucket; v = v.HashNext {
		if bytes.Equal(key, v.Key) {
			deleteNode(s, v)
			return true
		}
	}

	return false
}

func (s *Store) Cost() uint64 {
	return s.cost
}
