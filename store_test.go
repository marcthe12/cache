package cache

import (
	"bytes"
	"encoding/binary"
	"errors"
	"strconv"
	"testing"
	"time"
)

func setupTestStore(tb testing.TB) *store {
	tb.Helper()

	store := &store{}
	store.Init()

	return store
}

func TestStoreGetSet(t *testing.T) {
	t.Parallel()

	t.Run("Exists", func(t *testing.T) {
		t.Parallel()

		store := setupTestStore(t)

		want := []byte("Value")
		store.Set([]byte("Key"), want, 0)
		got, ttl, ok := store.Get([]byte("Key"))
		if !ok {
			t.Fatalf("expected key to exist")
		}
		if !bytes.Equal(want, got) {
			t.Errorf("got %v, want %v", got, want)
		}
		if ttl.Round(time.Second) != 0 {
			t.Errorf("ttl same: got %v expected %v", ttl.Round(time.Second), 1*time.Hour)
		}

	})

	t.Run("Exists Non Expiry", func(t *testing.T) {
		t.Parallel()

		store := setupTestStore(t)

		want := []byte("Value")
		store.Set([]byte("Key"), want, 1*time.Hour)
		got, ttl, ok := store.Get([]byte("Key"))
		if !ok {
			t.Fatalf("expected key to exist")
		}
		if !bytes.Equal(want, got) {
			t.Errorf("got %v, want %v", got, want)
		}
		if ttl.Round(time.Second) != 1*time.Hour {
			t.Errorf("ttl same: got %v expected %v", ttl.Round(time.Second), 1*time.Hour)
		}

	})

	t.Run("Exists TTL", func(t *testing.T) {
		t.Parallel()

		store := setupTestStore(t)

		want := []byte("Value")
		store.Set([]byte("Key"), want, time.Nanosecond)
		if _, _, ok := store.Get([]byte("Key")); ok {
			t.Errorf("expected key to not exist")
		}
	})

	t.Run("Not Exists", func(t *testing.T) {
		t.Parallel()

		store := setupTestStore(t)
		if _, _, ok := store.Get([]byte("Key")); ok {
			t.Errorf("expected key to not exist")
		}
	})

	t.Run("Update", func(t *testing.T) {
		t.Parallel()

		store := setupTestStore(t)

		store.Set([]byte("Key"), []byte("Other"), 0)

		want := []byte("Value")
		store.Set([]byte("Key"), want, 0)
		got, _, ok := store.Get([]byte("Key"))
		if !ok {
			t.Fatal("expected key to exist")
		}
		if !bytes.Equal(want, got) {
			t.Errorf("got %v, want %v", got, want)
		}

	})

	t.Run("Resize", func(t *testing.T) {
		t.Parallel()

		store := setupTestStore(t)

		for i := range initialBucketSize {
			key := binary.LittleEndian.AppendUint64(nil, i)
			store.Set(key, key, 0)
		}

		for i := range store.Length {
			key := binary.LittleEndian.AppendUint64(nil, i)
			if _, _, ok := store.Get(key); !ok {
				t.Errorf("expected key %v to exist", i)
			}
		}

		if len(store.Bucket) != int(initialBucketSize)*2 {
			t.Errorf("expected bucket size to be %v, got %v", initialBucketSize*2, len(store.Bucket))
		}

		for i := range store.Length {
			key := binary.LittleEndian.AppendUint64(nil, i)
			if _, _, ok := store.Get(key); !ok {
				t.Errorf("expected key %d to exist", i)
			}
		}
	})
}

func TestStoreDelete(t *testing.T) {
	t.Parallel()

	t.Run("Exists", func(t *testing.T) {
		t.Parallel()

		store := setupTestStore(t)

		want := []byte("Value")
		store.Set([]byte("Key"), want, 0)

		if !store.Delete([]byte("Key")) {
			t.Errorf("expected key to be deleted")
		}
		if _, _, ok := store.Get([]byte("Key")); ok {
			t.Errorf("expected key to not exist")
		}
	})

	t.Run("Not Exists", func(t *testing.T) {
		t.Parallel()

		store := setupTestStore(t)

		if store.Delete([]byte("Key")) {
			t.Errorf("expected key to not exist")
		}
	})
}

