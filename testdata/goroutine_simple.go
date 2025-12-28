package main

import "fmt"
import "time"
import "github.com/amirkhaki/moriarty/pkg/goid"

func add(a, b int) int {
	return a + b
}

func main() {
	var x, y int
	x = 10
	y = 20

	// Test simple goroutine with literals
	go func() {
		fmt.Println("goroutine 1 with id", goid.Get())
		add(1, 2)
	}()

	// Test goroutine with variables
	go func() {
		fmt.Println("goroutine 2 with id", goid.Get())
		add(x, y)
	}()

	// Test goroutine with expressions
	go func() {
		fmt.Println("goroutine 3 with id", goid.Get())
		add(x+5, y*2)
	}()

	fmt.Println("Done")
	time.Sleep(1 * time.Second)
}
