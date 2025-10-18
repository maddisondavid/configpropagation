package events

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Recorder wraps a controller-runtime EventRecorder with helper methods
// specific to ConfigPropagation reconciliation.
//
// The helper methods guard against nil receivers so tests can pass a nil
// recorder when event emission is not under test.
type Recorder struct {
	recorder record.EventRecorder
}

// NewRecorder constructs a Recorder from the provided controller-runtime EventRecorder.
func NewRecorder(rec record.EventRecorder) *Recorder {
	return &Recorder{recorder: rec}
}

// TargetCreated records an event indicating the target ConfigMap was created.
func (r *Recorder) TargetCreated(obj client.Object, namespace, name string) {
	if r == nil || r.recorder == nil {
		return
	}
	r.recorder.Eventf(obj, corev1.EventTypeNormal, "TargetCreated", "ConfigMap %s/%s created", namespace, name)
}

// TargetUpdated records an event indicating the target ConfigMap was updated.
func (r *Recorder) TargetUpdated(obj client.Object, namespace, name string) {
	if r == nil || r.recorder == nil {
		return
	}
	r.recorder.Eventf(obj, corev1.EventTypeNormal, "TargetUpdated", "ConfigMap %s/%s updated", namespace, name)
}

// TargetSkipped records an event indicating the target was skipped.
func (r *Recorder) TargetSkipped(obj client.Object, namespace, name, reason, message string) {
	if r == nil || r.recorder == nil {
		return
	}
	r.recorder.Eventf(obj, corev1.EventTypeWarning, reason, "%s/%s skipped: %s", namespace, name, message)
}

// TargetPruned records an event indicating a previously managed target was pruned.
func (r *Recorder) TargetPruned(obj client.Object, namespace, name string) {
	if r == nil || r.recorder == nil {
		return
	}
	r.recorder.Eventf(obj, corev1.EventTypeNormal, "TargetPruned", "ConfigMap %s/%s pruned", namespace, name)
}

// TargetDetached records an event indicating a previously managed target was detached.
func (r *Recorder) TargetDetached(obj client.Object, namespace, name string) {
	if r == nil || r.recorder == nil {
		return
	}
	r.recorder.Eventf(obj, corev1.EventTypeNormal, "TargetDetached", "ConfigMap %s/%s detached", namespace, name)
}

// Error records an event indicating reconciliation failed.
func (r *Recorder) Error(obj client.Object, err error) {
	if r == nil || r.recorder == nil || err == nil {
		return
	}
	r.recorder.Eventf(obj, corev1.EventTypeWarning, "ReconcileError", "reconciliation error: %v", err)
}
