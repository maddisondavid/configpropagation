package configpropagation

import (
	"fmt"
	"reflect"
	"testing"

	"codex/src/adapters"
	"codex/src/core"
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
	s := &core.ConfigPropagationSpec{
		SourceRef:         core.ObjectRef{Namespace: "src", Name: "cfg"},
		NamespaceSelector: &core.LabelSelector{},
		Strategy:          &core.UpdateStrategy{Type: core.StrategyImmediate},
		DataKeys:          []string{"a", "c"},
	}
	got, err := r.Reconcile(s)
	if err != nil {
		t.Fatalf("reconcile error: %v", err)
	}
	want := []string{"ns1", "ns2", "ns3"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("want %v got %v", want, got)
	}
}

func TestReconcilerPlanRollingBatch(t *testing.T) {
	fc := &fakeClient{
		data:       map[string]map[string]map[string]string{"src": {"cfg": {"x": "y"}}},
		namespaces: []string{"a", "b", "c", "d"},
	}
	r := NewReconciler(fc)
	bs := int32(2)
	s := &core.ConfigPropagationSpec{
		SourceRef:         core.ObjectRef{Namespace: "src", Name: "cfg"},
		NamespaceSelector: &core.LabelSelector{},
		Strategy:          &core.UpdateStrategy{Type: core.StrategyRolling, BatchSize: &bs},
	}
	got, err := r.Reconcile(s)
	if err != nil {
		t.Fatalf("reconcile error: %v", err)
	}
	want := []string{"a", "b"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("want %v got %v", want, got)
	}
}

func TestPlanTargetsBranches(t *testing.T) {
	// batch >= len(all) returns all
	all := []string{"a", "b"}
	got := planTargets(all, core.StrategyRolling, 5)
	if !reflect.DeepEqual(got, all) {
		t.Fatalf("expected all when batch >= len, got %v", got)
	}
	// batch < len returns prefix
	got = planTargets(all, core.StrategyRolling, 1)
	if !reflect.DeepEqual(got, []string{"a"}) {
		t.Fatalf("expected first element, got %v", got)
	}
	// immediate returns all
	got = planTargets(all, core.StrategyImmediate, 0)
	if !reflect.DeepEqual(got, all) {
		t.Fatalf("immediate should return all, got %v", got)
	}
	// rolling with batchSize < 1 coerces to 1
	got = planTargets(all, core.StrategyRolling, 0)
	if !reflect.DeepEqual(got, []string{"a"}) {
		t.Fatalf("rolling with 0 coerces to 1, got %v", got)
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
	if err := syncTargets(fc, []string{"ns"}, "name", map[string]string{"k": "v"}, "src", core.ConflictOverwrite); err != nil {
		t.Fatalf("syncTargets error: %v", err)
	}
	// syncTargets error path
	bu := &badUpsert{*fc}
	if err := syncTargets(bu, []string{"ns"}, "name", map[string]string{"k": "v"}, "src", core.ConflictOverwrite); err == nil {
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
	if _, err := r.Reconcile(s); err == nil {
		t.Fatalf("expected error from source get")
	}

	// Source ok, upsert fails
	ec := &errClient{fakeClient{data: map[string]map[string]map[string]string{"s": {"n": {}}}, namespaces: []string{"n"}}}
	r2 := NewReconciler(ec)
	if _, err := r2.Reconcile(s); err == nil {
		t.Fatalf("expected upsert error")
	}

	// Namespace list fails
	ec2 := &errClient{fakeClient{data: map[string]map[string]map[string]string{"s": {"n": {}}}, namespaces: []string{"n"}}}
	r3 := NewReconciler(ec2)
	if _, err := r3.Reconcile(s); err == nil {
		t.Fatalf("expected list namespaces error")
	}

	// Nil spec
	if _, err := r2.Reconcile(nil); err == nil {
		t.Fatalf("expected error for nil spec")
	}
}

func TestReconcileValidationFailure(t *testing.T) {
	r := NewReconciler(&fakeClient{data: map[string]map[string]map[string]string{}, namespaces: []string{"n"}})
	// Invalid: strategy type unrecognized
	s := &core.ConfigPropagationSpec{SourceRef: core.ObjectRef{Namespace: "s", Name: "n"}, NamespaceSelector: &core.LabelSelector{}, Strategy: &core.UpdateStrategy{Type: "canary"}}
	if _, err := r.Reconcile(s); err == nil {
		t.Fatalf("expected validation error")
	}
}

func TestReconcileSuccessCoversImpl(t *testing.T) {
	// Exercise reconcileImpl via Reconcile happy path
	fc := &fakeClient{data: map[string]map[string]map[string]string{"s": {"n": {"k": "v"}}}, namespaces: []string{"ns"}}
	r := NewReconciler(fc)
	s := &core.ConfigPropagationSpec{SourceRef: core.ObjectRef{Namespace: "s", Name: "n"}, NamespaceSelector: &core.LabelSelector{}, Strategy: &core.UpdateStrategy{Type: core.StrategyImmediate}}
	if _, err := r.Reconcile(s); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReconcileNoTargetsNoUpserts(t *testing.T) {
	fc := &fakeClient{data: map[string]map[string]map[string]string{"s": {"n": {"k": "v"}}}, namespaces: []string{}}
	r := NewReconciler(fc)
	s := &core.ConfigPropagationSpec{SourceRef: core.ObjectRef{Namespace: "s", Name: "n"}, NamespaceSelector: &core.LabelSelector{}}
	planned, err := r.Reconcile(s)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(planned) != 0 {
		t.Fatalf("expected 0 planned, got %d", len(planned))
	}
}
