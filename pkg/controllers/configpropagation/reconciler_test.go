package configpropagation

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"configpropagation/pkg/adapters"
	"configpropagation/pkg/core"
)

type fakeClient struct {
	data       map[string]map[string]map[string]string
	namespaces []string
}

func (client *fakeClient) GetSourceConfigMap(namespace, name string) (map[string]string, error) {
	if namespaceData, exists := client.data[namespace]; exists {
		if configMapData, exists := namespaceData[name]; exists {
			copiedData := map[string]string{}

			for key, value := range configMapData {
				copiedData[key] = value
			}

			return copiedData, nil
		}
	}

	return nil, nil
}

func (client *fakeClient) ListNamespacesBySelector(_ map[string]string, _ []adapters.LabelSelectorRequirement) ([]string, error) {
	return append([]string(nil), client.namespaces...), nil
}

func (client *fakeClient) UpsertConfigMap(_ string, _ string, _ map[string]string, _ map[string]string, _ map[string]string) error {
	return nil
}

func (client *fakeClient) GetTargetConfigMap(namespace, name string) (map[string]string, map[string]string, map[string]string, bool, error) {
	return nil, nil, nil, false, nil
}

func (client *fakeClient) ListManagedTargetNamespaces(source string, name string) ([]string, error) {
	return []string{}, nil
}

func (client *fakeClient) DeleteConfigMap(namespace, name string) error { return nil }

func (client *fakeClient) UpdateConfigMapMetadata(namespace, name string, labels, annotations map[string]string) error {
	return nil
}

func TestReconcilerPlanImmediate(t *testing.T) {
	fakeKubeClient := &fakeClient{
		data:       map[string]map[string]map[string]string{"src": {"cfg": {"a": "1", "b": "2", "c": "3"}}},
		namespaces: []string{"ns1", "ns2", "ns3"},
	}
	reconciler := NewReconciler(fakeKubeClient, nil, nil)
	key := Key{Namespace: "default", Name: "cp"}
	spec := &core.ConfigPropagationSpec{
		SourceRef:         core.ObjectRef{Namespace: "src", Name: "cfg"},
		NamespaceSelector: &core.LabelSelector{},
		Strategy:          &core.UpdateStrategy{Type: core.StrategyImmediate},
		DataKeys:          []string{"a", "c"},
	}
	result, err := reconciler.Reconcile(key, spec)
	if err != nil {
		t.Fatalf("reconcile error: %v", err)
	}
	expectedNamespaces := []string{"ns1", "ns2", "ns3"}
	if !reflect.DeepEqual(result.Planned, expectedNamespaces) {
		t.Fatalf("want %v got %v", expectedNamespaces, result.Planned)
	}
	if result.CompletedCount != len(expectedNamespaces) || result.TotalTargets != len(expectedNamespaces) {
		t.Fatalf("expected all targets completed, got %+v", result)
	}
}

func TestReconcilerPlanRollingBatch(t *testing.T) {
	fakeKubeClient := &fakeClient{
		data:       map[string]map[string]map[string]string{"src": {"cfg": {"x": "y"}}},
		namespaces: []string{"a", "b", "c", "d"},
	}
	reconciler := NewReconciler(fakeKubeClient, nil, nil)
	batchSize := int32(2)
	key := Key{Namespace: "default", Name: "cp"}
	spec := &core.ConfigPropagationSpec{
		SourceRef:         core.ObjectRef{Namespace: "src", Name: "cfg"},
		NamespaceSelector: &core.LabelSelector{},
		Strategy:          &core.UpdateStrategy{Type: core.StrategyRolling, BatchSize: &batchSize},
	}
	result, err := reconciler.Reconcile(key, spec)
	if err != nil {
		t.Fatalf("reconcile error: %v", err)
	}
	expectedBatch := []string{"a", "b"}
	if !reflect.DeepEqual(result.Planned, expectedBatch) {
		t.Fatalf("want %v got %v", expectedBatch, result.Planned)
	}
	if result.CompletedCount != len(expectedBatch) {
		t.Fatalf("expected completed count to equal batch size, got %+v", result)
	}

	// next reconcile should continue with remaining namespaces
	nextResult, err := reconciler.Reconcile(key, spec)
	if err != nil {
		t.Fatalf("second reconcile error: %v", err)
	}
	expectedRemainingBatch := []string{"c", "d"}
	if !reflect.DeepEqual(nextResult.Planned, expectedRemainingBatch) {
		t.Fatalf("want %v got %v", expectedRemainingBatch, nextResult.Planned)
	}
	if nextResult.CompletedCount != len(expectedBatch)+len(expectedRemainingBatch) {
		t.Fatalf("expected completed count to accumulate, got %+v", nextResult)
	}
}

