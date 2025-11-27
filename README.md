# Moriarty - Go Race Detection Instrumentation

A Go library and CLI tool for instrumenting Go source code with memory read/write tracking and goroutine lifecycle hooks for race detection.

## Features

- **Type-aware instrumentation**: Uses `go/types` to distinguish between maps and arrays/slices, new declarations vs reassignments
- **Comprehensive coverage**: Instruments all memory operations including variables, pointers, structs, arrays, slices, maps, channels
- **Goroutine instrumentation**: Automatically wraps `go` statements with spawn/enter/exit hooks for tracking goroutine lifecycles
- **Smart handling**: Only instruments actual memory updates (assignments), not declarations
- **Runtime package**: Provides clean integration with runtime tracking functions
- **Library & CLI**: Use as a library in your tools or as a standalone CLI

## Quick Start

### CLI Usage

```bash
# Install
go install github.com/amirkhaki/moriarty@latest

# Instrument a file
moriarty input.go > instrumented.go
```

### Library Usage

```go
import "github.com/amirkhaki/moriarty/pkg/instrument"

instr := instrument.NewInstrumenter(nil)
fset := token.NewFileSet()
f, _ := instr.InstrumentFile(fset, "example.go", nil)
```

## How It Works

### Memory Operation Instrumentation

The tool inserts calls to `runtime.MemRead()` and `runtime.MemWrite()` before memory operations:

```go
// Original
func main() {
    x := 10      // Declaration
    x = 20       // Assignment
    y := x + 5   // Read
}

// Instrumented
func main() {
    x := 10                                                // No instrumentation (new variable)
    __moriarty_5decea860786e867.MemWrite(unsafe.Pointer(&x)) // Before assignment
    x = 20
    __moriarty_5decea860786e867.MemRead(unsafe.Pointer(&x))  // Before read
    y := x + 5
}
```

### Goroutine Instrumentation

The tool transforms `go` statements to capture argument values and add lifecycle hooks:

```go
// Original
func worker(id int, msg string) {
    fmt.Printf("Worker %d: %s\n", id, msg)
}

func main() {
    x := 5
    go worker(x+1, fmt.Sprintf("value: %d", x))
}

// Instrumented
func main() {
    x := 5
    {
        __moriarty_p0 := x + 1
        __moriarty_p1 := fmt.Sprintf("value: %d", x)
        __moriarty_5decea860786e867.Spawn(func() {
            __moriarty_5decea860786e867.GoroutineEnter()
            worker(__moriarty_p0, __moriarty_p1)
            __moriarty_5decea860786e867.GoroutineExit()
        })
    }
}
```

**Benefits:**
- Arguments are evaluated before the goroutine starts (preserving original semantics)
- `GoroutineEnter()` hook allows tracking goroutine creation
- `GoroutineExit()` hook allows cleanup and happens-before relationship tracking
- `Spawn()` wrapper enables custom goroutine scheduling

**Note:** The alias `__moriarty_5decea860786e867` is deterministically generated from the runtime package path.

## Package Structure

```
moriarty/
├── pkg/
│   ├── instrument/          # Core instrumentation library
│   │   ├── instrument.go
│   │   ├── instrument_test.go
│   │   └── README.md
│   └── runtime/            # Runtime tracking functions
│       └── runtime.go      # Stub implementations of MemRead/MemWrite
├── examples/
│   └── library_usage.go    # Example code
├── main.go                 # CLI tool
└── testdata/               # Test files
```

## Runtime Functions

The instrumented code imports `github.com/amirkhaki/moriarty/pkg/runtime` with a deterministically mangled alias which provides:

```go
// Memory operation hooks
func MemRead(addr unsafe.Pointer)
func MemWrite(addr unsafe.Pointer)

// Goroutine lifecycle hooks
func Spawn(f func())
func GoroutineEnter()
func GoroutineExit()
```

**Note:** The import alias is auto-generated (e.g., `__moriarty_5decea860786e867`) to avoid conflicts with Go's built-in `runtime` package and any user imports.

Stub implementations are provided. Implement your own race detection logic by:
1. Forking this repo and modifying `pkg/runtime/runtime.go`, or
2. Using a custom config to point to your own runtime package

### Goroutine Hook Usage

- **`Spawn(f func())`**: Called instead of Go's built-in `go` statement. Allows custom goroutine scheduling or tracking.
- **`GoroutineEnter()`**: Called at the start of each instrumented goroutine. Use for thread-local storage allocation or registration.
- **`GoroutineExit()`**: Called at the end of each instrumented goroutine. Use for cleanup and establishing happens-before relationships.

## Documentation

- [Agent Documentation (AGENTS.md)](AGENTS.md) - Architecture and design decisions
- [Instrumentation Library Documentation](pkg/instrument/README.md)
- [Example Usage](examples/library_usage.go)

## Testing

```bash
# Run library tests
cd pkg/instrument && go test -v

# Test CLI
make
./bin/moriarty testdata/vardef.go
```

## License

See LICENSE file.
