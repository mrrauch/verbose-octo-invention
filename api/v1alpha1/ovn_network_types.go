package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// OVNNetworkSpec defines the desired state of the OVN networking backend.
type OVNNetworkSpec struct {
	// NorthboundDBReplicas is the number of OVN Northbound DB replicas.
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=1
	// +optional
	NorthboundDBReplicas *int32 `json:"northboundDBReplicas,omitempty"`

	// SouthboundDBReplicas is the number of OVN Southbound DB replicas.
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=1
	// +optional
	SouthboundDBReplicas *int32 `json:"southboundDBReplicas,omitempty"`

	// NorthdReplicas is the number of ovn-northd replicas.
	// +kubebuilder:default=1
	// +optional
	NorthdReplicas *int32 `json:"northdReplicas,omitempty"`

	// DBStorage defines persistent storage for the OVN databases.
	// +optional
	DBStorage StorageConfig `json:"dbStorage,omitempty"`

	// EnableRaftClustering enables Raft-based HA for OVN databases (requires 3+ replicas).
	// +kubebuilder:default=false
	// +optional
	EnableRaftClustering bool `json:"enableRaftClustering,omitempty"`

	// SharedWithCNI indicates whether Neutron should share the OVN control plane
	// with the Kubernetes CNI (e.g., OVN-Kubernetes, Kube-OVN).
	// +kubebuilder:default=false
	// +optional
	SharedWithCNI bool `json:"sharedWithCNI,omitempty"`

	// ExternalNorthboundDB is the connection string for an external OVN NB DB
	// (used when sharedWithCNI is true).
	// +optional
	ExternalNorthboundDB string `json:"externalNorthboundDB,omitempty"`
}

// OVNNetworkStatus defines the observed state of OVNNetwork.
type OVNNetworkStatus struct {
	CommonStatus `json:",inline"`

	// NorthboundDBEndpoint is the connection string for the OVN NB DB.
	// +optional
	NorthboundDBEndpoint string `json:"northboundDBEndpoint,omitempty"`

	// SouthboundDBEndpoint is the connection string for the OVN SB DB.
	// +optional
	SouthboundDBEndpoint string `json:"southboundDBEndpoint,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// OVNNetwork is the Schema for the ovnnetworks API.
type OVNNetwork struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OVNNetworkSpec   `json:"spec,omitempty"`
	Status OVNNetworkStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// OVNNetworkList contains a list of OVNNetwork.
type OVNNetworkList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OVNNetwork `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OVNNetwork{}, &OVNNetworkList{})
}
