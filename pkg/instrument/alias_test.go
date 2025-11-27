package instrument_test

import (
	"testing"

	"github.com/amirkhaki/moriarty/pkg/instrument"
)

func TestDeterministicAliasGeneration(t *testing.T) {
	// Test that the same import path always generates the same alias
	config1 := instrument.DefaultConfig()
	config2 := instrument.DefaultConfig()

	instr1 := instrument.NewInstrumenter(config1)
	instr2 := instrument.NewInstrumenter(config2)

	if config1.RuntimeAlias != config2.RuntimeAlias {
		t.Errorf("Expected same alias for same import path, got %s and %s",
			config1.RuntimeAlias, config2.RuntimeAlias)
	}

	// Verify it starts with __moriarty_
	if len(config1.RuntimeAlias) < 11 || config1.RuntimeAlias[:11] != "__moriarty_" {
		t.Errorf("Expected alias to start with __moriarty_, got %s", config1.RuntimeAlias)
	}

	// Verify it's the right length (__moriarty_ + 16 hex chars = 27 chars)
	if len(config1.RuntimeAlias) != 27 {
		t.Errorf("Expected alias length of 27, got %d (%s)",
			len(config1.RuntimeAlias), config1.RuntimeAlias)
	}

	_ = instr1
	_ = instr2
}

func TestCustomRuntimeAlias(t *testing.T) {
	config := &instrument.Config{
		BaseRuntimeAddress: "custom/runtime",
		RuntimeAlias:       "myCustomAlias",
		MemReadFunc:        "Read",
		MemWriteFunc:       "Write",
		ImportRewrites:     map[string]string{},
	}

	instr := instrument.NewInstrumenter(config)

	// Should preserve custom alias
	if config.RuntimeAlias != "myCustomAlias" {
		t.Errorf("Expected custom alias to be preserved, got %s", config.RuntimeAlias)
	}

	_ = instr
}
