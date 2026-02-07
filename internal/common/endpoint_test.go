package common

import (
	"context"
	"testing"

	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestEnsureKeystoneEndpoint_CreatesJob(t *testing.T) {
	scheme := SetupScheme()
	client := fake.NewClientBuilder().WithScheme(scheme).Build()

	params := EndpointParams{
		Name:           "glance",
		Namespace:      "openstack",
		ServiceName:    "glance",
		ServiceType:    "image",
		InternalURL:    "http://glance-api.openstack.svc:9292",
		PublicURL:      "https://glance.example.com",
		AdminURL:       "http://glance-api.openstack.svc:9292",
		Region:         "RegionOne",
		KeystoneSecret: "keystone-admin-password",
		KeystoneURL:    "http://keystone-api.openstack.svc:5000/v3",
		BootstrapImage: "quay.io/openstack.kolla/keystone:2025.1",
	}

	err := EnsureKeystoneEndpoint(context.Background(), client, params, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	job := &batchv1.Job{}
	err = client.Get(context.Background(), types.NamespacedName{
		Name:      "glance-endpoint-create",
		Namespace: "openstack",
	}, job)
	if err != nil {
		t.Fatalf("expected endpoint-create Job: %v", err)
	}
}
