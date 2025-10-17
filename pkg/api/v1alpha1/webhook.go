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
	c.Status.SyncedCount = int32(result.CompletedCount)
	pending := result.TotalTargets - result.CompletedCount
	if pending < 0 {
		pending = 0
	}
	c.Status.OutOfSyncCount = int32(pending)
	c.Status.OutOfSync = nil
	reason := "Reconciled"
	message := fmt.Sprintf("propagated to %d/%d namespaces", result.CompletedCount, result.TotalTargets)
	status := "True"
	if pending > 0 {
		reason = "RollingUpdate"
		message = fmt.Sprintf("propagated to %d/%d namespaces (batch of %d)", result.CompletedCount, result.TotalTargets, len(result.Planned))
		status = "False"
	}
	c.Status.Conditions = []core.Condition{{
		Type:               core.CondReady,
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: now,
	}}
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
