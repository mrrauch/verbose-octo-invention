package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// FreezerSpec defines the desired state of the Freezer (Backup/Restore) service.
type FreezerSpec struct {
	ServiceTemplate `json:",inline"`

	// Database configures the Freezer database connection.
	// +optional
	Database DatabaseConfig `json:"database,omitempty"`

	// StorageBackend selects where backups are stored.
	// +kubebuilder:validation:Enum=swift;ssh;local
	// +kubebuilder:default="swift"
	// +optional
	StorageBackend string `json:"storageBackend,omitempty"`

	// SchedulerReplicas is the number of freezer-scheduler replicas.
	// +kubebuilder:default=1
	// +optional
	SchedulerReplicas *int32 `json:"schedulerReplicas,omitempty"`
}

// FreezerStatus defines the observed state of Freezer.
type FreezerStatus struct {
	CommonStatus `json:",inline"`

	// APIEndpoint is the internal API URL of the Freezer service.
	// +optional
	APIEndpoint string `json:"apiEndpoint,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Freezer is the Schema for the freezers API.
type Freezer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   FreezerSpec   `json:"spec,omitempty"`
	Status FreezerStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// FreezerList contains a list of Freezer.
type FreezerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Freezer `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Freezer{}, &FreezerList{})
}
