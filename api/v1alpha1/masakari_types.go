package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MasakariSpec defines the desired state of the Masakari (Instance HA) service.
type MasakariSpec struct {
	ServiceTemplate `json:",inline"`

	// Database configures the Masakari database connection.
	// +optional
	Database DatabaseConfig `json:"database,omitempty"`

	// MessageQueue configures the RabbitMQ connection.
	// +optional
	MessageQueue RabbitMQConfig `json:"messageQueue,omitempty"`

	// EngineReplicas is the number of masakari-engine replicas.
	// +kubebuilder:default=1
	// +optional
	EngineReplicas *int32 `json:"engineReplicas,omitempty"`

	// EnableMonitors deploys masakari-monitors as a DaemonSet on compute nodes.
	// +kubebuilder:default=true
	// +optional
	EnableMonitors bool `json:"enableMonitors,omitempty"`

	// RecoveryMethod selects how instances are recovered on host failure.
	// +kubebuilder:validation:Enum=evacuate;migrate
	// +kubebuilder:default="evacuate"
	// +optional
	RecoveryMethod string `json:"recoveryMethod,omitempty"`
}

// MasakariStatus defines the observed state of Masakari.
type MasakariStatus struct {
	CommonStatus `json:",inline"`

	// APIEndpoint is the internal API URL of the Masakari service.
	// +optional
	APIEndpoint string `json:"apiEndpoint,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Masakari is the Schema for the masakaris API.
type Masakari struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MasakariSpec   `json:"spec,omitempty"`
	Status MasakariStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// MasakariList contains a list of Masakari.
type MasakariList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Masakari `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Masakari{}, &MasakariList{})
}
