package core_test

import (
	core "configpropagation/pkg/core"
	"testing"
)

func TestWorkQueueFIFOAndDedupe(t *testing.T) {
	queue := core.NewWorkQueue[string]()

	if queue.Len() != 0 {
		t.Fatalf("expected len 0, got %d", queue.Len())
	}

	queue.Add("a")
	queue.Add("b")
	queue.Add("a") // duplicate should be ignored

	if queue.Len() != 2 {
		t.Fatalf("expected len 2, got %d", queue.Len())
	}

	if value, exists := queue.Get(); !exists || value != "a" {
		t.Fatalf("expected first 'a', got %v %v", value, exists)
	}

	if value, exists := queue.Get(); !exists || value != "b" {
		t.Fatalf("expected second 'b', got %v %v", value, exists)
	}

	if _, exists := queue.Get(); exists {
		t.Fatalf("expected empty queue")
	}
}
