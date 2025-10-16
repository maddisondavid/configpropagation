package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

var (
	// GroupVersion is group version used to register these objects.
	GroupVersion = schema.GroupVersion{Group: "configpropagator.platform.example.com", Version: "v1alpha1"}

	// SchemeBuilder registers our API types with a scheme.
	SchemeBuilder = &scheme.Builder{GroupVersion: GroupVersion}

	// AddToScheme adds the types to the scheme.
	AddToScheme = SchemeBuilder.AddToScheme
)
