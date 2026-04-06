# OCP UPI Deployer - Design Document

## Table of Contents
1. [Overview](#overview)
2. [Recent Improvements](#recent-improvements)
3. [Single VIP Architecture](#single-vip-architecture)
4. [Multi-Cluster Support](#multi-cluster-support)
5. [Complete Workflow](#complete-workflow)
6. [Configuration Structure](#configuration-structure)
7. [Implementation Modules](#implementation-modules)
8. [Dependencies](#dependencies)

## Overview

This tool deploys multiple OpenShift clusters (SNO or Multi-Node) on IBM Power Systems by directly configuring services on a helper/bastion node. It replaces the Ansible playbook approach with native Go implementation, providing better control, error handling, and multi-cluster support.

### Key Features

- ✅ **Single VIP Architecture** - One IP per cluster instead of two (50% IP savings)
- ✅ **Multi-Cluster Support** - Deploy multiple clusters from single helper node
- ✅ **Native Go Implementation** - No Ansible dependencies
- ✅ **Phase-Based Deployment** - Granular control with 10 distinct phases
- ✅ **Production Ready** - Tested on real IBM Power Systems infrastructure
- ✅ **Automatic MAC Capture** - Services configured after LPAR creation
- ✅ **Persistent Configuration** - IP aliases survive reboots
- ✅ **Simplified Configuration** - Single cluster name used throughout (no redundant fields)
- ✅ **State Management** - RHCOS filenames tracked in deployment state
- ✅ **Auto-Population** - Sensible defaults for optional fields

## Recent Improvements

### Cluster Directory Structure (2026-04-06)

**Feature**: Implemented a cluster directory structure that allows a single binary to manage multiple OpenShift clusters with isolated state and configuration files.

**Benefits**:
- **Single Binary Management**: One binary manages unlimited clusters
- **Better Organization**: Each cluster has its own isolated directory under `clusters/<cluster-name>/`
- **Automatic Config Preservation**: Config file copied to cluster directory during deployment
- **State Isolation**: Each cluster's state tracked in its own `state.json` file
- **List Command**: New command to view all managed clusters and their status
- **Optional Cleanup**: Delete command asks before removing cluster directory

**Directory Structure**:
```
./
├── ocp-upi-deployer              # Single binary
├── clusters/                     # Root directory for all clusters
│   ├── cluster-1/
│   │   ├── config.yaml          # Copy of config used
│   │   └── state.json           # Deployment state
│   ├── cluster-2/
│   │   ├── config.yaml
│   │   └── state.json
│   └── sno-prod/
│       ├── config.yaml
│       └── state.json
```

**New Commands**:
```bash
# List all managed clusters
./ocp-upi-deployer -command list

# Output shows cluster name, status, current phase, and last update time
```

**Implementation Details**:
- Added helper functions in `main.go` for cluster directory management
- Updated `orchestrator.go` to use cluster-specific state file paths
- Updated `lpar.go` to save state to cluster directory
- Deploy command creates cluster directory and copies config file
- Delete command optionally removes cluster directory after cleanup
- List command scans cluster directories and displays status table

See [`CLUSTER_DIRECTORY_IMPLEMENTATION.md`](CLUSTER_DIRECTORY_IMPLEMENTATION.md) for complete details.

### Dnsmasq Configuration Refactoring (2026-04-06)

**Feature**: Split monolithic dnsmasq configuration into three separate, granular phases for better modularity and debugging.

**Changes**:
- **Removed**: Single `setup_dnsmasq` phase
- **Added**: Three new phases:
  - `setup_dns` - Configure DNS A records and etcd SRV records
  - `setup_dhcp` - Configure DHCP with MAC-to-IP bindings
  - `setup_pxe` - Configure PXE/TFTP boot settings

**Configuration Files**:
```
/etc/dnsmasq.d/10-<cluster>-dns.conf    # DNS records
/etc/dnsmasq.d/20-<cluster>-dhcp.conf   # DHCP configuration
/etc/dnsmasq.d/30-<cluster>-pxe.conf    # PXE boot settings
```

**Benefits**:
- **Modularity**: Each networking component independently configurable
- **Better Debugging**: Issues isolated to specific components
- **State Tracking**: Each phase saves state independently
- **Configuration Order**: Numbered prefixes ensure correct load order
- **Flexibility**: Phases can be executed, skipped, or re-run independently

**SNO Support**: DNS template automatically skips etcd SRV records for Single Node OpenShift deployments.

See [`DNSMASQ_REFACTORING.md`](DNSMASQ_REFACTORING.md) for complete details.

### Network Adapter Verification Fix (2026-04-03)

**Problem**: Network boot was failing with "No network adapters found" error:
```
[HMC] Job status: FAILED_BEFORE_COMPLETION
lpar_netboot : No network adapters found
lpar_netboot: Unable to obtain network adapter information. Quitting.
```

**Root Cause**:
The `networkBootLPAR()` function was using undefined variables (`systemUUID`, `lparUUID`) instead of values stored in `LPARState`, causing compilation errors and preventing proper network adapter verification.

**Solution**: Enhanced state management and network adapter verification:

1. **Added SystemUUID to LPARState**: Store both system name and UUID for API calls
   ```go
   type LPARState struct {
       Name       string `json:"name"`
       UUID       string `json:"uuid"`
       SystemName string `json:"system_name"`
       SystemUUID string `json:"system_uuid"`  // NEW
       // ... other fields
   }
   ```

2. **Store SystemUUID During Creation**: Capture and store during LPAR provisioning
   ```go
   l.ctx.State.CreatedLPARs[node.Hostname] = LPARState{
       UUID:       lparUUID,
       SystemName: node.SystemName,
       SystemUUID: systemUUID,  // Store for later use
       // ... other fields
   }
   ```

3. **Verify Network Adapter Before Netboot**: Check adapter exists with proper error handling
   ```go
   // Use stored SystemUUID and UUID from LPARState
   adapters, err := l.hmcClient.GetClientNetworkAdapters(
       lparState.SystemUUID,
       lparState.UUID,
       l.ctx.Verbose
   )
   
   if len(adapters) == 0 {
       return fmt.Errorf("LPAR has no network adapters attached\n" +
           "Solution: Delete LPAR, remove state file, re-run create_lpars phase")
   }
   ```

**Benefits**:
- ✅ Proper error detection when network adapter is missing
- ✅ Clear error messages with recovery steps
- ✅ Prevents confusing netboot failures
- ✅ Validates state before attempting network boot

**Related**: See [`NETBOOT_FIX.md`](NETBOOT_FIX.md) for detailed technical documentation.

### Network Adapter Location Code Fix (2026-04-03)

**Problem**: Network boot was failing with location code mismatch errors:
```
lpar_netboot: can not find physical location U9105.22A.789C301-V10-C2-T1.
actual location is U9105.22A.789C301-V10-C2-T0
```

**Root Cause**:
1. HMC API returns location codes WITHOUT port suffix (e.g., `U9105.22A.789C301-V10-C2`)
2. Netboot command requires FULL location code WITH port suffix (e.g., `U9105.22A.789C301-V10-C2-T0`)
3. Different LPARs use different suffixes (`-T0` or `-T1`) depending on system configuration
4. Previous code hardcoded `-T1` suffix, causing failures on `-T0` systems

**Solution**: Implemented automatic location code detection with try-retry logic:

1. **Fetch from HMC**: Get base location code using `GetClientNetworkAdapters()` API
2. **Try -T0 First**: Attempt netboot with `-T0` suffix (most common)
3. **Retry with -T1**: If location error occurs, automatically retry with `-T1`
4. **Cache Result**: Store working suffix in deployment state for future boots

**Implementation**:
```go
// Fetch base location code from HMC
adapters, _ := hmcClient.GetClientNetworkAdapters(systemUUID, lparUUID, verbose)
baseLocationCode := adapters[0].LocationCode  // e.g., "U9105.22A.789C301-V10-C2"

// Try -T0 first, then -T1 if needed
terminals := []string{"-T0", "-T1"}
for _, t := range terminals {
    exactLocation := baseLocationCode + t
    status, err := hmcClient.PowerOnPartition(lparUUID, options, verbose)
    if err == nil {
        break  // Success!
    }
}
```

**Benefits**:
- ✅ Works automatically for all systems
- ✅ No manual configuration required
- ✅ Self-correcting on first boot
- ✅ Minimal performance impact

**Test Utility**: Created [`test-location-code`](../test-location-code/) utility to verify location codes before deployment.

### Configuration Simplification (2026-04-02)

**Problem**: The configuration had redundant naming fields that caused user confusion:
- `clusters[].name` - Deployment identifier
- `clusters[].cluster_config.network.cluster_name` - OpenShift cluster name
- `sno_node.name` and `sno_node.hostname` - Separate fields for similar purposes

**Solution**: Consolidated to use single naming convention throughout:

#### 1. Single Cluster Name
- **Removed**: `network.cluster_name` field
- **Now**: `clusters[].name` is used for everything:
  - Deployment isolation (file/directory naming)
  - DNS records (api.{name}.{base_domain})
  - OpenShift cluster naming
  - Default node hostname (for SNO)

**Before**:
```yaml
clusters:
  - name: "sno-refa"              # Deployment ID
    cluster_config:
      network:
        cluster_name: "sno"       # OpenShift cluster name (redundant!)
```

**After**:
```yaml
clusters:
  - name: "sno-refa"              # Single name for everything
    cluster_config:
      network:
        # cluster_name removed - derived from name above
        base_domain: "example.com"
```

#### 2. Auto-Population for SNO Nodes
- **Made Optional**: `sno_node.name` and `sno_node.hostname`
- **Auto-populated**:
  - `hostname` defaults to cluster name
  - `name` defaults to `{hostname}-master`

**Before**:
```yaml
sno_node:
  name: "sno-master"              # Required
  hostname: "sno-refa"            # Required
  ip: "198.51.100.17"
```

**After**:
```yaml
sno_node:
  # name and hostname are optional - auto-populated from cluster name
  ip: "198.51.100.17"
```

#### 3. RHCOS File State Management

**Problem**: PXE boot manager was looking for hardcoded filenames that didn't match downloaded files.

**Solution**: Implemented proper state management:

1. **Downloader** stores filenames in deployment state:
```go
d.ctx.State.ServiceEndpoints.RHCOSFiles = RHCOSFiles{
    Kernel:    "kernel",
    Initramfs: "initramfs.img",
    Rootfs:    "rootfs.img",
}
```

2. **PXE Boot Manager** reads filenames from state:
```go
rhcosFiles := p.ctx.State.ServiceEndpoints.RHCOSFiles
kernelSrc := rhcosFiles.Kernel
initramfsSrc := rhcosFiles.Initramfs
```

**Benefits**:
- ✅ Single source of truth for filenames
- ✅ Flexible - can handle different naming conventions
- ✅ Better error messages showing expected filenames
- ✅ Proper architectural separation of concerns

### Impact

| Aspect | Before | After | Improvement |
|--------|--------|-------|-------------|
| Configuration fields | 3 name fields | 1 name field | 67% reduction |
| User confusion | High (which name to use?) | Low (one name) | Simplified |
| SNO node config | 2 required fields | 0 required fields | Auto-populated |
| RHCOS file handling | Hardcoded names | State-managed | Flexible |
| Maintainability | Multiple sources of truth | Single source | Easier |

## Single VIP Architecture

### Why Single VIP?

**Traditional Approach (Dual VIP):**
```
Cluster needs 2 IPs:
- API VIP: 192.0.2.100 (for API and Machine Config Server)
- Ingress VIP: 192.0.2.101 (for HTTP/HTTPS ingress)
```

**New Approach (Single VIP):**
```
Cluster needs 1 IP:
- VIP: 192.0.2.100 (for ALL services via port-based routing)
```

### Benefits

| Aspect | Dual VIP | Single VIP | Improvement |
|--------|----------|------------|-------------|
| IPs per cluster | 2 | 1 | 50% reduction |
| Configuration complexity | Higher | Lower | Simplified |
| IP pool utilization | 50 clusters/100 IPs | 100 clusters/100 IPs | 2x capacity |
| DNS records | Split between VIPs | All point to one VIP | Easier management |
| Troubleshooting | Multiple entry points | Single entry point | Simpler debugging |

### Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│                   Single VIP: 192.0.2.100                      │
│                   (One IP for ALL services)                      │
└────────────────────────────┬────────────────────────────────────┘
                             │
                    ┌────────┴────────┐
                    │    HAProxy      │
                    │ (Port Routing)  │
                    └────────┬────────┘
                             │
        ┌────────────────────┼────────────────────┐
        │                    │                    │
   Port 6443            Port 22623           Port 80/443
   (API Server)     (Machine Config)        (Ingress)
        │                    │                    │
        ▼                    ▼                    ▼
  ┌──────────┐        ┌──────────┐        ┌──────────┐
  │ Master   │        │ Master   │        │ Worker   │
  │ Nodes    │        │ Nodes    │        │ Nodes    │
  └──────────┘        └──────────┘        └──────────┘
```

### DNS Configuration

All DNS records point to the single VIP:

```
api.cluster.example.com         → 192.0.2.100:6443
api-int.cluster.example.com     → 192.0.2.100:6443
*.apps.cluster.example.com      → 192.0.2.100:80/443
```

### HAProxy Configuration

HAProxy handles port-based routing:

```conf
# API Server (Port 6443)
frontend cluster-api
    bind 192.0.2.100:6443
    default_backend cluster-api
    mode tcp

backend cluster-api
    mode tcp
    server master1 198.51.100.10:6443 check
    server master2 198.51.100.11:6443 check
    server master3 198.51.100.12:6443 check

# Machine Config Server (Port 22623)
frontend cluster-mcs
    bind 192.0.2.100:22623
    default_backend cluster-mcs
    mode tcp

backend cluster-mcs
    mode tcp
    server master1 198.51.100.10:22623 check
    server master2 198.51.100.11:22623 check
    server master3 198.51.100.12:22623 check

# Ingress HTTP (Port 80)
frontend cluster-http
    bind 192.0.2.100:80
    default_backend cluster-http
    mode tcp

backend cluster-http
    mode tcp
    server worker1 198.51.100.20:80 check
    server worker2 198.51.100.21:80 check

# Ingress HTTPS (Port 443)
frontend cluster-https
    bind 192.0.2.100:443
    default_backend cluster-https
    mode tcp

backend cluster-https
    mode tcp
    server worker1 198.51.100.20:443 check
    server worker2 198.51.100.21:443 check
```

## Multi-Cluster Support

### Why IP Aliasing is Required

**The Problem:**
When deploying multiple clusters, each needs to expose services on standard ports (6443, 22623, 80, 443). However, HAProxy cannot bind multiple services to the same IP:PORT combination.

**Example Conflict:**
```
# Cluster1 config
frontend cluster1-api
    bind *:6443  # Binds to ALL IPs on port 6443

# Cluster2 config  
frontend cluster2-api
    bind *:6443  # ERROR: Port 6443 already in use!
```

**The Solution - IP Aliasing:**
Each cluster gets a dedicated VIP on the helper node:
```
# Cluster1 config
frontend cluster1-api
    bind 192.0.2.100:6443  # Cluster1 VIP

# Cluster2 config
frontend cluster2-api
    bind 192.0.2.110:6443  # Cluster2 VIP (different IP!)
```

### Multi-Cluster Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                     Helper/Bastion Node                          │
│                                                                   │
│  Network Interfaces (IP Aliasing):                               │
│    eth0       192.0.2.10  (Primary helper IP)                 │
│    eth0:vip-sno1     192.0.2.100  (Cluster1 VIP)              │
│    eth0:vip-sno2     192.0.2.110  (Cluster2 VIP)              │
│    eth0:vip-prod     192.0.2.120  (Cluster3 VIP)              │
│                                                                   │
│  Services:                                                        │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │  /etc/dnsmasq.d/                                           │ │
│  │    ├── 10-sno1.conf  (DHCP+DNS+TFTP)                      │ │
│  │    ├── 10-sno2.conf  (DHCP+DNS+TFTP)                      │ │
│  │    └── 10-prod.conf  (DHCP+DNS+TFTP)                      │ │
│  ├────────────────────────────────────────────────────────────┤ │
│  │  /etc/haproxy/conf.d/                                      │ │
│  │    ├── 10-sno1.cfg   (binds to 192.0.2.100)            │ │
│  │    ├── 10-sno2.cfg   (binds to 192.0.2.110)            │ │
│  │    └── 10-prod.cfg   (binds to 192.0.2.120)            │ │
│  ├────────────────────────────────────────────────────────────┤ │
│  │  /var/www/html/                                            │ │
│  │    ├── sno1/ignition/ + rhcos/                            │ │
│  │    ├── sno2/ignition/ + rhcos/                            │ │
│  │    └── prod/ignition/ + rhcos/                            │ │
│  ├────────────────────────────────────────────────────────────┤ │
│  │  /var/lib/tftpboot/boot/grub2/                            │ │
│  │    ├── grub.cfg-01-{mac1}  (sno1 master)                  │ │
│  │    ├── grub.cfg-01-{mac2}  (sno2 master)                  │ │
│  │    ├── grub.cfg-01-{mac3}  (prod master1)                 │ │
│  │    ├── grub.cfg-01-{mac4}  (prod master2)                 │ │
│  │    └── grub.cfg-01-{mac5}  (prod master3)                 │ │
│  └────────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
```

## Complete Workflow

> **Note**: The workflow has been improved to configure services AFTER LPAR creation, ensuring MAC addresses are available for proper DHCP and PXE boot configuration. See [ORCHESTRATOR_REFACTORING.md](ORCHESTRATOR_REFACTORING.md) for details.

### Phase 1: Validate Configuration ✅

**Purpose**: Ensure all prerequisites are met before deployment

**Actions**:
- Parse YAML configuration files
- Validate cluster definitions
- Check HMC connectivity and authentication
- Verify helper node SSH access
- Validate VIP availability (ensure no conflicts)
- Check resource requirements (CPU, memory, storage)
- Verify network configuration

**Success Criteria**:
- All configuration files are valid YAML
- HMC is reachable and credentials work
- Helper node is accessible via SSH
- VIPs are not already in use
- Sufficient resources available on Power systems

### Phase 2: Setup Helper Services ✅

**Purpose**: Prepare helper node with required packages and IP aliases

#### 2.1 Install Required Packages
```bash
dnf install -y dnsmasq haproxy httpd tftp-server syslinux-tftpboot firewalld
```

**Packages**:
- `dnsmasq` - DNS, DHCP, and TFTP server
- `haproxy` - Load balancer for cluster services
- `httpd` - HTTP server for ignition files and RHCOS images
- `tftp-server` - TFTP for PXE boot
- `syslinux-tftpboot` - PXE boot files
- `firewalld` - Firewall management

#### 2.2 Setup IP Aliases (Persistent)

Create persistent IP aliases using NetworkManager or ifcfg files:

**Using NetworkManager (Preferred)**:
```bash
# Add IP alias for cluster VIP
nmcli con mod eth0 +ipv4.addresses 192.0.2.100/24
nmcli con up eth0
```

**Using ifcfg files (Legacy)**:
```bash
cat > /etc/sysconfig/network-scripts/ifcfg-eth0:vip-sno1 <<EOF
DEVICE=eth0:vip-sno1
BOOTPROTO=static
IPADDR=192.0.2.100
NETMASK=255.255.240.0
ONBOOT=yes
EOF

ifup eth0:vip-sno1
```

**Verification**:
```bash
ip addr show eth0 | grep 192.0.2.100
```

#### 2.3 Configure Firewall
```bash
firewall-cmd --permanent --add-port={67/udp,53/tcp,53/udp,69/udp,80/tcp,443/tcp,6443/tcp,22623/tcp,8080/tcp}
firewall-cmd --reload
```

**Ports**:
- `67/udp` - DHCP
- `53/tcp,udp` - DNS
- `69/udp` - TFTP
- `80/tcp` - HTTP
- `443/tcp` - HTTPS
- `6443/tcp` - Kubernetes API
- `22623/tcp` - Machine Config Server
- `8080/tcp` - HTTP server for installation files

#### 2.4 Create Directory Structure
```bash
mkdir -p /var/www/html/{cluster-name}/{ignition,rhcos}
mkdir -p /var/lib/tftpboot/boot/grub2
mkdir -p /etc/dnsmasq.d
mkdir -p /etc/haproxy/conf.d
```

### Phase 3: Create LPARs ✅

**Purpose**: Create logical partitions on IBM Power Systems

**Actions**:
- Create LPAR via HMC REST API
- Configure processor allocation (shared/dedicated)
- Configure memory allocation
- Create and attach storage volumes (via VIOS or SVC)
- Create and attach network adapters
- **Capture MAC addresses** for DHCP/PXE configuration

**LPAR Configuration Example**:
```go
LPAR{
    Name: "sno-master",
    OSType: "AIX/Linux",
    Processor: {
        Type: "shared",
        Units: 4.0,
        VirtualProcs: 16,
    },
    Memory: {
        DesiredMB: 65536,  // 64GB
    },
    Storage: {
        BootDisk: 120GB,
        EtcdDisk: 100GB,
        ContainerStorage: 300GB,
    },
}
```

**MAC Address Capture**:
```
Network Adapter Created:
  MAC: DA:20:D2:A7:EC:02
  VLAN: 1337
  VSwitch: ETHERNET0
```

### Phase 4: Configure DNS ✅

> **Important**: This phase runs AFTER LPAR creation so MAC addresses are available for subsequent DHCP and PXE configuration.

#### 4.1 Generate DNS Configuration

**File**: `/etc/dnsmasq.d/10-{cluster-name}-dns.conf`

```conf
# DNS A Records - Nodes
address=/sno-master.sno1.example.com/198.51.100.16

# DNS A Records - API (points to VIP)
address=/api.sno1.example.com/192.0.2.100
address=/api-int.sno1.example.com/192.0.2.100

# DNS A Records - Ingress (points to VIP)
address=/.apps.sno1.example.com/192.0.2.100

# etcd SRV Records (Multi-Node only, skipped for SNO)
# srv-host=_etcd-server-ssl._tcp.sno1.example.com,master-0.sno1.example.com,2380,0,10
```

**Actions**:
1. Generate DNS configuration from template
2. Upload to `/etc/dnsmasq.d/10-{cluster-name}-dns.conf`
3. Track file in deployment state
4. Restart dnsmasq service

### Phase 5: Configure DHCP ✅

#### 5.1 Generate DHCP Configuration

**File**: `/etc/dnsmasq.d/20-{cluster-name}-dhcp.conf`

```conf
# DHCP Configuration
dhcp-range=tag:sno1,198.51.100.11,198.51.100.254,12h
dhcp-option=tag:sno1,option:router,192.0.2.254
dhcp-option=tag:sno1,option:dns-server,192.0.2.10
dhcp-option=tag:sno1,option:domain-name,sno1.example.com

# Static DHCP assignments (MAC → IP)
dhcp-host=DA:20:D2:A7:EC:02,198.51.100.16,sno-master,infinite
```

**Actions**:
1. Generate DHCP configuration with MAC-to-IP bindings
2. Upload to `/etc/dnsmasq.d/20-{cluster-name}-dhcp.conf`
3. Track file in deployment state
4. Restart dnsmasq service

### Phase 6: Configure PXE Boot ✅

#### 6.1 Setup TFTP Directory Structure

```bash
mkdir -p /var/lib/tftpboot/{cluster-name}
cp /var/www/html/{cluster-name}/rhcos/kernel /var/lib/tftpboot/{cluster-name}/
cp /var/www/html/{cluster-name}/rhcos/initramfs /var/lib/tftpboot/{cluster-name}/
```

#### 6.2 Generate PXE Configuration

**File**: `/etc/dnsmasq.d/30-{cluster-name}-pxe.conf`

```conf
# TFTP/PXE Configuration
enable-tftp
tftp-root=/var/lib/tftpboot
```

#### 6.3 Configure GRUB2 Boot Files

**Setup GRUB2 Network Boot** (one-time):
```bash
grub2-mknetdir --net-directory=/var/lib/tftpboot/boot
```

**Create Per-Node GRUB Configs** (using captured MAC addresses):

**File**: `/var/lib/tftpboot/boot/grub2/grub.cfg-01-da-20-d2-a7-ec-02`

```conf
set timeout=10
set default=0

menuentry 'Install RHCOS - sno-master' {
    echo 'Loading kernel...'
    linux /rhcos/sno1/rhcos-live-kernel-ppc64le \
        ip=198.51.100.16::192.0.2.254:255.255.240.0:sno-master.sno1.example.com:enP1p1s0f0:none \
        nameserver=192.0.2.10 \
        coreos.inst.install_dev=/dev/sda \
        coreos.inst.ignition_url=http://192.0.2.100:8080/sno1/ignition/master-sno.ign \
        coreos.live.rootfs_url=http://192.0.2.100:8080/sno1/rhcos/rhcos-live-rootfs.ppc64le.img

    echo 'Loading initramfs...'
    initrd /rhcos/sno1/rhcos-live-initramfs.ppc64le.img
}
```

**Key Parameters**:
- `ip=` - Static IP configuration
- `coreos.inst.install_dev` - Target disk for installation
- `coreos.inst.ignition_url` - Ignition file URL (uses VIP)
- `coreos.live.rootfs_url` - Root filesystem URL (uses VIP)

**Actions**:
1. Create TFTP directory structure
2. Copy RHCOS kernel and initramfs files
3. Generate PXE configuration
4. Upload to `/etc/dnsmasq.d/30-{cluster-name}-pxe.conf`
5. Generate GRUB2 boot files for each node
6. Track files in deployment state
7. Restart dnsmasq service

### Phase 7: Configure HTTP Server ✅

**Purpose**: Serve ignition files and RHCOS images

**Directory Structure**:
```
/var/www/html/sno1/
├── ignition/
│   ├── master-sno.ign
│   ├── worker.ign
│   └── bootstrap.ign
└── rhcos/
    ├── rhcos-live-kernel-ppc64le
    ├── rhcos-live-initramfs.ppc64le.img
    └── rhcos-live-rootfs.ppc64le.img
```

**HTTP Server Configuration**:
```bash
# Enable and start httpd
systemctl enable httpd --now

# Set SELinux context
restorecon -Rv /var/www/html/

# Test HTTP access
curl http://192.0.2.100:8080/sno1/ignition/master-sno.ign
```

**URLs Generated**:
- Ignition: `http://192.0.2.100:8080/sno1/ignition/master-sno.ign`
- Kernel: `http://192.0.2.100:8080/sno1/rhcos/rhcos-live-kernel-ppc64le`
- Initramfs: `http://192.0.2.100:8080/sno1/rhcos/rhcos-live-initramfs.ppc64le.img`
- Rootfs: `http://192.0.2.100:8080/sno1/rhcos/rhcos-live-rootfs.ppc64le.img`

### Phase 8: Configure HAProxy ✅

**Purpose**: Load balance cluster services using single VIP

**File**: `/etc/haproxy/conf.d/10-{cluster-name}.cfg`

```conf
# ==========================================
# Cluster: sno1
# Type: SNO
# OCP Version: 4.21
# VIP: 192.0.2.100 (Single VIP)
# Generated: 2026-03-31
# ==========================================

# API Server (Port 6443)
frontend sno1-openshift-api-server
    bind 192.0.2.100:6443
    default_backend sno1-openshift-api-server
    mode tcp
    option tcplog

backend sno1-openshift-api-server
    balance source
    mode tcp
    server sno-master 198.51.100.16:6443 check

# Machine Config Server (Port 22623)
frontend sno1-machine-config-server
    bind 192.0.2.100:22623
    default_backend sno1-machine-config-server
    mode tcp
    option tcplog

backend sno1-machine-config-server
    balance source
    mode tcp
    server sno-master 198.51.100.16:22623 check

# Ingress HTTP (Port 80)
frontend sno1-ingress-http
    bind 192.0.2.100:80
    default_backend sno1-ingress-http
    mode tcp
    option tcplog

backend sno1-ingress-http
    balance source
    mode tcp
    server sno-master-http-router0 198.51.100.16:80 check

# Ingress HTTPS (Port 443)
frontend sno1-ingress-https
    bind 192.0.2.100:443
    default_backend sno1-ingress-https
    mode tcp
    option tcplog

backend sno1-ingress-https
    balance source
    mode tcp
    server sno-master-https-router0 198.51.100.16:443 check
```

**Key Points**:
- All frontends bind to the same VIP (192.0.2.100)
- Different ports for different services
- HAProxy handles port-based routing
- No port conflicts between clusters (each has unique VIP)

**Enable HAProxy**:
```bash
systemctl enable haproxy --now
systemctl status haproxy
```

### Phase 9: Download Images ✅

**Purpose**: Download RHCOS images and OpenShift tools

**Downloads**:
```bash
# RHCOS Images
wget -P /var/www/html/sno1/rhcos/ \
  https://mirror.openshift.com/pub/openshift-v4/ppc64le/dependencies/rhcos/4.21/4.21.0/rhcos-live-kernel-ppc64le

wget -P /var/www/html/sno1/rhcos/ \
  https://mirror.openshift.com/pub/openshift-v4/ppc64le/dependencies/rhcos/4.21/4.21.0/rhcos-live-initramfs.ppc64le.img

wget -P /var/www/html/sno1/rhcos/ \
  https://mirror.openshift.com/pub/openshift-v4/ppc64le/dependencies/rhcos/4.21/4.21.0/rhcos-live-rootfs.ppc64le.img

# OpenShift Client and Installer
wget -P /usr/local/src/sno1/ \
  https://mirror.openshift.com/pub/openshift-v4/multi/clients/ocp/latest-4.21/ppc64le/openshift-client-linux.tar.gz

wget -P /usr/local/src/sno1/ \
  https://mirror.openshift.com/pub/openshift-v4/multi/clients/ocp/latest-4.21/ppc64le/openshift-install-linux.tar.gz

# Extract tools
tar -xzf /usr/local/src/sno1/openshift-client-linux.tar.gz -C /usr/local/bin/
tar -xzf /usr/local/src/sno1/openshift-install-linux.tar.gz -C /usr/local/bin/
```

### Phase 10: Generate Ignition Files ✅

**Purpose**: Create ignition configuration for RHCOS installation

**Create Install Config**:
```yaml
# /root/ocp4-workdir-sno1/install-config.yaml
apiVersion: v1
baseDomain: example.com
metadata:
  name: sno1
networking:
  networkType: OVNKubernetes
  clusterNetwork:
  - cidr: 10.128.0.0/14
    hostPrefix: 23
  serviceNetwork:
  - 172.30.0.0/16
  machineNetwork:
  - cidr: 192.0.2.0/20
compute:
- name: worker
  replicas: 0
controlPlane:
  name: master
  replicas: 1
platform:
  none: {}
pullSecret: '{"auths":...}'
sshKey: 'ssh-rsa AAAA...'
```

**Generate Ignition**:
```bash
cd /root/ocp4-workdir-sno1
openshift-install create single-node-ignition-config --dir=.

# Copy to HTTP server
cp *.ign /var/www/html/sno1/ignition/
restorecon -Rv /var/www/html/
```

### Phase 11: Network Boot LPARs ✅

**Purpose**: Start LPAR installation using network boot (netboot)

**Implementation**: Uses HMC REST API to perform network boot with static IP configuration

**Actions**:
1. Retrieve LPAR profile UUID
2. Translate MAC address to physical location code
3. Configure network boot parameters:
   - Client IP (LPAR IP address)
   - Server IP (Helper node IP)
   - Gateway and netmask
   - Boot device location code
4. Execute network boot via HMC API
5. LPAR PXE boots:
   - DHCP request → Gets IP from dnsmasq (static binding by MAC)
   - TFTP request → Downloads GRUB config (by MAC address)
   - GRUB loads → Kernel + initramfs from HTTP
   - RHCOS boots → Downloads rootfs and ignition from HTTP
   - Installation begins automatically

**Network Boot Flow**:
```
┌─────────────────────────────────────────────────────────────┐
│ 0. Fetch network adapter details from HMC                  │
│    - GetClientNetworkAdapters(systemUUID, lparUUID)        │
│    - Get base LocationCode (e.g., U9105.22A.789C301-V10-C2)│
│    - Append -T0 suffix (try first)                         │
└────────────────────────┬────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────┐
│ 1. HMC sends network boot command with parameters          │
│    - ProfileUUID, BootMode: "netboot"                      │
│    - LocationCode with -T0 suffix                          │
│    - ClientIP, ServerIP, Gateway, Netmask (0.0.0.0 for DHCP)│
└────────────────────────┬────────────────────────────────────┘
                         │
                         ▼
                    ┌────┴────┐
                    │ Success?│
                    └────┬────┘
                         │
              ┌──────────┴──────────┐
              │ No (Location Error) │ Yes
              ▼                     ▼
┌─────────────────────────────┐    │
│ Retry with -T1 suffix       │    │
│ LocationCode + "-T1"        │    │
└────────────┬────────────────┘    │
             │                     │
             └──────────┬──────────┘
                        │
                        ▼
┌─────────────────────────────────────────────────────────────┐
│ 2. LPAR sends DHCP request with MAC address                │
└────────────────────────┬────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────┐
│ 3. DNSmasq assigns static IP based on MAC                  │
│    dhcp-host=52:54:00:12:34:56,198.51.100.16,sno-master     │
└────────────────────────┬────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────┐
│ 4. LPAR fetches PXE config via TFTP                        │
│    /boot/grub2/grub.cfg-01-52-54-00-12-34-56               │
└────────────────────────┬────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────┐
│ 5. LPAR downloads kernel, initramfs, rootfs via HTTP       │
│    http://198.51.100.10:8080/sno-test/rhcos/rhcos-kernel    │
│    http://198.51.100.10:8080/sno-test/rhcos/rhcos-initramfs │
│    http://198.51.100.10:8080/sno-test/rhcos/rhcos-rootfs    │
└────────────────────────┬────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────┐
│ 6. LPAR fetches ignition configuration via HTTP            │
│    http://198.51.100.10:8080/sno-test/ignition/master.ign   │
└────────────────────────┬────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────┐
│ 7. RHCOS installs to boot disk using ignition              │
└────────────────────────┬────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────┐
│ 8. LPAR reboots from boot disk                             │
└─────────────────────────────────────────────────────────────┘
```

**Code Implementation**:
```go
// Network boot parameters
options := &hmc.PowerOnOptions{
    ProfileUUID:  profileUUID,
    BootMode:     "netboot",
    LocationCode: locationCode + "-T1",  // Append -T1 for Virtual Ethernet
    ClientIP:     node.IP,
    ServerIP:     helperIP,
    Gateway:      gateway,
    Netmask:      netmask,
}

status, err := hmcClient.PowerOnPartition(lparUUID, options, false)
```

**Key Features**:
- **MAC to Location Code Translation**: Automatically translates MAC address to physical location code
- **Static IP Assignment**: Uses network boot parameters for initial IP configuration
- **Automated Boot**: No manual intervention required after network boot command
- **Per-Cluster HTTP Directories**: Each cluster has isolated HTTP directory structure

**Documentation**: See [NETWORK_BOOT_IMPLEMENTATION.md](NETWORK_BOOT_IMPLEMENTATION.md) for complete details

### Phase 12: Wait for Installation ✅

**Purpose**: Monitor OpenShift installation progress using `openshift-install` commands

**Implementation**: Automated monitoring with different flows for SNO vs Multi-Node

#### SNO Deployment Flow

```
┌─────────────────────────────────────────────────────────────┐
│ 1. Skip bootstrap wait (no separate bootstrap node)        │
└────────────────────────┬────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────┐
│ 2. Wait for install-complete (30-45 minutes)               │
│    openshift-install wait-for install-complete             │
└────────────────────────┬────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────┐
│ 3. Save kubeconfig (optional)                              │
└────────────────────────┬────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────┐
│ 4. Display cluster information and credentials             │
└─────────────────────────────────────────────────────────────┘
```

#### Multi-Node Deployment Flow

```
┌─────────────────────────────────────────────────────────────┐
│ 1. Wait for bootstrap-complete (20-30 minutes)             │
│    openshift-install wait-for bootstrap-complete           │
└────────────────────────┬────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────┐
│ 2. Bootstrap node can be powered off/deleted               │
└────────────────────────┬────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────┐
│ 3. Wait for install-complete (30-60 minutes)               │
│    openshift-install wait-for install-complete             │
└────────────────────────┬────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────┐
│ 4. Save kubeconfig (optional)                              │
└────────────────────────┬────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────┐
│ 5. Display cluster information and credentials             │
└─────────────────────────────────────────────────────────────┘
```

**Monitoring Commands**:
```bash
# For multi-node: Wait for bootstrap complete
cd /var/www/html/cluster-name/install
timeout 1800 ./openshift-install wait-for bootstrap-complete --log-level=info

# Wait for installation complete (both SNO and multi-node)
cd /var/www/html/cluster-name/install
timeout 3600 ./openshift-install wait-for install-complete --log-level=info
```

**Configuration**:
```yaml
deployment:
  timeouts:
    bootstrap_complete: 1800      # 30 minutes (multi-node only)
    installation_complete: 3600   # 60 minutes

advanced:
  save_kubeconfig: true
  kubeconfig_path: "./kubeconfig-cluster-name"
```

**Expected Timeline**:

| Phase | SNO | Multi-Node | Description |
|-------|-----|------------|-------------|
| Network Boot | 2-5 min | 5-10 min | LPAR boots, fetches RHCOS |
| RHCOS Install | 5-10 min | 10-15 min | RHCOS installs to disk |
| First Boot | 5-10 min | 5-10 min | System boots, applies ignition |
| Bootstrap | N/A | 20-30 min | Bootstrap initializes cluster |
| Control Plane | 15-20 min | 15-20 min | Control plane stabilizes |
| Operators | 10-15 min | 15-20 min | Cluster operators deploy |
| **Total** | **30-45 min** | **60-90 min** | Complete installation |

**Output Example**:
```
=== Waiting for Installation Complete ===
Timeout: 3600 seconds (60 minutes)
This may take 30-45 minutes for SNO...
Executing: openshift-install wait-for install-complete

INFO Install complete!
INFO Access the OpenShift web-console here: https://console-openshift-console.apps.sno-test.example.com
INFO Login to the console with user: "kubeadmin", and password: "xxxxx-xxxxx-xxxxx-xxxxx"

✓ Installation Complete!

======================================================================
CLUSTER INFORMATION
======================================================================
Cluster Name: sno-test
Base Domain:  example.com
Cluster VIP:  198.51.100.50
Console URL:  https://console-openshift-console.apps.sno-test.example.com
API URL:      https://api.sno-test.example.com:6443
======================================================================

✓ Kubeconfig saved to: ./kubeconfig-sno-test
```

**Features**:
- **Automated Monitoring**: No manual intervention required
- **Type-Aware**: Different handling for SNO vs multi-node
- **Real-time Feedback**: See installation progress in real-time
- **Error Detection**: Immediate notification of failures
- **Credential Extraction**: Automatically extracts and displays credentials
- **Kubeconfig Management**: Optionally saves kubeconfig locally
- **Timeout Protection**: Prevents indefinite waiting

**Documentation**: See [INSTALLATION_MONITORING.md](INSTALLATION_MONITORING.md) for complete details

## Configuration Structure

### Main Configuration File

```yaml
# config.yaml - Main multi-cluster configuration
helper_node:
  hostname: "helper.example.com"
  ip: "192.0.2.10"
  ssh_user: "root"
  ssh_key_file: "~/.ssh/id_rsa"
  network_interface: "eth0"
  
  required_packages:
    - dnsmasq
    - haproxy
    - httpd
    - tftp-server
    - syslinux-tftpboot
    - firewalld
  
  vip_pool:
    start: "192.0.2.100"
    end: "192.0.2.199"

hmc:
  ip: "192.0.2.1"
  username: "REDACTED_HMC_USER<=="
  password: "REDACTED_HMC_PASS<=="

clusters:
  - name: "sno1"
    type: "sno"
    ocp_version: "4.21"
    vip: "192.0.2.100"  # Single VIP
    config_file: "./cluster-sno.yaml"
  
  - name: "prod"
    type: "multi-node"
    ocp_version: "4.21"
    vip: "192.0.2.110"  # Single VIP
    config_file: "./cluster-multi.yaml"
```

### Cluster-Specific Configuration

```yaml
# cluster-sno.yaml - SNO cluster configuration
power_systems:
  - name: "LTC09U31-ZZ"
    vswitch_name: "ETHERNET0(Default)"
    vlan_id: 1337
    max_lpars: 5
    available_memory_gb: 256
    available_processors: 16

storage:
  type: "vios"
  vios:
    - system_name: "LTC09U31-ZZ"
      vios_name: "ltc09u31-vios1"
      volume_group: "auto_vg01"

network:
  domain: "example.com"
  # NOTE: cluster_name is derived from the deployment 'name' field (sno1)
  # The cluster will be accessible at:
  #   - API: https://api.sno1.example.com:6443
  #   - Apps: https://*.apps.sno1.example.com
  base_domain: "example.com"
  network_cidr: "192.0.2.0/20"
  gateway: "192.0.2.254"
  netmask: "255.255.240.0"
  nameserver: "192.0.2.10"
  dns_forwarders:
    - "10.0.10.4"
    - "10.0.10.5"
  ntp_servers:
    - "10.0.10.4"
    - "10.0.10.5"
  mac_prefix: "52:54:00"

openshift:
  version: "4.21"
  pull_secret_file: "./pull-secret.json"
  ssh_public_key_file: "~/.ssh/id_rsa.pub"
  cluster_network_cidr: "10.128.0.0/14"
  cluster_network_host_prefix: 23
  service_network: "172.30.0.0/16"
  machine_network: "192.0.2.0/20"
  
  rhcos_images:
    kernel_url: "https://mirror.openshift.com/pub/openshift-v4/ppc64le/dependencies/rhcos/4.21/4.21.0/rhcos-live-kernel.ppc64le"
    initramfs_url: "https://mirror.openshift.com/pub/openshift-v4/ppc64le/dependencies/rhcos/4.21/4.21.0/rhcos-live-initramfs.ppc64le.img"
    rootfs_url: "https://mirror.openshift.com/pub/openshift-v4/ppc64le/dependencies/rhcos/4.21/4.21.0/rhcos-live-rootfs.ppc64le.img"
  
  ocp_client_config:
    ocp_client: "https://mirror.openshift.com/pub/openshift-v4/multi/clients/ocp/latest-4.21/ppc64le/openshift-client-linux.tar.gz"
    ocp_installer: "https://mirror.openshift.com/pub/openshift-v4/multi/clients/ocp/latest-4.21/ppc64le/openshift-install-linux.tar.gz"

sno_node:
  # NOTE: For SNO deployments, 'hostname' defaults to cluster name if not specified
  #       The 'name' field defaults to "{hostname}-master" if not specified
  # name: "sno-master"              # (Optional) HAProxy backend name
  # hostname: "sno1"                # (Optional) OS Hostname - defaults to cluster name
  ip: "198.51.100.16"
  system_name: "LTC09U31-ZZ"
  
  lpar:
    os_type: "AIX/Linux"
    processor:
      type: "shared"
      units: 4.0
      virtual_procs: 16
    memory:
      desired_mb: 65536
    storage:
      boot_disk_gb: 120
      etcd_disk_gb: 100
      container_storage_gb: 300

deployment:
  phases:
    - validate
    - setup_helper_services
    - create_lpars
    - setup_dnsmasq
    - setup_http
    - setup_haproxy
    - download_images
    - generate_ignition
    - power_on
    - wait_bootstrap
    - wait_installation
  
  timeouts:
    lpar_creation: 10
    power_on: 5
    helper_setup: 30
    installation_complete: 90
  
  retry:
    max_attempts: 3
    delay_seconds: 30
```

## Cluster Deletion

### Deletion Process

The deletion process follows a 4-step sequence with **intelligent partial failure handling**:

```bash
./ocp-upi-deployer -command delete -cluster sno1
```

#### Step 1: Close Virtual Terminals & Power Off LPARs
- Closes virtual terminals via SSH (prevents HMC REST API conflicts)
- Powers off all LPARs gracefully
- Non-fatal if terminal not open or LPAR already off

#### Step 2: Unmap Storage from LPARs
- Groups volumes by VIOS and LPAR for batch unmapping
- Uses `DeleteVirtualDiskMaps` for efficient bulk operations
- Caches VIOS UUIDs to minimize API calls

#### Step 3: Delete Storage Volumes
- Deletes virtual disks from VIOS volume groups
- Tracks failed deletions (e.g., VIOS not found, disk in use)
- Preserves failed volumes in state for retry

#### Step 4: Delete LPARs
- Removes LPAR partitions from HMC
- Tracks failed deletions
- Preserves failed LPARs in state for retry

### Partial Failure Handling

**Key Feature**: The deletion process now handles partial failures intelligently:

```go
// Maps to track items that FAILED to delete
retainedLPARs := make(map[string]LPARState)
retainedVolumes := make(map[string]VolumeState)
var deletionErrors []string
```

**Behavior**:
1. **Tracks Each Failure**: Records which resources failed to delete and why
2. **Preserves State**: Only failed resources remain in state file
3. **Enables Retry**: Re-running delete command only attempts failed resources
4. **Clear Reporting**: Returns error with list of resources that remain

**Example Output**:
```
Step 3: Deleting storage volumes...
  Deleting volume: snonew5-n-b-a3f9...
    ⚠ Failed to delete disk snonew5-n-b-a3f9: disk in use
  Deleting volume: snonew5-n-d-b7c2...
    ✅ Deleted virtual disk: snonew5-n-d-b7c2

Step 4: Deleting LPARs...
  Deleting LPAR: sno-new-5...
    ✅ LPAR deleted successfully

Error: infrastructure deletion completed with errors. The following resources
remain and have been preserved in state: Volume: snonew5-n-b-a3f9
```

**State File After Partial Failure**:
```json
{
  "created_lpars": {},
  "created_volumes": {
    "snonew5-n-b-a3f9": {
      "name": "snonew5-n-b-a3f9",
      "size_gb": 120,
      "storage_type": "vios",
      "vios_name": "vios1",
      "volume_group": "datavg"
    }
  }
}
```

**Retry Deletion**:
```bash
# Re-run delete - only attempts the failed volume
./ocp-upi-deployer -command delete -cluster sno1
```

### Helper Node Cleanup

After infrastructure deletion, the following helper node resources are removed:
- IP alias (eth0:vip-sno1)
- /etc/dnsmasq.d/10-sno1.conf
- /etc/haproxy/conf.d/10-sno1.cfg
- /var/www/html/sno1/
- /var/lib/tftpboot/boot/grub2/grub.cfg-01-{mac}

### Cluster Directory Cleanup

After successful deletion, the tool prompts:
```
Do you want to remove the cluster directory? (y/n):
```

- **Yes**: Removes `clusters/sno1/` (config.yaml, state.json, backups)
- **No**: Preserves directory for audit/reference

### Benefits of Improved Deletion

1. **Idempotency**: Safe to re-run delete command multiple times
2. **Resilience**: Partial failures don't corrupt state or lose resource tracking
3. **Transparency**: Clear visibility into what succeeded/failed
4. **Resource Safety**: No orphaned resources - everything tracked until deleted
5. **Debugging**: Error messages include context (e.g., "VIOS missing", "disk in use")
6. **Audit Trail**: State file accurately reflects reality at all times
```

## Implementation Modules

| Module | File | Purpose |
|--------|------|---------|
| Configuration | [`types.go`](types.go) | Type definitions and structures |
| Validation | [`validator.go`](validator.go) | Configuration validation |
| SSH Client | [`ssh.go`](ssh.go) | Helper node operations |
| HMC Client | Uses `powerhmc-go` | HMC API operations |
| DNSmasq | [`dnsmasq.go`](dnsmasq.go) | DNS/DHCP/TFTP configuration |
| HAProxy | [`haproxy.go`](haproxy.go) | Load balancer configuration |
| HTTP Server | [`httpserver.go`](httpserver.go) | HTTP server setup |
| Ignition | [`ignition.go`](ignition.go) | Ignition file generation |
| PXE Boot | [`pxeboot.go`](pxeboot.go) | GRUB configuration |
| LPAR | [`lpar.go`](lpar.go) | LPAR provisioning |
| Orchestrator | [`orchestrator.go`](orchestrator.go) | Deployment orchestration |
| Main | [`main.go`](main.go) | CLI entry point |

## Dependencies

```go
require (
    github.com/sudeeshjohn/powerhmc-go v0.0.0  // HMC API client
    golang.org/x/crypto/ssh v0.0.0              // SSH client
    gopkg.in/yaml.v3 v3.0.1                     // YAML parsing
)
```

## Testing

### Unit Tests
```bash
go test ./...
```

### Integration Tests
```bash
# Test configuration validation
./ocp-upi-deployer -command validate -config config-test.yaml

# Test single VIP implementation
./test-single-vip.sh

# Test phase flow
./test-phase-flow.sh
```

### Production Deployment
```bash
# Deploy cluster
./ocp-upi-deployer -command deploy -config config.yaml -cluster sno1 -verbose

# Check status
./ocp-upi-deployer -command status -config config.yaml -cluster sno1

# Delete cluster
./ocp-upi-deployer -command delete -config config.yaml -cluster sno1
```

## Documentation

- [`README.md`](README.md) - Getting started guide
- [`DESIGN.md`](DESIGN.md) - This document
- [`ORCHESTRATOR_REFACTORING.md`](ORCHESTRATOR_REFACTORING.md) - Phase flow improvements
- [`SINGLE_VIP_IMPLEMENTATION.md`](SINGLE_VIP_IMPLEMENTATION.md) - Single VIP details
- [`SINGLE_VIP_TEST_RESULTS.md`](SINGLE_VIP_TEST_RESULTS.md) - Test results
- [`DEPLOYMENT_RUN_RESULTS.md`](DEPLOYMENT_RUN_RESULTS.md) - Production deployment log

## Support

For issues, questions, or contributions:
- GitHub Issues: [Create an issue](https://github.com/sudeeshjohn/powerhmc-go/issues)
- Documentation: See docs above
- Examples: Check `examples/` directory

---

**Last Updated**: 2026-03-31  
**Version**: 2.0 (Single VIP Architecture)  
**Status**: Production Ready ✅