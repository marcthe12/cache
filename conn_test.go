package cache

import (
	"errors"
	"strconv"
	"testing"
	"time"
)

func setupTestCache[K, V any](tb testing.TB) *Cache[K, V] {
	tb.Helper()

	db, err := OpenMem[K, V]()
	if err != nil {
		tb.Fatalf("unexpected error: %v", err)
	}

	tb.Cleanup(func() {
		if err := db.Close(); err != nil {
			tb.Fatalf("unexpected error: %v", err)
		}
	})

	return &db
}

func TestCacheSetConfig(t *testing.T) {
	tests := []struct {
		name            string
		options         []Option
		wantErr         bool
		expectedPolicy  EvictionPolicyType
		expectedMaxCost uint64
		snapshotTime    time.Duration
		cleanupTime     time.Duration
	}{
		{
			name: "Set all valid options",
			options: []Option{
				WithPolicy(PolicyLRU),
				WithMaxCost(10000),
				SetSnapshotTime(2 * time.Minute),
				SetCleanupTime(30 * time.Second),
			},
			wantErr:         false,
			expectedPolicy:  PolicyLRU,
			expectedMaxCost: 10000,
			snapshotTime:    2 * time.Minute,
			cleanupTime:     30 * time.Second,
		},
		{
			name: "Invalid policy returns error",
			options: []Option{
				WithPolicy(-1),
			},
			wantErr: true,
		},
		{
			name: "Set only max cost",
			options: []Option{
				WithMaxCost(2048),
			},
			wantErr:         false,
			expectedMaxCost: 2048,
		},
		{
			name: "Set only snapshot and cleanup",
			options: []Option{
				SetSnapshotTime(15 * time.Second),
				SetCleanupTime(1 * time.Minute),
			},
			wantErr:      false,
			snapshotTime: 15 * time.Second,
			cleanupTime:  1 * time.Minute,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := setupTestCache[string, string](t)

			err := c.SetConfig(tt.options...)
			if (err != nil) != tt.wantErr {
				t.Fatalf("SetConfig() error = %v, wantErr = %v", err, tt.wantErr)
			}

			if !tt.wantErr {
				if c.Store.Policy.Type != tt.expectedPolicy {
					t.Errorf("Expected policy %v, got %v", tt.expectedPolicy, c.Store.Policy.Type)
				}
				if tt.expectedMaxCost != 0 && c.Store.MaxCost != tt.expectedMaxCost {
					t.Errorf("Expected MaxCost %d, got %d", tt.expectedMaxCost, c.Store.MaxCost)
				}
				if tt.snapshotTime != 0 && c.Store.SnapshotTicker.GetDuration() != tt.snapshotTime {
					t.Errorf("Expected SnapshotTime %v, got %v", tt.snapshotTime, c.Store.SnapshotTicker.GetDuration())
				}
				if tt.cleanupTime != 0 && c.Store.CleanupTicker.GetDuration() != tt.cleanupTime {
					t.Errorf("Expected CleanupTime %v, got %v", tt.cleanupTime, c.Store.CleanupTicker.GetDuration())
				}
			}
		})
	}
}

func TestCacheGetSet(t *testing.T) {
	t.Parallel()

	t.Run("Exists", func(t *testing.T) {
		t.Parallel()

		db := setupTestCache[string, string](t)

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

		db := setupTestCache[string, string](t)

		if _, _, err := db.GetValue("Key"); !errors.Is(err, ErrKeyNotFound) {
			t.Fatalf("expected error: %v, got: %v", ErrKeyNotFound, err)
		}
	})

	t.Run("Update", func(t *testing.T) {
		t.Parallel()

		db := setupTestCache[string, string](t)

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

		db := setupTestCache[string, string](t)

		if err := db.Set("Key", "Value", 500*time.Millisecond); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		time.Sleep(600 * time.Millisecond)

		if _, _, err := db.GetValue("Key"); !errors.Is(err, ErrKeyNotFound) {
			t.Fatalf("expected error: %v, got: %v", ErrKeyNotFound, err)
		}
	})
}

