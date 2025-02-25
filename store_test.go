package cache

import (
	"bytes"
	"encoding/binary"
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
		store.Set([]byte("Key"), want, 1*time.Hour)
		got, ttl, ok := store.Get([]byte("Key"))
		if !ok {
			t.Errorf("expected key to exist")
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
		if !bytes.Equal(want, got) {
			t.Errorf("got %v, want %v", got, want)
		}
		if !ok {
			t.Errorf("expected key to exist")
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

func BenchmarkStoreGet(b *testing.B) {
	for n := 1; n <= 10000; n *= 10 {
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			want := setupTestStore(b)

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
}

func BenchmarkStoreSet(b *testing.B) {
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
