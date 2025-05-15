package cache

import (
	"bytes"
	"encoding/binary"
	"os"
	"strconv"
	"testing"
	"time"
)

func TestDecodeUint64Error(t *testing.T) {
	t.Parallel()

	buf := bytes.NewReader([]byte{0xFF})

	decoder := newDecoder(buf)

	_, err := decoder.DecodeUint64()
	if err == nil {
		t.Errorf("expected an error but got none")
	}
}

func TestEncodeDecodeUint64(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value uint64
	}{
		{name: "Positive", value: 1234567890},
		{name: "Zero", value: 0},
		{name: "Max", value: ^uint64(0)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			e := newEncoder(&buf)

			if err := e.EncodeUint64(tt.value); err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if err := e.Flush(); err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			decoder := newDecoder(bytes.NewReader(buf.Bytes()))

			decodedValue, err := decoder.DecodeUint64()
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if tt.value != decodedValue {
				t.Errorf("expected %v, got %v", tt.value, decodedValue)
			}
		})
	}
}

func TestEncodeDecodeTime(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value time.Time
	}{
		{name: "Time Now", value: time.Now()},
		{name: "Unix Epoch", value: time.Unix(0, 0)},
		{name: "Time Zero", value: time.Time{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			e := newEncoder(&buf)

			if err := e.EncodeTime(tt.value); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if err := e.Flush(); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			decoder := newDecoder(bytes.NewReader(buf.Bytes()))

			decodedValue, err := decoder.DecodeTime()
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if tt.value.Unix() != decodedValue.Unix() {
				t.Errorf("expected %v, got %v", tt.value, decodedValue)
			}
		})
	}
}

func TestDecodeBytesError(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	e := newEncoder(&buf)

	if err := e.EncodeBytes([]byte("DEADBEEF")); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if err := e.Flush(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	decoder := newDecoder(bytes.NewReader(buf.Bytes()[:10]))

	if _, err := decoder.DecodeBytes(); err == nil {
		t.Errorf("expected an error but got none")
	}
}

func TestEncodeDecodeBytes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value []byte
	}{
		{name: "Empty", value: []byte{}},
		{name: "Non-Empty", value: []byte("hello world")},
		{name: "Bytes Large", value: []byte("A very long string of characters to test large data")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			e := newEncoder(&buf)

			if err := e.EncodeBytes(tt.value); err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if err := e.Flush(); err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			decoder := newDecoder(bytes.NewReader(buf.Bytes()))

			decodedValue, err := decoder.DecodeBytes()
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if !bytes.Equal(tt.value, decodedValue) {
				t.Errorf("expected %v, got %v", tt.value, decodedValue)
			}
		})
	}
}

func TestEncodeDecodeNode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value *node
	}{
		{
			name: "Empty",
			value: &node{
				Hash:       1234567890,
				Expiration: time.Now(),
				Access:     987654321,
				Key:        []byte("testKey"),
				Value:      []byte("testValue"),
			},
		},
		{
			name: "Non-Empty",
			value: &node{
				Hash:       1234567890,
				Expiration: time.Now(),
				Access:     987654321,
				Key:        []byte("testKey"),
				Value:      []byte("testValue"),
			},
		},
		{
			name: "Bytes Large",
			value: &node{
				Hash:       1234567890,
				Expiration: time.Now(),
				Access:     987654321,
				Key:        []byte("testKey"),
				Value:      []byte("testValue"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			e := newEncoder(&buf)

			if err := e.EncodeNode(tt.value); err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if err := e.Flush(); err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			decoder := newDecoder(bytes.NewReader(buf.Bytes()))

			decodedValue, err := decoder.DecodeNodes()
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if tt.value.Hash != decodedValue.Hash {
				t.Errorf("expected %v, got %v", tt.value.Hash, decodedValue.Hash)
			}

			if !tt.value.Expiration.Equal(decodedValue.Expiration) &&
				tt.value.Expiration.Sub(decodedValue.Expiration) > time.Second {
				t.Errorf("expected %v to be within %v of %v",
					decodedValue.Expiration, time.Second, tt.value.Expiration)
			}

			if tt.value.Access != decodedValue.Access {
				t.Errorf("expected %v, got %v", tt.value.Access, decodedValue.Access)
			}

			if !bytes.Equal(tt.value.Key, decodedValue.Key) {
				t.Errorf("expected %v, got %v", tt.value.Key, decodedValue.Key)
			}

			if !bytes.Equal(tt.value.Value, decodedValue.Value) {
				t.Errorf("expected %v, got %v", tt.value.Value, decodedValue.Value)
			}
		})
	}
}

