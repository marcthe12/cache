package cache

import (
	"errors"
	"sync"
)

// EvictionPolicyType defines the type of eviction policy.
type EvictionPolicyType int

const (
	// PolicyNone indicates no eviction policy.
	PolicyNone EvictionPolicyType = iota
	PolicyFIFO
	PolicyLRU
	PolicyLFU
	PolicyLTR
)

// evictionStrategies interface defines the methods for eviction strategies.
type evictionStrategies interface {
	OnInsert(n *node)
	OnUpdate(n *node)
	OnAccess(n *node)
	Evict() *node
}

// evictionPolicy struct holds the eviction strategy and its type.
type evictionPolicy struct {
	evictionStrategies
	Type     EvictionPolicyType
	Sentinel *node
	ListLock *sync.RWMutex
}

// pushEvict adds a node to the eviction list.
func pushEvict(node, sentinnel *node) {
	node.EvictPrev = sentinnel
	node.EvictNext = node.EvictPrev.EvictNext
	node.EvictNext.EvictPrev = node
	node.EvictPrev.EvictNext = node
}

var ErrInvalidPolicy = errors.New("invalid policy")

// SetPolicy sets the eviction policy based on the given type.
func (e *evictionPolicy) SetPolicy(y EvictionPolicyType) error {
	store := map[EvictionPolicyType]func() evictionStrategies{
		PolicyNone: func() evictionStrategies {
			return fifoPolicy{List: e.Sentinel, ShouldEvict: false, Lock: e.ListLock}
		},
		PolicyFIFO: func() evictionStrategies {
			return fifoPolicy{List: e.Sentinel, ShouldEvict: true, Lock: e.ListLock}
		},
		PolicyLRU: func() evictionStrategies {
			return lruPolicy{List: e.Sentinel, Lock: e.ListLock}
		},
		PolicyLFU: func() evictionStrategies {
			return lfuPolicy{List: e.Sentinel, Lock: e.ListLock}
		},
		PolicyLTR: func() evictionStrategies {
			return ltrPolicy{List: e.Sentinel, EvictZero: true, Lock: e.ListLock}
		},
	}

	factory, ok := store[y]
	if !ok {
		return ErrInvalidPolicy
	}

	e.evictionStrategies = factory()
	e.Type = y

	return nil
}

type evictOrderedPolicy interface {
	evictionStrategies
	getEvict() *node
}

type fifoPolicy struct {
	List        *node
	Lock        *sync.RWMutex
	ShouldEvict bool
}

// OnInsert adds a node to the eviction list.
func (s fifoPolicy) OnInsert(n *node) {
	s.Lock.Lock()
	defer s.Lock.Unlock()

	pushEvict(n, s.List)
}

// OnAccess is a no-op for fifoPolicy.
func (fifoPolicy) OnAccess(n *node) {
	// Noop
}

// OnUpdate is a no-op for fifoPolicy.
func (fifoPolicy) OnUpdate(n *node) {
	// Noop
}

// Evict returns the oldest node for fifoPolicy.
func (s fifoPolicy) Evict() *node {
	if s.ShouldEvict && s.List.EvictPrev != s.List {
		return s.List.EvictPrev
	} else {
		return nil
	}
}

func (s fifoPolicy) getEvict() *node {
	return s.List
}

// lruPolicy struct represents the Least Recently Used eviction policy.
type lruPolicy struct {
	List *node
	Lock *sync.RWMutex
}

// OnInsert adds a node to the eviction list.
func (s lruPolicy) OnInsert(n *node) {
	s.Lock.Lock()
	defer s.Lock.Unlock()

	pushEvict(n, s.List)
}

// OnUpdate moves the accessed node to the front of the eviction list.
func (s lruPolicy) OnUpdate(n *node) {
	s.OnAccess(n)
}

// OnAccess moves the accessed node to the front of the eviction list.
func (s lruPolicy) OnAccess(n *node) {
	s.Lock.Lock()
	defer s.Lock.Unlock()

	n.EvictNext.EvictPrev = n.EvictPrev
	n.EvictPrev.EvictNext = n.EvictNext

	pushEvict(n, s.List)
}

// Evict returns the least recently used node for lruPolicy.
func (s lruPolicy) Evict() *node {
	if s.List.EvictPrev != s.List {
		return s.List.EvictPrev
	} else {
		return nil
	}
}

