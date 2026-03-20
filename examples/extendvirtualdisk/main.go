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
	
	viosName := flag.String("vios-name", "ltc09u31-vios1", "Target VIOS (Required)")
	diskName := flag.String("disk-name", "auto_lv01", "Name of the Virtual Disk to extend")
	addSize := flag.Int("add-size", 555120, "Amount of additional space to add in Megabytes")
	
	verbose := flag.Bool("verbose", false, "Enable verbose output")
	flag.Parse()

	if *password == "" || *viosName == "" || *diskName == "" || *addSize <= 0 {
		log.Fatal("Error: hmc-pass, vios-name, disk-name, and a valid add-size (>0) are required.")
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

	// =========================================================================
	// RESOLVE SYSTEM & VIOS UUID
	// =========================================================================
	fmt.Printf("\nResolving System Name: %s...\n", *sysName)
	sysUUID, _, err := restClient.GetManagedSystemByName(*sysName, *verbose)
	if err != nil || sysUUID == "" {
		log.Fatalf("❌ System '%s' not found: %v", *sysName, err)
	}

	// We must find the VIOS UUID so the SDK can use it for the capacity check
	viosUUID, err := hmc.GetViosID(restClient, sysUUID, *viosName, *verbose)
	if err != nil || viosUUID == "" {
		log.Fatalf("❌ VIOS '%s' not found on system '%s'.", *viosName, *sysName)
	}

	// =========================================================================
	// EXECUTE EXTENSION
	// =========================================================================
	fmt.Printf("\n🚀 Attempting to extend Virtual Disk '%s' on VIOS '%s' by %d MB...\n", *diskName, *viosName, *addSize)

	// FIXED: Passed viosUUID as the second argument
	err = restClient.ExtendVirtualDisk(*sysName, viosUUID, *viosName, *diskName, *addSize, *verbose)
	if err != nil {
		log.Fatalf("❌ Failed to extend Virtual Disk: %v", err)
	}

	fmt.Printf("\n📈 Successfully added %d MB to Virtual Disk '%s'!\n", *addSize, *diskName)
}