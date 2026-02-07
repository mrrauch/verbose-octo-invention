package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// OctaviaSpec defines the desired state of the Octavia (Load Balancer) service.
type OctaviaSpec struct {
	ServiceTemplate `json:",inline"`

	// Database configures the Octavia database connection.
	// +optional
	Database DatabaseConfig `json:"database,omitempty"`

	// MessageQueue configures the RabbitMQ connection.
	// +optional
	MessageQueue RabbitMQConfig `json:"messageQueue,omitempty"`

	// Provider selects the load balancer provider.
	// +kubebuilder:validation:Enum=amphora;ovn
	// +kubebuilder:default="amphora"
	// +optional
	Provider string `json:"provider,omitempty"`

	// AmphoraImageSecretName references a Secret containing the amphora image URL/credentials.
	// Only used when provider is "amphora".
	// +optional
	AmphoraImageSecretName string `json:"amphoraImageSecretName,omitempty"`

	// ManagementNetworkCIDR is the CIDR for the Octavia management network.
	// +kubebuilder:default="172.23.0.0/24"
	// +optional
	ManagementNetworkCIDR string `json:"managementNetworkCIDR,omitempty"`

	// WorkerReplicas is the number of octavia-worker replicas.
	// +kubebuilder:default=1
	// +optional
	WorkerReplicas *int32 `json:"workerReplicas,omitempty"`

	// HealthManagerReplicas is the number of octavia-health-manager replicas.
	// +kubebuilder:default=1
	// +optional
	HealthManagerReplicas *int32 `json:"healthManagerReplicas,omitempty"`
}

// OctaviaStatus defines the observed state of Octavia.
type OctaviaStatus struct {
	CommonStatus `json:",inline"`

	// APIEndpoint is the internal API URL of the Octavia service.
	// +optional
	APIEndpoint string `json:"apiEndpoint,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Provider",type=string,JSONPath=`.spec.provider`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Octavia is the Schema for the octavias API.
type Octavia struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OctaviaSpec   `json:"spec,omitempty"`
	Status OctaviaStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// OctaviaList contains a list of Octavia.
type OctaviaList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Octavia `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Octavia{}, &OctaviaList{})
}
