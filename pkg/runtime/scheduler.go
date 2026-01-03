package runtime

// scheduler coordinates goroutines and delegates to a strategy.
type scheduler struct {
	strategy Strategy
	events   chan Event
}


func newScheduler(strategy Strategy) *scheduler {
	s := &scheduler{
		strategy:   strategy,
		events:     make(chan Event),
	}
	go s.run()
	return s
}

func (s *scheduler) registerGoroutine(goID uint64) {
	s.strategy.RegisterGoroutine(goID)
}

func (s *scheduler) run() {
	for e := range s.events {
		s.strategy.OnEvent(e)
	}
}
func (s *scheduler) unregisterGoroutine(goID uint64) {
	s.strategy.UnregisterGoroutine(goID)
}

func (s *scheduler) yield(e Event) {
	s.events <- e
	s.strategy.Wait(e)
}
