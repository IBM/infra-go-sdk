# Delete Partition Workflow - Block Flow Chart

## Overview

This flowchart illustrates the complete workflow for deleting a PowerVM partition with comprehensive storage cleanup from both HMC and SVC.

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
│  • System name, LPAR name to delete                              │
│  • Verbose flag                                                  │
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
═══════════════════════════════════════════════════════════════════
                    PHASE 1: HMC RESOLUTION & SHUTDOWN
═══════════════════════════════════════════════════════════════════
                             │
                             ▼
┌─────────────────────────────────────────────────────────────────┐
│                  STEP 1: LOCATE & SHUTDOWN PARTITION             │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │ 1.1 Get Managed System UUID                               │  │
│  │     • GetManagedSystemByName()                            │  │
│  ├───────────────────────────────────────────────────────────┤  │
│  │ 1.2 Get All Partitions on System                          │  │
│  │     • GetLogicalPartitionsQuickAll()                      │  │
│  ├───────────────────────────────────────────────────────────┤  │
│  │ 1.3 Find Target LPAR                                      │  │
│  │     • Match by partition name                             │  │
│  │     • Extract UUID and current state                      │  │
│  ├───────────────────────────────────────────────────────────┤  │
│  │ 1.4 Check Partition State                                 │  │
│  │     ┌─────────────────────────────────────────────────┐   │  │
│  │     │ If state != "not activated":                    │   │  │
│  │     │   • PowerOffPartition(Immediate)                │   │  │
│  │     │   • Poll for "not activated" state (20 retries) │   │  │
│  │     │   • Wait 5 seconds between polls                │   │  │
│  │     │ Else:                                            │   │  │
│  │     │   • Skip shutdown (already powered off)         │   │  │
│  │     └─────────────────────────────────────────────────┘   │  │
│  └───────────────────────────────────────────────────────────┘  │
└────────────────────────────┬────────────────────────────────────┘
                             │
                             ▼
═══════════════════════════════════════════════════════════════════
                    PHASE 2: STORAGE DISCOVERY
═══════════════════════════════════════════════════════════════════
                             │
                             ▼
┌─────────────────────────────────────────────────────────────────┐
│              STEP 2: DISCOVER STORAGE MAPPINGS                   │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │ 2.1 Get All VIOS on System                                │  │
│  │     • GetVirtualIOServersQuick()                          │  │
│  ├───────────────────────────────────────────────────────────┤  │
│  │ 2.2 For Each VIOS:                                        │  │
│  │     ┌─────────────────────────────────────────────────┐   │  │
│  │     │ A. Build Slot-to-UUID Map                       │   │  │
│  │     │    • GetVirtualSCSIServerAdapters()             │   │  │
│  │     │    • Map VirtualSlotNumber → Adapter UUID       │   │  │
│  │     ├─────────────────────────────────────────────────┤   │  │
│  │     │ B. Get Detailed VSCSI Mappings                  │   │  │
│  │     │    • GetViosSCSIMappingDetails()                │   │  │
│  │     ├─────────────────────────────────────────────────┤   │  │
│  │     │ C. Filter Mappings for Target LPAR             │   │  │
│  │     │    • Match AssociatedLparURI with target UUID   │   │  │
│  │     ├─────────────────────────────────────────────────┤   │  │
│  │     │ D. Extract Mapping Data:                        │   │  │
│  │     │    • VIOS UUID & Name                           │   │  │
│  │     │    • Volume Name (BackingDeviceName)            │   │  │
│  │     │    • VTD Name (TargetName)                      │   │  │
│  │     │    • Server Adapter UUID                        │   │  │
│  │     │    • Volume UDID                                │   │  │
│  │     └─────────────────────────────────────────────────┘   │  │
│  ├───────────────────────────────────────────────────────────┤  │
│  │ 2.3 Store All Discovered Mappings                         │  │
│  │     • Build array of mappingData structs                  │  │
│  └───────────────────────────────────────────────────────────┘  │
└────────────────────────────┬────────────────────────────────────┘
                             │
                             ▼
                    ┌────────────────┐
                    │ Check Mappings │
                    └────────┬───────┘
                             │
                ┌────────────┴────────────┐
                │                         │
                ▼                         ▼
        ┌───────────────┐         ┌──────────────┐
        │ No Mappings?  │         │ Has Mappings │
        └───────┬───────┘         └──────┬───────┘
                │                        │
                ▼                        ▼
        ┌───────────────┐         ┌──────────────────────┐
        │ Skip Storage  │         │ PROCEED WITH CLEANUP │
        │   Cleanup     │         │   (Steps 3-7)        │
        │ Go to Step 8  │         └──────┬───────────────┘
        └───────────────┘                │
                                         ▼
