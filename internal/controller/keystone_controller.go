package controller

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
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

// KeystoneReconciler reconciles a Keystone object.
type KeystoneReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// Reconcile handles the reconciliation loop for Keystone resources.
func (r *KeystoneReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	instance := &openstackv1alpha1.Keystone{}
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

	// Ensure admin password secret
	adminSecretName := fmt.Sprintf("%s-admin-password", instance.Name)
	if err := common.EnsureSecret(ctx, r.Client, adminSecretName, instance.Namespace, map[string]int{"password": 32}, instance); err != nil {
		return ctrl.Result{}, err
	}

	// Ensure DB password secret
	dbSecretName := fmt.Sprintf("%s-db-password", instance.Name)
	if err := common.EnsureSecret(ctx, r.Client, dbSecretName, instance.Namespace, map[string]int{"password": 32}, instance); err != nil {
		return ctrl.Result{}, err
	}

	// Ensure database
	if err := common.EnsureDatabase(ctx, r.Client, common.DatabaseParams{
		Name:          instance.Name,
		Namespace:     instance.Namespace,
		DatabaseName:  "keystone",
		Username:      "keystone",
		SecretName:    dbSecretName,
		MariaDBSecret: "mariadb-root-password",
		MariaDBHost:   "mariadb.openstack.svc",
	}, instance); err != nil {
		return ctrl.Result{}, err
	}

	// Wait for db-create to complete
	dbCreateDone, err := common.IsJobComplete(ctx, r.Client, fmt.Sprintf("%s-db-create", instance.Name), instance.Namespace)
	if err != nil {
		return ctrl.Result{}, err
	}
	if !dbCreateDone {
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	// Ensure db_sync
	image := images.ImageOrDefault(instance.Spec.Image, images.DefaultKeystone)
	if err := common.EnsureDBSync(ctx, r.Client, common.DBSyncParams{
		Name:       instance.Name,
		Namespace:  instance.Namespace,
		Image:      image,
		Command:    []string{"keystone-manage", "db_sync"},
		SecretName: dbSecretName,
	}, instance); err != nil {
		return ctrl.Result{}, err
	}

	// Wait for db-sync to complete
	dbSyncDone, err := common.IsJobComplete(ctx, r.Client, fmt.Sprintf("%s-db-sync", instance.Name), instance.Namespace)
	if err != nil {
		return ctrl.Result{}, err
	}
	if !dbSyncDone {
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	// Ensure Deployment
	deploymentName := fmt.Sprintf("%s-api", instance.Name)
	if err := r.ensureDeployment(ctx, instance, deploymentName, image); err != nil {
		return ctrl.Result{}, err
	}

	// Ensure Service
	if err := r.ensureService(ctx, instance, deploymentName); err != nil {
		return ctrl.Result{}, err
	}

	// Ensure HTTPRoute
	if err := common.EnsureHTTPRoute(ctx, r.Client, common.HTTPRouteParams{
		Name:             deploymentName,
		Namespace:        instance.Namespace,
		Hostname:         instance.Spec.PublicHostname,
		ServiceName:      deploymentName,
		ServicePort:      5000,
		GatewayName:      instance.Spec.GatewayRef.Name,
		GatewayNamespace: instance.Spec.GatewayRef.Namespace,
		ListenerName:     instance.Spec.GatewayRef.ListenerName,
	}, instance); err != nil {
		return ctrl.Result{}, err
	}

	// Ensure bootstrap job
	if err := r.ensureBootstrap(ctx, instance, image, adminSecretName, deploymentName); err != nil {
		return ctrl.Result{}, err
	}

	// Update status with apiEndpoint
	instance.Status.APIEndpoint = fmt.Sprintf("http://%s.%s.svc:5000/v3", deploymentName, instance.Namespace)

	// Check Deployment readiness
	dep := &appsv1.Deployment{}
	if err := r.Get(ctx, types.NamespacedName{Name: deploymentName, Namespace: instance.Namespace}, dep); err != nil {
		return ctrl.Result{}, err
	}

	if dep.Status.ReadyReplicas > 0 && dep.Status.ReadyReplicas == dep.Status.Replicas {
		instance.Status.Conditions = common.SetCondition(
			instance.Status.Conditions, "Ready",
			metav1.ConditionTrue, "DeploymentReady", "Keystone is ready",
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

func (r *KeystoneReconciler) ensureDeployment(ctx context.Context, instance *openstackv1alpha1.Keystone, name, image string) error {
	dep := &appsv1.Deployment{}
	err := r.Get(ctx, types.NamespacedName{Name: name, Namespace: instance.Namespace}, dep)
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

	dep = &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: instance.Namespace,
			Labels:    labelsForKeystone(instance.Name),
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labelsForKeystone(instance.Name),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labelsForKeystone(instance.Name),
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "keystone-api",
							Image: image,
							Ports: []corev1.ContainerPort{
								{ContainerPort: 5000, Name: "api"},
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

func (r *KeystoneReconciler) ensureService(ctx context.Context, instance *openstackv1alpha1.Keystone, name string) error {
	svc := &corev1.Service{}
	err := r.Get(ctx, types.NamespacedName{Name: name, Namespace: instance.Namespace}, svc)
	if err == nil {
		return nil
	}
	if !errors.IsNotFound(err) {
		return err
	}

	svc = &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: instance.Namespace,
			Labels:    labelsForKeystone(instance.Name),
		},
		Spec: corev1.ServiceSpec{
			Selector: labelsForKeystone(instance.Name),
			Ports: []corev1.ServicePort{
				{
					Name:       "api",
					Port:       5000,
					TargetPort: intstr.FromInt32(5000),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}
	_ = controllerutil.SetOwnerReference(instance, svc, r.Scheme)
	return r.Create(ctx, svc)
}

func (r *KeystoneReconciler) ensureBootstrap(ctx context.Context, instance *openstackv1alpha1.Keystone, image, adminSecretName, deploymentName string) error {
	jobName := fmt.Sprintf("%s-bootstrap", instance.Name)

	existing := &batchv1.Job{}
	err := r.Get(ctx, types.NamespacedName{Name: jobName, Namespace: instance.Namespace}, existing)
	if err == nil {
		return nil
	}
	if !errors.IsNotFound(err) {
		return err
	}

	internalURL := fmt.Sprintf("http://%s.%s.svc:5000/v3", deploymentName, instance.Namespace)
	publicURL := fmt.Sprintf("https://%s/v3", instance.Spec.PublicHostname)

	bootstrapCmd := fmt.Sprintf(
		`keystone-manage bootstrap --bootstrap-password "$ADMIN_PASSWORD" --bootstrap-admin-url %s --bootstrap-internal-url %s --bootstrap-public-url %s --bootstrap-region-id RegionOne`,
		internalURL, internalURL, publicURL,
	)

	backoffLimit := int32(4)
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: instance.Namespace,
			Labels:    labelsForKeystone(instance.Name),
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: &backoffLimit,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyOnFailure,
					Containers: []corev1.Container{
						{
							Name:    "bootstrap",
							Image:   image,
							Command: []string{"sh", "-c", bootstrapCmd},
							Env: []corev1.EnvVar{
								{
									Name: "ADMIN_PASSWORD",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{Name: adminSecretName},
											Key:                  "password",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	_ = controllerutil.SetOwnerReference(instance, job, r.Scheme)
	return r.Create(ctx, job)
}

// SetupWithManager sets up the controller with the Manager.
func (r *KeystoneReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&openstackv1alpha1.Keystone{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.Secret{}).
		Owns(&batchv1.Job{}).
		Complete(r)
}

func labelsForKeystone(name string) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       "keystone",
		"app.kubernetes.io/instance":   name,
		"app.kubernetes.io/managed-by": "openstack-operator",
	}
}
