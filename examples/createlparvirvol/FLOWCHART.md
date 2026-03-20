# Create LPAR with Virtual Volume (Native Storage) - Flow Chart

## Overview

This flowchart illustrates the complete workflow for creating a PowerVM LPAR with native virtual disk storage using concurrent branch execution.

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
│  • System name, LPAR name, OS type                               │
│  • Network config (vSwitch name, VLAN ID)                        │
│  • Storage config (VIOS, VG, disk name, size)                    │
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
              CONCURRENT BRANCH EXECUTION (2 Branches)
═══════════════════════════════════════════════════════════════════
                             │
        ┌────────────────────┴────────────────────────────────────────────┐
        │                                                                  │
        ▼                                                                  ▼
┌───────────────────────────────────┐              ┌───────────────────────────────────┐
│  BRANCH 1: LPAR + NETWORK         │              │  BRANCH 2: STORAGE DISCOVERY      │
│  (Goroutine 1)                    │              │  (Goroutine 2)                    │
│                                   │              │                                   │
│  ┌─────────────────────────────┐  │              │  ┌─────────────────────────────┐  │
│  │ Step 1.1: Create LPAR       │  │              │  │ Step 2.1: Smart VG Discovery│  │
│  │ • CreateLogicalPartition()  │  │              │  │ • provisionVirtualDisk()    │  │
│  │ • CPU: 0.1-2.0 units        │  │              │  │                             │  │
│  │ • Memory: 2-8 GB            │  │              │  │ ┌─────────────────────────┐ │  │
│  │ • vCPUs: 1-4                │  │              │  │ │ Get All VIOS            │ │  │
│  │ • Sharing: uncapped         │  │              │  │ │ GetVirtualIOServersQuick│ │  │
│  │                             │  │              │  │ └───────────┬─────────────┘ │  │
│  └──────────┬──────────────────┘  │              │  │             │               │  │
│             │                      │              │  │             ▼               │  │
│             ▼                      │              │  │ ┌─────────────────────────┐ │  │
│  ┌─────────────────────────────┐  │              │  │ │ For Each VIOS:          │ │  │
│  │ Step 1.2: Send LPAR UUID    │  │              │  │ │ • Filter by target VIOS │ │  │
│  │ • lparUUIDCh ← lparUUID     │  │              │  │ │   (if specified)        │ │  │
│  │ • Unlock main thread        │  │              │  │ └───────────┬─────────────┘ │  │
│  └──────────┬──────────────────┘  │              │  │             │               │  │
│             │                      │              │  │             ▼               │  │
│             ▼                      │              │  │ ┌─────────────────────────┐ │  │
│  ┌─────────────────────────────┐  │              │  │ │ Get Volume Groups       │ │  │
│  │ Step 1.3: Resolve vSwitch   │  │              │  │ │ GetVolumeGroups()       │ │  │
│  │ • GetVirtualSwitchQuickAll()│  │              │  │ └───────────┬─────────────┘ │  │
│  │ • Match by name             │  │              │  │             │               │  │
│  │ • Get vSwitch UUID          │  │              │  │             ▼               │  │
│  └──────────┬──────────────────┘  │              │  │ ┌─────────────────────────┐ │  │
│             │                      │              │  │ │ For Each VG:            │ │  │
│             ▼                      │              │  │ │ • Filter by target VG   │ │  │
│  ┌─────────────────────────────┐  │              │  │ │   (if specified)        │ │  │
│  │ Step 1.4: Attach Network    │  │              │  │ │ • Check name collision  │ │  │
│  │ • CreateClientNetworkAdapter│  │              │  │ │ • Check free space      │ │  │
│  │ • Attach VLAN to LPAR       │  │              │  │ │ • Prefer Data VG over   │ │  │
│  │ • Use vSwitch UUID          │  │              │  │ │   rootvg                │ │  │
│  │ • Configure VLAN ID         │  │              │  │ └───────────┬─────────────┘ │  │
│  └──────────┬──────────────────┘  │              │  │             │               │  │
│             │                      │              │  │             ▼               │  │
│             ▼                      │              │  │ ┌─────────────────────────┐ │  │
│  ┌─────────────────────────────┐  │              │  │ │ VG Selection Logic:     │ │  │
│  │ Step 1.5: Signal Complete   │  │              │  │ │ • Data VG? → Select     │ │  │
│  │ • networkErrCh ← nil        │  │              │  │ │ • rootvg? → Fallback    │ │  │
│  │   (success)                 │  │              │  │ │ • None? → Error         │ │  │
│  └─────────────────────────────┘  │              │  │ └───────────┬─────────────┘ │  │
│                                   │              │  │             │               │  │
└───────────────────────────────────┘              │  └─────────────┼───────────────┘  │
                                                   │                │                  │
                                                   │                ▼                  │
                                                   │  ┌─────────────────────────────┐  │
                                                   │  │ Step 2.2: Create Virtual Disk│ │
                                                   │  │ • CreateVirtualDisk()       │  │
                                                   │  │ • Create LV in selected VG  │  │
                                                   │  │ • Size: specified MB        │  │
                                                   │  └──────────┬──────────────────┘  │
                                                   │             │                     │
                                                   │             ▼                     │
                                                   │  ┌─────────────────────────────┐  │
                                                   │  │ Step 2.3: Send Storage Info │  │
                                                   │  │ • storageResCh ← result     │  │
                                                   │  │   (VIOS UUID + Name)        │  │
                                                   │  └─────────────────────────────┘  │
                                                   │                                   │
                                                   └───────────────────────────────────┘
                                                                   │
        ┌──────────────────────────────────────────────────────────┴──────────────────────────────────────────────────────────┐
        │                                                                                                                       │
        ▼                                                                                                                       ▼
