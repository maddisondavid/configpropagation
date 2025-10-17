package core_test

import (
	core "configpropagation/pkg/core"
	"testing"
)

func TestHashDataDeterministicAndOrderIndependent(t *testing.T) {
	firstDataSet := map[string]string{"a": "1", "b": "2", "c": "3"}
	secondDataSet := map[string]string{"c": "3", "b": "2", "a": "1"}

	firstHash := core.HashData(firstDataSet)
	secondHash := core.HashData(secondDataSet)

	if firstHash == "" {
		t.Fatalf("hash should not be empty for non-empty data")
	}

	if firstHash != secondHash {
		t.Fatalf("hash must be order independent: %s vs %s", firstHash, secondHash)
	}
}

func TestHashDataEmpty(t *testing.T) {
	if hashValue := core.HashData(nil); hashValue != "" {
		t.Fatalf("expected empty hash for nil, got %q", hashValue)
	}

	if hashValue := core.HashData(map[string]string{}); hashValue != "" {
		t.Fatalf("expected empty hash for empty, got %q", hashValue)
	}
}
