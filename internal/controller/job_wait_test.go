package controller

import (
	"context"
	"testing"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/mrrauch/openstack-operator/internal/common"
)

func TestWaitForJobCompletion_DeletesFailedJob(t *testing.T) {
	scheme := common.SetupScheme()
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "glance-db-sync",
			Namespace: "openstack",
		},
		Status: batchv1.JobStatus{
			Conditions: []batchv1.JobCondition{
				{
					Type:   batchv1.JobFailed,
					Status: corev1.ConditionTrue,
				},
			},
		},
	}

	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(job).Build()

	done, result, err := waitForJobCompletion(context.Background(), client, "glance-db-sync", "openstack", 5*time.Second, 2*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if done {
		t.Fatal("expected done=false for failed job")
	}
	if result.RequeueAfter != 2*time.Second {
		t.Fatalf("expected failed requeue of 2s, got %s", result.RequeueAfter)
	}

	remaining := &batchv1.Job{}
	if err := client.Get(context.Background(), types.NamespacedName{Name: "glance-db-sync", Namespace: "openstack"}, remaining); err == nil {
		t.Fatal("expected failed job to be deleted for recreation")
	}
}

func TestWaitForJobCompletion_CompleteJob(t *testing.T) {
	scheme := common.SetupScheme()
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "glance-db-sync",
			Namespace: "openstack",
		},
		Status: batchv1.JobStatus{
			Conditions: []batchv1.JobCondition{
				{
					Type:   batchv1.JobComplete,
					Status: corev1.ConditionTrue,
				},
			},
		},
	}

	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(job).Build()

	done, result, err := waitForJobCompletion(context.Background(), client, "glance-db-sync", "openstack", 5*time.Second, 2*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !done {
		t.Fatal("expected done=true for complete job")
	}
	if result.RequeueAfter != 0 {
		t.Fatalf("expected no requeue for complete job, got %s", result.RequeueAfter)
	}
}