func TestPlanTargetsBranches(t *testing.T) {
	rolloutPlanner := core.NewRolloutPlanner()
	identifier := core.NamespacedName{Namespace: "ns", Name: "cp"}
	allTargets := []string{"a", "b"}

	plannedTargets, completedCount := rolloutPlanner.Plan(identifier, "hash", core.StrategyRolling, 1, allTargets)
	if !reflect.DeepEqual(plannedTargets, []string{"a"}) || completedCount != 0 {
		t.Fatalf("expected first element with empty completed, got planned=%v completed=%d", plannedTargets, completedCount)
	}

	rolloutPlanner.MarkCompleted(identifier, "hash", plannedTargets)

	plannedTargets, completedCount = rolloutPlanner.Plan(identifier, "hash", core.StrategyRolling, 2, allTargets)
	if !reflect.DeepEqual(plannedTargets, []string{"b"}) || completedCount != 1 {
		t.Fatalf("expected second element with completed count 1, got planned=%v completed=%d", plannedTargets, completedCount)
	}

	rolloutPlanner.MarkCompleted(identifier, "hash", plannedTargets)

	plannedTargets, completedCount = rolloutPlanner.Plan(identifier, "hash", core.StrategyRolling, 2, allTargets)
	if len(plannedTargets) != 0 || completedCount != 2 {
		t.Fatalf("expected no remaining targets, got planned=%v completed=%d", plannedTargets, completedCount)
	}
	// Immediate returns all and treats them as completed
	plannedTargets, completedCount = rolloutPlanner.Plan(identifier, "hash", core.StrategyImmediate, 0, allTargets)
	if !reflect.DeepEqual(plannedTargets, allTargets) || completedCount != len(allTargets) {
		t.Fatalf("immediate strategy should plan all, got planned=%v completed=%d", plannedTargets, completedCount)
	}
}

func TestHelpersComputeEffectiveAndListTargetsAndSyncTargets(t *testing.T) {
	// computeEffective with nil src and keys -> returns empty map
	effectiveData := computeEffective(nil, nil)
	if len(effectiveData) != 0 {
		t.Fatalf("expected empty effective for nil src")
	}
	// computeEffective copy-all path
	effectiveData = computeEffective(map[string]string{"a": "1"}, nil)
	if !reflect.DeepEqual(effectiveData, map[string]string{"a": "1"}) {
		t.Fatalf("copy-all failed: %+v", effectiveData)
	}
	// listTargets exercises adapter translation and nilIfEmpty
	fakeKubeClient := &fakeClient{data: map[string]map[string]map[string]string{}, namespaces: []string{"x"}}
	selector := &core.LabelSelector{MatchLabels: map[string]string{}, MatchExpressions: []core.LabelSelectorReq{{Key: "k", Operator: "Exists"}}}
	namespaces, err := listTargets(fakeKubeClient, selector)
	if err != nil || !reflect.DeepEqual(namespaces, []string{"x"}) {
		t.Fatalf("listTargets failed: %v %v", namespaces, err)
	}
	// syncTargets executes loop and returns nil
	hashValue := core.HashData(map[string]string{"k": "v"})
	reconciler := NewReconciler(fakeKubeClient, nil, nil)
	if err := reconciler.syncTargets(Key{Namespace: "default", Name: "cp"}, []string{"ns"}, "name", map[string]string{"k": "v"}, hashValue, "src", core.ConflictOverwrite); err != nil {
		t.Fatalf("syncTargets error: %v", err)
	}
	// syncTargets error path
	failingUpsertClient := &badUpsert{*fakeKubeClient}
	failingReconciler := NewReconciler(failingUpsertClient, nil, nil)
	if err := failingReconciler.syncTargets(Key{Namespace: "default", Name: "cp"}, []string{"ns"}, "name", map[string]string{"k": "v"}, hashValue, "src", core.ConflictOverwrite); err == nil {
		t.Fatalf("expected syncTargets to error on upsert")
	}
}

