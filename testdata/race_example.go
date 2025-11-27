package main

import (
	"fmt"
	"sync"
	"time"
)

var counter int

func increment(wg *sync.WaitGroup, n int) {
	defer wg.Done()
	for i := 0; i < n; i++ {
		counter++
	}
}

func main() {
	var wg sync.WaitGroup
	
	fmt.Println("Starting race detection demo...")
	
	// Spawn goroutines that will race on counter
	wg.Add(3)
	go increment(&wg, 100)
	go increment(&wg, 200)
	go increment(&wg, 300)
	
	wg.Wait()
	fmt.Printf("Final counter: %d (expected 600)\n", counter)
	time.Sleep(10 * time.Millisecond)
}
