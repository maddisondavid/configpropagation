package v1alpha1

import (
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/runtime"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"configpropagation/pkg/core"
)

var _ webhook.Defaulter = &ConfigPropagation{}
var _ webhook.Validator = &ConfigPropagation{}
var _ runtime.Object = &ConfigPropagation{}
var _ runtime.Object = &ConfigPropagationList{}

// Default implements webhook.Defaulter.
func (c *ConfigPropagation) Default() { core.DefaultSpec(&c.Spec) }

// SetupWebhookWithManager registers the webhook with the provided manager.
func (c *ConfigPropagation) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(c).
		Complete()
}

// ValidateCreate implements webhook.Validator.
func (c *ConfigPropagation) ValidateCreate() (admission.Warnings, error) {
	if err := core.ValidateSpec(&c.Spec); err != nil {
		return nil, err
	}
	return nil, nil
}

// ValidateUpdate implements webhook.Validator.
func (c *ConfigPropagation) ValidateUpdate(runtime.Object) (admission.Warnings, error) {
	if err := core.ValidateSpec(&c.Spec); err != nil {
		return nil, err
	}
	return nil, nil
}

// ValidateDelete implements webhook.Validator.
func (c *ConfigPropagation) ValidateDelete() (admission.Warnings, error) { return nil, nil }

// ApplyRolloutStatus updates status fields after a reconcile using rollout progress.
func (c *ConfigPropagation) ApplyRolloutStatus(result core.RolloutResult) {
	now := time.Now().UTC().Format(time.RFC3339)
	c.Status.LastSyncTime = now
	c.Status.TargetCount = int32(result.TotalTargets)
	synced := result.CompletedCount
	if synced < 0 {
		synced = 0
	}
	c.Status.SyncedCount = int32(synced)
	pending := result.TotalTargets - synced
	if pending < 0 {
		pending = 0
	}
	c.Status.OutOfSyncCount = int32(pending)
	if len(result.SkippedTargets) > 0 {
		c.Status.OutOfSync = append([]core.OutOfSyncItem(nil), result.SkippedTargets...)
	} else {
		c.Status.OutOfSync = nil
	}
	c.Status.Conditions = computeConditions(now, result, synced)
}

func computeConditions(now string, result core.RolloutResult, synced int) []core.Condition {
	total := result.TotalTargets
	outOfSync := len(result.SkippedTargets)
	pending := total - synced
	if pending < 0 {
		pending = 0
	}

	ready := core.Condition{Type: core.CondReady, LastTransitionTime: now}
	switch {
	case total == 0:
		ready.Status = "True"
		ready.Reason = "NoTargets"
		ready.Message = "no target namespaces selected"
	case outOfSync > 0:
		ready.Status = "False"
		ready.Reason = "OutOfSync"
		ready.Message = fmt.Sprintf("%d targets out of sync", outOfSync)
	case pending == 0:
		ready.Status = "True"
		ready.Reason = "AllSynced"
		ready.Message = fmt.Sprintf("propagated to %d/%d namespaces", synced, total)
	default:
		ready.Status = "False"
		ready.Reason = "InProgress"
		ready.Message = fmt.Sprintf("propagated to %d/%d namespaces", synced, total)
	}

	progressing := core.Condition{Type: core.CondProgressing, LastTransitionTime: now}
	switch {
	case outOfSync > 0:
		progressing.Status = "False"
		progressing.Reason = "Blocked"
		progressing.Message = fmt.Sprintf("%d targets blocked by conflicts", outOfSync)
	case pending > 0:
		progressing.Status = "True"
		progressing.Reason = "RollingUpdate"
		if batch := len(result.Planned); batch > 0 {
			progressing.Message = fmt.Sprintf("processing batch of %d (synced %d/%d)", batch, synced, total)
		} else {
			progressing.Message = fmt.Sprintf("waiting for rollout completion (%d/%d)", synced, total)
		}
	default:
		progressing.Status = "False"
		progressing.Reason = "Completed"
		progressing.Message = "all targets synced"
	}

	degraded := core.Condition{Type: core.CondDegraded, LastTransitionTime: now}
	if outOfSync > 0 {
		degraded.Status = "True"
		degraded.Reason = "OutOfSync"
		degraded.Message = fmt.Sprintf("%d targets skipped", outOfSync)
	} else {
		degraded.Status = "False"
		degraded.Reason = "NoIssues"
		degraded.Message = "no drift detected"
	}

	return []core.Condition{ready, progressing, degraded}
}

