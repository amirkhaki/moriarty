package runtime

import (
	"fmt"
	"os"
	"sync"
)

// RecordStrategy records all events to a trace file.
// It doesn't enforce any particular ordering - just observes.
type RecordStrategy struct {
	trace     []Event
	mu        sync.Mutex
	traceFile string
}

// NewRecordStrategy creates a new recording strategy.
func NewRecordStrategy(traceFile string) *RecordStrategy {
	return &RecordStrategy{traceFile: traceFile}
}

func (s *RecordStrategy) RegisterGoroutine(goID uint64) {}
func (s *RecordStrategy) UnregisterGoroutine(goID uint64) {}
// OnEvent records the event without blocking.
func (s *RecordStrategy) OnEvent(e Event) {
	s.mu.Lock()
	s.trace = append(s.trace, e)
	s.mu.Unlock()
}

func (s *RecordStrategy) Wait(e Event) {}
// OnFinalize saves the recorded trace to file.
func (s *RecordStrategy) OnFinalize() {
	if s.traceFile == "" {
		return
	}
	if err := SaveTrace(s.traceFile, s.trace); err != nil {
		fmt.Fprintf(os.Stderr, "moriarty: %v\n", err)
	}
}

// RecordTrace saves the current trace to file.
func (s *RecordStrategy) RecordTrace() error {
	return SaveTrace(s.traceFile, s.trace)
}
