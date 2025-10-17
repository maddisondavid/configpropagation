package webhooks

import (
	"testing"

	core "configpropagation/pkg/core"
)

func TestDefaultConfigPropagation(t *testing.T) {
	spec := &core.ConfigPropagationSpec{
		SourceRef:         core.ObjectRef{Namespace: "src", Name: "cfg"},
		NamespaceSelector: &core.LabelSelector{MatchLabels: map[string]string{"team": "a"}},
	}

	DefaultConfigPropagation(spec)

	if spec.Strategy == nil || spec.Strategy.Type != core.StrategyRolling {
		t.Fatalf("expected strategy default to rolling, got %+v", spec.Strategy)
	}
	if spec.Strategy.BatchSize == nil || *spec.Strategy.BatchSize != 5 {
		t.Fatalf("expected batchSize default 5, got %+v", spec.Strategy.BatchSize)
	}
	if spec.ConflictPolicy != core.ConflictOverwrite {
		t.Fatalf("expected conflictPolicy default overwrite, got %s", spec.ConflictPolicy)
	}
	if spec.Prune == nil || *spec.Prune != true {
		t.Fatalf("expected prune default true")
	}
}

func TestDefaultConfigPropagationRespectsExistingValues(t *testing.T) {
	prune := false
	batch := int32(7)
	spec := &core.ConfigPropagationSpec{
		SourceRef:         core.ObjectRef{Namespace: "src", Name: "cfg"},
		NamespaceSelector: &core.LabelSelector{MatchLabels: map[string]string{"team": "a"}},
		Strategy:          &core.UpdateStrategy{Type: core.StrategyImmediate, BatchSize: &batch},
		ConflictPolicy:    core.ConflictSkip,
		Prune:             &prune,
	}

	DefaultConfigPropagation(spec)

	if spec.Strategy.Type != core.StrategyImmediate {
		t.Fatalf("defaulting overwrote strategy type")
	}
	if spec.Strategy.BatchSize == nil || *spec.Strategy.BatchSize != 7 {
		t.Fatalf("defaulting overwrote strategy batch size")
	}
	if spec.ConflictPolicy != core.ConflictSkip {
		t.Fatalf("defaulting overwrote conflict policy")
	}
	if spec.Prune == nil || *spec.Prune != false {
		t.Fatalf("defaulting overwrote prune")
	}
}
