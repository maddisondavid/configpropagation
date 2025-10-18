package v1alpha1

import (
	"fmt"
	"time"

	"configpropagation/pkg/core"
)

// ApplyRolloutStatus updates status fields after a reconcile using rollout progress.
func (c *ConfigPropagation) ApplyRolloutStatus(result core.RolloutResult) {
	now := time.Now().UTC().Format(time.RFC3339)
	c.Status.LastSyncTime = now
	c.Status.TargetCount = int32(result.TotalTargets)
	synced := result.CompletedCount
	if synced < 0 {
		synced = 0
	}
	if synced > result.TotalTargets {
		synced = result.TotalTargets
	}
	c.Status.SyncedCount = int32(synced)
	pending := result.TotalTargets - synced
	if pending < 0 {
		pending = 0
	}
	c.Status.OutOfSyncCount = int32(pending)
	if len(result.SkippedTargets) > 0 {
		c.Status.OutOfSync = append([]core.OutOfSyncItem(nil), result.SkippedTargets...)
	} else {
		c.Status.OutOfSync = nil
	}
	c.Status.Conditions = computeConditions(now, result, synced)
}

func computeConditions(now string, result core.RolloutResult, synced int) []core.Condition {
	total := result.TotalTargets
	outOfSync := len(result.SkippedTargets)
	pending := total - synced
	if pending < 0 {
		pending = 0
	}

	ready := core.Condition{Type: core.CondReady, LastTransitionTime: now}
	switch {
	case total == 0:
		ready.Status = "True"
		ready.Reason = "NoTargets"
		ready.Message = "no target namespaces selected"
	case outOfSync > 0:
		ready.Status = "False"
		ready.Reason = "OutOfSync"
		ready.Message = fmt.Sprintf("%d targets out of sync", outOfSync)
	case pending == 0:
		ready.Status = "True"
		ready.Reason = "AllSynced"
		ready.Message = fmt.Sprintf("propagated to %d/%d namespaces", synced, total)
	default:
		ready.Status = "False"
		ready.Reason = "InProgress"
		ready.Message = fmt.Sprintf("propagated to %d/%d namespaces", synced, total)
	}

	progressing := core.Condition{Type: core.CondProgressing, LastTransitionTime: now}
	switch {
	case outOfSync > 0:
		progressing.Status = "False"
		progressing.Reason = "Blocked"
		progressing.Message = fmt.Sprintf("%d targets blocked by conflicts", outOfSync)
	case pending > 0:
		progressing.Status = "True"
		progressing.Reason = "RollingUpdate"
		if batch := len(result.Planned); batch > 0 {
			progressing.Message = fmt.Sprintf("processing batch of %d (synced %d/%d)", batch, synced, total)
		} else {
			progressing.Message = fmt.Sprintf("waiting for rollout completion (%d/%d)", synced, total)
		}
	default:
		progressing.Status = "False"
		progressing.Reason = "Completed"
		progressing.Message = "all targets synced"
	}

	degraded := core.Condition{Type: core.CondDegraded, LastTransitionTime: now}
	if outOfSync > 0 {
		degraded.Status = "True"
		degraded.Reason = "OutOfSync"
		degraded.Message = fmt.Sprintf("%d targets skipped", outOfSync)
	} else {
		degraded.Status = "False"
		degraded.Reason = "NoIssues"
		degraded.Message = "no drift detected"
	}

	return []core.Condition{ready, progressing, degraded}
}
