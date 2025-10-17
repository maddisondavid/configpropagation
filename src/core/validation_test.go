package core_test

import (
	core "configpropagation/src/core"
	"testing"
)

func TestDefaultAndValidateSpec(t *testing.T) {
	s := &core.ConfigPropagationSpec{
		SourceRef:         core.ObjectRef{Namespace: "ns", Name: "cfg"},
		NamespaceSelector: &core.LabelSelector{MatchLabels: map[string]string{"team": "a"}},
	}
	core.DefaultSpec(s)
	if s.Strategy == nil || s.Strategy.Type != core.StrategyRolling {
		t.Fatalf("expected default strategy rolling, got %+v", s.Strategy)
	}
	if s.Strategy.BatchSize == nil || *s.Strategy.BatchSize != 5 {
		t.Fatalf("expected default batchSize 5, got %+v", s.Strategy.BatchSize)
	}
	if s.ConflictPolicy != core.ConflictOverwrite {
		t.Fatalf("expected default conflictPolicy overwrite, got %s", s.ConflictPolicy)
	}
	if s.Prune == nil || *s.Prune != true {
		t.Fatalf("expected default prune true")
	}
	if err := core.ValidateSpec(s); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestDefaultSpecBatchSizeFromEnv(t *testing.T) {
	s := &core.ConfigPropagationSpec{
		SourceRef:         core.ObjectRef{Namespace: "ns", Name: "cfg"},
		NamespaceSelector: &core.LabelSelector{MatchLabels: map[string]string{"team": "a"}},
	}
	t.Setenv("BATCH_SIZE", "10")
	core.DefaultSpec(s)
	if s.Strategy.BatchSize == nil || *s.Strategy.BatchSize != 10 {
		t.Fatalf("expected batchSize 10 from env, got %+v", s.Strategy.BatchSize)
	}

	t.Setenv("BATCH_SIZE", "0")
	s2 := &core.ConfigPropagationSpec{
		SourceRef:         core.ObjectRef{Namespace: "ns", Name: "cfg"},
		NamespaceSelector: &core.LabelSelector{MatchLabels: map[string]string{"team": "a"}},
	}
	core.DefaultSpec(s2)
	if s2.Strategy.BatchSize == nil || *s2.Strategy.BatchSize != 5 {
		t.Fatalf("invalid env should fall back to default 5, got %+v", s2.Strategy.BatchSize)
	}

	t.Setenv("BATCH_SIZE", "not-a-number")
	s3 := &core.ConfigPropagationSpec{
		SourceRef:         core.ObjectRef{Namespace: "ns", Name: "cfg"},
		NamespaceSelector: &core.LabelSelector{MatchLabels: map[string]string{"team": "a"}},
	}
	core.DefaultSpec(s3)
	if s3.Strategy.BatchSize == nil || *s3.Strategy.BatchSize != 5 {
		t.Fatalf("non-numeric env should fall back to default 5, got %+v", s3.Strategy.BatchSize)
	}
}

func TestValidateSpecFailures(t *testing.T) {
	// Missing source and selector should fail
	s1 := &core.ConfigPropagationSpec{}
	if err := core.ValidateSpec(s1); err == nil {
		t.Fatalf("expected error for missing sourceRef and selector")
	}

	// Invalid strategy type, batch size, conflict policy, and resync
	zero := int32(0)
	s2 := &core.ConfigPropagationSpec{
		SourceRef:           core.ObjectRef{Namespace: "ns", Name: "cfg"},
		NamespaceSelector:   &core.LabelSelector{},
		Strategy:            &core.UpdateStrategy{Type: "canary", BatchSize: &zero},
		ConflictPolicy:      "reject",
		ResyncPeriodSeconds: &zero,
	}
	if err := core.ValidateSpec(s2); err == nil {
		t.Fatalf("expected validation errors for invalid fields")
	}
}

func TestValidateSpecInvalidBatchOnly(t *testing.T) {
	zero := int32(0)
	s := &core.ConfigPropagationSpec{
		SourceRef:         core.ObjectRef{Namespace: "ns", Name: "cfg"},
		NamespaceSelector: &core.LabelSelector{},
		Strategy:          &core.UpdateStrategy{Type: core.StrategyRolling, BatchSize: &zero},
	}
	if err := core.ValidateSpec(s); err == nil {
		t.Fatalf("expected error for batchSize < 1")
	}
}

func TestValidateSpecInvalidConflictOnly(t *testing.T) {
	s := &core.ConfigPropagationSpec{
		SourceRef:         core.ObjectRef{Namespace: "ns", Name: "cfg"},
		NamespaceSelector: &core.LabelSelector{},
		ConflictPolicy:    "reject",
	}
	if err := core.ValidateSpec(s); err == nil {
		t.Fatalf("expected error for invalid conflictPolicy")
	}
}

func TestValidateSpecInvalidResyncOnly(t *testing.T) {
	nine := int32(9)
	s := &core.ConfigPropagationSpec{
		SourceRef:           core.ObjectRef{Namespace: "ns", Name: "cfg"},
		NamespaceSelector:   &core.LabelSelector{},
		ResyncPeriodSeconds: &nine,
	}
	if err := core.ValidateSpec(s); err == nil {
		t.Fatalf("expected error for resyncPeriodSeconds < 10")
	}
}

func TestValidateSpecNilAndMissingSelector(t *testing.T) {
	if err := core.ValidateSpec(nil); err == nil {
		t.Fatalf("expected error for nil spec")
	}
	s := &core.ConfigPropagationSpec{SourceRef: core.ObjectRef{Namespace: "ns", Name: "cfg"}}
	if err := core.ValidateSpec(s); err == nil {
		t.Fatalf("expected error when namespaceSelector is missing")
	}
}

func TestValidateSpecMissingNameOnly(t *testing.T) {
	s := &core.ConfigPropagationSpec{SourceRef: core.ObjectRef{Namespace: "ns", Name: ""}, NamespaceSelector: &core.LabelSelector{}}
	if err := core.ValidateSpec(s); err == nil {
		t.Fatalf("expected error when sourceRef.name is empty")
	}
}

func TestValidateSpecValidStrategyAndBatch(t *testing.T) {
	one := int32(1)
	s := &core.ConfigPropagationSpec{
		SourceRef:         core.ObjectRef{Namespace: "ns", Name: "cfg"},
		NamespaceSelector: &core.LabelSelector{},
		Strategy:          &core.UpdateStrategy{Type: core.StrategyImmediate, BatchSize: &one},
		// ConflictPolicy empty (allowed), Resync nil (allowed)
	}
	if err := core.ValidateSpec(s); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestValidateSpecSuccessVariants(t *testing.T) {
	bs := int32(10)
	rs := int32(15)
	prune := false
	s := &core.ConfigPropagationSpec{
		SourceRef:           core.ObjectRef{Namespace: "ns", Name: "cfg"},
		NamespaceSelector:   &core.LabelSelector{},
		Strategy:            &core.UpdateStrategy{Type: core.StrategyImmediate, BatchSize: &bs},
		ConflictPolicy:      core.ConflictSkip,
		Prune:               &prune,
		ResyncPeriodSeconds: &rs,
	}
	if err := core.ValidateSpec(s); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestDefaultSpecDoesNotOverrideSetFields(t *testing.T) {
	bs := int32(2)
	prune := false
	s := &core.ConfigPropagationSpec{
		SourceRef:         core.ObjectRef{Namespace: "ns", Name: "cfg"},
		NamespaceSelector: &core.LabelSelector{MatchLabels: map[string]string{"team": "a"}},
		Strategy:          &core.UpdateStrategy{Type: core.StrategyImmediate, BatchSize: &bs},
		ConflictPolicy:    core.ConflictSkip,
		Prune:             &prune,
	}
	core.DefaultSpec(s)
	if s.Strategy.Type != core.StrategyImmediate {
		t.Fatalf("defaulting overrode strategy.type")
	}
	if s.Strategy.BatchSize == nil || *s.Strategy.BatchSize != 2 {
		t.Fatalf("defaulting overrode strategy.batchSize")
	}
	if s.ConflictPolicy != core.ConflictSkip {
		t.Fatalf("defaulting overrode conflictPolicy")
	}
	if s.Prune == nil || *s.Prune != false {
		t.Fatalf("defaulting overrode prune")
	}
}

func TestValidateSpecPassBranches(t *testing.T) {
	// Strategy present but empty type and nil batchSize; resync at boundary; valid conflict policy
	ten := int32(10)
	s := &core.ConfigPropagationSpec{
		SourceRef:           core.ObjectRef{Namespace: "ns", Name: "cfg"},
		NamespaceSelector:   &core.LabelSelector{},
		Strategy:            &core.UpdateStrategy{},
		ConflictPolicy:      core.ConflictOverwrite,
		ResyncPeriodSeconds: &ten,
	}
	if err := core.ValidateSpec(s); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestValidateSpecWithNilStrategyPasses(t *testing.T) {
	s := &core.ConfigPropagationSpec{
		SourceRef:         core.ObjectRef{Namespace: "ns", Name: "cfg"},
		NamespaceSelector: &core.LabelSelector{},
		// Strategy is nil on purpose
	}
	if err := core.ValidateSpec(s); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}
