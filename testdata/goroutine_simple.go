package main

import "fmt"
import "time"

func add(a, b int) int {
	return a + b
}

func main() {
	x := 10
	y := 20

	// Test simple goroutine with literals
	go add(1, 2)

	// Test goroutine with variables
	go add(x, y)

	// Test goroutine with expressions
	go add(x+5, y*2)

	fmt.Println("Done")
	time.Sleep(1 * time.Second)
}
