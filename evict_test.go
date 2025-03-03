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
			name:       "FIFO",
			policyType: PolicyFIFO,
			tests: []test{
				{
					name:       "OnInsert",
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
			},
		},
		{
			name:       "LRU",
			policyType: PolicyLRU,
			tests: []test{
				{
					name:       "OnInsert",
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
					name:       "OnAccess",
					flag:       true,
					numOfNodes: 2,
					actions: func(policy evictOrderedPolicy, nodes []*node) {
						policy.OnInsert(nodes[0])
						policy.OnInsert(nodes[1])

						policy.OnAccess(nodes[0])
					},
					expected: func(nodes []*node) []*node {
						return []*node{nodes[0], nodes[1]}
					},
				},
			},
		},
		{
			name:       "LFU",
			policyType: PolicyLFU,
			tests: []test{
				{
					name:       "OnInsert",
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
					name:       "OnAccess",
					flag:       true,
					numOfNodes: 2,
					actions: func(policy evictOrderedPolicy, nodes []*node) {
						policy.OnInsert(nodes[0])
						policy.OnInsert(nodes[1])
						policy.OnAccess(nodes[0])
					},
					expected: func(nodes []*node) []*node {
						return []*node{nodes[0], nodes[1]}
					},
				},
			},
		},
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
			name:       "FIFO",
			policyType: PolicyFIFO,
			tests: []test{
				{
					name:       "",
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
					name:       "Policy None",
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
		{
			name:       "LFU",
			policyType: PolicyLFU,
			tests: []test{
				{
					name:       "",
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
					name:       "Access",
					numOfNodes: 2,
					actions: func(policy evictOrderedPolicy, nodes []*node) {
						policy.OnInsert(nodes[0])
						policy.OnInsert(nodes[1])

						policy.OnAccess(nodes[0])

					},
					expected: func(nodes []*node) *node {
						return nodes[1]
					},
				},
				{
					name:       "Access Interleved",
					numOfNodes: 2,
					actions: func(policy evictOrderedPolicy, nodes []*node) {
						policy.OnInsert(nodes[0])

						policy.OnAccess(nodes[0])

						policy.OnInsert(nodes[1])
					},
					expected: func(nodes []*node) *node {
						return nodes[0]
					},
				},
				{
					name:       "Empty List",
					numOfNodes: 0,
					actions:    func(policy evictOrderedPolicy, nodes []*node) {},
					expected: func(nodes []*node) *node {
						return nil
					},
				},
			},
		},
		{
			name:       "LRU",
			policyType: PolicyLRU,
			tests: []test{
				{
					name:       "",
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
					name:       "Access",
					numOfNodes: 2,
					actions: func(policy evictOrderedPolicy, nodes []*node) {
						policy.OnInsert(nodes[0])
						policy.OnInsert(nodes[1])

						policy.OnAccess(nodes[0])

					},
					expected: func(nodes []*node) *node {
						return nodes[1]
					},
				},
				{
					name:       "Multiple Access",
					numOfNodes: 2,
					actions: func(policy evictOrderedPolicy, nodes []*node) {
						policy.OnInsert(nodes[0])
						policy.OnInsert(nodes[1])

						policy.OnAccess(nodes[0])

						policy.OnAccess(nodes[1])
						policy.OnAccess(nodes[1])
					},
					expected: func(nodes []*node) *node {
						return nodes[0]
					},
				},
				{
					name:       "Empty List",
					numOfNodes: 0,
					actions:    func(policy evictOrderedPolicy, nodes []*node) {},
					expected: func(nodes []*node) *node {
						return nil
					},
				},
			},
		},
		{
			name:       "LTR",
			policyType: PolicyLTR,
			tests: []test{
				{
					name:       "",
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

						nodes[1].Expiration = time.Now().Add(20 * time.Minute)
						policy.OnUpdate(nodes[1])
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
						t.Errorf("expected\n %#v\n got %#v", tt.expected(nodes), evictedNode)
					}
				})
			}
		})
	}
}

func TestSetPolicy(t *testing.T) {
	t.Parallel()

	type test struct {
		name         string
		policyType   EvictionPolicyType
		expectedType EvictionPolicyType
		expectedErr  error
	}

	tests := []test{
		{
			name:         "PolicyNone",
			policyType:   PolicyNone,
			expectedType: PolicyNone,
			expectedErr:  nil,
		},
		{
			name:         "PolicyFIFO",
			policyType:   PolicyFIFO,
			expectedType: PolicyFIFO,
			expectedErr:  nil,
		},
		{
			name:         "PolicyLRU",
			policyType:   PolicyLRU,
			expectedType: PolicyLRU,
			expectedErr:  nil,
		},
		{
			name:         "PolicyLFU",
			policyType:   PolicyLFU,
			expectedType: PolicyLFU,
			expectedErr:  nil,
		},
		{
			name:         "PolicyLTR",
			policyType:   PolicyLTR,
			expectedType: PolicyLTR,
			expectedErr:  nil,
		},
		{
			name:         "InvalidPolicy",
			policyType:   EvictionPolicyType(999), // Invalid policy type
			expectedType: PolicyNone,              // Default type
			expectedErr:  ErrInvalidPolicy,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			policy := &evictionPolicy{
				Sentinel: createSentinel(t),
				ListLock: &sync.RWMutex{},
			}

			err := policy.SetPolicy(tt.policyType)
			if err != tt.expectedErr {
				t.Errorf("expected error %v, got %v", tt.expectedErr, err)
			}

			if policy.Type != tt.expectedType {
				t.Errorf("expected policy type %v, got %v", tt.expectedType, policy.Type)
			}
		})
	}
}

func TestSetPolicyMultipleTimes(t *testing.T) {
	t.Parallel()

	policy := &evictionPolicy{
		Sentinel: createSentinel(t),
		ListLock: &sync.RWMutex{},
	}

	// Set policy to FIFO
	err := policy.SetPolicy(PolicyFIFO)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if policy.Type != PolicyFIFO {
		t.Errorf("expected policy type %v, got %v", PolicyFIFO, policy.Type)
	}

	// Set policy to LRU
	err = policy.SetPolicy(PolicyLRU)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if policy.Type != PolicyLRU {
		t.Errorf("expected policy type %v, got %v", PolicyLRU, policy.Type)
	}

	// Set policy to LFU
	err = policy.SetPolicy(PolicyLFU)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if policy.Type != PolicyLFU {
		t.Errorf("expected policy type %v, got %v", PolicyLFU, policy.Type)
	}

	// Set policy to LTR
	err = policy.SetPolicy(PolicyLTR)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if policy.Type != PolicyLTR {
		t.Errorf("expected policy type %v, got %v", PolicyLTR, policy.Type)
	}

	// Set policy to None
	err = policy.SetPolicy(PolicyNone)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if policy.Type != PolicyNone {
		t.Errorf("expected policy type %v, got %v", PolicyNone, policy.Type)
	}
}
