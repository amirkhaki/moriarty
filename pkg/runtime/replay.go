package runtime

import (
	"sync"
)

// ReplayStrategy replays events in the exact recorded order.
// Goroutines are blocked until it's their turn according to the trace.
type ReplayStrategy struct {
	trace     []Event
	idx       int
	cond      *sync.Cond
	traceFile string
}

// NewReplayStrategy creates a replay strategy from a trace file.
func NewReplayStrategy(traceFile string) (*ReplayStrategy, error) {
	trace, err := LoadTrace(traceFile)
	if err != nil {
		return nil, err
	}
	s := &ReplayStrategy{trace: trace, traceFile: traceFile}
	s.cond = sync.NewCond(&sync.Mutex{})
	return s, nil
}

// Yield blocks until this goroutine's event matches the next expected event.
func (s *ReplayStrategy) Yield(e Event) {
	s.cond.L.Lock()
	defer s.cond.L.Unlock()

	for {
		if s.idx >= len(s.trace) {
			// Trace exhausted - allow execution to continue
			return
		}

		expected := s.trace[s.idx]
		if expected.GoID == e.GoID && expected.Kind == e.Kind {
			// It's our turn!
			s.idx++
			s.cond.Broadcast()
			return
		}

		// Not our turn - wait
		s.cond.Wait()
	}
}

// OnFinalize does nothing for replay.
func (s *ReplayStrategy) OnFinalize() {}

// ReplayTrace reloads the trace file.
func (s *ReplayStrategy) ReplayTrace() error {
	trace, err := LoadTrace(s.traceFile)
	if err != nil {
		return err
	}
	s.cond.L.Lock()
	s.trace = trace
	s.idx = 0
	s.cond.L.Unlock()
	return nil
}
