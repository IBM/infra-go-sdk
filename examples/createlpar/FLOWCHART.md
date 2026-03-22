# Create LPAR with Multi-Storage Workflow - Block Flow Chart

## Overview

This flowchart illustrates the complete workflow for creating a PowerVM LPAR with three types of storage: Physical SAN disk (via SVC FlashCopy), Virtual Disk (native VIOS logical volume), and Virtual Optical Media (ISO files).

---

## Main Flow Diagram

┌─────────────────────────────────────────────────────────────────┐
│                         START PROGRAM                            │
└────────────────────────────┬────────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────────┐
│                    PARSE COMMAND LINE FLAGS                      │
│  • HMC credentials (IP, username, password)                      │
│  • SVC credentials (IP, username, password)                      │
│  • System name, LPAR name, OS type                               │
│  • Virtual Switch, VLAN ID                                       │
│  • Base image, VIOS, Volume Group                                │
│  • Virtual disk name & size                                      │
│  • Optical media names (comma-separated)                         │
│  • Verbose flag                                                  │
└────────────────────────────┬────────────────────────────────────┘
                             │
                             ▼
═══════════════════════════════════════════════════════════════════
                PHASE 1: PARALLEL AUTHENTICATION (HMC || SVC)
═══════════════════════════════════════════════════════════════════
                             │
                             ▼
┌─────────────────────────────────────────────────────────────────┐
│                  PARALLEL AUTHENTICATION THREADS                 │
│  ┌───────────────────────────┬───────────────────────────────┐  │
│  │  [Auth-HMC Thread]        │  [Auth-SVC Thread]            │  │
│  │  ┌─────────────────────┐  │  ┌─────────────────────────┐  │  │
│  │  │ Connect to HMC      │  │  │ Connect to SVC          │  │  │
│  │  │ Login with creds    │  │  │ Login with creds        │  │  │
│  │  │ Verify session      │  │  │ Verify session          │  │  │
│  │  └──────────┬──────────┘  │  └──────────┬──────────────┘  │  │
│  └─────────────┼─────────────┴─────────────┼─────────────────┘  │
│                │                             │                    │
│                └──────────┬──────────────────┘                    │
│                           ▼                                       │
│                  ┌─────────────────┐                              │
│                  │ Both Successful?│                              │
│                  └────┬────────┬───┘                              │
│                       │ No     │ Yes                              │
│                       ▼        ▼                                  │
│                   [ERROR]   [CONTINUE]                            │
└─────────────────────────────────────────────────────────────────┘
                             │
                             ▼
═══════════════════════════════════════════════════════════════════
          PHASE 2: 3-WAY PARALLEL OPERATIONS (LPAR || VIOS || vSwitch)
═══════════════════════════════════════════════════════════════════
                             │
                             ▼
┌─────────────────────────────────────────────────────────────────┐
│                    PARALLEL OPERATION THREADS                    │
│  ┌──────────────┬──────────────────┬──────────────────────────┐ │
│  │ [Thread-LPAR]│ [Thread-VIOS]    │ [Thread-vSwitch]         │ │
│  │ ┌──────────┐ │ ┌──────────────┐ │ ┌──────────────────────┐ │ │
│  │ │ Create   │ │ │ Get VIOS     │ │ │ Resolve Virtual      │ │ │
│  │ │ LPAR     │ │ │ List         │ │ │ Switch by Name       │ │ │
│  │ │          │ │ │              │ │ │                      │ │ │
│  │ │ Set:     │ │ │ For Each:    │ │ │ Get UUID             │ │ │
│  │ │ • Name   │ │ │ • Get FC     │ │ │                      │ │ │
│  │ │ • Memory │ │ │   Ports      │ │ │                      │ │ │
│  │ │ • CPUs   │ │ │ • Extract    │ │ │                      │ │ │
│  │ │ • OS Type│ │ │   WWPNs      │ │ │                      │ │ │
│  │ └────┬─────┘ │ └──────┬───────┘ │ └──────────┬───────────┘ │ │
│  └──────┼───────┴────────┼─────────┴────────────┼─────────────┘ │
│         │                 │                      │                │
│         └─────────────────┼──────────────────────┘                │
│                           ▼                                       │
│                  ┌─────────────────┐                              │
│                  │ All 3 Complete? │                              │
│                  └────┬────────┬───┘                              │
│                       │ No     │ Yes                              │
│                       ▼        ▼                                  │
│                   [ERROR]   [CONTINUE]                            │
└─────────────────────────────────────────────────────────────────┘
                             │
                             ▼
═══════════════════════════════════════════════════════════════════
                    PHASE 3: ATTACH NETWORK ADAPTER
═══════════════════════════════════════════════════════════════════
                             │
                             ▼
