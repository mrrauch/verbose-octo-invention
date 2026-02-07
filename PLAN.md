# OpenStack Kubernetes Operator — Architectural Plan

## 1. Project Overview

**Goal**: Build a Kubernetes operator (in Go, using kubebuilder/operator-sdk) that
fully orchestrates the installation, configuration, and lifecycle management of an
OpenStack cloud running on Kubernetes. The operator controls every base service —
compute, networking, storage, identity, image, block storage, orchestration, and
dashboard — plus all actively maintained extended services (load balancing, DNS,
bare metal, shared filesystems, key management, containers, telemetry, NFV, and
more), along with their shared infrastructure dependencies.

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
                 ┌─────────────────────┼──────────────────────┐
                 │                     │                       │
            Infrastructure         Identity               Services
                 │                     │                       │
        ┌────────┼────────┐            │         ┌─────────────┼─────────────┐
        │        │        │            │         │             │             │
     MariaDB  RabbitMQ Memcached   Keystone    Core       Extended       UI/Ops
                                                │             │             │
                                    ┌───────────┤    ┌────────┤      ┌──────┤
                                    │           │    │        │      │      │
                                  Glance    Placement │     Octavia Horizon Skyline
                                    │           │    │        │
                                  Neutron     Nova  Barbican Designate
                                    │           │    │        │
                                  Cinder      Heat  Swift   Manila
                                                     │        │
                                              ┌──────┤    ┌───┤
                                              │      │    │   │
                                           Ironic Magnum │  Trove
                                              │      │   │    │
                                           Masakari  │  Tacker│
                                              │     Zun  │  Mistral
                                           Cyborg    │   │    │
                                              │   Blazar │  Aodh
                                           CloudKitty│  │    │
                                              │  Freezer│  Ceilometer
                                           Watcher  │   │    │
                                              │  Vitrage│   Venus
                                           Zaqar  Adjutant
                                              │
                                           Storlets
                                              │
                                     ┌────────┴────────┐
                                     │                  │
                               Storage Backend    Network Backend
                               (Ceph / LVM)       (OVN / OVS)
```

### Strict deployment order

Core services are deployed first; extended services can be deployed in any
order after their specific dependencies are satisfied.

```
Phase 1 — Infrastructure
  1. MariaDB (Galera)          — all services need a database
  2. RabbitMQ                  — inter-process messaging
  3. Memcached                 — token caching

Phase 2 — Identity & Security
  4. Keystone                  — identity; everything authenticates through it
  5. Barbican                  — key manager (optional, but Octavia/TLS need it)

Phase 3 — Core Services
  6. Glance                    — image service (Nova needs it)
  7. Placement                 — resource tracking (Nova needs it)
  8. Neutron                   — networking (Nova needs it at VM boot)
  9. Nova                      — compute
  10. Cinder                   — block storage

Phase 4 — Extended Services (order flexible, based on enabled flags)
  11. Heat                     — orchestration
  12. Swift                    — object storage
  13. Octavia                  — load balancer (needs Nova, Neutron, Glance)
  14. Designate                — DNS (needs Neutron)
  15. Manila                   — shared filesystems (needs Neutron)
  16. Ironic                   — bare metal (needs Glance, Neutron)
  17. Magnum                   — container infra (needs Nova, Neutron, Heat)
  18. Trove                    — database-as-a-service (needs Nova, Neutron, Cinder)
  19. Aodh                     — alarming (needs Ceilometer)
  20. Ceilometer               — telemetry data collection
  21. Tacker                   — NFV orchestration (needs Nova, Neutron, Heat)
  22. Mistral                  — workflow engine
  23. Masakari                 — instance HA (needs Nova)
  24. Cyborg                   — accelerator management (needs Nova, Placement)
  25. Blazar                   — resource reservation (needs Nova)
  26. Zun                      — containers (needs Neutron, optionally Cinder)
  27. CloudKitty               — rating/billing (needs Ceilometer)
  28. Watcher                  — resource optimization (needs Nova, Ceilometer)
  29. Vitrage                  — root cause analysis (needs Ceilometer, Aodh)
  30. Zaqar                    — messaging service
  31. Freezer                  — backup/restore (needs Swift)
  32. Venus                    — log management
  33. Adjutant                 — ops process automation (needs Keystone)
  34. Storlets                 — compute-in-storage (needs Swift)

Phase 5 — Dashboards
  35. Horizon                  — classic dashboard
  36. Skyline                  — modern dashboard (React-based)
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

**Infrastructure & Control**

