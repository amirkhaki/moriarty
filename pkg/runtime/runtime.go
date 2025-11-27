// Package runtime provides stub implementations of memory tracking functions
// for race detection instrumentation.
package runtime

import "unsafe"

// MemRead is called before a memory read operation.
// addr points to the memory location being read.
func MemRead(addr unsafe.Pointer) {
	// Stub function for memory read instrumentation
	// To be implemented by race detector runtime
	//
	// Example implementation might:
	// - Record the read in a thread-local shadow memory
	// - Check for conflicting writes from other goroutines
	// - Report race if detected
	println("mem read")
}

// MemWrite is called before a memory write operation.
// addr points to the memory location being written.
func MemWrite(addr unsafe.Pointer) {
	// Stub function for memory write instrumentation
	// To be implemented by race detector runtime
	//
	// Example implementation might:
	// - Record the write in a thread-local shadow memory
	// - Check for conflicting reads/writes from other goroutines
	// - Report race if detected
	println("mem write")
}

// Spawn launches a new goroutine with the given function.
// This wraps the standard go statement to enable tracking.
func Spawn(f func()) {
	// Stub function for goroutine spawn instrumentation
	// To be implemented by race detector runtime
	go f()
}

// GoroutineEnter is called at the start of each instrumented goroutine.
func GoroutineEnter() {
	// Stub function for goroutine entry hook
	// To be implemented by race detector runtime
	//
	// Example implementation might:
	// - Allocate thread-local storage for this goroutine
	// - Register the goroutine in a global tracker
	// - Set up happens-before relationships
	println("goroutine enter")
}

// GoroutineExit is called at the end of each instrumented goroutine.
func GoroutineExit() {
	// Stub function for goroutine exit hook
	// To be implemented by race detector runtime
	//
	// Example implementation might:
	// - Clean up thread-local storage
	// - Update happens-before relationships
	// - Flush pending race reports
	println("goroutine exit")
}
