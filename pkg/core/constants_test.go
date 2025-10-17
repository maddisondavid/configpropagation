package core_test

import (
	core "configpropagation/pkg/core"
	"testing"
)

func TestConstantsStability(t *testing.T) {
	if core.ManagedLabel != "configpropagator.platform.example.com/managed" {
		t.Fatalf("ManagedLabel changed: %s", core.ManagedLabel)
	}
	if core.SourceAnnotation != "configpropagator.platform.example.com/source" {
		t.Fatalf("SourceAnnotation changed: %s", core.SourceAnnotation)
	}
	if core.HashAnnotation != "configpropagator.platform.example.com/hash" {
		t.Fatalf("HashAnnotation changed: %s", core.HashAnnotation)
	}
	if core.Finalizer != "configpropagator.platform.example.com/finalizer" {
		t.Fatalf("Finalizer changed: %s", core.Finalizer)
	}
}
