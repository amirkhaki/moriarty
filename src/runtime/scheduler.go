// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package runtime

// Scheduler is an interface that abstracts the goroutine scheduling algorithm.
// This interface allows experimenting with different scheduling strategies
// while maintaining the same runtime infrastructure.
//
// The scheduler's primary responsibility is to select the next goroutine to run
// on a processor (P). Different implementations can use various algorithms
// for fairness, priority, work stealing, etc.
//
// Note: The actual implementation uses runtime internal types (*p, *m, *g) which
// are defined in runtime2.go. This interface is defined here for documentation
// and will be properly typed when integrated with the runtime.
type Scheduler interface {
	// NextTask finds and returns the next goroutine to run on the given processor.
	// It should return:
	//   - gp: the goroutine to execute
	//   - inheritTime: whether the goroutine should inherit the remaining time slice
	//   - tryWakeP: whether to try waking up another P if this is a special goroutine
	//
	// This method may block until work is available.
	// Signature: NextTask(pp *p, mp *m) (gp *g, inheritTime bool, tryWakeP bool)
	NextTask(pp, mp interface{}) (gp interface{}, inheritTime bool, tryWakeP bool)

	// QueueTask adds a runnable goroutine to the appropriate run queue.
	// The next parameter indicates if this goroutine should run next (if possible).
	// The local parameter indicates if it should be added to the local queue (true)
	// or global queue (false).
	// Signature: QueueTask(gp *g, pp *p, next bool, local bool)
	QueueTask(gp, pp interface{}, next bool, local bool)

	// StealWork attempts to steal work from other processors.
	// Returns:
	//   - gp: a goroutine stolen from another P
	//   - inheritTime: whether the goroutine should inherit time
	//   - tnow: current time
	//   - pollUntil: time of next timer to poll
	//   - newWork: whether new work was discovered that requires rescanning
	// Signature: StealWork(pp *p, mp *m, now int64) (gp *g, inheritTime bool, tnow int64, pollUntil int64, newWork bool)
	StealWork(pp, mp interface{}, now int64) (gp interface{}, inheritTime bool, tnow int64, pollUntil int64, newWork bool)

	// GetFromLocalQueue gets a goroutine from the processor's local run queue.
	// Returns the goroutine and whether it should inherit the current time slice.
	// Signature: GetFromLocalQueue(pp *p) (gp *g, inheritTime bool)
	GetFromLocalQueue(pp interface{}) (gp interface{}, inheritTime bool)

	// GetFromGlobalQueue gets goroutines from the global run queue.
	// Returns the first goroutine to run and a batch of additional goroutines
	// to add to the local queue.
	// Signature: GetFromGlobalQueue(pp *p) (gp *g)
	GetFromGlobalQueue(pp interface{}) (gp interface{})

	// CheckGlobalQueue checks if the global queue should be checked for fairness.
	// This is typically done periodically to prevent starvation.
	// Signature: CheckGlobalQueue(pp *p) bool
	CheckGlobalQueue(pp interface{}) bool
}

// schedulerImpl is the default scheduler implementation that maintains
// the current Go scheduler behavior.
type schedulerImpl struct{}

var defaultScheduler Scheduler = &schedulerImpl{}

// GetScheduler returns the current scheduler implementation.
// This can be overridden for testing or experimentation.
func getScheduler() Scheduler {
	return defaultScheduler
}

// SetScheduler sets a custom scheduler implementation.
// This should only be called during initialization before any goroutines are running.
// For testing and experimentation only.
func setScheduler(s Scheduler) {
	defaultScheduler = s
}
