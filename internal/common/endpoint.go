package common

import (
	"context"
	"fmt"
	"strings"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// EndpointParams holds parameters for creating a Keystone service + endpoints.
type EndpointParams struct {
	Name           string
	Namespace      string
	ServiceName    string
	ServiceType    string
	InternalURL    string
	PublicURL      string
	AdminURL       string
	Region         string
	KeystoneSecret string
	KeystoneURL    string
	BootstrapImage string
}

// EnsureKeystoneEndpoint creates a Job that registers the service and its endpoints in Keystone.
func EnsureKeystoneEndpoint(ctx context.Context, c client.Client, params EndpointParams, owner metav1.Object) error {
	jobName := fmt.Sprintf("%s-endpoint-create", params.Name)

	existing := &batchv1.Job{}
	err := c.Get(ctx, types.NamespacedName{Name: jobName, Namespace: params.Namespace}, existing)
	if err == nil {
		return nil
	}
	if !errors.IsNotFound(err) {
		return err
	}

	script := strings.Join([]string{
		fmt.Sprintf(`openstack service show %s >/dev/null 2>&1 || openstack service create --name %s --description "%s service" %s`, params.ServiceName, params.ServiceName, params.ServiceName, params.ServiceType),
		fmt.Sprintf(`openstack endpoint list --service %s --interface internal --region %s -f value -c ID | grep -q . || openstack endpoint create --region %s %s internal %s`, params.ServiceType, params.Region, params.Region, params.ServiceType, params.InternalURL),
		fmt.Sprintf(`openstack endpoint list --service %s --interface public --region %s -f value -c ID | grep -q . || openstack endpoint create --region %s %s public %s`, params.ServiceType, params.Region, params.Region, params.ServiceType, params.PublicURL),
		fmt.Sprintf(`openstack endpoint list --service %s --interface admin --region %s -f value -c ID | grep -q . || openstack endpoint create --region %s %s admin %s`, params.ServiceType, params.Region, params.Region, params.ServiceType, params.AdminURL),
	}, " && ")

	backoffLimit := int32(6)
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: params.Namespace,
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: &backoffLimit,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyOnFailure,
					Containers: []corev1.Container{
						{
							Name:    "endpoint-create",
							Image:   params.BootstrapImage,
							Command: []string{"sh", "-c", script},
							Env: []corev1.EnvVar{
								{Name: "OS_AUTH_URL", Value: params.KeystoneURL},
								{Name: "OS_USERNAME", Value: "admin"},
								{Name: "OS_PROJECT_NAME", Value: "admin"},
								{Name: "OS_USER_DOMAIN_NAME", Value: "Default"},
								{Name: "OS_PROJECT_DOMAIN_NAME", Value: "Default"},
								{Name: "OS_IDENTITY_API_VERSION", Value: "3"},
								{
									Name: "OS_PASSWORD",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{Name: params.KeystoneSecret},
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

	if owner != nil {
		_ = controllerutil.SetOwnerReference(owner, job, c.Scheme())
	}
	return c.Create(ctx, job)
}
