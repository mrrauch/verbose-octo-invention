package controller

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	openstackv1alpha1 "github.com/mrrauch/openstack-operator/api/v1alpha1"
	"github.com/mrrauch/openstack-operator/internal/common"
)

const requeueDelay = 10 * time.Second

// ControlPlaneReconciler reconciles an OpenStackControlPlane object.
type ControlPlaneReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// Reconcile handles the reconciliation loop for OpenStackControlPlane resources.
func (r *ControlPlaneReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	instance := &openstackv1alpha1.OpenStackControlPlane{}
	if err := r.Get(ctx, req.NamespacedName, instance); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Handle deletion
	if !instance.DeletionTimestamp.IsZero() {
		if common.HasFinalizer(instance, common.FinalizerName) {
			common.RemoveFinalizer(instance, common.FinalizerName)
			return ctrl.Result{}, r.Update(ctx, instance)
		}
		return ctrl.Result{}, nil
	}

	// Ensure finalizer
	if !common.HasFinalizer(instance, common.FinalizerName) {
		common.AddFinalizer(instance, common.FinalizerName)
		if err := r.Update(ctx, instance); err != nil {
			return ctrl.Result{}, err
		}
	}

	switch instance.Status.Phase {
	case "", openstackv1alpha1.ControlPlanePhasePending:
		return r.reconcileInfrastructure(ctx, instance)
	case openstackv1alpha1.ControlPlanePhaseInfrastructure:
		return r.reconcileIdentity(ctx, instance)
	case openstackv1alpha1.ControlPlanePhaseIdentity:
		return r.reconcileCoreServices(ctx, instance)
	case openstackv1alpha1.ControlPlanePhaseCoreServices:
		return r.reconcileCompute(ctx, instance)
	case openstackv1alpha1.ControlPlanePhaseCompute:
		return r.reconcileReady(ctx, instance)
	case openstackv1alpha1.ControlPlanePhaseReady:
		return ctrl.Result{}, nil
	default:
		return ctrl.Result{}, nil
	}
}

// reconcileInfrastructure creates MariaDB, RabbitMQ, Memcached, and (optionally) OVN child CRs,
// then advances the phase to Infrastructure.
func (r *ControlPlaneReconciler) reconcileInfrastructure(ctx context.Context, instance *openstackv1alpha1.OpenStackControlPlane) (ctrl.Result, error) {
	name := instance.Name
	ns := instance.Namespace

	if err := r.ensureChildCR(ctx, instance, &openstackv1alpha1.MariaDB{
		ObjectMeta: metav1.ObjectMeta{Name: name + "-mariadb", Namespace: ns},
		Spec:       instance.Spec.MariaDB,
	}); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.ensureChildCR(ctx, instance, &openstackv1alpha1.RabbitMQ{
		ObjectMeta: metav1.ObjectMeta{Name: name + "-rabbitmq", Namespace: ns},
		Spec:       instance.Spec.RabbitMQ,
	}); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.ensureChildCR(ctx, instance, &openstackv1alpha1.Memcached{
		ObjectMeta: metav1.ObjectMeta{Name: name + "-memcached", Namespace: ns},
		Spec:       instance.Spec.Memcached,
	}); err != nil {
		return ctrl.Result{}, err
	}

	if instance.Spec.NetworkBackend == "ovn" {
		spec := openstackv1alpha1.OVNNetworkSpec{}
		if instance.Spec.OVNNetwork != nil {
			spec = *instance.Spec.OVNNetwork
		}
		if err := r.ensureChildCR(ctx, instance, &openstackv1alpha1.OVNNetwork{
			ObjectMeta: metav1.ObjectMeta{Name: name + "-ovn", Namespace: ns},
			Spec:       spec,
		}); err != nil {
			return ctrl.Result{}, err
		}
	}

	return r.setPhase(ctx, instance, openstackv1alpha1.ControlPlanePhaseInfrastructure)
}

