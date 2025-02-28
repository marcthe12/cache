package cache

import (
	"strconv"
	"sync"
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

func createPolicy(tb testing.TB, policyType EvictionPolicyType, flag bool) evictOrderedPolicy {
	tb.Helper()

	switch policyType {
	case PolicyFIFO:
		return &fifoPolicy{List: createSentinel(tb), ShouldEvict: flag, Lock: &sync.RWMutex{}}
	case PolicyLTR:
		return &ltrPolicy{List: createSentinel(tb), EvictZero: flag, Lock: &sync.RWMutex{}}
	case PolicyLRU:
		return &lruPolicy{List: createSentinel(tb), Lock: &sync.RWMutex{}}
	case PolicyLFU:
		return &lfuPolicy{List: createSentinel(tb), Lock: &sync.RWMutex{}}
	}
	tb.Fatalf("unknown policy type: %v", policyType)
	return nil
}

func getListOrder(tb testing.TB, evict *node) []*node {
	tb.Helper()

	var order []*node

	for current := evict.EvictNext; current != evict; current = current.EvictNext {
		order = append(order, current)
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
			tb.Errorf("element %v did not match: expected: %#v got: %#v", i, string(n.Key), string(order[i].Key))
		}
	}
}

func TestFIFOPolicy(t *testing.T) {
	t.Parallel()

	t.Run("OnInsert", func(t *testing.T) {
		t.Parallel()

		policy := fifoPolicy{List: createSentinel(t), ShouldEvict: true, Lock: &sync.RWMutex{}}

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

			policy := fifoPolicy{List: createSentinel(t), ShouldEvict: true, Lock: &sync.RWMutex{}}

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

			policy := fifoPolicy{List: createSentinel(t), ShouldEvict: false, Lock: &sync.RWMutex{}}

			policy.OnInsert(&node{})

			if policy.Evict() != nil {
				t.Errorf("expected nil, got %#v", policy.Evict())
			}
		})

		t.Run("Empty List", func(t *testing.T) {
			t.Parallel()

			policy := fifoPolicy{List: createSentinel(t), ShouldEvict: true, Lock: &sync.RWMutex{}}
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

		policy := lruPolicy{List: createSentinel(t), Lock: &sync.RWMutex{}}

		n0 := &node{Key: []byte("0")}
		n1 := &node{Key: []byte("1")}

		policy.OnInsert(n0)
		policy.OnInsert(n1)

		checkOrder(t, policy, []*node{n1, n0})
	})

	t.Run("OnAccess", func(t *testing.T) {
		t.Parallel()

		policy := lruPolicy{List: createSentinel(t), Lock: &sync.RWMutex{}}

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

			policy := lruPolicy{List: createSentinel(t), Lock: &sync.RWMutex{}}

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

			policy := lruPolicy{List: createSentinel(t), Lock: &sync.RWMutex{}}

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

			policy := lruPolicy{List: createSentinel(t), Lock: &sync.RWMutex{}}

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

			policy := lruPolicy{List: createSentinel(t), Lock: &sync.RWMutex{}}
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

		policy := lfuPolicy{List: createSentinel(t), Lock: &sync.RWMutex{}}

		n0 := &node{Key: []byte("0")}
		n1 := &node{Key: []byte("1")}

		policy.OnInsert(n0)
		policy.OnInsert(n1)

		checkOrder(t, policy, []*node{n1, n0})
	})

	t.Run("OnAccess", func(t *testing.T) {
		t.Parallel()

		policy := lfuPolicy{List: createSentinel(t), Lock: &sync.RWMutex{}}

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

			policy := lfuPolicy{List: createSentinel(t), Lock: &sync.RWMutex{}}

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

			policy := lfuPolicy{List: createSentinel(t), Lock: &sync.RWMutex{}}

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

			policy := lfuPolicy{List: createSentinel(t), Lock: &sync.RWMutex{}}
			if policy.Evict() != nil {
				t.Errorf("expected nil, got %#v", policy.Evict())
			}
		})
	})
}

