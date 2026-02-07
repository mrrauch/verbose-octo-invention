package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MistralSpec defines the desired state of the Mistral (Workflow) service.
type MistralSpec struct {
	ServiceTemplate `json:",inline"`

	// Database configures the Mistral database connection.
	// +optional
	Database DatabaseConfig `json:"database,omitempty"`

	// MessageQueue configures the RabbitMQ connection.
	// +optional
	MessageQueue RabbitMQConfig `json:"messageQueue,omitempty"`

	// EngineReplicas is the number of mistral-engine replicas.
	// +kubebuilder:default=1
	// +optional
	EngineReplicas *int32 `json:"engineReplicas,omitempty"`

	// ExecutorReplicas is the number of mistral-executor replicas.
	// +kubebuilder:default=1
	// +optional
	ExecutorReplicas *int32 `json:"executorReplicas,omitempty"`
}

// MistralStatus defines the observed state of Mistral.
type MistralStatus struct {
	CommonStatus `json:",inline"`

	// APIEndpoint is the internal API URL of the Mistral service.
	// +optional
	APIEndpoint string `json:"apiEndpoint,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Mistral is the Schema for the mistrals API.
type Mistral struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MistralSpec   `json:"spec,omitempty"`
	Status MistralStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// MistralList contains a list of Mistral.
type MistralList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Mistral `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Mistral{}, &MistralList{})
}
