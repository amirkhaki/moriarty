package runtime

import (
	"math/rand"
	"sort"
	"sync"
)

// RandomStrategy replays events but randomly picks which goroutine proceeds
// when multiple goroutines are waiting. This explores different interleavings
// of the same program execution.
type RandomStrategy struct {
	// Events grouped by goroutine ID
	pending map[uint64][]Event
	mu      sync.Mutex
	cond    *sync.Cond

	// Random source
	rng *rand.Rand

	// Track which goroutines are currently blocked in Yield
	waiting map[uint64]bool

	traceFile string
}

// NewRandomStrategy creates a strategy that randomly orders goroutine execution.
// seed controls the random permutation (use same seed for reproducibility).
func NewRandomStrategy(traceFile string, seed int64) (*RandomStrategy, error) {
	trace, err := LoadTrace(traceFile)
	if err != nil {
		return nil, err
	}

	s := &RandomStrategy{
		pending:   groupByGoID(trace),
		rng:       rand.New(rand.NewSource(seed)),
		waiting:   make(map[uint64]bool),
		traceFile: traceFile,
	}
	s.cond = sync.NewCond(&s.mu)
	return s, nil
}

// Yield blocks until this goroutine is randomly selected to proceed.
// OnEvent blocks until this goroutine is randomly selected to proceed.
func (s *RandomStrategy) OnEvent(e Event) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if this goroutine has pending events
	events, hasPending := s.pending[e.GoID]
	if !hasPending || len(events) == 0 {
		// No more events for this goroutine - allow it to proceed
		return
	}

	// Verify the event matches what we expect
	expected := events[0]
	if expected.Kind != e.Kind {
		// Event mismatch - allow to proceed (trace diverged)
		return
	}

	// Mark this goroutine as waiting
	s.waiting[e.GoID] = true
	s.cond.Broadcast()

	// Wait until we're selected
	for {
		// Get all waiting goroutines (sorted for determinism)
		var waitingIDs []uint64
		for id, isWaiting := range s.waiting {
			if isWaiting {
				// Only include if they have pending events
				if evts, ok := s.pending[id]; ok && len(evts) > 0 {
					waitingIDs = append(waitingIDs, id)
				}
			}
		}
		// Sort for deterministic ordering with same seed
		sortUint64(waitingIDs)

		if len(waitingIDs) == 0 {
			// No one waiting, proceed
			break
		}

		// Randomly pick one
		selectedIdx := s.rng.Intn(len(waitingIDs))
		selectedID := waitingIDs[selectedIdx]

		if selectedID == e.GoID {
			// We're selected! Consume the event and proceed
			s.pending[e.GoID] = s.pending[e.GoID][1:]
			s.waiting[e.GoID] = false
			s.cond.Broadcast()
			return
		}

		// Not selected, wait for next round
		s.cond.Wait()
	}

	// Consume our event
	s.pending[e.GoID] = s.pending[e.GoID][1:]
	s.waiting[e.GoID] = false
}
func (s *RandomStrategy) RegisterGoroutine(goID uint64) {}
func (s *RandomStrategy) UnregisterGoroutine(goID uint64) {}

// OnFinalize does nothing.
func (s *RandomStrategy) OnFinalize() {}

func (s *RandomStrategy) Wait(e Event) {}
// ReplayTrace reloads and re-randomizes.
func (s *RandomStrategy) ReplayTrace() error {
	trace, err := LoadTrace(s.traceFile)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.pending = groupByGoID(trace)
	s.waiting = make(map[uint64]bool)
	return nil
}

func sortUint64(s []uint64) {
	sort.Slice(s, func(i, j int) bool { return s[i] < s[j] })
}
