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

	// GatewayRef defines the default Gateway used for public API routes.
	// +optional
	GatewayRef GatewayRef `json:"gatewayRef,omitempty"`

	// PublicDomain is used to generate default hostnames:
	// keystone.<publicDomain>, glance.<publicDomain>, etc.
	// +kubebuilder:default="openstack.local"
	// +optional
	PublicDomain string `json:"publicDomain,omitempty"`

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

	// --- Extended Services: Tier 1 (Commonly Deployed) ---

	// Swift defines the Swift (Object Storage) deployment spec.
	// +optional
	Swift *SwiftSpec `json:"swift,omitempty"`

	// Barbican defines the Barbican (Key Manager) deployment spec.
	// +optional
	Barbican *BarbicanSpec `json:"barbican,omitempty"`

	// Octavia defines the Octavia (Load Balancer) deployment spec.
	// Requires: Nova, Neutron, Glance. Optional: Barbican.
	// +optional
	Octavia *OctaviaSpec `json:"octavia,omitempty"`

	// Designate defines the Designate (DNS) deployment spec.
	// Requires: Neutron.
	// +optional
	Designate *DesignateSpec `json:"designate,omitempty"`

	// Manila defines the Manila (Shared Filesystems) deployment spec.
	// Requires: Neutron.
	// +optional
	Manila *ManilaSpec `json:"manila,omitempty"`

	// Ironic defines the Ironic (Bare Metal) deployment spec.
	// Requires: Glance, Neutron.
	// +optional
	Ironic *IronicSpec `json:"ironic,omitempty"`

	// Magnum defines the Magnum (Container Infrastructure) deployment spec.
	// Requires: Nova, Neutron, Glance, Heat.
	// +optional
	Magnum *MagnumSpec `json:"magnum,omitempty"`

	// --- Extended Services: Tier 2 (Moderately Deployed) ---

	// Trove defines the Trove (Database as a Service) deployment spec.
	// Requires: Nova, Neutron, Cinder, Glance, Swift.
	// +optional
	Trove *TroveSpec `json:"trove,omitempty"`

	// Ceilometer defines the Ceilometer (Telemetry) deployment spec.
	// +optional
	Ceilometer *CeilometerSpec `json:"ceilometer,omitempty"`

	// Aodh defines the Aodh (Alarming) deployment spec.
	// Requires: Ceilometer.
	// +optional
	Aodh *AodhSpec `json:"aodh,omitempty"`

	// Masakari defines the Masakari (Instance HA) deployment spec.
	// Requires: Nova.
	// +optional
	Masakari *MasakariSpec `json:"masakari,omitempty"`

	// Mistral defines the Mistral (Workflow) deployment spec.
	// +optional
	Mistral *MistralSpec `json:"mistral,omitempty"`

	// Tacker defines the Tacker (NFV Orchestration) deployment spec.
	// Requires: Nova, Neutron, Glance, Heat.
	// +optional
	Tacker *TackerSpec `json:"tacker,omitempty"`

	// --- Extended Services: Tier 3 (Niche / Specialized) ---

	// Cyborg defines the Cyborg (Accelerator Management) deployment spec.
	// Requires: Nova, Placement.
	// +optional
	Cyborg *CyborgSpec `json:"cyborg,omitempty"`

	// Blazar defines the Blazar (Resource Reservation) deployment spec.
	// Requires: Nova.
	// +optional
	Blazar *BlazarSpec `json:"blazar,omitempty"`

	// Zun defines the Zun (Container) deployment spec.
	// Requires: Neutron.
	// +optional
	Zun *ZunSpec `json:"zun,omitempty"`

	// CloudKitty defines the CloudKitty (Rating/Billing) deployment spec.
	// Requires: Ceilometer or Prometheus.
	// +optional
	CloudKitty *CloudKittySpec `json:"cloudKitty,omitempty"`

	// Watcher defines the Watcher (Resource Optimization) deployment spec.
	// Requires: Nova, Ceilometer.
	// +optional
	Watcher *WatcherSpec `json:"watcher,omitempty"`

	// Vitrage defines the Vitrage (Root Cause Analysis) deployment spec.
	// Requires: Ceilometer, Aodh.
	// +optional
	Vitrage *VitrageSpec `json:"vitrage,omitempty"`

	// Zaqar defines the Zaqar (Messaging) deployment spec.
	// +optional
	Zaqar *ZaqarSpec `json:"zaqar,omitempty"`

	// Freezer defines the Freezer (Backup/Restore) deployment spec.
	// Requires: Swift.
	// +optional
	Freezer *FreezerSpec `json:"freezer,omitempty"`

	// Venus defines the Venus (Log Management) deployment spec.
	// +optional
	Venus *VenusSpec `json:"venus,omitempty"`

	// Adjutant defines the Adjutant (Ops Automation) deployment spec.
	// +optional
	Adjutant *AdjutantSpec `json:"adjutant,omitempty"`

	// Storlets defines the Storlets (Compute in Object Storage) deployment spec.
	// Requires: Swift.
	// +optional
	Storlets *StorletsSpec `json:"storlets,omitempty"`

	// --- Dashboards ---

	// Horizon defines the Horizon (Classic Dashboard) deployment spec.
	// +optional
	Horizon *HorizonSpec `json:"horizon,omitempty"`

	// Skyline defines the Skyline (Modern Dashboard) deployment spec.
	// +optional
	Skyline *SkylineSpec `json:"skyline,omitempty"`

	// --- Infrastructure Backends ---

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
// +kubebuilder:validation:Enum=Pending;Infrastructure;Identity;CoreServices;Compute;ExtendedServices;Dashboards;Ready;Failed
type ControlPlanePhase string

const (
	ControlPlanePhasePending          ControlPlanePhase = "Pending"
	ControlPlanePhaseInfrastructure   ControlPlanePhase = "Infrastructure"
	ControlPlanePhaseIdentity         ControlPlanePhase = "Identity"
	ControlPlanePhaseCoreServices     ControlPlanePhase = "CoreServices"
	ControlPlanePhaseCompute          ControlPlanePhase = "Compute"
	ControlPlanePhaseExtendedServices ControlPlanePhase = "ExtendedServices"
	ControlPlanePhaseDashboards       ControlPlanePhase = "Dashboards"
	ControlPlanePhaseReady            ControlPlanePhase = "Ready"
	ControlPlanePhaseFailed           ControlPlanePhase = "Failed"
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
