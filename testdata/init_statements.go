package main

import "fmt"

func main() {
	// If with init statement
	if x := 10; x > 5 {
		fmt.Println("x is large")
		x++
		fmt.Println(x)
	}
	
	// Switch with init
	switch y := 20; y {
	case 20:
		fmt.Println("y is 20")
		y++
	default:
		fmt.Println("other")
	}
	
	fmt.Println("Done")
}
