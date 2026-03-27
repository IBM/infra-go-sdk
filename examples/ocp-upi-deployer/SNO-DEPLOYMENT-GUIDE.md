# Single Node OpenShift (SNO) Deployment Guide

Complete step-by-step guide to deploy Single Node OpenShift on IBM Power Systems.

## What is SNO?

Single Node OpenShift (SNO) is a minimal OpenShift deployment with:
- **1 Helper Node** (bastion for services)
- **1 Master Node** (acts as both control plane and worker)
- **NO Bootstrap Node** (SNO uses different installation method)
- **NO Worker Nodes** (master handles workloads)

**Ideal for:**
- Edge deployments
- Development/testing
- Resource-constrained environments
- Single-system deployments

## Prerequisites

### 1. Hardware Requirements

**Helper Node (Pre-existing):**
- RHEL 8/9 or CentOS Stream installed
- 8GB RAM minimum
- 100GB disk space
- Network connectivity to HMC and cluster network

**Power System:**
- IBM Power System (POWER9 or POWER10)
- HMC access
- Available resources:
  - 64GB RAM for SNO master
  - 4 CPU units (16 virtual processors)
  - 500GB storage

### 2. Network Requirements

- Static IP addresses for helper and master
- Network connectivity between all nodes
- Internet access (for downloading RHCOS images)
- DNS resolution

### 3. Software Requirements

- OpenShift pull secret from Red Hat
- SSH key pair for cluster access
- HMC credentials

## Step-by-Step Deployment

### Step 1: Prepare Helper Node

```bash
# SSH to your helper node
ssh root@<helper-ip>

# Update system
yum update -y

# Install git
yum install -y git

# Generate SSH key if not exists
ssh-keygen -t rsa -b 4096 -N '' -f ~/.ssh/id_rsa
```

### Step 2: Get OpenShift Pull Secret

1. Go to https://console.redhat.com/openshift/install/pull-secret
2. Download your pull secret
3. Save as `pull-secret.json`

### Step 3: Configure SNO Deployment

Edit `config-sno.yaml`:

```yaml
# HMC Configuration
hmc:
  ip: "192.0.2.1"              # Your HMC IP
  username: "REDACTED_HMC_USER<=="              # Your HMC username
  password: "your-password"        # Your HMC password

# Power System
power_systems:
  - name: "YOUR-SYSTEM-NAME"       # Your Power system name
    vswitch_name: "ETHERNET0"      # Your virtual switch
    vlan_id: 1337                  # Your VLAN ID
    max_lpars: 5
    available_memory_gb: 256
    available_processors: 16

# Storage (choose one)
storage:
  type: "vios"                     # or "svc" or "physical"
  vios:
    - system_name: "YOUR-SYSTEM-NAME"
      vios_name: "vios1"           # Your VIOS name
      volume_group: "rootvg"

# Network
network:
  domain: "example.com"            # Your domain
  cluster_name: "sno"              # Cluster name
  base_domain: "example.com"
  
  network_cidr: "192.168.7.0/24"   # Your network
  gateway: "REDACTED_LAB_GW<=="           # Your gateway
  netmask: "255.255.255.0"
  broadcast: "192.168.7.255"
  nameserver: "REDACTED_LAB_IP<=="       # Helper node IP
  
  dns_forwarders:
    - "8.8.8.8"
    - "8.8.4.4"
  
  mac_prefix: "52:54:00"

# OpenShift
openshift:
  version: "4.14"                  # OpenShift version
  pull_secret_file: "./pull-secret.json"
  ssh_public_key_file: "~/.ssh/id_rsa.pub"

# Helper Node (Pre-existing)
helper_node:
  hostname: "helper.sno.example.com"
  ip: "REDACTED_LAB_IP<=="               # Your helper IP
  ssh_user: "root"
  ssh_key_file: "~/.ssh/id_rsa"
  
  services:
    dnsmasq:
      enabled: true
      dhcp_range_start: "192.168.7.50"
      dhcp_range_end: "REDACTED_LAB_GW<==00"
      dhcp_lease_time: "12h"
      tftp_root: "/var/lib/tftpboot"
    
    http:
      enabled: true
      port: 8080
      document_root: "/var/www/html"
    
    haproxy:
      enabled: false               # Not needed for SNO
    
    nfs:
      enabled: false               # Optional
  
  network_interface: "env2"        # Your interface name

# SNO Node Configuration (Single Node)
sno_node:
  name: "sno-master"
  hostname: "sno-master.sno.example.com"
  ip: "REDACTED_LAB_GW<==0"               # Master IP
  system_name: "YOUR-SYSTEM-NAME"
  
  lpar:
    os_type: "AIX/Linux"
    processor:
      type: "shared"
      units: 4.0                   # 4 CPU units for control plane + workloads
      virtual_procs: 16
      min_units: 2.0
      max_units: 8.0
      min_procs: 8
      max_procs: 32
    memory:
      desired_mb: 65536            # 64GB RAM for control plane + workloads
      min_mb: 32768                # Minimum 32GB
      max_mb: 131072               # Maximum 128GB
    storage:
      boot_disk_gb: 120            # OS and OpenShift binaries
      etcd_disk_gb: 100            # etcd database
      container_storage_gb: 300    # Container images and storage
```

