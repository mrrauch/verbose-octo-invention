package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CyborgSpec defines the desired state of the Cyborg (Accelerator Management) service.
type CyborgSpec struct {
	ServiceTemplate `json:",inline"`

	// Database configures the Cyborg database connection.
	// +optional
	Database DatabaseConfig `json:"database,omitempty"`

	// MessageQueue configures the RabbitMQ connection.
	// +optional
	MessageQueue RabbitMQConfig `json:"messageQueue,omitempty"`

	// ConductorReplicas is the number of cyborg-conductor replicas.
	// +kubebuilder:default=1
	// +optional
	ConductorReplicas *int32 `json:"conductorReplicas,omitempty"`

	// EnableAgent deploys cyborg-agent as a DaemonSet on compute nodes with accelerators.
	// +kubebuilder:default=true
	// +optional
	EnableAgent bool `json:"enableAgent,omitempty"`

	// EnabledDrivers lists the enabled accelerator drivers (e.g., gpu, fpga).
	// +optional
	EnabledDrivers []string `json:"enabledDrivers,omitempty"`
}

// CyborgStatus defines the observed state of Cyborg.
type CyborgStatus struct {
	CommonStatus `json:",inline"`

	// APIEndpoint is the internal API URL of the Cyborg service.
	// +optional
	APIEndpoint string `json:"apiEndpoint,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Cyborg is the Schema for the cyborgs API.
type Cyborg struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CyborgSpec   `json:"spec,omitempty"`
	Status CyborgStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// CyborgList contains a list of Cyborg.
type CyborgList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Cyborg `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Cyborg{}, &CyborgList{})
}
