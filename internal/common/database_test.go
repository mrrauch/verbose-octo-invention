package common

import (
	"context"
	"strings"
	"testing"

	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestEnsureDatabase_CreatesJob(t *testing.T) {
	scheme := SetupScheme()
	client := fake.NewClientBuilder().WithScheme(scheme).Build()

	params := DatabaseParams{
		Name:           "keystone",
		Namespace:      "openstack",
		DatabaseName:   "keystone",
		Username:       "keystone",
		SecretName:     "keystone-db-password",
		DatabaseSecret: "database-root-password",
		DatabaseHost:   "database.openstack.svc",
	}

	err := EnsureDatabase(context.Background(), client, params, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	job := &batchv1.Job{}
	err = client.Get(context.Background(), types.NamespacedName{
		Name:      "keystone-db-create",
		Namespace: "openstack",
	}, job)
	if err != nil {
		t.Fatalf("expected db-create Job to be created: %v", err)
	}
}

func TestEnsureDatabase_CreatesMySQLJob(t *testing.T) {
	scheme := SetupScheme()
	client := fake.NewClientBuilder().WithScheme(scheme).Build()

	params := DatabaseParams{
		Name:           "placement",
		Namespace:      "openstack",
		Engine:         "mysql",
		DatabaseName:   "placement",
		Username:       "placement",
		SecretName:     "placement-db-password",
		DatabaseSecret: "database-root-password",
		DatabaseHost:   "database.openstack.svc",
	}

	err := EnsureDatabase(context.Background(), client, params, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	job := &batchv1.Job{}
	err = client.Get(context.Background(), types.NamespacedName{
		Name:      "placement-db-create",
		Namespace: "openstack",
	}, job)
	if err != nil {
		t.Fatalf("expected db-create Job to be created: %v", err)
	}

	container := job.Spec.Template.Spec.Containers[0]
	if container.Image != "mysql:8.4" {
		t.Fatalf("expected mysql image mysql:8.4, got %s", container.Image)
	}
	if len(container.Command) < 3 || !strings.Contains(container.Command[2], "mysql -h database.openstack.svc") {
		t.Fatalf("expected mysql command in script, got %v", container.Command)
	}
}

func TestEnsureDBSync_CreatesJob(t *testing.T) {
	scheme := SetupScheme()
	client := fake.NewClientBuilder().WithScheme(scheme).Build()

	params := DBSyncParams{
		Name:       "keystone",
		Namespace:  "openstack",
		Image:      "quay.io/openstack.kolla/keystone:2025.1",
		Command:    []string{"keystone-manage", "db_sync"},
		SecretName: "keystone-db-password",
	}

	err := EnsureDBSync(context.Background(), client, params, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	job := &batchv1.Job{}
	err = client.Get(context.Background(), types.NamespacedName{
		Name:      "keystone-db-sync",
		Namespace: "openstack",
	}, job)
	if err != nil {
		t.Fatalf("expected db-sync Job to be created: %v", err)
	}
	if len(job.Spec.Template.Spec.Containers) != 1 {
		t.Fatalf("expected one container, got %d", len(job.Spec.Template.Spec.Containers))
	}
	var found bool
	for _, env := range job.Spec.Template.Spec.Containers[0].Env {
		if env.Name == "DB_PASSWORD" && env.ValueFrom != nil && env.ValueFrom.SecretKeyRef != nil {
			if env.ValueFrom.SecretKeyRef.Name == "keystone-db-password" && env.ValueFrom.SecretKeyRef.Key == "password" {
				found = true
				break
			}
		}
	}
	if !found {
		t.Fatal("expected DB_PASSWORD env var sourced from keystone-db-password secret")
	}
}
