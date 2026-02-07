package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// HeatSpec defines the desired state of the Heat (Orchestration) service.
type HeatSpec struct {
	ServiceTemplate `json:",inline"`

	// Database configures the Heat database connection.
	// +optional
	Database DatabaseConfig `json:"database,omitempty"`

	// MessageQueue configures the RabbitMQ connection.
	// +optional
	MessageQueue RabbitMQConfig `json:"messageQueue,omitempty"`

	// EngineReplicas is the number of heat-engine replicas.
	// +kubebuilder:default=1
	// +optional
	EngineReplicas *int32 `json:"engineReplicas,omitempty"`
}

// HeatStatus defines the observed state of Heat.
type HeatStatus struct {
	CommonStatus `json:",inline"`

	// APIEndpoint is the internal API URL of the Heat service.
	// +optional
	APIEndpoint string `json:"apiEndpoint,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Heat is the Schema for the heats API.
type Heat struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HeatSpec   `json:"spec,omitempty"`
	Status HeatStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// HeatList contains a list of Heat.
type HeatList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Heat `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Heat{}, &HeatList{})
}
