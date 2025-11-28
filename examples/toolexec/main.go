package main

import (
	_ "github.com/amirkhaki/moriarty/pkg/runtime"
)

func main() {
	x := 10
	x = 20
	y := x + 5
	println("y =", y)
}
