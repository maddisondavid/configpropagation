package status

import (
        "errors"
        "testing"
        "time"

        "configpropagation/pkg/agents/summary"
        "configpropagation/pkg/core"
)

func TestComputeReady(t *testing.T) {
        prev := core.ConfigPropagationStatus{}
        sum := &summary.Summary{Planned: []string{"a", "b"}}
        now := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
        status := Compute(prev, sum, nil, now)
        if status.TargetCount != 2 || status.SyncedCount != 2 || status.OutOfSyncCount != 0 {
                t.Fatalf("unexpected counters: %+v", status)
        }
        ready := findCondition(status.Conditions, core.CondReady)
        if ready.Status != "True" || ready.Reason != "Reconciled" {
                t.Fatalf("ready condition unexpected: %+v", ready)
        }
        if ready.LastTransitionTime != now.Format(time.RFC3339) {
                t.Fatalf("ready transition time incorrect: %s", ready.LastTransitionTime)
        }
        progressing := findCondition(status.Conditions, core.CondProgressing)
        if progressing.Status != "False" {
                t.Fatalf("progressing condition unexpected: %+v", progressing)
        }
        degraded := findCondition(status.Conditions, core.CondDegraded)
        if degraded.Status != "False" {
                t.Fatalf("degraded condition unexpected: %+v", degraded)
        }
}

func TestComputeProgressingOutOfSync(t *testing.T) {
        prev := core.ConfigPropagationStatus{}
        sum := &summary.Summary{Planned: []string{"a", "b"}, OutOfSync: []core.OutOfSyncItem{{Namespace: "b"}}}
        now := time.Date(2024, 2, 2, 0, 0, 0, 0, time.UTC)
        status := Compute(prev, sum, nil, now)
        ready := findCondition(status.Conditions, core.CondReady)
        if ready.Status != "False" || ready.Reason != "OutOfSync" {
                t.Fatalf("expected ready false, got %+v", ready)
        }
        progressing := findCondition(status.Conditions, core.CondProgressing)
        if progressing.Status != "True" {
                t.Fatalf("expected progressing true, got %+v", progressing)
        }
        if status.OutOfSyncCount != 1 || len(status.OutOfSync) != 1 {
                t.Fatalf("expected out-of-sync recorded: %+v", status)
        }
}

func TestComputeDegradedPreservesTransition(t *testing.T) {
        prev := core.ConfigPropagationStatus{Conditions: []core.Condition{{
                Type:               core.CondReady,
                Status:             "True",
                Reason:             "Reconciled",
                Message:            "ok",
                LastTransitionTime: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).Format(time.RFC3339),
        }}}
        err := errors.New("boom")
        now := time.Date(2024, 3, 3, 0, 0, 0, 0, time.UTC)
        status := Compute(prev, nil, err, now)
        ready := findCondition(status.Conditions, core.CondReady)
        if ready.Status != "False" || ready.Reason != "Error" {
                t.Fatalf("expected ready false error, got %+v", ready)
        }
        degraded := findCondition(status.Conditions, core.CondDegraded)
        if degraded.Status != "True" {
                t.Fatalf("expected degraded true, got %+v", degraded)
        }
        if ready.LastTransitionTime != now.Format(time.RFC3339) {
                t.Fatalf("expected new transition time on error")
        }
}

func TestComputeRetainsTransitionWhenUnchanged(t *testing.T) {
        prev := core.ConfigPropagationStatus{Conditions: []core.Condition{{
                Type:               core.CondReady,
                Status:             "True",
                Reason:             "Reconciled",
                Message:            "propagated to 1 namespaces",
                LastTransitionTime: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).Format(time.RFC3339),
        }}}
        sum := &summary.Summary{Planned: []string{"a"}}
        now := time.Date(2024, 1, 1, 1, 0, 0, 0, time.UTC)
        status := Compute(prev, sum, nil, now)
        ready := findCondition(status.Conditions, core.CondReady)
        if ready.LastTransitionTime != prev.Conditions[0].LastTransitionTime {
                        t.Fatalf("expected transition time to remain unchanged")
        }
}

func findCondition(conds []core.Condition, t string) core.Condition {
        for _, c := range conds {
                if c.Type == t {
                        return c
                }
        }
        return core.Condition{}
}

