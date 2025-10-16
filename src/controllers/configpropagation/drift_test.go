package configpropagation

import (
	"codex/src/adapters"
	core "codex/src/core"
	"testing"
)

// fakeDriftClient simulates existing targets with varying annotations/labels.
type fakeDriftClient struct {
	src map[string]map[string]map[string]string
	ns  []string
	// target annotations/labels pre-existing
	tgtAnn  map[string]string
	tgtLbl  map[string]string
	upserts int
}

func (f *fakeDriftClient) GetSourceConfigMap(ns, name string) (map[string]string, error) {
	if m, ok := f.src[ns]; ok {
		if d, ok := m[name]; ok {
			return d, nil
		}
	}
	return nil, nil
}
func (f *fakeDriftClient) ListNamespacesBySelector(_ map[string]string, _ []adapters.LabelSelectorRequirement) ([]string, error) {
	return f.ns, nil
}

func (f *fakeDriftClient) UpsertConfigMap(ns, name string, data map[string]string, labels, annotations map[string]string) error {
	f.upserts++
	return nil
}
func (f *fakeDriftClient) GetTargetConfigMap(namespace, name string) (map[string]string, map[string]string, map[string]string, bool, error) {
	return nil, f.tgtLbl, f.tgtAnn, true, nil
}
func (f *fakeDriftClient) ListManagedTargetNamespaces(source string, name string) ([]string, error) {
	return []string{"a"}, nil
}
func (f *fakeDriftClient) DeleteConfigMap(namespace, name string) error { return nil }
func (f *fakeDriftClient) UpdateConfigMapMetadata(namespace, name string, labels, annotations map[string]string) error {
	return nil
}

func TestDriftOverwriteUpdates(t *testing.T) {
	// Source hash will be for {k:v}
	f := &fakeDriftClient{src: map[string]map[string]map[string]string{"s": {"n": {"k": "v"}}}, ns: []string{"a"}, tgtAnn: map[string]string{core.HashAnnotation: "different"}, tgtLbl: map[string]string{core.ManagedLabel: "true"}}
	r := NewReconciler(f)
	s := &core.ConfigPropagationSpec{SourceRef: core.ObjectRef{Namespace: "s", Name: "n"}, NamespaceSelector: &core.LabelSelector{}, ConflictPolicy: core.ConflictOverwrite, Strategy: &core.UpdateStrategy{Type: core.StrategyImmediate}}
	if _, err := r.Reconcile(s); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.upserts != 1 {
		t.Fatalf("expected one upsert, got %d", f.upserts)
	}
}

func TestDriftSkipDoesNotUpdate(t *testing.T) {
	f := &fakeDriftClient{src: map[string]map[string]map[string]string{"s": {"n": {"k": "v"}}}, ns: []string{"a"}, tgtAnn: map[string]string{core.HashAnnotation: "different"}, tgtLbl: map[string]string{core.ManagedLabel: "true"}}
	r := NewReconciler(f)
	s := &core.ConfigPropagationSpec{SourceRef: core.ObjectRef{Namespace: "s", Name: "n"}, NamespaceSelector: &core.LabelSelector{}, ConflictPolicy: core.ConflictSkip, Strategy: &core.UpdateStrategy{Type: core.StrategyImmediate}}
	if _, err := r.Reconcile(s); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.upserts != 0 {
		t.Fatalf("expected no upserts for skip, got %d", f.upserts)
	}
}

func TestNoOpWhenHashesMatch(t *testing.T) {
	// Compute the same hash by using same data and setting target hash afterwards via syncTargets path.
	f := &fakeDriftClient{src: map[string]map[string]map[string]string{"s": {"n": {"k": "v"}}}, ns: []string{"a"}, tgtAnn: map[string]string{}, tgtLbl: map[string]string{core.ManagedLabel: "true"}}
	r := NewReconciler(f)
	s := &core.ConfigPropagationSpec{SourceRef: core.ObjectRef{Namespace: "s", Name: "n"}, NamespaceSelector: &core.LabelSelector{}, ConflictPolicy: core.ConflictOverwrite, Strategy: &core.UpdateStrategy{Type: core.StrategyImmediate}}
	// First reconcile writes and sets hash
	if _, err := r.Reconcile(s); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.upserts != 1 {
		t.Fatalf("expected initial upsert, got %d", f.upserts)
	}
	// Simulate target now having matching hash by reusing same fake that returns found with same annotations set by previous call
	// We approximate by setting tgtAnn to the source hash using core.HashData
	f.tgtAnn[core.HashAnnotation] = core.HashData(map[string]string{"k": "v"})
	if _, err := r.Reconcile(s); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.upserts != 1 {
		t.Fatalf("expected no additional upsert when hashes match, got %d", f.upserts)
	}
}

func TestNonManagedTargetIsNotMutated(t *testing.T) {
	f := &fakeDriftClient{src: map[string]map[string]map[string]string{"s": {"n": {"k": "v"}}}, ns: []string{"a"}, tgtAnn: map[string]string{"some": "annotation"}, tgtLbl: map[string]string{}}
	r := NewReconciler(f)
	s := &core.ConfigPropagationSpec{SourceRef: core.ObjectRef{Namespace: "s", Name: "n"}, NamespaceSelector: &core.LabelSelector{}, ConflictPolicy: core.ConflictOverwrite, Strategy: &core.UpdateStrategy{Type: core.StrategyImmediate}}
	if _, err := r.Reconcile(s); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.upserts != 0 {
		t.Fatalf("expected no upsert on non-managed target, got %d", f.upserts)
	}
}
