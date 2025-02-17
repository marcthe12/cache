package cache

import "errors"

type EvictionPolicyType int

const (
	StrategyNone EvictionPolicyType = iota
	StrategyFIFO
	StrategyLRU
	StrategyLFU
)

type EvictionStrategies interface {
	OnInsert(n *Node)
	OnAccess(n *Node)
	Evict() *Node
}

type EvictionPolicy struct {
	EvictionStrategies
	Type  EvictionPolicyType
	evict *Node
}

func (e *EvictionPolicy) SetPolicy(y EvictionPolicyType) error {
	store := map[EvictionPolicyType]func() EvictionStrategies{
		StrategyNone: func() EvictionStrategies {
			return NoneStrategies{evict: e.evict}
		},
		StrategyFIFO: func() EvictionStrategies {
			return FIFOStrategies{evict: e.evict}
		},
		StrategyLRU: func() EvictionStrategies {
			return LRUStrategies{evict: e.evict}
		},
	}
	factory, ok := store[y]
	if !ok {
		return errors.New("Invalid Policy")
	}
	e.EvictionStrategies = factory()
	return nil
}

type NoneStrategies struct {
	evict *Node
}

func (s NoneStrategies) OnInsert(node *Node) {
	node.EvictPrev = s.evict
	node.EvictNext = node.EvictPrev.EvictNext
	node.EvictNext.EvictPrev = node
	node.EvictPrev.EvictNext = node
}

func (NoneStrategies) OnAccess(n *Node) {
}

func (NoneStrategies) Evict() *Node {
	return nil
}

type FIFOStrategies struct {
	evict *Node
}

func (s FIFOStrategies) OnInsert(node *Node) {
	node.EvictPrev = s.evict
	node.EvictNext = node.EvictPrev.EvictNext
	node.EvictNext.EvictPrev = node
	node.EvictPrev.EvictNext = node
}

func (FIFOStrategies) OnAccess(n *Node) {
}

func (s FIFOStrategies) Evict() *Node {
	return s.evict.EvictPrev
}

type LRUStrategies struct {
	evict *Node
}

func (s LRUStrategies) OnInsert(node *Node) {
}

func (s LRUStrategies) OnAccess(node *Node) {
	node.EvictNext.EvictPrev = node.EvictPrev
	node.EvictPrev.EvictNext = node.EvictNext

	node.EvictPrev = s.evict
	node.EvictNext = node.EvictPrev.EvictNext
	node.EvictNext.EvictPrev = node
	node.EvictPrev.EvictNext = node
}

func (s LRUStrategies) Evict() *Node {
	return s.evict.EvictPrev
}

type LFUStrategies struct {
	evict *Node
}

func (s LFUStrategies) OnInsert(node *Node) {
	node.EvictPrev = s.evict
	node.EvictNext = node.EvictPrev.EvictNext
	node.EvictNext.EvictPrev = node
	node.EvictPrev.EvictNext = node
}

func (s LFUStrategies) OnAccess(node *Node) {
	node.Access += 1

	node.EvictNext.EvictPrev = node.EvictPrev
	node.EvictPrev.EvictNext = node.EvictNext

	for v := node.EvictPrev; v != s.evict; v = v.EvictPrev {
		if v.Access >= node.Access {
			node.EvictPrev = v
			node.EvictNext = node.EvictPrev.EvictNext
			node.EvictNext.EvictPrev = node
			node.EvictPrev.EvictNext = node
			break
		}
	}

	node.EvictPrev = s.evict
	node.EvictNext = node.EvictPrev.EvictNext
	node.EvictNext.EvictPrev = node
	node.EvictPrev.EvictNext = node
}

func (s LFUStrategies) Evict() *Node {
	return s.evict.EvictPrev
}
