package cache

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStoreGetSet(t *testing.T) {
	t.Parallel()

	t.Run("Exists", func(t *testing.T) {
		t.Parallel()

		store := &New[any, any]().Store
		want := []byte("Value")
		store.Set([]byte("Key"), want, 0)
		got, _, ok := store.Get([]byte("Key"))
		assert.Equal(t, want, got)
		assert.True(t, ok)
	})

	t.Run("Not Exists", func(t *testing.T) {
		t.Parallel()

		store := &New[any, any]().Store
		_, _, ok := store.Get([]byte("Key"))
		assert.False(t, ok)
	})

	t.Run("Update", func(t *testing.T) {
		t.Parallel()

		store := &New[any, any]().Store
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

		store := &New[any, any]().Store
		want := []byte("Value")
		store.Set([]byte("Key"), want, 0)
		ok := store.Delete([]byte("Key"))
		assert.True(t, ok)
		_, _, ok = store.Get([]byte("Key"))
		assert.False(t, ok)
	})

	t.Run("Not Exists", func(t *testing.T) {
		t.Parallel()

		store := &New[any, any]().Store
		ok := store.Delete([]byte("Key"))
		assert.False(t, ok)
	})
}

func TestStoreClear(t *testing.T) {
	t.Parallel()

	store := &New[any, any]().Store
	want := []byte("Value")
	store.Set([]byte("Key"), want, 0)
	store.Clear()
	_, _, ok := store.Get([]byte("Key"))
	assert.False(t, ok)
}

func TestHashMapGetSet(t *testing.T) {
	t.Parallel()

	t.Run("Exists", func(t *testing.T) {
		t.Parallel()

		store := New[string, string]()
		want := "Value"
		store.Set("Key", want, 0)
		got, _, err := store.Get("Key")
		assert.Equal(t, want, got)
		assert.NoError(t, err)
	})

	t.Run("Not Exists", func(t *testing.T) {
		t.Parallel()

		store := New[string, string]()
		_, _, err := store.Get("Key")
		assert.ErrorIs(t, err, ErrKeyNotFound)
	})

	t.Run("Update", func(t *testing.T) {
		t.Parallel()

		store := New[string, string]()
		store.Set("Key", "Other", 0)
		want := "Value"
		store.Set("Key", want, 0)
		got, _, err := store.Get("Key")
		assert.Equal(t, want, got)
		assert.NoError(t, err)
	})
}

func TestHashMapDelete(t *testing.T) {
	t.Parallel()

	t.Run("Exists", func(t *testing.T) {
		t.Parallel()

		store := New[string, string]()
		want := "Value"
		store.Set("Key", want, 0)
		err := store.Delete("Key")
		assert.NoError(t, err)
	})

	t.Run("Not Exists", func(t *testing.T) {
		t.Parallel()

		store := New[string, string]()
		err := store.Delete("Key")
		assert.ErrorIs(t, err, ErrKeyNotFound)
	})
}

func BenchmarkStoreGet(b *testing.B) {
	store := &New[any, any]().Store
	key := []byte("Key")
	store.Set(key, []byte("Store"), 0)
	b.SetBytes(1)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			store.Get(key)
		}
	})
}

func BenchmarkStoreSet(b *testing.B) {
	store := &New[any, any]().Store
	key := []byte("Key")
	b.SetBytes(1)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			store.Set(key, []byte("Store"), 0)
		}
	})
}
