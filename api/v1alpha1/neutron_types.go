package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NeutronSpec defines the desired state of the Neutron (Networking) service.
type NeutronSpec struct {
	ServiceTemplate `json:",inline"`

	// Database configures the Neutron database connection.
	// +optional
	Database DatabaseConfig `json:"database,omitempty"`

	// MessageQueue configures the RabbitMQ connection.
	// +optional
	MessageQueue RabbitMQConfig `json:"messageQueue,omitempty"`

	// Mechanism selects the ML2 mechanism driver.
	// +kubebuilder:validation:Enum=ovn;ovs
	// +kubebuilder:default="ovn"
	// +optional
	Mechanism string `json:"mechanism,omitempty"`

	// ExternalBridge is the OVS bridge used for external (provider) network connectivity.
	// +kubebuilder:default="br-ex"
	// +optional
	ExternalBridge string `json:"externalBridge,omitempty"`

	// TunnelType selects the tunnel encapsulation type for overlay networks.
	// +kubebuilder:validation:Enum=geneve;vxlan;gre
	// +kubebuilder:default="geneve"
	// +optional
	TunnelType string `json:"tunnelType,omitempty"`
}

// NeutronStatus defines the observed state of Neutron.
type NeutronStatus struct {
	CommonStatus `json:",inline"`

	// APIEndpoint is the internal API URL of the Neutron service.
	// +optional
	APIEndpoint string `json:"apiEndpoint,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Mechanism",type=string,JSONPath=`.spec.mechanism`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Neutron is the Schema for the neutrons API.
type Neutron struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NeutronSpec   `json:"spec,omitempty"`
	Status NeutronStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// NeutronList contains a list of Neutron.
type NeutronList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Neutron `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Neutron{}, &NeutronList{})
}
