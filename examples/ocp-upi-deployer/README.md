# OpenShift UPI Deployer for IBM Power Systems

A comprehensive Go-based tool for deploying multiple OpenShift clusters (SNO or Multi-Node) on IBM Power Systems using User-Provisioned Infrastructure (UPI) method with **full automation** from LPAR creation to cluster ready state.

## Overview

This tool provides **end-to-end automation** for OpenShift deployment on IBM Power Systems:
- ✅ **Network Boot (Netboot)**: Automated PXE boot using HMC REST API
- ✅ **Installation Monitoring**: Automated monitoring with `openshift-install` commands
- ✅ **Single VIP Architecture**: One IP per cluster (50% IP savings)
- ✅ **Multi-Cluster Support**: Deploy multiple clusters from single helper node
- ✅ **Per-Cluster HTTP Directories**: Isolated `/var/www/html/{cluster-name}` structure
- ✅ **Automatic MAC Capture**: Services configured after LPAR creation
- ✅ **Resume Functionality**: Resume failed deployments from last completed phase
- ✅ **State Management**: Track deployment progress with JSON state files
- ✅ **Cluster Directory Structure**: Isolated directories for each cluster with config and state
- ✅ **Granular Dnsmasq Configuration**: Separate DNS, DHCP, and PXE phases for better modularity

## Key Features

### 🚀 Network Boot Implementation
- **Automated Network Boot**: Uses HMC REST API for netboot with static IP configuration
- **MAC to Location Code Translation**: Automatically translates MAC addresses to physical location codes
- **PXE Boot Flow**: Complete automation from DHCP → TFTP → HTTP → Ignition
- **No Manual Intervention**: LPARs boot and install automatically after network boot command

### 📊 Installation Monitoring
- **Automated Monitoring**: Uses `openshift-install wait-for` commands
- **Type-Aware**: Different handling for SNO (skip bootstrap) vs Multi-Node
- **Real-time Feedback**: See installation progress with detailed output
- **Credential Extraction**: Automatically displays console URL and kubeadmin password
- **Kubeconfig Management**: Optionally saves kubeconfig locally

### 🏗️ Architecture

#### Single VIP Architecture
- **One IP per Cluster**: Replaces traditional dual VIP (API + Ingress) approach
- **50% IP Savings**: Deploy twice as many clusters with same IP pool
- **Port-Based Routing**: HAProxy routes traffic based on port (6443, 22623, 80, 443)
- **Simplified DNS**: All DNS records point to single VIP

#### Multi-Cluster Support
- **IP Aliasing**: Each cluster gets dedicated VIP on helper node
- **Per-Cluster Isolation**: Separate directories, configs, and service instances
- **HTTP Directory Structure**: `/var/www/html/{cluster-name}/{ignition,rhcos,tools,scripts}`
- **Standard Ports**: All clusters use standard ports via IP aliasing

### Components

1. **Configuration Management** (`types.go`, `validator.go`)
   - Multi-cluster configuration with per-cluster settings
   - Comprehensive validation of all configuration aspects
   - Support for SNO and Multi-Node topologies

2. **SSH Client** (`ssh.go`)
   - Remote command execution on helper node
   - File upload/download capabilities
   - Streaming output support

3. **Service Configuration Generators**
   - **DNSmasq** (`dnsmasq.go`): Per-cluster DNS, DHCP, and TFTP
   - **HAProxy** (`haproxy.go`): Per-cluster load balancing with VIP binding
   - **HTTP Server** (`httpserver.go`, `downloader.go`, `httphelper.go`): Per-cluster web server for ignition files and RHCOS images

4. **Ignition Generator** (`ignition.go`)
   - Creates `install-config.yaml` from cluster configuration
   - Generates manifests and ignition configs using `openshift-install`
   - Handles SNO vs Multi-Node differences
   - Copies ignition files to HTTP directory

5. **PXE Boot Manager** (`pxeboot.go`)
   - Generates GRUB configs for network booting
   - MAC-based boot configuration
   - Per-node boot parameters

