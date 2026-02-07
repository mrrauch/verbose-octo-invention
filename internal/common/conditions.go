package common

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SetCondition sets or updates a condition in the given slice.
// Returns the updated slice.
func SetCondition(conditions []metav1.Condition, condType string, status metav1.ConditionStatus, reason, message string, observedGeneration int64) []metav1.Condition {
	now := metav1.NewTime(time.Now())
	for i, c := range conditions {
		if c.Type == condType {
			if c.Status != status {
				conditions[i].LastTransitionTime = now
			}
			conditions[i].Status = status
			conditions[i].Reason = reason
			conditions[i].Message = message
			conditions[i].ObservedGeneration = observedGeneration
			return conditions
		}
	}
	return append(conditions, metav1.Condition{
		Type:               condType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: observedGeneration,
		LastTransitionTime: now,
	})
}

// IsReady returns true if the "Ready" condition is True.
func IsReady(conditions []metav1.Condition) bool {
	for _, c := range conditions {
		if c.Type == "Ready" {
			return c.Status == metav1.ConditionTrue
		}
	}
	return false
}