| CRD Kind | API Version | Owner |
|----------|------------|-------|
| `OpenStackControlPlane` | `v1alpha1` | meta-operator (top-level) |
| `MariaDB` | `v1alpha1` | mariadb-controller |
| `RabbitMQ` | `v1alpha1` | rabbitmq-controller |
| `Memcached` | `v1alpha1` | memcached-controller |
| `OpenStackDataPlane` | `v1alpha1` | dataplane-controller |
| `CephStorage` | `v1alpha1` | ceph-controller |
| `OVNNetwork` | `v1alpha1` | ovn-controller |

**Core Services**

| CRD Kind | API Version | Owner |
|----------|------------|-------|
| `Keystone` | `v1alpha1` | keystone-controller |
| `Glance` | `v1alpha1` | glance-controller |
| `Placement` | `v1alpha1` | placement-controller |
| `Neutron` | `v1alpha1` | neutron-controller |
| `Nova` | `v1alpha1` | nova-controller |
| `Cinder` | `v1alpha1` | cinder-controller |
| `Heat` | `v1alpha1` | heat-controller |

**Extended Services — Tier 1 (commonly deployed)**

| CRD Kind | API Version | Owner | Function |
|----------|------------|-------|----------|
| `Swift` | `v1alpha1` | swift-controller | Object storage |
| `Barbican` | `v1alpha1` | barbican-controller | Key/secret management |
| `Octavia` | `v1alpha1` | octavia-controller | Load balancing (LBaaS) |
| `Designate` | `v1alpha1` | designate-controller | DNS as a service |
| `Manila` | `v1alpha1` | manila-controller | Shared filesystems |
| `Ironic` | `v1alpha1` | ironic-controller | Bare metal provisioning |
| `Magnum` | `v1alpha1` | magnum-controller | Kubernetes cluster management |

**Extended Services — Tier 2 (moderately deployed)**

| CRD Kind | API Version | Owner | Function |
|----------|------------|-------|----------|
| `Trove` | `v1alpha1` | trove-controller | Database as a service |
| `Ceilometer` | `v1alpha1` | ceilometer-controller | Telemetry data collection |
| `Aodh` | `v1alpha1` | aodh-controller | Alarming |
| `Masakari` | `v1alpha1` | masakari-controller | Instance HA / auto-recovery |
| `Mistral` | `v1alpha1` | mistral-controller | Workflow engine |
| `Tacker` | `v1alpha1` | tacker-controller | NFV orchestration |

**Extended Services — Tier 3 (niche / specialized)**

| CRD Kind | API Version | Owner | Function |
|----------|------------|-------|----------|
| `Cyborg` | `v1alpha1` | cyborg-controller | GPU/FPGA accelerator management |
| `Blazar` | `v1alpha1` | blazar-controller | Resource reservation |
| `Zun` | `v1alpha1` | zun-controller | Container management |
| `CloudKitty` | `v1alpha1` | cloudkitty-controller | Rating / billing |
| `Watcher` | `v1alpha1` | watcher-controller | Resource optimization |
| `Vitrage` | `v1alpha1` | vitrage-controller | Root cause analysis |
| `Zaqar` | `v1alpha1` | zaqar-controller | Messaging service |
| `Freezer` | `v1alpha1` | freezer-controller | Backup and restore |
| `Venus` | `v1alpha1` | venus-controller | Log management |
| `Adjutant` | `v1alpha1` | adjutant-controller | Ops process automation |
| `Storlets` | `v1alpha1` | storlets-controller | Compute inside object storage |

**Dashboards**

| CRD Kind | API Version | Owner | Function |
|----------|------------|-------|----------|
| `Horizon` | `v1alpha1` | horizon-controller | Classic dashboard |
| `Skyline` | `v1alpha1` | skyline-controller | Modern React dashboard |

### 4.3 Ownership & Garbage Collection

```
OpenStackControlPlane (parent)
  │
  ├── Infrastructure
  │   ├── ownerRef → MariaDB
  │   ├── ownerRef → RabbitMQ
  │   ├── ownerRef → Memcached
  │   ├── ownerRef → CephStorage
  │   └── ownerRef → OVNNetwork
  │
  ├── Core Services
  │   ├── ownerRef → Keystone
  │   ├── ownerRef → Glance
  │   ├── ownerRef → Placement
  │   ├── ownerRef → Neutron
  │   ├── ownerRef → Nova
  │   ├── ownerRef → Cinder
  │   └── ownerRef → Heat
  │
  ├── Extended — Tier 1
  │   ├── ownerRef → Swift
  │   ├── ownerRef → Barbican
  │   ├── ownerRef → Octavia
  │   ├── ownerRef → Designate
  │   ├── ownerRef → Manila
  │   ├── ownerRef → Ironic
  │   └── ownerRef → Magnum
  │
  ├── Extended — Tier 2
  │   ├── ownerRef → Trove
  │   ├── ownerRef → Ceilometer
  │   ├── ownerRef → Aodh
  │   ├── ownerRef → Masakari
  │   ├── ownerRef → Mistral
  │   └── ownerRef → Tacker
  │
  ├── Extended — Tier 3
  │   ├── ownerRef → Cyborg
  │   ├── ownerRef → Blazar
  │   ├── ownerRef → Zun
  │   ├── ownerRef → CloudKitty
  │   ├── ownerRef → Watcher
  │   ├── ownerRef → Vitrage
  │   ├── ownerRef → Zaqar
  │   ├── ownerRef → Freezer
  │   ├── ownerRef → Venus
  │   ├── ownerRef → Adjutant
  │   └── ownerRef → Storlets
  │
  └── Dashboards
      ├── ownerRef → Horizon
      └── ownerRef → Skyline
```