func TestCacheDelete(t *testing.T) {
	t.Parallel()

	t.Run("Exists", func(t *testing.T) {
		t.Parallel()

		db := setupTestCache[string, string](t)
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

		db := setupTestCache[string, string](t)

		if err := db.Delete("Key"); !errors.Is(err, ErrKeyNotFound) {
			t.Fatalf("expected error: %v, got: %v", ErrKeyNotFound, err)
		}
	})
}

func TestCacheUpdateInPlace(t *testing.T) {
	t.Parallel()

	t.Run("Exists", func(t *testing.T) {
		t.Parallel()

		store := setupTestCache[string, string](t)

		want := "Value"

		if err := store.Set("Key", "Initial", 1*time.Hour); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		processFunc := func(v string) (string, error) {
			return want, nil
		}

		if err := store.UpdateInPlace("Key", processFunc, 1*time.Hour); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		got, _, err := store.GetValue("Key")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if want != got {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("Not Exists", func(t *testing.T) {
		t.Parallel()

		store := setupTestCache[string, string](t)

		want := "Value"

		processFunc := func(v string) (string, error) {
			return want, nil
		}

		if err := store.UpdateInPlace("Key", processFunc, 1*time.Hour); !errors.Is(err, ErrKeyNotFound) {
			t.Fatalf("expected error: %v, got: %v", ErrKeyNotFound, err)
		}
	})
}

func TestCacheMemoize(t *testing.T) {
	t.Parallel()

	t.Run("Cache Miss", func(t *testing.T) {
		t.Parallel()

		store := setupTestCache[string, string](t)

		want := "Value"

		factoryFunc := func() (string, error) {
			return want, nil
		}

		got, err := store.Memorize("Key", factoryFunc, 1*time.Hour)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if got != "Value" {
			t.Fatalf("expected: %v, got: %v", "Value", got)
		}

		got, _, err = store.GetValue("Key")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if want != got {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("Cache Hit", func(t *testing.T) {
		t.Parallel()

		store := setupTestCache[string, string](t)

		want := "NewValue"

		if err := store.Set("Key", "Value", 1*time.Hour); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		factoryFunc := func() (string, error) {
			return want, nil
		}

		got, err := store.Memorize("Key", factoryFunc, 1*time.Hour)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if got != "Value" {
			t.Fatalf("expected: %v, got: %v", "Value", got)
		}
	})
}

func BenchmarkCacheGet(b *testing.B) {
	for n := 1; n <= 100000; n *= 10 {
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			db := setupTestCache[int, int](b)
			for i := range n {
				if err := db.Set(i, i, 0); err != nil {
					b.Fatalf("unexpected error: %v", err)
				}
			}

			b.ReportAllocs()

			for b.Loop() {
				if _, _, err := db.GetValue(n - 1); err != nil {
					b.Fatalf("unexpected error: %v", err)
				}
			}
		})
	}
}

func BenchmarkCacheSet(b *testing.B) {
	for n := 1; n <= 100000; n *= 10 {
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			db := setupTestCache[int, int](b)
			for i := range n - 1 {
				if err := db.Set(i, i, 0); err != nil {
					b.Fatalf("unexpected error: %v", err)
				}
			}

			b.ReportAllocs()

			for b.Loop() {
				if err := db.Set(n, n, 0); err != nil {
					b.Fatalf("unexpected error: %v", err)
				}
			}
		})
	}
}

func BenchmarkCacheDelete(b *testing.B) {
	for n := 1; n <= 100000; n *= 10 {
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			db := setupTestCache[int, int](b)
			for i := range n - 1 {
				if err := db.Set(i, i, 0); err != nil {
					b.Fatalf("unexpected error: %v", err)
				}
			}

			b.ReportAllocs()

			for b.Loop() {
				if err := db.Set(n, n, 0); err != nil {
					b.Fatalf("unexpected error: %v", err)
				}

				if err := db.Delete(n); err != nil {
					b.Fatalf("unexpected error: %v", err)
				}
			}
		})
	}
}
