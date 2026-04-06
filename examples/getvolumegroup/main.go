package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"

	hmc "github.com/sudeeshjohn/powerhmc-go" // Adjust to your actual package path
)

func main() {
	// =========================================================================
	// CONFIGURATION & FLAGS
	// =========================================================================
	hmcIP := flag.String("hmc-ip", "192.0.2.2", "HMC IP address")
	username := flag.String("hmc-user", "REDACTED_HMC_USER<==", "HMC username")
	password := flag.String("hmc-pass", "7UGadmin1Q2024", "HMC password")
	sysName := flag.String("system-name", "LTC13U29-Ranier", "Managed System Name")
	verbose := flag.Bool("verbose", true, "Enable verbose output")
	flag.Parse()

	if *hmcIP == "" || *username == "" || *password == "" || *sysName == "" {
		log.Fatal("Error: hmc-ip, hmc-user, hmc-pass, and system-name are required.")
	}

	// =========================================================================
	// AUTHENTICATION
	// =========================================================================
	fmt.Printf("Logging into HMC at %s...\n", *hmcIP)
	restClient := hmc.NewHmcRestClient(*hmcIP)
	if err := restClient.Login(*username, *password, *verbose); err != nil {
		log.Fatalf("HMC Logon failed: %v", err)
	}
	defer restClient.Logoff()
	fmt.Println("✅ Successfully authenticated with HMC.")

	// =========================================================================
	// 1. DYNAMIC SYSTEM & VIOS DISCOVERY
	// =========================================================================
	fmt.Printf("\nResolving System Name: %s...\n", *sysName)
	sysUUID, _, err := restClient.GetManagedSystemByName(*sysName, *verbose)
	if err != nil || sysUUID == "" {
		log.Fatalf("❌ System %s not found: %v", *sysName, err)
	}

	fmt.Println("Discovering Virtual I/O Servers...")
	viosList, err := restClient.GetVirtualIOServersQuick(sysUUID, *verbose)
	if err != nil {
		log.Fatalf("❌ Failed to fetch VIOS instances: %v", err)
	}
	if len(viosList) == 0 {
		log.Fatalf("No Virtual I/O Servers found on system %s.", *sysName)
	}

	// =========================================================================
	// 2. DISCOVER AND FETCH SPECIFIC VOLUME GROUPS
	// =========================================================================
	for _, vios := range viosList {
		fmt.Printf("\n===============================================================================\n")
		fmt.Printf(" VIOS: %s (UUID: %s)\n", vios.PartitionName, vios.UUID)
		fmt.Printf("===============================================================================\n")

		// First, get the list of Volume Groups to discover their UUIDs
		vgList, err := restClient.GetVolumeGroups(vios.UUID, *verbose)
		if err != nil {
			log.Printf("⚠️ Warning: Failed to fetch Volume Groups list for %s: %v", vios.PartitionName, err)
			continue
		}

		if len(vgList) == 0 {
			fmt.Println("  No Volume Groups found on this VIOS.")
			continue
		}

		fmt.Printf("  Discovered %d Volume Group(s). Fetching specific details for each...\n", len(vgList))

		// Now, loop through the discovered UUIDs and fetch their specific details
		for _, discoveredVG := range vgList {
			fmt.Printf("\n  -> Fetching specific details for Volume Group '%s' (UUID: %s)...\n", discoveredVG.GroupName, discoveredVG.UUID)

			// Call the new targeted API function
			detailedVG, err := restClient.GetVolumeGroup(vios.UUID, discoveredVG.UUID, *verbose)
			if err != nil {
				log.Printf("  ❌ Failed to fetch specific details for %s: %v", discoveredVG.GroupName, err)
				continue
			}

			// Pretty-print the deeply nested struct as JSON
			output, err := json.MarshalIndent(detailedVG, "     ", "  ")
			if err != nil {
				log.Printf("  ⚠️ Warning: Failed to format JSON output: %v", err)
				continue
			}

			fmt.Println("     " + string(output))
		}
	}
	
	fmt.Println("\n🎉 Dynamic Volume Group discovery complete.")
}