═══════════════════════════════════════════════════════════════════
                    PHASE 3: HMC MAPPING REMOVAL
                    (Critical Sequence: Client → VTD → Server)
═══════════════════════════════════════════════════════════════════
                                         │
                                         ▼
┌─────────────────────────────────────────────────────────────────┐
│           STEP 3: REMOVE HMC VSCSI ARCHITECTURES                 │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │ For Each Discovered Mapping:                              │  │
│  │                                                            │  │
│  │ ┌──────────────────────────────────────────────────────┐  │  │
│  │ │ 3.1 Delete Client Adapter (LPAR Side)                │  │  │
│  │ │     • RemoveVolumeLPARMapping()                      │  │  │
│  │ │     • Removes client-side VSCSI adapter              │  │  │
│  │ │     • Unmaps volume from LPAR                        │  │  │
│  │ │     • Wait 10 seconds for VIOS to process            │  │  │
│  │ └──────────────────────────────────────────────────────┘  │  │
│  │                         │                                  │  │
│  │                         ▼                                  │  │
│  │ ┌──────────────────────────────────────────────────────┐  │  │
│  │ │ 3.2 Remove Virtual Target Device (VTD) via CLI       │  │  │
│  │ │     • RunVIOSCommand("rmvdev -vtd <vtd_name>")       │  │  │
│  │ │     • Removes backing device from vhost              │  │  │
│  │ │     • Unlocks server adapter for deletion            │  │  │
│  │ │     • Wait 5 seconds for device removal              │  │  │
│  │ └──────────────────────────────────────────────────────┘  │  │
│  │                         │                                  │  │
│  │                         ▼                                  │  │
│  │ ┌──────────────────────────────────────────────────────┐  │  │
│  │ │ 3.3 Delete Server Adapter (VIOS Side) via REST       │  │  │
│  │ │     • DeleteVirtualSCSIServerAdapter()               │  │  │
│  │ │     • Removes vhost adapter from VIOS                │  │  │
│  │ │     • Completes VSCSI architecture removal           │  │  │
│  │ └──────────────────────────────────────────────────────┘  │  │
│  └───────────────────────────────────────────────────────────┘  │
└────────────────────────────┬────────────────────────────────────┘
                             │
                             ▼
═══════════════════════════════════════════════════════════════════
                    PHASE 4: VIRTUAL DISK DELETION
═══════════════════════════════════════════════════════════════════
                             │
                             ▼
┌─────────────────────────────────────────────────────────────────┐
│              STEP 4: DELETE VIRTUAL DISKS FROM VIOS              │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │ For Each Discovered Mapping:                              │  │
│  │                                                            │  │
│  │ ┌──────────────────────────────────────────────────────┐  │  │
│  │ │ 4.1 Filter Virtual Disks                             │  │  │
│  │ │     • Skip if has Volume UDID (physical volume)      │  │  │
│  │ │     • Skip if `hdisk*` or `nvme*` (physical)         │  │  │
│  │ │     • Skip if optical media (`vtopt*`)               │  │  │
│  │ │     • Skip if empty volume name                      │  │  │
│  │ ├──────────────────────────────────────────────────────┤  │  │
│  │ │ 4.2 Delete Logical Volume from VIOS                  │  │  │
│  │ │     • RunVIOSCommand("rmlv -f <lv_name>")            │  │  │
│  │ │     • Removes virtual disk (logical volume)          │  │  │
│  │ │     • Frees up space in volume group                 │  │  │
│  │ └──────────────────────────────────────────────────────┘  │  │
│  └───────────────────────────────────────────────────────────┘  │
└────────────────────────────┬────────────────────────────────────┘
                             │
                             ▼
═══════════════════════════════════════════════════════════════════
                    PHASE 5: SVC STORAGE CLEANUP
═══════════════════════════════════════════════════════════════════
                             │
                             ▼
