package controller

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	openstackv1alpha1 "github.com/mrrauch/openstack-operator/api/v1alpha1"
	"github.com/mrrauch/openstack-operator/internal/common"
)

func ready(gen int64) []metav1.Condition {
	return []metav1.Condition{{
		Type:               "Ready",
		Status:             metav1.ConditionTrue,
		ObservedGeneration: gen,
		LastTransitionTime: metav1.Now(),
		Reason:             "Ready",
	}}
}

func TestControlPlaneReconciler_CreatesInfrastructureCRs(t *testing.T) {
	scheme := common.SetupScheme()
	cp := &openstackv1alpha1.OpenStackControlPlane{
		ObjectMeta: metav1.ObjectMeta{Name: "my-cloud", Namespace: "openstack", Finalizers: []string{common.FinalizerName}},
		Spec: openstackv1alpha1.OpenStackControlPlaneSpec{
			NetworkBackend: "ovn",
		},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cp).WithStatusSubresource(cp).Build()
	r := &ControlPlaneReconciler{Client: c, Scheme: scheme}

	if _, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "my-cloud", Namespace: "openstack"},
	}); err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}

	for _, obj := range []struct {
		name string
		dst  client.Object
	}{
		{name: "my-cloud-database", dst: &openstackv1alpha1.Database{}},
		{name: "my-cloud-rabbitmq", dst: &openstackv1alpha1.RabbitMQ{}},
		{name: "my-cloud-memcached", dst: &openstackv1alpha1.Memcached{}},
		{name: "my-cloud-ovn", dst: &openstackv1alpha1.OVNNetwork{}},
	} {
		if err := c.Get(context.Background(), types.NamespacedName{Name: obj.name, Namespace: "openstack"}, obj.dst); err != nil {
			t.Fatalf("expected child CR %s: %v", obj.name, err)
		}
	}

	fresh := &openstackv1alpha1.OpenStackControlPlane{}
	_ = c.Get(context.Background(), types.NamespacedName{Name: "my-cloud", Namespace: "openstack"}, fresh)
	if fresh.Status.Phase != openstackv1alpha1.ControlPlanePhaseInfrastructure {
		t.Fatalf("expected phase Infrastructure, got %s", fresh.Status.Phase)
	}
}

func TestControlPlaneReconciler_AdvancesToIdentity(t *testing.T) {
	scheme := common.SetupScheme()
	cp := &openstackv1alpha1.OpenStackControlPlane{
		ObjectMeta: metav1.ObjectMeta{Name: "my-cloud", Namespace: "openstack", Finalizers: []string{common.FinalizerName}},
		Spec: openstackv1alpha1.OpenStackControlPlaneSpec{
			NetworkBackend: "ovn",
			PublicDomain:   "cloud.example.com",
			GatewayRef: openstackv1alpha1.GatewayRef{
				Name:      "edge-gw",
				Namespace: "edge-system",
			},
		},
		Status: openstackv1alpha1.OpenStackControlPlaneStatus{
			Phase: openstackv1alpha1.ControlPlanePhaseInfrastructure,
		},
	}
	database := &openstackv1alpha1.Database{ObjectMeta: metav1.ObjectMeta{Name: "my-cloud-database", Namespace: "openstack"}, Status: openstackv1alpha1.DatabaseStatus{CommonStatus: openstackv1alpha1.CommonStatus{Conditions: ready(1)}}}
	rabbit := &openstackv1alpha1.RabbitMQ{ObjectMeta: metav1.ObjectMeta{Name: "my-cloud-rabbitmq", Namespace: "openstack"}, Status: openstackv1alpha1.RabbitMQStatus{CommonStatus: openstackv1alpha1.CommonStatus{Conditions: ready(1)}}}
	memcached := &openstackv1alpha1.Memcached{ObjectMeta: metav1.ObjectMeta{Name: "my-cloud-memcached", Namespace: "openstack"}, Status: openstackv1alpha1.MemcachedStatus{CommonStatus: openstackv1alpha1.CommonStatus{Conditions: ready(1)}}}
	ovn := &openstackv1alpha1.OVNNetwork{ObjectMeta: metav1.ObjectMeta{Name: "my-cloud-ovn", Namespace: "openstack"}, Status: openstackv1alpha1.OVNNetworkStatus{CommonStatus: openstackv1alpha1.CommonStatus{Conditions: ready(1)}}}

	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(cp, database, rabbit, memcached, ovn).
		WithStatusSubresource(cp, database, rabbit, memcached, ovn).
		Build()
	r := &ControlPlaneReconciler{Client: c, Scheme: scheme}

	if _, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "my-cloud", Namespace: "openstack"},
	}); err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}

	// Verify Keystone CR was created with inherited fields
	ks := &openstackv1alpha1.Keystone{}
	if err := c.Get(context.Background(), types.NamespacedName{Name: "my-cloud-keystone", Namespace: "openstack"}, ks); err != nil {
		t.Fatalf("expected Keystone CR: %v", err)
	}
	if ks.Spec.PublicHostname != "keystone.cloud.example.com" {
		t.Fatalf("expected publicHostname keystone.cloud.example.com, got %q", ks.Spec.PublicHostname)
	}
	if ks.Spec.GatewayRef.Name != "edge-gw" || ks.Spec.GatewayRef.Namespace != "edge-system" {
		t.Fatalf("expected inherited gatewayRef edge-system/edge-gw, got %s/%s", ks.Spec.GatewayRef.Namespace, ks.Spec.GatewayRef.Name)
	}

	fresh := &openstackv1alpha1.OpenStackControlPlane{}
	_ = c.Get(context.Background(), types.NamespacedName{Name: "my-cloud", Namespace: "openstack"}, fresh)
	if fresh.Status.Phase != openstackv1alpha1.ControlPlanePhaseIdentity {
		t.Fatalf("expected phase Identity, got %s", fresh.Status.Phase)
	}
}

