package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// KeystoneSpec defines the desired state of the Keystone (Identity) service.
type KeystoneSpec struct {
	ServiceTemplate `json:",inline"`

	// Database configures the Keystone database connection.
	// +optional
	Database DatabaseConfig `json:"database,omitempty"`

	// MessageQueue configures the RabbitMQ connection.
	// +optional
	MessageQueue RabbitMQConfig `json:"messageQueue,omitempty"`

	// AdminPasswordSecretName references the Secret containing the admin password.
	// Auto-generated if empty.
	// +optional
	AdminPasswordSecretName string `json:"adminPasswordSecretName,omitempty"`

	// FernetKeyRotationInterval is the interval (in hours) between Fernet key rotations.
	// +kubebuilder:default=24
	// +optional
	FernetKeyRotationInterval int32 `json:"fernetKeyRotationInterval,omitempty"`

	// PublicHostname overrides the generated external hostname for this API service.
	// +optional
	PublicHostname string `json:"publicHostname,omitempty"`

	// GatewayRef overrides the control-plane default gatewayRef for this service.
	// +optional
	GatewayRef GatewayRef `json:"gatewayRef,omitempty"`
}

// KeystoneStatus defines the observed state of Keystone.
type KeystoneStatus struct {
	CommonStatus `json:",inline"`

	// APIEndpoint is the internal API URL of the Keystone service.
	// +optional
	APIEndpoint string `json:"apiEndpoint,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Endpoint",type=string,JSONPath=`.status.apiEndpoint`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Keystone is the Schema for the keystones API.
type Keystone struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KeystoneSpec   `json:"spec,omitempty"`
	Status KeystoneStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// KeystoneList contains a list of Keystone.
type KeystoneList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Keystone `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Keystone{}, &KeystoneList{})
}
