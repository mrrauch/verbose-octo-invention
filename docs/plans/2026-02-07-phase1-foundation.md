# Phase 1 Foundation Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement the complete Phase 1 MVP — all infrastructure controllers (MariaDB, RabbitMQ, Memcached), networking (OVN), identity (Keystone), and core services (Glance, Placement, Neutron, Nova), plus the top-level OpenStackControlPlane orchestrator. Result: a working operator that can reconcile a minimal OpenStack cloud on Kubernetes.

**Architecture:** Each service gets its own controller following the standard Kubebuilder reconciler pattern (fetch CR, handle finalizer, ensure resources, update status). Shared logic lives in `internal/common/` (condition helpers, secret generation, database provisioning, config rendering). The top-level `OpenStackControlPlane` controller creates child CRs in dependency order and waits for each phase to become Ready before advancing.

**Tech Stack:** Go 1.22, Kubebuilder v4, controller-runtime v0.18.4, envtest for controller tests, Kolla container images, Kubernetes Gateway API v1.4.x

---

## Cluster Prerequisites

The base Kubernetes cluster **must** have the following installed before deploying the operator. The operator does NOT install these — they are infrastructure-level concerns owned by the cluster admin.

### Required

| Component | Purpose | Minimum Version | Notes |
|-----------|---------|-----------------|-------|
| **Kubernetes** | Control plane | v1.28+ | Required for Gateway API v1.4 and controller-runtime v0.18 |
| **StorageClass (RWO)** | PVCs for MariaDB, RabbitMQ, OVN databases, Glance images | Any CSI driver | Must be set as default or explicitly named in CR specs. Any CSI-backed StorageClass that supports `ReadWriteOnce` works (e.g., `local-path`, `longhorn`, `ceph-rbd`, `ebs-csi`, `pd-csi`). |
| **Gateway API CRDs** | Service exposure via HTTPRoute | v1.4.x | Install via `kubectl apply -f https://github.com/kubernetes-sigs/gateway-api/releases/download/v1.4.1/standard-install.yaml` |
| **Gateway API implementation** | Data plane for HTTPRoute | Any conformant | See "Supported Implementations" below |
| **Gateway resource** | Listener for OpenStack API traffic | — | A `Gateway` resource in the `openstack` namespace by default. Cross-namespace Gateway attachment is supported via `spec.gatewayRef.namespace` + Gateway listener `allowedRoutes` policy. |
| **cert-manager** (if TLS enabled) | Automated certificate provisioning | v1.14+ | Required when `spec.tls.enabled=true`. Operator references an `Issuer` or `ClusterIssuer` via `spec.tls.issuerRef`. |

### Storage Details

**What the operator creates:**
- MariaDB: 1x PVC (default 10Gi RWO) via StatefulSet VolumeClaimTemplate
- RabbitMQ: 1x PVC (default 10Gi RWO) via StatefulSet VolumeClaimTemplate
- OVN NB DB: 1x PVC (default 1Gi RWO)
- OVN SB DB: 1x PVC (default 1Gi RWO)
- Glance (PVC backend): 1x PVC (default 10Gi RWO) — only when `storageType=pvc`

**StorageClass requirements:**
- All Phase 1 PVCs use `ReadWriteOnce` access mode — no shared storage needed
- If no `storageClassName` is specified in the CR, the cluster's **default StorageClass** is used
- For production: use a CSI driver with volume expansion support (`allowVolumeExpansion: true`)
- For dev/test: `local-path-provisioner` or `hostpath` is sufficient

**Validation command:**
```bash
# Verify a default StorageClass exists
kubectl get storageclass -o jsonpath='{range .items[?(@.metadata.annotations.storageclass\.kubernetes\.io/is-default-class=="true")]}{.metadata.name}{"\n"}{end}'
```

### Networking Details

**What the cluster needs:**
- Standard Kubernetes networking (pod-to-pod, pod-to-service DNS)
- A Gateway API implementation for external API access
- No special CNI requirements for Phase 1 (OVN for OpenStack VMs runs as its own overlay, independent of the cluster CNI)

**Gateway API — supported implementations** (any conformant implementation works):

| Implementation | Notes |
|----------------|-------|
| **Envoy Gateway** | CNCF project, first-class Gateway API support, recommended for new clusters |
| **Istio** | Full mesh + gateway, ambient mode eliminates sidecars |
| **Cilium** | eBPF-native, best performance if already your CNI |
| **NGINX Gateway Fabric** | Successor to nginx-ingress (EOL March 2026), production-ready |
| **Contour** | Envoy-based, lightweight |
| **Traefik v3** | Multi-protocol, proven stability |
| **HAProxy Kubernetes Gateway** | High-performance, low-latency |

**Operator creates these Gateway API resources:**
- One `HTTPRoute` per public OpenStack API (Keystone, Glance, Nova, Neutron, Placement)
- Routes attach to a pre-existing `Gateway` named in `spec.gatewayRef` (or default `openstack-gateway` in the same namespace)
- TLS termination is handled by the Gateway listener (via cert-manager), not by the operator

**Required API fields for Gateway routing:**
- `OpenStackControlPlane.spec.gatewayRef` (name/namespace/listenerName) for default Gateway attachment
- `OpenStackControlPlane.spec.publicDomain` for default host generation
- Per-service override fields on Keystone/Glance/Placement/Neutron/Nova specs:
  - `publicHostname`
  - `gatewayRef` (optional override)

**Minimal Gateway setup example:**
```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: openstack-gateway
  namespace: openstack
spec:
  gatewayClassName: eg  # or istio, cilium, nginx, etc.
  listeners:
    - name: https
      protocol: HTTPS
      port: 443
      tls:
        mode: Terminate
        certificateRefs:
          - name: openstack-tls
      allowedRoutes:
        namespaces:
          from: Same
    - name: http
      protocol: HTTP
      port: 80
      allowedRoutes:
        namespaces:
          from: Same
```

### Quick-Start Checklist

```bash
# 1. Verify Kubernetes version
kubectl version --short

# 2. Verify default StorageClass
kubectl get sc

# 3. Install Gateway API CRDs (if not present)
kubectl apply -f https://github.com/kubernetes-sigs/gateway-api/releases/download/v1.4.1/standard-install.yaml

# 4. Install a Gateway API implementation (example: Envoy Gateway)
helm install eg oci://docker.io/envoyproxy/gateway-helm --version v1.3.0 -n envoy-gateway-system --create-namespace

# 5. Create the openstack namespace + Gateway
kubectl create namespace openstack
kubectl apply -f config/samples/gateway.yaml

# 6. (Optional) Install cert-manager for TLS
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.16.0/cert-manager.yaml
```

---

## Dependency Order

Tasks MUST be implemented in this order because later tasks depend on earlier ones:

```
Task 1:  API exposure fields + Gateway API dependency + code generation
Task 2:  internal/common/conditions.go — used by ALL controllers
Task 3:  internal/common/secret.go — used by ALL controllers
Task 4:  internal/common/finalizer.go — used by ALL controllers
Task 5:  internal/images/defaults.go — used by ALL controllers
Task 6:  MariaDB controller — first controller, proves the pattern
Task 7:  RabbitMQ controller — follows MariaDB pattern
Task 8:  Memcached controller — simplest infra controller
Task 9:  internal/common/database.go — DB provisioning helpers (used by Keystone+)
Task 10: internal/common/endpoint.go — Keystone endpoint helpers (used by Glance+)
Task 11: internal/common/httproute.go — Gateway API HTTPRoute helpers (used by Keystone+)
Task 12: config/samples/gateway.yaml — sample Gateway resource for cluster setup
Task 13: Keystone controller — first service controller, needs DB + bootstrap + HTTPRoute
Task 14: Glance controller — needs DB + Keystone endpoint + HTTPRoute
Task 15: Placement controller — needs DB + Keystone endpoint + HTTPRoute
Task 16: OVN Network controller — networking infra (no HTTPRoute, internal only)
Task 17: Neutron controller — needs DB + Keystone + OVN + HTTPRoute
Task 18: Nova controller — most complex: api, scheduler, conductor, compute + HTTPRoute
Task 19: OpenStackControlPlane controller — orchestrates all of the above
Task 20: Wire all controllers in main.go
Task 21: Build + E2E acceptance validation
```

---

### Task 1: Add API exposure fields, Gateway dependency, and generate manifests

**Files:**
- Modify: `api/v1alpha1/common_types.go`
- Modify: `api/v1alpha1/openstack_controlplane_types.go`
- Modify: `api/v1alpha1/keystone_types.go`
- Modify: `api/v1alpha1/glance_types.go`
- Modify: `api/v1alpha1/placement_types.go`
- Modify: `api/v1alpha1/neutron_types.go`
- Modify: `api/v1alpha1/nova_types.go`
- Generated: `api/v1alpha1/zz_generated.deepcopy.go`
- Generated: `config/crd/bases/*.yaml`

**Step 1: Add Gateway API dependency and install dependencies**

```bash
cd /home/mrrauch/git/go/src/verbose-octo-invention
go get sigs.k8s.io/gateway-api@v1.4.1
go mod tidy
```

