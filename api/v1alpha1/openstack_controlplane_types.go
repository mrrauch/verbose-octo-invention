package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// OpenStackControlPlaneSpec defines the desired state of the entire OpenStack deployment.
type OpenStackControlPlaneSpec struct {
	// Region is the OpenStack region name.
	// +kubebuilder:default="RegionOne"
	// +optional
	Region string `json:"region,omitempty"`

	// StorageBackend selects the storage backend for OpenStack services.
	// +kubebuilder:validation:Enum=ceph;lvm;pvc
	// +kubebuilder:default="pvc"
	// +optional
	StorageBackend string `json:"storageBackend,omitempty"`

	// NetworkBackend selects the networking backend.
	// +kubebuilder:validation:Enum=ovn;ovs
	// +kubebuilder:default="ovn"
	// +optional
	NetworkBackend string `json:"networkBackend,omitempty"`

	// TLS configures TLS for all service endpoints.
	// +optional
	TLS TLSConfig `json:"tls,omitempty"`

	// MariaDB defines the MariaDB deployment spec.
	MariaDB MariaDBSpec `json:"mariadb,omitempty"`

	// RabbitMQ defines the RabbitMQ deployment spec.
	RabbitMQ RabbitMQServiceSpec `json:"rabbitmq,omitempty"`

	// Memcached defines the Memcached deployment spec.
	Memcached MemcachedSpec `json:"memcached,omitempty"`

	// Keystone defines the Keystone (Identity) deployment spec.
	Keystone KeystoneSpec `json:"keystone,omitempty"`

	// Glance defines the Glance (Image) deployment spec.
	Glance GlanceSpec `json:"glance,omitempty"`

	// Placement defines the Placement deployment spec.
	Placement PlacementSpec `json:"placement,omitempty"`

	// Neutron defines the Neutron (Networking) deployment spec.
	Neutron NeutronSpec `json:"neutron,omitempty"`

	// Nova defines the Nova (Compute) deployment spec.
	Nova NovaSpec `json:"nova,omitempty"`

	// Cinder defines the Cinder (Block Storage) deployment spec.
	// +optional
	Cinder *CinderSpec `json:"cinder,omitempty"`

	// Heat defines the Heat (Orchestration) deployment spec.
	// +optional
	Heat *HeatSpec `json:"heat,omitempty"`

	// Horizon defines the Horizon (Dashboard) deployment spec.
	// +optional
	Horizon *HorizonSpec `json:"horizon,omitempty"`

	// CephStorage defines the Ceph storage backend configuration.
	// Required when storageBackend is "ceph".
	// +optional
	CephStorage *CephStorageSpec `json:"cephStorage,omitempty"`

	// OVNNetwork defines the OVN networking configuration.
	// Required when networkBackend is "ovn".
	// +optional
	OVNNetwork *OVNNetworkSpec `json:"ovnNetwork,omitempty"`
}

// ControlPlanePhase represents the current phase of the deployment.
// +kubebuilder:validation:Enum=Pending;Infrastructure;Identity;CoreServices;Compute;OptionalServices;Ready;Failed
type ControlPlanePhase string

const (
	ControlPlanePhasePending            ControlPlanePhase = "Pending"
	ControlPlanePhaseInfrastructure     ControlPlanePhase = "Infrastructure"
	ControlPlanePhaseIdentity           ControlPlanePhase = "Identity"
	ControlPlanePhaseCoreServices       ControlPlanePhase = "CoreServices"
	ControlPlanePhaseCompute            ControlPlanePhase = "Compute"
	ControlPlanePhaseOptionalServices   ControlPlanePhase = "OptionalServices"
	ControlPlanePhaseReady              ControlPlanePhase = "Ready"
	ControlPlanePhaseFailed             ControlPlanePhase = "Failed"
)

// OpenStackControlPlaneStatus defines the observed state of OpenStackControlPlane.
type OpenStackControlPlaneStatus struct {
	CommonStatus `json:",inline"`

	// Phase indicates the current deployment phase.
	// +optional
	Phase ControlPlanePhase `json:"phase,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=oscp
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// OpenStackControlPlane is the top-level CR that orchestrates an entire OpenStack deployment.
type OpenStackControlPlane struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OpenStackControlPlaneSpec   `json:"spec,omitempty"`
	Status OpenStackControlPlaneStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// OpenStackControlPlaneList contains a list of OpenStackControlPlane.
type OpenStackControlPlaneList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OpenStackControlPlane `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OpenStackControlPlane{}, &OpenStackControlPlaneList{})
}
