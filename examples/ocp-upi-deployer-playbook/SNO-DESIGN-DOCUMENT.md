# Single Node OpenShift (SNO) Design Document
## OpenShift UPI Deployer for IBM Power Systems

**Version:** 1.0  
**Date:** March 30, 2026  
**Author:** OCP UPI Deployer Team

---

## Table of Contents

1. [Executive Summary](#executive-summary)
2. [Architecture Overview](#architecture-overview)
3. [SNO vs Multi-Node Comparison](#sno-vs-multi-node-comparison)
4. [Deployment Workflow](#deployment-workflow)
5. [Component Design](#component-design)
6. [Network Architecture](#network-architecture)
7. [Storage Architecture](#storage-architecture)
8. [Installation Process](#installation-process)
9. [Configuration Management](#configuration-management)
10. [State Management](#state-management)
11. [Error Handling & Recovery](#error-handling--recovery)
12. [Security Considerations](#security-considerations)
13. [Performance & Scalability](#performance--scalability)
14. [Monitoring & Observability](#monitoring--observability)
15. [Future Enhancements](#future-enhancements)

---

## 1. Executive Summary

### 1.1 Purpose

This document describes the design and implementation of Single Node OpenShift (SNO) deployment capability for IBM Power Systems using the OCP UPI Deployer. SNO provides a minimal OpenShift deployment suitable for edge computing, development environments, and resource-constrained scenarios.

### 1.2 Key Features

- **Minimal Footprint**: Single node acts as both control plane and worker
- **Simplified Deployment**: Reduced complexity with 5-phase deployment process
- **Edge-Ready**: Optimized for edge computing and remote locations
- **Resource Efficient**: Lower resource requirements compared to multi-node clusters
- **Automated Setup**: End-to-end automation from LPAR creation to cluster ready state

### 1.3 Use Cases

| Use Case | Description | Benefits |
|----------|-------------|----------|
| **Edge Computing** | Deploy OpenShift at remote edge locations | Minimal hardware, reduced operational complexity |
| **Development/Testing** | Create isolated development environments | Fast provisioning, easy cleanup |
| **Proof of Concept** | Demonstrate OpenShift capabilities | Quick setup, minimal resources |
| **Resource-Constrained** | Deploy in environments with limited resources | Single system deployment, lower costs |
| **Remote Sites** | Deploy at branch offices or remote facilities | Simplified management, reduced footprint |

---

## 2. Architecture Overview

### 2.1 High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                    Hardware Management Console (HMC)             │
│                         192.0.2.1                             │
└────────────────────────────┬────────────────────────────────────┘
                             │
                             │ REST API
                             │
┌────────────────────────────▼────────────────────────────────────┐
│                    IBM Power System (LTC09U31-ZZ)                │
│                                                                   │
│  ┌─────────────────────┐              ┌─────────────────────┐  │
│  │   VIOS (ltc09u31)   │              │   Virtual Network   │  │
│  │   - Volume Groups   │◄────────────►│   ETHERNET0         │  │
│  │   - Virtual Disks   │              │   VLAN 1337         │  │
│  └─────────────────────┘              └──────────┬──────────┘  │
│                                                   │              │
│  ┌───────────────────────────────────────────────┼──────────┐  │
│  │                    LPARs                       │          │  │
│  │                                                │          │  │
│  │  ┌──────────────────────────┐    ┌───────────▼────────┐ │  │
│  │  │   Helper/Bastion Node    │    │   SNO Master Node  │ │  │
│  │  │   192.0.2.10          │    │   198.51.100.16     │ │  │
│  │  │                          │    │                    │ │  │
│  │  │  Services:               │    │  Roles:            │ │  │
│  │  │  - dnsmasq (DNS/DHCP)   │    │  - Control Plane   │ │  │
│  │  │  - HTTP Server          │    │  - Worker Node     │ │  │
│  │  │  - TFTP Server          │    │  - etcd            │ │  │
│  │  │  - NFS (optional)       │    │  - API Server      │ │  │
│  │  │                          │    │  - Scheduler       │ │  │
│  │  │  Storage:                │    │  - Workloads       │ │  │
│  │  │  - 120GB Boot            │    │                    │ │  │
│  │  │  - 200GB Data            │    │  Storage:          │ │  │
│  │  └──────────────────────────┘    │  - 120GB Boot      │ │  │
│  │                                   │  - 100GB etcd      │ │  │
│  │                                   │  - 300GB Container │ │  │
│  │                                   └────────────────────┘ │  │
│  └───────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
```

### 2.2 Component Relationships

```
┌─────────────────────────────────────────────────────────────────┐
│                      OCP UPI Deployer                            │
│                                                                   │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────┐      │
│  │ Configuration│───►│  Validator   │───►│ Orchestrator │      │
│  │   Loader     │    │              │    │              │      │
│  └──────────────┘    └──────────────┘    └──────┬───────┘      │
│                                                   │              │
│  ┌────────────────────────────────────────────────┼──────────┐  │
│  │              Deployment Phases                 │          │  │
│  │                                                 │          │  │
│  │  Phase 1: validate_config ─────────────────────┤          │  │
│  │  Phase 2: check_resources ─────────────────────┤          │  │
│  │  Phase 3: create_lpars ────────────────────────┤          │  │
│  │  Phase 4: setup_helper_node ───────────────────┤          │  │
│  │  Phase 5: run_playbook ────────────────────────┘          │  │
│  └─────────────────────────────────────────────────────────────┘  │
│                                                                   │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────┐      │
│  │  HMC Client  │    │  VIOS Client │    │  SSH Client  │      │
│  └──────┬───────┘    └──────┬───────┘    └──────┬───────┘      │
└─────────┼────────────────────┼────────────────────┼──────────────┘
          │                    │                    │
          ▼                    ▼                    ▼
     HMC REST API         VIOS Commands        Helper Node
```

### 2.3 Technology Stack

| Layer | Technology | Purpose |
|-------|-----------|---------|
| **Orchestration** | Go 1.21+ | Main deployment logic |
| **HMC Integration** | powerhmc-go SDK | LPAR lifecycle management |
| **Storage Integration** | svc-go-sdk | SVC volume management |
| **Configuration** | YAML | Declarative configuration |
| **State Management** | JSON | Deployment state persistence |
| **Remote Execution** | SSH (golang.org/x/crypto/ssh) | Helper node setup |
| **Helper Setup** | Ansible Playbook | Service configuration |
| **Container Platform** | OpenShift 4.21+ | Kubernetes distribution |
| **Operating System** | RHCOS (Red Hat CoreOS) | Immutable OS for OpenShift |

---

## 3. SNO vs Multi-Node Comparison

### 3.1 Architecture Differences

| Aspect | SNO | Multi-Node |
|--------|-----|------------|
| **Control Plane** | 1 node (combined) | 3 nodes (dedicated) |
| **Worker Nodes** | 0 (master is schedulable) | 2+ nodes (dedicated) |
| **Bootstrap Node** | Not used | Required during install |
| **etcd Cluster** | Single instance | 3-node cluster |
| **High Availability** | None | Full HA |
| **Minimum Nodes** | 2 (helper + master) | 6 (helper + bootstrap + 3 masters + 2 workers) |

### 3.2 Resource Comparison

| Resource | SNO | Multi-Node (Minimal) |
|----------|-----|---------------------|
| **Total CPU** | 4.5 units | 13.5 units |
| **Total Memory** | 72 GB | 208 GB |
| **Total Storage** | 640 GB | 1,640 GB |
| **Network Ports** | 2 | 6 |
| **Power Systems** | 1 | 1-2 (for HA) |

### 3.3 Deployment Time Comparison

| Phase | SNO | Multi-Node |
|-------|-----|------------|
| **LPAR Creation** | 5-10 min | 15-30 min |
| **Storage Provisioning** | 5 min | 15 min |
| **Helper Setup** | 15-20 min | 15-20 min |
| **Cluster Installation** | 60-90 min | 45-60 min |
| **Total Time** | ~90-120 min | ~90-125 min |

### 3.4 Use Case Suitability

```
┌─────────────────────────────────────────────────────────────────┐
│                    Use Case Suitability Matrix                   │
├─────────────────────────┬──────────────┬────────────────────────┤
│ Use Case                │ SNO          │ Multi-Node             │
├─────────────────────────┼──────────────┼────────────────────────┤
│ Production Workloads    │ ⚠️  Limited  │ ✅ Recommended         │
│ Edge Computing          │ ✅ Ideal     │ ⚠️  Over-provisioned   │
│ Development/Testing     │ ✅ Ideal     │ ⚠️  Resource intensive │
│ High Availability       │ ❌ Not       │ ✅ Full HA             │
│ Resource Constrained    │ ✅ Ideal     │ ❌ High requirements   │
│ Remote Sites            │ ✅ Ideal     │ ⚠️  Complex            │
│ Proof of Concept        │ ✅ Ideal     │ ⚠️  Overkill           │
│ Mission Critical        │ ❌ Not       │ ✅ Recommended         │
└─────────────────────────┴──────────────┴────────────────────────┘
```

---

## 4. Deployment Workflow

### 4.1 Simplified 5-Phase Workflow

```
┌─────────────────────────────────────────────────────────────────┐
│                    SNO Deployment Workflow                       │
└─────────────────────────────────────────────────────────────────┘

Phase 1: validate_config (~1 min)
├─ Load YAML configuration
├─ Validate syntax and required fields
├─ Check file references (pull-secret, SSH keys)
├─ Validate network configuration
└─ Verify SNO-specific settings
    ↓
Phase 2: check_resources (~2-3 min)
├─ Connect to HMC
├─ Verify Power system availability
├─ Check available CPU/Memory/Storage
├─ Validate VIOS configuration
├─ Verify network (vswitch, VLAN)
└─ Confirm sufficient resources
    ↓
Phase 3: create_lpars (~10-15 min)
├─ Create Helper Node LPAR
│  ├─ Allocate CPU (0.5 units)
│  ├─ Allocate Memory (8 GB)
│  ├─ Create boot disk (120 GB)
│  ├─ Create data disk (200 GB)
│  ├─ Attach network adapter
│  └─ Power on LPAR
├─ Create SNO Master LPAR
│  ├─ Allocate CPU (4.0 units)
│  ├─ Allocate Memory (64 GB)
│  ├─ Create boot disk (120 GB)
│  ├─ Create etcd disk (100 GB)
│  ├─ Create container disk (300 GB)
│  ├─ Attach network adapter
│  └─ Keep powered off (for netboot)
└─ Save state
    ↓
Phase 4: setup_helper_node (~20-30 min)
├─ Wait for Helper Node to boot
├─ Verify SSH connectivity
├─ Copy setup script to helper
├─ Execute setup-bastion.sh
│  ├─ Install required packages
│  ├─ Configure dnsmasq (DNS/DHCP)
│  ├─ Configure TFTP server
│  ├─ Configure HTTP server
│  ├─ Download RHCOS images
│  ├─ Download OpenShift client/installer
│  └─ Configure PXE boot files
└─ Verify services are running
    ↓
Phase 5: run_playbook (~60-90 min)
├─ Generate Ansible inventory
├─ Generate ansible-vars.yaml
├─ Copy files to helper node
├─ Execute Ansible playbook
│  ├─ Generate install-config.yaml
│  ├─ Create single-node ignition config
│  ├─ Configure GRUB for netboot
│  ├─ Setup HTTP serving of ignition
│  └─ Trigger SNO master netboot (via HMC)
├─ Monitor installation progress
│  ├─ Wait for API server (30-45 min)
│  ├─ Wait for cluster operators (15-30 min)
│  └─ Wait for cluster ready (5-15 min)
└─ Save kubeconfig
    ↓
✅ Deployment Complete
```

---

## 5. Component Design

### 5.1 Configuration Structure

The SNO configuration is defined in YAML format with the following key sections:

```yaml
# SNO Configuration Structure
hmc:                    # HMC connection details
power_systems:          # Power system specifications
storage:                # Storage backend (VIOS/SVC)
network:                # Network configuration
openshift:              # OpenShift cluster settings
helper_node:            # Helper/bastion node config
sno_node:               # SNO master node config
ansible:                # Ansible playbook settings
deployment:             # Deployment phases and timeouts
advanced:               # Advanced options
```

### 5.2 Key Configuration Differences for SNO

**SNO-Specific Settings**:
```yaml
openshift:
  install_type: "sno"              # Must be "sno"
  
advanced:
  sno_mode: true                   # Enable SNO-specific logic
  skip_bootstrap: true             # Skip bootstrap node creation
  skip_workers: true               # Skip worker node creation
  master_schedulable: true         # Allow workloads on master
```

**Node Configuration**:
```yaml
# Only helper_node and sno_node are defined
# bootstrap, masters, and workers sections are omitted

helper_node:
  hostname: "helper.sno.example.com"
  ip: "192.0.2.10"
  # ... other settings

sno_node:
  name: "sno-master"
  hostname: "sno-master.sno.example.com"
  ip: "198.51.100.16"
  lpar:
    processor:
      units: 4.0                   # Enhanced CPU for combined role
    memory:
      desired_mb: 65536            # 64GB for control plane + workloads
    storage:
      boot_disk_gb: 120
      etcd_disk_gb: 100
      container_storage_gb: 300
```

---

## 6. Network Architecture

### 6.1 Network Topology

```
External Network (10.0.10.0/24)
    │
    │ Gateway: 10.0.10.1
    │ DNS: 10.0.10.4, 10.0.10.5
    │ NTP: 10.0.10.4, 10.0.10.5
    │
    ▼
┌───────────────────────────────────────────────────────────────┐
│  Power System Virtual Network (VLAN 1337)                     │
│  Network: 192.0.2.0/20                                      │
│  Gateway: 192.0.2.254                                         │
│  Netmask: 255.255.240.0                                       │
│  Broadcast: 203.0.113.255                                     │
│                                                                │
│  ┌──────────────────────────────────────────────────────────┐ │
│  │  Helper Node (192.0.2.10)                             │ │
│  │  - DNS Server (port 53)                                  │ │
│  │  - DHCP Server (range: 198.51.100.11-254)                │ │
│  │  - TFTP Server (port 69)                                 │ │
│  │  - HTTP Server (port 8080)                               │ │
│  │  - NFS Server (port 2049) [optional]                     │ │
│  └──────────────────────────────────────────────────────────┘ │
│                                                                │
│  ┌──────────────────────────────────────────────────────────┐ │
│  │  SNO Master (198.51.100.16)                               │ │
│  │  - API Server (port 6443)                                │ │
│  │  - Ingress Router (ports 80, 443)                        │ │
│  │  - etcd (ports 2379, 2380)                               │ │
│  │  - Kubelet (port 10250)                                  │ │
│  └──────────────────────────────────────────────────────────┘ │
└───────────────────────────────────────────────────────────────┘
```

### 6.2 DNS Configuration

**Forward Zone (sno.example.com)**:
```
api.sno.example.com.              IN  A  198.51.100.16
api-int.sno.example.com.          IN  A  198.51.100.16
*.apps.sno.example.com.           IN  A  198.51.100.16
sno-master.sno.example.com.       IN  A  198.51.100.16
helper.sno.example.com.           IN  A  192.0.2.10
```

**Reverse Zone**:
```
16.189.20.10.in-addr.arpa.        IN  PTR  sno-master.sno.example.com.
230.181.20.10.in-addr.arpa.       IN  PTR  helper.sno.example.com.
```

---

## 7. Storage Architecture

### 7.1 Storage Layout

```
VIOS Volume Group: auto_vg01
├─ Helper Node Storage (320 GB)
│  ├─ helper_boot (120 GB) → /dev/sda
│  └─ helper_data (200 GB) → /dev/sdb
│
└─ SNO Master Storage (520 GB)
   ├─ sno_master_boot (120 GB) → /dev/sda (RHCOS root)
   ├─ sno_master_etcd (100 GB) → /dev/sdb (etcd data)
   └─ sno_master_container (300 GB) → /dev/sdc (container storage)
```

### 7.2 Storage Requirements

| Node Type | Boot Disk | Additional Disks | Total |
|-----------|-----------|------------------|-------|
| **Helper** | 120 GB | 200 GB (data) | 320 GB |
| **SNO Master** | 120 GB | 100 GB (etcd) + 300 GB (container) | 520 GB |
| **Total** | 240 GB | 600 GB | **840 GB** |

---

## 8. Installation Process

### 8.1 Installation Timeline

```
T+0:00  - SNO master netboots via HMC
T+0:05  - RHCOS kernel loads from TFTP
T+0:10  - Ignition applies configuration
T+0:15  - System reboots with RHCOS installed
T+0:20  - OpenShift services start
T+0:30  - API server becomes available
T+0:45  - Cluster operators deploying
T+1:00  - Most operators ready
T+1:15  - Cluster stabilizing
T+1:30  - Installation complete ✅
```

### 8.2 Ignition Configuration

SNO uses a single ignition config that combines control plane and worker configurations:

```yaml
# install-config.yaml for SNO
apiVersion: v1
baseDomain: example.com
metadata:
  name: sno
compute:
- name: worker
  replicas: 0              # No separate workers
controlPlane:
  name: master
  replicas: 1              # Single master node
  architecture: ppc64le
networking:
  clusterNetwork:
  - cidr: 10.128.0.0/14
    hostPrefix: 23
  serviceNetwork:
  - 172.30.0.0/16
  machineNetwork:
  - cidr: 192.0.2.0/20
platform:
  none: {}
pullSecret: '<content>'
sshKey: '<content>'
```

---

## 9. State Management

### 9.1 Deployment State Structure

```json
{
  "current_phase": "run_playbook",
  "completed_phases": [
    "validate_config",
    "check_resources",
    "create_lpars",
    "setup_helper_node"
  ],
  "created_lpars": {
    "helper": {
      "uuid": "abc-123-def",
      "name": "helper.sno.example.com",
      "ip": "192.0.2.10",
      "status": "powered_on"
    },
    "sno_master": {
      "uuid": "xyz-789-ghi",
      "name": "sno-master.sno.example.com",
      "ip": "198.51.100.16",
      "status": "installing"
    }
  },
  "created_volumes": [
    "helper_boot",
    "helper_data",
    "sno_master_boot",
    "sno_master_etcd",
    "sno_master_container"
  ],
  "status": "in_progress",
  "started_at": "2026-03-30T08:00:00Z",
  "last_updated": "2026-03-30T09:15:00Z"
}
```

---

## 10. Error Handling & Recovery

### 10.1 Resume Capability

The deployer supports resuming from any phase:

```bash
# Resume from a specific phase
./ocp-upi-deployer --config config-sno.yaml --resume create_lpars
```

### 10.2 Common Failure Scenarios

| Failure Point | Recovery Action |
|---------------|-----------------|
| **LPAR Creation Failed** | Cleanup partial resources, retry with `--resume create_lpars` |
| **Helper Setup Failed** | Fix helper node issues, retry with `--resume setup_helper_node` |
| **Installation Timeout** | Check logs, extend timeout, retry with `--resume run_playbook` |
| **Network Issues** | Verify network config, fix issues, resume from last phase |

---

## 11. Security Considerations

### 11.1 Security Best Practices

- **SSH Keys**: Use strong SSH keys for authentication
- **Pull Secret**: Protect Red Hat pull secret
- **Network Isolation**: Use VLANs for network isolation
- **Firewall Rules**: Configure appropriate firewall rules
- **RBAC**: Implement proper RBAC in OpenShift
- **Secrets Management**: Use OpenShift secrets for sensitive data

---

## 12. Performance & Scalability

### 12.1 Resource Allocation

**SNO Master Node**:
- **CPU**: 4.0 processing units (16 virtual processors)
- **Memory**: 64 GB RAM
- **Storage**: 520 GB total (boot + etcd + container)

**Performance Characteristics**:
- Suitable for 10-20 lightweight applications
- Can handle moderate workloads
- Not suitable for high-availability requirements

---

## 13. Monitoring & Observability

### 13.1 Monitoring Points

- LPAR creation status
- Storage provisioning progress
- Helper node service status
- Installation progress
- Cluster operator status
- Node health

### 13.2 Logging

All deployment activities are logged with timestamps and status indicators:
```
2026-03-30 08:00:00 [INFO] 🚀 Starting phase: create_lpars
2026-03-30 08:05:00 [INFO] ✅ Created LPAR: helper.sno.example.com
2026-03-30 08:10:00 [INFO] ✅ Created LPAR: sno-master.sno.example.com
2026-03-30 08:15:00 [INFO] ✅ Phase completed: create_lpars
```

---

## 14. Future Enhancements

### 14.1 Planned Features

1. **Automated Upgrades**: Support for automated OpenShift upgrades
2. **Backup/Restore**: Automated backup and restore capabilities
3. **Multi-SNO Management**: Manage multiple SNO clusters
4. **Monitoring Integration**: Integration with Prometheus/Grafana
5. **GitOps Support**: Integration with ArgoCD/Flux
6. **Day-2 Operations**: Automated day-2 operations support

### 14.2 Potential Improvements

- Parallel LPAR creation for faster deployment
- Enhanced error recovery mechanisms
- Web-based UI for deployment management
- Integration with CI/CD pipelines
- Support for disconnected installations

---

## 15. Conclusion

The SNO deployment capability provides a streamlined, automated approach to deploying Single Node OpenShift on IBM Power Systems. With its simplified 5-phase workflow, comprehensive error handling, and state management, it enables rapid deployment of edge-ready OpenShift clusters with minimal resource requirements.

For more information, refer to:
- [README.md](README.md) - General usage and setup
- [config-sno.yaml](config-sno.yaml) - SNO configuration example
- [OpenShift Documentation](https://docs.openshift.com/)
- [IBM Power Systems Documentation](https://www.ibm.com/docs/en/power10)

---

**Document Version History**:
- v1.0 (2026-03-30): Initial design document