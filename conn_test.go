package cache

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func setupTestDB[K any, V any](t testing.TB) *DB[K, V] {
	t.Helper()

	db, err := OpenMem[K, V]()
	assert.NoError(t, err)
	t.Cleanup(func() {
		db.Close()
	})
	return &db
func TestDBConcurrentAccess(t *testing.T) {
    db := setupTestDB[string, string](t)

    go func() {
        for i := 0; i < 100; i++ {
            db.Set(fmt.Sprintf("Key%d", i), "Value", 0)
        }
    }()

    go func() {
        for i := 0; i < 100; i++ {
            db.GetValue(fmt.Sprintf("Key%d", i))
        }
    }()

    // Allow some time for goroutines to complete
    time.Sleep(1 * time.Second)
}

func TestDBGetSet(t *testing.T) {
	t.Parallel()

	t.Run("Exists", func(t *testing.T) {
		t.Parallel()

		db := setupTestDB[string, string](t)

		want := "Value"
		err := db.Set("Key", want, 1*time.Hour)
		assert.NoError(t, err)

		got, ttl, err := db.GetValue("Key")
		assert.NoError(t, err)
		assert.Equal(t, want, got)

		now := time.Now()
		assert.WithinDuration(t, now.Add(ttl), now.Add(1*time.Hour), 1*time.Millisecond)
	})

	t.Run("Not Exists", func(t *testing.T) {
		t.Parallel()

		db := setupTestDB[string, string](t)

		_, _, err := db.GetValue("Key")
		assert.ErrorIs(t, err, ErrKeyNotFound)
	})

	t.Run("Update", func(t *testing.T) {
		t.Parallel()

		db := setupTestDB[string, string](t)

		err := db.Set("Key", "Other", 0)
		assert.NoError(t, err)

		want := "Value"
		err = db.Set("Key", want, 0)
		assert.NoError(t, err)

		got, _, err := db.GetValue("Key")
		assert.NoError(t, err)
		assert.Equal(t, want, got)
	})

	t.Run("Key Expiry", func(t *testing.T) {
		t.Parallel()

		db := setupTestDB[string, string](t)

		err := db.Set("Key", "Value", 500*time.Millisecond)
		assert.NoError(t, err)

		time.Sleep(600 * time.Millisecond)

		_, _, err = db.GetValue("Key")
		assert.ErrorIs(t, err, ErrKeyNotFound)
	})
}

func TestDBDelete(t *testing.T) {
	t.Parallel()

	t.Run("Exists", func(t *testing.T) {
		t.Parallel()

		db := setupTestDB[string, string](t)
		want := "Value"
		err := db.Set("Key", want, 0)
		assert.NoError(t, err)

		err = db.Delete("Key")
		assert.NoError(t, err)

		_, _, err = db.GetValue("Key")
		assert.ErrorIs(t, err, ErrKeyNotFound)
	})

	t.Run("Not Exists", func(t *testing.T) {
		t.Parallel()

		db := setupTestDB[string, string](t)

		err := db.Delete("Key")
		assert.ErrorIs(t, err, ErrKeyNotFound)
	})
}

func BenchmarkDBGet(b *testing.B) {
	for n := 1; n <= 10000; n *= 10 {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			db := setupTestDB[int, int](b)
			for i := 0; i < n; i++ {
				db.Set(i, i, 0)
			}
			b.ReportAllocs()

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				db.GetValue(n - 1)
			}
		})
	}
}

func BenchmarkDBSet(b *testing.B) {
	for n := 1; n <= 10000; n *= 10 {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			db := setupTestDB[int, int](b)
			for i := 0; i < n-1; i++ {
				db.Set(i, i, 0)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				db.Set(n, n, 0)
			}
		})
	}
}

func BenchmarkDBDelete(b *testing.B) {
	for n := 1; n <= 10000; n *= 10 {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			db := setupTestDB[int, int](b)
			for i := 0; i < n-1; i++ {
				db.Set(i, i, 0)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				db.Set(n, n, 0)
				db.Delete(n)
			}
		})
	}
}
