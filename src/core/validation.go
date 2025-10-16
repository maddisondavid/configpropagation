package core

import "fmt"

// ValidateSpec enforces basic guardrails that match the CRD schema.
func ValidateSpec(s *ConfigPropagationSpec) error {
	if s == nil {
		return fmt.Errorf("spec is required")
	}
	if s.SourceRef.Namespace == "" || s.SourceRef.Name == "" {
		return fmt.Errorf("sourceRef.namespace and sourceRef.name are required")
	}
	if s.NamespaceSelector == nil {
		return fmt.Errorf("namespaceSelector is required")
	}
	if s.Strategy != nil {
		if s.Strategy.Type != "" && s.Strategy.Type != StrategyRolling && s.Strategy.Type != StrategyImmediate {
			return fmt.Errorf("invalid strategy.type: %s", s.Strategy.Type)
		}
		if s.Strategy.BatchSize != nil && *s.Strategy.BatchSize < 1 {
			return fmt.Errorf("strategy.batchSize must be >= 1")
		}
	}
	if s.ConflictPolicy != "" && s.ConflictPolicy != ConflictOverwrite && s.ConflictPolicy != ConflictSkip {
		return fmt.Errorf("invalid conflictPolicy: %s", s.ConflictPolicy)
	}
	if s.ResyncPeriodSeconds != nil && *s.ResyncPeriodSeconds < 10 {
		return fmt.Errorf("resyncPeriodSeconds must be >= 10")
	}
	return nil
}

// DefaultSpec applies safe defaults consistent with CRD defaults.
func DefaultSpec(s *ConfigPropagationSpec) {
	if s.Strategy == nil {
		s.Strategy = &UpdateStrategy{}
	}
	if s.Strategy.Type == "" {
		s.Strategy.Type = StrategyRolling
	}
	if s.Strategy.BatchSize == nil {
		var d int32 = 5
		s.Strategy.BatchSize = &d
	}
	if s.ConflictPolicy == "" {
		s.ConflictPolicy = ConflictOverwrite
	}
	if s.Prune == nil {
		b := true
		s.Prune = &b
	}
}
