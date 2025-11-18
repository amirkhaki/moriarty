# Scheduler Interface Design

## Overview

This document describes the scheduler interface implementation for the Go runtime, which allows experimenting with different goroutine scheduling algorithms while maintaining the existing runtime infrastructure.

## Motivation

The Go scheduler is responsible for distributing ready-to-run goroutines across available processor (P) resources. While the current scheduler works well for most workloads, there are scenarios where alternative scheduling strategies might be beneficial:

- Real-time or latency-sensitive applications requiring priority-based scheduling
- Workloads with specific affinity requirements
- Research and experimentation with novel scheduling algorithms
- A/B testing different scheduling strategies

## Interface Design

The `Scheduler` interface abstracts the core scheduling operations:

```go
type Scheduler interface {
    // NextTask finds and returns the next goroutine to run
    NextTask(pp *p, mp *m) (gp *g, inheritTime bool, tryWakeP bool)
    
    // QueueTask adds a runnable goroutine to a run queue
    QueueTask(gp *g, pp *p, next bool, local bool)
    
    // StealWork attempts to steal work from other processors
    StealWork(pp *p, mp *m, now int64) (gp *g, inheritTime bool, tnow int64, pollUntil int64, newWork bool)
    
    // GetFromLocalQueue gets a goroutine from the processor's local queue
    GetFromLocalQueue(pp *p) (gp *g, inheritTime bool)
    
    // GetFromGlobalQueue gets goroutines from the global run queue
    GetFromGlobalQueue(pp *p) (gp *g)
    
    // CheckGlobalQueue determines if the global queue should be checked
    CheckGlobalQueue(pp *p) bool
}
```

### Key Methods

#### NextTask

The primary method for selecting the next goroutine to execute. This method:
- Receives the current processor (P) and machine (M) as context
- Returns the goroutine to execute, whether it should inherit time, and whether to wake another P
- May block until work becomes available

Different implementations can use various strategies:
- FIFO (First In, First Out)
- Priority-based selection
- Work stealing with different policies
- Deadline-aware scheduling

#### QueueTask

Adds a goroutine to a run queue. Parameters:
- `gp`: The goroutine to queue
- `pp`: The processor (can be nil for global queue)
- `next`: If true, try to run this goroutine immediately after the current one
- `local`: If true, add to local queue; otherwise, add to global queue

#### StealWork

Attempts to steal work from other processors for load balancing. Different implementations can:
- Use different victim selection strategies
- Adjust how much work to steal
- Implement NUMA-aware stealing

#### CheckGlobalQueue

Controls fairness by determining when to check the global queue. The current default checks every 61 scheduler ticks, but this can be adjusted:
- More frequent checks increase fairness but may reduce performance
- Less frequent checks improve local affinity but may cause starvation

## Implementation Examples

### Default Scheduler

The `schedulerImpl` type implements the current Go scheduler behavior. It serves as:
1. The baseline implementation
2. A reference for new implementations
3. A fallback for methods not overridden by custom schedulers

### Priority Scheduler

```go
type priorityScheduler struct {
    schedulerImpl
}

func (s *priorityScheduler) CheckGlobalQueue(pp *p) bool {
    // Check more frequently (every 31 ticks) for better fairness
    return pp.schedtick%31 == 0 && !sched.runq.empty()
}
```

### FIFO Scheduler

```go
type fifoScheduler struct {
    schedulerImpl
}

func (s *fifoScheduler) CheckGlobalQueue(pp *p) bool {
    // Check less frequently (every 127 ticks) for stronger local affinity
    return pp.schedtick%127 == 0 && !sched.runq.empty()
}
```

### Work Stealing Scheduler

```go
type workStealingScheduler struct {
    schedulerImpl
}

func (s *workStealingScheduler) CheckGlobalQueue(pp *p) bool {
    // Prefer work stealing over global queue
    return pp.schedtick%101 == 0 && !sched.runq.empty()
}
```

## Usage

### Basic Usage

```go
import "runtime"

func init() {
    // Switch to priority scheduler
    runtime.SetScheduler(runtime.NewPriorityScheduler())
}
```

### A/B Testing

```go
func selectScheduler(algo string) {
    switch algo {
    case "priority":
        runtime.SetScheduler(runtime.NewPriorityScheduler())
    case "fifo":
        runtime.SetScheduler(runtime.NewFIFOScheduler())
    case "workstealing":
        runtime.SetScheduler(runtime.NewWorkStealingScheduler())
    default:
        // Use default scheduler
    }
}
```

