package controller

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
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

// NeutronReconciler reconciles a Neutron object.
type NeutronReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// Reconcile handles the reconciliation loop for Neutron resources.
func (r *NeutronReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	instance := &openstackv1alpha1.Neutron{}
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
	dbHost, dbRootSecret := databaseDependency(instance.Name, "-neutron", instance.Namespace)
	if err := common.EnsureDatabase(ctx, r.Client, common.DatabaseParams{
		Name:           instance.Name,
		Namespace:      instance.Namespace,
		DatabaseName:   "neutron",
		Username:       "neutron",
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
	image := images.ImageOrDefault(instance.Spec.Image, images.DefaultNeutronServer)
	if err := common.EnsureDBSync(ctx, r.Client, common.DBSyncParams{
		Name:       instance.Name,
		Namespace:  instance.Namespace,
		Image:      image,
		Command:    []string{"neutron-db-manage", "upgrade", "heads"},
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
	deploymentName := fmt.Sprintf("%s-server", instance.Name)
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
		ServicePort:      9696,
		GatewayName:      instance.Spec.GatewayRef.Name,
		GatewayNamespace: instance.Spec.GatewayRef.Namespace,
		ListenerName:     instance.Spec.GatewayRef.ListenerName,
	}, instance); err != nil {
		return ctrl.Result{}, err
	}

	// Ensure Keystone endpoint
	internalURL := fmt.Sprintf("http://%s.%s.svc:9696", deploymentName, instance.Namespace)
	publicURL := fmt.Sprintf("https://%s", instance.Spec.PublicHostname)
	keystoneURL, keystoneSecret := keystoneDependency(instance.Name, "-neutron", instance.Namespace)
	if err := common.EnsureKeystoneEndpoint(ctx, r.Client, common.EndpointParams{
		Name:           instance.Name,
		Namespace:      instance.Namespace,
		ServiceName:    "neutron",
		ServiceType:    "network",
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
	instance.Status.APIEndpoint = fmt.Sprintf("http://%s.%s.svc:9696", deploymentName, instance.Namespace)

	// Check Deployment readiness
	dep := &appsv1.Deployment{}
	if err := r.Get(ctx, types.NamespacedName{Name: deploymentName, Namespace: instance.Namespace}, dep); err != nil {
		return ctrl.Result{}, err
	}

	if dep.Status.ReadyReplicas > 0 && dep.Status.ReadyReplicas == dep.Status.Replicas {
		instance.Status.Conditions = common.SetCondition(
			instance.Status.Conditions, "Ready",
			metav1.ConditionTrue, "DeploymentReady", "Neutron is ready",
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

func (r *NeutronReconciler) ensureDeployment(ctx context.Context, instance *openstackv1alpha1.Neutron, name, image string) error {
	replicas := int32(1)
	if instance.Spec.Replicas != nil {
		replicas = *instance.Spec.Replicas
	}

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: instance.Namespace,
		},
	}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, dep, func() error {
		dep.Labels = labelsForNeutron(instance.Name)
		dep.Spec.Replicas = &replicas
		dep.Spec.Selector = &metav1.LabelSelector{
			MatchLabels: labelsForNeutron(instance.Name),
		}
		dep.Spec.Template = corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: labelsForNeutron(instance.Name),
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  "neutron-server",
						Image: image,
						Ports: []corev1.ContainerPort{
							{ContainerPort: 9696, Name: "api"},
						},
						Env: []corev1.EnvVar{
							{Name: "OVN_NB_DB_CONNECTION", Value: ovnNorthboundDBEndpoint(instance)},
							{Name: "OVN_SB_DB_CONNECTION", Value: ovnSouthboundDBEndpoint(instance)},
						},
					},
				},
			},
		}
		return controllerutil.SetOwnerReference(instance, dep, r.Scheme)
	})
	return err
}

func ovnNorthboundDBEndpoint(instance *openstackv1alpha1.Neutron) string {
	nbDB, _ := ovnDependency(instance.Name, "-neutron", instance.Namespace)
	return nbDB
}

func ovnSouthboundDBEndpoint(instance *openstackv1alpha1.Neutron) string {
	_, sbDB := ovnDependency(instance.Name, "-neutron", instance.Namespace)
	return sbDB
}

func (r *NeutronReconciler) ensureService(ctx context.Context, instance *openstackv1alpha1.Neutron, name string) error {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: instance.Namespace,
		},
	}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, svc, func() error {
		svc.Labels = labelsForNeutron(instance.Name)
		svc.Spec.Selector = labelsForNeutron(instance.Name)
		svc.Spec.Ports = []corev1.ServicePort{
			{
				Name:       "api",
				Port:       9696,
				TargetPort: intstr.FromInt32(9696),
				Protocol:   corev1.ProtocolTCP,
			},
		}
		return controllerutil.SetOwnerReference(instance, svc, r.Scheme)
	})
	return err
}

// SetupWithManager sets up the controller with the Manager.
func (r *NeutronReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&openstackv1alpha1.Neutron{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.Secret{}).
		Owns(&batchv1.Job{}).
		Complete(r)
}

func labelsForNeutron(name string) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       "neutron",
		"app.kubernetes.io/instance":   name,
		"app.kubernetes.io/managed-by": "openstack-operator",
	}
}
