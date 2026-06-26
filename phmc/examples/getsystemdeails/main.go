package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"

	hmc "github.com/IBM/infra-go-sdk/phmc" // Adjust to your actual package path
)

func main() {
	// =========================================================================
	// CONFIGURATION & FLAGS
	// =========================================================================
	hmcIP := flag.String("hmc-ip", "", "HMC IP address")
	username := flag.String("hmc-user", "", "HMC username")
	password := flag.String("hmc-pass", "", "HMC password")
	sysName := flag.String("system-name", "", "Managed System Name")
	verbose := flag.Bool("verbose", false, "Enable verbose output")
	flag.Parse()

	if *password == "" || *sysName == "" {
		log.Fatal("Error: hmc-pass and system-name are required.")
	}

	// =========================================================================
	// AUTHENTICATION
	// =========================================================================
	fmt.Printf("Logging into HMC at %s...\n", *hmcIP)
	restClient := hmc.NewRestClient(*hmcIP)
	if err := restClient.Login(context.Background(), *username, *password, *verbose); err != nil {
		log.Fatalf("❌ HMC Logon failed: %v", err)
	}
	defer restClient.Logoff(context.Background())

	// =========================================================================
	// RESOLVE SYSTEM UUID
	// =========================================================================
	fmt.Printf("Resolving Managed System '%s'...\n", *sysName)
	_, sysUUID, err := restClient.GetManagedSystemByNameQuick(context.Background(), *sysName, *verbose)
	if err != nil || sysUUID == "" {
		log.Fatalf("❌ System '%s' not found: %v", *sysName, err)
	}

	// =========================================================================
	// FETCH COMPREHENSIVE DETAILS
	// =========================================================================
	fmt.Printf("Fetching comprehensive XML payload for system UUID: %s...\n", sysUUID)
	
	detailedSystem, err := restClient.GetManagedSystem(context.Background(), sysUUID, *verbose)
	if err != nil {
		log.Fatalf("❌ Failed to fetch detailed system info: %v", err)
	}

	// =========================================================================
	// DISPLAY RESULTS
	// =========================================================================
	fmt.Printf("\n✅ Successfully retrieved deep configuration for: %s\n", detailedSystem.SystemName)
	fmt.Println("=========================================================================")

	// Marshal the Go struct into beautifully indented JSON for the terminal
	prettyJSON, err := json.MarshalIndent(detailedSystem, "", "    ")
	if err != nil {
		log.Fatalf("❌ Failed to format output: %v", err)
	}

	fmt.Println(string(prettyJSON))
	fmt.Println("=========================================================================")
	
	fmt.Printf("\n📊 Quick Stats extracted directly from Struct:\n")
	fmt.Printf("   - Installed Memory: %.0f MB\n", detailedSystem.MemoryConfig.InstalledSystemMemory)
	fmt.Printf("   - Active Migrations Allowed: %d\n", detailedSystem.MigrationInfo.MaximumActiveMigrations)
	fmt.Printf("   - Total Dedicated I/O Adapters Found: %d\n", len(detailedSystem.IOConfig.IOAdapters))
}