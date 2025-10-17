package core_test

import (
	core "configpropagation/src/core"
	"testing"
)

func TestHashDataDeterministicAndOrderIndependent(t *testing.T) {
	m1 := map[string]string{"a": "1", "b": "2", "c": "3"}
	m2 := map[string]string{"c": "3", "b": "2", "a": "1"}
	h1 := core.HashData(m1)
	h2 := core.HashData(m2)
	if h1 == "" {
		t.Fatalf("hash should not be empty for non-empty data")
	}
	if h1 != h2 {
		t.Fatalf("hash must be order independent: %s vs %s", h1, h2)
	}
}

func TestHashDataEmpty(t *testing.T) {
	if got := core.HashData(nil); got != "" {
		t.Fatalf("expected empty hash for nil, got %q", got)
	}
	if got := core.HashData(map[string]string{}); got != "" {
		t.Fatalf("expected empty hash for empty, got %q", got)
	}
}
