# OpenStack Kubernetes Operator — Architectural Plan

## 1. Project Overview

**Goal**: Build a Kubernetes operator (in Go, using kubebuilder/operator-sdk) that
fully orchestrates the installation, configuration, and lifecycle management of an
OpenStack cloud running on Kubernetes. The operator controls every base service —
compute, networking, storage, identity, image, block storage, orchestration, and
dashboard — along with their shared infrastructure dependencies.

**Name**: `openstack-operator`
**API Group**: `openstack.k8s.io`
**Language**: Go
**Framework**: Kubebuilder v4 (operator-sdk compatible)
**License**: Apache-2.0

---

## 2. Design Principles

| # | Principle | Rationale |
|---|-----------|-----------|
| 1 | **Hierarchical CRDs** | A single top-level `OpenStackControlPlane` CR owns per-service CRs. Each service has its own controller. This mirrors the proven design of openstack-k8s-operators. |
| 2 | **Idempotent reconciliation** | Every controller must converge toward desired state no matter how many times it runs. No event-driven shortcuts. |
| 3 | **Dependency-ordered rollout** | Infrastructure (DB, MQ, cache) → Keystone → remaining services. The top-level controller enforces ordering via status conditions. |
| 4 | **Native containers, not VMs** | OpenStack services run as regular Pods/Deployments. No KubeVirt layer — this reduces overhead and simplifies upgrades (validated by StarlingX in production for 5+ years). |
| 5 | **One controller per CRD** | Clean separation of concerns, independent watch/reconcile cycles, and simpler testing. |
| 6 | **Separation of control plane and data plane** | Control-plane services run inside Kubernetes. Compute/storage/network data-plane nodes are managed via a separate `OpenStackDataPlane` CRD that drives Ansible jobs. |
| 7 | **Minimal viable scope first** | Phase 1 targets a working single-node cloud. HA, multi-site, and advanced backends come in later phases. |

---

## 3. OpenStack Service Dependency Graph

```
                         OpenStackControlPlane
                                  │
                 ┌────────────────┼────────────────┐
                 │                │                 │
            Infrastructure    Identity          Services
                 │                │                 │
        ┌────────┼────────┐       │      ┌──────┬──────┬──────┬──────┬──────┐
        │        │        │       │      │      │      │      │      │      │
     MariaDB  RabbitMQ Memcached Keystone Glance Placement Neutron Nova Cinder Heat Horizon
                                          │                  │       │     │
                                          │                  │       │     │
                                       (images)          (network)(compute)(block)
                                          │                  │       │     │
                                          └──────────────────┴───┬───┴─────┘
                                                                 │
                                                          Storage Backend
                                                          (Ceph / LVM)
                                                                 │
                                                          Network Backend
                                                          (OVN / OVS)
```

### Strict deployment order

```
1. MariaDB (Galera)      — all services need a database
2. RabbitMQ              — inter-process messaging
3. Memcached             — token caching
4. Keystone              — identity; everything authenticates through it
5. Glance               — image service (Nova needs it)
6. Placement             — resource tracking (Nova needs it)
7. Neutron               — networking (Nova needs it at VM boot)
8. Nova                  — compute
9. Cinder               — block storage (optional but common)
10. Heat                 — orchestration (optional)
11. Horizon              — dashboard (optional)
```

---

## 4. CRD Hierarchy

### 4.1 Top-Level CR

```yaml
apiVersion: openstack.k8s.io/v1alpha1
kind: OpenStackControlPlane
metadata:
  name: my-cloud
spec:
  # Global settings
  region: RegionOne
  storageBackend: ceph        # ceph | lvm
  networkBackend: ovn          # ovn | ovs
  tls:
    enabled: true
    issuerRef:
      name: openstack-ca
      kind: ClusterIssuer

  # Per-service templates (inline)
  mariadb:
    replicas: 3
    storageSize: 50Gi
  rabbitmq:
    replicas: 3
  memcached:
    replicas: 2
  keystone:
    replicas: 2
  glance:
    replicas: 2
    storageType: ceph          # ceph | pvc | swift
  placement:
    replicas: 2
  neutron:
    replicas: 2
    mechanism: ovn
  nova:
    replicas: 2
  cinder:
    enabled: true
    replicas: 2
    backends:
      - name: ceph-rbd
        type: ceph
  heat:
    enabled: false
  horizon:
    enabled: true
    replicas: 1
```