### Step 4: Run Go Tool

```bash
# Build the tool (if not already built)
cd powerhmc-go/examples/ocp-upi-deployer
go build -o ocp-upi-deployer

# Run the deployment
./ocp-upi-deployer --config config-sno.yaml

# Expected output:
# ✓ Validating configuration
# ✓ Connecting to HMC
# ✓ Setting up helper node
#   - Configuring dnsmasq
#   - Configuring httpd
# ✓ Creating LPAR for SNO master
# ✓ Attaching storage
# ✓ Configuring network
# ✓ Generating vars.yaml
# ✓ Copying files to helper node
# 
# Next steps:
# 1. SSH to helper node: ssh root@REDACTED_LAB_IP<==
# 2. Clone playbook: git clone https://github.com/cs-zhang/ocp4-ai-powervm.git
# 3. Run playbook: cd ocp4-ai-powervm && ansible-playbook -i inventory playbook.yml -e @/root/vars-sno.yaml
```

### Step 5: Run Ansible Playbook

```bash
# SSH to helper node
ssh root@REDACTED_LAB_IP<==

# Clone the ocp4-ai-powervm playbook
git clone https://github.com/cs-zhang/ocp4-ai-powervm.git
cd ocp4-ai-powervm

# Copy your pull secret
cp /path/to/pull-secret.json .

# Run the playbook
ansible-playbook -i inventory playbook.yml -e @/root/vars-sno.yaml

# This will:
# - Download RHCOS images
# - Generate ignition configs
# - Setup PXE boot
# - Power on the SNO master
# - Monitor installation (60-90 minutes)
```

### Step 6: Monitor Installation

```bash
# Watch the installation progress
tail -f /var/log/messages

# Or use the OpenShift installer
openshift-install wait-for install-complete --dir=/root/sno

# Installation complete when you see:
# INFO Install complete!
# INFO To access the cluster as the system:admin user when using 'oc', run 'export KUBECONFIG=/root/sno/auth/kubeconfig'
```

### Step 7: Access Your Cluster

```bash
# Set kubeconfig
export KUBECONFIG=/root/sno/auth/kubeconfig

# Verify node
oc get nodes
# NAME         STATUS   ROLES                         AGE   VERSION
# sno-master   Ready    control-plane,master,worker   10m   v1.27.0

# Verify cluster operators
oc get co
# All operators should be Available=True

# Get console URL
oc whoami --show-console
# https://console-openshift-console.apps.sno.example.com

# Get admin password
cat /root/sno/auth/kubeadmin-password
```

## Troubleshooting

### Issue 1: LPAR Creation Fails

