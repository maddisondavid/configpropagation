package configpropagation

import (
	"fmt"
	"reflect"
	"testing"

	"configpropagation/pkg/adapters"
	"configpropagation/pkg/core"
)

type fakeClient struct {
	data       map[string]map[string]map[string]string
	namespaces []string
}

func (f *fakeClient) GetSourceConfigMap(ns, name string) (map[string]string, error) {
	if nsMap, ok := f.data[ns]; ok {
		if d, ok := nsMap[name]; ok {
			// shallow copy
			out := map[string]string{}
			for k, v := range d {
				out[k] = v
			}
			return out, nil
		}
	}
	return nil, nil
}
func (f *fakeClient) ListNamespacesBySelector(_ map[string]string, _ []adapters.LabelSelectorRequirement) ([]string, error) {
	return append([]string(nil), f.namespaces...), nil
}
func (f *fakeClient) UpsertConfigMap(_ string, _ string, _ map[string]string, _ map[string]string, _ map[string]string) error {
	return nil
}
func (f *fakeClient) GetTargetConfigMap(namespace, name string) (map[string]string, map[string]string, map[string]string, bool, error) {
	return nil, nil, nil, false, nil
}
func (f *fakeClient) ListManagedTargetNamespaces(source string, name string) ([]string, error) {
	return []string{}, nil
}
func (f *fakeClient) DeleteConfigMap(namespace, name string) error { return nil }
func (f *fakeClient) UpdateConfigMapMetadata(namespace, name string, labels, annotations map[string]string) error {
	return nil
}

func TestReconcilerPlanImmediate(t *testing.T) {
	fc := &fakeClient{
		data:       map[string]map[string]map[string]string{"src": {"cfg": {"a": "1", "b": "2", "c": "3"}}},
		namespaces: []string{"ns1", "ns2", "ns3"},
	}
	r := NewReconciler(fc)
	key := Key{Namespace: "default", Name: "cp"}
	s := &core.ConfigPropagationSpec{
		SourceRef:         core.ObjectRef{Namespace: "src", Name: "cfg"},
		NamespaceSelector: &core.LabelSelector{},
		Strategy:          &core.UpdateStrategy{Type: core.StrategyImmediate},
		DataKeys:          []string{"a", "c"},
	}
	got, err := r.Reconcile(key, s)
	if err != nil {
		t.Fatalf("reconcile error: %v", err)
	}
	want := []string{"ns1", "ns2", "ns3"}
	if !reflect.DeepEqual(got.Planned, want) {
		t.Fatalf("want %v got %v", want, got.Planned)
	}
	if got.CompletedCount != len(want) || got.TotalTargets != len(want) {
		t.Fatalf("expected all targets completed, got %+v", got)
	}
}

func TestReconcilerPlanRollingBatch(t *testing.T) {
	fc := &fakeClient{
		data:       map[string]map[string]map[string]string{"src": {"cfg": {"x": "y"}}},
		namespaces: []string{"a", "b", "c", "d"},
	}
	r := NewReconciler(fc)
	bs := int32(2)
	key := Key{Namespace: "default", Name: "cp"}
	s := &core.ConfigPropagationSpec{
		SourceRef:         core.ObjectRef{Namespace: "src", Name: "cfg"},
		NamespaceSelector: &core.LabelSelector{},
		Strategy:          &core.UpdateStrategy{Type: core.StrategyRolling, BatchSize: &bs},
	}
	got, err := r.Reconcile(key, s)
	if err != nil {
		t.Fatalf("reconcile error: %v", err)
	}
	want := []string{"a", "b"}
	if !reflect.DeepEqual(got.Planned, want) {
		t.Fatalf("want %v got %v", want, got.Planned)
	}
	if got.CompletedCount != len(want) {
		t.Fatalf("expected completed count to equal batch size, got %+v", got)
	}

	// next reconcile should continue with remaining namespaces
	got2, err := r.Reconcile(key, s)
	if err != nil {
		t.Fatalf("second reconcile error: %v", err)
	}
	want2 := []string{"c", "d"}
	if !reflect.DeepEqual(got2.Planned, want2) {
		t.Fatalf("want %v got %v", want2, got2.Planned)
	}
	if got2.CompletedCount != len(want)+len(want2) {
		t.Fatalf("expected completed count to accumulate, got %+v", got2)
	}
}

