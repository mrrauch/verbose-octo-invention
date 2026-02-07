package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PlacementSpec defines the desired state of the Placement service.
type PlacementSpec struct {
	ServiceTemplate `json:",inline"`

	// Database configures the Placement database connection.
	// +optional
	Database DatabaseConfig `json:"database,omitempty"`

	// PublicHostname overrides the generated external hostname for this API service.
	// +optional
	PublicHostname string `json:"publicHostname,omitempty"`

	// GatewayRef overrides the control-plane default gatewayRef for this service.
	// +optional
	GatewayRef GatewayRef `json:"gatewayRef,omitempty"`
}

// PlacementStatus defines the observed state of Placement.
type PlacementStatus struct {
	CommonStatus `json:",inline"`

	// APIEndpoint is the internal API URL of the Placement service.
	// +optional
	APIEndpoint string `json:"apiEndpoint,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Endpoint",type=string,JSONPath=`.status.apiEndpoint`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Placement is the Schema for the placements API.
type Placement struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PlacementSpec   `json:"spec,omitempty"`
	Status PlacementStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// PlacementList contains a list of Placement.
type PlacementList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Placement `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Placement{}, &PlacementList{})
}