┌─────────────────────────────────────────────────────────────────┐
│                  STEP 5: SVC SAN CLEANUP                         │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │ 4.1 Initialize SVC Client                                 │  │
│  │     • Create SVC client with TLS insecure                 │  │
│  │     • Authenticate to SVC                                 │  │
│  ├───────────────────────────────────────────────────────────┤  │
│  │ 4.2 Get All SVC Volumes                                   │  │
│  │     • LsVdisk() - Fetch all volumes once                  │  │
│  ├───────────────────────────────────────────────────────────┤  │
│  │ 4.3 For Each Discovered Mapping:                          │  │
│  │     ┌─────────────────────────────────────────────────┐   │  │
│  │     │ A. Skip if no Volume UDID (optical media)       │   │  │
│  │     ├─────────────────────────────────────────────────┤   │  │
│  │     │ B. Convert UDID to SVC Format                   │   │  │
│  │     │    • GetSvcUidFixed() - Normalize UID           │   │  │
│  │     ├─────────────────────────────────────────────────┤   │  │
│  │     │ C. Find Matching SVC Volume                     │   │  │
│  │     │    • Match VdiskUID with converted UDID         │   │  │
│  │     │    • Extract volume name                        │   │  │
│  │     ├─────────────────────────────────────────────────┤   │  │
│  │     │ D. Resolve SVC Host via WWPNs                   │   │  │
│  │     │    • GetVirtualIOServer() - Get VIOS details    │   │  │
│  │     │    • For each FibreChannelPort:                 │   │  │
│  │     │      - GetHostByWWPN() - Find SVC host          │   │  │
│  │     │      - Break on first match                     │   │  │
│  │     ├─────────────────────────────────────────────────┤   │  │
│  │     │ E. Unmap Volume from Host                       │   │  │
│  │     │    • Rmvdiskhostmap(host, volume)               │   │  │
│  │     ├─────────────────────────────────────────────────┤   │  │
│  │     │ F. Delete Volume from SVC                       │   │  │
│  │     │    • Rmvdisk(volume, force=true)                │   │  │
│  │     │    • Purge volume from SAN                      │   │  │
│  │     └─────────────────────────────────────────────────┘   │  │
│  └───────────────────────────────────────────────────────────┘  │
└────────────────────────────┬────────────────────────────────────┘
                             │
                             ▼
═══════════════════════════════════════════════════════════════════
                    PHASE 6: VIOS DEVICE WIPING
═══════════════════════════════════════════════════════════════════
                             │
                             ▼
┌─────────────────────────────────────────────────────────────────┐
│              STEP 6: WIPE PHYSICAL DISKS FROM VIOS               │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │ For Each Discovered Mapping:                              │  │
│  │                                                            │  │
│  │ ┌──────────────────────────────────────────────────────┐  │  │
│  │ │ 6.1 Filter Physical Disks                            │  │  │
│  │ │     • Skip if not `hdisk*` or `nvme*`                │  │  │
│  │ │     • Skip virtual optical media (`vopt*`)           │  │  │
│  │ ├──────────────────────────────────────────────────────┤  │  │
│  │ │ 6.2 Remove Device from VIOS ODM                      │  │  │
│  │ │     • RunVIOSCommand("rmdev -dev \<disk\> -recursive") │  │  │
│  │ │     • Removes device and all children                │  │  │
│  │ │     • Cleans up ODM entries                          │  │  │
│  │ ├──────────────────────────────────────────────────────┤  │  │
│  │ │ 6.3 Track Processed VIOS                             │  │  │
│  │ │     • Store VIOS UUID and name                       │  │  │
│  │ │     • Prepare for cfgdev operation                   │  │  │
│  │ └──────────────────────────────────────────────────────┘  │  │
│  └───────────────────────────────────────────────────────────┘  │
└────────────────────────────┬────────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────────┐
│              STEP 7: REFRESH VIOS DEVICE TREE                    │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │ For Each Processed VIOS:                                  │  │
│  │   • ConfigDevice(vios_uuid, "")                           │  │
│  │   • Runs cfgdev to refresh device tree                    │  │
│  │   • Ensures clean state                                   │  │
│  │   • Confirms no stale devices remain                      │  │
│  └───────────────────────────────────────────────────────────┘  │
└────────────────────────────┬────────────────────────────────────┘
                             │
                             ▼
