package v1alpha1

import (
	"fmt"
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
	ready := conditionByType(t, cp.Status.Conditions, core.CondReady)
	if ready.Status != "True" || ready.Reason != "Reconciled" {
		t.Fatalf("expected Ready True/Reconciled, got %+v", ready)
	}
	progressing := conditionByType(t, cp.Status.Conditions, core.CondProgressing)
	if progressing.Status != "False" || progressing.Reason != "RolloutComplete" {
		t.Fatalf("expected Progressing False/RolloutComplete, got %+v", progressing)
	}
	degraded := conditionByType(t, cp.Status.Conditions, core.CondDegraded)
	if degraded.Status != "False" || degraded.Reason != "Healthy" {
		t.Fatalf("expected Degraded False/Healthy, got %+v", degraded)
	}
}

func TestApplyRolloutStatusRolling(t *testing.T) {
	cp := &ConfigPropagation{}
	result := core.RolloutResult{Planned: []string{"batch"}, TotalTargets: 5, CompletedCount: 2}
	cp.ApplyRolloutStatus(result)
	if cp.Status.TargetCount != 5 || cp.Status.SyncedCount != 2 || cp.Status.OutOfSyncCount != 3 {
		t.Fatalf("unexpected status counters for rolling: %+v", cp.Status)
	}
	ready := conditionByType(t, cp.Status.Conditions, core.CondReady)
	if ready.Status != "False" || ready.Reason != "RollingUpdate" {
		t.Fatalf("expected Ready False/RollingUpdate, got %+v", ready)
	}
	progressing := conditionByType(t, cp.Status.Conditions, core.CondProgressing)
	if progressing.Status != "True" || progressing.Reason != "RollingUpdate" {
		t.Fatalf("expected Progressing True/RollingUpdate, got %+v", progressing)
	}
	degraded := conditionByType(t, cp.Status.Conditions, core.CondDegraded)
	if degraded.Status != "False" || degraded.Reason != "Healthy" {
		t.Fatalf("expected Degraded False/Healthy, got %+v", degraded)
	}
}

func TestApplyErrorStatusSetsDegraded(t *testing.T) {
	cp := &ConfigPropagation{}
	cp.ApplyErrorStatus(fmt.Errorf("boom"))
	if len(cp.Status.Conditions) != 3 {
		t.Fatalf("expected three conditions for error, got %+v", cp.Status.Conditions)
	}
	ready := conditionByType(t, cp.Status.Conditions, core.CondReady)
	if ready.Status != "False" || ready.Reason != "Error" {
		t.Fatalf("expected Ready False/Error, got %+v", ready)
	}
	progressing := conditionByType(t, cp.Status.Conditions, core.CondProgressing)
	if progressing.Status != "False" || progressing.Reason != "Error" {
		t.Fatalf("expected Progressing False/Error, got %+v", progressing)
	}
	degraded := conditionByType(t, cp.Status.Conditions, core.CondDegraded)
	if degraded.Status != "True" || degraded.Reason != "ReconcileError" {
		t.Fatalf("expected Degraded True/ReconcileError, got %+v", degraded)
	}
	if degraded.Message != "boom" {
		t.Fatalf("expected degraded message to include error, got %q", degraded.Message)
	}
}

func conditionByType(t *testing.T, conditions []core.Condition, conditionType string) core.Condition {
	t.Helper()
	for _, condition := range conditions {
		if condition.Type == conditionType {
			return condition
		}
	}
	t.Fatalf("condition %s not found in %+v", conditionType, conditions)
	return core.Condition{}
}
