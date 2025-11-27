package main

type Point struct {
	X, Y int
}

var global int

func main() {
	// Basic variable operations
	x := 10
	y := x + 5
	x++
	x--
	
	// Pointer operations
	p := &x
	*p = 20
	z := *p
	
	// Struct operations
	pt := Point{X: 1, Y: 2}
	pt.X = 3
	a := pt.Y
	
	// Array/slice operations
	arr := []int{1, 2, 3}
	arr[0] = 10
	b := arr[1]
	
	// Map operations
	m := make(map[string]int)
	m["key"] = 42
	c := m["key"]
	
	// Channel operations
	ch := make(chan int)
	go func() {
		ch <- 100
	}()
	d := <-ch
	
	// Range
	for i, v := range arr {
		_ = i
		_ = v
	}
	
	// Function call
	result := add(x, y)
	
	// Return
	_ = result
	_ = z
	_ = a
	_ = b
	_ = c
	_ = d
}

func add(a, b int) int {
	return a + b
}
