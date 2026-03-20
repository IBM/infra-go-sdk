package main

import (
	"flag"
	"fmt"
	"log"
	"time"

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
	mediaName := flag.String("media-name", fmt.Sprintf("test_iso_%d", time.Now().Unix()), "Name of the Virtual Optical Media inside the repository")
	fileName := flag.String("file-name", "/home/padmin/test.iso", "Absolute path to the existing ISO file on the VIOS filesystem")
	verbose := flag.Bool("verbose", false, "Enable verbose output")
	flag.Parse()

	if *password == "" || *viosName == "" || *mediaName == "" || *fileName == "" {
		log.Fatal("Error: hmc-pass, vios-name, media-name, and file-name are required.")
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

	system, _, err := restClient.GetManagedSystemByNameQuick(*sysName, *verbose)
	if err != nil || system.UUID == "" {
		log.Fatalf("❌ System '%s' not found: %v", *sysName, err)
	}

	viosUUID, err := hmc.GetViosID(restClient, system.UUID, *viosName, *verbose)
	if err != nil || viosUUID == "" {
		log.Fatalf("❌ VIOS '%s' not found.", *viosName)
	}

	// =========================================================================
	// EXECUTE NATIVE REST ISO IMPORT
	// =========================================================================
	fmt.Printf("\n🚀 Attempting to import ISO '%s' into the Media Repository as '%s'...\n", *fileName, *mediaName)

	err = restClient.AddVirtualOpticalMedia(viosUUID, *mediaName, *fileName, *verbose)
	if err != nil {
		log.Fatalf("❌ Failed to add Optical Media: %v", err)
	}

	fmt.Printf("\n💿 Successfully imported Virtual Optical Media '%s' natively via REST!\n", *mediaName)
}
