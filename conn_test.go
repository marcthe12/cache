package cache

import (
	"strconv"
	"testing"
	"time"

	"errors"
)

func setupTestDB[K any, V any](tb testing.TB) *DB[K, V] {
	tb.Helper()

	db, err := OpenMem[K, V]()
	if err != nil {
		tb.Fatalf("unexpected error: %v", err)
	}
	tb.Cleanup(func() {
		db.Close()
	})

	return &db
}

func TestDBGetSet(t *testing.T) {
	t.Parallel()

	t.Run("Exists", func(t *testing.T) {
		t.Parallel()

		db := setupTestDB[string, string](t)

		want := "Value"

		if err := db.Set("Key", want, 1*time.Hour); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		got, ttl, err := db.GetValue("Key")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if want != got {
			t.Fatalf("expected: %v, got: %v", want, got)
		}

		if ttl.Round(time.Second) != 1*time.Hour {
			t.Fatalf("expected duration %v, got: %v", time.Hour, ttl.Round(time.Second))
		}
	})

	t.Run("Not Exists", func(t *testing.T) {
		t.Parallel()

		db := setupTestDB[string, string](t)

		if _, _, err := db.GetValue("Key"); !errors.Is(err, ErrKeyNotFound) {
			t.Fatalf("expected error: %v, got: %v", ErrKeyNotFound, err)
		}
	})

	t.Run("Update", func(t *testing.T) {
		t.Parallel()

		db := setupTestDB[string, string](t)

		if err := db.Set("Key", "Other", 0); err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		want := "Value"
		if err := db.Set("Key", want, 0); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		got, _, err := db.GetValue("Key")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if want != got {
			t.Fatalf("expected: %v, got: %v", want, got)
		}
	})

	t.Run("Key Expiry", func(t *testing.T) {
		t.Parallel()

		db := setupTestDB[string, string](t)

		if err := db.Set("Key", "Value", 500*time.Millisecond); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		time.Sleep(600 * time.Millisecond)

		if _, _, err := db.GetValue("Key"); !errors.Is(err, ErrKeyNotFound) {
			t.Fatalf("expected error: %v, got: %v", ErrKeyNotFound, err)
		}
	})
}

func TestDBDelete(t *testing.T) {
	t.Parallel()

	t.Run("Exists", func(t *testing.T) {
		t.Parallel()

		db := setupTestDB[string, string](t)
		want := "Value"
		if err := db.Set("Key", want, 0); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if err := db.Delete("Key"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if _, _, err := db.GetValue("Key"); !errors.Is(err, ErrKeyNotFound) {
			t.Fatalf("expected error: %v, got: %v", ErrKeyNotFound, err)
		}
	})

	t.Run("Not Exists", func(t *testing.T) {
		t.Parallel()

		db := setupTestDB[string, string](t)

		if err := db.Delete("Key"); !errors.Is(err, ErrKeyNotFound) {
			t.Fatalf("expected error: %v, got: %v", ErrKeyNotFound, err)
		}
	})
}

func BenchmarkDBGet(b *testing.B) {
	for n := 1; n <= 10000; n *= 10 {
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			db := setupTestDB[int, int](b)
			for i := range n {
				db.Set(i, i, 0)
			}

			b.ReportAllocs()

			b.ResetTimer()

			for b.Loop() {
				db.GetValue(n - 1)
			}
		})
	}
}

func BenchmarkDBSet(b *testing.B) {
	for n := 1; n <= 10000; n *= 10 {
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			db := setupTestDB[int, int](b)
			for i := range n - 1 {
				db.Set(i, i, 0)
			}

			b.ReportAllocs()
			b.ResetTimer()

			for b.Loop() {
				db.Set(n, n, 0)
			}
		})
	}
}

func BenchmarkDBDelete(b *testing.B) {
	for n := 1; n <= 10000; n *= 10 {
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			db := setupTestDB[int, int](b)
			for i := range n - 1 {
				db.Set(i, i, 0)
			}

			b.ReportAllocs()
			b.ResetTimer()

			for b.Loop() {
				db.Set(n, n, 0)
				db.Delete(n)
			}
		})
	}
}
