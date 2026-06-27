# PowerHMC Go SDK

[![Go Version](https://img.shields.io/badge/Go-1.23%2B-blue)](https://golang.org/dl/)
[![License](https://img.shields.io/badge/License-Apache%202.0-green.svg)](LICENSE)

A comprehensive Go SDK for interacting with IBM PowerVM Hardware Management Console (HMC) REST API. This library provides a complete set of functions to automate and manage PowerVM infrastructure including logical partitions (LPARs), Virtual I/O Servers (VIOS), storage, and networking resources.

## 🚀 Features

### Core Capabilities

- **Authentication & Session Management**: Secure login/logout with HMC REST API
- **Managed Systems**: Query and manage PowerVM systems and their configurations
- **Logical Partitions (LPARs)**: Full lifecycle management - create, configure, power control, and delete
- **Virtual I/O Servers (VIOS)**: Complete VIOS management and configuration
- **Partition Profiles**: Create, update, and manage partition profiles
- **Job Management**: Monitor and track asynchronous HMC operations

### Storage Management

- Physical volume discovery and mapping
- Virtual disk creation and management
- Volume group operations (create, extend, reduce)
- SCSI adapter and mapping management
- Virtual optical media management
- Storage repository operations

### Network Management

- Virtual switch operations and configuration
- Client network adapter management
- Virtual Ethernet adapter configuration
- VLAN configuration and management
- SR-IOV logical port management
- Dedicated virtual NIC operations

### Advanced Features

- Partition templates for rapid deployment
- Parallel operations for improved performance
- Comprehensive error handling and logging
- Integration with IBM SAN Volume Controller (SVC)
- SSH-based CLI operations for advanced scenarios

## 📋 Requirements

- **Go**: 1.23.0 or higher
- **HMC**: IBM PowerVM Hardware Management Console with REST API enabled
- **Credentials**: Valid HMC user credentials with appropriate permissions
- **Network**: Network connectivity to HMC management interface

## 📦 Installation

```bash
go get github.com/IBM/infra-go-sdk/phmc
```

## 🎯 Quick Start

### Basic Authentication and System Query

```go
package main

import (
    "context"
    "log"
    
    hmc "github.com/IBM/infra-go-sdk/phmc"
)

func main() {
    // Create HMC client
    client := hmc.NewHmcRestClient("your-hmc-ip")
    
    // Login to HMC
    ctx := context.Background()
    if err := client.Login(ctx, "username", "password", false); err != nil {
        log.Fatalf("Failed to login: %v", err)
    }
    defer client.Logoff(ctx)
    
    // Get all managed systems
    systems, err := client.GetManagedSystemQuickAll(ctx, false)
    if err != nil {
        log.Fatalf("Failed to get systems: %v", err)
    }
    
    for _, system := range systems {
        log.Printf("System: %s (UUID: %s)", system.SystemName, system.UUID)
    }
}
```

### Creating an LPAR with Storage

```go
// Create LPAR request
req := hmc.CreateLparRequest{
    Name:             "MyLPAR",
    OsType:           "AIX/Linux",
    MinMem:           2048,
    DesiredMem:       4096,
    MaxMem:           8192,
    MinProcUnits:     0.1,
    DesiredProcUnits: 0.5,
    MaxProcUnits:     2.0,
    MinVcpus:         1,
    DesiredVcpus:     2,
    MaxVcpus:         4,
    SharingMode:      "uncapped",
    DedicatedProc:    false,
}

// Create the LPAR
lparDetails, err := client.CreateLogicalPartition(systemUUID, req, false)
if err != nil {
    log.Fatalf("Failed to create LPAR: %v", err)
}

log.Printf("LPAR created with UUID: %s", lparDetails.MetadataID)
```

## 📚 Core API Reference

### Authentication

```go
// Create client
client := hmc.NewHmcRestClient(hmcIP)

// Login
err := client.Login(ctx, username, password, verbose)

// Logout
defer client.Logoff(ctx)
```

### Managed Systems

```go
// Get all systems (quick view)
systems, err := client.GetManagedSystemQuickAll(ctx, verbose)

// Get system by name
system, uuid, err := client.GetManagedSystemByNameQuick(ctx, systemName, verbose)

// Get system by UUID
system, err := client.GetManagedSystemQuick(ctx, systemUUID, verbose)
```

### Logical Partitions (LPARs)

```go
// Create LPAR
lparDetails, err := client.CreateLogicalPartition(systemUUID, req, verbose)

// Get LPAR by name
lpar, uuid, err := client.GetLogicalPartitionByName(ctx, systemUUID, lparName, verbose)

// Get LPAR details
lpar, err := client.GetLogicalPartitionDetailed(ctx, lparUUID, verbose)

// Power on LPAR
options := &hmc.PowerOnOptions{
    ProfileUUID: profileUUID,
    Keylock:     "normal",
    OSType:      "AIX/Linux",
}
_, err := client.PowerOnPartition(ctx, lparUUID, options, verbose)

// Power off LPAR
err := client.PowerOffPartition(ctx, lparUUID, "shutdown", false, verbose)

// Delete LPAR
err := client.DeleteLogicalPartition(ctx, systemUUID, lparUUID, verbose)
```

### Virtual I/O Server (VIOS)

```go
// Create VIOS
req := hmc.CreateViosRequest{
    Name:             "MyVIOS",
    MinMem:           4096,
    DesiredMem:       8192,
    MaxMem:           16384,
    MinProcUnits:     0.1,
    DesiredProcUnits: 1.0,
    MaxProcUnits:     4.0,
    MinVcpus:         1,
    DesiredVcpus:     2,
    MaxVcpus:         8,
    SharingMode:      "uncapped",
    MaxVirtualSlots:  500,
}
viosUUID, err := client.CreateVirtualIOServer(ctx, systemUUID, req, verbose)

// Get VIOS instances
viosServers, err := client.GetVirtualIOServersQuick(ctx, systemUUID, verbose)

// Get specific VIOS
vios, err := client.GetVirtualIOServer(viosUUID, verbose)

// Delete VIOS
err := client.DeleteVirtualIOServer(ctx, systemUUID, viosUUID, verbose)

// Configure device on VIOS
err := client.ConfigDevice(ctx, viosUUID, deviceName, verbose)

// Get SCSI mappings
mappings, err := client.GetViosSCSIMappings(viosUUID, verbose)
```

### Storage Management API

```go
// Create virtual disk
err := client.CreateVirtualDisk(ctx, systemName, viosUUID, viosName, 
    volumeGroup, diskName, diskSizeMB, verbose)

// Create physical volume mapping
mappingUUID, err := client.CreatePhysicalVolumeMaps(systemUUID, viosUUID, 
    lparUUID, []string{diskName}, verbose)

// Create virtual disk mapping
mappingUUID, err := client.CreateVirtualDiskMaps(systemUUID, viosUUID, 
    lparUUID, []string{diskName}, verbose)

// Get free physical volumes
volumes, err := client.GetFreePhyVolume(viosUUID, verbose)

// Get volume groups
vgs, err := client.GetVolumeGroups(ctx, viosUUID, verbose)

// Extend volume group
err := client.ExtendVolumeGroup(ctx, systemName, viosUUID, viosName, 
    vgName, []string{"hdisk5", "hdisk6"}, verbose)

// Reduce volume group
err := client.ReduceVolumeGroup(ctx, systemName, viosUUID, viosName, 
    vgName, []string{"hdisk3"}, verbose)
```

### Network Management API

```go
// Get virtual switches
vswitches, err := client.GetVirtualSwitchQuickAll(ctx, systemUUID, verbose)

// Create client network adapter
adapter, err := client.CreateClientNetworkAdapter(ctx, systemUUID, lparUUID, 
    vswitchUUID, vlanID, verbose)

// Delete client network adapter
err := client.DeleteClientNetworkAdapter(ctx, lparUUID, adapterUUID, verbose)
```

### Partition Profiles

```go
// Get partition profile
profile, err := client.GetPartitionProfile(ctx, lparUUID, profileName, verbose)

// Save current configuration to profile
err := client.SaveCurrentLparConfig(ctx, lparUUID, profileName, overwrite, verbose)

// Update partition profile
err := client.UpdatePartitionProfile(ctx, profileUUID, updatedProfile, verbose)
```

## 📖 Examples

The `examples/` directory contains 50+ comprehensive examples demonstrating various operations. All examples now require command-line flags for credentials and configuration (no hardcoded values).

### LPAR Management Examples

| Example | Description |
| ------- | ----------- |
| [`createlpar/`](examples/createlpar/) | Create LPAR with flexible storage options (physical/virtual/optical) |
| [`createlparphyvol/`](examples/createlparphyvol/) | Create LPAR with physical SAN storage via SVC |
| [`createlparvirvol/`](examples/createlparvirvol/) | Create LPAR with native virtual storage |
| [`createpartviatemplate/`](examples/createpartviatemplate/) | Create partition from template |
| [`deletepartition/`](examples/deletepartition/) | Delete logical partition |
| [`poweronpartition/`](examples/poweronpartition/) | Power on partition |
| [`poweroffpartition/`](examples/poweroffpartition/) | Power off partition |
| [`getlogicalpartition/`](examples/getlogicalpartition/) | Get detailed LPAR information |
| [`getlogicalpartitions/`](examples/getlogicalpartitions/) | List all LPARs on a system |
| [`searchlpars/`](examples/searchlpars/) | Search for LPARs by criteria |

### VIOS Management Examples

| Example | Description |
| ------- | ----------- |
| [`createvios/`](examples/createvios/) | Create Virtual I/O Server |
| [`deletevios/`](examples/deletevios/) | Delete Virtual I/O Server |
| [`getvirtualioserver/`](examples/getvirtualioserver/) | Get VIOS details |
| [`getvirtualioservers/`](examples/getvirtualioservers/) | List all VIOS instances |
| [`getviosscsimappings/`](examples/getviosscsimappings/) | Get SCSI mappings |

### Storage Examples

| Example | Description |
| --------- | ------------- |
| [`createvirtualdiskmapcli/`](examples/createvirtualdiskmapcli/) | Create virtual disk mapping |
| [`createvolumegroup/`](examples/createvolumegroup/) | Create volume group |
| [`extendvg/`](examples/extendvg/) | Extend volume group with physical volumes |
| [`vgreduce/`](examples/vgreduce/) | Reduce volume group by removing physical volumes |
| [`virtualdisk/`](examples/virtualdisk/) | Virtual disk operations |
| [`virtualdiskmaps/`](examples/virtualdiskmaps/) | Virtual disk mapping operations |
| [`physicalvolumemaps/`](examples/physicalvolumemaps/) | Physical volume mapping operations |
| [`getfreephysicalvolume/`](examples/getfreephysicalvolume/) | List free physical volumes |
| [`getvolumegroups/`](examples/getvolumegroups/) | List volume groups |

### Network Examples

| Example | Description |
| ------- | ----------- |
| [`clientnetadapter/`](examples/clientnetadapter/) | Client network adapter operations |
| [`getvirtualswitch/`](examples/getvirtualswitch/) | Get virtual switch details |
| [`getdedicatedvirtualnic/`](examples/getdedicatedvirtualnic/) | Get dedicated virtual NIC info |
| [`getsriovadapters/`](examples/getsriovadapters/) | Get SR-IOV adapter information |
| [`sriovlogicalport/`](examples/sriovlogicalport/) | SR-IOV logical port operations |

### Profile Management Examples

| Example | Description |
| ------- | ----------- |
| [`changedefaultprofile/`](examples/changedefaultprofile/) | Change default partition profile |
| [`getpartitionprofile/`](examples/getpartitionprofile/) | Get partition profile details |
| [`savecurrentlparconfig/`](examples/savecurrentlparconfig/) | Save current LPAR configuration |
| [`updatepartitionprofile/`](examples/updatepartitionprofile/) | Update partition profile |

### Query Examples

| Example | Description |
| ------- | ----------- |
| [`getallsystems/`](examples/getallsystems/) | List all managed systems |
| [`getalllogicalpartitions/`](examples/getalllogicalpartitions/) | List all LPARs across systems |
| [`getallhmcpartitions/`](examples/getallhmcpartitions/) | List all HMC-managed partitions |
| [`getsystemdeails/`](examples/getsystemdeails/) | Get detailed system information |

### Running Examples

All examples now require command-line flags (no hardcoded credentials):

```bash
cd examples/createlpar
go run main.go \
  -hmc-ip <your-hmc-ip> \
  -hmc-user <username> \
  -hmc-pass <password> \
  -system-name <system-name> \
  -lpar-name <lpar-name>
```

For detailed usage of each example, run with `-h` flag:

```bash
go run main.go -h
```

## 🔧 Advanced Usage

### Parallel Operations

The SDK supports parallel operations for improved performance:

```go
// Example: Parallel authentication to HMC and SVC
var wg sync.WaitGroup
var hmcErr, svcErr error

wg.Add(2)

// Authenticate to HMC
go func() {
    defer wg.Done()
    hmcErr = hmcClient.Login(ctx, username, password, verbose)
}()

// Authenticate to SVC
go func() {
    defer wg.Done()
    svcErr = svcClient.Authenticate()
}()

wg.Wait()
```

### Integration with SVC Storage

This SDK integrates seamlessly with the SVC SDK (located in `svc/` directory of this monorepo) for IBM SAN Volume Controller operations:

```go
import (
    hmc "github.com/IBM/infra-go-sdk/phmc"
    svc "github.com/IBM/infra-go-sdk/svc"
)

// Complete workflow:
// 1. Authenticate with HMC and SVC (parallel)
// 2. Create LPAR on PowerVM
// 3. Provision storage on SVC with FlashCopy
// 4. Map storage to LPAR via VIOS
// 5. Power on LPAR
```

See [`examples/createlparphyvol/`](examples/createlparphyvol/) for a complete end-to-end implementation.

### Error Handling

All functions return errors that should be checked:

```go
lpar, uuid, err := client.GetLogicalPartitionByName(ctx, systemUUID, lparName, verbose)
if err != nil {
    log.Fatalf("Failed to get LPAR: %v", err)
}

// Check for specific error conditions
if strings.Contains(err.Error(), "not found") {
    // Handle not found case
}
```

## 🏗️ Architecture

### Main Components

- **`HmcRestClient`**: Core client for HMC REST API operations
- **`ManagedSystemQuick`**: Managed system information and metadata
- **`LogicalPartitionDetailed`**: Comprehensive LPAR configuration and state
- **`VirtualIOServerQuick`**: VIOS information and configuration
- **`PartitionTemplate`**: Template-based partition deployment

### Key Types

```go
type CreateLparRequest struct {
    Name             string
    OsType           string
    MinMem           int
    DesiredMem       int
    MaxMem           int
    MinProcUnits     float64
    DesiredProcUnits float64
    MaxProcUnits     float64
    MinVcpus         int
    DesiredVcpus     int
    MaxVcpus         int
    SharingMode      string
    DedicatedProc    bool
}

type PowerOnOptions struct {
    ProfileUUID string
    Keylock     string
    OSType      string
}
```

## 🔒 Security Best Practices

1. **Never hardcode credentials** - Use environment variables or secure vaults
2. **Use HTTPS** - Ensure HMC REST API is configured with TLS
3. **Rotate credentials** - Regularly update HMC passwords
4. **Limit permissions** - Use HMC users with minimum required privileges
5. **Audit logging** - Enable HMC audit logs for compliance

## 🤝 Contributing

Contributions are welcome! Please follow these guidelines:

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

### Development Setup

```bash
# Clone the repository
git clone https://github.com/IBM/infra-go-sdk.git
cd infra-go-sdk/phmc

# Install dependencies
go mod download

# Run tests
go test ./...

# Run examples
cd examples/getallsystems
go run main.go -hmc-ip <ip> -hmc-user <user> -hmc-pass <pass>
```

## 📊 Supported Versions

To successfully deploy and use this SDK, various components of the IBM Power software stack must be at the minimum levels listed below:

| Component | P10 | P11 |
| :---------- | :---: | :---: |
| **Hardware Management Console (HMC)** | V10R1 (1061) | V11R1 (1110) |
| **Partition Firmware (PFW)** | 1050.50, 1060.50 | 1110.00 |
| **Virtual I/O Server (VIOS)** | 4.1.1.0/4.1.2.0 | 4.1.1.0/4.1.2.0 |

> **Note**: These are minimum recommended versions. Higher versions within the same major release are generally compatible.

## 📄 License

This project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.

## 📞 Support

For issues, questions, and contributions:

- **Issues**: [GitHub Issues](https://github.com/IBM/infra-go-sdk/issues)
- **Examples**: Check the `examples/` directory for usage patterns
- **Documentation**: See inline code documentation

## 🙏 Acknowledgments

This SDK provides Go bindings for the IBM PowerVM Hardware Management Console REST API, enabling infrastructure-as-code and automation of PowerVM environments.

---

## Made with ❤️ for the PowerVM community
