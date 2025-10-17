package webhooks

import (
	core "configpropagation/pkg/core"
)

// DefaultConfigPropagation applies server-side style defaults to the incoming
// ConfigPropagation spec. The webhook deals strictly with the spec portion of
// the resource because status is managed by the controller loop.
func DefaultConfigPropagation(spec *core.ConfigPropagationSpec) {
	if spec == nil {
		return
	}
	core.DefaultSpec(spec)
}
