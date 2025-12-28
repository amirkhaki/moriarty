package runtime

// Kind represents the type of event
type Kind uint8

const (
	KindRead Kind = iota + 1
	KindWrite
	KindSpawn
	KindGoEnter
	KindGoExit
)

func (k Kind) String() string {
	switch k {
	case KindRead:
		return "read"
	case KindWrite:
		return "write"
	case KindSpawn:
		return "spawn"
	case KindGoEnter:
		return "enter"
	case KindGoExit:
		return "exit"
	default:
		return "unknown"
	}
}

// Event represents a single traced event
type Event struct {
	GoID uint64  `json:"goid"`
	Kind Kind    `json:"kind"`
	Addr uintptr `json:"addr,omitempty"` // Memory address for read/write events
}
