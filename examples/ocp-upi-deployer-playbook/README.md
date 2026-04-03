# OpenShift UPI Deployer for IBM Power Systems

Automated deployment tool for OpenShift Container Platform in User-Provisioned Infrastructure (UPI) mode on IBM Power Systems managed by Hardware Management Console (HMC).

## Overview

This tool automates the complete deployment workflow:

1. **LPAR Provisioning**: Creates LPARs on Power systems with specified resources
2. **Storage Provisioning**: Provisions storage volumes (SVC/VIOS/Physical)
3. **Network Configuration**: Configures virtual network adapters
4. **Helper Node Setup**: Deploys bastion node with DHCP, DNS, HAProxy, TFTP, HTTP, NFS
5. **OpenShift Deployment**: Deploys bootstrap, master, and worker nodes
6. **Installation Monitoring**: Monitors installation to completion

## Features

- ✅ **Multi-System Support**: Distribute LPARs across multiple Power systems for HA
- ✅ **Flexible Storage**: Supports SVC, VIOS, or physical storage backends
- ✅ **Comprehensive Validation**: Pre-flight checks for configuration and resources
- ✅ **State Management**: Resume deployment from any phase after failure
- ✅ **Parallel Operations**: Create LPARs in parallel for faster deployment
- ✅ **Detailed Logging**: Verbose logging for troubleshooting

## Prerequisites

### Software Requirements

- Go 1.21 or later
- Access to HMC managing Power systems
- Access to storage backend (SVC/VIOS)
- Red Hat OpenShift pull secret
- SSH key pair for cluster access

### Network Requirements

- Dedicated network segment for cluster nodes
- Gateway and DNS server access
- Internet connectivity (or disconnected registry)

### Resource Requirements

**Per Node Type:**

| Node Type | Min CPU | Min Memory | Min Storage |
|-----------|---------|------------|-------------|
| Helper    | 0.5 units | 8 GB | 120 GB + 200 GB data |
| Bootstrap | 1.0 units | 16 GB | 120 GB |
| Master    | 2.0 units | 32 GB | 120 GB + 100 GB etcd |
| Worker    | 2.0 units | 32 GB | 120 GB + 200 GB storage |

## Installation

```bash
# Clone the repository
cd powerhmc-go/examples/ocp-upi-deployer

# Install dependencies
go mod tidy

# Build the binary
go build -o ocp-upi-deployer
```

## Configuration

### 1. Create Configuration File

Copy and customize the sample configuration:

```bash
cp config.yaml my-cluster-config.yaml
```

### 2. Key Configuration Sections

#### HMC Connection
```yaml
hmc:
  ip: "192.0.2.1"
  username: "REDACTED_HMC_USER<=="
  password: "REDACTED_HMC_PASS<=="
```

#### Power Systems
```yaml
power_systems:
  - name: "System1"
    vswitch_name: "ETHERNET0"
    vlan_id: 1337
```

#### Storage Backend
```yaml
storage:
  type: "svc"  # or "vios" or "physical"
  svc:
    ip: "192.0.2.7"
    username: "REDACTED_SVC_USER<=="
    password: "REDACTED_HMC_PASS<=="
    pool_name: "Pool0"
    volume_prefix: "ocp_"
```

#### Node Distribution
```yaml
masters:
  nodes:
    - name: "master-0"
      ip: "REDACTED_LAB_GW<==0"
      system_name: "System1"  # Specify which Power system
    - name: "master-1"
      ip: "REDACTED_LAB_GW<==1"
      system_name: "System1"
    - name: "master-2"
      ip: "REDACTED_LAB_GW<==2"
      system_name: "System2"  # Distribute for HA
```

### 3. Prepare Required Files

```bash
# Download OpenShift pull secret from Red Hat
# https://console.redhat.com/openshift/install/pull-secret
cp ~/Downloads/pull-secret.json ./

# Generate SSH key if needed
ssh-keygen -t rsa -b 4096 -f ~/.ssh/id_rsa -N ""
```

## Usage

### Validate Configuration

```bash
./ocp-upi-deployer --config my-cluster-config.yaml --validate
```

### Deploy Cluster

```bash
./ocp-upi-deployer --config my-cluster-config.yaml
```

### Resume Failed Deployment

```bash
./ocp-upi-deployer --config my-cluster-config.yaml --resume create_lpars
```

### Command-Line Options

```
--config string     Path to configuration file (default "config.yaml")
--validate          Only validate configuration without deploying
--resume string     Resume deployment from a specific phase
--version           Show version information
```

## Deployment Phases

The deployment executes these phases in order:

