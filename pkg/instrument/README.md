# Moriarty Instrumentation Library

A Go library and CLI tool for instrumenting Go source code with memory read/write tracking for race detection.

## Features

- **Type-aware instrumentation**: Uses `go/types` to distinguish between maps and arrays/slices, new declarations vs reassignments
- **Comprehensive coverage**: Instruments all memory operations including:
  - Variable assignments and declarations
  - Pointer dereferences
  - Struct field access
  - Array/slice element access
  - Map operations (read-only tracking)
  - Channel operations
  - Inc/Dec operations
  - Op-assignments (+=, -=, etc.)
- **Library and CLI**: Can be used as a library or command-line tool

## Installation

```bash
go get github.com/amirkhaki/moriarty
```

## CLI Usage

```bash
# Instrument a Go source file
moriarty /path/to/source.go > instrumented.go
```

## Library Usage

```go
package main

import (
    "go/printer"
    "go/token"
    "github.com/amirkhaki/moriarty/pkg/instrument"
)

func main() {
    // Create an instrumenter with default config
    instr := instrument.NewInstrumenter(nil)
    
    // Create a file set
    fset := token.NewFileSet()
    
    // Instrument the file
    f, err := instr.InstrumentFile(fset, "example.go", nil)
    if err != nil {
        panic(err)
    }
    
    // Print the instrumented code
    printer.Fprint(os.Stdout, fset, f)
}
```

### Custom Configuration

```go
config := &instrument.Config{
    BaseRuntimeAddress: "mypackage/runtime",
    RuntimeAlias:       "myrt",          // Custom alias (optional, auto-generated if empty)
    MemReadFunc:        "TrackRead",     // Custom function name
    MemWriteFunc:       "TrackWrite",    // Custom function name
    ImportRewrites: map[string]string{
        "sync": "mypackage/runtime/sync",
    },
}
instr := instrument.NewInstrumenter(config)
```

**Note on RuntimeAlias:**
- If not specified, a deterministic mangled alias is auto-generated using SHA-256 hash
- Format: `__moriarty_<16-hex-chars>` (e.g., `__moriarty_5decea860786e867`)
- This ensures no conflicts with any user imports

## How It Works

The library instruments Go source code by inserting `MemRead()` and `MemWrite()` calls before memory operations:

### Example

**Input:**
```go
func main() {
    x := 10      // Declaration
    x = 20       // Assignment
    y := x + 5   // Mixed
}
```

**Output:**
```go
func main() {
	import (
		"unsafe"
		__moriarty_5decea860786e867 "github.com/amirkhaki/moriarty/pkg/runtime"
	)
	
	x := 10                                                // No write (new variable)
	__moriarty_5decea860786e867.MemWrite(unsafe.Pointer(&x)) // Write before assignment
	x = 20
	__moriarty_5decea860786e867.MemRead(unsafe.Pointer(&x))  // Read x
	y := x + 5                                             // No write (new variable)
}
```

**Note:** The import alias `__moriarty_5decea860786e867` is deterministically generated from the runtime package path to avoid conflicts.

### Key Features

1. **Smart Declaration Handling**: Uses type information to distinguish between:
   - `x := 10` - Pure declaration (no write instrumentation)
   - `x, y := 20, 30` where `x` exists - Mixed (only `x` gets write instrumentation)
   - `x = 30` - Pure assignment (write instrumentation before)

2. **Addressability Detection**: 
   - Array/slice elements: Fully instrumented (addressable)
   - Map elements: Only map container tracked (elements not addressable)

3. **Timing**:
   - Regular assignments (`=`): Instrumentation **before** the statement
   - Declarations (`:=`): Instrumentation **after** the statement (for reassignments only)

## Runtime Functions

The instrumented code will import and call functions from `github.com/amirkhaki/moriarty/pkg/runtime` with an auto-generated mangled alias:

```go
import __moriarty_5decea860786e867 "github.com/amirkhaki/moriarty/pkg/runtime"

// Calls in instrumented code
__moriarty_5decea860786e867.MemRead(unsafe.Pointer(&x))
__moriarty_5decea860786e867.MemWrite(unsafe.Pointer(&x))
```

**Import Alias Generation:**
- The alias is deterministically generated using SHA-256 hash of the import path
- Format: `__moriarty_<16-hex-chars>`
- This ensures no conflicts with Go's built-in `runtime` package or any user imports
- You can override it by setting `Config.RuntimeAlias`

// Calls in instrumented code
moriarty_runtime.MemRead(unsafe.Pointer(&x))
moriarty_runtime.MemWrite(unsafe.Pointer(&x))
```

**Note:** The import uses the alias `moriarty_runtime` to avoid conflicts with Go's built-in `runtime` package or any other runtime packages in user code.

The package provides stub implementations. You can replace these with your own race detection logic.

### Example Runtime Implementation

```go
package runtime

import (
	"fmt"
	"unsafe"
)

func MemRead(addr unsafe.Pointer) {
	// Log or track the read operation
	fmt.Printf("Read from %p\n", addr)
	
	// Your race detection logic here:
	// - Check shadow memory for conflicting writes
	// - Record this read in thread-local state
	// - Report race if detected
}

func MemWrite(addr unsafe.Pointer) {
	// Log or track the write operation
	fmt.Printf("Write to %p\n", addr)
	
	// Your race detection logic here:
	// - Check shadow memory for conflicting reads/writes
	// - Record this write in thread-local state
	// - Report race if detected
}
```

## Package Structure

```
moriarty/
├── pkg/
│   ├── instrument/
│   │   ├── instrument.go       # Core instrumentation library
│   │   ├── instrument_test.go  # Unit tests
│   │   └── README.md           # Library documentation
│   └── runtime/
│       └── runtime.go          # Stub runtime functions (MemRead/MemWrite)
├── examples/
│   └── library_usage.go        # Example of library usage
├── main.go                     # CLI tool
└── testdata/                   # Test files
```

## API Documentation

### Types

#### `Config`
Configuration for the instrumenter.

```go
type Config struct {
    ImportRewrites     map[string]string
    BaseRuntimeAddress string
}
```

#### `Instrumenter`
Main type for performing instrumentation.

```go
type Instrumenter struct {
    // private fields
}
```

### Functions

#### `NewInstrumenter(config *Config) *Instrumenter`
Creates a new Instrumenter. Pass `nil` to use default configuration.

#### `(instr *Instrumenter) InstrumentFile(fset *token.FileSet, filename string, src interface{}) (*ast.File, error)`
Instruments a Go source file. The `src` parameter can be `nil`, a `string`, `[]byte`, or `io.Reader`.

#### `(instr *Instrumenter) InstrumentAST(fset *token.FileSet, f *ast.File) (*ast.File, error)`
Instruments an already-parsed AST.

#### `DefaultConfig() *Config`
Returns a Config with default settings.

## License

See LICENSE file.
