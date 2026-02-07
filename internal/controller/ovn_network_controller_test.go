package controller

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	openstackv1alpha1 "github.com/mrrauch/openstack-operator/api/v1alpha1"
	"github.com/mrrauch/openstack-operator/internal/common"
)

func TestOVNNetworkReconciler_CreatesResources(t *testing.T) {
	scheme := common.SetupScheme()
	ovn := &openstackv1alpha1.OVNNetwork{
		ObjectMeta: metav1.ObjectMeta{Name: "ovn", Namespace: "openstack", Finalizers: []string{common.FinalizerName}},
		Spec:       openstackv1alpha1.OVNNetworkSpec{},
	}
	client := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(ovn).
		WithStatusSubresource(ovn).
		Build()

	r := &OVNNetworkReconciler{Client: client, Scheme: scheme}
	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "ovn", Namespace: "openstack"},
	})
	if err != nil {
		t.Fatalf("reconcile error: %v", err)
	}

	// Verify NB DB StatefulSet
	nbSts := &appsv1.StatefulSet{}
	if err := client.Get(context.Background(), types.NamespacedName{Name: "ovn-nb-db", Namespace: "openstack"}, nbSts); err != nil {
		t.Fatalf("expected ovn-nb-db StatefulSet: %v", err)
	}

	// Verify SB DB StatefulSet
	sbSts := &appsv1.StatefulSet{}
	if err := client.Get(context.Background(), types.NamespacedName{Name: "ovn-sb-db", Namespace: "openstack"}, sbSts); err != nil {
		t.Fatalf("expected ovn-sb-db StatefulSet: %v", err)
	}

	// Verify northd Deployment
	northd := &appsv1.Deployment{}
	if err := client.Get(context.Background(), types.NamespacedName{Name: "ovn-northd", Namespace: "openstack"}, northd); err != nil {
		t.Fatalf("expected ovn-northd Deployment: %v", err)
	}

	// Verify NB DB headless Service
	nbSvc := &corev1.Service{}
	if err := client.Get(context.Background(), types.NamespacedName{Name: "ovn-nb-db", Namespace: "openstack"}, nbSvc); err != nil {
		t.Fatalf("expected ovn-nb-db Service: %v", err)
	}

	// Verify SB DB headless Service
	sbSvc := &corev1.Service{}
	if err := client.Get(context.Background(), types.NamespacedName{Name: "ovn-sb-db", Namespace: "openstack"}, sbSvc); err != nil {
		t.Fatalf("expected ovn-sb-db Service: %v", err)
	}
}

func TestOVNNetworkReconciler_NotFound(t *testing.T) {
	scheme := common.SetupScheme()
	client := fake.NewClientBuilder().WithScheme(scheme).Build()
	r := &OVNNetworkReconciler{Client: client, Scheme: scheme}
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