Deleting the `OpenStackControlPlane` cascades deletion to all child CRs.
Each child CR uses finalizers for external resource cleanup (e.g., databases, Ceph pools).
Extended services are only created when explicitly enabled in the control plane spec.

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
  │   ├─ Ensure Memcached CR exists & status=Ready
  │   ├─ Ensure CephStorage CR exists & status=Ready (if storageBackend=ceph)
  │   └─ Ensure OVNNetwork CR exists & status=Ready (if networkBackend=ovn)
  │
  ├─ Phase: Identity & Security
  │   ├─ Ensure Keystone CR exists & status=Ready
  │   └─ Ensure Barbican CR exists & status=Ready (if enabled)
  │
  ├─ Phase: Core Services
  │   ├─ Ensure Glance CR exists & status=Ready
  │   ├─ Ensure Placement CR exists & status=Ready
  │   ├─ Ensure Neutron CR exists & status=Ready
  │   ├─ Ensure Nova CR exists & status=Ready
  │   └─ Ensure Cinder CR exists & status=Ready (if enabled)
  │
  ├─ Phase: Extended Services (all gated by .spec.<service>.enabled)
  │   ├─ Ensure Heat CR exists & status=Ready
  │   ├─ Ensure Swift CR exists & status=Ready
  │   ├─ Ensure Octavia CR exists & status=Ready       (needs: Nova, Neutron, Glance)
  │   ├─ Ensure Designate CR exists & status=Ready     (needs: Neutron)
  │   ├─ Ensure Manila CR exists & status=Ready        (needs: Neutron)
  │   ├─ Ensure Ironic CR exists & status=Ready        (needs: Glance, Neutron)
  │   ├─ Ensure Magnum CR exists & status=Ready        (needs: Nova, Neutron, Heat)
  │   ├─ Ensure Trove CR exists & status=Ready         (needs: Nova, Neutron, Cinder)
  │   ├─ Ensure Ceilometer CR exists & status=Ready
  │   ├─ Ensure Aodh CR exists & status=Ready          (needs: Ceilometer)
  │   ├─ Ensure Masakari CR exists & status=Ready      (needs: Nova)
  │   ├─ Ensure Mistral CR exists & status=Ready
  │   ├─ Ensure Tacker CR exists & status=Ready        (needs: Nova, Neutron, Heat)
  │   ├─ Ensure Cyborg CR exists & status=Ready        (needs: Nova, Placement)
  │   ├─ Ensure Blazar CR exists & status=Ready        (needs: Nova)
  │   ├─ Ensure Zun CR exists & status=Ready           (needs: Neutron)
  │   ├─ Ensure CloudKitty CR exists & status=Ready    (needs: Ceilometer)
  │   ├─ Ensure Watcher CR exists & status=Ready       (needs: Nova, Ceilometer)
  │   ├─ Ensure Vitrage CR exists & status=Ready       (needs: Ceilometer, Aodh)
  │   ├─ Ensure Zaqar CR exists & status=Ready
  │   ├─ Ensure Freezer CR exists & status=Ready       (needs: Swift)
  │   ├─ Ensure Venus CR exists & status=Ready
  │   ├─ Ensure Adjutant CR exists & status=Ready
  │   └─ Ensure Storlets CR exists & status=Ready      (needs: Swift)
  │
  ├─ Phase: Dashboards
  │   ├─ Ensure Horizon CR exists & status=Ready (if enabled)
  │   └─ Ensure Skyline CR exists & status=Ready (if enabled)
  │
  └─ Update OpenStackControlPlane status
      ├─ conditions: [{type: Ready, status: True/False}]
      └─ phase: "Infrastructure" | "Identity" | "CoreServices" | "ExtendedServices" | ...
