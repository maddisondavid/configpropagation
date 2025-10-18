package adapters

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	record "k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	configv1alpha1 "configpropagation/pkg/api/v1alpha1"
	"configpropagation/pkg/core"
)

// EventRecorder emits Kubernetes events for ConfigPropagation resources.
type EventRecorder interface {
	// Normalf emits a Normal event with formatted message.
	Normalf(name core.NamespacedName, reason, messageFmt string, args ...interface{})
	// Warningf emits a Warning event with formatted message.
	Warningf(name core.NamespacedName, reason, messageFmt string, args ...interface{})
}

// NewNoopEventRecorder returns an EventRecorder that discards events.
func NewNoopEventRecorder() EventRecorder {
	return noopEventRecorder{}
}

type noopEventRecorder struct{}

func (noopEventRecorder) Normalf(core.NamespacedName, string, string, ...interface{}) {}

func (noopEventRecorder) Warningf(core.NamespacedName, string, string, ...interface{}) {}

// NewControllerRuntimeEventRecorder wraps a controller-runtime EventRecorder.
func NewControllerRuntimeEventRecorder(recorder record.EventRecorder) EventRecorder {
	if recorder == nil {
		return NewNoopEventRecorder()
	}
	return &controllerRuntimeEventRecorder{recorder: recorder}
}

type controllerRuntimeEventRecorder struct {
	recorder record.EventRecorder
}

func (eventRecorder *controllerRuntimeEventRecorder) Normalf(name core.NamespacedName, reason, messageFmt string, args ...interface{}) {
	eventRecorder.emit(name, corev1.EventTypeNormal, reason, messageFmt, args...)
}

func (eventRecorder *controllerRuntimeEventRecorder) Warningf(name core.NamespacedName, reason, messageFmt string, args ...interface{}) {
	eventRecorder.emit(name, corev1.EventTypeWarning, reason, messageFmt, args...)
}

func (eventRecorder *controllerRuntimeEventRecorder) emit(name core.NamespacedName, eventType, reason, messageFmt string, args ...interface{}) {
	obj := minimalConfigPropagationObject(name)
	if obj == nil {
		return
	}
	eventRecorder.recorder.Eventf(obj, eventType, reason, messageFmt, args...)
}

func minimalConfigPropagationObject(name core.NamespacedName) client.Object {
	if name.Name == "" {
		return nil
	}
	return &configv1alpha1.ConfigPropagation{
		TypeMeta: metav1.TypeMeta{Kind: "ConfigPropagation", APIVersion: configv1alpha1.GroupVersion.String()},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: name.Namespace,
			Name:      name.Name,
		},
	}
}

// Ensure the object satisfies runtime.Object for recorder compatibility.
var _ runtime.Object = (*configv1alpha1.ConfigPropagation)(nil)

// Ensure the object satisfies client.Object.
var _ client.Object = (*configv1alpha1.ConfigPropagation)(nil)