═══════════════════════════════════════════════════════════════════
                    PHASE 7: LPAR DELETION
═══════════════════════════════════════════════════════════════════
                             │
                             ▼
┌─────────────────────────────────────────────────────────────────┐
│              STEP 8: DELETE LOGICAL PARTITION                    │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │ • DeleteLogicalPartition(lpar_uuid)                       │  │
│  │ • Removes LPAR definition from HMC                        │  │
│  │ • Frees up partition ID                                   │  │
│  │ • Completes partition removal                             │  │
│  └───────────────────────────────────────────────────────────┘  │
└────────────────────────────┬────────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────────┐
│                         SUCCESS - EXIT                           │
│  Partition and all associated resources removed:                 │
│    ✅ Partition powered off                                      │
│    ✅ VSCSI mappings removed (Client → VTD → Server)             │
│    ✅ SVC volumes unmapped and deleted                           │
│    ✅ VIOS devices wiped from ODM                                │
│    ✅ VIOS device trees refreshed                                │
│    ✅ LPAR deleted from HMC                                      │
└─────────────────────────────────────────────────────────────────┘

---

## Critical Sequence: VSCSI Removal Order

┌─────────────────────────────────────────────────────────────────┐
│           WHY THE ORDER MATTERS (Client → VTD → Server)          │
└────────────────────────────────────────────────────────────────┘

Step 1: Delete Client Adapter (LPAR Side)
┌──────────────────────────────────────────┐
│ • Removes LPAR's connection to storage   │
│ • LPAR can no longer access the volume   │
│ • VIOS still has vhost with backing dev  │
└────────────┬─────────────────────────────┘
             │
             ▼ Wait 10s for VIOS to process
             │
Step 2: Remove VTD (Virtual Target Device)
┌──────────────────────────────────────────┐
│ • Removes backing device from vhost      │
│ • Unlocks the server adapter             │
│ • CRITICAL: Must happen before Step 3    │
│ • Without this, Step 3 will fail with:   │
│   "device is busy" error                 │
└────────────┬─────────────────────────────┘
             │
             ▼ Wait 5s for device removal
             │
Step 3: Delete Server Adapter (vhost)
┌──────────────────────────────────────────┐
│ • Now safe to remove vhost adapter       │
│ • No backing devices attached            │
│ • Clean removal via REST API             │
│ • Completes VSCSI architecture cleanup   │
└──────────────────────────────────────────┘

⚠️  IMPORTANT: Skipping Step 2 causes Step 3 to fail!
    The error: "0931-029 device is busy"

---

## Storage Discovery Detail

┌─────────────────────────────────────────────────────────────────┐
│                  STORAGE DISCOVERY (Step 2)                      │
└────────────────────────────┬────────────────────────────────────┘
                             │
                             ▼
                ┌────────────────────────┐
                │ Get All VIOS on System │
                └────────┬───────────────┘
                         │
                         ▼
        ┌────────────────────────────────────┐
        │  For Each VIOS:                    │
        └────────┬───────────────────────────┘
                 │
                 ▼
    ┌────────────────────────────────────────────────┐
    │  Build Slot-to-UUID Mapping                    │
    │  ┌──────────────────────────────────────────┐  │
    │  │ GetVirtualSCSIServerAdapters()           │  │
    │  │ Map: VirtualSlotNumber → Adapter UUID    │  │
    │  │ Example: "6" → "51a59687-f5bd-..."       │  │
    │  └──────────────────────────────────────────┘  │
    └────────┬───────────────────────────────────────┘
             │
             ▼
    ┌────────────────────────────────────────────────┐
    │  Get Detailed VSCSI Mappings                   │
    │  ┌──────────────────────────────────────────┐  │
    │  │ GetViosSCSIMappingDetails()              │  │
    │  │ Returns array of mapping structures      │  │
    │  └──────────────────────────────────────────┘  │
    └────────┬───────────────────────────────────────┘
             │
             ▼
    ┌────────────────────────────────────────────────┐
    │  Filter for Target LPAR                        │
    │  ┌──────────────────────────────────────────┐  │
    │  │ Check AssociatedLparURI                  │  │
    │  │ Match with target LPAR UUID              │  │
    │  │ Skip mappings for other LPARs            │  │
    │  └──────────────────────────────────────────┘  │
    └────────┬───────────────────────────────────────┘
             │
             ▼
    ┌────────────────────────────────────────────────┐
    │  Extract Mapping Data                          │
    │  ┌──────────────────────────────────────────┐  │
    │  │ ViosUUID: VIOS identifier                │  │
    │  │ ViosName: VIOS partition name            │  │
    │  │ VolName: BackingDeviceName (hdisk3)      │  │
    │  │ VtdName: TargetName (vtscsi0)            │  │
    │  │ AdapterUUID: From slot mapping           │  │
    │  │ VolumeUDID: Storage.VolumeUniqueID       │  │
    │  └──────────────────────────────────────────┘  │
    └────────┬───────────────────────────────────────┘
             │
             ▼
    ┌────────────────────────────────────────────────┐
    │  Store in discoveredMappings Array             │
    │  Ready for cleanup operations                  │
    └────────────────────────────────────────────────┘