6. **LPAR Provisioner** (`lpar.go`)
   - LPAR creation and management via HMC REST API
   - Storage attachment (VIOS/SVC)
   - MAC address capture and configuration update
   - **Network boot implementation** with HMC API

7. **Orchestrator** (`orchestrator.go`)
   - 13-phase deployment workflow (expanded from 12 phases)
   - Granular dnsmasq configuration (DNS, DHCP, PXE as separate phases)
   - Split installation monitoring (wait_bootstrap and wait_installation phases)
   - Resume functionality for failed deployments
   - State management with cluster-specific JSON files
   - Real-time phase tracking and status updates

## Project Status

### ✅ Fully Implemented Components

1. **Design Document** (`DESIGN.md`)
   - Complete architecture documentation
   - Single VIP architecture explanation
   - Per-cluster service configuration
   - Network boot and installation monitoring flows

2. **Configuration Types** (`types.go`)
   - Multi-cluster configuration structure
   - SNO and Multi-Node support
   - Deployment state tracking
   - Resume functionality support

3. **Validator** (`validator.go`)
   - Comprehensive configuration validation
   - Helper node, HMC, VIP pool validation
   - Power systems, storage, network validation

4. **SSH Client** (`ssh.go`)
   - Full implementation with all required methods
   - Command execution, file transfer, streaming output

5. **Service Generators**
   - **DNSmasq** (`dnsmasq.go`): DNS, DHCP, TFTP with MAC-based static bindings
   - **HAProxy** (`haproxy.go`): Single VIP load balancing with port-based routing
   - **HTTP Server** (`httpserver.go`): Per-cluster directory structure in `/var/www/html`

6. **Ignition Generator** (`ignition.go`)
   - Complete workflow implementation
   - install-config.yaml generation
   - Manifest and ignition config generation
   - File copying to per-cluster HTTP directory

7. **PXE Boot Manager** (`pxeboot.go`)
   - GRUB configuration generation
   - MAC-based boot files (`grub.cfg-01-{mac}`)
   - Per-node boot parameters with cluster-specific URLs

8. **LPAR Provisioner** (`lpar.go`)
   - Full LPAR lifecycle management
   - Network boot implementation using HMC REST API
   - MAC address capture and configuration update
   - Storage attachment (VIOS virtual disks)

9. **Orchestrator** (`orchestrator.go`)
   - 12-phase deployment workflow with granular dnsmasq configuration
   - **Network boot** instead of simple power-on
   - **Installation monitoring** with `openshift-install` commands
   - Resume functionality for failed deployments
   - State management with cluster-specific JSON files in `clusters/<name>/state.json`

10. **Example Configurations**
    - SNO configuration (`cluster-sno.yaml`, `cluster-sno-test.yaml`)
    - Multi-cluster configuration (`config.yaml`, `config-test.yaml`)

### 🎯 Recent Implementations

1. **Cluster Directory Structure** ([`CLUSTER_DIRECTORY_IMPLEMENTATION.md`](CLUSTER_DIRECTORY_IMPLEMENTATION.md))
   - Single binary manages multiple clusters
   - Each cluster has isolated directory: `clusters/<cluster-name>/`
   - Automatic config preservation during deployment
   - State file isolation per cluster
   - New `list` command to view all managed clusters
   - Optional directory cleanup on deletion

2. **Intelligent Deletion with Partial Failure Handling**
   - Tracks failed deletions and preserves them in state
   - Idempotent: safe to re-run delete command
   - Only retries resources that failed to delete
   - Clear error reporting with resource-specific context
   - No orphaned resources - everything tracked until deleted
   - State file accurately reflects reality after each deletion attempt

3. **Dnsmasq Configuration Refactoring** ([`DNSMASQ_REFACTORING.md`](DNSMASQ_REFACTORING.md))
   - Split monolithic dnsmasq phase into three granular phases
   - `setup_dns`: Configure DNS A records and etcd SRV records
   - `setup_dhcp`: Configure DHCP with MAC-to-IP bindings
   - `setup_pxe`: Configure PXE/TFTP boot settings
   - Numbered config files for proper load order (10-, 20-, 30-)
   - Better debugging and modularity

