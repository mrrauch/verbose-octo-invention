package controller

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	openstackv1alpha1 "github.com/mrrauch/openstack-operator/api/v1alpha1"
	"github.com/mrrauch/openstack-operator/internal/common"
	"github.com/mrrauch/openstack-operator/internal/images"
)

// MemcachedReconciler reconciles a Memcached object.
type MemcachedReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// Reconcile handles the reconciliation loop for Memcached resources.
func (r *MemcachedReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	instance := &openstackv1alpha1.Memcached{}
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

	// Set status to Progressing
	instance.Status.Conditions = common.SetCondition(
		instance.Status.Conditions, "Ready",
		metav1.ConditionFalse, "Reconciling", "Reconciliation in progress",
		instance.Generation,
	)

	// Ensure Service
	if err := r.ensureService(ctx, instance); err != nil {
		return ctrl.Result{}, err
	}

	// Ensure Deployment
	if err := r.ensureDeployment(ctx, instance); err != nil {
		return ctrl.Result{}, err
	}

	// Check if Deployment is ready
	deploy := &appsv1.Deployment{}
	if err := r.Get(ctx, types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}, deploy); err != nil {
		return ctrl.Result{}, err
	}

	if deploy.Status.ReadyReplicas == deploy.Status.Replicas && deploy.Status.ReadyReplicas > 0 {
		instance.Status.Conditions = common.SetCondition(
			instance.Status.Conditions, "Ready",
			metav1.ConditionTrue, "DeploymentReady", "Memcached is ready",
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

func (r *MemcachedReconciler) ensureService(ctx context.Context, instance *openstackv1alpha1.Memcached) error {
	svc := &corev1.Service{}
	err := r.Get(ctx, types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}, svc)
	if err == nil {
		return nil
	}
	if !errors.IsNotFound(err) {
		return err
	}

	svc = &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Name,
			Namespace: instance.Namespace,
			Labels:    labelsForMemcached(instance.Name),
		},
		Spec: corev1.ServiceSpec{
			Selector: labelsForMemcached(instance.Name),
			Ports: []corev1.ServicePort{
				{Name: "memcached", Port: 11211, Protocol: corev1.ProtocolTCP},
			},
		},
	}
	_ = controllerutil.SetOwnerReference(instance, svc, r.Scheme)
	return r.Create(ctx, svc)
}

func (r *MemcachedReconciler) ensureDeployment(ctx context.Context, instance *openstackv1alpha1.Memcached) error {
	deploy := &appsv1.Deployment{}
	err := r.Get(ctx, types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}, deploy)
	if err == nil {
		return nil
	}
	if !errors.IsNotFound(err) {
		return err
	}

	replicas := int32(1)
	if instance.Spec.Replicas != nil {
		replicas = *instance.Spec.Replicas
	}

	image := images.ImageOrDefault(instance.Spec.Image, images.DefaultMemcached)

	deploy = &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Name,
			Namespace: instance.Namespace,
			Labels:    labelsForMemcached(instance.Name),
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labelsForMemcached(instance.Name),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labelsForMemcached(instance.Name),
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "memcached",
							Image: image,
							Ports: []corev1.ContainerPort{
								{ContainerPort: 11211, Name: "memcached"},
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									TCPSocket: &corev1.TCPSocketAction{
										Port: intstr.FromInt32(11211),
									},
								},
								InitialDelaySeconds: 5,
								PeriodSeconds:       5,
							},
						},
					},
				},
			},
		},
	}
	_ = controllerutil.SetOwnerReference(instance, deploy, r.Scheme)
	return r.Create(ctx, deploy)
}

// SetupWithManager sets up the controller with the Manager.
func (r *MemcachedReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&openstackv1alpha1.Memcached{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Complete(r)
}

func labelsForMemcached(name string) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       "memcached",
		"app.kubernetes.io/instance":   name,
		"app.kubernetes.io/managed-by": "openstack-operator",
	}
}
