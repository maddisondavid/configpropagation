package webhooks

import (
	"fmt"
	"os"

	core "configpropagation/src/core"
)

const (
	// strictSelectorEnv toggles an additional guardrail that rejects wide-open
	// namespace selectors (those without any label constraints).
	strictSelectorEnv = "STRICT_SELECTOR_GUARD"
	// immutableSourceEnv toggles immutability enforcement for the sourceRef
	// field on updates.
	immutableSourceEnv = "ENFORCE_SOURCE_IMMUTABILITY"
)

// ValidateConfigPropagation evaluates the new spec against validation rules and
// optional policy guardrails. The old spec should be provided for update
// operations; pass nil on create.
func ValidateConfigPropagation(newSpec, oldSpec *core.ConfigPropagationSpec) error {
	if err := core.ValidateSpec(newSpec); err != nil {
		return err
	}

	if parseBoolEnv(os.Getenv(strictSelectorEnv)) {
		if isSelectorWideOpen(newSpec.NamespaceSelector) {
			return fmt.Errorf("namespaceSelector must specify matchLabels or matchExpressions when %s is enabled", strictSelectorEnv)
		}
	}

	if parseBoolEnv(os.Getenv(immutableSourceEnv)) && oldSpec != nil {
		if oldSpec.SourceRef.Namespace != newSpec.SourceRef.Namespace || oldSpec.SourceRef.Name != newSpec.SourceRef.Name {
			return fmt.Errorf("sourceRef is immutable when %s is enabled", immutableSourceEnv)
		}
	}

	return nil
}

func isSelectorWideOpen(sel *core.LabelSelector) bool {
	if sel == nil {
		return true
	}
	if len(sel.MatchLabels) > 0 {
		return false
	}
	if len(sel.MatchExpressions) > 0 {
		return false
	}
	return true
}
