# VIOS SCSI Mapping Structure Analysis

## Overview
This document compares the actual XML structure from the HMC REST API response with the Go struct definitions in `types.go` for VirtualSCSIMapping.

## XML Structure Analysis

### VirtualSCSIMapping Element Structure
```xml
<VirtualSCSIMapping schemaVersion="V1_0">
    <Metadata>
        <Atom/>
    </Metadata>
    <AssociatedLogicalPartition kb="CUR" kxe="false" href="..." rel="related"/>
    <ClientAdapter kb="CUR" kxe="false" schemaVersion="V1_0">
        <!-- Client adapter details -->
    </ClientAdapter>
    <ServerAdapter kb="CUR" kxe="false" schemaVersion="V1_0">
        <!-- Server adapter details -->
    </ServerAdapter>
    <Storage kb="CUR" kxe="false">
        <!-- Either PhysicalVolume or VirtualDisk -->
    </Storage>
    <TargetDevice kxe="false" kb="CUR">
        <!-- Either PhysicalVolumeVirtualTargetDevice or LogicalVolumeVirtualTargetDevice -->
    </TargetDevice>
</VirtualSCSIMapping>
```

### ClientAdapter Structure (Lines 378-392)
```xml
<ClientAdapter kb="CUR" kxe="false" schemaVersion="V1_0">
    <Metadata><Atom/></Metadata>
    <AdapterType>Client</AdapterType>
    <DynamicReconfigurationConnectorName>U9009.42G.1342500-V5-C2</DynamicReconfigurationConnectorName>
    <LocationCode>U9009.42G.1342500-V5-C2</LocationCode>
    <LocalPartitionID>5</LocalPartitionID>
    <RequiredAdapter>false</RequiredAdapter>
    <VariedOn>true</VariedOn>
    <VirtualSlotNumber>2</VirtualSlotNumber>
    <RemoteLogicalPartitionID>1</RemoteLogicalPartitionID>
    <RemoteSlotNumber>10</RemoteSlotNumber>
    <ServerLocationCode>U9009.42G.1342500-V1-C10</ServerLocationCode>
</ClientAdapter>
```

### ServerAdapter Structure (Lines 393-409)
```xml
<ServerAdapter kb="CUR" kxe="false" schemaVersion="V1_0">
    <Metadata><Atom/></Metadata>
    <AdapterType>Server</AdapterType>
    <DynamicReconfigurationConnectorName>U9009.42G.1342500-V1-C10</DynamicReconfigurationConnectorName>
    <LocationCode>U9009.42G.1342500-V1-C10</LocationCode>
    <LocalPartitionID>1</LocalPartitionID>
    <RequiredAdapter>false</RequiredAdapter>
    <VariedOn>true</VariedOn>
    <VirtualSlotNumber>10</VirtualSlotNumber>
    <AdapterName>vhost3</AdapterName>
    <RemoteLogicalPartitionID>5</RemoteLogicalPartitionID>
    <RemoteSlotNumber>2</RemoteSlotNumber>
    <ServerLocationCode>U9009.42G.1342500-V5-C2</ServerLocationCode>
    <UniqueDeviceID>1eU9009.42G.1342500-V1-C10</UniqueDeviceID>
</ServerAdapter>
```

**Note**: ServerAdapter has additional fields:
- `AdapterName` (e.g., "vhost3")
- `BackingDeviceName` (e.g., "hdisk3" - optional, only when storage is mapped)
- `UniqueDeviceID`

### Storage Structure - PhysicalVolume (Lines 487-508)
```xml
<Storage kb="CUR" kxe="false">
    <PhysicalVolume schemaVersion="V1_0">
        <Metadata><Atom/></Metadata>
        <Description>MPIO IBM 2076 FC Disk</Description>
        <LocationCode>U78D2.001.WZS0B89-P1-C6-T1-W50050768102505EF-L1000000000000</LocationCode>
        <PersistentReserveKeyValue>none</PersistentReserveKeyValue>
        <ReservePolicy>NoReserve</ReservePolicy>
        <ReservePolicyAlgorithm>Load_Balance</ReservePolicyAlgorithm>
        <UniqueDeviceID>01M0lCTTIxNDVBRjg2MDA1MDc2ODEwODAwMDJGNzgwMDAwMDAwMDAxMEE0RA==</UniqueDeviceID>
        <AvailableForUsage>false</AvailableForUsage>
        <VolumeCapacity>122880</VolumeCapacity>
        <VolumeName>hdisk3</VolumeName>
        <VolumeState>active</VolumeState>
        <VolumeUniqueID>33213600507681080002F7800000000010A4D04214503IBMfcp</VolumeUniqueID>
        <IsFibreChannelBacked>false</IsFibreChannelBacked>
        <IsISCSIBacked>false</IsISCSIBacked>
        <StorageLabel>aGVscGVybm9kZV9ib290Xzg2NzU=</StorageLabel>
        <DescriptorPage83>NjAwNTA3NjgxMDgwMDAyRjc4MDAwMDAwMDAwMTBBNEQ=</DescriptorPage83>
    </PhysicalVolume>
</Storage>
```

