package configpropagation

import (
	"configpropagation/pkg/adapters"
	core "configpropagation/pkg/core"
	"reflect"
	"testing"
)

type detachRecord struct {
	namespace   string
	name        string
	labels      map[string]string
	annotations map[string]string
}

type fakePruneClient struct {
	// pre-known managed namespaces
	managed []string
	// optional metadata for targets
	targetLabels      map[string]map[string]string
	targetAnnotations map[string]map[string]string
	// record actions
	deleted  [][2]string
	detached []detachRecord
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
	labels := map[string]string{}
	annotations := map[string]string{}

	if namespaceLabels, ok := f.targetLabels[namespace]; ok {
		for key, value := range namespaceLabels {
			labels[key] = value
		}
	}

	if namespaceAnnotations, ok := f.targetAnnotations[namespace]; ok {
		for key, value := range namespaceAnnotations {
			annotations[key] = value
		}
	}

	if len(labels) == 0 && len(annotations) == 0 {
		return nil, nil, nil, false, nil
	}

	return nil, labels, annotations, true, nil
}
func (f *fakePruneClient) ListManagedTargetNamespaces(source string, name string) ([]string, error) {
	return append([]string(nil), f.managed...), nil
}
func (f *fakePruneClient) DeleteConfigMap(namespace, name string) error {
	f.deleted = append(f.deleted, [2]string{namespace, name})
	return nil
}
func (f *fakePruneClient) UpdateConfigMapMetadata(namespace, name string, labels, annotations map[string]string) error {
	copiedLabels := map[string]string{}
	for key, value := range labels {
		copiedLabels[key] = value
	}

	copiedAnnotations := map[string]string{}
	for key, value := range annotations {
		copiedAnnotations[key] = value
	}

	f.detached = append(f.detached, detachRecord{namespace: namespace, name: name, labels: copiedLabels, annotations: copiedAnnotations})
	return nil
}

func TestCleanupDeselectedPruneDeletes(t *testing.T) {
	fc := &fakePruneClient{managed: []string{"a", "b"}}
	s := &core.ConfigPropagationSpec{SourceRef: core.ObjectRef{Namespace: "s", Name: "n"}, NamespaceSelector: &core.LabelSelector{}, Prune: boolPtr(true)}
	if err := cleanupDeselected(fc, s, []string{"a"}); err != nil {
		t.Fatalf("cleanup error: %v", err)
	}
	if len(fc.deleted) != 1 || fc.deleted[0] != [2]string{"b", "n"} {
		t.Fatalf("expected delete b/n, got %+v", fc.deleted)
	}
	if len(fc.detached) != 0 {
		t.Fatalf("no detaches expected")
	}
}

func TestCleanupDeselectedDetachWhenPruneFalse(t *testing.T) {
	fc := &fakePruneClient{
		managed: []string{"a", "b"},
		targetLabels: map[string]map[string]string{
			"b": {core.ManagedLabel: "true", "keep": "value"},
		},
		targetAnnotations: map[string]map[string]string{
			"b": {core.SourceAnnotation: "s/n", core.HashAnnotation: "hash", "stay": "yes"},
		},
	}
	s := &core.ConfigPropagationSpec{SourceRef: core.ObjectRef{Namespace: "s", Name: "n"}, NamespaceSelector: &core.LabelSelector{}, Prune: boolPtr(false)}
	if err := cleanupDeselected(fc, s, []string{"a"}); err != nil {
		t.Fatalf("cleanup error: %v", err)
	}
	if len(fc.detached) != 1 {
		t.Fatalf("expected single detach, got %+v", fc.detached)
	}
	record := fc.detached[0]
	if record.namespace != "b" || record.name != "n" {
		t.Fatalf("expected detach b/n, got %+v", record)
	}
	if record.labels["keep"] != "value" || record.labels[core.ManagedLabel] != "" {
		t.Fatalf("expected managed label removed but other labels kept: %+v", record.labels)
	}
	if record.annotations["stay"] != "yes" || record.annotations[core.SourceAnnotation] != "" || record.annotations[core.HashAnnotation] != "" {
		t.Fatalf("expected managed annotations removed but others kept: %+v", record.annotations)
	}
	if len(fc.deleted) != 0 {
		t.Fatalf("no deletes expected")
	}
}

func TestCleanupDeselectedDetachSkipsWhenTargetMissing(t *testing.T) {
	fc := &fakePruneClient{managed: []string{"a"}}
	s := &core.ConfigPropagationSpec{SourceRef: core.ObjectRef{Namespace: "s", Name: "n"}, NamespaceSelector: &core.LabelSelector{}, Prune: boolPtr(false)}
	if err := cleanupDeselected(fc, s, []string{}); err != nil {
		t.Fatalf("cleanup error: %v", err)
	}
	if len(fc.detached) != 0 {
		t.Fatalf("expected no detach when target missing")
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
	if err := cleanupDeselected(fc, s, []string{"a"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fc.deleted) != 0 || len(fc.detached) != 0 {
		t.Fatalf("expected no actions")
	}
}

func boolPtr(b bool) *bool { return &b }
