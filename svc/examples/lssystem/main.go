package main

import (
	"log"
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/IBM/infra-go-sdk/svc" // Adjust if your package path differs
)

func main() {
	// Command line flags
	verbose := flag.Bool("verbose", false, "Enable verbose output")
	svcIP := flag.String("svc-ip", "", "SVC IP address (required)")
	svcUser := flag.String("svc-user", "", "SVC username (required)")
	svcPass := flag.String("svc-pass", "", "SVC password (required)")
	flag.Parse()

	if *svcIP == "" || *svcUser == "" || *svcPass == "" {
		log.Fatal("Usage: lssystem -svc-ip <ip> -svc-user <user> -svc-pass <pass>")
	}

	ctx := context.Background()
	// Initialize the client
	client := svc.NewClient(*svcIP, *svcUser, *svcPass).WithTLSInsecure()

	// Enable debug logging if the verbose flag is passed
	if *verbose {
		log.Printf("Verbose mode enabled. Connecting to SVC.: ip=%v user=%v", *svcIP, *svcUser)
	}

	if err := client.Authenticate(ctx); err != nil {
		log.Printf("Authentication error: error=%v", err)
		os.Exit(1)
	}
	log.Println("✅ Authenticated")

	log.Println("Fetching system information...")

	// systemInfo is returned as a *svc.SystemInfo pointer
	systemInfo, err := client.Lssystem(ctx)
	if err != nil {
		log.Printf("lssystem error: error=%v", err)
		os.Exit(1)
	}

	log.Println("✅ System information retrieved successfully!")

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