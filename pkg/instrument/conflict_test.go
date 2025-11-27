package instrument_test

import (
	"bytes"
	"go/printer"
	"go/token"
	"strings"
	"testing"

	"github.com/amirkhaki/moriarty/pkg/instrument"
)

func TestNoRuntimeConflict(t *testing.T) {
	// Code that uses Go's built-in runtime package
	src := `package main

import "runtime"

func main() {
	n := runtime.NumCPU()
	x := 10
	x = 20
	_ = n
}
`

	instr := instrument.NewInstrumenter(nil)
	fset := token.NewFileSet()

	f, err := instr.InstrumentFile(fset, "test.go", src)
	if err != nil {
		t.Fatalf("InstrumentFile failed: %v", err)
	}

	var buf bytes.Buffer
	if err := printer.Fprint(&buf, fset, f); err != nil {
		t.Fatalf("Failed to print AST: %v", err)
	}

	result := buf.String()

	// Should have both imports
	if !strings.Contains(result, `"runtime"`) {
		t.Error("Expected original runtime import to be preserved")
	}

	if !strings.Contains(result, `__moriarty_`) {
		t.Error("Expected mangled moriarty runtime alias")
	}

	// Should use the correct runtime in each context
	if !strings.Contains(result, "runtime.NumCPU()") {
		t.Error("Expected runtime.NumCPU() to remain unchanged")
	}

	if !strings.Contains(result, ".MemWrite") {
		t.Error("Expected MemWrite for instrumentation")
	}

	// Make sure we didn't accidentally change the user's runtime calls
	// NumCPU should be called on "runtime", not the mangled alias
	if strings.Contains(result, "__moriarty_") {
		lines := strings.Split(result, "\n")
		for _, line := range lines {
			if strings.Contains(line, ".NumCPU") {
				if !strings.Contains(line, "runtime.NumCPU") {
					t.Error("NumCPU should use 'runtime' package, not mangled alias")
				}
			}
		}
	}
}