### 4.2 Per-Service CRDs

Each service gets its own CRD under the `openstack.k8s.io` API group:

| CRD Kind | API Version | Owner |
|----------|------------|-------|
| `OpenStackControlPlane` | `v1alpha1` | meta-operator (top-level) |
| `MariaDB` | `v1alpha1` | mariadb-controller |
| `RabbitMQ` | `v1alpha1` | rabbitmq-controller |
| `Memcached` | `v1alpha1` | memcached-controller |
| `Keystone` | `v1alpha1` | keystone-controller |
| `Glance` | `v1alpha1` | glance-controller |
| `Placement` | `v1alpha1` | placement-controller |
| `Neutron` | `v1alpha1` | neutron-controller |
| `Nova` | `v1alpha1` | nova-controller |
| `Cinder` | `v1alpha1` | cinder-controller |
| `Heat` | `v1alpha1` | heat-controller |
| `Horizon` | `v1alpha1` | horizon-controller |
| `OpenStackDataPlane` | `v1alpha1` | dataplane-controller |
| `CephStorage` | `v1alpha1` | ceph-controller |
| `OVNNetwork` | `v1alpha1` | ovn-controller |

### 4.3 Ownership & Garbage Collection

```
OpenStackControlPlane (parent)
  ├── ownerRef → MariaDB
  ├── ownerRef → RabbitMQ
  ├── ownerRef → Memcached
  ├── ownerRef → Keystone
  ├── ownerRef → Glance
  ├── ownerRef → Placement
  ├── ownerRef → Neutron
  ├── ownerRef → Nova
  ├── ownerRef → Cinder
  ├── ownerRef → Heat
  ├── ownerRef → Horizon
  ├── ownerRef → CephStorage
  └── ownerRef → OVNNetwork
```

Deleting the `OpenStackControlPlane` cascades deletion to all child CRs.
Each child CR uses finalizers for external resource cleanup (e.g., databases, Ceph pools).

---

## 5. Controller Design

### 5.1 Top-Level Controller: `OpenStackControlPlane`

**Responsibilities**:
- Create/update child CRs in dependency order
- Wait for each dependency's status conditions before proceeding
- Aggregate status from all children into a top-level status

**Reconciliation flow**:

```
Reconcile(OpenStackControlPlane)
  │
  ├─ Phase: Infrastructure
  │   ├─ Ensure MariaDB CR exists & status=Ready
  │   ├─ Ensure RabbitMQ CR exists & status=Ready
  │   └─ Ensure Memcached CR exists & status=Ready
  │
  ├─ Phase: Identity
  │   └─ Ensure Keystone CR exists & status=Ready
  │
  ├─ Phase: Core Services
  │   ├─ Ensure Glance CR exists & status=Ready
  │   ├─ Ensure Placement CR exists & status=Ready
  │   └─ Ensure Neutron CR exists & status=Ready
  │
  ├─ Phase: Compute & Storage
  │   ├─ Ensure Nova CR exists & status=Ready
  │   └─ Ensure Cinder CR exists & status=Ready (if enabled)
  │
  ├─ Phase: Optional Services
  │   ├─ Ensure Heat CR exists & status=Ready (if enabled)
  │   └─ Ensure Horizon CR exists & status=Ready (if enabled)
  │
  └─ Update OpenStackControlPlane status
      ├─ conditions: [{type: Ready, status: True/False}]
      └─ phase: "Infrastructure" | "Identity" | "CoreServices" | ...
```

### 5.2 Per-Service Controller Pattern

Every service controller follows the same template:

