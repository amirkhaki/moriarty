package main

import (
	"fmt"
)

var z = 3

func printZ() {
	fmt.Println(z)
	fmt.Println(&z)
}

func main() {
	z = 5
	printZ()
	x, z := 1, 2
	fmt.Println("address of z", &z)
	z, m := 3, 4
	fmt.Println("address of z", &z)
	_ = x
	_ = m
	printZ()
}