**Step 2: Add Gateway exposure fields to the API types**

Add shared Gateway reference type:

```go
// api/v1alpha1/common_types.go
type GatewayRef struct {
	// Name of the Gateway resource.
	// +optional
	Name string `json:"name,omitempty"`

	// Namespace of the Gateway resource. If empty, service namespace is used.
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// Optional listener name to bind routes to a specific Gateway listener.
	// +optional
	ListenerName string `json:"listenerName,omitempty"`
}
```

Add control-plane defaults:

```go
// api/v1alpha1/openstack_controlplane_types.go
// GatewayRef defines the default Gateway used for public API routes.
// +optional
GatewayRef GatewayRef `json:"gatewayRef,omitempty"`

// PublicDomain is used to generate default hostnames:
// keystone.<publicDomain>, glance.<publicDomain>, etc.
// +kubebuilder:default="openstack.local"
// +optional
PublicDomain string `json:"publicDomain,omitempty"`
```

Add per-service route fields to Keystone/Glance/Placement/Neutron/Nova specs:

```go
// PublicHostname overrides the generated external hostname for this API service.
// +optional
PublicHostname string `json:"publicHostname,omitempty"`

// GatewayRef overrides the control-plane default gatewayRef for this service.
// +optional
GatewayRef GatewayRef `json:"gatewayRef,omitempty"`
```

**Step 3: Run code generation**

```bash
make generate
```

Expected: `zz_generated.deepcopy.go` created, CRD YAML files generated in `config/crd/bases/`.

**Step 4: Verify the build compiles**

```bash
go build ./...
```

Expected: Clean build, no errors.

**Step 5: Commit**

```bash
git add -A
git commit -m "feat: add Gateway API exposure fields and regenerate CRDs"
```

---

### Task 2: Condition helpers — `internal/common/conditions.go`

**Files:**
- Create: `internal/common/conditions.go`
- Test: `internal/common/conditions_test.go`

**Step 1: Write the failing test**

```go
// internal/common/conditions_test.go
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
```

**Step 2: Run test to verify it fails**

```bash
go test ./internal/common/ -v -run TestSetCondition
```

Expected: FAIL — package doesn't exist yet.

**Step 3: Write minimal implementation**

```go
// internal/common/conditions.go
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
```

**Step 4: Run test to verify it passes**

```bash
go test ./internal/common/ -v -run TestSetCondition && go test ./internal/common/ -v -run TestIsReady
```

Expected: PASS

**Step 5: Commit**

```bash
git add internal/common/conditions.go internal/common/conditions_test.go
git commit -m "feat: add condition helpers for status management"
```

---

### Task 3: Secret generation — `internal/common/secret.go`

**Files:**
- Create: `internal/common/secret.go`
- Test: `internal/common/secret_test.go`

**Step 1: Write the failing test**

```go
// internal/common/secret_test.go
package common

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	openstackv1alpha1 "github.com/mrrauch/openstack-operator/api/v1alpha1"
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
	p1 := GeneratePassword(32)
	p2 := GeneratePassword(32)
	if len(p1) != 32 {
		t.Errorf("expected length 32, got %d", len(p1))
	}
	if p1 == p2 {
		t.Error("expected different passwords")
	}
}
```

Also create the scheme helper (needed by secret and later tests):

```go
// internal/common/scheme.go
package common

import (
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	openstackv1alpha1 "github.com/mrrauch/openstack-operator/api/v1alpha1"
)

// SetupScheme returns a runtime.Scheme with core K8s, Gateway API, and OpenStack types registered.
func SetupScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(s))
	utilruntime.Must(gatewayv1.Install(s))
	utilruntime.Must(openstackv1alpha1.AddToScheme(s))
	return s
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./internal/common/ -v -run TestEnsureSecret
```

Expected: FAIL — `EnsureSecret` not defined.

**Step 3: Write minimal implementation**

```go
// internal/common/secret.go
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
func GeneratePassword(length int) string {
	b := make([]byte, (length+1)/2)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)[:length]
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
		data[k] = []byte(GeneratePassword(length))
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
```

**Step 4: Run test to verify it passes**

```bash
go test ./internal/common/ -v -run "TestEnsureSecret|TestGeneratePassword"
```

Expected: PASS

**Step 5: Commit**

```bash
git add internal/common/secret.go internal/common/secret_test.go internal/common/scheme.go
git commit -m "feat: add secret generation helpers"
```

---

### Task 4: Finalizer helpers — `internal/common/finalizer.go`

**Files:**
- Create: `internal/common/finalizer.go`
- Test: `internal/common/finalizer_test.go`

**Step 1: Write the failing test**

```go
// internal/common/finalizer_test.go
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
```

**Step 2: Run test to verify it fails**

```bash
go test ./internal/common/ -v -run TestHasFinalizer
```

Expected: FAIL

**Step 3: Write minimal implementation**

```go
// internal/common/finalizer.go
package common

const FinalizerName = "openstack.k8s.io/cleanup"

// objectWithFinalizers is any object that has Get/SetFinalizers.
type objectWithFinalizers interface {
	GetFinalizers() []string
	SetFinalizers([]string)
}

// HasFinalizer returns true if the object has the given finalizer.
func HasFinalizer(obj objectWithFinalizers, finalizer string) bool {
	for _, f := range obj.GetFinalizers() {
		if f == finalizer {
			return true
		}
	}
	return false
}

// AddFinalizer adds the finalizer if not already present.
func AddFinalizer(obj objectWithFinalizers, finalizer string) {
	if !HasFinalizer(obj, finalizer) {
		obj.SetFinalizers(append(obj.GetFinalizers(), finalizer))
	}
}

// RemoveFinalizer removes the finalizer if present.
func RemoveFinalizer(obj objectWithFinalizers, finalizer string) {
	var result []string
	for _, f := range obj.GetFinalizers() {
		if f != finalizer {
			result = append(result, f)
		}
	}
	obj.SetFinalizers(result)
}
```

**Step 4: Run test to verify it passes**

```bash
go test ./internal/common/ -v -run "TestHasFinalizer|TestAddFinalizer|TestRemoveFinalizer"
```

Expected: PASS

**Step 5: Commit**

```bash
git add internal/common/finalizer.go internal/common/finalizer_test.go
git commit -m "feat: add finalizer helpers"
```

---

### Task 5: Default images — `internal/images/defaults.go`

**Files:**
- Create: `internal/images/defaults.go`

**Step 1: Write the implementation**

```go
// internal/images/defaults.go
package images

// Default container images for OpenStack services.
// All images are from the Kolla project for the 2025.1 (Epoxy) release.
const (
	DefaultMariaDB    = "quay.io/openstack.kolla/mariadb-server:2025.1"
	DefaultRabbitMQ   = "quay.io/openstack.kolla/rabbitmq:2025.1"
	DefaultMemcached  = "quay.io/openstack.kolla/memcached:2025.1"
	DefaultKeystone   = "quay.io/openstack.kolla/keystone:2025.1"
	DefaultGlanceAPI  = "quay.io/openstack.kolla/glance-api:2025.1"
	DefaultPlacement  = "quay.io/openstack.kolla/placement-api:2025.1"
	DefaultNeutronServer  = "quay.io/openstack.kolla/neutron-server:2025.1"
	DefaultNovaAPI        = "quay.io/openstack.kolla/nova-api:2025.1"
	DefaultNovaScheduler  = "quay.io/openstack.kolla/nova-scheduler:2025.1"
	DefaultNovaConductor  = "quay.io/openstack.kolla/nova-conductor:2025.1"
	DefaultNovaCompute    = "quay.io/openstack.kolla/nova-compute:2025.1"
	DefaultOVNNorthd      = "quay.io/openstack.kolla/ovn-northd:2025.1"
	DefaultOVNNBDB        = "quay.io/openstack.kolla/ovn-nb-db-server:2025.1"
	DefaultOVNSBDB        = "quay.io/openstack.kolla/ovn-sb-db-server:2025.1"
)

// ImageOrDefault returns the image if non-empty, otherwise the defaultImage.
func ImageOrDefault(image, defaultImage string) string {
	if image != "" {
		return image
	}
	return defaultImage
}
```

**Step 2: Verify it compiles**

```bash
go build ./internal/images/
```

Expected: Clean build.

**Step 3: Commit**

```bash
git add internal/images/defaults.go
git commit -m "feat: add default container image constants"
```

---

### Task 6: MariaDB controller

This is the first real controller. It proves the pattern that all others follow.

**Files:**
- Create: `internal/controller/mariadb_controller.go`
- Test: `internal/controller/mariadb_controller_test.go`

**Step 1: Write the failing test**

