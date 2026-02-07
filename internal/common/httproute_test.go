package common

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func TestEnsureHTTPRoute_CreatesRoute(t *testing.T) {
	scheme := SetupScheme()
	client := fake.NewClientBuilder().WithScheme(scheme).Build()

	params := HTTPRouteParams{
		Name:             "keystone-api",
		Namespace:        "openstack",
		Hostname:         "keystone.example.com",
		ServiceName:      "keystone-api",
		ServicePort:      5000,
		GatewayName:      "openstack-gateway",
		GatewayNamespace: "edge-system",
		ListenerName:     "https",
	}

	err := EnsureHTTPRoute(context.Background(), client, params, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	route := &gatewayv1.HTTPRoute{}
	err = client.Get(context.Background(), types.NamespacedName{
		Name:      "keystone-api",
		Namespace: "openstack",
	}, route)
	if err != nil {
		t.Fatalf("expected HTTPRoute to be created: %v", err)
	}

	if len(route.Spec.Hostnames) != 1 || string(route.Spec.Hostnames[0]) != "keystone.example.com" {
		t.Errorf("expected hostname keystone.example.com, got %v", route.Spec.Hostnames)
	}

	if len(route.Spec.ParentRefs) != 1 || string(route.Spec.ParentRefs[0].Name) != "openstack-gateway" {
		t.Errorf("expected parentRef openstack-gateway, got %v", route.Spec.ParentRefs)
	}
	if route.Spec.ParentRefs[0].Namespace == nil || string(*route.Spec.ParentRefs[0].Namespace) != "edge-system" {
		t.Errorf("expected parentRef namespace edge-system, got %v", route.Spec.ParentRefs[0].Namespace)
	}
	if route.Spec.ParentRefs[0].SectionName == nil || string(*route.Spec.ParentRefs[0].SectionName) != "https" {
		t.Errorf("expected listener sectionName=https, got %v", route.Spec.ParentRefs[0].SectionName)
	}

	rules := route.Spec.Rules
	if len(rules) != 1 || len(rules[0].BackendRefs) != 1 {
		t.Fatalf("expected 1 rule with 1 backendRef, got %d rules", len(rules))
	}
	ref := rules[0].BackendRefs[0]
	if string(ref.Name) != "keystone-api" {
		t.Errorf("expected backend keystone-api, got %s", ref.Name)
	}
	if *ref.Port != 5000 {
		t.Errorf("expected port 5000, got %d", *ref.Port)
	}
}

func TestEnsureHTTPRoute_UpdatesWhenSpecDrifts(t *testing.T) {
	scheme := SetupScheme()
	existing := &gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "keystone-api",
			Namespace: "openstack",
		},
		Spec: gatewayv1.HTTPRouteSpec{
			Hostnames: []gatewayv1.Hostname{"old.example.com"},
		},
	}

	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(existing).Build()

	params := HTTPRouteParams{
		Name:        "keystone-api",
		Namespace:   "openstack",
		Hostname:    "keystone.example.com",
		ServiceName: "keystone-api",
		ServicePort: 5000,
		GatewayName: "openstack-gateway",
	}

	err := EnsureHTTPRoute(context.Background(), client, params, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	updated := &gatewayv1.HTTPRoute{}
	if err := client.Get(context.Background(), types.NamespacedName{Name: "keystone-api", Namespace: "openstack"}, updated); err != nil {
		t.Fatalf("get route: %v", err)
	}
	if len(updated.Spec.Hostnames) != 1 || string(updated.Spec.Hostnames[0]) != "keystone.example.com" {
		t.Fatalf("expected hostname to be reconciled, got %v", updated.Spec.Hostnames)
	}
}

func TestEnsureHTTPRoute_AllowsEmptyHostname(t *testing.T) {
	scheme := SetupScheme()
	client := fake.NewClientBuilder().WithScheme(scheme).Build()

	params := HTTPRouteParams{
		Name:        "placement-api",
		Namespace:   "openstack",
		ServiceName: "placement-api",
		ServicePort: 8778,
		GatewayName: "openstack-gateway",
	}

	err := EnsureHTTPRoute(context.Background(), client, params, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	route := &gatewayv1.HTTPRoute{}
	if err := client.Get(context.Background(), types.NamespacedName{Name: "placement-api", Namespace: "openstack"}, route); err != nil {
		t.Fatalf("expected HTTPRoute to be created: %v", err)
	}
	if len(route.Spec.Hostnames) != 0 {
		t.Fatalf("expected no hostnames for empty hostname input, got %v", route.Spec.Hostnames)
	}
}
