package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ZunSpec defines the desired state of the Zun (Container) service.
type ZunSpec struct {
	ServiceTemplate `json:",inline"`

	// Database configures the Zun database connection.
	// +optional
	Database DatabaseConfig `json:"database,omitempty"`

	// MessageQueue configures the RabbitMQ connection.
	// +optional
	MessageQueue RabbitMQConfig `json:"messageQueue,omitempty"`

	// ComputeReplicas is the number of zun-compute replicas.
	// +kubebuilder:default=1
	// +optional
	ComputeReplicas *int32 `json:"computeReplicas,omitempty"`

	// EnableWebsocketProxy enables the zun-wsproxy for console access.
	// +kubebuilder:default=true
	// +optional
	EnableWebsocketProxy bool `json:"enableWebsocketProxy,omitempty"`
}

// ZunStatus defines the observed state of Zun.
type ZunStatus struct {
	CommonStatus `json:",inline"`

	// APIEndpoint is the internal API URL of the Zun service.
	// +optional
	APIEndpoint string `json:"apiEndpoint,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Zun is the Schema for the zuns API.
type Zun struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ZunSpec   `json:"spec,omitempty"`
	Status ZunStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ZunList contains a list of Zun.
type ZunList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Zun `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Zun{}, &ZunList{})
}
