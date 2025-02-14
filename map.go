package cache

import (
	"bytes"
	"hash/fnv"
	"iter"
	"sync"
	"time"
)

func zero[T any]() T {
	var ret T
	return ret
}

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

type Store struct {
	bucket   []Node
	lenght   uint64
	cost     uint64
	evict    Node
	max_cost uint64
	strategy EvictionStrategies
	mu       sync.RWMutex
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

func hash(data []byte) uint64 {
	hasher := fnv.New64()
	if _, err := hasher.Write(data); err != nil {
		panic(err)
	}
	return hasher.Sum64()
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
			s.strategy.OnAccess(v)
			return v.Value, 0, true
		}
	}

	return nil, 0, false
}

func resize(s *Store) {
	bucket := make([]Node, 2*len(s.bucket))

	for v := s.evict.EvictNext; v != &s.evict; v = v.EvictNext {
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
		Hash:       hash,
		Key:        key,
		Value:      value,
		Expiration: time.Now().Add(ttl),
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

func (s *Store) Delete(key []byte) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	idx, _ := lookup(s, key)

	bucket := &s.bucket[idx]

	lazyInitBucket(bucket)

	for v := bucket.HashNext; v != bucket; v = v.HashNext {
		if bytes.Equal(key, v.Key) {
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
			return true
		}
	}

	return false
}

func (s *Store) Cost() uint64 {
	return s.cost
}
