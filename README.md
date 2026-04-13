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
go get github.com/sudeeshjohn/svc-go-sdk
```

## Import

```go
import svc "github.com/sudeeshjohn/svc-go-sdk"
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

```go
if err := client.Authenticate(); err != nil {
	log.Fatalf("auth error: %v", err)
}
fmt.Println("Authenticated")
```

## Example: list system information

```go
systemInfo, err := client.Lssystem()
if err != nil {
	log.Fatalf("lssystem error: %v", err)
}
fmt.Printf("System: %+v\n", systemInfo)
```

## Example: list FC ports

```go
ports, err := client.Lsportfc()
if err != nil {
	log.Fatalf("lsportfc error: %v", err)
}
fmt.Printf("Ports: %+v\n", ports)
```

## Example: create a FlashCopy mapping

```go
copyRate := 50
err := client.Mkfcmap(svc.FlashCopyMapping{
	Name:        "fcmap-demo",
	Source:      "source-vol",
	Target:      "target-vol",
	Incremental: true,
	CopyRate:    &copyRate,
})
if err != nil {
	log.Fatalf("mkfcmap error: %v", err)
}
```

**Note:** Fields like `CopyRate`, `CleanRate`, and `GrainSize` use pointer types (`*int`) to distinguish between "not provided" (nil) and "explicitly set to zero" (0). This is important because zero is a valid value for some parameters (e.g., `CopyRate: 0` disables background copying in FlashCopy).

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
  -svc-ip svc-hostname-or-ip \
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

if err := client.Authenticate(); err != nil {
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
    client.Lssystem() // May read logger concurrently
}()
```

## Notes

- `WithTLSInsecure()` is useful for lab environments with self-signed certificates.
- `Lsfabric()` now uses the client's configured timeout and no longer mutates shared HTTP client state.
- Live integration tests currently validate authentication and `Lssystem()`.
- Example programs are available under `svc-go-sdk/examples/`.
