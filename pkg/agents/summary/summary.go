package summary

import (
        "sort"

        "configpropagation/pkg/core"
)

// ActionType enumerates the type of action taken on a target namespace.
type ActionType string

// Action types emitted by the reconciler for observability.
const (
        ActionCreated ActionType = "created"
        ActionUpdated ActionType = "updated"
        ActionSkipped ActionType = "skipped"
        ActionPruned  ActionType = "pruned"
)

// Reason values describing why an action occurred.
const (
        ReasonApplied          = "Applied"
        ReasonAlreadySynced    = "AlreadySynced"
        ReasonForeignOwner     = "ForeignOwner"
        ReasonConflictPolicy   = "ConflictPolicySkip"
        ReasonPruned           = "Pruned"
        ReasonDetached         = "Detached"
)

// TargetAction captures a single action taken for a namespace during reconciliation.
type TargetAction struct {
        Namespace string
        Action    ActionType
        Reason    string
}

// Summary aggregates reconciliation outcomes for metrics, status, and events.
type Summary struct {
        Planned   []string
        Actions   []TargetAction
        OutOfSync []core.OutOfSyncItem
}

// Count returns the number of actions for the provided type.
func (s *Summary) Count(t ActionType) int {
        if s == nil {
                return 0
        }
        count := 0
        for _, a := range s.Actions {
                if a.Action == t {
                        count++
                }
        }
        return count
}

// OutOfSyncCount returns the number of out-of-sync entries.
func (s *Summary) OutOfSyncCount() int {
        if s == nil {
                return 0
        }
        return len(s.OutOfSync)
}

// SyncedCount returns the number of namespaces considered in sync.
func (s *Summary) SyncedCount() int {
        if s == nil {
                return 0
        }
        return len(s.Planned) - len(s.OutOfSync)
}

// SortedOutOfSync returns a copy of the out-of-sync slice ordered by namespace for determinism.
func (s *Summary) SortedOutOfSync() []core.OutOfSyncItem {
        if s == nil || len(s.OutOfSync) == 0 {
                        return nil
        }
        out := append([]core.OutOfSyncItem(nil), s.OutOfSync...)
        sort.Slice(out, func(i, j int) bool { return out[i].Namespace < out[j].Namespace })
        return out
}
