package main

import (
	"fmt"
	"unsafe"
)

type Node struct {
	value int
	next  *Node
}

type Data struct {
	x, y int
	ptr  *int
}

func main() {
	// Test 1: Basic pointer operations
	x := 42
	p := &x
	*p = 100
	fmt.Println("x =", x)

	// Test 2: Pointer to pointer
	pp := &p
	**pp = 200
	fmt.Println("x =", x)

	// Test 3: Struct with pointers
	data := Data{x: 10, y: 20}
	data.ptr = &data.x
	*data.ptr = 30
	fmt.Println("data.x =", data.x)

	// Test 4: Linked list with pointers
	node1 := &Node{value: 1, next: nil}
	node2 := &Node{value: 2, next: node1}
	node3 := &Node{value: 3, next: node2}

	// Traverse and modify
	current := node3
	for current != nil {
		current.value *= 10
		current = current.next
	}

	// Test 5: Slice of pointers
	ptrs := make([]*int, 5)
	for i := 0; i < 5; i++ {
		val := (i + 1) * 10
		ptrs[i] = &val
	}
	
	// Modify via pointers
	for _, p := range ptrs {
		*p *= 2
		fmt.Printf("value = %d\n", *p)
	}

	// Test 6: Slice manipulation
	slice := []int{1, 2, 3, 4, 5}
	slicePtr := &slice[2]
	*slicePtr = 999
	fmt.Println("slice =", slice)

	// Test 7: Pointer swap
	a, b := 100, 200
	pa, pb := &a, &b
	*pa, *pb = *pb, *pa
	fmt.Printf("After swap: a=%d, b=%d\n", a, b)

	// Test 8: Function returning pointer
	result := allocateAndSet(42)
	fmt.Println("result =", *result)

	// Test 9: Nil pointer checks
	var nilPtr *int
	if nilPtr == nil {
		nilPtr = new(int)
		*nilPtr = 777
	}
	fmt.Println("nilPtr =", *nilPtr)

	// Test 10: Pointer to array element
	matrix := [2][3]int{{1, 2, 3}, {4, 5, 6}}
	ptr2d := &matrix[1][2]
	*ptr2d = 99
	fmt.Println("matrix[1][2] =", matrix[1][2])
}

func allocateAndSet(val int) *int {
	x := val
	return &x
}
