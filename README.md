# PowerHMC Go SDK

A Go SDK for interacting with IBM PowerVM Hardware Management Console (HMC) REST API. This library provides a comprehensive set of functions to manage PowerVM logical partitions (LPARs), Virtual I/O Servers (VIOS), storage, and networking resources.

## Features

- **Authentication & Session Management**: Secure login/logout with HMC
- **Managed Systems**: Query and manage PowerVM systems
- **Logical Partitions (LPARs)**: Create, configure, power on/off, and delete partitions
- **Virtual I/O Servers (VIOS)**: Manage VIOS instances and configurations
- **Storage Management**:
  - Physical volume mapping
  - Virtual disk creation and management
  - Volume group operations
  - SCSI mappings
- **Network Management**:
  - Virtual switch operations
  - Client network adapter management
  - VLAN configuration
- **Partition Templates**: Create and deploy partitions from templates
- **Job Management**: Monitor and track asynchronous HMC operations

## Installation

```bash
go get github.com/sudeeshjohn/powerhmc-go
```

## Requirements

- Go 1.23.0 or higher
- Access to IBM PowerVM HMC with REST API enabled
- Valid HMC credentials

## Quick Start

```go
package main

import (
    "log"
    hmc "github.com/sudeeshjohn/powerhmc-go"
)

func main() {
    // Create HMC client
    client := hmc.NewHmcRestClient("192.0.2.1", "REDACTED_HMC_USER<==", "password", true)
    
    // Login to HMC
    if err := client.Login(); err != nil {
        log.Fatalf("Failed to login: %v", err)
    }
    defer client.Logout()
    
    // Get all managed systems
    systems, err := client.GetManagedSystemQuickAll(true)
    if err != nil {
        log.Fatalf("Failed to get systems: %v", err)
    }
    
    for _, system := range systems {
        log.Printf("System: %s (UUID: %s)", system.SystemName, system.UUID)
    }
}
```

## Core Components

### Authentication

```go
client := hmc.NewHmcRestClient(hmcIP, username, password, verbose)
err := client.Login()
defer client.Logout()
```

### Managed Systems

```go
// Get all systems (quick view)
systems, err := client.GetManagedSystemQuickAll(verbose)

// Get specific system by name
system, err := client.GetManagedSystemByName(systemName, verbose)

// Get system by UUID
system, err := client.GetManagedSystemQuick(systemUUID, verbose)
```

### Logical Partitions

```go
// Create LPAR
lparUUID, err := client.CreateLogicalPartition(systemUUID, lparName, osType, 
    minCPU, desiredCPU, maxCPU, minMem, desiredMem, maxMem, 
    minVCPU, desiredVCPU, maxVCPU, sharingMode, verbose)

// Get LPAR details
lpar, err := client.GetLogicalPartitionByName(systemUUID, lparName, verbose)

// Power operations
err := client.PowerOnPartition(lparUUID, profileUUID, keylock, iIPLsource, osType, verbose)
err := client.PowerOffPartition(lparUUID, operation, immediate, verbose)

// Delete LPAR
err := client.DeleteLogicalPartition(lparUUID, verbose)
```

### Virtual I/O Server (VIOS)

```go
// Get VIOS instances
viosServers, err := client.GetVirtualIOServersQuick(systemUUID, verbose)

// Get specific VIOS
vios, err := client.GetVirtualIOServer(viosUUID, verbose)

// Configure device on VIOS
err := client.ConfigDevice(viosID, deviceName, verbose)

// Get SCSI mappings
mappings, err := client.GetViosSCSIMappings(viosUUID, verbose)
```

### Storage Management

```go
// Create virtual disk
diskUUID, err := client.CreateVirtualDisk(viosUUID, diskName, diskSize, volumeGroup, verbose)

// Create physical volume mapping
err := client.CreatePhysicalVolumeMap(viosUUID, lparUUID, physicalVolume, verbose)

// Get free physical volumes
volumes, err := client.GetFreePhyVolume(viosUUID, verbose)

// Create volume group
vgUUID, err := client.CreateVolumeGroup(viosUUID, vgName, physicalVolumes, verbose)
```

### Network Management

```go
// Get virtual switches
vswitches, err := client.GetVirtualSwitchQuickAll(systemUUID, verbose)

// Create client network adapter
adapterUUID, err := client.CreateClientNetworkAdapter(lparUUID, vswitchUUID, vlanID, verbose)

// Delete client network adapter
err := client.DeleteClientNetworkAdapter(lparUUID, adapterUUID, verbose)
```

