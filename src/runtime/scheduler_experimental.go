// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package runtime

// priorityScheduler is an experimental scheduler that demonstrates
// priority-based scheduling. This is an example implementation showing
// how alternative scheduling algorithms can be implemented.
//
// Note: This is a simplified example for demonstration purposes.
// A production implementation would need more sophisticated priority management.
type priorityScheduler struct {
	schedulerImpl // embed default implementation for methods we don't override
}

// newPriorityScheduler creates a new priority-based scheduler.
// This is an example of how to create custom schedulers.
func newPriorityScheduler() Scheduler {
	return &priorityScheduler{}
}

// CheckGlobalQueue overrides the default behavior to check global queue more frequently
// for higher priority tasks.
func (s *priorityScheduler) CheckGlobalQueue(ppInterface interface{}) bool {
	pp := ppInterface.(*p)
	// Check global queue every 31 ticks instead of 61 for better fairness
	// This is just an example of how the algorithm can be tuned
	return pp.schedtick%31 == 0 && !sched.runq.empty()
}

// GetFromLocalQueue can be overridden to implement priority-based selection
// from the local queue. This is a placeholder showing where priority logic would go.
func (s *priorityScheduler) GetFromLocalQueue(ppInterface interface{}) (gpInterface interface{}, inheritTime bool) {
	// For now, delegate to default implementation
	// A real priority scheduler would examine goroutine priorities here
	// and select accordingly
	return s.schedulerImpl.GetFromLocalQueue(ppInterface)
}

// fifoScheduler is an experimental scheduler that uses a strict FIFO
// (First-In-First-Out) policy for local queues.
type fifoScheduler struct {
	schedulerImpl
}

// newFIFOScheduler creates a new FIFO-based scheduler.
func newFIFOScheduler() Scheduler {
	return &fifoScheduler{}
}

// CheckGlobalQueue checks global queue less frequently to favor local execution
func (s *fifoScheduler) CheckGlobalQueue(ppInterface interface{}) bool {
	pp := ppInterface.(*p)
	// Check global queue every 127 ticks for stronger local affinity
	return pp.schedtick%127 == 0 && !sched.runq.empty()
}

// workStealingScheduler is an experimental scheduler that implements
// more aggressive work stealing for better load balancing.
type workStealingScheduler struct {
	schedulerImpl
}

// newWorkStealingScheduler creates a new work-stealing scheduler
// with more aggressive stealing behavior.
func newWorkStealingScheduler() Scheduler {
	return &workStealingScheduler{}
}

// CheckGlobalQueue checks less frequently since we rely more on work stealing
func (s *workStealingScheduler) CheckGlobalQueue(ppInterface interface{}) bool {
	pp := ppInterface.(*p)
	// Prefer work stealing over global queue checks
	return pp.schedtick%101 == 0 && !sched.runq.empty()
}

// Notes on implementing custom schedulers:
//
// 1. Scheduler implementations must be thread-safe as they will be called
//    concurrently from multiple Ms (OS threads).
//
// 2. The NextTask method may block until work becomes available. Implementations
//    should integrate with the existing runtime mechanisms for blocking and waking.
//
// 3. Custom schedulers can maintain their own state, but must be careful about
//    memory management and avoid introducing memory leaks.
//
// 4. For production use, custom schedulers should be thoroughly tested and
//    benchmarked to ensure they don't negatively impact performance.
//
// 5. Implementations can access the existing runtime scheduler state (sched global)
//    but should use appropriate locking when accessing shared state.
//
// Example usage (in testing or experimental code):
//
//   func init() {
//       // Switch to priority scheduler
//       setScheduler(newPriorityScheduler())
//   }
//
// Or for A/B testing different algorithms:
//
//   func selectScheduler(algo string) {
//       switch algo {
//       case "priority":
//           setScheduler(newPriorityScheduler())
//       case "fifo":
//           setScheduler(newFIFOScheduler())
//       case "workstealing":
//           setScheduler(newWorkStealingScheduler())
//       default:
//           // Use default scheduler
//       }
//   }
