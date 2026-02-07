package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MariaDBSpec defines the desired state of the MariaDB deployment.
type MariaDBSpec struct {
	ServiceTemplate `json:",inline"`

	// Storage defines the persistent storage configuration.
	// +optional
	Storage StorageConfig `json:"storage,omitempty"`

	// GaleraEnabled enables Galera multi-master clustering.
	// +kubebuilder:default=false
	// +optional
	GaleraEnabled bool `json:"galeraEnabled,omitempty"`
}

// MariaDBStatus defines the observed state of MariaDB.
type MariaDBStatus struct {
	CommonStatus `json:",inline"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// MariaDB is the Schema for the mariadbs API.
type MariaDB struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MariaDBSpec   `json:"spec,omitempty"`
	Status MariaDBStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// MariaDBList contains a list of MariaDB.
type MariaDBList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MariaDB `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MariaDB{}, &MariaDBList{})
}
