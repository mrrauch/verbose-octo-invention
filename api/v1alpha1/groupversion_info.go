// Package v1alpha1 contains API Schema definitions for the openstack v1alpha1 API group.
// +kubebuilder:object:generate=true
// +groupName=openstack.k8s.io
package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

var (
	// GroupVersion is the group version used to register these objects.
	GroupVersion = schema.GroupVersion{Group: "openstack.k8s.io", Version: "v1alpha1"}

	// SchemeBuilder is used to add go types to the GroupVersionResource.
	SchemeBuilder = &scheme.Builder{GroupVersion: GroupVersion}

	// AddToScheme adds the types in this group-version to the given scheme.
	AddToScheme = SchemeBuilder.AddToScheme
)
