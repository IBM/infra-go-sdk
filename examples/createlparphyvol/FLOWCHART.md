# Create LPAR with Physical Volume (SAN Storage) - Flow Chart

## Overview

This flowchart illustrates the complete workflow for creating a PowerVM LPAR with SAN storage provisioning using parallel execution.

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
│  • Network config (vSwitch name, VLAN ID)                        │
│  • Storage config (base image name)                              │
└────────────────────────────┬────────────────────────────────────┘
                             │
                             ▼
═══════════════════════════════════════════════════════════════════
              PHASE 1: PARALLEL AUTHENTICATION (2 Threads)
═══════════════════════════════════════════════════════════════════
                             │
        ┌────────────────────┴────────────────────┐
        │                                         │
        ▼                                         ▼
┌──────────────────┐                    ┌──────────────────┐
│  Thread 1: HMC   │                    │  Thread 2: SVC   │
│                  │                    │                  │
│ • NewHmcRestClient                    │ • NewClient      │
│ • Login()        │                    │ • Authenticate() │
│                  │                    │                  │
└────────┬─────────┘                    └────────┬─────────┘
         │                                       │
         └───────────────┬───────────────────────┘
                         │
                         ▼
                ┌────────────────┐
                │  Wait for Both │
                │   wg.Wait()    │
                └────────┬───────┘
                         │
                         ▼
                ┌────────────────┐
                │ Check Errors?  │
                └────────┬───────┘
                         │
         ┌───────────────┼───────────────┐
         │               │               │
         ▼               ▼               ▼
    ┌────────┐    ┌──────────┐    ┌──────────┐
    │HMC Fail│    │SVC Fail  │    │Both OK   │
    │  EXIT  │    │  EXIT    │    │Continue  │
    └────────┘    └──────────┘    └────┬─────┘
                                        │
                                        ▼
┌─────────────────────────────────────────────────────────────────┐
│              STEP 1: SYSTEM VALIDATION                           │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │ • resolveSystemUUID()                                     │  │
│  │   - GetManagedSystemQuickAll()                            │  │
│  │   - Find system by name                                   │  │
│  │   - Return system UUID                                    │  │
│  ├───────────────────────────────────────────────────────────┤  │
│  │ • ensureLparDoesNotExist()                                │  │
│  │   - GetLogicalPartitionByName()                           │  │
│  │   - Verify LPAR name is available                         │  │
│  │   - Fatal error if exists                                 │  │
│  └───────────────────────────────────────────────────────────┘  │
└────────────────────────────┬────────────────────────────────────┘
                             │
                             ▼
═══════════════════════════════════════════════════════════════════
         PHASE 2: 3-WAY PARALLEL OPERATIONS (3 Threads)
═══════════════════════════════════════════════════════════════════
                             │
        ┌────────────────────┼────────────────────┐
        │                    │                    │
        ▼                    ▼                    ▼
┌───────────────┐   ┌────────────────┐   ┌───────────────┐
│ Thread 1:     │   │  Thread 2:     │   │  Thread 3:    │
│ Create LPAR   │   │  Discover VIOS │   │  Find vSwitch │
│               │   │                │   │               │
│ CreateLogical │   │ getViosWwpnMap │   │ GetVirtualSwitch
│ Partition()   │   │ ()             │   │ QuickAll()    │
│               │   │                │   │               │
│ • Name        │   │ • Get all VIOS │   │ • Get switches│
│ • CPU: 0.1-2.0│   │ • Extract FC   │   │ • Match by    │
│ • Mem: 2-8 GB │   │   WWPNs        │   │   name        │
│ • Sharing:    │   │ • Build WWPN   │   │ • Return UUID │
│   uncapped    │   │   map          │   │               │
│               │   │                │   │               │
└───────┬───────┘   └────────┬───────┘   └───────┬───────┘
        │                    │                    │
        └────────────────────┼────────────────────┘
                             │
                             ▼
                    ┌────────────────┐
                    │  Wait for All  │
                    │   wg.Wait()    │
                    └────────┬───────┘
                             │
                             ▼
                    ┌────────────────┐
                    │ Check Errors?  │
                    └────────┬───────┘
                             │
         ┌───────────────────┼───────────────────┐
         │                   │                   │
         ▼                   ▼                   ▼
    ┌────────┐         ┌─────────┐         ┌─────────┐
    │Any Fail│         │All OK   │         │Continue │
    │  EXIT  │         │         │         │         │
    └────────┘         └─────────┘         └────┬────┘
                                                 │
                                                 ▼
═══════════════════════════════════════════════════════════════════
         PHASE 3: 2-WAY PARALLEL OPERATIONS (2 Threads)
