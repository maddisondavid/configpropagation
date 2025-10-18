package configpropagation

import (
	"configpropagation/pkg/adapters"
	core "configpropagation/pkg/core"
	"reflect"
	"testing"
)

type fakePruneClient struct {
	// pre-known managed namespaces
	managed []string
	// record actions
	deleted  [][2]string
	detached [][2]string
}

func (f *fakePruneClient) GetSourceConfigMap(ns, name string) (map[string]string, error) {
	return map[string]string{"k": "v"}, nil
}
func (f *fakePruneClient) ListNamespacesBySelector(_ map[string]string, _ []adapters.LabelSelectorRequirement) ([]string, error) {
	return nil, nil
}
func (f *fakePruneClient) UpsertConfigMap(ns, name string, data map[string]string, labels, annotations map[string]string) error {
	return nil
}
func (f *fakePruneClient) GetTargetConfigMap(namespace, name string) (map[string]string, map[string]string, map[string]string, bool, error) {
	return nil, nil, nil, false, nil
}
func (f *fakePruneClient) ListManagedTargetNamespaces(source string, name string) ([]string, error) {
	return append([]string(nil), f.managed...), nil
}
func (f *fakePruneClient) DeleteConfigMap(namespace, name string) error {
	f.deleted = append(f.deleted, [2]string{namespace, name})
	return nil
}
func (f *fakePruneClient) UpdateConfigMapMetadata(namespace, name string, labels, annotations map[string]string) error {
	f.detached = append(f.detached, [2]string{namespace, name})
	return nil
}

func TestCleanupDeselectedPruneDeletes(t *testing.T) {
	fc := &fakePruneClient{managed: []string{"a", "b"}}
	s := &core.ConfigPropagationSpec{SourceRef: core.ObjectRef{Namespace: "s", Name: "n"}, NamespaceSelector: &core.LabelSelector{}, Prune: boolPtr(true)}
	summary, err := cleanupDeselected(fc, s, []string{"a"})
	if err != nil {
		t.Fatalf("cleanup error: %v", err)
	}
	if !reflect.DeepEqual(summary.prunedNamespaces, []string{"b"}) {
		t.Fatalf("expected summary with pruned b, got %+v", summary)
	}
	if len(fc.deleted) != 1 || fc.deleted[0] != [2]string{"b", "n"} {
		t.Fatalf("expected delete b/n, got %+v", fc.deleted)
	}
	if len(fc.detached) != 0 {
		t.Fatalf("no detaches expected")
	}
}

func TestCleanupDeselectedDetachWhenPruneFalse(t *testing.T) {
	fc := &fakePruneClient{managed: []string{"a", "b"}}
	s := &core.ConfigPropagationSpec{SourceRef: core.ObjectRef{Namespace: "s", Name: "n"}, NamespaceSelector: &core.LabelSelector{}, Prune: boolPtr(false)}
	summary, err := cleanupDeselected(fc, s, []string{"a"})
	if err != nil {
		t.Fatalf("cleanup error: %v", err)
	}
	if len(summary.prunedNamespaces) != 0 {
		t.Fatalf("expected no pruned namespaces when pruning disabled, got %+v", summary)
	}
	if len(fc.detached) != 1 || fc.detached[0] != [2]string{"b", "n"} {
		t.Fatalf("expected detach b/n, got %+v", fc.detached)
	}
	if len(fc.deleted) != 0 {
		t.Fatalf("no deletes expected")
	}
}

func TestFinalizeCleansAll(t *testing.T) {
	fc := &fakePruneClient{managed: []string{"a", "b"}}
	r := NewReconciler(fc)
	s := &core.ConfigPropagationSpec{SourceRef: core.ObjectRef{Namespace: "s", Name: "n"}, NamespaceSelector: &core.LabelSelector{}, Prune: boolPtr(true)}
	if err := r.Finalize(s); err != nil {
		t.Fatalf("finalize error: %v", err)
	}
	want := [][2]string{{"a", "n"}, {"b", "n"}}
	if !reflect.DeepEqual(fc.deleted, want) {
		t.Fatalf("expected deletes %v, got %v", want, fc.deleted)
	}
}

func TestCleanupDeselectedNoopsWhenNoneManaged(t *testing.T) {
	fc := &fakePruneClient{managed: []string{}}
	s := &core.ConfigPropagationSpec{SourceRef: core.ObjectRef{Namespace: "s", Name: "n"}, NamespaceSelector: &core.LabelSelector{}, Prune: boolPtr(true)}
	if _, err := cleanupDeselected(fc, s, []string{"a"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fc.deleted) != 0 || len(fc.detached) != 0 {
		t.Fatalf("expected no actions")
	}
}

func boolPtr(b bool) *bool { return &b }