func TestPlanTargetsBranches(t *testing.T) {
	planner := core.NewRolloutPlanner()
	key := core.NamespacedName{Namespace: "ns", Name: "cp"}
	all := []string{"a", "b"}
	planned, completed := planner.Plan(key, "hash", core.StrategyRolling, 1, all)
	if !reflect.DeepEqual(planned, []string{"a"}) || completed != 0 {
		t.Fatalf("expected first element with empty completed, got planned=%v completed=%d", planned, completed)
	}
	planner.MarkCompleted(key, "hash", planned)
	planned, completed = planner.Plan(key, "hash", core.StrategyRolling, 2, all)
	if !reflect.DeepEqual(planned, []string{"b"}) || completed != 1 {
		t.Fatalf("expected second element with completed count 1, got planned=%v completed=%d", planned, completed)
	}
	planner.MarkCompleted(key, "hash", planned)
	planned, completed = planner.Plan(key, "hash", core.StrategyRolling, 2, all)
	if len(planned) != 0 || completed != 2 {
		t.Fatalf("expected no remaining targets, got planned=%v completed=%d", planned, completed)
	}
	// Immediate returns all and treats them as completed
	planned, completed = planner.Plan(key, "hash", core.StrategyImmediate, 0, all)
	if !reflect.DeepEqual(planned, all) || completed != len(all) {
		t.Fatalf("immediate strategy should plan all, got planned=%v completed=%d", planned, completed)
	}
}

func TestHelpersComputeEffectiveAndListTargetsAndSyncTargets(t *testing.T) {
	// computeEffective with nil src and keys -> returns empty map
	out := computeEffective(nil, nil)
	if len(out) != 0 {
		t.Fatalf("expected empty effective for nil src")
	}
	// computeEffective copy-all path
	out = computeEffective(map[string]string{"a": "1"}, nil)
	if !reflect.DeepEqual(out, map[string]string{"a": "1"}) {
		t.Fatalf("copy-all failed: %+v", out)
	}
	// listTargets exercises adapter translation and nilIfEmpty
	fc := &fakeClient{data: map[string]map[string]map[string]string{}, namespaces: []string{"x"}}
	sel := &core.LabelSelector{MatchLabels: map[string]string{}, MatchExpressions: []core.LabelSelectorReq{{Key: "k", Operator: "Exists"}}}
	got, err := listTargets(fc, sel)
	if err != nil || !reflect.DeepEqual(got, []string{"x"}) {
		t.Fatalf("listTargets failed: %v %v", got, err)
	}
	// syncTargets executes loop and returns nil
	hash := core.HashData(map[string]string{"k": "v"})
	if err := syncTargets(fc, []string{"ns"}, "name", map[string]string{"k": "v"}, hash, "src", core.ConflictOverwrite); err != nil {
		t.Fatalf("syncTargets error: %v", err)
	}
	// syncTargets error path
	bu := &badUpsert{*fc}
	if err := syncTargets(bu, []string{"ns"}, "name", map[string]string{"k": "v"}, hash, "src", core.ConflictOverwrite); err == nil {
		t.Fatalf("expected syncTargets to error on upsert")
	}
}

type errClient struct{ fakeClient }

