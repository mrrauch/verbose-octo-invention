package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MagnumSpec defines the desired state of the Magnum (Container Infrastructure) service.
type MagnumSpec struct {
	ServiceTemplate `json:",inline"`

	// Database configures the Magnum database connection.
	// +optional
	Database DatabaseConfig `json:"database,omitempty"`

	// MessageQueue configures the RabbitMQ connection.
	// +optional
	MessageQueue RabbitMQConfig `json:"messageQueue,omitempty"`

	// ConductorReplicas is the number of magnum-conductor replicas.
	// +kubebuilder:default=1
	// +optional
	ConductorReplicas *int32 `json:"conductorReplicas,omitempty"`

	// DefaultCOE is the default Container Orchestration Engine for new clusters.
	// +kubebuilder:validation:Enum=kubernetes;swarm
	// +kubebuilder:default="kubernetes"
	// +optional
	DefaultCOE string `json:"defaultCOE,omitempty"`
}

// MagnumStatus defines the observed state of Magnum.
type MagnumStatus struct {
	CommonStatus `json:",inline"`

	// APIEndpoint is the internal API URL of the Magnum service.
	// +optional
	APIEndpoint string `json:"apiEndpoint,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Magnum is the Schema for the magnums API.
type Magnum struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MagnumSpec   `json:"spec,omitempty"`
	Status MagnumStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// MagnumList contains a list of Magnum.
type MagnumList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Magnum `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Magnum{}, &MagnumList{})
}
