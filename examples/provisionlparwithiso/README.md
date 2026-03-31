# LPAR Provisioning with ISO - Complete Lifecycle Management

This example demonstrates complete LPAR lifecycle management including creation, provisioning, and deletion with automatic resource cleanup.

## Features

### Create Mode (`--create`)
- **Complete LPAR Creation**: Creates a new LPAR with configurable CPU, memory, and network resources
- **Auto-Discovery**: Automatically discovers and selects active VIOS servers (or use `--vios-name` to specify)
- **Network Configuration**: Creates network adapter with vSwitch and VLAN support
- **NFS Integration**: Mounts NFS share on VIOS for ISO access
- **Virtual Optical Media**: Creates and maps ISO files as virtual optical media
- **Virtual Disk Management**: Creates and maps virtual disks for storage
- **Profile Management**: Saves partition profile for future use
- **Automatic Power-On**: Powers on LPAR with saved profile
- **Rollback Protection**: Automatically cleans up all resources if any step fails

### Delete Mode (`--delete`)
- **Complete Resource Cleanup**: Deletes LPAR and all associated resources
- **Graceful Shutdown**: Powers off LPAR before deletion
- **SCSI Mapping Removal**: Automatically unmaps all virtual disks and optical media
- **Virtual Disk Deletion**: Removes all virtual disks created for the LPAR
- **Network Cleanup**: Removes network adapters (deleted with LPAR)
- **Auto-Discovery Support**: Can auto-discover VIOS if not specified

## Usage

### Prerequisites
- HMC access with appropriate credentials
- NFS server with ISO files
- Managed system with available resources
- Active VIOS server(s)

### Create a New LPAR

```bash
# Basic creation with auto-discovery of active VIOS
./provisionlparwithiso \
  --create \
  --hmc-ip 192.0.2.1 \
  --hmc-pass REDACTED_HMC_PASS<== \
  --system-name LTC09U31-ZZ \
  --lpar-name my-test-lpar \
  --nfs-server 192.0.2.20 \
  --export-path /var/www/html/isos \
  --iso-files fedora.iso,rhcos.iso

# Creation with specific VIOS and custom resources
./provisionlparwithiso \
  --create \
  --hmc-ip 192.0.2.1 \
  --hmc-pass REDACTED_HMC_PASS<== \
  --system-name LTC09U31-ZZ \
  --lpar-name my-test-lpar \
  --vios-name ltc09u31-vios1 \
  --desired-proc-units 2.0 \
  --desired-vcpus 4 \
  --desired-mem 8192 \
  --vswitch-name ETHERNET0 \
  --vlan-id 1337 \
  --disk-names disk1,disk2 \
  --disk-sizes-mb 20480,40960 \
  --iso-files fedora.iso
```

### Delete an Existing LPAR

```bash
# Basic deletion with auto-discovery of active VIOS
./provisionlparwithiso \
  --delete \
  --hmc-ip 192.0.2.1 \
  --hmc-pass REDACTED_HMC_PASS<== \
  --system-name LTC09U31-ZZ \
  --lpar-name my-test-lpar

# Deletion with specific VIOS
./provisionlparwithiso \
  --delete \
  --hmc-ip 192.0.2.1 \
  --hmc-pass REDACTED_HMC_PASS<== \
  --system-name LTC09U31-ZZ \
  --lpar-name my-test-lpar \
  --vios-name ltc09u31-vios1
```

## Command-Line Flags

### Operation Mode (Mutually Exclusive - Required)
- `--create`: Create and provision a new LPAR
- `--delete`: Delete an existing LPAR and its resources

### Required Flags (Both Modes)
- `--hmc-ip`: HMC IP address (default: "192.0.2.1")
- `--hmc-pass`: HMC password (required)
- `--lpar-name`: Name of the LPAR (required)
- `--system-name`: Managed System Name (default: "LTC09U31-ZZ")

### VIOS Configuration (Optional - Both Modes)
- `--vios-name`: Target VIOS name (optional - will auto-select active VIOS if not specified)

### Create Mode - LPAR Configuration
- `--os-type`: OS type (default: "AIX/Linux")
- `--lpar-profile`: LPAR profile name (default: "default_profile")

### Create Mode - CPU Configuration
- `--min-proc-units`: Minimum processing units (default: 0.5)
- `--desired-proc-units`: Desired processing units (default: 1.0)
- `--max-proc-units`: Maximum processing units (default: 2.0)
- `--min-vcpus`: Minimum virtual CPUs (default: 1)
- `--desired-vcpus`: Desired virtual CPUs (default: 2)
- `--max-vcpus`: Maximum virtual CPUs (default: 4)
- `--sharing-mode`: Processor sharing mode (default: "uncapped")

### Create Mode - Memory Configuration (MB)
- `--min-mem`: Minimum memory in MB (default: 2048)
- `--desired-mem`: Desired memory in MB (default: 4096)
- `--max-mem`: Maximum memory in MB (default: 8192)

### Create Mode - Network Configuration
- `--vswitch-name`: Virtual switch name (default: "ETHERNET0(Default)")
- `--vlan-id`: VLAN ID for network adapter (default: 1337)

### Create Mode - NFS Configuration
- `--nfs-server`: NFS server IP or hostname (default: "192.0.2.20")
- `--export-path`: NFS export path on server (default: "/var/www/html/f43")
- `--mount-point`: Local mount point on VIOS (default: "/mnt")

