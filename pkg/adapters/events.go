package adapters

import (
        corev1 "k8s.io/api/core/v1"
        "k8s.io/client-go/tools/record"
        "sigs.k8s.io/controller-runtime/pkg/client"

        "configpropagation/pkg/agents/summary"
)

// EventEmitter wraps a Kubernetes EventRecorder to provide high level helpers.
type EventEmitter struct {
        recorder record.EventRecorder
}

// NewEventEmitter constructs an EventEmitter.
func NewEventEmitter(r record.EventRecorder) *EventEmitter {
        return &EventEmitter{recorder: r}
}

// EmitSummary emits events for each action in the summary.
func (e *EventEmitter) EmitSummary(obj client.Object, sum *summary.Summary, configName string) {
        if e == nil || e.recorder == nil || obj == nil || sum == nil {
                return
        }
        for _, action := range sum.Actions {
                switch action.Action {
                case summary.ActionCreated:
                        e.recorder.Eventf(obj, corev1.EventTypeNormal, "PropagationCreated", "Created ConfigMap %s in namespace %s", configName, action.Namespace)
                case summary.ActionUpdated:
                        e.recorder.Eventf(obj, corev1.EventTypeNormal, "PropagationUpdated", "Updated ConfigMap %s in namespace %s", configName, action.Namespace)
                case summary.ActionSkipped:
                        e.recorder.Eventf(obj, corev1.EventTypeNormal, "PropagationSkipped", "Skipped namespace %s: %s", action.Namespace, action.Reason)
                case summary.ActionPruned:
                        e.recorder.Eventf(obj, corev1.EventTypeNormal, "PropagationPruned", "Pruned namespace %s (%s)", action.Namespace, action.Reason)
                }
        }
        if sum.OutOfSyncCount() > 0 {
                e.recorder.Eventf(obj, corev1.EventTypeNormal, "PropagationOutOfSync", "%d namespaces out of sync", sum.OutOfSyncCount())
        }
}

// EmitError emits a warning event for reconciliation errors.
func (e *EventEmitter) EmitError(obj client.Object, err error) {
        if e == nil || e.recorder == nil || obj == nil || err == nil {
                return
        }
        e.recorder.Eventf(obj, corev1.EventTypeWarning, "PropagationError", "Reconcile failed: %v", err)
}