═══════════════════════════════════════════════════════════════════
                                                 │
                        ┌────────────────────────┴────────────────────────┐
                        │                                                 │
                        ▼                                                 ▼
        ┌───────────────────────────┐                   ┌───────────────────────────┐
        │  Thread 1: Network        │                   │  Thread 2: SVC Storage    │
        │                           │                   │                           │
        │  CreateClientNetwork      │                   │  provisionSVCStorage()    │
        │  Adapter()                │                   │                           │
        │                           │                   │  ┌─────────────────────┐  │
        │  • Attach VLAN to LPAR    │                   │  │ 1. Fetch Fabric     │  │
        │  • Use vSwitch UUID       │                   │  │    Lsfabric()       │  │
        │  • Configure VLAN ID      │                   │  ├─────────────────────┤  │
        │                           │                   │  │ 2. Map WWPNs        │  │
        │                           │                   │  │    to SVC Hosts     │  │
        │                           │                   │  ├─────────────────────┤  │
        │                           │                   │  │ 3. Host Management  │  │
        │                           │                   │  │    • Check exists   │  │
        │                           │                   │  │    • Mkhost() if    │  │
        │                           │                   │  │      needed         │  │
        │                           │                   │  │    • LshostByTarget │  │
        │                           │                   │  ├─────────────────────┤  │
        │                           │                   │  │ 4. Create Volume    │  │
        │                           │                   │  │    • Mkvdisk()      │  │
        │                           │                   │  │    • 120GB thin     │  │
        │                           │                   │  ├─────────────────────┤  │
        │                           │                   │  │ 5. FlashCopy Clone  │  │
        │                           │                   │  │    • LsVdiskByName  │  │
        │                           │                   │  │      (source/target)│  │
        │                           │                   │  │    • Mkfcmap()      │  │
        │                           │                   │  │    • Startfcmap()   │  │
        │                           │                   │  ├─────────────────────┤  │
        │                           │                   │  │ 6. Map to Host      │  │
        │                           │                   │  │    • Mkvdiskhostmap │  │
        │                           │                   │  └─────────────────────┘  │
        └─────────────┬─────────────┘                   └─────────────┬─────────────┘
                      │                                               │
                      └───────────────────┬───────────────────────────┘
                                          │
                                          ▼
                                 ┌────────────────┐
                                 │  Wait for Both │
                                 │   wg.Wait()    │
                                 └────────┬───────┘
                                          │
                                          ▼
                                 ┌────────────────┐
                                 │ Check Errors?  │
                                 └────────┬───────┘
                                          │
                      ┌───────────────────┼───────────────────┐
                      │                   │                   │
                      ▼                   ▼                   ▼
                 ┌────────┐         ┌─────────┐         ┌─────────┐
                 │Any Fail│         │Both OK  │         │Continue │
                 │  EXIT  │         │         │         │         │
                 └────────┘         └─────────┘         └────┬────┘
                                                              │
                                                              ▼
┌─────────────────────────────────────────────────────────────────┐
│           STEP 2: DISCOVER & MAP PHYSICAL VOLUME                 │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │ 2.1 Run cfgdev on VIOS                                    │  │
│  │     • ConfigDevice()                                      │  │
│  │     • Scan for new SAN LUN                                │  │
│  ├───────────────────────────────────────────────────────────┤  │
│  │ 2.2 Identify Physical Volume                              │  │
│  │     • identifyFreeVolume()                                │  │
│  │     • GetFreePhyVolume()                                  │  │
│  │     • Match by SVC volume UID                             │  │
│  │     • Return disk name (e.g., hdisk3)                     │  │
│  ├───────────────────────────────────────────────────────────┤  │
│  │ 2.3 Map Physical Volume to LPAR                           │  │
│  │     • CreatePhysicalVolumeMap()                           │  │
│  │     • Attach disk to LPAR                                 │  │
│  │     • Handle RMC warnings (expected for offline LPAR)     │  │
│  └───────────────────────────────────────────────────────────┘  │
└────────────────────────────┬────────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────────┐
│                  STEP 3: SAVE CONFIGURATION                      │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │ • SaveCurrentLparConfig()                                 │  │
│  │ • Save to profile: "default_profile"                      │  │
│  │ • Overwrite if exists                                     │  │
│  └───────────────────────────────────────────────────────────┘  │
└────────────────────────────┬────────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────────┐
│                  STEP 4: POWER ON LPAR                           │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │ • GetPartitionProfile()                                   │  │
│  │   - Get profile UUID                                      │  │
│  │ • PowerOnPartition()                                      │  │
│  │   - Boot mode: normal                                     │  │
│  │   - OS type specific parameters                           │  │
│  └───────────────────────────────────────────────────────────┘  │
└────────────────────────────┬────────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────────┐
│                         SUCCESS - EXIT                           │
│  LPAR created and running with:                                  │
│    ✅ CPU: 0.1-2.0 units, Memory: 2-8 GB                         │
│    ✅ Network adapter attached (VLAN configured)                 │
│    ✅ SAN storage mapped (120GB FlashCopy volume)                │
│    ✅ Configuration saved to profile                             │
│    ✅ LPAR powered on and booting                                │
└─────────────────────────────────────────────────────────────────┘

