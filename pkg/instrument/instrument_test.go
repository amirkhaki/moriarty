package instrument_test

import (
	"bytes"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/amirkhaki/moriarty/pkg/instrument"
)

func TestInstrumentFile(t *testing.T) {
	src := `package main

func main() {
	x := 10
	x = 20
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

	// Check that unsafe is imported
	if !strings.Contains(result, `import`) && !strings.Contains(result, `"unsafe"`) {
		t.Error("Expected unsafe import")
	}

	// Check that runtime package is imported
	if !strings.Contains(result, "github.com/amirkhaki/moriarty/pkg/runtime") {
		t.Error("Expected runtime package import")
	}

	// Check that mangled alias is used (starts with __moriarty_)
	if !strings.Contains(result, "__moriarty_") {
		t.Error("Expected mangled runtime alias starting with __moriarty_")
	}

	// Check that MemWrite is called with the mangled alias
	if !strings.Contains(result, ".MemWrite") {
		t.Error("Expected MemWrite call")
	}

	// Check that x := 10 doesn't have MemWrite before it
	lines := strings.Split(result, "\n")
	for i, line := range lines {
		if strings.Contains(line, "x := 10") {
			if i > 0 && strings.Contains(lines[i-1], "MemWrite") {
				t.Error("Pure declaration should not have MemWrite before it")
			}
		}
	}
}

func TestMixedDeclaration(t *testing.T) {
	src := `package main

func main() {
	x := 10
	x, y := 20, 30
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

	// Should have MemWrite for x (reassignment) but not y (new)
	if !strings.Contains(result, ".MemWrite") {
		t.Error("Expected MemWrite for reassignment")
	}
}

func TestMapVsSlice(t *testing.T) {
	src := `package main

func main() {
	arr := []int{1, 2, 3}
	arr[0] = 10
	
	m := map[string]int{"a": 1}
	m["b"] = 20
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

	// Should instrument array element
	if !strings.Contains(result, "MemWrite(unsafe.Pointer(&arr[0]))") {
		t.Error("Expected instrumentation for array element write")
	}

	// Should NOT instrument map element (not addressable)
	if strings.Contains(result, "&m[") {
		t.Error("Should not try to take address of map element")
	}
}

func TestCustomConfig(t *testing.T) {
	config := &instrument.Config{
		BaseRuntimeAddress: "custom/runtime",
		ImportRewrites:     map[string]string{},
	}

	instr := instrument.NewInstrumenter(config)
	if instr == nil {
		t.Fatal("NewInstrumenter returned nil")
	}
}

func TestMultiFileInstrumentation(t *testing.T) {
	// Simulate multiple files in same package that reference each other
	file1 := `package testpkg

type Counter struct {
	value int
}

func NewCounter() *Counter {
	return &Counter{value: 0}
}

func (c *Counter) Increment() {
	c.value++
}
`

	file2 := `package testpkg

func UseCounter() {
	c := NewCounter()
	c.Increment()
	x := c.value
	_ = x
}
`

	// Create temp files for testing
	tmpDir := t.TempDir()
	file1Path := filepath.Join(tmpDir, "counter.go")
	file2Path := filepath.Join(tmpDir, "user.go")

	if err := os.WriteFile(file1Path, []byte(file1), 0644); err != nil {
		t.Fatalf("Failed to write file1: %v", err)
	}
	if err := os.WriteFile(file2Path, []byte(file2), 0644); err != nil {
		t.Fatalf("Failed to write file2: %v", err)
	}

	// Test multi-file instrumentation
	instr := instrument.NewInstrumenter(nil)
	fset := token.NewFileSet()

	files, err := instr.InstrumentFiles(fset, []string{file1Path, file2Path})
	if err != nil {
		t.Fatalf("InstrumentFiles failed: %v", err)
	}

	if len(files) != 2 {
		t.Fatalf("Expected 2 instrumented files, got %d", len(files))
	}

	// Check that both files are instrumented
	for i, f := range files {
		var buf bytes.Buffer
		if err := printer.Fprint(&buf, fset, f); err != nil {
			t.Fatalf("Failed to print AST for file %d: %v", i, err)
		}

		result := buf.String()

		// Check that runtime package is imported
		if !strings.Contains(result, "github.com/amirkhaki/moriarty/pkg/runtime") {
			t.Errorf("File %d: Expected runtime package import", i)
		}

		// Check that instrumentation calls are present
		if !strings.Contains(result, "MemRead") && !strings.Contains(result, "MemWrite") {
			t.Errorf("File %d: Expected memory instrumentation", i)
		}
	}
}
