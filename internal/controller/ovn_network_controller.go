package controller

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	openstackv1alpha1 "github.com/mrrauch/openstack-operator/api/v1alpha1"
	"github.com/mrrauch/openstack-operator/internal/common"
	"github.com/mrrauch/openstack-operator/internal/images"
)

// OVNNetworkReconciler reconciles an OVNNetwork object.
type OVNNetworkReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// Reconcile handles the reconciliation loop for OVNNetwork resources.
func (r *OVNNetworkReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	instance := &openstackv1alpha1.OVNNetwork{}
	if err := r.Get(ctx, req.NamespacedName, instance); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Handle deletion
	if !instance.DeletionTimestamp.IsZero() {
		if common.HasFinalizer(instance, common.FinalizerName) {
			common.RemoveFinalizer(instance, common.FinalizerName)
			if err := r.Update(ctx, instance); err != nil {
				return ctrl.Result{}, err
			}
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

	// Set status to Reconciling
	instance.Status.Conditions = common.SetCondition(
		instance.Status.Conditions, "Ready",
		metav1.ConditionFalse, "Reconciling", "Reconciliation in progress",
		instance.Generation,
	)

	// Ensure NB DB (StatefulSet + headless Service)
	nbReplicas := int32(1)
	if instance.Spec.NorthboundDBReplicas != nil {
		nbReplicas = *instance.Spec.NorthboundDBReplicas
	}
	if err := r.ensureOVNDB(ctx, instance, "ovn-nb-db", 6641, images.DefaultOVNNBDB, nbReplicas); err != nil {
		return ctrl.Result{}, err
	}

	// Ensure SB DB (StatefulSet + headless Service)
	sbReplicas := int32(1)
	if instance.Spec.SouthboundDBReplicas != nil {
		sbReplicas = *instance.Spec.SouthboundDBReplicas
	}
	if err := r.ensureOVNDB(ctx, instance, "ovn-sb-db", 6642, images.DefaultOVNSBDB, sbReplicas); err != nil {
		return ctrl.Result{}, err
	}

	// Ensure northd Deployment
	if err := r.ensureNorthd(ctx, instance); err != nil {
		return ctrl.Result{}, err
	}

	// Update status with DB endpoints
	instance.Status.NorthboundDBEndpoint = fmt.Sprintf("tcp:ovn-nb-db.%s.svc:6641", instance.Namespace)
	instance.Status.SouthboundDBEndpoint = fmt.Sprintf("tcp:ovn-sb-db.%s.svc:6642", instance.Namespace)

	// Check if NB DB StatefulSet is ready
	nbSts := &appsv1.StatefulSet{}
	if err := r.Get(ctx, types.NamespacedName{Name: "ovn-nb-db", Namespace: instance.Namespace}, nbSts); err != nil {
		return ctrl.Result{}, err
	}

	if nbSts.Status.ReadyReplicas == nbSts.Status.Replicas && nbSts.Status.ReadyReplicas > 0 {
		instance.Status.Conditions = common.SetCondition(
			instance.Status.Conditions, "Ready",
			metav1.ConditionTrue, "OVNReady", "OVN network is ready",
			instance.Generation,
		)
	}

	instance.Status.ObservedGeneration = instance.Generation
	if err := r.Status().Update(ctx, instance); err != nil {
		logger.Error(err, "failed to update status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *OVNNetworkReconciler) ensureOVNDB(ctx context.Context, instance *openstackv1alpha1.OVNNetwork, name string, port int32, image string, replicas int32) error {
	// Ensure headless Service
	if err := r.ensureHeadlessService(ctx, instance, name, port); err != nil {
		return err
	}

	// Ensure StatefulSet
	return r.ensureDBStatefulSet(ctx, instance, name, port, image, replicas)
}

func (r *OVNNetworkReconciler) ensureHeadlessService(ctx context.Context, instance *openstackv1alpha1.OVNNetwork, name string, port int32) error {
	svc := &corev1.Service{}
	err := r.Get(ctx, types.NamespacedName{Name: name, Namespace: instance.Namespace}, svc)
	if err == nil {
		return nil
	}
	if !errors.IsNotFound(err) {
		return err
	}

	component := componentFromName(name)
	svc = &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: instance.Namespace,
			Labels:    labelsForOVN(instance.Name, component),
		},
		Spec: corev1.ServiceSpec{
			ClusterIP: "None",
			Selector:  labelsForOVN(instance.Name, component),
			Ports: []corev1.ServicePort{
				{Name: "ovsdb", Port: port, Protocol: corev1.ProtocolTCP},
			},
		},
	}
	_ = controllerutil.SetOwnerReference(instance, svc, r.Scheme)
	return r.Create(ctx, svc)
}

func (r *OVNNetworkReconciler) ensureDBStatefulSet(ctx context.Context, instance *openstackv1alpha1.OVNNetwork, name string, port int32, image string, replicas int32) error {
	sts := &appsv1.StatefulSet{}
	err := r.Get(ctx, types.NamespacedName{Name: name, Namespace: instance.Namespace}, sts)
	if err == nil {
		return nil
	}
	if !errors.IsNotFound(err) {
		return err
	}

	storageSize := instance.Spec.DBStorage.Size
	if storageSize.IsZero() {
		storageSize = resource.MustParse("1Gi")
	}

	component := componentFromName(name)
	mountPath := fmt.Sprintf("/var/lib/ovn/%s", name)

	sts = &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: instance.Namespace,
			Labels:    labelsForOVN(instance.Name, component),
		},
		Spec: appsv1.StatefulSetSpec{
			ServiceName: name,
			Replicas:    &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labelsForOVN(instance.Name, component),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labelsForOVN(instance.Name, component),
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  name,
							Image: image,
							Ports: []corev1.ContainerPort{
								{ContainerPort: port, Name: "ovsdb"},
							},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "data", MountPath: mountPath},
							},
						},
					},
				},
			},
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "data"},
					Spec: corev1.PersistentVolumeClaimSpec{
						AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
						Resources: corev1.VolumeResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: storageSize,
							},
						},
						StorageClassName: instance.Spec.DBStorage.StorageClassName,
					},
				},
			},
		},
	}
	_ = controllerutil.SetOwnerReference(instance, sts, r.Scheme)
	return r.Create(ctx, sts)
}

