package common

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestHasFinalizer(t *testing.T) {
	obj := &unstructured.Unstructured{}
	obj.SetFinalizers([]string{"openstack.k8s.io/cleanup"})
	if !HasFinalizer(obj, "openstack.k8s.io/cleanup") {
		t.Error("expected finalizer to be present")
	}
	if HasFinalizer(obj, "other") {
		t.Error("expected finalizer to be absent")
	}
}

func TestAddFinalizer(t *testing.T) {
	obj := &unstructured.Unstructured{}
	AddFinalizer(obj, "openstack.k8s.io/cleanup")
	if len(obj.GetFinalizers()) != 1 {
		t.Fatalf("expected 1 finalizer, got %d", len(obj.GetFinalizers()))
	}
	// Adding again should not duplicate
	AddFinalizer(obj, "openstack.k8s.io/cleanup")
	if len(obj.GetFinalizers()) != 1 {
		t.Fatalf("expected 1 finalizer after duplicate add, got %d", len(obj.GetFinalizers()))
	}
}

func TestRemoveFinalizer(t *testing.T) {
	obj := &unstructured.Unstructured{}
	obj.SetFinalizers([]string{"openstack.k8s.io/cleanup", "other"})
	RemoveFinalizer(obj, "openstack.k8s.io/cleanup")
	if len(obj.GetFinalizers()) != 1 {
		t.Fatalf("expected 1 finalizer, got %d", len(obj.GetFinalizers()))
	}
	if obj.GetFinalizers()[0] != "other" {
		t.Error("wrong finalizer removed")
	}
}
