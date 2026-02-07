package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// IronicSpec defines the desired state of the Ironic (Bare Metal) service.
type IronicSpec struct {
	ServiceTemplate `json:",inline"`

	// Database configures the Ironic database connection.
	// +optional
	Database DatabaseConfig `json:"database,omitempty"`

	// MessageQueue configures the RabbitMQ connection.
	// +optional
	MessageQueue RabbitMQConfig `json:"messageQueue,omitempty"`

	// ConductorReplicas is the number of ironic-conductor replicas.
	// +kubebuilder:default=1
	// +optional
	ConductorReplicas *int32 `json:"conductorReplicas,omitempty"`

	// EnableInspector enables the ironic-inspector sub-service for hardware introspection.
	// +kubebuilder:default=true
	// +optional
	EnableInspector bool `json:"enableInspector,omitempty"`

	// DHCPRange is the DHCP IP range for PXE provisioning.
	// +optional
	DHCPRange string `json:"dhcpRange,omitempty"`

	// DeployInterface selects the deployment mechanism.
	// +kubebuilder:validation:Enum=direct;iscsi
	// +kubebuilder:default="direct"
	// +optional
	DeployInterface string `json:"deployInterface,omitempty"`

	// EnabledDrivers lists the enabled hardware drivers.
	// +kubebuilder:default={"ipmi","redfish"}
	// +optional
	EnabledDrivers []string `json:"enabledDrivers,omitempty"`
}

// IronicStatus defines the observed state of Ironic.
type IronicStatus struct {
	CommonStatus `json:",inline"`

	// APIEndpoint is the internal API URL of the Ironic service.
	// +optional
	APIEndpoint string `json:"apiEndpoint,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Ironic is the Schema for the ironics API.
type Ironic struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   IronicSpec   `json:"spec,omitempty"`
	Status IronicStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// IronicList contains a list of Ironic.
type IronicList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Ironic `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Ironic{}, &IronicList{})
}
