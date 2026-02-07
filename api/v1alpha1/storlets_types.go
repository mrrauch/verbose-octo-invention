package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// StorletsSpec defines the desired state of the Storlets (Compute Inside Object Storage) service.
type StorletsSpec struct {
	ServiceTemplate `json:",inline"`

	// GatewayReplicas is the number of storlets Docker gateway replicas.
	// +kubebuilder:default=1
	// +optional
	GatewayReplicas *int32 `json:"gatewayReplicas,omitempty"`
}

// StorletsStatus defines the observed state of Storlets.
type StorletsStatus struct {
	CommonStatus `json:",inline"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Storlets is the Schema for the storlets API.
type Storlets struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   StorletsSpec   `json:"spec,omitempty"`
	Status StorletsStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// StorletsList contains a list of Storlets.
type StorletsList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Storlets `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Storlets{}, &StorletsList{})
}
