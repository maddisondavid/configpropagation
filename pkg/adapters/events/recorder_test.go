package events

import (
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	configv1alpha1 "configpropagation/pkg/api/v1alpha1"
)

type fakeEventRecorder struct {
	events []struct {
		eventType string
		reason    string
		message   string
	}
}

func (f *fakeEventRecorder) Event(object runtime.Object, eventtype, reason, message string) {}

func (f *fakeEventRecorder) Eventf(object runtime.Object, eventtype, reason, messageFmt string, args ...interface{}) {
	f.events = append(f.events, struct {
		eventType string
		reason    string
		message   string
	}{eventType: eventtype, reason: reason, message: sprintf(messageFmt, args...)})
}

func (f *fakeEventRecorder) AnnotatedEventf(object runtime.Object, annotations map[string]string, eventtype, reason, messageFmt string, args ...interface{}) {
}

func sprintf(format string, args ...interface{}) string {
	return fmt.Sprintf(format, args...)
}

func TestRecorderHelpers(t *testing.T) {
	fake := &fakeEventRecorder{}
	rec := NewRecorder(fake)
	obj := &configv1alpha1.ConfigPropagation{ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "cp"}}

	rec.TargetCreated(obj, "ns-a", "cfg")
	rec.TargetUpdated(obj, "ns-b", "cfg")
	rec.TargetSkipped(obj, "ns-c", "cfg", "Conflict", "skipped")
	rec.TargetPruned(obj, "ns-d", "cfg")
	rec.TargetDetached(obj, "ns-e", "cfg")
	rec.Error(obj, fmt.Errorf("boom"))

	if len(fake.events) != 6 {
		t.Fatalf("expected 6 events, got %d", len(fake.events))
	}
	if fake.events[0].reason != "TargetCreated" || fake.events[0].eventType != corev1.EventTypeNormal {
		t.Fatalf("unexpected create event: %+v", fake.events[0])
	}
	if fake.events[2].eventType != corev1.EventTypeWarning || fake.events[2].reason != "Conflict" {
		t.Fatalf("expected warning skip event, got %+v", fake.events[2])
	}
	if fake.events[4].reason != "TargetDetached" || fake.events[4].eventType != corev1.EventTypeNormal {
		t.Fatalf("expected detached event, got %+v", fake.events[4])
	}
	if fake.events[5].reason != "ReconcileError" {
		t.Fatalf("expected error reason, got %+v", fake.events[5])
	}
}

func TestRecorderNilSafe(t *testing.T) {
	var rec *Recorder
	rec.TargetCreated(nil, "ns", "name")
	// ensure no panic
}
