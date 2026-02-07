package controller

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
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

// GlanceReconciler reconciles a Glance object.
type GlanceReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// Reconcile handles the reconciliation loop for Glance resources.
func (r *GlanceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	instance := &openstackv1alpha1.Glance{}
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

	// Ensure database
	dbHost, dbRootSecret := databaseDependency(instance.Name, "-glance", instance.Namespace)
	if err := common.EnsureDatabase(ctx, r.Client, common.DatabaseParams{
		Name:           instance.Name,
		Namespace:      instance.Namespace,
		DatabaseName:   "glance",
		Username:       "glance",
		SecretName:     dbSecretName,
		DatabaseSecret: dbRootSecret,
		DatabaseHost:   dbHost,
	}, instance); err != nil {
		return ctrl.Result{}, err
	}

	// Wait for db-create to complete
	dbCreateDone, result, err := waitForJobCompletion(ctx, r.Client, fmt.Sprintf("%s-db-create", instance.Name), instance.Namespace, 5*time.Second, 2*time.Second)
	if err != nil {
		return ctrl.Result{}, err
	}
	if !dbCreateDone {
		return result, nil
	}

	// Ensure db_sync
	image := images.ImageOrDefault(instance.Spec.Image, images.DefaultGlanceAPI)
	if err := common.EnsureDBSync(ctx, r.Client, common.DBSyncParams{
		Name:       instance.Name,
		Namespace:  instance.Namespace,
		Image:      image,
		Command:    []string{"glance-manage", "db_sync"},
		SecretName: dbSecretName,
	}, instance); err != nil {
		return ctrl.Result{}, err
	}

	// Wait for db-sync to complete
	dbSyncDone, result, err := waitForJobCompletion(ctx, r.Client, fmt.Sprintf("%s-db-sync", instance.Name), instance.Namespace, 5*time.Second, 2*time.Second)
	if err != nil {
		return ctrl.Result{}, err
	}
	if !dbSyncDone {
		return result, nil
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
		ServicePort:      9292,
		GatewayName:      instance.Spec.GatewayRef.Name,
		GatewayNamespace: instance.Spec.GatewayRef.Namespace,
		ListenerName:     instance.Spec.GatewayRef.ListenerName,
	}, instance); err != nil {
		return ctrl.Result{}, err
	}

	// Ensure Keystone endpoint
	internalURL := fmt.Sprintf("http://%s.%s.svc:9292", deploymentName, instance.Namespace)
	publicURL := fmt.Sprintf("https://%s", instance.Spec.PublicHostname)
	keystoneURL, keystoneSecret := keystoneDependency(instance.Name, "-glance", instance.Namespace)
	if err := common.EnsureKeystoneEndpoint(ctx, r.Client, common.EndpointParams{
		Name:           instance.Name,
		Namespace:      instance.Namespace,
		ServiceName:    "glance",
		ServiceType:    "image",
		InternalURL:    internalURL,
		PublicURL:      publicURL,
		AdminURL:       internalURL,
		Region:         "RegionOne",
		KeystoneSecret: keystoneSecret,
		KeystoneURL:    keystoneURL,
		BootstrapImage: images.DefaultKeystone,
	}, instance); err != nil {
		return ctrl.Result{}, err
	}
	endpointDone, result, err := waitForJobCompletion(ctx, r.Client, fmt.Sprintf("%s-endpoint-create", instance.Name), instance.Namespace, 5*time.Second, 2*time.Second)
	if err != nil {
		return ctrl.Result{}, err
	}
	if !endpointDone {
		return result, nil
	}

	// Update status with apiEndpoint
	instance.Status.APIEndpoint = fmt.Sprintf("http://%s.%s.svc:9292", deploymentName, instance.Namespace)

	// Check Deployment readiness
	dep := &appsv1.Deployment{}
	if err := r.Get(ctx, types.NamespacedName{Name: deploymentName, Namespace: instance.Namespace}, dep); err != nil {
		return ctrl.Result{}, err
	}

	if dep.Status.ReadyReplicas > 0 && dep.Status.ReadyReplicas == dep.Status.Replicas {
		instance.Status.Conditions = common.SetCondition(
			instance.Status.Conditions, "Ready",
			metav1.ConditionTrue, "DeploymentReady", "Glance is ready",
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

func (r *GlanceReconciler) ensureDeployment(ctx context.Context, instance *openstackv1alpha1.Glance, name, image string) error {
	replicas := int32(1)
	if instance.Spec.Replicas != nil {
		replicas = *instance.Spec.Replicas
	}

	var volumes []corev1.Volume
	var volumeMounts []corev1.VolumeMount

	// Add PVC storage when storageType is "pvc" or empty (default)
	if instance.Spec.StorageType == "" || instance.Spec.StorageType == "pvc" {
		storageSize := instance.Spec.Storage.Size
		if storageSize.IsZero() {
			storageSize = resource.MustParse("10Gi")
		}

		volumes = append(volumes, corev1.Volume{
			Name: "glance-images",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: fmt.Sprintf("%s-images", name),
				},
			},
		})
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      "glance-images",
			MountPath: "/var/lib/glance/images",
		})

		// Ensure PVC exists
		if pvcErr := r.ensurePVC(ctx, instance, fmt.Sprintf("%s-images", name), storageSize); pvcErr != nil {
			return pvcErr
		}
	}

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: instance.Namespace,
		},
	}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, dep, func() error {
		dep.Labels = labelsForGlance(instance.Name)
		dep.Spec.Replicas = &replicas
		dep.Spec.Selector = &metav1.LabelSelector{
			MatchLabels: labelsForGlance(instance.Name),
		}
		dep.Spec.Template = corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: labelsForGlance(instance.Name),
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  "glance-api",
						Image: image,
						Ports: []corev1.ContainerPort{
							{ContainerPort: 9292, Name: "api"},
						},
						VolumeMounts: volumeMounts,
					},
				},
				Volumes: volumes,
			},
		}
		return controllerutil.SetOwnerReference(instance, dep, r.Scheme)
	})
	return err
}

func (r *GlanceReconciler) ensurePVC(ctx context.Context, instance *openstackv1alpha1.Glance, name string, size resource.Quantity) error {
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: instance.Namespace,
		},
	}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, pvc, func() error {
		pvc.Labels = labelsForGlance(instance.Name)
		pvc.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}
		pvc.Spec.Resources = corev1.VolumeResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceStorage: size,
			},
		}
		pvc.Spec.StorageClassName = instance.Spec.Storage.StorageClassName
		return controllerutil.SetOwnerReference(instance, pvc, r.Scheme)
	})
	return err
}

func (r *GlanceReconciler) ensureService(ctx context.Context, instance *openstackv1alpha1.Glance, name string) error {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: instance.Namespace,
		},
	}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, svc, func() error {
		svc.Labels = labelsForGlance(instance.Name)
		svc.Spec.Selector = labelsForGlance(instance.Name)
		svc.Spec.Ports = []corev1.ServicePort{
			{
				Name:       "api",
				Port:       9292,
				TargetPort: intstr.FromInt32(9292),
				Protocol:   corev1.ProtocolTCP,
			},
		}
		return controllerutil.SetOwnerReference(instance, svc, r.Scheme)
	})
	return err
}

// SetupWithManager sets up the controller with the Manager.
func (r *GlanceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&openstackv1alpha1.Glance{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.Secret{}).
		Owns(&batchv1.Job{}).
		Complete(r)
}

func labelsForGlance(name string) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       "glance",
		"app.kubernetes.io/instance":   name,
		"app.kubernetes.io/managed-by": "openstack-operator",
	}
}
