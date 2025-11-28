# Toolexec Example

This example demonstrates using Moriarty with Go's `-toolexec` flag to automatically instrument code during compilation.

## Setup

1. Initialize a Go module:
```bash
go mod init example
go mod edit -replace github.com/amirkhaki/moriarty=../..
go mod tidy
```

2. Build Moriarty:
```bash
cd ../..
go build -o bin/moriarty
cd examples/toolexec
```

## Usage

Build and run with instrumentation:
```bash
go build -toolexec=../../bin/moriarty
./example
```

## How it works

When you use `-toolexec=moriarty`, Go calls Moriarty before each tool (compile, link, asm).
Moriarty:
1. Detects when the `compile` tool is called
2. Instruments all `.go` source files
3. Passes the instrumented code to the compiler
4. Passes through all other tools unchanged

## Requirements

Your source code must import the runtime package (even if unused):
```go
import _ "github.com/amirkhaki/moriarty/pkg/runtime"
```

This ensures the runtime functions (MemRead, MemWrite, etc.) are available at compile time.
