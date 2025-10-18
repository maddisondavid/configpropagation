package v1alpha1

import (
	"testing"

	"configpropagation/pkg/core"
)

func TestApplyRolloutStatusImmediate(t *testing.T) {
	cp := &ConfigPropagation{}
	result := core.RolloutResult{Planned: []string{"a", "b"}, TotalTargets: 2, CompletedTargets: []string{"a", "b"}, CompletedCount: 2}
	cp.ApplyRolloutStatus(result)
	if cp.Status.TargetCount != 2 || cp.Status.SyncedCount != 2 || cp.Status.OutOfSyncCount != 0 {
		t.Fatalf("unexpected status counters: %+v", cp.Status)
	}
	if len(cp.Status.Conditions) != 3 {
		t.Fatalf("expected three conditions, got %+v", cp.Status.Conditions)
	}
	ready := cp.Status.Conditions[0]
	if ready.Type != core.CondReady || ready.Status != "True" || ready.Reason != "AllSynced" {
		t.Fatalf("expected Ready=True AllSynced, got %+v", ready)
	}
	progressing := cp.Status.Conditions[1]
	if progressing.Type != core.CondProgressing || progressing.Status != "False" || progressing.Reason != "Completed" {
		t.Fatalf("expected Progressing False Completed, got %+v", progressing)
	}
	degraded := cp.Status.Conditions[2]
	if degraded.Type != core.CondDegraded || degraded.Status != "False" || degraded.Reason != "NoIssues" {
		t.Fatalf("expected Degraded False NoIssues, got %+v", degraded)
	}
}

func TestApplyRolloutStatusRolling(t *testing.T) {
	cp := &ConfigPropagation{}
	result := core.RolloutResult{Planned: []string{"batch"}, TotalTargets: 5, CompletedTargets: []string{"ns1", "ns2"}, CompletedCount: 2}
	cp.ApplyRolloutStatus(result)
	if cp.Status.TargetCount != 5 || cp.Status.SyncedCount != 2 || cp.Status.OutOfSyncCount != 3 {
		t.Fatalf("unexpected status counters for rolling: %+v", cp.Status)
	}
	ready := cp.Status.Conditions[0]
	if ready.Status != "False" || ready.Reason != "InProgress" {
		t.Fatalf("expected Ready False InProgress, got %+v", ready)
	}
	progressing := cp.Status.Conditions[1]
	if progressing.Status != "True" || progressing.Reason != "RollingUpdate" {
		t.Fatalf("expected Progressing True RollingUpdate, got %+v", progressing)
	}
	degraded := cp.Status.Conditions[2]
	if degraded.Status != "False" || degraded.Reason != "NoIssues" {
		t.Fatalf("expected Degraded False NoIssues, got %+v", degraded)
	}
}

func TestApplyRolloutStatusOutOfSync(t *testing.T) {
	cp := &ConfigPropagation{}
	result := core.RolloutResult{
		TotalTargets:     3,
		CompletedTargets: []string{"ns1"},
		CompletedCount:   1,
		SkippedTargets:   []core.OutOfSyncItem{{Namespace: "ns2", Reason: "Conflict"}},
	}
	cp.ApplyRolloutStatus(result)
	if cp.Status.OutOfSyncCount != 2 || len(cp.Status.OutOfSync) != 1 {
		t.Fatalf("expected pending count 2 with one detail, got %+v", cp.Status)
	}
	ready := cp.Status.Conditions[0]
	if ready.Status != "False" || ready.Reason != "OutOfSync" {
		t.Fatalf("expected Ready False OutOfSync, got %+v", ready)
	}
	progressing := cp.Status.Conditions[1]
	if progressing.Status != "False" || progressing.Reason != "Blocked" {
		t.Fatalf("expected Progressing False Blocked, got %+v", progressing)
	}
	degraded := cp.Status.Conditions[2]
	if degraded.Status != "True" || degraded.Reason != "OutOfSync" {
		t.Fatalf("expected Degraded True OutOfSync, got %+v", degraded)
	}
}