```go
func (r *KeystoneReconciler) Reconcile(ctx, req) (Result, error) {
    // 1. Fetch the CR
    instance := &v1alpha1.Keystone{}
    if err := r.Get(ctx, req.NamespacedName, instance); err != nil {
        return ctrl.Result{}, client.IgnoreNotFound(err)
    }

    // 2. Handle deletion (finalizer)
    if !instance.DeletionTimestamp.IsZero() {
        return r.handleDeletion(ctx, instance)
    }
    if err := r.ensureFinalizer(ctx, instance); err != nil {
        return ctrl.Result{}, err
    }

    // 3. Set status to Progressing
    r.setCondition(instance, "Ready", metav1.ConditionUnknown, "Reconciling")

    // 4. Ensure database exists (create DB, user, grant)
    if err := r.ensureDatabase(ctx, instance); err != nil {
        return ctrl.Result{RequeueAfter: 10s}, err
    }

    // 5. Ensure config (ConfigMap with keystone.conf, etc.)
    if err := r.ensureConfig(ctx, instance); err != nil {
        return ctrl.Result{}, err
    }

    // 6. Run DB migrations (Job)
    if err := r.ensureDBSync(ctx, instance); err != nil {
        return ctrl.Result{RequeueAfter: 30s}, err
    }

    // 7. Ensure Deployment
    if err := r.ensureDeployment(ctx, instance); err != nil {
        return ctrl.Result{}, err
    }

    // 8. Ensure Service + Ingress
    if err := r.ensureService(ctx, instance); err != nil {
        return ctrl.Result{}, err
    }

    // 9. Bootstrap (create admin user, endpoints, etc.)
    if err := r.ensureBootstrap(ctx, instance); err != nil {
        return ctrl.Result{RequeueAfter: 30s}, err
    }

    // 10. Mark Ready
    r.setCondition(instance, "Ready", metav1.ConditionTrue, "DeploymentReady")
    return ctrl.Result{}, nil
}
```

### 5.3 Cross-Service Communication

Services discover each other via the Keystone service catalog. The operator:
1. Registers each service endpoint in Keystone during bootstrap
2. Generates `clouds.yaml` / service config with auth URLs pointing to the Keystone internal endpoint
3. Uses Kubernetes Services for internal DNS (e.g., `keystone-api.openstack.svc.cluster.local`)

---

## 6. Storage Architecture

### 6.1 Ceph Integration (Primary Backend)

```
CephStorage CR
  │
  ├─ Option A: Deploy Rook-Ceph (operator manages Rook CRs)
  │   ├─ CephCluster CR → Rook deploys MON, OSD, MGR
  │   ├─ CephBlockPool CR → pool for Cinder volumes
  │   ├─ CephBlockPool CR → pool for Glance images
  │   └─ CephBlockPool CR → pool for Nova ephemeral
  │
  └─ Option B: Connect to external Ceph cluster
      ├─ Store ceph.conf + keyring in Secret
      └─ Create pools via ceph CLI Job
```

**Integration points**:
- **Cinder** → `rbd` backend driver, talks to Ceph via librados
- **Glance** → `rbd` store, images stored as RBD images
- **Nova** → Ephemeral disks on RBD, live migration support
- **Manila** (future) → CephFS backend

### 6.2 LVM Backend (Development/Single-Node)

- Cinder uses `LVMVolumeDriver` with a local VG
- Backed by a PVC or hostPath volume
- No HA — suitable only for dev/test

---

## 7. Networking Architecture

### 7.1 OVN Integration (Default)

```
OVNNetwork CR
  │
  ├─ OVN Northbound DB    (StatefulSet, 3 replicas for HA)
  ├─ OVN Southbound DB    (StatefulSet, 3 replicas for HA)
  ├─ ovn-northd           (Deployment, leader-elected)
  ├─ ovn-controller       (DaemonSet on compute nodes)
  └─ ovn-metadata-agent   (DaemonSet on compute nodes)
```

