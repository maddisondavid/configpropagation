package configpropagation

import (
	"fmt"
	"reflect"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"configpropagation/pkg/adapters"
	"configpropagation/pkg/core"
)

type fakeClientSync struct {
	sources  map[string]map[string]map[string]string
	nsLabels map[string]map[string]string
	upserts  []struct {
		ns, name            string
		data                map[string]string
		labels, annotations map[string]string
	}
}

type fakeRetryClient struct {
	*fakeClientSync
	perNamespaceErr map[string]error
}

func (f *fakeRetryClient) GetTargetConfigMap(namespace, name string) (map[string]string, map[string]string, map[string]string, bool, error) {
	if err, ok := f.perNamespaceErr[namespace]; ok {
		return nil, nil, nil, false, err
	}
	return f.fakeClientSync.GetTargetConfigMap(namespace, name)
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

func (f *fakeClientSync) GetTargetConfigMap(string, string) (map[string]string, map[string]string, map[string]string, bool, error) {
	return nil, nil, nil, false, nil
}

func (f *fakeClientSync) ListManagedTargetNamespaces(string, string) ([]string, error) {
	return []string{}, nil
}

func (f *fakeClientSync) DeleteConfigMap(string, string) error { return nil }

func (f *fakeClientSync) UpdateConfigMapMetadata(string, string, map[string]string, map[string]string) error {
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
	r.backoff = noSleepBackoff()
	key := Key{Namespace: "default", Name: "cp"}
	spec := &core.ConfigPropagationSpec{
		SourceRef:         core.ObjectRef{Namespace: "src", Name: "cfg"},
		NamespaceSelector: &core.LabelSelector{MatchLabels: map[string]string{"team": "a"}},
		DataKeys:          []string{"a", "b"},
		Strategy:          &core.UpdateStrategy{Type: core.StrategyImmediate},
	}
	result, err := r.Reconcile(key, spec)
	if err != nil {
		t.Fatalf("reconcile error: %v", err)
	}
	if len(result.Planned) != 2 || len(result.Synced) != 2 {
		t.Fatalf("expected two planned/synced namespaces, got %+v", result)
	}
	if len(fc.upserts) != 2 {
		t.Fatalf("expected 2 upserts, got %d", len(fc.upserts))
	}
	for _, u := range fc.upserts {
		if !reflect.DeepEqual(u.data, map[string]string{"a": "1", "b": "2"}) {
			t.Fatalf("unexpected data: %+v", u.data)
		}
		if u.labels[core.ManagedLabel] != "true" || u.annotations[core.SourceAnnotation] != "src/cfg" || u.annotations[core.HashAnnotation] == "" {
			t.Fatalf("managed metadata missing: labels=%+v annotations=%+v", u.labels, u.annotations)
		}
	}
}

func TestSyncWhenSourceMissingAndEvents(t *testing.T) {
	fc := &fakeClientSync{
		sources: map[string]map[string]map[string]string{},
		nsLabels: map[string]map[string]string{
			"ns": {"team": "x"},
		},
	}
	r := NewReconciler(fc)
	r.backoff = noSleepBackoff()
	r.OnCRChange("ns", "name")
	r.OnSourceChange("ns", "name")
	r.OnNamespaceLabelChange("ns", "name")

	key := Key{Namespace: "default", Name: "cp"}
	spec := &core.ConfigPropagationSpec{
		SourceRef:         core.ObjectRef{Namespace: "src", Name: "cfg"},
		NamespaceSelector: &core.LabelSelector{MatchLabels: map[string]string{"team": "x"}},
		Strategy:          &core.UpdateStrategy{Type: core.StrategyImmediate},
	}
	result, err := r.Reconcile(key, spec)
	if err != nil {
		t.Fatalf("reconcile error: %v", err)
	}
	if len(result.Planned) != 1 || len(fc.upserts) != 1 {
		t.Fatalf("expected one planned namespace and one upsert, got %+v upserts=%d", result, len(fc.upserts))
	}
	if len(fc.upserts[0].data) != 0 || fc.upserts[0].annotations[core.HashAnnotation] != "" {
		t.Fatalf("expected empty data/hash for missing source, got %+v", fc.upserts[0])
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
	r.backoff = noSleepBackoff()
	key := Key{Namespace: "default", Name: "cp"}
	spec := &core.ConfigPropagationSpec{
		SourceRef: core.ObjectRef{Namespace: "src", Name: "cfg"},
		NamespaceSelector: &core.LabelSelector{
			MatchLabels:      map[string]string{"team": "z"},
			MatchExpressions: []core.LabelSelectorReq{{Key: "team", Operator: "In", Values: []string{"z"}}},
		},
		Strategy: &core.UpdateStrategy{Type: core.StrategyImmediate},
	}
	result, err := r.Reconcile(key, spec)
	if err != nil {
		t.Fatalf("reconcile error: %v", err)
	}
	if len(result.Planned) != 1 {
		t.Fatalf("expected 1 planned, got %d", len(result.Planned))
	}
	if !reflect.DeepEqual(fc.upserts[0].data, map[string]string{"k1": "v1", "k2": "v2"}) {
		t.Fatalf("expected full copy of data, got %+v", fc.upserts[0].data)
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
	r.backoff = noSleepBackoff()
	key := Key{Namespace: "default", Name: "cp"}
	spec := &core.ConfigPropagationSpec{
		SourceRef:         core.ObjectRef{Namespace: "src", Name: "cfg"},
		NamespaceSelector: &core.LabelSelector{MatchLabels: map[string]string{"team": "z"}},
		DataKeys:          []string{"missing", "only"},
		Strategy:          &core.UpdateStrategy{Type: core.StrategyImmediate},
	}
	result, err := r.Reconcile(key, spec)
	if err != nil {
		t.Fatalf("reconcile error: %v", err)
	}
	if len(result.Planned) != 1 || len(fc.upserts) != 1 {
		t.Fatalf("expected single planned namespace and upsert, got %+v upserts=%d", result, len(fc.upserts))
	}
	if !reflect.DeepEqual(fc.upserts[0].data, map[string]string{"only": "here"}) {
		t.Fatalf("unexpected data after filtering: %+v", fc.upserts[0].data)
	}
}

func TestSyncRecordsRBACFailuresAndContinues(t *testing.T) {
	base := &fakeClientSync{
		sources: map[string]map[string]map[string]string{
			"src": {"cfg": {"a": "1"}},
		},
		nsLabels: map[string]map[string]string{
			"good": {"team": "x"},
			"bad":  {"team": "x"},
		},
	}
	rc := &fakeRetryClient{
		fakeClientSync: base,
		perNamespaceErr: map[string]error{
			"bad": apierrors.NewForbidden(schema.GroupResource{Group: "", Resource: "configmaps"}, "cfg", fmt.Errorf("denied")),
		},
	}
	r := NewReconciler(rc)
	r.backoff = noSleepBackoff()
	key := Key{Namespace: "default", Name: "cp"}
	spec := &core.ConfigPropagationSpec{
		SourceRef:         core.ObjectRef{Namespace: "src", Name: "cfg"},
		NamespaceSelector: &core.LabelSelector{MatchLabels: map[string]string{"team": "x"}},
		Strategy:          &core.UpdateStrategy{Type: core.StrategyImmediate},
	}
	result, err := r.Reconcile(key, spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Synced) != 1 || result.Synced[0] != "good" {
		t.Fatalf("expected good namespace synced, got %+v", result.Synced)
	}
	if len(result.Failed) != 1 || result.Failed[0].Namespace != "bad" || result.Failed[0].Reason != core.ReasonRBACDenied {
		t.Fatalf("expected RBAC failure recorded, got %+v", result.Failed)
	}
	if result.Retries["bad"] == 0 {
		t.Fatalf("expected retry attempts recorded for bad namespace")
	}
}

func TestSyncWarnsAndBlocksOnPayloadSize(t *testing.T) {
	warnSize := core.ConfigMapSizeWarnThresholdBytes + 10
	dataWarn := string(make([]byte, warnSize))
	base := &fakeClientSync{
		sources: map[string]map[string]map[string]string{
			"src": {"cfg": {"key": dataWarn}},
		},
		nsLabels: map[string]map[string]string{"ns": {"team": "x"}},
	}
	r := NewReconciler(base)
	r.backoff = noSleepBackoff()
	key := Key{Namespace: "default", Name: "cp"}
	spec := &core.ConfigPropagationSpec{
		SourceRef:         core.ObjectRef{Namespace: "src", Name: "cfg"},
		NamespaceSelector: &core.LabelSelector{MatchLabels: map[string]string{"team": "x"}},
		Strategy:          &core.UpdateStrategy{Type: core.StrategyImmediate},
	}
	result, err := r.Reconcile(key, spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Warnings) != 1 || result.Warnings[0].Reason != core.WarningLargePayload {
		t.Fatalf("expected warning recorded, got %+v", result.Warnings)
	}

	tooBig := string(make([]byte, core.ConfigMapSizeLimitBytes+1))
	base.sources["src"]["cfg"]["key"] = tooBig
	base.upserts = nil
	result, err = r.Reconcile(key, spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Failed) != 1 || result.Failed[0].Reason != core.ReasonPayloadTooLarge {
		t.Fatalf("expected payload too large failure, got %+v", result.Failed)
	}
	if len(base.upserts) != 0 {
		t.Fatalf("expected no upserts when payload too large")
	}
}
