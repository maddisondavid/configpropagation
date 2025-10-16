package core

// Managed metadata keys and finalizer
const (
	ManagedLabel     = "configpropagator.platform.example.com/managed"
	SourceAnnotation = "configpropagator.platform.example.com/source"
	HashAnnotation   = "configpropagator.platform.example.com/hash"

	Finalizer = "configpropagator.platform.example.com/finalizer"
)

// Condition types
const (
	CondReady       = "Ready"
	CondProgressing = "Progressing"
	CondDegraded    = "Degraded"
)

// Strategy enums
const (
	StrategyImmediate = "immediate"
	StrategyRolling   = "rolling"
)

// Conflict policy enums
const (
	ConflictOverwrite = "overwrite"
	ConflictSkip      = "skip"
)
