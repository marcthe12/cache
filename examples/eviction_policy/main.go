package main

import (
	"fmt"

	"github.com/marcthe12/cache"
)

func main() {
	// Create an in-memory cache with LRU eviction policy
	db, err := cache.OpenMem[string, string](cache.WithPolicy(cache.PolicyLRU))
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	defer db.Close()

	// Set values
	db.Set("key1", "value1", 0)
	db.Set("key2", "value2", 0)
	db.Set("key3", "value3", 0)

	// Access some keys
	db.GetValue("key1")
	db.GetValue("key2")

	// Add another key to trigger eviction
	db.Set("key4", "value4", 0)

	// Check which keys are present
	for _, key := range []string{"key1", "key2", "key3", "key4"} {
		value, _, err := db.GetValue(key)
		if err != nil {
			fmt.Printf("Key %s not found\n", key)
		} else {
			fmt.Printf("Key %s found with value: %s\n", key, value)
		}
	}
}
