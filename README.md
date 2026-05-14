# IBM Infrastructure Go SDK

[![Go Version](https://img.shields.io/badge/Go-1.23%2B-blue)](https://golang.org/dl/)
[![License](https://img.shields.io/badge/License-Apache%202.0-green.svg)](LICENSE)

A comprehensive collection of Go SDKs for managing IBM Power Systems infrastructure and storage solutions. This repository provides production-ready libraries for automating PowerVM environments and IBM Spectrum Virtualize storage systems.

## 📦 Available SDKs

### [PowerHMC SDK (`phmc/`)](phmc/)

A complete Go SDK for IBM PowerVM Hardware Management Console (HMC) REST API automation.

**Key Features:**

- 🖥️ **Managed Systems**: Query and manage PowerVM systems
- 💻 **LPAR Management**: Full lifecycle management of logical partitions
- 🔧 **VIOS Operations**: Virtual I/O Server configuration and management
- 💾 **Storage Management**: Physical volumes, virtual disks, volume groups, SCSI mappings
- 🌐 **Network Management**: Virtual switches, adapters, VLANs, SR-IOV
- 📋 **Partition Profiles**: Create and manage partition configurations
- ⚡ **Advanced Features**: Templates, parallel operations, job tracking

**Quick Start:**

```go
import hmc "github.ibm.com/sudeeshjohn/infra-go-sdk/phmc"

client := hmc.NewHmcRestClient("hmc-ip")
ctx := context.Background()
client.Login(ctx, "username", "password", false)
defer client.Logoff(ctx)

systems, _ := client.GetManagedSystemQuickAll(ctx, false)
```

[📖 Full Documentation](phmc/README.md)

---

### [SVC SDK (`svc/`)](svc/)

A lightweight Go SDK for IBM Spectrum Virtualize / IBM SAN Volume Controller (SVC) REST API.

**Key Features:**

- 🔐 **Authentication**: Simple client construction with secure authentication
- 💾 **Volume Management**: Create, list, map, and delete virtual disks
- 🖥️ **Host Management**: Host configuration and mapping operations
- 🔄 **FlashCopy**: Consistency groups and FlashCopy mapping management
- 📊 **System Information**: Query system, fabric, and port details
- ⏱️ **Context Support**: Built-in timeout and cancellation control
- 📝 **Structured Logging**: Configurable log levels for debugging

**Quick Start:**

```go
import svc "github.ibm.com/sudeeshjohn/infra-go-sdk/svc"

client := svc.NewClient("svc-hostname", "username", "password")
ctx := context.Background()
client.Authenticate(ctx)

systemInfo, _ := client.Lssystem(ctx)
```

[📖 Full Documentation](svc/README.md)

---

## 🚀 Installation

Install both SDKs:

```bash
go get github.ibm.com/sudeeshjohn/infra-go-sdk/phmc
go get github.ibm.com/sudeeshjohn/infra-go-sdk/svc
```

Or install individually:

```bash
# PowerHMC SDK only
go get github.ibm.com/sudeeshjohn/infra-go-sdk/phmc

# SVC SDK only
go get github.ibm.com/sudeeshjohn/infra-go-sdk/svc
```

## 📋 Requirements

### PowerHMC SDK

- Go 1.23.0 or higher
- IBM PowerVM Hardware Management Console with REST API enabled
- Valid HMC credentials with appropriate permissions
- Network connectivity to HMC management interface

### SVC SDK

- Go 1.23.0 or higher
- IBM Spectrum Virtualize or IBM SVC system
- Valid SVC credentials
- Network connectivity to SVC management interface

## 🎯 Use Cases

### PowerHMC SDK Use Cases

- Automate LPAR provisioning and lifecycle management
- Manage Virtual I/O Server configurations
- Configure storage and network resources
- Implement infrastructure-as-code for PowerVM environments
- Build custom orchestration and automation tools
- Integrate with CI/CD pipelines for Power Systems

### SVC SDK Use Cases

- Automate storage provisioning workflows
- Manage volume and host mappings
- Implement FlashCopy operations for backup/recovery
- Monitor storage system health and capacity
- Build custom storage management tools
- Integrate with cloud orchestration platforms

## 📚 Examples

Both SDKs include comprehensive example programs demonstrating common operations:

- **PowerHMC**: 70+ examples in [`phmc/examples/`](phmc/examples/)
- **SVC**: 15+ examples in [`svc/examples/`](svc/examples/)

## 🤝 Contributing

Contributions are welcome! Please feel free to submit issues, feature requests, or pull requests.

## 📄 License

This project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.

## 🔗 Related Resources

- [IBM PowerVM Documentation](https://www.ibm.com/docs/en/powervm)
- [IBM Spectrum Virtualize Documentation](https://www.ibm.com/docs/en/spectrum-virtualize)
- [IBM Power Systems](https://www.ibm.com/power)

## 📞 Support

For issues, questions, or contributions, please use the GitHub issue tracker.

---

**Note**: These SDKs are designed for production use but should be thoroughly tested in your environment before deployment to critical systems.