### Storage Structure - VirtualDisk (Lines 629-639)
```xml
<Storage kb="CUR" kxe="false">
    <VirtualDisk schemaVersion="V1_0">
        <Metadata><Atom/></Metadata>
        <DiskCapacity>120</DiskCapacity>
        <DiskLabel>None</DiskLabel>
        <DiskName>snomas_b2304</DiskName>
        <UniqueDeviceID>0300c4250000004b000000019cf6991bb0.1</UniqueDeviceID>
    </VirtualDisk>
</Storage>
```

### TargetDevice Structure - PhysicalVolumeVirtualTargetDevice (Lines 509-518)
```xml
<TargetDevice kxe="false" kb="CUR">
    <PhysicalVolumeVirtualTargetDevice schemaVersion="V1_0">
        <Metadata><Atom/></Metadata>
        <LogicalUnitAddress>0x8100000000000000</LogicalUnitAddress>
        <TargetName>vtscsi0</TargetName>
        <UniqueDeviceID>0864cd3319fd42b94d</UniqueDeviceID>
    </PhysicalVolumeVirtualTargetDevice>
</TargetDevice>
```

### TargetDevice Structure - LogicalVolumeVirtualTargetDevice (Lines 640-649)
```xml
<TargetDevice kxe="false" kb="CUR">
    <LogicalVolumeVirtualTargetDevice schemaVersion="V1_0">
        <Metadata><Atom/></Metadata>
        <LogicalUnitAddress>0x8100000000000000</LogicalUnitAddress>
        <TargetName>vtscsi2</TargetName>
        <UniqueDeviceID>09b0907dcddf5907a0</UniqueDeviceID>
    </LogicalVolumeVirtualTargetDevice>
</TargetDevice>
```

## Current Go Struct Definitions (types.go)

### ViosSCSIMappingDetails (Lines 837-844)
```go
type ViosSCSIMappingDetails struct {
	AssociatedLparURI string
	ClientAdapter     VSCSIClientAdapter
	ServerAdapter     VSCSIServerAdapter
	Storage           VSCSIStorage
	TargetDevice      VSCSITargetDevice
}
```

### VSCSIClientAdapter (Lines 846-858)
```go
type VSCSIClientAdapter struct {
	AdapterType                         string
	DynamicReconfigurationConnectorName string
	LocationCode                        string
	LocalPartitionID                    string
	RequiredAdapter                     string
	VariedOn                            string
	VirtualSlotNumber                   string
	RemoteLogicalPartitionID            string
	RemoteSlotNumber                    string
	ServerLocationCode                  string
}
```

### VSCSIServerAdapter (Lines 860-875)
```go
type VSCSIServerAdapter struct {
	AdapterType                         string
	DynamicReconfigurationConnectorName string
	LocationCode                        string
	LocalPartitionID                    string
	RequiredAdapter                     string
	VariedOn                            string
	VirtualSlotNumber                   string
	AdapterName                         string // e.g., "vhost3"
	BackingDeviceName                   string // e.g., "hdisk3" or "vopt_..."
	RemoteLogicalPartitionID            string
	RemoteSlotNumber                    string
	ServerLocationCode                  string
	UniqueDeviceID                      string
}
```

### VSCSIStorage (Lines 877-903)
```go
type VSCSIStorage struct {
	StorageType               string // "PhysicalVolume" or "VirtualOpticalMedia"
	
	// Virtual Optical Media Fields
	MediaName                 string
	MediaUDID                 string
	MountType                 string
	Size                      string
	
	// Physical Volume Fields
	Description               string
	LocationCode              string
	PersistentReserveKeyValue string
	ReservePolicy             string
	ReservePolicyAlgorithm    string
	UniqueDeviceID            string
	AvailableForUsage         string
	VolumeCapacity            string
	VolumeName                string
	VolumeState               string
	VolumeUniqueID            string
	IsFibreChannelBacked      string
	IsISCSIBacked             string
	StorageLabel              string
	DescriptorPage83          string
}
```

### VSCSITargetDevice (Lines 905-911)
```go
type VSCSITargetDevice struct {
	DeviceType         string // "VirtualOpticalTargetDevice" or "PhysicalVolumeVirtualTargetDevice"
	LogicalUnitAddress string
	TargetName         string // e.g., "vtscsi0" or "vtopt1"
	UniqueDeviceID     string
}
```

## Issues Found

### ❌ CRITICAL: Missing XML Tags
**All struct fields are missing XML tags!** The structs cannot be automatically unmarshaled from XML responses.

### Required Changes

1. **Add XML tags to all structs**
2. **Add XMLName fields for proper element identification**
3. **Handle polymorphic Storage and TargetDevice elements**

## Recommended Structure with XML Tags

The structs need XML tags to match the actual XML element names from the REST API response.
