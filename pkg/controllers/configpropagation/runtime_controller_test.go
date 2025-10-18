package configpropagation

import (
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"

	configv1alpha1 "configpropagation/pkg/api/v1alpha1"
	"configpropagation/pkg/core"
)

func TestEmitEventsPublishesExpectedReasons(t *testing.T) {
	recorder := record.NewFakeRecorder(10)
	cp := &configv1alpha1.ConfigPropagation{ObjectMeta: metav1.ObjectMeta{Name: "sample", Namespace: "default"}}
	result := core.RolloutResult{
		TotalTargets:   3,
		CompletedCount: 2,
		Counters: core.PropagationCounters{
			Created: 2,
			Updated: 1,
			Skipped: 1,
			Pruned:  1,
		},
	}

	emitEvents(recorder, cp, result)

	reasons := collectReasons(t, recorder, 4)
	expected := []string{"ConfigMapsCreated", "ConfigMapsUpdated", "TargetsSkipped", "ConfigMapsPruned"}
	for _, reason := range expected {
		if !containsReason(reasons, reason) {
			t.Fatalf("expected reason %s in %v", reason, reasons)
		}
	}
}

func TestEmitEventsCompletionAndNoTargets(t *testing.T) {
	recorder := record.NewFakeRecorder(5)
	cp := &configv1alpha1.ConfigPropagation{ObjectMeta: metav1.ObjectMeta{Name: "sample", Namespace: "default"}}

	emitEvents(recorder, cp, core.RolloutResult{})
	reasons := collectReasons(t, recorder, 1)
	if !containsReason(reasons, "NoTargets") {
		t.Fatalf("expected NoTargets event, got %v", reasons)
	}

	recorder = record.NewFakeRecorder(5)
	result := core.RolloutResult{TotalTargets: 2, CompletedCount: 2}
	emitEvents(recorder, cp, result)
	reasons = collectReasons(t, recorder, 1)
	if !containsReason(reasons, "PropagationComplete") {
		t.Fatalf("expected PropagationComplete event, got %v", reasons)
	}
}

func collectReasons(t *testing.T, recorder *record.FakeRecorder, count int) []string {
	t.Helper()
	reasons := []string{}
	for i := 0; i < count; i++ {
		select {
		case event := <-recorder.Events:
			parts := strings.Split(event, " ")
			if len(parts) >= 2 {
				reasons = append(reasons, parts[1])
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("timed out waiting for event, collected %v", reasons)
		}
	}
	return reasons
}

func containsReason(reasons []string, want string) bool {
	for _, reason := range reasons {
		if reason == want {
			return true
		}
	}
	return false
}