```

**Extended service dependency resolution**: The top-level controller checks
each extended service's upstream dependencies before creating its CR. For
example, `Octavia` is only created after `Nova`, `Neutron`, and `Glance`
report `Ready`. If a dependency is not enabled, the dependent service is
skipped with a status condition explaining why.

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
- **Manila** → CephFS backend for shared filesystems
- **Swift** → Can use Ceph RGW (RADOS Gateway) as a Swift-compatible object store
- **Trove** → Database instance volumes on Ceph RBD
- **Freezer** → Backup targets can use Swift (backed by Ceph RGW)

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
│       ├── common_types.go              # Shared structs (ServiceTemplate, etc.)
│       ├── openstack_controlplane_types.go
│       │
│       │   # Infrastructure
│       ├── mariadb_types.go
│       ├── rabbitmq_types.go
│       ├── memcached_types.go
│       ├── ceph_storage_types.go
│       ├── ovn_network_types.go
│       ├── openstack_dataplane_types.go
│       │
│       │   # Core Services
│       ├── keystone_types.go
│       ├── glance_types.go
│       ├── placement_types.go
│       ├── neutron_types.go
│       ├── nova_types.go
│       ├── cinder_types.go
│       ├── heat_types.go
│       │
│       │   # Extended — Tier 1 (commonly deployed)
│       ├── swift_types.go
│       ├── barbican_types.go
│       ├── octavia_types.go
│       ├── designate_types.go
│       ├── manila_types.go
│       ├── ironic_types.go
│       ├── magnum_types.go
│       │
│       │   # Extended — Tier 2 (moderately deployed)
│       ├── trove_types.go
│       ├── ceilometer_types.go
│       ├── aodh_types.go
│       ├── masakari_types.go
│       ├── mistral_types.go
│       ├── tacker_types.go
│       │
│       │   # Extended — Tier 3 (niche / specialized)
│       ├── cyborg_types.go
│       ├── blazar_types.go
│       ├── zun_types.go
│       ├── cloudkitty_types.go
│       ├── watcher_types.go
│       ├── vitrage_types.go
│       ├── zaqar_types.go
│       ├── freezer_types.go
│       ├── venus_types.go
│       ├── adjutant_types.go
│       ├── storlets_types.go
│       │
│       │   # Dashboards
│       ├── horizon_types.go
│       ├── skyline_types.go
│       │
│       └── zz_generated.deepcopy.go
├── internal/
│   ├── controller/
│   │   ├── openstack_controlplane_controller.go
│   │   │   # Infrastructure
│   │   ├── mariadb_controller.go
│   │   ├── rabbitmq_controller.go
│   │   ├── memcached_controller.go
│   │   ├── ceph_storage_controller.go
│   │   ├── ovn_network_controller.go
│   │   ├── openstack_dataplane_controller.go
│   │   │   # Core Services
│   │   ├── keystone_controller.go
│   │   ├── glance_controller.go
│   │   ├── placement_controller.go
│   │   ├── neutron_controller.go
│   │   ├── nova_controller.go
│   │   ├── cinder_controller.go
│   │   ├── heat_controller.go
│   │   │   # Extended — Tier 1
│   │   ├── swift_controller.go
│   │   ├── barbican_controller.go
│   │   ├── octavia_controller.go
│   │   ├── designate_controller.go
│   │   ├── manila_controller.go
│   │   ├── ironic_controller.go
│   │   ├── magnum_controller.go
│   │   │   # Extended — Tier 2
│   │   ├── trove_controller.go
│   │   ├── ceilometer_controller.go
│   │   ├── aodh_controller.go
│   │   ├── masakari_controller.go
│   │   ├── mistral_controller.go
│   │   ├── tacker_controller.go
│   │   │   # Extended — Tier 3
│   │   ├── cyborg_controller.go
│   │   ├── blazar_controller.go
│   │   ├── zun_controller.go
│   │   ├── cloudkitty_controller.go
│   │   ├── watcher_controller.go
│   │   ├── vitrage_controller.go
│   │   ├── zaqar_controller.go
│   │   ├── freezer_controller.go
│   │   ├── venus_controller.go
│   │   ├── adjutant_controller.go
│   │   ├── storlets_controller.go
│   │   │   # Dashboards
│   │   ├── horizon_controller.go
│   │   └── skyline_controller.go
│   ├── common/
│   │   ├── conditions.go                # Status condition helpers
│   │   ├── database.go                  # DB creation / migration helpers
│   │   ├── endpoint.go                  # Keystone endpoint registration
│   │   ├── secret.go                    # Secret generation (passwords, keys)
│   │   └── template.go                  # Config file rendering
│   └── images/
│       └── defaults.go                  # Default container images (all services)
├── config/
│   ├── crd/
│   │   └── bases/                       # Generated CRD YAMLs
│   ├── manager/
│   │   └── manager.yaml                 # Operator Deployment
│   ├── rbac/
│   │   └── role.yaml                    # Generated RBAC
│   ├── samples/
│   │   ├── controlplane_minimal.yaml    # Core only
│   │   ├── controlplane_ha.yaml         # Core + HA
│   │   ├── controlplane_ceph.yaml       # Core + Ceph storage
│   │   ├── controlplane_full.yaml       # All services enabled
│   │   └── controlplane_telco.yaml      # Core + NFV stack (Tacker, Ironic, etc.)
│   └── default/
│       └── kustomization.yaml
├── templates/
│   ├── keystone/                        # One directory per service
│   ├── glance/
│   ├── nova/
│   ├── neutron/
│   ├── cinder/
│   ├── swift/
│   ├── barbican/
│   ├── octavia/
│   ├── designate/
│   ├── manila/
│   ├── ironic/
│   ├── magnum/
│   ├── trove/
│   ├── ceilometer/
│   ├── aodh/
│   ├── ...                              # (one per service)
│   └── skyline/
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

### Phase 5 — Extended Services: Tier 1 (Commonly Deployed)

**Goal**: Add the most frequently deployed optional services, covering load
balancing, DNS, object storage, shared filesystems, bare metal, key management,
and container infrastructure.

| Step | Deliverable | Dependencies |
|------|-------------|--------------|
| 5.1 | `Heat` controller (orchestration: API, engine, CloudFormation) | Keystone |
| 5.2 | `Horizon` controller (classic dashboard) | Keystone |
| 5.3 | `Skyline` controller (modern React dashboard) | Keystone |
| 5.4 | `Barbican` controller (key manager: API, worker, keystone-listener) | Keystone |
| 5.5 | `Swift` controller (proxy-server, account/container/object servers, or Ceph RGW) | Keystone |
| 5.6 | `Octavia` controller (API, worker, health-manager, housekeeping + amphora image) | Keystone, Nova, Neutron, Glance; optional Barbican |
| 5.7 | `Designate` controller (API, central, worker, producer, mdns, sink + backend DNS) | Keystone, Neutron |
| 5.8 | `Manila` controller (API, scheduler, share; backends: CephFS, NFS, NetApp) | Keystone, Neutron |
| 5.9 | `Ironic` controller (API, conductor, inspector, DHCP/TFTP/PXE) | Keystone, Glance, Neutron |
| 5.10 | `Magnum` controller (API, conductor + cluster templates) | Keystone, Nova, Neutron, Glance, Heat |
| 5.11 | TLS everywhere (cert-manager integration, Barbican-backed) | Barbican |
| 5.12 | E2E tests: LB creation, DNS record, share mount, bare-metal provision |  |

### Phase 6 — Extended Services: Tier 2 (Moderately Deployed)

**Goal**: Add telemetry, database-as-a-service, instance HA, workflow, and
NFV orchestration.

| Step | Deliverable | Dependencies |
|------|-------------|--------------|
| 6.1 | `Ceilometer` controller (central-agent, notification-agent, compute-agent) | Keystone; meters Nova, Neutron, Cinder, etc. |
| 6.2 | `Aodh` controller (API, evaluator, listener, notifier) | Keystone, Ceilometer |
| 6.3 | `Trove` controller (API, taskmanager, conductor, guestagent image) | Keystone, Nova, Neutron, Cinder, Swift |
| 6.4 | `Masakari` controller (API, engine + masakari-monitors DaemonSet) | Keystone, Nova |
| 6.5 | `Mistral` controller (API, engine, executor, event-engine) | Keystone |
| 6.6 | `Tacker` controller (server, conductor; VNFM + NFVO) | Keystone, Nova, Neutron, Glance, Heat; optional Mistral, Barbican |
| 6.7 | Monitoring integration (Prometheus exporters for all services, Grafana dashboards) | Ceilometer |
| 6.8 | E2E tests: alarm trigger, DB instance create, host-failure recovery, NFV deploy |  |

### Phase 7 — Extended Services: Tier 3 (Niche / Specialized)

**Goal**: Add accelerator management, resource reservation, containers,
billing, optimization, root cause analysis, messaging, backup, logging, and
remaining services.

| Step | Deliverable | Dependencies |
|------|-------------|--------------|
| 7.1 | `Cyborg` controller (API, conductor, agent DaemonSet) | Keystone, Nova, Placement |
| 7.2 | `Blazar` controller (API, manager) | Keystone, Nova |
| 7.3 | `Zun` controller (API, compute, websocket-proxy + Kuryr CNI) | Keystone, Neutron; optional Cinder |
| 7.4 | `CloudKitty` controller (API, processor; hashmap/noop rating) | Keystone, Ceilometer or Prometheus |
| 7.5 | `Watcher` controller (API, decision-engine, applier) | Keystone, Nova, Ceilometer |
| 7.6 | `Vitrage` controller (API, graph, notifier, ML) | Keystone, Ceilometer, Aodh, Nova, Neutron |
| 7.7 | `Zaqar` controller (API, websocket; MongoDB/Redis/Swift backend) | Keystone |
| 7.8 | `Freezer` controller (API, scheduler, agent) | Keystone, Swift |
| 7.9 | `Venus` controller (API, manager + Elasticsearch backend) | Keystone |
| 7.10 | `Adjutant` controller (API + Django task engine) | Keystone |
| 7.11 | `Storlets` controller (proxy middleware, Docker gateway) | Swift |
| 7.12 | Upgrade orchestration (rolling upgrades between OpenStack releases) |  |
| 7.13 | Backup/restore for MariaDB and Ceph (Freezer integration) | Freezer |

### Phase 8 — Advanced Networking & Multi-Site

| Step | Deliverable |
|------|-------------|
| 8.1 | Shared OVN with Kubernetes CNI (Kube-OVN / OVN-Kubernetes) |
| 8.2 | BGP integration for provider networks |
| 8.3 | SR-IOV support for high-performance workloads |
| 8.4 | Multi-site / distributed cloud (OVN-IC, federated Keystone) |
| 8.5 | OLM packaging for OperatorHub distribution |

---

## 10. Extended Service Catalog & Dependency Map

All actively maintained OpenStack services as of the 2025.1 (Epoxy) release.
Services marked **RETIRED** (Sahara, Senlin, Monasca, Murano, Searchlight,
Karbor, Solum, Panko, Gnocchi) are intentionally excluded.

### 10.1 Tier 1 — Commonly Deployed

| Service | Function | Key Dependencies | Sub-Components |
|---------|----------|------------------|----------------|
| **Swift** | Object storage (S3-compatible via middleware) | Keystone; self-contained storage ring | proxy-server, account-server, container-server, object-server |
| **Barbican** | Secret/key/certificate management; HSM integration | Keystone | barbican-api, barbican-worker, barbican-keystone-listener |
| **Octavia** | Load balancing as a service (LBaaS v2) | Keystone, Nova, Neutron, Glance; opt. Barbican | octavia-api, octavia-worker, octavia-health-manager, octavia-housekeeping |
| **Designate** | DNS as a service with Neutron integration | Keystone, Neutron; BIND9 or PowerDNS backend | designate-api, designate-central, designate-worker, designate-producer, designate-mdns, designate-sink |
| **Manila** | Shared filesystem as a service (NFS/CIFS) | Keystone, Neutron; CephFS/NetApp/NFS backend | manila-api, manila-scheduler, manila-share |
| **Ironic** | Bare metal provisioning (physical servers) | Keystone, Glance, Neutron; opt. Nova, Swift | ironic-api, ironic-conductor, ironic-inspector; DHCP/TFTP/PXE |
| **Magnum** | Kubernetes-on-OpenStack cluster management | Keystone, Nova, Neutron, Glance, Heat; opt. Octavia, Cinder, Barbican | magnum-api, magnum-conductor |

### 10.2 Tier 2 — Moderately Deployed

| Service | Function | Key Dependencies | Sub-Components |
|---------|----------|------------------|----------------|
| **Trove** | Database as a service (MySQL, PostgreSQL, Redis, etc.) | Keystone, Nova, Neutron, Cinder, Glance, Swift | trove-api, trove-taskmanager, trove-conductor; guest agent in DB VMs |
| **Ceilometer** | Telemetry data collection (metering/monitoring agent) | Keystone; publishes to Prometheus/Gnocchi | ceilometer-central-agent, ceilometer-notification-agent, ceilometer-compute-agent (DaemonSet) |
| **Aodh** | Threshold-based alarming for auto-scaling | Keystone, Ceilometer | aodh-api, aodh-evaluator, aodh-listener, aodh-notifier |
| **Masakari** | Automated instance recovery on host failure | Keystone, Nova | masakari-api, masakari-engine; masakari-monitors (DaemonSet on compute) |
| **Mistral** | Workflow as a service (YAML DSL) | Keystone | mistral-api, mistral-engine, mistral-executor, mistral-event-engine |
| **Tacker** | NFV orchestration (ETSI MANO: VNFM + NFVO) | Keystone, Nova, Neutron, Glance, Heat; opt. Mistral, Barbican | tacker-server, tacker-conductor |

### 10.3 Tier 3 — Niche / Specialized

| Service | Function | Key Dependencies | Sub-Components |
|---------|----------|------------------|----------------|
| **Cyborg** | GPU/FPGA/accelerator lifecycle management | Keystone, Nova, Placement | cyborg-api, cyborg-conductor, cyborg-agent (DaemonSet) |
| **Blazar** | Advance resource reservation (hosts, instances, GPUs) | Keystone, Nova | blazar-api, blazar-manager |
| **Zun** | Run containers as first-class OpenStack resources | Keystone, Neutron; opt. Cinder; Kuryr for networking | zun-api, zun-compute, zun-wsproxy |
| **CloudKitty** | Rating/billing engine (chargeback/showback) | Keystone, Ceilometer or Prometheus | cloudkitty-api, cloudkitty-processor |
| **Watcher** | Infrastructure audit and optimization | Keystone, Nova, Ceilometer | watcher-api, watcher-decision-engine, watcher-applier |
| **Vitrage** | Root cause analysis via topology graph | Keystone, Ceilometer, Aodh, Nova, Neutron, Cinder | vitrage-api, vitrage-graph, vitrage-notifier, vitrage-ml |
| **Zaqar** | Multi-tenant messaging (queues + pub/sub) | Keystone; MongoDB/Redis/Swift backend | zaqar-server (API + transport) |
| **Freezer** | Backup, restore, and disaster recovery | Keystone, Swift | freezer-api, freezer-scheduler, freezer-agent |
| **Venus** | Centralized log management | Keystone; Elasticsearch backend | venus-api, venus-manager |
| **Adjutant** | Self-service registration and ops automation | Keystone | adjutant-api (Django) |
| **Storlets** | User-defined compute inside Swift object store | Swift; Docker runtime | storlets-proxy-middleware, storlets-docker-gateway |
| **Skyline** | Modern web dashboard (React frontend) | Keystone; all other service APIs | skyline-apiserver, skyline-console |

### 10.4 Extended Service Dependency Graph

```
Keystone ──────────────────────────────────────────────────────────────────┐
   │                                                                       │
   ├── Barbican                                                            │
   │     └── (used by Octavia for TLS, Nova for encrypted volumes)         │
   │                                                                       │
   ├── Swift ──────────────────────────┬───────────────────┐               │
   │     │                             │                   │               │
   │     ├── Freezer                   ├── Storlets        │               │
   │     └── Trove (backups)           └── Zaqar (backend) │               │
   │                                                       │               │
   ├── Glance ─────────────────────────────────────────────┤               │
   │     │                                                 │               │
   │     ├── Ironic (deploy images)                        │               │
   │     └── Octavia (amphora images)                      │               │
   │                                                       │               │
   ├── Neutron ────────────────────────────────────────────┤               │
   │     │                                                 │               │
   │     ├── Designate (auto-DNS for instances)            │               │
   │     ├── Manila (network-connected shares)             │               │
   │     ├── Ironic (PXE/DHCP provisioning)                │               │
   │     ├── Octavia (VIP/member ports)                    │               │
   │     ├── Zun (container networking via Kuryr)          │               │
   │     └── Tacker (VNF networking)                       │               │
   │                                                       │               │
   ├── Nova ───────────────────────────────────────────────┤               │
   │     │                                                 │               │
   │     ├── Octavia (amphora VMs)                         │               │
   │     ├── Trove (DB instance VMs)                       │               │
   │     ├── Magnum (K8s node VMs)                         │               │
   │     ├── Masakari (instance evacuation)                │               │
   │     ├── Blazar (host/instance reservation)            │               │
   │     ├── Watcher (compute optimization)                │               │
   │     ├── Cyborg (accelerator attach)                   │               │
   │     └── Tacker (VNF VMs)                              │               │
   │                                                       │               │
   ├── Placement ──── Cyborg (accelerator resource tracking)               │
   │                                                       │               │
   ├── Cinder ─────── Trove (DB volumes) ─── Zun (container volumes)       │
   │                                                       │               │
   ├── Heat ──────────┬── Magnum (cluster orchestration)   │               │
   │                  └── Tacker (VNF lifecycle)            │               │
   │                                                       │               │
   ├── Ceilometer ────┬── Aodh (alarm data source)         │               │
   │                  ├── CloudKitty (metering data)        │               │
   │                  ├── Watcher (resource metrics)        │               │
   │                  └── Vitrage (topology + alarm data)   │               │
   │                                                       │               │
   ├── Mistral ────── Tacker (workflow execution)           │               │
   │                                                       │               │
   ├── Venus (standalone log collector)                     │               │
   ├── Adjutant (standalone ops automation)                 │               │
   ├── Zaqar (standalone messaging)                        │               │
   │                                                       │               │
   └── Dashboards                                          │               │
         ├── Horizon (classic)                              │               │
         └── Skyline (modern) ─── all service APIs ────────┘───────────────┘
