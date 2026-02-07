package common

import (
	"context"
	"crypto/rand"
	"encoding/hex"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// GeneratePassword returns a random hex string of the given length.
func GeneratePassword(length int) (string, error) {
	b := make([]byte, (length+1)/2)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b)[:length], nil
}

// EnsureSecret creates a Secret with generated random values for each key if it doesn't exist.
// The keys map specifies key name -> desired password length.
// If owner is non-nil, an owner reference is set.
func EnsureSecret(ctx context.Context, c client.Client, name, namespace string, keys map[string]int, owner metav1.Object) error {
	existing := &corev1.Secret{}
	err := c.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, existing)
	if err == nil {
		return nil // already exists
	}
	if !errors.IsNotFound(err) {
		return err
	}

	data := make(map[string][]byte, len(keys))
	for k, length := range keys {
		password, genErr := GeneratePassword(length)
		if genErr != nil {
			return genErr
		}
		data[k] = []byte(password)
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: data,
	}

	if owner != nil {
		_ = controllerutil.SetOwnerReference(owner, secret, c.Scheme())
	}

	return c.Create(ctx, secret)
}