```go
// internal/controller/mariadb_controller_test.go
package controller

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	openstackv1alpha1 "github.com/mrrauch/openstack-operator/api/v1alpha1"
	"github.com/mrrauch/openstack-operator/internal/common"
)

func TestMariaDBReconciler_CreatesStatefulSet(t *testing.T) {
	scheme := common.SetupScheme()
	mariadb := &openstackv1alpha1.MariaDB{
		ObjectMeta: metav1.ObjectMeta{Name: "mariadb", Namespace: "openstack"},
		Spec: openstackv1alpha1.MariaDBSpec{},
	}

	client := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(mariadb).
		WithStatusSubresource(mariadb).
		Build()

	r := &MariaDBReconciler{Client: client, Scheme: scheme}
	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "mariadb", Namespace: "openstack"},
	})
	if err != nil {
		t.Fatalf("reconcile error: %v", err)
	}

	// Verify StatefulSet was created
	sts := &appsv1.StatefulSet{}
	err = client.Get(context.Background(), types.NamespacedName{Name: "mariadb", Namespace: "openstack"}, sts)
	if err != nil {
		t.Fatalf("expected StatefulSet to be created: %v", err)
	}

	// Verify Service was created
	svc := &corev1.Service{}
	err = client.Get(context.Background(), types.NamespacedName{Name: "mariadb", Namespace: "openstack"}, svc)
	if err != nil {
		t.Fatalf("expected Service to be created: %v", err)
	}

	// Verify Secret was created
	secret := &corev1.Secret{}
	err = client.Get(context.Background(), types.NamespacedName{Name: "mariadb-root-password", Namespace: "openstack"}, secret)
	if err != nil {
		t.Fatalf("expected Secret to be created: %v", err)
	}
}

func TestMariaDBReconciler_NotFound(t *testing.T) {
	scheme := common.SetupScheme()
	client := fake.NewClientBuilder().WithScheme(scheme).Build()

	r := &MariaDBReconciler{Client: client, Scheme: scheme}
	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "missing", Namespace: "openstack"},
	})
	if err != nil {
		t.Fatalf("expected no error for missing CR, got: %v", err)
	}
	if result.Requeue {
		t.Error("expected no requeue for missing CR")
	}
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./internal/controller/ -v -run TestMariaDBReconciler
```

Expected: FAIL — `MariaDBReconciler` not defined.

**Step 3: Write minimal implementation**

```go
// internal/controller/mariadb_controller.go
package controller

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	openstackv1alpha1 "github.com/mrrauch/openstack-operator/api/v1alpha1"
	"github.com/mrrauch/openstack-operator/internal/common"
	"github.com/mrrauch/openstack-operator/internal/images"
)

type MariaDBReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func (r *MariaDBReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	instance := &openstackv1alpha1.MariaDB{}
	if err := r.Get(ctx, req.NamespacedName, instance); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Handle deletion
	if !instance.DeletionTimestamp.IsZero() {
		if common.HasFinalizer(instance, common.FinalizerName) {
			common.RemoveFinalizer(instance, common.FinalizerName)
			if err := r.Update(ctx, instance); err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	// Ensure finalizer
	if !common.HasFinalizer(instance, common.FinalizerName) {
		common.AddFinalizer(instance, common.FinalizerName)
		if err := r.Update(ctx, instance); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Set status to Progressing
	instance.Status.Conditions = common.SetCondition(
		instance.Status.Conditions, "Ready", metav1.ConditionFalse, "Reconciling", "Reconciliation in progress", instance.Generation,
	)

	// Ensure root password secret
	secretName := fmt.Sprintf("%s-root-password", instance.Name)
	if err := common.EnsureSecret(ctx, r.Client, secretName, instance.Namespace, map[string]int{"password": 32}, instance); err != nil {
		return ctrl.Result{}, err
	}

	// Ensure headless Service
	if err := r.ensureService(ctx, instance); err != nil {
		return ctrl.Result{}, err
	}

	// Ensure StatefulSet
	if err := r.ensureStatefulSet(ctx, instance, secretName); err != nil {
		return ctrl.Result{}, err
	}

	// Check if StatefulSet is ready
	sts := &appsv1.StatefulSet{}
	if err := r.Get(ctx, types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}, sts); err != nil {
		return ctrl.Result{}, err
	}

	if sts.Status.ReadyReplicas == sts.Status.Replicas && sts.Status.ReadyReplicas > 0 {
		instance.Status.Conditions = common.SetCondition(
			instance.Status.Conditions, "Ready", metav1.ConditionTrue, "StatefulSetReady", "MariaDB is ready", instance.Generation,
		)
	}

	instance.Status.ObservedGeneration = instance.Generation
	if err := r.Status().Update(ctx, instance); err != nil {
		logger.Error(err, "failed to update status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *MariaDBReconciler) ensureService(ctx context.Context, instance *openstackv1alpha1.MariaDB) error {
	svc := &corev1.Service{}
	err := r.Get(ctx, types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}, svc)
	if err == nil {
		return nil
	}
	if !errors.IsNotFound(err) {
		return err
	}

	svc = &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Name,
			Namespace: instance.Namespace,
			Labels:    labelsForMariaDB(instance.Name),
		},
		Spec: corev1.ServiceSpec{
			ClusterIP: "None",
			Selector:  labelsForMariaDB(instance.Name),
			Ports: []corev1.ServicePort{
				{Name: "mysql", Port: 3306, Protocol: corev1.ProtocolTCP},
			},
		},
	}
	_ = controllerutil.SetOwnerReference(instance, svc, r.Scheme)
	return r.Create(ctx, svc)
}

func (r *MariaDBReconciler) ensureStatefulSet(ctx context.Context, instance *openstackv1alpha1.MariaDB, secretName string) error {
	sts := &appsv1.StatefulSet{}
	err := r.Get(ctx, types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}, sts)
	if err == nil {
		return nil
	}
	if !errors.IsNotFound(err) {
		return err
	}

	replicas := int32(1)
	if instance.Spec.Replicas != nil {
		replicas = *instance.Spec.Replicas
	}

	image := images.ImageOrDefault(instance.Spec.Image, images.DefaultMariaDB)
	storageSize := instance.Spec.Storage.Size
	if storageSize.IsZero() {
		storageSize = resource.MustParse("10Gi")
	}

	sts = &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Name,
			Namespace: instance.Namespace,
			Labels:    labelsForMariaDB(instance.Name),
		},
		Spec: appsv1.StatefulSetSpec{
			ServiceName: instance.Name,
			Replicas:    &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labelsForMariaDB(instance.Name),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labelsForMariaDB(instance.Name),
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "mariadb",
							Image: image,
							Ports: []corev1.ContainerPort{
								{ContainerPort: 3306, Name: "mysql"},
							},
							Env: []corev1.EnvVar{
								{
									Name: "MYSQL_ROOT_PASSWORD",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
											Key:                  "password",
										},
									},
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "data", MountPath: "/var/lib/mysql"},
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									Exec: &corev1.ExecAction{
										Command: []string{"mysqladmin", "ping", "-h", "127.0.0.1"},
									},
								},
								InitialDelaySeconds: 10,
								PeriodSeconds:       5,
							},
						},
					},
				},
			},
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "data"},
					Spec: corev1.PersistentVolumeClaimSpec{
						AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
						Resources: corev1.VolumeResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: storageSize,
							},
						},
						StorageClassName: instance.Spec.Storage.StorageClassName,
					},
				},
			},
		},
	}
	_ = controllerutil.SetOwnerReference(instance, sts, r.Scheme)
	return r.Create(ctx, sts)
}

func (r *MariaDBReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&openstackv1alpha1.MariaDB{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.Secret{}).
		Complete(r)
}

func labelsForMariaDB(name string) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       "mariadb",
		"app.kubernetes.io/instance":   name,
		"app.kubernetes.io/managed-by": "openstack-operator",
	}
}
```

**Step 4: Run test to verify it passes**

```bash
go test ./internal/controller/ -v -run TestMariaDBReconciler
```

Expected: PASS

**Step 5: Commit**

```bash
git add internal/controller/mariadb_controller.go internal/controller/mariadb_controller_test.go
git commit -m "feat: implement MariaDB controller with StatefulSet, Service, and Secret"
```

---

### Task 7: RabbitMQ controller

Follows the same StatefulSet pattern as MariaDB.

**Files:**
- Create: `internal/controller/rabbitmq_controller.go`
- Test: `internal/controller/rabbitmq_controller_test.go`

**Step 1: Write the failing test**

Same structure as MariaDB test but for `RabbitMQReconciler`:
- Creates `RabbitMQ` CR → expects StatefulSet, headless Service, and Secret (`rabbitmq-credentials` with keys `username` and `password`)
- Missing CR → no error, no requeue

**Step 2: Run test to verify it fails**

```bash
go test ./internal/controller/ -v -run TestRabbitMQReconciler
```

**Step 3: Write minimal implementation**

Same structure as MariaDB controller but:
- StatefulSet uses image `images.DefaultRabbitMQ`
- Port 5672 (amqp) + 15672 (management)
- Env: `RABBITMQ_DEFAULT_USER` and `RABBITMQ_DEFAULT_PASS` from secret
- Readiness probe: `rabbitmq-diagnostics check_port_connectivity`
- Secret keys: `username` (length 16) + `password` (length 32)
- Labels: `app.kubernetes.io/name: rabbitmq`

