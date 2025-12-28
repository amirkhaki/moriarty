// Package runtime provides deterministic execution and trace recording/replay
// for race detection instrumentation.
package runtime

import (
	"fmt"
	"os"
	"sync"
	"unsafe"

	"github.com/amirkhaki/moriarty/pkg/goid"
)

// Global scheduler instance
var (
	sched   *scheduler
	schedMu sync.Mutex
)

// SetStrategy sets the scheduling strategy. Must be called before Initialize.
func SetStrategy(s Strategy) {
	schedMu.Lock()
	sched = newScheduler(s)
	schedMu.Unlock()
}

// GetStrategy returns the current strategy.
func GetStrategy() Strategy {
	schedMu.Lock()
	defer schedMu.Unlock()
	if sched == nil {
		return nil
	}
	return sched.strategy
}

// Initialize sets up the runtime. Must be called at the start of main.
// Environment variables:
//   - MORIARTY_MODE: "record" (default), "replay", or "random"
//   - MORIARTY_TRACE: path to trace file (default: "moriarty.trace")
//   - MORIARTY_SEED: random seed for "random" mode (default: 0)
func Initialize() {
	traceFile := os.Getenv("MORIARTY_TRACE")
	if traceFile == "" {
		traceFile = "moriarty.trace"
	}

	schedMu.Lock()
	if sched == nil {
		var strategy Strategy
		modeStr := os.Getenv("MORIARTY_MODE")
		switch modeStr {
		case "replay":
			s, err := NewReplayStrategy(traceFile)
			if err != nil {
				schedMu.Unlock()
				fmt.Fprintf(os.Stderr, "moriarty: failed to load trace: %v\n", err)
				os.Exit(1)
			}
			strategy = s
		case "random":
			seed := int64(0)
			if seedStr := os.Getenv("MORIARTY_SEED"); seedStr != "" {
				if _, err := fmt.Sscanf(seedStr, "%d", &seed); err != nil {
					schedMu.Unlock()
					fmt.Fprintf(os.Stderr, "moriarty: invalid seed %q: %v\n", seedStr, err)
					os.Exit(1)
				}
			}
			s, err := NewRandomStrategy(traceFile, seed)
			if err != nil {
				schedMu.Unlock()
				fmt.Fprintf(os.Stderr, "moriarty: failed to load trace: %v\n", err)
				os.Exit(1)
			}
			strategy = s
		default:
			strategy = NewRecordStrategy(traceFile)
		}
		sched = newScheduler(strategy)
	}
	schedMu.Unlock()

	// Register main goroutine
	id := goid.Get()
	sched.registerGoroutine(id)
}

// Finalize cleans up the runtime. Must be called at the end of main.
func Finalize() {
	schedMu.Lock()
	s := sched
	schedMu.Unlock()

	if s != nil {
		s.strategy.OnFinalize()
	}
}

// --- Instrumentation Hooks ---

// MemRead is called before a memory read operation.
func MemRead(addr unsafe.Pointer) {
	id := goid.Get()
	sched.yield(Event{GoID: id, Kind: KindRead, Addr: uintptr(addr)})
}

// MemWrite is called before a memory write operation.
func MemWrite(addr unsafe.Pointer) {
	id := goid.Get()
	sched.yield(Event{GoID: id, Kind: KindWrite, Addr: uintptr(addr)})
}

// Spawn launches a new goroutine with the given function.
func Spawn(f func()) {
	id := goid.Get()
	sched.yield(Event{GoID: id, Kind: KindSpawn})

	newID := goid.Gen()
	sched.registerGoroutine(newID)

	go func() {
		goid.Assign(newID)
		f()
	}()
}

// GoroutineEnter is called at the start of each instrumented goroutine.
func GoroutineEnter() {
	id := goid.Get()
	sched.yield(Event{GoID: id, Kind: KindGoEnter})
}

// GoroutineExit is called at the end of each instrumented goroutine.
func GoroutineExit() {
	id := goid.Get()
	sched.yield(Event{GoID: id, Kind: KindGoExit})
	sched.unregisterGoroutine(id)
	goid.Delete()
}
