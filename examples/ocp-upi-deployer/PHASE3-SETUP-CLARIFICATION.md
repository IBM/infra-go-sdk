# Phase 3: Helper Node Setup - Execution Scenarios

## Overview

Phase 3 (`setup_helper_node`) prepares the helper/bastion node with required services (DHCP, DNS, TFTP, HTTP) for OpenShift deployment. Based on the [ocp4-ai-powervm](https://github.com/cs-zhang/ocp4-ai-powervm) reference implementation, there are **two distinct execution scenarios** depending on where you run the orchestrator program.

---

## Key Understanding from Reference Implementation

The reference implementation uses **Ansible playbooks** that must run **ON the helper node itself** because:
1. Ansible installs packages locally using `yum/dnf`
2. Configures local services (dnsmasq, httpd, tftp)
3. Modifies local system files (`/etc/selinux/config`, firewall rules)
4. Downloads OpenShift installer and creates ignition configs locally

**Critical Note**: The reference workflow assumes:
- Helper node already exists with RHEL/CentOS installed
- You manually create LPARs first, then note their MAC addresses
- You manually update `vars.yaml` with MAC addresses
- You run Ansible playbook on the helper node

---

## Scenario 1: Running Orchestrator from Your Laptop

### Architecture
```
[Your Laptop] --HMC API--> [HMC] --manages--> [Power Systems]
     |                                              |
     |                                              +-- [Helper Node LPAR]
     |                                              +-- [Master Node LPAR]
     |
     +--SSH--> [Helper Node] (to execute setup)
```

### Current Implementation (Phase 3)

**What it does:**
1. ✅ Generates `setup-bastion.sh` script locally (in `./generated/`)
2. ✅ Generates `ansible-vars.yaml` with MAC addresses from created LPARs
3. ⚠️  **Attempts SSH** to helper node (currently placeholder)
4. ⚠️  **Falls back** to manual instructions if SSH unavailable

**What you need to do:**

#### Option A: Manual Execution (Current Working Approach)
```bash
# 1. Run orchestrator on your laptop
./ocp-upi-deployer -config config-sno.yaml

# 2. Orchestrator creates LPARs and generates files in ./generated/:
#    - setup-bastion.sh
#    - ansible-vars.yaml

# 3. Copy files to helper node
scp ./generated/setup-bastion.sh root@<helper-ip>:/root/
scp ./generated/ansible-vars.yaml root@<helper-ip>:/root/

# 4. SSH to helper node
ssh root@<helper-ip>

# 5. On helper node, run setup script
chmod +x /root/setup-bastion.sh
/root/setup-bastion.sh

# 6. Clone the Ansible playbook
git clone https://github.com/cs-zhang/ocp4-ai-powervm.git
cd ocp4-ai-powervm

# 7. Copy the generated vars file
cp /root/ansible-vars.yaml vars.yaml

# 8. Run Ansible playbook
ansible-playbook -e @vars.yaml playbooks/main.yaml
```

#### Option B: Automated SSH Execution (Needs Implementation)
To fully automate, we need to enhance `executeHelperSetupViaSSH()`:

```go
func (o *Orchestrator) executeHelperSetupViaSSH(setupScriptPath, ansibleVarsPath string) error {
    // 1. SCP files to helper node
    // 2. SSH and execute setup-bastion.sh
    // 3. SSH and clone ocp4-ai-powervm repo
    // 4. SSH and copy ansible-vars.yaml to repo
    // 5. SSH and run ansible-playbook
    // 6. Stream output back to orchestrator
}
```

**Pros:**
- ✅ Orchestrator has full visibility of all phases
- ✅ Can manage multiple deployments from one laptop
- ✅ Centralized logging and state management
- ✅ MAC addresses automatically captured and injected

**Cons:**
- ❌ Requires SSH access to helper node
- ❌ Network connectivity from laptop to helper node required
- ❌ More complex error handling for remote execution

---

## Scenario 2: Running Orchestrator ON the Helper Node

### Architecture
```
[Helper Node LPAR] --HMC API--> [HMC] --manages--> [Power Systems]
     |                                                   |
     | (localhost)                                       +-- [Master Node LPAR]
     |
     +-- Orchestrator runs here
     +-- Ansible runs here
     +-- Services (DHCP/DNS/TFTP/HTTP) run here
```

### Modified Implementation Needed

**What changes:**
1. ✅ Generates `setup-bastion.sh` locally (same)
2. ✅ Generates `ansible-vars.yaml` locally (same)
3. ✅ **Execute setup-bastion.sh directly** (no SSH needed)
4. ✅ **Clone and run Ansible playbook directly** (no SSH needed)

**Implementation approach:**

```go
func (o *Orchestrator) phaseSetupHelperNode() error {
    log.Println("🛠️  Setting up helper node services...")
    
    // Detect if running on helper node
    isLocalHelper := o.isRunningOnHelperNode()
    
    if isLocalHelper {
        // Scenario 2: Direct local execution
        return o.setupHelperNodeLocally()
    } else {
        // Scenario 1: Remote SSH execution
        return o.setupHelperNodeRemotely()
    }
}

func (o *Orchestrator) isRunningOnHelperNode() bool {
    // Check if current host IP matches helper node IP
    // Or check for environment variable: RUNNING_ON_HELPER=true
    hostname, _ := os.Hostname()
    localIPs := getLocalIPs()
    return contains(localIPs, o.config.HelperNode.IP)
}

func (o *Orchestrator) setupHelperNodeLocally() error {
    // 1. Generate files in /root/ or /opt/ocp-deployer/
    // 2. Execute setup-bastion.sh directly
    cmd := exec.Command("bash", "/root/setup-bastion.sh")
    output, err := cmd.CombinedOutput()
    // ... handle output
    
    // 3. Clone Ansible repo
    cmd = exec.Command("git", "clone", "https://github.com/cs-zhang/ocp4-ai-powervm.git")
    // ... execute
    
    // 4. Copy ansible-vars.yaml
    // 5. Run Ansible playbook
    cmd = exec.Command("ansible-playbook", "-e", "@vars.yaml", "playbooks/main.yaml")
    // ... execute and stream output
    
    return nil
}
```

**Pros:**
- ✅ No SSH complexity
- ✅ Direct execution with immediate feedback
- ✅ Simpler error handling
- ✅ No network dependency between orchestrator and helper

**Cons:**
- ❌ Orchestrator must run on helper node (less flexible)
- ❌ Can't manage multiple deployments from one location
- ❌ Helper node needs Go runtime and HMC API access

---

## Recommended Approach

### For Your Current Implementation

**Keep Scenario 1 (Laptop-based) as primary** because:
1. More flexible - manage multiple clusters from one location
2. Better separation of concerns
3. Helper node can be minimal (just RHEL + services)
4. Matches typical enterprise workflow

**Phase 3 Enhancement Plan:**

```yaml
# In config-sno.yaml, add execution mode
helper_node:
  ip: "192.168.1.100"
  ssh_user: "root"
  ssh_key_file: "~/.ssh/id_rsa"
  execution_mode: "remote"  # or "local"
```

**Implementation steps:**
1. ✅ **Current**: Generate files locally
2. ✅ **Current**: Provide manual instructions
3. 🔄 **Enhance**: Implement full SSH automation in `executeHelperSetupViaSSH()`
4. 🔄 **Add**: Detection for local vs remote execution
5. 🔄 **Add**: Local execution path for Scenario 2

---

## What Phase 3 Should Do (Final State)

### Scenario 1 (Remote - from laptop):
```
1. Generate setup-bastion.sh
2. Generate ansible-vars.yaml with MAC addresses
3. SCP files to helper node
4. SSH: Execute setup-bastion.sh
5. SSH: Clone ocp4-ai-powervm repo
6. SSH: Copy ansible-vars.yaml to repo
7. SSH: Run ansible-playbook
8. Stream output back to orchestrator
9. Verify services are running (dnsmasq, httpd, tftp)
```

### Scenario 2 (Local - on helper node):
```
1. Generate setup-bastion.sh in /root/
2. Generate ansible-vars.yaml with MAC addresses
3. Execute setup-bastion.sh directly
4. Clone ocp4-ai-powervm repo
5. Copy ansible-vars.yaml to repo
6. Run ansible-playbook directly
7. Verify services are running
```

---

## Current Status & Next Steps

### ✅ Completed
- Generate setup-bastion.sh with package installation
- Generate ansible-vars.yaml with MAC addresses from LPARs
- Manual instruction fallback

### 🔄 Needs Implementation
- Full SSH automation for Scenario 1
- Local execution detection and path for Scenario 2
- Service verification (check if dnsmasq/httpd are running)
- Better error handling and retry logic

### 📝 Recommendation
**Start with Scenario 1 (remote execution)** since:
- You're likely running from your laptop
- It's more flexible for production use
- Current manual fallback works as interim solution

**Add Scenario 2 later** if needed for:
- Air-gapped environments
- Simplified single-node deployments
- Testing scenarios

---

## Questions to Clarify

1. **Where will you run the orchestrator?**
   - From your laptop → Focus on Scenario 1 (SSH automation)
   - On helper node → Focus on Scenario 2 (local execution)

2. **Do you have SSH access to helper node?**
   - Yes → Implement full SSH automation
   - No → Keep manual instruction approach

3. **Is helper node already provisioned?**
   - Yes → Phase 3 just configures it
   - No → Need Phase 0 to create helper LPAR first

4. **Should orchestrator create helper LPAR too?**
   - Currently assumes helper exists
   - Could add Phase 0: Create helper LPAR with RHEL ISO

Let me know which scenario matches your use case, and I'll implement the appropriate solution!