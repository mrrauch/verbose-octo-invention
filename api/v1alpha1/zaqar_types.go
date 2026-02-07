package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ZaqarSpec defines the desired state of the Zaqar (Messaging) service.
type ZaqarSpec struct {
	ServiceTemplate `json:",inline"`

	// MessageStore selects the backend for queue message storage.
	// +kubebuilder:validation:Enum=mongodb;redis;swift
	// +kubebuilder:default="redis"
	// +optional
	MessageStore string `json:"messageStore,omitempty"`

	// EnableWebsocket enables the websocket transport.
	// +kubebuilder:default=true
	// +optional
	EnableWebsocket bool `json:"enableWebsocket,omitempty"`
}

// ZaqarStatus defines the observed state of Zaqar.
type ZaqarStatus struct {
	CommonStatus `json:",inline"`

	// APIEndpoint is the internal API URL of the Zaqar service.
	// +optional
	APIEndpoint string `json:"apiEndpoint,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Zaqar is the Schema for the zaqars API.
type Zaqar struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ZaqarSpec   `json:"spec,omitempty"`
	Status ZaqarStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ZaqarList contains a list of Zaqar.
type ZaqarList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Zaqar `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Zaqar{}, &ZaqarList{})
}
