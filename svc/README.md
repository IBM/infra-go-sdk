# SVC API SDK

A lightweight Go SDK for interacting with IBM Spectrum Virtualize / IBM SVC REST APIs.

## Features

- Simple client construction and authentication
- Structured request and response models
- Built-in JSON request handling
- IBM error decoding helpers
- Structured logging with configurable log levels
- Examples for common SVC operations

## Installation

```bash
go get github.ibm.com/sudeeshjohn/infra-go-sdk/svc
```

## Import

```go
import svc "github.ibm.com/sudeeshjohn/infra-go-sdk/svc"
```

## Create a client

```go
client := svc.NewClient("svc-hostname-or-ip", "username", "REDACTED_HMC_PASS<==")
```

Optional client configuration:

```go
client := svc.NewClient("svc-hostname-or-ip", "username", "REDACTED_HMC_PASS<==").WithTLSInsecure()
client.WithPort(7443)
```

## Authenticate

All API methods require a `context.Context` parameter for timeout and cancellation support:

```go
ctx := context.Background()
if err := client.Authenticate(ctx); err != nil {
 log.Fatalf("auth error: %v", err)
}
fmt.Println("Authenticated")
```

## Context Usage

The SDK supports Go's context package for timeout and cancellation control:

### Basic Usage

```go
ctx := context.Background()
systemInfo, err := client.Lssystem(ctx)
if err != nil {
 log.Fatalf("lssystem error: %v", err)
}
```

### With Timeout

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

hosts, err := client.Lshost(ctx)
if err != nil {
 log.Fatalf("lshost error: %v", err)
}
```

### With Cancellation

```go
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

// Cancel on interrupt signal
go func() {
 <-sigChan
 cancel()
}()

vdisks, err := client.LsVdisk(ctx)
if err != nil {
 log.Fatalf("lsvdisk error: %v", err)
}
```

### With Deadline

```go
deadline := time.Now().Add(1 * time.Minute)
ctx, cancel := context.WithDeadline(context.Background(), deadline)
defer cancel()

ports, err := client.Lsportfc(ctx)
if err != nil {
 log.Fatalf("lsportfc error: %v", err)
}
```

## Examples

### System Information

List system information:

```go
ctx := context.Background()
systemInfo, err := client.Lssystem(ctx)
if err != nil {
 log.Fatalf("lssystem error: %v", err)
}
fmt.Printf("System: %+v\n", systemInfo)
```

### Volume Management

#### Create a Volume

Create a thin-provisioned volume with auto-expand:

```go
ctx := context.Background()
grainSize := 256
volume := svc.Volume{
 Name:       "my_volume",
 MdiskGrp:   "Pool0",
 Size:       100,
 Unit:       "gb",
 RSize:      "2%",        // Thin provisioning at 2%
 Warning:    "80%",       // Warning threshold
 AutoExpand: true,
 GrainSize:  &grainSize,
}

if err := client.Mkvdisk(ctx, volume); err != nil {
 log.Fatalf("mkvdisk error: %v", err)
}
fmt.Println("Volume created successfully")
```

#### List All Volumes

```go
ctx := context.Background()
vdisks, err := client.LsVdisk(ctx)
if err != nil {
 log.Fatalf("lsvdisk error: %v", err)
}
for _, vdisk := range vdisks {
 fmt.Printf("Volume: %s (ID: %s, Capacity: %s)\n",
  vdisk.Name, vdisk.ID, vdisk.Capacity)
}
```

#### Get Volume Details

```go
ctx := context.Background()
vdisk, err := client.LsVdiskByName(ctx, "my_volume")
if err != nil {
 log.Fatalf("lsvdisk error: %v", err)
}
fmt.Printf("Volume Details: %+v\n", vdisk)
```

#### Delete a Volume

```go
ctx := context.Background()
err := client.Rmvdisk(ctx, "my_volume", svc.VolumeRemove{
 RemoveHostMappings: true, // Remove host mappings before deletion
})
if err != nil {
 log.Fatalf("rmvdisk error: %v", err)
}
fmt.Println("Volume deleted successfully")
```

### Host Management

#### Create a Host

```go
ctx := context.Background()
hostParams := svc.Host{
 Name:     "host1",
 Fcwwpn:   []string{"21000024FF3C4D2E", "210100E08B251EE6"},
 Type:     "generic",
 Protocol: "scsi",
}

