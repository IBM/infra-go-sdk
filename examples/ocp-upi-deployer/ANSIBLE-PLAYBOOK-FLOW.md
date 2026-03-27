# ocp4-ai-powervm Ansible Playbook Flow Analysis

## Overview
This document explains how the [ocp4-ai-powervm](https://github.com/cs-zhang/ocp4-ai-powervm) Ansible playbook behaves with and without the `pvm_hmc` variable.

## Flow WITH pvm_hmc Set

When `pvm_hmc` is defined in vars.yaml (e.g., `pvm_hmc: "REDACTED_HMC_USER<==@192.0.2.100"`):

### Complete Automated Flow
```
1. Helper Node Setup
   ├── Install packages (Ansible, httpd, dnsmasq, tftp, wget, git)
   ├── Configure DNS (dnsmasq)
   ├── Configure DHCP (dnsmasq)
   ├── Configure TFTP server
   ├── Configure HTTP server
   ├── Download RHCOS images (kernel, initramfs, rootfs)
   ├── Download OpenShift installer and client
   └── Set up PXE boot configuration

2. Generate OpenShift Configs
   ├── Create install-config.yaml
   ├── Generate ignition configs
   └── Place ignition files in HTTP directory

3. **NETBOOT ROLE EXECUTES** (because pvm_hmc is defined)
   ├── SSH to HMC using pvm_hmc credentials
   ├── For each LPAR (bootstrap, masters, workers):
   │   ├── Power off LPAR if running
   │   ├── Execute HMC netboot command
   │   └── LPAR boots from network (PXE)
   └── LPARs download RHCOS and install

4. Monitor Installation
   ├── Wait for bootstrap complete
   ├── Wait for cluster operators
   └── Installation complete
```

### Key Points
- **Fully automated**: Ansible handles everything including powering on LPARs
- **Requires**: Password-less SSH access to HMC
- **HMC Commands**: Ansible executes HMC CLI commands via SSH to netboot LPARs
- **User Action**: Just run `ansible-playbook` and wait

### Ansible Playbook Structure (with pvm_hmc)
```yaml
# playbooks/roles/netboot/tasks/main.yaml
- name: Netboot LPARs via HMC
  when: pvm_hmc is defined
  block:
    - name: Power off LPAR
      shell: |
        ssh {{ pvm_hmc }} "chsysstate -r lpar -m {{ pvmcec }} -n {{ pvmlpar }} -o shutdown --immed"
    
    - name: Netboot LPAR
      shell: |
        ssh {{ pvm_hmc }} "lpar_netboot -t ent -m {{ macaddr }} -s auto \
          -d auto -S {{ server_ip }} -G {{ gateway }} -C {{ client_ip }} {{ pvmlpar }} \
          {{ profile_name }} {{ pvmcec}}"
```

## Flow WITHOUT pvm_hmc (Commented Out)

When `pvm_hmc` is NOT defined (commented out or omitted):

### Semi-Automated Flow
```
1. Helper Node Setup (SAME AS ABOVE)
   ├── Install packages
   ├── Configure DNS, DHCP, TFTP, HTTP
   ├── Download RHCOS images
   ├── Download OpenShift installer/client
   └── Set up PXE boot configuration

2. Generate OpenShift Configs (SAME AS ABOVE)
   ├── Create install-config.yaml
   ├── Generate ignition configs
   └── Place ignition files in HTTP directory

3. **NETBOOT ROLE SKIPPED** (pvm_hmc not defined)
   └── Ansible playbook completes here

4. **MANUAL STEP REQUIRED**
   └── User must power on LPARs manually:
       ├── Option A: Use HMC GUI
       ├── Option B: Use HMC CLI manually
       └── Option C: Use ocp-upi-deployer tool (Phase 6)

5. LPARs Boot
   ├── LPARs netboot from helper node
   ├── Download RHCOS via PXE
   └── Install and join cluster

6. Monitor Installation (manual or scripted)
   ├── Wait for bootstrap complete
   ├── Wait for cluster operators
   └── Installation complete
```

### Key Points
- **Semi-automated**: Ansible sets up infrastructure, user powers on LPARs
- **No HMC SSH required**: Ansible doesn't need HMC access
- **User Control**: You decide when and how to power on LPARs
- **Flexibility**: Can use different tools for LPAR management

## Comparison Table

| Aspect | WITH pvm_hmc | WITHOUT pvm_hmc |
|--------|--------------|-----------------|
| **Automation Level** | Fully automated | Semi-automated |
| **HMC SSH Access** | Required | Not required |
| **Netboot Role** | Executes | Skipped |
| **LPAR Power Control** | Ansible via HMC SSH | Manual or external tool |
| **User Intervention** | None (just run playbook) | Must power on LPARs |
| **Flexibility** | Less (Ansible controls timing) | More (you control timing) |
| **Best For** | Hands-off deployment | Controlled/staged deployment |

## Our ocp-upi-deployer Integration

### Current Approach: Ansible Handles Netboot (SIMPLIFIED)

```yaml
# In ansible-vars.yaml.tmpl (line 136)
pvm_hmc: "{{.HMC.Username}}@{{.HMC.IP}}"  # ENABLED
```

**Why We Enable pvm_hmc:**
1. **Simplicity**: Fully automated deployment - easier for customers
2. **Less Complexity**: No need to manage netboot in Go code
3. **Proven Solution**: Ansible playbook handles netboot reliably
4. **Single Tool**: Ansible does everything from setup to netboot
5. **Customer Friendly**: Just run the tool and wait - no manual steps

### Our Simplified Deployment Flow

```
Phase 1-3: Create LPARs
   └── ocp-upi-deployer creates LPARs via HMC REST API

Phase 4: Setup Helper Node & Netboot LPARs
   ├── ocp-upi-deployer generates ansible-vars.yaml (pvm_hmc enabled)
   ├── SSH to helper node
   ├── Run Ansible playbook
   ├── Ansible sets up infrastructure (DNS, DHCP, PXE, etc.)
   ├── Ansible EXECUTES netboot role (pvm_hmc is defined)
   │   ├── SSH to HMC using pvm_hmc credentials
   │   ├── Power off LPARs if running
   │   ├── Execute HMC netboot commands
   │   └── LPARs boot from network and install RHCOS
   └── Ansible completes

Phase 5: Monitor Installation
   └── ocp-upi-deployer monitors cluster installation
```

**Note**: Phase 6 (manual netboot via Go code) is no longer needed since Ansible handles it.

## When to Use Each Approach

### Use WITH pvm_hmc (Fully Automated) - **OUR CHOICE**
- ✅ Simple, straightforward deployments
- ✅ Trust Ansible to handle everything
- ✅ Customer-friendly (less complexity)
- ✅ One-shot deployment
- ✅ Proven, reliable solution
- ✅ Less code to maintain

### Use WITHOUT pvm_hmc (Semi-Automated)
- ✅ Need control over LPAR power operations
- ✅ Want to use HMC REST API instead of SSH
- ✅ Need to pause/resume between steps
- ✅ Want custom error handling and retry logic
- ✅ Staged/phased deployment approach
- ⚠️ More complex for end users

## Technical Details

### Netboot Role Conditional Logic
```yaml
# In playbooks/roles/netboot/tasks/main.yaml
- name: Include netboot tasks
  include_tasks: netboot_lpars.yaml
  when: pvm_hmc is defined and pvm_hmc != ""
```

### HMC Commands Used (when pvm_hmc is set)
```bash
# Power off LPAR
ssh REDACTED_HMC_USER<==@hmc "chsysstate -r lpar -m System -n lpar-name -o shutdown --immed"

# Netboot LPAR
ssh REDACTED_HMC_USER<==@hmc "lpar_netboot -t ent -m 52:54:00:xx:xx:xx \
  -s auto -d auto -S REDACTED_LAB_IP<== -G REDACTED_LAB_GW<== \
  -C REDACTED_LAB_IP<== lpar-name profile-name System"
```

### Our HMC REST API Approach (without pvm_hmc)
```go
// Phase 6: Power on via REST API
options := &hmc.PowerOnOptions{
    BootMode:        "netboot",
    ProfileUUID:     profileUUID,
    ClientIP:        "REDACTED_LAB_IP<==",
    ServerIP:        "REDACTED_LAB_IP<==",
    Gateway:         "REDACTED_LAB_GW<==",
    Netmask:         "255.255.255.0",
    ConnectionSpeed: "auto",
    DuplexMode:      "auto",
}
status, err := hmcClient.PowerOnPartition(lparUUID, options, true)
```

## Conclusion

**For ocp-upi-deployer**: We **enable** `pvm_hmc` because:
1. **Simplicity**: Fully automated deployment is easier for customers
2. **Less Code**: No need to implement netboot logic in Go
3. **Proven Solution**: Ansible playbook handles netboot reliably
4. **Single Workflow**: Ansible does everything in one run
5. **Customer Experience**: Just run the tool and wait - no manual intervention

The Ansible playbook does ALL the heavy lifting: DNS, DHCP, PXE, RHCOS downloads, AND netboot operations. This provides the best customer experience with minimal complexity.

### SSH Setup Requirement

Since we enable `pvm_hmc`, the tool must ensure:
1. Helper node can SSH to HMC without password (SSH key setup)
2. HMC credentials are provided in config.yaml
3. The `pvm_hmc` variable is properly formatted: `username@hmc_ip`

The ocp-upi-deployer tool handles this by:
- Reading HMC credentials from config.yaml
- Generating the correct `pvm_hmc` value in ansible-vars.yaml
- Ansible playbook uses this to SSH to HMC and execute netboot commands