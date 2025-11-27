package main

import (
	"bytes"
	"fmt"
	"go/printer"
	"go/token"

	"github.com/amirkhaki/moriarty/pkg/instrument"
)

func main() {
	// Example Go source code
	src := `package main

func main() {
	x := 10
	x = 20
	y := x + 5
	_ = y
}
`

	// Create an instrumenter with default config
	// This will use github.com/amirkhaki/moriarty/pkg/runtime
	instr := instrument.NewInstrumenter(nil)

	// Create a file set
	fset := token.NewFileSet()

	// Instrument the source code
	f, err := instr.InstrumentFile(fset, "example.go", src)
	if err != nil {
		fmt.Printf("Error instrumenting: %v\n", err)
		return
	}

	// Write the instrumented code to a buffer
	var buf bytes.Buffer
	if err := printer.Fprint(&buf, fset, f); err != nil {
		fmt.Printf("Error printing: %v\n", err)
		return
	}

	// Print the instrumented code
	fmt.Println("Original code:")
	fmt.Println(src)
	fmt.Println("\nInstrumented code:")
	fmt.Println(buf.String())
	fmt.Println("\nNote: The instrumented code imports:")
	fmt.Println("  - unsafe (for pointer operations)")
	fmt.Println("  - A deterministically mangled alias for github.com/amirkhaki/moriarty/pkg/runtime")
	fmt.Println("  - Format: __moriarty_<16-hex-chars> (e.g., __moriarty_5decea860786e867)")
	fmt.Println("  - This guarantees no conflicts with any user imports including Go's runtime package")
}
