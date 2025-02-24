package cache

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncodeDecodeUint64(t *testing.T) {
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
			var buf bytes.Buffer
			e := newEncoder(&buf)

			err := e.EncodeUint64(tt.value)
			assert.NoError(t, err)
			err = e.Flush()
			assert.NoError(t, err)

			decoder := newDecoder(bytes.NewReader(buf.Bytes()))

			decodedValue, err := decoder.DecodeUint64()
			assert.NoError(t, err)

			assert.Equal(t, tt.value, decodedValue)
		})
	}
}

func TestEncodeDecodeTime(t *testing.T) {
	tests := []struct {
		name  string
		value time.Time
	}{
		{name: "Time Now", value: time.Now()},
		{name: "Time Zero", value: time.Time{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			e := newEncoder(&buf)

			err := e.EncodeTime(tt.value)
			assert.NoError(t, err)
			err = e.Flush()
			assert.NoError(t, err)

			decoder := newDecoder(bytes.NewReader(buf.Bytes()))

			decodedValue, err := decoder.DecodeTime()
			assert.NoError(t, err)

			assert.WithinDuration(t, tt.value, decodedValue, time.Second)
		})
	}
}

func TestEncodeDecodeBytes(t *testing.T) {
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
			var buf bytes.Buffer
			e := newEncoder(&buf)

			err := e.EncodeBytes(tt.value)
			assert.NoError(t, err)
			err = e.Flush()
			assert.NoError(t, err)

			decoder := newDecoder(bytes.NewReader(buf.Bytes()))

			decodedValue, err := decoder.DecodeBytes()
			assert.NoError(t, err)

			assert.Equal(t, tt.value, decodedValue)
		})
	}
}

func TestEncodeDecodeNode(t *testing.T) {
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
			var buf bytes.Buffer
			e := newEncoder(&buf)

			err := e.EncodeNode(tt.value)
			assert.NoError(t, err)
			err = e.Flush()
			assert.NoError(t, err)

			decoder := newDecoder(bytes.NewReader(buf.Bytes()))

			decodedValue, err := decoder.DecodeNodes()
			assert.NoError(t, err)

			assert.Equal(t, tt.value.Hash, decodedValue.Hash)
			assert.WithinDuration(t, tt.value.Expiration, decodedValue.Expiration, 1*time.Second)
			assert.Equal(t, tt.value.Access, decodedValue.Access)
			assert.Equal(t, tt.value.Key, decodedValue.Key)
			assert.Equal(t, tt.value.Value, decodedValue.Value)
		})
	}
}

func TestEncodeDecodeStrorage(t *testing.T) {
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
				"Test1": "Test",
				"Test2": "Test",
				"Test3": "Test",
				"Test4": "Test",
			},
			policy:  PolicyNone,
			maxCost: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			e := newEncoder(&buf)

			want := setupTestStore(t)
			want.MaxCost = uint64(tt.maxCost)
			err := want.Policy.SetPolicy(tt.policy)
			assert.NoError(t, err)

			for k, v := range tt.store {
				want.Set([]byte(k), []byte(v), 0)
			}

			err = e.EncodeStore(want)
			assert.NoError(t, err)
			err = e.Flush()
			assert.NoError(t, err)

			decoder := newDecoder(bytes.NewReader(buf.Bytes()))
			got := setupTestStore(t)

			err = decoder.DecodeStore(got)
			assert.NoError(t, err)

			assert.Equal(t, want.MaxCost, got.MaxCost)
			assert.Equal(t, want.Length, got.Length)
			assert.Equal(t, want.Policy.Type, got.Policy.Type)

			gotOrder := getListOrder(t, &got.Evict)
			for i, v := range getListOrder(t, &want.Evict) {
				assert.Equal(t, v.Key, gotOrder[i].Key)
			}

			for k, v := range tt.store {
				gotVal, _, ok := want.Get([]byte(k))
				require.True(t, ok)
				require.Equal(t, []byte(v), gotVal)
			}
		})
	}
}

type MockSeeker struct {
	*bytes.Buffer
}

func BenchmarkEncoder_EncodeStore(b *testing.B) {
	file, err := os.CreateTemp("", "benchmark_test_")
	if err != nil {
		b.Fatal(err)
	}
	defer os.Remove(file.Name())
	defer file.Close()

	for n := 1; n <= 10000; n *= 10 {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			want := setupTestStore(b)

			for i := range n {
				buf := make([]byte, 8)
				binary.LittleEndian.PutUint64(buf, uint64(i))
				want.Set(buf, buf, 0)
			}

			err = want.Snapshot(file)
			require.NoError(b, err)

			fileInfo, err := file.Stat()
			require.NoError(b, err)
			b.SetBytes(int64(fileInfo.Size()))
			b.ReportAllocs()

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				want.Snapshot(file)
			}
		})
	}

}

func BenchmarkDecoder_DecodeStore(b *testing.B) {

	file, err := os.CreateTemp("", "benchmark_test_")
	require.NoError(b, err)
	defer os.Remove(file.Name())
	defer file.Close()

	for n := 1; n <= 10000; n *= 10 {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			want := setupTestStore(b)
			for i := range n {
				buf := make([]byte, 8)
				binary.LittleEndian.PutUint64(buf, uint64(i))
				want.Set(buf, buf, 0)
			}

			err = want.Snapshot(file)
			require.NoError(b, err)
			fileInfo, err := file.Stat()
			require.NoError(b, err)
			b.SetBytes(int64(fileInfo.Size()))
			b.ReportAllocs()

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				want.LoadSnapshot(file)
			}
		})
	}
}
