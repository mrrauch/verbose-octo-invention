package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SkylineSpec defines the desired state of the Skyline (Modern Dashboard) service.
type SkylineSpec struct {
	ServiceTemplate `json:",inline"`

	// Hostname is the external hostname for the Skyline Ingress.
	// +optional
	Hostname string `json:"hostname,omitempty"`

	// APIServerReplicas is the number of skyline-apiserver replicas.
	// +kubebuilder:default=1
	// +optional
	APIServerReplicas *int32 `json:"apiServerReplicas,omitempty"`
}

// SkylineStatus defines the observed state of Skyline.
type SkylineStatus struct {
	CommonStatus `json:",inline"`

	// URL is the external URL of the Skyline dashboard.
	// +optional
	URL string `json:"url,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="URL",type=string,JSONPath=`.status.url`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Skyline is the Schema for the skylines API.
type Skyline struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SkylineSpec   `json:"spec,omitempty"`
	Status SkylineStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SkylineList contains a list of Skyline.
type SkylineList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Skyline `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Skyline{}, &SkylineList{})
}