4. **Network Boot** ([`NETWORK_BOOT_IMPLEMENTATION.md`](NETWORK_BOOT_IMPLEMENTATION.md))
   - Replaces simple power-on with full network boot
   - MAC to location code translation
   - Static IP configuration via HMC API
   - Based on [`lparnetboot`](../lparnetboot/main.go) example

5. **Installation Monitoring** ([`INSTALLATION_MONITORING.md`](INSTALLATION_MONITORING.md))
   - Automated monitoring with `openshift-install wait-for` commands
   - Type-aware: SNO skips bootstrap, multi-node waits for bootstrap first
   - Credential extraction and display
   - Optional kubeconfig saving

6. **HTTP Directory Structure** ([`HTTP_DIRECTORY_STRUCTURE.md`](HTTP_DIRECTORY_STRUCTURE.md))
   - Per-cluster directories in `/var/www/html/{cluster-name}`
   - Subdirectories: `ignition/`, `rhcos/`, `tools/`, `scripts/`
   - Proper permissions and SELinux contexts

6. **MAC Address Bug Fix** ([`MAC_ADDRESS_BUG_FIX.md`](MAC_ADDRESS_BUG_FIX.md))
   - MAC address captured during LPAR creation
   - Written back to node configuration
   - Enables proper DNSmasq static DHCP bindings

7. **Resume Functionality** ([`RESUME_DEPLOYMENT.md`](RESUME_DEPLOYMENT.md))
   - Automatic resume from last completed phase
   - `-resume` command-line flag
   - State file tracking with JSON

### 📚 Documentation

- [`DESIGN.md`](DESIGN.md) - Complete architecture and workflow (updated with 12-phase deployment)
- [`CLUSTER_DIRECTORY_IMPLEMENTATION.md`](CLUSTER_DIRECTORY_IMPLEMENTATION.md) - Cluster directory structure details
- [`DNSMASQ_REFACTORING.md`](DNSMASQ_REFACTORING.md) - Granular dnsmasq configuration details
- [`ARCHITECTURE_EXPLANATION.md`](ARCHITECTURE_EXPLANATION.md) - Single VIP architecture details
- [`NETWORK_BOOT_IMPLEMENTATION.md`](NETWORK_BOOT_IMPLEMENTATION.md) - Network boot details
- [`INSTALLATION_MONITORING.md`](INSTALLATION_MONITORING.md) - Installation monitoring details
- [`HTTP_DIRECTORY_STRUCTURE.md`](HTTP_DIRECTORY_STRUCTURE.md) - HTTP server organization
- [`MAC_ADDRESS_BUG_FIX.md`](MAC_ADDRESS_BUG_FIX.md) - MAC address capture fix
- [`MAC_ADDRESS_FLOW.md`](MAC_ADDRESS_FLOW.md) - MAC address flow diagram
- [`RESUME_DEPLOYMENT.md`](RESUME_DEPLOYMENT.md) - Resume functionality guide
- [`DEPLOYMENT_STATE_FIX.md`](DEPLOYMENT_STATE_FIX.md) - State file configuration
- [`HAPROXY_FIX.md`](HAPROXY_FIX.md) - HAProxy configuration fix

### ⚠️ Known Limitations

1. **Storage Backend**
   - Framework is in place
   - Requires full implementation of:
     - LPAR creation using powerhmc-go
     - Network adapter creation
     - Storage attachment (VIOS/SVC)
     - MAC address capture
     - Power on/off operations
   - See `powerhmc-go/examples/createlpar/main.go` for reference

2. **Deployment Orchestrator** (NOT YET CREATED)
   - Needs to coordinate all phases:
     1. Validate configuration
     2. Setup helper node services
     3. Download RHCOS images and tools
     4. Generate ignition configs
     5. Configure PXE boot
     6. Create LPARs
     7. Power on and monitor installation
   - Should handle state management and rollback

