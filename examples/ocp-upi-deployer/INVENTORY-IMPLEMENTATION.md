# Ansible Inventory File Implementation

## Overview

This document explains the implementation of Ansible inventory file generation for the OpenShift UPI deployer, which resolves the inventory parsing errors encountered during Ansible playbook execution.

## Problem Statement

When running the Ansible playbook from the `ocp4-ai-powervm` repository, the following error was encountered:

```
[WARNING]: Unable to parse /root/ocp4-ai-powervm/inventory as an inventory source
[WARNING]: No inventory was parsed, only implicit localhost is available
[WARNING]: provided hosts list is empty, only localhost is available. Note that the implicit localhost does not match 'all'
```

This occurred because the Ansible playbook expected an inventory file but none was provided.

## Solution

### 1. Inventory Template

Created a simple inventory template at `templates/inventory.tmpl`:

```ini
[bastion]
{{.HelperNode.IP}} ansible_connection=local
```

**Key Points:**
- Uses `[bastion]` group as required by the ocp4-ai-powervm playbook
- Specifies `ansible_connection=local` since the playbook runs on the bastion/helper node itself
- Template variable `{{.HelperNode.IP}}` is populated from the configuration

### 2. Inventory Generation Function

Added `generateInventory()` method in [`main.go`](main.go:993-1023):

```go
func (o *Orchestrator) generateInventory() string {
    tmplPath := filepath.Join("templates", "inventory.tmpl")
    tmplContent, err := os.ReadFile(tmplPath)
    if err != nil {
        // Fallback to simple format if template not found
        return fmt.Sprintf("[bastion]\n%s ansible_connection=local\n", o.config.HelperNode.IP)
    }
    
    tmpl, err := template.New("inventory").Parse(string(tmplContent))
    if err != nil {
        return fmt.Sprintf("[bastion]\n%s ansible_connection=local\n", o.config.HelperNode.IP)
    }
    
    var buf bytes.Buffer
    if err := tmpl.Execute(&buf, o.config); err != nil {
        return fmt.Sprintf("[bastion]\n%s ansible_connection=local\n", o.config.HelperNode.IP)
    }
    
    return buf.String()
}
```

**Features:**
- Reads and parses the inventory template
- Provides fallback to simple format if template is missing or parsing fails
- Returns properly formatted inventory content

### 3. Integration into Helper Setup Phase

Modified [`phaseSetupHelperNode()`](main.go:509-516) to generate the inventory file:

```go
// Step 3: Generate Ansible inventory file
log.Println("[Helper] Generating Ansible inventory...")
inventory := o.generateInventory()
inventoryPath := filepath.Join(outputDir, "inventory")
if err := os.WriteFile(inventoryPath, []byte(inventory), 0644); err != nil {
    return fmt.Errorf("failed to write inventory: %v", err)
}
log.Printf("[Helper] ✅ Generated: %s", inventoryPath)
```

### 4. SSH File Transfer

Updated [`executeHelperSetupViaSSH()`](main.go:1075) to copy the inventory file:

**Function Signature:**
```go
func (o *Orchestrator) executeHelperSetupViaSSH(setupScriptPath, ansibleVarsPath, inventoryPath string) error
```

**File Transfer Steps:**
1. Copy `setup-bastion.sh` to `/tmp/setup-bastion.sh`
2. Copy `ansible-vars.yaml` to `/tmp/ansible-vars.yaml`
3. **Copy `inventory` to `/tmp/inventory`** (NEW)
4. Execute setup script
5. Clone ocp4-ai-powervm repository
6. Copy both files to repository: `cp /tmp/ansible-vars.yaml /root/ocp4-ai-powervm/vars.yaml && cp /tmp/inventory /root/ocp4-ai-powervm/inventory`

### 5. Ansible Playbook Execution

Updated the Ansible command to use the inventory file:

**Before:**
```bash
ansible-playbook -e @vars.yaml playbooks/main.yaml
```

**After:**
```bash
ansible-playbook -i inventory -e @vars.yaml playbooks/main.yaml
```

The `-i inventory` flag explicitly specifies the inventory file location.

## File Locations

### Generated Files (in output directory)

```
output/
├── setup-bastion.sh          # Helper node setup script
├── ansible-vars.yaml          # Ansible variables
└── inventory                  # Ansible inventory (NEW)
```

### Files on Helper Node

```
/tmp/
├── setup-bastion.sh
├── ansible-vars.yaml
└── inventory                  # Copied via SSH

/root/ocp4-ai-powervm/
├── vars.yaml                  # Copy of ansible-vars.yaml
└── inventory                  # Copy of inventory file
```

## Inventory File Format

The generated inventory file follows this simple format:

```ini
[bastion]
192.168.100.10 ansible_connection=local
```

**Explanation:**
- `[bastion]`: Group name expected by the ocp4-ai-powervm playbook
- `192.168.100.10`: IP address of the helper/bastion node (from configuration)
- `ansible_connection=local`: Tells Ansible to execute commands locally rather than over SSH

## Reference

This implementation is based on the example inventory from the ocp4-ai-powervm repository:
- Repository: https://github.com/cs-zhang/ocp4-ai-powervm
- Example: https://github.com/cs-zhang/ocp4-ai-powervm/blob/main/example-inventory

## Testing

To verify the inventory file generation:

1. **Build the tool:**
   ```bash
   cd powerhmc-go/examples/ocp-upi-deployer
   go build -o ocp-upi-deployer .
   ```

2. **Run the deployment:**
   ```bash
   ./ocp-upi-deployer -config config-sno.yaml
   ```

3. **Check generated files:**
   ```bash
   ls -la output/
   cat output/inventory
   ```

4. **Expected output:**
   ```
   [bastion]
   192.168.100.10 ansible_connection=local
   ```

## Benefits

1. **Eliminates Inventory Warnings**: Ansible no longer complains about missing inventory
2. **Proper Host Targeting**: Playbook tasks execute on the correct host (bastion)
3. **Local Execution**: Uses `ansible_connection=local` for efficient local execution
4. **Template-Based**: Easy to customize if additional inventory configuration is needed
5. **Fallback Support**: Provides simple format if template is unavailable

## Future Enhancements

Potential improvements for more complex deployments:

1. **Multiple Hosts**: Support for additional hosts in inventory
2. **Host Variables**: Add host-specific variables if needed
3. **Group Variables**: Support for group-level configuration
4. **Dynamic Inventory**: Generate inventory based on discovered infrastructure

## Conclusion

The inventory file implementation resolves the Ansible playbook execution errors by providing a properly formatted inventory file that specifies the bastion host with local connection. This is a critical component for successful helper node setup and subsequent OpenShift deployment.