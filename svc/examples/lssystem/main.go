package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.ibm.com/sudeeshjohn/infra-go-sdk/svc" // Adjust if your package path differs
)

func main() {
	// Command line flags
	verbose := flag.Bool("verbose", false, "Enable verbose output")
	svcIP := flag.String("svc-ip", "REDACTED_SVC_IP<==", "SVC IP address")
	svcUser := flag.String("svc-user", "REDACTED_SVC_USER<==", "SVC username")
	svcPass := flag.String("svc-pass", "REDACTED_SVC_PASS<==", "SVC password")
	flag.Parse()
	ctx := context.Background()
	// Initialize the client
	client := svc.NewClient(*svcIP, *svcUser, *svcPass).WithTLSInsecure()

	// Enable debug logging if the verbose flag is passed
	if *verbose {
		client = client.WithDebug()
		client.Logger.Debug("Verbose mode enabled. Connecting to SVC.", "ip", *svcIP, "user", *svcUser)
	}

	if err := client.Authenticate(ctx); err != nil {
		client.Logger.Error("Authentication error", "error", err)
		os.Exit(1)
	}
	client.Logger.Info("✅ Authenticated")

	client.Logger.Info("Fetching system information...")

	// systemInfo is returned as a *svc.SystemInfo pointer
	systemInfo, err := client.Lssystem(ctx)
	if err != nil {
		client.Logger.Error("lssystem error", "error", err)
		os.Exit(1)
	}

	client.Logger.Info("✅ System information retrieved successfully!")

	// ---------------------------------------------------------
	// Option 1: Print specific fields directly from the struct
	// ---------------------------------------------------------
	fmt.Println("\n--- Custom Selected Details ---")
	fmt.Printf("System Name       : %s\n", systemInfo.Name)
	fmt.Printf("System ID         : %s\n", systemInfo.ID)
	fmt.Printf("Code Level        : %s\n", systemInfo.CodeLevel)
	fmt.Printf("Total Capacity    : %s\n", systemInfo.TotalMDiskCapacity)
	fmt.Printf("Total Free Space  : %s\n", systemInfo.TotalFreeSpace)
	fmt.Printf("Primary Contact   : %s (%s)\n", systemInfo.EmailContact, systemInfo.EmailContactPrimary)

	// ---------------------------------------------------------
	// Option 2: Dump the entire struct with field names using %+v
	// ---------------------------------------------------------
	if *verbose {
		fmt.Println("\n--- Full Struct Dump ---")
		fmt.Printf("%+v\n", systemInfo)
	}
}