// reconcileIdentity waits for all infrastructure CRs to be ready, creates Keystone,
// then advances the phase to Identity.
func (r *ControlPlaneReconciler) reconcileIdentity(ctx context.Context, instance *openstackv1alpha1.OpenStackControlPlane) (ctrl.Result, error) {
	name := instance.Name
	ns := instance.Namespace

	infraReady, err := r.allChildrenReady(ctx, ns, []childCheck{
		{name: name + "-mariadb", obj: &openstackv1alpha1.MariaDB{}},
		{name: name + "-rabbitmq", obj: &openstackv1alpha1.RabbitMQ{}},
		{name: name + "-memcached", obj: &openstackv1alpha1.Memcached{}},
	})
	if err != nil {
		return ctrl.Result{}, err
	}

	if instance.Spec.NetworkBackend == "ovn" {
		ovnReady, err := r.isChildReady(ctx, name+"-ovn", ns, &openstackv1alpha1.OVNNetwork{})
		if err != nil {
			return ctrl.Result{}, err
		}
		infraReady = infraReady && ovnReady
	}

	if !infraReady {
		return ctrl.Result{RequeueAfter: requeueDelay}, nil
	}

	keystoneSpec := instance.Spec.Keystone
	applyDefaults(&keystoneSpec.PublicHostname, &keystoneSpec.GatewayRef, "keystone", instance)

	if err := r.ensureChildCR(ctx, instance, &openstackv1alpha1.Keystone{
		ObjectMeta: metav1.ObjectMeta{Name: name + "-keystone", Namespace: ns},
		Spec:       keystoneSpec,
	}); err != nil {
		return ctrl.Result{}, err
	}

	return r.setPhase(ctx, instance, openstackv1alpha1.ControlPlanePhaseIdentity)
}

// reconcileCoreServices waits for Keystone to be ready, creates Glance/Placement/Neutron,
// then advances the phase to CoreServices.
func (r *ControlPlaneReconciler) reconcileCoreServices(ctx context.Context, instance *openstackv1alpha1.OpenStackControlPlane) (ctrl.Result, error) {
	name := instance.Name
	ns := instance.Namespace

	keystoneReady, err := r.isChildReady(ctx, name+"-keystone", ns, &openstackv1alpha1.Keystone{})
	if err != nil {
		return ctrl.Result{}, err
	}
	if !keystoneReady {
		return ctrl.Result{RequeueAfter: requeueDelay}, nil
	}

	glanceSpec := instance.Spec.Glance
	applyDefaults(&glanceSpec.PublicHostname, &glanceSpec.GatewayRef, "glance", instance)
	if err := r.ensureChildCR(ctx, instance, &openstackv1alpha1.Glance{
		ObjectMeta: metav1.ObjectMeta{Name: name + "-glance", Namespace: ns},
		Spec:       glanceSpec,
	}); err != nil {
		return ctrl.Result{}, err
	}

	placementSpec := instance.Spec.Placement
	applyDefaults(&placementSpec.PublicHostname, &placementSpec.GatewayRef, "placement", instance)
	if err := r.ensureChildCR(ctx, instance, &openstackv1alpha1.Placement{
		ObjectMeta: metav1.ObjectMeta{Name: name + "-placement", Namespace: ns},
		Spec:       placementSpec,
	}); err != nil {
		return ctrl.Result{}, err
	}

	neutronSpec := instance.Spec.Neutron
	applyDefaults(&neutronSpec.PublicHostname, &neutronSpec.GatewayRef, "neutron", instance)
	if err := r.ensureChildCR(ctx, instance, &openstackv1alpha1.Neutron{
		ObjectMeta: metav1.ObjectMeta{Name: name + "-neutron", Namespace: ns},
		Spec:       neutronSpec,
	}); err != nil {
		return ctrl.Result{}, err
	}

	return r.setPhase(ctx, instance, openstackv1alpha1.ControlPlanePhaseCoreServices)
}

// reconcileCompute waits for Glance/Placement/Neutron to be ready, creates Nova,
// then advances the phase to Compute.
func (r *ControlPlaneReconciler) reconcileCompute(ctx context.Context, instance *openstackv1alpha1.OpenStackControlPlane) (ctrl.Result, error) {
	name := instance.Name
	ns := instance.Namespace

	allReady, err := r.allChildrenReady(ctx, ns, []childCheck{
		{name: name + "-glance", obj: &openstackv1alpha1.Glance{}},
		{name: name + "-placement", obj: &openstackv1alpha1.Placement{}},
		{name: name + "-neutron", obj: &openstackv1alpha1.Neutron{}},
	})
	if err != nil {
		return ctrl.Result{}, err
	}
	if !allReady {
		return ctrl.Result{RequeueAfter: requeueDelay}, nil
	}

	novaSpec := instance.Spec.Nova
	applyDefaults(&novaSpec.PublicHostname, &novaSpec.GatewayRef, "nova", instance)
	if err := r.ensureChildCR(ctx, instance, &openstackv1alpha1.Nova{
		ObjectMeta: metav1.ObjectMeta{Name: name + "-nova", Namespace: ns},
		Spec:       novaSpec,
	}); err != nil {
		return ctrl.Result{}, err
	}

	return r.setPhase(ctx, instance, openstackv1alpha1.ControlPlanePhaseCompute)
}

