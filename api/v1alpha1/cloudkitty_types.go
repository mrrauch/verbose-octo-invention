package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CloudKittySpec defines the desired state of the CloudKitty (Rating/Billing) service.
type CloudKittySpec struct {
	ServiceTemplate `json:",inline"`

	// Database configures the CloudKitty database connection.
	// +optional
	Database DatabaseConfig `json:"database,omitempty"`

	// MessageQueue configures the RabbitMQ connection.
	// +optional
	MessageQueue RabbitMQConfig `json:"messageQueue,omitempty"`

	// ProcessorReplicas is the number of cloudkitty-processor replicas.
	// +kubebuilder:default=1
	// +optional
	ProcessorReplicas *int32 `json:"processorReplicas,omitempty"`

	// Collector selects the data collection backend.
	// +kubebuilder:validation:Enum=ceilometer;prometheus;gnocchi
	// +kubebuilder:default="prometheus"
	// +optional
	Collector string `json:"collector,omitempty"`
}

// CloudKittyStatus defines the observed state of CloudKitty.
type CloudKittyStatus struct {
	CommonStatus `json:",inline"`

	// APIEndpoint is the internal API URL of the CloudKitty service.
	// +optional
	APIEndpoint string `json:"apiEndpoint,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// CloudKitty is the Schema for the cloudkittys API.
type CloudKitty struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CloudKittySpec   `json:"spec,omitempty"`
	Status CloudKittyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// CloudKittyList contains a list of CloudKitty.
type CloudKittyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CloudKitty `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CloudKitty{}, &CloudKittyList{})
}