**Step 4: Run tests, verify pass**

**Step 5: Commit**

```bash
git add internal/controller/rabbitmq_controller.go internal/controller/rabbitmq_controller_test.go
git commit -m "feat: implement RabbitMQ controller"
```

---

### Task 8: Memcached controller

Simplest infra controller — Deployment (not StatefulSet), no secrets, no persistent storage.

**Files:**
- Create: `internal/controller/memcached_controller.go`
- Test: `internal/controller/memcached_controller_test.go`

**Step 1: Write the failing test**

- Creates `Memcached` CR → expects Deployment and Service
- No Secret needed (memcached has no auth in basic config)

**Step 2: Run test to verify it fails**

```bash
go test ./internal/controller/ -v -run TestMemcachedReconciler
```

**Step 3: Write minimal implementation**

- Deployment with `images.DefaultMemcached`
- Port 11211
- Service (ClusterIP, not headless)
- Readiness probe: TCP socket on port 11211
- Labels: `app.kubernetes.io/name: memcached`

**Step 4: Run tests, verify pass**

**Step 5: Commit**

```bash
git add internal/controller/memcached_controller.go internal/controller/memcached_controller_test.go
git commit -m "feat: implement Memcached controller"
```

---

### Task 9: Database provisioning helpers — `internal/common/database.go`

Helpers for service controllers to create databases and users in MariaDB.

**Files:**
- Create: `internal/common/database.go`
- Test: `internal/common/database_test.go`

**Step 1: Write the failing test**

```go
// internal/common/database_test.go
package common

import (
	"context"
	"testing"

	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestEnsureDatabase_CreatesJob(t *testing.T) {
	scheme := SetupScheme()
	client := fake.NewClientBuilder().WithScheme(scheme).Build()

	params := DatabaseParams{
		Name:          "keystone",
		Namespace:     "openstack",
		DatabaseName:  "keystone",
		Username:      "keystone",
		SecretName:    "keystone-db-password",
		MariaDBSecret: "mariadb-root-password",
		MariaDBHost:   "mariadb.openstack.svc",
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
```

**Step 2: Run test to verify it fails**

```bash
go test ./internal/common/ -v -run "TestEnsureDatabase|TestEnsureDBSync"
```

**Step 3: Write minimal implementation**

```go
// internal/common/database.go
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
	Name          string // resource name prefix
	Namespace     string
	DatabaseName  string // SQL database name
	Username      string // SQL username
	SecretName    string // Secret containing the service DB password (key: "password")
	MariaDBSecret string // Secret containing the MariaDB root password (key: "password")
	MariaDBHost   string // MariaDB hostname (e.g., "mariadb.openstack.svc")
}

// EnsureDatabase creates a Job that provisions a database and user in MariaDB.
// The Job is idempotent (uses IF NOT EXISTS). Skips creation if the Job already exists.
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
		`mysql -h %s -u root -p"$ROOT_PASSWORD" -e "CREATE DATABASE IF NOT EXISTS %s; CREATE USER IF NOT EXISTS '%s'@'%%' IDENTIFIED BY '$SERVICE_PASSWORD'; GRANT ALL ON %s.* TO '%s'@'%%'; FLUSH PRIVILEGES;"`,
		params.MariaDBHost, params.DatabaseName, params.Username, params.DatabaseName, params.Username,
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
							Image:   "mariadb:11",
							Command: []string{"sh", "-c", script},
							Env: []corev1.EnvVar{
								{
									Name: "ROOT_PASSWORD",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{Name: params.MariaDBSecret},
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
	SecretName string // Secret containing the service DB password (key: "password")
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
```

Note: `SetupScheme()` in `scheme.go` must also register `batchv1` types. Update it to add:

```go
import batchv1 "k8s.io/api/batch/v1"
// and in SetupScheme, before the Gateway API line:
utilruntime.Must(batchv1.AddToScheme(s))
```

**Step 4: Run test to verify it passes**

```bash
go test ./internal/common/ -v -run "TestEnsureDatabase|TestEnsureDBSync"
```

**Step 5: Commit**

```bash
git add internal/common/database.go internal/common/database_test.go internal/common/scheme.go
git commit -m "feat: add database provisioning and db_sync helpers"
```

---

### Task 10: Endpoint helpers — `internal/common/endpoint.go`

Helpers for registering services in the Keystone catalog via bootstrap Jobs.

**Files:**
- Create: `internal/common/endpoint.go`
- Test: `internal/common/endpoint_test.go`

**Step 1: Write the failing test**

```go
// internal/common/endpoint_test.go
package common

import (
	"context"
	"testing"

	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestEnsureKeystoneEndpoint_CreatesJob(t *testing.T) {
	scheme := SetupScheme()
	client := fake.NewClientBuilder().WithScheme(scheme).Build()

	params := EndpointParams{
		Name:            "glance",
		Namespace:       "openstack",
		ServiceName:     "glance",
		ServiceType:     "image",
		InternalURL:     "http://glance-api.openstack.svc:9292",
		PublicURL:       "https://glance.example.com",
		AdminURL:        "http://glance-api.openstack.svc:9292",
		Region:          "RegionOne",
		KeystoneSecret:  "keystone-admin-password",
		KeystoneURL:     "http://keystone-api.openstack.svc:5000/v3",
		BootstrapImage:  "quay.io/openstack.kolla/keystone:2025.1",
	}

	err := EnsureKeystoneEndpoint(context.Background(), client, params, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	job := &batchv1.Job{}
	err = client.Get(context.Background(), types.NamespacedName{
		Name:      "glance-endpoint-create",
		Namespace: "openstack",
	}, job)
	if err != nil {
		t.Fatalf("expected endpoint-create Job: %v", err)
	}
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./internal/common/ -v -run TestEnsureKeystoneEndpoint
```

**Step 3: Write minimal implementation**

```go
// internal/common/endpoint.go
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
	ServiceName    string // e.g., "glance"
	ServiceType    string // e.g., "image"
	InternalURL    string
	PublicURL      string
	AdminURL       string
	Region         string
	KeystoneSecret string // Secret containing admin password (key: "password")
	KeystoneURL    string // e.g., "http://keystone-api.openstack.svc:5000/v3"
	BootstrapImage string // Keystone image for running openstack CLI
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

	// Script uses the openstack CLI to create/update service and endpoints
	script := strings.Join([]string{
		fmt.Sprintf(`openstack service create --name %s --description "%s service" %s || true`, params.ServiceName, params.ServiceName, params.ServiceType),
		fmt.Sprintf(`openstack endpoint create --region %s %s internal %s || true`, params.Region, params.ServiceType, params.InternalURL),
		fmt.Sprintf(`openstack endpoint create --region %s %s public %s || true`, params.Region, params.ServiceType, params.PublicURL),
		fmt.Sprintf(`openstack endpoint create --region %s %s admin %s || true`, params.Region, params.ServiceType, params.AdminURL),
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
```

**Step 4: Run test to verify it passes**

```bash
go test ./internal/common/ -v -run TestEnsureKeystoneEndpoint
```

**Step 5: Commit**

```bash
git add internal/common/endpoint.go internal/common/endpoint_test.go
git commit -m "feat: add Keystone endpoint registration helper"
```

---

### Task 11: HTTPRoute helpers — `internal/common/httproute.go`

Helpers for creating/updating Gateway API HTTPRoute resources to expose OpenStack API endpoints externally.

**Files:**
- Create: `internal/common/httproute.go`
- Test: `internal/common/httproute_test.go`

**Step 1: Write the failing test**

