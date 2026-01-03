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
	var done1, done2, done3 int

	// Test simple goroutine with literals
	go func() {
		fmt.Println("goroutine 1 with id", goid.Get())
		add(1, 2)
		done1 = 1
	}()

	// Test goroutine with variables
	go func() {
		fmt.Println("goroutine 2 with id", goid.Get())
		add(x, y)
		done2 = 1
	}()

	// Test goroutine with expressions
	go func() {
		fmt.Println("goroutine 3 with id", goid.Get())
		add(x+5, y*2)
		done3 = 1
	}()

	time.Sleep(1 * time.Second)
	// Read done flags to ensure goroutines completed before printing
	_ = done1 + done2 + done3
	fmt.Println("Done", y)
}
