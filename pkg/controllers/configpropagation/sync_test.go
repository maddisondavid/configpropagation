package configpropagation

import (
	"reflect"
	"testing"

	"configpropagation/pkg/adapters"
	"configpropagation/pkg/core"
)

type fakeClientSync struct {
	// namespace -> name -> data
	sources map[string]map[string]map[string]string
	// namespace labels
	nsLabels map[string]map[string]string
	upserts  []struct {
		ns, name            string
		data                map[string]string
		labels, annotations map[string]string
	}
}

func (f *fakeClientSync) GetSourceConfigMap(ns, name string) (map[string]string, error) {
	if m1, ok := f.sources[ns]; ok {
		if d, ok := m1[name]; ok {
			out := map[string]string{}
			for k, v := range d {
				out[k] = v
			}
			return out, nil
		}
	}
	return nil, nil
}

func (f *fakeClientSync) ListNamespacesBySelector(matchLabels map[string]string, _ []adapters.LabelSelectorRequirement) ([]string, error) {
	var res []string
	for ns, lbls := range f.nsLabels {
		ok := true
		for k, v := range matchLabels {
			if lbls[k] != v {
				ok = false
				break
			}
		}
		if ok {
			res = append(res, ns)
		}
	}
	return res, nil
}

func (f *fakeClientSync) UpsertConfigMap(ns, name string, data map[string]string, labels, annotations map[string]string) error {
	// shallow copies for verification stability
	d := map[string]string{}
	for k, v := range data {
		d[k] = v
	}
	l := map[string]string{}
	for k, v := range labels {
		l[k] = v
	}
	a := map[string]string{}
	for k, v := range annotations {
		a[k] = v
	}
	f.upserts = append(f.upserts, struct {
		ns, name            string
		data                map[string]string
		labels, annotations map[string]string
	}{ns, name, d, l, a})
	return nil
}

func (f *fakeClientSync) GetTargetConfigMap(namespace, name string) (map[string]string, map[string]string, map[string]string, bool, error) {
	// Default: no existing target
	return nil, nil, nil, false, nil
}
func (f *fakeClientSync) ListManagedTargetNamespaces(source string, name string) ([]string, error) {
	return []string{}, nil
}
func (f *fakeClientSync) DeleteConfigMap(namespace, name string) error { return nil }
func (f *fakeClientSync) UpdateConfigMapMetadata(namespace, name string, labels, annotations map[string]string) error {
	return nil
}

func TestSyncCopiesFilteredDataAndSetsManagedMetadata(t *testing.T) {
	fc := &fakeClientSync{
		sources: map[string]map[string]map[string]string{
			"src": {"cfg": {"a": "1", "b": "2", "x": "z"}},
		},
		nsLabels: map[string]map[string]string{
			"nsa": {"team": "a"},
			"nsb": {"team": "a"},
			"nsc": {"team": "b"},
		},
	}
	r := NewReconciler(fc)
	s := &core.ConfigPropagationSpec{
		SourceRef:         core.ObjectRef{Namespace: "src", Name: "cfg"},
		NamespaceSelector: &core.LabelSelector{MatchLabels: map[string]string{"team": "a"}},
		DataKeys:          []string{"a", "b"},
		Strategy:          &core.UpdateStrategy{Type: core.StrategyImmediate},
	}
	planned, err := r.Reconcile(s)
	if err != nil {
		t.Fatalf("reconcile error: %v", err)
	}
	// Expect two namespaces selected
	if len(planned) != 2 {
		t.Fatalf("expected 2 planned namespaces, got %d", len(planned))
	}
	// Verify upserts
	if len(fc.upserts) != 2 {
		t.Fatalf("expected 2 upserts, got %d", len(fc.upserts))
	}
	for _, u := range fc.upserts {
		if u.name != "cfg" {
			t.Fatalf("target name should match source: %s", u.name)
		}
		// Data filtered to keys a,b only
		if !reflect.DeepEqual(u.data, map[string]string{"a": "1", "b": "2"}) {
			t.Fatalf("unexpected data: %+v", u.data)
		}
		if u.labels[core.ManagedLabel] != "true" {
			t.Fatalf("managed label missing: %+v", u.labels)
		}
		if u.annotations[core.SourceAnnotation] != "src/cfg" {
			t.Fatalf("source annotation missing: %+v", u.annotations)
		}
		// Hash should be non-empty for non-empty data
		if u.annotations[core.HashAnnotation] == "" {
			t.Fatalf("hash annotation should be set")
		}
	}
}

