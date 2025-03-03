# Cache

## Features

- **In-Memory Cache**: Fast access to cached data.

- **Uses Generics with serialization**: To make it type safe and support several types via msgpack.
s
- **File-Backed Storage**: Persistent storage of cache data.

- **Eviction Policies**: Support for FIFO, LRU, LFU, and LTR eviction policies.

- **Concurrency**: Thread-safe operations with the use of locks(mutex). The persistant storage is locked via file locks to avoid issues

- **Periodic Tasks**: Background worker for snapshotting and cleanup of expired entries.

## Installation

To use the Cache Library in your Go project, you can install it using `go get`:

```sh
go get github.com/marcthe12/cache
```

## Usage

### Opening a Cache

 To open a file-backed cache, use the `OpenFile` function:

```go
package main

import (
	"github.com/marcthe12/cache"
	"time"
	"log"
)

func main() {
	db, err := cache.OpenFile[string, string]("cache.db", cache.WithPolicy(cache.PolicyLRU))
	if err != nil {
		log.Fatal(err)
	}

	defer db.Close()

	// Set a key-value pair with a TTL of 1 hour
	if err := db.Set("key", "value", 1*time.Hour); err != nil {
		log.Fatal(err)
	}

	// Get a value by key
	value, ttl, err := db.GetValue("key")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Value: %s, TTL: %v\n", value, ttl)
}
```

To open an in-memory cache, use the `OpenMem` function:

```go
package main

import (
	"github.com/marcthe12/cache"
	"time"
	"log"
)

func main() {
	db, err := cache.OpenMem[string, string](cache.WithPolicy(cache.PolicyLRU))

	if err != nil {
		log.Fatal(err)
	}

	defer db.Close()

	// Set a key-value pair with a TTL of 1 hour
	if err := db.Set("key", "value", 1*time.Hour); err != nil {
		log.Fatal(err)
	}

	// Get a value by key

	value, ttl, err := db.GetValue("key")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Value: %s, TTL: %v\n", value, ttl)
}
```

More Examples in the ```/examples``` directory

### Eviction Policies

The Cache Library supports the following eviction policies:

- **None**: Does not do any evictions.

- **FIFO (First In, First Out)**: Evicts the oldest entries first.

- **LRU (Least Recently Used)**: Evicts the least recently used entries first.

- **LFU (Least Frequently Used)**: Evicts the least frequently used entries first.

- **LTR (Least Remaining Time)**: Evicts entries with the least remaining time to live first.

You can set the eviction policy when opening the cache using the `WithPolicy` option.

### Configuration Options

The Cache Library supports the following configuration options:

- `WithPolicy`: Sets the eviction policy.

- `WithMaxCost`: Sets the maximum cost for the cache. The Cost is the size of the binary encoded KV pair.

- `SetSnapshotTime`: Sets the interval for taking snapshots of the cache.

- `SetCleanupTime`: Sets the interval for cleaning up expired entries.

### Additional Methods

- `Get`: Retrieves a value from the cache by key and returns its TTL.

- `GetValue`: Retrieves a value from the cache by key and returns the value and its TTL.

- `Set`: Adds a key-value pair to the cache with a specified TTL.

- `Delete`: Removes a key-value pair from the cache.

- `UpdateInPlace`: Retrieves a value from the cache, processes it using the provided function, and then sets the result back into the cache with the same key.

- `Memorize`: Attempts to retrieve a value from the cache. If the retrieval fails, it sets the result of the factory function into the cache and returns that result. Note this locks the db duing the factory function which prevent concurent acces to the db during the operation.

## Documentation

 For detailed documentation on the public API, you can use `godoc`:

```sh
godoc -http=:6060
```

Then open your browser and navigate to `http://localhost:6060/pkg/github.com/marcthe12/cache`.
