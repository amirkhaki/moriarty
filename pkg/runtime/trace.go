package runtime

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
)

// LoadTrace reads a trace from a JSON-lines file.
func LoadTrace(filename string) ([]Event, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open trace file: %w", err)
	}
	defer f.Close()

	var trace []Event
	dec := json.NewDecoder(bufio.NewReader(f))
	for dec.More() {
		var e Event
		if err := dec.Decode(&e); err != nil {
			return nil, fmt.Errorf("failed to decode event: %w", err)
		}
		trace = append(trace, e)
	}
	return trace, nil
}

// SaveTrace writes a trace to a JSON-lines file.
func SaveTrace(filename string, trace []Event) error {
	f, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create trace file: %w", err)
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	enc := json.NewEncoder(w)
	for _, e := range trace {
		if err := enc.Encode(e); err != nil {
			return fmt.Errorf("failed to encode event: %w", err)
		}
	}
	return w.Flush()
}

// groupByGoID groups events by their goroutine ID, preserving order within each group.
func groupByGoID(trace []Event) map[uint64][]Event {
	grouped := make(map[uint64][]Event)
	for _, e := range trace {
		grouped[e.GoID] = append(grouped[e.GoID], e)
	}
	return grouped
}
