package core

import (
	"fmt"
	"os"
	"strconv"
)

// ValidateSpec enforces basic guardrails that match the CRD schema.
func ValidateSpec(spec *ConfigPropagationSpec) error {
	if spec == nil {
		return fmt.Errorf("spec is required")
	}

	if spec.SourceRef.Namespace == "" || spec.SourceRef.Name == "" {
		return fmt.Errorf("sourceRef.namespace and sourceRef.name are required")
	}

	if spec.NamespaceSelector == nil {
		return fmt.Errorf("namespaceSelector is required")
	}

	if spec.Strategy != nil {
		if spec.Strategy.Type != "" && spec.Strategy.Type != StrategyRolling && spec.Strategy.Type != StrategyImmediate {
			return fmt.Errorf("invalid strategy.type: %s", spec.Strategy.Type)
		}

		if spec.Strategy.BatchSize != nil && *spec.Strategy.BatchSize < 1 {
			return fmt.Errorf("strategy.batchSize must be >= 1")
		}
	}

	if spec.ConflictPolicy != "" && spec.ConflictPolicy != ConflictOverwrite && spec.ConflictPolicy != ConflictSkip {
		return fmt.Errorf("invalid conflictPolicy: %s", spec.ConflictPolicy)
	}

	if spec.ResyncPeriodSeconds != nil && *spec.ResyncPeriodSeconds < 10 {
		return fmt.Errorf("resyncPeriodSeconds must be >= 10")
	}

	return nil
}

// DefaultSpec applies safe defaults consistent with CRD defaults.
func DefaultSpec(spec *ConfigPropagationSpec) {
	if spec.Strategy == nil {
		spec.Strategy = &UpdateStrategy{}
	}

	if spec.Strategy.Type == "" {
		spec.Strategy.Type = StrategyRolling
	}

	if spec.Strategy.BatchSize == nil {
		defaultValue := defaultBatchSize()
		spec.Strategy.BatchSize = &defaultValue
	}

	if spec.ConflictPolicy == "" {
		spec.ConflictPolicy = ConflictOverwrite
	}

	if spec.Prune == nil {
		shouldPrune := true
		spec.Prune = &shouldPrune
	}
}

// defaultBatchSize determines the rollout batch size from environment defaults.
func defaultBatchSize() int32 {
	if environmentValue := os.Getenv("BATCH_SIZE"); environmentValue != "" {
		if parsed, err := strconv.Atoi(environmentValue); err == nil && parsed >= 1 {
			return int32(parsed)
		}
	}

	return 5
}
