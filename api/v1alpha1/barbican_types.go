package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// BarbicanSpec defines the desired state of the Barbican (Key Manager) service.
type BarbicanSpec struct {
	ServiceTemplate `json:",inline"`

	// Database configures the Barbican database connection.
	// +optional
	Database DatabaseConfig `json:"database,omitempty"`

	// MessageQueue configures the RabbitMQ connection.
	// +optional
	MessageQueue RabbitMQConfig `json:"messageQueue,omitempty"`

	// SecretStore selects the backend for storing secrets.
	// +kubebuilder:validation:Enum=simple_crypto;pkcs11;vault;kmip
	// +kubebuilder:default="simple_crypto"
	// +optional
	SecretStore string `json:"secretStore,omitempty"`

	// WorkerReplicas is the number of barbican-worker replicas.
	// +kubebuilder:default=1
	// +optional
	WorkerReplicas *int32 `json:"workerReplicas,omitempty"`
}

// BarbicanStatus defines the observed state of Barbican.
type BarbicanStatus struct {
	CommonStatus `json:",inline"`

	// APIEndpoint is the internal API URL of the Barbican service.
	// +optional
	APIEndpoint string `json:"apiEndpoint,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Barbican is the Schema for the barbicans API.
type Barbican struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BarbicanSpec   `json:"spec,omitempty"`
	Status BarbicanStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// BarbicanList contains a list of Barbican.
type BarbicanList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Barbican `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Barbican{}, &BarbicanList{})
}
