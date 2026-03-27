# OpenShift UPI Deployer Architecture

## Overview

A hybrid Go + Ansible tool for deploying OpenShift Container Platform on IBM Power Systems using User-Provisioned Infrastructure (UPI) mode.

## Design Philosophy

**Keep It Simple**: Use DNS round-robin for load distribution instead of HAProxy, reducing complexity while maintaining functionality.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    User Input                                │
│  - config.yaml (cluster configuration)                       │
│  - Pre-existing helper node with RHEL/CentOS                │
└─────────────────────────────────────────────────────────────┘
                            ↓
┌─────────────────────────────────────────────────────────────┐
│              Go Tool (ocp-upi-deployer)                      │
│                                                              │
│  Phase 1: Helper Node Setup                                 │
│    ✓ Configure dnsmasq (DHCP + DNS + TFTP)                 │
│    ✓ Configure Apache/httpd                                 │
│    ✓ Use DNS round-robin for load distribution             │
│                                                              │
│  Phase 2: LPAR Provisioning                                 │
│    ✓ Create LPARs via HMC API                              │
│    ✓ Attach storage (SVC/VIOS/Physical)                    │
│    ✓ Configure network adapters                             │
│    ✓ Generate MAC addresses                                 │
│                                                              │
│  Phase 3: Ansible Integration                               │
│    ✓ Generate vars.yaml for ocp4-ai-powervm                │
│    ✓ Copy to helper node                                    │
│    ✓ Provide next steps                                     │
└─────────────────────────────────────────────────────────────┘
                            ↓
┌─────────────────────────────────────────────────────────────┐
│         Ansible Playbook (ocp4-ai-powervm)                   │
│                                                              │
│    ✓ Download RHCOS images                                  │
│    ✓ Generate ignition configs                              │
│    ✓ Setup PXE boot                                          │
│    ✓ Power on cluster nodes                                  │
│    ✓ Monitor installation                                    │
│    ✓ Complete OpenShift setup                                │
└─────────────────────────────────────────────────────────────┘
                            ↓
┌─────────────────────────────────────────────────────────────┐
│              OpenShift Cluster Ready                         │
└─────────────────────────────────────────────────────────────┘
```

## Components

### 1. Helper Node Services

#### dnsmasq (All-in-One)
- **DHCP Server**: Static IP reservations for cluster nodes
- **DNS Server**: DNS records with round-robin load distribution
- **TFTP Server**: PXE boot files for RHCOS installation

#### Apache/httpd
- **Ignition Files**: Serve cluster configuration
- **RHCOS Images**: Serve kernel, initramfs, rootfs

### 2. DNS Round-Robin Load Distribution

Instead of HAProxy, we use DNS round-robin for simplicity:

**API Endpoints:**
```
api.ocp4.example.com → REDACTED_LAB_IP<== (master0)
api.ocp4.example.com → 192.168.7.22 (master1)
api.ocp4.example.com → 192.168.7.23 (master2)
```

**Wildcard Apps:**
```
*.apps.ocp4.example.com → REDACTED_LAB_GW<==1 (worker0)
*.apps.ocp4.example.com → REDACTED_LAB_GW<==2 (worker1)
*.apps.ocp4.example.com → REDACTED_LAB_GW<==3 (worker2)
```

**How it works:**
1. Client queries DNS
2. dnsmasq returns all IPs
3. Client picks one (rotates on subsequent queries)
4. Traffic distributed across nodes

**Limitations:**
- No health checks
- No session persistence
- Basic load distribution

**Good for:**
- Development environments
- Test clusters
- Small deployments
- Edge deployments

## Directory Structure

```
ocp-upi-deployer/
├── config.yaml                      # Full HA cluster config
├── config-sno.yaml                  # Single Node OpenShift config
├── ARCHITECTURE.md                  # This file
├── README.md                        # User documentation
├── main.go                          # Main program
├── types.go                         # Configuration types
├── validator.go                     # Configuration validator
├── go.mod                           # Go dependencies
└── templates/
    ├── ansible-vars.yaml.tmpl       # Ansible vars generation
    ├── setup-helper.sh.tmpl         # Helper setup script
    ├── dnsmasq.d/
    │   ├── README.md                # dnsmasq multi-cluster guide
    │   ├── global.conf.tmpl         # Global dnsmasq settings
    │   ├── dhcp.conf.tmpl           # DHCP configuration
    │   ├── dns.conf.tmpl            # DNS with round-robin
    │   └── tftp.conf.tmpl           # TFTP/PXE configuration
    └── httpd.d/
        ├── README.md                # httpd multi-cluster guide
        └── vhost.conf.tmpl          # Virtual host configuration
