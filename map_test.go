package cache

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func setupTestStore(t testing.TB) *store {
	t.Helper()

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
		got, _, ok := store.Get([]byte("Key"))
		assert.Equal(t, want, got)
		assert.True(t, ok)
	})

	t.Run("Not Exists", func(t *testing.T) {
		t.Parallel()

		store := setupTestStore(t)

		_, _, ok := store.Get([]byte("Key"))
		assert.False(t, ok)
	})

	t.Run("Update", func(t *testing.T) {
		t.Parallel()

		store := setupTestStore(t)

		store.Set([]byte("Key"), []byte("Other"), 0)
		want := []byte("Value")
		store.Set([]byte("Key"), want, 0)
		got, _, ok := store.Get([]byte("Key"))
		assert.Equal(t, want, got)
		assert.True(t, ok)
	})
}

func TestStoreDelete(t *testing.T) {
	t.Parallel()

	t.Run("Exists", func(t *testing.T) {
		t.Parallel()

		store := setupTestStore(t)

		want := []byte("Value")
		store.Set([]byte("Key"), want, 0)
		ok := store.Delete([]byte("Key"))
		assert.True(t, ok)
		_, _, ok = store.Get([]byte("Key"))
		assert.False(t, ok)
	})

	t.Run("Not Exists", func(t *testing.T) {
		t.Parallel()

		store := setupTestStore(t)

		ok := store.Delete([]byte("Key"))
		assert.False(t, ok)
	})
}

func TestStoreClear(t *testing.T) {
	t.Parallel()

	store := setupTestStore(t)

	want := []byte("Value")
	store.Set([]byte("Key"), want, 0)
	store.Clear()
	_, _, ok := store.Get([]byte("Key"))
	assert.False(t, ok)
}

func BenchmarkStoreGet(b *testing.B) {
	store := setupTestStore(b)

	key := []byte("Key")
	store.Set(key, []byte("Store"), 0)
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		store.Get(key)
	}
}

func BenchmarkStoreSet(b *testing.B) {
	store := setupTestStore(b)

	key := []byte("Key")
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		store.Set(key, []byte("Store"), 0)
	}
}
