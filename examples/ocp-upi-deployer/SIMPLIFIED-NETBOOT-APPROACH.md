# Simplified Netboot Approach - Design Decision

## Overview
This document explains the design decision to simplify the ocp-upi-deployer by letting Ansible handle netboot operations instead of managing them through Go code.

## Previous Approach (Complex)

### Architecture
```
Phase 1-3: Create LPARs (Go code via HMC REST API)
Phase 4: Setup Helper Node (Ansible playbook)
Phase 5: Generate Ignition Configs (Go code)
Phase 6: Netboot LPARs (Go code via HMC REST API)  ← COMPLEX
Phase 7: Monitor Installation (Go code)
```

### Issues with Previous Approach
1. **Complexity**: Required implementing netboot logic in Go using HMC REST API
2. **MAC to Location Code Translation**: Needed complex logic to convert MAC addresses to location codes
3. **Customer Confusion**: Users had to understand when to run Ansible vs when to run Go tool
4. **Dual Control**: Split responsibility between Ansible (setup) and Go (netboot)
5. **More Code**: Additional Go code to maintain for netboot operations
6. **Error Handling**: Had to implement retry logic and error handling in Go

### Configuration (Previous)
```yaml
# ansible-vars.yaml.tmpl
# pvm_hmc: "{{.HMC.Username}}@{{.HMC.IP}}"  # COMMENTED OUT
```

## New Approach (Simplified)

### Architecture
```
Phase 1-3: Create LPARs (Go code via HMC REST API)
Phase 4: Setup Helper Node & Netboot (Ansible playbook)  ← SIMPLIFIED
Phase 5: Monitor Installation (Go code)
```

### Benefits of New Approach
1. **Simplicity**: Fully automated - just run the tool and wait
2. **Less Code**: No need to implement netboot logic in Go
3. **Proven Solution**: Ansible playbook handles netboot reliably
4. **Single Workflow**: Ansible does everything in one run
5. **Customer Friendly**: No manual intervention required
6. **Less Maintenance**: Fewer lines of code to maintain

### Configuration (New)
```yaml
# ansible-vars.yaml.tmpl
pvm_hmc: "{{.HMC.Username}}@{{.HMC.IP}}"  # ENABLED
```

## What Changed

### 1. Template File Update
**File**: `powerhmc-go/examples/ocp-upi-deployer/templates/ansible-vars.yaml.tmpl`

**Before** (line 136):
```yaml
# pvm_hmc: "REDACTED_HMC_USER<==@192.0.2.100"
```

**After** (line 136):
```yaml
pvm_hmc: "{{.HMC.Username}}@{{.HMC.IP}}"
```

### 2. Documentation Updates
**File**: `ANSIBLE-PLAYBOOK-FLOW.md`

- Updated "Our ocp-upi-deployer Integration" section
- Changed from "Why We Keep pvm_hmc Commented Out" to "Current Approach: Ansible Handles Netboot (SIMPLIFIED)"
- Updated deployment flow to show Ansible handling netboot
- Removed Phase 6 (manual netboot) from workflow
- Updated conclusion to reflect new approach

### 3. Configuration Files
**Files**: `types.go`, `config-sno.yaml`

- HMC configuration already present (no changes needed)
- `HMCConfig` struct has `IP`, `Username`, `Password` fields
- config-sno.yaml already has HMC credentials section

## Technical Details

### How Ansible Netboot Works

When `pvm_hmc` is defined, Ansible:

1. **SSH to HMC**: Uses credentials from `pvm_hmc` variable
2. **Power Off LPARs**: Ensures clean state
   ```bash
   ssh REDACTED_HMC_USER<==@hmc "chsysstate -r lpar -m System -n lpar-name -o shutdown --immed"
   ```
3. **Execute Netboot**: Boots LPARs from network
   ```bash
   ssh REDACTED_HMC_USER<==@hmc "lpar_netboot -t ent -m 52:54:00:xx:xx:xx \
     -s auto -d auto -S REDACTED_LAB_IP<== -G REDACTED_LAB_GW<== \
     -C REDACTED_LAB_IP<== lpar-name profile-name System"
   ```
4. **LPARs Boot**: Download RHCOS via PXE and install

### Conditional Logic in Ansible
```yaml
# playbooks/roles/netboot/tasks/main.yaml
- name: Include netboot tasks
  include_tasks: netboot_lpars.yaml
  when: pvm_hmc is defined and pvm_hmc != ""
```

## Requirements

### SSH Setup
Since we enable `pvm_hmc`, the deployment requires:

1. **Password-less SSH**: Helper node must be able to SSH to HMC without password
2. **SSH Key Setup**: Public key must be added to HMC's authorized_keys
3. **HMC Credentials**: Must be provided in config.yaml

### Configuration
```yaml
# config-sno.yaml
hmc:
  ip: "192.0.2.1"
  username: "REDACTED_HMC_USER<=="
  password: "REDACTED_HMC_PASS<=="  # Used for initial setup, then SSH keys
```

## Customer Experience

### Before (Complex)
```
1. Run ocp-upi-deployer (creates LPARs)
2. Wait for Phase 4 to complete (Ansible setup)
3. Manually run Phase 6 or use HMC GUI to netboot
4. Wait for installation
```

### After (Simple)
```
1. Run ocp-upi-deployer
2. Wait for completion (everything automated)
```

## Migration Path

### For Existing Deployments
If you have existing deployments using the old approach:

1. **Update Template**: Uncomment `pvm_hmc` line in ansible-vars.yaml.tmpl
2. **Setup SSH Keys**: Ensure helper node can SSH to HMC
3. **Remove Phase 6**: No longer needed in workflow
4. **Update Documentation**: Reflect simplified flow

### For New Deployments
- Just use the updated configuration
- Ansible handles everything automatically
- No manual netboot steps required

## Conclusion

The simplified approach provides:
- ✅ Better customer experience (fully automated)
- ✅ Less code to maintain
- ✅ Proven, reliable solution (Ansible playbook)
- ✅ Single workflow (no split between Ansible and Go)
- ✅ Easier troubleshooting (all in Ansible logs)

The trade-off is requiring SSH access to HMC, but this is a reasonable requirement for automated deployments and is already supported by the Ansible playbook.