3. **Main Entry Point** (`main.go` - NOT YET CREATED)
   - CLI with commands: deploy, delete, validate, status
   - Flag parsing for configuration file
   - Error handling and logging
   - State file management

4. **README Documentation** (THIS FILE)
   - Usage examples needed
   - Troubleshooting guide needed
   - Prerequisites documentation needed

## Configuration

### Multi-Cluster Configuration

The tool uses a two-level configuration:

1. **Top-level config** (`config.yaml`): Defines helper node, HMC, and cluster references
2. **Per-cluster config** (`cluster-sno.yaml`, `cluster-multi.yaml`): Defines cluster-specific settings

Example top-level configuration:

```yaml
helper_node:
  hostname: helper.example.com
  ip: 192.168.1.10
  ssh_user: root
  ssh_key_file: ~/.ssh/id_rsa
  network_interface: eth0
  vip_pool:
    start: 192.168.1.100
    end: 192.168.1.200

hmc:
  ip: 192.168.1.5
  username: REDACTED_HMC_USER<==
  password: REDACTED_HMC_PASS<==

clusters:
  - name: ocp-sno
    type: sno
    ocp_version: "4.21"
    api_vip: 192.168.1.100
    ingress_vip: 192.168.1.101
    config_file: ./config-sno.yaml
  
  - name: ocp-prod
    type: multi-node
    ocp_version: "4.21"
    api_vip: 192.168.1.110
    ingress_vip: 192.168.1.111
    config_file: ./config-multi.yaml
```

### Cluster-Specific Configuration

See `config-sno.yaml` and `config-multi.yaml` for complete examples.

Key sections:
- **power_systems**: Power systems to use for LPARs
- **storage**: VIOS or SVC storage configuration
- **network**: Cluster networking (domain, CIDR, gateway, etc.)
- **openshift**: OpenShift configuration (version, pull secret, SSH key, etc.)
- **sno_node** / **bootstrap** / **masters** / **workers**: Node definitions
- **deployment**: Deployment phases and timeouts
- **advanced**: Advanced options (parallel operations, monitoring, etc.)

## Usage

### Cluster Management Commands

The deployer provides several commands for managing multiple OpenShift clusters:

```bash
# List all managed clusters
./ocp-upi-deployer -command list

# Deploy a new cluster (config file required)
./ocp-upi-deployer -command deploy -config config.yaml -cluster ocp-sno

# Resume a failed deployment (loads config from cluster directory)
./ocp-upi-deployer -command deploy -cluster ocp-sno -resume

# Check cluster status
./ocp-upi-deployer -command status -cluster ocp-sno

# Delete a cluster (loads config from cluster directory)
./ocp-upi-deployer -command delete -cluster ocp-sno
```

### Cluster Deletion with Partial Failure Handling

The delete command now includes **intelligent partial failure handling** that ensures safe and reliable cleanup:

**Key Features**:
- ✅ **Idempotent**: Safe to re-run multiple times
- ✅ **Partial Failure Recovery**: Tracks and retries only failed deletions
- ✅ **State Preservation**: Failed resources remain in state for retry
- ✅ **Clear Reporting**: Shows exactly what succeeded and what failed
- ✅ **No Orphaned Resources**: Everything tracked until successfully deleted

**Deletion Process** (4 steps):
1. **Close Virtual Terminals & Power Off LPARs** - Graceful shutdown
2. **Unmap Storage** - Batch unmapping from LPARs
3. **Delete Volumes** - Remove virtual disks from VIOS/SVC
4. **Delete LPARs** - Remove partitions from HMC

**Example - Partial Failure Scenario**:
```bash
$ ./ocp-upi-deployer -command delete -cluster ocp-sno

Step 3: Deleting storage volumes...
  Deleting volume: snonew5-n-b-a3f9...
    ⚠ Failed to delete disk snonew5-n-b-a3f9: disk in use
  Deleting volume: snonew5-n-d-b7c2...
    ✅ Deleted virtual disk: snonew5-n-d-b7c2

Step 4: Deleting LPARs...
  Deleting LPAR: sno-new-5...
    ✅ LPAR deleted successfully

Error: infrastructure deletion completed with errors.
The following resources remain: Volume: snonew5-n-b-a3f9

# State file now contains ONLY the failed volume
# Re-run delete to retry only the failed resource:
$ ./ocp-upi-deployer -command delete -cluster ocp-sno
```

