package main

import (
	"fmt"
	"go/printer"
	"go/token"
	"os"

	"github.com/amirkhaki/moriarty/pkg/instrument"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: moriarty /path/to/source.go")
		return
	}
	filepath := os.Args[1]

	// Create instrumenter with default config
	instr := instrument.NewInstrumenter(nil)

	// Create a new file set
	fset := token.NewFileSet()

	// Instrument the file
	f, err := instr.InstrumentFile(fset, filepath, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Print the instrumented code to stdout
	printer.Fprint(os.Stdout, fset, f)
}
