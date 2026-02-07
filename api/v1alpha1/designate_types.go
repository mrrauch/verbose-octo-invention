package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DesignateSpec defines the desired state of the Designate (DNS) service.
type DesignateSpec struct {
	ServiceTemplate `json:",inline"`

	// Database configures the Designate database connection.
	// +optional
	Database DatabaseConfig `json:"database,omitempty"`

	// MessageQueue configures the RabbitMQ connection.
	// +optional
	MessageQueue RabbitMQConfig `json:"messageQueue,omitempty"`

	// DNSBackend selects the DNS server backend.
	// +kubebuilder:validation:Enum=bind9;powerdns
	// +kubebuilder:default="bind9"
	// +optional
	DNSBackend string `json:"dnsBackend,omitempty"`

	// WorkerReplicas is the number of designate-worker replicas.
	// +kubebuilder:default=1
	// +optional
	WorkerReplicas *int32 `json:"workerReplicas,omitempty"`

	// EnableNeutronIntegration enables automatic DNS record creation for Neutron ports.
	// +kubebuilder:default=true
	// +optional
	EnableNeutronIntegration bool `json:"enableNeutronIntegration,omitempty"`
}

// DesignateStatus defines the observed state of Designate.
type DesignateStatus struct {
	CommonStatus `json:",inline"`

	// APIEndpoint is the internal API URL of the Designate service.
	// +optional
	APIEndpoint string `json:"apiEndpoint,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Backend",type=string,JSONPath=`.spec.dnsBackend`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Designate is the Schema for the designates API.
type Designate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DesignateSpec   `json:"spec,omitempty"`
	Status DesignateStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// DesignateList contains a list of Designate.
type DesignateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Designate `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Designate{}, &DesignateList{})
}
