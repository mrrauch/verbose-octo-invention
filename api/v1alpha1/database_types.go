package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DatabaseEngine selects the database backend.
// +kubebuilder:validation:Enum=postgresql
type DatabaseEngine string

const (
	DatabaseEnginePostgreSQL DatabaseEngine = "postgresql"
)

// DatabaseSpec defines the desired state of a Database deployment.
type DatabaseSpec struct {
	ServiceTemplate `json:",inline"`

	// Engine selects the database backend.
	// +kubebuilder:default="postgresql"
	// +optional
	Engine DatabaseEngine `json:"engine,omitempty"`

	// Storage configures the persistent volume for the database.
	// +optional
	Storage StorageConfig `json:"storage,omitempty"`

	// HAEnabled enables high-availability clustering.
	// +optional
	HAEnabled bool `json:"haEnabled,omitempty"`
}

// DatabaseStatus defines the observed state of Database.
type DatabaseStatus struct {
	CommonStatus `json:",inline"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Engine",type=string,JSONPath=`.spec.engine`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Database is the Schema for the databases API.
type Database struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DatabaseSpec   `json:"spec,omitempty"`
	Status DatabaseStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// DatabaseList contains a list of Database.
type DatabaseList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Database `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Database{}, &DatabaseList{})
}
