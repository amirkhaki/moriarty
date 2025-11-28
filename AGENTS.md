# Moriarty - Agent Documentation

This document describes the architecture, design decisions, and implementation details of the Moriarty race detection instrumentation tool.

## Table of Contents

- [Overview](#overview)
- [Architecture](#architecture)
- [Instrumentation Strategy](#instrumentation-strategy)
- [Memory Operation Tracking](#memory-operation-tracking)
- [Goroutine Instrumentation](#goroutine-instrumentation)
- [Edge Cases and Fixes](#edge-cases-and-fixes)
- [Design Decisions](#design-decisions)
- [API Reference](#api-reference)
- [Future Enhancements](#future-enhancements)

---

## Overview

Moriarty is a Go source-to-source instrumentation tool that adds race detection hooks to Go programs. It operates at the AST (Abstract Syntax Tree) level, inserting calls to runtime functions before memory operations and goroutine spawns.

### Key Features

- **Type-aware instrumentation**: Uses Go's type system to make intelligent decisions
- **Comprehensive coverage**: Instruments variables, pointers, structs, arrays, slices, maps, and channels
- **Goroutine lifecycle tracking**: Wraps goroutine spawns with enter/exit hooks
- **Smart filtering**: Skips constants, package names, built-ins, and address-of operations
- **Three-pass design**: Control flow lowering, memory instrumentation, goroutine instrumentation
- **Go toolchain integration**: Works with `go build -toolexec` for seamless compilation
- **Smart import management**: Only adds runtime imports when actually used

---

## Architecture

### Component Structure

```
moriarty/
├── main.go                      # CLI entry point
├── pkg/
│   ├── instrument/
│   │   ├── instrument.go        # Core instrumentation logic
│   │   ├── instrument_test.go   # Unit tests
│   │   └── README.md           # Library documentation
│   └── runtime/
│       └── runtime.go          # Stub runtime functions
├── examples/                    # Example usage
└── testdata/                   # Test cases
```

### Processing Pipeline

```
Source Code (.go files)
      ↓
[1. Parse] → AST (multiple files together for a package)
      ↓
[2. Type Check] → Type Information (with importcfg support)
      ↓
[3. Pass 0: Control Flow Lowering] → Simplify control structures
      ↓
[4. Pass 1: Memory Instrumentation] → Add MemRead/MemWrite
      ↓
[5. Pass 2: Goroutine Instrumentation] → Wrap go statements
      ↓
[6. Add Imports] → Only if instrumentation was added
      ↓
[7. Output] → Instrumented Code (to temp directory)
```

### Multi-File Support and Go Toolchain Integration

**Toolexec Mode (`-toolexec`)**

Moriarty integrates with the Go toolchain via the `-toolexec` flag:

```bash
go build -toolexec /path/to/moriarty
```

When invoked this way:
1. **Intercepts `compile` commands**: Moriarty wraps the Go compiler
2. **Instruments before compilation**: Processes all `.go` files in the package
3. **Preserves build artifacts**: Uses Go's temp directory (`WORK` env var)
4. **Type checking with importcfg**: Loads dependency type information from `-importcfg`
5. **Skips GOROOT files**: Doesn't instrument standard library code

**Multi-file type checking**:
- When using `-toolexec`, the compiler passes multiple `.go` files together
- Moriarty type-checks them as a package to support cross-file references
- Example: `NewCounter()` from `types.go` used in `main.go`

**Implementation modes:**

```go
// Single file mode (standalone CLI usage)
func (instr *Instrumenter) InstrumentFile(fset, filename, src) (*ast.File, error)

// Multi-file mode (used by -toolexec)
func (instr *Instrumenter) InstrumentFiles(fset, filenames) ([]*ast.File, error)
```

**Import configuration**:
- Uses `-importcfg` to resolve package dependencies
- Loads type information for imported packages
- Ensures complete type resolution for instrumentation decisions

---

## Instrumentation Strategy

### Three-Pass Approach

**Pass 0: Control Flow Lowering**
- Transforms complex control flow to simpler forms
- `if init; cond { body }` → `{ init; if cond { body } }`
- `for init; cond; post { body }` → `{ init; for cond { body; post } }`
- Wraps lowered code in blocks to preserve scoping
- Eliminates special case handling in instrumentation
- Only lowers when `InsertBefore` is available

**Pass 1: Memory Operations**
- Instruments: assignments, inc/dec, sends, ranges, returns, expression statements
- Instruments: if/for/switch conditions (now simple after lowering)
- Uses `InsertBefore` to add MemRead/MemWrite calls
- Only operates on nodes in statement slices (checked via `cursor.Index()`)

**Pass 2: Goroutine Spawns**
- Transforms `go f(x, y)` statements
- Evaluates arguments before spawning
- Wraps goroutine with lifecycle hooks
- Must run after first pass to avoid interference

### Why Three Passes?

1. **Lowering first**: Simplifies subsequent instrumentation
2. **Separation of concerns**: Control flow, memory operations, goroutine lifecycle
3. **Avoid instrumentation conflicts**: Each pass operates on well-defined structures
4. **Simpler logic**: No special cases needed for init/post statements

---

## Memory Operation Tracking

### Read Instrumentation

Reads are tracked for:
- **Variable access**: `x`, `obj.field`
- **Array/slice indexing**: `arr[i]` (not maps - can't take address)
- **Pointer dereferencing**: `*p`, `**pp`
- **Binary/unary operations**: `x + y`, `!flag`, `-x`
- **Function arguments**: `f(x, y)`
- **Return values**: `return x`

Reads are **NOT** tracked for:
- **Constants**: `time.Millisecond`, `MAX_VALUE`
- **Package names**: `fmt`, `time`
- **Type names**: `int`, `MyStruct`
- **Built-in functions**: `len`, `make`, `append`
- **Nil values**: `nil`
- **Address-of operations**: `&x` (only gets location, not value)

### Control Flow Lowering and Instrumentation

Control flow structures with init/post statements are lowered before instrumentation:

**If with init statement**:
```go
// Original
if y := 10; x > 5 {
    body
}

// After lowering (Pass 0) - wrapped in block for scoping
{
    y := 10
    if x > 5 {
        body
    }
}

// After instrumentation (Pass 1)
{
    y := 10
    MemRead(&x)
    if x > 5 {
        body
    }
}
```

**For loop with init and post**:
```go
// Original
for i := 0; i < 10; i++ {
    x++
}

// After lowering (Pass 0) - wrapped in block for scoping
{
    i := 0
    for i < 10 {
        x++
        i++
    }
}

// After instrumentation (Pass 1)
{
    i := 0
    MemRead(&i)           // Before loop (first condition check)
    for i < 10 {
        MemRead(&x)
        MemWrite(&x)
        x++
        MemRead(&i)       // Read for post increment
        MemWrite(&i)
        i++
        MemRead(&i)       // Check condition for next iteration
    }
}
```

**For loop with just condition**:
```go
// Original
for x < 100 {
    body
}

// After instrumentation (no lowering needed)
MemRead(&x)           // Before loop (first condition check)
for x < 100 {
    body
    MemRead(&x)       // Check condition for next iteration
}
```

**Switch statements**:
```go
// Original
switch x {
case 1:
    body1
case 2:
    body2
}

// After instrumentation (tag evaluated once before switch)
MemRead(&x)
switch x {
case 1:
    body1
case 2:
    body2
}
```

**Benefits of lowering**:
1. **Simpler instrumentation**: No special cases for init/post
2. **Correct semantics**: Init and post execute exactly once at right times
3. **Better coverage**: Post statements can be instrumented properly
4. **Maintainable**: Single instrumentation strategy for all control flow

### Write Instrumentation

Writes are tracked for:
- **Assignments**: `x = 10`
- **Pointer writes**: `*p = 20`
- **Struct field writes**: `obj.field = 30`
- **Array/slice writes**: `arr[i] = 40`
- **Inc/Dec**: `x++`, `x--`
- **Op-assignments**: `x += 10`, `x *= 2`

Special case - Variable declarations:
- `x := 10` → **Not instrumented** (new variable)
- `x, y := 1, 2` where `x` is new → **Not instrumented**
- `x, y := 1, 2` where `x` already exists → **Instrumented** (redeclaration)

Uses `typeInfo.Defs[ident]` to detect true declarations vs redeclarations.

### Example Transformations

**Simple assignment:**
```go
// Original
x = 20

// Instrumented
MemWrite(&x)
x = 20
```

**Pointer write:**
```go
// Original
*p = 100

// Instrumented
MemRead(&p)      // Read pointer to get target address
MemWrite(&*p)    // Write to target
*p = 100
```

**Double pointer:**
```go
// Original
**pp = 200

// Instrumented
MemRead(&pp)     // Read pp to get address of p
MemRead(&*pp)    // Read p to get address of x
MemWrite(&**pp)  // Write to x
**pp = 200
```

---

## Goroutine Instrumentation

### Transformation Strategy

```go
// Original
go worker(x, y)

// Instrumented
{
    MemRead(&x)          // Track read of x
    __moriarty_p0 := x   // Capture x value
    MemRead(&y)          // Track read of y
    __moriarty_p1 := y   // Capture y value
    runtime.Spawn(func() {
        runtime.GoroutineEnter()
        worker(__moriarty_p0, __moriarty_p1)
        runtime.GoroutineExit()
    })
}
```

### Why This Design?

1. **Argument evaluation**: Arguments are evaluated in parent goroutine (preserves Go semantics)
2. **Memory tracking**: Reads of variables passed to goroutines are tracked
3. **Lifecycle hooks**: Enter/Exit allow tracking goroutine creation/termination
4. **Spawn wrapper**: Enables custom scheduling or additional tracking
5. **Isolation**: New goroutine code is wrapped in a block to avoid scope issues

### Import Management

**Alias Generation**

To avoid conflicts with Go's built-in `runtime` package and user imports, we generate a deterministic alias:

```go
import __moriarty_5decea860786e867 "github.com/amirkhaki/moriarty/pkg/runtime"
import "unsafe"  // Only added when needed
```

The hash `5decea860786e867` is SHA256 of the package path, ensuring:
- No conflicts with user code
- Deterministic (same input → same alias)
- Recognizable prefix for debugging

**Smart Import Addition**

Imports are only added when necessary:
- `moriarty/pkg/runtime`: Added only if memory or goroutine instrumentation was performed
- `unsafe`: Added only if any instrumentation occurred
- Prevents "imported but not used" compile errors
- Checks if instrumentation actually modified the AST before adding imports

**Why this matters**:
- Some files may have no instrumentable code (e.g., only type definitions)
- Adding unused imports would cause compilation errors
- Reduces clutter in instrumented output

---

## Edge Cases and Fixes

### 1. InsertBefore Panic Fix

**Problem**: `InsertBefore` only works when node is in a slice field.

**Solution**: Use `cursor.Index() >= 0` to check if insertion is safe.

**Affected contexts**:
- For loop init/post: `for i := 0; i < 10; i++`
- If statement init: `if x := getValue(); x > 0`
- Switch init: `switch y := getValue(); y`
- Select comm: `case val := <-ch:`

**Implementation**:
```go
func canInsertBefore(c *astutil.Cursor) bool {
    // Index() returns >= 0 if node is in a slice, < 0 otherwise
    return c.Index() >= 0
}
```

**Workaround for conditions**: When `InsertBefore` fails, we use alternative strategies:
- Use the `Init` statement slot if empty
- Wrap in a block for `else if` chains
- Prepend to loop body for conditions evaluated on each iteration

### 2. Constant Instrumentation Fix

**Problem**: Constants like `time.Millisecond` were being instrumented, causing compile errors.

**Solution**: Check type information and skip constants.

```go
case *ast.SelectorExpr:
    if obj := typeInfo.Uses[e.Sel]; obj != nil {
        switch obj.(type) {
        case *types.Const, *types.PkgName, *types.TypeName, *types.Nil:
            return  // Skip instrumentation
        }
    }
```

### 3. Address-of Operator Fix

**Problem**: `&x` was instrumented as a read of `x`, but taking an address doesn't read the value.

**Solution**: Skip instrumentation for `token.AND` unary expressions.

```go
case *ast.UnaryExpr:
    if e.Op == token.AND {
        // Taking address (&x) doesn't read the value
        return
    }
```

**Rationale**:
- `&x` only needs the memory location, not the value
- The actual read happens when dereferencing: `*p`
- This reduces false positives in race detection

### 4. Control Flow Lowering

**Problem**: Control flow with init/post statements (`if init; cond`, `for init; cond; post`) created special cases:
- Init/post statements are single fields, not in slices
- Can't use `InsertBefore` on them
- Post statement `i++` in `for` was being skipped

**Solution**: Lower control flow structures BEFORE instrumentation (Pass 0):

```go
// Transform if
if init; cond { body }
→
{
    init
    if cond { body }
}

// Transform for  
for init; cond; post { body }
→
{
    init
    for cond {
        body
        post
    }
}
```

**Benefits**:
1. **Eliminates special cases**: Init and post are now regular statements
2. **Post can be instrumented**: `i++` is now in loop body (a slice)
3. **Simpler code**: No complex logic for init slot handling
4. **Correct semantics**: Init runs once, post runs after each iteration
5. **Scoping preserved**: Block wrapper prevents variable conflicts with outer scope

**Example**:
```go
// Original
for i := 0; i < 10; i++ { x++ }

// After lowering (wrapped in block)
{
    i := 0
    for i < 10 {
        x++
        i++
    }
}

// After instrumentation
{
    i := 0
    MemRead(&i)  // First condition check
    for i < 10 {
        MemRead(&x); MemWrite(&x); x++
        MemRead(&i); MemWrite(&i); i++
        MemRead(&i)  // Condition check for next iteration
    }
}

// If there was an outer 'i', it's not affected!
```

### 5. GOROOT Filtering

**Problem**: Instrumenting standard library files causes issues:
- Cannot modify standard library code
- Type information may be incomplete for stdlib
- Unnecessary overhead

**Solution**: Skip files in GOROOT.

```go
if strings.HasPrefix(filename, runtime.GOROOT()) {
    return // Skip instrumentation
}
```

**Detection**: Uses `runtime.GOROOT()` to check if file is in stdlib.

### 6. Conditional Import Addition

**Problem**: Adding imports when no instrumentation occurred causes "unused import" errors.

**Solution**: Track whether instrumentation was actually performed.

```go
func (instr *Instrumenter) instrumentASTs(...) {
    instrumented := false
    
    // Perform instrumentation...
    if /* any instrumentation happened */ {
        instrumented = true
    }
    
    // Only add imports if we actually instrumented
    if instrumented {
        addImports(file, runtimeImport, unsafeImport)
    }
}
```

**Tracking**:
- Check if any MemRead/MemWrite calls were added
- Check if any goroutine transformations occurred
- Only add imports when at least one instrumentation happened

---

## Design Decisions

### Why Control Flow Lowering?

**Alternative**: Special case handling for init/post statements.

**Our choice**: Lower control flow to simpler forms first.

**Transformation**:
- `if init; cond { body }` → `{ init; if cond { body } }`
- `for init; cond; post { body }` → `{ init; for cond { body; post } }`

**Benefits**:
1. **Eliminates special cases**: No need to handle Init/Post separately
2. **Simpler instrumentation**: Single strategy for all control flow
3. **Better coverage**: Post statements can be instrumented (were skipped before)
4. **Maintainable**: Clear separation between lowering and instrumentation
5. **Correct semantics**: Init runs once before, post runs after each iteration
6. **Scoping preserved**: Block wrapping prevents variable name conflicts

**Example**:
```go
// Original
for i := 0; i < 10; i++ { x++ }

// Lowered (wrapped in block)
{
    i := 0
    for i < 10 { x++; i++ }
}

// Now i++ can be instrumented normally!
// And 'i' doesn't conflict with outer scope
```

### Why Not Runtime Rewriting?

**Alternative**: Use `runtime` package hooks at execution time.

**Our choice**: Source-to-source transformation.

**Reasons**:
1. **Visibility**: Users can see instrumented code
2. **Debugging**: Easier to debug transformation issues
3. **Flexibility**: Can instrument before compilation
4. **No runtime overhead**: Instrumentation is explicit
5. **Portability**: Works with any Go toolchain

### Why Separate collectReads/collectWrites?

**Reason**: Different semantics for LHS vs RHS.

**Example**:
```go
x = y + z
```
- LHS (`x`): **Write**
- RHS (`y + z`): **Reads**

For compound operations:
```go
*p = *q
```
- Read `p` to get target address
- Read `q` to get target address  
- Read `*q` to get value
- Write to `*p`

### Why Skip New Variable Declarations?

**Reason**: Race detection tracks concurrent access to shared memory.

**Logic**:
- New variable: `x := 10` - No one else has access yet
- Reassignment: `x = 20` - Could race with other accesses
- Redeclaration: `x, y := 1, 2` where `x` exists - Could race

We use `typeInfo.Defs[ident]` to distinguish:
- `Defs[ident] != nil` → New declaration (skip)
- `Defs[ident] == nil` → Redeclaration (instrument)

---

## API Reference

### Core Types

```go
type Config struct {
    BaseRuntimeAddress string
    RuntimeAlias       string
    MemReadFunc        string
    MemWriteFunc       string
    SpawnFunc          string
    GoroutineEnterFunc string
    GoroutineExitFunc  string
    ImportRewrites     map[string]string
}

type Instrumenter struct {
    config   *Config
    typeInfo *types.Info
}
```

### Main Functions

```go
// Create instrumenter with config
func NewInstrumenter(config *Config) *Instrumenter

// Instrument a single file by path
func (instr *Instrumenter) InstrumentFile(
    fset *token.FileSet, 
    filename string, 
    src interface{},
) (*ast.File, error)

// Instrument multiple files together (for proper cross-file type checking)
func (instr *Instrumenter) InstrumentFiles(
    fset *token.FileSet,
    filenames []string,
) ([]*ast.File, error)

// Instrument a single AST directly
func (instr *Instrumenter) InstrumentAST(
    fset *token.FileSet, 
    f *ast.File,
) (*ast.File, error)

// Instrument multiple ASTs together (for proper cross-file type checking)
func (instr *Instrumenter) InstrumentASTs(
    fset *token.FileSet,
    files []*ast.File,
) ([]*ast.File, error)
```

**When to use each:**
- `InstrumentFile`: For single standalone files or CLI usage
- `InstrumentFiles`: For multi-file packages (used by `-toolexec`)
- `InstrumentAST`: For pre-parsed single files
- `InstrumentASTs`: For pre-parsed multi-file packages

### Configuration Options

**Config struct fields:**
- `BaseRuntimeAddress`: Import path for runtime package (default: `github.com/amirkhaki/moriarty/pkg/runtime`)
- `RuntimeAlias`: Generated alias to avoid conflicts (computed from hash)
- `MemReadFunc`: Name of memory read hook (default: `MemRead`)
- `MemWriteFunc`: Name of memory write hook (default: `MemWrite`)
- `SpawnFunc`: Name of goroutine spawn wrapper (default: `Spawn`)
- `GoroutineEnterFunc`: Name of goroutine enter hook (default: `GoroutineEnter`)
- `GoroutineExitFunc`: Name of goroutine exit hook (default: `GoroutineExit`)
- `ImportRewrites`: Map for rewriting import paths (advanced use)


### CLI Usage

**Standalone mode:**
```bash
# Instrument a single file
moriarty input.go -o output.go

# Instrument and write to stdout
moriarty input.go
```

**Toolexec mode (integrated with go build):**
```bash
# Build with instrumentation
go build -toolexec /path/to/moriarty

# Build specific package
go build -toolexec ./bin/moriarty ./mypackage

# Works with other go commands
go test -toolexec ./bin/moriarty
go install -toolexec ./bin/moriarty
```

**How toolexec works:**
1. Go invokes moriarty for each tool in the build chain
2. Moriarty detects `compile` commands (ignores others like `link`, `asm`)
3. Instruments source files before compilation
4. Passes instrumented files to the real compiler
5. Build output includes instrumented code

### Runtime Functions (Stubs)

```go
// Memory operation hooks
func MemRead(addr unsafe.Pointer)
func MemWrite(addr unsafe.Pointer)

// Goroutine lifecycle hooks
func Spawn(f func())
func GoroutineEnter()
func GoroutineExit()
```

**Implementation Note**: These are stubs. Users should implement their own race detection logic.

---

## Future Enhancements

### Potential Improvements

1. **Channel Operation Tracking**
   - `ch <- val` (send)
   - `val := <-ch` (receive)
   - `close(ch)`

2. **Sync Primitive Instrumentation**
   - `sync.Mutex.Lock/Unlock`
   - `sync.RWMutex` operations
   - `sync.WaitGroup` operations
   - `atomic` package operations

3. **Map Operation Tracking**
   - Currently skipped (can't take address of map element)
   - Could track map itself: `m[key] = val`

4. **Defer Statement Handling**
   - Track deferred function calls
   - Ensure proper ordering with function exit

5. **Type Conversion Handling**
   - Fix `(*int)(ptr)` instrumentation
   - Handle `unsafe.Sizeof(Type)` correctly

6. **Performance Optimization**
   - Cache type lookups
   - Reduce redundant instrumentation
   - Optional instrumentation levels (basic/full)

7. **Better Error Messages**
   - Report why instrumentation was skipped
   - Warnings for unsupported constructs

8. **Configuration Options**
   - Include/exclude patterns
   - Selective instrumentation by package
   - Custom runtime package paths

9. **Switch Statement Lowering** (Design Discussion)
   - Convert all switch statements to if-else in Pass 0
   - **Value switches**: `switch x { case 1: ... }` → `if x == 1 { ... }`
   - **Multi-value cases**: `case 1, 2, 3:` → `if x == 1 || x == 2 || x == 3`
   - **Fallthrough**: Use goto labels to jump to next case body
   - **Type switches**: `switch v := x.(type)` → `if v, ok := x.(int); ok { ... }`
   - **Benefits**:
     - Eliminates all switch-specific instrumentation code
     - Unified design: everything becomes if-else
     - Tag/expression evaluated exactly once (stored in temp variable)
     - Reuses existing if-else instrumentation logic
     - Fits perfectly with lowering philosophy
   - **Current approach**: Direct instrumentation (InsertBefore switch)
   - **Trade-off**: Conversion adds complexity to lowering pass, but simplifies instrumentation pass
   - **Priority**: Low - current approach works well, this is an optimization

---

## Testing Strategy

### Test Categories

1. **Unit Tests** (`pkg/instrument/instrument_test.go`)
   - Alias generation
   - Import handling
   - Type detection
   - Multi-file instrumentation

2. **Integration Tests** (`testdata/`)
   - Variable operations
   - Pointer manipulation
   - Goroutine spawning
   - Edge cases (for loops, select, etc.)

3. **Compilation Tests**
   - All instrumented code must compile
   - All instrumented code must run correctly
   - Multi-file packages with cross-file references

### Test Coverage

- ✅ Basic variables
- ✅ Pointers (single, double, triple indirection)
- ✅ Structs and fields
- ✅ Arrays and slices
- ✅ For loops (standard, range, infinite)
- ✅ If/switch with init statements
- ✅ Select statements
- ✅ Goroutines with various argument types
- ✅ Constants and built-ins
- ✅ Address-of operations
- ✅ Linked lists and complex data structures
- ✅ Multi-file packages with cross-file type references

---

## Contributing

When adding new features:

1. **Add test cases** in `testdata/`
2. **Update this document** with design rationale
3. **Run all tests**: `go test ./...`
4. **Test instrumented code**: Ensure it compiles and runs
5. **Check edge cases**: For loops, select, defers, etc.

---

## References

- [Go AST Documentation](https://pkg.go.dev/go/ast)
- [Go Types Documentation](https://pkg.go.dev/go/types)
- [AST Utilities](https://pkg.go.dev/golang.org/x/tools/go/ast/astutil)
- [ThreadSanitizer Paper](https://static.googleusercontent.com/media/research.google.com/en//pubs/archive/35604.pdf)
- [Go Race Detector](https://go.dev/blog/race-detector)

---

## Usage Examples

### Basic Instrumentation

```bash
# Instrument a simple program
moriarty examples/hello/main.go -o /tmp/instrumented.go

# Run instrumented version
go run /tmp/instrumented.go
```

### Building with Toolexec

```bash
# Build the moriarty binary first
make build

# Use it with go build
go build -toolexec ./bin/moriarty ./examples/hello

# The resulting binary includes instrumentation
./hello
```

### Multi-file Package

```bash
# Instrument a package with multiple files
go build -toolexec ./bin/moriarty -o myapp ./mypackage
```

### Debugging Instrumentation

```bash
# Set WORK environment variable to see intermediate files
WORK=/tmp/moriarty go build -toolexec ./bin/moriarty

# Check the instrumented source in /tmp/moriarty
```

---

*Last Updated: 2025-11-28*
*Version: 1.1*
