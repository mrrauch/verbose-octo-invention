package common

import (
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	openstackv1alpha1 "github.com/mrrauch/openstack-operator/api/v1alpha1"
)

// SetupScheme returns a runtime.Scheme with core K8s, Gateway API, and OpenStack types registered.
func SetupScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(s))
	utilruntime.Must(gatewayv1.Install(s))
	utilruntime.Must(openstackv1alpha1.AddToScheme(s))
	return s
}
