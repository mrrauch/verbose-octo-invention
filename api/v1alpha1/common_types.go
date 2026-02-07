package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ServiceTemplate contains common fields for all OpenStack service specs.
type ServiceTemplate struct {
	// Replicas is the number of pod replicas for the service.
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=0
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`

	// Image is the container image to use. If empty, the operator default is used.
	// +optional
	Image string `json:"image,omitempty"`

	// Resources defines compute resource requirements for the service pods.
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// NodeSelector constrains scheduling to nodes matching these labels.
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
}

// DatabaseConfig defines the database connection parameters for a service.
type DatabaseConfig struct {
	// SecretName references the Secret containing database credentials.
	// The operator auto-generates this if left empty.
	// +optional
	SecretName string `json:"secretName,omitempty"`

	// Engine selects which SQL backend this service should use.
	// +kubebuilder:validation:Enum=postgresql;mysql;mariadb
	// +optional
	Engine DatabaseEngine `json:"engine,omitempty"`
}

// RabbitMQConfig defines the message queue connection parameters.
type RabbitMQConfig struct {
	// SecretName references the Secret containing RabbitMQ credentials.
	// +optional
	SecretName string `json:"secretName,omitempty"`
}

// TLSConfig defines TLS settings for a service.
type TLSConfig struct {
	// Enabled toggles TLS for the service endpoints.
	// +kubebuilder:default=false
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// IssuerRef references a cert-manager Issuer or ClusterIssuer.
	// +optional
	IssuerRef *CertIssuerRef `json:"issuerRef,omitempty"`
}

// CertIssuerRef references a cert-manager issuer.
type CertIssuerRef struct {
	Name string `json:"name"`
	Kind string `json:"kind"` // Issuer or ClusterIssuer
}

// StorageConfig defines persistent storage settings.
type StorageConfig struct {
	// Size is the requested storage size.
	// +kubebuilder:default="10Gi"
	Size resource.Quantity `json:"size,omitempty"`

	// StorageClassName is the name of the StorageClass to use.
	// +optional
	StorageClassName *string `json:"storageClassName,omitempty"`
}

// GatewayRef references a Gateway API Gateway resource for external routing.
type GatewayRef struct {
	// Name of the Gateway resource.
	// +optional
	Name string `json:"name,omitempty"`

	// Namespace of the Gateway resource. If empty, service namespace is used.
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// Optional listener name to bind routes to a specific Gateway listener.
	// +optional
	ListenerName string `json:"listenerName,omitempty"`
}

// ConditionType represents the type of a status condition.
type ConditionType string

const (
	// ConditionReady indicates the resource is fully operational.
	ConditionReady ConditionType = "Ready"

	// ConditionDatabaseReady indicates the service database has been created and synced.
	ConditionDatabaseReady ConditionType = "DatabaseReady"

	// ConditionDeploymentReady indicates the service Deployment is available.
	ConditionDeploymentReady ConditionType = "DeploymentReady"

	// ConditionBootstrapReady indicates the service has been bootstrapped.
	ConditionBootstrapReady ConditionType = "BootstrapReady"
)

// CommonStatus contains status fields shared by all service CRs.
type CommonStatus struct {
	// Conditions represent the latest available observations of the resource's state.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// ObservedGeneration is the most recent generation observed by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}
