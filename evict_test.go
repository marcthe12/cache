package cache

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func createSentinel(t testing.TB) *node {
	t.Helper()
	n1 := &node{Key: []byte("Sentinel")}
	n1.EvictNext = n1
	n1.EvictPrev = n1
	return n1
}

func getListOrder(t testing.TB, evict *node) []*node {
	t.Helper()

	var order []*node
	current := evict.EvictNext
	for current != evict {
		order = append(order, current)
		current = current.EvictNext
	}
	for _, n := range order {
		assert.Same(t, n, n.EvictPrev.EvictNext)
	}
	return order
}

func TestNonePolicy(t *testing.T) {
	t.Parallel()

	t.Run("OnInsert", func(t *testing.T) {
		t.Parallel()

		policy := nonePolicy{evict: createSentinel(t)}

		n0 := &node{Key: []byte("0")}
		n1 := &node{Key: []byte("1")}

		policy.OnInsert(n0)
		policy.OnInsert(n1)

		order := getListOrder(t, policy.evict)
		assert.Len(t, order, 2)
		assert.Contains(t, order, n0)
		assert.Contains(t, order, n1)
	})

	t.Run("Evict", func(t *testing.T) {
		t.Parallel()

		policy := nonePolicy{evict: createSentinel(t)}

		policy.OnInsert(&node{})

		assert.Nil(t, policy.Evict())
	})
}

func TestFIFOPolicy(t *testing.T) {
	t.Parallel()

	t.Run("OnInsert", func(t *testing.T) {
		t.Parallel()

		policy := fifoPolicy{evict: createSentinel(t)}

		n0 := &node{Key: []byte("0")}
		n1 := &node{Key: []byte("1")}

		policy.OnInsert(n0)
		policy.OnInsert(n1)

		order := getListOrder(t, policy.evict)
		assert.Len(t, order, 2)
		assert.Same(t, order[0], n1)
		assert.Same(t, order[1], n0)
	})

	t.Run("Evict", func(t *testing.T) {
		t.Parallel()

		t.Run("Evict", func(t *testing.T) {
			t.Parallel()

			policy := fifoPolicy{evict: createSentinel(t)}

			n0 := &node{Key: []byte("0")}
			n1 := &node{Key: []byte("1")}

			policy.OnInsert(n0)
			policy.OnInsert(n1)

			evictedNode := policy.Evict()
			assert.Same(t, n0, evictedNode)
		})
		t.Run("Empty List", func(t *testing.T) {
			t.Parallel()

			policy := fifoPolicy{evict: createSentinel(t)}

			assert.Nil(t, policy.Evict())
		})
	})
}

func TestLRUPolicy(t *testing.T) {
	t.Run("OnInsert", func(t *testing.T) {
		t.Parallel()

		policy := lruPolicy{evict: createSentinel(t)}

		n0 := &node{Key: []byte("0")}
		n1 := &node{Key: []byte("1")}

		policy.OnInsert(n0)
		policy.OnInsert(n1)

		order := getListOrder(t, policy.evict)
		assert.Len(t, order, 2)
		assert.Same(t, order[0], n1)
		assert.Same(t, order[1], n0)
	})

	t.Run("OnAccess", func(t *testing.T) {
		t.Parallel()

		policy := lruPolicy{evict: createSentinel(t)}

		n0 := &node{Key: []byte("0")}
		n1 := &node{Key: []byte("1")}

		policy.OnInsert(n0)
		policy.OnInsert(n1)

		policy.OnAccess(n0)

		order := getListOrder(t, policy.evict)
		assert.Len(t, order, 2)
		assert.Same(t, order[0], n0)
		assert.Same(t, order[1], n1)
	})

	t.Run("Evict", func(t *testing.T) {
		t.Parallel()

		t.Run("Evict", func(t *testing.T) {
			t.Parallel()

			policy := lruPolicy{evict: createSentinel(t)}

			n0 := &node{Key: []byte("0")}
			n1 := &node{Key: []byte("1")}

			policy.OnInsert(n0)
			policy.OnInsert(n1)

			evictedNode := policy.Evict()
			assert.Same(t, n0, evictedNode)
		})

		t.Run("OnAccess End", func(t *testing.T) {
			t.Parallel()

			policy := lruPolicy{evict: createSentinel(t)}

			n0 := &node{Key: []byte("0")}
			n1 := &node{Key: []byte("1")}

			policy.OnInsert(n0)
			policy.OnInsert(n1)

			policy.OnAccess(n0)

			evictedNode := policy.Evict()
			assert.Same(t, n1, evictedNode)
		})

		t.Run("OnAccess Interleaved", func(t *testing.T) {
			t.Parallel()

			policy := lruPolicy{evict: createSentinel(t)}

			n0 := &node{Key: []byte("0")}
			n1 := &node{Key: []byte("1")}

			policy.OnInsert(n0)
			policy.OnAccess(n0)
			policy.OnInsert(n1)

			evictedNode := policy.Evict()
			assert.Same(t, n0, evictedNode)
		})

		t.Run("Empty", func(t *testing.T) {
			t.Parallel()

			policy := lruPolicy{evict: createSentinel(t)}

			assert.Nil(t, policy.Evict())
		})
	})
}

func TestLFUPolicy(t *testing.T) {
	t.Parallel()

	t.Run("OnInsert", func(t *testing.T) {
		t.Parallel()

		policy := lfuPolicy{evict: createSentinel(t)}

		n0 := &node{Key: []byte("0")}
		n1 := &node{Key: []byte("1")}

		policy.OnInsert(n0)
		policy.OnInsert(n1)

		order := getListOrder(t, policy.evict)
		assert.Len(t, order, 2)
		assert.Contains(t, order, n0)
		assert.Contains(t, order, n1)
	})

	t.Run("OnAccess", func(t *testing.T) {
		t.Parallel()

		policy := lfuPolicy{evict: createSentinel(t)}

		n0 := &node{Key: []byte("0")}
		n1 := &node{Key: []byte("1")}

		policy.OnInsert(n0)
		policy.OnInsert(n1)

		policy.OnAccess(n0)

		order := getListOrder(t, policy.evict)
		assert.Len(t, order, 2)
		assert.Same(t, order[0], n0)
		assert.Same(t, order[1], n1)
	})

	t.Run("Evict", func(t *testing.T) {
		t.Parallel()

		t.Run("Evict", func(t *testing.T) {
			t.Parallel()

			policy := lfuPolicy{evict: createSentinel(t)}

			n0 := &node{Key: []byte("0")}
			n1 := &node{Key: []byte("1")}

			policy.OnInsert(n0)
			policy.OnInsert(n1)

			policy.OnAccess(n0)

			evictedNode := policy.Evict()
			assert.Same(t, n1, evictedNode)
		})

		t.Run("Evict After Multiple Accesses", func(t *testing.T) {
			t.Parallel()

			policy := lfuPolicy{evict: createSentinel(t)}

			n0 := &node{Key: []byte("1")}
			n1 := &node{Key: []byte("1")}

			policy.OnInsert(n0)
			policy.OnInsert(n1)

			policy.OnAccess(n0)

			policy.OnAccess(n1)
			policy.OnAccess(n1)

			evictedNode := policy.Evict()
			assert.Same(t, n0, evictedNode)
		})

		t.Run("Empty List", func(t *testing.T) {
			t.Parallel()

			policy := lfuPolicy{evict: createSentinel(t)}

			assert.Nil(t, policy.Evict())
		})
	})
}
