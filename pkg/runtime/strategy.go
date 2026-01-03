package runtime

// Strategy defines the interface for scheduling strategies.
// Implementations control how goroutines are scheduled during execution.
type Strategy interface {
	RegisterGoroutine(goID uint64)
	UnregisterGoroutine(goID uint64)
	// OnEvent is called when a goroutine wants to perform an operation.
	// The strategy records or processes the event.
	OnEvent(e Event)
	// Wait blocks the caller until it's appropriate to proceed.
	Wait(e Event)

	// OnFinalize is called at the end of main to perform cleanup (e.g., save trace).
	OnFinalize()
}

// Recorder is a strategy that can save its execution trace.
type Recorder interface {
	Strategy
	RecordTrace() error
}

// Replayer is a strategy that can load and replay a trace.
type Replayer interface {
	Strategy
	ReplayTrace() error
}
