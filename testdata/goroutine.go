package main

import (
	"fmt"
	"time"
)

func worker(id int, msg string) {
	fmt.Printf("Worker %d: %s\n", id, msg)
}

func main() {
	// Simple goroutine
	go worker(1, "hello")
	
	// Goroutine with multiple arguments
	go worker(2, "world")
	
	// Goroutine with expression arguments
	x := 5
	go worker(x+1, fmt.Sprintf("value: %d", x))
	
	// Anonymous function goroutine
	go func() {
		fmt.Println("Anonymous goroutine")
	}()
	
	time.Sleep(100 * time.Millisecond)
}
