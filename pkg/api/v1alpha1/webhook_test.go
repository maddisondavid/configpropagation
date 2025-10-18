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
	if len(cp.Status.Conditions) != 3 {
		t.Fatalf("expected three conditions, got %+v", cp.Status.Conditions)
	}
	ready := findCondition(cp.Status.Conditions, core.CondReady)
	progressing := findCondition(cp.Status.Conditions, core.CondProgressing)
	degraded := findCondition(cp.Status.Conditions, core.CondDegraded)
	if ready == nil || ready.Status != "True" || ready.Reason != "Reconciled" {
		t.Fatalf("expected Ready=True/Reconciled, got %+v", ready)
	}
	if progressing == nil || progressing.Status != "False" {
		t.Fatalf("expected Progressing False, got %+v", progressing)
	}
	if degraded == nil || degraded.Status != "False" {
		t.Fatalf("expected Degraded False, got %+v", degraded)
	}
}

func TestApplyRolloutStatusRolling(t *testing.T) {
	cp := &ConfigPropagation{}
	result := core.RolloutResult{Planned: []string{"batch"}, TotalTargets: 5, CompletedCount: 2}
	cp.ApplyRolloutStatus(result)
	if cp.Status.TargetCount != 5 || cp.Status.SyncedCount != 2 || cp.Status.OutOfSyncCount != 3 {
		t.Fatalf("unexpected status counters for rolling: %+v", cp.Status)
	}
	ready := findCondition(cp.Status.Conditions, core.CondReady)
	progressing := findCondition(cp.Status.Conditions, core.CondProgressing)
	degraded := findCondition(cp.Status.Conditions, core.CondDegraded)
	if ready == nil || ready.Status != "False" || ready.Reason != "RollingUpdate" {
		t.Fatalf("expected Ready False RollingUpdate, got %+v", ready)
	}
	if progressing == nil || progressing.Status != "True" {
		t.Fatalf("expected Progressing True, got %+v", progressing)
	}
	if degraded == nil || degraded.Status != "False" {
		t.Fatalf("expected Degraded False during rolling, got %+v", degraded)
	}
}

func TestApplyRolloutStatusOutOfSync(t *testing.T) {
	cp := &ConfigPropagation{}
	result := core.RolloutResult{
		Planned:        []string{},
		TotalTargets:   3,
		CompletedCount: 1,
		OutOfSync: []core.OutOfSyncItem{{
			Namespace: "ns2",
			Reason:    "ConflictSkipped",
		}},
	}
	cp.ApplyRolloutStatus(result)
	if cp.Status.OutOfSyncCount != 2 {
		t.Fatalf("expected two out-of-sync targets, got %+v", cp.Status.OutOfSyncCount)
	}
	if len(cp.Status.OutOfSync) != 1 || cp.Status.OutOfSync[0].Namespace != "ns2" {
		t.Fatalf("expected out-of-sync entries to be copied, got %+v", cp.Status.OutOfSync)
	}
	ready := findCondition(cp.Status.Conditions, core.CondReady)
	progressing := findCondition(cp.Status.Conditions, core.CondProgressing)
	degraded := findCondition(cp.Status.Conditions, core.CondDegraded)
	if ready == nil || ready.Status != "False" || ready.Reason != "OutOfSync" {
		t.Fatalf("expected Ready False OutOfSync, got %+v", ready)
	}
	if progressing == nil || progressing.Status != "False" || progressing.Reason != "Blocked" {
		t.Fatalf("expected Progressing False Blocked, got %+v", progressing)
	}
	if degraded == nil || degraded.Status != "True" {
		t.Fatalf("expected Degraded True, got %+v", degraded)
	}
}

func findCondition(conditions []core.Condition, conditionType string) *core.Condition {
	for index := range conditions {
		condition := &conditions[index]
		if condition.Type == conditionType {
			return condition
		}
	}
	return nil
}