## Parallel Execution Timeline

Time →  0s      2s      4s      6s      8s      10s     12s     14s     16s     18s     20s     22s     24s
        │       │       │       │       │       │       │       │       │       │       │       │       │
        ├───────┤
        │ HMC   │
        │ Auth  │
        ├───────┤
        │ SVC   │
        │ Auth  │
        └───────┴───────┐
                        │ Validate
                        └───────┬───────────────────────────────┐
                                │ Create LPAR                   │
                                ├───────────────┐               │
                                │ Discover VIOS │               │
                                ├───────┐       │               │
                                │vSwitch│       │               │
                                └───────┴───────┴───────────────┴───────────────────────────────┐
                                                                │ Network                       │
                                                                ├───────────────────────────────┤
                                                                │ SVC Storage (FlashCopy)       │
                                                                └───────────────────────────────┴───────┬───────┬───────┬───────┐
                                                                                                        │cfgdev │ Map   │Power  │
                                                                                                        └───────┴───────┴───────┘

## SVC Storage Provisioning Detail

┌─────────────────────────────────────────────────────────────────┐
│              provisionSVCStorage() - Detailed Flow               │
└────────────────────────────┬────────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────────┐
│  INPUT: baseImageName, viosWwpnMap                               │
└────────────────────────────┬────────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────────┐
│  STEP 1: Fetch SVC Fabric Logins                                 │
│  • Lsfabric()                                                    │
│  • Get all logged-in WWPNs                                       │
│  • Build WWPN → Host mapping                                     │
└────────────────────────────┬────────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────────┐
│  STEP 2: Match VIOS WWPNs to SVC Hosts                           │
│  • Iterate through viosWwpnMap                                   │
│  • Check if WWPN exists in fabric logins                         │
│  • If match found: hostExists = true                             │
└────────────────────────────┬────────────────────────────────────┘
                             │
                             ▼
                    ┌────────────────┐
                    │  Host Exists?  │
                    └────────┬───────┘
                             │
         ┌───────────────────┼───────────────────┐
         │                   │                   │
         ▼                   ▼                   ▼
    ┌────────┐         ┌─────────┐         ┌─────────┐
    │  Yes   │         │   No    │         │Continue │
    │Use Host│         │Create   │         │         │
    └────┬───┘         │New Host │         └────┬────┘
         │             └────┬────┘              │
         │                  │                   │
         │                  ▼                   │
         │         ┌─────────────────┐          │
         │         │  Mkhost()       │          │
         │         │  • Name         │          │
         │         │  • WWPNs        │          │
         │         │  • Type: generic│          │
         │         │  • Protocol:scsi│          │
         │         └────┬────────────┘          │
         │              │                       │
         └──────────────┴───────────────────────┘
                        │
                        ▼
┌─────────────────────────────────────────────────────────────────┐
│  STEP 3: Get Host Details                                        │
│  • LshostByTarget()                                              │
│  • Retrieve host ID                                              │
└────────────────────────────┬────────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────────┐
│  STEP 4: Create Volume                                           │
│  • Mkvdisk()                                                     │
│  • Name: lpar_boot_vol_\<timestamp\>                             │
│  • Size: 120 GB                                                  │
│  • Thin provisioned (rsize: 2%, autoexpand: true)                │
│  • Mdisk group: 0                                                │
└────────────────────────────┬────────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────────┐
│  STEP 5: Get Source & Target Volume Details                      │
│  • LsVdiskByName(baseImageName) → sourceVol                      │
│  • LsVdiskByName(newVolumeName) → targetVol                      │
└────────────────────────────┬────────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────────┐
│  STEP 6: Create FlashCopy Mapping                                │
│  • Mkfcmap()                                                     │
│  • Name: fcmap_\<timestamp\>                                       │
│  • Source: sourceVol.ID                                          │
│  • Target: targetVol.ID                                          │
│  • Copy rate: 150                                                │
│  • Incremental: true, AutoDelete: true                           │
└────────────────────────────┬────────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────────┐
│  STEP 7: Start FlashCopy Operation                               │
│  • Startfcmap()                                                  │
│  • Prep: true, Restore: true                                     │
│  • Begin cloning source → target                                 │
└────────────────────────────┬────────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────────┐
│  STEP 8: Map Volume to Host                                      │
│  • Mkvdiskhostmap()                                              │
│  • Host: targetHost.ID                                           │
│  • VDisk: volume.Name                                            │
│  • Force: true                                                   │
│  • Makes LUN visible to VIOS                                     │
└────────────────────────────┬────────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────────┐
│  OUTPUT: targetVol, selectedViosName, nil (success)              │
└─────────────────────────────────────────────────────────────────┘
---

