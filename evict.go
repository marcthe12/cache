package cache

import (
	"errors"
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
	Type  EvictionPolicyType
	evict *node
}

// pushEvict adds a node to the eviction list.
func pushEvict(node *node, sentinnel *node) {
	node.EvictPrev = sentinnel
	node.EvictNext = node.EvictPrev.EvictNext
	node.EvictNext.EvictPrev = node
	node.EvictPrev.EvictNext = node
}

// SetPolicy sets the eviction policy based on the given type.
func (e *evictionPolicy) SetPolicy(y EvictionPolicyType) error {
	store := map[EvictionPolicyType]func() evictionStrategies{
		PolicyNone: func() evictionStrategies {
			return fifoPolicy{evict: e.evict, shouldEvict: false}
		},
		PolicyFIFO: func() evictionStrategies {
			return fifoPolicy{evict: e.evict, shouldEvict: true}
		},
		PolicyLRU: func() evictionStrategies {
			return lruPolicy{evict: e.evict}
		},
		PolicyLFU: func() evictionStrategies {
			return lfuPolicy{evict: e.evict}
		},
		PolicyLTR: func() evictionStrategies {
			return ltrPolicy{evict: e.evict}
		},
	}

	factory, ok := store[y]
	if !ok {
		return errors.New("invalid policy")
	}

	e.evictionStrategies = factory()

	return nil
}

type evictOrderedPolicy interface {
	evictionStrategies
	getEvict() *node
}

type fifoPolicy struct {
	evict       *node
	shouldEvict bool
}

// OnInsert adds a node to the eviction list.
func (s fifoPolicy) OnInsert(node *node) {
	pushEvict(node, s.evict)
}

// OnAccess is a no-op for fifoPolicy.
func (fifoPolicy) OnAccess(n *node) {
}

// OnUpdate is a no-op for fifoPolicy.
func (fifoPolicy) OnUpdate(n *node) {
}

// Evict returns the oldest node for fifoPolicy.
func (s fifoPolicy) Evict() *node {
	if s.shouldEvict && s.evict.EvictPrev != s.evict {
		return s.evict.EvictPrev
	} else {
		return nil
	}
}

func (s fifoPolicy) getEvict() *node {
	return s.evict
}

// lruPolicy struct represents the Least Recently Used eviction policy.
type lruPolicy struct {
	evict *node
}

// OnInsert adds a node to the eviction list.
func (s lruPolicy) OnInsert(node *node) {
	pushEvict(node, s.evict)
}

// OnUpdate moves the accessed node to the front of the eviction list.
func (s lruPolicy) OnUpdate(node *node) {
	s.OnAccess(node)
}

// OnAccess moves the accessed node to the front of the eviction list.
func (s lruPolicy) OnAccess(node *node) {
	node.EvictNext.EvictPrev = node.EvictPrev
	node.EvictPrev.EvictNext = node.EvictNext
	s.OnInsert(node)
}

// Evict returns the least recently used node for lruPolicy.
func (s lruPolicy) Evict() *node {
	if s.evict.EvictPrev != s.evict {
		return s.evict.EvictPrev
	} else {
		return nil
	}
}

func (s lruPolicy) getEvict() *node {
	return s.evict
}

// lfuPolicy struct represents the Least Frequently Used eviction policy.
type lfuPolicy struct {
	evict *node
}

// OnInsert adds a node to the eviction list and initializes its access count.
func (s lfuPolicy) OnInsert(node *node) {
	pushEvict(node, s.evict)
}

// OnUpdate increments the access count of the node and reorders the list.
func (s lfuPolicy) OnUpdate(node *node) {
	s.OnAccess(node)
}

// OnAccess increments the access count of the node and reorders the list.
func (s lfuPolicy) OnAccess(node *node) {
	node.Access++

	for v := node.EvictPrev; v.EvictPrev != s.evict; v = v.EvictPrev {
		if v.Access <= node.Access {
			node.EvictNext.EvictPrev = node.EvictPrev
			node.EvictPrev.EvictNext = node.EvictNext

			node.EvictPrev = v
			node.EvictNext = node.EvictPrev.EvictNext
			node.EvictNext.EvictPrev = node
			node.EvictPrev.EvictNext = node

			return
		}
	}

	node.EvictNext.EvictPrev = node.EvictPrev
	node.EvictPrev.EvictNext = node.EvictNext

	node.EvictPrev = s.evict
	node.EvictNext = node.EvictPrev.EvictNext
	node.EvictNext.EvictPrev = node
	node.EvictPrev.EvictNext = node
}

// Evict returns the least frequently used node for LFU.
func (s lfuPolicy) Evict() *node {
	if s.evict.EvictPrev != s.evict {
		return s.evict.EvictPrev
	} else {
		return nil
	}
}

func (s ltrPolicy) getEvict() *node {
	return s.evict
}

// ltrPolicy struct represents the Least Remaining Time eviction policy.
type ltrPolicy struct {
	evict     *node
	evictZero bool
}

// OnInsert adds a node to the eviction list based on its TTL (Time To Live).
// It places the node in the correct position in the list based on TTL.
func (s ltrPolicy) OnInsert(node *node) {
	pushEvict(node, s.evict)

	s.OnUpdate(node)
}

// OnAccess is a no-op for ltrPolicy.
// It does not perform any action when a node is accessed.
func (s ltrPolicy) OnAccess(node *node) {
}

// OnUpdate updates the position of the node in the eviction list based on its TTL.
// It reorders the list to maintain the correct order based on TTL.
func (s ltrPolicy) OnUpdate(node *node) {
	if node.TTL() == 0 {
		return
	}

	for v := node.EvictPrev; v.EvictPrev != s.evict; v = v.EvictPrev {
		if v.TTL() == 0 {
			continue
		}

		if v.TTL() < node.TTL() {
			node.EvictNext.EvictPrev = node.EvictPrev
			node.EvictPrev.EvictNext = node.EvictNext

			node.EvictPrev = v
			node.EvictNext = node.EvictPrev.EvictNext
			node.EvictNext.EvictPrev = node
			node.EvictPrev.EvictNext = node

			return
		}
	}

	for v := node.EvictNext; v.EvictNext != s.evict; v = v.EvictNext {
		if v.TTL() == 0 {
			continue
		}

		if v.TTL() > node.TTL() {
			node.EvictNext.EvictPrev = node.EvictPrev
			node.EvictPrev.EvictNext = node.EvictNext

			node.EvictPrev = v
			node.EvictNext = node.EvictPrev.EvictNext
			node.EvictNext.EvictPrev = node
			node.EvictPrev.EvictNext = node

			return
		}
	}
}

// Evict returns the node with the least remaining time to live for ltrPolicy.
// It returns the node at the end of the eviction list.
func (s ltrPolicy) Evict() *node {
	if s.evict.EvictPrev != s.evict && (s.evict.EvictPrev.TTL() != 0 || s.evictZero) {
		return s.evict.EvictPrev
	}

	return nil
}

func (s lfuPolicy) getEvict() *node {
	return s.evict
}
