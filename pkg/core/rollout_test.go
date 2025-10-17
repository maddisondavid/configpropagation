package core

import "testing"

func TestRolloutPlannerRollingProgress(t *testing.T) {
	planner := NewRolloutPlanner()
	id := NamespacedName{Namespace: "ns", Name: "cp"}
	targets := []string{"a", "b", "c"}

	planned, completed := planner.Plan(id, "h1", StrategyRolling, 2, targets)
	if len(planned) != 2 || completed != 0 {
		t.Fatalf("expected first batch of 2, got planned=%v completed=%d", planned, completed)
	}
	if got := planner.MarkCompleted(id, "h1", planned); got != 2 {
		t.Fatalf("expected completed count 2, got %d", got)
	}

	planned, completed = planner.Plan(id, "h1", StrategyRolling, 2, targets)
	if len(planned) != 1 || planned[0] != "c" || completed != 2 {
		t.Fatalf("expected remaining target with completed=2, got planned=%v completed=%d", planned, completed)
	}
	if got := planner.MarkCompleted(id, "h1", planned); got != 3 {
		t.Fatalf("expected completed count 3, got %d", got)
	}

	planned, completed = planner.Plan(id, "h1", StrategyRolling, 2, targets)
	if len(planned) != 0 || completed != 3 {
		t.Fatalf("expected no pending targets, got planned=%v completed=%d", planned, completed)
	}
}

func TestRolloutPlannerHashChangeResets(t *testing.T) {
	planner := NewRolloutPlanner()
	id := NamespacedName{Namespace: "ns", Name: "cp"}
	targets := []string{"a", "b"}

	planned, _ := planner.Plan(id, "h1", StrategyRolling, 1, targets)
	planner.MarkCompleted(id, "h1", planned)

	planned, completed := planner.Plan(id, "h2", StrategyRolling, 2, targets)
	if len(planned) != 2 || completed != 0 {
		t.Fatalf("expected reset on hash change, got planned=%v completed=%d", planned, completed)
	}
}

func TestRolloutPlannerImmediateReturnsAll(t *testing.T) {
	planner := NewRolloutPlanner()
	id := NamespacedName{Namespace: "ns", Name: "cp"}
	targets := []string{"a", "b"}

	planned, completed := planner.Plan(id, "h1", StrategyImmediate, 5, targets)
	if len(planned) != 2 || completed != 2 {
		t.Fatalf("expected immediate to plan all, got planned=%v completed=%d", planned, completed)
	}

	if got := planner.MarkCompleted(id, "h1", nil); got != 0 {
		t.Fatalf("marking empty namespaces should not change state, got %d", got)
	}
}
