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
func (configPropagation *ConfigPropagation) Default() { core.DefaultSpec(&configPropagation.Spec) }

// SetupWebhookWithManager registers the webhook with the provided manager.
func (configPropagation *ConfigPropagation) SetupWebhookWithManager(manager ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(manager).
		For(configPropagation).
		Complete()
}

// ValidateCreate implements webhook.Validator.
func (configPropagation *ConfigPropagation) ValidateCreate() (admission.Warnings, error) {
	if err := core.ValidateSpec(&configPropagation.Spec); err != nil {
		return nil, err
	}

	return nil, nil
}

// ValidateUpdate implements webhook.Validator.
func (configPropagation *ConfigPropagation) ValidateUpdate(runtime.Object) (admission.Warnings, error) {
	if err := core.ValidateSpec(&configPropagation.Spec); err != nil {
		return nil, err
	}

	return nil, nil
}

// ValidateDelete implements webhook.Validator.
func (configPropagation *ConfigPropagation) ValidateDelete() (admission.Warnings, error) {
	return nil, nil
}

// ApplyRolloutStatus updates status fields after a reconcile using rollout progress.
func (configPropagation *ConfigPropagation) ApplyRolloutStatus(result core.RolloutResult) {
	currentTime := time.Now().UTC().Format(time.RFC3339)

	configPropagation.Status.LastSyncTime = currentTime
	configPropagation.Status.TargetCount = int32(result.TotalTargets)
	configPropagation.Status.SyncedCount = int32(result.CompletedCount)

	pendingCount := result.TotalTargets - result.CompletedCount
	if pendingCount < 0 {
		pendingCount = 0
	}

	configPropagation.Status.OutOfSyncCount = int32(pendingCount)
	if result.OutOfSync != nil {
		configPropagation.Status.OutOfSync = append([]core.OutOfSyncItem(nil), result.OutOfSync...)
	} else {
		configPropagation.Status.OutOfSync = nil
	}

	readyStatus := "True"
	readyReason := "Reconciled"
	readyMessage := fmt.Sprintf("propagated to %d/%d namespaces", result.CompletedCount, result.TotalTargets)

	progressingStatus := "False"
	progressingReason := "Idle"
	progressingMessage := "reconciliation complete"

	degradedStatus := "False"
	degradedReason := "Healthy"
	degradedMessage := "targets in sync"

	if len(result.OutOfSync) > 0 {
		readyStatus = "False"
		readyReason = "OutOfSync"
		readyMessage = fmt.Sprintf("%d namespaces remain out of sync", len(result.OutOfSync))

		progressingStatus = "False"
		progressingReason = "Blocked"
		progressingMessage = "waiting for conflicts to resolve"

		degradedStatus = "True"
		degradedReason = "OutOfSync"
		degradedMessage = readyMessage
	} else if pendingCount > 0 {
		readyStatus = "False"
		readyReason = "RollingUpdate"
		readyMessage = fmt.Sprintf("propagated to %d/%d namespaces (batch of %d)", result.CompletedCount, result.TotalTargets, len(result.Planned))

		progressingStatus = "True"
		progressingReason = "RollingUpdate"
		progressingMessage = readyMessage

		degradedStatus = "False"
		degradedReason = "Healthy"
		degradedMessage = "rollout in progress"
	}

	configPropagation.Status.Conditions = []core.Condition{
		{
			Type:               core.CondReady,
			Status:             readyStatus,
			Reason:             readyReason,
			Message:            readyMessage,
			LastTransitionTime: currentTime,
		},
		{
			Type:               core.CondProgressing,
			Status:             progressingStatus,
			Reason:             progressingReason,
			Message:            progressingMessage,
			LastTransitionTime: currentTime,
		},
		{
			Type:               core.CondDegraded,
			Status:             degradedStatus,
			Reason:             degradedReason,
			Message:            degradedMessage,
			LastTransitionTime: currentTime,
		},
	}
}

// DeepCopyInto copies the receiver into out.
func (configPropagation *ConfigPropagation) DeepCopyInto(out *ConfigPropagation) {
	if configPropagation == nil || out == nil {
		return
	}
	*out = *configPropagation
	configPropagation.ObjectMeta.DeepCopyInto(&out.ObjectMeta)

	specCopy := deepCopySpec((*core.ConfigPropagationSpec)(&configPropagation.Spec))
	out.Spec = specCopy

	statusCopy := deepCopyStatus((*core.ConfigPropagationStatus)(&configPropagation.Status))
	out.Status = statusCopy
}