// reconcileReady waits for Nova to be ready and then sets the phase to Ready.
func (r *ControlPlaneReconciler) reconcileReady(ctx context.Context, instance *openstackv1alpha1.OpenStackControlPlane) (ctrl.Result, error) {
	novaReady, err := r.isChildReady(ctx, instance.Name+"-nova", instance.Namespace, &openstackv1alpha1.Nova{})
	if err != nil {
		return ctrl.Result{}, err
	}
	if !novaReady {
		return ctrl.Result{RequeueAfter: requeueDelay}, nil
	}

	return r.setPhase(ctx, instance, openstackv1alpha1.ControlPlanePhaseReady)
}

// setPhase updates the control plane's phase and persists it via a status update.
func (r *ControlPlaneReconciler) setPhase(ctx context.Context, instance *openstackv1alpha1.OpenStackControlPlane, phase openstackv1alpha1.ControlPlanePhase) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	instance.Status.Phase = phase
	instance.Status.ObservedGeneration = instance.Generation
	if err := r.Status().Update(ctx, instance); err != nil {
		logger.Error(err, "failed to update control plane status", "phase", phase)
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// ensureChildCR creates a child CR if it does not already exist.
func (r *ControlPlaneReconciler) ensureChildCR(ctx context.Context, owner *openstackv1alpha1.OpenStackControlPlane, obj client.Object) error {
	existing := obj.DeepCopyObject().(client.Object)
	err := r.Get(ctx, types.NamespacedName{Name: obj.GetName(), Namespace: obj.GetNamespace()}, existing)
	if err == nil {
		return nil
	}
	if !errors.IsNotFound(err) {
		return err
	}

	_ = controllerutil.SetOwnerReference(owner, obj, r.Scheme)
	return r.Create(ctx, obj)
}

// childCheck pairs a child CR name with an empty typed object for readiness checking.
type childCheck struct {
	name string
	obj  client.Object
}

// allChildrenReady returns true only if every child CR in the list exists and has Ready=True.
func (r *ControlPlaneReconciler) allChildrenReady(ctx context.Context, namespace string, children []childCheck) (bool, error) {
	for _, child := range children {
		ready, err := r.isChildReady(ctx, child.name, namespace, child.obj)
		if err != nil {
			return false, err
		}
		if !ready {
			return false, nil
		}
	}
	return true, nil
}

// isChildReady fetches a child CR and returns whether its Ready condition is True.
func (r *ControlPlaneReconciler) isChildReady(ctx context.Context, name, namespace string, obj client.Object) (bool, error) {
	if err := r.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, obj); err != nil {
		if errors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return common.IsReady(getConditions(obj)), nil
}

// getConditions extracts status conditions from a typed child CR.
func getConditions(obj client.Object) []metav1.Condition {
	switch o := obj.(type) {
	case *openstackv1alpha1.MariaDB:
		return o.Status.Conditions
	case *openstackv1alpha1.RabbitMQ:
		return o.Status.Conditions
	case *openstackv1alpha1.Memcached:
		return o.Status.Conditions
	case *openstackv1alpha1.OVNNetwork:
		return o.Status.Conditions
	case *openstackv1alpha1.Keystone:
		return o.Status.Conditions
	case *openstackv1alpha1.Glance:
		return o.Status.Conditions
	case *openstackv1alpha1.Placement:
		return o.Status.Conditions
	case *openstackv1alpha1.Neutron:
		return o.Status.Conditions
	case *openstackv1alpha1.Nova:
		return o.Status.Conditions
	default:
		return nil
	}
}

// applyDefaults sets PublicHostname and GatewayRef on a child service spec when the child
// does not provide its own values.
func applyDefaults(hostname *string, gatewayRef *openstackv1alpha1.GatewayRef, serviceName string, cp *openstackv1alpha1.OpenStackControlPlane) {
	if *hostname == "" {
		domain := cp.Spec.PublicDomain
		if domain == "" {
			domain = "openstack.local"
		}
		*hostname = fmt.Sprintf("%s.%s", serviceName, domain)
	}

	if gatewayRef.Name == "" {
		*gatewayRef = cp.Spec.GatewayRef
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *ControlPlaneReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&openstackv1alpha1.OpenStackControlPlane{}).
		Owns(&openstackv1alpha1.MariaDB{}).
		Owns(&openstackv1alpha1.RabbitMQ{}).
		Owns(&openstackv1alpha1.Memcached{}).
		Owns(&openstackv1alpha1.OVNNetwork{}).
		Owns(&openstackv1alpha1.Keystone{}).
		Owns(&openstackv1alpha1.Glance{}).
		Owns(&openstackv1alpha1.Placement{}).
		Owns(&openstackv1alpha1.Neutron{}).
		Owns(&openstackv1alpha1.Nova{}).
		Complete(r)
}
