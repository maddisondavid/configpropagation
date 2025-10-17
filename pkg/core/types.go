package core

// ConfigPropagationSpec models the desired state of propagation.
type ConfigPropagationSpec struct {
	SourceRef           ObjectRef       `json:"sourceRef"`
	NamespaceSelector   *LabelSelector  `json:"namespaceSelector"`
	DataKeys            []string        `json:"dataKeys,omitempty"`
	Strategy            *UpdateStrategy `json:"strategy,omitempty"`
	ConflictPolicy      string          `json:"conflictPolicy,omitempty"`
	Prune               *bool           `json:"prune,omitempty"`
	ResyncPeriodSeconds *int32          `json:"resyncPeriodSeconds,omitempty"`
}

// ObjectRef references a namespaced object (ConfigMap source).
type ObjectRef struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

// LabelSelector matches namespaces by labels.
type LabelSelector struct {
	MatchLabels      map[string]string  `json:"matchLabels,omitempty"`
	MatchExpressions []LabelSelectorReq `json:"matchExpressions,omitempty"`
}

// LabelSelectorReq models a single selector requirement.
type LabelSelectorReq struct {
	Key      string   `json:"key"`
	Operator string   `json:"operator"` // In, NotIn, Exists, DoesNotExist
	Values   []string `json:"values,omitempty"`
}

// UpdateStrategy configures rollout behavior.
type UpdateStrategy struct {
	Type      string `json:"type,omitempty"`      // rolling|immediate
	BatchSize *int32 `json:"batchSize,omitempty"` // >=1, default 5
}

// ConfigPropagationStatus reports controller state.
type ConfigPropagationStatus struct {
	Conditions     []Condition     `json:"conditions,omitempty"`
	TargetCount    int32           `json:"targetCount,omitempty"`
	SyncedCount    int32           `json:"syncedCount,omitempty"`
	OutOfSyncCount int32           `json:"outOfSyncCount,omitempty"`
	OutOfSync      []OutOfSyncItem `json:"outOfSync,omitempty"`
	LastSyncTime   string          `json:"lastSyncTime,omitempty"` // RFC3339
}

// Condition is a standard status condition.
type Condition struct {
	Type               string `json:"type"`
	Status             string `json:"status"` // True|False|Unknown
	Reason             string `json:"reason,omitempty"`
	Message            string `json:"message,omitempty"`
	LastTransitionTime string `json:"lastTransitionTime,omitempty"`
}

// OutOfSyncItem gives details about a non-synced namespace.
type OutOfSyncItem struct {
	Namespace string `json:"namespace"`
	Reason    string `json:"reason"`
	Message   string `json:"message,omitempty"`
}
