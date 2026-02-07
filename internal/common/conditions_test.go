package common

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSetCondition_AddsNew(t *testing.T) {
	var conditions []metav1.Condition
	conditions = SetCondition(conditions, "Ready", metav1.ConditionTrue, "AllGood", "everything is fine", 1)
	if len(conditions) != 1 {
		t.Fatalf("expected 1 condition, got %d", len(conditions))
	}
	if conditions[0].Type != "Ready" {
		t.Errorf("expected type Ready, got %s", conditions[0].Type)
	}
	if conditions[0].Status != metav1.ConditionTrue {
		t.Errorf("expected status True, got %s", conditions[0].Status)
	}
	if conditions[0].ObservedGeneration != 1 {
		t.Errorf("expected observedGeneration 1, got %d", conditions[0].ObservedGeneration)
	}
}

func TestSetCondition_UpdatesExisting(t *testing.T) {
	conditions := []metav1.Condition{
		{Type: "Ready", Status: metav1.ConditionFalse, Reason: "Waiting", Message: "not yet"},
	}
	conditions = SetCondition(conditions, "Ready", metav1.ConditionTrue, "AllGood", "done", 2)
	if len(conditions) != 1 {
		t.Fatalf("expected 1 condition, got %d", len(conditions))
	}
	if conditions[0].Status != metav1.ConditionTrue {
		t.Errorf("expected status True, got %s", conditions[0].Status)
	}
	if conditions[0].Reason != "AllGood" {
		t.Errorf("expected reason AllGood, got %s", conditions[0].Reason)
	}
	if conditions[0].ObservedGeneration != 2 {
		t.Errorf("expected observedGeneration 2, got %d", conditions[0].ObservedGeneration)
	}
}

func TestIsReady(t *testing.T) {
	conditions := []metav1.Condition{
		{Type: "Ready", Status: metav1.ConditionTrue, Reason: "AllGood"},
	}
	if !IsReady(conditions) {
		t.Error("expected IsReady to return true")
	}
	if IsReady(nil) {
		t.Error("expected IsReady to return false for nil conditions")
	}
}
