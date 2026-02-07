package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CinderBackend defines a single Cinder storage backend.
type CinderBackend struct {
	// Name is a unique identifier for this backend.
	Name string `json:"name"`

	// Type selects the volume driver.
	// +kubebuilder:validation:Enum=ceph;lvm
	Type string `json:"type"`

	// CephPoolName is the RBD pool name (required when type is "ceph").
	// +optional
	CephPoolName string `json:"cephPoolName,omitempty"`

	// LVMVolumeGroup is the LVM VG name (required when type is "lvm").
	// +optional
	LVMVolumeGroup string `json:"lvmVolumeGroup,omitempty"`
}

// CinderSpec defines the desired state of the Cinder (Block Storage) service.
type CinderSpec struct {
	ServiceTemplate `json:",inline"`

	// Database configures the Cinder database connection.
	// +optional
	Database DatabaseConfig `json:"database,omitempty"`

	// MessageQueue configures the RabbitMQ connection.
	// +optional
	MessageQueue RabbitMQConfig `json:"messageQueue,omitempty"`

	// Backends lists the storage backends to configure.
	// +kubebuilder:validation:MinItems=1
	Backends []CinderBackend `json:"backends"`

	// DefaultBackend is the name of the default backend (must match one of Backends[].Name).
	// +optional
	DefaultBackend string `json:"defaultBackend,omitempty"`
}

// CinderStatus defines the observed state of Cinder.
type CinderStatus struct {
	CommonStatus `json:",inline"`

	// APIEndpoint is the internal API URL of the Cinder service.
	// +optional
	APIEndpoint string `json:"apiEndpoint,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Endpoint",type=string,JSONPath=`.status.apiEndpoint`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Cinder is the Schema for the cinders API.
type Cinder struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CinderSpec   `json:"spec,omitempty"`
	Status CinderStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// CinderList contains a list of Cinder.
type CinderList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Cinder `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Cinder{}, &CinderList{})
}
