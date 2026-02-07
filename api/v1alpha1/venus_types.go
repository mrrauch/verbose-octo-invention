package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// VenusSpec defines the desired state of the Venus (Log Management) service.
type VenusSpec struct {
	ServiceTemplate `json:",inline"`

	// Database configures the Venus database connection.
	// +optional
	Database DatabaseConfig `json:"database,omitempty"`

	// ElasticsearchURL is the URL of the Elasticsearch backend.
	// +optional
	ElasticsearchURL string `json:"elasticsearchURL,omitempty"`

	// ManagerReplicas is the number of venus-manager replicas.
	// +kubebuilder:default=1
	// +optional
	ManagerReplicas *int32 `json:"managerReplicas,omitempty"`
}

// VenusStatus defines the observed state of Venus.
type VenusStatus struct {
	CommonStatus `json:",inline"`

	// APIEndpoint is the internal API URL of the Venus service.
	// +optional
	APIEndpoint string `json:"apiEndpoint,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Venus is the Schema for the venuses API.
type Venus struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VenusSpec   `json:"spec,omitempty"`
	Status VenusStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// VenusList contains a list of Venus.
type VenusList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Venus `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Venus{}, &VenusList{})
}
