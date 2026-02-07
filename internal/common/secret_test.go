package common

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestEnsureSecret_CreatesWhenMissing(t *testing.T) {
	scheme := SetupScheme()
	client := fake.NewClientBuilder().WithScheme(scheme).Build()

	name := "test-secret"
	namespace := "default"
	keys := map[string]int{"password": 16}

	err := EnsureSecret(context.Background(), client, name, namespace, keys, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	secret := &corev1.Secret{}
	err = client.Get(context.Background(), types.NamespacedName{Name: name, Namespace: namespace}, secret)
	if err != nil {
		t.Fatalf("secret not found: %v", err)
	}
	if len(secret.Data["password"]) != 16 {
		t.Errorf("expected password length 16, got %d", len(secret.Data["password"]))
	}
}

func TestEnsureSecret_NoOpWhenExists(t *testing.T) {
	scheme := SetupScheme()
	existing := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "test-secret", Namespace: "default"},
		Data:       map[string][]byte{"password": []byte("existingvalue123")},
	}
	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(existing).Build()

	err := EnsureSecret(context.Background(), client, "test-secret", "default", map[string]int{"password": 16}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	secret := &corev1.Secret{}
	_ = client.Get(context.Background(), types.NamespacedName{Name: "test-secret", Namespace: "default"}, secret)
	if string(secret.Data["password"]) != "existingvalue123" {
		t.Error("expected existing secret data to be preserved")
	}
}

func TestGeneratePassword(t *testing.T) {
	p1, err := GeneratePassword(32)
	if err != nil {
		t.Fatalf("unexpected error generating password: %v", err)
	}
	p2, err := GeneratePassword(32)
	if err != nil {
		t.Fatalf("unexpected error generating password: %v", err)
	}
	if len(p1) != 32 {
		t.Errorf("expected length 32, got %d", len(p1))
	}
	if p1 == p2 {
		t.Error("expected different passwords")
	}
}
