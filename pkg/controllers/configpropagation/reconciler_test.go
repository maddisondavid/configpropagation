package configpropagation

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"configpropagation/pkg/adapters"
	"configpropagation/pkg/core"
)

func noSleepBackoff() func() core.BackoffStrategy {
	return func() core.BackoffStrategy {
		b := core.DefaultBackoff()
		b.Sleeper = core.FuncSleeper(func(time.Duration) {})
		b.Rand = func() float64 { return 0 }
		return b
	}
}

type fakeClient struct {
	data       map[string]map[string]map[string]string
	namespaces []string
}

func (f *fakeClient) GetSourceConfigMap(ns, name string) (map[string]string, error) {
	if nsMap, ok := f.data[ns]; ok {
		if d, ok := nsMap[name]; ok {
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

func (f *fakeClient) GetTargetConfigMap(string, string) (map[string]string, map[string]string, map[string]string, bool, error) {
	return nil, nil, nil, false, nil
}

func (f *fakeClient) ListManagedTargetNamespaces(string, string) ([]string, error) {
	return []string{}, nil
}

func (f *fakeClient) DeleteConfigMap(string, string) error { return nil }

func (f *fakeClient) UpdateConfigMapMetadata(string, string, map[string]string, map[string]string) error {
	return nil
}

func TestReconcilerPlanImmediate(t *testing.T) {
	fc := &fakeClient{
		data:       map[string]map[string]map[string]string{"src": {"cfg": {"a": "1", "b": "2", "c": "3"}}},
		namespaces: []string{"ns1", "ns2", "ns3"},
	}
	r := NewReconciler(fc)
	r.backoff = noSleepBackoff()
	key := Key{Namespace: "default", Name: "cp"}
	spec := &core.ConfigPropagationSpec{
		SourceRef:         core.ObjectRef{Namespace: "src", Name: "cfg"},
		NamespaceSelector: &core.LabelSelector{},
		Strategy:          &core.UpdateStrategy{Type: core.StrategyImmediate},
		DataKeys:          []string{"a", "c"},
	}

	got, err := r.Reconcile(key, spec)
	if err != nil {
		t.Fatalf("reconcile error: %v", err)
	}
	want := []string{"ns1", "ns2", "ns3"}
	if !reflect.DeepEqual(got.Planned, want) {
		t.Fatalf("want %v got %v", want, got.Planned)
	}
	if !reflect.DeepEqual(got.Synced, want) {
		t.Fatalf("expected all namespaces synced, got %v", got.Synced)
	}
	if got.CompletedCount != len(want) || got.TotalTargets != len(want) {
		t.Fatalf("expected completed count to equal total, got %+v", got)
	}
}

func TestReconcilerPlanRollingBatch(t *testing.T) {
	fc := &fakeClient{
		data:       map[string]map[string]map[string]string{"src": {"cfg": {"x": "y"}}},
		namespaces: []string{"a", "b", "c", "d"},
	}
	r := NewReconciler(fc)
	r.backoff = noSleepBackoff()
	bs := int32(2)
	key := Key{Namespace: "default", Name: "cp"}
	spec := &core.ConfigPropagationSpec{
		SourceRef:         core.ObjectRef{Namespace: "src", Name: "cfg"},
		NamespaceSelector: &core.LabelSelector{},
		Strategy:          &core.UpdateStrategy{Type: core.StrategyRolling, BatchSize: &bs},
	}

	first, err := r.Reconcile(key, spec)
	if err != nil {
		t.Fatalf("reconcile error: %v", err)
	}
	if !reflect.DeepEqual(first.Planned, []string{"a", "b"}) {
		t.Fatalf("unexpected planned set: %+v", first.Planned)
	}
	if !reflect.DeepEqual(first.Synced, first.Planned) {
		t.Fatalf("expected synced namespaces to match planned: %+v", first)
	}
	if first.CompletedCount != len(first.Synced) {
		t.Fatalf("expected completed count to equal synced length, got %+v", first)
	}

	second, err := r.Reconcile(key, spec)
	if err != nil {
		t.Fatalf("second reconcile error: %v", err)
	}
	if !reflect.DeepEqual(second.Planned, []string{"c", "d"}) {
		t.Fatalf("unexpected planned set on second reconcile: %+v", second.Planned)
	}
	if second.CompletedCount != len(first.Synced)+len(second.Synced) {
		t.Fatalf("expected completed count to accumulate, got %+v", second)
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
	planned, completed = planner.Plan(key, "hash", core.StrategyImmediate, 0, all)
	if !reflect.DeepEqual(planned, all) || completed != len(all) {
		t.Fatalf("immediate strategy should plan all, got planned=%v completed=%d", planned, completed)
	}
}

func TestHelpersComputeEffectiveAndListTargetsAndSyncTargets(t *testing.T) {
	out := computeEffective(nil, nil)
	if len(out) != 0 {
		t.Fatalf("expected empty effective for nil src")
	}
	out = computeEffective(map[string]string{"a": "1"}, nil)
	if !reflect.DeepEqual(out, map[string]string{"a": "1"}) {
		t.Fatalf("copy-all failed: %+v", out)
	}
	fc := &fakeClient{data: map[string]map[string]map[string]string{}, namespaces: []string{"x"}}
	sel := &core.LabelSelector{MatchLabels: map[string]string{}, MatchExpressions: []core.LabelSelectorReq{{Key: "k", Operator: "Exists"}}}
	got, err := listTargets(fc, sel)
	if err != nil || !reflect.DeepEqual(got, []string{"x"}) {
		t.Fatalf("listTargets failed: %v %v", got, err)
	}
	hash := core.HashData(map[string]string{"k": "v"})
	outcome := syncTargets(fc, []string{"ns"}, "name", map[string]string{"k": "v"}, hash, "src", core.ConflictOverwrite, noSleepBackoff())
	if len(outcome.synced) != 1 || len(outcome.failed) != 0 {
		t.Fatalf("expected successful sync, got %+v", outcome)
	}
	bu := &badUpsert{*fc}
	outcome = syncTargets(bu, []string{"ns"}, "name", map[string]string{"k": "v"}, hash, "src", core.ConflictOverwrite, noSleepBackoff())
	if len(outcome.failed) != 1 || outcome.failed[0].Reason != core.ReasonPermanentError {
		t.Fatalf("expected permanent failure recorded, got %+v", outcome.failed)
	}
}

type errClient struct{ fakeClient }

type badUpsert struct{ fakeClient }

func (e *errClient) GetSourceConfigMap(ns, name string) (map[string]string, error) {
	return nil, fmt.Errorf("boom")
}

func (e *errClient) UpsertConfigMap(ns, name string, d map[string]string, l, a map[string]string) error {
	return fmt.Errorf("fail")
}

func (e *errClient) ListNamespacesBySelector(map[string]string, []adapters.LabelSelectorRequirement) ([]string, error) {
	return nil, fmt.Errorf("nslist")
}

func (b *badUpsert) UpsertConfigMap(ns, name string, d map[string]string, l, a map[string]string) error {
	return fmt.Errorf("nope")
}

func TestReconcileErrorPaths(t *testing.T) {
	r := NewReconciler(&errClient{fakeClient{data: map[string]map[string]map[string]string{}, namespaces: []string{"n"}}})
	r.backoff = noSleepBackoff()
	spec := &core.ConfigPropagationSpec{SourceRef: core.ObjectRef{Namespace: "s", Name: "n"}, NamespaceSelector: &core.LabelSelector{}}
	if _, err := r.Reconcile(Key{Namespace: "ns", Name: "cp"}, spec); err == nil {
		t.Fatalf("expected error from source get")
	}

	ec := &badUpsert{fakeClient{data: map[string]map[string]map[string]string{"s": {"n": {}}}, namespaces: []string{"n"}}}
	r2 := NewReconciler(ec)
	r2.backoff = noSleepBackoff()
	res, err := r2.Reconcile(Key{Namespace: "ns", Name: "cp"}, spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.Failed) != 1 || res.Failed[0].Namespace != "n" {
		t.Fatalf("expected failure recorded for namespace n, got %+v", res.Failed)
	}

	ec2 := &errClient{fakeClient{data: map[string]map[string]map[string]string{"s": {"n": {}}}, namespaces: []string{"n"}}}
	r3 := NewReconciler(ec2)
	r3.backoff = noSleepBackoff()
	if _, err := r3.Reconcile(Key{Namespace: "ns", Name: "cp"}, spec); err == nil {
		t.Fatalf("expected list namespaces error")
	}

	if _, err := r2.Reconcile(Key{Namespace: "ns", Name: "cp"}, nil); err == nil {
		t.Fatalf("expected error for nil spec")
	}
}

func TestReconcileValidationFailure(t *testing.T) {
	r := NewReconciler(&fakeClient{data: map[string]map[string]map[string]string{}, namespaces: []string{"n"}})
	r.backoff = noSleepBackoff()
	spec := &core.ConfigPropagationSpec{SourceRef: core.ObjectRef{Namespace: "s", Name: "n"}, NamespaceSelector: &core.LabelSelector{}, Strategy: &core.UpdateStrategy{Type: "canary"}}
	if _, err := r.Reconcile(Key{Namespace: "ns", Name: "cp"}, spec); err == nil {
		t.Fatalf("expected validation error")
	}
}

func TestReconcileNoTargetsNoUpserts(t *testing.T) {
	fc := &fakeClient{data: map[string]map[string]map[string]string{"s": {"n": {"k": "v"}}}, namespaces: []string{}}
	r := NewReconciler(fc)
	r.backoff = noSleepBackoff()
	spec := &core.ConfigPropagationSpec{SourceRef: core.ObjectRef{Namespace: "s", Name: "n"}, NamespaceSelector: &core.LabelSelector{}}
	result, err := r.Reconcile(Key{Namespace: "ns", Name: "cp"}, spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Planned) != 0 || len(result.Synced) != 0 {
		t.Fatalf("expected no planned or synced namespaces, got %+v", result)
	}
}