---

## SVC Cleanup Detail

┌─────────────────────────────────────────────────────────────────┐
│                    SVC CLEANUP (Step 4)                          │
└────────────────────────────┬────────────────────────────────────┘
                             │
                             ▼
                ┌────────────────────────┐
                │ Authenticate to SVC    │
                └────────┬───────────────┘
                         │
                         ▼
                ┌────────────────────────┐
                │ Get All SVC Volumes    │
                │ (LsVdisk - once)       │
                └────────┬───────────────┘
                         │
                         ▼
        ┌────────────────────────────────────┐
        │  For Each Mapping:                 │
        └────────┬───────────────────────────┘
                 │
                 ▼
    ┌────────────────────────────────────────────────┐
    │  Convert HMC UDID to SVC Format                │
    │  ┌──────────────────────────────────────────┐  │
    │  │ HMC UDID: 33213600507681080002F78...     │  │
    │  │ GetSvcUidFixed() → Normalized UID        │  │
    │  │ SVC UID: 600507681080002F78...           │  │
    │  └──────────────────────────────────────────┘  │
    └────────┬───────────────────────────────────────┘
             │
             ▼
    ┌────────────────────────────────────────────────┐
    │  Find Matching SVC Volume                      │
    │  ┌──────────────────────────────────────────┐  │
    │  │ Compare VdiskUID with converted UDID     │  │
    │  │ Extract volume name (e.g., test_volume2) │  │
    │  └──────────────────────────────────────────┘  │
    └────────┬───────────────────────────────────────┘
             │
             ▼
    ┌────────────────────────────────────────────────┐
    │  Resolve SVC Host                              │
    │  ┌──────────────────────────────────────────┐  │
    │  │ GetVirtualIOServer() - Get VIOS details  │  │
    │  │ For each FibreChannelPort:               │  │
    │  │   • Extract WWPN                         │  │
    │  │   • GetHostByWWPN() - Find SVC host      │  │
    │  │   • Break on first match                 │  │
    │  └──────────────────────────────────────────┘  │
    └────────┬───────────────────────────────────────┘
             │
             ▼
    ┌────────────────────────────────────────────────┐
    │  Unmap Volume from Host                        │
    │  ┌──────────────────────────────────────────┐  │
    │  │ Rmvdiskhostmap(host, volume)             │  │
    │  │ Removes host-to-volume mapping           │  │
    │  └──────────────────────────────────────────┘  │
    └────────┬───────────────────────────────────────┘
             │
             ▼
    ┌────────────────────────────────────────────────┐
    │  Delete Volume from SVC                        │
    │  ┌──────────────────────────────────────────┐  │
    │  │ Rmvdisk(volume, force=true)              │  │
    │  │ Permanently removes volume from SAN      │  │
    │  └──────────────────────────────────────────┘  │
    └────────────────────────────────────────────────┘

---

## Error Handling Flow

