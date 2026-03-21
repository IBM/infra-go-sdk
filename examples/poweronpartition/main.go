package main

import (
	"flag"
	"fmt"
	"log"

	hmc "github.com/sudeeshjohn/powerhmc-go" // Adjust to your actual package path
)

func main() {
	// =========================================================================
	// CONFIGURATION & FLAGS
	// =========================================================================
	hmcIP := flag.String("hmc-ip", "192.0.2.1", "HMC IP address")
	username := flag.String("hmc-user", "REDACTED_HMC_USER<==", "HMC username")
	password := flag.String("hmc-pass", "REDACTED_HMC_PASS<==", "HMC password")
	sysName := flag.String("system-name", "LTC09U31-ZZ", "Managed System Name")
	lparName := flag.String("lpar-name", "Go_LPAR_91", "Target LPAR Name")
	verbose := flag.Bool("verbose", false, "Enable verbose output")
	
	flag.Parse()

	// Validation
	if *password == "" || *sysName == "" || *lparName == "" {
		log.Fatal("❌ Error: hmc-pass, system-name, and lpar-name are required.")
	}

	// =========================================================================
	// AUTHENTICATION
	// =========================================================================
	restClient := hmc.NewHmcRestClient(*hmcIP)

	if *verbose {
		log.Printf("Attempting to log on to HMC at %s with username %s", *hmcIP, *username)
	}
	if err := restClient.Login(*username, *password, *verbose); err != nil {
		if *verbose {
			log.Fatalf("Logon failed: %v", err)
		}
		log.Fatal("❌ Logon failed. (Run with -verbose for details)")
	}
	defer func() {
		if err := restClient.Logoff(); err != nil {
			if *verbose {
				log.Printf("Logoff failed: %v", err)
			}
		} else if *verbose {
			log.Println("Logged off successfully")
		}
	}()

	// =========================================================================
	// DYNAMIC RESOLUTION & STATE CHECK
	// =========================================================================
	
	// 1. Resolve System UUID from System Name 
	if *verbose {
		fmt.Printf("\nResolving System UUID for '%s'...\n", *sysName)
	}
	_, sysUUID, err := restClient.GetManagedSystemByNameQuick(*sysName, *verbose)
	if err != nil || sysUUID == "" {
		if *verbose {
			log.Fatalf("System '%s' not found: %v", *sysName, err)
		}
		log.Fatalf("❌ System '%s' not found.", *sysName)
	}

	// 2. Resolve LPAR UUID and Details from LPAR Name 
	if *verbose {
		fmt.Printf("Resolving LPAR UUID for '%s'...\n", *lparName)
	}
	lpar, partUUID, err := restClient.GetLogicalPartitionByName(sysUUID, *lparName, *verbose)
	if err != nil || partUUID == "" {
		if *verbose {
			log.Fatalf("LPAR '%s' not found: %v", *lparName, err)
		}
		log.Fatalf("❌ LPAR '%s' not found.", *lparName)
	}
	
	if *verbose {
		fmt.Printf("✅ Found LPAR UUID: %s\n", partUUID)
		fmt.Printf("🔍 Current LPAR State: %s\n", lpar.PartitionState)
	}

	// -> NEW: Check the state before proceeding
	if lpar.PartitionState == "running" {
		// Even if not verbose, it's good practice to let the user know why we skipped
		fmt.Printf("⚠️ LPAR '%s' is already running. Skipping Power On.\n", *lparName)
		return
	}

	// 3. Resolve Profile UUID from LPAR UUID 
	if *verbose {
		fmt.Printf("Resolving default Profile UUID for LPAR '%s'...\n", *lparName)
	}
	profileUUID, err := restClient.GetPartitionProfile(partUUID, *verbose)
	if err != nil || profileUUID == "" {
		if *verbose {
			log.Fatalf("Failed to retrieve partition profile: %v", err)
		}
		log.Fatal("❌ Failed to retrieve partition profile.")
	}
	if *verbose {
		fmt.Printf("✅ Found Profile UUID: %s\n\n", profileUUID)
	}

	// =========================================================================
	// EXECUTE POWER ON
	// =========================================================================
	if *verbose {
		fmt.Println("Initiating Power On...")
	}
	status, err := restClient.PowerOnPartition(partUUID, profileUUID, "normal", "", "", *verbose)
	if err != nil {
		if *verbose {
			log.Fatalf("Failed to power on partition: %v", err)
		}
		log.Fatal("❌ Failed to power on partition.")
	}
	
	// This will print the status seamlessly.
	fmt.Printf("🚀 PowerOn Job Status: %s\n", status)
}