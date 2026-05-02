package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"

	hmc "github.com/sudeeshjohn/powerhmc-go" // Adjust to your actual package path
)

func main() {
	// =========================================================================
	// CONFIGURATION - Command Line Flags
	// =========================================================================
	hmcIP := flag.String("hmc-ip", "192.0.2.1", "HMC IP address")
	username := flag.String("hmc-user", "REDACTED_HMC_USER<==", "HMC username")
	password := flag.String("hmc-pass", "REDACTED_HMC_PASS<==", "HMC password")
	sysName := flag.String("system-name", "LTC09U31-ZZ", "Managed System Name")
	verbose := flag.Bool("verbose", false, "Enable verbose output")

	flag.Parse()

	// Quick validation to ensure required fields aren't completely empty if passed empty via CLI
	if *hmcIP == "" || *username == "" || *password == "" || *sysName == "" {
		log.Fatal("Error: hmc-ip, hmc-user, hmc-pass, and system-name are all required.")
	}

	// =========================================================================
	// PHASE 1: HMC AUTHENTICATION
	// =========================================================================
	fmt.Printf("Logging into HMC at %s...\n", *hmcIP)
	restClient := hmc.NewHmcRestClient(*hmcIP)
	
	if err := restClient.Login(context.Background(), *username, *password, *verbose); err != nil {
		log.Fatalf("HMC Logon failed: %v", err)
	}
	defer restClient.Logoff(context.Background())

	fmt.Println("✅ Successfully authenticated with HMC.")

	// =========================================================================
	// PHASE 2: RESOLVE MANAGED SYSTEM & VIOS UUIDs
	// =========================================================================
	fmt.Printf("\nStep 1: Locating System [%s]...\n", *sysName)
	sysUUID, _, err := restClient.GetManagedSystemByName(context.Background(), *sysName, *verbose)
	if err != nil || sysUUID == "" {
		log.Fatalf("System %s not found: %v", *sysName, err)
	}
	fmt.Printf("✅ Found System UUID: %s\n", sysUUID)

	fmt.Printf("Step 2: Fetching Virtual I/O Servers for System...\n")
	viosList, err := restClient.GetVirtualIOServersQuick(context.Background(), sysUUID, *verbose)
	if err != nil {
		log.Fatalf("Failed to fetch VIOS partitions: %v", err)
	}

	if len(viosList) == 0 {
		fmt.Println("No Virtual I/O Servers found on this managed system.")
		return
	}
	fmt.Printf("✅ Found %d Virtual I/O Server(s).\n", len(viosList))

	// =========================================================================
	// PHASE 3: FETCH AND PRINT VIRTUAL SCSI SERVER ADAPTERS (VHOSTS)
	// =========================================================================
	fmt.Println("\nStep 3: Retrieving Virtual SCSI Server Adapters...")

	for i, vios := range viosList {
		fmt.Printf("\n======================================================\n")
		fmt.Printf(" VIOS %d: %s (UUID: %s)\n", i+1, vios.PartitionName, vios.UUID)
		fmt.Printf("======================================================\n")

		// Call the new SDK method
		adapters, err := restClient.GetVirtualSCSIServerAdapters(vios.UUID, *verbose)
		if err != nil {
			log.Printf("⚠️ Warning: Failed to fetch adapters for VIOS %s: %v", vios.PartitionName, err)
			continue
		}

		if len(adapters) == 0 {
			fmt.Println("  No Virtual SCSI Server Adapters (vhost) found.")
			continue
		}

		fmt.Printf("  Found %d vhost adapter(s):\n\n", len(adapters))

		// Pretty-print the slice of structs as JSON
		output, err := json.MarshalIndent(adapters, "", "  ")
		if err != nil {
			log.Printf("⚠️ Warning: Failed to format JSON output: %v", err)
			continue
		}

		fmt.Println(string(output))
	}
}
