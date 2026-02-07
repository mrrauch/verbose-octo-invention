package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GlanceSpec defines the desired state of the Glance (Image) service.
type GlanceSpec struct {
	ServiceTemplate `json:",inline"`

	// Database configures the Glance database connection.
	// +optional
	Database DatabaseConfig `json:"database,omitempty"`

	// MessageQueue configures the RabbitMQ connection.
	// +optional
	MessageQueue RabbitMQConfig `json:"messageQueue,omitempty"`

	// StorageType selects the image storage backend.
	// +kubebuilder:validation:Enum=pvc;ceph;swift
	// +kubebuilder:default="pvc"
	// +optional
	StorageType string `json:"storageType,omitempty"`

	// Storage defines PVC settings when storageType is "pvc".
	// +optional
	Storage StorageConfig `json:"storage,omitempty"`

	// CephPoolName is the RBD pool name when storageType is "ceph".
	// +kubebuilder:default="glance-images"
	// +optional
	CephPoolName string `json:"cephPoolName,omitempty"`

	// PublicHostname overrides the generated external hostname for this API service.
	// +optional
	PublicHostname string `json:"publicHostname,omitempty"`

	// GatewayRef overrides the control-plane default gatewayRef for this service.
	// +optional
	GatewayRef GatewayRef `json:"gatewayRef,omitempty"`
}

// GlanceStatus defines the observed state of Glance.
type GlanceStatus struct {
	CommonStatus `json:",inline"`

	// APIEndpoint is the internal API URL of the Glance service.
	// +optional
	APIEndpoint string `json:"apiEndpoint,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Endpoint",type=string,JSONPath=`.status.apiEndpoint`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Glance is the Schema for the glances API.
type Glance struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GlanceSpec   `json:"spec,omitempty"`
	Status GlanceStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// GlanceList contains a list of Glance.
type GlanceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Glance `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Glance{}, &GlanceList{})
}
