# Create Partition Workflow - Block Flow Chart

## Overview
This flowchart illustrates the complete workflow for creating a PowerVM partition with storage provisioning.

---

## Main Flow Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│                         START PROGRAM                            │
└────────────────────────────┬────────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────────┐
│                    PARSE COMMAND LINE FLAGS                      │
│  • HMC credentials (IP, username, password)                      │
│  • SVC credentials (IP, username, password)                      │
│  • System name, OS type, template options                        │
│  • Partition config (CPU, memory, network)                       │
└────────────────────────────┬────────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────────┐
│                    INITIALIZE HMC CLIENT                         │
│  • Create HmcRestClient                                          │
│  • Login to HMC                                                  │
│  • Set up deferred logout                                        │
└────────────────────────────┬────────────────────────────────────┘
                             │
                             ▼
                    ┌────────┴────────┐
                    │  Check Flags    │
                    └────────┬────────┘
                             │
        ┌────────────────────┼────────────────────┐
        │                    │                    │
        ▼                    ▼                    ▼
┌──────────────┐   ┌──────────────┐   ┌──────────────────┐
│List Template?│   │Get Template? │   │Create Partition? │
│    (--list)  │   │   (--name)   │   │   (--os-type)    │
└──────┬───────┘   └──────┬───────┘   └────────┬─────────┘
       │                  │                     │
       ▼                  ▼                     ▼
┌──────────────┐   ┌──────────────┐   ┌──────────────────┐
│List All IDs  │   │Get Specific  │   │ MAIN WORKFLOW    │
│   & EXIT     │   │  ID & EXIT   │   │  (See Below)     │
└──────────────┘   └──────────────┘   └────────┬─────────┘
                                                │
                                                ▼
═══════════════════════════════════════════════════════════════════
                    MAIN PARTITION CREATION WORKFLOW
═══════════════════════════════════════════════════════════════════
                                                │
                                                ▼