┌─────────────────────────────────────────────────────────────────┐
│              STEP 3: CREATE CLIENT NETWORK ADAPTER              │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │ 3.1 Create ClientNetworkAdapter                           │  │
│  │     • Attach to LPAR                                      │  │
│  │     • Connect to Virtual Switch                           │  │
│  │     • Assign VLAN ID                                      │  │
│  │     • Set adapter slot                                    │  │
│  └───────────────────────────────────────────────────────────┘  │
└────────────────────────────┬────────────────────────────────────┘
                             │
                             ▼
═══════════════════════════════════════════════════════════════════
        PHASE 4: PARALLEL STORAGE PROVISIONING (Physical || Virtual)
═══════════════════════════════════════════════════════════════════
                             │
                             ▼
┌─────────────────────────────────────────────────────────────────┐
│                  PARALLEL STORAGE PROVISIONING                   │
│  ┌───────────────────────────┬───────────────────────────────┐  │
│  │  [Branch-A: Physical SAN] │  [Branch-B: Virtual Disk]     │  │
│  │  ┌─────────────────────┐  │  ┌─────────────────────────┐  │  │
│  │  │ 4A.1 Create SVC     │  │  │ 4B.1 Get Volume Groups  │  │  │
│  │  │      FlashCopy      │  │  │      from VIOS          │  │  │
│  │  │      • Source: Base │  │  │                         │  │  │
│  │  │        Image        │  │  │ 4B.2 Select Optimal VG  │  │  │
│  │  │      • Target: New  │  │  │      • Check free space │  │  │
│  │  │        Volume       │  │  │      • Match size req   │  │  │
│  │  ├─────────────────────┤  │  ├─────────────────────────┤  │  │
│  │  │ 4A.2 Get SVC Hosts  │  │  │ 4B.3 Create Virtual     │  │  │
│  │  │      • Match VIOS   │  │  │      Disk (LV)          │  │  │
│  │  │        WWPNs        │  │  │      • mklv command     │  │  │
│  │  ├─────────────────────┤  │  │      • Set size         │  │  │
│  │  │ 4A.3 Map Volume to  │  │  │      • Set name         │  │  │
│  │  │      VIOS Host      │  │  ├─────────────────────────┤  │  │
│  │  │      • mkvdiskhostmap│ │  │ 4B.4 Map Virtual Disk   │  │  │
│  │  ├─────────────────────┤  │  │      to LPAR            │  │  │
│  │  │ 4A.4 Run cfgdev on  │  │  │      • DeleteVirtualDisk│ │  │
│  │  │      VIOS           │  │  │        Maps             │  │  │
│  │  │      • Discover LUN │  │  │                         │  │  │
│  │  ├─────────────────────┤  │  │                         │  │  │
│  │  │ 4A.5 Identify New   │  │  │                         │  │  │
│  │  │      Physical Volume│  │  │                         │  │  │
│  │  │      • Match UID    │  │  │                         │  │  │
│  │  ├─────────────────────┤  │  │                         │  │  │
│  │  │ 4A.6 Map Physical   │  │  │                         │  │  │
│  │  │      Volume to LPAR │  │  │                         │  │  │
│  │  │      • DeletePhysical│ │  │                         │  │  │
│  │  │        VolumeMaps   │  │  │                         │  │  │
│  │  └──────────┬──────────┘  │  └──────────┬──────────────┘  │  │
│  └─────────────┼─────────────┴─────────────┼─────────────────┘  │
│                │                             │                    │
│                └──────────┬──────────────────┘                    │
│                           ▼                                       │
│                  ┌─────────────────┐                              │
│                  │ Both Successful?│                              │
│                  └────┬────────┬───┘                              │
│                       │ No     │ Yes                              │
│                       ▼        ▼                                  │
│              ┌────────────┐  [CONTINUE]                           │
│              │  CLEANUP:  │                                       │
│              │  • Delete  │                                       │
│              │    Virtual │                                       │
│              │    Disk    │                                       │
│              │  • Delete  │                                       │
│              │    SVC Vol │                                       │
│              │  • Delete  │                                       │
│              │    LPAR    │                                       │
│              └─────┬──────┘                                       │
│                    ▼                                              │
│                 [ERROR]                                           │
└─────────────────────────────────────────────────────────────────┘
                             │
                             ▼
═══════════════════════════════════════════════════════════════════
              PHASE 5: OPTICAL MEDIA MAPPING (Optional)
═══════════════════════════════════════════════════════════════════
                             │
                             ▼
