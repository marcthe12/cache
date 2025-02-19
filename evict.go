package cache

import "errors"

type EvictionPolicyType int

const (
	PolicyNone EvictionPolicyType = iota
	PolicyFIFO
	PolicyLRU
	PolicyLFU
	PolicyLTR
)

type evictionStrategies interface {
	OnInsert(n *node)
	OnAccess(n *node)
	Evict() *node
}

type evictionPolicy struct {
	evictionStrategies
	Type  EvictionPolicyType
	evict *node
}

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
		return errors.New("invalid olicy")
	}
	e.evictionStrategies = factory()
	return nil
}

type nonePolicy struct {
	evict *node
}

func (s nonePolicy) OnInsert(node *node) {
	node.EvictPrev = s.evict
	node.EvictNext = node.EvictPrev.EvictNext
	node.EvictNext.EvictPrev = node
	node.EvictPrev.EvictNext = node
}

func (nonePolicy) OnAccess(n *node) {
}

func (nonePolicy) Evict() *node {
	return nil
}

type fifoPolicy struct {
	evict *node
}

func (s fifoPolicy) OnInsert(node *node) {
	node.EvictPrev = s.evict
	node.EvictNext = node.EvictPrev.EvictNext
	node.EvictNext.EvictPrev = node
	node.EvictPrev.EvictNext = node
}

func (fifoPolicy) OnAccess(n *node) {
}

func (s fifoPolicy) Evict() *node {
	if s.evict.EvictPrev != s.evict {
		return s.evict.EvictPrev
	} else {
		return nil
	}
}

type lruPolicy struct {
	evict *node
}

func (s lruPolicy) OnInsert(node *node) {
	node.EvictPrev = s.evict
	node.EvictNext = node.EvictPrev.EvictNext
	node.EvictNext.EvictPrev = node
	node.EvictPrev.EvictNext = node
}

func (s lruPolicy) OnAccess(node *node) {
	node.EvictNext.EvictPrev = node.EvictPrev
	node.EvictPrev.EvictNext = node.EvictNext

	node.EvictPrev = s.evict
	node.EvictNext = node.EvictPrev.EvictNext
	node.EvictNext.EvictPrev = node
	node.EvictPrev.EvictNext = node
}

func (s lruPolicy) Evict() *node {
	if s.evict.EvictPrev != s.evict {
		return s.evict.EvictPrev
	} else {
		return nil
	}
}

type lfuPolicy struct {
	evict *node
}

func (s lfuPolicy) OnInsert(node *node) {
	node.EvictPrev = s.evict
	node.EvictNext = node.EvictPrev.EvictNext
	node.EvictNext.EvictPrev = node
	node.EvictPrev.EvictNext = node
	node.Access = 0
}

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

func (s lfuPolicy) Evict() *node {
	if s.evict.EvictPrev != s.evict {
		return s.evict.EvictPrev
	} else {
		return nil
	}
}
