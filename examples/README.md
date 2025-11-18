# Scheduler Interface Examples

This directory contains example programs demonstrating the scheduler interface concepts.

## scheduler_demo.go

A conceptual demonstration showing how different scheduling algorithms might be characterized:

- **Default Scheduler**: Balances local affinity with global fairness (checks global queue every 61 ticks)
- **Priority Scheduler**: More frequent global queue checks (every 31 ticks) for better fairness
- **FIFO Scheduler**: Less frequent global queue checks (every 127 ticks) for stronger local affinity
- **Work-Stealing Scheduler**: Emphasizes work stealing over global queue polling

### Running the demo:

```bash
export PATH=/path/to/moriarty/bin:$PATH
go run examples/scheduler_demo.go
```

Note: The current demo uses the default scheduler for all workloads. The different timing characteristics are illustrative. To actually use different schedulers, you would need to integrate the scheduler interface into the runtime's `schedule()` function (see SCHEDULER_INTERFACE.md for the roadmap).

## Future Examples

Additional examples could include:

- Real-time scheduling with deadlines
- NUMA-aware scheduling
- Custom priority management
- Workload-specific scheduler tuning
- A/B testing framework for schedulers
