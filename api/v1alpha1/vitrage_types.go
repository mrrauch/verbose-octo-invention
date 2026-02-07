package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// VitrageSpec defines the desired state of the Vitrage (Root Cause Analysis) service.
type VitrageSpec struct {
	ServiceTemplate `json:",inline"`

	// Database configures the Vitrage database connection.
	// +optional
	Database DatabaseConfig `json:"database,omitempty"`

	// MessageQueue configures the RabbitMQ connection.
	// +optional
	MessageQueue RabbitMQConfig `json:"messageQueue,omitempty"`

	// GraphReplicas is the number of vitrage-graph replicas.
	// +kubebuilder:default=1
	// +optional
	GraphReplicas *int32 `json:"graphReplicas,omitempty"`

	// EnableML enables the Vitrage ML component for anomaly detection.
	// +kubebuilder:default=false
	// +optional
	EnableML bool `json:"enableML,omitempty"`
}

// VitrageStatus defines the observed state of Vitrage.
type VitrageStatus struct {
	CommonStatus `json:",inline"`

	// APIEndpoint is the internal API URL of the Vitrage service.
	// +optional
	APIEndpoint string `json:"apiEndpoint,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Vitrage is the Schema for the vitrages API.
type Vitrage struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VitrageSpec   `json:"spec,omitempty"`
	Status VitrageStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// VitrageList contains a list of Vitrage.
type VitrageList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Vitrage `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Vitrage{}, &VitrageList{})
}