═══════════════════════════════════════════════════════════════════════════════════════════════════════════════════════════════
                                            SYNCHRONIZATION POINT 1
═══════════════════════════════════════════════════════════════════════════════════════════════════════════════════════════════
        │                                                                                                                       │
        ▼                                                                                                                       ▼
┌───────────────────────────┐                                                                                   ┌───────────────────────────┐
│ Main Thread:              │                                                                                   │ Main Thread:              │
│ Wait for LPAR UUID        │                                                                                   │ Wait for Storage Info     │
│ ← lparUUIDCh              │                                                                                   │ ← storageResCh            │
└───────────┬───────────────┘                                                                                   └───────────┬───────────────┘
            │                                                                                                               │
            └───────────────────────────────────────────────┬───────────────────────────────────────────────────────────────┘
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
│                  STEP 2: MAP VIRTUAL DISK TO LPAR                │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │ • CreateVirtualDiskMap()                                  │  │
│  │ • Map disk to LPAR                                        │  │
│  │ • Use VIOS UUID from storage result                       │  │
│  │ • Use LPAR UUID from branch 1                             │  │
│  │ • Handle RMC warnings (expected for offline LPAR)         │  │
│  └───────────────────────────────────────────────────────────┘  │
└────────────────────────────┬────────────────────────────────────┘
                             │
                             ▼
═══════════════════════════════════════════════════════════════════
                    SYNCHRONIZATION POINT 2
═══════════════════════════════════════════════════════════════════
                             │
                             ▼
                    ┌────────────────┐
                    │ Wait for Network
                    │ ← networkErrCh │
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
    │Net Fail│         │Network  │         │Continue │
    │  EXIT  │         │   OK    │         │         │
    └────────┘         └─────────┘         └────┬────┘
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
│    ✅ Native virtual disk mapped (from optimal VG)               │
│    ✅ Configuration saved to profile                             │
│    ✅ LPAR powered on and booting                                │
└─────────────────────────────────────────────────────────────────┘

---

## Concurrent Execution Timeline

