# SVC API SDK

A lightweight and idiomatic Golang SDK for interacting with the IBM Spectrum Virtualize. This SDK allows developers to easily authenticate, send requests, and handle responses from the IBM Spectrum Virtualize API.

## Features

- Easy configuration and authentication
- RESTful API interaction with structured request/response models
- Built-in error handling and response parsing

## Installation

```bash
go get github.com/mkumatag/svc-go-sdk
```

## Usage

Import the SDK

```go
import "github.com/yourusername/example-api-sdk/sdk"
```

Initialize the Client

```go
client := svc.NewClient("svc-hostname-or-ip", "username", "REDACTED_HMC_PASS<==")
```

Authentication

```go
if err := client.Authenticate(); err != nil {
    log.Fatalf("auth error: %v", err)
}
fmt.Println("Authenticated")
```

Example: List system

```go
systemInfo, err := client.Lssystem()
if err != nil {
    log.Fatalf("lssystem error: %v", err)
}
fmt.Printf("System: %+v\n", systemInfo)
```

Authenticating with insecure TLS (not recommended for production use)

```go
client := svc.NewClient("svc-hostname-or-ip", "username", "REDACTED_HMC_PASS<==").WithTLSInsecure()
```
