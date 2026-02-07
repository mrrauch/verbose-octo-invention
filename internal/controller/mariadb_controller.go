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

// MariaDBReconciler reconciles a MariaDB object.
type MariaDBReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// Reconcile handles the reconciliation loop for MariaDB resources.
func (r *MariaDBReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	instance := &openstackv1alpha1.MariaDB{}
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
			metav1.ConditionTrue, "StatefulSetReady", "MariaDB is ready",
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

func (r *MariaDBReconciler) ensureService(ctx context.Context, instance *openstackv1alpha1.MariaDB) error {
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
			Labels:    labelsForMariaDB(instance.Name),
		},
		Spec: corev1.ServiceSpec{
			ClusterIP: "None",
			Selector:  labelsForMariaDB(instance.Name),
			Ports: []corev1.ServicePort{
				{Name: "mysql", Port: 3306, Protocol: corev1.ProtocolTCP},
			},
		},
	}
	_ = controllerutil.SetOwnerReference(instance, svc, r.Scheme)
	return r.Create(ctx, svc)
}

func (r *MariaDBReconciler) ensureStatefulSet(ctx context.Context, instance *openstackv1alpha1.MariaDB, secretName string) error {
	sts := &appsv1.StatefulSet{}
	err := r.Get(ctx, types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}, sts)
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

	image := images.ImageOrDefault(instance.Spec.Image, images.DefaultMariaDB)

	storageSize := instance.Spec.Storage.Size
	if storageSize.IsZero() {
		storageSize = resource.MustParse("10Gi")
	}

	sts = &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Name,
			Namespace: instance.Namespace,
			Labels:    labelsForMariaDB(instance.Name),
		},
		Spec: appsv1.StatefulSetSpec{
			ServiceName: instance.Name,
			Replicas:    &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labelsForMariaDB(instance.Name),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labelsForMariaDB(instance.Name),
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "mariadb",
							Image: image,
							Ports: []corev1.ContainerPort{
								{ContainerPort: 3306, Name: "mysql"},
							},
							Env: []corev1.EnvVar{
								{
									Name: "MYSQL_ROOT_PASSWORD",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
											Key:                  "password",
										},
									},
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "data", MountPath: "/var/lib/mysql"},
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									Exec: &corev1.ExecAction{
										Command: []string{"mysqladmin", "ping", "-h", "127.0.0.1"},
									},
								},
								InitialDelaySeconds: 10,
								PeriodSeconds:       5,
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
						StorageClassName: instance.Spec.Storage.StorageClassName,
					},
				},
			},
		},
	}
	_ = controllerutil.SetOwnerReference(instance, sts, r.Scheme)
	return r.Create(ctx, sts)
}

// SetupWithManager sets up the controller with the Manager.
func (r *MariaDBReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&openstackv1alpha1.MariaDB{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.Secret{}).
		Complete(r)
}

func labelsForMariaDB(name string) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       "mariadb",
		"app.kubernetes.io/instance":   name,
		"app.kubernetes.io/managed-by": "openstack-operator",
	}
}
