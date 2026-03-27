# SSH Remote Execution - Phase 3 Implementation

## Overview

Phase 3 (`setup_helper_node`) now supports **full remote execution via SSH** from your laptop. The orchestrator will automatically:

1. Connect to the helper node via SSH
2. Copy generated files (setup-bastion.sh, ansible-vars.yaml)
3. Execute the setup script remotely
4. Clone the Ansible playbook repository
5. Run the Ansible playbook remotely
6. Verify services are running

All of this happens **automatically** when you run the orchestrator from your laptop!

---

## Architecture

```
┌─────────────────┐
│  Your Laptop    │
│                 │
│  Orchestrator   │──────HMC API────────┐
│  Running Here   │                     │
└────────┬────────┘                     │
         │                              │
         │ SSH Connection               ▼
         │                      ┌──────────────┐
         │                      │     HMC      │
         │                      └──────┬───────┘
         │                             │
         │                             │ Manages
         │                             │
         ▼                             ▼
┌─────────────────┐          ┌──────────────────┐
│  Helper Node    │          │  Power Systems   │
│                 │          │                  │
│  • Ansible runs │          │  • Master LPAR   │
│  • Services run │          │  • Worker LPARs  │
│  • DHCP/DNS     │          │                  │
│  • TFTP/HTTP    │          │                  │
└─────────────────┘          └──────────────────┘
```

---

## Configuration Requirements

### In your config-sno.yaml:

```yaml
helper_node:
  name: "helper"
  ip: "192.168.1.100"           # Helper node IP address
  ssh_user: "root"               # SSH user (typically root)
  ssh_key_file: "~/.ssh/id_rsa" # Path to SSH private key
  system_name: "power-system-1"
  vios_name: "vios1"
  # ... other settings
```

### SSH Key Setup

Before running the orchestrator, ensure:

1. **SSH key exists** on your laptop:
   ```bash
   ls -la ~/.ssh/id_rsa
   ```

2. **Public key is on helper node**:
   ```bash
   ssh-copy-id root@192.168.1.100
   ```

3. **Test SSH connection**:
   ```bash
   ssh root@192.168.1.100 "echo 'SSH works!'"
   ```

---

## What Happens During Phase 3

### Step-by-Step Execution

```
[Laptop] Phase 3: setup_helper_node starts
   │
   ├─► [1] Generate setup-bastion.sh locally
   │      └─► Saved to: ./generated/setup-bastion.sh
   │
   ├─► [2] Generate ansible-vars.yaml with MAC addresses
   │      └─► Saved to: ./generated/ansible-vars.yaml
   │
   ├─► [3] SSH Connect to helper node
   │      └─► ssh root@192.168.1.100
   │
   ├─► [4] SCP: Copy setup-bastion.sh
   │      └─► /tmp/setup-bastion.sh on helper
   │
   ├─► [5] SCP: Copy ansible-vars.yaml
   │      └─► /tmp/ansible-vars.yaml on helper
   │
   ├─► [6] SSH Execute: setup-bastion.sh
   │      └─► Installs: Ansible, git, wget, httpd, etc.
   │      └─► Duration: ~2-5 minutes
   │      └─► Output streamed to your laptop console
   │
   ├─► [7] SSH Execute: Clone Ansible repo
   │      └─► git clone https://github.com/cs-zhang/ocp4-ai-powervm.git
   │      └─► Location: /root/ocp4-ai-powervm
   │
   ├─► [8] SSH Execute: Copy vars.yaml
   │      └─► cp /tmp/ansible-vars.yaml /root/ocp4-ai-powervm/vars.yaml
   │
   ├─► [9] SSH Execute: Run Ansible playbook
   │      └─► ansible-playbook -e @vars.yaml playbooks/main.yaml
   │      └─► Duration: ~10-15 minutes
   │      └─► Output streamed to your laptop console
   │      └─► Configures: DHCP, DNS, TFTP, HTTP services
   │
   └─► [10] SSH Execute: Verify services
          └─► systemctl is-active dnsmasq httpd
          └─► Confirms services are running

[Laptop] Phase 3: Complete! ✅
```

---

## Execution Flow

### Automatic SSH Execution (Default)

When SSH is configured, the orchestrator will:

```bash
# Run from your laptop
./ocp-upi-deployer -config config-sno.yaml

# Output you'll see:
🛠️  Setting up helper node services...
[Helper] Generating setup-bastion.sh script...
[Helper] ✅ Generated: ./generated/setup-bastion.sh
[Helper] Generating ansible-vars.yaml with LPAR MAC addresses...
[Helper] ✅ Generated: ./generated/ansible-vars.yaml
[Helper] Attempting SSH connection to root@192.168.1.100...
[SSH] Connecting to root@192.168.1.100...
[SSH] ✅ Connected successfully
[SSH] Copying setup-bastion.sh to helper node...
[SSH] ✅ setup-bastion.sh copied
[SSH] Copying ansible-vars.yaml to helper node...
[SSH] ✅ ansible-vars.yaml copied
[SSH] Executing setup-bastion.sh (this may take several minutes)...
# ... setup script output streams here ...
[SSH] ✅ setup-bastion.sh completed
[SSH] Cloning ocp4-ai-powervm repository...
[SSH] ✅ Repository cloned
[SSH] Copying ansible-vars.yaml to repository...
[SSH] ✅ vars.yaml copied to repository
[SSH] Running Ansible playbook (this will take 10-15 minutes)...
[SSH] This will install and configure: DHCP, DNS, TFTP, HTTP services
# ... Ansible playbook output streams here ...
[SSH] ✅ Ansible playbook completed successfully
[SSH] Verifying services...
[SSH] ✅ Services verified and running
```

