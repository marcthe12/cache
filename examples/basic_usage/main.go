package main

import (
	"fmt"
	"time"

	"go.sudomsg.com/cache"
)

func main() {
	// Create an in-memory cache
	db, err := cache.OpenMem[string, string]()
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	defer func() {
		err := db.Close()
		if err != nil {
			fmt.Println("Error:", err)
		}
	}()

	// Set a value with a TTL of 5 seconds
	if err = db.Set("key1", "value1", 5*time.Second); err != nil {
		fmt.Println("Set Error:", err)
		return
	}

	// Get the value
	value, ttl, err := db.GetValue("key1")
	if err != nil {
		fmt.Println("Get Error:", err)
	} else {
		fmt.Printf("Got value: %s, TTL: %s\n", value, ttl)
	}

	// Wait for 6 seconds and try to get the value again
	time.Sleep(6 * time.Second)

	value, ttl, err = db.GetValue("key1")
	if err != nil {
		fmt.Println("Get Error after TTL:", err)
	} else {
		fmt.Printf("Got value after TTL: %s, TTL: %s\n", value, ttl)
	}
}