┌─────────────────────────────────────────────────────────────────┐
│              STEP 5: MAP VIRTUAL OPTICAL MEDIA                   │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │ 5.1 Check if Media Names Provided                         │  │
│  │     ┌─────────────────────────────────────────────────┐   │  │
│  │     │ If media-names flag is empty:                   │   │  │
│  │     │   • Skip optical media mapping                   │   │  │
│  │     │   • Proceed to success                           │   │  │
│  │     │                                                  │   │  │
│  │     │ Else:                                            │   │  │
│  │     │   • Parse comma-separated media names           │   │  │
│  │     │   • For each media name:                        │   │  │
│  │     │     - Create VirtualOpticalMedia mapping        │   │  │
│  │     │     - Attach to LPAR                            │   │  │
│  │     │     - Set mount type (read-write)               │   │  │
│  │     └─────────────────────────────────────────────────┘   │  │
│  └───────────────────────────────────────────────────────────┘  │
└────────────────────────────┬────────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────────┐
│                         SUCCESS                                  │
│  ✅ LPAR Created and Configured                                 │
│  ✅ Network Adapter Attached                                    │
│  ✅ Physical SAN Disk Mapped                                    │
│  ✅ Virtual Disk Created and Mapped                             │
│  ✅ Optical Media Mounted (if specified)                        │
│                                                                  │
│  Summary:                                                        │
│  • LPAR Name: [specified name]                                  │
│  • LPAR UUID: [generated UUID]                                  │
│  • Physical Volumes: 1                                          │
│  • Virtual Disks: 1                                             │
│  • Optical Media: [count]                                       │
└────────────────────────────┬────────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────────┐
│                         END PROGRAM                              │
└─────────────────────────────────────────────────────────────────┘

---

## Detailed Phase Breakdown

### Phase 1: Parallel Authentication (HMC || SVC)

**Purpose** - Authenticate to both HMC and SVC simultaneously to save time.

**HMC Thread**

- Connect to HMC REST API
- Login with credentials
- Establish session

**SVC Thread**

- Connect to SVC SSH interface
- Authenticate with credentials
- Verify access

**Synchronization** - Both threads must complete successfully before proceeding.

---

### Phase 2: 3-Way Parallel Operations (LPAR || VIOS || vSwitch)

**Purpose** - Perform three independent operations concurrently.

**Thread 1 - LPAR Creation**
```
• GetManagedSystemByName()
• CreateLogicalPartition()
  - Set name, memory, CPUs
  - Set OS type (AIX/Linux/IBM i)
  - Configure boot settings
```

**Thread 2 - VIOS Discovery**
```
• GetVirtualIOServersQuick()
• For each VIOS:
  - GetVirtualIOServer() with detailed info
  - Extract Fibre Channel ports
  - Collect WWPNs for SAN connectivity
```

**Thread 3 - Virtual Switch Resolution**
```
• GetVirtualSwitches()
• Find switch by name
• Extract UUID for network adapter creation
```

**Synchronization** - All three threads must complete before network configuration.

---

### Phase 3: Attach Network Adapter

**Purpose** - Connect LPAR to network via Virtual Switch.

**Steps**

1. Create ClientNetworkAdapter
2. Attach to LPAR (using UUID from Thread 1)
3. Connect to Virtual Switch (using UUID from Thread 3)
4. Assign VLAN ID
5. Configure adapter slot

---

### Phase 4: Parallel Storage Provisioning (Physical || Virtual)

**Purpose** - Provision two types of storage simultaneously.

#### Branch A: Physical SAN Storage (via SVC)

**4A.1 - Create SVC FlashCopy**

```
• Source: Base image volume (e.g., "image-ibm-default-centos-10")
• Target: New volume with LPAR-specific name
• Type: FlashCopy (instant clone)
```

**4A.2 - Get SVC Hosts**

```
• Query all SVC hosts
• Match VIOS WWPNs (from Thread 2)
• Identify target host for mapping
```

**4A.3 - Map Volume to VIOS Host**

```
• mkvdiskhostmap command
• Creates host-to-volume mapping
• Makes LUN visible to VIOS
```

**4A.4 - Run cfgdev on VIOS**

```
• ConfigDevice() API call
• Scans for new LUNs
• Updates VIOS device tree
```

**4A.5 - Identify New Physical Volume**

```
• GetFreePhysicalVolumes()
• Match volume UID with SVC volume
• Verify volume is available
```

**4A.6 - Map Physical Volume to LPAR**

```
• DeletePhysicalVolumeMaps() API
• Creates VSCSI mapping
• Makes disk visible to LPAR
```

#### Branch B: Virtual Disk Storage (Native VIOS)

**4B.1 - Get Volume Groups**

```
• GetVolumeGroups() from VIOS
• List all available VGs
• Check free space on each
```

**4B.2 - Select Optimal VG**

```
• If vg-name specified: Use that VG
• Else: Auto-select VG with most free space
• Verify sufficient space for requested size
```

**4B.3 - Create Virtual Disk**

```
• RunVIOSCommand("mklv -lv [name] [vg] [size]")
• Creates logical volume
• Sets name and size
```

**4B.4 - Map Virtual Disk to LPAR**