Time →  0s      2s      4s      6s      8s      10s     12s     14s     16s     18s
        │       │       │       │       │       │       │       │       │       │
        ├───────┤
        │ HMC   │
        │ Auth  │
        └───────┴───────┐
                        │ Validate
                        └───────┬───────────────────────────────────────────────────────────┐
                                │                                                           │
                        ┌───────┴───────┐                                           ┌───────┴───────┐
                        │ Branch 1      │                                           │ Branch 2      │
                        ├───────────────┤                                           ├───────────────┤
                        │ Create LPAR   │                                           │ Smart VG      │
                        ├───────────────┤                                           │ Discovery     │
                        │ Send UUID ★   │ ← Unlocks Main Thread                     ├───────────────┤
                        ├───────────────┤                                           │ Create Disk   │
                        │ Find vSwitch  │                                           ├───────────────┤
                        ├───────────────┤                                           │ Send Info ★   │
                        │ Attach Network│                                           └───────────────┘
                        ├───────────────┤                                                   │
                        │ Signal Done ★ │                                                   │
                        └───────────────┘                                                   │
                                │                                                           │
                                └───────────────────┬───────────────────────────────────────┘
                                                    │ ★ Sync Point 1
                                                    ├───────────────┐
                                                    │ Map Disk      │
                                                    └───────┬───────┘
                                                            │ ★ Sync Point 2
                                                            ├───────┬───────┬───────┐
                                                            │ Save  │Profile│Power  │
                                                            └───────┴───────┴───────┘

---

## Smart VG Selection Algorithm Detail

┌─────────────────────────────────────────────────────────────────┐
│           provisionVirtualDisk() - Smart VG Selection            │
└────────────────────────────┬────────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────────┐
│  INPUT: diskSize, targetVIOS (optional), targetVG (optional)     │
└────────────────────────────┬────────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────────┐
│  STEP 1: Get All VIOS Instances                                  │
│  • GetVirtualIOServersQuick()                                    │
│  • Retrieve all VIOS on the system                               │
└────────────────────────────┬────────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────────┐
│  STEP 2: Iterate Through VIOS                                    │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │ For each VIOS:                                            │  │
│  │   ┌───────────────────────────────────────────────────┐   │  │
│  │   │ IF targetVIOS specified:                          │   │  │
│  │   │   • Check if VIOS name matches                    │   │  │
│  │   │   • Skip if no match                              │   │  │
│  │   └───────────────────────────────────────────────────┘   │  │
│  │   ┌───────────────────────────────────────────────────┐   │  │
│  │   │ Get Volume Groups:                                │   │  │
│  │   │   • GetVolumeGroups(viosUUID)                     │   │  │
│  │   └───────────────────────────────────────────────────┘   │  │
│  └───────────────────────────────────────────────────────────┘  │
└────────────────────────────┬────────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────────┐
│  STEP 3: Iterate Through Volume Groups                           │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │ For each VG:                                              │  │
│  │   ┌───────────────────────────────────────────────────┐   │  │
│  │   │ IF targetVG specified:                            │   │  │
│  │   │   • Check if VG name matches                      │   │  │
│  │   │   • Skip if no match                              │   │  │
│  │   └───────────────────────────────────────────────────┘   │  │
│  │   ┌───────────────────────────────────────────────────┐   │  │
│  │   │ Check Name Collision:                             │   │  │
│  │   │   • Iterate through existing virtual disks        │   │  │
│  │   │   • Check if disk name already exists             │   │  │
│  │   │   • Skip VG if collision found                    │   │  │
│  │   └───────────────────────────────────────────────────┘   │  │
│  │   ┌───────────────────────────────────────────────────┐   │  │
│  │   │ Check Capacity:                                   │   │  │
│  │   │   • Parse free space (string → float64)           │   │  │
│  │   │   • Convert disk size MB → GB                     │   │  │
│  │   │   • Skip if insufficient space                    │   │  │
│  │   └───────────────────────────────────────────────────┘   │  │
│  └───────────────────────────────────────────────────────────┘  │
└────────────────────────────┬────────────────────────────────────┘
                             │
                             ▼
                    ┌────────────────┐
                    │  VG Type?      │
                    └────────┬───────┘
                             │
         ┌───────────────────┼───────────────────┐
         │                   │                   │
         ▼                   ▼                   ▼
