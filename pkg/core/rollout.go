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
	Synced         []string
	Failed         []OutOfSyncItem
	Warnings       []NamespaceWarning
	Retries        map[string]int
}

// RolloutPlanner tracks per-object rollout progress for rolling strategies.
type RolloutPlanner struct {
	mu     sync.Mutex
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
func (p *RolloutPlanner) Plan(id NamespacedName, desiredHash, strategy string, batchSize int32, targets []string) ([]string, int) {
	if strategy == StrategyImmediate {
		return append([]string(nil), targets...), len(targets)
	}
	if batchSize < 1 {
		batchSize = 1
	}
	p.mu.Lock()
	defer p.mu.Unlock()

	st := p.ensureStateLocked(id, desiredHash)

	allowed := make(map[string]struct{}, len(targets))
	for _, ns := range targets {
		allowed[ns] = struct{}{}
	}
	// Drop completed entries that are no longer selected so future plans include them if reselected.
	for ns := range st.completed {
		if _, ok := allowed[ns]; !ok {
			delete(st.completed, ns)
		}
	}

	planned := make([]string, 0, min(int(batchSize), len(targets)))
	for _, ns := range targets {
		if _, done := st.completed[ns]; done {
			continue
		}
		planned = append(planned, ns)
		if int32(len(planned)) >= batchSize {
			break
		}
	}
	return planned, len(st.completed)
}

// MarkCompleted records the provided namespaces as completed for the object and returns the updated completion count.
func (p *RolloutPlanner) MarkCompleted(id NamespacedName, desiredHash string, namespaces []string) int {
	if len(namespaces) == 0 {
		p.mu.Lock()
		defer p.mu.Unlock()
		if st, ok := p.states[id]; ok && st.hash == desiredHash {
			return len(st.completed)
		}
		return 0
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	st := p.ensureStateLocked(id, desiredHash)
	for _, ns := range namespaces {
		st.completed[ns] = struct{}{}
	}
	return len(st.completed)
}

// Forget removes any stored rollout state for the provided object.
func (p *RolloutPlanner) Forget(id NamespacedName) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.states, id)
}

func (p *RolloutPlanner) ensureStateLocked(id NamespacedName, desiredHash string) *rolloutState {
	st, ok := p.states[id]
	if !ok {
		st = &rolloutState{hash: desiredHash, completed: map[string]struct{}{}}
		p.states[id] = st
		return st
	}
	if st.hash != desiredHash {
		st.hash = desiredHash
		st.completed = map[string]struct{}{}
	}
	return st
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
