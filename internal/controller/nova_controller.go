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

// NovaReconciler reconciles a Nova object.
type NovaReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// Reconcile handles the reconciliation loop for Nova resources.
func (r *NovaReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	instance := &openstackv1alpha1.Nova{}
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

	// Ensure DB password secret
	dbSecretName := fmt.Sprintf("%s-db-password", instance.Name)
	if err := common.EnsureSecret(ctx, r.Client, dbSecretName, instance.Namespace, map[string]int{"password": 32}, instance); err != nil {
		return ctrl.Result{}, err
	}

	// Ensure databases (nova, nova_api, nova_cell0) via custom Job
	if err := r.ensureDBCreate(ctx, instance, dbSecretName); err != nil {
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

	// Ensure db-sync via custom Job (api_db sync + db sync)
	if err := r.ensureDBSync(ctx, instance, dbSecretName); err != nil {
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

	// Ensure cell setup Job
	if err := r.ensureCellSetup(ctx, instance); err != nil {
		return ctrl.Result{}, err
	}

	// Ensure Deployments
	apiImage := images.ImageOrDefault(instance.Spec.Image, images.DefaultNovaAPI)
	apiName := fmt.Sprintf("%s-api", instance.Name)

	apiReplicas := int32(1)
	if instance.Spec.Replicas != nil {
		apiReplicas = *instance.Spec.Replicas
	}
	if err := r.ensureDeployment(ctx, instance, apiName, apiImage, "api", apiReplicas, []corev1.ContainerPort{
		{ContainerPort: 8774, Name: "api"},
	}); err != nil {
		return ctrl.Result{}, err
	}

	schedulerName := fmt.Sprintf("%s-scheduler", instance.Name)
	if err := r.ensureDeployment(ctx, instance, schedulerName, images.DefaultNovaScheduler, "scheduler", 1, nil); err != nil {
		return ctrl.Result{}, err
	}

	conductorName := fmt.Sprintf("%s-conductor", instance.Name)
	if err := r.ensureDeployment(ctx, instance, conductorName, images.DefaultNovaConductor, "conductor", 1, nil); err != nil {
		return ctrl.Result{}, err
	}

	computeReplicas := int32(1)
	if instance.Spec.ComputeReplicas != nil {
		computeReplicas = *instance.Spec.ComputeReplicas
	}
	computeName := fmt.Sprintf("%s-compute", instance.Name)
	if err := r.ensureDeployment(ctx, instance, computeName, images.DefaultNovaCompute, "compute", computeReplicas, nil); err != nil {
		return ctrl.Result{}, err
	}

	// Ensure Service for nova-api
	if err := r.ensureService(ctx, instance, apiName); err != nil {
		return ctrl.Result{}, err
	}

	// Ensure HTTPRoute for nova-api
	if err := common.EnsureHTTPRoute(ctx, r.Client, common.HTTPRouteParams{
		Name:             apiName,
		Namespace:        instance.Namespace,
		Hostname:         instance.Spec.PublicHostname,
		ServiceName:      apiName,
		ServicePort:      8774,
		GatewayName:      instance.Spec.GatewayRef.Name,
		GatewayNamespace: instance.Spec.GatewayRef.Namespace,
		ListenerName:     instance.Spec.GatewayRef.ListenerName,
	}, instance); err != nil {
		return ctrl.Result{}, err
	}

	// Ensure Keystone endpoint
	internalURL := fmt.Sprintf("http://%s.%s.svc:8774/v2.1", apiName, instance.Namespace)
	publicURL := fmt.Sprintf("https://%s/v2.1", instance.Spec.PublicHostname)
	if err := common.EnsureKeystoneEndpoint(ctx, r.Client, common.EndpointParams{
		Name:           instance.Name,
		Namespace:      instance.Namespace,
		ServiceName:    "nova",
		ServiceType:    "compute",
		InternalURL:    internalURL,
		PublicURL:      publicURL,
		AdminURL:       internalURL,
		Region:         "RegionOne",
		KeystoneSecret: "keystone-admin-password",
		KeystoneURL:    "http://keystone-api.openstack.svc:5000/v3",
		BootstrapImage: images.DefaultKeystone,
	}, instance); err != nil {
		return ctrl.Result{}, err
	}

	// Update status with apiEndpoint
	instance.Status.APIEndpoint = fmt.Sprintf("http://%s.%s.svc:8774", apiName, instance.Namespace)

	// Check API Deployment readiness
	dep := &appsv1.Deployment{}
	if err := r.Get(ctx, types.NamespacedName{Name: apiName, Namespace: instance.Namespace}, dep); err != nil {
		return ctrl.Result{}, err
	}

	if dep.Status.ReadyReplicas > 0 && dep.Status.ReadyReplicas == dep.Status.Replicas {
		instance.Status.Conditions = common.SetCondition(
			instance.Status.Conditions, "Ready",
			metav1.ConditionTrue, "DeploymentReady", "Nova is ready",
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

func (r *NovaReconciler) ensureDBCreate(ctx context.Context, instance *openstackv1alpha1.Nova, secretName string) error {
	jobName := fmt.Sprintf("%s-db-create", instance.Name)

	existing := &batchv1.Job{}
	err := r.Get(ctx, types.NamespacedName{Name: jobName, Namespace: instance.Namespace}, existing)
	if err == nil {
		return nil
	}
	if !errors.IsNotFound(err) {
		return err
	}

	script := `mysql -h mariadb.openstack.svc -u root -p"$ROOT_PASSWORD" -e "` +
		`CREATE DATABASE IF NOT EXISTS nova; ` +
		`CREATE DATABASE IF NOT EXISTS nova_api; ` +
		`CREATE DATABASE IF NOT EXISTS nova_cell0; ` +
		`CREATE USER IF NOT EXISTS 'nova'@'%' IDENTIFIED BY '$SERVICE_PASSWORD'; ` +
		`GRANT ALL ON nova.* TO 'nova'@'%'; ` +
		`GRANT ALL ON nova_api.* TO 'nova'@'%'; ` +
		`GRANT ALL ON nova_cell0.* TO 'nova'@'%'; ` +
		`FLUSH PRIVILEGES;"`

	backoffLimit := int32(4)
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: instance.Namespace,
			Labels:    labelsForNova(instance.Name, "api"),
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: &backoffLimit,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyOnFailure,
					Containers: []corev1.Container{
						{
							Name:    "db-create",
							Image:   "mariadb:11",
							Command: []string{"sh", "-c", script},
							Env: []corev1.EnvVar{
								{
									Name: "ROOT_PASSWORD",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{Name: "mariadb-root-password"},
											Key:                  "password",
										},
									},
								},
								{
									Name: "SERVICE_PASSWORD",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
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

func (r *NovaReconciler) ensureDBSync(ctx context.Context, instance *openstackv1alpha1.Nova, secretName string) error {
	jobName := fmt.Sprintf("%s-db-sync", instance.Name)

	existing := &batchv1.Job{}
	err := r.Get(ctx, types.NamespacedName{Name: jobName, Namespace: instance.Namespace}, existing)
	if err == nil {
		return nil
	}
	if !errors.IsNotFound(err) {
		return err
	}

	image := images.ImageOrDefault(instance.Spec.Image, images.DefaultNovaAPI)
	backoffLimit := int32(4)
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: instance.Namespace,
			Labels:    labelsForNova(instance.Name, "api"),
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: &backoffLimit,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyOnFailure,
					Containers: []corev1.Container{
						{
							Name:    "db-sync",
							Image:   image,
							Command: []string{"sh", "-c", "nova-manage api_db sync && nova-manage db sync"},
							Env: []corev1.EnvVar{
								{
									Name: "DB_PASSWORD",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
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

func (r *NovaReconciler) ensureCellSetup(ctx context.Context, instance *openstackv1alpha1.Nova) error {
	jobName := fmt.Sprintf("%s-cell-setup", instance.Name)

	existing := &batchv1.Job{}
	err := r.Get(ctx, types.NamespacedName{Name: jobName, Namespace: instance.Namespace}, existing)
	if err == nil {
		return nil
	}
	if !errors.IsNotFound(err) {
		return err
	}

	image := images.ImageOrDefault(instance.Spec.Image, images.DefaultNovaAPI)
	backoffLimit := int32(4)
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: instance.Namespace,
			Labels:    labelsForNova(instance.Name, "api"),
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: &backoffLimit,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyOnFailure,
					Containers: []corev1.Container{
						{
							Name:    "cell-setup",
							Image:   image,
							Command: []string{"sh", "-c", "nova-manage cell_v2 map_cell0 && nova-manage cell_v2 create_cell --name cell1 || true"},
						},
					},
				},
			},
		},
	}
	_ = controllerutil.SetOwnerReference(instance, job, r.Scheme)
	return r.Create(ctx, job)
}

func (r *NovaReconciler) ensureDeployment(ctx context.Context, instance *openstackv1alpha1.Nova, name, image, component string, replicas int32, ports []corev1.ContainerPort) error {
	dep := &appsv1.Deployment{}
	err := r.Get(ctx, types.NamespacedName{Name: name, Namespace: instance.Namespace}, dep)
	if err == nil {
		return nil
	}
	if !errors.IsNotFound(err) {
		return err
	}

	dep = &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: instance.Namespace,
			Labels:    labelsForNova(instance.Name, component),
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labelsForNova(instance.Name, component),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labelsForNova(instance.Name, component),
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  component,
							Image: image,
							Ports: ports,
						},
					},
				},
			},
		},
	}
	_ = controllerutil.SetOwnerReference(instance, dep, r.Scheme)
	return r.Create(ctx, dep)
}

func (r *NovaReconciler) ensureService(ctx context.Context, instance *openstackv1alpha1.Nova, name string) error {
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
			Labels:    labelsForNova(instance.Name, "api"),
		},
		Spec: corev1.ServiceSpec{
			Selector: labelsForNova(instance.Name, "api"),
			Ports: []corev1.ServicePort{
				{
					Name:       "api",
					Port:       8774,
					TargetPort: intstr.FromInt32(8774),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}
	_ = controllerutil.SetOwnerReference(instance, svc, r.Scheme)
	return r.Create(ctx, svc)
}

// SetupWithManager sets up the controller with the Manager.
func (r *NovaReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&openstackv1alpha1.Nova{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.Secret{}).
		Owns(&batchv1.Job{}).
		Complete(r)
}

func labelsForNova(name, component string) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       "nova",
		"app.kubernetes.io/instance":   name,
		"app.kubernetes.io/component":  component,
		"app.kubernetes.io/managed-by": "openstack-operator",
	}
}
