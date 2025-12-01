// Package runtime provides stub implementations of memory tracking functions
// for race detection instrumentation.
package runtime

import (
	"github.com/amirkhaki/moriarty/pkg/goid"
	"sync"
	"unsafe"
)

type val struct{}
type kind uint64

const (
	_ kind = 1 << iota
	kRead
	kWrite
	kSpawn
	kGoEnter
	kGoExit
)

type event struct {
	id uint64
	k  kind
}

type scheduler struct {
	channels map[uint64]chan val
	events   chan event
	mu       sync.Mutex
}

func (s *scheduler) register(id uint64, ch chan val) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.channels[id] = ch
}

func (s *scheduler) run() {
	for e := range s.events {
		switch e.k {
		case kRead:
		case kWrite:
		case kSpawn:
		case kGoEnter:
		case kGoExit:
		}
	}
}

var sched scheduler

func init() {
	sched.channels = make(map[uint64]chan val)
	sched.events = make(chan event)
	go sched.run()
}

// MemRead is called before a memory read operation.
// addr points to the memory location being read.
func MemRead(addr unsafe.Pointer) {
	id := goid.Get()
	sched.events <- event{id, kRead}
	<-sched.channels[id]
}

// MemWrite is called before a memory write operation.
// addr points to the memory location being written.
func MemWrite(addr unsafe.Pointer) {
	id := goid.Get()
	sched.events <- event{id, kWrite}
	<-sched.channels[id]
}

// Spawn launches a new goroutine with the given function.
// This wraps the standard go statement to enable tracking.
func Spawn(f func()) {
	id := goid.Get()
	sched.events <- event{id, kSpawn}
	<-sched.channels[id]
	go f()
}

// GoroutineEnter is called at the start of each instrumented goroutine.
func GoroutineEnter() {
	id := goid.Get()
	ch := make(chan val)
	sched.register(id, ch)
	sched.events <- event{id, kGoEnter}
	<-ch
}

// GoroutineExit is called at the end of each instrumented goroutine.
func GoroutineExit() {
	id := goid.Get()
	sched.events <- event{id, kGoExit}
	<-sched.channels[id]
}