```

---

## 11. Key Technical Decisions

### 11.1 Container Images

Use upstream OpenStack container images from `quay.io/openstack.kolla/` (Kolla-built images) or build custom images. The operator stores default image references in `internal/images/defaults.go` and allows overrides via the CR spec and webhooks.

### 11.2 Configuration Management

- OpenStack service configs (`*.conf`) are rendered from Go templates in `templates/`
- Templates are populated from CR spec fields + generated secrets
- Rendered configs are stored in ConfigMaps, mounted into service pods
- Config changes trigger rolling restarts via annotation hashing

### 11.3 Secret Management

- Database passwords, RabbitMQ credentials, Keystone admin password, and service user
  passwords are auto-generated and stored in Kubernetes Secrets
- Ceph keyrings are either generated (Rook) or provided (external Ceph)
- Fernet keys for Keystone are generated and rotated via a CronJob

### 11.4 Database Lifecycle

Each service controller:
1. Creates a database and user in MariaDB (via a Job or direct SQL)
2. Runs `<service>-manage db_sync` as a Kubernetes Job
3. Waits for Job completion before creating the Deployment
4. On upgrade, runs db_sync again before rolling out new Deployment

### 11.5 Ingress & Service Exposure

- Internal services use ClusterIP Services with internal DNS
- Public API endpoints use an Ingress controller (default: HAProxy Ingress)
- Each public service gets a unique hostname or path-based route
- TLS termination at the Ingress level (cert-manager integration)

---

## 12. Testing Strategy

| Layer | Tool | Scope |
|-------|------|-------|
| Unit | Go `testing` + gomock | Individual functions, config rendering |
| Controller | envtest (controller-runtime) | Controller reconciliation against fake API server |
| Integration | Kind cluster + test suite | Full CRD lifecycle on a real (local) cluster |
| E2E | Kind/real cluster + OpenStack CLI | Deploy full cloud, create VM, attach volume, verify networking |
| Chaos | Litmus / custom scripts | Kill pods, nodes; verify self-healing reconciliation |

---

## 13. Open Questions / Risks

| # | Question | Notes |
|---|----------|-------|
| 1 | **Target OpenStack release?** | Recommend starting with 2025.1 (Epoxy) as it is the latest coordinated release. |
| 2 | **Kolla vs custom images?** | Kolla images are well-maintained and support all 33+ services. Custom images add flexibility but maintenance burden. |
| 3 | **Rook-managed vs external Ceph?** | Supporting both is ideal. Phase 1 can skip Ceph entirely (use PVC/local). |
| 4 | **Compute on K8s nodes or bare metal?** | Phase 1: K8s nodes (nested virt or privileged containers). Phase 4: bare metal via DataPlane CRD. |
| 5 | **Multi-tenancy of the operator itself?** | Single operator instance per cluster, multiple `OpenStackControlPlane` CRs in different namespaces. |
| 6 | **OLM distribution?** | Plan for OperatorHub/OLM packaging in Phase 8. |
| 7 | **Octavia amphora vs OVN provider?** | OVN-native LB is simpler but less feature-rich. Amphora is production-standard but requires managing amphora VM images. Support both as backend options. |
| 8 | **Designate DNS backend?** | BIND9 is simplest to containerize. PowerDNS offers better API. Make it pluggable. |
| 9 | **Ceilometer backend: Gnocchi vs Prometheus?** | Gnocchi is retired from governance. Recommend Prometheus as the default metrics backend, with optional Gnocchi support for legacy. |
| 10 | **Ironic networking model?** | Flat vs tenant networking for bare metal. Flat is simpler for Phase 5; tenant networking adds complexity but isolation. |
| 11 | **Swift native vs Ceph RGW?** | Ceph RGW provides Swift-compatible API with unified storage. Native Swift is more feature-complete but adds operational burden. Support both. |
| 12 | **Tier 3 services worth maintaining?** | Some Tier 3 services (Storlets, Adjutant) have very few contributors upstream. Consider marking them as "community-contributed" with lower support guarantees. |

---

## 14. References

- [openstack-k8s-operators](https://github.com/openstack-k8s-operators) — Red Hat's operator-based OpenStack on K8s
- [OpenStack-Helm](https://docs.openstack.org/openstack-helm/latest/) — Helm chart approach
- [StarlingX](https://www.starlingx.io/) — Production-grade containerized OpenStack
- [Canonical Sunbeam](https://canonical.com/microstack) — Juju charm approach
- [Kubebuilder Book](https://book.kubebuilder.io/) — Operator framework documentation
- [Operator SDK](https://sdk.operatorframework.io/) — Operator development toolkit
- [Rook Ceph](https://rook.io/) — Ceph on Kubernetes
- [OVN Architecture](https://www.ovn.org/en/architecture/) — OVN networking