```go
// internal/common/httproute_test.go
package common

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestEnsureHTTPRoute_CreatesRoute(t *testing.T) {
	scheme := SetupScheme()
	client := fake.NewClientBuilder().WithScheme(scheme).Build()

	params := HTTPRouteParams{
		Name:             "keystone-api",
		Namespace:        "openstack",
		Hostname:         "keystone.example.com",
		ServiceName:      "keystone-api",
		ServicePort:      5000,
		GatewayName:      "openstack-gateway",
		GatewayNamespace: "edge-system",
		ListenerName:     "https",
	}

	err := EnsureHTTPRoute(context.Background(), client, params, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	route := &gatewayv1.HTTPRoute{}
	err = client.Get(context.Background(), types.NamespacedName{
		Name:      "keystone-api",
		Namespace: "openstack",
	}, route)
	if err != nil {
		t.Fatalf("expected HTTPRoute to be created: %v", err)
	}

	if len(route.Spec.Hostnames) != 1 || string(route.Spec.Hostnames[0]) != "keystone.example.com" {
		t.Errorf("expected hostname keystone.example.com, got %v", route.Spec.Hostnames)
	}

	if len(route.Spec.ParentRefs) != 1 || string(route.Spec.ParentRefs[0].Name) != "openstack-gateway" {
		t.Errorf("expected parentRef openstack-gateway, got %v", route.Spec.ParentRefs)
	}
	if route.Spec.ParentRefs[0].Namespace == nil || string(*route.Spec.ParentRefs[0].Namespace) != "edge-system" {
		t.Errorf("expected parentRef namespace edge-system, got %v", route.Spec.ParentRefs[0].Namespace)
	}
	if route.Spec.ParentRefs[0].SectionName == nil || string(*route.Spec.ParentRefs[0].SectionName) != "https" {
		t.Errorf("expected listener sectionName=https, got %v", route.Spec.ParentRefs[0].SectionName)
	}

	rules := route.Spec.Rules
	if len(rules) != 1 || len(rules[0].BackendRefs) != 1 {
		t.Fatalf("expected 1 rule with 1 backendRef, got %d rules", len(rules))
	}
	ref := rules[0].BackendRefs[0]
	if string(ref.Name) != "keystone-api" {
		t.Errorf("expected backend keystone-api, got %s", ref.Name)
	}
	if *ref.Port != 5000 {
		t.Errorf("expected port 5000, got %d", *ref.Port)
	}
}

func TestEnsureHTTPRoute_UpdatesWhenSpecDrifts(t *testing.T) {
	scheme := SetupScheme()
	existing := &gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "keystone-api",
			Namespace: "openstack",
		},
		Spec: gatewayv1.HTTPRouteSpec{
			Hostnames: []gatewayv1.Hostname{"old.example.com"},
		},
	}

	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(existing).Build()

	params := HTTPRouteParams{
		Name:        "keystone-api",
		Namespace:   "openstack",
		Hostname:    "keystone.example.com",
		ServiceName: "keystone-api",
		ServicePort: 5000,
		GatewayName: "openstack-gateway",
	}

	err := EnsureHTTPRoute(context.Background(), client, params, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	updated := &gatewayv1.HTTPRoute{}
	if err := client.Get(context.Background(), types.NamespacedName{Name: "keystone-api", Namespace: "openstack"}, updated); err != nil {
		t.Fatalf("get route: %v", err)
	}
	if len(updated.Spec.Hostnames) != 1 || string(updated.Spec.Hostnames[0]) != "keystone.example.com" {
		t.Fatalf("expected hostname to be reconciled, got %v", updated.Spec.Hostnames)
	}
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./internal/common/ -v -run TestEnsureHTTPRoute
```

Expected: FAIL — `EnsureHTTPRoute` not defined.

**Step 3: Write minimal implementation**

```go
// internal/common/httproute.go
package common

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// HTTPRouteParams holds parameters for creating a Gateway API HTTPRoute.
type HTTPRouteParams struct {
	Name             string // HTTPRoute resource name
	Namespace        string
	Hostname         string // External hostname (e.g., "keystone.example.com")
	ServiceName      string // Backend Kubernetes Service name
	ServicePort      int32  // Backend Service port
	GatewayName      string // Name of the Gateway to attach to (default: "openstack-gateway")
	GatewayNamespace string // Namespace of Gateway (default: same as HTTPRoute namespace)
	ListenerName     string // Optional Gateway listener name (e.g., "https")
}

// EnsureHTTPRoute reconciles an HTTPRoute that routes external traffic to an OpenStack service.
// It is create-or-update, so hostname/gateway/backend changes are applied on subsequent reconciles.
func EnsureHTTPRoute(ctx context.Context, c client.Client, params HTTPRouteParams, owner metav1.Object) error {
	route := &gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      params.Name,
			Namespace: params.Namespace,
		},
	}

	gatewayName := params.GatewayName
	if gatewayName == "" {
		gatewayName = "openstack-gateway"
	}
	gatewayNamespace := params.GatewayNamespace
	if gatewayNamespace == "" {
		gatewayNamespace = params.Namespace
	}

	port := gatewayv1.PortNumber(params.ServicePort)

	_, err := controllerutil.CreateOrUpdate(ctx, c, route, func() error {
		route.Labels = map[string]string{
			"app.kubernetes.io/managed-by": "openstack-operator",
		}

		parent := gatewayv1.ParentReference{
			Name: gatewayv1.ObjectName(gatewayName),
		}
		if gatewayNamespace != params.Namespace {
			ns := gatewayv1.Namespace(gatewayNamespace)
			parent.Namespace = &ns
		}
		if params.ListenerName != "" {
			ln := gatewayv1.SectionName(params.ListenerName)
			parent.SectionName = &ln
		}

		route.Spec = gatewayv1.HTTPRouteSpec{
			CommonRouteSpec: gatewayv1.CommonRouteSpec{
				ParentRefs: []gatewayv1.ParentReference{parent},
			},
			Hostnames: []gatewayv1.Hostname{
				gatewayv1.Hostname(params.Hostname),
			},
			Rules: []gatewayv1.HTTPRouteRule{
				{
					BackendRefs: []gatewayv1.HTTPBackendRef{
						{
							BackendRef: gatewayv1.BackendRef{
								BackendObjectReference: gatewayv1.BackendObjectReference{
									Name: gatewayv1.ObjectName(params.ServiceName),
									Port: &port,
								},
							},
						},
					},
				},
			},
		}

		if owner != nil {
			return controllerutil.SetOwnerReference(owner, route, c.Scheme())
		}
		return nil
	})
	if err != nil {
		return err
	}

	return nil
}
```

**Step 4: Run test to verify it passes**

```bash
go test ./internal/common/ -v -run TestEnsureHTTPRoute
```

Expected: PASS

**Step 5: Commit**

```bash
git add internal/common/httproute.go internal/common/httproute_test.go
git commit -m "feat: add Gateway API HTTPRoute helper for service exposure"
```

---

### Task 12: Gateway sample — `config/samples/gateway.yaml`

A sample Gateway resource that cluster admins apply before deploying the operator.

**Files:**
- Create: `config/samples/gateway.yaml`

**Step 1: Write the sample**

```yaml
# config/samples/gateway.yaml
#
# Sample Gateway for the OpenStack operator.
# Apply this BEFORE creating an OpenStackControlPlane CR.
# Adjust gatewayClassName to match your installed Gateway API implementation
# (e.g., "eg" for Envoy Gateway, "istio" for Istio, "cilium" for Cilium).
#
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: openstack-gateway
  namespace: openstack
spec:
  gatewayClassName: eg  # Change to match your Gateway implementation
  listeners:
    - name: https
      protocol: HTTPS
      port: 443
      tls:
        mode: Terminate
        certificateRefs:
          - name: openstack-tls
      allowedRoutes:
        namespaces:
          from: Same
    - name: http
      protocol: HTTP
      port: 80
      allowedRoutes:
        namespaces:
          from: Same
```

If you attach routes from a different namespace, update the listener policy:
- set `allowedRoutes.namespaces.from: All` (or `Selector`)
- set `spec.gatewayRef.namespace` on the OpenStack control plane/service CRs

**Step 2: Verify valid YAML**

```bash
python3 -c "import yaml; yaml.safe_load(open('config/samples/gateway.yaml'))"
```

Expected: No errors.

**Step 3: Commit**

```bash
git add config/samples/gateway.yaml
git commit -m "feat: add sample Gateway resource for cluster setup"
```

---

### Task 13: Keystone controller

First service controller. Needs database, db_sync, deployment, service, bootstrap (admin user, admin project, service endpoints), and HTTPRoute for external access.

**Files:**
- Create: `internal/controller/keystone_controller.go`
- Test: `internal/controller/keystone_controller_test.go`

**Step 1: Write the failing test**

Tests should verify:
- Creates `Keystone` CR → expects:
  - Secret: `keystone-admin-password` (keys: `password`)
  - Secret: `keystone-db-password` (keys: `password`)
  - Job: `keystone-db-create`
  - Job: `keystone-db-sync`
  - Deployment: `keystone-api`
  - Service: `keystone-api`
  - Job: `keystone-bootstrap`
  - HTTPRoute: `keystone-api` (hostname from `spec.publicHostname`; parentRef from `spec.gatewayRef`)
- Missing CR → no error, no requeue

**Step 2: Run test to verify it fails**

```bash
go test ./internal/controller/ -v -run TestKeystoneReconciler
```

**Step 3: Write minimal implementation**

Reconcile flow:
1. Fetch CR, handle finalizer
2. Ensure admin password Secret
3. Ensure DB password Secret
4. Ensure database (Job: `keystone-db-create`)
5. Wait for db-create Job to complete
6. Ensure db_sync (Job: `keystone-db-sync`, command: `keystone-manage db_sync`)
7. Wait for db-sync Job to complete
8. Ensure Deployment (`keystone-api` on port 5000)
9. Ensure Service (ClusterIP, port 5000)
10. Ensure HTTPRoute (`keystone-api`)
11. Ensure bootstrap (Job: `keystone-bootstrap`, runs `keystone-manage bootstrap`)
12. Update status with `apiEndpoint` = `http://keystone-api.<namespace>.svc:5000/v3`