func TestStoreClear(t *testing.T) {
	t.Parallel()

	store := setupTestStore(t)

	want := []byte("Value")
	store.Set([]byte("Key"), want, 0)
	store.Clear()
	if _, _, ok := store.Get([]byte("Key")); ok {
		t.Errorf("expected key to not exist")
	}
}

func TestStoreUpdateInPlace(t *testing.T) {
	t.Parallel()

	t.Run("Exists", func(t *testing.T) {
		t.Parallel()

		store := setupTestStore(t)

		want := []byte("Value")
		store.Set([]byte("Key"), []byte("Initial"), 1*time.Hour)

		processFunc := func(v []byte) ([]byte, error) {
			return want, nil
		}

		if err := store.UpdateInPlace([]byte("Key"), processFunc, 1*time.Hour); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		got, _, ok := store.Get([]byte("Key"))
		if !ok {
			t.Fatalf("expected key to exist")
		}
		if !bytes.Equal(want, got) {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("Not Exists", func(t *testing.T) {
		t.Parallel()

		store := setupTestStore(t)

		processFunc := func(v []byte) ([]byte, error) {
			return []byte("Value"), nil
		}

		if err := store.UpdateInPlace([]byte("Key"), processFunc, 1*time.Hour); !errors.Is(err, ErrKeyNotFound) {
			t.Fatalf("expected error: %v, got: %v", ErrKeyNotFound, err)
		}
	})
}

func TestStoreMemoize(t *testing.T) {
	t.Parallel()

	t.Run("Cache Miss", func(t *testing.T) {
		t.Parallel()

		store := setupTestStore(t)

		factoryFunc := func() ([]byte, error) {
			return []byte("Value"), nil
		}

		got, err := store.Memorize([]byte("Key"), factoryFunc, 1*time.Hour)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !bytes.Equal(got, []byte("Value")) {
			t.Fatalf("expected: %v, got: %v", "Value", got)
		}

		got, _, ok := store.Get([]byte("Key"))
		if !ok {
			t.Fatalf("expected key to exist")
		}
		if !bytes.Equal(got, []byte("Value")) {
			t.Fatalf("expected: %v, got: %v", "Value", got)
		}
	})

	t.Run("Cache Hit", func(t *testing.T) {
		t.Parallel()

		store := setupTestStore(t)

		store.Set([]byte("Key"), []byte("Value"), 1*time.Hour)

		factoryFunc := func() ([]byte, error) {
			return []byte("NewValue"), nil
		}

		got, err := store.Memorize([]byte("Key"), factoryFunc, 1*time.Hour)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !bytes.Equal(got, []byte("Value")) {
			t.Fatalf("expected: %v, got: %v", "Value", got)
		}
	})
}

func TestStoreCleanup(t *testing.T) {
	t.Parallel()

	t.Run("Cleanup Expired", func(t *testing.T) {
		t.Parallel()

		store := setupTestStore(t)

		store.Set([]byte("1"), []byte("1"), 500*time.Millisecond)
		store.Set([]byte("2"), []byte("2"), 1*time.Hour)

		time.Sleep(600 * time.Millisecond)

		store.Cleanup()

		if _, _, ok := store.Get([]byte("1")); ok {
			t.Fatalf("expected 1 to not exist")
		}

		if _, _, ok := store.Get([]byte("2")); !ok {
			t.Fatalf("expected 2 to exist")
		}
	})

	t.Run("No Cleanup", func(t *testing.T) {
		t.Parallel()

		store := setupTestStore(t)

		store.Set([]byte("Key"), []byte("Value"), 1*time.Hour)

		// No cleanup should occur
		store.Cleanup()

		if _, _, ok := store.Get([]byte("Key")); !ok {
			t.Fatalf("expected key to exist")
		}
	})
}

func TestStoreEvict(t *testing.T) {
	t.Parallel()

	t.Run("Evict FIFO", func(t *testing.T) {
		t.Parallel()

		store := setupTestStore(t)
		if err := store.Policy.SetPolicy(PolicyFIFO); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		store.MaxCost = 5

		store.Set([]byte("1"), []byte("1"), 0)
		store.Set([]byte("2"), []byte("2"), 0)

		// Trigger eviction
		store.Set([]byte("3"), []byte("3"), 0)
		store.Evict()

		if _, _, ok := store.Get([]byte("1")); ok {
			t.Fatalf("expected key 1 to not exist")
		}

		if _, _, ok := store.Get([]byte("2")); !ok {
			t.Fatalf("expected key 2 to exist")
		}
	})

	t.Run("No Evict", func(t *testing.T) {
		t.Parallel()

		store := setupTestStore(t)
		if err := store.Policy.SetPolicy(PolicyFIFO); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		store.MaxCost = 10

		store.Set([]byte("1"), []byte("1"), 0)
		store.Set([]byte("2"), []byte("2"), 0)

		// No eviction should occur
		store.Set([]byte("3"), []byte("3"), 0)
		store.Evict()

		if _, _, ok := store.Get([]byte("1")); !ok {
			t.Fatalf("expected key 1 to exist")
		}

		if _, _, ok := store.Get([]byte("2")); !ok {
			t.Fatalf("expected key 2 to exist")
		}
	})

	t.Run("No Evict PolicyNone", func(t *testing.T) {
		t.Parallel()

		store := setupTestStore(t)
		if err := store.Policy.SetPolicy(PolicyNone); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		store.MaxCost = 5

		store.Set([]byte("1"), []byte("1"), 0)
		store.Set([]byte("2"), []byte("2"), 0)

		// No eviction should occur
		store.Set([]byte("3"), []byte("3"), 0)
		store.Evict()

		if _, _, ok := store.Get([]byte("1")); !ok {
			t.Fatalf("expected key 1 to exist")
		}

		if _, _, ok := store.Get([]byte("2")); !ok {
			t.Fatalf("expected key 2 to exist")
		}
	})

	t.Run("No Evict MaxCost Zero", func(t *testing.T) {
		t.Parallel()

		store := setupTestStore(t)
		if err := store.Policy.SetPolicy(PolicyFIFO); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		store.MaxCost = 0

		store.Set([]byte("1"), []byte("1"), 0)
		store.Set([]byte("2"), []byte("2"), 0)

		store.Evict()

		if _, _, ok := store.Get([]byte("1")); !ok {
			t.Fatalf("expected key 1 to exist")
		}

		if _, _, ok := store.Get([]byte("2")); !ok {
			t.Fatalf("expected key 2 to exist")
		}
	})
}

func BenchmarkStoreGet(b *testing.B) {
	policy := map[string]EvictionPolicyType{
		"None": PolicyNone,
		"FIFO": PolicyFIFO,
		"LRU":  PolicyLRU,
		"LFU":  PolicyLFU,
		"LTR":  PolicyLTR,
	}
	for k, v := range policy {
		b.Run(k, func(b *testing.B) {
			for n := 1; n <= 10000; n *= 10 {
				b.Run(strconv.Itoa(n), func(b *testing.B) {
					want := setupTestStore(b)

					if err := want.Policy.SetPolicy(v); err != nil {
						b.Fatalf("unexpected error: %v", err)
					}

					for i := range n - 1 {
						buf := make([]byte, 8)
						binary.LittleEndian.PutUint64(buf, uint64(i))
						want.Set(buf, buf, 0)
					}

					key := []byte("Key")
					want.Set(key, []byte("Store"), 0)
					b.ReportAllocs()

					b.ResetTimer()

					for b.Loop() {
						want.Get(key)
					}
				})
			}
		})
	}
}

func BenchmarkStoreGetParallel(b *testing.B) {
	policy := map[string]EvictionPolicyType{
		"None": PolicyNone,
		"FIFO": PolicyFIFO,
		"LRU":  PolicyLRU,
		"LFU":  PolicyLFU,
		"LTR":  PolicyLTR,
	}
	for k, v := range policy {
		b.Run(k, func(b *testing.B) {
			for n := 1; n <= 10000; n *= 10 {
				b.Run(strconv.Itoa(n), func(b *testing.B) {
					want := setupTestStore(b)

					if err := want.Policy.SetPolicy(v); err != nil {
						b.Fatalf("unexpected error: %v", err)
					}

					for i := range n - 1 {
						buf := make([]byte, 8)
						binary.LittleEndian.PutUint64(buf, uint64(i))
						want.Set(buf, buf, 0)
					}

					key := []byte("Key")
					want.Set(key, []byte("Store"), 0)
					b.ReportAllocs()

					b.ResetTimer()

					b.RunParallel(func(pb *testing.PB) {
						for pb.Next() {
							want.Get(key)
						}
					})
				})
			}
		})
	}
}

func BenchmarkStoreSet(b *testing.B) {
	policy := map[string]EvictionPolicyType{
		"None": PolicyNone,
		"FIFO": PolicyFIFO,
		"LRU":  PolicyLRU,
		"LFU":  PolicyLFU,
		"LTR":  PolicyLTR,
	}
	for k, v := range policy {
		b.Run(k, func(b *testing.B) {
			for n := 1; n <= 10000; n *= 10 {
				b.Run(strconv.Itoa(n), func(b *testing.B) {
					want := setupTestStore(b)

					if err := want.Policy.SetPolicy(v); err != nil {
						b.Fatalf("unexpected error: %v", err)
					}

					for i := range n - 1 {
						buf := make([]byte, 8)
						binary.LittleEndian.PutUint64(buf, uint64(i))
						want.Set(buf, buf, 0)
					}

					key := []byte("Key")
					store := []byte("Store")

					b.ReportAllocs()
					b.ResetTimer()

					b.RunParallel(func(pb *testing.PB) {
						for pb.Next() {
							want.Set(key, store, 0)
						}
					})
				})
			}
		})
	}
}

func BenchmarkStoreSetParallel(b *testing.B) {
	policy := map[string]EvictionPolicyType{
		"None": PolicyNone,
		"FIFO": PolicyFIFO,
		"LRU":  PolicyLRU,
		"LFU":  PolicyLFU,
		"LTR":  PolicyLTR,
	}
	for k, v := range policy {
		b.Run(k, func(b *testing.B) {
			for n := 1; n <= 10000; n *= 10 {
				b.Run(strconv.Itoa(n), func(b *testing.B) {
					want := setupTestStore(b)

					if err := want.Policy.SetPolicy(v); err != nil {
						b.Fatalf("unexpected error: %v", err)
					}

					for i := range n - 1 {
						buf := make([]byte, 8)
						binary.LittleEndian.PutUint64(buf, uint64(i))
						want.Set(buf, buf, 0)
					}

					key := []byte("Key")
					store := []byte("Store")

					b.ReportAllocs()
					b.ResetTimer()

					for b.Loop() {
						want.Set(key, store, 0)
					}
				})
			}
		})
	}
}

func BenchmarkStoreSetInsert(b *testing.B) {
	policy := map[string]EvictionPolicyType{
		"None": PolicyNone,
		"FIFO": PolicyFIFO,
		"LRU":  PolicyLRU,
		"LFU":  PolicyLFU,
		"LTR":  PolicyLTR,
	}
	for k, v := range policy {
		b.Run(k, func(b *testing.B) {
			for n := 1; n <= 10000; n *= 10 {
				b.Run(strconv.Itoa(n), func(b *testing.B) {
					want := setupTestStore(b)

					if err := want.Policy.SetPolicy(v); err != nil {
						b.Fatalf("unexpected error: %v", err)
					}

					list := make([][]byte, n)
					for i := range n {
						buf := make([]byte, 8)
						binary.LittleEndian.PutUint64(buf, uint64(i))
						list = append(list, buf)
					}

					b.ReportAllocs()
					b.ResetTimer()

					for b.Loop() {
						for _, k := range list {
							want.Set(k, k, 0)
						}
					}
				})
			}
		})
	}
}

func BenchmarkStoreDelete(b *testing.B) {
	for n := 1; n <= 10000; n *= 10 {
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			want := setupTestStore(b)

			for i := range n - 1 {
				buf := make([]byte, 8)
				binary.LittleEndian.PutUint64(buf, uint64(i))
				want.Set(buf, buf, 0)
			}

			key := []byte("Key")
			store := []byte("Store")

			b.ReportAllocs()
			b.ResetTimer()

			for b.Loop() {
				want.Set(key, store, 0)
				want.Delete(key)
			}
		})
	}
}