┌─────────────────────────────────────────────────────────────────┐
│                  STEP 1: SYSTEM VALIDATION                       │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │ • Get Managed System by Name                              │  │
│  │ • Retrieve System UUID                                    │  │
│  │ • Validate Processor Compatibility Mode                   │  │
│  │ • Check for Existing LPAR with same name                  │  │
│  └───────────────────────────────────────────────────────────┘  │
└────────────────────────────┬────────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────────┐
│              STEP 2: TEMPLATE CREATION & CONFIGURATION           │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │ 2.1 Select Reference Template                             │  │
│  │     • AIX/Linux: QuickStart_lpar_rpa_2                    │  │
│  │     • IBMi: QuickStart_lpar_IBMi_2                        │  │
│  ├───────────────────────────────────────────────────────────┤  │
│  │ 2.2 Generate Unique Temporary Template Name               │  │
│  │     • Format: hmctool_powervm_create_XXXX                 │  │
│  ├───────────────────────────────────────────────────────────┤  │
│  │ 2.3 Copy Reference Template                               │  │
│  │     • CopyPartitionTemplate()                             │  │
│  ├───────────────────────────────────────────────────────────┤  │
│  │ 2.4 Retrieve Template XML                                 │  │
│  │     • GetPartitionTemplate()                              │  │
│  │     • Extract Template UUID                               │  │
│  ├───────────────────────────────────────────────────────────┤  │
│  │ 2.5 Update Partition Name & ID Configuration              │  │
│  │     • UpdateLparNameAndIDToDom()                          │  │
│  │     • Set VM name, partition ID                           │  │
│  ├───────────────────────────────────────────────────────────┤  │
│  │ 2.6 Update Processor & Memory Configuration               │  │
│  │     • UpdateProcMemSettingsToDom()                        │  │
│  │     • Set CPU count, proc units, memory                   │  │
│  │     • Configure min/max/desired values                    │  │
│  │     • Set processor mode (capped/uncapped)                │  │
│  │     • Configure weight and shared proc pool               │  │
│  ├───────────────────────────────────────────────────────────┤  │
│  │ 2.7 Update Virtual Network Configuration                  │  │
│  │     • UpdateVirtualNWSettingsToDom()                      │  │
│  │     ┌─────────────────────────────────────────────────┐   │  │
│  │     │ For each network in VirtNetworkConfigs:        │   │  │
│  │     │   • Network Name (e.g., VNET0)                 │   │  │
│  │     │   • Slot Number (e.g., 49)                     │   │  │
│  │     │   • Virtual Slot Number (e.g., 49)             │   │  │
│  │     │   • Create VirtualEthernetAdapter XML          │   │  │
│  │     │   • Add to template ClientNetworkAdapters      │   │  │
│  │     └─────────────────────────────────────────────────┘   │  │
│  └───────────────────────────────────────────────────────────┘  │
└────────────────────────────┬────────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────────┐
│                STEP 3: STORAGE CONFIGURATION                     │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │ 3.1 SVC Authentication                                     │  │
│  │     • Initialize SVC Client                               │  │
│  │     • Authenticate to SVC                                 │  │
│  ├───────────────────────────────────────────────────────────┤  │
│  │ 3.2 Host Management                                        │  │
│  │     ┌─────────────────────────────────────────────────┐   │  │
│  │     │ • Check if WWPNs already associated             │   │  │
│  │     │ • If exists: Get existing host ID               │   │  │
│  │     │ • If not: Create new host (Mkhost)              │   │  │
│  │     │   - Name, WWPNs, Type, Protocol                 │   │  │
│  │     │ • Retrieve host details (LshostByTarget)        │   │  │
│  │     └─────────────────────────────────────────────────┘   │  │
│  ├───────────────────────────────────────────────────────────┤  │
│  │ 3.3 Volume Creation                                        │  │
│  │     ┌─────────────────────────────────────────────────┐   │  │
│  │     │ • Create volume (Mkvdisk)                       │   │  │
│  │     │   - Name, size, mdisk group                     │   │  │
│  │     │   - Thin provisioning settings                  │   │  │
│  │     │ • Get source volume details (for cloning)       │   │  │
│  │     │ • Get target volume details                     │   │  │
│  │     │ • Create FlashCopy mapping (Mkfcmap)            │   │  │
│  │     │   - Source, target, copy rate                   │   │  │
│  │     │ • Start FlashCopy (Startfcmap)                  │   │  │
│  │     │   - Prep and restore flags                      │   │  │
│  │     └─────────────────────────────────────────────────┘   │  │
│  ├───────────────────────────────────────────────────────────┤  │
│  │ 3.4 Volume to Host Mapping                                │  │
│  │     • Map volume to host (Mkvdiskhostmap)                 │  │
│  │     • Assign SCSI LUN (optional)                          │  │
│  ├───────────────────────────────────────────────────────────┤  │
│  │ 3.5 VSCSI Configuration                                    │  │
│  │     ┌─────────────────────────────────────────────────┐   │  │
│  │     │ For each volume in VolumeConfigs:              │   │  │
│  │     │   • Get VIOS UUID (GetViosID)                   │   │  │
│  │     │   • Configure device (ConfigDevice)             │   │  │
│  │     │   • Identify free volume on VIOS                │   │  │
│  │     │     - Match by volume UID                       │   │  │
│  │     │   • Generate VSCSI payload (AddVSCSIPayload)    │   │  │
│  │     │     - Client adapter configuration              │   │  │
│  │     │     - Server adapter configuration              │   │  │
│  │     │     - Storage device mapping                    │   │  │
│  │     │   • Add VSCSI to template (AddVSCSI)            │   │  │
│  │     │   • Update template (UpdatePartitionTemplate)   │   │  │
│  │     └─────────────────────────────────────────────────┘   │  │
│  └───────────────────────────────────────────────────────────┘  │
└────────────────────────────┬────────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────────┐
│          STEP 4: TEMPLATE TRANSFORMATION & VALIDATION            │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │ 4.1 Transform Template                                     │  │
│  │     • TransformPartitionTemplate()                        │  │
│  │     • Convert template to deployable format               │  │
│  │     • Resolve system-specific settings                    │  │
│  ├───────────────────────────────────────────────────────────┤  │
│  │ 4.2 Validate Template                                      │  │
│  │     • CheckPartitionTemplate()                            │  │
│  │     • Ensure template is valid for deployment             │  │
│  │     • Verify all required resources are available         │  │
│  └───────────────────────────────────────────────────────────┘  │
└────────────────────────────┬────────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────────┐
│                  STEP 5: PARTITION DEPLOYMENT                    │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │ • Deploy partition from template                          │  │
│  │ • DeployPartitionTemplate()                               │  │
│  │ • Create LPAR with all configured resources:              │  │
│  │   - Compute (CPU, Memory)                                 │  │
│  │   - Network (Virtual Ethernet Adapters)                   │  │
│  │   - Storage (VSCSI Adapters & Volumes)                    │  │
│  │ • Retrieve partition UUID                                 │  │
│  └───────────────────────────────────────────────────────────┘  │
└────────────────────────────┬────────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────────┐
│                  STEP 6: POWER ON PARTITION                      │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │ • Get partition profile UUID                              │  │
│  │ • Power on partition (PowerOnPartition)                   │  │
│  │ • Boot mode: manual                                       │  │
│  │ • OS type specific boot parameters                        │  │
│  └───────────────────────────────────────────────────────────┘  │
└────────────────────────────┬────────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────────┐
│                  STEP 7: CLEANUP & FINALIZATION                  │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │ • Delete temporary template (deferred)                    │  │
│  │ • Log success message                                     │  │
│  │ • Return partition UUID                                   │  │
│  └───────────────────────────────────────────────────────────┘  │
└────────────────────────────┬────────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────────┐
│                         SUCCESS - EXIT                           │
│  Partition created with:                                         │
│    ✅ Compute resources configured                               │
│    ✅ Network adapters attached                                  │
│    ✅ Storage volumes mapped                                     │
│    ✅ Partition powered on                                       │
└─────────────────────────────────────────────────────────────────┘
```

---

## Network Configuration Detail

```
┌─────────────────────────────────────────────────────────────────┐
│              NETWORK CONFIGURATION (Step 2.7)                    │
└────────────────────────────┬────────────────────────────────────┘
                             │
                             ▼
                ┌────────────────────────┐
                │ VirtNetworkConfigs     │
                │ (Array of Networks)    │
                └────────┬───────────────┘
                         │
                         ▼
        ┌────────────────────────────────────┐
        │  For Each Network Configuration:   │
        └────────┬───────────────────────────┘
                 │
                 ▼
    ┌────────────────────────────────────────────────┐
    │  Network Parameters:                           │
    │  • NetworkName: "VNET0"                        │
    │  • SlotNumber: 49                              │
    │  • VirtualSlotNumber: 49                       │
    └────────┬───────────────────────────────────────┘
             │
             ▼
    ┌────────────────────────────────────────────────┐
    │  UpdateVirtualNWSettingsToDom()                │
    │  ┌──────────────────────────────────────────┐  │
    │  │ 1. Create VirtualEthernetAdapter XML     │  │
    │  │    • Set adapter type                    │  │
    │  │    • Set slot numbers                    │  │
    │  │    • Set network name                    │  │
    │  │    • Configure MAC address (auto)        │  │
    │  │    • Set port VLAN ID                    │  │
    │  ├──────────────────────────────────────────┤  │
    │  │ 2. Add to ClientNetworkAdapters          │  │
    │  │    • Insert into template XML            │  │
    │  │    • Update adapter count                │  │
    │  └──────────────────────────────────────────┘  │
    └────────┬───────────────────────────────────────┘
             │
             ▼
    ┌────────────────────────────────────────────────┐
    │  Result: Network adapter configured in         │
    │  partition template, ready for deployment      │
    └────────────────────────────────────────────────┘
