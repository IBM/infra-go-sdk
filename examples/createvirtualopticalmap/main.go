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
	lparName := flag.String("lpar-name", "Go_LPAR_04", "Target LPAR Name")
	mediaName := flag.String("media-name", "test_iso", "Name of the ISO file to map")
	
	verbose := flag.Bool("verbose", false, "Enable verbose output")
	flag.Parse()

	if *password == "" || *viosName == "" || *lparName == "" || *mediaName == "" {
		log.Fatal("Error: hmc-pass, vios-name, lpar-name, and media-name are required.")
	}

	// =========================================================================
	// AUTHENTICATION & RESOLUTION
	// =========================================================================
	fmt.Printf("Logging into HMC at %s...\n", *hmcIP)
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

	lparUUID, err := restClient.GetLogicalPartitionByName(sysUUID, *lparName, *verbose)
	if err != nil || lparUUID == "" {
		log.Fatalf("❌ LPAR '%s' not found.", *lparName)
	}

	// =========================================================================
	// EXECUTE OPTICAL MAPPING
	// =========================================================================
	fmt.Printf("\n⚠️  Attempting to map Virtual Optical Media '%s' from VIOS '%s' to LPAR '%s'...\n", *mediaName, *viosName, *lparName)

	mappingUUID, err := restClient.CreateVirtualOpticalMap(sysUUID, viosUUID, lparUUID, *mediaName, *verbose)
	if err != nil {
		log.Fatalf("❌ Storage Mapping Failed: %v", err)
	}

	fmt.Printf("\n💿 Successfully created Virtual Optical Device and loaded media! Result: %s\n", mappingUUID)
}
