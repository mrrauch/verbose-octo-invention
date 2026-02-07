package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NovaSpec defines the desired state of the Nova (Compute) service.
type NovaSpec struct {
	ServiceTemplate `json:",inline"`

	// Database configures the Nova database connection.
	// +optional
	Database DatabaseConfig `json:"database,omitempty"`

	// MessageQueue configures the RabbitMQ connection.
	// +optional
	MessageQueue RabbitMQConfig `json:"messageQueue,omitempty"`

	// VirtType selects the virtualization technology.
	// +kubebuilder:validation:Enum=kvm;qemu
	// +kubebuilder:default="kvm"
	// +optional
	VirtType string `json:"virtType,omitempty"`

	// CellName is the Nova cell name for this deployment.
	// +kubebuilder:default="cell1"
	// +optional
	CellName string `json:"cellName,omitempty"`

	// ComputeReplicas is the number of nova-compute pod replicas.
	// Only relevant when compute runs inside Kubernetes (not on external data-plane nodes).
	// +kubebuilder:default=1
	// +optional
	ComputeReplicas *int32 `json:"computeReplicas,omitempty"`

	// EphemeralStorage selects the backend for ephemeral disks.
	// +kubebuilder:validation:Enum=local;ceph
	// +kubebuilder:default="local"
	// +optional
	EphemeralStorage string `json:"ephemeralStorage,omitempty"`
}

// NovaStatus defines the observed state of Nova.
type NovaStatus struct {
	CommonStatus `json:",inline"`

	// APIEndpoint is the internal API URL of the Nova service.
	// +optional
	APIEndpoint string `json:"apiEndpoint,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Endpoint",type=string,JSONPath=`.status.apiEndpoint`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Nova is the Schema for the novas API.
type Nova struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NovaSpec   `json:"spec,omitempty"`
	Status NovaStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// NovaList contains a list of Nova.
type NovaList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Nova `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Nova{}, &NovaList{})
}
