package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TackerSpec defines the desired state of the Tacker (NFV Orchestration) service.
type TackerSpec struct {
	ServiceTemplate `json:",inline"`

	// Database configures the Tacker database connection.
	// +optional
	Database DatabaseConfig `json:"database,omitempty"`

	// MessageQueue configures the RabbitMQ connection.
	// +optional
	MessageQueue RabbitMQConfig `json:"messageQueue,omitempty"`

	// ConductorReplicas is the number of tacker-conductor replicas.
	// +kubebuilder:default=1
	// +optional
	ConductorReplicas *int32 `json:"conductorReplicas,omitempty"`

	// EnableVNFM enables the VNF Manager function.
	// +kubebuilder:default=true
	// +optional
	EnableVNFM bool `json:"enableVNFM,omitempty"`

	// EnableNFVO enables the NFV Orchestrator function.
	// +kubebuilder:default=true
	// +optional
	EnableNFVO bool `json:"enableNFVO,omitempty"`
}

// TackerStatus defines the observed state of Tacker.
type TackerStatus struct {
	CommonStatus `json:",inline"`

	// APIEndpoint is the internal API URL of the Tacker service.
	// +optional
	APIEndpoint string `json:"apiEndpoint,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Tacker is the Schema for the tackers API.
type Tacker struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TackerSpec   `json:"spec,omitempty"`
	Status TackerStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// TackerList contains a list of Tacker.
type TackerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Tacker `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Tacker{}, &TackerList{})
}
