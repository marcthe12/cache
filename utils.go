package cache

import (
	"hash/fnv"
)

// zero returns the zero value for the specified type.
func zero[T any]() T {
	var ret T

	return ret
}

// hash computes the 64-bit FNV-1a hash of the provided data.
func hash(data []byte) uint64 {
	hasher := fnv.New64()
	if _, err := hasher.Write(data); err != nil {
		panic(err)
	}

	return hasher.Sum64()
}
