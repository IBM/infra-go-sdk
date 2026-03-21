package main

import (
	"flag"
	"fmt"
	"log"

	hmc "github.com/sudeeshjohn/powerhmc-go"
)

func main() {
	// =========================================================================
	// CONFIGURATION & FLAGS
	// =========================================================================
	hmcIP := flag.String("hmc-ip", "192.0.2.1", "HMC IP address")
	username := flag.String("hmc-user", "REDACTED_HMC_USER<==", "HMC username")
	password := flag.String("hmc-pass", "REDACTED_HMC_PASS<==", "HMC password")
	sysName := flag.String("system-name", "LTC09U31-ZZ", "Managed System Name")
	viosName := flag.String("vios-name", "ltc09u31-vios1", "Target VIOS Name")
	lparName := flag.String("lpar-name", "Go_LPAR_01", "Target LPAR Name")
	volumeName := flag.String("volume-name", "hdisk1", "Volume name to remove mapping")
	verbose := flag.Bool("verbose", false, "Enable verbose output")
	flag.Parse()

	if *password == "" || *sysName == "" || *viosName == "" || *lparName == "" || *volumeName == "" {
		log.Fatal("Error: hmc-pass, system-name, vios-name, lpar-name, and volume-name are required.")
	}

	// =========================================================================
	// AUTHENTICATION & RESOLUTION
	// =========================================================================
	restClient := hmc.NewHmcRestClient(*hmcIP)
	if err := restClient.Login(*username, *password, *verbose); err != nil {
		log.Fatalf("HMC Logon failed: %v", err)
	}
	defer restClient.Logoff()

	// Resolve System UUID
	systems, _, err := restClient.GetManagedSystemByNameQuick(*sysName, *verbose)
	if err != nil || systems.UUID == "" {
		log.Fatalf("❌ System '%s' not found.", *sysName)
	}
	sysUUID := systems.UUID

	// Resolve VIOS UUID
	viosUUID, err := hmc.GetViosID(restClient, sysUUID, *viosName, *verbose)
	if err != nil || viosUUID == "" {
		log.Fatalf("❌ VIOS '%s' not found.", *viosName)
	}

	// Resolve LPAR UUID
	_, lparUUID, err := restClient.GetLogicalPartitionByName(sysUUID, *lparName, *verbose)
	if err != nil || lparUUID == "" {
		log.Fatalf("❌ LPAR '%s' not found.", *lparName)
	}

	// =========================================================================
	// REMOVE VOLUME MAPPING
	// =========================================================================
	fmt.Printf("\n⚠️  Attempting to remove mapping for volume '%s' from VIOS '%s' to LPAR '%s'...\n", *volumeName, *viosName, *lparName)

	results, err := restClient.RemoveVolumeLPARMapping(viosUUID, lparUUID, []string{*volumeName}, *verbose)
	if err != nil {
		log.Fatalf("❌ Failed to remove volume-LPAR mapping: %v", err)
	}

	if len(results) > 0 {
		result := results[0]
		fmt.Printf("\n✅ Successfully removed mapping for volume '%s'\n", result.VolumeName)
		fmt.Printf("   - VTD Name: %s\n", result.VTDName)
		fmt.Printf("   - Client Slot: %s\n", result.ClientSlotNumber)
		fmt.Printf("   - Server Slot: %s\n", result.ServerSlotNumber)
		if result.ServerAdapterDeleteURL != "" {
			fmt.Printf("   - Server Adapter URL: %s\n", result.ServerAdapterDeleteURL)
		}
	}

	fmt.Println("\n🎉 Volume mapping removal completed successfully!")
}
