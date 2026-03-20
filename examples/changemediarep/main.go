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
	viosName := flag.String("vios-name", "ltc09u31-vios1", "Target VIOS")
	
	// Incremental size
	addSize := flag.Int("add-size", 10240, "Amount of additional space to add in MB")
	
	verbose := flag.Bool("verbose", false, "Enable verbose output")
	flag.Parse()

	if *password == "" || *viosName == "" || *addSize <= 0 {
		log.Fatal("Error: hmc-pass, vios-name, and a valid add-size (>0) are required.")
	}

	// =========================================================================
	// AUTHENTICATION & RESOLUTION
	// =========================================================================
	restClient := hmc.NewHmcRestClient(*hmcIP)
	if err := restClient.Login(*username, *password, *verbose); err != nil {
		log.Fatalf("HMC Logon failed: %v", err)
	}
	defer restClient.Logoff()

	sysUUID, _, err := restClient.GetManagedSystemByName(*sysName, *verbose)
	if err != nil || sysUUID == "" {
		log.Fatalf("❌ System '%s' not found.", *sysName)
	}

	viosUUID, err := hmc.GetViosID(restClient, sysUUID, *viosName, *verbose)
	if err != nil || viosUUID == "" {
		log.Fatalf("❌ VIOS '%s' not found.", *viosName)
	}

	// =========================================================================
	// EXECUTE SMART EXPANSION
	// =========================================================================
	fmt.Printf("\n🚀 Attempting to add %d MB to the Media Repository on VIOS '%s'...\n", *addSize, *viosName)

	err = restClient.ChangeMediaRepository(*sysName, viosUUID, *viosName, *addSize, *verbose)
	if err != nil {
		log.Fatalf("❌ Expansion Failed: %v", err)
	}

	fmt.Printf("\n📈 Successfully increased the Virtual Media Repository size!\n")
}