┌────────────────┐  ┌────────────────┐  ┌────────────────┐
│    rootvg      │  │   Data VG      │  │   Continue     │
│                │  │                │  │                │
│ • Mark as      │  │ • Select       │  │ • Check next   │
│   fallback     │  │   immediately  │  │   VG           │
│ • Continue     │  │ • Return       │  │                │
│   searching    │  │   success      │  │                │
│                │  │                │  │                │
└────────┬───────┘  └────────┬───────┘  └────────┬───────┘
         │                   │                   │
         └───────────────────┼───────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────────┐
│  STEP 4: Final Decision                                          │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │ IF Data VG found:                                         │  │
│  │   • Use Data VG (optimal choice)                          │  │
│  │   • Create virtual disk                                   │  │
│  │   • Return success                                        │  │
│  ├───────────────────────────────────────────────────────────┤  │
│  │ ELSE IF rootvg fallback available:                        │  │
│  │   • Use rootvg (suboptimal but functional)                │  │
│  │   • Create virtual disk                                   │  │
│  │   • Return success                                        │  │
│  ├───────────────────────────────────────────────────────────┤  │
│  │ ELSE:                                                     │  │
│  │   • No suitable VG found                                  │  │
│  │   • Return error                                          │  │
│  └───────────────────────────────────────────────────────────┘  │
└────────────────────────────┬────────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────────┐
│  STEP 5: Create Virtual Disk                                     │
│  • CreateVirtualDisk()                                           │
│  • Create LV in selected VG                                      │
│  • Size: specified in MB                                         │
│  • Return VIOS UUID + Name                                       │
└─────────────────────────────────────────────────────────────────┘

---

## Channel Communication Detail

┌─────────────────────────────────────────────────────────────────┐
│                    GO CHANNEL COMMUNICATION                      │
└────────────────────────────┬────────────────────────────────────┘
                             │
        ┌────────────────────┼────────────────────┐
        │                    │                    │
        ▼                    ▼                    ▼
┌───────────────┐   ┌────────────────┐   ┌───────────────┐
│   Branch 1    │   │   Channels     │   │   Branch 2    │
│  (Goroutine)  │   │ (Thread-Safe)  │   │  (Goroutine)  │
│               │   │                │   │               │
│ ┌───────────┐ │   │ ┌────────────┐ │   │ ┌───────────┐ │
│ │LPAR Create│ │   │ │lparUUIDCh  │ │   │ │VG Discovery│ │
│ └─────┬─────┘ │   │ │            │ │   │ └─────┬─────┘ │
│       │       │   │ │            │ │   │       │       │
│       ▼       │   │ │            │ │   │       ▼       │
│ ┌───────────┐ │   │ │            │ │   │ ┌───────────┐ │
│ │Send UUID  │─┼───┼▶│ UUID       │ │   │ │Create Disk│ │
│ │to Channel │ │   │ │            │ │   │ └─────┬─────┘ │
│ └───────────┘ │   │ └────────────┘ │   │       │       │
│       │       │   │                │   │       ▼       │
│       ▼       │   │ ┌────────────┐ │   │ ┌───────────┐ │
│ ┌───────────┐ │   │ │lparErrCh   │ │   │ │Send Result│─┼───┐
│ │Network    │ │   │ │            │ │   │ │to Channel │ │   │
│ │Setup      │ │   │ │            │ │   │ └───────────┘ │   │
│ └─────┬─────┘ │   │ │            │ │   │               │   │
│       │       │   │ │            │ │   │               │   │
│       ▼       │   │ └────────────┘ │   │               │   │
│ ┌───────────┐ │   │                │   │               │   │
│ │Send Result│─┼───┼─┐              │   │               │   │
│ │to Channel │ │   │ │              │   │               │   │
│ └───────────┘ │   │ │              │   │               │   │
│               │   │ │              │   │               │   │
└───────────────┘   │ │              └───────────────────┘   │
                    │ │                                      │
                    │ ▼                                      │
                    │ ┌────────────┐                        │
                    │ │networkErrCh│                        │
                    │ │            │                        │
                    │ │            │                        │
                    │ │            │                        │
                    │ └────────────┘                        │
                    │                                       │
                    │ ┌────────────┐                        │
                    └▶│storageResCh│◀───────────────────────┘
                      │            │
                      │            │
                      │            │
                      └────────────┘
                             │
                             ▼
                    ┌────────────────┐
                    │  Main Thread   │
                    │                │
                    │ • Wait for     │
                    │   lparUUIDCh   │
                    │ • Wait for     │
                    │   storageResCh │
                    │ • Map disk     │
                    │ • Wait for     │
                    │   networkErrCh │
                    └────────────────┘

