package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// HorizonSpec defines the desired state of the Horizon (Dashboard) service.
type HorizonSpec struct {
	ServiceTemplate `json:",inline"`

	// Hostname is the external hostname for the Horizon Ingress.
	// +optional
	Hostname string `json:"hostname,omitempty"`
}

// HorizonStatus defines the observed state of Horizon.
type HorizonStatus struct {
	CommonStatus `json:",inline"`

	// URL is the external URL of the Horizon dashboard.
	// +optional
	URL string `json:"url,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="URL",type=string,JSONPath=`.status.url`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Horizon is the Schema for the horizons API.
type Horizon struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HorizonSpec   `json:"spec,omitempty"`
	Status HorizonStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// HorizonList contains a list of Horizon.
type HorizonList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Horizon `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Horizon{}, &HorizonList{})
}
