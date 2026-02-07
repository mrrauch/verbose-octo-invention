package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DataPlaneNodeSpec defines a single data-plane node (compute, storage, or network).
type DataPlaneNodeSpec struct {
	// Hostname is the DNS/SSH hostname of the node.
	Hostname string `json:"hostname"`

	// IP is the management IP address of the node.
	IP string `json:"ip"`

	// Role indicates the node's purpose.
	// +kubebuilder:validation:Enum=compute;storage;networker
	Role string `json:"role"`

	// SSHSecretName references the Secret containing the SSH private key for this node.
	SSHSecretName string `json:"sshSecretName"`
}

// OpenStackDataPlaneSpec defines the desired state of the data-plane node set.
type OpenStackDataPlaneSpec struct {
	// Nodes lists the data-plane nodes to manage.
	// +kubebuilder:validation:MinItems=1
	Nodes []DataPlaneNodeSpec `json:"nodes"`

	// AnsiblePlaybook is the name of the playbook to run for provisioning.
	// Defaults to the built-in data-plane playbook.
	// +optional
	AnsiblePlaybook string `json:"ansiblePlaybook,omitempty"`

	// AnsibleExtraVars are additional variables passed to Ansible.
	// +optional
	AnsibleExtraVars map[string]string `json:"ansibleExtraVars,omitempty"`

	// ServicesOverride allows enabling/disabling specific services on data-plane nodes.
	// +optional
	ServicesOverride *DataPlaneServices `json:"servicesOverride,omitempty"`
}

// DataPlaneServices controls which services are installed on data-plane nodes.
type DataPlaneServices struct {
	// NovaCompute enables nova-compute on compute nodes.
	// +kubebuilder:default=true
	// +optional
	NovaCompute bool `json:"novaCompute,omitempty"`

	// OVNController enables ovn-controller on compute/network nodes.
	// +kubebuilder:default=true
	// +optional
	OVNController bool `json:"ovnController,omitempty"`

	// CephOSD enables Ceph OSD on storage nodes.
	// +kubebuilder:default=true
	// +optional
	CephOSD bool `json:"cephOSD,omitempty"`

	// Libvirt enables libvirtd on compute nodes.
	// +kubebuilder:default=true
	// +optional
	Libvirt bool `json:"libvirt,omitempty"`
}

// OpenStackDataPlaneStatus defines the observed state of OpenStackDataPlane.
type OpenStackDataPlaneStatus struct {
	CommonStatus `json:",inline"`

	// DeployedNodes is the count of nodes that have been successfully provisioned.
	// +optional
	DeployedNodes int32 `json:"deployedNodes,omitempty"`

	// TotalNodes is the total number of nodes in the node set.
	// +optional
	TotalNodes int32 `json:"totalNodes,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Deployed",type=integer,JSONPath=`.status.deployedNodes`
// +kubebuilder:printcolumn:name="Total",type=integer,JSONPath=`.status.totalNodes`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// OpenStackDataPlane is the Schema for the openstackdataplanes API.
type OpenStackDataPlane struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OpenStackDataPlaneSpec   `json:"spec,omitempty"`
	Status OpenStackDataPlaneStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// OpenStackDataPlaneList contains a list of OpenStackDataPlane.
type OpenStackDataPlaneList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OpenStackDataPlane `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OpenStackDataPlane{}, &OpenStackDataPlaneList{})
}