// DeepCopy creates a new deep copy of the receiver.
func (configPropagation *ConfigPropagation) DeepCopy() *ConfigPropagation {
	if configPropagation == nil {
		return nil
	}

	out := new(ConfigPropagation)

	configPropagation.DeepCopyInto(out)
	return out
}

// DeepCopyObject returns a deep copy as a runtime.Object.
func (configPropagation *ConfigPropagation) DeepCopyObject() runtime.Object {
	if configPropagation == nil {
		return nil
	}

	return configPropagation.DeepCopy()
}

// DeepCopyInto copies the receiver into out.
func (configPropagationList *ConfigPropagationList) DeepCopyInto(out *ConfigPropagationList) {
	if configPropagationList == nil || out == nil {
		return
	}
	*out = *configPropagationList
	configPropagationList.ListMeta.DeepCopyInto(&out.ListMeta)

	if configPropagationList.Items != nil {
		out.Items = make([]ConfigPropagation, len(configPropagationList.Items))

		for index := range configPropagationList.Items {
			configPropagationList.Items[index].DeepCopyInto(&out.Items[index])
		}
	}
}

// DeepCopy creates a new deep copy of the list.
func (configPropagationList *ConfigPropagationList) DeepCopy() *ConfigPropagationList {
	if configPropagationList == nil {
		return nil
	}

	out := new(ConfigPropagationList)

	configPropagationList.DeepCopyInto(out)
	return out
}

// DeepCopyObject returns a deep copy of the list as a runtime.Object.
func (configPropagationList *ConfigPropagationList) DeepCopyObject() runtime.Object {
	if configPropagationList == nil {
		return nil
	}

	return configPropagationList.DeepCopy()
}

func deepCopySpec(source *core.ConfigPropagationSpec) core.ConfigPropagationSpec {
	if source == nil {
		return core.ConfigPropagationSpec{}
	}
	copiedSpec := *source

	if source.NamespaceSelector != nil {
		selectorCopy := *source.NamespaceSelector

		if source.NamespaceSelector.MatchLabels != nil {
			selectorCopy.MatchLabels = make(map[string]string, len(source.NamespaceSelector.MatchLabels))

			for labelKey, labelValue := range source.NamespaceSelector.MatchLabels {
				selectorCopy.MatchLabels[labelKey] = labelValue
			}
		}

		if source.NamespaceSelector.MatchExpressions != nil {
			selectorCopy.MatchExpressions = append([]core.LabelSelectorReq(nil), source.NamespaceSelector.MatchExpressions...)
		}

		copiedSpec.NamespaceSelector = &selectorCopy
	} else {
		copiedSpec.NamespaceSelector = nil
	}

	if source.DataKeys != nil {
		copiedSpec.DataKeys = append([]string(nil), source.DataKeys...)
	}

	if source.Strategy != nil {
		strategyCopy := *source.Strategy

		if source.Strategy.BatchSize != nil {
			batchSizeCopy := *source.Strategy.BatchSize
			strategyCopy.BatchSize = &batchSizeCopy
		} else {
			strategyCopy.BatchSize = nil
		}

		copiedSpec.Strategy = &strategyCopy
	} else {
		copiedSpec.Strategy = nil
	}

	if source.Prune != nil {
		pruneCopy := *source.Prune
		copiedSpec.Prune = &pruneCopy
	} else {
		copiedSpec.Prune = nil
	}

	if source.ResyncPeriodSeconds != nil {
		resyncPeriodCopy := *source.ResyncPeriodSeconds
		copiedSpec.ResyncPeriodSeconds = &resyncPeriodCopy
	} else {
		copiedSpec.ResyncPeriodSeconds = nil
	}

	return copiedSpec
}

func deepCopyStatus(source *core.ConfigPropagationStatus) core.ConfigPropagationStatus {
	if source == nil {
		return core.ConfigPropagationStatus{}
	}
	copiedStatus := *source

	if source.Conditions != nil {
		copiedStatus.Conditions = append([]core.Condition(nil), source.Conditions...)
	}

	if source.OutOfSync != nil {
		copiedStatus.OutOfSync = append([]core.OutOfSyncItem(nil), source.OutOfSync...)
	}

	return copiedStatus
}