**Neutron integration**:
- Neutron uses `ML2/OVN` mechanism driver
- `networking-ovn` plugin talks to OVN Northbound DB
- No legacy agents needed (no L3 agent, no DHCP agent — OVN handles these natively)

**Optional: Shared OVN with Kubernetes CNI**
- If the cluster uses OVN-Kubernetes or Kube-OVN as its CNI, Neutron can connect
  to the same OVN Northbound DB, unifying VM and container networking

### 7.2 OVS Backend (Legacy)

- Neutron ML2/OVS with L2 agent, L3 agent, DHCP agent, metadata agent
- Each agent runs as a DaemonSet on compute/network nodes
- More components to manage, but wider existing deployment base

### 7.3 Network Topology

```
External Network (provider)
       │
  ┌────▼─────────────────────────────────────┐
  │  OVN Gateway Chassis / L3 Router         │
  │  (North-South traffic: SNAT/DNAT/FIP)    │
  └────┬─────────────────────────────────────┘
       │
  ┌────▼─────────────────────────────────────┐
  │  OVN Logical Switch (tenant network)     │
  │  (East-West traffic: inter-VM)           │
  │  DHCP built-in, metadata built-in        │
  └────┬───────────────┬─────────────────────┘
       │               │
   ┌───▼───┐       ┌───▼───┐
   │  VM1  │       │  VM2  │
   └───────┘       └───────┘
```

---

## 8. Project Structure

```
openstack-operator/
├── cmd/
│   └── main.go                          # Entrypoint
├── api/
│   └── v1alpha1/
│       ├── groupversion_info.go
│       ├── openstack_controlplane_types.go
│       ├── mariadb_types.go
│       ├── rabbitmq_types.go
│       ├── memcached_types.go
│       ├── keystone_types.go
│       ├── glance_types.go
│       ├── placement_types.go
│       ├── neutron_types.go
│       ├── nova_types.go
│       ├── cinder_types.go
│       ├── heat_types.go
│       ├── horizon_types.go
│       ├── ceph_storage_types.go
│       ├── ovn_network_types.go
│       ├── openstack_dataplane_types.go
│       ├── common_types.go              # Shared structs
│       └── zz_generated.deepcopy.go
├── internal/
│   ├── controller/
│   │   ├── openstack_controlplane_controller.go
│   │   ├── mariadb_controller.go
│   │   ├── rabbitmq_controller.go
│   │   ├── memcached_controller.go
│   │   ├── keystone_controller.go
│   │   ├── glance_controller.go
│   │   ├── placement_controller.go
│   │   ├── neutron_controller.go
│   │   ├── nova_controller.go
│   │   ├── cinder_controller.go
│   │   ├── heat_controller.go
│   │   ├── horizon_controller.go
│   │   ├── ceph_storage_controller.go
│   │   ├── ovn_network_controller.go
│   │   └── openstack_dataplane_controller.go
│   ├── common/
│   │   ├── conditions.go                # Status condition helpers
│   │   ├── database.go                  # DB creation / migration helpers
│   │   ├── endpoint.go                  # Keystone endpoint registration
│   │   ├── secret.go                    # Secret generation (passwords, keys)
│   │   └── template.go                  # Config file rendering
│   └── images/
│       └── defaults.go                  # Default container images
├── config/
│   ├── crd/
│   │   └── bases/                       # Generated CRD YAMLs
│   ├── manager/
│   │   └── manager.yaml                 # Operator Deployment
│   ├── rbac/
│   │   └── role.yaml                    # Generated RBAC
│   ├── samples/
│   │   ├── controlplane_minimal.yaml
│   │   ├── controlplane_ha.yaml
│   │   └── controlplane_ceph.yaml
│   └── default/
│       └── kustomization.yaml
├── templates/
│   ├── keystone/
│   │   ├── keystone.conf.tmpl
│   │   └── wsgi-keystone.conf.tmpl
│   ├── glance/
│   │   └── glance-api.conf.tmpl
│   ├── nova/
│   │   ├── nova.conf.tmpl
│   │   └── nova-compute.conf.tmpl
│   ├── neutron/
│   │   └── neutron.conf.tmpl
│   ├── cinder/
│   │   └── cinder.conf.tmpl
│   └── ...
├── test/
│   ├── e2e/
│   │   ├── controlplane_test.go
│   │   └── suite_test.go
│   └── functional/
│       ├── keystone_test.go
│       └── ...
├── Dockerfile
├── Makefile
├── go.mod
├── go.sum
├── PROJECT                              # Kubebuilder project metadata
└── README.md
```

