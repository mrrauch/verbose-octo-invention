package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RabbitMQServiceSpec defines the desired state of the RabbitMQ deployment.
type RabbitMQServiceSpec struct {
	ServiceTemplate `json:",inline"`

	// Storage defines the persistent storage configuration.
	// +optional
	Storage StorageConfig `json:"storage,omitempty"`

	// ClusterEnabled enables RabbitMQ clustering with quorum queues.
	// +kubebuilder:default=false
	// +optional
	ClusterEnabled bool `json:"clusterEnabled,omitempty"`
}

// RabbitMQStatus defines the observed state of RabbitMQ.
type RabbitMQStatus struct {
	CommonStatus `json:",inline"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// RabbitMQ is the Schema for the rabbitmqs API.
type RabbitMQ struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RabbitMQServiceSpec `json:"spec,omitempty"`
	Status RabbitMQStatus      `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// RabbitMQList contains a list of RabbitMQ.
type RabbitMQList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RabbitMQ `json:"items"`
}

func init() {
	SchemeBuilder.Register(&RabbitMQ{}, &RabbitMQList{})
}
