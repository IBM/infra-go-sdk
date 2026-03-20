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
	
	// Added repo-name to match the SDK check
	repoName := flag.String("repo-name", "VMLibrary", "Name of the repository to delete")
	force := flag.Bool("force", false, "Force deletion even if ISO media exists")
	
	verbose := flag.Bool("verbose", false, "Enable verbose output")
	flag.Parse()

	if *password == "" || *viosName == "" {
		log.Fatal("Error: hmc-pass and vios-name are required.")
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
	// EXECUTE SMART DELETION
	// =========================================================================
	fmt.Printf("\n⚠️  Attempting to safely remove Media Repository '%s' from '%s'...\n", *repoName, *viosName)

	err = restClient.DeleteMediaRepository(*sysName, viosUUID, *viosName, *repoName, *force, *verbose)
	if err != nil {
		log.Fatalf("❌ Deletion Failed: %v", err)
	}

	fmt.Printf("\n🗑️  Successfully removed the Virtual Media Repository.\n")
}
