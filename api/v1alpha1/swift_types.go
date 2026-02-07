package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SwiftSpec defines the desired state of the Swift (Object Storage) service.
type SwiftSpec struct {
	ServiceTemplate `json:",inline"`

	// Mode selects whether to deploy native Swift or use Ceph RGW as a Swift-compatible backend.
	// +kubebuilder:validation:Enum=native;ceph-rgw
	// +kubebuilder:default="native"
	// +optional
	Mode string `json:"mode,omitempty"`

	// ProxyReplicas is the number of swift-proxy replicas.
	// +kubebuilder:default=1
	// +optional
	ProxyReplicas *int32 `json:"proxyReplicas,omitempty"`

	// StoragePolicy defines the default storage policy (replica count, etc.).
	// +optional
	StoragePolicy *SwiftStoragePolicy `json:"storagePolicy,omitempty"`

	// Storage defines PVC settings for Swift account/container/object servers.
	// +optional
	Storage StorageConfig `json:"storage,omitempty"`
}

// SwiftStoragePolicy defines a Swift storage policy.
type SwiftStoragePolicy struct {
	// ReplicaCount is the number of object replicas.
	// +kubebuilder:default=3
	ReplicaCount int32 `json:"replicaCount,omitempty"`
}

// SwiftStatus defines the observed state of Swift.
type SwiftStatus struct {
	CommonStatus `json:",inline"`

	// APIEndpoint is the internal API URL of the Swift proxy.
	// +optional
	APIEndpoint string `json:"apiEndpoint,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Mode",type=string,JSONPath=`.spec.mode`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Swift is the Schema for the swifts API.
type Swift struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SwiftSpec   `json:"spec,omitempty"`
	Status SwiftStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SwiftList contains a list of Swift.
type SwiftList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Swift `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Swift{}, &SwiftList{})
}