if err := client.Mkhost(ctx, hostParams); err != nil {
 log.Fatalf("mkhost error: %v", err)
}
fmt.Println("Host created successfully")
```

#### List All Hosts

```go
ctx := context.Background()
hosts, err := client.Lshost(ctx)
if err != nil {
 log.Fatalf("lshost error: %v", err)
}
for _, host := range hosts {
 fmt.Printf("Host: %s (ID: %s)\n", host.Name, host.ID)
}
```

#### Get Host Details

```go
ctx := context.Background()
host, err := client.LshostByTarget(ctx, "host1")
if err != nil {
 log.Fatalf("lshost error: %v", err)
}
fmt.Printf("Host Details: %+v\n", host)
```

#### Delete a Host

```go
ctx := context.Background()
if err := client.Rmhost(ctx, "host1", svc.HostRemove{}); err != nil {
 log.Fatalf("rmhost error: %v", err)
}
fmt.Println("Host deleted successfully")
```

### Volume-to-Host Mapping

#### Map Volume to Host

```go
ctx := context.Background()
mapping := svc.VolumeHostMap{
 Host:  "host1",
 VDisk: "my_volume",
 SCSI:  "0",  // Optional: specific SCSI LUN ID
}

if err := client.Mkvdiskhostmap(ctx, mapping); err != nil {
 log.Fatalf("mkvdiskhostmap error: %v", err)
}
fmt.Println("Volume mapped to host successfully")
```

#### Unmap Volume from Host

```go
ctx := context.Background()
if err := client.Rmvdiskhostmap(ctx, "host1", "my_volume"); err != nil {
 log.Fatalf("rmvdiskhostmap error: %v", err)
}
fmt.Println("Volume unmapped from host successfully")
```

### FlashCopy Operations

#### Create a FlashCopy Mapping

```go
ctx := context.Background()
copyRate := 50
err := client.Mkfcmap(ctx, svc.FlashCopyMapping{
 Name:        "fcmap-demo",
 Source:      "source-vol",
 Target:      "target-vol",
 Incremental: true,
 CopyRate:    &copyRate,
})
if err != nil {
 log.Fatalf("mkfcmap error: %v", err)
}
fmt.Println("FlashCopy mapping created successfully")
```

**Note:** Fields like `CopyRate`, `CleanRate`, and `GrainSize` use pointer types (`*int`) to distinguish between "not provided" (nil) and "explicitly set to zero" (0). This is important because zero is a valid value for some parameters (e.g., `CopyRate: 0` disables background copying in FlashCopy).

#### List FlashCopy Mappings

```go
ctx := context.Background()
fcmaps, err := client.Lsfcmap(ctx)
if err != nil {
 log.Fatalf("lsfcmap error: %v", err)
}
for _, fcmap := range fcmaps {
 fmt.Printf("FlashCopy: %s (Source: %s, Target: %s)\n",
  fcmap.Name, fcmap.SourceVdiskName, fcmap.TargetVdiskName)
}
```

#### Prepare and Start FlashCopy

```go
ctx := context.Background()

// Prepare the FlashCopy mapping
if err := client.Prestartfcmap(ctx, "fcmap-demo"); err != nil {
 log.Fatalf("prestartfcmap error: %v", err)
}

// Start the FlashCopy operation
if err := client.Startfcmap(ctx, "fcmap-demo"); err != nil {
 log.Fatalf("startfcmap error: %v", err)
}
fmt.Println("FlashCopy started successfully")
```

#### Delete a FlashCopy Mapping

```go
ctx := context.Background()
if err := client.Rmfcmap(ctx, "fcmap-demo", true); err != nil {
 log.Fatalf("rmfcmap error: %v", err)
}
fmt.Println("FlashCopy mapping deleted successfully")
```

### FlashCopy Consistency Groups

#### Create a Consistency Group

```go
ctx := context.Background()
err := client.Mkfcconsistgrp(ctx, svc.FlashCopyConsistencyGroup{
 Name:       "cg-demo",
 AutoDelete: true,
})
if err != nil {
 log.Fatalf("mkfcconsistgrp error: %v", err)
}
fmt.Println("Consistency group created successfully")
```

#### List Consistency Groups

```go
ctx := context.Background()
groups, err := client.Lsfcconsistgrp(ctx)
if err != nil {
 log.Fatalf("lsfcconsistgrp error: %v", err)
}
for _, group := range groups {
 fmt.Printf("Consistency Group: %s (ID: %s)\n", group.Name, group.ID)
}
```

#### Prepare and Start Consistency Group

```go
ctx := context.Background()

// Prepare all FlashCopy mappings in the group
if err := client.Prestartfcconsistgrp(ctx, "cg-demo"); err != nil {
 log.Fatalf("prestartfcconsistgrp error: %v", err)
}

