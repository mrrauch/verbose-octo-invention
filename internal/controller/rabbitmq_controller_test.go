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

func TestRabbitMQReconciler_CreatesStatefulSet(t *testing.T) {
	scheme := common.SetupScheme()
	rabbitmq := &openstackv1alpha1.RabbitMQ{
		ObjectMeta: metav1.ObjectMeta{Name: "rabbitmq", Namespace: "openstack"},
		Spec:       openstackv1alpha1.RabbitMQServiceSpec{},
	}

	client := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(rabbitmq).
		WithStatusSubresource(rabbitmq).
		Build()

	r := &RabbitMQReconciler{Client: client, Scheme: scheme}
	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "rabbitmq", Namespace: "openstack"},
	})
	if err != nil {
		t.Fatalf("reconcile error: %v", err)
	}

	// Verify StatefulSet was created
	sts := &appsv1.StatefulSet{}
	err = client.Get(context.Background(), types.NamespacedName{Name: "rabbitmq", Namespace: "openstack"}, sts)
	if err != nil {
		t.Fatalf("expected StatefulSet to be created: %v", err)
	}

	// Verify Service was created
	svc := &corev1.Service{}
	err = client.Get(context.Background(), types.NamespacedName{Name: "rabbitmq", Namespace: "openstack"}, svc)
	if err != nil {
		t.Fatalf("expected Service to be created: %v", err)
	}

	// Verify Secret was created
	secret := &corev1.Secret{}
	err = client.Get(context.Background(), types.NamespacedName{Name: "rabbitmq-credentials", Namespace: "openstack"}, secret)
	if err != nil {
		t.Fatalf("expected Secret to be created: %v", err)
	}
}

func TestRabbitMQReconciler_NotFound(t *testing.T) {
	scheme := common.SetupScheme()
	client := fake.NewClientBuilder().WithScheme(scheme).Build()

	r := &RabbitMQReconciler{Client: client, Scheme: scheme}
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
