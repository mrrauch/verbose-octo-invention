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

func ptr(i int32) *int32 { return &i }

func TestNovaReconciler_CreatesResources(t *testing.T) {
	scheme := common.SetupScheme()
	nova := &openstackv1alpha1.Nova{
		ObjectMeta: metav1.ObjectMeta{Name: "nova", Namespace: "openstack", Finalizers: []string{common.FinalizerName}},
		Spec: openstackv1alpha1.NovaSpec{
			PublicHostname:  "nova.example.com",
			ComputeReplicas: ptr(2),
			GatewayRef: openstackv1alpha1.GatewayRef{
				Name:      "my-gateway",
				Namespace: "edge",
			},
		},
	}
	client := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(nova).
		WithStatusSubresource(nova).
		Build()

	r := &NovaReconciler{Client: client, Scheme: scheme}
	req := ctrl.Request{NamespacedName: types.NamespacedName{Name: "nova", Namespace: "openstack"}}

	// First reconcile -- creates secret and db-create job
	_, _ = r.Reconcile(context.Background(), req)

	// Verify db-password secret
	dbSecret := &corev1.Secret{}
	if err := client.Get(context.Background(), types.NamespacedName{Name: "nova-db-password", Namespace: "openstack"}, dbSecret); err != nil {
		t.Fatalf("expected db password secret: %v", err)
	}

	// Verify db-create job
	dbJob := &batchv1.Job{}
	if err := client.Get(context.Background(), types.NamespacedName{Name: "nova-db-create", Namespace: "openstack"}, dbJob); err != nil {
		t.Fatalf("expected db-create job: %v", err)
	}

	// Mark db-create as complete and reconcile
	dbJob.Status.Conditions = append(dbJob.Status.Conditions, batchv1.JobCondition{
		Type:   batchv1.JobComplete,
		Status: corev1.ConditionTrue,
	})
	_ = client.Status().Update(context.Background(), dbJob)

	_, _ = r.Reconcile(context.Background(), req)

	// Verify db-sync job
	syncJob := &batchv1.Job{}
	if err := client.Get(context.Background(), types.NamespacedName{Name: "nova-db-sync", Namespace: "openstack"}, syncJob); err != nil {
		t.Fatalf("expected db-sync job: %v", err)
	}

	// Mark db-sync as complete and reconcile
	syncJob.Status.Conditions = append(syncJob.Status.Conditions, batchv1.JobCondition{
		Type:   batchv1.JobComplete,
		Status: corev1.ConditionTrue,
	})
	_ = client.Status().Update(context.Background(), syncJob)

	_, _ = r.Reconcile(context.Background(), req)

	// Verify cell-setup job
	cellJob := &batchv1.Job{}
	if err := client.Get(context.Background(), types.NamespacedName{Name: "nova-cell-setup", Namespace: "openstack"}, cellJob); err != nil {
		t.Fatalf("expected cell-setup job: %v", err)
	}
	cellJob.Status.Conditions = append(cellJob.Status.Conditions, batchv1.JobCondition{
		Type:   batchv1.JobComplete,
		Status: corev1.ConditionTrue,
	})
	_ = client.Status().Update(context.Background(), cellJob)

	_, _ = r.Reconcile(context.Background(), req)

	// Verify nova-api Deployment
	apiDep := &appsv1.Deployment{}
	if err := client.Get(context.Background(), types.NamespacedName{Name: "nova-api", Namespace: "openstack"}, apiDep); err != nil {
		t.Fatalf("expected nova-api deployment: %v", err)
	}
	if apiDep.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort != 8774 {
		t.Errorf("expected nova-api port 8774, got %d", apiDep.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort)
	}

	// Verify nova-scheduler Deployment
	schedulerDep := &appsv1.Deployment{}
	if err := client.Get(context.Background(), types.NamespacedName{Name: "nova-scheduler", Namespace: "openstack"}, schedulerDep); err != nil {
		t.Fatalf("expected nova-scheduler deployment: %v", err)
	}

	// Verify nova-conductor Deployment
	conductorDep := &appsv1.Deployment{}
	if err := client.Get(context.Background(), types.NamespacedName{Name: "nova-conductor", Namespace: "openstack"}, conductorDep); err != nil {
		t.Fatalf("expected nova-conductor deployment: %v", err)
	}

	// Verify nova-compute Deployment with 2 replicas
	computeDep := &appsv1.Deployment{}
	if err := client.Get(context.Background(), types.NamespacedName{Name: "nova-compute", Namespace: "openstack"}, computeDep); err != nil {
		t.Fatalf("expected nova-compute deployment: %v", err)
	}
	if *computeDep.Spec.Replicas != 2 {
		t.Errorf("expected 2 compute replicas, got %d", *computeDep.Spec.Replicas)
	}

	// Verify nova-api Service
	svc := &corev1.Service{}
	if err := client.Get(context.Background(), types.NamespacedName{Name: "nova-api", Namespace: "openstack"}, svc); err != nil {
		t.Fatalf("expected nova-api service: %v", err)
	}
	if svc.Spec.Ports[0].Port != 8774 {
		t.Errorf("expected service port 8774, got %d", svc.Spec.Ports[0].Port)
	}

	// Verify HTTPRoute
	route := &gatewayv1.HTTPRoute{}
	if err := client.Get(context.Background(), types.NamespacedName{Name: "nova-api", Namespace: "openstack"}, route); err != nil {
		t.Fatalf("expected HTTPRoute: %v", err)
	}
	if len(route.Spec.Hostnames) != 1 || string(route.Spec.Hostnames[0]) != "nova.example.com" {
		t.Errorf("expected hostname nova.example.com, got %v", route.Spec.Hostnames)
	}

	// Verify endpoint-create job
	endpointJob := &batchv1.Job{}
	if err := client.Get(context.Background(), types.NamespacedName{Name: "nova-endpoint-create", Namespace: "openstack"}, endpointJob); err != nil {
		t.Fatalf("expected endpoint-create job: %v", err)
	}
}

func TestNovaReconciler_NotFound(t *testing.T) {
	scheme := common.SetupScheme()
	client := fake.NewClientBuilder().WithScheme(scheme).Build()
	r := &NovaReconciler{Client: client, Scheme: scheme}
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
