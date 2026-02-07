package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CephStorageSpec defines the desired state of the Ceph storage backend.
type CephStorageSpec struct {
	// Mode selects whether to manage Ceph via Rook or connect to an external cluster.
	// +kubebuilder:validation:Enum=rook;external
	// +kubebuilder:default="rook"
	Mode string `json:"mode,omitempty"`

	// Rook defines Rook-managed Ceph cluster settings.
	// Required when mode is "rook".
	// +optional
	Rook *RookCephConfig `json:"rook,omitempty"`

	// External defines connection settings for a pre-existing Ceph cluster.
	// Required when mode is "external".
	// +optional
	External *ExternalCephConfig `json:"external,omitempty"`

	// Pools defines the RBD pools to create for OpenStack services.
	// +optional
	Pools []CephPool `json:"pools,omitempty"`
}

// RookCephConfig defines settings for a Rook-managed Ceph cluster.
type RookCephConfig struct {
	// Namespace is the namespace where Rook will deploy the Ceph cluster.
	// +kubebuilder:default="rook-ceph"
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// MonCount is the number of Ceph monitors.
	// +kubebuilder:default=3
	// +kubebuilder:validation:Minimum=1
	// +optional
	MonCount int32 `json:"monCount,omitempty"`

	// OSDCount is the number of OSD daemons.
	// +kubebuilder:default=3
	// +kubebuilder:validation:Minimum=1
	// +optional
	OSDCount int32 `json:"osdCount,omitempty"`

	// StorageDeviceFilter is a regex to select devices for OSDs (e.g., "^sd[b-z]").
	// +optional
	StorageDeviceFilter string `json:"storageDeviceFilter,omitempty"`
}

// ExternalCephConfig defines connection parameters for an external Ceph cluster.
type ExternalCephConfig struct {
	// CephConfSecretName references a Secret containing ceph.conf.
	CephConfSecretName string `json:"cephConfSecretName"`

	// KeyringSecretName references a Secret containing the Ceph keyring.
	KeyringSecretName string `json:"keyringSecretName"`

	// MonitorHosts is a comma-separated list of monitor addresses.
	MonitorHosts string `json:"monitorHosts"`
}

// CephPool defines an RBD pool.
type CephPool struct {
	// Name of the pool.
	Name string `json:"name"`

	// ReplicaCount is the number of data replicas.
	// +kubebuilder:default=3
	// +optional
	ReplicaCount int32 `json:"replicaCount,omitempty"`

	// Purpose identifies which OpenStack service uses this pool.
	// +kubebuilder:validation:Enum=cinder-volumes;glance-images;nova-ephemeral
	// +optional
	Purpose string `json:"purpose,omitempty"`
}

// CephStorageStatus defines the observed state of CephStorage.
type CephStorageStatus struct {
	CommonStatus `json:",inline"`

	// ClusterID is the Ceph FSID / cluster identifier.
	// +optional
	ClusterID string `json:"clusterID,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Mode",type=string,JSONPath=`.spec.mode`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// CephStorage is the Schema for the cephstorages API.
type CephStorage struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CephStorageSpec   `json:"spec,omitempty"`
	Status CephStorageStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// CephStorageList contains a list of CephStorage.
type CephStorageList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CephStorage `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CephStorage{}, &CephStorageList{})
}
