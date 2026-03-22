package main

import (
	"flag"
	"fmt"
	"log"
	"strings"

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
	mediaNamesStr := flag.String("media-names", "test_iso", "Comma-separated list of ISO files to map (e.g., 'rhel9.iso,aix73.iso')")
	
	verbose := flag.Bool("verbose", false, "Enable verbose output")
	flag.Parse()

	if *password == "" || *viosName == "" || *lparName == "" || *mediaNamesStr == "" {
		log.Fatal("Error: hmc-pass, vios-name, lpar-name, and media-names are required.")
	}

	// Parse comma-separated media names
	mediaNames := strings.Split(*mediaNamesStr, ",")
	for i := range mediaNames {
		mediaNames[i] = strings.TrimSpace(mediaNames[i])
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

	_,lparUUID, err := restClient.GetLogicalPartitionByName(sysUUID, *lparName, *verbose)
	if err != nil || lparUUID == "" {
		log.Fatalf("❌ LPAR '%s' not found.", *lparName)
	}

	// =========================================================================
	// EXECUTE OPTICAL MAPPING (BATCH OPERATION)
	// =========================================================================
	fmt.Printf("\n⚠️  Attempting to map %d Virtual Optical Media from VIOS '%s' to LPAR '%s'...\n", len(mediaNames), *viosName, *lparName)
	fmt.Printf("Media to map: %v\n", mediaNames)

	mappingUUID, err := restClient.CreateVirtualOpticalMaps(sysUUID, viosUUID, lparUUID, mediaNames, *verbose)
	if err != nil {
		log.Fatalf("❌ Storage Mapping Failed: %v", err)
	}

	fmt.Printf("\n💿 Successfully created %d Virtual Optical Device(s) and loaded media! Status: %s\n", len(mediaNames), mappingUUID)
}
