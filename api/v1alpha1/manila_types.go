package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ManilaSpec defines the desired state of the Manila (Shared Filesystems) service.
type ManilaSpec struct {
	ServiceTemplate `json:",inline"`

	// Database configures the Manila database connection.
	// +optional
	Database DatabaseConfig `json:"database,omitempty"`

	// MessageQueue configures the RabbitMQ connection.
	// +optional
	MessageQueue RabbitMQConfig `json:"messageQueue,omitempty"`

	// Backend selects the share backend driver.
	// +kubebuilder:validation:Enum=cephfs;nfs;netapp
	// +kubebuilder:default="cephfs"
	// +optional
	Backend string `json:"backend,omitempty"`

	// CephFSPoolName is the CephFS data pool name (when backend is "cephfs").
	// +kubebuilder:default="manila-data"
	// +optional
	CephFSPoolName string `json:"cephFSPoolName,omitempty"`

	// ShareReplicas is the number of manila-share replicas.
	// +kubebuilder:default=1
	// +optional
	ShareReplicas *int32 `json:"shareReplicas,omitempty"`
}

// ManilaStatus defines the observed state of Manila.
type ManilaStatus struct {
	CommonStatus `json:",inline"`

	// APIEndpoint is the internal API URL of the Manila service.
	// +optional
	APIEndpoint string `json:"apiEndpoint,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Backend",type=string,JSONPath=`.spec.backend`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Manila is the Schema for the manilas API.
type Manila struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ManilaSpec   `json:"spec,omitempty"`
	Status ManilaStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ManilaList contains a list of Manila.
type ManilaList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Manila `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Manila{}, &ManilaList{})
}
