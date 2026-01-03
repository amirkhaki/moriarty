package runtime

import (
	"sync"
)

// ReplayStrategy replays events in the exact recorded order.
// Goroutines are blocked until it's their turn according to the trace.
type ReplayStrategy struct {
	trace     []Event
	idx       int
	traceFile string
	registered   map[uint64]chan struct{}
	registeredMu sync.Mutex
	backLog map[uint64]Event
	i uint64
}

// NewReplayStrategy creates a replay strategy from a trace file.
func NewReplayStrategy(traceFile string) (*ReplayStrategy, error) {
	trace, err := LoadTrace(traceFile)
	if err != nil {
		return nil, err
	}
	s := &ReplayStrategy{
		trace: trace,
		traceFile: traceFile,
		registered: make(map[uint64]chan struct{}),
		backLog: make(map[uint64]Event),
	}
	return s, nil
}

func (s *ReplayStrategy) RegisterGoroutine(goID uint64) {
	s.registeredMu.Lock()
	s.registered[goID] = make(chan struct{})
	s.registeredMu.Unlock()
}
func (s *ReplayStrategy) UnregisterGoroutine(goID uint64) {
	s.registeredMu.Lock()
	delete(s.registered, goID)
	s.registeredMu.Unlock()
}
// Wait blocks until this goroutine's event matches the next expected event.
func (s *ReplayStrategy) Wait(e Event) {
	s.registeredMu.Lock()
	var blockChan = s.registered[e.GoID]
	s.registeredMu.Unlock()
	<-blockChan
}
func (s *ReplayStrategy) unblock(e Event) {
	s.registeredMu.Lock()
	var blockChan = s.registered[e.GoID]
	s.registeredMu.Unlock()
	blockChan <- struct{}{}
}
// OnEvent processes the event
func (s *ReplayStrategy) OnEvent(e Event) {
	if s.idx >= len(s.trace) {
		s.unblock(e)
		return
	}

	s.backLog[s.i] = e
	s.i++
	
	// Process as many events from backlog as possible in trace order
	for s.idx < len(s.trace) {
		expected := s.trace[s.idx]
		found := false
		for k, v := range s.backLog {
			if expected.GoID == v.GoID && expected.Kind == v.Kind {
				// It's this goroutine's turn!
				s.idx++
				s.unblock(v)
				delete(s.backLog, k)
				found = true
				break
			}
		}
		if !found {
			// No matching event in backlog yet, wait for more events
			break
		}
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
	s.trace = trace
	s.idx = 0
	return nil
}
