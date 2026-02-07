package controller

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
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

// DatabaseReconciler reconciles a Database object.
type DatabaseReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// Reconcile handles the reconciliation loop for Database resources.
func (r *DatabaseReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	instance := &openstackv1alpha1.Database{}
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
		return ctrl.Result{Requeue: true}, nil
	}

	// Set status to Progressing
	instance.Status.Conditions = common.SetCondition(
		instance.Status.Conditions, "Ready",
		metav1.ConditionFalse, "Reconciling", "Reconciliation in progress",
		instance.Generation,
	)

	// Ensure root password secret
	secretName := fmt.Sprintf("%s-root-password", instance.Name)
	if err := common.EnsureSecret(ctx, r.Client, secretName, instance.Namespace, map[string]int{"password": 32}, instance); err != nil {
		return ctrl.Result{}, err
	}

	// Ensure headless Service
	if err := r.ensureService(ctx, instance); err != nil {
		return ctrl.Result{}, err
	}

	// Ensure StatefulSet
	if err := r.ensureStatefulSet(ctx, instance, secretName); err != nil {
		return ctrl.Result{}, err
	}

	// Check if StatefulSet is ready
	sts := &appsv1.StatefulSet{}
	if err := r.Get(ctx, types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}, sts); err != nil {
		return ctrl.Result{}, err
	}

	if sts.Status.ReadyReplicas == sts.Status.Replicas && sts.Status.ReadyReplicas > 0 {
		instance.Status.Conditions = common.SetCondition(
			instance.Status.Conditions, "Ready",
			metav1.ConditionTrue, "StatefulSetReady", "Database is ready",
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

func (r *DatabaseReconciler) ensureService(ctx context.Context, instance *openstackv1alpha1.Database) error {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Name,
			Namespace: instance.Namespace,
		},
	}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, svc, func() error {
		svc.Labels = labelsForDatabase(instance.Name)
		svc.Spec.ClusterIP = corev1.ClusterIPNone
		svc.Spec.Selector = labelsForDatabase(instance.Name)
		svc.Spec.Ports = []corev1.ServicePort{
			{Name: "postgresql", Port: 5432, Protocol: corev1.ProtocolTCP},
		}
		return controllerutil.SetOwnerReference(instance, svc, r.Scheme)
	})
	return err
}

func (r *DatabaseReconciler) ensureStatefulSet(ctx context.Context, instance *openstackv1alpha1.Database, secretName string) error {
	replicas := int32(1)
	if instance.Spec.Replicas != nil {
		replicas = *instance.Spec.Replicas
	}

	image := images.ImageOrDefault(instance.Spec.Image, images.DefaultPostgreSQL)

	storageSize := instance.Spec.Storage.Size
	if storageSize.IsZero() {
		storageSize = resource.MustParse("10Gi")
	}

	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Name,
			Namespace: instance.Namespace,
		},
	}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, sts, func() error {
		sts.Labels = labelsForDatabase(instance.Name)
		sts.Spec.ServiceName = instance.Name
		sts.Spec.Replicas = &replicas
		sts.Spec.Selector = &metav1.LabelSelector{
			MatchLabels: labelsForDatabase(instance.Name),
		}
		sts.Spec.Template = corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: labelsForDatabase(instance.Name),
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  "postgresql",
						Image: image,
						Ports: []corev1.ContainerPort{
							{ContainerPort: 5432, Name: "postgresql"},
						},
						Env: []corev1.EnvVar{
							{
								Name: "POSTGRES_PASSWORD",
								ValueFrom: &corev1.EnvVarSource{
									SecretKeyRef: &corev1.SecretKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
										Key:                  "password",
									},
								},
							},
						},
						VolumeMounts: []corev1.VolumeMount{
							{Name: "data", MountPath: "/var/lib/postgresql/data"},
						},
						ReadinessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								Exec: &corev1.ExecAction{
									Command: []string{"pg_isready", "-U", "postgres"},
								},
							},
							InitialDelaySeconds: 10,
							PeriodSeconds:       5,
						},
					},
				},
			},
		}
		sts.Spec.VolumeClaimTemplates = []corev1.PersistentVolumeClaim{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "data"},
				Spec: corev1.PersistentVolumeClaimSpec{
					AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
					Resources: corev1.VolumeResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceStorage: storageSize,
						},
					},
					StorageClassName: instance.Spec.Storage.StorageClassName,
				},
			},
		}
		return controllerutil.SetOwnerReference(instance, sts, r.Scheme)
	})
	return err
}

// SetupWithManager sets up the controller with the Manager.
func (r *DatabaseReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&openstackv1alpha1.Database{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.Secret{}).
		Complete(r)
}

func labelsForDatabase(name string) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       "database",
		"app.kubernetes.io/instance":   name,
		"app.kubernetes.io/managed-by": "openstack-operator",
	}
}