### Testing Custom Schedulers

```go
func TestCustomScheduler(t *testing.T) {
    original := runtime.GetScheduler()
    defer runtime.SetScheduler(original)
    
    // Test with custom scheduler
    runtime.SetScheduler(runtime.NewPriorityScheduler())
    
    // Run your test workload
    // ...
}
```

## Implementation Guidelines

### Thread Safety

Scheduler implementations must be thread-safe as they will be called concurrently from multiple OS threads. Use appropriate synchronization:
- Atomic operations for shared counters
- Mutexes for complex state updates
- Lock-free algorithms where possible

### Memory Management

- Avoid allocations in hot paths
- Be careful with goroutine lifecycle management
- Don't introduce memory leaks

### Integration with Runtime

Custom schedulers can access existing runtime state:
- `sched` global for scheduler state
- Per-P run queues via `pp.runq*`
- Global run queue via `sched.runq`

Always use appropriate locking when accessing shared state.

### Performance Considerations

1. **Hot Path Optimization**: `NextTask` is called frequently; optimize it carefully
2. **Fairness vs Performance**: Balance preventing starvation with maximizing throughput
3. **Work Stealing**: Tune stealing frequency and batch sizes
4. **Global Queue Checks**: Adjust check frequency based on workload characteristics

## Roadmap for Future Enhancements

### Phase 1: Interface Definition (Current)
- [x] Define Scheduler interface
- [x] Implement default scheduler wrapping current behavior
- [x] Add example alternative schedulers
- [x] Create basic tests

### Phase 2: Integration
- [ ] Integrate NextTask calls into schedule() function
- [ ] Update ready() to use QueueTask interface
- [ ] Add runtime flags to select scheduler at startup
- [ ] Performance testing and optimization

### Phase 3: Advanced Features
- [ ] Priority management API for goroutines
- [ ] Deadline-aware scheduling support
- [ ] NUMA-aware work stealing
- [ ] Real-time scheduling guarantees

### Phase 4: Observability
- [ ] Scheduler metrics and statistics
- [ ] Tracing integration for scheduler decisions
- [ ] Performance profiling tools
- [ ] Visualization of scheduling behavior

## Testing Strategy

1. **Unit Tests**: Test individual scheduler methods
2. **Integration Tests**: Test with actual goroutines
3. **Stress Tests**: High concurrency scenarios
4. **Benchmark Tests**: Performance comparisons
5. **Correctness Tests**: Verify fairness and starvation prevention

## Limitations and Caveats

1. **Runtime State**: Schedulers have deep access to runtime internals
2. **Initialization**: SetScheduler should be called before goroutines run
3. **Stability**: Custom schedulers are experimental; use with caution
4. **Performance**: Poorly designed schedulers can severely degrade performance
5. **Safety**: Bugs in schedulers can cause runtime panics or deadlocks

## References

- [Go Scheduler Design Doc](https://golang.org/s/go11sched)
- Current scheduler implementation in `runtime/proc.go`
- HTTP/2 WriteScheduler interface in `net/http/h2_bundle.go` (design inspiration)

## Contributing

When implementing custom schedulers:

1. Start by extending `schedulerImpl` and overriding specific methods
2. Add comprehensive tests
3. Benchmark against the default scheduler
4. Document the algorithm and design choices
5. Consider edge cases and failure modes

## Example: Implementing a Custom Scheduler

```go
package runtime

type myScheduler struct {
    schedulerImpl  // Embed default implementation
    // Add custom state
}

func NewMyScheduler() Scheduler {
    return &myScheduler{}
}

// Override specific methods
func (s *myScheduler) NextTask(pp *p, mp *m) (gp *g, inheritTime bool, tryWakeP bool) {
    // Custom scheduling logic
    // Can call s.schedulerImpl methods for default behavior
    return s.schedulerImpl.NextTask(pp, mp)
}

func (s *myScheduler) CheckGlobalQueue(pp *p) bool {
    // Custom fairness policy
    return pp.schedtick%my_custom_value == 0 && !sched.runq.empty()
}
```

## Conclusion

The scheduler interface provides a flexible foundation for experimenting with different scheduling algorithms in the Go runtime. By abstracting the core scheduling operations behind a well-defined interface, we enable innovation while maintaining compatibility with the existing runtime infrastructure.
