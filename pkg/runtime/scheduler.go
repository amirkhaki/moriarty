package runtime

import "sync"

// scheduler coordinates goroutines and delegates to a strategy.
type scheduler struct {
	strategy Strategy

	// Track registered goroutines (for future use by advanced strategies)
	registered   map[uint64]bool
	registeredMu sync.Mutex
}

func newScheduler(strategy Strategy) *scheduler {
	return &scheduler{
		strategy:   strategy,
		registered: make(map[uint64]bool),
	}
}

func (s *scheduler) registerGoroutine(goID uint64) {
	s.registeredMu.Lock()
	s.registered[goID] = true
	s.registeredMu.Unlock()
}

func (s *scheduler) unregisterGoroutine(goID uint64) {
	s.registeredMu.Lock()
	delete(s.registered, goID)
	s.registeredMu.Unlock()
}

func (s *scheduler) yield(e Event) {
	s.strategy.Yield(e)
}
