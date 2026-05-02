package main

import (
	"context"
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
	systemName := flag.String("system-name", "LTC09U31-ZZ", "Managed system name")
	lparName := flag.String("lpar-name", "Go_LPAR_100", "Name of the LPAR")
	profileName := flag.String("profile-name", "", "Profile name to delete (required)")
	verbose := flag.Bool("verbose", true, "Enable verbose logging")

	flag.Parse()

	// Validate required parameters
	if *profileName == "" {
		log.Fatal("Error: -profile-name is required\n\n" +
			"Usage: go run main.go -profile-name=<profile>\n" +
			"Example: go run main.go -profile-name=test_profile\n\n" +
			"WARNING: This will permanently delete the profile!")
	}

	fmt.Println("=== Delete Partition Profile Example ===")
	fmt.Printf("⚠️  WARNING: This will permanently delete profile '%s'\n", *profileName)
	fmt.Println()

	// =========================================================================
	// AUTHENTICATION
	// =========================================================================
	restClient := hmc.NewHmcRestClient(*hmcIP)
	if err := restClient.Login(context.Background(), *username, *password, *verbose); err != nil {
		log.Fatalf("HMC Logon failed: %v", err)
	}
	defer restClient.Logoff(context.Background())

	// =========================================================================
	// RESOLVE SYSTEM UUID
	// =========================================================================
	fmt.Printf("Resolving managed system: %s\n", *systemName)
	
	systemUUID, system, err := restClient.GetManagedSystemByName(context.Background(), *systemName, *verbose)
	if err != nil {
		log.Fatalf("❌ Failed to get managed system: %v", err)
	}
	
	fmt.Printf("✅ Found system: %s (UUID: %s)\n", system.SystemName, systemUUID)
	fmt.Println()

	// =========================================================================
	// RESOLVE LPAR UUID
	// =========================================================================
	fmt.Printf("Resolving LPAR: %s\n", *lparName)
	
	_, lparUUID, err := restClient.GetLogicalPartitionByName(context.Background(), systemUUID, *lparName, *verbose)
	if err != nil {
		log.Fatalf("❌ Failed to get LPAR: %v", err)
	}
	
	fmt.Printf("✅ Found LPAR: %s (UUID: %s)\n", *lparName, lparUUID)
	fmt.Println()

	// =========================================================================
	// RESOLVE PROFILE UUID
	// =========================================================================
	fmt.Printf("Resolving profile UUID for: %s\n", *profileName)
	
	quickProfiles, err := restClient.GetPartitionProfiles(lparUUID, *verbose)
	if err != nil {
		log.Fatalf("❌ Failed to retrieve partition profiles: %v", err)
	}

	var profileUUID string
	for _, p := range quickProfiles {
		if p.ProfileName == *profileName {
			profileUUID = p.UUID
			break
		}
	}

	if profileUUID == "" {
		log.Fatalf("❌ Profile '%s' not found for LPAR '%s'", *profileName, *lparName)
	}

	fmt.Printf("✅ Found profile: %s (UUID: %s)\n", *profileName, profileUUID)
	fmt.Println()

	// =========================================================================
	// DELETE PARTITION PROFILE
	// =========================================================================
	fmt.Printf("⚠️  Deleting profile '%s'...\n", *profileName)
	fmt.Println()

	err = restClient.DeleteLogicalPartitionProfile(lparUUID, profileUUID, *verbose)
	if err != nil {
		log.Fatalf("❌ Failed to delete partition profile: %v", err)
	}

	fmt.Println("═════════════════════════════════════════════════════════════")
	fmt.Printf("✅ Profile '%s' deleted successfully!\n", *profileName)
	fmt.Println("═════════════════════════════════════════════════════════════")
	fmt.Println()
	fmt.Println("⚠️  Important Notes:")
	fmt.Println("   - The profile has been permanently removed")
	fmt.Println("   - You cannot delete a profile that is currently in use")
	fmt.Println("   - You cannot delete the last remaining profile of a partition")
	fmt.Println("   - Consider backing up profile configurations before deletion")
}

// Made with Bob