package controller

import (
	"context"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/mrrauch/openstack-operator/internal/common"
)

func waitForJobCompletion(ctx context.Context, c client.Client, name, namespace string, pendingDelay, failedDelay time.Duration) (done bool, result ctrl.Result, err error) {
	complete, err := common.IsJobComplete(ctx, c, name, namespace)
	if err != nil {
		return false, ctrl.Result{}, err
	}
	if complete {
		return true, ctrl.Result{}, nil
	}

	failed, err := common.IsJobFailed(ctx, c, name, namespace)
	if err != nil {
		return false, ctrl.Result{}, err
	}
	if failed {
		job := &batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
		}
		if err := c.Delete(ctx, job); err != nil && !apierrors.IsNotFound(err) {
			return false, ctrl.Result{}, err
		}
		return false, ctrl.Result{RequeueAfter: failedDelay}, nil
	}

	return false, ctrl.Result{RequeueAfter: pendingDelay}, nil
}