func TestSyncWhenSourceMissingAndEvents(t *testing.T) {
	fc := &fakeClientSync{
		sources: map[string]map[string]map[string]string{}, // no source present
		nsLabels: map[string]map[string]string{
			"ns": {"team": "x"},
		},
	}
	r := NewReconciler(fc)
	// Exercise event enqueue helpers for coverage
	r.OnCRChange("ns", "name")
	r.OnSourceChange("ns", "name")
	r.OnNamespaceLabelChange("ns", "name")

	s := &core.ConfigPropagationSpec{
		SourceRef:         core.ObjectRef{Namespace: "src", Name: "cfg"},
		NamespaceSelector: &core.LabelSelector{MatchLabels: map[string]string{"team": "x"}},
		Strategy:          &core.UpdateStrategy{Type: core.StrategyImmediate},
	}
	planned, err := r.Reconcile(s)
	if err != nil {
		t.Fatalf("reconcile error: %v", err)
	}
	if len(planned) != 1 {
		t.Fatalf("expected 1 planned, got %d", len(planned))
	}
	if len(fc.upserts) != 1 {
		t.Fatalf("expected 1 upsert, got %d", len(fc.upserts))
	}
	u := fc.upserts[0]
	if len(u.data) != 0 {
		t.Fatalf("expected empty data when source missing, got %+v", u.data)
	}
	if u.annotations[core.HashAnnotation] != "" {
		t.Fatalf("expected empty hash for empty data")
	}
}

func TestSyncCopiesAllWhenNoDataKeysAndExpressionsProvided(t *testing.T) {
	fc := &fakeClientSync{
		sources: map[string]map[string]map[string]string{
			"src": {"cfg": {"k1": "v1", "k2": "v2"}},
		},
		nsLabels: map[string]map[string]string{"ns": {"team": "z"}},
	}
	r := NewReconciler(fc)
	s := &core.ConfigPropagationSpec{
		SourceRef: core.ObjectRef{Namespace: "src", Name: "cfg"},
		NamespaceSelector: &core.LabelSelector{
			MatchLabels:      map[string]string{"team": "z"},
			MatchExpressions: []core.LabelSelectorReq{{Key: "team", Operator: "In", Values: []string{"z"}}},
		},
		Strategy: &core.UpdateStrategy{Type: core.StrategyImmediate},
	}
	planned, err := r.Reconcile(s)
	if err != nil {
		t.Fatalf("reconcile error: %v", err)
	}
	if len(planned) != 1 {
		t.Fatalf("expected 1 planned, got %d", len(planned))
	}
	u := fc.upserts[0]
	if len(u.data) != 2 {
		t.Fatalf("expected full copy of data, got %+v", u.data)
	}
}

func TestSyncIgnoresMissingDataKeys(t *testing.T) {
	fc := &fakeClientSync{
		sources: map[string]map[string]map[string]string{
			"src": {"cfg": {"only": "here"}},
		},
		nsLabels: map[string]map[string]string{"ns": {"team": "z"}},
	}
	r := NewReconciler(fc)
	s := &core.ConfigPropagationSpec{
		SourceRef:         core.ObjectRef{Namespace: "src", Name: "cfg"},
		NamespaceSelector: &core.LabelSelector{MatchLabels: map[string]string{"team": "z"}},
		DataKeys:          []string{"missing", "only"},
		Strategy:          &core.UpdateStrategy{Type: core.StrategyImmediate},
	}
	_, err := r.Reconcile(s)
	if err != nil {
		t.Fatalf("reconcile error: %v", err)
	}
	if len(fc.upserts) != 1 {
		t.Fatalf("expected 1 upsert")
	}
	u := fc.upserts[0]
	if !reflect.DeepEqual(u.data, map[string]string{"only": "here"}) {
		t.Fatalf("unexpected data after filtering: %+v", u.data)
	}
}
