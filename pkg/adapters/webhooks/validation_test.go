package webhooks

import (
	"testing"

	core "configpropagation/pkg/core"
)

func TestValidateConfigPropagationPassesByDefault(t *testing.T) {
	spec := &core.ConfigPropagationSpec{
		SourceRef:           core.ObjectRef{Namespace: "src", Name: "cfg"},
		NamespaceSelector:   &core.LabelSelector{MatchLabels: map[string]string{"team": "a"}},
		Strategy:            &core.UpdateStrategy{Type: core.StrategyRolling},
		ConflictPolicy:      core.ConflictOverwrite,
		ResyncPeriodSeconds: int32Ptr(30),
	}

	if err := ValidateConfigPropagation(spec, nil); err != nil {
		t.Fatalf("expected validation to pass, got %v", err)
	}
}

func TestValidateConfigPropagationRejectsInvalidStrategy(t *testing.T) {
	zero := int32(0)
	spec := &core.ConfigPropagationSpec{
		SourceRef:         core.ObjectRef{Namespace: "src", Name: "cfg"},
		NamespaceSelector: &core.LabelSelector{MatchLabels: map[string]string{"team": "a"}},
		Strategy:          &core.UpdateStrategy{Type: "canary", BatchSize: &zero},
	}

	if err := ValidateConfigPropagation(spec, nil); err == nil {
		t.Fatalf("expected validation error for invalid strategy")
	}
}

func TestValidateConfigPropagationStrictSelectorGuard(t *testing.T) {
	t.Setenv(strictSelectorEnv, "true")
	spec := &core.ConfigPropagationSpec{
		SourceRef:         core.ObjectRef{Namespace: "src", Name: "cfg"},
		NamespaceSelector: &core.LabelSelector{},
	}

	if err := ValidateConfigPropagation(spec, nil); err == nil {
		t.Fatalf("expected guardrail to reject wide-open selector")
	}

	spec.NamespaceSelector.MatchLabels = map[string]string{"team": "a"}
	if err := ValidateConfigPropagation(spec, nil); err != nil {
		t.Fatalf("expected selector with label to pass, got %v", err)
	}
}

func TestValidateConfigPropagationSourceImmutability(t *testing.T) {
	t.Setenv(immutableSourceEnv, "1")
	oldSpec := &core.ConfigPropagationSpec{
		SourceRef:         core.ObjectRef{Namespace: "src", Name: "cfg"},
		NamespaceSelector: &core.LabelSelector{MatchLabels: map[string]string{"team": "a"}},
	}
	newSpec := &core.ConfigPropagationSpec{
		SourceRef:         core.ObjectRef{Namespace: "src", Name: "cfg2"},
		NamespaceSelector: &core.LabelSelector{MatchLabels: map[string]string{"team": "a"}},
	}

	if err := ValidateConfigPropagation(newSpec, oldSpec); err == nil {
		t.Fatalf("expected immutability guard to reject change")
	}

	newSpec.SourceRef.Name = "cfg"
	if err := ValidateConfigPropagation(newSpec, oldSpec); err != nil {
		t.Fatalf("expected immutability guard to allow unchanged source, got %v", err)
	}
}

func int32Ptr(v int32) *int32 {
	return &v
}