type errClient struct{ fakeClient }

func (client *errClient) GetSourceConfigMap(namespace, name string) (map[string]string, error) {
	return nil, fmt.Errorf("boom")
}

func (client *errClient) UpsertConfigMap(namespace, name string, data map[string]string, labels, annotations map[string]string) error {
	return fmt.Errorf("fail")
}

func (client *errClient) ListNamespacesBySelector(_ map[string]string, _ []adapters.LabelSelectorRequirement) ([]string, error) {
	return nil, fmt.Errorf("nslist")
}

type badUpsert struct{ fakeClient }

func (client *badUpsert) UpsertConfigMap(namespace, name string, data map[string]string, labels, annotations map[string]string) error {
	return fmt.Errorf("nope")
}

func TestReconcileErrorPaths(t *testing.T) {
	reconciler := NewReconciler(&errClient{fakeClient{data: map[string]map[string]map[string]string{}, namespaces: []string{"n"}}}, nil, nil)
	spec := &core.ConfigPropagationSpec{SourceRef: core.ObjectRef{Namespace: "s", Name: "n"}, NamespaceSelector: &core.LabelSelector{}}

	if _, err := reconciler.Reconcile(Key{Namespace: "ns", Name: "cp"}, spec); err == nil {
		t.Fatalf("expected error from source get")
	}

	// Source ok, upsert fails
	failingUpsertErrClient := &errClient{fakeClient{data: map[string]map[string]map[string]string{"s": {"n": {}}}, namespaces: []string{"n"}}}
	reconcilerWithUpsertError := NewReconciler(failingUpsertErrClient, nil, nil)

	if _, err := reconcilerWithUpsertError.Reconcile(Key{Namespace: "ns", Name: "cp"}, spec); err == nil {
		t.Fatalf("expected upsert error")
	}

	// Namespace list fails
	failingListErrClient := &errClient{fakeClient{data: map[string]map[string]map[string]string{"s": {"n": {}}}, namespaces: []string{"n"}}}
	reconcilerWithListError := NewReconciler(failingListErrClient, nil, nil)

	if _, err := reconcilerWithListError.Reconcile(Key{Namespace: "ns", Name: "cp"}, spec); err == nil {
		t.Fatalf("expected list namespaces error")
	}

	// Nil spec
	if _, err := reconcilerWithUpsertError.Reconcile(Key{Namespace: "ns", Name: "cp"}, nil); err == nil {
		t.Fatalf("expected error for nil spec")
	}
}

func TestReconcileValidationFailure(t *testing.T) {
	reconciler := NewReconciler(&fakeClient{data: map[string]map[string]map[string]string{}, namespaces: []string{"n"}}, nil, nil)
	// Invalid: strategy type unrecognized
	spec := &core.ConfigPropagationSpec{SourceRef: core.ObjectRef{Namespace: "s", Name: "n"}, NamespaceSelector: &core.LabelSelector{}, Strategy: &core.UpdateStrategy{Type: "canary"}}

	if _, err := reconciler.Reconcile(Key{Namespace: "ns", Name: "cp"}, spec); err == nil {
		t.Fatalf("expected validation error")
	}
}