func TestControlPlaneReconciler_AdvancesToCompute(t *testing.T) {
	scheme := common.SetupScheme()
	cp := &openstackv1alpha1.OpenStackControlPlane{
		ObjectMeta: metav1.ObjectMeta{Name: "my-cloud", Namespace: "openstack", Finalizers: []string{common.FinalizerName}},
		Spec:       openstackv1alpha1.OpenStackControlPlaneSpec{},
		Status: openstackv1alpha1.OpenStackControlPlaneStatus{
			Phase: openstackv1alpha1.ControlPlanePhaseCoreServices,
		},
	}
	glance := &openstackv1alpha1.Glance{ObjectMeta: metav1.ObjectMeta{Name: "my-cloud-glance", Namespace: "openstack"}, Status: openstackv1alpha1.GlanceStatus{CommonStatus: openstackv1alpha1.CommonStatus{Conditions: ready(1)}}}
	placement := &openstackv1alpha1.Placement{ObjectMeta: metav1.ObjectMeta{Name: "my-cloud-placement", Namespace: "openstack"}, Status: openstackv1alpha1.PlacementStatus{CommonStatus: openstackv1alpha1.CommonStatus{Conditions: ready(1)}}}
	neutron := &openstackv1alpha1.Neutron{ObjectMeta: metav1.ObjectMeta{Name: "my-cloud-neutron", Namespace: "openstack"}, Status: openstackv1alpha1.NeutronStatus{CommonStatus: openstackv1alpha1.CommonStatus{Conditions: ready(1)}}}

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cp, glance, placement, neutron).WithStatusSubresource(cp, glance, placement, neutron).Build()
	r := &ControlPlaneReconciler{Client: c, Scheme: scheme}

	if _, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "my-cloud", Namespace: "openstack"},
	}); err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}

	nova := &openstackv1alpha1.Nova{}
	if err := c.Get(context.Background(), types.NamespacedName{Name: "my-cloud-nova", Namespace: "openstack"}, nova); err != nil {
		t.Fatalf("expected Nova CR: %v", err)
	}

	fresh := &openstackv1alpha1.OpenStackControlPlane{}
	_ = c.Get(context.Background(), types.NamespacedName{Name: "my-cloud", Namespace: "openstack"}, fresh)
	if fresh.Status.Phase != openstackv1alpha1.ControlPlanePhaseCompute {
		t.Fatalf("expected phase Compute, got %s", fresh.Status.Phase)
	}
}

func TestControlPlaneReconciler_NotFound(t *testing.T) {
	scheme := common.SetupScheme()
	c := fake.NewClientBuilder().WithScheme(scheme).Build()
	r := &ControlPlaneReconciler{Client: c, Scheme: scheme}
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
