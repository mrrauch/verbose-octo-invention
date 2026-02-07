package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TroveSpec defines the desired state of the Trove (Database as a Service) service.
type TroveSpec struct {
	ServiceTemplate `json:",inline"`

	// Database configures the Trove database connection.
	// +optional
	Database DatabaseConfig `json:"database,omitempty"`

	// MessageQueue configures the RabbitMQ connection.
	// +optional
	MessageQueue RabbitMQConfig `json:"messageQueue,omitempty"`

	// TaskManagerReplicas is the number of trove-taskmanager replicas.
	// +kubebuilder:default=1
	// +optional
	TaskManagerReplicas *int32 `json:"taskManagerReplicas,omitempty"`

	// EnabledDatastores lists the database engines available to tenants.
	// +kubebuilder:default={"mysql","postgresql"}
	// +optional
	EnabledDatastores []string `json:"enabledDatastores,omitempty"`
}

// TroveStatus defines the observed state of Trove.
type TroveStatus struct {
	CommonStatus `json:",inline"`

	// APIEndpoint is the internal API URL of the Trove service.
	// +optional
	APIEndpoint string `json:"apiEndpoint,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Trove is the Schema for the troves API.
type Trove struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TroveSpec   `json:"spec,omitempty"`
	Status TroveStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// TroveList contains a list of Trove.
type TroveList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Trove `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Trove{}, &TroveList{})
}