mermaid
┌─────────────────────────────────────────────────────────────────┐
│                      ERROR AT ANY STEP                           │
└────────────────────────────┬────────────────────────────────────┘
                             │
                             ▼
                    ┌────────────────┐
                    │  Log Warning   │
                    │  with Context  │
                    └────────┬───────┘
                             │
                             ▼
                    ┌────────────────┐
                    │ Continue with  │
                    │  Next Step     │
                    └────────┬───────┘
                             │
                             ▼
                    ┌────────────────┐
                    │ Best Effort    │
                    │   Cleanup      │
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
                    │ (May have some │
                    │ partial cleanup│
                    └────────────────┘

Note: The workflow uses "best effort" cleanup -
      it continues even if some steps fail,
      attempting to clean up as much as possible.

---

## Data Structures

### mappingData Structure

go
type mappingData struct {
    ViosUUID    string  // VIOS UUID
    ViosName    string  // VIOS partition name
    VolName     string  // Volume name (hdisk3, vopt_xxx)
    VtdName     string  // Virtual target device (vtscsi0)
    AdapterUUID string  // Server adapter UUID
    VolumeUDID  string  // Volume unique ID
}

### Discovery Flow

HMC VIOS Mappings
        │
        ▼
┌───────────────────┐
│ GetViosSCSI       │
│ MappingDetails()  │
└────────┬──────────┘
         │
         ▼
┌───────────────────┐
│ Filter by LPAR    │
│ URI               │
└────────┬──────────┘
         │
         ▼
┌───────────────────┐
│ Extract Data      │
│ → mappingData     │
└────────┬──────────┘
         │
         ▼
┌───────────────────┐
│ Store in Array    │
│ for Processing    │
└───────────────────┘

---

## Key Function Mapping

| Phase | Step | Main Operations | Functions Called |
| ------- | ------ | ---------------- | ------------------ |
| **1: Resolution** | Step 1 | Locate & Shutdown | `GetManagedSystemByName()`, `GetLogicalPartitionsQuickAll()`, `PowerOffPartition()`, `GetLogicalPartitionQuick()` |
| **2: Discovery** | Step 2 | Find Storage | `GetVirtualIOServersQuick()`, `GetVirtualSCSIServerAdapters()`, `GetViosSCSIMappingDetails()` |
| **3: HMC Cleanup** | Step 3 | Remove VSCSI | `RemoveVolumeLPARMapping()`, `RunVIOSCommand()` (rmvdev), `DeleteVirtualSCSIServerAdapter()` |
| **4: SVC Cleanup** | Step 4 | SAN Cleanup | `Authenticate()`, `LsVdisk()`, `GetSvcUidFixed()`, `GetVirtualIOServer()`, `GetHostByWWPN()`, `Rmvdiskhostmap()`, `Rmvdisk()` |
| **5: VIOS Wipe** | Step 5 | Remove Devices | `RunVIOSCommand()` (rmdev) |
| **6: VIOS Refresh** | Step 6 | Refresh Tree | `ConfigDevice()` |
| **7: LPAR Delete** | Step 7 | Delete Partition | `DeleteLogicalPartition()` |

---

## Configuration Parameters

### Command Line Flags

- `--hmc-ip`: HMC IP address (default: 192.0.2.1)
- `--hmc-user`: HMC username (default: REDACTED_HMC_USER<==)
- `--hmc-pass`: HMC password (required)
- `--system-name`: Managed system name (required)
- `--lpar-name`: LPAR name to delete (required)
- `--verbose`: Enable verbose logging (default: false)
- `--svc-ip`: SVC IP address (default: 192.0.2.8)
- `--svc-user`: SVC username (default: REDACTED_SVC_USER<==)
- `--svc-pass`: SVC password (required)

---

## Success Indicators

Throughout the workflow, success is indicated by:

- ✅ Emoji markers for completed steps
- ⚠️  Warning markers for non-critical errors
- Verbose logging (when enabled)
- Final success message with complete summary

---

## Important Notes

1. **Order is Critical**: The VSCSI removal sequence (Client → VTD → Server) must be followed exactly to avoid "device busy" errors.

2. **Best Effort Cleanup**: The workflow continues even if some steps fail, attempting to clean up as much as possible.

3. **Wait Times**: Strategic wait times are included after critical operations to allow the VIOS to process changes.

4. **Device Filtering**: Only physical disks (`hdisk*`, `nvme*`) are wiped from VIOS; virtual optical media is skipped.

5. **WWPN Resolution**: SVC host is dynamically resolved using VIOS WWPNs, matching the creation workflow.

6. **UID Conversion**: HMC volume UDIDs are converted to SVC format for proper matching.

7. **Deferred Logout**: HMC logout is deferred to ensure it happens even if errors occur.

8. **No Rollback**: Once deletion starts, there's no rollback mechanism. The partition and its resources are permanently removed.