---

## 9. Implementation Phases

### Phase 1 — Foundation (MVP: single-node, minimal cloud)

**Goal**: Deploy a working OpenStack cloud (Keystone + Glance + Nova + Neutron + Placement) on a single Kubernetes cluster with local storage and OVN networking.

| Step | Deliverable |
|------|-------------|
| 1.1 | Scaffold project with kubebuilder, define all API types |
| 1.2 | Implement `MariaDB` controller (single-instance, PVC-backed) |
| 1.3 | Implement `RabbitMQ` controller (single-instance) |
| 1.4 | Implement `Memcached` controller (single-instance) |
| 1.5 | Implement `Keystone` controller (deploy, db-sync, bootstrap) |
| 1.6 | Implement `Glance` controller (local/PVC storage backend) |
| 1.7 | Implement `Placement` controller |
| 1.8 | Implement `OVNNetwork` controller (deploy OVN NB/SB/northd) |
| 1.9 | Implement `Neutron` controller (ML2/OVN) |
| 1.10 | Implement `Nova` controller (api, scheduler, conductor, compute) |
| 1.11 | Implement `OpenStackControlPlane` controller (dependency orchestration) |
| 1.12 | End-to-end test: create a VM on the deployed cloud |

### Phase 2 — Storage & Block Services

| Step | Deliverable |
|------|-------------|
| 2.1 | Implement `CephStorage` controller (Rook integration or external) |
| 2.2 | Implement `Cinder` controller (ceph-rbd backend) |
| 2.3 | Wire Glance → Ceph RBD store |
| 2.4 | Wire Nova → Ceph ephemeral disks |
| 2.5 | Test: create volume, attach to VM, write data, detach, reattach |

### Phase 3 — High Availability

| Step | Deliverable |
|------|-------------|
| 3.1 | MariaDB Galera clustering (3-node) |
| 3.2 | RabbitMQ clustering (3-node, quorum queues) |
| 3.3 | Multi-replica API services behind Ingress |
| 3.4 | OVN NB/SB database HA (Raft clustering) |
| 3.5 | Anti-affinity rules, PodDisruptionBudgets |
| 3.6 | Chaos testing (kill pods, nodes, verify recovery) |

### Phase 4 — Data Plane Management

| Step | Deliverable |
|------|-------------|
| 4.1 | `OpenStackDataPlane` CRD for managing external compute nodes |
| 4.2 | Ansible runner integration (Job-based execution) |
| 4.3 | Compute node provisioning (nova-compute, ovn-controller, libvirt) |
| 4.4 | Storage node provisioning (Ceph OSD deployment) |
| 4.5 | Network node provisioning (OVN gateway chassis) |

### Phase 5 — Optional Services & Extras

| Step | Deliverable |
|------|-------------|
| 5.1 | `Heat` controller (orchestration) |
| 5.2 | `Horizon` controller (dashboard) |
| 5.3 | TLS everywhere (cert-manager integration) |
| 5.4 | Upgrade orchestration (rolling upgrades between OpenStack releases) |
| 5.5 | Monitoring integration (Prometheus exporters, Grafana dashboards) |
| 5.6 | Backup/restore for MariaDB and Ceph |

### Phase 6 — Advanced Networking & Multi-Site