---

## Key Function Mapping

| Phase | Main Function | Sub-Functions Called |
| ------- | --------------- | --------------------- |
| **Initialization** | `main()` | `flag.Parse()`, `Login()` |
| **Validation** | `resolveSystemUUID()` | `GetManagedSystemQuickAll()` |
| | `ensureLparDoesNotExist()` | `GetLogicalPartitionByName()` |
| **Branch 1** | Goroutine 1 | `CreateLogicalPartition()` |
| | | `GetVirtualSwitchQuickAll()` |
| | | `CreateClientNetworkAdapter()` |
| **Branch 2** | `provisionVirtualDisk()` | `GetVirtualIOServersQuick()` |
| | | `GetVolumeGroups()` |
| | | `CreateVirtualDisk()` |
| **Sync Point 1** | Main thread | Waits on channels |
| **Mapping** | `CreateVirtualDiskMap()` | Maps disk to LPAR |
| **Sync Point 2** | Main thread | Waits for network |
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

### System Configuration

- `system-name`: Managed system name (default: LTC09U31-ZZ)
- `lpar-name`: LPAR name (default: Go_LPAR_03)
- `os-type`: OS type - aix, linux, aix_linux, ibmi (default: linux)

### Network Configuration

- `vswitch-name`: Virtual switch name (default: VNET0)
- `vlan-id`: VLAN ID (default: 1)

### Storage Configuration

- `vios-name`: Target VIOS name (optional - auto-select if empty)
- `vg-name`: Target Volume Group name (optional - smart select if empty)
- `disk-name`: Virtual disk name (default: lpar03_boot_lv)
- `disk-size`: Disk size in MB (default: 51200 = 50GB)

### LPAR Resources (Hardcoded)

- **CPU**: Min: 0.1, Desired: 0.5, Max: 2.0 units
- **Memory**: Min: 2048, Desired: 4096, Max: 8192 MB
- **vCPUs**: Min: 1, Desired: 1, Max: 4
- **Sharing Mode**: uncapped

---

## Success Indicators

- ✅ HMC authentication completed
- ✅ System validated
- ✅ Branch 1: LPAR created, network attached
- ✅ Branch 2: Optimal VG selected, virtual disk created
- ✅ Synchronization Point 1: Both branches completed
- ✅ Virtual disk mapped to LPAR
- ✅ Synchronization Point 2: Network verified
- ✅ Configuration saved to profile
- ✅ LPAR powered on

---

## Notes

1. **Concurrent Branches**: Uses Go channels for thread-safe communication between goroutines and main thread.

2. **Smart VG Selection**: Automatically finds the best Volume Group:
   - Prefers Data VGs over rootvg
   - Checks for name collisions and capacity
   - Uses rootvg as fallback if no Data VG available

3. **Synchronization Points**: Two critical sync points ensure proper ordering:
   - Point 1: Wait for LPAR UUID + Storage Info before mapping
   - Point 2: Wait for network completion before finalization

4. **Channel Communication**: Uses 5 channels for coordination:
   - `lparUUIDCh`: LPAR UUID from Branch 1
   - `lparErrCh`: LPAR creation errors
   - `networkErrCh`: Network configuration errors
   - `storageResCh`: Storage result (VIOS UUID + Name)
   - `storageErrCh`: Storage provisioning errors

5. **Native Storage**: Uses VIOS Volume Groups and Logical Volumes - no external SAN required.

6. **Deferred Cleanup**: HMC logout is deferred to ensure cleanup even if errors occur.

7. **RMC Warnings**: Expected warnings about RMC (Resource Monitoring and Control) for offline LPARs are handled gracefully.
