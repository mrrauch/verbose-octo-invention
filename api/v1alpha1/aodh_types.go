package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AodhSpec defines the desired state of the Aodh (Alarming) service.
type AodhSpec struct {
	ServiceTemplate `json:",inline"`

	// Database configures the Aodh database connection.
	// +optional
	Database DatabaseConfig `json:"database,omitempty"`

	// MessageQueue configures the RabbitMQ connection.
	// +optional
	MessageQueue RabbitMQConfig `json:"messageQueue,omitempty"`

	// EvaluatorReplicas is the number of aodh-evaluator replicas.
	// +kubebuilder:default=1
	// +optional
	EvaluatorReplicas *int32 `json:"evaluatorReplicas,omitempty"`

	// NotifierReplicas is the number of aodh-notifier replicas.
	// +kubebuilder:default=1
	// +optional
	NotifierReplicas *int32 `json:"notifierReplicas,omitempty"`
}

// AodhStatus defines the observed state of Aodh.
type AodhStatus struct {
	CommonStatus `json:",inline"`

	// APIEndpoint is the internal API URL of the Aodh service.
	// +optional
	APIEndpoint string `json:"apiEndpoint,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Aodh is the Schema for the aodhs API.
type Aodh struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AodhSpec   `json:"spec,omitempty"`
	Status AodhStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// AodhList contains a list of Aodh.
type AodhList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Aodh `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Aodh{}, &AodhList{})
}