1. **validate_config**: Validate YAML configuration
2. **check_resources**: Verify available resources on Power systems
3. **create_lpars**: Create all LPARs (helper, bootstrap, masters, workers)
4. **attach_storage**: Create and attach storage volumes
5. **configure_network**: Configure virtual network adapters
6. **power_on_helper**: Power on helper node
7. **setup_helper_node**: Install RHEL and run ansible playbook
8. **generate_ignition**: Generate ignition configs for cluster nodes
9. **power_on_bootstrap**: Power on bootstrap node
10. **power_on_masters**: Power on master nodes
11. **wait_bootstrap**: Wait for bootstrap to complete
12. **power_on_workers**: Power on worker nodes
13. **monitor_installation**: Monitor installation progress
14. **cleanup_bootstrap**: Remove bootstrap node after completion

## State Management

The tool saves deployment state to `deployment-state.json` after each phase. This allows you to:

- Resume deployment after failure
- Track progress
- Audit what was created

Example state file:
```json
{
  "current_phase": "power_on_masters",
  "completed_phases": ["validate_config", "check_resources", "create_lpars"],
  "created_lpars": {
    "helper": {
      "uuid": "abc-123",
      "status": "powered_on"
    }
  },
  "status": "in_progress"
}
```

## Troubleshooting

### Validation Errors

```bash
# Check configuration syntax
./ocp-upi-deployer --config my-cluster-config.yaml --validate

# Common issues:
# - Invalid IP addresses
# - Duplicate node names/IPs
# - Insufficient resources
# - Missing required files
```

### Deployment Failures

```bash
# Check deployment state
cat deployment-state.json

# Resume from last successful phase
./ocp-upi-deployer --config my-cluster-config.yaml --resume <phase_name>

# Enable verbose logging (in config.yaml)
deployment:
  verbose: true
```

### LPAR Creation Issues

- Verify HMC connectivity
- Check Power system has available resources
- Ensure vswitch exists and is accessible
- Verify VLAN ID is valid

### Storage Issues

- Verify SVC/VIOS connectivity
- Check storage pool has sufficient capacity
- Ensure volume prefix doesn't conflict with existing volumes

## Integration with Helper Node Playbook

This tool generates `helper-vars.yaml` compatible with the [Red Hat CoP ocp4-helpernode](https://github.com/redhat-cop/ocp4-helpernode) playbook.

The generated file includes:
- DHCP configuration for all nodes
- DNS entries for cluster services
- HAProxy configuration for API and ingress
- TFTP/HTTP configuration for PXE boot

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                         HMC                                  │
│                    (192.0.2.1)                           │
└────────────┬────────────────────────────┬───────────────────┘
             │                            │
    ┌────────▼────────┐          ┌───────▼────────┐
    │   Power System1  │          │  Power System2  │
    │  (LTC09U31-ZZ)  │          │  (LTC09U32-AA)  │
    └────────┬────────┘          └───────┬─────────┘
             │                            │
    ┌────────▼────────────────────────────▼─────────┐
    │              Network (VLAN 1337)               │
    │            192.168.7.0/24                      │
    └────────┬───────────────────────────────────────┘
             │
    ┌────────▼────────┐
    │  Helper Node     │  DHCP, DNS, HAProxy, TFTP, HTTP
    │  REDACTED_LAB_IP<==    │
    └──────────────────┘
             │
    ┌────────┴────────┬────────────┬────────────┐
    │                 │            │            │
┌───▼───┐      ┌──────▼──┐  ┌─────▼──┐  ┌─────▼──┐
│Bootstrap│    │Master-0 │  │Master-1│  │Master-2│
│  .20    │    │   .10   │  │  .11   │  │  .12   │
└─────────┘    └─────────┘  └────────┘  └────────┘
                     │            │            │
              ┌──────┴────────────┴────────────┴──────┐
              │                                        │
         ┌────▼────┐  ┌────────┐  ┌────────┐
         │Worker-0 │  │Worker-1│  │Worker-2│
         │   .30   │  │  .31   │  │  .32   │
         └─────────┘  └────────┘  └────────┘
```

## Contributing

Contributions are welcome! Please:

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests if applicable
5. Submit a pull request

## License

This project is licensed under the Apache License 2.0 - see the LICENSE file for details.

## Support

For issues and questions:
- GitHub Issues: [Create an issue](https://github.com/sudeeshjohn/powerhmc-go/issues)
- Documentation: [IBM Power Systems Documentation](https://www.ibm.com/docs/en/power10)
- OpenShift Documentation: [Red Hat OpenShift Documentation](https://docs.openshift.com/)

## Acknowledgments

- [Red Hat CoP ocp4-helpernode](https://github.com/redhat-cop/ocp4-helpernode) - Helper node ansible playbook
- IBM Power Systems team for HMC REST API documentation
- OpenShift community for UPI deployment guides