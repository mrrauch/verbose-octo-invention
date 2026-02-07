package common

import (
	"context"
	"fmt"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// DatabaseParams holds parameters for creating a database and user.
type DatabaseParams struct {
	Name           string
	Namespace      string
	DatabaseName   string
	Username       string
	SecretName     string
	DatabaseSecret string
	DatabaseHost   string
}

// EnsureDatabase creates a Job that provisions a database and user in PostgreSQL.
func EnsureDatabase(ctx context.Context, c client.Client, params DatabaseParams, owner metav1.Object) error {
	jobName := fmt.Sprintf("%s-db-create", params.Name)

	existing := &batchv1.Job{}
	err := c.Get(ctx, types.NamespacedName{Name: jobName, Namespace: params.Namespace}, existing)
	if err == nil {
		return nil
	}
	if !errors.IsNotFound(err) {
		return err
	}

	script := fmt.Sprintf(
		`PGPASSWORD="$ROOT_PASSWORD" psql -h %s -U postgres -tc "SELECT 1 FROM pg_database WHERE datname='%s'" | grep -q 1 || PGPASSWORD="$ROOT_PASSWORD" psql -h %s -U postgres -c "CREATE DATABASE %s"; `+
			`PGPASSWORD="$ROOT_PASSWORD" psql -h %s -U postgres -c "DO \$\$BEGIN IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname='%s') THEN CREATE ROLE %s LOGIN PASSWORD '$SERVICE_PASSWORD'; END IF; END\$\$; GRANT ALL PRIVILEGES ON DATABASE %s TO %s;"`,
		params.DatabaseHost, params.DatabaseName,
		params.DatabaseHost, params.DatabaseName,
		params.DatabaseHost, params.Username, params.Username, params.DatabaseName, params.Username,
	)

	backoffLimit := int32(4)
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
							Name:    "db-create",
							Image:   "postgres:17",
							Command: []string{"sh", "-c", script},
							Env: []corev1.EnvVar{
								{
									Name: "ROOT_PASSWORD",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{Name: params.DatabaseSecret},
											Key:                  "password",
										},
									},
								},
								{
									Name: "SERVICE_PASSWORD",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{Name: params.SecretName},
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

// DBSyncParams holds parameters for running a database migration.
type DBSyncParams struct {
	Name       string
	Namespace  string
	Image      string
	Command    []string
	SecretName string
}

// EnsureDBSync creates a Job that runs the service's db_sync command.
func EnsureDBSync(ctx context.Context, c client.Client, params DBSyncParams, owner metav1.Object) error {
	jobName := fmt.Sprintf("%s-db-sync", params.Name)

	existing := &batchv1.Job{}
	err := c.Get(ctx, types.NamespacedName{Name: jobName, Namespace: params.Namespace}, existing)
	if err == nil {
		return nil
	}
	if !errors.IsNotFound(err) {
		return err
	}

	backoffLimit := int32(4)
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
							Name:    "db-sync",
							Image:   params.Image,
							Command: params.Command,
							Env: []corev1.EnvVar{
								{
									Name: "DB_PASSWORD",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{Name: params.SecretName},
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

// IsJobComplete returns true if the Job has a Complete condition.
func IsJobComplete(ctx context.Context, c client.Client, name, namespace string) (bool, error) {
	job := &batchv1.Job{}
	if err := c.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, job); err != nil {
		if errors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	for _, cond := range job.Status.Conditions {
		if cond.Type == batchv1.JobComplete && cond.Status == corev1.ConditionTrue {
			return true, nil
		}
	}
	return false, nil
}

// IsJobFailed returns true if the Job has a Failed condition.
func IsJobFailed(ctx context.Context, c client.Client, name, namespace string) (bool, error) {
	job := &batchv1.Job{}
	if err := c.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, job); err != nil {
		if errors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	for _, cond := range job.Status.Conditions {
		if cond.Type == batchv1.JobFailed && cond.Status == corev1.ConditionTrue {
			return true, nil
		}
	}
	return false, nil
}
