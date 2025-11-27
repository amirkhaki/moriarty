package main

import "fmt"

type Point struct {
	X, Y int
}

func main() {
	// Variable operations
	x := 10
	y := x + 1
	x++
	
	// Pointer operations
	p := &x
	*p = 20
	
	// Struct operations
	pt := Point{X: 1, Y: 2}
	pt.X = 3
	
	// Array operations
	arr := []int{1, 2, 3}
	arr[0] = 10
	
	// Map operations
	m := map[string]int{"a": 1}
	m["b"] = 2
	val := m["a"]
	
	// Channel operations
	ch := make(chan int, 1)
	ch <- 42
	received := <-ch
	
	// Range
	for i, v := range arr {
		fmt.Println(i, v)
	}
	
	// Use variables
	_, _, _, _ = y, pt, val, received
}