**After successful deletion**, you'll be prompted:
```
Do you want to remove the cluster directory? (y/n):
```
- **Yes**: Removes `clusters/ocp-sno/` completely
- **No**: Preserves directory for audit/reference

### Cluster Directory Structure

Each cluster is managed in its own directory under `clusters/<cluster-name>/`:

```
./
├── ocp-upi-deployer              # Single binary
├── clusters/                     # Root directory for all clusters
│   ├── ocp-sno/
│   │   ├── config.yaml          # Copy of config used for deployment
│   │   └── state.json           # Deployment state tracking
│   ├── ocp-prod/
│   │   ├── config.yaml
│   │   └── state.json
│   └── ocp-test/
│       ├── config.yaml
│       └── state.json
```

**Benefits**:
- Single binary manages unlimited clusters
- Each cluster has isolated configuration and state
- Config file automatically preserved during deployment
- Easy to backup/restore individual clusters
- Optional directory cleanup on deletion

### Prerequisites

1. **Helper/Bastion Node**:
   - RHEL 8/9 or compatible Linux
   - Root SSH access
   - Network connectivity to Power systems and HMC
   - Sufficient disk space for RHCOS images (~5GB per cluster)

2. **HMC**:
   - HMC with REST API enabled
   - User credentials with LPAR management permissions

3. **Power Systems**:
   - Managed by HMC
   - Sufficient resources (CPU, memory, storage)
   - Virtual switches configured

4. **Storage**:
   - VIOS with volume groups OR
   - SVC with storage pools

5. **Network**:
   - DHCP range for cluster nodes
   - DNS forwarders configured
   - VIP pool for API and Ingress endpoints

### Installation

```bash
# Clone the repository
git clone https://github.com/sudeeshjohn/powerhmc-go.git
cd powerhmc-go/examples/ocp-upi-deployer

# Build the tool
go build -o ocp-upi-deployer

# Verify installation
./ocp-upi-deployer --version
```

### Deployment Workflow

**NOTE**: The main.go entry point is not yet implemented. The following is the intended workflow:

```bash
# 1. Validate configuration
./ocp-upi-deployer validate --config config.yaml

# 2. Deploy cluster(s)
./ocp-upi-deployer deploy --config config.yaml

# 3. Check deployment status
./ocp-upi-deployer status --config config.yaml

# 4. Delete cluster(s)
./ocp-upi-deployer delete --config config.yaml --cluster ocp-sno
```

### Manual Testing of Components

Since the orchestrator is not yet complete, you can test individual components:

```go
// Example: Test SSH connection
sshClient, err := NewSSHClient(helperConfig)
if err != nil {
    log.Fatal(err)
}
defer sshClient.Close()

output, err := sshClient.ExecuteCommand("hostname")
fmt.Println(output)

// Example: Generate DNSmasq config
dnsmasq := NewDNSmasqManager(ctx, sshClient)
if err := dnsmasq.Configure(); err != nil {
    log.Fatal(err)
}

// Example: Generate ignition configs
ignition := NewIgnitionGenerator(ctx, sshClient)
if err := ignition.Generate(); err != nil {
    log.Fatal(err)
}
```

## Implementation Roadmap

### Phase 1: Complete LPAR Provisioner ✅ (Skeleton Done)

The LPAR provisioner framework is in place but needs full implementation:

1. **LPAR Creation**:
   - Use `powerhmc-go` to create LPARs with specified resources
   - Reference: `powerhmc-go/examples/createlpar/main.go`
   - Implement processor, memory, and network configuration

2. **Storage Attachment**:
   - VIOS: Create virtual disks and map to LPARs
   - SVC: Create volumes, map to hosts, create physical volume mappings
   - Reference: `powerhmc-go/examples/createvirtualdisk` and `createphyvolmap`