Key details:
- Image: `images.DefaultKeystone`
- ConfigMap: `keystone-config` with connection strings in env vars
- Deployment listens on port 5000 (unified keystone port)
- HTTPRoute hostname comes from `keystone.spec.publicHostname` (required for standalone Keystone CRs; auto-populated by control-plane controller for managed child CRs)
- Gateway parentRef comes from `keystone.spec.gatewayRef` (default `openstack-gateway` in same namespace when empty)
- Bootstrap public URL must use Gateway hostname, e.g. `https://<publicHostname>/v3` (or `http://` if your Gateway listener is plain HTTP)

**Step 4: Run tests, verify pass**

**Step 5: Commit**

```bash
git add internal/controller/keystone_controller.go internal/controller/keystone_controller_test.go
git commit -m "feat: implement Keystone controller with DB, bootstrap, and endpoint"
```

---

### Task 14: Glance controller

Image service. Needs database, db_sync, deployment, service, Keystone endpoint, HTTPRoute.

**Files:**
- Create: `internal/controller/glance_controller.go`
- Test: `internal/controller/glance_controller_test.go`

**Step 1: Write the failing test**

Tests should verify:
- Creates `Glance` CR → expects:
  - Secret: `glance-db-password`
  - Job: `glance-db-create`
  - Job: `glance-db-sync`
  - Deployment: `glance-api`
  - Service: `glance-api` (port 9292)
  - Job: `glance-endpoint-create`
  - HTTPRoute: `glance-api`

**Step 2: Run test to verify it fails**

```bash
go test ./internal/controller/ -v -run TestGlanceReconciler
```

**Step 3: Write minimal implementation**

Same pattern as Keystone but:
- Image: `images.DefaultGlanceAPI`
- Port: 9292
- DB name: `glance`
- db_sync command: `glance-manage db_sync`
- Endpoint registration: service type `image`; public URL must use Gateway hostname (not cluster service DNS)
- HTTPRoute: hostname from `glance.spec.publicHostname`, backend `glance-api:9292`
- For PVC storage backend: mount a PVC at `/var/lib/glance/images/`
- Status: `apiEndpoint = http://glance-api.<namespace>.svc:9292`

**Step 4: Run tests, verify pass**

**Step 5: Commit**

```bash
git add internal/controller/glance_controller.go internal/controller/glance_controller_test.go
git commit -m "feat: implement Glance controller"
```

---

### Task 15: Placement controller

Resource tracking API. Simple — API server, DB, Keystone endpoint, HTTPRoute.

**Files:**
- Create: `internal/controller/placement_controller.go`
- Test: `internal/controller/placement_controller_test.go`

**Step 1: Write the failing test**

Same pattern. Expects:
- Secret, db-create Job, db-sync Job, Deployment, Service (port 8778), endpoint-create Job, HTTPRoute

**Step 2: Run test to verify it fails**

```bash
go test ./internal/controller/ -v -run TestPlacementReconciler
```

**Step 3: Write minimal implementation**

- Image: `images.DefaultPlacement`
- Port: 8778
- DB name: `placement`
- db_sync command: `placement-manage db sync`
- Endpoint: service type `placement`; public URL must use Gateway hostname
- HTTPRoute: hostname from `placement.spec.publicHostname`, backend `placement-api:8778`
- Status: `apiEndpoint = http://placement-api.<namespace>.svc:8778`

**Step 4: Run tests, verify pass**

**Step 5: Commit**

```bash
git add internal/controller/placement_controller.go internal/controller/placement_controller_test.go
git commit -m "feat: implement Placement controller"
```

---

### Task 16: OVN Network controller

Deploys OVN infrastructure: Northbound DB, Southbound DB, ovn-northd. No HTTPRoute — OVN is internal-only.

**Files:**
- Create: `internal/controller/ovn_network_controller.go`
- Test: `internal/controller/ovn_network_controller_test.go`

**Step 1: Write the failing test**

Tests should verify:
- Creates `OVNNetwork` CR → expects:
  - StatefulSet: `ovn-nb-db` (port 6641)
  - StatefulSet: `ovn-sb-db` (port 6642)
  - Deployment: `ovn-northd`
  - Service: `ovn-nb-db` (headless, port 6641)
  - Service: `ovn-sb-db` (headless, port 6642)

**Step 2: Run test to verify it fails**

```bash
go test ./internal/controller/ -v -run TestOVNNetworkReconciler
```

**Step 3: Write minimal implementation**

No database or Keystone integration needed — OVN is pure infrastructure.
- NB DB: StatefulSet, image `images.DefaultOVNNBDB`, port 6641, PVC for data
- SB DB: StatefulSet, image `images.DefaultOVNSBDB`, port 6642, PVC for data
- northd: Deployment, image `images.DefaultOVNNorthd`, connects to NB/SB via env vars
- Status: `northboundDBEndpoint = tcp:ovn-nb-db.<namespace>.svc:6641`
- Status: `southboundDBEndpoint = tcp:ovn-sb-db.<namespace>.svc:6642`

**Step 4: Run tests, verify pass**

**Step 5: Commit**

```bash
git add internal/controller/ovn_network_controller.go internal/controller/ovn_network_controller_test.go
git commit -m "feat: implement OVN Network controller"
```

---

### Task 17: Neutron controller

Networking service. Needs DB, db_sync, Keystone endpoint, OVN connection config, HTTPRoute.

**Files:**
- Create: `internal/controller/neutron_controller.go`
- Test: `internal/controller/neutron_controller_test.go`

**Step 1: Write the failing test**

Expects:
- Secret: `neutron-db-password`
- Job: `neutron-db-create`
- Job: `neutron-db-sync`
- Deployment: `neutron-server`
- Service: `neutron-server` (port 9696)
- Job: `neutron-endpoint-create`
- HTTPRoute: `neutron-server`

**Step 2: Run test to verify it fails**

```bash
go test ./internal/controller/ -v -run TestNeutronReconciler
```

**Step 3: Write minimal implementation**

- Image: `images.DefaultNeutronServer`
- Port: 9696
- DB name: `neutron`
- db_sync command: `neutron-db-manage upgrade heads`
- Endpoint: service type `network`; public URL must use Gateway hostname
- HTTPRoute: hostname from `neutron.spec.publicHostname`, backend `neutron-server:9696`
- Env vars include OVN NB/SB connection strings
- Status: `apiEndpoint = http://neutron-server.<namespace>.svc:9696`

**Step 4: Run tests, verify pass**

**Step 5: Commit**

```bash
git add internal/controller/neutron_controller.go internal/controller/neutron_controller_test.go
git commit -m "feat: implement Neutron controller"
```

---

### Task 18: Nova controller

Most complex controller. Multiple sub-components: API, scheduler, conductor, compute. HTTPRoute for nova-api.

**Files:**
- Create: `internal/controller/nova_controller.go`
- Test: `internal/controller/nova_controller_test.go`

**Step 1: Write the failing test**

Expects:
- Secret: `nova-db-password`
- Job: `nova-db-create` (creates `nova`, `nova_api`, `nova_cell0` databases)
- Job: `nova-db-sync` (runs `nova-manage api_db sync` + `nova-manage db sync`)
- Job: `nova-cell-setup` (runs `nova-manage cell_v2 map_cell0` + `nova-manage cell_v2 create_cell --name cell1`)
- Deployment: `nova-api` (port 8774)
- Deployment: `nova-scheduler`
- Deployment: `nova-conductor`
- Deployment: `nova-compute` (replicas from `computeReplicas`)
- Service: `nova-api` (port 8774)
- Job: `nova-endpoint-create`
- HTTPRoute: `nova-api`

**Step 2: Run test to verify it fails**

```bash
go test ./internal/controller/ -v -run TestNovaReconciler
```

**Step 3: Write minimal implementation**

Reconcile flow:
1. Fetch CR, handle finalizer
2. Ensure DB password Secret
3. Ensure databases (3 DBs: `nova`, `nova_api`, `nova_cell0`)
4. Wait for db-create to complete
5. Ensure db_sync (api_db sync + db sync)
6. Wait for db-sync to complete
7. Ensure cell setup Job
8. Ensure Deployments: nova-api, nova-scheduler, nova-conductor, nova-compute
9. Ensure Service for nova-api (port 8774)
10. Ensure HTTPRoute for nova-api
11. Ensure Keystone endpoint (service type `compute`; public URL uses Gateway hostname)
12. Update status

Key details:
- nova-api: image `images.DefaultNovaAPI`, port 8774
- nova-scheduler: image `images.DefaultNovaScheduler`, no external port
- nova-conductor: image `images.DefaultNovaConductor`, no external port
- nova-compute: image `images.DefaultNovaCompute`, needs privileged for libvirt, `computeReplicas` from spec
- HTTPRoute: hostname from `nova.spec.publicHostname`, backend `nova-api:8774`
- All share RabbitMQ and DB connection env vars

**Step 4: Run tests, verify pass**

**Step 5: Commit**

```bash
git add internal/controller/nova_controller.go internal/controller/nova_controller_test.go
git commit -m "feat: implement Nova controller with api, scheduler, conductor, compute"
```

---

### Task 19: OpenStackControlPlane controller

Top-level orchestrator. Creates child CRs in dependency order, waits for each phase.

**Files:**
- Create: `internal/controller/controlplane_controller.go`
- Test: `internal/controller/controlplane_controller_test.go`

