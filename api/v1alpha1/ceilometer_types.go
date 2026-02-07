package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CeilometerSpec defines the desired state of the Ceilometer (Telemetry) service.
type CeilometerSpec struct {
	ServiceTemplate `json:",inline"`

	// MetricsBackend selects where Ceilometer publishes metrics.
	// +kubebuilder:validation:Enum=prometheus;gnocchi
	// +kubebuilder:default="prometheus"
	// +optional
	MetricsBackend string `json:"metricsBackend,omitempty"`

	// CentralAgentReplicas is the number of ceilometer-central-agent replicas.
	// +kubebuilder:default=1
	// +optional
	CentralAgentReplicas *int32 `json:"centralAgentReplicas,omitempty"`

	// NotificationAgentReplicas is the number of ceilometer-notification-agent replicas.
	// +kubebuilder:default=1
	// +optional
	NotificationAgentReplicas *int32 `json:"notificationAgentReplicas,omitempty"`

	// EnableComputeAgent deploys ceilometer-compute-agent as a DaemonSet on compute nodes.
	// +kubebuilder:default=true
	// +optional
	EnableComputeAgent bool `json:"enableComputeAgent,omitempty"`

	// MessageQueue configures the RabbitMQ connection (for consuming notifications).
	// +optional
	MessageQueue RabbitMQConfig `json:"messageQueue,omitempty"`
}

// CeilometerStatus defines the observed state of Ceilometer.
type CeilometerStatus struct {
	CommonStatus `json:",inline"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Backend",type=string,JSONPath=`.spec.metricsBackend`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Ceilometer is the Schema for the ceilometers API.
type Ceilometer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CeilometerSpec   `json:"spec,omitempty"`
	Status CeilometerStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// CeilometerList contains a list of Ceilometer.
type CeilometerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Ceilometer `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Ceilometer{}, &CeilometerList{})
}