**Symptoms:**
```
ERROR: Failed to create LPAR
```

**Solution:**
```bash
# Check HMC connectivity
ping <hmc-ip>

# Verify HMC credentials
ssh REDACTED_HMC_USER<==@<hmc-ip>

# Check available resources
lssyscfg -r sys -F name,state
```

### Issue 2: Network Issues

**Symptoms:**
```
ERROR: Cannot reach helper node
```

**Solution:**
```bash
# Verify helper node is accessible
ping REDACTED_LAB_IP<==

# Check dnsmasq is running
systemctl status dnsmasq

# Check DHCP leases
cat /var/lib/dnsmasq/dnsmasq.leases

# Test DNS resolution
dig api.sno.example.com @REDACTED_LAB_IP<==
```

### Issue 3: Installation Hangs

**Symptoms:**
- Installation stuck at "Waiting for bootstrap"
- No progress for >30 minutes

**Solution:**
```bash
# Check bootstrap logs
ssh core@REDACTED_LAB_GW<==0
journalctl -b -f -u bootkube.service

# Check if master can reach helper
curl http://REDACTED_LAB_IP<==:8080/sno/ignition/master.ign

# Verify ignition files exist
ls -la /var/www/html/sno/ignition/
```

### Issue 4: Storage Issues

**Symptoms:**
```
ERROR: Failed to attach storage
```

**Solution:**
```bash
# For VIOS storage
ssh padmin@vios
lspv
lsvg rootvg

# For SVC storage
svcinfo lsvdisk
svcinfo lshost

# Check volume availability
```

## Post-Installation

### Configure Image Registry

```bash
# SNO needs special registry configuration
oc patch configs.imageregistry.operator.openshift.io cluster --type merge --patch '{"spec":{"managementState":"Managed","storage":{"emptyDir":{}}}}'
```

### Add Users

```bash
# Create htpasswd file
htpasswd -c -B -b users.htpasswd admin password123

# Create secret
oc create secret generic htpass-secret --from-file=htpasswd=users.htpasswd -n openshift-config

# Configure OAuth
oc apply -f - <<EOF
apiVersion: config.openshift.io/v1
kind: OAuth
metadata:
  name: cluster
spec:
  identityProviders:
  - name: htpasswd_provider
    mappingMethod: claim
    type: HTPasswd
    htpasswd:
      fileData:
        name: htpass-secret
EOF
```

### Configure Persistent Storage

```bash
# If using NFS
oc create -f - <<EOF
apiVersion: v1
kind: PersistentVolume
metadata:
  name: pv-nfs
spec:
  capacity:
    storage: 100Gi
  accessModes:
    - ReadWriteMany
  nfs:
    path: /exports/sno
    server: REDACTED_LAB_IP<==
  persistentVolumeReclaimPolicy: Retain
EOF
```

## Cleanup

To remove the SNO cluster:

```bash
# On helper node
CLUSTER_NAME="sno"

# Remove dnsmasq configs
rm -f /etc/dnsmasq.d/${CLUSTER_NAME}-*.conf
systemctl reload dnsmasq

# Remove httpd configs
rm -f /etc/httpd/conf.d/${CLUSTER_NAME}-vhost.conf
rm -rf /var/www/html/${CLUSTER_NAME}
systemctl reload httpd

# Remove TFTP files
rm -rf /var/lib/tftpboot/${CLUSTER_NAME}

# On HMC (via Go tool or manually)
# Delete the LPAR
```

## Resources

- [OpenShift SNO Documentation](https://docs.openshift.com/container-platform/latest/installing/installing_sno/install-sno-preparing-to-install-sno.html)
- [ocp4-ai-powervm Playbook](https://github.com/cs-zhang/ocp4-ai-powervm)
- [IBM Power Systems Documentation](https://www.ibm.com/docs/en/power10)

## Support

For issues:
1. Check logs: `/var/log/messages` on helper node
2. Check HMC logs
3. Review OpenShift installer logs
4. Consult Red Hat support or community forums