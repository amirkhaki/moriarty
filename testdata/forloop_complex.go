package main

import "fmt"

func main() {
	// Test various for loop scenarios
	
	// Standard for loop
	for i := 0; i < 5; i++ {
		fmt.Println(i)
	}
	
	// For with assignment in init
	x := 10
	for x = 0; x < 5; x++ {
		fmt.Println(x)
	}
	
	// For with complex post
	y := 0
	for i := 0; i < 5; {
		y++
		i++
	}
	
	// Infinite loop with break
	z := 0
	for {
		z++
		if z > 3 {
			break
		}
	}
	
	// While-style loop
	w := 0
	for w < 5 {
		w++
	}
	
	fmt.Println("All tests passed")
}
