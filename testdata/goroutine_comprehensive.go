package main

import (
	"fmt"
)

func simpleFunc() {
	fmt.Println("Simple function")
}

func withArgs(a int, b string, c bool) {
	fmt.Printf("Args: %d, %s, %t\n", a, b, c)
}

func main() {
	// Test 1: Goroutine with no arguments
	go simpleFunc()
	
	// Test 2: Goroutine with literal arguments
	go withArgs(42, "hello", true)
	
	// Test 3: Goroutine with variable arguments
	x := 10
	msg := "world"
	flag := false
	go withArgs(x, msg, flag)
	
	// Test 4: Goroutine with expression arguments
	go withArgs(x*2, fmt.Sprintf("value:%d", x), x > 5)
	
	// Test 5: Anonymous function goroutine
	go func() {
		fmt.Println("Anonymous")
	}()
	
	// Test 6: Anonymous function with closure
	go func(val int) {
		fmt.Printf("Closure: %d\n", val)
	}(x + 100)
	
	fmt.Println("Main done")
}