func TestReconcileSuccessCoversImpl(t *testing.T) {
	// Exercise reconcileImpl via Reconcile happy path
	fakeKubeClient := &fakeClient{data: map[string]map[string]map[string]string{"s": {"n": {"k": "v"}}}, namespaces: []string{"ns"}}
	reconciler := NewReconciler(fakeKubeClient, nil, nil)
	spec := &core.ConfigPropagationSpec{SourceRef: core.ObjectRef{Namespace: "s", Name: "n"}, NamespaceSelector: &core.LabelSelector{}, Strategy: &core.UpdateStrategy{Type: core.StrategyImmediate}}

	if _, err := reconciler.Reconcile(Key{Namespace: "ns", Name: "cp"}, spec); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReconcileNoTargetsNoUpserts(t *testing.T) {
	fakeKubeClient := &fakeClient{data: map[string]map[string]map[string]string{"s": {"n": {"k": "v"}}}, namespaces: []string{}}
	reconciler := NewReconciler(fakeKubeClient, nil, nil)
	spec := &core.ConfigPropagationSpec{SourceRef: core.ObjectRef{Namespace: "s", Name: "n"}, NamespaceSelector: &core.LabelSelector{}}
	rolloutResult, err := reconciler.Reconcile(Key{Namespace: "ns", Name: "cp"}, spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rolloutResult.Planned) != 0 {
		t.Fatalf("expected 0 planned, got %d", len(rolloutResult.Planned))
	}
}

type capturedEvent struct {
	namespacedName core.NamespacedName
	reason         string
	message        string
	eventType      string
}

type capturingEventRecorder struct {
	events []capturedEvent
}

func (recorder *capturingEventRecorder) Normalf(name core.NamespacedName, reason, messageFmt string, args ...interface{}) {
	recorder.events = append(recorder.events, capturedEvent{
		namespacedName: name,
		reason:         reason,
		message:        fmt.Sprintf(messageFmt, args...),
		eventType:      "Normal",
	})
}

func (recorder *capturingEventRecorder) Warningf(name core.NamespacedName, reason, messageFmt string, args ...interface{}) {
	recorder.events = append(recorder.events, capturedEvent{
		namespacedName: name,
		reason:         reason,
		message:        fmt.Sprintf(messageFmt, args...),
		eventType:      "Warning",
	})
}

type capturingMetricsRecorder struct {
	counts    map[string]int
	errors    map[string]int
	targets   []struct{ total, outOfSync int }
	durations []time.Duration
}

func newCapturingMetricsRecorder() *capturingMetricsRecorder {
	return &capturingMetricsRecorder{counts: map[string]int{}, errors: map[string]int{}}
}

func (recorder *capturingMetricsRecorder) AddPropagations(action string, count int) {
	recorder.counts[action] += count
}

func (recorder *capturingMetricsRecorder) ObserveTargets(total, outOfSync int) {
	recorder.targets = append(recorder.targets, struct{ total, outOfSync int }{total: total, outOfSync: outOfSync})
}

func (recorder *capturingMetricsRecorder) ObserveReconcileDuration(duration time.Duration) {
	recorder.durations = append(recorder.durations, duration)
}

func (recorder *capturingMetricsRecorder) IncError(stage string) {
	recorder.errors[stage]++
}

type instrumentationClient struct {
	upserts  []string
	deletes  []string
	skipHash string
}

func newInstrumentationClient() *instrumentationClient {
	return &instrumentationClient{skipHash: core.HashData(map[string]string{"key": "value"})}
}

func (client *instrumentationClient) GetSourceConfigMap(namespace, name string) (map[string]string, error) {
	return map[string]string{"key": "value"}, nil
}

func (client *instrumentationClient) ListNamespacesBySelector(map[string]string, []adapters.LabelSelectorRequirement) ([]string, error) {
	return []string{"new", "skip", "update"}, nil
}

func (client *instrumentationClient) UpsertConfigMap(namespace, name string, data map[string]string, labels, annotations map[string]string) error {
	client.upserts = append(client.upserts, fmt.Sprintf("%s/%s", namespace, name))
	return nil
}

func (client *instrumentationClient) GetTargetConfigMap(namespace, name string) (map[string]string, map[string]string, map[string]string, bool, error) {
	switch namespace {
	case "new":
		return nil, nil, nil, false, nil
	case "skip":
		return nil,
			map[string]string{core.ManagedLabel: "true"},
			map[string]string{core.SourceAnnotation: "src/cfg", core.HashAnnotation: client.skipHash},
			true, nil
	case "update":
		return nil,
			map[string]string{core.ManagedLabel: "true"},
			map[string]string{core.SourceAnnotation: "src/cfg", core.HashAnnotation: "outdated"},
			true, nil
	default:
		return nil, nil, nil, false, nil
	}
}

func (client *instrumentationClient) ListManagedTargetNamespaces(source string, name string) ([]string, error) {
	if source == "src/cfg" && name == "cfg" {
		return []string{"orphan"}, nil
	}
	return []string{}, nil
}

func (client *instrumentationClient) DeleteConfigMap(namespace, name string) error {
	client.deletes = append(client.deletes, fmt.Sprintf("%s/%s", namespace, name))
	return nil
}

func (client *instrumentationClient) UpdateConfigMapMetadata(namespace, name string, labels, annotations map[string]string) error {
	return nil
}

func TestReconcilerEmitsEventsAndMetrics(t *testing.T) {
	client := newInstrumentationClient()
	eventRecorder := &capturingEventRecorder{}
	metricsRecorder := newCapturingMetricsRecorder()
	reconciler := NewReconciler(client, eventRecorder, metricsRecorder)

	prune := true
	spec := &core.ConfigPropagationSpec{
		SourceRef:         core.ObjectRef{Namespace: "src", Name: "cfg"},
		NamespaceSelector: &core.LabelSelector{},
		Strategy:          &core.UpdateStrategy{Type: core.StrategyImmediate},
		ConflictPolicy:    core.ConflictOverwrite,
		Prune:             &prune,
	}

	result, err := reconciler.Reconcile(Key{Namespace: "default", Name: "cp"}, spec)
	if err != nil {
		t.Fatalf("unexpected reconcile error: %v", err)
	}
	if result.TotalTargets != 3 || result.CompletedCount != 3 {
		t.Fatalf("unexpected rollout result: %+v", result)
	}

	if len(client.upserts) != 2 {
		t.Fatalf("expected 2 upserts, got %d", len(client.upserts))
	}
	if len(client.deletes) != 1 || client.deletes[0] != "orphan/cfg" {
		t.Fatalf("expected prune of orphan/cfg, got %+v", client.deletes)
	}

	reasonCounts := map[string]int{}
	for _, event := range eventRecorder.events {
		if event.eventType == "Warning" {
			t.Fatalf("unexpected warning event: %+v", event)
		}
		reasonCounts[event.reason]++
	}

	if reasonCounts[eventReasonConfigCreated] != 1 {
		t.Fatalf("expected create event, got %+v", reasonCounts)
	}
	if reasonCounts[eventReasonConfigUpdated] != 1 {
		t.Fatalf("expected update event, got %+v", reasonCounts)
	}
	if reasonCounts[eventReasonConfigSkipped] != 1 {
		t.Fatalf("expected skip event, got %+v", reasonCounts)
	}
	if reasonCounts[eventReasonConfigPruned] != 1 {
		t.Fatalf("expected prune event, got %+v", reasonCounts)
	}

	if metricsRecorder.counts[adapters.MetricsActionCreate] != 1 {
		t.Fatalf("expected create metric count 1, got %+v", metricsRecorder.counts)
	}
	if metricsRecorder.counts[adapters.MetricsActionUpdate] != 1 {
		t.Fatalf("expected update metric count 1, got %+v", metricsRecorder.counts)
	}
	if metricsRecorder.counts[adapters.MetricsActionSkip] != 1 {
		t.Fatalf("expected skip metric count 1, got %+v", metricsRecorder.counts)
	}
	if metricsRecorder.counts[adapters.MetricsActionPrune] != 1 {
		t.Fatalf("expected prune metric count 1, got %+v", metricsRecorder.counts)
	}

	if len(metricsRecorder.targets) == 0 {
		t.Fatalf("expected target gauge observations")
	}
	lastTarget := metricsRecorder.targets[len(metricsRecorder.targets)-1]
	if lastTarget.total != 3 || lastTarget.outOfSync != 0 {
		t.Fatalf("unexpected gauge observation: %+v", lastTarget)
	}

	if len(metricsRecorder.durations) == 0 {
		t.Fatalf("expected reconcile duration observation")
	}
	if len(metricsRecorder.errors) != 0 {
		t.Fatalf("unexpected errors recorded: %+v", metricsRecorder.errors)
	}
}
