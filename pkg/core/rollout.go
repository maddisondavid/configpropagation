package core

import "sync"

// NamespacedName identifies a namespaced Kubernetes resource.
type NamespacedName struct {
	Namespace string
	Name      string
}

// RolloutResult captures the outcome of a reconciliation loop with rollout progress.
type RolloutResult struct {
	Planned        []string
	TotalTargets   int
	CompletedCount int
}

// RolloutPlanner tracks per-object rollout progress for rolling strategies.
type RolloutPlanner struct {
	mutex  sync.Mutex
	states map[NamespacedName]*rolloutState
}

type rolloutState struct {
	hash      string
	completed map[string]struct{}
}

// NewRolloutPlanner constructs an empty planner.
func NewRolloutPlanner() *RolloutPlanner {
	return &RolloutPlanner{states: map[NamespacedName]*rolloutState{}}
}

// Plan determines the next batch of namespaces to update given the desired targets and strategy.
// It returns the namespaces to process now and the count of namespaces already completed prior to this plan.
func (planner *RolloutPlanner) Plan(identifier NamespacedName, desiredHash, strategy string, batchSize int32, targets []string) ([]string, int) {
	if strategy == StrategyImmediate {
		return append([]string(nil), targets...), len(targets)
	}

	if batchSize < 1 {
		batchSize = 1
	}

	planner.mutex.Lock()
	defer planner.mutex.Unlock()

	state := planner.ensureStateLocked(identifier, desiredHash)

	allowedTargets := make(map[string]struct{}, len(targets))

	for _, namespace := range targets {
		allowedTargets[namespace] = struct{}{}
	}

	// Drop completed entries that are no longer selected so future plans include them if reselected.
	for namespace := range state.completed {
		if _, exists := allowedTargets[namespace]; !exists {
			delete(state.completed, namespace)
		}
	}

	plannedTargets := make([]string, 0, min(int(batchSize), len(targets)))

	for _, namespace := range targets {
		if _, alreadyCompleted := state.completed[namespace]; alreadyCompleted {
			continue
		}

		plannedTargets = append(plannedTargets, namespace)

		if int32(len(plannedTargets)) >= batchSize {
			break
		}
	}

	return plannedTargets, len(state.completed)
}

// MarkCompleted records the provided namespaces as completed for the object and returns the updated completion count.
func (planner *RolloutPlanner) MarkCompleted(identifier NamespacedName, desiredHash string, namespaces []string) int {
	if len(namespaces) == 0 {
		planner.mutex.Lock()
		defer planner.mutex.Unlock()

		if state, exists := planner.states[identifier]; exists && state.hash == desiredHash {
			return len(state.completed)
		}

		return 0
	}

	planner.mutex.Lock()
	defer planner.mutex.Unlock()

	state := planner.ensureStateLocked(identifier, desiredHash)

	for _, namespace := range namespaces {
		state.completed[namespace] = struct{}{}
	}

	return len(state.completed)
}

// Forget removes any stored rollout state for the provided object.
func (planner *RolloutPlanner) Forget(identifier NamespacedName) {
	planner.mutex.Lock()
	defer planner.mutex.Unlock()

	delete(planner.states, identifier)
}

func (planner *RolloutPlanner) ensureStateLocked(identifier NamespacedName, desiredHash string) *rolloutState {
	state, exists := planner.states[identifier]

	if !exists {
		state = &rolloutState{hash: desiredHash, completed: map[string]struct{}{}}
		planner.states[identifier] = state

		return state
	}

	if state.hash != desiredHash {
		state.hash = desiredHash
		state.completed = map[string]struct{}{}
	}

	return state
}

func min(firstValue, secondValue int) int {
	if firstValue < secondValue {
		return firstValue
	}

	return secondValue
}