// Start all FlashCopy operations in the group
if err := client.Startfcconsistgrp(ctx, "cg-demo"); err != nil {
 log.Fatalf("startfcconsistgrp error: %v", err)
}
fmt.Println("Consistency group started successfully")
```

### Fabric and Port Information

#### List FC Ports

```go
ctx := context.Background()
ports, err := client.Lsportfc(ctx)
if err != nil {
 log.Fatalf("lsportfc error: %v", err)
}
for _, port := range ports {
 fmt.Printf("Port: %s (WWPN: %s, Status: %s)\n",
  port.ID, port.WWPN, port.Status)
}
```

#### List Fabric Information

```go
ctx := context.Background()
fabrics, err := client.Lsfabric(ctx)
if err != nil {
 log.Fatalf("lsfabric error: %v", err)
}
for _, fabric := range fabrics {
 fmt.Printf("Fabric: %s (WWNN: %s)\n", fabric.Name, fabric.RemoteWWNN)
}
```

### Complete Workflow Example

Here's a complete example that creates a volume, maps it to a host, and creates a FlashCopy:

```go
package main

import (
 "context"
 "log"
 "time"

 svc "github.ibm.com/sudeeshjohn/infra-go-sdk/svc"
)

func main() {
 // Create and configure client
 client := svc.NewClient("svc-hostname-or-ip", "username", "REDACTED_HMC_PASS<==").
  WithTLSInsecure().
  WithDebug()

 // Set timeout context
 ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
 defer cancel()

 // Authenticate
 if err := client.Authenticate(ctx); err != nil {
  log.Fatalf("Authentication failed: %v", err)
 }

 // Create source volume
 grainSize := 256
 sourceVol := svc.Volume{
  Name:       "source_vol",
  MdiskGrp:   "Pool0",
  Size:       50,
  Unit:       "gb",
  RSize:      "2%",
  Warning:    "80%",
  AutoExpand: true,
  GrainSize:  &grainSize,
 }
 if err := client.Mkvdisk(ctx, sourceVol); err != nil {
  log.Fatalf("Failed to create source volume: %v", err)
 }
 log.Println("Source volume created")

 // Create target volume for FlashCopy
 targetVol := sourceVol
 targetVol.Name = "target_vol"
 if err := client.Mkvdisk(ctx, targetVol); err != nil {
  log.Fatalf("Failed to create target volume: %v", err)
 }
 log.Println("Target volume created")

 // Create host
 host := svc.Host{
  Name:     "app_server",
  Fcwwpn:   []string{"21000024FF3C4D2E", "210100E08B251EE6"},
  Type:     "generic",
  Protocol: "scsi",
 }
 if err := client.Mkhost(ctx, host); err != nil {
  log.Fatalf("Failed to create host: %v", err)
 }
 log.Println("Host created")

 // Map source volume to host
 mapping := svc.VolumeHostMap{
  Host:  "app_server",
  VDisk: "source_vol",
 }
 if err := client.Mkvdiskhostmap(ctx, mapping); err != nil {
  log.Fatalf("Failed to map volume: %v", err)
 }
 log.Println("Volume mapped to host")

 // Create FlashCopy mapping
 copyRate := 50
 fcmap := svc.FlashCopyMapping{
  Name:        "backup_fcmap",
  Source:      "source_vol",
  Target:      "target_vol",
  Incremental: false,
  CopyRate:    &copyRate,
 }
 if err := client.Mkfcmap(ctx, fcmap); err != nil {
  log.Fatalf("Failed to create FlashCopy: %v", err)
 }
 log.Println("FlashCopy mapping created")

 // Prepare and start FlashCopy
 if err := client.Prestartfcmap(ctx, "backup_fcmap"); err != nil {
  log.Fatalf("Failed to prepare FlashCopy: %v", err)
 }
 if err := client.Startfcmap(ctx, "backup_fcmap"); err != nil {
  log.Fatalf("Failed to start FlashCopy: %v", err)
 }
 log.Println("FlashCopy started successfully")
}
```

## Logging

The client uses a default warning-level logger. For visible progress in examples or applications, set an info or debug logger explicitly.

Enable debug logging:

```go
client := svc.NewClient("svc-hostname-or-ip", "username", "REDACTED_HMC_PASS<==").WithTLSInsecure()
client = client.WithDebug()
```

Enable info-level logging:

```go
logger := svc.NewLogger(log.InfoLevel, os.Stderr)
client := svc.NewClient("svc-hostname-or-ip", "username", "REDACTED_HMC_PASS<==").WithTLSInsecure()
client = client.WithLogger(logger)
```

Notes:

- default client logger level is `Warn`
- `WithDebug()` enables verbose request/response progress logs
- several SDK operations now emit `Info` logs for visible progress when the logger level allows it
- example programs such as `examples/lsfabric` configure visible output even without `-verbose`

## Running tests

Unit and package tests:

```bash
go test ./...
```

Integration tests against a real SVC are available in `sdk_test.go` and accept these flags:

```bash
go test -run 'TestSVCAuthenticationIntegration|TestSVCLssystemIntegration' -v ./... \
  -svc-ip <svc-ip> \
  -svc-user <username> \
  -svc-pass <REDACTED_HMC_PASS<==>
