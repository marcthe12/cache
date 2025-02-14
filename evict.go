package cache

import "errors"

type EvictionPolicy int

const (
	StrategyNone EvictionPolicy = iota
	StrategyFIFO
	StrategyLRU
	StrategyLFU
)

type EvictionStrategies interface {
	OnInsert(n *Node)
	OnAccess(n *Node)
	Evict() *Node
}

func (e EvictionPolicy) ToStratergy(evict *Node) (EvictionStrategies, error) {
	store := map[EvictionPolicy]func() EvictionStrategies{
		StrategyNone: func() EvictionStrategies {
			return NoneStrategies{evict: evict}
		},
		StrategyFIFO: func() EvictionStrategies {
			return FIFOStrategies{evict: evict}
		},
		StrategyLRU: func() EvictionStrategies {
			return LRUStrategies{evict: evict}
		},
	}
	factory, ok := store[e]
	if !ok {
		return nil, errors.New("Invalid Policy")
	}
	return factory(), nil
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
