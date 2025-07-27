package main

import (
	"fmt"
	"os"
	"time"

	"go.sudomsg.com/cache"
)

func main() {
	// Create an in-memory cache with LRU eviction policy
	db, err := cache.OpenMem[string, string](
		cache.WithPolicy(cache.PolicyLRU),
		cache.WithMaxCost(30),
		cache.SetCleanupTime(1*time.Second),
	)
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}

	defer func() {
		err := db.Close()
		if err != nil {
			fmt.Println("Error:", err)
		}
	}()

	// Set values
	if err := db.Set("key1", "value1", 0); err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}

	if err := db.Set("key2", "value2", 0); err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}

	if err := db.Set("key3", "value3", 0); err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}

	// Access some keys
	if _, _, err := db.GetValue("key1"); err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}

	if _, _, err := db.GetValue("key2"); err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}

	// Add another key to trigger eviction
	if err := db.Set("key4", "value4", 0); err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}

	time.Sleep(2 * time.Second)

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