```

To skip slower/live integration tests:

```bash
go test -short ./...
```

## Thread Safety

The SDK client is designed to be **thread-safe for concurrent API requests** after initial setup. However, there are important considerations:

### Safe for Concurrent Use

- Making API calls (e.g., `Lssystem()`, `Mkfcmap()`, etc.) from multiple goroutines is safe
- Token refresh is automatically handled with mutex protection
- No data races occur during normal API operations

### Configuration Methods (Not Thread-Safe)

Builder methods like `WithDebug()`, `WithPort()`, `WithLogger()`, and `WithTLSInsecure()` **mutate the client in place** and should **only be called during initial client setup**, not while the client is actively being used by other goroutines.

**Safe usage pattern:**

```go
// Configure client before using it
client := svc.NewClient("svc-hostname-or-ip", "username", "REDACTED_HMC_PASS<==").
    WithTLSInsecure().
    WithDebug()

ctx := context.Background()
if err := client.Authenticate(ctx); err != nil {
    log.Fatal(err)
}

// Now safe to use from multiple goroutines
go makeAPICall1(client)
go makeAPICall2(client)
```

**Unsafe usage pattern:**

```go
// DON'T do this - causes data races!
go func() {
    client.WithDebug() // Mutates logger while other goroutines use it
}()
go func() {
    ctx := context.Background()
    client.Lssystem(ctx) // May read logger concurrently
}()
```

## Available Examples

The SDK includes working example programs for all operations. See the [`examples/`](examples/) directory:

**System & Information:**

- [`lssystem`](examples/lssystem/) - List system information
- [`lsportfc`](examples/lsportfc/) - List FC ports
- [`lsfabric`](examples/lsfabric/) - List fabric information

**Volume Management:**

- [`mkvdisk`](examples/mkvdisk/) - Create a volume
- [`lsvdisk`](examples/lsvdisk/) - List volumes
- [`rmvdisk`](examples/rmvdisk/) - Delete a volume

**Host Management:**

- [`mkhost`](examples/mkhost/) - Create a host
- [`lshost`](examples/lshost/) - List hosts

**Volume-to-Host Mapping:**

- [`mkvdiskhostmap`](examples/mkvdiskhostmap/) - Map volume to host
- [`rmvdiskhostmap`](examples/rmvdiskhostmap/) - Unmap volume from host

**FlashCopy Operations:**

- [`mkfcmap`](examples/mkfcmap/) - Create FlashCopy mapping
- [`lsfcmap`](examples/lsfcmap/) - List FlashCopy mappings
- [`prestartfcmap`](examples/prestartfcmap/) - Prepare FlashCopy
- [`startfcmap`](examples/startfcmap/) - Start FlashCopy
- [`rmfcmap`](examples/rmfcmap/) - Delete FlashCopy mapping

**FlashCopy Consistency Groups:**

- [`mkfcconsistgrp`](examples/mkfcconsistgrp/) - Create consistency group
- [`lsfcconsistgrp`](examples/lsfcconsistgrp/) - List consistency groups
- [`prestartfcconsistgrp`](examples/prestartfcconsistgrp/) - Prepare consistency group
- [`startfcconsistgrp`](examples/startfcconsistgrp/) - Start consistency group

Each example can be run with:

```bash
cd examples/<example-name>
go run main.go -svc-ip <svc-ip> -svc-user <username> -svc-pass <REDACTED_HMC_PASS<==> -verbose
```

## Notes

- `WithTLSInsecure()` is useful for lab environments with self-signed certificates.
- `Lsfabric()` now uses the client's configured timeout and no longer mutates shared HTTP client state.
- Live integration tests currently validate authentication and `Lssystem()`.
- All API methods support context for timeout and cancellation control.
- Error responses from the SVC API are automatically decoded for better error messages.
