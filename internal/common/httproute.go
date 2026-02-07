package common

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// HTTPRouteParams holds parameters for creating a Gateway API HTTPRoute.
type HTTPRouteParams struct {
	Name             string
	Namespace        string
	Hostname         string
	ServiceName      string
	ServicePort      int32
	GatewayName      string
	GatewayNamespace string
	ListenerName     string
}

// EnsureHTTPRoute reconciles an HTTPRoute that routes external traffic to an OpenStack service.
func EnsureHTTPRoute(ctx context.Context, c client.Client, params HTTPRouteParams, owner metav1.Object) error {
	route := &gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      params.Name,
			Namespace: params.Namespace,
		},
	}

	gatewayName := params.GatewayName
	if gatewayName == "" {
		gatewayName = "openstack-gateway"
	}
	gatewayNamespace := params.GatewayNamespace
	if gatewayNamespace == "" {
		gatewayNamespace = params.Namespace
	}

	port := gatewayv1.PortNumber(params.ServicePort)

	_, err := controllerutil.CreateOrUpdate(ctx, c, route, func() error {
		route.Labels = map[string]string{
			"app.kubernetes.io/managed-by": "openstack-operator",
		}

		parent := gatewayv1.ParentReference{
			Name: gatewayv1.ObjectName(gatewayName),
		}
		if gatewayNamespace != params.Namespace {
			ns := gatewayv1.Namespace(gatewayNamespace)
			parent.Namespace = &ns
		}
		if params.ListenerName != "" {
			ln := gatewayv1.SectionName(params.ListenerName)
			parent.SectionName = &ln
		}

		spec := gatewayv1.HTTPRouteSpec{
			CommonRouteSpec: gatewayv1.CommonRouteSpec{
				ParentRefs: []gatewayv1.ParentReference{parent},
			},
			Rules: []gatewayv1.HTTPRouteRule{
				{
					BackendRefs: []gatewayv1.HTTPBackendRef{
						{
							BackendRef: gatewayv1.BackendRef{
								BackendObjectReference: gatewayv1.BackendObjectReference{
									Name: gatewayv1.ObjectName(params.ServiceName),
									Port: &port,
								},
							},
						},
					},
				},
			},
		}
		if params.Hostname != "" {
			spec.Hostnames = []gatewayv1.Hostname{
				gatewayv1.Hostname(params.Hostname),
			}
		}
		route.Spec = spec

		if owner != nil {
			return controllerutil.SetOwnerReference(owner, route, c.Scheme())
		}
		return nil
	})
	return err
}
