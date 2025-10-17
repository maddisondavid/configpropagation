package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"configpropagation/src/core"
)

// ConfigPropagationSpec defines the desired state of ConfigPropagation.
type ConfigPropagationSpec = core.ConfigPropagationSpec

// ConfigPropagationStatus defines observed state.
type ConfigPropagationStatus = core.ConfigPropagationStatus

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=cprop
// +kubebuilder:printcolumn:name="Targets",type="integer",JSONPath=".status.targetCount"
// +kubebuilder:printcolumn:name="Synced",type="integer",JSONPath=".status.syncedCount"
// +kubebuilder:printcolumn:name="OutOfSync",type="integer",JSONPath=".status.outOfSyncCount"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// ConfigPropagation is the Schema for the API.
type ConfigPropagation struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ConfigPropagationSpec   `json:"spec,omitempty"`
	Status ConfigPropagationStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ConfigPropagationList contains a list of ConfigPropagation.
type ConfigPropagationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ConfigPropagation `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ConfigPropagation{}, &ConfigPropagationList{})
}
