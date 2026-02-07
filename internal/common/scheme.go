package common

import (
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"

	openstackv1alpha1 "github.com/mrrauch/openstack-operator/api/v1alpha1"
)

// SetupScheme returns a runtime.Scheme with core K8s and OpenStack types registered.
// Gateway API types will be added later when httproute.go is implemented.
func SetupScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(s))
	utilruntime.Must(openstackv1alpha1.AddToScheme(s))
	return s
}
