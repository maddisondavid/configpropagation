package adapters

// KubeClient defines the minimal interactions the reconciler needs.
type KubeClient interface {
	// GetSourceConfigMap returns the data from the source ConfigMap or nil if not found.
	GetSourceConfigMap(namespace, name string) (map[string]string, error)
	// ListNamespacesBySelector returns namespaces names matching the given selector.
	ListNamespacesBySelector(matchLabels map[string]string, exprs []LabelSelectorRequirement) ([]string, error)
	// UpsertConfigMap creates or updates the target ConfigMap with given data and metadata.
	UpsertConfigMap(namespace, name string, data map[string]string, labels, annotations map[string]string) error
	// GetTargetConfigMap returns existing target metadata for drift detection.
	// found=false indicates it does not exist.
	GetTargetConfigMap(namespace, name string) (data map[string]string, labels map[string]string, annotations map[string]string, found bool, err error)
	// ListManagedTargetNamespaces returns namespaces of targets managed for a given source (ns/name string) and configmap name.
	ListManagedTargetNamespaces(source string, name string) ([]string, error)
	// DeleteConfigMap deletes a target ConfigMap.
	DeleteConfigMap(namespace, name string) error
	// UpdateConfigMapMetadata updates labels/annotations on a target (used to detach).
	UpdateConfigMapMetadata(namespace, name string, labels, annotations map[string]string) error
}

// LabelSelectorRequirement mirrors a subset of core.LabelSelectorReq to avoid import cycles.
type LabelSelectorRequirement struct {
	Key      string
	Operator string // In, NotIn, Exists, DoesNotExist
	Values   []string
}
