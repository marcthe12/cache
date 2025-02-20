package cache

import "errors"

// EvictionPolicyType defines the type of eviction policy.
type EvictionPolicyType int

const (
	PolicyNone EvictionPolicyType = iota
	PolicyFIFO
	PolicyLRU
	PolicyLFU
	PolicyLTR
)

// evictionStrategies interface defines the methods for eviction strategies.
type evictionStrategies interface {
	OnInsert(n *node)
	OnAccess(n *node)
	Evict() *node
}

// evictionPolicy struct holds the eviction strategy and its type.
type evictionPolicy struct {
	evictionStrategies
	Type  EvictionPolicyType
	evict *node
}

// SetPolicy sets the eviction policy based on the given type.
func (e *evictionPolicy) SetPolicy(y EvictionPolicyType) error {
	store := map[EvictionPolicyType]func() evictionStrategies{
		PolicyNone: func() evictionStrategies {
			return nonePolicy{evict: e.evict}
		},
		PolicyFIFO: func() evictionStrategies {
			return fifoPolicy{evict: e.evict}
		},
		PolicyLRU: func() evictionStrategies {
			return lruPolicy{evict: e.evict}
		},
		PolicyLFU: func() evictionStrategies {
			return lfuPolicy{evict: e.evict}
		},
	}
	factory, ok := store[y]
	if !ok {
		return errors.New("invalid policy")
	}
	e.evictionStrategies = factory()
	return nil
}

// nonePolicy struct represents the no eviction policy.
type nonePolicy struct {
	evict *node
}

// OnInsert adds a node to the eviction list.
func (s nonePolicy) OnInsert(node *node) {
	node.EvictPrev = s.evict
	node.EvictNext = node.EvictPrev.EvictNext
	node.EvictNext.EvictPrev = node
	node.EvictPrev.EvictNext = node
}

// OnAccess is a no-op for nonePolicy.
func (nonePolicy) OnAccess(n *node) {
}

// Evict returns nil for nonePolicy.
func (nonePolicy) Evict() *node {
	return nil
}

// fifoPolicy struct represents the First-In-First-Out eviction policy.
type fifoPolicy struct {
	evict *node
}

// OnInsert adds a node to the eviction list.
func (s fifoPolicy) OnInsert(node *node) {
	node.EvictPrev = s.evict
	node.EvictNext = node.EvictPrev.EvictNext
	node.EvictNext.EvictPrev = node
	node.EvictPrev.EvictNext = node
}

// OnAccess is a no-op for fifoPolicy.
func (fifoPolicy) OnAccess(n *node) {
}

// Evict returns the oldest node for fifoPolicy.
func (s fifoPolicy) Evict() *node {
	if s.evict.EvictPrev != s.evict {
		return s.evict.EvictPrev
	} else {
		return nil
	}
}

// lruPolicy struct represents the Least Recently Used eviction policy.
type lruPolicy struct {
	evict *node
}

// OnInsert adds a node to the eviction list.
func (s lruPolicy) OnInsert(node *node) {
	node.EvictPrev = s.evict
	node.EvictNext = node.EvictPrev.EvictNext
	node.EvictNext.EvictPrev = node
	node.EvictPrev.EvictNext = node
}

// OnAccess moves the accessed node to the front of the eviction list.
func (s lruPolicy) OnAccess(node *node) {
	node.EvictNext.EvictPrev = node.EvictPrev
	node.EvictPrev.EvictNext = node.EvictNext

	node.EvictPrev = s.evict
	node.EvictNext = node.EvictPrev.EvictNext
	node.EvictNext.EvictPrev = node
	node.EvictPrev.EvictNext = node
}

// Evict returns the least recently used node for lruPolicy.
func (s lruPolicy) Evict() *node {
	if s.evict.EvictPrev != s.evict {
		return s.evict.EvictPrev
	} else {
		return nil
	}
}

// lfuPolicy struct represents the Least Frequently Used eviction policy.
type lfuPolicy struct {
	evict *node
}

// OnInsert adds a node to the eviction list and initializes its access count.
func (s lfuPolicy) OnInsert(node *node) {
	node.EvictPrev = s.evict
	node.EvictNext = node.EvictPrev.EvictNext
	node.EvictNext.EvictPrev = node
	node.EvictPrev.EvictNext = node
	node.Access = 0
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