## Key Function Mapping

| Phase | Main Function | Sub-Functions Called |
| ------- | --------------- | --------------------- |
| **Initialization** | `main()` | `flag.Parse()` |
| **Phase 1: Auth** | Parallel goroutines | `Login()`, `Authenticate()` |
| **Validation** | `resolveSystemUUID()` | `GetManagedSystemQuickAll()` |
| | `ensureLparDoesNotExist()` | `GetLogicalPartitionByName()` |
| **Phase 2: 3-Way** | Thread 1 | `CreateLogicalPartition()` |
| | Thread 2 | `getViosWwpnMap()`, `GetVirtualIOServersQuick()` |
| | Thread 3 | `GetVirtualSwitchQuickAll()` |
| **Phase 3: 2-Way** | Thread 1 | `CreateClientNetworkAdapter()` |
| | Thread 2 | `provisionSVCStorage()` (see detail above) |
| **Storage Mapping** | `ConfigDevice()` | Runs cfgdev on VIOS |
| | `identifyFreeVolume()` | `GetFreePhyVolume()` |
| | `CreatePhysicalVolumeMap()` | Maps disk to LPAR |
| **Finalization** | `SaveCurrentLparConfig()` | Saves to profile |
| | `GetPartitionProfile()` | Gets profile UUID |
| | `PowerOnPartition()` | Boots LPAR |

---

## Error Handling

┌─────────────────────────────────────────────────────────────────┐
│                      ERROR AT ANY STEP                           │
└────────────────────────────┬────────────────────────────────────┘
                             │
                             ▼
                    ┌────────────────┐
                    │  Log Error     │
                    │  log.Fatalf()  │
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

---

## Configuration Parameters

### HMC Configuration

- `hmc-ip`: HMC IP address (default: 192.0.2.1)
- `hmc-user`: HMC username (default: REDACTED_HMC_USER<==)
- `hmc-pass`: HMC password (default: REDACTED_HMC_PASS<==)

### SVC Configuration

- `svc-ip`: SVC IP address (default: 192.0.2.8)
- `svc-user`: SVC username (default: REDACTED_SVC_USER<==)
- `svc-pass`: SVC password (default: REDACTED_HMC_PASS<==)

### System Configuration

- `system-name`: Managed system name (default: LTC09U31-ZZ)
- `lpar-name`: LPAR name (default: Go_LPAR_99)
- `os-type`: OS type - aix, linux, aix_linux, ibmi (default: linux)

### Network Configuration

- `vswitch-name`: Virtual switch name (default: VNET0)
- `vlan-id`: VLAN ID (default: 1)

### Storage Configuration

- `base-image`: Base image for FlashCopy (default: image-ibm-default-centos-10)

### LPAR Resources (Hardcoded)

- **CPU**: Min: 0.1, Desired: 0.5, Max: 2.0 units
- **Memory**: Min: 2048, Desired: 4096, Max: 8192 MB
- **vCPUs**: Min: 1, Desired: 1, Max: 4
- **Sharing Mode**: uncapped

---

## Success Indicators

- ✅ Parallel authentication completed
- ✅ System validated
- ✅ LPAR created
- ✅ VIOS WWPNs discovered
- ✅ vSwitch resolved
- ✅ Network adapter attached
- ✅ SVC storage provisioned (FlashCopy complete)
- ✅ Physical volume mapped to LPAR
- ✅ Configuration saved
- ✅ LPAR powered on

---

## Notes

1. **Parallel Execution**: Uses Go's goroutines and sync.WaitGroup for concurrent operations, significantly reducing total execution time.

2. **Error Propagation**: Each parallel thread captures errors independently and reports them after synchronization.

3. **SAN Storage**: Uses IBM SVC FlashCopy technology for instant volume cloning from a base image.

4. **VIOS Integration**: Automatically discovers VIOS Fibre Channel WWPNs and maps them to SVC hosts.

5. **RMC Warnings**: Expected warnings about RMC (Resource Monitoring and Control) for offline LPARs are handled gracefully.

6. **Deferred Cleanup**: HMC logout is deferred to ensure cleanup even if errors occur.
