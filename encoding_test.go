package cache

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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
		{name: "Empty", value: &node{
			Hash:       1234567890,
			Expiration: time.Now(),
			Access:     987654321,
			Key:        []byte("testKey"),
			Value:      []byte("testValue"),
		}},
		{name: "Non-Empty", value: &node{
			Hash:       1234567890,
			Expiration: time.Now(),
			Access:     987654321,
			Key:        []byte("testKey"),
			Value:      []byte("testValue"),
		}},
		{name: "Bytes Large", value: &node{
			Hash:       1234567890,
			Expiration: time.Now(),
			Access:     987654321,
			Key:        []byte("testKey"),
			Value:      []byte("testValue"),
		}},
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