```

---

## Complete Resource Configuration Flow

```
┌─────────────────────────────────────────────────────────────────┐
│                    PARTITION RESOURCES                           │
└────────────────────────────┬────────────────────────────────────┘
                             │
        ┌────────────────────┼────────────────────┐
        │                    │                    │
        ▼                    ▼                    ▼
┌───────────────┐   ┌────────────────┐   ┌───────────────┐
│   COMPUTE     │   │    NETWORK     │   │   STORAGE     │
│               │   │                │   │               │
│ • Processors  │   │ • Virtual      │   │ • VSCSI       │
│ • Proc Units  │   │   Ethernet     │   │   Adapters    │
│ • Memory      │   │   Adapters     │   │ • Physical    │
│ • Proc Mode   │   │ • Network      │   │   Volumes     │
│ • Weight      │   │   Names        │   │ • Volume      │
│ • Compat Mode │   │ • Slot Numbers │   │   Mappings    │
│               │   │ • MAC Address  │   │               │
└───────┬───────┘   └────────┬───────┘   └───────┬───────┘
        │                    │                    │
        └────────────────────┼────────────────────┘
                             │
                             ▼
                    ┌────────────────┐
                    │  Template XML  │
                    │   (Complete)   │
                    └────────┬───────┘
                             │
                             ▼
                    ┌────────────────┐
                    │   Transform    │
                    │   & Validate   │
                    └────────┬───────┘
                             │
                             ▼
                    ┌────────────────┐
                    │     Deploy     │
                    │   Partition    │
                    └────────────────┘
