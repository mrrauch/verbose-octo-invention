package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// WatcherSpec defines the desired state of the Watcher (Resource Optimization) service.
type WatcherSpec struct {
	ServiceTemplate `json:",inline"`

	// Database configures the Watcher database connection.
	// +optional
	Database DatabaseConfig `json:"database,omitempty"`

	// MessageQueue configures the RabbitMQ connection.
	// +optional
	MessageQueue RabbitMQConfig `json:"messageQueue,omitempty"`

	// DecisionEngineReplicas is the number of watcher-decision-engine replicas.
	// +kubebuilder:default=1
	// +optional
	DecisionEngineReplicas *int32 `json:"decisionEngineReplicas,omitempty"`

	// ApplierReplicas is the number of watcher-applier replicas.
	// +kubebuilder:default=1
	// +optional
	ApplierReplicas *int32 `json:"applierReplicas,omitempty"`
}

// WatcherStatus defines the observed state of Watcher.
type WatcherStatus struct {
	CommonStatus `json:",inline"`

	// APIEndpoint is the internal API URL of the Watcher service.
	// +optional
	APIEndpoint string `json:"apiEndpoint,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Watcher is the Schema for the watchers API.
type Watcher struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   WatcherSpec   `json:"spec,omitempty"`
	Status WatcherStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// WatcherList contains a list of Watcher.
type WatcherList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Watcher `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Watcher{}, &WatcherList{})
}
