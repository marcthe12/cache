package cache

import (
	"testing"
	"time"
)

func createSentinel(tb testing.TB) *node {
	tb.Helper()

	n1 := &node{Key: []byte("Sentinel")}
	n1.EvictNext = n1
	n1.EvictPrev = n1

	return n1
}

func getListOrder(tb testing.TB, evict *node) []*node {
	tb.Helper()

	var order []*node

	current := evict.EvictNext
	for current != evict {
		order = append(order, current)
		current = current.EvictNext
	}

	for _, n := range order {
		tb.Helper()
		if n != n.EvictPrev.EvictNext {
			tb.Fatalf("expected %#v, got %#v", n, n.EvictPrev.EvictNext)
		}
	}

	return order
}

func checkOrder(tb testing.TB, policy evictOrderedPolicy, expected []*node) {
	tb.Helper()

	order := getListOrder(tb, policy.getEvict())

	if len(order) != len(expected) {
		tb.Errorf("expected length %v, got %v", len(expected), len(order))
	}

	for i, n := range expected {
		if order[i] != n {
			tb.Errorf("element %v did not match: \nexpected: %#v\n got: %#v", i, n, order[i])
		}
	}
}

func TestFIFOPolicy(t *testing.T) {
	t.Parallel()

	t.Run("OnInsert", func(t *testing.T) {
		t.Parallel()

		policy := fifoPolicy{evict: createSentinel(t), shouldEvict: true}

		n0 := &node{Key: []byte("0")}
		n1 := &node{Key: []byte("1")}

		policy.OnInsert(n1)
		policy.OnInsert(n0)

		checkOrder(t, policy, []*node{n0, n1})
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
			if n0 != evictedNode {
				t.Errorf("expected %#v, got %#v", n0, evictedNode)
			}
		})

		t.Run("Evict noEvict", func(t *testing.T) {
			t.Parallel()

			policy := fifoPolicy{evict: createSentinel(t), shouldEvict: false}

			policy.OnInsert(&node{})

			if policy.Evict() != nil {
				t.Errorf("expected nil, got %#v", policy.Evict())
			}
		})

		t.Run("Empty List", func(t *testing.T) {
			t.Parallel()

			policy := fifoPolicy{evict: createSentinel(t)}
			if policy.Evict() != nil {
				t.Errorf("expected nil, got %#v", policy.Evict())
			}
		})
	})
}

func TestLRUPolicy(t *testing.T) {
	t.Parallel()

	t.Run("OnInsert", func(t *testing.T) {
		t.Parallel()

		policy := lruPolicy{evict: createSentinel(t)}

		n0 := &node{Key: []byte("0")}
		n1 := &node{Key: []byte("1")}

		policy.OnInsert(n0)
		policy.OnInsert(n1)

		checkOrder(t, policy, []*node{n1, n0})
	})

	t.Run("OnAccess", func(t *testing.T) {
		t.Parallel()

		policy := lruPolicy{evict: createSentinel(t)}

		n0 := &node{Key: []byte("0")}
		n1 := &node{Key: []byte("1")}

		policy.OnInsert(n0)
		policy.OnInsert(n1)

		policy.OnAccess(n0)

		checkOrder(t, policy, []*node{n0, n1})
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
			if n0 != evictedNode {
				t.Errorf("expected %#v, got %#v", n0, evictedNode)
			}
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
			if n1 != evictedNode {
				t.Errorf("expected %#v, got %#v", n1, evictedNode)
			}
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

			if n0 != evictedNode {
				t.Errorf("expected %#v, got %#v", n0, evictedNode)
			}
		})

		t.Run("Empty", func(t *testing.T) {
			t.Parallel()

			policy := lruPolicy{evict: createSentinel(t)}
			if policy.Evict() != nil {
				t.Errorf("expected nil, got %#v", policy.Evict())
			}
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

		checkOrder(t, policy, []*node{n1, n0})
	})

	t.Run("OnAccess", func(t *testing.T) {
		t.Parallel()

		policy := lfuPolicy{evict: createSentinel(t)}

		n0 := &node{Key: []byte("0")}
		n1 := &node{Key: []byte("1")}

		policy.OnInsert(n0)
		policy.OnInsert(n1)

		policy.OnAccess(n0)

		checkOrder(t, policy, []*node{n0, n1})
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
			if n1 != evictedNode {
				t.Errorf("expected %#v, got %#v", n1, evictedNode)
			}
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

			if n0 != evictedNode {
				t.Errorf("expected %#v, got %#v", n0, evictedNode)
			}
		})

		t.Run("Empty List", func(t *testing.T) {
			t.Parallel()

			policy := lfuPolicy{evict: createSentinel(t)}
			if policy.Evict() != nil {
				t.Errorf("expected nil, got %#v", policy.Evict())
			}
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

			checkOrder(t, policy, []*node{n0, n1})
		})

		t.Run("Without TTL", func(t *testing.T) {
			t.Parallel()

			policy := ltrPolicy{evict: createSentinel(t), evictZero: true}

			n0 := &node{Key: []byte("0")}
			n1 := &node{Key: []byte("1")}

			policy.OnInsert(n0)
			policy.OnInsert(n1)

			checkOrder(t, policy, []*node{n1, n0})
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

			checkOrder(t, policy, []*node{n1, n0})
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

			checkOrder(t, policy, []*node{n0, n1})
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
			if n0 != evictedNode {
				t.Errorf("expected %#v, got %#v", n0, evictedNode)
			}
		})

		t.Run("no evictZero", func(t *testing.T) {
			t.Parallel()

			policy := ltrPolicy{evict: createSentinel(t), evictZero: false}

			n0 := &node{Key: []byte("0")}
			n1 := &node{Key: []byte("1")}

			policy.OnInsert(n0)
			policy.OnInsert(n1)

			if policy.Evict() != nil {
				t.Errorf("expected nil, got %#v", policy.Evict())
			}
		})

		t.Run("Evict TTL", func(t *testing.T) {
			t.Parallel()

			policy := ltrPolicy{evict: createSentinel(t), evictZero: true}

			n0 := &node{Key: []byte("0"), Expiration: time.Now().Add(1 * time.Hour)}
			n1 := &node{Key: []byte("1"), Expiration: time.Now().Add(2 * time.Hour)}

			policy.OnInsert(n0)
			policy.OnInsert(n1)

			evictedNode := policy.Evict()

			if n1 != evictedNode {
				t.Errorf("expected %#v, got %#v", n0, evictedNode)
			}
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

			if n0 != evictedNode {
				t.Errorf("expected %#v, got %#v", n0, evictedNode)
			}
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

			if n1 != evictedNode {
				t.Errorf("expected %#v, got %#v", n0, evictedNode)
			}
		})

		t.Run("Empty List", func(t *testing.T) {
			t.Parallel()

			policy := ltrPolicy{evict: createSentinel(t), evictZero: true}
			if policy.Evict() != nil {
				t.Errorf("expected nil, got %#v", policy.Evict())
			}
		})
	})
}