```

---

## Error Handling Flow

```
┌─────────────────────────────────────────────────────────────────┐
│                      ERROR AT ANY STEP                           │
└────────────────────────────┬────────────────────────────────────┘
                             │
                             ▼
                    ┌────────────────┐
                    │  Log Error     │
                    │  with Context  │
                    └────────┬───────┘
                             │
                             ▼
                    ┌────────────────┐
                    │ Cleanup Actions│
                    │ (if needed)    │
                    └────────┬───────┘
                             │
                             ▼
                    ┌────────────────┐
                    │ HMC Logout     │
                    │  (deferred)    │
                    └────────┬───────┘
                             │
                             ▼
                    ┌────────────────┐
                    │  EXIT PROGRAM  │
                    │  with Error    │
                    └────────────────┘
```

---

## Key Function Mapping

| Step | Main Function | Sub-Functions Called |
|------|---------------|---------------------|
| **Initialization** | `main()` | `parseFlags()`, `Login()` |
| **System Validation** | `createPartition()` | `GetManagedSystemByName()`, `validateProcessorCompatibilityMode()`, `checkExistingLPAR()` |
| **Template Creation** | `createTemporaryTemplate()` | `CopyPartitionTemplate()`, `GetPartitionTemplate()`, `UpdateLparNameAndIDToDom()`, `UpdateProcMemSettingsToDom()`, `UpdateVirtualNWSettingsToDom()` |
| **Network Config** | `UpdateVirtualNWSettingsToDom()` | Creates VirtualEthernetAdapter XML, adds to ClientNetworkAdapters |
| **Storage Config** | `configureStorage()` | `createOrGetSVCHost()`, `createSVCVolume()`, `mapVolumeToHost()`, `configureVSCSI()` |
| **SVC Host** | `createOrGetSVCHost()` | `GetHostByWWPN()`, `Mkhost()`, `LshostByTarget()` |
| **SVC Volume** | `createSVCVolume()` | `Mkvdisk()`, `LsVdiskByName()`, `Mkfcmap()`, `Startfcmap()` |
| **VSCSI** | `configureVSCSI()` | `GetViosID()`, `ConfigDevice()`, `identifyFreeVolume()`, `AddVSCSIPayload()`, `AddVSCSI()`, `UpdatePartitionTemplate()` |
| **Transformation** | `transformAndValidateTemplate()` | `TransformPartitionTemplate()`, `CheckPartitionTemplate()` |
| **Deployment** | `deployPartition()` | `DeployPartitionTemplate()` |
| **Power On** | `powerOnPartition()` | `GetPartitionProfile()`, `PowerOnPartition()` |
| **Cleanup** | `deleteTemporaryTemplate()` | `DeletePartitionTemplate()` |

---

## Data Flow

```
┌──────────────┐
│ Command Line │
│    Flags     │
└──────┬───────┘
       │
       ▼