func TestPolicyHooks(t *testing.T) {
	t.Parallel()

	type test struct {
		name       string
		flag       bool
		numOfNodes int
		actions    func(policy evictOrderedPolicy, nodes []*node)
		expected   func(nodes []*node) []*node
	}

	tests := []struct {
		name       string
		policyType EvictionPolicyType
		tests      []test
	}{
		{
			name:       "LTR",
			policyType: PolicyLTR,
			tests: []test{
				{
					name:       "OnInsert With TTL",
					flag:       true,
					numOfNodes: 2,
					actions: func(policy evictOrderedPolicy, nodes []*node) {
						nodes[0].Expiration = time.Now().Add(1 * time.Hour)
						nodes[1].Expiration = time.Now().Add(2 * time.Hour)

						policy.OnInsert(nodes[0])
						policy.OnInsert(nodes[1])
					},
					expected: func(nodes []*node) []*node {
						return []*node{nodes[0], nodes[1]}
					},
				},
				{
					name:       "OnInsert Without TTL",
					flag:       true,
					numOfNodes: 2,
					actions: func(policy evictOrderedPolicy, nodes []*node) {
						policy.OnInsert(nodes[0])
						policy.OnInsert(nodes[1])
					},
					expected: func(nodes []*node) []*node {
						return []*node{nodes[1], nodes[0]}
					},
				},
				{
					name:       "OnUpdate With TTL",
					flag:       true,
					numOfNodes: 2,
					actions: func(policy evictOrderedPolicy, nodes []*node) {
						nodes[0].Expiration = time.Now().Add(1 * time.Hour)
						nodes[1].Expiration = time.Now().Add(2 * time.Hour)
						policy.OnInsert(nodes[0])
						policy.OnInsert(nodes[1])

						nodes[0].Expiration = time.Now().Add(3 * time.Hour)
						policy.OnUpdate(nodes[0])
					},
					expected: func(nodes []*node) []*node {
						return []*node{nodes[1], nodes[0]}
					},
				},
				{
					name:       "OnUpdate With TTL Decrease",
					flag:       true,
					numOfNodes: 2,
					actions: func(policy evictOrderedPolicy, nodes []*node) {
						nodes[0].Expiration = time.Now().Add(1 * time.Hour)
						nodes[1].Expiration = time.Now().Add(2 * time.Hour)
						policy.OnInsert(nodes[0])
						policy.OnInsert(nodes[1])

						nodes[0].Expiration = time.Now().Add(20 * time.Minute)
						policy.OnUpdate(nodes[0])
					},
					expected: func(nodes []*node) []*node {
						return []*node{nodes[0], nodes[1]}
					},
				},
			},
		},
	}

	for _, ts := range tests {
		t.Run(ts.name, func(t *testing.T) {
			t.Parallel()

			for _, tt := range ts.tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					policy := createPolicy(t, ts.policyType, tt.flag)

					nodes := make([]*node, tt.numOfNodes)
					for i := range nodes {
						nodes[i] = &node{Key: []byte(strconv.Itoa(i))}
					}

					tt.actions(policy, nodes)

					checkOrder(t, policy, tt.expected(nodes))
				})
			}
		})
	}
}

func TestPolicyEvict(t *testing.T) {
	t.Parallel()

	type test struct {
		name       string
		flag       bool
		numOfNodes int
		actions    func(policy evictOrderedPolicy, nodes []*node)
		expected   func(nodes []*node) *node
	}

	tests := []struct {
		name       string
		policyType EvictionPolicyType
		tests      []test
	}{
		{
			name:       "LTR",
			policyType: PolicyLTR,
			tests: []test{
				{
					name:       "Evict",
					flag:       true,
					numOfNodes: 2,
					actions: func(policy evictOrderedPolicy, nodes []*node) {
						policy.OnInsert(nodes[0])
						policy.OnInsert(nodes[1])
					},
					expected: func(nodes []*node) *node {
						return nodes[0]
					},
				},
				{
					name:       "no evictZero",
					flag:       false,
					numOfNodes: 2,
					actions: func(policy evictOrderedPolicy, nodes []*node) {
						policy.OnInsert(nodes[0])
						policy.OnInsert(nodes[1])
					},
					expected: func(nodes []*node) *node {
						return nil
					},
				},
				{
					name:       "Evict TTL",
					flag:       true,
					numOfNodes: 2,
					actions: func(policy evictOrderedPolicy, nodes []*node) {
						nodes[0].Expiration = time.Now().Add(1 * time.Hour)
						nodes[1].Expiration = time.Now().Add(2 * time.Hour)

						policy.OnInsert(nodes[0])
						policy.OnInsert(nodes[1])
					},
					expected: func(nodes []*node) *node {
						return nodes[1]
					},
				},
				{
					name:       "Evict TTL Update",
					flag:       true,
					numOfNodes: 2,
					actions: func(policy evictOrderedPolicy, nodes []*node) {
						nodes[0].Expiration = time.Now().Add(1 * time.Hour)
						nodes[1].Expiration = time.Now().Add(2 * time.Hour)

						policy.OnInsert(nodes[0])
						policy.OnInsert(nodes[1])

						nodes[0].Expiration = time.Now().Add(3 * time.Hour)
						policy.OnUpdate(nodes[0])
					},
					expected: func(nodes []*node) *node {
						return nodes[0]
					},
				},
				{
					name:       "Evict TTL Update Down",
					flag:       true,
					numOfNodes: 2,
					actions: func(policy evictOrderedPolicy, nodes []*node) {
						nodes[0].Expiration = time.Now().Add(1 * time.Hour)
						nodes[1].Expiration = time.Now().Add(2 * time.Hour)

						policy.OnInsert(nodes[0])
						policy.OnInsert(nodes[1])

						nodes[0].Expiration = time.Now().Add(20 * time.Minute)
						policy.OnUpdate(nodes[0])
					},
					expected: func(nodes []*node) *node {
						return nodes[1]
					},
				},
				{
					name:       "Empty List",
					flag:       true,
					numOfNodes: 0,
					actions:    func(policy evictOrderedPolicy, nodes []*node) {},
					expected: func(nodes []*node) *node {
						return nil
					},
				},
			},
		},
	}

	for _, ts := range tests {
		t.Run(ts.name, func(t *testing.T) {
			t.Parallel()

			for _, tt := range ts.tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					policy := createPolicy(t, ts.policyType, tt.flag)

					nodes := make([]*node, tt.numOfNodes)
					for i := range nodes {
						nodes[i] = &node{Key: []byte(strconv.Itoa(i))}
					}

					tt.actions(policy, nodes)

					evictedNode := policy.Evict()
					if evictedNode != tt.expected(nodes) {
						t.Errorf("expected %#v, got %#v", tt.expected(nodes), evictedNode)
					}
				})
			}
		})
	}
}
