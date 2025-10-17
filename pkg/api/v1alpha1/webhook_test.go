package v1alpha1

import (
	"testing"

	"configpropagation/pkg/core"
)

func TestApplyRolloutStatusImmediate(t *testing.T) {
	cp := &ConfigPropagation{}
	result := core.RolloutResult{Planned: []string{"a", "b"}, TotalTargets: 2, CompletedCount: 2}
	cp.ApplyRolloutStatus(result)
	if cp.Status.TargetCount != 2 || cp.Status.SyncedCount != 2 || cp.Status.OutOfSyncCount != 0 {
		t.Fatalf("unexpected status counters: %+v", cp.Status)
	}
	if len(cp.Status.Conditions) != 1 || cp.Status.Conditions[0].Status != "True" || cp.Status.Conditions[0].Reason != "Reconciled" {
		t.Fatalf("expected Ready=True Reconciled condition, got %+v", cp.Status.Conditions)
	}
}

func TestApplyRolloutStatusRolling(t *testing.T) {
	cp := &ConfigPropagation{}
	result := core.RolloutResult{Planned: []string{"batch"}, TotalTargets: 5, CompletedCount: 2}
	cp.ApplyRolloutStatus(result)
	if cp.Status.TargetCount != 5 || cp.Status.SyncedCount != 2 || cp.Status.OutOfSyncCount != 3 {
		t.Fatalf("unexpected status counters for rolling: %+v", cp.Status)
	}
	if len(cp.Status.Conditions) != 1 {
		t.Fatalf("expected a single condition")
	}
	cond := cp.Status.Conditions[0]
	if cond.Status != "False" || cond.Reason != "RollingUpdate" {
		t.Fatalf("expected RollingUpdate condition, got %+v", cond)
	}
}