### Partition Templates

```go
// Get partition templates
templates, err := client.GetPartitionTemplates(verbose)

// Create partition from template
lparUUID, err := client.CreatePartitionViaTemplate(systemUUID, templateName, 
    newLparName, verbose)
```

## Examples

The `examples/` directory contains comprehensive examples for various operations:

### LPAR Management

- [`createlparphyvol/`](examples/createlparphyvol/) - Create LPAR with physical SAN storage
- [`createlparvirvol/`](examples/createlparvirvol/) - Create LPAR with virtual storage
- [`createpartviatemplate/`](examples/createpartviatemplate/) - Create partition from template
- [`deletepartition/`](examples/deletepartition/) - Delete logical partition
- [`poweronartition/`](examples/poweronartition/) - Power on partition
- [`poweroffartition/`](examples/poweroffartition/) - Power off partition

### Storage Operations

- [`createvirtualdisk/`](examples/createvirtualdisk/) - Create virtual disk
- [`createphyvolmap/`](examples/createphyvolmap/) - Map physical volume to LPAR
- [`createvirtualdiskmap/`](examples/createvirtualdiskmap/) - Map virtual disk to LPAR
- [`createvolumegroup/`](examples/createvolumegroup/) - Create volume group
- [`deletevirtualdisk/`](examples/deletevirtualdisk/) - Delete virtual disk
- [`extendvirtualdisk/`](examples/extendvirtualdisk/) - Extend virtual disk size

### Network Operations

- [`createclientnetadapter/`](examples/createclientnetadapter/) - Create network adapter
- [`deleteclientnetadapter/`](examples/deleteclientnetadapter/) - Delete network adapter
- [`getvirtualswitch/`](examples/getvirtualswitch/) - Get virtual switch details

### Query Operations

- [`getallsystems/`](examples/getallsystems/) - List all managed systems
- [`getalllogicalpartitions/`](examples/getalllogicalpartitions/) - List all LPARs
- [`getvirtualioservers/`](examples/getvirtualioservers/) - List all VIOS instances
- [`getviosscsimapping/`](examples/getviosscsimapping/) - Get SCSI mappings

### Running Examples

```bash
cd examples/createlparphyvol
go run main.go \
  -hmc-ip 192.0.2.1 \
  -hmc-user REDACTED_HMC_USER<== \
  -hmc-pass password \
  -system-name MySystem \
  -lpar-name MyLPAR
```

## API Structure

### Main Types

- **`HmcRestClient`**: Main client for HMC operations
- **`ManagedSystemQuick`**: Managed system information
- **`LogicalPartitionQuick`**: LPAR details
- **`VirtualIOServerQuick`**: VIOS information
- **`PartitionTemplate`**: Template for partition creation

### Error Handling

All functions return errors that should be checked:

```go
lpar, err := client.GetLogicalPartitionByName(systemUUID, lparName, verbose)
if err != nil {
    log.Fatalf("Failed to get LPAR: %v", err)
}
```

## Integration with SVC Storage

This SDK integrates with [`svc-go-sdk`](../svc-go-sdk/) for IBM SAN Volume Controller operations, enabling end-to-end storage provisioning workflows.

Example workflow:

1. Authenticate with HMC and SVC (parallel)
2. Create LPAR on PowerVM
3. Provision storage on SVC
4. Map storage to LPAR via VIOS
5. Power on LPAR

See [`examples/createlparphyvol/`](examples/createlparphyvol/) for a complete implementation.

## Logging

Enable verbose logging by setting the `verbose` parameter to `true`:

```go
client := hmc.NewHmcRestClient(hmcIP, username, password, true)
```

## License

Apache License 2.0 - See [LICENSE](LICENSE) file for details.

## Contributing

Contributions are welcome! Please feel free to submit issues and pull requests.

## Related Projects

- [svc-go-sdk](../svc-go-sdk/) - IBM SAN Volume Controller Go SDK

## Support

For issues and questions:

- Open an issue on GitHub
- Check the examples directory for usage patterns

## Acknowledgments

This SDK provides Go bindings for the IBM PowerVM HMC REST API, enabling automation of PowerVM infrastructure management.
