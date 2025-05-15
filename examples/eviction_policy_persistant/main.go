package main

import (
	"fmt"
	"os"
	"time"

	"github.com/marcthe12/cache"
)

func main() {
	// Create an in-memory cache with LRU eviction policy
	db, err := cache.Open[int, int](
		"cache.db",
		cache.WithPolicy(cache.PolicyLRU),
		cache.WithMaxCost(20),
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

	fmt.Println("Loaded Cache")
	// Check which keys are present
	for n := range 4 {
		key := n + 1
		value, _, err := db.GetValue(key)

		if err != nil {
			fmt.Printf("Key %d not found\n", key)
		} else {
			fmt.Printf("Key %d found with value: %d\n", key, value)
		}
	}

	fmt.Println("Set Values")
	// Set values
	if err := db.Set(1, -1, 10*time.Second); err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}

	if err := db.Set(2, -2, 10*time.Second); err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}

	if err := db.Set(3, -3, 10*time.Second); err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}

	// Access some keys
	if _, _, err := db.GetValue(1); err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}

	if _, _, err := db.GetValue(2); err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}

	fmt.Println("Add Key 4")
	// Add another key to trigger eviction
	if err := db.Set(4, -4, 0); err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}

	fmt.Println("Sleep")
	time.Sleep(2 * time.Second)

	fmt.Println("Resume")
	// Check which keys are present
	for n := range 4 {
		key := n + 1
		value, _, err := db.GetValue(key)

		if err != nil {
			fmt.Printf("Key %d not found\n", key)
		} else {
			fmt.Printf("Key %d found with value: %d\n", key, value)
		}
	}
}
