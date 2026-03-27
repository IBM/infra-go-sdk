# SNO Deployment Workflow Analysis

Based on: https://github.com/cs-zhang/ocp4-ai-powervm/blob/main/docs/SNO.md

## Key Insights from SNO Documentation

### 1. Resource Requirements

| Component | vCPU | Memory | Storage |
|-----------|------|--------|---------|
| Bastion   | 2    | 8GB    | 50GB    |
| SNO       | 8    | 16GB   | 120GB   |

**Important**: SNO node needs **static IP with internet access**

### 2. Bastion Setup Requirements

The bastion node must have:
- **SELinux in permissive mode** (`/etc/selinux/config` → `SELINUX=permissive`)
- **Services**: dnsmasq (DNS+DHCP+PXE), httpd (HTTP), tftp (TFTP)
- **Packages**: Ansible, git, wget, httpd, etc.

### 3. Complete SNO Deployment Workflow

```
Phase 1: Bastion Setup
├─► Install packages (Ansible, httpd, dnsmasq, etc.)
├─► Configure SELinux to permissive
├─► Configure dnsmasq (DNS, DHCP, TFTP, PXE)
├─► Configure httpd
└─► Download RHCOS images

Phase 2: Generate Ignition Config
├─► Download openshift-install binary
├─► Create install-config.yaml for SNO
├─► Generate single-node-ignition-config
├─► Copy ignition file to HTTP directory
└─► Configure PXE boot with ignition URL

Phase 3: SNO Installation
├─► Power on SNO LPAR (PXE boot)
├─► LPAR boots from network
├─► Downloads RHCOS kernel/initramfs via TFTP
├─► Downloads ignition config via HTTP
├─► Installs RHCOS and applies ignition
└─► OpenShift installation begins automatically

Phase 4: Monitor Installation
├─► Wait for bootstrap complete (~10-15 min)
├─► Wait for installation complete (~30-40 min)
└─► Verify cluster is operational
```

## Critical Differences from Our Current Implementation

### ✅ What We Got Right

1. **Phase 1-2**: Create LPARs with proper resources ✅
2. **Phase 3**: Setup helper node with Ansible ✅
3. **SSH Remote Execution**: Trigger Ansible from laptop ✅

### ⚠️ What Needs Adjustment

#### 1. **Bastion Setup (Phase 3) - Needs Enhancement**

**Current**: Runs Ansible playbook from ocp4-ai-powervm repo
**Issue**: That playbook is for multi-node clusters, not SNO-specific

**What we need to do**:
```bash
# After Ansible playbook completes, we need to:
1. Download RHCOS images (kernel, initramfs, rootfs)
2. Configure dnsmasq for SNO (single MAC address)
3. Configure PXE boot files
4. Set up HTTP directory structure
```

#### 2. **Generate Ignition (Phase 4) - NEW PHASE NEEDED**

**Current**: Not implemented
**Required**:
```bash
# On bastion node:
1. Download openshift-install binary
2. Create install-config.yaml with SNO settings
3. Run: openshift-install create single-node-ignition-config
4. Copy ignition file to /var/www/html/ignition/
5. Configure grub.cfg for PXE boot with ignition URL
```

#### 3. **Power On SNO (Phase 5) - Simplified**

**Current**: Planned as separate phase
**Required**: Just power on the LPAR - it will PXE boot automatically

#### 4. **Monitor Installation (Phase 6) - NEW PHASE NEEDED**

**Current**: Not implemented
**Required**:
```bash
# Monitor from bastion:
1. Watch bootstrap progress
2. Wait for installation complete
3. Export kubeconfig
4. Verify cluster access
```

## Updated Phase Breakdown

### Phase 1: validate_config ✅
- Validate configuration file
- Check HMC connectivity

### Phase 2: check_resources ✅
- Verify managed system resources
- Check VIOS availability

### Phase 3: create_lpars ✅
- Create bastion LPAR (2 vCPU, 8GB, 50GB)
- Create SNO LPAR (8 vCPU, 16GB, 120GB)
- Attach network adapters
- Capture MAC addresses

### Phase 4: setup_helper_node ✅ (with enhancements needed)
**Current**: Runs Ansible playbook
**Enhancement needed**:
```go
func (o *Orchestrator) phaseSetupHelperNode() error {
    // 1. Run Ansible playbook (current implementation) ✅
    
    // 2. Download RHCOS images (NEW)
    o.downloadRHCOSImages()
    
    // 3. Configure dnsmasq for SNO (NEW)
    o.configureDnsmasqForSNO()
    
    // 4. Setup PXE boot files (NEW)
    o.setupPXEBoot()
    
    return nil
}
```

### Phase 5: generate_ignition_sno (NEW - CRITICAL)
**Purpose**: Generate SNO-specific ignition configuration

```go
func (o *Orchestrator) phaseGenerateIgnition() error {
    // 1. Download openshift-install binary
    o.downloadOpenshiftInstall()
    
    // 2. Create install-config.yaml for SNO
    o.createInstallConfig()
    
    // 3. Generate single-node ignition config
    o.generateSNOIgnition()
    
    // 4. Copy ignition to HTTP directory
    o.deployIgnitionFile()
    
    // 5. Configure PXE grub.cfg with ignition URL
    o.configurePXEGrub()
    
    return nil
}
```

### Phase 6: power_on_sno_master (SIMPLIFIED)
**Purpose**: Power on SNO LPAR to start PXE boot

