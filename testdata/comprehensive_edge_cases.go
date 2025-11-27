package main

import (
	"fmt"
	"time"
)

const MAX = 100

func main() {
	// Test 1: For loop with post increment
	sum := 0
	for i := 0; i < 10; i++ {
		sum += i
	}
	fmt.Printf("Sum: %d\n", sum)
	
	// Test 2: Constants should not be instrumented
	duration := 5 * time.Millisecond
	time.Sleep(duration)
	
	// Test 3: If with init
	if x := MAX; x > 50 {
		fmt.Println("x > 50")
	}
	
	// Test 4: Goroutine with for loop
	go func() {
		for i := 0; i < 3; i++ {
			fmt.Printf("Goroutine: %d\n", i)
		}
	}()
	
	time.Sleep(10 * time.Millisecond)
	fmt.Println("All edge cases passed!")
}
