package status

import (
        "fmt"
        "time"

        "configpropagation/pkg/agents/summary"
        "configpropagation/pkg/core"
)

// Compute builds a ConfigPropagationStatus from the provided summary and error.
func Compute(previous core.ConfigPropagationStatus, sum *summary.Summary, reconcileErr error, now time.Time) core.ConfigPropagationStatus {
        status := previous
        timestamp := now.UTC().Format(time.RFC3339)
        status.LastSyncTime = timestamp
        if sum != nil {
                status.TargetCount = int32(len(sum.Planned))
                status.OutOfSyncCount = int32(sum.OutOfSyncCount())
                status.SyncedCount = int32(sum.SyncedCount())
                status.OutOfSync = sum.SortedOutOfSync()
        }
        status.Conditions = mergeConditions(previous.Conditions, desiredConditions(sum, reconcileErr, timestamp))
        return status
}

func desiredConditions(sum *summary.Summary, reconcileErr error, timestamp string) map[string]core.Condition {
        ready := core.Condition{Type: core.CondReady, Status: "False", Reason: "Reconciling", Message: "waiting for reconciliation", LastTransitionTime: timestamp}
        progressing := core.Condition{Type: core.CondProgressing, Status: "False", Reason: "Idle", Message: "no pending work", LastTransitionTime: timestamp}
        degraded := core.Condition{Type: core.CondDegraded, Status: "False", Reason: "Healthy", Message: "no errors", LastTransitionTime: timestamp}

        switch {
        case reconcileErr != nil:
                ready.Status = "False"
                ready.Reason = "Error"
                ready.Message = fmt.Sprintf("reconciliation failed: %v", reconcileErr)
                degraded.Status = "True"
                degraded.Reason = "Error"
                degraded.Message = fmt.Sprintf("reconciliation failed: %v", reconcileErr)
                progressing.Status = "False"
                progressing.Reason = "Error"
                progressing.Message = "paused due to error"
        case sum != nil && sum.OutOfSyncCount() > 0:
                ready.Status = "False"
                ready.Reason = "OutOfSync"
                ready.Message = fmt.Sprintf("%d namespaces out of sync", sum.OutOfSyncCount())
                progressing.Status = "True"
                progressing.Reason = "OutOfSync"
                progressing.Message = fmt.Sprintf("reconciling %d namespaces", sum.OutOfSyncCount())
                degraded.Status = "False"
                degraded.Reason = "OutOfSync"
                degraded.Message = "waiting for propagation"
        default:
                ready.Status = "True"
                ready.Reason = "Reconciled"
                if sum != nil {
                        ready.Message = fmt.Sprintf("propagated to %d namespaces", len(sum.Planned))
                } else {
                        ready.Message = "propagation succeeded"
                }
                progressing.Status = "False"
                progressing.Reason = "Reconciled"
                progressing.Message = "all namespaces in sync"
                degraded.Status = "False"
                degraded.Reason = "Healthy"
                degraded.Message = "no errors"
        }

        return map[string]core.Condition{
                core.CondReady:       ready,
                core.CondProgressing: progressing,
                core.CondDegraded:    degraded,
        }
}

func mergeConditions(previous []core.Condition, desired map[string]core.Condition) []core.Condition {
        byType := map[string]core.Condition{}
        for _, cond := range previous {
                byType[cond.Type] = cond
        }
        result := make([]core.Condition, 0, len(desired))
        for _, cond := range desired {
                if prev, ok := byType[cond.Type]; ok {
                        if prev.Status == cond.Status && prev.Reason == cond.Reason && prev.Message == cond.Message {
                                cond.LastTransitionTime = prev.LastTransitionTime
                        }
                }
                result = append(result, cond)
        }
        return result
}

