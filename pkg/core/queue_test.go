package core_test

import (
	core "configpropagation/pkg/core"
	"testing"
)

func TestWorkQueueFIFOAndDedupe(t *testing.T) {
	q := core.NewWorkQueue[string]()
	if q.Len() != 0 {
		t.Fatalf("expected len 0, got %d", q.Len())
	}
	q.Add("a")
	q.Add("b")
	q.Add("a") // duplicate should be ignored
	if q.Len() != 2 {
		t.Fatalf("expected len 2, got %d", q.Len())
	}
	if v, ok := q.Get(); !ok || v != "a" {
		t.Fatalf("expected first 'a', got %v %v", v, ok)
	}
	if v, ok := q.Get(); !ok || v != "b" {
		t.Fatalf("expected second 'b', got %v %v", v, ok)
	}
	if _, ok := q.Get(); ok {
		t.Fatalf("expected empty queue")
	}
}