3. **MAC Address Capture**:
   - Query network adapters after LPAR creation
   - Extract MAC addresses for DHCP/PXE configuration

4. **Power Operations**:
   - Implement network boot power-on
   - Implement graceful and immediate power-off
   - Reference: `powerhmc-go/examples/poweronpartition`

### Phase 2: Create Deployment Orchestrator ⏳ (Not Started)

Create `orchestrator.go` to coordinate all deployment phases:

```go
type Orchestrator struct {
    config      *MultiClusterConfig
    hmcClient   *hmc.HmcRestClient
    sshClient   *SSHClient
    state       *DeploymentState
}

func (o *Orchestrator) Deploy(clusterName string) error {
    // 1. Validate configuration
    // 2. Setup helper node services
    // 3. Download RHCOS images and tools
    // 4. Generate ignition configs
    // 5. Configure PXE boot
    // 6. Create LPARs
    // 7. Power on and monitor
    // 8. Wait for installation complete
    // 9. Save kubeconfig
}
```

Key features:
- Phase-by-phase execution with state saving
- Rollback on failure (if cleanup_on_failure enabled)
- Progress monitoring and logging
- Parallel operations support (if enabled)

### Phase 3: Create Main Entry Point ⏳ (Not Started)

Create `main.go` with CLI interface:

```go
func main() {
    // Parse flags
    cmd := flag.String("command", "", "Command: deploy, delete, validate, status")
    configFile := flag.String("config", "config.yaml", "Configuration file")
    clusterName := flag.String("cluster", "", "Cluster name (optional)")
    verbose := flag.Bool("verbose", false, "Verbose output")
    flag.Parse()

    // Load configuration
    config, err := LoadMultiClusterConfig(*configFile)
    
    // Execute command
    switch *cmd {
    case "validate":
        // Validate configuration
    case "deploy":
        // Deploy cluster(s)
    case "delete":
        // Delete cluster(s)
    case "status":
        // Show deployment status
    }
}
```

### Phase 4: Testing and Documentation ⏳ (Partial)

1. **Unit Tests**: Add tests for each component
2. **Integration Tests**: End-to-end deployment tests
3. **Documentation**: Complete usage guide, troubleshooting, examples
4. **CI/CD**: Automated builds and tests

## Directory Structure

```
ocp-upi-deployer/
├── README.md                    # This file
├── DESIGN.md                    # Architecture documentation
├── go.mod                       # Go module definition
├── go.sum                       # Go dependencies (generated)
├── main.go                      # Main entry point (NOT YET CREATED)
├── types.go                     # Configuration types ✅
├── validator.go                 # Configuration validator ✅
├── ssh.go                       # SSH client ✅
├── dnsmasq.go                   # DNSmasq configuration ✅
├── haproxy.go                   # HAProxy configuration ✅
├── httpserver.go                # HTTP server setup ✅
├── downloader.go                # RHCOS/tools downloader ✅
├── httphelper.go                # HTTP helper script generator ✅
├── ignition.go                  # Ignition generator ✅
├── pxeboot.go                   # PXE boot manager ✅
├── lpar.go                      # LPAR provisioner ⚠️ (skeleton)
├── orchestrator.go              # Deployment orchestrator (NOT YET CREATED)
├── config-sno.yaml              # SNO example configuration ✅
├── config-multi.yaml            # Multi-node example configuration ✅
└── templates/                   # Configuration templates
    ├── dnsmasq.d/              # DNSmasq templates
    ├── httpd.d/                # Apache templates
    └── ...
```

## Troubleshooting

### Common Issues

1. **SSH Connection Failures**:
   - Verify SSH key permissions (`chmod 600 ~/.ssh/id_rsa`)
   - Check helper node firewall rules
   - Verify SSH user has sudo privileges

2. **HMC Connection Failures**:
   - Verify HMC IP and credentials
   - Check HMC REST API is enabled
   - Verify network connectivity

3. **LPAR Creation Failures**:
   - Check Power system resources (CPU, memory)
   - Verify VIOS/SVC configuration
   - Check HMC logs for detailed errors

