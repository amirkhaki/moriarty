package main

type Node struct {
	value int
	next  *Node
}

func main() {
	// Test 1: Complex pointer operations
	var p *int
	x := 10
	p = &x
	*p = 20
	y := *p
	
	// Test 2: Nested struct access
	node := &Node{value: 1}
	node.value = 2
	val := node.value
	
	// Test 3: Multi-dimensional arrays
	matrix := [][]int{{1, 2}, {3, 4}}
	matrix[0][1] = 5
	z := matrix[1][0]
	
	// Test 4: Op-assignments
	a := 10
	a += 5
	a *= 2
	
	// Test 5: Multiple assignment
	b, c := 1, 2
	b, c = c, b
	
	// Test 6: Type assertions and conversions
	var i interface{} = 42
	num, ok := i.(int)
	
	// Use variables
	_, _, _, _, _, _ = y, val, z, a, num, ok
}