func TestStoreSnapshot(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		store   map[string]string
		policy  EvictionPolicyType
		maxCost int
	}{
		{
			name:    "Empty",
			store:   map[string]string{},
			policy:  PolicyNone,
			maxCost: 0,
		},
		{
			name: "Single Item",
			store: map[string]string{
				"Test": "Test",
			},
			policy:  PolicyNone,
			maxCost: 0,
		},
		{
			name: "Many Items",
			store: map[string]string{
				"1": "Test",
				"2": "Test",
				"3": "Test",
				"4": "Test",
				"5": "Test",
				"6": "Test",
				"7": "Test",
				"8": "Test",
			},
			policy:  PolicyNone,
			maxCost: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer

			want := setupTestStore(t)
			want.MaxCost = uint64(tt.maxCost)

			if err := want.Policy.SetPolicy(tt.policy); err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			for k, v := range tt.store {
				want.Set([]byte(k), []byte(v), 0)
			}

			if err := want.Snapshot(&buf); err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			reader := bytes.NewReader(buf.Bytes())

			got := setupTestStore(t)

			if err := got.LoadSnapshot(reader); err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if want.MaxCost != got.MaxCost {
				t.Errorf("expected %v, got %v", want.MaxCost, got.MaxCost)
			}

			if want.Length != got.Length {
				t.Errorf("expected %v, got %v", want.Length, got.Length)
			}

			if want.Policy.Type != got.Policy.Type {
				t.Errorf("expected %v, got %v", want.Policy.Type, got.Policy.Type)
			}

			gotOrder := getListOrder(t, &got.EvictList)
			for i, v := range getListOrder(t, &want.EvictList) {
				if !bytes.Equal(v.Key, gotOrder[i].Key) {
					t.Errorf("expected %#v, got %#v", v.Key, gotOrder[i].Key)
				}
			}

			for k, v := range tt.store {
				gotVal, _, ok := want.Get([]byte(k))
				if !ok {
					t.Fatalf("expected condition to be true")
				}

				if !bytes.Equal([]byte(v), gotVal) {
					t.Fatalf("expected %v, got %v", []byte(v), gotVal)
				}
			}
		})
	}
}

func createTestFile(tb testing.TB, pattern string) *os.File {
	tb.Helper()

	file, err := os.CreateTemp(tb.TempDir(), pattern)
	if err != nil {
		tb.Fatal(err)
	}

	tb.Cleanup(func() {
		if err := os.Remove(file.Name()); err != nil {
			tb.Fatalf("unexpected error: %v", err)
		}

		_ = file.Close()
	})

	return file
}

func BenchmarkStoreSnapshot(b *testing.B) {
	file := createTestFile(b, "benchmark_test_")

	for n := 1; n <= 10000; n *= 10 {
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			want := setupTestStore(b)

			for i := range n {
				buf := make([]byte, 8)
				binary.LittleEndian.PutUint64(buf, uint64(i))
				want.Set(buf, buf, 0)
			}

			if err := want.Snapshot(file); err != nil {
				b.Fatalf("unexpected error: %v", err)
			}

			fileInfo, err := file.Stat()
			if err != nil {
				b.Fatalf("unexpected error: %v", err)
			}

			b.SetBytes(fileInfo.Size())
			b.ReportAllocs()

			for b.Loop() {
				if err := want.Snapshot(file); err != nil {
					b.Fatalf("unexpected error: %v", err)
				}
			}
		})
	}
}

func BenchmarkStoreLoadSnapshot(b *testing.B) {
	file := createTestFile(b, "benchmark_test_")

	for n := 1; n <= 10000; n *= 10 {
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			want := setupTestStore(b)

			for i := range n {
				buf := make([]byte, 8)
				binary.LittleEndian.PutUint64(buf, uint64(i))
				want.Set(buf, buf, 0)
			}

			if err := want.Snapshot(file); err != nil {
				b.Fatalf("unexpected error: %v", err)
			}

			fileInfo, err := file.Stat()
			if err != nil {
				b.Fatalf("unexpected error: %v", err)
			}

			b.SetBytes(fileInfo.Size())
			b.ReportAllocs()

			for b.Loop() {
				want.Clear()

				if err := want.LoadSnapshot(file); err != nil {
					b.Fatalf("unexpected error: %v", err)
				}
			}
		})
	}
}
