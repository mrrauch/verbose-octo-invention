package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// BlazarSpec defines the desired state of the Blazar (Resource Reservation) service.
type BlazarSpec struct {
	ServiceTemplate `json:",inline"`

	// Database configures the Blazar database connection.
	// +optional
	Database DatabaseConfig `json:"database,omitempty"`

	// MessageQueue configures the RabbitMQ connection.
	// +optional
	MessageQueue RabbitMQConfig `json:"messageQueue,omitempty"`

	// ManagerReplicas is the number of blazar-manager replicas.
	// +kubebuilder:default=1
	// +optional
	ManagerReplicas *int32 `json:"managerReplicas,omitempty"`

	// EnabledPlugins lists the enabled reservation plugins.
	// +kubebuilder:default={"physical.host.plugin","virtual.instance.plugin"}
	// +optional
	EnabledPlugins []string `json:"enabledPlugins,omitempty"`
}

// BlazarStatus defines the observed state of Blazar.
type BlazarStatus struct {
	CommonStatus `json:",inline"`

	// APIEndpoint is the internal API URL of the Blazar service.
	// +optional
	APIEndpoint string `json:"apiEndpoint,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Blazar is the Schema for the blazars API.
type Blazar struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BlazarSpec   `json:"spec,omitempty"`
	Status BlazarStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// BlazarList contains a list of Blazar.
type BlazarList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Blazar `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Blazar{}, &BlazarList{})
}
