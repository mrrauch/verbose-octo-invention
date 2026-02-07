package controller

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	openstackv1alpha1 "github.com/mrrauch/openstack-operator/api/v1alpha1"
	"github.com/mrrauch/openstack-operator/internal/common"
)

func TestNeutronReconciler_CreatesResources(t *testing.T) {
	scheme := common.SetupScheme()
	neutron := &openstackv1alpha1.Neutron{
		ObjectMeta: metav1.ObjectMeta{Name: "neutron", Namespace: "openstack"},
		Spec: openstackv1alpha1.NeutronSpec{
			PublicHostname: "neutron.example.com",
			GatewayRef: openstackv1alpha1.GatewayRef{
				Name:      "my-gateway",
				Namespace: "edge",
			},
		},
	}
	client := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(neutron).
		WithStatusSubresource(neutron).
		Build()

	r := &NeutronReconciler{Client: client, Scheme: scheme}

	// First reconcile -- creates secret and db-create job
	_, _ = r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "neutron", Namespace: "openstack"},
	})

	// Verify db-password secret
	dbSecret := &corev1.Secret{}
	if err := client.Get(context.Background(), types.NamespacedName{Name: "neutron-db-password", Namespace: "openstack"}, dbSecret); err != nil {
		t.Fatalf("expected db password secret: %v", err)
	}

	// Verify db-create job
	dbJob := &batchv1.Job{}
	if err := client.Get(context.Background(), types.NamespacedName{Name: "neutron-db-create", Namespace: "openstack"}, dbJob); err != nil {
		t.Fatalf("expected db-create job: %v", err)
	}

	// Mark db-create as complete and reconcile
	dbJob.Status.Conditions = append(dbJob.Status.Conditions, batchv1.JobCondition{
		Type:   batchv1.JobComplete,
		Status: corev1.ConditionTrue,
	})
	_ = client.Status().Update(context.Background(), dbJob)

	_, _ = r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "neutron", Namespace: "openstack"},
	})

	// Verify db-sync job
	syncJob := &batchv1.Job{}
	if err := client.Get(context.Background(), types.NamespacedName{Name: "neutron-db-sync", Namespace: "openstack"}, syncJob); err != nil {
		t.Fatalf("expected db-sync job: %v", err)
	}

	// Mark db-sync as complete and reconcile
	syncJob.Status.Conditions = append(syncJob.Status.Conditions, batchv1.JobCondition{
		Type:   batchv1.JobComplete,
		Status: corev1.ConditionTrue,
	})
	_ = client.Status().Update(context.Background(), syncJob)

	_, _ = r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "neutron", Namespace: "openstack"},
	})

	// Verify Deployment
	dep := &appsv1.Deployment{}
	if err := client.Get(context.Background(), types.NamespacedName{Name: "neutron-server", Namespace: "openstack"}, dep); err != nil {
		t.Fatalf("expected deployment: %v", err)
	}

	// Verify Service
	svc := &corev1.Service{}
	if err := client.Get(context.Background(), types.NamespacedName{Name: "neutron-server", Namespace: "openstack"}, svc); err != nil {
		t.Fatalf("expected service: %v", err)
	}

	// Verify HTTPRoute
	route := &gatewayv1.HTTPRoute{}
	if err := client.Get(context.Background(), types.NamespacedName{Name: "neutron-server", Namespace: "openstack"}, route); err != nil {
		t.Fatalf("expected HTTPRoute: %v", err)
	}
	if len(route.Spec.Hostnames) != 1 || string(route.Spec.Hostnames[0]) != "neutron.example.com" {
		t.Errorf("expected hostname neutron.example.com, got %v", route.Spec.Hostnames)
	}

	// Verify endpoint-create job
	endpointJob := &batchv1.Job{}
	if err := client.Get(context.Background(), types.NamespacedName{Name: "neutron-endpoint-create", Namespace: "openstack"}, endpointJob); err != nil {
		t.Fatalf("expected endpoint-create job: %v", err)
	}
}

func TestNeutronReconciler_NotFound(t *testing.T) {
	scheme := common.SetupScheme()
	client := fake.NewClientBuilder().WithScheme(scheme).Build()
	r := &NeutronReconciler{Client: client, Scheme: scheme}
	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "missing", Namespace: "openstack"},
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if result.Requeue {
		t.Error("expected no requeue")
	}
}