| Step | Deliverable |
|------|-------------|
| 6.1 | Shared OVN with Kubernetes CNI (Kube-OVN / OVN-Kubernetes) |
| 6.2 | BGP integration for provider networks |
| 6.3 | SR-IOV support for high-performance workloads |
| 6.4 | Multi-site / distributed cloud (OVN-IC, federated Keystone) |

---

## 10. Key Technical Decisions

### 10.1 Container Images

Use upstream OpenStack container images from `quay.io/openstack.kolla/` (Kolla-built images) or build custom images. The operator stores default image references in `internal/images/defaults.go` and allows overrides via the CR spec and webhooks.

### 10.2 Configuration Management

- OpenStack service configs (`*.conf`) are rendered from Go templates in `templates/`
- Templates are populated from CR spec fields + generated secrets
- Rendered configs are stored in ConfigMaps, mounted into service pods
- Config changes trigger rolling restarts via annotation hashing

### 10.3 Secret Management

- Database passwords, RabbitMQ credentials, Keystone admin password, and service user
  passwords are auto-generated and stored in Kubernetes Secrets
- Ceph keyrings are either generated (Rook) or provided (external Ceph)
- Fernet keys for Keystone are generated and rotated via a CronJob

### 10.4 Database Lifecycle

Each service controller:
1. Creates a database and user in MariaDB (via a Job or direct SQL)
2. Runs `<service>-manage db_sync` as a Kubernetes Job
3. Waits for Job completion before creating the Deployment
4. On upgrade, runs db_sync again before rolling out new Deployment

### 10.5 Ingress & Service Exposure

- Internal services use ClusterIP Services with internal DNS
- Public API endpoints use an Ingress controller (default: HAProxy Ingress)
- Each public service gets a unique hostname or path-based route
- TLS termination at the Ingress level (cert-manager integration)

---

## 11. Testing Strategy

| Layer | Tool | Scope |
|-------|------|-------|
| Unit | Go `testing` + gomock | Individual functions, config rendering |
| Controller | envtest (controller-runtime) | Controller reconciliation against fake API server |
| Integration | Kind cluster + test suite | Full CRD lifecycle on a real (local) cluster |
| E2E | Kind/real cluster + OpenStack CLI | Deploy full cloud, create VM, attach volume, verify networking |
| Chaos | Litmus / custom scripts | Kill pods, nodes; verify self-healing reconciliation |

---

## 12. Open Questions / Risks

| # | Question | Notes |
|---|----------|-------|
| 1 | **Target OpenStack release?** | Recommend starting with 2025.2 (Epoxy) as it is the latest stable. |
| 2 | **Kolla vs custom images?** | Kolla images are well-maintained and support all services. Custom images add flexibility but maintenance burden. |
| 3 | **Rook-managed vs external Ceph?** | Supporting both is ideal. Phase 1 can skip Ceph entirely (use PVC/local). |
| 4 | **Compute on K8s nodes or bare metal?** | Phase 1: K8s nodes (nested virt or privileged containers). Phase 4: bare metal via DataPlane CRD. |
| 5 | **Multi-tenancy of the operator itself?** | Single operator instance per cluster, multiple `OpenStackControlPlane` CRs in different namespaces. |
| 6 | **OLM distribution?** | Plan for OperatorHub/OLM packaging in Phase 5+. |

---

## 13. References

- [openstack-k8s-operators](https://github.com/openstack-k8s-operators) — Red Hat's operator-based OpenStack on K8s
- [OpenStack-Helm](https://docs.openstack.org/openstack-helm/latest/) — Helm chart approach
- [StarlingX](https://www.starlingx.io/) — Production-grade containerized OpenStack
- [Canonical Sunbeam](https://canonical.com/microstack) — Juju charm approach
- [Kubebuilder Book](https://book.kubebuilder.io/) — Operator framework documentation
- [Operator SDK](https://sdk.operatorframework.io/) — Operator development toolkit
- [Rook Ceph](https://rook.io/) — Ceph on Kubernetes
- [OVN Architecture](https://www.ovn.org/en/architecture/) — OVN networking