┌──────────────┐     ┌─────────────┐
│    Config    │────▶│  HMC Client │
│    Struct    │     └──────┬──────┘
└──────┬───────┘            │
       │                    │
       │                    ▼
       │            ┌───────────────┐
       │            │ System UUID   │
       │            │ System Element│
       │            └───────┬───────┘
       │                    │
       ▼                    ▼
┌──────────────┐    ┌───────────────┐
│  SVC Client  │    │Template UUID  │
└──────┬───────┘    │Template XML   │
       │            │  + Network    │
       │            │  + Compute    │
       │            └───────┬───────┘
       │                    │
       ▼                    │
┌──────────────┐            │
│  Host ID     │            │
│  Volume Name │            │
│  Volume UID  │            │
└──────┬───────┘            │
       │                    │
       └────────┬───────────┘
                │
                ▼
        ┌───────────────┐
        │ Partition UUID│
        │  (Deployed)   │
        └───────────────┘
```

---

## Configuration Parameters

### HMC Configuration
- `HMCIP`: HMC IP address
- `HMCUser`: HMC username
- `HMCPassword`: HMC password

### SVC Configuration
- `SVCIP`: SVC IP address
- `SVCUser`: SVC username
- `SVCPassword`: SVC password

### Partition Configuration
- `VMName`: Partition name
- `Proc`: Number of processors
- `ProcUnit`: Processor units
- `Memory`: Memory in MB
- `MaxVirtualSlots`: Maximum virtual slots
- `ProcMode`: Processor mode (uncapped/capped)
- `Weight`: Processor weight
- `ProcCompatibilityMode`: CPU compatibility mode
- `SharedProcPool`: Shared processor pool ID

### Network Configuration
- `VirtNetworkConfigs`: Array of virtual network configurations
  - `NetworkName`: Network name (e.g., "VNET0")
  - `SlotNumber`: Physical slot number
  - `VirtualSlotNumber`: Virtual slot number
  - **Auto-configured**: MAC address, port VLAN ID

### Storage Configuration
- `VolumeConfigs`: Array of volume configurations
  - `ViosName`: VIOS name managing the volume
  - `VolumeName`: Volume name on VIOS

---

## Success Indicators

Throughout the workflow, success is indicated by:
- ✅ Emoji markers in console output
- Verbose logging (when enabled)
- Successful function returns without errors
- Final message: "🎉 SUCCESS: Partition created successfully"

---

## Notes

1. **Network Configuration**: Virtual Ethernet adapters are configured in Step 2.7, before storage configuration. This ensures network connectivity is available when the partition boots.

2. **Deferred Cleanup**: The temporary template is deleted using Go's `defer` mechanism, ensuring cleanup even if errors occur.

3. **Error Propagation**: Errors are propagated up the call stack with context, making debugging easier.

4. **Idempotency**: The workflow checks for existing resources (LPAR, host) to avoid duplicates.

5. **Verbose Mode**: When enabled, provides detailed logging at each step for troubleshooting.

6. **Modular Design**: Each major step is encapsulated in its own function, making the code maintainable and testable.

7. **Resource Order**: Resources are configured in this order:
   - Compute (CPU, Memory)
   - Network (Virtual Ethernet)
   - Storage (VSCSI, Volumes)