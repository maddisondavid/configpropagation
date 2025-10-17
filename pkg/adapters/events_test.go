package adapters

import (
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"configpropagation/pkg/agents/summary"
	configv1alpha1 "configpropagation/pkg/api/v1alpha1"
)

type fakeEventRecorder struct {
	events []recordedEvent
}

type recordedEvent struct {
	eventType string
	reason    string
	message   string
}

func (f *fakeEventRecorder) Event(object runtime.Object, eventtype, reason, message string) {
	f.events = append(f.events, recordedEvent{eventType: eventtype, reason: reason, message: message})
}

func (f *fakeEventRecorder) Eventf(object runtime.Object, eventtype, reason, messageFmt string, args ...interface{}) {
	f.events = append(f.events, recordedEvent{eventType: eventtype, reason: reason, message: fmt.Sprintf(messageFmt, args...)})
}

func (f *fakeEventRecorder) PastEventf(object runtime.Object, timestamp metav1.Time, eventtype, reason, messageFmt string, args ...interface{}) {
}
func (f *fakeEventRecorder) AnnotatedEventf(object runtime.Object, annotations map[string]string, eventtype, reason, messageFmt string, args ...interface{}) {
}

func TestEventEmitter(t *testing.T) {
	rec := &fakeEventRecorder{}
	emitter := NewEventEmitter(rec)
	obj := &configv1alpha1.ConfigPropagation{}
	sum := &summary.Summary{Actions: []summary.TargetAction{
		{Namespace: "a", Action: summary.ActionCreated, Reason: summary.ReasonApplied},
		{Namespace: "b", Action: summary.ActionUpdated, Reason: summary.ReasonApplied},
		{Namespace: "c", Action: summary.ActionSkipped, Reason: summary.ReasonConflictPolicy},
		{Namespace: "d", Action: summary.ActionPruned, Reason: summary.ReasonPruned},
	}}
	emitter.EmitSummary(obj, sum, "cfg")
	if len(rec.events) != 4 {
		t.Fatalf("expected 4 events, got %d", len(rec.events))
	}
	if rec.events[0].reason != "PropagationCreated" || rec.events[0].eventType != corev1.EventTypeNormal {
		t.Fatalf("unexpected first event: %+v", rec.events[0])
	}
	if rec.events[2].reason != "PropagationSkipped" {
		t.Fatalf("expected skip event, got %+v", rec.events[2])
	}
	emitter.EmitError(obj, fmt.Errorf("boom"))
	if len(rec.events) != 5 {
		t.Fatalf("expected error event appended, got %d", len(rec.events))
	}
	last := rec.events[len(rec.events)-1]
	if last.reason != "PropagationError" || last.eventType != corev1.EventTypeWarning {
		t.Fatalf("unexpected error event: %+v", last)
	}
}
