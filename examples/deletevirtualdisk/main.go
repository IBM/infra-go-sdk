package main

import (
	"flag"
	"fmt"
	"log"

	hmc "github.com/sudeeshjohn/PowerHMC" // Adjust to your actual package path
)

func main() {
	// =========================================================================
	// CONFIGURATION & FLAGS
	// =========================================================================
	hmcIP := flag.String("hmc-ip", "192.0.2.1", "HMC IP address")
	username := flag.String("hmc-user", "REDACTED_HMC_USER<==", "HMC username")
	password := flag.String("hmc-pass", "REDACTED_HMC_PASS<==", "HMC password")
	sysName := flag.String("system-name", "LTC09U31-ZZ", "Managed System Name")
	
	// For destructive operations, we make vios-name mandatory for safety.
	viosName := flag.String("vios-name", "ltc09u31-vios1", "Target VIOS (Required)")
	diskName := flag.String("disk-name", "auto_lv01", "Name of the Virtual Disk to delete")
	
	verbose := flag.Bool("verbose", false, "Enable verbose output")
	flag.Parse()

	if *password == "" || *viosName == "" || *diskName == "" {
		log.Fatal("Error: hmc-pass, vios-name, and disk-name are strictly required for deletion.")
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
	// EXECUTE DELETION
	// =========================================================================
	fmt.Printf("\n⚠️ WARNING: Attempting to permanently delete Virtual Disk '%s' on VIOS '%s'...\n", *diskName, *viosName)

	err := restClient.DeleteVirtualDisk(*sysName, *viosName, *diskName, *verbose)
	if err != nil {
		log.Fatalf("❌ Failed to delete Virtual Disk: %v", err)
	}

	fmt.Printf("\n🗑️  Successfully deleted Virtual Disk '%s'!\n", *diskName)
}