func (r *OVNNetworkReconciler) ensureNorthd(ctx context.Context, instance *openstackv1alpha1.OVNNetwork) error {
	name := "ovn-northd"
	dep := &appsv1.Deployment{}
	err := r.Get(ctx, types.NamespacedName{Name: name, Namespace: instance.Namespace}, dep)
	if err == nil {
		return nil
	}
	if !errors.IsNotFound(err) {
		return err
	}

	replicas := int32(1)
	if instance.Spec.NorthdReplicas != nil {
		replicas = *instance.Spec.NorthdReplicas
	}

	dep = &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: instance.Namespace,
			Labels:    labelsForOVN(instance.Name, "northd"),
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labelsForOVN(instance.Name, "northd"),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labelsForOVN(instance.Name, "northd"),
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "ovn-northd",
							Image: images.DefaultOVNNorthd,
							Env: []corev1.EnvVar{
								{Name: "OVN_NB_DB", Value: fmt.Sprintf("tcp:ovn-nb-db.%s.svc:6641", instance.Namespace)},
								{Name: "OVN_SB_DB", Value: fmt.Sprintf("tcp:ovn-sb-db.%s.svc:6642", instance.Namespace)},
							},
						},
					},
				},
			},
		},
	}
	_ = controllerutil.SetOwnerReference(instance, dep, r.Scheme)
	return r.Create(ctx, dep)
}

// SetupWithManager sets up the controller with the Manager.
func (r *OVNNetworkReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&openstackv1alpha1.OVNNetwork{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Complete(r)
}

func labelsForOVN(name, component string) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       "ovn",
		"app.kubernetes.io/instance":   name,
		"app.kubernetes.io/component":  component,
		"app.kubernetes.io/managed-by": "openstack-operator",
	}
}

// componentFromName derives a component label value from the resource name.
func componentFromName(name string) string {
	switch name {
	case "ovn-nb-db":
		return "nb-db"
	case "ovn-sb-db":
		return "sb-db"
	default:
		return name
	}
}