**Step 1: Write the failing test**

```go
// internal/controller/controlplane_controller_test.go
package controller

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	openstackv1alpha1 "github.com/mrrauch/openstack-operator/api/v1alpha1"
	"github.com/mrrauch/openstack-operator/internal/common"
)

func ready(gen int64) []metav1.Condition {
	return []metav1.Condition{{
		Type:               "Ready",
		Status:             metav1.ConditionTrue,
		ObservedGeneration: gen,
		LastTransitionTime: metav1.Now(),
		Reason:             "Ready",
	}}
}

func TestControlPlaneReconciler_CreatesInfrastructureCRs(t *testing.T) {
	scheme := common.SetupScheme()
	cp := &openstackv1alpha1.OpenStackControlPlane{
		ObjectMeta: metav1.ObjectMeta{Name: "my-cloud", Namespace: "openstack"},
		Spec: openstackv1alpha1.OpenStackControlPlaneSpec{
			NetworkBackend: "ovn",
		},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cp).WithStatusSubresource(cp).Build()
	r := &ControlPlaneReconciler{Client: c, Scheme: scheme}

	if _, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "my-cloud", Namespace: "openstack"},
	}); err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}

	for _, obj := range []struct {
		name string
		dst  client.Object
	}{
		{name: "my-cloud-mariadb", dst: &openstackv1alpha1.MariaDB{}},
		{name: "my-cloud-rabbitmq", dst: &openstackv1alpha1.RabbitMQ{}},
		{name: "my-cloud-memcached", dst: &openstackv1alpha1.Memcached{}},
		{name: "my-cloud-ovn", dst: &openstackv1alpha1.OVNNetwork{}},
	} {
		if err := c.Get(context.Background(), types.NamespacedName{Name: obj.name, Namespace: "openstack"}, obj.dst); err != nil {
			t.Fatalf("expected child CR %s: %v", obj.name, err)
		}
	}

	fresh := &openstackv1alpha1.OpenStackControlPlane{}
	_ = c.Get(context.Background(), types.NamespacedName{Name: "my-cloud", Namespace: "openstack"}, fresh)
	if fresh.Status.Phase != openstackv1alpha1.ControlPlanePhaseInfrastructure {
		t.Fatalf("expected phase Infrastructure, got %s", fresh.Status.Phase)
	}
}

func TestControlPlaneReconciler_AdvancesToIdentity(t *testing.T) {
	scheme := common.SetupScheme()
	cp := &openstackv1alpha1.OpenStackControlPlane{
		ObjectMeta: metav1.ObjectMeta{Name: "my-cloud", Namespace: "openstack"},
		Spec: openstackv1alpha1.OpenStackControlPlaneSpec{
			NetworkBackend: "ovn",
			PublicDomain:   "cloud.example.com",
			GatewayRef: openstackv1alpha1.GatewayRef{
				Name:      "edge-gw",
				Namespace: "edge-system",
			},
		},
		Status: openstackv1alpha1.OpenStackControlPlaneStatus{
			Phase: openstackv1alpha1.ControlPlanePhaseInfrastructure,
		},
	}
	mariadb := &openstackv1alpha1.MariaDB{ObjectMeta: metav1.ObjectMeta{Name: "my-cloud-mariadb", Namespace: "openstack"}, Status: openstackv1alpha1.MariaDBStatus{CommonStatus: openstackv1alpha1.CommonStatus{Conditions: ready(1)}}}
	rabbit := &openstackv1alpha1.RabbitMQ{ObjectMeta: metav1.ObjectMeta{Name: "my-cloud-rabbitmq", Namespace: "openstack"}, Status: openstackv1alpha1.RabbitMQStatus{CommonStatus: openstackv1alpha1.CommonStatus{Conditions: ready(1)}}}
	memcached := &openstackv1alpha1.Memcached{ObjectMeta: metav1.ObjectMeta{Name: "my-cloud-memcached", Namespace: "openstack"}, Status: openstackv1alpha1.MemcachedStatus{CommonStatus: openstackv1alpha1.CommonStatus{Conditions: ready(1)}}}
	ovn := &openstackv1alpha1.OVNNetwork{ObjectMeta: metav1.ObjectMeta{Name: "my-cloud-ovn", Namespace: "openstack"}, Status: openstackv1alpha1.OVNNetworkStatus{CommonStatus: openstackv1alpha1.CommonStatus{Conditions: ready(1)}}}

	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(cp, mariadb, rabbit, memcached, ovn).
		WithStatusSubresource(cp, mariadb, rabbit, memcached, ovn).
		Build()
	r := &ControlPlaneReconciler{Client: c, Scheme: scheme}

	if _, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "my-cloud", Namespace: "openstack"},
	}); err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}

	if err := c.Get(context.Background(), types.NamespacedName{Name: "my-cloud-keystone", Namespace: "openstack"}, &openstackv1alpha1.Keystone{}); err != nil {
		t.Fatalf("expected Keystone CR to be created: %v", err)
	}
	ks := &openstackv1alpha1.Keystone{}
	_ = c.Get(context.Background(), types.NamespacedName{Name: "my-cloud-keystone", Namespace: "openstack"}, ks)
	if ks.Spec.PublicHostname != "keystone.cloud.example.com" {
		t.Fatalf("expected default public hostname, got %q", ks.Spec.PublicHostname)
	}
	if ks.Spec.GatewayRef.Name != "edge-gw" || ks.Spec.GatewayRef.Namespace != "edge-system" {
		t.Fatalf("expected inherited gatewayRef edge-system/edge-gw, got %s/%s", ks.Spec.GatewayRef.Namespace, ks.Spec.GatewayRef.Name)
	}
	fresh := &openstackv1alpha1.OpenStackControlPlane{}
	_ = c.Get(context.Background(), types.NamespacedName{Name: "my-cloud", Namespace: "openstack"}, fresh)
	if fresh.Status.Phase != openstackv1alpha1.ControlPlanePhaseIdentity {
		t.Fatalf("expected phase Identity, got %s", fresh.Status.Phase)
	}
}

func TestControlPlaneReconciler_AdvancesToCompute(t *testing.T) {
	scheme := common.SetupScheme()
	cp := &openstackv1alpha1.OpenStackControlPlane{
		ObjectMeta: metav1.ObjectMeta{Name: "my-cloud", Namespace: "openstack"},
		Status: openstackv1alpha1.OpenStackControlPlaneStatus{
			Phase: openstackv1alpha1.ControlPlanePhaseCoreServices,
		},
	}
	glance := &openstackv1alpha1.Glance{ObjectMeta: metav1.ObjectMeta{Name: "my-cloud-glance", Namespace: "openstack"}, Status: openstackv1alpha1.GlanceStatus{CommonStatus: openstackv1alpha1.CommonStatus{Conditions: ready(1)}}}
	placement := &openstackv1alpha1.Placement{ObjectMeta: metav1.ObjectMeta{Name: "my-cloud-placement", Namespace: "openstack"}, Status: openstackv1alpha1.PlacementStatus{CommonStatus: openstackv1alpha1.CommonStatus{Conditions: ready(1)}}}
	neutron := &openstackv1alpha1.Neutron{ObjectMeta: metav1.ObjectMeta{Name: "my-cloud-neutron", Namespace: "openstack"}, Status: openstackv1alpha1.NeutronStatus{CommonStatus: openstackv1alpha1.CommonStatus{Conditions: ready(1)}}}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cp, glance, placement, neutron).WithStatusSubresource(cp, glance, placement, neutron).Build()
	r := &ControlPlaneReconciler{Client: c, Scheme: scheme}

	if _, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "my-cloud", Namespace: "openstack"},
	}); err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}

	if err := c.Get(context.Background(), types.NamespacedName{Name: "my-cloud-nova", Namespace: "openstack"}, &openstackv1alpha1.Nova{}); err != nil {
		t.Fatalf("expected Nova CR to be created: %v", err)
	}
	fresh := &openstackv1alpha1.OpenStackControlPlane{}
	_ = c.Get(context.Background(), types.NamespacedName{Name: "my-cloud", Namespace: "openstack"}, fresh)
	if fresh.Status.Phase != openstackv1alpha1.ControlPlanePhaseCompute {
		t.Fatalf("expected phase Compute, got %s", fresh.Status.Phase)
	}
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./internal/controller/ -v -run TestControlPlaneReconciler
```

**Step 3: Write minimal implementation**

Reconcile flow:
1. Fetch CR, handle finalizer
2. Phase: Infrastructure
   - Ensure MariaDB CR (from `spec.mariadb`)
   - Ensure RabbitMQ CR (from `spec.rabbitmq`)
   - Ensure Memcached CR (from `spec.memcached`)
   - If `spec.networkBackend == "ovn"`: ensure `OVNNetwork` CR
   - If `spec.ovnNetwork == nil`, create the child OVN CR with zero-value spec so API defaults apply
   - Wait for all infra to be Ready → advance to Identity