```

## Multi-Cluster Support

All services use **flat configuration with cluster-prefixed filenames**:

### dnsmasq (`/etc/dnsmasq.d/`)
```
global.conf                    # Shared settings
ocp4-prod-dhcp.conf           # Cluster 1 DHCP
ocp4-prod-dns.conf            # Cluster 1 DNS
ocp4-prod-tftp.conf           # Cluster 1 TFTP
ocp4-dev-dhcp.conf            # Cluster 2 DHCP
ocp4-dev-dns.conf             # Cluster 2 DNS
ocp4-dev-tftp.conf            # Cluster 2 TFTP
```

### httpd (`/etc/httpd/conf.d/`)
```
ocp4-prod-vhost.conf          # Cluster 1 virtual host
ocp4-dev-vhost.conf           # Cluster 2 virtual host
```

### File System (`/var/www/html/`)
```
ocp4-prod/
├── ignition/
└── images/
ocp4-dev/
├── ignition/
└── images/
```

## Configuration Files

### config.yaml
Main configuration for full HA cluster:
- HMC connection details
- Multiple Power systems
- Storage backend (SVC/VIOS/Physical)
- Network configuration
- Helper node (pre-existing)
- Bootstrap node
- 3 Master nodes
- 3 Worker nodes

### config-sno.yaml
Configuration for Single Node OpenShift:
- Simplified structure with `sno_node` instead of `masters`/`workers`
- No bootstrap node section
- Single node acts as control plane + worker
- No separate worker nodes
- Reduced resource requirements
- No HAProxy or load balancing needed

## Deployment Workflow

### Step 1: Prepare
```bash
# Edit configuration
vi config.yaml

# Ensure helper node is accessible
ssh root@helper-node
```

### Step 2: Run Go Tool
```bash
./ocp-upi-deployer --config config.yaml

# Output:
# ✓ Helper node configured (dnsmasq + httpd)
# ✓ LPARs created (bootstrap + 3 masters + 3 workers)
# ✓ Storage attached
# ✓ Network configured
# ✓ Generated vars.yaml
# ✓ Ready for Ansible playbook
```

### Step 3: Run Ansible Playbook
```bash
# SSH to helper node
ssh root@helper-node

# Clone playbook
git clone https://github.com/cs-zhang/ocp4-ai-powervm.git
cd ocp4-ai-powervm

# Run playbook
ansible-playbook -i inventory playbook.yml -e @/root/vars.yaml
```

### Step 4: Access Cluster
```bash
# Get kubeconfig
export KUBECONFIG=/root/ocp4/auth/kubeconfig

# Verify cluster
oc get nodes
oc get co
```

## Benefits

### ✅ Simplicity
- Only 2 services: dnsmasq + httpd
- No HAProxy complexity
- Fewer moving parts

### ✅ Multi-Cluster
- Multiple clusters on same helper
- Isolated configurations
- Easy management

### ✅ Hybrid Approach
- Go for infrastructure automation
- Ansible for OpenShift installation
- Best of both worlds

### ✅ Proven Tools
- Uses battle-tested ocp4-ai-powervm playbook
- Community-maintained
- Well-documented

### ✅ Flexibility
- Supports full HA and SNO
- Multiple storage backends
- Multi-system distribution

## Limitations

### DNS Round-Robin
- No health checks
- No session persistence
- Basic load distribution
- Not suitable for production with strict SLAs

### Workarounds
- For production: Add HAProxy later
- For edge: DNS round-robin is sufficient
- For dev/test: Perfect as-is

## Future Enhancements

1. **Optional HAProxy**: Add HAProxy support for production
2. **Monitoring**: Add cluster health monitoring
3. **Upgrades**: Support cluster upgrades
4. **Backup/Restore**: Add etcd backup automation
5. **Multi-Arch**: Support x86_64 alongside ppc64le

## References

- [ocp4-ai-powervm Playbook](https://github.com/cs-zhang/ocp4-ai-powervm)
- [OpenShift UPI Documentation](https://docs.openshift.com/container-platform/latest/installing/installing_platform_agnostic/installing-platform-agnostic.html)
- [IBM Power Systems Documentation](https://www.ibm.com/docs/en/power10)
- [dnsmasq Documentation](http://www.thekelleys.org.uk/dnsmasq/doc.html)