package v1alpha1

import (
	"fmt"
	"time"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"codex/src/core"
)

var _ webhook.Defaulter = &ConfigPropagation{}
var _ webhook.Validator = &ConfigPropagation{}

// Default implements webhook.Defaulter.
func (c *ConfigPropagation) Default() { core.DefaultSpec(&c.Spec) }

// SetupWebhookWithManager registers the webhook with the provided manager.
func (c *ConfigPropagation) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(c).
		Complete()
}

// ValidateCreate implements webhook.Validator.
func (c *ConfigPropagation) ValidateCreate() error {
	if err := core.ValidateSpec(&c.Spec); err != nil {
		return err
	}
	return nil
}

// ValidateUpdate implements webhook.Validator.
func (c *ConfigPropagation) ValidateUpdate(webhook.Object) error {
	if err := core.ValidateSpec(&c.Spec); err != nil {
		return err
	}
	return nil
}

// ValidateDelete implements webhook.Validator.
func (c *ConfigPropagation) ValidateDelete() error { return nil }

// ApplySuccessStatus updates status fields after a successful reconcile.
func (c *ConfigPropagation) ApplySuccessStatus(planned int) {
	now := time.Now().UTC().Format(time.RFC3339)
	c.Status.LastSyncTime = now
	c.Status.TargetCount = int32(planned)
	c.Status.SyncedCount = int32(planned)
	c.Status.OutOfSyncCount = 0
	c.Status.OutOfSync = nil
	c.Status.Conditions = []core.Condition{{
		Type:               core.CondReady,
		Status:             "True",
		Reason:             "Reconciled",
		Message:            fmt.Sprintf("propagated to %d namespaces", planned),
		LastTransitionTime: now,
	}}
}