```go
func (o *Orchestrator) phasePowerOnSNO() error {
    // Just power on - PXE boot happens automatically
    snoLPAR := o.state.CreatedLPARs["sno-master"]
    return o.hmcClient.PowerOnPartition(snoLPAR.UUID, nil)
}
```

### Phase 7: monitor_sno_installation (NEW - CRITICAL)
**Purpose**: Monitor installation progress and verify completion

```go
func (o *Orchestrator) phaseMonitorInstallation() error {
    // 1. Wait for bootstrap complete (~10-15 min)
    o.waitForBootstrapComplete()
    
    // 2. Wait for installation complete (~30-40 min)
    o.waitForInstallComplete()
    
    // 3. Export kubeconfig
    o.exportKubeconfig()
    
    // 4. Verify cluster access
    o.verifyClusterAccess()
    
    return nil
}
```

## Key Files and Configurations

### 1. install-config.yaml (for SNO)
```yaml
apiVersion: v1
baseDomain: <domain>
compute:
  - name: worker
    replicas: 0  # SNO has no workers
controlPlane:
  name: master
  replicas: 1    # Single node
metadata:
  name: <cluster_id>
networking:
  clusterNetwork:
    - cidr: 10.128.0.0/14
      hostPrefix: 23
  machineNetwork:
    - cidr: REDACTED_INTERNAL_CIDR<==/20
  networkType: OVNKubernetes
  serviceNetwork:
    - 172.30.0.0/16
platform:
  none: {}
bootstrapInPlace:
  installationDisk: <device>  # e.g., /dev/sda
pullSecret: '<pull_secret>'
sshKey: |
  <ssh_key>
```

### 2. dnsmasq Configuration
```conf
# DNS
domain-needed
bogus-priv
enable-ra
bind-dynamic
no-hosts
expand-hosts

# DHCP
dhcp-range=REDACTED_INTERNAL_IP<==,static
dhcp-option=option:router,REDACTED_INTERNAL_IP<==
dhcp-option=option:netmask,255.255.240.0
dhcp-option=option:dns-server,REDACTED_INTERNAL_IP<==

# SNO LPAR MAC address mapping
dhcp-host=REDACTED_MAC_ADDR<==,sno-82,REDACTED_INTERNAL_IP<==,infinite

# PXE Boot
enable-tftp
tftp-root=/var/lib/tftpboot
dhcp-boot=boot/grub2/powerpc-ieee1275/core.elf
```

### 3. PXE grub.cfg
```conf
default=0
fallback=1
timeout=1

if [ ${net_default_mac} == REDACTED_MAC_ADDR<== ]; then
  default=0
  fallback=1
  timeout=1
  
  menuentry "CoreOS (BIOS)" {
    echo "Loading kernel"
    linux /rhcos/kernel ip=dhcp rd.neednet=1 ignition.platform.id=metal ignition.firstboot ignition.config.url=http://REDACTED_INTERNAL_IP<==/ignition/sno.ign
    echo "Loading initrd"
    initrd /rhcos/initramfs.img
  }
fi
```

## Implementation Priority

### High Priority (Must Have for SNO)
1. ✅ Phase 3: Create LPARs with correct resources
2. ✅ Phase 4: Setup bastion with Ansible
3. 🔄 Phase 4 Enhancement: Download RHCOS images
4. 🔄 Phase 4 Enhancement: Configure dnsmasq for SNO
5. ❌ Phase 5: Generate SNO ignition config
6. ❌ Phase 6: Power on SNO LPAR
7. ❌ Phase 7: Monitor installation

### Medium Priority (Nice to Have)
- Automatic RHCOS version detection
- Retry logic for downloads
- Better error messages
- Installation progress bar

### Low Priority (Future Enhancement)
- Multi-SNO support
- Custom network configurations
- Air-gapped installation support

## Next Steps

1. **Enhance Phase 4** (setup_helper_node):
   - Add RHCOS image download
   - Configure dnsmasq with SNO MAC address
   - Setup PXE boot files

2. **Implement Phase 5** (generate_ignition_sno):
   - Download openshift-install
   - Create install-config.yaml
   - Generate SNO ignition
   - Deploy to HTTP directory

3. **Implement Phase 6** (power_on_sno_master):
   - Simple power-on operation
   - PXE boot happens automatically

4. **Implement Phase 7** (monitor_sno_installation):
   - Monitor bootstrap progress
   - Wait for installation complete
   - Verify cluster access

## Questions to Clarify

1. **RHCOS Version**: Which version should we download? (from config or auto-detect from OCP version)
2. **Pull Secret**: Where does user provide it? (in config file or separate file)
3. **SSH Key**: Use same key as bastion SSH or generate new one?
4. **Installation Disk**: How to determine device name? (/dev/sda, /dev/vda, or by-id?)
5. **Monitoring**: Should we SSH to bastion to monitor, or use HMC console?

## Summary

The SNO documentation reveals that our current implementation is **80% complete** but needs:

1. **Phase 4 enhancements**: RHCOS download, dnsmasq config, PXE setup
2. **Phase 5 (new)**: Ignition generation - **CRITICAL**
3. **Phase 6 (simplified)**: Just power on
4. **Phase 7 (new)**: Installation monitoring - **CRITICAL**

The good news: Our SSH remote execution framework is perfect for this! We can trigger all these steps remotely from the laptop.