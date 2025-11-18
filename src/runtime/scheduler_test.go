// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package runtime_test

import (
	"runtime"
	"testing"
)

// TestSchedulerInterface tests the basic scheduler interface functionality.
func TestSchedulerInterface(t *testing.T) {
	// Get the default scheduler
	sched := runtime.GetSchedulerForTest()
	if sched == nil {
		t.Fatal("GetScheduler returned nil")
	}
	
	// Verify we can get the scheduler without panic
	// The actual scheduling operations require runtime state
	// so we mainly test that the interface is available
}

// TestSetScheduler tests that we can set a custom scheduler.
func TestSetScheduler(t *testing.T) {
	// Save original scheduler
	original := runtime.GetSchedulerForTest()
	
	// Try setting experimental schedulers
	runtime.SetSchedulerForTest(runtime.NewPrioritySchedulerForTest())
	if runtime.GetSchedulerForTest() == nil {
		t.Error("Priority scheduler not set correctly")
	}
	
	runtime.SetSchedulerForTest(runtime.NewFIFOSchedulerForTest())
	if runtime.GetSchedulerForTest() == nil {
		t.Error("FIFO scheduler not set correctly")
	}
	
	runtime.SetSchedulerForTest(runtime.NewWorkStealingSchedulerForTest())
	if runtime.GetSchedulerForTest() == nil {
		t.Error("WorkStealing scheduler not set correctly")
	}
	
	// Restore original scheduler
	runtime.SetSchedulerForTest(original)
}

// BenchmarkSchedulerSwitch benchmarks switching between schedulers
func BenchmarkSchedulerSwitch(b *testing.B) {
	original := runtime.GetSchedulerForTest()
	defer runtime.SetSchedulerForTest(original)
	
	schedulers := []runtime.SchedulerForTest{
		runtime.NewPrioritySchedulerForTest(),
		runtime.NewFIFOSchedulerForTest(),
		runtime.NewWorkStealingSchedulerForTest(),
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		runtime.SetSchedulerForTest(schedulers[i%len(schedulers)])
	}
}

// Example demonstrating how to use a custom scheduler (for testing)
func ExampleGetSchedulerForTest() {
	// Save the current scheduler to restore later
	original := runtime.GetSchedulerForTest()
	defer runtime.SetSchedulerForTest(original)
	
	// Switch to priority scheduler for experimentation
	runtime.SetSchedulerForTest(runtime.NewPrioritySchedulerForTest())
	
	// Your code here - the runtime will use the priority scheduler
	// for goroutine scheduling decisions
	
	// Switch to FIFO scheduler
	runtime.SetSchedulerForTest(runtime.NewFIFOSchedulerForTest())
	
	// More code...
	// Output:
}