### Create Mode - ISO Configuration
- `--iso-files`: Comma-separated list of ISO filenames on NFS (default: "f43.iso")
- `--media-prefix`: Prefix for media names in repository (default: "media_<timestamp>")

### Create Mode - Virtual Disk Configuration
- `--vg-name`: Volume Group name for virtual disks (default: "rootvg")
- `--disk-names`: Comma-separated list of virtual disk names (default: "provision_disk")
- `--disk-sizes-mb`: Comma-separated list of disk sizes in MB (default: "10240")

### General
- `--verbose`: Enable verbose output (default: true)
- `--hmc-user`: HMC username (default: "REDACTED_HMC_USER<==")

## Workflow Details

### Create Mode Workflow (10 Steps)

1. **Authentication & Resolution**
   - Login to HMC
   - Resolve managed system UUID
   - Auto-discover or validate VIOS

2. **Create LPAR**
   - Create LPAR with specified resources
   - Validate LPAR doesn't already exist

3. **Network Configuration**
   - Resolve vSwitch UUID
   - Create network adapter with VLAN

4. **NFS Mount**
   - Mount NFS share on VIOS
   - Handle already-mounted scenarios

5. **Create Virtual Optical Media**
   - Create optical media from ISO files
   - Track created media for rollback

6. **Map Optical Media**
   - Map optical media to LPAR
   - Enable ISO boot capability

7. **Create Virtual Disks**
   - Create virtual disks in volume group
   - Validate disk sizes and names

8. **Map Virtual Disks**
   - Map virtual disks to LPAR
   - Provide storage for OS installation

9. **Save Partition Profile**
   - Save current configuration
   - Enable future profile-based operations

10. **Power On LPAR**
    - Extract profile UUID
    - Power on with saved profile
    - Ready for OS installation

### Delete Mode Workflow (7 Steps)

1. **Authentication & Resolution**
   - Login to HMC
   - Resolve managed system UUID
   - Auto-discover or validate VIOS

2. **Find LPAR**
   - Locate LPAR by name
   - Get current state

3. **Resolve VIOS**
   - Use specified VIOS or auto-discover
   - Validate VIOS availability

4. **Power Off LPAR**
   - Gracefully shutdown if running
   - Wait for shutdown completion

5. **Remove SCSI Mappings**
   - Get all SCSI mappings for LPAR
   - Unmap virtual disks
   - Delete virtual disks
   - Unmap optical media

6. **Remove Network Adapters**
   - Network adapters deleted with LPAR

7. **Delete LPAR**
   - Remove LPAR from system
   - Complete cleanup

## Rollback Protection (Create Mode)

If any step fails during creation, the program automatically:
1. Powers off LPAR (if powered on)
2. Unmaps virtual disks (if mapped)
3. Deletes virtual disks (if created)
4. Unmaps optical media (if mapped)
5. Deletes network adapter (with LPAR)
6. Deletes LPAR (if created)

**Note**: Optical media files remain in VIOS repository for potential reuse.

## Examples

### Example 1: Quick Test LPAR
```bash
./provisionlparwithiso \
  --create \
  --lpar-name quick-test \
  --iso-files test.iso \
  --disk-names testdisk \
  --disk-sizes-mb 10240
```

### Example 2: Production LPAR with Multiple Disks
```bash
./provisionlparwithiso \
  --create \
  --lpar-name prod-server \
  --desired-proc-units 4.0 \
  --desired-vcpus 8 \
  --desired-mem 16384 \
  --disk-names rootdisk,datadisk,logdisk \
  --disk-sizes-mb 51200,102400,51200 \
  --iso-files rhel9.iso \
  --vlan-id 100
```

### Example 3: Delete LPAR
```bash
./provisionlparwithiso \
  --delete \
  --lpar-name quick-test
```

## Error Handling

- **Mutually Exclusive Flags**: Program exits if both `--create` and `--delete` are specified
- **Missing Required Flags**: Program exits if required flags are not provided
- **Resource Conflicts**: Checks for existing LPAR before creation
- **VIOS Discovery**: Automatically finds active VIOS if not specified
- **Graceful Failures**: Provides clear error messages and cleanup on failure

## Notes

1. **VIOS Auto-Discovery**: When `--vios-name` is not specified, the program automatically discovers and selects the first active VIOS server
2. **Optical Media Persistence**: Optical media files remain in VIOS repository after deletion for potential reuse
3. **Network Adapters**: Network adapters are automatically deleted when the LPAR is deleted
4. **Disk Count Validation**: Number of disk names must match number of disk sizes
5. **State Checking**: Delete mode checks LPAR state and powers off if necessary before deletion

## Troubleshooting

### LPAR Already Exists
```
❌ LPAR 'my-test-lpar' already exists (UUID: xxx)
```
**Solution**: Use `--delete` to remove the existing LPAR first, or choose a different name.

### No Active VIOS Found
```
❌ Failed to find active VIOS
```
**Solution**: Specify a VIOS name explicitly with `--vios-name` or check VIOS server status.

### NFS Mount Failed
```
❌ Failed to mount NFS
```
**Solution**: Verify NFS server is accessible and export path is correct.

### Disk Size Mismatch
```
❌ Error: Number of disk names (2) must match number of disk sizes (1)
```
**Solution**: Ensure `--disk-names` and `--disk-sizes-mb` have the same number of comma-separated values.

## See Also

- [PowerHMC Go SDK](../../README.md)
- [LPAR Creation Example](../createlpar/)
- [Delete Partition Example](../deletepartition/)
- [VIOS Management](../../vios.go)