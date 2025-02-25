package cache

import (
	"testing"
	"time"

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

func TestFIFOPolicy(t *testing.T) {
	t.Parallel()

	t.Run("OnInsert", func(t *testing.T) {
		t.Parallel()

		policy := fifoPolicy{evict: createSentinel(t), shouldEvict: true}

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

			policy := fifoPolicy{evict: createSentinel(t), shouldEvict: true}

			n0 := &node{Key: []byte("0")}
			n1 := &node{Key: []byte("1")}

			policy.OnInsert(n0)
			policy.OnInsert(n1)

			evictedNode := policy.Evict()
			assert.Same(t, n0, evictedNode)
		})

		t.Run("Evict noEvict", func(t *testing.T) {
			t.Parallel()

			policy := fifoPolicy{evict: createSentinel(t), shouldEvict: false}

			policy.OnInsert(&node{})

			assert.Nil(t, policy.Evict())
		})

		t.Run("Empty List", func(t *testing.T) {
			t.Parallel()

			policy := fifoPolicy{evict: createSentinel(t)}

			assert.Nil(t, policy.Evict())
		})
	})

	t.Run("Eviction Order", func(t *testing.T) {
		t.Parallel()

		policy := lfuPolicy{evict: createSentinel(t)}

		n0 := &node{Key: []byte("0"), Access: 1}
		n1 := &node{Key: []byte("1"), Access: 1}

		policy.OnInsert(n0)
		policy.OnInsert(n1)

		evictedNode := policy.Evict()
		assert.Same(t, n0, evictedNode) // Assuming FIFO order for same access count
	})

	t.Run("With Zero TTL", func(t *testing.T) {
		t.Parallel()

		policy := ltrPolicy{evict: createSentinel(t), evictZero: false}

		n0 := &node{Key: []byte("0"), Expiration: time.Time{}}
		n1 := &node{Key: []byte("1"), Expiration: time.Now().Add(1 * time.Hour)}

		policy.OnInsert(n0)
		policy.OnInsert(n1)

		evictedNode := policy.Evict()
		assert.Same(t, n1, evictedNode) // n0 should not be evicted due to zero TTL
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

			n0 := &node{Key: []byte("0")}
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

func TestLTRPolicy(t *testing.T) {
	t.Parallel()

	t.Run("OnInsert", func(t *testing.T) {
		t.Parallel()

		t.Run("With TTL", func(t *testing.T) {
			t.Parallel()

			policy := ltrPolicy{evict: createSentinel(t), evictZero: true}

			n0 := &node{Key: []byte("0"), Expiration: time.Now().Add(1 * time.Hour)}
			n1 := &node{Key: []byte("1"), Expiration: time.Now().Add(2 * time.Hour)}

			policy.OnInsert(n0)
			policy.OnInsert(n1)

			order := getListOrder(t, policy.evict)
			assert.Len(t, order, 2)
			assert.Same(t, n0, order[0])
			assert.Same(t, n1, order[1])
		})

		t.Run("Without TTL", func(t *testing.T) {
			t.Parallel()

			policy := ltrPolicy{evict: createSentinel(t), evictZero: true}

			n0 := &node{Key: []byte("0")}
			n1 := &node{Key: []byte("1")}

			policy.OnInsert(n0)
			policy.OnInsert(n1)

			order := getListOrder(t, policy.evict)
			assert.Len(t, order, 2)
			assert.Same(t, n1, order[0])
			assert.Same(t, n0, order[1])
		})
	})

	t.Run("OnUpdate", func(t *testing.T) {
		t.Parallel()

		t.Run("With TTL", func(t *testing.T) {
			t.Parallel()

			policy := ltrPolicy{evict: createSentinel(t), evictZero: true}

			n0 := &node{Key: []byte("0"), Expiration: time.Now().Add(1 * time.Hour)}
			n1 := &node{Key: []byte("1"), Expiration: time.Now().Add(2 * time.Hour)}

			policy.OnInsert(n0)
			policy.OnInsert(n1)

			n0.Expiration = time.Now().Add(3 * time.Hour)
			policy.OnUpdate(n0)

			order := getListOrder(t, policy.evict)
			assert.Len(t, order, 2)
			assert.Same(t, n0, order[1])
			assert.Same(t, n1, order[0])
		})

		t.Run("With TTL Decrease", func(t *testing.T) {
			t.Parallel()

			policy := ltrPolicy{evict: createSentinel(t), evictZero: true}

			n0 := &node{Key: []byte("0"), Expiration: time.Now().Add(1 * time.Hour)}
			n1 := &node{Key: []byte("1"), Expiration: time.Now().Add(2 * time.Hour)}

			policy.OnInsert(n0)
			policy.OnInsert(n1)

			n1.Expiration = time.Now().Add(30 * time.Minute)
			policy.OnUpdate(n1)

			order := getListOrder(t, policy.evict)
			assert.Len(t, order, 2)
			assert.Same(t, n1, order[1])
			assert.Same(t, n0, order[0])
		})
	})

	t.Run("Evict", func(t *testing.T) {
		t.Parallel()

		t.Run("Evict", func(t *testing.T) {
			t.Parallel()

			policy := ltrPolicy{evict: createSentinel(t), evictZero: true}

			n0 := &node{Key: []byte("0")}
			n1 := &node{Key: []byte("1")}

			policy.OnInsert(n0)
			policy.OnInsert(n1)

			evictedNode := policy.Evict()
			assert.Same(t, n0, evictedNode)
		})

		t.Run("Evict TTL", func(t *testing.T) {
			t.Parallel()

			policy := ltrPolicy{evict: createSentinel(t), evictZero: true}

			n0 := &node{Key: []byte("0"), Expiration: time.Now().Add(1 * time.Hour)}
			n1 := &node{Key: []byte("1"), Expiration: time.Now().Add(2 * time.Hour)}

			policy.OnInsert(n0)
			policy.OnInsert(n1)

			evictedNode := policy.Evict()
			assert.Same(t, n1, evictedNode)
		})

		t.Run("Evict TTL Update", func(t *testing.T) {
			t.Parallel()

			policy := ltrPolicy{evict: createSentinel(t), evictZero: true}

			n0 := &node{Key: []byte("0"), Expiration: time.Now().Add(1 * time.Hour)}
			n1 := &node{Key: []byte("1"), Expiration: time.Now().Add(2 * time.Hour)}

			policy.OnInsert(n0)
			policy.OnInsert(n1)

			n0.Expiration = time.Now().Add(3 * time.Hour)
			policy.OnUpdate(n0)

			evictedNode := policy.Evict()
			assert.Same(t, n0, evictedNode)
		})

		t.Run("Evict TTL Update Down", func(t *testing.T) {
			t.Parallel()

			policy := ltrPolicy{evict: createSentinel(t), evictZero: true}

			n0 := &node{Key: []byte("0"), Expiration: time.Now().Add(1 * time.Hour)}
			n1 := &node{Key: []byte("1"), Expiration: time.Now().Add(2 * time.Hour)}

			policy.OnInsert(n0)
			policy.OnInsert(n1)

			n1.Expiration = time.Now().Add(20 * time.Minute)
			policy.OnUpdate(n1)

			evictedNode := policy.Evict()
			assert.Same(t, n1, evictedNode)
		})

		t.Run("Empty List", func(t *testing.T) {
			t.Parallel()

			policy := ltrPolicy{evict: createSentinel(t), evictZero: true}

			assert.Nil(t, policy.Evict())
		})
	})
}