3. Phase: Identity
   - Ensure Keystone CR (from `spec.keystone`)
   - If empty, set child `keystone.spec.publicHostname = keystone.<controlPlane.spec.publicDomain>`
   - If empty, inherit child `keystone.spec.gatewayRef = controlPlane.spec.gatewayRef`
   - Wait for Keystone Ready → advance to CoreServices
4. Phase: CoreServices
   - Ensure Glance CR (set/inherit `publicHostname` + `gatewayRef` defaults)
   - Ensure Placement CR (set/inherit `publicHostname` + `gatewayRef` defaults)
   - Ensure Neutron CR (set/inherit `publicHostname` + `gatewayRef` defaults)
   - Wait for all core service APIs to be Ready → advance to Compute
5. Phase: Compute
   - Ensure Nova CR (set/inherit `publicHostname` + `gatewayRef` defaults)
   - Wait for Nova Ready → advance to Ready
6. Update phase + conditions

Key implementation detail: each child CR is created with:
- `ownerReference` pointing to the control plane CR
- Namespace inherited from the control plane CR
- Name derived from control plane name (e.g., `<cpname>-mariadb`)

Helper function for ensuring a child CR:

```go
func (r *ControlPlaneReconciler) ensureChildCR(ctx context.Context, owner *openstackv1alpha1.OpenStackControlPlane, obj client.Object) error {
	existing := obj.DeepCopyObject().(client.Object)
	err := r.Get(ctx, types.NamespacedName{Name: obj.GetName(), Namespace: obj.GetNamespace()}, existing)
	if err == nil {
		return nil // already exists
	}
	if !errors.IsNotFound(err) {
		return err
	}
	_ = controllerutil.SetOwnerReference(owner, obj, r.Scheme)
	return r.Create(ctx, obj)
}
```

Helper to check if a child CR is ready without reflection:

```go
func (r *ControlPlaneReconciler) isChildReady(ctx context.Context, name, namespace string, obj client.Object) (bool, error) {
	if err := r.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, obj); err != nil {
		if errors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return common.IsReady(getConditions(obj)), nil
}

func getConditions(obj client.Object) []metav1.Condition {
	switch o := obj.(type) {
	case *openstackv1alpha1.MariaDB:
		return o.Status.Conditions
	case *openstackv1alpha1.RabbitMQ:
		return o.Status.Conditions
	case *openstackv1alpha1.Memcached:
		return o.Status.Conditions
	case *openstackv1alpha1.OVNNetwork:
		return o.Status.Conditions
	case *openstackv1alpha1.Keystone:
		return o.Status.Conditions
	case *openstackv1alpha1.Glance:
		return o.Status.Conditions
	case *openstackv1alpha1.Placement:
		return o.Status.Conditions
	case *openstackv1alpha1.Neutron:
		return o.Status.Conditions
	case *openstackv1alpha1.Nova:
		return o.Status.Conditions
	default:
		return nil
	}
}
```

**Step 4: Run tests, verify pass**

**Step 5: Commit**

```bash
git add internal/controller/controlplane_controller.go internal/controller/controlplane_controller_test.go
git commit -m "feat: implement OpenStackControlPlane controller with phased dependency orchestration"
```

---

### Task 20: Wire all controllers in main.go

**Files:**
- Modify: `cmd/main.go`

**Step 1: Update main.go to register all controllers**

Replace the TODO block in `cmd/main.go` with:

```go
import (
	// ... existing imports ...
	"github.com/mrrauch/openstack-operator/internal/controller"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// In init(), add Gateway API scheme registration:
func init() {
	// ... existing scheme registrations ...
	utilruntime.Must(gatewayv1.Install(scheme))
}

// In main(), replace the TODO comment block with:

controllers := []struct {
	name  string
	setup func(mgr ctrl.Manager) error
}{
	{"MariaDB", (&controller.MariaDBReconciler{Client: mgr.GetClient(), Scheme: mgr.GetScheme()}).SetupWithManager},
	{"RabbitMQ", (&controller.RabbitMQReconciler{Client: mgr.GetClient(), Scheme: mgr.GetScheme()}).SetupWithManager},
	{"Memcached", (&controller.MemcachedReconciler{Client: mgr.GetClient(), Scheme: mgr.GetScheme()}).SetupWithManager},
	{"OVNNetwork", (&controller.OVNNetworkReconciler{Client: mgr.GetClient(), Scheme: mgr.GetScheme()}).SetupWithManager},
	{"Keystone", (&controller.KeystoneReconciler{Client: mgr.GetClient(), Scheme: mgr.GetScheme()}).SetupWithManager},
	{"Glance", (&controller.GlanceReconciler{Client: mgr.GetClient(), Scheme: mgr.GetScheme()}).SetupWithManager},
	{"Placement", (&controller.PlacementReconciler{Client: mgr.GetClient(), Scheme: mgr.GetScheme()}).SetupWithManager},
	{"Neutron", (&controller.NeutronReconciler{Client: mgr.GetClient(), Scheme: mgr.GetScheme()}).SetupWithManager},
	{"Nova", (&controller.NovaReconciler{Client: mgr.GetClient(), Scheme: mgr.GetScheme()}).SetupWithManager},
	{"OpenStackControlPlane", (&controller.ControlPlaneReconciler{Client: mgr.GetClient(), Scheme: mgr.GetScheme()}).SetupWithManager},
}
for _, c := range controllers {
	if err := c.setup(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", c.name)
		os.Exit(1)
	}
}
```

**Step 2: Verify it compiles**

```bash
cd /home/mrrauch/git/go/src/verbose-octo-invention && go build ./...
```

Expected: Clean build.

**Step 3: Commit**

```bash
git add cmd/main.go
git commit -m "feat: register all Phase 1 controllers in main.go"
```

---

### Task 21: Build validation, full test suite, and E2E acceptance

**Step 1: Run code generation**

```bash
make generate
```

**Step 2: Run formatting and vet**

```bash
make fmt && make vet
```

**Step 3: Run all tests**

```bash
make test
```

Expected: All tests pass with coverage output.

**Step 4: Run lint (if golangci-lint is available)**

```bash
make lint
```

Fix any issues.

**Step 5: Build the binary**

```bash
make build
```

Expected: `bin/openstack-operator` binary created.

**Step 6: Run Phase 1 E2E acceptance (VM smoke test)**

This closes the Phase 1 requirement from `PLAN.md` step 1.12.

```bash
# Deploy operator and sample control plane
kubectl apply -k config/default/
kubectl apply -f config/samples/controlplane_minimal.yaml

# Wait until control plane reaches Ready
kubectl wait --for=jsonpath='{.status.phase}'=Ready \
  openstackcontrolplane/my-cloud -n openstack --timeout=45m

# Smoke test through OpenStack API (requires openstack client + credentials)
openstack token issue
openstack server create --flavor m1.tiny --image cirros --network private phase1-smoke-vm
openstack server show phase1-smoke-vm -f value -c status
```

Expected: server status reaches `ACTIVE`.

**Step 7: Commit any fixes**

```bash
git add -A
git commit -m "chore: fix lint and build issues"
```

---

## Summary

| Task | Component | Files Created | Tests |
|------|-----------|---------------|-------|
| 1 | API exposure fields + generation | API type updates, `zz_generated.deepcopy.go`, CRD YAMLs | - |
| 2 | Condition helpers | `internal/common/conditions.go` | 3 tests |
| 3 | Secret helpers | `internal/common/secret.go`, `scheme.go` | 3 tests |
| 4 | Finalizer helpers | `internal/common/finalizer.go` | 3 tests |
| 5 | Default images | `internal/images/defaults.go` | - |
| 6 | MariaDB controller | `internal/controller/mariadb_controller.go` | 2 tests |
| 7 | RabbitMQ controller | `internal/controller/rabbitmq_controller.go` | 2 tests |
| 8 | Memcached controller | `internal/controller/memcached_controller.go` | 2 tests |
| 9 | Database helpers | `internal/common/database.go` | 2 tests |
| 10 | Endpoint helpers | `internal/common/endpoint.go` | 1 test |
| 11 | HTTPRoute helpers | `internal/common/httproute.go` | 2 tests |
| 12 | Gateway sample | `config/samples/gateway.yaml` | - |
| 13 | Keystone controller | `internal/controller/keystone_controller.go` | 2+ tests |
| 14 | Glance controller | `internal/controller/glance_controller.go` | 2+ tests |
| 15 | Placement controller | `internal/controller/placement_controller.go` | 2+ tests |
| 16 | OVN Network controller | `internal/controller/ovn_network_controller.go` | 2+ tests |
| 17 | Neutron controller | `internal/controller/neutron_controller.go` | 2+ tests |
| 18 | Nova controller | `internal/controller/nova_controller.go` | 2+ tests |
| 19 | ControlPlane controller | `internal/controller/controlplane_controller.go` | 3+ tests |
| 20 | Wire main.go | `cmd/main.go` (modify) | - |
| 21 | Build + acceptance validation | - | Full suite + VM E2E smoke |

**Total: ~21 tasks, ~35+ unit tests + 1 E2E smoke scenario, ~17 new files**