func (s lruPolicy) getEvict() *node {
	return s.List
}

// lfuPolicy struct represents the Least Frequently Used eviction policy.
type lfuPolicy struct {
	List *node
	Lock *sync.RWMutex
}

// OnInsert adds a node to the eviction list and initializes its access count.
func (s lfuPolicy) OnInsert(n *node) {
	s.Lock.Lock()
	defer s.Lock.Unlock()

	pushEvict(n, s.List)
}

// OnUpdate increments the access count of the node and reorders the list.
func (s lfuPolicy) OnUpdate(n *node) {
	s.OnAccess(n)
}

// OnAccess increments the access count of the node and reorders the list.
func (s lfuPolicy) OnAccess(n *node) {
	s.Lock.Lock()
	defer s.Lock.Unlock()

	n.Access++

	for v := n.EvictPrev; v.EvictPrev != s.List; v = v.EvictPrev {
		if v.Access <= n.Access {
			n.EvictNext.EvictPrev = n.EvictPrev
			n.EvictPrev.EvictNext = n.EvictNext

			n.EvictPrev = v
			n.EvictNext = n.EvictPrev.EvictNext
			n.EvictNext.EvictPrev = n
			n.EvictPrev.EvictNext = n

			return
		}
	}

	n.EvictNext.EvictPrev = n.EvictPrev
	n.EvictPrev.EvictNext = n.EvictNext

	n.EvictPrev = s.List
	n.EvictNext = n.EvictPrev.EvictNext
	n.EvictNext.EvictPrev = n
	n.EvictPrev.EvictNext = n
}

// Evict returns the least frequently used node for LFU.
func (s lfuPolicy) Evict() *node {
	if s.List.EvictPrev != s.List {
		return s.List.EvictPrev
	} else {
		return nil
	}
}

func (s lfuPolicy) getEvict() *node {
	return s.List
}

// ltrPolicy struct represents the Least Remaining Time eviction policy.
type ltrPolicy struct {
	List      *node
	Lock      *sync.RWMutex
	EvictZero bool
}

// OnInsert adds a node to the eviction list based on its TTL (Time To Live).
// It places the node in the correct position in the list based on TTL.
func (s ltrPolicy) OnInsert(n *node) {
	s.Lock.Lock()
	defer s.Lock.Unlock()

	pushEvict(n, s.List)

	s.update(n)
}

// OnAccess is a no-op for ltrPolicy.
// It does not perform any action when a node is accessed.
func (s ltrPolicy) OnAccess(n *node) {
	// Noop
}

// OnUpdate updates the position of the node in the eviction list based on its TTL.
// It reorders the list to maintain the correct order based on TTL.
func (s ltrPolicy) OnUpdate(n *node) {
	s.Lock.Lock()
	defer s.Lock.Unlock()

	s.update(n)
}

func (s ltrPolicy) update(n *node) {
	if n.TTL() == 0 {
		return
	}

	for v := n.EvictPrev; v.EvictPrev != s.List; v = v.EvictPrev {
		if v.TTL() == 0 {
			continue
		}

		if v.TTL() < n.TTL() {
			n.EvictNext.EvictPrev = n.EvictPrev
			n.EvictPrev.EvictNext = n.EvictNext

			n.EvictPrev = v
			n.EvictNext = n.EvictPrev.EvictNext
			n.EvictNext.EvictPrev = n
			n.EvictPrev.EvictNext = n

			return
		}
	}

	for v := n.EvictNext; v.EvictNext != s.List; v = v.EvictNext {
		if v.TTL() == 0 {
			continue
		}

		if v.TTL() > n.TTL() {
			n.EvictNext.EvictPrev = n.EvictPrev
			n.EvictPrev.EvictNext = n.EvictNext

			n.EvictPrev = v
			n.EvictNext = n.EvictPrev.EvictNext
			n.EvictNext.EvictPrev = n
			n.EvictPrev.EvictNext = n

			return
		}
	}
}

// Evict returns the node with the least remaining time to live for ltrPolicy.
// It returns the node at the end of the eviction list.
func (s ltrPolicy) Evict() *node {
	if s.List.EvictPrev != s.List && (s.List.EvictPrev.TTL() != 0 || s.EvictZero) {
		return s.List.EvictPrev
	}

	return nil
}

func (s ltrPolicy) getEvict() *node {
	return s.List
}