// DeepCopyInto copies the receiver into out.
func (c *ConfigPropagation) DeepCopyInto(out *ConfigPropagation) {
	if c == nil || out == nil {
		return
	}
	*out = *c
	c.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	specCopy := deepCopySpec((*core.ConfigPropagationSpec)(&c.Spec))
	out.Spec = specCopy
	statusCopy := deepCopyStatus((*core.ConfigPropagationStatus)(&c.Status))
	out.Status = statusCopy
}

// DeepCopy creates a new deep copy of the receiver.
func (c *ConfigPropagation) DeepCopy() *ConfigPropagation {
	if c == nil {
		return nil
	}
	out := new(ConfigPropagation)
	c.DeepCopyInto(out)
	return out
}

// DeepCopyObject returns a deep copy as a runtime.Object.
func (c *ConfigPropagation) DeepCopyObject() runtime.Object {
	if c == nil {
		return nil
	}
	return c.DeepCopy()
}

// DeepCopyInto copies the receiver into out.
func (c *ConfigPropagationList) DeepCopyInto(out *ConfigPropagationList) {
	if c == nil || out == nil {
		return
	}
	*out = *c
	c.ListMeta.DeepCopyInto(&out.ListMeta)
	if c.Items != nil {
		out.Items = make([]ConfigPropagation, len(c.Items))
		for i := range c.Items {
			c.Items[i].DeepCopyInto(&out.Items[i])
		}
	}
}

// DeepCopy creates a new deep copy of the list.
func (c *ConfigPropagationList) DeepCopy() *ConfigPropagationList {
	if c == nil {
		return nil
	}
	out := new(ConfigPropagationList)
	c.DeepCopyInto(out)
	return out
}

// DeepCopyObject returns a deep copy of the list as a runtime.Object.
func (c *ConfigPropagationList) DeepCopyObject() runtime.Object {
	if c == nil {
		return nil
	}
	return c.DeepCopy()
}

func deepCopySpec(in *core.ConfigPropagationSpec) core.ConfigPropagationSpec {
	if in == nil {
		return core.ConfigPropagationSpec{}
	}
	out := *in
	if in.NamespaceSelector != nil {
		selector := *in.NamespaceSelector
		if in.NamespaceSelector.MatchLabels != nil {
			selector.MatchLabels = make(map[string]string, len(in.NamespaceSelector.MatchLabels))
			for k, v := range in.NamespaceSelector.MatchLabels {
				selector.MatchLabels[k] = v
			}
		}
		if in.NamespaceSelector.MatchExpressions != nil {
			selector.MatchExpressions = append([]core.LabelSelectorReq(nil), in.NamespaceSelector.MatchExpressions...)
		}
		out.NamespaceSelector = &selector
	} else {
		out.NamespaceSelector = nil
	}
	if in.DataKeys != nil {
		out.DataKeys = append([]string(nil), in.DataKeys...)
	}
	if in.Strategy != nil {
		strategy := *in.Strategy
		if in.Strategy.BatchSize != nil {
			batch := *in.Strategy.BatchSize
			strategy.BatchSize = &batch
		} else {
			strategy.BatchSize = nil
		}
		out.Strategy = &strategy
	} else {
		out.Strategy = nil
	}
	if in.Prune != nil {
		prune := *in.Prune
		out.Prune = &prune
	} else {
		out.Prune = nil
	}
	if in.ResyncPeriodSeconds != nil {
		period := *in.ResyncPeriodSeconds
		out.ResyncPeriodSeconds = &period
	} else {
		out.ResyncPeriodSeconds = nil
	}
	return out
}

func deepCopyStatus(in *core.ConfigPropagationStatus) core.ConfigPropagationStatus {
	if in == nil {
		return core.ConfigPropagationStatus{}
	}
	out := *in
	if in.Conditions != nil {
		out.Conditions = append([]core.Condition(nil), in.Conditions...)
	}
	if in.OutOfSync != nil {
		out.OutOfSync = append([]core.OutOfSyncItem(nil), in.OutOfSync...)
	}
	return out
}
