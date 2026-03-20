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
	vgName := flag.String("vg-name", "rootvg", "Storage Pool / Volume Group to build the repository in")
	repSize := flag.Int("rep-size", 20480, "Size of the Virtual Media Repository in Megabytes")
	
	verbose := flag.Bool("verbose", false, "Enable verbose output")
	flag.Parse()

	if *password == "" || *viosName == "" || *vgName == "" || *repSize <= 0 {
		log.Fatal("Error: hmc-pass, vios-name, vg-name, and a valid rep-size (>0) are required.")
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

	// Resolve the VIOS UUID for the capacity check
	viosUUID, err := hmc.GetViosID(restClient, sysUUID, *viosName, *verbose)
	if err != nil || viosUUID == "" {
		log.Fatalf("❌ VIOS '%s' not found on system '%s'.", *viosName, *sysName)
	}

	// =========================================================================
	// EXECUTE CREATION
	// =========================================================================
	fmt.Printf("\n🚀 Attempting to create a %d MB Virtual Media Repository in VG '%s' on VIOS '%s'...\n", *repSize, *vgName, *viosName)

	err = restClient.CreateMediaRepository(*sysName, viosUUID, *viosName, *vgName, *repSize, *verbose)
	if err != nil {
		log.Fatalf("❌ Failed to create Media Repository: %v", err)
	}

	fmt.Printf("\n📂 Successfully provisioned the Virtual Media Repository!\n")
}