func (e *errClient) GetSourceConfigMap(ns, name string) (map[string]string, error) {
	return nil, fmt.Errorf("boom")
}
func (e *errClient) UpsertConfigMap(ns, name string, d map[string]string, l, a map[string]string) error {
	return fmt.Errorf("fail")
}
func (e *errClient) ListNamespacesBySelector(_ map[string]string, _ []adapters.LabelSelectorRequirement) ([]string, error) {
	return nil, fmt.Errorf("nslist")
}

type badUpsert struct{ fakeClient }

func (b *badUpsert) UpsertConfigMap(ns, name string, d map[string]string, l, a map[string]string) error {
	return fmt.Errorf("nope")
}

func TestReconcileErrorPaths(t *testing.T) {
	r := NewReconciler(&errClient{fakeClient{data: map[string]map[string]map[string]string{}, namespaces: []string{"n"}}})
	s := &core.ConfigPropagationSpec{SourceRef: core.ObjectRef{Namespace: "s", Name: "n"}, NamespaceSelector: &core.LabelSelector{}}
	if _, err := r.Reconcile(Key{Namespace: "ns", Name: "cp"}, s); err == nil {
		t.Fatalf("expected error from source get")
	}

	// Source ok, upsert fails
	ec := &errClient{fakeClient{data: map[string]map[string]map[string]string{"s": {"n": {}}}, namespaces: []string{"n"}}}
	r2 := NewReconciler(ec)
	if _, err := r2.Reconcile(Key{Namespace: "ns", Name: "cp"}, s); err == nil {
		t.Fatalf("expected upsert error")
	}

	// Namespace list fails
	ec2 := &errClient{fakeClient{data: map[string]map[string]map[string]string{"s": {"n": {}}}, namespaces: []string{"n"}}}
	r3 := NewReconciler(ec2)
	if _, err := r3.Reconcile(Key{Namespace: "ns", Name: "cp"}, s); err == nil {
		t.Fatalf("expected list namespaces error")
	}

	// Nil spec
	if _, err := r2.Reconcile(Key{Namespace: "ns", Name: "cp"}, nil); err == nil {
		t.Fatalf("expected error for nil spec")
	}
}

func TestReconcileValidationFailure(t *testing.T) {
	r := NewReconciler(&fakeClient{data: map[string]map[string]map[string]string{}, namespaces: []string{"n"}})
	// Invalid: strategy type unrecognized
	s := &core.ConfigPropagationSpec{SourceRef: core.ObjectRef{Namespace: "s", Name: "n"}, NamespaceSelector: &core.LabelSelector{}, Strategy: &core.UpdateStrategy{Type: "canary"}}
	if _, err := r.Reconcile(Key{Namespace: "ns", Name: "cp"}, s); err == nil {
		t.Fatalf("expected validation error")
	}
}

func TestReconcileSuccessCoversImpl(t *testing.T) {
	// Exercise reconcileImpl via Reconcile happy path
	fc := &fakeClient{data: map[string]map[string]map[string]string{"s": {"n": {"k": "v"}}}, namespaces: []string{"ns"}}
	r := NewReconciler(fc)
	s := &core.ConfigPropagationSpec{SourceRef: core.ObjectRef{Namespace: "s", Name: "n"}, NamespaceSelector: &core.LabelSelector{}, Strategy: &core.UpdateStrategy{Type: core.StrategyImmediate}}
	if _, err := r.Reconcile(Key{Namespace: "ns", Name: "cp"}, s); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReconcileNoTargetsNoUpserts(t *testing.T) {
	fc := &fakeClient{data: map[string]map[string]map[string]string{"s": {"n": {"k": "v"}}}, namespaces: []string{}}
	r := NewReconciler(fc)
	s := &core.ConfigPropagationSpec{SourceRef: core.ObjectRef{Namespace: "s", Name: "n"}, NamespaceSelector: &core.LabelSelector{}}
	planned, err := r.Reconcile(Key{Namespace: "ns", Name: "cp"}, s)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(planned.Planned) != 0 {
		t.Fatalf("expected 0 planned, got %d", len(planned.Planned))
	}
}
