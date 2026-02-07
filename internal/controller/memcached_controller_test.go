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

func TestMemcachedReconciler_CreatesDeployment(t *testing.T) {
	scheme := common.SetupScheme()
	memcached := &openstackv1alpha1.Memcached{
		ObjectMeta: metav1.ObjectMeta{Name: "memcached", Namespace: "openstack"},
		Spec:       openstackv1alpha1.MemcachedSpec{},
	}

	client := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(memcached).
		WithStatusSubresource(memcached).
		Build()

	r := &MemcachedReconciler{Client: client, Scheme: scheme}
	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "memcached", Namespace: "openstack"},
	})
	if err != nil {
		t.Fatalf("reconcile error: %v", err)
	}

	// Verify Deployment was created
	deploy := &appsv1.Deployment{}
	err = client.Get(context.Background(), types.NamespacedName{Name: "memcached", Namespace: "openstack"}, deploy)
	if err != nil {
		t.Fatalf("expected Deployment to be created: %v", err)
	}

	// Verify Service was created
	svc := &corev1.Service{}
	err = client.Get(context.Background(), types.NamespacedName{Name: "memcached", Namespace: "openstack"}, svc)
	if err != nil {
		t.Fatalf("expected Service to be created: %v", err)
	}
}

func TestMemcachedReconciler_NotFound(t *testing.T) {
	scheme := common.SetupScheme()
	client := fake.NewClientBuilder().WithScheme(scheme).Build()

	r := &MemcachedReconciler{Client: client, Scheme: scheme}
	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "missing", Namespace: "openstack"},
	})
	if err != nil {
		t.Fatalf("expected no error for missing CR, got: %v", err)
	}
	if result.Requeue {
		t.Error("expected no requeue for missing CR")
	}
}