```
• DeleteVirtualDiskMaps() API
• Creates VSCSI mapping
• Makes virtual disk visible to LPAR
```

**Error Handling**

- If Physical branch fails: Delete virtual disk and LPAR
- If Virtual branch fails: Delete SVC volume and LPAR
- Ensures no orphaned resources

---

### Phase 5: Optical Media Mapping (Optional)

**Purpose** - Mount ISO files for OS installation or utilities.

**Steps**

1. Check if media-names flag is provided
2. If empty: Skip this phase
3. If provided:
   - Parse comma-separated list
   - For each media name:
     * Create VirtualOpticalMedia mapping
     * Attach to LPAR
     * Set mount type (read-write)

**Example**

```bash
-media-names "rhel9.iso,aix73.iso"
```
Results in two optical media mappings.

---

## Storage Types Summary

| Storage Type | Technology | Provisioning Method | Use Case |
|-------------|-----------|---------------------|----------|
| **Physical SAN** | SVC FlashCopy | SVC → VIOS → LPAR | Boot disk, high-performance |
| **Virtual Disk** | VIOS Logical Volume | VIOS VG → LV → LPAR | Data disk, flexible sizing |
| **Optical Media** | ISO files | VIOS Media Repo → LPAR | OS installation, utilities |

---

## Command Line Flags

```bash
# HMC Configuration
-hmc-ip          HMC IP address (default: "192.0.2.1")
-hmc-user        HMC username (default: "REDACTED_HMC_USER<==")
-hmc-pass        HMC password (required)

# System Configuration
-system-name     Managed System Name (default: "LTC09U31-ZZ")
-lpar-name       Name for the new LPAR (default: "Go_LPAR_100")
-os-type         OS type: aix, linux, aix_linux, ibmi (default: "linux")

# Network Configuration
-vswitch-name    Virtual Switch name (default: "VNET0")
-vlan-id         VLAN ID (default: 1)

# SVC Configuration (Physical Storage)
-svc-ip          SVC IP address (default: "192.0.2.8")
-svc-user        SVC username (default: "REDACTED_SVC_USER<==")
-svc-pass        SVC password (required)
-base-image      Base image for FlashCopy (default: "image-ibm-default-centos-10")

# Virtual Disk Configuration
-vios-name       Target VIOS (auto-select if empty)
-vg-name         Target Volume Group (auto-select if empty)
-vdisk-name      Virtual Disk name (auto-generated if empty)
-vdisk-size      Virtual Disk size in MB (default: 51200)

# Optical Media Configuration
-media-names     Comma-separated ISO files (e.g., "rhel9.iso,aix73.iso")

# Other
-verbose         Enable verbose output
```

---

## Example Usage

### Create LPAR with all storage types

```bash
go run main.go \
  -hmc-pass "password" \
  -svc-pass "password" \
  -lpar-name "MyLPAR" \
  -vdisk-size 102400 \
  -media-names "rhel9.iso"
```

### Create LPAR with auto-selected VIOS and VG

```bash
go run main.go \
  -hmc-pass "password" \
  -svc-pass "password" \
  -lpar-name "AutoLPAR"
```

### Create LPAR without optical media

```bash
go run main.go \
  -hmc-pass "password" \
  -svc-pass "password" \
  -lpar-name "NoMediaLPAR" \
  -media-names ""
```

---

## Success Criteria

✅ LPAR created and visible in HMC
✅ Network adapter attached to VLAN
✅ Physical SAN disk mapped and accessible
✅ Virtual disk created and mapped
✅ Optical media mounted (if specified)
✅ All storage visible in LPAR OS

---

## Error Scenarios and Recovery

### Authentication Failure

- **Symptom**: Cannot connect to HMC or SVC
- **Recovery**: Verify credentials and network connectivity
- **Impact**: Program exits, no resources created

### LPAR Creation Failure

- **Symptom**: CreateLogicalPartition() fails
- **Recovery**: Check system resources (memory, CPU availability)
- **Impact**: Program exits, no cleanup needed

### Physical Storage Failure

- **Symptom**: SVC FlashCopy or mapping fails
- **Recovery**: Automatic cleanup of virtual disk and LPAR
- **Impact**: No orphaned resources

### Virtual Storage Failure

- **Symptom**: Virtual disk creation or mapping fails
- **Recovery**: Automatic cleanup of SVC volume and LPAR
- **Impact**: No orphaned resources

### Optical Media Failure

- **Symptom**: Media mapping fails
- **Recovery**: Warning logged, LPAR remains functional
- **Impact**: LPAR usable without optical media

---

## Related Examples

- **deletepartition**: Complete LPAR and storage cleanup
- **getviosscsimappings**: View all SCSI mappings on VIOS
- **createlparphyvol**: Create LPAR with physical volume only
- **createlparvirvol**: Create LPAR with virtual disk only