4. **PXE Boot Failures**:
   - Verify DHCP configuration
   - Check TFTP directory permissions
   - Verify GRUB configuration files
   - Check network boot order in LPAR profile

5. **Ignition Failures**:
   - Verify pull secret is valid
   - Check SSH public key format
   - Verify RHCOS image URLs are accessible
   - Check ignition file syntax

6. **Network Boot "No Network Adapters Found" Error**:
   ```
   [HMC] Job status: FAILED_BEFORE_COMPLETION
   lpar_netboot : No network adapters found
   lpar_netboot: Unable to obtain network adapter information. Quitting.
   ```
   
   **Root Cause**: LPAR exists but has no network adapter attached. This typically occurs when:
   - LPAR was created in a previous run but network adapter creation failed
   - Network adapter was manually deleted from the LPAR
   - Attempting to netboot an LPAR created without a network adapter
   
   **Solution**: The deployer now verifies network adapter exists before attempting netboot and provides clear recovery steps:
   ```
   LPAR sno-new-2 has no network adapters attached
   This usually means the LPAR was created but network adapter creation failed.
   Solution:
     1. Delete the LPAR from HMC
     2. Delete deployment-state-sno-new-2.json
     3. Re-run deployment from create_lpars phase:
        ./main -command deploy -config config.yaml -cluster sno-new-2 -phases create_lpars,setup_dnsmasq,power_on
   ```
   
   **Prevention**: The fix ensures proper state management by:
   - Storing both `SystemName` and `SystemUUID` in LPARState
   - Verifying network adapter exists before netboot attempt
   - Providing actionable error messages with recovery steps
   
   See [`NETBOOT_FIX.md`](NETBOOT_FIX.md) for technical details.

7. **Network Boot Location Code Errors**:
   ```
   lpar_netboot: can not find physical location U9105.22A.789C301-V10-C2-T1.
   actual location is U9105.22A.789C301-V10-C2-T0
   ```
   
   **Root Cause**: HMC API returns location codes without port suffix (`-T0` or `-T1`), but netboot requires the full location code with suffix.
   
   **Solution**: The deployer automatically:
   - Fetches base location code from HMC using `GetClientNetworkAdapters()`
   - Tries netboot with `-T0` suffix first (most common)
   - If that fails with location error, retries with `-T1` suffix
   - Caches the working suffix for future boots
   
   **Manual Verification**: Use the test utility to check location codes:
   ```bash
   cd powerhmc-go/examples/test-location-code
   go build
   ./test-location-code \
     -hmc-ip 192.0.2.1 \
     -hmc-user REDACTED_HMC_USER<== \
     -hmc-pass <password> \
     -system-name <system-name> \
     -lpar-name <lpar-name>
   ```
   
   Then verify in LPAR SMS menu: `Boot Options` → `Select Boot Options` → `Network` to see the actual location code with suffix.

## Contributing

Contributions are welcome! Please focus on:

1. **Completing LPAR Provisioner**: Full implementation using powerhmc-go
2. **Creating Orchestrator**: Coordinate all deployment phases
3. **Creating Main Entry Point**: CLI interface
4. **Adding Tests**: Unit and integration tests
5. **Improving Documentation**: Usage examples, troubleshooting

## References

- [OpenShift UPI Documentation](https://docs.openshift.com/container-platform/latest/installing/installing_ibm_power/installing-ibm-power.html)
- [ocp4-helpernode](https://github.com/RedHatOfficial/ocp4-helpernode)
- [powerhmc-go](https://github.com/sudeeshjohn/powerhmc-go)
- [IBM Power Systems Documentation](https://www.ibm.com/docs/en/power-systems)

## License

This project is part of the powerhmc-go repository and follows the same license.

## Authors

- Sudeesh John (@sudeeshjohn)
- Built with assistance from Bob (AI Assistant)

## Acknowledgments

- Based on the ocp4-helpernode Ansible playbooks
- Uses powerhmc-go for HMC REST API interactions
- Inspired by the need for native Go implementation without Ansible dependencies