### Fallback to Manual Instructions

If SSH fails (network issue, wrong key, etc.), the orchestrator will:

```bash
[Helper] ⚠️  SSH execution failed: <error details>
[Helper] Falling back to manual instructions...

========================================================================
 📋 MANUAL SETUP INSTRUCTIONS
========================================================================

Generated files are available in: ./generated/

Step 1: Copy files to helper node
  scp ./generated/setup-bastion.sh root@192.168.1.100:/tmp/
  scp ./generated/ansible-vars.yaml root@192.168.1.100:/tmp/

Step 2: SSH to helper node and run setup script
  ssh root@192.168.1.100
  sudo bash /tmp/setup-bastion.sh

Step 3: Run Ansible playbook (after setup completes)
  # Copy your Ansible playbook to helper node
  git clone https://github.com/cs-zhang/ocp4-ai-powervm.git
  cd ocp4-ai-powervm
  cp /tmp/ansible-vars.yaml vars.yaml
  ansible-playbook -e @vars.yaml playbooks/main.yaml
```

---

## Implementation Details

### SSH Functions

The implementation includes three key functions:

#### 1. `createSSHClient()`
- Reads SSH private key from file
- Parses the key
- Establishes SSH connection
- Returns authenticated SSH client

#### 2. `scpFile()`
- Implements SCP protocol
- Copies local files to remote host
- Used for setup-bastion.sh and ansible-vars.yaml

#### 3. `executeSSHCommand()`
- Executes commands on remote host
- Supports output streaming (for long-running commands)
- Captures stdout/stderr
- Returns errors with full context

### Error Handling

The implementation includes robust error handling:

```go
// If SSH connection fails
if err := o.createSSHClient(...); err != nil {
    log.Printf("⚠️  SSH execution failed: %v", err)
    log.Println("Falling back to manual instructions...")
    o.printManualInstructions(...)
    return nil // Don't fail the phase, just warn
}
```

This ensures the deployment can continue even if SSH automation fails.

---

## Benefits of Remote Execution

### ✅ Advantages

1. **Fully Automated**: No manual steps required
2. **Real-time Feedback**: See output as it happens
3. **Centralized Control**: Manage from your laptop
4. **Error Detection**: Immediate notification of failures
5. **Reproducible**: Same process every time
6. **Multi-cluster**: Manage multiple deployments easily

### ⚠️ Requirements

1. **Network Access**: Laptop must reach helper node IP
2. **SSH Key**: Private key must be accessible
3. **Helper Node**: Must be running with SSH enabled
4. **Firewall**: Port 22 must be open

---

## Troubleshooting

### SSH Connection Fails

**Problem**: `failed to dial SSH: connection refused`

**Solutions**:
```bash
# 1. Verify helper node is reachable
ping 192.168.1.100

# 2. Check SSH service is running on helper
ssh root@192.168.1.100 "systemctl status sshd"

# 3. Verify firewall allows SSH
ssh root@192.168.1.100 "firewall-cmd --list-all"
```

### SSH Key Authentication Fails

**Problem**: `failed to parse SSH key: ssh: no key found`

**Solutions**:
```bash
# 1. Verify key file exists
ls -la ~/.ssh/id_rsa

# 2. Check key permissions
chmod 600 ~/.ssh/id_rsa

# 3. Test key manually
ssh -i ~/.ssh/id_rsa root@192.168.1.100
```

### Ansible Playbook Fails

**Problem**: Ansible playbook execution fails

**Solutions**:
```bash
# 1. SSH to helper node manually
ssh root@192.168.1.100

# 2. Check Ansible installation
ansible --version

# 3. Run playbook manually with verbose output
cd /root/ocp4-ai-powervm
ansible-playbook -e @vars.yaml playbooks/main.yaml -vvv
```

---

## Comparison: Remote vs Manual

| Aspect | Remote SSH Execution | Manual Execution |
|--------|---------------------|------------------|
| **Automation** | Fully automated | Manual steps required |
| **Time** | ~15-20 minutes total | ~30-40 minutes (with manual steps) |
| **Errors** | Immediate detection | Delayed discovery |
| **Logging** | Centralized on laptop | Scattered across systems |
| **Reproducibility** | 100% consistent | Varies by operator |
| **Network Dependency** | Required | Not required |
| **Complexity** | Higher (SSH setup) | Lower (just run commands) |

---

## Next Steps

After Phase 3 completes successfully:

1. **Verify Services**: Helper node should have:
   - ✅ dnsmasq (DHCP + DNS)
   - ✅ httpd (HTTP server)
   - ✅ tftp (TFTP server)

2. **Check Configuration**:
   ```bash
   ssh root@192.168.1.100
   systemctl status dnsmasq httpd
   ls -la /var/www/html/
   ```

3. **Proceed to Phase 4**: Generate ignition configs
   ```bash
   # Orchestrator will continue automatically
   # Or resume with: ./ocp-upi-deployer -config config-sno.yaml -resume generate_ignition_sno
   ```

---

## Summary

**Yes, you can absolutely trigger the Ansible playbook remotely from your laptop!**

The implementation:
- ✅ Connects via SSH from your laptop
- ✅ Copies all necessary files
- ✅ Executes setup script remotely
- ✅ Runs Ansible playbook remotely
- ✅ Streams output back to your laptop
- ✅ Verifies services are running
- ✅ Falls back to manual instructions if SSH fails

**You don't need to be on the helper node** - everything is orchestrated from your laptop via SSH!